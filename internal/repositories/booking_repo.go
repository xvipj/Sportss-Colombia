package repositories

import (
	"context"
	"time"

	"gorm.io/gorm"

	"sportss/internal/models"
)

type bookingRepository struct {
	db *gorm.DB
}

func NewBookingRepository(db *gorm.DB) BookingRepository {
	return &bookingRepository{db: db}
}

func (r *bookingRepository) WithTx(ctx context.Context, fn func(BookingRepository) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&bookingRepository{db: tx})
	})
}

func (r *bookingRepository) Create(ctx context.Context, b *models.Booking) error {
	return translateError(r.db.WithContext(ctx).Create(b).Error)
}

func (r *bookingRepository) Update(ctx context.Context, id uint, fields map[string]any) error {
	res := r.db.WithContext(ctx).Model(&models.Booking{}).Where("id = ?", id).Updates(fields)
	if res.Error != nil {
		return translateError(res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *bookingRepository) FindByID(ctx context.Context, id uint) (*models.Booking, error) {
	var b models.Booking
	err := r.db.WithContext(ctx).
		Preload("User").
		Preload("Court").Preload("Court.Owner").Preload("Court.Photos").
		First(&b, id).Error
	if err != nil {
		return nil, translateError(err)
	}
	return &b, nil
}

func (r *bookingRepository) FindOverlapping(ctx context.Context, courtID uint, date time.Time, start, end string) (bool, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&models.Booking{}).
		Where("court_id = ? AND date = ? AND status IN (?, ?, ?) AND start_time < ? AND end_time > ?",
			courtID, date, models.BookingPending, models.BookingConfirmed, models.BookingCompleted, end, start).
		Count(&n).Error
	return n > 0, translateError(err)
}

func (r *bookingRepository) ActiveForDate(ctx context.Context, courtID uint, date time.Time) ([]models.Booking, error) {
	var bookings []models.Booking
	err := r.db.WithContext(ctx).
		Where("court_id = ? AND date = ? AND status IN (?, ?, ?)",
			courtID, date, models.BookingPending, models.BookingConfirmed, models.BookingCompleted).
		Order("start_time ASC").Find(&bookings).Error
	return bookings, translateError(err)
}

func (r *bookingRepository) ListByUser(ctx context.Context, userID uint, status string, offset, limit int) ([]models.Booking, int64, error) {
	q := r.db.WithContext(ctx).Model(&models.Booking{}).Where("bookings.user_id = ?", userID)
	if status != "" {
		q = q.Where("bookings.status = ?", status)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, translateError(err)
	}
	var bookings []models.Booking
	err := q.Preload("Court").Preload("Court.Photos").
		Order("bookings.date DESC, bookings.start_time DESC").Offset(offset).Limit(limit).Find(&bookings).Error
	return bookings, total, translateError(err)
}

func (r *bookingRepository) ListByOwner(ctx context.Context, ownerID uint, status, date string, offset, limit int) ([]models.Booking, int64, error) {
	q := r.db.WithContext(ctx).Model(&models.Booking{}).
		Joins("JOIN courts ON courts.id = bookings.court_id AND courts.deleted_at IS NULL").
		Where("courts.owner_id = ?", ownerID)
	if status != "" {
		q = q.Where("bookings.status = ?", status)
	}
	if date != "" {
		q = q.Where("DATE(bookings.date) = ?", date)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, translateError(err)
	}
	var bookings []models.Booking
	err := q.Preload("User").Preload("Court").Preload("Court.Owner").
		Order("bookings.date DESC, bookings.start_time DESC").Offset(offset).Limit(limit).Find(&bookings).Error
	return bookings, total, translateError(err)
}

func (r *bookingRepository) ListAll(ctx context.Context, status, date string, offset, limit int) ([]models.Booking, int64, error) {
	q := r.db.WithContext(ctx).Model(&models.Booking{})
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if date != "" {
		q = q.Where("DATE(date) = ?", date)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, translateError(err)
	}
	var bookings []models.Booking
	err := q.Preload("User").Preload("Court").Preload("Court.Owner").
		Order("date DESC, start_time DESC").Offset(offset).Limit(limit).Find(&bookings).Error
	return bookings, total, translateError(err)
}

func (r *bookingRepository) ExpirePending(ctx context.Context, cutoff time.Time) ([]models.Booking, error) {
	var bookings []models.Booking
	if err := r.db.WithContext(ctx).
		Where("status = ? AND created_at <= ?", models.BookingPending, cutoff).
		Find(&bookings).Error; err != nil {
		return nil, translateError(err)
	}
	if len(bookings) > 0 {
		ids := make([]uint, 0, len(bookings))
		for _, b := range bookings {
			ids = append(ids, b.ID)
		}
		if err := r.db.WithContext(ctx).Model(&models.Booking{}).
			Where("id IN ? AND status = ?", ids, models.BookingPending).
			Update("status", models.BookingExpired).Error; err != nil {
			return nil, translateError(err)
		}
	}
	return bookings, nil
}

func (r *bookingRepository) ConfirmedStartingBetween(ctx context.Context, from, to time.Time) ([]models.Booking, error) {
	var bookings []models.Booking
	err := r.db.WithContext(ctx).
		Where("status = ? AND reminder_sent = ? AND date >= ? AND date <= ?",
			models.BookingConfirmed, false, dateOnly(from), dateOnly(to)).
		Preload("Court").Preload("User").Find(&bookings).Error
	return bookings, translateError(err)
}

func (r *bookingRepository) MarkReminderSent(ctx context.Context, id uint) error {
	return translateError(r.db.WithContext(ctx).Model(&models.Booking{}).
		Where("id = ?", id).Update("reminder_sent", true).Error)
}

// AutoCompletePast finaliza reservas confirmadas cuya hora final ya pasó.
func (r *bookingRepository) AutoCompletePast(ctx context.Context, now time.Time) ([]models.Booking, error) {
	var candidates []models.Booking
	err := r.db.WithContext(ctx).
		Where("status = ? AND date <= ?", models.BookingConfirmed, dateOnly(now)).
		Preload("Court").Preload("User").Find(&candidates).Error
	if err != nil {
		return nil, translateError(err)
	}
	completed := make([]models.Booking, 0)
	for _, b := range candidates {
		end, err := time.ParseInLocation("2006-01-02 15:04",
			b.Date.Format("2006-01-02")+" "+b.EndTime, now.Location())
		if err != nil || end.After(now) {
			continue
		}
		res := r.db.WithContext(ctx).Model(&models.Booking{}).
			Where("id = ? AND status = ?", b.ID, models.BookingConfirmed).
			Updates(map[string]any{"status": models.BookingCompleted, "completed_at": now})
		if res.Error == nil && res.RowsAffected > 0 {
			b.Status = models.BookingCompleted
			b.CompletedAt = &now
			completed = append(completed, b)
		}
	}
	return completed, nil
}

// dateOnly normaliza una fecha calendario a medianoche UTC para que todas las
// reservas de un mismo día se comparen de forma consistente en SQLite.
func dateOnly(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}
