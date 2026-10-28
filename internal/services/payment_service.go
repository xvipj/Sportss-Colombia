package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"sportss/internal/cache"
	"sportss/internal/config"
	"sportss/internal/models"
	"sportss/internal/payments"
	"sportss/internal/repositories"
)

type PaymentService struct {
	cfg      *config.Config
	payments repositories.PaymentRepository
	bookings repositories.BookingRepository
	invoices *InvoiceService
	notifier *NotificationService
	mp       *payments.Client
	cache    cache.Store
}

func NewPaymentService(
	cfg *config.Config,
	paymentsRepo repositories.PaymentRepository,
	bookingsRepo repositories.BookingRepository,
	invoices *InvoiceService,
	notifier *NotificationService,
	mp *payments.Client,
	cacheStore cache.Store,
) *PaymentService {
	return &PaymentService{
		cfg: cfg, payments: paymentsRepo, bookings: bookingsRepo, invoices: invoices,
		notifier: notifier, mp: mp, cache: cacheStore,
	}
}

func (s *PaymentService) StartCheckout(ctx context.Context, bookingID, userID uint) (string, error) {
	booking, err := s.bookings.FindByID(ctx, bookingID)
	if err != nil {
		return "", NotFound("Reserva no encontrada.")
	}
	if booking.UserID != userID {
		return "", Forbidden("Esta reserva no te pertenece.")
	}
	if booking.Status == models.BookingConfirmed || booking.Status == models.BookingCompleted {
		return "", Conflict("Esta reserva ya está pagada.")
	}
	if booking.Status != models.BookingPending {
		return "", BadRequest("Esta reserva ya no admite pagos.")
	}
	if time.Since(booking.CreatedAt) > time.Duration(s.cfg.PendingMinutes)*time.Minute {
		return "", BadRequest("El tiempo para pagar expiró. Crea una nueva reserva.")
	}

	existing, findErr := s.payments.FindByBooking(ctx, bookingID)
	if findErr != nil && !errors.Is(findErr, repositories.ErrNotFound) {
		return "", Internal("No fue posible consultar el pago", findErr)
	}
	if existing != nil && existing.Status == models.PaymentApproved {
		return "", Conflict("Esta reserva ya está pagada.")
	}
	if existing != nil && existing.CheckoutURL != "" && time.Since(existing.CreatedAt) < time.Duration(s.cfg.PendingMinutes)*time.Minute {
		return existing.CheckoutURL, nil
	}

	if s.cfg.PaymentMockEnabled() {
		url := s.cfg.BaseURL + "/checkout/mock/" + strconv.FormatUint(uint64(booking.ID), 10)
		payment := &models.Payment{
			BookingID: booking.ID, UserID: userID, Provider: "mock", Status: models.PaymentPending,
			Amount: booking.TotalAmount, Currency: "COP", CheckoutURL: url,
		}
		if existing != nil {
			payment.ID = existing.ID
			if err := s.payments.Update(ctx, payment); err != nil {
				return "", Internal("No fue posible preparar el pago", err)
			}
		} else if err := s.payments.Create(ctx, payment); err != nil {
			return "", Internal("No fue posible preparar el pago", err)
		}
		return url, nil
	}

	req := payments.PreferenceRequest{
		Items: []payments.PreferenceItem{{
			Title:      fmt.Sprintf("%s - %s %s", booking.Court.Name, booking.Date.Format("02/01/2006"), booking.StartTime),
			Quantity:   1,
			CurrencyID: "COP",
			UnitPrice:  booking.TotalAmount,
		}},
		ExternalReference: strconv.FormatUint(uint64(booking.ID), 10),
		NotificationURL:   s.cfg.BaseURL + "/api/payments/webhook",
		BackURLs: payments.BackURLs{
			Success: s.cfg.BaseURL + "/pago/exito?booking_id=" + strconv.FormatUint(uint64(booking.ID), 10),
			Failure: s.cfg.BaseURL + "/pago/fallo?booking_id=" + strconv.FormatUint(uint64(booking.ID), 10),
			Pending: s.cfg.BaseURL + "/pago/pendiente?booking_id=" + strconv.FormatUint(uint64(booking.ID), 10),
		},
		StatementDescriptor: "SPORTSS COL",
		Metadata:            map[string]any{"booking_id": booking.ID},
	}
	if strings.HasPrefix(strings.ToLower(s.cfg.BaseURL), "https://") {
		req.AutoReturn = "approved"
	}
	pref, err := s.mp.CreatePreference(ctx, req)
	if err != nil {
		return "", Internal("Mercado Pago no pudo crear el checkout", err)
	}
	checkoutURL := pref.InitPoint
	if s.cfg.MPSandbox && pref.SandboxInitPoint != "" {
		checkoutURL = pref.SandboxInitPoint
	}
	raw, _ := json.Marshal(pref)
	payment := &models.Payment{
		BookingID: booking.ID, UserID: userID, Provider: "mercadopago", Status: models.PaymentPending,
		Amount: booking.TotalAmount, Currency: "COP", PreferenceID: pref.ID, CheckoutURL: checkoutURL,
		RawPayload: string(raw),
	}
	if existing != nil {
		payment.ID = existing.ID
		if err := s.payments.Update(ctx, payment); err != nil {
			return "", Internal("No fue posible guardar el pago", err)
		}
	} else if err := s.payments.Create(ctx, payment); err != nil {
		return "", Internal("No fue posible guardar el pago", err)
	}
	return checkoutURL, nil
}

