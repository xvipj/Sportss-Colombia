package repositories

import (
	"context"
	"strings"

	"gorm.io/gorm"

	"sportss/internal/models"
)

type userRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) Create(ctx context.Context, u *models.User) error {
	return translateError(r.db.WithContext(ctx).Create(u).Error)
}

func (r *userRepository) Update(ctx context.Context, u *models.User) error {
	return translateError(r.db.WithContext(ctx).Omit("PasswordHash").Save(u).Error)
}

func (r *userRepository) FindByID(ctx context.Context, id uint) (*models.User, error) {
	var u models.User
	if err := r.db.WithContext(ctx).First(&u, id).Error; err != nil {
		return nil, translateError(err)
	}
	return &u, nil
}

func (r *userRepository) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	var u models.User
	err := r.db.WithContext(ctx).Where("LOWER(email) = ?", strings.ToLower(strings.TrimSpace(email))).First(&u).Error
	if err != nil {
		return nil, translateError(err)
	}
	return &u, nil
}

func (r *userRepository) List(ctx context.Context, search string, offset, limit int) ([]models.User, error) {
	var users []models.User
	q := r.db.WithContext(ctx).Model(&models.User{})
	if search != "" {
		like := "%" + strings.ToLower(search) + "%"
		q = q.Where("LOWER(name) LIKE ? OR LOWER(email) LIKE ? OR LOWER(business_name) LIKE ?", like, like, like)
	}
	err := q.Order("created_at DESC").Offset(offset).Limit(limit).Find(&users).Error
	return users, translateError(err)
}

func (r *userRepository) Count(ctx context.Context, search string) (int64, error) {
	var n int64
	q := r.db.WithContext(ctx).Model(&models.User{})
	if search != "" {
		like := "%" + strings.ToLower(search) + "%"
		q = q.Where("LOWER(name) LIKE ? OR LOWER(email) LIKE ? OR LOWER(business_name) LIKE ?", like, like, like)
	}
	err := q.Count(&n).Error
	return n, translateError(err)
}

func (r *userRepository) SetActive(ctx context.Context, id uint, active bool) error {
	res := r.db.WithContext(ctx).Model(&models.User{}).Where("id = ?", id).Update("active", active)
	if res.Error != nil {
		return translateError(res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *userRepository) SetPassword(ctx context.Context, id uint, hash string) error {
	res := r.db.WithContext(ctx).Model(&models.User{}).Where("id = ?", id).Update("password_hash", hash)
	if res.Error != nil {
		return translateError(res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *userRepository) CountAll(ctx context.Context) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&models.User{}).Count(&n).Error
	return n, translateError(err)
}
