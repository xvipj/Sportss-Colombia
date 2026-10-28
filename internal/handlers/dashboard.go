package handlers

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"sportss/internal/middleware"
	"sportss/internal/models"
	"sportss/internal/repositories"
	"sportss/internal/services"
)

func (a *App) Dashboard(c *gin.Context) {
	user := middleware.CurrentUser(c)
	if user.Role == models.RoleAdmin {
		c.Redirect(http.StatusFound, "/admin/reportes")
		return
	}
	from, to := a.rangeFromQuery(c)
	ownerID := user.ID
	summary, err := a.analytics.Summary(c.Request.Context(), &ownerID, from, to)
	if err != nil {
		a.renderError(c, errorStatus(err), "Panel", errorMessage(err))
		return
	}
	courts, _ := a.courts.ListByOwner(c.Request.Context(), user.ID)
	recent, _, _ := a.bookings.ListByOwner(c.Request.Context(), user.ID, "", "", 1, 6)
	reviewCount, _ := a.reviews.CountByOwner(c.Request.Context(), user.ID)
	a.renderPage(c, http.StatusOK, "dashboard", "Panel de administración", map[string]any{
		"Summary": summary, "Courts": courts, "Recent": recent, "ReviewCount": reviewCount,
		"From": from, "To": to, "FromISO": from.Format("2006-01-02"), "ToISO": to.Format("2006-01-02"),
		"Chart": chartPayload(summary),
	})
}

func (a *App) PanelCourts(c *gin.Context) {
	user := middleware.CurrentUser(c)
	courts, err := a.courts.ListByOwner(c.Request.Context(), user.ID)
	if err != nil {
		a.renderError(c, errorStatus(err), "Mis canchas", errorMessage(err))
		return
	}
	a.renderPage(c, http.StatusOK, "panel_courts", "Mis canchas", map[string]any{
		"Courts": courts,
	})
}

func (a *App) NewCourtForm(c *gin.Context) {
	a.renderCourtForm(c, http.StatusOK, nil, "", false)
}

func (a *App) EditCourtForm(c *gin.Context) {
	user := middleware.CurrentUser(c)
	id, ok := a.parseID(c, "id")
	if !ok {
		return
	}
	court, err := a.courts.Get(c.Request.Context(), id)
	if err != nil {
		a.renderError(c, errorStatus(err), "Editar cancha", errorMessage(err))
		return
	}
	if court.OwnerID != user.ID && user.Role != models.RoleAdmin {
		a.renderError(c, http.StatusForbidden, "Editar cancha", "No tienes permiso para editar esta cancha.")
		return
	}
	a.renderCourtForm(c, http.StatusOK, court, "", true)
}

func (a *App) renderCourtForm(c *gin.Context, status int, court *models.Court, errMsg string, isEdit bool) {
	title := "Publicar cancha"
	if isEdit {
		title = "Editar cancha"
	}
	a.renderPage(c, status, "court_form", title, map[string]any{
		"Court": court, "IsEdit": isEdit, "Error": errMsg,
		"SurfacesJSON": services.SurfacesBySport,
		"Today":        time.Now().In(a.cfg.Location).Format("2006-01-02"),
	})
}

func (a *App) SaveCourt(c *gin.Context) {
	user := middleware.CurrentUser(c)
	idStr := c.Param("id")
	isEdit := idStr != ""
	var existing *models.Court
	if isEdit {
		id, err := strconv.ParseUint(idStr, 10, 64)
		if err != nil || id == 0 {
			a.renderError(c, http.StatusNotFound, "Editar cancha", "Cancha no encontrada.")
			return
		}
		court, err := a.courts.Get(c.Request.Context(), uint(id))
		if err != nil {
			a.renderError(c, errorStatus(err), "Editar cancha", errorMessage(err))
			return
		}
		if court.OwnerID != user.ID && user.Role != models.RoleAdmin {
			a.renderError(c, http.StatusForbidden, "Editar cancha", "No tienes permiso para editar esta cancha.")
			return
		}
		existing = court
	}

	input, err := courtInputFromForm(c)
	if err != nil {
		a.renderCourtForm(c, http.StatusBadRequest, existing, err.Error(), isEdit)
		return
	}

	if err := c.Request.ParseMultipartForm(32 << 20); err != nil {
		a.renderCourtForm(c, http.StatusBadRequest, existing, "No pudimos leer el formulario.", isEdit)
		return
	}
	files := c.Request.MultipartForm.File["photos"]
	existingCount := 0
	if existing != nil {
		existingCount = len(existing.Photos)
	}
	deletes := c.PostFormArray("delete_photos")
	remaining := existingCount - len(deletes)
	if remaining < 0 {
		remaining = 0
	}
	if len(files) > 0 && len(files)+remaining > 5 {
		a.renderCourtForm(c, http.StatusBadRequest, existing, "Una cancha puede tener máximo 5 fotos en total.", isEdit)
		return
	}
	savedPaths := make([]string, 0, len(files))
	for _, fh := range files {
		path, saveErr := a.saveUploadedImage(fh, "court")
		if saveErr != nil {
			for _, saved := range savedPaths {
				a.deleteUploadPath(saved)
			}
			a.renderCourtForm(c, http.StatusBadRequest, existing, "Foto inválida: "+saveErr.Error(), isEdit)
			return
		}
		savedPaths = append(savedPaths, path)
	}

	var court *models.Court
	if isEdit {
		court, err = a.courts.Update(c.Request.Context(), existing.ID, user.ID, user.Role == models.RoleAdmin, input)
	} else {
		court, err = a.courts.Create(c.Request.Context(), user.ID, input)
	}
	if err != nil {
		for _, path := range savedPaths {
			a.deleteUploadPath(path)
		}
		status := errorStatus(err)
		a.renderCourtForm(c, status, existing, errorMessage(err), isEdit)
		return
	}

	for _, path := range savedPaths {
		if err := a.courts.AddPhoto(c.Request.Context(), court.ID, user.ID, path); err != nil {
			a.deleteUploadPath(path)
			a.setFlash(c, "error", "La cancha se guardó, pero una foto no pudo agregarse: "+errorMessage(err))
		}
	}
	for _, rawID := range deletes {
		photoID, parseErr := strconv.ParseUint(rawID, 10, 64)
		if parseErr == nil {
			_ = a.courts.DeletePhoto(c.Request.Context(), court.ID, uint(photoID), user.ID, user.Role == models.RoleAdmin)
		}
	}
	a.setFlash(c, "success", "Cancha guardada correctamente.")
	c.Redirect(http.StatusFound, "/panel/canchas")
}