// ConfirmMock permite probar todo el flujo sin credenciales de Mercado Pago.
func (s *PaymentService) ConfirmMock(ctx context.Context, bookingID, userID uint) error {
	if !s.cfg.PaymentMockEnabled() {
		return BadRequest("El pago simulado está deshabilitado.")
	}
	booking, err := s.bookings.FindByID(ctx, bookingID)
	if err != nil {
		return NotFound("Reserva no encontrada.")
	}
	if booking.UserID != userID {
		return Forbidden("Esta reserva no te pertenece.")
	}
	return s.confirmApproved(ctx, booking, "mock", "mock-"+strconv.FormatUint(uint64(booking.ID), 10), "mock", `{"provider":"mock"}`)
}

// HandleWebhook consulta el pago en Mercado Pago y confirma la reserva.
func (s *PaymentService) HandleWebhook(ctx context.Context, paymentID string) error {
	if s.cfg.PaymentMockEnabled() {
		return BadRequest("Webhook no disponible sin credenciales de Mercado Pago.")
	}
	info, err := s.mp.GetPayment(ctx, paymentID)
	if err != nil {
		return Internal("No fue posible consultar el pago en Mercado Pago", err)
	}
	if info.Status != "approved" {
		// Notificaciones de pagos rechazados/pendientes no confirman la reserva.
		return nil
	}
	raw, _ := json.Marshal(info)
	return s.ConfirmApprovedInfo(ctx, info, string(raw))
}

func (s *PaymentService) ConfirmApprovedInfo(ctx context.Context, info *payments.PaymentInfo, raw string) error {
	bookingID64, err := strconv.ParseUint(strings.TrimSpace(info.ExternalReference), 10, 64)
	if err != nil {
		return BadRequest("Referencia externa de pago inválida.")
	}
	bookingID := uint(bookingID64)
	booking, err := s.bookings.FindByID(ctx, bookingID)
	if err != nil {
		return NotFound("Reserva asociada al pago no encontrada.")
	}
	if info.TransactionAmount > 0 {
		amount := int64(math.Round(info.TransactionAmount))
		if amount != booking.TotalAmount {
			return Conflict("El valor pagado no coincide con la reserva.")
		}
	}
	method := info.PaymentMethodID
	externalID := strconv.FormatInt(info.ID, 10)
	return s.confirmApproved(ctx, booking, "mercadopago", externalID, method, raw)
}

func (s *PaymentService) confirmApproved(ctx context.Context, booking *models.Booking, provider, externalID, method, raw string) error {
	if booking.Status == models.BookingCancelled || booking.Status == models.BookingExpired {
		return Conflict("La reserva ya fue cancelada o expiró. El pago requiere revisión manual.")
	}
	alreadyPaid := booking.Status == models.BookingConfirmed || booking.Status == models.BookingCompleted

	payment, err := s.payments.FindByBooking(ctx, booking.ID)
	if err != nil && !errors.Is(err, repositories.ErrNotFound) {
		return Internal("No fue posible consultar el pago", err)
	}
	if payment == nil {
		payment = &models.Payment{BookingID: booking.ID, UserID: booking.UserID, Amount: booking.TotalAmount, Currency: "COP"}
	}
	payment.Provider = provider
	payment.ExternalID = externalID
	payment.Status = models.PaymentApproved
	payment.Method = method
	payment.RawPayload = raw
	if payment.ID == 0 {
		if err := s.payments.Create(ctx, payment); err != nil {
			return Internal("No fue posible registrar el pago", err)
		}
	} else if err := s.payments.Update(ctx, payment); err != nil {
		return Internal("No fue posible actualizar el pago", err)
	}

	if !alreadyPaid {
		now := time.Now().In(s.cfg.Location)
		if err := s.bookings.Update(ctx, booking.ID, map[string]any{
			"status": models.BookingConfirmed, "completed_at": nil, "cancelled_at": nil,
		}); err != nil {
			return Internal("No fue posible confirmar la reserva", err)
		}
		booking.Status = models.BookingConfirmed
		_ = now
		_ = s.cache.DeletePrefix(ctx, "courts:search:")
	}

	// La factura se genera también en reintentos si antes falló.
	if _, err := s.invoices.GenerateForBooking(ctx, booking.ID, method); err != nil {
		return err
	}
	if alreadyPaid {
		return nil
	}

	s.notifier.Notify(ctx, booking.UserID, models.NotifyPaymentOK, "Pago confirmado",
		fmt.Sprintf("Tu reserva en %s para el %s de %s a %s quedó confirmada. Descarga tu factura en Mis reservas.",
			booking.Court.Name, booking.Date.Format("02/01/2006"), booking.StartTime, booking.EndTime), "/mis-reservas")
	s.notifier.Notify(ctx, booking.Court.OwnerID, models.NotifyNewBooking, "Reserva pagada",
		fmt.Sprintf("Se confirmó una reserva en %s para el %s de %s a %s.", booking.Court.Name,
			booking.Date.Format("02/01/2006"), booking.StartTime, booking.EndTime), "/panel/reservas")
	return nil
}

func (s *PaymentService) PaymentForBooking(ctx context.Context, bookingID uint) (*models.Payment, error) {
	p, err := s.payments.FindByBooking(ctx, bookingID)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, NotFound("Pago no encontrado.")
		}
		return nil, Internal("No fue posible cargar el pago", err)
	}
	return p, nil
}

func (s *PaymentService) ListByUser(ctx context.Context, userID uint) ([]models.Payment, error) {
	items, err := s.payments.ListByUser(ctx, userID, 30)
	if err != nil {
		return nil, Internal("No fue posible cargar tus pagos", err)
	}
	return items, nil
}

func (s *PaymentService) ValidateWebhookSignature(dataID, requestID, signature string) bool {
	return s.mp.ValidateWebhook(dataID, requestID, signature)
}
