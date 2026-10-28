package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"sportss/internal/config"
	"sportss/internal/models"
	"sportss/internal/services"
)

// Auth carga el usuario desde la cookie JWT. No bloquea invitados.
func Auth(cfg *config.Config, auth *services.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie(cfg.CookieName)
		if err != nil || token == "" {
			c.Next()
			return
		}
		user, err := auth.Authenticate(c.Request.Context(), token)
		if err != nil {
			c.SetCookie(cfg.CookieName, "", -1, "/", "", cfg.CookieSecure, true)
			c.Next()
			return
		}
		c.Set(CtxUser, user)
		c.Next()
	}
}

// RequireAuth exige una sesión válida y redirige a login en vistas.
func RequireAuth(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if CurrentUser(c) == nil {
			abortUnauthorized(c)
			return
		}
		c.Next()
	}
}

// RequireRole restringe por rol.
func RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		u := CurrentUser(c)
		if u == nil {
			abortUnauthorized(c)
			return
		}
		for _, r := range roles {
			if u.Role == r {
				c.Next()
				return
			}
		}
		if IsAPI(c) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "No tienes permisos para esta acción."})
		} else {
			c.String(http.StatusForbidden, "No tienes permisos para acceder a esta página.")
			c.Abort()
		}
	}
}

func RequireBusiness() gin.HandlerFunc {
	return RequireRole(models.RoleBusiness, models.RoleAdmin)
}

func RequireAdmin() gin.HandlerFunc {
	return RequireRole(models.RoleAdmin)
}

// NoCache evita que el navegador muestre vistas autenticadas desde caché.
func NoCache() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store, no-cache, must-revalidate")
		c.Next()
	}
}

func nextPath(c *gin.Context) string {
	n := c.Query("next")
	if !strings.HasPrefix(n, "/") || strings.HasPrefix(n, "//") {
		return "/"
	}
	return n
}
