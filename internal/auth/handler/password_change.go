package handler

import (
	"errors"
	"io"

	"github.com/gin-gonic/gin"

	"vitamins-backend_2/internal/auth/middleware"
	"vitamins-backend_2/internal/auth/service"
)

// PasswordChangeVerifyRequest — body for /users/me/password/change/verify.
type PasswordChangeVerifyRequest struct {
	Code string `json:"code"`
}

// PasswordChangeConfirmRequest — body for /users/me/password/change/confirm.
type PasswordChangeConfirmRequest struct {
	ChangeToken     string `json:"changeToken"`
	Password        string `json:"password"`
	PasswordConfirm string `json:"passwordConfirm"`
}

// PasswordChangeVerifyResponse — successful response of …/change/verify.
type PasswordChangeVerifyResponse struct {
	ChangeToken string `json:"changeToken"`
}

// RequestPasswordChange godoc
// @Summary      Запрос кода смены пароля
// @Description  Отправляет код на e-mail текущего пользователя
// @Tags         users
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200
// @Failure      401 {object} ErrorResponse "AUTH_REQUIRED, INVALID_TOKEN"
// @Failure      404 {object} ErrorResponse "USER_NOT_FOUND"
// @Failure      429 {object} ErrorResponse "TOO_MANY_REQUESTS"
// @Failure      500 {object} ErrorResponse "MAILER_NOT_CONFIGURED, REDIS_NOT_CONFIGURED"
// @Router       /users/me/password/change/request [post]
func (h *Handler) RequestPasswordChange(c *gin.Context) {
	var r struct{}
	if err := c.ShouldBindJSON(&r); err != nil && !errors.Is(err, io.EOF) {
		send(c, 400, "BAD_REQUEST", "Неверный формат запроса")
		return
	}
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		send(c, 401, "AUTH_REQUIRED", "Требуется авторизация")
		return
	}
	ctx := c.Request.Context()
	if err := h.s.RequestPasswordChange(ctx, userID); err != nil {
		switch err {
		case service.ErrTooManyRequests:
			send(c, 429, "TOO_MANY_REQUESTS", "Слишком часто. Попробуйте позже.")
		case service.ErrMailerNotConfigured:
			send(c, 500, "MAILER_NOT_CONFIGURED", "Сервис отправки писем не настроен")
		case service.ErrRedisNotConfigured:
			send(c, 500, "REDIS_NOT_CONFIGURED", "Кэш не настроен")
		case service.ErrUserNotFound:
			send(c, 404, "USER_NOT_FOUND", "Пользователь не найден")
		default:
			logApp(c).ErrorContext(ctx, "password change request failed", "operation", "auth.password_change.request", "user_id", userID, "error", err.Error())
			send(c, 500, "INTERNAL_ERROR", "Что-то пошло не так.")
		}
		return
	}
	logAudit(c).InfoContext(ctx, "password change requested", "operation", "auth.password_change.request", "user_id", userID)
	c.Status(200)
}

