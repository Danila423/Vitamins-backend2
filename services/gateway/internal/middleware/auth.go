package middleware

import (
	"crypto/subtle"
	"strings"

	"github.com/gin-gonic/gin"

	"vitamins-backend_2/pkg/jwt"
)

const userIDKey = "userID"

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func respond(c *gin.Context, status int, code, message string) {
	c.JSON(status, errorBody{Code: code, Message: message})
}

func AuthMiddleware(jwtMgr *jwt.JWTManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := strings.TrimSpace(c.GetHeader("Authorization"))
		if authHeader == "" {
			respond(c, 401, "AUTH_REQUIRED", "Требуется авторизация")
			c.Abort()
			return
		}
		const prefix = "Bearer "
		if !strings.HasPrefix(authHeader, prefix) {
			respond(c, 401, "INVALID_TOKEN", "Неверный токен")
			c.Abort()
			return
		}
		token := strings.TrimSpace(strings.TrimPrefix(authHeader, prefix))
		if token == "" {
			respond(c, 401, "INVALID_TOKEN", "Неверный токен")
			c.Abort()
			return
		}
		claims, err := jwtMgr.ParseWithType(token, jwt.TokenTypeAccess)
		if err != nil {
			respond(c, 401, "INVALID_TOKEN", "Неверный токен")
			c.Abort()
			return
		}
		c.Set(userIDKey, claims.UserID)
		c.Next()
	}
}

func OptionalAuthMiddleware(jwtMgr *jwt.JWTManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := strings.TrimSpace(c.GetHeader("Authorization"))
		if authHeader == "" {
			c.Next()
			return
		}
		const prefix = "Bearer "
		if !strings.HasPrefix(authHeader, prefix) {
			respond(c, 401, "INVALID_TOKEN", "Неверный токен")
			c.Abort()
			return
		}
		token := strings.TrimSpace(strings.TrimPrefix(authHeader, prefix))
		if token == "" {
			respond(c, 401, "INVALID_TOKEN", "Неверный токен")
			c.Abort()
			return
		}
		claims, err := jwtMgr.ParseWithType(token, jwt.TokenTypeAccess)
		if err != nil {
			respond(c, 401, "INVALID_TOKEN", "Неверный токен")
			c.Abort()
			return
		}
		c.Set(userIDKey, claims.UserID)
		c.Next()
	}
}

func AdminTokenMiddleware(adminToken string) gin.HandlerFunc {
	expected := []byte(strings.TrimSpace(adminToken))
	return func(c *gin.Context) {
		if len(expected) == 0 {
			respond(c, 401, "ADMIN_REQUIRED", "Админ токен не настроен")
			c.Abort()
			return
		}
		provided := []byte(strings.TrimSpace(c.GetHeader("X-Admin-Token")))
		if len(provided) != len(expected) || subtle.ConstantTimeCompare(provided, expected) != 1 {
			respond(c, 401, "ADMIN_REQUIRED", "Требуется админ токен")
			c.Abort()
			return
		}
		c.Next()
	}
}

func UserIDFromContext(c *gin.Context) (int64, bool) {
	v, ok := c.Get(userIDKey)
	if !ok {
		return 0, false
	}
	id, ok := v.(int64)
	return id, ok
}
