package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"sportss/internal/cache"
	"sportss/internal/config"
	"sportss/internal/models"
	"sportss/internal/repositories"
)

type BookingService struct {
	cfg      *config.Config
	bookings repositories.BookingRepository
	courts   repositories.CourtRepository
	cache    cache.Store
	notifier *NotificationService
}

type CreateBookingInput struct {
	CourtID   uint
	Date      string
	StartTime string
	Hours     int
}

func NewBookingService(cfg *config.Config, bookings repositories.BookingRepository, courts repositories.CourtRepository, cacheStore cache.Store, notifier *NotificationService) *BookingService {
	return &BookingService{cfg: cfg, bookings: bookings, courts: courts, cache: cacheStore, notifier: notifier}
}

func (s *BookingService) Create(ctx context.Context, user *models.User, in CreateBookingInput) (*models.Booking, error) {
	if in.Hours < 1 || in.Hours > 12 {
		return nil, BadRequest("Puedes reservar entre 1 y 12 horas consecutivas.")
	}
	court, err := s.courts.FindByID(ctx, in.CourtID)
	if err != nil {
		return nil, NotFound("Cancha no encontrada.")
	}
	if court.Status != models.CourtActive {
		return nil, BadRequest("Esta cancha no está disponible para reservas.")
	}
	date, err := s.parseDate(in.Date)
	if err != nil {
		return nil, err
	}
	if date.Before(dateOnlyUTC(time.Now().In(s.cfg.Location))) {
		return nil, BadRequest("No puedes reservar en el pasado.")
	}
	if !dayAvailable(court.AvailableDays, int(date.Weekday())) {
		return nil, BadRequest("La cancha no atiende ese día.")
	}
	start, err := time.Parse("15:04", in.StartTime)
	if err != nil {
		return nil, BadRequest("Selecciona una hora de inicio válida.")
	}
	open, _ := time.Parse("15:04", court.OpenTime)
	close, _ := time.Parse("15:04", court.CloseTime)
	end := start.Add(time.Duration(in.Hours) * time.Hour)
	if start.Before(open) || end.After(close) {
		return nil, BadRequest("El horario seleccionado está fuera de la atención de la cancha.")
	}
	bookingStart := time.Date(date.Year(), date.Month(), date.Day(), start.Hour(), start.Minute(), 0, 0, s.cfg.Location)
	if bookingStart.Before(time.Now().In(s.cfg.Location)) {
		return nil, BadRequest("La hora de inicio ya pasó.")
	}
	startStr := start.Format("15:04")
	endStr := end.Format("15:04")

	// Bloqueo distribuido por cada hora mientras se completa el pago.
	lockTTL := time.Duration(s.cfg.BookingLockMinutes) * time.Minute
	acquired := make(map[string]string)
	for i := 0; i < in.Hours; i++ {
		hour := start.Add(time.Duration(i) * time.Hour).Format("15:04")
		key := fmt.Sprintf("booking:lock:%d:%s:%s", court.ID, date.Format("2006-01-02"), hour)
		token, ok, lockErr := s.cache.AcquireLock(ctx, key, lockTTL)
		if lockErr != nil {
			s.releaseLocks(ctx, acquired)
			return nil, Internal("No fue posible bloquear el horario", lockErr)
		}
		if !ok {
			s.releaseLocks(ctx, acquired)
			return nil, Conflict("Ese horario acaba de ser tomado por otra persona. Elige otra hora.")
		}
		acquired[key] = token
	}

	total := court.PricePerHour * int64(in.Hours)
	commission := int64(float64(total)*s.cfg.CommissionPercent/100 + 0.5)
	booking := &models.Booking{
		CourtID: court.ID, UserID: user.ID, Date: date, StartTime: startStr, EndTime: endStr,
		Hours: in.Hours, TotalAmount: total, CommissionAmount: commission, NetAmount: total - commission,
		Status: models.BookingPending,
	}

	err = s.bookings.WithTx(ctx, func(tx repositories.BookingRepository) error {
		overlap, err := tx.FindOverlapping(ctx, court.ID, date, startStr, endStr)
		if err != nil {
			return err
		}
		if overlap {
			return repositories.ErrDuplicate
		}
		return tx.Create(ctx, booking)
	})
	if err != nil {
		s.releaseLocks(ctx, acquired)
		if errors.Is(err, repositories.ErrDuplicate) {
			return nil, Conflict("Ese horario ya está reservado. Elige otra hora.")
		}
		return nil, Internal("No fue posible crear la reserva", err)
	}
	// Los locks se mantienen hasta que expire el pago; la reserva pendiente
	// también protege el horario en la base de datos.
	_ = s.cache.DeletePrefix(ctx, "courts:search:")

	s.notifier.Notify(ctx, user.ID, models.NotifyBookingCreated, "Reserva creada",
		fmt.Sprintf("Reservaste %s en %s para el %s de %s a %s. Completa el pago para confirmar.",
			pluralHours(in.Hours), court.Name, date.Format("02/01/2006"), startStr, endStr), "/mis-reservas")
	s.notifier.Notify(ctx, court.OwnerID, models.NotifyNewBooking, "Nueva reserva recibida",
		fmt.Sprintf("%s quiere reservar %s el %s de %s a %s.", user.Name, court.Name,
			date.Format("02/01/2006"), startStr, endStr), "/panel/reservas")

	return booking, nil
}