// VerifyPasswordChangeCode godoc
// @Summary      Проверка кода смены пароля
// @Description  Проверяет код и возвращает changeToken
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        request body PasswordChangeVerifyRequest true "Код из письма"
// @Security     BearerAuth
// @Success      200 {object} PasswordChangeVerifyResponse
// @Failure      400 {object} ErrorResponse "CHANGE_CODE_REQUIRED"
// @Failure      401 {object} ErrorResponse "CHANGE_CODE_INVALID, CHANGE_CODE_TOO_MANY_ATTEMPTS, AUTH_REQUIRED, INVALID_TOKEN"
// @Failure      404 {object} ErrorResponse "USER_NOT_FOUND"
// @Failure      500 {object} ErrorResponse "REDIS_NOT_CONFIGURED"
// @Router       /users/me/password/change/verify [post]
func (h *Handler) VerifyPasswordChangeCode(c *gin.Context) {
	var r PasswordChangeVerifyRequest
	if err := c.ShouldBindJSON(&r); err != nil {
		send(c, 400, "BAD_REQUEST", "Неверный формат запроса")
		return
	}
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		send(c, 401, "AUTH_REQUIRED", "Требуется авторизация")
		return
	}
	ctx := c.Request.Context()
	token, err := h.s.VerifyPasswordChangeCode(ctx, userID, r.Code)
	if err != nil {
		switch err {
		case service.ErrChangeCodeRequired:
			send(c, 400, "CHANGE_CODE_REQUIRED", "Введите код из письма")
		case service.ErrChangeCodeInvalid:
			send(c, 401, "CHANGE_CODE_INVALID", "Неверный код")
		case service.ErrChangeCodeAttempts:
			send(c, 401, "CHANGE_CODE_TOO_MANY_ATTEMPTS", "Превышено число попыток, запросите новый код")
		case service.ErrRedisNotConfigured:
			send(c, 500, "REDIS_NOT_CONFIGURED", "Кэш не настроен")
		case service.ErrUserNotFound:
			send(c, 404, "USER_NOT_FOUND", "Пользователь не найден")
		default:
			logApp(c).ErrorContext(ctx, "password change verify failed", "operation", "auth.password_change.verify", "user_id", userID, "error", err.Error())
			send(c, 500, "INTERNAL_ERROR", "Что-то пошло не так.")
		}
		return
	}
	logAudit(c).InfoContext(ctx, "password change code verified", "operation", "auth.password_change.verify", "user_id", userID)
	c.JSON(200, PasswordChangeVerifyResponse{ChangeToken: token})
}

// ConfirmPasswordChange godoc
// @Summary      Смена пароля
// @Description  Устанавливает новый пароль по changeToken
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        request body PasswordChangeConfirmRequest true "Change token и новый пароль"
// @Security     BearerAuth
// @Success      200
// @Failure      400 {object} ErrorResponse "PASSWORD_REQUIRED, INVALID_PASSWORD_FORMAT, PASSWORD_CONFIRMATION_MISMATCH, CHANGE_TOKEN_REQUIRED"
// @Failure      401 {object} ErrorResponse "CHANGE_SESSION_INVALID, CHANGE_SESSION_EXPIRED"
// @Failure      500 {object} ErrorResponse "REDIS_NOT_CONFIGURED"
// @Router       /users/me/password/change/confirm [post]
func (h *Handler) ConfirmPasswordChange(c *gin.Context) {
	var r PasswordChangeConfirmRequest
	if err := c.ShouldBindJSON(&r); err != nil {
		send(c, 400, "BAD_REQUEST", "Неверный формат запроса")
		return
	}
	ctx := c.Request.Context()
	userID, _ := middleware.UserIDFromContext(c)
	if err := h.s.ConfirmPasswordChange(ctx, r.ChangeToken, r.Password, r.PasswordConfirm); err != nil {
		switch err {
		case service.ErrChangeTokenRequired:
			send(c, 400, "CHANGE_TOKEN_REQUIRED", "Отсутствует токен подтверждения")
		case service.ErrPasswordRequired:
			send(c, 400, "PASSWORD_REQUIRED", "Введите пароль")
		case service.ErrInvalidPasswordRules:
			send(c, 400, "INVALID_PASSWORD_FORMAT", "Пароль должен содержать минимум 8 символов и цифру")
		case service.ErrPasswordMismatch:
			send(c, 400, "PASSWORD_CONFIRMATION_MISMATCH", "Пароли не совпадают")
		case service.ErrChangeSessionInvalid:
			send(c, 401, "CHANGE_SESSION_INVALID", "Неверный токен подтверждения")
		case service.ErrChangeSessionExpired:
			send(c, 401, "CHANGE_SESSION_EXPIRED", "Токен подтверждения истек")
		case service.ErrRedisNotConfigured:
			send(c, 500, "REDIS_NOT_CONFIGURED", "Кэш не настроен")
		default:
			logApp(c).ErrorContext(ctx, "password change confirm failed", "operation", "auth.password_change.confirm", "user_id", userID, "error", err.Error())
			send(c, 500, "INTERNAL_ERROR", "Что-то пошло не так.")
		}
		return
	}
	logAudit(c).InfoContext(ctx, "password changed", "operation", "auth.password_change.confirm", "user_id", userID)
	c.Status(200)
}
