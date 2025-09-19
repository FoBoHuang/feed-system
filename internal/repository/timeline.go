package repository

import (
	"context"
	"fmt"

	"github.com/feed-system/feed-system/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type TimelineRepository struct {
	db *gorm.DB
}

func NewTimelineRepository(db *gorm.DB) *TimelineRepository {
	return &TimelineRepository{db: db}
}

func (r *TimelineRepository) Create(ctx context.Context, timeline *models.Timeline) error {
	if err := r.db.WithContext(ctx).Create(timeline).Error; err != nil {
		return fmt.Errorf("failed to create timeline: %w", err)
	}
	return nil
}

func (r *TimelineRepository) CreateBatch(ctx context.Context, timelines []*models.Timeline) error {
	if err := r.db.WithContext(ctx).CreateInBatches(timelines, 100).Error; err != nil {
		return fmt.Errorf("failed to create timelines in batch: %w", err)
	}
	return nil
}

func (r *TimelineRepository) GetByUserID(ctx context.Context, userID uuid.UUID, offset, limit int) ([]*models.Timeline, error) {
	var timelines []*models.Timeline
	if err := r.db.WithContext(ctx).
		Preload("Post.User").
		Where("user_id = ?", userID).
		Order("score DESC, created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&timelines).Error; err != nil {
		return nil, fmt.Errorf("failed to get timeline: %w", err)
	}
	return timelines, nil
}

func (r *TimelineRepository) DeleteByPostID(ctx context.Context, postID uuid.UUID) error {
	if err := r.db.WithContext(ctx).
		Where("post_id = ?", postID).
		Delete(&models.Timeline{}).Error; err != nil {
		return fmt.Errorf("failed to delete timeline by post ID: %w", err)
	}
	return nil
}

func (r *TimelineRepository) DeleteByUserIDAndPostID(ctx context.Context, userID, postID uuid.UUID) error {
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND post_id = ?", userID, postID).
		Delete(&models.Timeline{}).Error; err != nil {
		return fmt.Errorf("failed to delete timeline by user ID and post ID: %w", err)
	}
	return nil
}

func (r *TimelineRepository) CountByUserID(ctx context.Context, userID uuid.UUID) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&models.Timeline{}).
		Where("user_id = ?", userID).
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("failed to count timeline: %w", err)
	}
	return count, nil
}