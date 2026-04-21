package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthMiddleware(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	jwt := NewJWTManager("test-secret", time.Minute, time.Minute)

	t.Run("missing header", func(t *testing.T) {
		t.Parallel()
		r := gin.New()
		r.Use(AuthMiddleware(jwt))
		r.GET("/protected", func(c *gin.Context) { c.Status(http.StatusOK) })

		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("invalid token", func(t *testing.T) {
		t.Parallel()
		r := gin.New()
		r.Use(AuthMiddleware(jwt))
		r.GET("/protected", func(c *gin.Context) { c.Status(http.StatusOK) })

		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Bearer bad-token")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("valid token sets user id", func(t *testing.T) {
		t.Parallel()
		r := gin.New()
		r.Use(AuthMiddleware(jwt))
		r.GET("/protected", func(c *gin.Context) {
			userID, ok := UserIDFromContext(c)
			assert.True(t, ok)
			assert.Equal(t, int64(42), userID)
			c.Status(http.StatusOK)
		})

		pair, err := jwt.GenerateTokenPair(42)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Bearer "+pair.AccessToken)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})
}
