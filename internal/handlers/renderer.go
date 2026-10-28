package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"sportss/internal/config"
	"sportss/internal/models"
	"sportss/internal/services"
)

// Renderer compone el layout y la página solicitada. Las plantillas se parsean
// por vista para evitar colisiones entre bloques "content".
type Renderer struct {
	cfg *config.Config
}

func NewRenderer(cfg *config.Config) *Renderer {
	return &Renderer{cfg: cfg}
}

func (r *Renderer) Render(c *gin.Context, status int, page string, data map[string]any) {
	funcs := template.FuncMap{
		"money":         services.FormatCOP,
		"daysLabel":     services.DaysLabel,
		"dayName":       services.DayName,
		"statusLabel":   bookingStatusLabel,
		"statusClass":   bookingStatusClass,
		"payLabel":      paymentStatusLabel,
		"courtStatus":   courtStatusLabel,
		"roleLabel":     roleLabel,
		"date":          func(t time.Time) string { return t.Format("02/01/2006") },
		"datetime":      func(t time.Time) string { return t.Format("02/01/2006 15:04") },
		"dateISO":       func(t time.Time) string { return t.Format("2006-01-02") },
		"json":          jsonString,
		"seq":           seq,
		"add":           func(a, b int) int { return a + b },
		"sub":           func(a, b int) int { return a - b },
		"mul":           func(a, b int64) int64 { return a * b },
		"pct":           func(v float64) string { return fmt.Sprintf("%.1f%%", v) },
		"stars":         func(v float64) string { return fmt.Sprintf("%.1f", v) },
		"initials":      initials,
		"truncate":      truncate,
		"hasDay":        hasDay,
		"lower":         strings.ToLower,
		"upper":         strings.ToUpper,
		"year":          func(t time.Time) int { return t.Year() },
		"ratingRounded": func(v float64) int { return int(v + 0.5) },
		"roleClass":     roleClass,
		"bbox": func(lat, lng float64) string {
			return fmt.Sprintf("%.5f,%.5f,%.5f,%.5f", lng-0.006, lat-0.004, lng+0.006, lat+0.004)
		},
	}

	files := []string{
		filepath.Join(r.cfg.TemplateDir, "layouts", "base.html"),
		filepath.Join(r.cfg.TemplateDir, "partials", "nav.html"),
		filepath.Join(r.cfg.TemplateDir, "partials", "footer.html"),
		filepath.Join(r.cfg.TemplateDir, "partials", "court_card.html"),
		filepath.Join(r.cfg.TemplateDir, "partials", "panel_nav.html"),
		filepath.Join(r.cfg.TemplateDir, "partials", "admin_nav.html"),
		filepath.Join(r.cfg.TemplateDir, "pages", page+".html"),
	}
	tmpl, err := template.New("base").Funcs(funcs).ParseFiles(files...)
	if err != nil {
		slog.Error("error de plantilla", "page", page, "error", err)
		c.String(http.StatusInternalServerError, "Error interno de plantilla")
		return
	}

	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "base", data); err != nil {
		slog.Error("error ejecutando plantilla", "page", page, "error", err)
		c.String(http.StatusInternalServerError, "Error interno al renderizar la vista")
		return
	}
	c.Status(status)
	c.Header("Content-Type", "text/html; charset=utf-8")
	_, _ = c.Writer.Write(buf.Bytes())
}

func jsonString(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "null"
	}
	return string(b)
}

func seq(n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = i + 1
	}
	return out
}

func initials(name string) string {
	parts := strings.Fields(name)
	if len(parts) == 0 {
		return "S"
	}
	if len(parts) == 1 {
		return strings.ToUpper(parts[0][:1])
	}
	return strings.ToUpper(parts[0][:1] + parts[1][:1])
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func hasDay(days string, day int) bool {
	want := strconv.Itoa(day)
	for _, d := range strings.Split(days, ",") {
		if strings.TrimSpace(d) == want {
			return true
		}
	}
	return false
}

func bookingStatusLabel(status string) string {
	switch status {
	case models.BookingPending:
		return "Pendiente de pago"
	case models.BookingConfirmed:
		return "Confirmada"
	case models.BookingCancelled:
		return "Cancelada"
	case models.BookingExpired:
		return "Expirada"
	case models.BookingCompleted:
		return "Completada"
	default:
		return status
	}
}

func bookingStatusClass(status string) string {
	switch status {
	case models.BookingConfirmed, models.BookingCompleted:
		return "ok"
	case models.BookingPending:
		return "warn"
	case models.BookingCancelled, models.BookingExpired:
		return "danger"
	default:
		return "neutral"
	}
}

func paymentStatusLabel(status string) string {
	switch status {
	case models.PaymentApproved:
		return "Aprobado"
	case models.PaymentPending:
		return "Pendiente"
	case models.PaymentFailed:
		return "Fallido"
	case models.PaymentRefunded:
		return "Reembolsado"
	default:
		return status
	}
}

func courtStatusLabel(status string) string {
	if status == models.CourtPaused {
		return "Pausada"
	}
	return "Activa"
}

func roleLabel(role string) string {
	switch role {
	case models.RoleAdmin:
		return "Super admin"
	case models.RoleBusiness:
		return "Empresa"
	default:
		return "Jugador"
	}
}

func roleClass(role string) string {
	switch role {
	case models.RoleAdmin:
		return "danger"
	case models.RoleBusiness:
		return "warn"
	default:
		return "ok"
	}
}
