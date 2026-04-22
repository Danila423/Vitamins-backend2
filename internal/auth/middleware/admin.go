package middleware

import (
	"crypto/subtle"
	"strings"

	"github.com/gin-gonic/gin"
)

// AdminTokenMiddleware rejects requests missing or with wrong X-Admin-Token.
// Uses constant-time comparison to avoid timing leaks.
// When the admin token is not configured on the server, any request is refused
// with ADMIN_REQUIRED (same code already expected by clients).
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
