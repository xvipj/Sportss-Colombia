package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"sportss/internal/middleware"
	"sportss/internal/services"
)

type loginRequest struct {
	Email    string `json:"email" form:"email"`
	Password string `json:"password" form:"password"`
}

type registerRequest struct {
	Name            string `json:"name" form:"name"`
	Email           string `json:"email" form:"email"`
	Password        string `json:"password" form:"password"`
	ConfirmPassword string `json:"confirm_password" form:"confirm_password"`
	AccountType     string `json:"account_type" form:"account_type"`
	BusinessName    string `json:"business_name" form:"business_name"`
	NIT             string `json:"nit" form:"nit"`
	Phone           string `json:"phone" form:"phone"`
}

func (a *App) LoginPage(c *gin.Context) {
	if middleware.CurrentUser(c) != nil {
		c.Redirect(http.StatusFound, "/")
		return
	}
	a.renderPage(c, http.StatusOK, "login", "Iniciar sesión", map[string]any{
		"Next": c.Query("next"),
	})
}

func (a *App) LoginSubmit(c *gin.Context) {
	var req loginRequest
	if strings.Contains(c.GetHeader("Content-Type"), "application/json") {
		if err := c.ShouldBindJSON(&req); err != nil {
			a.writeError(c, services.BadRequest("Datos de acceso inválidos."))
			return
		}
	} else {
		req.Email = c.PostForm("email")
		req.Password = c.PostForm("password")
	}
	user, token, err := a.auth.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		if middleware.IsAPI(c) {
			a.writeError(c, err)
			return
		}
		a.renderPage(c, errorStatus(err), "login", "Iniciar sesión", map[string]any{
			"Error": errorMessage(err), "Email": req.Email, "Next": c.PostForm("next"),
		})
		return
	}
	a.setSession(c, token)
	if middleware.IsAPI(c) {
		c.JSON(http.StatusOK, gin.H{"user": user, "message": "Sesión iniciada"})
		return
	}
	a.setFlash(c, "success", "Bienvenido de nuevo, "+user.Name+".")
	next := safeNext(c.PostForm("next"))
	if next == "/" && user.Role != "player" {
		next = "/panel"
	}
	c.Redirect(http.StatusFound, next)
}

func (a *App) RegisterPage(c *gin.Context) {
	if middleware.CurrentUser(c) != nil {
		c.Redirect(http.StatusFound, "/")
		return
	}
	accountType := "persona"
	if c.Query("tipo") == "empresa" {
		accountType = "empresa"
	}
	a.renderPage(c, http.StatusOK, "register", "Crear cuenta", map[string]any{
		"Form": map[string]string{"account_type": accountType},
	})
}

func (a *App) RegisterSubmit(c *gin.Context) {
	var req registerRequest
	if strings.Contains(c.GetHeader("Content-Type"), "application/json") {
		if err := c.ShouldBindJSON(&req); err != nil {
			a.writeError(c, services.BadRequest("Datos de registro inválidos."))
			return
		}
	} else {
		req.Name = c.PostForm("name")
		req.Email = c.PostForm("email")
		req.Password = c.PostForm("password")
		req.ConfirmPassword = c.PostForm("confirm_password")
		req.AccountType = c.PostForm("account_type")
		req.BusinessName = c.PostForm("business_name")
		req.NIT = c.PostForm("nit")
		req.Phone = c.PostForm("phone")
	}
	user, token, err := a.auth.Register(c.Request.Context(), services.RegisterInput{
		Name: req.Name, Email: req.Email, Password: req.Password, ConfirmPassword: req.ConfirmPassword,
		AccountType: req.AccountType, BusinessName: req.BusinessName, NIT: req.NIT, Phone: req.Phone,
	})
	if err != nil {
		if middleware.IsAPI(c) {
			a.writeError(c, err)
			return
		}
		a.renderPage(c, errorStatus(err), "register", "Crear cuenta", map[string]any{
			"Error": errorMessage(err),
			"Form": map[string]string{
				"name": req.Name, "email": req.Email, "account_type": req.AccountType,
				"business_name": req.BusinessName, "nit": req.NIT, "phone": req.Phone,
			},
		})
		return
	}
	a.setSession(c, token)
	if middleware.IsAPI(c) {
		c.JSON(http.StatusCreated, gin.H{"user": user, "message": "Cuenta creada"})
		return
	}
	a.setFlash(c, "success", "Tu cuenta fue creada. ¡Bienvenido a Sportss Colombia!")
	if user.Role == "business" {
		c.Redirect(http.StatusFound, "/panel")
		return
	}
	c.Redirect(http.StatusFound, "/")
}

func (a *App) Logout(c *gin.Context) {
	a.clearSession(c)
	if middleware.IsAPI(c) {
		c.JSON(http.StatusOK, gin.H{"message": "Sesión cerrada"})
		return
	}
	a.setFlash(c, "success", "Cerraste sesión correctamente.")
	c.Redirect(http.StatusFound, "/login")
}
