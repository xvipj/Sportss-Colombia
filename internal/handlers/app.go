package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"sportss/internal/config"
	"sportss/internal/middleware"
	"sportss/internal/services"
)

type App struct {
	cfg           *config.Config
	renderer      *Renderer
	auth          *services.AuthService
	courts        *services.CourtService
	bookings      *services.BookingService
	payments      *services.PaymentService
	invoices      *services.InvoiceService
	reviews       *services.ReviewService
	notifications *services.NotificationService
	analytics     *services.AnalyticsService
}

func NewApp(
	cfg *config.Config,
	auth *services.AuthService,
	courts *services.CourtService,
	bookings *services.BookingService,
	payments *services.PaymentService,
	invoices *services.InvoiceService,
	reviews *services.ReviewService,
	notifications *services.NotificationService,
	analytics *services.AnalyticsService,
) *App {
	return &App{
		cfg: cfg, renderer: NewRenderer(cfg), auth: auth, courts: courts, bookings: bookings,
		payments: payments, invoices: invoices, reviews: reviews, notifications: notifications,
		analytics: analytics,
	}
}

const flashCookie = "sportss_flash"

func (a *App) renderPage(c *gin.Context, status int, page, title string, data map[string]any) {
	user := middleware.CurrentUser(c)
	unread := int64(0)
	if user != nil {
		unread, _ = a.notifications.Unread(c.Request.Context(), user.ID)
	}
	flash, flashType := readFlash(c)
	viewData := map[string]any{
		"Title":       title,
		"AppName":     a.cfg.AppName,
		"Year":        time.Now().In(a.cfg.Location).Year(),
		"User":        user,
		"CSRF":        c.GetString(middleware.CtxCSRF),
		"Unread":      unread,
		"Flash":       flash,
		"FlashType":   flashType,
		"CurrentPath": c.Request.URL.Path,
		"Sports":      services.Sports,
		"Surfaces":    services.SurfacesBySport,
		"BodyClass":   "",
	}
	for k, v := range data {
		viewData[k] = v
	}
	if viewData["Title"] == "" {
		viewData["Title"] = a.cfg.AppName
	}
	a.renderer.Render(c, status, page, viewData)
}

func (a *App) renderError(c *gin.Context, status int, title, message string) {
	a.renderPage(c, status, "error", title, map[string]any{
		"Status": status, "Message": message,
	})
}

func (a *App) NotFound(c *gin.Context) {
	if middleware.IsAPI(c) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Recurso no encontrado."})
		return
	}
	a.renderError(c, http.StatusNotFound, "Página no encontrada", "La página que buscas no existe o fue movida.")
}

func (a *App) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok", "app": a.cfg.AppName})
}

func (a *App) setFlash(c *gin.Context, kind, message string) {
	value := url.QueryEscape(kind + "|" + message)
	http.SetCookie(c.Writer, &http.Cookie{
		Name: flashCookie, Value: value, Path: "/", MaxAge: 20, HttpOnly: true,
		Secure: a.cfg.CookieSecure, SameSite: http.SameSiteLaxMode,
	})
}

func readFlash(c *gin.Context) (string, string) {
	raw, err := c.Cookie(flashCookie)
	if err != nil || raw == "" {
		return "", ""
	}
	decoded, err := url.QueryUnescape(raw)
	if err != nil {
		return "", ""
	}
	parts := strings.SplitN(decoded, "|", 2)
	http.SetCookie(c.Writer, &http.Cookie{Name: flashCookie, Value: "", Path: "/", MaxAge: -1})
	if len(parts) != 2 {
		return parts[0], "success"
	}
	return parts[1], parts[0]
}

func (a *App) setSession(c *gin.Context, token string) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name: a.cfg.CookieName, Value: token, Path: "/", MaxAge: 7 * 24 * 60 * 60,
		HttpOnly: true, Secure: a.cfg.CookieSecure, SameSite: http.SameSiteLaxMode,
	})
}

func (a *App) clearSession(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name: a.cfg.CookieName, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: a.cfg.CookieSecure, SameSite: http.SameSiteLaxMode,
	})
}

func (a *App) parseID(c *gin.Context, param string) (uint, bool) {
	n, err := strconv.ParseUint(c.Param(param), 10, 64)
	if err != nil || n == 0 {
		a.renderError(c, http.StatusNotFound, "No encontrado", "El recurso que buscas no existe.")
		return 0, false
	}
	return uint(n), true
}

func (a *App) writeError(c *gin.Context, err error) {
	var appErr *services.AppError
	if errors.As(err, &appErr) {
		if appErr.Status >= 500 {
			slog.Error("error de servicio", "error", appErr.Message)
			c.JSON(appErr.Status, gin.H{"error": "Ocurrió un error inesperado. Intenta de nuevo."})
			return
		}
		c.JSON(appErr.Status, gin.H{"error": appErr.Message})
		return
	}
	slog.Error("error no controlado", "error", err)
	c.JSON(http.StatusInternalServerError, gin.H{"error": "Ocurrió un error inesperado."})
}

func errorMessage(err error) string {
	var appErr *services.AppError
	if errors.As(err, &appErr) {
		return appErr.Message
	}
	return "Ocurrió un error inesperado. Intenta de nuevo."
}

func errorStatus(err error) int {
	var appErr *services.AppError
	if errors.As(err, &appErr) {
		return appErr.Status
	}
	return http.StatusInternalServerError
}

func safeNext(next string) string {
	if strings.HasPrefix(next, "/") && !strings.HasPrefix(next, "//") {
		return next
	}
	return "/"
}

func (a *App) saveUploadedImage(fh *multipart.FileHeader, prefix string) (string, error) {
	maxBytes := a.cfg.UploadMaxMB * 1024 * 1024
	if fh.Size <= 0 || fh.Size > maxBytes {
		return "", fmt.Errorf("cada imagen debe pesar máximo %d MB", a.cfg.UploadMaxMB)
	}
	src, err := fh.Open()
	if err != nil {
		return "", err
	}
	defer src.Close()

	head := make([]byte, 512)
	n, err := io.ReadFull(src, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
		return "", err
	}
	mimeType := http.DetectContentType(head[:n])
	ext := ""
	switch mimeType {
	case "image/jpeg":
		ext = ".jpg"
	case "image/png":
		ext = ".png"
	case "image/webp":
		ext = ".webp"
	default:
		return "", fmt.Errorf("formato no permitido. Usa JPG, PNG o WebP")
	}
	if _, err := src.Seek(0, io.SeekStart); err != nil {
		return "", err
	}

	random := make([]byte, 12)
	_, _ = rand.Read(random)
	name := fmt.Sprintf("%s-%d-%s%s", prefix, time.Now().Unix(), hex.EncodeToString(random), ext)
	dest := filepath.Join(a.cfg.UploadDir, name)
	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(out, io.LimitReader(src, maxBytes+1)); err != nil {
		out.Close()
		_ = os.Remove(dest)
		return "", err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(dest)
		return "", err
	}
	return "/static/uploads/" + name, nil
}

func (a *App) deleteUploadPath(publicPath string) {
	if !strings.HasPrefix(publicPath, "/static/uploads/") {
		return
	}
	name := filepath.Base(publicPath)
	if name == "." || name == "" {
		return
	}
	_ = os.Remove(filepath.Join(a.cfg.UploadDir, name))
}