func (s *BookingService) Get(ctx context.Context, id uint) (*models.Booking, error) {
	b, err := s.bookings.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, NotFound("Reserva no encontrada.")
		}
		return nil, Internal("No fue posible cargar la reserva", err)
	}
	return b, nil
}

func (s *BookingService) GetAccessible(ctx context.Context, id, userID uint, isAdmin bool) (*models.Booking, error) {
	b, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if b.UserID != userID && b.Court.OwnerID != userID && !isAdmin {
		return nil, Forbidden("No tienes acceso a esta reserva.")
	}
	return b, nil
}

func (s *BookingService) ListByUser(ctx context.Context, userID uint, status string, page, limit int) ([]models.Booking, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit <= 0 {
		limit = 10
	}
	items, total, err := s.bookings.ListByUser(ctx, userID, status, (page-1)*limit, limit)
	if err != nil {
		return nil, 0, Internal("No fue posible cargar tus reservas", err)
	}
	return items, total, nil
}

// HasCompletedForCourt indica si el usuario puede reseñar una cancha.
func (s *BookingService) HasCompletedForCourt(ctx context.Context, userID, courtID uint) (bool, error) {
	items, _, err := s.bookings.ListByUser(ctx, userID, models.BookingCompleted, 0, 200)
	if err != nil {
		return false, Internal("No fue posible validar tus reservas", err)
	}
	for _, b := range items {
		if b.CourtID == courtID {
			return true, nil
		}
	}
	return false, nil
}

// AllByUser devuelve el historial completo para cruzar facturas y reseñas.
func (s *BookingService) AllByUser(ctx context.Context, userID uint) ([]models.Booking, error) {
	items, _, err := s.bookings.ListByUser(ctx, userID, "", 0, 200)
	if err != nil {
		return nil, Internal("No fue posible cargar tus reservas", err)
	}
	return items, nil
}

func (s *BookingService) ListByOwner(ctx context.Context, ownerID uint, status, date string, page, limit int) ([]models.Booking, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit <= 0 {
		limit = 20
	}
	items, total, err := s.bookings.ListByOwner(ctx, ownerID, status, date, (page-1)*limit, limit)
	if err != nil {
		return nil, 0, Internal("No fue posible cargar las reservas", err)
	}
	return items, total, nil
}

func (s *BookingService) ListAll(ctx context.Context, status, date string, page, limit int) ([]models.Booking, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit <= 0 {
		limit = 20
	}
	items, total, err := s.bookings.ListAll(ctx, status, date, (page-1)*limit, limit)
	if err != nil {
		return nil, 0, Internal("No fue posible cargar las reservas", err)
	}
	return items, total, nil
}

