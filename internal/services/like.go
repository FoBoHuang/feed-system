package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/feed-system/feed-system/internal/models"
	"github.com/feed-system/feed-system/internal/repository"
	"github.com/feed-system/feed-system/pkg/logger"
	"github.com/feed-system/feed-system/pkg/queue"
	"github.com/google/uuid"
)

type LikeService struct {
	postRepo  *repository.PostRepository
	likeRepo  *repository.LikeRepository
	userRepo  *repository.UserRepository
	producer  *queue.KafkaProducer
	logger    *logger.Logger
}

func NewLikeService(postRepo *repository.PostRepository, likeRepo *repository.LikeRepository, userRepo *repository.UserRepository, producer *queue.KafkaProducer, logger *logger.Logger) *LikeService {
	return &LikeService{
		postRepo: postRepo,
		likeRepo: likeRepo,
		userRepo: userRepo,
		producer: producer,
		logger:   logger,
	}
}

func (s *LikeService) LikePost(ctx context.Context, userID, postID string) error {
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("invalid user ID: %w", err)
	}

	postUUID, err := uuid.Parse(postID)
	if err != nil {
		return fmt.Errorf("invalid post ID: %w", err)
	}

	// 检查用户是否存在
	user, err := s.userRepo.GetByID(ctx, userUUID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		return errors.New("user not found")
	}

	// 检查帖子是否存在
	post, err := s.postRepo.GetByID(ctx, postUUID)
	if err != nil {
		return fmt.Errorf("failed to get post: %w", err)
	}
	if post == nil {
		return errors.New("post not found")
	}

	// 检查是否已经点赞
	existingLike, err := s.likeRepo.Get(ctx, userUUID, postUUID)
	if err != nil {
		return fmt.Errorf("failed to check like status: %w", err)
	}
	if existingLike != nil {
		return errors.New("already liked")
	}

	// 创建点赞记录
	like := &models.Like{
		UserID:    userUUID,
		PostID:    postUUID,
		CreatedAt: post.CreatedAt,
	}

	if err := s.likeRepo.Create(ctx, like); err != nil {
		return fmt.Errorf("failed to create like: %w", err)
	}

	// 更新帖子点赞数
	if err := s.postRepo.UpdateLikeCount(ctx, postUUID, 1); err != nil {
		s.logger.WithError(err).Error("Failed to update post like count")
	}

	// 发送点赞事件
	event := queue.Event{
		Type:      queue.EventLikeCreated,
		Timestamp: like.CreatedAt,
		Data: queue.LikeEventData{
			UserID: userID,
			PostID: postID,
		},
	}
	if err := s.producer.Publish(ctx, userID, event); err != nil {
		s.logger.WithError(err).Error("Failed to publish like created event")
	}

	s.logger.WithFields(map[string]interface{}{
		"user_id": userID,
		"post_id": postID,
	}).Info("Post liked successfully")

	return nil
}

func (s *LikeService) UnlikePost(ctx context.Context, userID, postID string) error {
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("invalid user ID: %w", err)
	}

	postUUID, err := uuid.Parse(postID)
	if err != nil {
		return fmt.Errorf("invalid post ID: %w", err)
	}

	// 检查点赞记录是否存在
	existingLike, err := s.likeRepo.Get(ctx, userUUID, postUUID)
	if err != nil {
		return fmt.Errorf("failed to check like status: %w", err)
	}
	if existingLike == nil {
		return errors.New("not liked")
	}

	// 删除点赞记录
	if err := s.likeRepo.Delete(ctx, userUUID, postUUID); err != nil {
		return fmt.Errorf("failed to delete like: %w", err)
	}

	// 更新帖子点赞数
	if err := s.postRepo.UpdateLikeCount(ctx, postUUID, -1); err != nil {
		s.logger.WithError(err).Error("Failed to update post like count")
	}

	// 发送取消点赞事件
	event := queue.Event{
		Type:      queue.EventLikeDeleted,
		Timestamp: time.Now(),
		Data: queue.LikeEventData{
			UserID: userID,
			PostID: postID,
		},
	}
	if err := s.producer.Publish(ctx, userID, event); err != nil {
		s.logger.WithError(err).Error("Failed to publish like deleted event")
	}

	s.logger.WithFields(map[string]interface{}{
		"user_id": userID,
		"post_id": postID,
	}).Info("Post unliked successfully")

	return nil
}

func (s *LikeService) GetPostLikes(ctx context.Context, postID string, offset, limit int) ([]*models.Like, error) {
	postUUID, err := uuid.Parse(postID)
	if err != nil {
		return nil, fmt.Errorf("invalid post ID: %w", err)
	}

	likes, err := s.likeRepo.GetByPostID(ctx, postUUID, offset, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get post likes: %w", err)
	}

	return likes, nil
}

func (s *LikeService) IsLiked(ctx context.Context, userID, postID string) (bool, error) {
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return false, fmt.Errorf("invalid user ID: %w", err)
	}

	postUUID, err := uuid.Parse(postID)
	if err != nil {
		return false, fmt.Errorf("invalid post ID: %w", err)
	}

	return s.likeRepo.IsLiked(ctx, userUUID, postUUID)
}

func (s *LikeService) GetLikeCount(ctx context.Context, postID string) (int64, error) {
	postUUID, err := uuid.Parse(postID)
	if err != nil {
		return 0, fmt.Errorf("invalid post ID: %w", err)
	}

	count, err := s.likeRepo.CountByPostID(ctx, postUUID)
	if err != nil {
		return 0, fmt.Errorf("failed to get like count: %w", err)
	}

	return count, nil
}