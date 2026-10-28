package repositories

import (
	"context"

	"gorm.io/gorm"

	"sportss/internal/models"
)

type paymentRepository struct{ db *gorm.DB }

func NewPaymentRepository(db *gorm.DB) PaymentRepository { return &paymentRepository{db: db} }

func (r *paymentRepository) Create(ctx context.Context, p *models.Payment) error {
	return translateError(r.db.WithContext(ctx).Create(p).Error)
}

func (r *paymentRepository) Update(ctx context.Context, p *models.Payment) error {
	return translateError(r.db.WithContext(ctx).Save(p).Error)
}

func (r *paymentRepository) FindByBooking(ctx context.Context, bookingID uint) (*models.Payment, error) {
	var p models.Payment
	if err := r.db.WithContext(ctx).Where("booking_id = ?", bookingID).First(&p).Error; err != nil {
		return nil, translateError(err)
	}
	return &p, nil
}

func (r *paymentRepository) FindByExternalID(ctx context.Context, externalID string) (*models.Payment, error) {
	var p models.Payment
	if err := r.db.WithContext(ctx).Where("external_id = ?", externalID).First(&p).Error; err != nil {
		return nil, translateError(err)
	}
	return &p, nil
}

func (r *paymentRepository) ListByUser(ctx context.Context, userID uint, limit int) ([]models.Payment, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	var payments []models.Payment
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).
		Order("created_at DESC").Limit(limit).Find(&payments).Error
	return payments, translateError(err)
}
