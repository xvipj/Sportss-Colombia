package repositories

import (
	"context"

	"gorm.io/gorm"

	"sportss/internal/models"
)

type invoiceRepository struct{ db *gorm.DB }

func NewInvoiceRepository(db *gorm.DB) InvoiceRepository { return &invoiceRepository{db: db} }

func (r *invoiceRepository) Create(ctx context.Context, i *models.Invoice) error {
	return translateError(r.db.WithContext(ctx).Create(i).Error)
}

func (r *invoiceRepository) Update(ctx context.Context, i *models.Invoice) error {
	return translateError(r.db.WithContext(ctx).Save(i).Error)
}

func (r *invoiceRepository) FindByID(ctx context.Context, id uint) (*models.Invoice, error) {
	var i models.Invoice
	if err := r.db.WithContext(ctx).First(&i, id).Error; err != nil {
		return nil, translateError(err)
	}
	return &i, nil
}

func (r *invoiceRepository) FindByBooking(ctx context.Context, bookingID uint) (*models.Invoice, error) {
	var i models.Invoice
	if err := r.db.WithContext(ctx).Where("booking_id = ?", bookingID).First(&i).Error; err != nil {
		return nil, translateError(err)
	}
	return &i, nil
}

func (r *invoiceRepository) ListByUser(ctx context.Context, userID uint, limit int) ([]models.Invoice, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	var invoices []models.Invoice
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).
		Order("issued_at DESC").Limit(limit).Find(&invoices).Error
	return invoices, translateError(err)
}
