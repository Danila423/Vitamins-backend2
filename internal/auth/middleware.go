package auth

import (
	"strings"

	"github.com/gin-gonic/gin"
)

const userIDKey = "userID"

func AuthMiddleware(jwt *JWTManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := strings.TrimSpace(c.GetHeader("Authorization"))
		if authHeader == "" {
			send(c, 401, "AUTH_REQUIRED", "Требуется авторизация")
			c.Abort()
			return
		}
		const prefix = "Bearer "
		if !strings.HasPrefix(authHeader, prefix) {
			send(c, 401, "INVALID_TOKEN", "Неверный токен")
			c.Abort()
			return
		}
		token := strings.TrimSpace(strings.TrimPrefix(authHeader, prefix))
		if token == "" {
			send(c, 401, "INVALID_TOKEN", "Неверный токен")
			c.Abort()
			return
		}
		claims, err := jwt.Parse(token)
		if err != nil {
			send(c, 401, "INVALID_TOKEN", "Неверный токен")
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
