package api

import (
	"net/http"
	"slices"

	"github.com/gin-gonic/gin"
)

// cors allows the Vite dev server (a different origin than the Go API) to call
// the API from the browser. In production the SPA is served same-origin, so the
// allow-list stays empty-by-default rather than wide open.
func cors(allowedOrigins []string) gin.HandlerFunc {
	allowAny := slices.Contains(allowedOrigins, "*")

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")

		switch {
		case origin == "":
			// Not a browser cross-origin request; nothing to negotiate.
		case allowAny:
			c.Header("Access-Control-Allow-Origin", "*")
		case slices.Contains(allowedOrigins, origin):
			c.Header("Access-Control-Allow-Origin", origin)
			// Vary matters once the header depends on the request origin.
			c.Header("Vary", "Origin")
		}

		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		c.Header("Access-Control-Max-Age", "600")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
