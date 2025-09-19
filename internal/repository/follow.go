package repository

import (
	"context"
	"fmt"

	"github.com/feed-system/feed-system/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type FollowRepository struct {
	db *gorm.DB
}

func NewFollowRepository(db *gorm.DB) *FollowRepository {
	return &FollowRepository{db: db}
}

func (r *FollowRepository) Create(ctx context.Context, follow *models.Follow) error {
	if err := r.db.WithContext(ctx).Create(follow).Error; err != nil {
		return fmt.Errorf("failed to create follow: %w", err)
	}
	return nil
}

func (r *FollowRepository) Delete(ctx context.Context, followerID, followingID uuid.UUID) error {
	if err := r.db.WithContext(ctx).
		Where("follower_id = ? AND following_id = ?", followerID, followingID).
		Delete(&models.Follow{}).Error; err != nil {
		return fmt.Errorf("failed to delete follow: %w", err)
	}
	return nil
}

func (r *FollowRepository) Get(ctx context.Context, followerID, followingID uuid.UUID) (*models.Follow, error) {
	var follow models.Follow
	if err := r.db.WithContext(ctx).
		Where("follower_id = ? AND following_id = ?", followerID, followingID).
		First(&follow).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get follow: %w", err)
	}
	return &follow, nil
}

func (r *FollowRepository) GetFollowers(ctx context.Context, userID uuid.UUID, offset, limit int) ([]*models.User, error) {
	var users []*models.User
	if err := r.db.WithContext(ctx).
		Table("users").
		Joins("JOIN follows ON follows.follower_id = users.id").
		Where("follows.following_id = ?", userID).
		Offset(offset).
		Limit(limit).
		Find(&users).Error; err != nil {
		return nil, fmt.Errorf("failed to get followers: %w", err)
	}
	return users, nil
}

func (r *FollowRepository) GetFollowing(ctx context.Context, userID uuid.UUID, offset, limit int) ([]*models.User, error) {
	var users []*models.User
	if err := r.db.WithContext(ctx).
		Table("users").
		Joins("JOIN follows ON follows.following_id = users.id").
		Where("follows.follower_id = ?", userID).
		Offset(offset).
		Limit(limit).
		Find(&users).Error; err != nil {
		return nil, fmt.Errorf("failed to get following: %w", err)
	}
	return users, nil
}

func (r *FollowRepository) CountFollowers(ctx context.Context, userID uuid.UUID) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&models.Follow{}).
		Where("following_id = ?", userID).
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("failed to count followers: %w", err)
	}
	return count, nil
}

func (r *FollowRepository) CountFollowing(ctx context.Context, userID uuid.UUID) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&models.Follow{}).
		Where("follower_id = ?", userID).
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("failed to count following: %w", err)
	}
	return count, nil
}

func (r *FollowRepository) IsFollowing(ctx context.Context, followerID, followingID uuid.UUID) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&models.Follow{}).
		Where("follower_id = ? AND following_id = ?", followerID, followingID).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("failed to check follow status: %w", err)
	}
	return count > 0, nil
}