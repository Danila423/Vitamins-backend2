package handler

import (
	"github.com/gin-gonic/gin"

	"vitamins-backend_2/internal/auth/middleware"
	"vitamins-backend_2/internal/auth/service"
)

// UpdateProfileRequest — body for PATCH /users/me. Pointers distinguish
// "field not provided" from "field set to empty string".
type UpdateProfileRequest struct {
	FirstName *string `json:"firstName"`
	LastName  *string `json:"lastName"`
	Email     *string `json:"email"`
}

// UserProfileResponse — body for GET /users/me and successful PATCH /users/me.
type UserProfileResponse struct {
	ID        int64  `json:"id"`
	Email     string `json:"email"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
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
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		send(c, 401, "AUTH_REQUIRED", "Требуется авторизация")
		return
	}
	ctx := c.Request.Context()
	u, err := h.s.UpdateProfile(ctx, userID, service.ProfileUpdate{
		FirstName: r.FirstName,
		LastName:  r.LastName,
		Email:     r.Email,
	})
	if err != nil {
		switch err {
		case service.ErrNoFieldsToUpdate:
			send(c, 400, "NO_FIELDS_TO_UPDATE", "Нечего обновлять")
		case service.ErrEmailRequired:
			send(c, 400, "EMAIL_REQUIRED", "Введите e-mail")
		case service.ErrInvalidEmailFormat:
			send(c, 400, "INVALID_EMAIL_FORMAT", "Проверьте формат: name@domain.com")
		case service.ErrEmailAlreadyExists:
			send(c, 409, "EMAIL_ALREADY_EXISTS", "Такой e-mail уже зарегистрирован")
		case service.ErrUserNotFound:
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
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		send(c, 401, "AUTH_REQUIRED", "Требуется авторизация")
		return
	}
	ctx := c.Request.Context()
	u, err := h.s.GetProfile(ctx, userID)
	if err != nil {
		switch err {
		case service.ErrUserNotFound:
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
