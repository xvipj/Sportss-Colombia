package handlers

import (
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"sportss/internal/middleware"
	"sportss/internal/models"
	"sportss/internal/services"
)

type MyBookingRow struct {
	Booking   models.Booking
	Invoice   *models.Invoice
	Review    *models.Review
	CanReview bool
	CanPay    bool
	CanCancel bool
}

func (a *App) MyBookings(c *gin.Context) {
	user := middleware.CurrentUser(c)
	status := c.Query("status")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	items, total, err := a.bookings.ListByUser(c.Request.Context(), user.ID, status, page, 8)
	if err != nil {
		a.renderError(c, errorStatus(err), "Mis reservas", errorMessage(err))
		return
	}
	invoices, _ := a.invoices.ListByUser(c.Request.Context(), user.ID)
	invoiceMap := make(map[uint]*models.Invoice)
	for i := range invoices {
		invoiceMap[invoices[i].BookingID] = &invoices[i]
	}
	rows := make([]MyBookingRow, 0, len(items))
	now := time.Now().In(a.cfg.Location)
	for _, b := range items {
		row := MyBookingRow{Booking: b, Invoice: invoiceMap[b.ID]}
		if b.Status == models.BookingCompleted {
			if rev, _ := a.reviews.ByBooking(c.Request.Context(), b.ID); rev != nil {
				row.Review = rev
			} else {
				row.CanReview = true
			}
		}
		if b.Status == models.BookingPending {
			row.CanPay = now.Sub(b.CreatedAt) < time.Duration(a.cfg.PendingMinutes)*time.Minute
			row.CanCancel = true
		}
		if b.Status == models.BookingConfirmed {
			start := time.Date(b.Date.Year(), b.Date.Month(), b.Date.Day(), parseHour(b.StartTime), 0, 0, 0, a.cfg.Location)
			row.CanCancel = a.cfg.CancellationHours == 0 || now.Add(time.Duration(a.cfg.CancellationHours)*time.Hour).Before(start)
		}
		rows = append(rows, row)
	}
	values := url.Values{}
	if status != "" {
		values.Set("status", status)
	}
	a.renderPage(c, http.StatusOK, "my_bookings", "Mis reservas", map[string]any{
		"Rows": rows, "Status": status, "Total": total, "Page": page,
		"Pages": pageLinks("/mis-reservas", values, page, int(total), 8),
		"Statuses": []string{
			models.BookingPending, models.BookingConfirmed, models.BookingCompleted,
			models.BookingCancelled, models.BookingExpired,
		},
	})
}

func (a *App) OwnerBookings(c *gin.Context) {
	user := middleware.CurrentUser(c)
	status := c.Query("status")
	date := c.Query("date")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	var (
		items []models.Booking
		total int64
		err   error
	)
	if user.Role == models.RoleAdmin {
		items, total, err = a.bookings.ListAll(c.Request.Context(), status, date, page, 20)
	} else {
		items, total, err = a.bookings.ListByOwner(c.Request.Context(), user.ID, status, date, page, 20)
	}
	if err != nil {
		a.renderError(c, errorStatus(err), "Reservas", errorMessage(err))
		return
	}
	values := url.Values{}
	if status != "" {
		values.Set("status", status)
	}
	if date != "" {
		values.Set("date", date)
	}
	a.renderPage(c, http.StatusOK, "owner_bookings", "Gestión de reservas", map[string]any{
		"Bookings": items, "Total": total, "Page": page, "Status": status, "Date": date,
		"Pages": pageLinks("/panel/reservas", values, page, int(total), 20),
		"Statuses": []string{
			models.BookingPending, models.BookingConfirmed, models.BookingCompleted,
			models.BookingCancelled, models.BookingExpired,
		},
	})
}

func (a *App) APICreateBooking(c *gin.Context) {
	user := middleware.CurrentUser(c)
	var req struct {
		CourtID   uint   `json:"court_id"`
		Date      string `json:"date"`
		StartTime string `json:"start_time"`
		Hours     int    `json:"hours"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		a.writeError(c, services.BadRequest("Datos de reserva inválidos."))
		return
	}
	if req.Hours == 0 {
		req.Hours = 1
	}
	booking, err := a.bookings.Create(c.Request.Context(), user, services.CreateBookingInput{
		CourtID: req.CourtID, Date: req.Date, StartTime: req.StartTime, Hours: req.Hours,
	})
	if err != nil {
		a.writeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"message":           "Reserva creada. Completa el pago para confirmarla.",
		"booking":           booking,
		"checkout_endpoint": "/api/payments/checkout/" + strconv.FormatUint(uint64(booking.ID), 10),
	})
}

func (a *App) APIGetBooking(c *gin.Context) {
	user := middleware.CurrentUser(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		a.writeError(c, services.NotFound("Reserva no encontrada."))
		return
	}
	booking, err := a.bookings.GetAccessible(c.Request.Context(), uint(id), user.ID, user.Role == models.RoleAdmin)
	if err != nil {
		a.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"booking": booking})
}

func (a *App) APICancelBooking(c *gin.Context) {
	user := middleware.CurrentUser(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		a.writeError(c, services.NotFound("Reserva no encontrada."))
		return
	}
	if err := a.bookings.CancelByUser(c.Request.Context(), uint(id), user.ID); err != nil {
		a.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Reserva cancelada."})
}

func (a *App) APICompleteBooking(c *gin.Context) {
	user := middleware.CurrentUser(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		a.writeError(c, services.NotFound("Reserva no encontrada."))
		return
	}
	if err := a.bookings.CompleteByOwner(c.Request.Context(), uint(id), user.ID, user.Role == models.RoleAdmin); err != nil {
		a.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Reserva marcada como completada."})
}

func (a *App) OwnerCancelBooking(c *gin.Context) {
	user := middleware.CurrentUser(c)
	id, ok := a.parseID(c, "id")
	if !ok {
		return
	}
	if err := a.bookings.CancelByOwner(c.Request.Context(), id, user.ID, user.Role == models.RoleAdmin); err != nil {
		a.setFlash(c, "error", errorMessage(err))
	} else {
		a.setFlash(c, "success", "Reserva cancelada y horario liberado.")
	}
	c.Redirect(http.StatusFound, "/panel/reservas")
}

func (a *App) OwnerCompleteBooking(c *gin.Context) {
	user := middleware.CurrentUser(c)
	id, ok := a.parseID(c, "id")
	if !ok {
		return
	}
	if err := a.bookings.CompleteByOwner(c.Request.Context(), id, user.ID, user.Role == models.RoleAdmin); err != nil {
		a.setFlash(c, "error", errorMessage(err))
	} else {
		a.setFlash(c, "success", "Reserva marcada como completada.")
	}
	c.Redirect(http.StatusFound, "/panel/reservas")
}

func parseHour(hhmm string) int {
	t, err := time.Parse("15:04", hhmm)
	if err != nil {
		return 0
	}
	return t.Hour()
}

func pageLinks(base string, values url.Values, current, total, limit int) []PageLink {
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
		v := url.Values{}
		for key, list := range values {
			v[key] = append([]string(nil), list...)
		}
		v.Set("page", strconv.Itoa(p))
		links = append(links, PageLink{Number: p, URL: base + "?" + v.Encode(), Active: p == current})
	}
	return links
}
