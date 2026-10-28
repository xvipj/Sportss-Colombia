package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"sportss/internal/config"
)

// SecurityHeaders agrega cabeceras defensivas y una CSP compatible con las
// fuentes, Chart.js y el mapa embebido de OpenStreetMap.
func SecurityHeaders(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "SAMEORIGIN")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Permissions-Policy", "geolocation=(self), camera=(), microphone=()")
		h.Set("Content-Security-Policy",
			"default-src 'self'; "+
				"img-src 'self' data: https:; "+
				"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; "+
				"font-src 'self' https://fonts.gstatic.com; "+
				"script-src 'self' https://cdn.jsdelivr.net; "+
				"connect-src 'self'; "+
				"frame-src https://www.openstreetmap.org; "+
				"object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'self'")
		if cfg.IsProduction() {
			h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		c.Next()
	}
}

// BodyLimit limita el tamaño de las peticiones con cuerpo.
func BodyLimit(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		}
		c.Next()
	}
}
