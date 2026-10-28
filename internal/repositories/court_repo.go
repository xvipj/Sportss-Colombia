package repositories

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"sportss/internal/models"
)

type courtRepository struct {
	db *gorm.DB
}

func NewCourtRepository(db *gorm.DB) CourtRepository {
	return &courtRepository{db: db}
}

func (r *courtRepository) Create(ctx context.Context, c *models.Court) error {
	return translateError(r.db.WithContext(ctx).Create(c).Error)
}

func (r *courtRepository) Update(ctx context.Context, c *models.Court) error {
	return translateError(r.db.WithContext(ctx).Omit("Owner", "Photos").Save(c).Error)
}

func (r *courtRepository) FindByID(ctx context.Context, id uint) (*models.Court, error) {
	var c models.Court
	err := r.db.WithContext(ctx).
		Preload("Owner").
		Preload("Photos", func(db *gorm.DB) *gorm.DB { return db.Order("sort_order ASC, id ASC") }).
		First(&c, id).Error
	if err != nil {
		return nil, translateError(err)
	}
	return &c, nil
}

func (r *courtRepository) Delete(ctx context.Context, id uint) error {
	res := r.db.WithContext(ctx).Delete(&models.Court{}, id)
	if res.Error != nil {
		return translateError(res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *courtRepository) ListByOwner(ctx context.Context, ownerID uint) ([]models.Court, error) {
	var courts []models.Court
	err := r.db.WithContext(ctx).
		Where("owner_id = ?", ownerID).
		Preload("Photos", func(db *gorm.DB) *gorm.DB { return db.Order("sort_order ASC, id ASC") }).
		Order("created_at DESC").Find(&courts).Error
	return courts, translateError(err)
}

func (r *courtRepository) Search(ctx context.Context, p CourtSearchParams) ([]models.Court, int64, error) {
	q := r.db.WithContext(ctx).Model(&models.Court{}).Where("courts.status = ?", models.CourtActive)

	if p.Query != "" {
		like := "%" + strings.ToLower(p.Query) + "%"
		q = q.Where(`LOWER(courts.name) LIKE ? OR LOWER(courts.description) LIKE ? OR
			LOWER(courts.city) LIKE ? OR LOWER(courts.neighborhood) LIKE ? OR LOWER(courts.address) LIKE ?`,
			like, like, like, like, like)
	}
	if p.Sport != "" {
		q = q.Where("courts.sport = ?", p.Sport)
	}
	if p.City != "" {
		q = q.Where("LOWER(courts.city) = ?", strings.ToLower(strings.TrimSpace(p.City)))
	}
	if p.Department != "" {
		q = q.Where("LOWER(courts.department) = ?", strings.ToLower(strings.TrimSpace(p.Department)))
	}
	if p.MinPrice > 0 {
		q = q.Where("courts.price_per_hour >= ?", p.MinPrice)
	}
	if p.MaxPrice > 0 {
		q = q.Where("courts.price_per_hour <= ?", p.MaxPrice)
	}
	if p.Covered != nil {
		q = q.Where("courts.covered = ?", *p.Covered)
	}
	if p.Date != nil {
		q = q.Where("(',' || courts.available_days || ',') LIKE ?", "%,"+strconv.Itoa(int(p.Date.Weekday()))+",%")
		if p.Hour != "" {
			end, err := addOneHour(p.Hour)
			if err == nil {
				q = q.Where(`NOT EXISTS (
					SELECT 1 FROM bookings b
					WHERE b.court_id = courts.id
					  AND b.date = ?
					  AND b.status IN (?, ?, ?)
					  AND b.start_time < ?
					  AND b.end_time > ?
				)`, p.Date.Format("2006-01-02"), models.BookingPending, models.BookingConfirmed, models.BookingCompleted, end, p.Hour)
			}
		}
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, translateError(err)
	}

	switch p.Sort {
	case "price_asc":
		q = q.Order("courts.price_per_hour ASC, courts.rating_avg DESC")
	case "price_desc":
		q = q.Order("courts.price_per_hour DESC, courts.rating_avg DESC")
	case "rating":
		q = q.Order("courts.rating_avg DESC, courts.rating_count DESC")
	default: // relevancia
		q = q.Order("courts.rating_avg DESC, courts.rating_count DESC, courts.created_at DESC")
	}

	limit := p.Limit
	if limit <= 0 || limit > 60 {
		limit = 9
	}
	page := p.Page
	if page < 1 {
		page = 1
	}

	var courts []models.Court
	err := q.Preload("Owner").
		Preload("Photos", func(db *gorm.DB) *gorm.DB { return db.Order("sort_order ASC, id ASC") }).
		Offset((page - 1) * limit).Limit(limit).Find(&courts).Error
	return courts, total, translateError(err)
}

func (r *courtRepository) Featured(ctx context.Context, limit int) ([]models.Court, error) {
	if limit <= 0 {
		limit = 6
	}
	var courts []models.Court
	err := r.db.WithContext(ctx).Where("status = ?", models.CourtActive).
		Preload("Photos", func(db *gorm.DB) *gorm.DB { return db.Order("sort_order ASC, id ASC") }).
		Order("rating_avg DESC, rating_count DESC, created_at DESC").Limit(limit).Find(&courts).Error
	return courts, translateError(err)
}

func (r *courtRepository) UpdateRating(ctx context.Context, courtID uint) error {
	var agg struct {
		Avg   float64
		Count int
	}
	err := r.db.WithContext(ctx).Model(&models.Review{}).
		Select("COALESCE(AVG(rating), 0) AS avg, COUNT(*) AS count").
		Where("court_id = ? AND hidden = ?", courtID, false).Scan(&agg).Error
	if err != nil {
		return translateError(err)
	}
	return translateError(r.db.WithContext(ctx).Model(&models.Court{}).Where("id = ?", courtID).
		Updates(map[string]any{"rating_avg": agg.Avg, "rating_count": agg.Count}).Error)
}

func (r *courtRepository) SetStatus(ctx context.Context, id uint, status string) error {
	res := r.db.WithContext(ctx).Model(&models.Court{}).Where("id = ?", id).Update("status", status)
	if res.Error != nil {
		return translateError(res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *courtRepository) ListAll(ctx context.Context, search, status string, offset, limit int) ([]models.Court, int64, error) {
	q := r.db.WithContext(ctx).Model(&models.Court{})
	if search != "" {
		like := "%" + strings.ToLower(search) + "%"
		q = q.Where("LOWER(name) LIKE ? OR LOWER(city) LIKE ? OR LOWER(neighborhood) LIKE ?", like, like, like)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, translateError(err)
	}
	var courts []models.Court
	err := q.Preload("Owner").Preload("Photos").
		Order("created_at DESC").Offset(offset).Limit(limit).Find(&courts).Error
	return courts, total, translateError(err)
}

func (r *courtRepository) CountAll(ctx context.Context) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&models.Court{}).Count(&n).Error
	return n, translateError(err)
}

func (r *courtRepository) CountByOwner(ctx context.Context, ownerID uint) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&models.Court{}).Where("owner_id = ?", ownerID).Count(&n).Error
	return n, translateError(err)
}

func (r *courtRepository) AddPhoto(ctx context.Context, photo *models.CourtPhoto) error {
	return translateError(r.db.WithContext(ctx).Create(photo).Error)
}

func (r *courtRepository) DeletePhoto(ctx context.Context, courtID, photoID uint) (string, error) {
	var photo models.CourtPhoto
	if err := r.db.WithContext(ctx).Where("id = ? AND court_id = ?", photoID, courtID).First(&photo).Error; err != nil {
		return "", translateError(err)
	}
	if err := r.db.WithContext(ctx).Delete(&photo).Error; err != nil {
		return "", translateError(err)
	}
	return photo.Path, nil
}

func (r *courtRepository) CountPhotos(ctx context.Context, courtID uint) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&models.CourtPhoto{}).Where("court_id = ?", courtID).Count(&n).Error
	return n, translateError(err)
}

func addOneHour(hhmm string) (string, error) {
	t, err := time.Parse("15:04", hhmm)
	if err != nil {
		return "", fmt.Errorf("hora inválida: %s", hhmm)
	}
	return t.Add(time.Hour).Format("15:04"), nil
}
