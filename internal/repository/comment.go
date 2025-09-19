package repository

import (
	"context"
	"fmt"

	"github.com/feed-system/feed-system/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type CommentRepository struct {
	db *gorm.DB
}

func NewCommentRepository(db *gorm.DB) *CommentRepository {
	return &CommentRepository{db: db}
}

func (r *CommentRepository) Create(ctx context.Context, comment *models.Comment) error {
	if err := r.db.WithContext(ctx).Create(comment).Error; err != nil {
		return fmt.Errorf("failed to create comment: %w", err)
	}
	return nil
}

func (r *CommentRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Comment, error) {
	var comment models.Comment
	if err := r.db.WithContext(ctx).
		Preload("User").
		Preload("Post").
		First(&comment, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get comment: %w", err)
	}
	return &comment, nil
}

func (r *CommentRepository) GetByPostID(ctx context.Context, postID uuid.UUID, offset, limit int) ([]*models.Comment, error) {
	var comments []*models.Comment
	if err := r.db.WithContext(ctx).
		Preload("User").
		Where("post_id = ?", postID).
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&comments).Error; err != nil {
		return nil, fmt.Errorf("failed to get comments by post: %w", err)
	}
	return comments, nil
}

func (r *CommentRepository) Update(ctx context.Context, comment *models.Comment) error {
	if err := r.db.WithContext(ctx).Save(comment).Error; err != nil {
		return fmt.Errorf("failed to update comment: %w", err)
	}
	return nil
}

func (r *CommentRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if err := r.db.WithContext(ctx).
		Delete(&models.Comment{}, "id = ?", id).Error; err != nil {
		return fmt.Errorf("failed to delete comment: %w", err)
	}
	return nil
}

func (r *CommentRepository) UpdateLikeCount(ctx context.Context, commentID uuid.UUID, delta int64) error {
	if err := r.db.WithContext(ctx).Model(&models.Comment{}).
		Where("id = ?", commentID).
		UpdateColumn("like_count", gorm.Expr("like_count + ?", delta)).Error; err != nil {
		return fmt.Errorf("failed to update comment like count: %w", err)
	}
	return nil
}

func (r *CommentRepository) CountByPostID(ctx context.Context, postID uuid.UUID) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&models.Comment{}).
		Where("post_id = ?", postID).
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("failed to count comments: %w", err)
	}
	return count, nil
}