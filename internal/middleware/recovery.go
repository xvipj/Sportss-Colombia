package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

// RequestLogger registra cada petición con su estado y duración.
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		attrs := []any{
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"duration_ms", time.Since(start).Milliseconds(),
			"ip", c.ClientIP(),
		}
		if u := CurrentUser(c); u != nil {
			attrs = append(attrs, "user_id", u.ID)
		}
		if c.Writer.Status() >= 500 {
			slog.Error("request", attrs...)
		} else {
			slog.Info("request", attrs...)
		}
	}
}

// Recovery captura panics y responde de forma consistente.
func Recovery() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered any) {
		slog.Error("panic recuperado", "error", recovered, "path", c.Request.URL.Path, "ip", c.ClientIP())
		if IsAPI(c) {
			c.AbortWithStatusJSON(500, gin.H{"error": "Ocurrió un error inesperado. Intenta de nuevo."})
			return
		}
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(500, "<!doctype html><html lang=\"es\"><meta charset=\"utf-8\"><title>Error</title>"+
			"<body style=\"font-family:system-ui;padding:3rem;text-align:center;background:#f8fafc\">"+
			"<h1>Algo salió mal</h1><p>Estamos revisando el problema. Vuelve a intentarlo.</p>"+
			"<a href=\"/\" style=\"color:#0f766e\">Volver al inicio</a></body></html>")
		c.Abort()
	})
}
