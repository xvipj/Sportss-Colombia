package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"sportss/internal/models"
	"sportss/internal/repositories"
)

type ReviewService struct {
	reviews  repositories.ReviewRepository
	bookings repositories.BookingRepository
	courts   repositories.CourtRepository
	notifier *NotificationService
}

func NewReviewService(reviews repositories.ReviewRepository, bookings repositories.BookingRepository, courts repositories.CourtRepository, notifier *NotificationService) *ReviewService {
	return &ReviewService{reviews: reviews, bookings: bookings, courts: courts, notifier: notifier}
}

func (s *ReviewService) Create(ctx context.Context, userID, bookingID uint, rating int, comment string) (*models.Review, error) {
	if rating < 1 || rating > 5 {
		return nil, BadRequest("La calificación debe estar entre 1 y 5 estrellas.")
	}
	comment = strings.TrimSpace(comment)
	if len(comment) < 3 {
		return nil, BadRequest("Escribe un comentario sobre tu experiencia.")
	}
	if len(comment) > 1500 {
		return nil, BadRequest("El comentario es demasiado largo.")
	}
	booking, err := s.bookings.FindByID(ctx, bookingID)
	if err != nil {
		return nil, NotFound("Reserva no encontrada.")
	}
	if booking.UserID != userID {
		return nil, Forbidden("Esta reserva no te pertenece.")
	}
	if booking.Status != models.BookingCompleted {
		return nil, BadRequest("Solo puedes reseñar reservas completadas.")
	}
	if _, err := s.reviews.FindByBooking(ctx, bookingID); err == nil {
		return nil, Conflict("Ya dejaste una reseña para esta reserva.")
	} else if !errors.Is(err, repositories.ErrNotFound) {
		return nil, Internal("No fue posible validar la reseña", err)
	}
	review := &models.Review{
		BookingID: booking.ID, CourtID: booking.CourtID, UserID: userID,
		Rating: rating, Comment: comment,
	}
	if err := s.reviews.Create(ctx, review); err != nil {
		return nil, Internal("No fue posible publicar la reseña", err)
	}
	if err := s.courts.UpdateRating(ctx, booking.CourtID); err != nil {
		return nil, Internal("No fue posible actualizar la calificación", err)
	}
	s.notifier.Notify(ctx, booking.Court.OwnerID, models.NotifyNewReview, "Nueva reseña recibida",
		fmt.Sprintf("%s calificó %s con %d estrellas.", booking.User.Name, booking.Court.Name, rating), "/panel/resenas")
	return review, nil
}

func (s *ReviewService) Respond(ctx context.Context, reviewID, ownerID uint, response string, isAdmin bool) error {
	response = strings.TrimSpace(response)
	if len(response) < 2 {
		return BadRequest("Escribe una respuesta.")
	}
	if len(response) > 1000 {
		return BadRequest("La respuesta es demasiado larga.")
	}
	review, err := s.reviews.FindByID(ctx, reviewID)
	if err != nil {
		return NotFound("Reseña no encontrada.")
	}
	if !isAdmin && review.Court.OwnerID != ownerID {
		return Forbidden("No puedes responder esta reseña.")
	}
	review.OwnerResponse = response
	if err := s.reviews.Update(ctx, review); err != nil {
		return Internal("No fue posible guardar la respuesta", err)
	}
	s.notifier.Notify(ctx, review.UserID, models.NotifyNewReview, "El establecimiento respondió tu reseña",
		fmt.Sprintf("Respondieron tu reseña en %s.", review.Court.Name), "/canchas/"+fmt.Sprint(review.CourtID))
	return nil
}

func (s *ReviewService) SetHidden(ctx context.Context, reviewID uint, hidden bool) error {
	review, err := s.reviews.FindByID(ctx, reviewID)
	if err != nil {
		return NotFound("Reseña no encontrada.")
	}
	review.Hidden = hidden
	if err := s.reviews.Update(ctx, review); err != nil {
		return Internal("No fue posible moderar la reseña", err)
	}
	if err := s.courts.UpdateRating(ctx, review.CourtID); err != nil {
		return Internal("No fue posible actualizar la calificación", err)
	}
	return nil
}

func (s *ReviewService) ListByCourt(ctx context.Context, courtID uint) ([]models.Review, error) {
	items, err := s.reviews.ListByCourt(ctx, courtID, false, 50)
	if err != nil {
		return nil, Internal("No fue posible cargar las reseñas", err)
	}
	return items, nil
}

func (s *ReviewService) ListByOwner(ctx context.Context, ownerID uint, page, limit int) ([]models.Review, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit <= 0 {
		limit = 20
	}
	items, total, err := s.reviews.ListByOwner(ctx, ownerID, (page-1)*limit, limit)
	if err != nil {
		return nil, 0, Internal("No fue posible cargar las reseñas", err)
	}
	return items, total, nil
}

func (s *ReviewService) ListAll(ctx context.Context, search string, page, limit int) ([]models.Review, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit <= 0 {
		limit = 20
	}
	items, total, err := s.reviews.ListAll(ctx, search, (page-1)*limit, limit)
	if err != nil {
		return nil, 0, Internal("No fue posible cargar las reseñas", err)
	}
	return items, total, nil
}

func (s *ReviewService) CountByOwner(ctx context.Context, ownerID uint) (int64, error) {
	n, err := s.reviews.CountByOwner(ctx, ownerID)
	if err != nil {
		return 0, Internal("No fue posible contar las reseñas", err)
	}
	return n, nil
}

func (s *ReviewService) ByBooking(ctx context.Context, bookingID uint) (*models.Review, error) {
	rev, err := s.reviews.FindByBooking(ctx, bookingID)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, nil
		}
		return nil, Internal("No fue posible cargar la reseña", err)
	}
	return rev, nil
}
