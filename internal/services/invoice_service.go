package services

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"sportss/internal/config"
	"sportss/internal/models"
	"sportss/internal/pdf"
	"sportss/internal/repositories"
)

type InvoiceService struct {
	cfg      *config.Config
	invoices repositories.InvoiceRepository
	bookings repositories.BookingRepository
}

func NewInvoiceService(cfg *config.Config, invoices repositories.InvoiceRepository, bookings repositories.BookingRepository) *InvoiceService {
	return &InvoiceService{cfg: cfg, invoices: invoices, bookings: bookings}
}

// GenerateForBooking es idempotente: si la factura ya existe y el archivo está
// presente, simplemente la devuelve.
func (s *InvoiceService) GenerateForBooking(ctx context.Context, bookingID uint, method string) (*models.Invoice, error) {
	booking, err := s.bookings.FindByID(ctx, bookingID)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, NotFound("Reserva no encontrada.")
		}
		return nil, Internal("No fue posible cargar la reserva", err)
	}

	inv, err := s.invoices.FindByBooking(ctx, bookingID)
	if err != nil && !errors.Is(err, repositories.ErrNotFound) {
		return nil, Internal("No fue posible consultar la factura", err)
	}
	if inv != nil && inv.Path != "" {
		if _, statErr := os.Stat(inv.Path); statErr == nil {
			return inv, nil
		}
	}

	if inv == nil {
		now := time.Now().In(s.cfg.Location)
		inv = &models.Invoice{
			BookingID: booking.ID, UserID: booking.UserID, CourtID: booking.CourtID,
			Number:      fmt.Sprintf("TMP-%d-%d", booking.ID, now.UnixNano()),
			TotalAmount: booking.TotalAmount, CommissionAmount: booking.CommissionAmount,
			NetAmount: booking.NetAmount, IssuedAt: now,
		}
		if err := s.invoices.Create(ctx, inv); err != nil {
			if errors.Is(err, repositories.ErrDuplicate) {
				// Otro webhook generó la factura al mismo tiempo.
				if existing, findErr := s.invoices.FindByBooking(ctx, bookingID); findErr == nil {
					inv = existing
				} else {
					return nil, Internal("No fue posible crear la factura", err)
				}
			} else {
				return nil, Internal("No fue posible crear la factura", err)
			}
		}
	}

	if inv.Number == "" || strings.HasPrefix(inv.Number, "TMP") {
		inv.Number = fmt.Sprintf("FAC-%d-%06d", inv.IssuedAt.Year(), inv.ID)
	}
	data := pdf.InvoiceData{
		Number: inv.Number, IssuedAt: inv.IssuedAt.Format("02/01/2006 15:04"),
		CustomerName: booking.User.Name, CustomerMail: booking.User.Email, CustomerDoc: booking.User.NIT,
		CourtName: booking.Court.Name, CourtSport: booking.Court.Sport, CourtCity: booking.Court.City,
		OwnerName: booking.Court.Owner.Name, BusinessName: booking.Court.Owner.BusinessName,
		OwnerNIT: booking.Court.Owner.NIT,
		Date:     booking.Date.Format("02/01/2006"), StartTime: booking.StartTime, EndTime: booking.EndTime,
		Hours: booking.Hours, PricePerHour: FormatCOP(booking.Court.PricePerHour),
		Total: FormatCOP(booking.TotalAmount), Commission: FormatCOP(booking.CommissionAmount),
		Net: FormatCOP(booking.NetAmount), Method: paymentMethodLabel(method), BookingID: booking.ID,
	}
	path, err := pdf.GenerateInvoice(data, s.cfg.InvoiceDir)
	if err != nil {
		return nil, Internal("No fue posible generar el PDF de la factura", err)
	}
	inv.Path = path
	if err := s.invoices.Update(ctx, inv); err != nil {
		return nil, Internal("No fue posible guardar la factura", err)
	}
	return inv, nil
}

func (s *InvoiceService) GetForBooking(ctx context.Context, bookingID uint) (*models.Invoice, error) {
	inv, err := s.invoices.FindByBooking(ctx, bookingID)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, NotFound("La factura aún no está disponible.")
		}
		return nil, Internal("No fue posible cargar la factura", err)
	}
	return inv, nil
}

func (s *InvoiceService) ListByUser(ctx context.Context, userID uint) ([]models.Invoice, error) {
	items, err := s.invoices.ListByUser(ctx, userID, 30)
	if err != nil {
		return nil, Internal("No fue posible cargar tus facturas", err)
	}
	return items, nil
}

func paymentMethodLabel(method string) string {
	switch method {
	case "credit_card":
		return "Tarjeta de crédito (Mercado Pago)"
	case "debit_card":
		return "Tarjeta débito (Mercado Pago)"
	case "pse":
		return "PSE (Mercado Pago)"
	case "mock":
		return "Pago simulado (desarrollo)"
	case "":
		return "Mercado Pago"
	default:
		return "Mercado Pago - " + method
	}
}
