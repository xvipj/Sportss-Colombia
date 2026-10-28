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
	"sportss/internal/repositories"
	"sportss/internal/services"
)

type PageLink struct {
	Number int
	URL    string
	Active bool
}

func (a *App) Home(c *gin.Context) {
	featured, err := a.courts.Featured(c.Request.Context(), 6)
	if err != nil {
		featured = nil
	}
	totalCourts, _ := a.courts.CountAll(c.Request.Context())
	a.renderPage(c, http.StatusOK, "home", "Reserva canchas deportivas en Colombia", map[string]any{
		"Featured": featured, "TotalCourts": totalCourts,
		"Today": time.Now().In(a.cfg.Location).Format("2006-01-02"),
		"Hero":  true,
	})
}

func (a *App) SearchCourts(c *gin.Context) {
	params, err := a.parseCourtSearch(c)
	if err != nil {
		a.renderError(c, errorStatus(err), "Búsqueda inválida", errorMessage(err))
		return
	}
	result, err := a.courts.Search(c.Request.Context(), params)
	if err != nil {
		a.renderError(c, errorStatus(err), "No pudimos buscar", errorMessage(err))
		return
	}
	pages := buildPagination(c.Request.URL.Query(), result.Page, int(result.Total), result.Limit)
	totalPages := (int(result.Total) + result.Limit - 1) / result.Limit
	if totalPages < 1 {
		totalPages = 1
	}
	prevURL, nextURL := "", ""
	if result.Page > 1 {
		v := cloneValues(c.Request.URL.Query())
		v.Set("page", strconv.Itoa(result.Page-1))
		prevURL = "/buscar?" + v.Encode()
	}
	if result.Page < totalPages {
		v := cloneValues(c.Request.URL.Query())
		v.Set("page", strconv.Itoa(result.Page+1))
		nextURL = "/buscar?" + v.Encode()
	}
	a.renderPage(c, http.StatusOK, "search", "Buscar canchas", map[string]any{
		"Result": result, "Params": params, "Pages": pages,
		"TotalPages": totalPages, "PrevURL": prevURL, "NextURL": nextURL,
		"Filters": map[string]string{
			"q": c.Query("q"), "sport": c.Query("sport"), "city": c.Query("city"),
			"department": c.Query("department"), "min_price": c.Query("min_price"),
			"max_price": c.Query("max_price"), "date": c.Query("date"), "hour": c.Query("hour"),
			"covered": c.Query("covered"), "sort": c.Query("sort"),
		},
		"Today": time.Now().In(a.cfg.Location).Format("2006-01-02"),
	})
}

func (a *App) CourtDetail(c *gin.Context) {
	id, ok := a.parseID(c, "id")
	if !ok {
		return
	}
	court, err := a.courts.Get(c.Request.Context(), id)
	if err != nil {
		a.renderError(c, errorStatus(err), "Cancha no encontrada", errorMessage(err))
		return
	}
	user := middleware.CurrentUser(c)
	isOwner := user != nil && (court.OwnerID == user.ID || user.Role == "admin")
	if court.Status != "activa" && !isOwner {
		a.renderError(c, http.StatusNotFound, "Cancha no disponible", "Esta cancha está pausada temporalmente.")
		return
	}
	dateStr := c.Query("date")
	if dateStr == "" {
		dateStr = time.Now().In(a.cfg.Location).Format("2006-01-02")
	}
	_, date, slots, availErr := a.courts.Availability(c.Request.Context(), id, dateStr)
	if availErr != nil {
		dateStr = time.Now().In(a.cfg.Location).Format("2006-01-02")
		_, date, slots, _ = a.courts.Availability(c.Request.Context(), id, dateStr)
	}
	reviews, _ := a.reviews.ListByCourt(c.Request.Context(), id)

	canReview := false
	var reviewBookingID uint
	if user != nil {
		bookings, _ := a.bookings.AllByUser(c.Request.Context(), user.ID)
		for _, b := range bookings {
			if b.CourtID != id || b.Status != "completada" {
				continue
			}
			rev, _ := a.reviews.ByBooking(c.Request.Context(), b.ID)
			if rev == nil {
				canReview = true
				reviewBookingID = b.ID
				break
			}
		}
	}

	bbox := ""
	if court.Latitude != 0 || court.Longitude != 0 {
		bbox = fmt.Sprintf("%.5f,%.5f,%.5f,%.5f",
			court.Longitude-0.006, court.Latitude-0.004, court.Longitude+0.006, court.Latitude+0.004)
	}
	a.renderPage(c, http.StatusOK, "court_detail", court.Name, map[string]any{
		"Court": court, "Date": date, "DateISO": date.Format("2006-01-02"), "Slots": slots,
		"Reviews": reviews, "CanReview": canReview, "ReviewBookingID": reviewBookingID,
		"Today":   time.Now().In(a.cfg.Location).Format("2006-01-02"),
		"MaxDate": time.Now().In(a.cfg.Location).AddDate(0, 0, 90).Format("2006-01-02"),
		"BBox":    bbox, "IsOwner": isOwner, "PendingMinutes": a.cfg.PendingMinutes,
	})
}

