package handler

import (
	"errors"
	"io"
	"strings"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	authv1 "vitamins-backend_2/gen/go/auth/v1"
	appLogger "vitamins-backend_2/pkg/logger"
	"vitamins-backend_2/pkg/metrics"
	"vitamins-backend_2/services/gateway/internal/middleware"
)

type AuthHandler struct {
	client authv1.AuthServiceClient
}

func NewAuthHandler(client authv1.AuthServiceClient) *AuthHandler {
	return &AuthHandler{client: client}
}

type authRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type refreshTokenRequest struct {
	RefreshToken      string `json:"refreshToken"`
	RefreshTokenSnake string `json:"refresh_token"`
}

type tokenPairJSON struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
}

type userProfileJSON struct {
	ID        int64  `json:"id"`
	Email     string `json:"email"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
}

type updateProfileRequest struct {
	FirstName *string `json:"firstName"`
	LastName  *string `json:"lastName"`
	Email     *string `json:"email"`
}

type passwordResetRequest struct {
	Email string `json:"email"`
}

type passwordResetVerifyRequest struct {
	Email string `json:"email"`
	Code  string `json:"code"`
}

type passwordResetConfirmRequest struct {
	ResetToken      string `json:"resetToken"`
	Password        string `json:"password"`
	PasswordConfirm string `json:"passwordConfirm"`
}

type passwordResetVerifyResponse struct {
	ResetToken string `json:"resetToken"`
}

type passwordChangeVerifyRequest struct {
	Code string `json:"code"`
}

type passwordChangeConfirmRequest struct {
	ChangeToken     string `json:"changeToken"`
	Password        string `json:"password"`
	PasswordConfirm string `json:"passwordConfirm"`
}

type passwordChangeVerifyResponse struct {
	ChangeToken string `json:"changeToken"`
}

func (h *AuthHandler) Register(c *gin.Context) {
	var r authRequest
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
	resp, err := h.client.Register(ctx, &authv1.RegisterRequest{
		Email:    r.Email,
		Password: r.Password,
	})
	if err != nil {
		metrics.ObserveAuth("register", "failure")
		mapAuthGRPCError(c, err)
		return
	}
	metrics.ObserveAuth("register", "success")
	logAudit(c).InfoContext(ctx, "user registered",
		"operation", "auth.register",
		"user.email_masked", appLogger.MaskEmail(r.Email),
	)
	c.JSON(200, tokenPairJSON{
		AccessToken:  resp.GetAccessToken(),
		RefreshToken: resp.GetRefreshToken(),
	})
}

func (h *AuthHandler) Login(c *gin.Context) {
	var r authRequest
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
	resp, err := h.client.Login(ctx, &authv1.LoginRequest{
		Email:    r.Email,
		Password: r.Password,
	})
	if err != nil {
		metrics.ObserveAuth("login", "failure")
		mapLoginGRPCError(c, err)
		return
	}
	metrics.ObserveAuth("login", "success")
	logAudit(c).InfoContext(ctx, "user logged in",
		"operation", "auth.login",
		"user.email_masked", appLogger.MaskEmail(r.Email),
	)
	c.JSON(200, tokenPairJSON{
		AccessToken:  resp.GetAccessToken(),
		RefreshToken: resp.GetRefreshToken(),
	})
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	var r refreshTokenRequest
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
	resp, err := h.client.Refresh(ctx, &authv1.RefreshRequest{RefreshToken: token})
	if err != nil {
		metrics.ObserveAuth("refresh", "failure")
		st, ok := status.FromError(err)
		if ok && st.Code() == codes.Unauthenticated {
			send(c, 401, "INVALID_REFRESH_TOKEN", "Неверный или истекший refresh token")
			return
		}
		logApp(c).ErrorContext(ctx, "refresh failed", "operation", "auth.refresh", "error", err.Error())
		send(c, 500, "INTERNAL_ERROR", "Что-то пошло не так.")
		return
	}
	metrics.ObserveAuth("refresh", "success")
	logAudit(c).InfoContext(ctx, "token refreshed", "operation", "auth.refresh")
	c.JSON(200, tokenPairJSON{
		AccessToken:  resp.GetAccessToken(),
		RefreshToken: resp.GetRefreshToken(),
	})
}

func (h *AuthHandler) GetProfile(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		send(c, 401, "AUTH_REQUIRED", "Требуется авторизация")
		return
	}
	ctx := c.Request.Context()
	resp, err := h.client.GetProfile(ctx, &authv1.GetProfileRequest{UserId: userID})
	if err != nil {
		st, ok := status.FromError(err)
		if ok {
			switch st.Code() {
			case codes.NotFound:
				send(c, 404, "USER_NOT_FOUND", "Пользователь не найден")
				return
			}
		}
		logApp(c).ErrorContext(ctx, "get profile failed", "operation", "auth.profile.get", "user_id", userID, "error", err.Error())
		send(c, 500, "INTERNAL_ERROR", "Что-то пошло не так.")
		return
	}
	c.JSON(200, userProfileJSON{
		ID:        resp.GetId(),
		Email:     resp.GetEmail(),
		FirstName: resp.GetFirstName(),
		LastName:  resp.GetLastName(),
	})
}

func (h *AuthHandler) UpdateProfile(c *gin.Context) {
	var r updateProfileRequest
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
	req := &authv1.UpdateProfileRequest{UserId: userID}
	if r.FirstName != nil {
		req.FirstName = r.FirstName
	}
	if r.LastName != nil {
		req.LastName = r.LastName
	}
	if r.Email != nil {
		req.Email = r.Email
	}
	resp, err := h.client.UpdateProfile(ctx, req)
	if err != nil {
		st, ok := status.FromError(err)
		if ok {
			msg := st.Message()
			switch st.Code() {
			case codes.InvalidArgument:
				switch {
				case strings.Contains(msg, "NO_FIELDS_TO_UPDATE"):
					send(c, 400, "NO_FIELDS_TO_UPDATE", "Нечего обновлять")
				case strings.Contains(msg, "EMAIL_REQUIRED"):
					send(c, 400, "EMAIL_REQUIRED", "Введите e-mail")
				case strings.Contains(msg, "INVALID_EMAIL_FORMAT"):
					send(c, 400, "INVALID_EMAIL_FORMAT", "Проверьте формат: name@domain.com")
				default:
					send(c, 400, "BAD_REQUEST", msg)
				}
				return
			case codes.AlreadyExists:
				send(c, 409, "EMAIL_ALREADY_EXISTS", "Такой e-mail уже зарегистрирован")
				return
			case codes.NotFound:
				send(c, 404, "USER_NOT_FOUND", "Пользователь не найден")
				return
			}
		}
		logApp(c).ErrorContext(ctx, "update profile failed", "operation", "auth.profile.update", "user_id", userID, "error", err.Error())
		send(c, 500, "INTERNAL_ERROR", "Что-то пошло не так.")
		return
	}
	logAudit(c).InfoContext(ctx, "profile updated", "operation", "auth.profile.update", "user_id", userID)
	c.JSON(200, userProfileJSON{
		ID:        resp.GetId(),
		Email:     resp.GetEmail(),
		FirstName: resp.GetFirstName(),
		LastName:  resp.GetLastName(),
	})
}

func (h *AuthHandler) RequestPasswordChange(c *gin.Context) {
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
	_, err := h.client.RequestPasswordChange(ctx, &authv1.RequestPasswordChangeRequest{UserId: userID})
	if err != nil {
		st, ok := status.FromError(err)
		if ok {
			msg := st.Message()
			switch st.Code() {
			case codes.ResourceExhausted:
				send(c, 429, "TOO_MANY_REQUESTS", "Слишком часто. Попробуйте позже.")
				return
			case codes.Unavailable:
				if strings.Contains(msg, "MAILER_NOT_CONFIGURED") {
					send(c, 500, "MAILER_NOT_CONFIGURED", "Сервис отправки писем не настроен")
				} else {
					send(c, 500, "REDIS_NOT_CONFIGURED", "Кэш не настроен")
				}
				return
			case codes.NotFound:
				send(c, 404, "USER_NOT_FOUND", "Пользователь не найден")
				return
			}
		}
		logApp(c).ErrorContext(ctx, "password change request failed", "operation", "auth.password_change.request", "user_id", userID, "error", err.Error())
		send(c, 500, "INTERNAL_ERROR", "Что-то пошло не так.")
		return
	}
	logAudit(c).InfoContext(ctx, "password change requested", "operation", "auth.password_change.request", "user_id", userID)
	c.Status(200)
}

func (h *AuthHandler) VerifyPasswordChangeCode(c *gin.Context) {
	var r passwordChangeVerifyRequest
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
	resp, err := h.client.VerifyPasswordChangeCode(ctx, &authv1.VerifyPasswordChangeCodeRequest{
		UserId: userID,
		Code:   r.Code,
	})
	if err != nil {
		st, ok := status.FromError(err)
		if ok {
			msg := st.Message()
			switch st.Code() {
			case codes.InvalidArgument:
				if strings.Contains(msg, "CHANGE_CODE_REQUIRED") {
					send(c, 400, "CHANGE_CODE_REQUIRED", "Введите код из письма")
					return
				}
			case codes.Unauthenticated:
				switch {
				case strings.Contains(msg, "CHANGE_CODE_INVALID"):
					send(c, 401, "CHANGE_CODE_INVALID", "Неверный код")
				case strings.Contains(msg, "CHANGE_CODE_TOO_MANY_ATTEMPTS"):
					send(c, 401, "CHANGE_CODE_TOO_MANY_ATTEMPTS", "Превышено число попыток, запросите новый код")
				default:
					send(c, 401, "CHANGE_CODE_INVALID", "Неверный код")
				}
				return
			case codes.Internal:
				if strings.Contains(msg, "REDIS_NOT_CONFIGURED") {
					send(c, 500, "REDIS_NOT_CONFIGURED", "Кэш не настроен")
					return
				}
			case codes.NotFound:
				send(c, 404, "USER_NOT_FOUND", "Пользователь не найден")
				return
			}
		}
		logApp(c).ErrorContext(ctx, "password change verify failed", "operation", "auth.password_change.verify", "user_id", userID, "error", err.Error())
		send(c, 500, "INTERNAL_ERROR", "Что-то пошло не так.")
		return
	}
	logAudit(c).InfoContext(ctx, "password change code verified", "operation", "auth.password_change.verify", "user_id", userID)
	c.JSON(200, passwordChangeVerifyResponse{ChangeToken: resp.GetToken()})
}

func (h *AuthHandler) ConfirmPasswordChange(c *gin.Context) {
	var r passwordChangeConfirmRequest
	if err := c.ShouldBindJSON(&r); err != nil {
		send(c, 400, "BAD_REQUEST", "Неверный формат запроса")
		return
	}
	ctx := c.Request.Context()
	userID, _ := middleware.UserIDFromContext(c)
	_, err := h.client.ConfirmPasswordChange(ctx, &authv1.ConfirmPasswordChangeRequest{
		ChangeToken:     r.ChangeToken,
		Password:        r.Password,
		PasswordConfirm: r.PasswordConfirm,
	})
	if err != nil {
		st, ok := status.FromError(err)
		if ok {
			msg := st.Message()
			switch st.Code() {
			case codes.InvalidArgument:
				switch {
				case strings.Contains(msg, "CHANGE_TOKEN_REQUIRED"):
					send(c, 400, "CHANGE_TOKEN_REQUIRED", "Отсутствует токен подтверждения")
				case strings.Contains(msg, "PASSWORD_REQUIRED"):
					send(c, 400, "PASSWORD_REQUIRED", "Введите пароль")
				case strings.Contains(msg, "INVALID_PASSWORD"):
					send(c, 400, "INVALID_PASSWORD_FORMAT", "Пароль должен содержать минимум 8 символов и цифру")
				case strings.Contains(msg, "PASSWORD_CONFIRMATION_MISMATCH"):
					send(c, 400, "PASSWORD_CONFIRMATION_MISMATCH", "Пароли не совпадают")
				default:
					send(c, 400, "BAD_REQUEST", msg)
				}
				return
			case codes.Unauthenticated:
				switch {
				case strings.Contains(msg, "CHANGE_SESSION_EXPIRED"):
					send(c, 401, "CHANGE_SESSION_EXPIRED", "Токен подтверждения истек")
				default:
					send(c, 401, "CHANGE_SESSION_INVALID", "Неверный токен подтверждения")
				}
				return
			case codes.Internal:
				if strings.Contains(msg, "REDIS_NOT_CONFIGURED") {
					send(c, 500, "REDIS_NOT_CONFIGURED", "Кэш не настроен")
					return
				}
			}
		}
		logApp(c).ErrorContext(ctx, "password change confirm failed", "operation", "auth.password_change.confirm", "user_id", userID, "error", err.Error())
		send(c, 500, "INTERNAL_ERROR", "Что-то пошло не так.")
		return
	}
	logAudit(c).InfoContext(ctx, "password changed", "operation", "auth.password_change.confirm", "user_id", userID)
	c.Status(200)
}

func (h *AuthHandler) RequestPasswordReset(c *gin.Context) {
	var r passwordResetRequest
	if err := c.ShouldBindJSON(&r); err != nil {
		send(c, 400, "BAD_REQUEST", "Неверный формат запроса")
		return
	}
	ctx := c.Request.Context()
	_, err := h.client.RequestPasswordReset(ctx, &authv1.RequestPasswordResetRequest{Email: r.Email})
	if err != nil {
		st, ok := status.FromError(err)
		if ok {
			msg := st.Message()
			switch st.Code() {
			case codes.InvalidArgument:
				switch {
				case strings.Contains(msg, "EMAIL_REQUIRED"):
					send(c, 400, "EMAIL_REQUIRED", "Введите e-mail")
				case strings.Contains(msg, "INVALID_EMAIL_FORMAT"):
					send(c, 400, "INVALID_EMAIL_FORMAT", "Проверьте формат: name@domain.com")
				default:
					send(c, 400, "BAD_REQUEST", msg)
				}
				return
			case codes.NotFound:
				send(c, 404, "USER_NOT_FOUND", "Пользователь не найден")
				return
			case codes.ResourceExhausted:
				send(c, 429, "TOO_MANY_REQUESTS", "Слишком часто. Попробуйте позже.")
				return
			case codes.Unavailable:
				if strings.Contains(msg, "MAILER_NOT_CONFIGURED") {
					send(c, 500, "MAILER_NOT_CONFIGURED", "Сервис отправки писем не настроен")
				} else {
					send(c, 500, "REDIS_NOT_CONFIGURED", "Кэш не настроен")
				}
				return
			}
		}
		logApp(c).ErrorContext(ctx, "password reset request failed",
			"operation", "auth.password_reset.request",
			"user.email_masked", appLogger.MaskEmail(r.Email),
			"error", err.Error(),
		)
		send(c, 500, "INTERNAL_ERROR", "Что-то пошло не так.")
		return
	}
	logAudit(c).InfoContext(ctx, "password reset requested",
		"operation", "auth.password_reset.request",
		"user.email_masked", appLogger.MaskEmail(r.Email),
	)
	c.Status(200)
}

func (h *AuthHandler) VerifyPasswordResetCode(c *gin.Context) {
	var r passwordResetVerifyRequest
	if err := c.ShouldBindJSON(&r); err != nil {
		send(c, 400, "BAD_REQUEST", "Неверный формат запроса")
		return
	}
	ctx := c.Request.Context()
	resp, err := h.client.VerifyPasswordResetCode(ctx, &authv1.VerifyPasswordResetCodeRequest{
		Email: r.Email,
		Code:  r.Code,
	})
	if err != nil {
		st, ok := status.FromError(err)
		if ok {
			msg := st.Message()
			switch st.Code() {
			case codes.InvalidArgument:
				switch {
				case strings.Contains(msg, "EMAIL_REQUIRED"):
					send(c, 400, "EMAIL_REQUIRED", "Введите e-mail")
				case strings.Contains(msg, "INVALID_EMAIL_FORMAT"):
					send(c, 400, "INVALID_EMAIL_FORMAT", "Проверьте формат: name@domain.com")
				case strings.Contains(msg, "RESET_CODE_REQUIRED"):
					send(c, 400, "RESET_CODE_REQUIRED", "Введите код из письма")
				default:
					send(c, 400, "BAD_REQUEST", msg)
				}
				return
			case codes.Unauthenticated:
				switch {
				case strings.Contains(msg, "RESET_CODE_EXPIRED"):
					send(c, 401, "RESET_CODE_EXPIRED", "Код истек, запросите новый")
				case strings.Contains(msg, "RESET_CODE_TOO_MANY_ATTEMPTS"):
					send(c, 401, "RESET_CODE_TOO_MANY_ATTEMPTS", "Превышено число попыток, запросите новый код")
				default:
					send(c, 401, "RESET_CODE_INVALID", "Неверный код")
				}
				return
			case codes.Internal:
				if strings.Contains(msg, "REDIS_NOT_CONFIGURED") {
					send(c, 500, "REDIS_NOT_CONFIGURED", "Кэш не настроен")
					return
				}
			}
		}
		logApp(c).ErrorContext(ctx, "password reset verify failed",
			"operation", "auth.password_reset.verify",
			"user.email_masked", appLogger.MaskEmail(r.Email),
			"error", err.Error(),
		)
		send(c, 500, "INTERNAL_ERROR", "Что-то пошло не так.")
		return
	}
	logAudit(c).InfoContext(ctx, "password reset code verified",
		"operation", "auth.password_reset.verify",
		"user.email_masked", appLogger.MaskEmail(r.Email),
	)
	c.JSON(200, passwordResetVerifyResponse{ResetToken: resp.GetToken()})
}

func (h *AuthHandler) ConfirmPasswordReset(c *gin.Context) {
	var r passwordResetConfirmRequest
	if err := c.ShouldBindJSON(&r); err != nil {
		send(c, 400, "BAD_REQUEST", "Неверный формат запроса")
		return
	}
	ctx := c.Request.Context()
	_, err := h.client.ConfirmPasswordReset(ctx, &authv1.ConfirmPasswordResetRequest{
		ResetToken:      r.ResetToken,
		Password:        r.Password,
		PasswordConfirm: r.PasswordConfirm,
	})
	if err != nil {
		st, ok := status.FromError(err)
		if ok {
			msg := st.Message()
			switch st.Code() {
			case codes.InvalidArgument:
				switch {
				case strings.Contains(msg, "RESET_TOKEN_REQUIRED"):
					send(c, 400, "RESET_TOKEN_REQUIRED", "Отсутствует токен восстановления")
				case strings.Contains(msg, "PASSWORD_REQUIRED"):
					send(c, 400, "PASSWORD_REQUIRED", "Введите пароль")
				case strings.Contains(msg, "INVALID_PASSWORD"):
					send(c, 400, "INVALID_PASSWORD_FORMAT", "Пароль должен содержать минимум 8 символов и цифру")
				case strings.Contains(msg, "PASSWORD_CONFIRMATION_MISMATCH"):
					send(c, 400, "PASSWORD_CONFIRMATION_MISMATCH", "Пароли не совпадают")
				default:
					send(c, 400, "BAD_REQUEST", msg)
				}
				return
			case codes.Unauthenticated:
				switch {
				case strings.Contains(msg, "RESET_SESSION_EXPIRED"):
					send(c, 401, "RESET_SESSION_EXPIRED", "Токен восстановления истек")
				default:
					send(c, 401, "RESET_SESSION_INVALID", "Неверный токен восстановления")
				}
				return
			case codes.Internal:
				if strings.Contains(msg, "REDIS_NOT_CONFIGURED") {
					send(c, 500, "REDIS_NOT_CONFIGURED", "Кэш не настроен")
					return
				}
			}
		}
		logApp(c).ErrorContext(ctx, "password reset confirm failed", "operation", "auth.password_reset.confirm", "error", err.Error())
		send(c, 500, "INTERNAL_ERROR", "Что-то пошло не так.")
		return
	}
	logAudit(c).InfoContext(ctx, "password reset confirmed", "operation", "auth.password_reset.confirm")
	c.Status(200)
}

func mapAuthGRPCError(c *gin.Context, err error) {
	st, ok := status.FromError(err)
	if !ok {
		logApp(c).ErrorContext(c.Request.Context(), "auth rpc failed", "error", err.Error())
		send(c, 500, "INTERNAL_ERROR", "Что-то пошло не так.")
		return
	}
	msg := st.Message()
	switch st.Code() {
	case codes.InvalidArgument:
		switch {
		case strings.Contains(msg, "EMAIL_REQUIRED"):
			send(c, 400, "EMAIL_REQUIRED", "Введите e-mail")
		case strings.Contains(msg, "INVALID_EMAIL_FORMAT"):
			send(c, 400, "INVALID_EMAIL_FORMAT", "Проверьте формат: name@domain.com")
		case strings.Contains(msg, "PASSWORD_REQUIRED"):
			send(c, 400, "PASSWORD_REQUIRED", "Введите пароль")
		case strings.Contains(msg, "INVALID_PASSWORD"):
			send(c, 400, "INVALID_PASSWORD_FORMAT", "Пароль должен содержать минимум 8 символов и цифру")
		default:
			send(c, 400, "BAD_REQUEST", msg)
		}
	case codes.AlreadyExists:
		send(c, 409, "EMAIL_ALREADY_EXISTS", "Такой e-mail уже зарегистрирован")
	case codes.Internal:
		logApp(c).ErrorContext(c.Request.Context(), "auth rpc internal error", "error", msg)
		send(c, 500, "INTERNAL_ERROR", "Что-то пошло не так.")
	default:
		logApp(c).ErrorContext(c.Request.Context(), "auth rpc failed", "error", msg)
		send(c, 500, "INTERNAL_ERROR", "Что-то пошло не так.")
	}
}

func mapLoginGRPCError(c *gin.Context, err error) {
	st, ok := status.FromError(err)
	if !ok {
		logApp(c).ErrorContext(c.Request.Context(), "login rpc failed", "error", err.Error())
		send(c, 500, "INTERNAL_ERROR", "Что-то пошло не так.")
		return
	}
	msg := st.Message()
	switch st.Code() {
	case codes.InvalidArgument:
		switch {
		case strings.Contains(msg, "EMAIL_REQUIRED"):
			send(c, 400, "EMAIL_REQUIRED", "Введите e-mail")
		case strings.Contains(msg, "INVALID_EMAIL_FORMAT"):
			send(c, 400, "INVALID_EMAIL_FORMAT", "Проверьте формат: name@domain.com")
		case strings.Contains(msg, "PASSWORD_REQUIRED"):
			send(c, 400, "PASSWORD_REQUIRED", "Введите пароль")
		case strings.Contains(msg, "INVALID_PASSWORD"):
			send(c, 400, "INVALID_PASSWORD_FORMAT", "Пароль должен содержать минимум 8 символов и цифру")
		default:
			send(c, 400, "BAD_REQUEST", msg)
		}
	case codes.AlreadyExists:
		send(c, 409, "EMAIL_ALREADY_EXISTS", "Такой e-mail уже зарегистрирован")
	case codes.Unauthenticated:
		send(c, 401, "INVALID_CREDENTIALS", "Неверный e-mail или пароль")
	case codes.Internal:
		logApp(c).ErrorContext(c.Request.Context(), "login rpc internal error", "error", msg)
		send(c, 500, "INTERNAL_ERROR", "Что-то пошло не так.")
	default:
		logApp(c).ErrorContext(c.Request.Context(), "login rpc failed", "error", msg)
		send(c, 500, "INTERNAL_ERROR", "Что-то пошло не так.")
	}
}
