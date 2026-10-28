package middleware

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"sportss/internal/config"
)

const csrfCookie = "sportss_csrf"

// CSRF implementa double-submit cookie. El token se incluye en los formularios
// SSR y en la cabecera X-CSRF-Token para peticiones fetch.
func CSRF(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie(csrfCookie)
		if err != nil || len(token) < 32 {
			token = randomCSRF()
			c.SetCookie(csrfCookie, token, 60*60*24*7, "/", "", cfg.CookieSecure, false)
		}
		c.Set(CtxCSRF, token)

		if isSafeMethod(c.Request.Method) || c.Request.URL.Path == "/api/payments/webhook" {
			c.Next()
			return
		}

		sent := c.GetHeader("X-CSRF-Token")
		if sent == "" {
			sent = c.Request.FormValue("_csrf")
		}
		if subtle.ConstantTimeCompare([]byte(sent), []byte(token)) != 1 {
			if IsAPI(c) {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Token de seguridad inválido. Recarga la página."})
			} else {
				c.AbortWithStatus(http.StatusForbidden)
			}
			return
		}
		c.Next()
	}
}

func randomCSRF() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return strings.Repeat("0", 64)
	}
	return hex.EncodeToString(b)
}

func isSafeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	default:
		return false
	}
}
