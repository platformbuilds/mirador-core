package middleware

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/platformbuilds/mirador-core/internal/config"
)

// CORSMiddleware handles Cross-Origin Resource Sharing for MIRADOR-UI communication
func CORSMiddleware(corsConfig config.CORSConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		// Check if origin is allowed
		if isOriginAllowed(origin, corsConfig.AllowedOrigins) {
			c.Header("Access-Control-Allow-Origin", origin)
		}

		// Set allowed methods
		if len(corsConfig.AllowedMethods) > 0 {
			c.Header("Access-Control-Allow-Methods", strings.Join(corsConfig.AllowedMethods, ", "))
		} else {
			// Default methods for MIRADOR-CORE API
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
		}

		// Set allowed headers
		if len(corsConfig.AllowedHeaders) > 0 {
			c.Header("Access-Control-Allow-Headers", strings.Join(corsConfig.AllowedHeaders, ", "))
		} else {
			// Default headers for MIRADOR-CORE
			c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization, X-Tenant-ID, X-Session-Token")
		}

		// Set exposed headers
		if len(corsConfig.ExposedHeaders) > 0 {
			c.Header("Access-Control-Expose-Headers", strings.Join(corsConfig.ExposedHeaders, ", "))
		} else {
			// Default exposed headers for MIRADOR-CORE
			c.Header("Access-Control-Expose-Headers", "X-Rate-Limit-Limit, X-Rate-Limit-Remaining, X-Rate-Limit-Reset, X-Tenant-ID")
		}

		// Set credentials
		if corsConfig.AllowCredentials {
			c.Header("Access-Control-Allow-Credentials", "true")
		}

		// Set max age
		if corsConfig.MaxAge > 0 {
			c.Header("Access-Control-Max-Age", strconv.Itoa(corsConfig.MaxAge))
		} else {
			// Default: 12 hours for MIRADOR-CORE
			c.Header("Access-Control-Max-Age", "43200")
		}

		// Handle preflight requests
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// isOriginAllowed checks if the given origin is in the allowed origins list
func isOriginAllowed(origin string, allowedOrigins []string) bool {
	if len(allowedOrigins) == 0 {
		// If no origins specified, allow localhost and common development origins
		return strings.Contains(origin, "localhost") || strings.Contains(origin, "127.0.0.1") || strings.Contains(origin, "mirador-ui")
	}

	for _, allowedOrigin := range allowedOrigins {
		if allowedOrigin == "*" {
			return true
		}
		if origin == allowedOrigin {
			return true
		}
		// Support wildcard subdomains (e.g., *.mirador.io)
		if strings.HasPrefix(allowedOrigin, "*.") {
			domain := strings.TrimPrefix(allowedOrigin, "*.")
			if strings.HasSuffix(origin, domain) {
				return true
			}
		}
	}

	return false
}
