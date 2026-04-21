package metrics

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		get().HTTPInFlight.Inc()
		defer get().HTTPInFlight.Dec()

		c.Next()

		route := c.FullPath()
		if route == "" {
			route = "unknown"
		}
		method := strings.ToUpper(strings.TrimSpace(c.Request.Method))
		if method == "" {
			method = "UNKNOWN"
		}
		statusCode := c.Writer.Status()
		statusClass := statusClassFromCode(statusCode)
		outcome := outcomeFromStatus(statusCode)

		get().HTTPRequestTotal.WithLabelValues(method, route, statusClass, outcome).Inc()
		get().HTTPRequestDuration.WithLabelValues(method, route, statusClass, outcome).
			Observe(time.Since(start).Seconds())
	}
}

func statusClassFromCode(code int) string {
	switch {
	case code >= http.StatusOK && code < 300:
		return "2xx"
	case code >= 300 && code < 400:
		return "3xx"
	case code >= 400 && code < 500:
		return "4xx"
	case code >= 500 && code < 600:
		return "5xx"
	default:
		return strconv.Itoa(code)
	}
}

func outcomeFromStatus(code int) string {
	switch {
	case code >= 200 && code < 400:
		return "success"
	case code >= 400 && code < 500:
		return "client_error"
	case code >= 500:
		return "server_error"
	default:
		return "unknown"
	}
}
