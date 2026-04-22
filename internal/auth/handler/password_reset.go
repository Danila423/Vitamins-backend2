package handler

import (
	appLogger "vitamins-backend_2/internal/logger"

	"github.com/gin-gonic/gin"

	"vitamins-backend_2/internal/auth/service"
)

// PasswordResetRequest — body for /auth/password/reset/request.
type PasswordResetRequest struct {
	Email string `json:"email"`
}

// PasswordResetVerifyRequest — body for /auth/password/reset/verify.
type PasswordResetVerifyRequest struct {
	Email string `json:"email"`
	Code  string `json:"code"`
}

// PasswordResetConfirmRequest — body for /auth/password/reset/confirm.
type PasswordResetConfirmRequest struct {
	ResetToken      string `json:"resetToken"`
	Password        string `json:"password"`
	PasswordConfirm string `json:"passwordConfirm"`
}

// PasswordResetVerifyResponse — successful response of /auth/password/reset/verify.
type PasswordResetVerifyResponse struct {
	ResetToken string `json:"resetToken"`
}

// RequestPasswordReset godoc
// @Summary      Запрос кода восстановления пароля
// @Description  Отправляет код на e-mail через SMTP
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body PasswordResetRequest true "E-mail"
// @Success      200
// @Failure      400 {object} ErrorResponse "EMAIL_REQUIRED, INVALID_EMAIL_FORMAT"
// @Failure      404 {object} ErrorResponse "USER_NOT_FOUND"
// @Failure      429 {object} ErrorResponse "TOO_MANY_REQUESTS"
// @Failure      500 {object} ErrorResponse
// @Router       /auth/password/reset/request [post]
func (h *Handler) RequestPasswordReset(c *gin.Context) {
	var r PasswordResetRequest
	if err := c.ShouldBindJSON(&r); err != nil {
		send(c, 400, "BAD_REQUEST", "Неверный формат запроса")
		return
	}
	ctx := c.Request.Context()
	if err := h.s.RequestPasswordReset(ctx, r.Email); err != nil {
		switch err {
		case service.ErrEmailRequired:
			send(c, 400, "EMAIL_REQUIRED", "Введите e-mail")
		case service.ErrInvalidEmailFormat:
			send(c, 400, "INVALID_EMAIL_FORMAT", "Проверьте формат: name@domain.com")
		case service.ErrUserNotFound:
			send(c, 404, "USER_NOT_FOUND", "Пользователь не найден")
		case service.ErrTooManyRequests:
			send(c, 429, "TOO_MANY_REQUESTS", "Слишком часто. Попробуйте позже.")
		case service.ErrMailerNotConfigured:
			send(c, 500, "MAILER_NOT_CONFIGURED", "Сервис отправки писем не настроен")
		case service.ErrRedisNotConfigured:
			send(c, 500, "REDIS_NOT_CONFIGURED", "Кэш не настроен")
		default:
			logApp(c).ErrorContext(ctx, "password reset request failed",
				"operation", "auth.password_reset.request",
				"user.email_masked", appLogger.MaskEmail(r.Email),
				"error", err.Error(),
			)
			send(c, 500, "INTERNAL_ERROR", "Что-то пошло не так.")
		}
		return
	}
	logAudit(c).InfoContext(ctx, "password reset requested",
		"operation", "auth.password_reset.request",
		"user.email_masked", appLogger.MaskEmail(r.Email),
	)
	c.Status(200)
}

