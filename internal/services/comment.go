package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/feed-system/feed-system/internal/models"
	"github.com/feed-system/feed-system/internal/repository"
	"github.com/feed-system/feed-system/pkg/logger"
	"github.com/feed-system/feed-system/pkg/queue"
	"github.com/google/uuid"
)

type CommentService struct {
	postRepo    *repository.PostRepository
	commentRepo *repository.CommentRepository
	userRepo    *repository.UserRepository
	producer    *queue.KafkaProducer
	logger      *logger.Logger
}

func NewCommentService(postRepo *repository.PostRepository, commentRepo *repository.CommentRepository, userRepo *repository.UserRepository, producer *queue.KafkaProducer, logger *logger.Logger) *CommentService {
	return &CommentService{
		postRepo:    postRepo,
		commentRepo: commentRepo,
		userRepo:    userRepo,
		producer:    producer,
		logger:      logger,
	}
}

type CreateCommentRequest struct {
	Content  string  `json:"content" binding:"required,min=1,max=500"`
	ParentID *string `json:"parent_id"`
}

func (s *CommentService) CreateComment(ctx context.Context, userID, postID string, req *CreateCommentRequest) (*models.Comment, error) {
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	postUUID, err := uuid.Parse(postID)
	if err != nil {
		return nil, fmt.Errorf("invalid post ID: %w", err)
	}

	// 检查用户是否存在
	user, err := s.userRepo.GetByID(ctx, userUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		return nil, errors.New("user not found")
	}

	// 检查帖子是否存在
	post, err := s.postRepo.GetByID(ctx, postUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get post: %w", err)
	}
	if post == nil {
		return nil, errors.New("post not found")
	}

	// 验证parent comment是否存在（如果是回复）
	var parentUUID *uuid.UUID
	if req.ParentID != nil {
		parentID, err := uuid.Parse(*req.ParentID)
		if err != nil {
			return nil, fmt.Errorf("invalid parent comment ID: %w", err)
		}

		parentComment, err := s.commentRepo.GetByID(ctx, parentID)
		if err != nil {
			return nil, fmt.Errorf("failed to get parent comment: %w", err)
		}
		if parentComment == nil {
			return nil, errors.New("parent comment not found")
		}

		if parentComment.PostID != postUUID {
			return nil, errors.New("parent comment does not belong to this post")
		}

		parentUUID = &parentID
	}

	// 创建评论
	comment := &models.Comment{
		UserID:    userUUID,
		PostID:    postUUID,
		Content:   req.Content,
		ParentID:  parentUUID,
		CreatedAt: post.CreatedAt,
	}

	if err := s.commentRepo.Create(ctx, comment); err != nil {
		return nil, fmt.Errorf("failed to create comment: %w", err)
	}

	// 更新帖子评论数
	if err := s.postRepo.UpdateCommentCount(ctx, postUUID, 1); err != nil {
		s.logger.WithError(err).Error("Failed to update post comment count")
	}

	// 发送评论创建事件
	event := queue.Event{
		Type:      queue.EventCommentCreated,
		Timestamp: comment.CreatedAt,
		Data: queue.CommentEventData{
			CommentID: comment.ID.String(),
			UserID:    userID,
			PostID:    postID,
			Content:   comment.Content,
		},
	}
	if err := s.producer.Publish(ctx, userID, event); err != nil {
		s.logger.WithError(err).Error("Failed to publish comment created event")
	}

	s.logger.WithFields(map[string]interface{}{
		"comment_id": comment.ID,
		"user_id":    userID,
		"post_id":    postID,
	}).Info("Comment created successfully")

	return comment, nil
}

func (s *CommentService) GetCommentByID(ctx context.Context, commentID string) (*models.Comment, error) {
	commentUUID, err := uuid.Parse(commentID)
	if err != nil {
		return nil, fmt.Errorf("invalid comment ID: %w", err)
	}

	comment, err := s.commentRepo.GetByID(ctx, commentUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get comment: %w", err)
	}
	if comment == nil {
		return nil, errors.New("comment not found")
	}

	return comment, nil
}

func (s *CommentService) GetPostComments(ctx context.Context, postID string, offset, limit int) ([]*models.Comment, error) {
	postUUID, err := uuid.Parse(postID)
	if err != nil {
		return nil, fmt.Errorf("invalid post ID: %w", err)
	}

	comments, err := s.commentRepo.GetByPostID(ctx, postUUID, offset, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get post comments: %w", err)
	}

	return comments, nil
}

func (s *CommentService) DeleteComment(ctx context.Context, userID, commentID string) error {
	commentUUID, err := uuid.Parse(commentID)
	if err != nil {
		return fmt.Errorf("invalid comment ID: %w", err)
	}

	comment, err := s.commentRepo.GetByID(ctx, commentUUID)
	if err != nil {
		return fmt.Errorf("failed to get comment: %w", err)
	}
	if comment == nil {
		return errors.New("comment not found")
	}

	// 检查权限
	if comment.UserID.String() != userID {
		return errors.New("permission denied")
	}

	// 删除评论
	if err := s.commentRepo.Delete(ctx, commentUUID); err != nil {
		return fmt.Errorf("failed to delete comment: %w", err)
	}

	// 更新帖子评论数
	if err := s.postRepo.UpdateCommentCount(ctx, comment.PostID, -1); err != nil {
		s.logger.WithError(err).Error("Failed to update post comment count")
	}

	s.logger.WithFields(map[string]interface{}{
		"comment_id": commentID,
		"user_id":    userID,
	}).Info("Comment deleted successfully")

	return nil
}

func (s *CommentService) GetCommentCount(ctx context.Context, postID string) (int64, error) {
	postUUID, err := uuid.Parse(postID)
	if err != nil {
		return 0, fmt.Errorf("invalid post ID: %w", err)
	}

	count, err := s.commentRepo.CountByPostID(ctx, postUUID)
	if err != nil {
		return 0, fmt.Errorf("failed to get comment count: %w", err)
	}

	return count, nil
}