func (a *App) ToggleCourtStatus(c *gin.Context) {
	user := middleware.CurrentUser(c)
	id, ok := a.parseID(c, "id")
	if !ok {
		return
	}
	status := c.PostForm("status")
	if err := a.courts.SetStatus(c.Request.Context(), id, user.ID, user.Role == models.RoleAdmin, status); err != nil {
		a.setFlash(c, "error", errorMessage(err))
	} else if status == models.CourtPaused {
		a.setFlash(c, "success", "Cancha pausada. Ya no aparece en las búsquedas.")
	} else {
		a.setFlash(c, "success", "Cancha activada y visible nuevamente.")
	}
	target := "/panel/canchas"
	if user.Role == models.RoleAdmin {
		target = "/admin/canchas"
	}
	c.Redirect(http.StatusFound, target)
}

func (a *App) DeleteCourt(c *gin.Context) {
	user := middleware.CurrentUser(c)
	id, ok := a.parseID(c, "id")
	if !ok {
		return
	}
	if err := a.courts.Delete(c.Request.Context(), id, user.ID, user.Role == models.RoleAdmin); err != nil {
		a.setFlash(c, "error", errorMessage(err))
	} else {
		a.setFlash(c, "success", "Cancha eliminada.")
	}
	target := "/panel/canchas"
	if user.Role == models.RoleAdmin {
		target = "/admin/canchas"
	}
	c.Redirect(http.StatusFound, target)
}

func (a *App) AdminUsers(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	search := c.Query("q")
	users, total, err := a.auth.ListUsers(c.Request.Context(), search, page, 20)
	if err != nil {
		a.renderError(c, errorStatus(err), "Usuarios", errorMessage(err))
		return
	}
	a.renderPage(c, http.StatusOK, "admin_users", "Gestión de usuarios", map[string]any{
		"Users": users, "Total": total, "Page": page, "Search": search,
		"Pages": pageLinks("/admin/usuarios?q="+queryEscape(search), nil, page, int(total), 20),
	})
}

func (a *App) AdminToggleUser(c *gin.Context) {
	user := middleware.CurrentUser(c)
	id, ok := a.parseID(c, "id")
	if !ok {
		return
	}
	if id == user.ID {
		a.setFlash(c, "error", "No puedes bloquear tu propia cuenta.")
		c.Redirect(http.StatusFound, "/admin/usuarios")
		return
	}
	active := c.PostForm("active") == "true"
	if err := a.auth.SetUserActive(c.Request.Context(), id, active); err != nil {
		a.setFlash(c, "error", errorMessage(err))
	} else if active {
		a.setFlash(c, "success", "Usuario activado.")
	} else {
		a.setFlash(c, "success", "Usuario bloqueado; no podrá iniciar sesión.")
	}
	c.Redirect(http.StatusFound, "/admin/usuarios")
}

func (a *App) AdminCourts(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	search := c.Query("q")
	status := c.Query("status")
	courts, total, err := a.courts.ListAll(c.Request.Context(), search, status, page, 20)
	if err != nil {
		a.renderError(c, errorStatus(err), "Canchas", errorMessage(err))
		return
	}
	a.renderPage(c, http.StatusOK, "admin_courts", "Moderación de canchas", map[string]any{
		"Courts": courts, "Total": total, "Page": page, "Search": search, "Status": status,
		"Pages": pageLinks("/admin/canchas?q="+queryEscape(search)+"&status="+queryEscape(status), nil, page, int(total), 20),
	})
}

