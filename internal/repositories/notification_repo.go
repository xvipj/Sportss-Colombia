package repositories

import (
	"context"

	"gorm.io/gorm"

	"sportss/internal/models"
)

type notificationRepository struct{ db *gorm.DB }

func NewNotificationRepository(db *gorm.DB) NotificationRepository {
	return &notificationRepository{db: db}
}

func (r *notificationRepository) Create(ctx context.Context, n *models.Notification) error {
	return translateError(r.db.WithContext(ctx).Create(n).Error)
}

func (r *notificationRepository) ListByUser(ctx context.Context, userID uint, limit int) ([]models.Notification, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	var notifications []models.Notification
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).
		Order("created_at DESC").Limit(limit).Find(&notifications).Error
	return notifications, translateError(err)
}

func (r *notificationRepository) CountUnread(ctx context.Context, userID uint) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&models.Notification{}).
		Where("user_id = ? AND read = ?", userID, false).Count(&n).Error
	return n, translateError(err)
}

func (r *notificationRepository) MarkRead(ctx context.Context, id, userID uint) error {
	res := r.db.WithContext(ctx).Model(&models.Notification{}).
		Where("id = ? AND user_id = ?", id, userID).Update("read", true)
	if res.Error != nil {
		return translateError(res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *notificationRepository) MarkAllRead(ctx context.Context, userID uint) error {
	return translateError(r.db.WithContext(ctx).Model(&models.Notification{}).
		Where("user_id = ? AND read = ?", userID, false).Update("read", true).Error)
}
