package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"sportss/internal/middleware"
	"sportss/internal/services"
)

func (a *App) APICreateReview(c *gin.Context) {
	user := middleware.CurrentUser(c)
	var req struct {
		BookingID uint   `json:"booking_id"`
		Rating    int    `json:"rating"`
		Comment   string `json:"comment"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		a.writeError(c, services.BadRequest("Datos de reseña inválidos."))
		return
	}
	review, err := a.reviews.Create(c.Request.Context(), user.ID, req.BookingID, req.Rating, req.Comment)
	if err != nil {
		a.writeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"message": "¡Gracias por tu reseña!", "review": review})
}

func (a *App) APIRespondReview(c *gin.Context) {
	user := middleware.CurrentUser(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		a.writeError(c, services.NotFound("Reseña no encontrada."))
		return
	}
	var req struct {
		Response string `json:"response"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		a.writeError(c, services.BadRequest("Respuesta inválida."))
		return
	}
	if err := a.reviews.Respond(c.Request.Context(), uint(id), user.ID, req.Response, user.Role == "admin"); err != nil {
		a.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Respuesta publicada."})
}

func (a *App) OwnerReviews(c *gin.Context) {
	user := middleware.CurrentUser(c)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	reviews, total, err := a.reviews.ListByOwner(c.Request.Context(), user.ID, page, 15)
	if err != nil {
		a.renderError(c, errorStatus(err), "Reseñas", errorMessage(err))
		return
	}
	a.renderPage(c, http.StatusOK, "owner_reviews", "Reseñas de mis canchas", map[string]any{
		"Reviews": reviews, "Total": total, "Page": page,
		"Pages": pageLinks("/panel/resenas", nil, page, int(total), 15),
	})
}

func (a *App) OwnerRespondReview(c *gin.Context) {
	user := middleware.CurrentUser(c)
	id, ok := a.parseID(c, "id")
	if !ok {
		return
	}
	response := c.PostForm("response")
	if err := a.reviews.Respond(c.Request.Context(), id, user.ID, response, user.Role == "admin"); err != nil {
		a.setFlash(c, "error", errorMessage(err))
	} else {
		a.setFlash(c, "success", "Respuesta publicada.")
	}
	c.Redirect(http.StatusFound, "/panel/resenas")
}

func (a *App) AdminReviews(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	search := c.Query("q")
	reviews, total, err := a.reviews.ListAll(c.Request.Context(), search, page, 20)
	if err != nil {
		a.renderError(c, errorStatus(err), "Moderación de reseñas", errorMessage(err))
		return
	}
	a.renderPage(c, http.StatusOK, "admin_reviews", "Moderación de reseñas", map[string]any{
		"Reviews": reviews, "Total": total, "Page": page, "Search": search,
		"Pages": pageLinks("/admin/resenas", nil, page, int(total), 20),
	})
}

func (a *App) AdminToggleReview(c *gin.Context) {
	id, ok := a.parseID(c, "id")
	if !ok {
		return
	}
	hidden := c.PostForm("hidden") == "true"
	if err := a.reviews.SetHidden(c.Request.Context(), id, hidden); err != nil {
		a.setFlash(c, "error", errorMessage(err))
	} else if hidden {
		a.setFlash(c, "success", "Reseña ocultada del perfil público.")
	} else {
		a.setFlash(c, "success", "Reseña visible nuevamente.")
	}
	c.Redirect(http.StatusFound, "/admin/resenas")
}
