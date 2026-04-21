package auth

import (
	"errors"
	"io"
	"log/slog"
	"strings"

	"github.com/gin-gonic/gin"
	appLogger "vitamins-backend_2/internal/logger"
	"vitamins-backend_2/internal/metrics"
)

type AuthRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type PasswordResetRequest struct {
	Email string `json:"email"`
}

type PasswordResetVerifyRequest struct {
	Email string `json:"email"`
	Code  string `json:"code"`
}

type PasswordResetConfirmRequest struct {
	ResetToken      string `json:"resetToken"`
	Password        string `json:"password"`
	PasswordConfirm string `json:"passwordConfirm"`
}

type PasswordResetVerifyResponse struct {
	ResetToken string `json:"resetToken"`
}

type RefreshTokenRequest struct {
	RefreshToken      string `json:"refreshToken"`
	RefreshTokenSnake string `json:"refresh_token"`
}

type UpdateProfileRequest struct {
	FirstName *string `json:"firstName"`
	LastName  *string `json:"lastName"`
	Email     *string `json:"email"`
}

type UserProfileResponse struct {
	ID        int64  `json:"id"`
	Email     string `json:"email"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
}

type PasswordChangeVerifyRequest struct {
	Code string `json:"code"`
}

type PasswordChangeConfirmRequest struct {
	ChangeToken     string `json:"changeToken"`
	Password        string `json:"password"`
	PasswordConfirm string `json:"passwordConfirm"`
}

type PasswordChangeVerifyResponse struct {
	ChangeToken string `json:"changeToken"`
}

type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Handler struct{ s ServiceAPI }

func NewHandler(s ServiceAPI) *Handler { return &Handler{s} }

func send(c *gin.Context, code int, k, m string) {
	c.JSON(code, ErrorResponse{Code: k, Message: m})
}

func logApp(c *gin.Context) *slog.Logger {
	return appLogger.WithContext(slog.Default(), c.Request.Context()).With("channel", "app")
}

func logAudit(c *gin.Context) *slog.Logger {
	return appLogger.WithContext(slog.Default(), c.Request.Context()).With("channel", "audit")
}

// Register godoc
// @Summary      Регистрация пользователя
// @Description  Создает учетную запись и возвращает пару access/refresh токенов
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body AuthRequest true "Данные пользователя"
// @Success      200 {object} TokenPair
// @Failure      400 {object} ErrorResponse "EMAIL_AND_PASSWORD_REQUIRED, EMAIL_REQUIRED, INVALID_EMAIL_FORMAT, PASSWORD_REQUIRED, INVALID_PASSWORD_FORMAT"
// @Failure      409 {object} ErrorResponse "EMAIL_ALREADY_EXISTS"
// @Failure      500 {object} ErrorResponse
// @Router       /auth/register [post]
func (h *Handler) Register(c *gin.Context) {
	var r AuthRequest
	if err := c.ShouldBindJSON(&r); err != nil {
		metrics.ObserveAuth("register", "failure")
		send(c, 400, "BAD_REQUEST", "Неверный формат запроса")
		return
	}
	if r.Email == "" && r.Password == "" {
		metrics.ObserveAuth("register", "failure")
		send(c, 400, "EMAIL_AND_PASSWORD_REQUIRED", "Введите e-mail и пароль")
		return
	}
	ctx := c.Request.Context()
	t, err := h.s.Register(ctx, r.Email, r.Password)
	if err != nil {
		metrics.ObserveAuth("register", "failure")
		switch err {
		case ErrEmailRequired:
			send(c, 400, "EMAIL_REQUIRED", "Введите e-mail")
		case ErrInvalidEmailFormat:
			send(c, 400, "INVALID_EMAIL_FORMAT", "Проверьте формат: name@domain.com")
		case ErrPasswordRequired:
			send(c, 400, "PASSWORD_REQUIRED", "Введите пароль")
		case ErrInvalidPasswordRules:
			send(c, 400, "INVALID_PASSWORD_FORMAT", "Пароль должен содержать минимум 8 символов и цифру")
		case ErrEmailAlreadyExists:
			send(c, 409, "EMAIL_ALREADY_EXISTS", "Такой e-mail уже зарегистрирован")
		default:
			logApp(c).ErrorContext(ctx, "register failed", "operation", "auth.register", "error", err.Error())
			send(c, 500, "INTERNAL_ERROR", "Что-то пошло не так.")
		}
		return
	}
	metrics.ObserveAuth("register", "success")
	logAudit(c).InfoContext(ctx, "user registered",
		"operation", "auth.register",
		"user.email_masked", appLogger.MaskEmail(r.Email),
	)
	c.JSON(200, t)
}

// Login godoc
// @Summary      Авторизация пользователя
// @Description  Проверяет учетные данные и возвращает новую пару токенов
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body AuthRequest true "Данные пользователя"
// @Success      200 {object} TokenPair
// @Failure      400 {object} ErrorResponse "EMAIL_AND_PASSWORD_REQUIRED, EMAIL_REQUIRED, INVALID_EMAIL_FORMAT, PASSWORD_REQUIRED, INVALID_PASSWORD_FORMAT"
// @Failure      401 {object} ErrorResponse "INVALID_CREDENTIALS"
// @Failure      500 {object} ErrorResponse
// @Router       /auth/login [post]
func (h *Handler) Login(c *gin.Context) {
	var r AuthRequest
	if err := c.ShouldBindJSON(&r); err != nil {
		metrics.ObserveAuth("login", "failure")
		send(c, 400, "BAD_REQUEST", "Неверный формат запроса")
		return
	}
	if r.Email == "" && r.Password == "" {
		metrics.ObserveAuth("login", "failure")
		send(c, 400, "EMAIL_AND_PASSWORD_REQUIRED", "Введите e-mail и пароль")
		return
	}
	ctx := c.Request.Context()
	t, err := h.s.Login(ctx, r.Email, r.Password)
	if err != nil {
		metrics.ObserveAuth("login", "failure")
		switch err {
		case ErrEmailRequired:
			send(c, 400, "EMAIL_REQUIRED", "Введите e-mail")
		case ErrInvalidEmailFormat:
			send(c, 400, "INVALID_EMAIL_FORMAT", "Проверьте формат: name@domain.com")
		case ErrPasswordRequired:
			send(c, 400, "PASSWORD_REQUIRED", "Введите пароль")
		case ErrInvalidPasswordRules:
			send(c, 400, "INVALID_PASSWORD_FORMAT", "Пароль должен содержать минимум 8 символов и цифру")
		case ErrInvalidCredentials:
			send(c, 401, "INVALID_CREDENTIALS", "Неверный e-mail или пароль")
		default:
			logApp(c).ErrorContext(ctx, "login failed", "operation", "auth.login", "error", err.Error())
			send(c, 500, "INTERNAL_ERROR", "Что-то пошло не так.")
		}
		return
	}
	metrics.ObserveAuth("login", "success")
	logAudit(c).InfoContext(ctx, "user logged in",
		"operation", "auth.login",
		"user.email_masked", appLogger.MaskEmail(r.Email),
	)
	c.JSON(200, t)
}

// Refresh godoc
// @Summary      Обновление токенов
// @Description  Обновляет access token по refresh token из Authorization Bearer и/или тела запроса
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body RefreshTokenRequest false "Refresh token (refreshToken или refresh_token)"
// @Param        Authorization header string false "Bearer <refreshToken>"
// @Success      200 {object} TokenPair
// @Failure      400 {object} ErrorResponse "BAD_REQUEST"
// @Failure      401 {object} ErrorResponse "INVALID_REFRESH_TOKEN"
// @Failure      500 {object} ErrorResponse
// @Router       /auth/refresh [post]
func (h *Handler) Refresh(c *gin.Context) {
	var r RefreshTokenRequest
	if err := c.ShouldBindJSON(&r); err != nil && !errors.Is(err, io.EOF) {
		metrics.ObserveAuth("refresh", "failure")
		send(c, 400, "BAD_REQUEST", "Неверный формат запроса")
		return
	}

	token := extractBearerToken(c.GetHeader("Authorization"))
	if token == "" {
		token = strings.TrimSpace(r.RefreshToken)
	}
	if token == "" {
		token = strings.TrimSpace(r.RefreshTokenSnake)
	}
	if token == "" {
		metrics.ObserveAuth("refresh", "failure")
		send(c, 401, "INVALID_REFRESH_TOKEN", "Неверный или истекший refresh token")
		return
	}

	ctx := c.Request.Context()
	t, err := h.s.Refresh(ctx, token)
	if err != nil {
		metrics.ObserveAuth("refresh", "failure")
		switch err {
		case ErrInvalidCredentials:
			send(c, 401, "INVALID_REFRESH_TOKEN", "Неверный или истекший refresh token")
		default:
			logApp(c).ErrorContext(ctx, "refresh failed", "operation", "auth.refresh", "error", err.Error())
			send(c, 500, "INTERNAL_ERROR", "Что-то пошло не так.")
		}
		return
	}
	metrics.ObserveAuth("refresh", "success")
	logAudit(c).InfoContext(ctx, "token refreshed", "operation", "auth.refresh")
	c.JSON(200, t)
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
		case ErrEmailRequired:
			send(c, 400, "EMAIL_REQUIRED", "Введите e-mail")
		case ErrInvalidEmailFormat:
			send(c, 400, "INVALID_EMAIL_FORMAT", "Проверьте формат: name@domain.com")
		case ErrUserNotFound:
			send(c, 404, "USER_NOT_FOUND", "Пользователь не найден")
		case ErrTooManyRequests:
			send(c, 429, "TOO_MANY_REQUESTS", "Слишком часто. Попробуйте позже.")
		case ErrMailerNotConfigured:
			send(c, 500, "MAILER_NOT_CONFIGURED", "Сервис отправки писем не настроен")
		case ErrRedisNotConfigured:
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
		case ErrEmailRequired:
			send(c, 400, "EMAIL_REQUIRED", "Введите e-mail")
		case ErrInvalidEmailFormat:
			send(c, 400, "INVALID_EMAIL_FORMAT", "Проверьте формат: name@domain.com")
		case ErrResetCodeRequired:
			send(c, 400, "RESET_CODE_REQUIRED", "Введите код из письма")
		case ErrResetCodeInvalid:
			send(c, 401, "RESET_CODE_INVALID", "Неверный код")
		case ErrResetCodeExpired:
			send(c, 401, "RESET_CODE_EXPIRED", "Код истек, запросите новый")
		case ErrResetCodeAttempts:
			send(c, 401, "RESET_CODE_TOO_MANY_ATTEMPTS", "Превышено число попыток, запросите новый код")
		case ErrRedisNotConfigured:
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
		case ErrResetTokenRequired:
			send(c, 400, "RESET_TOKEN_REQUIRED", "Отсутствует токен восстановления")
		case ErrPasswordRequired:
			send(c, 400, "PASSWORD_REQUIRED", "Введите пароль")
		case ErrInvalidPasswordRules:
			send(c, 400, "INVALID_PASSWORD_FORMAT", "Пароль должен содержать минимум 8 символов и цифру")
		case ErrPasswordMismatch:
			send(c, 400, "PASSWORD_CONFIRMATION_MISMATCH", "Пароли не совпадают")
		case ErrResetSessionInvalid:
			send(c, 401, "RESET_SESSION_INVALID", "Неверный токен восстановления")
		case ErrResetSessionExpired:
			send(c, 401, "RESET_SESSION_EXPIRED", "Токен восстановления истек")
		case ErrRedisNotConfigured:
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

// UpdateProfile godoc
// @Summary      Обновление профиля
// @Description  Обновляет имя, фамилию и/или e-mail текущего пользователя
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        request body UpdateProfileRequest true "Данные профиля"
// @Security     BearerAuth
// @Success      200 {object} UserProfileResponse
// @Failure      400 {object} ErrorResponse "NO_FIELDS_TO_UPDATE, EMAIL_REQUIRED, INVALID_EMAIL_FORMAT"
// @Failure      401 {object} ErrorResponse "AUTH_REQUIRED, INVALID_TOKEN"
// @Failure      404 {object} ErrorResponse "USER_NOT_FOUND"
// @Failure      409 {object} ErrorResponse "EMAIL_ALREADY_EXISTS"
// @Failure      500 {object} ErrorResponse
// @Router       /users/me [patch]
func (h *Handler) UpdateProfile(c *gin.Context) {
	var r UpdateProfileRequest
	if err := c.ShouldBindJSON(&r); err != nil {
		send(c, 400, "BAD_REQUEST", "Неверный формат запроса")
		return
	}
	userID, ok := UserIDFromContext(c)
	if !ok {
		send(c, 401, "AUTH_REQUIRED", "Требуется авторизация")
		return
	}
	ctx := c.Request.Context()
	u, err := h.s.UpdateProfile(ctx, userID, ProfileUpdate{
		FirstName: r.FirstName,
		LastName:  r.LastName,
		Email:     r.Email,
	})
	if err != nil {
		switch err {
		case ErrNoFieldsToUpdate:
			send(c, 400, "NO_FIELDS_TO_UPDATE", "Нечего обновлять")
		case ErrEmailRequired:
			send(c, 400, "EMAIL_REQUIRED", "Введите e-mail")
		case ErrInvalidEmailFormat:
			send(c, 400, "INVALID_EMAIL_FORMAT", "Проверьте формат: name@domain.com")
		case ErrEmailAlreadyExists:
			send(c, 409, "EMAIL_ALREADY_EXISTS", "Такой e-mail уже зарегистрирован")
		case ErrUserNotFound:
			send(c, 404, "USER_NOT_FOUND", "Пользователь не найден")
		default:
			logApp(c).ErrorContext(ctx, "update profile failed", "operation", "auth.profile.update", "user_id", userID, "error", err.Error())
			send(c, 500, "INTERNAL_ERROR", "Что-то пошло не так.")
		}
		return
	}
	logAudit(c).InfoContext(ctx, "profile updated", "operation", "auth.profile.update", "user_id", userID)
	c.JSON(200, UserProfileResponse{
		ID:        u.ID,
		Email:     u.Email,
		FirstName: u.FirstName,
		LastName:  u.LastName,
	})
}

// GetProfile godoc
// @Summary      Профиль пользователя
// @Description  Возвращает информацию о текущем пользователе
// @Tags         users
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} UserProfileResponse
// @Failure      401 {object} ErrorResponse "AUTH_REQUIRED, INVALID_TOKEN"
// @Failure      404 {object} ErrorResponse "USER_NOT_FOUND"
// @Failure      500 {object} ErrorResponse
// @Router       /users/me [get]
func (h *Handler) GetProfile(c *gin.Context) {
	userID, ok := UserIDFromContext(c)
	if !ok {
		send(c, 401, "AUTH_REQUIRED", "Требуется авторизация")
		return
	}
	ctx := c.Request.Context()
	u, err := h.s.GetProfile(ctx, userID)
	if err != nil {
		switch err {
		case ErrUserNotFound:
			send(c, 404, "USER_NOT_FOUND", "Пользователь не найден")
		default:
			logApp(c).ErrorContext(ctx, "get profile failed", "operation", "auth.profile.get", "user_id", userID, "error", err.Error())
			send(c, 500, "INTERNAL_ERROR", "Что-то пошло не так.")
		}
		return
	}
	c.JSON(200, UserProfileResponse{
		ID:        u.ID,
		Email:     u.Email,
		FirstName: u.FirstName,
		LastName:  u.LastName,
	})
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
	userID, ok := UserIDFromContext(c)
	if !ok {
		send(c, 401, "AUTH_REQUIRED", "Требуется авторизация")
		return
	}
	ctx := c.Request.Context()
	if err := h.s.RequestPasswordChange(ctx, userID); err != nil {
		switch err {
		case ErrTooManyRequests:
			send(c, 429, "TOO_MANY_REQUESTS", "Слишком часто. Попробуйте позже.")
		case ErrMailerNotConfigured:
			send(c, 500, "MAILER_NOT_CONFIGURED", "Сервис отправки писем не настроен")
		case ErrRedisNotConfigured:
			send(c, 500, "REDIS_NOT_CONFIGURED", "Кэш не настроен")
		case ErrUserNotFound:
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
	userID, ok := UserIDFromContext(c)
	if !ok {
		send(c, 401, "AUTH_REQUIRED", "Требуется авторизация")
		return
	}
	ctx := c.Request.Context()
	token, err := h.s.VerifyPasswordChangeCode(ctx, userID, r.Code)
	if err != nil {
		switch err {
		case ErrChangeCodeRequired:
			send(c, 400, "CHANGE_CODE_REQUIRED", "Введите код из письма")
		case ErrChangeCodeInvalid:
			send(c, 401, "CHANGE_CODE_INVALID", "Неверный код")
		case ErrChangeCodeAttempts:
			send(c, 401, "CHANGE_CODE_TOO_MANY_ATTEMPTS", "Превышено число попыток, запросите новый код")
		case ErrRedisNotConfigured:
			send(c, 500, "REDIS_NOT_CONFIGURED", "Кэш не настроен")
		case ErrUserNotFound:
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
	userID, _ := UserIDFromContext(c)
	if err := h.s.ConfirmPasswordChange(ctx, r.ChangeToken, r.Password, r.PasswordConfirm); err != nil {
		switch err {
		case ErrChangeTokenRequired:
			send(c, 400, "CHANGE_TOKEN_REQUIRED", "Отсутствует токен подтверждения")
		case ErrPasswordRequired:
			send(c, 400, "PASSWORD_REQUIRED", "Введите пароль")
		case ErrInvalidPasswordRules:
			send(c, 400, "INVALID_PASSWORD_FORMAT", "Пароль должен содержать минимум 8 символов и цифру")
		case ErrPasswordMismatch:
			send(c, 400, "PASSWORD_CONFIRMATION_MISMATCH", "Пароли не совпадают")
		case ErrChangeSessionInvalid:
			send(c, 401, "CHANGE_SESSION_INVALID", "Неверный токен подтверждения")
		case ErrChangeSessionExpired:
			send(c, 401, "CHANGE_SESSION_EXPIRED", "Токен подтверждения истек")
		case ErrRedisNotConfigured:
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

func extractBearerToken(authHeader string) string {
	value := strings.TrimSpace(authHeader)
	if value == "" {
		return ""
	}
	const prefix = "Bearer "
	if !strings.HasPrefix(value, prefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(value, prefix))
}
