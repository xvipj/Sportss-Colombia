package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"sportss/internal/models"
)

const (
	CtxUser  = "user"
	CtxCSRF  = "csrf"
	CtxFlash = "flash"
)

func CurrentUser(c *gin.Context) *models.User {
	v, ok := c.Get(CtxUser)
	if !ok {
		return nil
	}
	u, _ := v.(*models.User)
	return u
}

func IsAPI(c *gin.Context) bool {
	return strings.HasPrefix(c.Request.URL.Path, "/api/") ||
		strings.Contains(c.GetHeader("Accept"), "application/json")
}

func abortUnauthorized(c *gin.Context) {
	if IsAPI(c) {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Debes iniciar sesión."})
		return
	}
	next := c.Request.URL.RequestURI()
	c.Redirect(http.StatusFound, "/login?next="+next)
	c.Abort()
}
