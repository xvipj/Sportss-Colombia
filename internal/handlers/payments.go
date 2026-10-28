package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"sportss/internal/middleware"
	"sportss/internal/models"
	"sportss/internal/services"
)

func (a *App) APICheckout(c *gin.Context) {
	user := middleware.CurrentUser(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		a.writeError(c, services.NotFound("Reserva no encontrada."))
		return
	}
	checkoutURL, err := a.payments.StartCheckout(c.Request.Context(), uint(id), user.ID)
	if err != nil {
		a.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"checkout_url": checkoutURL})
}

func (a *App) PaymentWebhook(c *gin.Context) {
	raw, _ := c.GetRawData()
	var body map[string]any
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.UseNumber()
	_ = dec.Decode(&body)

	eventType := c.Query("type")
	if eventType == "" {
		eventType, _ = body["type"].(string)
	}
	dataID := c.Query("data.id")
	if dataID == "" {
		if data, ok := body["data"].(map[string]any); ok {
			switch v := data["id"].(type) {
			case string:
				dataID = v
			case json.Number:
				dataID = v.String()
			case float64:
				dataID = strconv.FormatFloat(v, 'f', 0, 64)
			}
		}
	}
	if eventType != "payment" || dataID == "" {
		c.JSON(http.StatusOK, gin.H{"ok": true, "ignored": true})
		return
	}
	valid := a.paymentsValidateWebhook(dataID, c.GetHeader("X-Request-Id"), c.GetHeader("X-Signature"))
	if !valid {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Firma de webhook inválida."})
		return
	}
	if err := a.payments.HandleWebhook(c.Request.Context(), dataID); err != nil {
		a.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *App) paymentsValidateWebhook(dataID, requestID, signature string) bool {
	// El cliente se configura en el constructor y se expone mediante el método
	// ValidateWebhook de PaymentService.
	return a.payments.ValidateWebhookSignature(dataID, requestID, signature)
}

func (a *App) PaymentSuccess(c *gin.Context) { a.paymentResult(c, "exito") }
func (a *App) PaymentPending(c *gin.Context) { a.paymentResult(c, "pendiente") }
func (a *App) PaymentFailure(c *gin.Context) { a.paymentResult(c, "fallo") }

func (a *App) paymentResult(c *gin.Context, kind string) {
	user := middleware.CurrentUser(c)
	id, _ := strconv.ParseUint(c.Query("booking_id"), 10, 64)
	var booking *models.Booking
	if id > 0 {
		b, err := a.bookings.Get(c.Request.Context(), uint(id))
		if err == nil && (b.UserID == user.ID || b.Court.OwnerID == user.ID || user.Role == models.RoleAdmin) {
			booking = b
		}
	}
	var invoice *models.Invoice
	if booking != nil {
		invoice, _ = a.invoices.GetForBooking(c.Request.Context(), booking.ID)
	}
	titles := map[string]string{"exito": "Pago exitoso", "pendiente": "Pago pendiente", "fallo": "Pago no completado"}
	a.renderPage(c, http.StatusOK, "payment_result", titles[kind], map[string]any{
		"Kind": kind, "Booking": booking, "Invoice": invoice, "TitleResult": titles[kind],
	})
}

func (a *App) MockCheckout(c *gin.Context) {
	user := middleware.CurrentUser(c)
	if !a.cfg.PaymentMockEnabled() {
		a.renderError(c, http.StatusNotFound, "No disponible", "El checkout simulado está deshabilitado.")
		return
	}
	id, ok := a.parseID(c, "id")
	if !ok {
		return
	}
	booking, err := a.bookings.GetAccessible(c.Request.Context(), id, user.ID, user.Role == models.RoleAdmin)
	if err != nil {
		a.renderError(c, errorStatus(err), "Checkout", errorMessage(err))
		return
	}
	if booking.UserID != user.ID {
		a.renderError(c, http.StatusForbidden, "Checkout", "Esta reserva no te pertenece.")
		return
	}
	a.renderPage(c, http.StatusOK, "mock_checkout", "Checkout de prueba", map[string]any{
		"Booking": booking, "Mock": true,
	})
}

func (a *App) MockConfirm(c *gin.Context) {
	user := middleware.CurrentUser(c)
	id, ok := a.parseID(c, "id")
	if !ok {
		return
	}
	if err := a.payments.ConfirmMock(c.Request.Context(), id, user.ID); err != nil {
		a.setFlash(c, "error", errorMessage(err))
		c.Redirect(http.StatusFound, "/checkout/mock/"+strconv.FormatUint(uint64(id), 10))
		return
	}
	a.setFlash(c, "success", "Pago simulado aprobado. Tu reserva quedó confirmada.")
	c.Redirect(http.StatusFound, "/pago/exito?booking_id="+strconv.FormatUint(uint64(id), 10))
}

func (a *App) DownloadInvoice(c *gin.Context) {
	user := middleware.CurrentUser(c)
	id, ok := a.parseID(c, "id")
	if !ok {
		return
	}
	inv, err := a.invoices.GetForBooking(c.Request.Context(), id)
	if err != nil {
		a.renderError(c, errorStatus(err), "Factura", errorMessage(err))
		return
	}
	booking, err := a.bookings.Get(c.Request.Context(), inv.BookingID)
	if err != nil {
		a.renderError(c, http.StatusNotFound, "Factura", "No encontramos la reserva asociada.")
		return
	}
	if user.ID != inv.UserID && user.ID != booking.Court.OwnerID && user.Role != models.RoleAdmin {
		a.renderError(c, http.StatusForbidden, "Factura", "No tienes permiso para descargar esta factura.")
		return
	}
	path := filepath.Join(a.cfg.InvoiceDir, filepath.Base(inv.Path))
	if _, statErr := os.Stat(path); statErr != nil {
		if !errors.Is(statErr, os.ErrNotExist) {
			a.renderError(c, http.StatusInternalServerError, "Factura", "No fue posible abrir la factura.")
			return
		}
		// El registro existe pero el PDF fue movido o eliminado: se regenera.
		regenerated, genErr := a.invoices.GenerateForBooking(c.Request.Context(), inv.BookingID, "")
		if genErr != nil {
			a.renderError(c, errorStatus(genErr), "Factura", errorMessage(genErr))
			return
		}
		inv = regenerated
		path = filepath.Join(a.cfg.InvoiceDir, filepath.Base(inv.Path))
		if _, err := os.Stat(path); err != nil {
			a.renderError(c, http.StatusNotFound, "Factura", "El archivo de la factura no está disponible.")
			return
		}
	}
	c.FileAttachment(path, "Factura-"+inv.Number+".pdf")
}
