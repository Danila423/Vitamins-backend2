package handler

import (
	"errors"
	"io"
	"strings"

	appLogger "vitamins-backend_2/internal/logger"
	"vitamins-backend_2/internal/metrics"

	"github.com/gin-gonic/gin"

	"vitamins-backend_2/internal/auth/service"
)

// AuthRequest is the body for /auth/register and /auth/login.
type AuthRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// RefreshTokenRequest accepts both camelCase and snake_case for backwards
// compatibility with older mobile clients.
type RefreshTokenRequest struct {
	RefreshToken      string `json:"refreshToken"`
	RefreshTokenSnake string `json:"refresh_token"`
}

// Register godoc
// @Summary      Регистрация пользователя
// @Description  Создает учетную запись и возвращает пару access/refresh токенов
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body AuthRequest true "Данные пользователя"
// @Success      200 {object} service.TokenPair
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
		case service.ErrEmailRequired:
			send(c, 400, "EMAIL_REQUIRED", "Введите e-mail")
		case service.ErrInvalidEmailFormat:
			send(c, 400, "INVALID_EMAIL_FORMAT", "Проверьте формат: name@domain.com")
		case service.ErrPasswordRequired:
			send(c, 400, "PASSWORD_REQUIRED", "Введите пароль")
		case service.ErrInvalidPasswordRules:
			send(c, 400, "INVALID_PASSWORD_FORMAT", "Пароль должен содержать минимум 8 символов и цифру")
		case service.ErrEmailAlreadyExists:
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
// @Success      200 {object} service.TokenPair
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
		case service.ErrEmailRequired:
			send(c, 400, "EMAIL_REQUIRED", "Введите e-mail")
		case service.ErrInvalidEmailFormat:
			send(c, 400, "INVALID_EMAIL_FORMAT", "Проверьте формат: name@domain.com")
		case service.ErrPasswordRequired:
			send(c, 400, "PASSWORD_REQUIRED", "Введите пароль")
		case service.ErrInvalidPasswordRules:
			send(c, 400, "INVALID_PASSWORD_FORMAT", "Пароль должен содержать минимум 8 символов и цифру")
		case service.ErrInvalidCredentials:
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
// @Success      200 {object} service.TokenPair
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
		case service.ErrInvalidCredentials:
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
