package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"sportss/internal/middleware"
	"sportss/internal/services"
)

func (a *App) Profile(c *gin.Context) {
	user := middleware.CurrentUser(c)
	bookings, _ := a.bookings.AllByUser(c.Request.Context(), user.ID)
	invoices, _ := a.invoices.ListByUser(c.Request.Context(), user.ID)
	payments, _ := a.payments.ListByUser(c.Request.Context(), user.ID)
	a.renderPage(c, http.StatusOK, "profile", "Mi perfil", map[string]any{
		"ProfileUser": user, "Bookings": bookings, "Invoices": invoices, "Payments": payments,
	})
}

func (a *App) UpdateProfile(c *gin.Context) {
	user := middleware.CurrentUser(c)
	oldAvatar := user.AvatarPath
	avatarPath := ""
	if fh, err := c.FormFile("avatar"); err == nil {
		path, err := a.saveUploadedImage(fh, "avatar")
		if err != nil {
			a.setFlash(c, "error", "No pudimos subir la foto: "+err.Error())
			c.Redirect(http.StatusFound, "/perfil")
			return
		}
		avatarPath = path
	}
	input := services.ProfileInput{
		Name: c.PostForm("name"), BusinessName: c.PostForm("business_name"),
		NIT: c.PostForm("nit"), Phone: c.PostForm("phone"),
		EmailNotify: c.PostForm("email_notify") == "on" || c.PostForm("email_notify") == "1",
		AvatarPath:  avatarPath,
	}
	err := a.auth.UpdateProfile(c.Request.Context(), user, input)
	if err != nil {
		if avatarPath != "" {
			a.deleteUploadPath(avatarPath)
		}
		a.setFlash(c, "error", errorMessage(err))
		c.Redirect(http.StatusFound, "/perfil")
		return
	}
	if avatarPath != "" {
		a.deleteUploadPath(oldAvatar)
	}
	a.setFlash(c, "success", "Perfil actualizado correctamente.")
	c.Redirect(http.StatusFound, "/perfil")
}

func (a *App) ChangePassword(c *gin.Context) {
	user := middleware.CurrentUser(c)
	err := a.auth.ChangePassword(c.Request.Context(), user,
		c.PostForm("current_password"), c.PostForm("new_password"), c.PostForm("confirm_password"))
	if err != nil {
		a.setFlash(c, "error", errorMessage(err))
		c.Redirect(http.StatusFound, "/perfil#seguridad")
		return
	}
	a.setFlash(c, "success", "Contraseña actualizada. Usa la nueva en tu próximo inicio de sesión.")
	c.Redirect(http.StatusFound, "/perfil#seguridad")
}
