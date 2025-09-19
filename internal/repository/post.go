package repository

import (
	"context"
	"fmt"

	"github.com/feed-system/feed-system/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PostRepository struct {
	db *gorm.DB
}

func NewPostRepository(db *gorm.DB) *PostRepository {
	return &PostRepository{db: db}
}

func (r *PostRepository) Create(ctx context.Context, post *models.Post) error {
	if err := r.db.WithContext(ctx).Create(post).Error; err != nil {
		return fmt.Errorf("failed to create post: %w", err)
	}
	return nil
}

func (r *PostRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Post, error) {
	var post models.Post
	if err := r.db.WithContext(ctx).
		Preload("User").
		First(&post, "id = ? AND is_deleted = ?", id, false).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get post: %w", err)
	}
	return &post, nil
}

func (r *PostRepository) GetByUserID(ctx context.Context, userID uuid.UUID, offset, limit int) ([]*models.Post, error) {
	var posts []*models.Post
	if err := r.db.WithContext(ctx).
		Preload("User").
		Where("user_id = ? AND is_deleted = ?", userID, false).
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&posts).Error; err != nil {
		return nil, fmt.Errorf("failed to get posts by user: %w", err)
	}
	return posts, nil
}

func (r *PostRepository) Update(ctx context.Context, post *models.Post) error {
	if err := r.db.WithContext(ctx).Save(post).Error; err != nil {
		return fmt.Errorf("failed to update post: %w", err)
	}
	return nil
}

func (r *PostRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if err := r.db.WithContext(ctx).
		Model(&models.Post{}).
		Where("id = ?", id).
		Update("is_deleted", true).Error; err != nil {
		return fmt.Errorf("failed to delete post: %w", err)
	}
	return nil
}

func (r *PostRepository) UpdateLikeCount(ctx context.Context, postID uuid.UUID, delta int64) error {
	if err := r.db.WithContext(ctx).Model(&models.Post{}).
		Where("id = ?", postID).
		UpdateColumn("like_count", gorm.Expr("like_count + ?", delta)).Error; err != nil {
		return fmt.Errorf("failed to update like count: %w", err)
	}
	return nil
}

func (r *PostRepository) UpdateCommentCount(ctx context.Context, postID uuid.UUID, delta int64) error {
	if err := r.db.WithContext(ctx).Model(&models.Post{}).
		Where("id = ?", postID).
		UpdateColumn("comment_count", gorm.Expr("comment_count + ?", delta)).Error; err != nil {
		return fmt.Errorf("failed to update comment count: %w", err)
	}
	return nil
}

func (r *PostRepository) UpdateShareCount(ctx context.Context, postID uuid.UUID, delta int64) error {
	if err := r.db.WithContext(ctx).Model(&models.Post{}).
		Where("id = ?", postID).
		UpdateColumn("share_count", gorm.Expr("share_count + ?", delta)).Error; err != nil {
		return fmt.Errorf("failed to update share count: %w", err)
	}
	return nil
}

func (r *PostRepository) Search(ctx context.Context, query string, offset, limit int) ([]*models.Post, error) {
	var posts []*models.Post
	db := r.db.WithContext(ctx).Preload("User").Where("is_deleted = ?", false)

	if query != "" {
		db = db.Where("content LIKE ?", "%"+query+"%")
	}

	if err := db.Order("created_at DESC").Offset(offset).Limit(limit).Find(&posts).Error; err != nil {
		return nil, fmt.Errorf("failed to search posts: %w", err)
	}
	return posts, nil
}