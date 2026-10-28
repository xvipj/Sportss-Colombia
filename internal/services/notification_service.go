package services

import (
	"context"
	"log/slog"
	"sync"

	"sportss/internal/models"
	"sportss/internal/repositories"
)

// NotificationService persiste notificaciones y las publica a los clientes SSE.
type NotificationService struct {
	repo repositories.NotificationRepository
	mu   sync.RWMutex
	subs map[uint]map[chan models.Notification]struct{}
}

func NewNotificationService(repo repositories.NotificationRepository) *NotificationService {
	return &NotificationService{repo: repo, subs: make(map[uint]map[chan models.Notification]struct{})}
}

func (s *NotificationService) Notify(ctx context.Context, userID uint, typ, title, message, link string) {
	n := &models.Notification{UserID: userID, Type: typ, Title: title, Message: message, Link: link}
	if err := s.repo.Create(ctx, n); err != nil {
		slog.Error("no se pudo guardar la notificación", "user_id", userID, "type", typ, "error", err)
		return
	}
	s.publish(*n)
}

func (s *NotificationService) publish(n models.Notification) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for ch := range s.subs[n.UserID] {
		select {
		case ch <- n:
		default: // Cliente lento: descartar el evento para no bloquear la petición.
		}
	}
}

func (s *NotificationService) Subscribe(userID uint) (<-chan models.Notification, func()) {
	ch := make(chan models.Notification, 8)
	s.mu.Lock()
	if s.subs[userID] == nil {
		s.subs[userID] = make(map[chan models.Notification]struct{})
	}
	s.subs[userID][ch] = struct{}{}
	s.mu.Unlock()

	unsubscribe := func() {
		s.mu.Lock()
		if set, ok := s.subs[userID]; ok {
			delete(set, ch)
			if len(set) == 0 {
				delete(s.subs, userID)
			}
		}
		s.mu.Unlock()
	}
	return ch, unsubscribe
}

func (s *NotificationService) List(ctx context.Context, userID uint, limit int) ([]models.Notification, error) {
	items, err := s.repo.ListByUser(ctx, userID, limit)
	if err != nil {
		return nil, Internal("No fue posible cargar las notificaciones", err)
	}
	return items, nil
}

func (s *NotificationService) Unread(ctx context.Context, userID uint) (int64, error) {
	n, err := s.repo.CountUnread(ctx, userID)
	if err != nil {
		return 0, Internal("No fue posible contar las notificaciones", err)
	}
	return n, nil
}

func (s *NotificationService) MarkRead(ctx context.Context, id, userID uint) error {
	if err := s.repo.MarkRead(ctx, id, userID); err != nil {
		return Internal("No fue posible marcar la notificación", err)
	}
	return nil
}

func (s *NotificationService) MarkAllRead(ctx context.Context, userID uint) error {
	if err := s.repo.MarkAllRead(ctx, userID); err != nil {
		return Internal("No fue posible actualizar las notificaciones", err)
	}
	return nil
}