func (a *App) AdminReports(c *gin.Context) {
	from, to := a.rangeFromQuery(c)
	summary, err := a.analytics.Summary(c.Request.Context(), nil, from, to)
	if err != nil {
		a.renderError(c, errorStatus(err), "Reportes", errorMessage(err))
		return
	}
	users, _ := a.auth.CountUsers(c.Request.Context())
	courts, _ := a.courts.CountAll(c.Request.Context())
	a.renderPage(c, http.StatusOK, "admin_reports", "Reportes globales", map[string]any{
		"Summary": summary, "Users": users, "Courts": courts,
		"From": from, "To": to, "FromISO": from.Format("2006-01-02"), "ToISO": to.Format("2006-01-02"),
		"Chart": chartPayload(summary),
	})
}

func (a *App) rangeFromQuery(c *gin.Context) (time.Time, time.Time) {
	loc := a.cfg.Location
	now := time.Now().In(loc)
	to := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	from := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	if v := c.Query("from"); v != "" {
		if t, err := time.ParseInLocation("2006-01-02", v, loc); err == nil {
			from = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
		}
	}
	if v := c.Query("to"); v != "" {
		if t, err := time.ParseInLocation("2006-01-02", v, loc); err == nil {
			to = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
		}
	}
	if from.After(to) {
		from, to = to, from
	}
	if to.Sub(from) > 400*24*time.Hour {
		from = to.AddDate(0, 0, -365)
	}
	return from, to
}

func chartPayload(summary *repositories.AnalyticsSummary) map[string]any {
	labels := make([]string, 0, len(summary.Daily))
	counts := make([]int64, 0, len(summary.Daily))
	revenue := make([]int64, 0, len(summary.Daily))
	for _, p := range summary.Daily {
		labels = append(labels, p.Label)
		counts = append(counts, p.Count)
		revenue = append(revenue, p.Revenue)
	}
	courtLabels := make([]string, 0, len(summary.CourtStats))
	courtRevenue := make([]int64, 0, len(summary.CourtStats))
	for _, c := range summary.CourtStats {
		courtLabels = append(courtLabels, c.Name)
		courtRevenue = append(courtRevenue, c.Revenue)
	}
	return map[string]any{
		"labels": labels, "counts": counts, "revenue": revenue,
		"courtLabels": courtLabels, "courtRevenue": courtRevenue,
	}
}

func courtInputFromForm(c *gin.Context) (services.CourtInput, error) {
	length, err := strconv.ParseFloat(strings.TrimSpace(c.PostForm("length")), 64)
	if err != nil {
		return services.CourtInput{}, fmt.Errorf("las medidas deben ser números válidos")
	}
	width, err := strconv.ParseFloat(strings.TrimSpace(c.PostForm("width")), 64)
	if err != nil {
		return services.CourtInput{}, fmt.Errorf("las medidas deben ser números válidos")
	}
	price, err := strconv.ParseInt(strings.TrimSpace(c.PostForm("price_per_hour")), 10, 64)
	if err != nil {
		return services.CourtInput{}, fmt.Errorf("el precio por hora debe ser un número entero en COP")
	}
	lat, _ := strconv.ParseFloat(strings.TrimSpace(c.PostForm("latitude")), 64)
	lng, _ := strconv.ParseFloat(strings.TrimSpace(c.PostForm("longitude")), 64)
	return services.CourtInput{
		Name: c.PostForm("name"), Description: c.PostForm("description"),
		Sport: c.PostForm("sport"), Surface: c.PostForm("surface"),
		Length: length, Width: width,
		Covered:      c.PostForm("covered") == "on" || c.PostForm("covered") == "true",
		Floodlights:  c.PostForm("floodlights") == "on" || c.PostForm("floodlights") == "true",
		Parking:      c.PostForm("parking") == "on" || c.PostForm("parking") == "true",
		Bathrooms:    c.PostForm("bathrooms") == "on" || c.PostForm("bathrooms") == "true",
		Lockers:      c.PostForm("lockers") == "on" || c.PostForm("lockers") == "true",
		Cafeteria:    c.PostForm("cafeteria") == "on" || c.PostForm("cafeteria") == "true",
		PricePerHour: price, OpenTime: c.PostForm("open_time"), CloseTime: c.PostForm("close_time"),
		AvailableDays: c.PostFormArray("days"), Department: c.PostForm("department"),
		City: c.PostForm("city"), Neighborhood: c.PostForm("neighborhood"), Address: c.PostForm("address"),
		Latitude: lat, Longitude: lng, Status: c.DefaultPostForm("status", models.CourtActive),
	}, nil
}

func queryEscape(s string) string {
	return url.QueryEscape(s)
}