// VerifyPasswordResetCode godoc
// @Summary      Проверка кода восстановления
// @Description  Проверяет код и возвращает resetToken
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body PasswordResetVerifyRequest true "E-mail и код"
// @Success      200 {object} PasswordResetVerifyResponse
// @Failure      400 {object} ErrorResponse "EMAIL_REQUIRED, INVALID_EMAIL_FORMAT, RESET_CODE_REQUIRED"
// @Failure      401 {object} ErrorResponse "RESET_CODE_INVALID, RESET_CODE_EXPIRED, RESET_CODE_TOO_MANY_ATTEMPTS"
// @Failure      500 {object} ErrorResponse
// @Router       /auth/password/reset/verify [post]
func (h *Handler) VerifyPasswordResetCode(c *gin.Context) {
	var r PasswordResetVerifyRequest
	if err := c.ShouldBindJSON(&r); err != nil {
		send(c, 400, "BAD_REQUEST", "Неверный формат запроса")
		return
	}
	ctx := c.Request.Context()
	token, err := h.s.VerifyPasswordResetCode(ctx, r.Email, r.Code)
	if err != nil {
		switch err {
		case service.ErrEmailRequired:
			send(c, 400, "EMAIL_REQUIRED", "Введите e-mail")
		case service.ErrInvalidEmailFormat:
			send(c, 400, "INVALID_EMAIL_FORMAT", "Проверьте формат: name@domain.com")
		case service.ErrResetCodeRequired:
			send(c, 400, "RESET_CODE_REQUIRED", "Введите код из письма")
		case service.ErrResetCodeInvalid:
			send(c, 401, "RESET_CODE_INVALID", "Неверный код")
		case service.ErrResetCodeExpired:
			send(c, 401, "RESET_CODE_EXPIRED", "Код истек, запросите новый")
		case service.ErrResetCodeAttempts:
			send(c, 401, "RESET_CODE_TOO_MANY_ATTEMPTS", "Превышено число попыток, запросите новый код")
		case service.ErrRedisNotConfigured:
			send(c, 500, "REDIS_NOT_CONFIGURED", "Кэш не настроен")
		default:
			logApp(c).ErrorContext(ctx, "password reset verify failed",
				"operation", "auth.password_reset.verify",
				"user.email_masked", appLogger.MaskEmail(r.Email),
				"error", err.Error(),
			)
			send(c, 500, "INTERNAL_ERROR", "Что-то пошло не так.")
		}
		return
	}
	logAudit(c).InfoContext(ctx, "password reset code verified",
		"operation", "auth.password_reset.verify",
		"user.email_masked", appLogger.MaskEmail(r.Email),
	)
	c.JSON(200, PasswordResetVerifyResponse{ResetToken: token})
}

// ConfirmPasswordReset godoc
// @Summary      Смена пароля
// @Description  Устанавливает новый пароль по resetToken
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body PasswordResetConfirmRequest true "Reset token и новый пароль"
// @Success      200
// @Failure      400 {object} ErrorResponse "PASSWORD_REQUIRED, INVALID_PASSWORD_FORMAT, PASSWORD_CONFIRMATION_MISMATCH, RESET_TOKEN_REQUIRED"
// @Failure      401 {object} ErrorResponse "RESET_SESSION_INVALID, RESET_SESSION_EXPIRED"
// @Failure      500 {object} ErrorResponse
// @Router       /auth/password/reset/confirm [post]
func (h *Handler) ConfirmPasswordReset(c *gin.Context) {
	var r PasswordResetConfirmRequest
	if err := c.ShouldBindJSON(&r); err != nil {
		send(c, 400, "BAD_REQUEST", "Неверный формат запроса")
		return
	}
	ctx := c.Request.Context()
	if err := h.s.ConfirmPasswordReset(ctx, r.ResetToken, r.Password, r.PasswordConfirm); err != nil {
		switch err {
		case service.ErrResetTokenRequired:
			send(c, 400, "RESET_TOKEN_REQUIRED", "Отсутствует токен восстановления")
		case service.ErrPasswordRequired:
			send(c, 400, "PASSWORD_REQUIRED", "Введите пароль")
		case service.ErrInvalidPasswordRules:
			send(c, 400, "INVALID_PASSWORD_FORMAT", "Пароль должен содержать минимум 8 символов и цифру")
		case service.ErrPasswordMismatch:
			send(c, 400, "PASSWORD_CONFIRMATION_MISMATCH", "Пароли не совпадают")
		case service.ErrResetSessionInvalid:
			send(c, 401, "RESET_SESSION_INVALID", "Неверный токен восстановления")
		case service.ErrResetSessionExpired:
			send(c, 401, "RESET_SESSION_EXPIRED", "Токен восстановления истек")
		case service.ErrRedisNotConfigured:
			send(c, 500, "REDIS_NOT_CONFIGURED", "Кэш не настроен")
		default:
			logApp(c).ErrorContext(ctx, "password reset confirm failed", "operation", "auth.password_reset.confirm", "error", err.Error())
			send(c, 500, "INTERNAL_ERROR", "Что-то пошло не так.")
		}
		return
	}
	logAudit(c).InfoContext(ctx, "password reset confirmed", "operation", "auth.password_reset.confirm")
	c.Status(200)
}
