package repositories

import (
	"context"
	"strings"

	"gorm.io/gorm"

	"sportss/internal/models"
)

type reviewRepository struct{ db *gorm.DB }

func NewReviewRepository(db *gorm.DB) ReviewRepository { return &reviewRepository{db: db} }

func (r *reviewRepository) Create(ctx context.Context, rev *models.Review) error {
	return translateError(r.db.WithContext(ctx).Create(rev).Error)
}

func (r *reviewRepository) Update(ctx context.Context, rev *models.Review) error {
	return translateError(r.db.WithContext(ctx).Save(rev).Error)
}

func (r *reviewRepository) FindByID(ctx context.Context, id uint) (*models.Review, error) {
	var rev models.Review
	err := r.db.WithContext(ctx).Preload("User").Preload("Court").Preload("Court.Owner").First(&rev, id).Error
	if err != nil {
		return nil, translateError(err)
	}
	return &rev, nil
}

func (r *reviewRepository) FindByBooking(ctx context.Context, bookingID uint) (*models.Review, error) {
	var rev models.Review
	if err := r.db.WithContext(ctx).Where("booking_id = ?", bookingID).First(&rev).Error; err != nil {
		return nil, translateError(err)
	}
	return &rev, nil
}

func (r *reviewRepository) ListByCourt(ctx context.Context, courtID uint, includeHidden bool, limit int) ([]models.Review, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	var reviews []models.Review
	q := r.db.WithContext(ctx).Where("court_id = ?", courtID)
	if !includeHidden {
		q = q.Where("hidden = ?", false)
	}
	err := q.Preload("User").Order("created_at DESC").Limit(limit).Find(&reviews).Error
	return reviews, translateError(err)
}

func (r *reviewRepository) ListByOwner(ctx context.Context, ownerID uint, offset, limit int) ([]models.Review, int64, error) {
	q := r.db.WithContext(ctx).Model(&models.Review{}).
		Joins("JOIN courts ON courts.id = reviews.court_id AND courts.deleted_at IS NULL").
		Where("courts.owner_id = ?", ownerID)
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, translateError(err)
	}
	var reviews []models.Review
	err := q.Preload("User").Preload("Court").Order("reviews.created_at DESC").
		Offset(offset).Limit(limit).Find(&reviews).Error
	return reviews, total, translateError(err)
}

func (r *reviewRepository) ListAll(ctx context.Context, search string, offset, limit int) ([]models.Review, int64, error) {
	q := r.db.WithContext(ctx).Model(&models.Review{})
	if search != "" {
		like := "%" + strings.ToLower(search) + "%"
		q = q.Where("LOWER(comment) LIKE ?", like)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, translateError(err)
	}
	var reviews []models.Review
	err := q.Preload("User").Preload("Court").Order("reviews.created_at DESC").
		Offset(offset).Limit(limit).Find(&reviews).Error
	return reviews, total, translateError(err)
}

func (r *reviewRepository) AverageByCourt(ctx context.Context, courtID uint) (float64, int, error) {
	var row struct {
		Avg   float64
		Count int
	}
	err := r.db.WithContext(ctx).Model(&models.Review{}).
		Select("COALESCE(AVG(rating), 0) AS avg, COUNT(*) AS count").
		Where("court_id = ? AND hidden = ?", courtID, false).Scan(&row).Error
	return row.Avg, row.Count, translateError(err)
}

func (r *reviewRepository) CountByOwner(ctx context.Context, ownerID uint) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&models.Review{}).
		Joins("JOIN courts ON courts.id = reviews.court_id AND courts.deleted_at IS NULL").
		Where("courts.owner_id = ?", ownerID).Count(&n).Error
	return n, translateError(err)
}