// CancelByUser aplica la política de cancelación configurada.
func (s *BookingService) CancelByUser(ctx context.Context, bookingID, userID uint) error {
	b, err := s.Get(ctx, bookingID)
	if err != nil {
		return err
	}
	if b.UserID != userID {
		return Forbidden("Esta reserva no te pertenece.")
	}
	if b.Status != models.BookingPending && b.Status != models.BookingConfirmed {
		return BadRequest("Esta reserva ya no se puede cancelar.")
	}
	if b.Status == models.BookingConfirmed && s.cfg.CancellationHours > 0 {
		start := s.bookingStart(b)
		if time.Now().In(s.cfg.Location).Add(time.Duration(s.cfg.CancellationHours) * time.Hour).After(start) {
			return BadRequest(fmt.Sprintf("Solo puedes cancelar hasta %d horas antes del inicio.", s.cfg.CancellationHours))
		}
	}
	return s.cancel(ctx, b, "Cancelaste la reserva")
}

func (s *BookingService) CancelByOwner(ctx context.Context, bookingID, ownerID uint, isAdmin bool) error {
	b, err := s.Get(ctx, bookingID)
	if err != nil {
		return err
	}
	if !isAdmin && b.Court.OwnerID != ownerID {
		return Forbidden("No tienes permiso sobre esta reserva.")
	}
	if b.Status != models.BookingPending && b.Status != models.BookingConfirmed {
		return BadRequest("Esta reserva ya no se puede cancelar.")
	}
	return s.cancel(ctx, b, "El establecimiento canceló la reserva")
}

func (s *BookingService) cancel(ctx context.Context, b *models.Booking, reason string) error {
	now := time.Now().In(s.cfg.Location)
	if err := s.bookings.Update(ctx, b.ID, map[string]any{
		"status": models.BookingCancelled, "cancelled_at": now,
	}); err != nil {
		return Internal("No fue posible cancelar la reserva", err)
	}
	_ = s.cache.DeletePrefix(ctx, "courts:search:")
	s.clearBookingLocks(ctx, b)
	s.notifier.Notify(ctx, b.UserID, models.NotifyCancelled, "Reserva cancelada",
		fmt.Sprintf("%s: %s del %s a las %s.", reason, b.Court.Name, b.Date.Format("02/01/2006"), b.StartTime), "/mis-reservas")
	if b.Court.OwnerID != b.UserID {
		s.notifier.Notify(ctx, b.Court.OwnerID, models.NotifyCancelled, "Reserva cancelada",
			fmt.Sprintf("%s para %s el %s a las %s.", reason, b.Court.Name, b.Date.Format("02/01/2006"), b.StartTime), "/panel/reservas")
	}
	return nil
}

func (s *BookingService) CompleteByOwner(ctx context.Context, bookingID, ownerID uint, isAdmin bool) error {
	b, err := s.Get(ctx, bookingID)
	if err != nil {
		return err
	}
	if !isAdmin && b.Court.OwnerID != ownerID {
		return Forbidden("No tienes permiso sobre esta reserva.")
	}
	if b.Status != models.BookingConfirmed {
		return BadRequest("Solo se pueden completar reservas confirmadas.")
	}
	now := time.Now().In(s.cfg.Location)
	if err := s.bookings.Update(ctx, b.ID, map[string]any{
		"status": models.BookingCompleted, "completed_at": now,
	}); err != nil {
		return Internal("No fue posible completar la reserva", err)
	}
	s.notifier.Notify(ctx, b.UserID, models.NotifyCompleted, "Reserva completada",
		fmt.Sprintf("Tu reserva en %s quedó completada. ¡Cuéntanos cómo te fue!", b.Court.Name), "/mis-reservas")
	return nil
}

// ExpirePending libera los horarios de reservas sin pago.
func (s *BookingService) ExpirePending(ctx context.Context) (int, error) {
	cutoff := time.Now().Add(-time.Duration(s.cfg.PendingMinutes) * time.Minute)
	expired, err := s.bookings.ExpirePending(ctx, cutoff)
	if err != nil {
		return 0, err
	}
	if len(expired) > 0 {
		_ = s.cache.DeletePrefix(ctx, "courts:search:")
		for i := range expired {
			s.clearBookingLocks(ctx, &expired[i])
		}
	}
	return len(expired), nil
}

