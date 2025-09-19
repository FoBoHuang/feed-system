package repository

import (
	"context"
	"fmt"

	"github.com/feed-system/feed-system/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type LikeRepository struct {
	db *gorm.DB
}

func NewLikeRepository(db *gorm.DB) *LikeRepository {
	return &LikeRepository{db: db}
}

func (r *LikeRepository) Create(ctx context.Context, like *models.Like) error {
	if err := r.db.WithContext(ctx).Create(like).Error; err != nil {
		return fmt.Errorf("failed to create like: %w", err)
	}
	return nil
}

func (r *LikeRepository) Delete(ctx context.Context, userID, postID uuid.UUID) error {
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND post_id = ?", userID, postID).
		Delete(&models.Like{}).Error; err != nil {
		return fmt.Errorf("failed to delete like: %w", err)
	}
	return nil
}

func (r *LikeRepository) Get(ctx context.Context, userID, postID uuid.UUID) (*models.Like, error) {
	var like models.Like
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND post_id = ?", userID, postID).
		First(&like).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get like: %w", err)
	}
	return &like, nil
}

func (r *LikeRepository) GetByPostID(ctx context.Context, postID uuid.UUID, offset, limit int) ([]*models.Like, error) {
	var likes []*models.Like
	if err := r.db.WithContext(ctx).
		Preload("User").
		Where("post_id = ?", postID).
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&likes).Error; err != nil {
		return nil, fmt.Errorf("failed to get likes by post: %w", err)
	}
	return likes, nil
}

func (r *LikeRepository) CountByPostID(ctx context.Context, postID uuid.UUID) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&models.Like{}).
		Where("post_id = ?", postID).
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("failed to count likes: %w", err)
	}
	return count, nil
}

func (r *LikeRepository) IsLiked(ctx context.Context, userID, postID uuid.UUID) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&models.Like{}).
		Where("user_id = ? AND post_id = ?", userID, postID).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("failed to check like status: %w", err)
	}
	return count > 0, nil
}