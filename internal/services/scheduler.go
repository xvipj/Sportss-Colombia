package services

import (
	"context"
	"log/slog"
	"time"
)

// Scheduler ejecuta tareas de mantenimiento cada minuto: expiración de pagos,
// recordatorios y finalización automática de reservas.
type Scheduler struct {
	bookings *BookingService
}

func NewScheduler(bookings *BookingService) *Scheduler {
	return &Scheduler{bookings: bookings}
}

func (s *Scheduler) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	s.runOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			slog.Info("scheduler detenido")
			return
		case <-ticker.C:
			s.runOnce(ctx)
		}
	}
}

func (s *Scheduler) runOnce(ctx context.Context) {
	if n, err := s.bookings.ExpirePending(ctx); err != nil {
		slog.Error("error expirando reservas", "error", err)
	} else if n > 0 {
		slog.Info("reservas pendientes expiradas", "cantidad", n)
	}
	if n, err := s.bookings.SendReminders(ctx); err != nil {
		slog.Error("error enviando recordatorios", "error", err)
	} else if n > 0 {
		slog.Info("recordatorios enviados", "cantidad", n)
	}
	if n, err := s.bookings.AutoComplete(ctx); err != nil {
		slog.Error("error completando reservas", "error", err)
	} else if n > 0 {
		slog.Info("reservas completadas automáticamente", "cantidad", n)
	}
}