// SendReminders avisa una hora antes de cada reserva confirmada.
func (s *BookingService) SendReminders(ctx context.Context) (int, error) {
	now := time.Now().In(s.cfg.Location)
	candidates, err := s.bookings.ConfirmedStartingBetween(ctx, now.Add(50*time.Minute), now.Add(70*time.Minute))
	if err != nil {
		return 0, err
	}
	sent := 0
	for _, b := range candidates {
		start := s.bookingStart(&b)
		diff := start.Sub(now)
		if diff < 50*time.Minute || diff > 70*time.Minute {
			continue
		}
		s.notifier.Notify(ctx, b.UserID, models.NotifyReminder, "Tu reserva empieza en una hora",
			fmt.Sprintf("Recuerda tu reserva en %s hoy a las %s.", b.Court.Name, b.StartTime), "/mis-reservas")
		s.notifier.Notify(ctx, b.Court.OwnerID, models.NotifyReminder, "Reserva próxima",
			fmt.Sprintf("Tienes una reserva en %s a las %s.", b.Court.Name, b.StartTime), "/panel/reservas")
		if err := s.bookings.MarkReminderSent(ctx, b.ID); err == nil {
			sent++
		}
	}
	return sent, nil
}

// AutoComplete cierra reservas cuyo horario ya terminó.
func (s *BookingService) AutoComplete(ctx context.Context) (int, error) {
	now := time.Now().In(s.cfg.Location)
	completed, err := s.bookings.AutoCompletePast(ctx, now)
	if err != nil {
		return 0, err
	}
	for _, b := range completed {
		s.notifier.Notify(ctx, b.UserID, models.NotifyCompleted, "Reserva completada",
			fmt.Sprintf("Tu reserva en %s finalizó. Ya puedes dejar una reseña.", b.Court.Name), "/mis-reservas")
	}
	return len(completed), nil
}

func (s *BookingService) bookingStart(b *models.Booking) time.Time {
	start, _ := time.Parse("15:04", b.StartTime)
	return time.Date(b.Date.Year(), b.Date.Month(), b.Date.Day(), start.Hour(), start.Minute(), 0, 0, s.cfg.Location)
}

func (s *BookingService) parseDate(dateStr string) (time.Time, error) {
	t, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(dateStr), s.cfg.Location)
	if err != nil {
		return time.Time{}, BadRequest("Selecciona una fecha válida.")
	}
	return dateOnlyUTC(t), nil
}

func (s *BookingService) releaseLocks(ctx context.Context, locks map[string]string) {
	for key, token := range locks {
		if err := s.cache.ReleaseLock(ctx, key, token); err != nil {
			slog.Warn("no se pudo liberar lock de reserva", "key", key, "error", err)
		}
	}
}

// clearBookingLocks libera los locks de las horas de una reserva cancelada o
// expirada. Es seguro borrarlos sin token: mientras la reserva estaba activa
// nadie más pudo adquirir esas mismas horas (la BD también lo impedía).
func (s *BookingService) clearBookingLocks(ctx context.Context, b *models.Booking) {
	start, err := time.Parse("15:04", b.StartTime)
	if err != nil {
		return
	}
	for i := 0; i < b.Hours; i++ {
		hour := start.Add(time.Duration(i) * time.Hour).Format("15:04")
		key := fmt.Sprintf("booking:lock:%d:%s:%s", b.CourtID, b.Date.Format("2006-01-02"), hour)
		if err := s.cache.Delete(ctx, key); err != nil {
			slog.Warn("no se pudo limpiar lock de reserva", "key", key, "error", err)
		}
	}
}

func pluralHours(n int) string {
	if n == 1 {
		return "1 hora"
	}
	return strconv.Itoa(n) + " horas"
}