func (a *App) APICourts(c *gin.Context) {
	params, err := a.parseCourtSearch(c)
	if err != nil {
		a.writeError(c, err)
		return
	}
	result, err := a.courts.Search(c.Request.Context(), params)
	if err != nil {
		a.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (a *App) APICourt(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		a.writeError(c, services.NotFound("Cancha no encontrada."))
		return
	}
	court, err := a.courts.Get(c.Request.Context(), uint(id))
	if err != nil {
		a.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"court": court})
}

func (a *App) APIAvailability(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		a.writeError(c, services.NotFound("Cancha no encontrada."))
		return
	}
	dateStr := c.Query("date")
	if dateStr == "" {
		dateStr = time.Now().In(a.cfg.Location).Format("2006-01-02")
	}
	court, date, slots, err := a.courts.Availability(c.Request.Context(), uint(id), dateStr)
	if err != nil {
		a.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"court_id": court.ID, "date": date.Format("2006-01-02"), "slots": slots,
		"open_time": court.OpenTime, "close_time": court.CloseTime,
	})
}

func (a *App) parseCourtSearch(c *gin.Context) (repositories.CourtSearchParams, error) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	minPrice, _ := strconv.ParseInt(c.Query("min_price"), 10, 64)
	maxPrice, _ := strconv.ParseInt(c.Query("max_price"), 10, 64)
	if minPrice < 0 {
		minPrice = 0
	}
	if maxPrice < 0 {
		maxPrice = 0
	}
	if minPrice > 0 && maxPrice > 0 && minPrice > maxPrice {
		return repositories.CourtSearchParams{}, services.BadRequest("El precio mínimo no puede superar al máximo.")
	}
	params := repositories.CourtSearchParams{
		Query: strings.TrimSpace(c.Query("q")), Sport: strings.TrimSpace(c.Query("sport")),
		City: strings.TrimSpace(c.Query("city")), Department: strings.TrimSpace(c.Query("department")),
		MinPrice: minPrice, MaxPrice: maxPrice, Sort: c.DefaultQuery("sort", "relevance"),
		Page: page, Limit: 9,
	}
	if dateStr := strings.TrimSpace(c.Query("date")); dateStr != "" {
		date, err := a.courts.ParseDate(dateStr)
		if err != nil {
			return params, err
		}
		params.Date = &date
		params.Hour = strings.TrimSpace(c.Query("hour"))
		if params.Hour != "" {
			if _, err := time.Parse("15:04", params.Hour); err != nil {
				return params, services.BadRequest("La hora del filtro no es válida.")
			}
		}
	}
	switch c.Query("covered") {
	case "si":
		v := true
		params.Covered = &v
	case "no":
		v := false
		params.Covered = &v
	}
	return params, nil
}

func buildPagination(values url.Values, current, total, limit int) []PageLink {
	totalPages := (total + limit - 1) / limit
	if totalPages < 1 {
		totalPages = 1
	}
	start := current - 2
	if start < 1 {
		start = 1
	}
	end := start + 4
	if end > totalPages {
		end = totalPages
		start = end - 4
		if start < 1 {
			start = 1
		}
	}
	links := make([]PageLink, 0, end-start+1)
	for p := start; p <= end; p++ {
		v := cloneValues(values)
		v.Set("page", strconv.Itoa(p))
		links = append(links, PageLink{Number: p, URL: "/buscar?" + v.Encode(), Active: p == current})
	}
	return links
}

func cloneValues(v url.Values) url.Values {
	out := make(url.Values, len(v))
	for key, list := range v {
		out[key] = append([]string(nil), list...)
	}
	return out
}
