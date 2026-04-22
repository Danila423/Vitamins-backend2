// Package middleware contains gin middleware that guards HTTP routes for
// authentication. It intentionally keeps the response shape identical to the
// handler package (ErrorResponse{code, message}) so the frontend contract
// does not change.
package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"

	"vitamins-backend_2/internal/auth/service"
)

const userIDKey = "userID"

// errorBody is the wire format middleware emits on auth failures. It is a
// duplicate of handler.ErrorResponse kept local here to avoid importing the
// handler package (which would create a cycle since handler imports this one).
type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func respond(c *gin.Context, status int, code, message string) {
	c.JSON(status, errorBody{Code: code, Message: message})
}

func AuthMiddleware(jwt *service.JWTManager) gin.HandlerFunc {
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
		claims, err := jwt.ParseWithType(token, service.TokenTypeAccess)
		if err != nil {
			respond(c, 401, "INVALID_TOKEN", "Неверный токен")
			c.Abort()
			return
		}
		c.Set(userIDKey, claims.UserID)
		c.Next()
	}
}

// OptionalAuthMiddleware parses an access token when present and sets the
// userID in the context. Absent or invalid tokens are ignored so the request
// can still be processed (used for analytics ingestion where the client may
// not be logged in).
func OptionalAuthMiddleware(jwt *service.JWTManager) gin.HandlerFunc {
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
		claims, err := jwt.ParseWithType(token, service.TokenTypeAccess)
		if err != nil {
			respond(c, 401, "INVALID_TOKEN", "Неверный токен")
			c.Abort()
			return
		}
		c.Set(userIDKey, claims.UserID)
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
