package workers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/feed-system/feed-system/internal/models"
	"github.com/feed-system/feed-system/internal/repository"
	"github.com/feed-system/feed-system/internal/services"
	"github.com/feed-system/feed-system/pkg/cache"
	"github.com/feed-system/feed-system/pkg/logger"
	"github.com/feed-system/feed-system/pkg/queue"
	"github.com/google/uuid"
)

type FeedWorker struct {
	feedService  *services.FeedService
	userService  *services.UserService
	postRepo     *repository.PostRepository
	timelineRepo *repository.TimelineRepository
	followRepo   *repository.FollowRepository
	userRepo     *repository.UserRepository
	cache        *cache.RedisClient
	consumer     *queue.KafkaConsumer
	logger       *logger.Logger
}

func NewFeedWorker(
	feedService *services.FeedService,
	userService *services.UserService,
	postRepo *repository.PostRepository,
	timelineRepo *repository.TimelineRepository,
	followRepo *repository.FollowRepository,
	userRepo *repository.UserRepository,
	cache *cache.RedisClient,
	consumer *queue.KafkaConsumer,
	logger *logger.Logger,
) *FeedWorker {
	return &FeedWorker{
		feedService:  feedService,
		userService:  userService,
		postRepo:     postRepo,
		timelineRepo: timelineRepo,
		followRepo:   followRepo,
		userRepo:     userRepo,
		cache:        cache,
		consumer:     consumer,
		logger:       logger,
	}
}

func (w *FeedWorker) Start(ctx context.Context) error {
	w.logger.Info("Starting feed worker...")

	return w.consumer.Subscribe(ctx, func(msg queue.Message) error {
		var event queue.Event
		data, err := json.Marshal(msg.Value)
		if err != nil {
			return fmt.Errorf("failed to marshal message value: %w", err)
		}

		if err := json.Unmarshal(data, &event); err != nil {
			return fmt.Errorf("failed to unmarshal event: %w", err)
		}

		w.logger.WithFields(map[string]interface{}{
			"event_type": event.Type,
			"timestamp":  event.Timestamp,
		}).Info("Processing event")

		switch event.Type {
		case queue.EventPostCreated:
			return w.handlePostCreated(ctx, event)
		case queue.EventPostDeleted:
			return w.handlePostDeleted(ctx, event)
		case queue.EventFollowCreated:
			return w.handleFollowCreated(ctx, event)
		case queue.EventFollowDeleted:
			return w.handleFollowDeleted(ctx, event)
		case queue.EventLikeCreated:
			return w.handleLikeCreated(ctx, event)
		case queue.EventLikeDeleted:
			return w.handleLikeDeleted(ctx, event)
		case queue.EventCommentCreated:
			return w.handleCommentCreated(ctx, event)
		default:
			w.logger.WithField("event_type", event.Type).Warn("Unknown event type")
			return nil
		}
	})
}

func (w *FeedWorker) handlePostCreated(ctx context.Context, event queue.Event) error {
	data, ok := event.Data.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid post created event data")
	}

	postID, ok := data["post_id"].(string)
	if !ok {
		return fmt.Errorf("missing post_id in event data")
	}

	userID, ok := data["user_id"].(string)
	if !ok {
		return fmt.Errorf("missing user_id in event data")
	}

	w.logger.WithFields(map[string]interface{}{
		"post_id": postID,
		"user_id": userID,
	}).Info("Handling post created event")

	// 清除相关缓存
	if err := w.clearUserFeedCache(ctx, userID); err != nil {
		w.logger.WithError(err).Error("Failed to clear user feed cache")
	}

	return nil
}

func (w *FeedWorker) handlePostDeleted(ctx context.Context, event queue.Event) error {
	data, ok := event.Data.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid post deleted event data")
	}

	postID, ok := data["post_id"].(string)
	if !ok {
		return fmt.Errorf("missing post_id in event data")
	}

	userID, ok := data["user_id"].(string)
	if !ok {
		return fmt.Errorf("missing user_id in event data")
	}

	postUUID, err := uuid.Parse(postID)
	if err != nil {
		return fmt.Errorf("invalid post ID: %w", err)
	}

	w.logger.WithFields(map[string]interface{}{
		"post_id": postID,
		"user_id": userID,
	}).Info("Handling post deleted event")

	// 从所有timeline中删除该帖子
	if err := w.timelineRepo.DeleteByPostID(ctx, postUUID); err != nil {
		return fmt.Errorf("failed to delete timeline entries: %w", err)
	}

	// 清除相关缓存
	if err := w.clearUserFeedCache(ctx, userID); err != nil {
		w.logger.WithError(err).Error("Failed to clear user feed cache")
	}

	return nil
}

func (w *FeedWorker) handleFollowCreated(ctx context.Context, event queue.Event) error {
	data, ok := event.Data.(queue.FollowEventData)
	if !ok {
		// 尝试解析为map
		mapData, mapOk := event.Data.(map[string]interface{})
		if !mapOk {
			return fmt.Errorf("invalid follow created event data")
		}

		followerID, ok1 := mapData["follower_id"].(string)
		followingID, ok2 := mapData["following_id"].(string)
		if !ok1 || !ok2 {
			return fmt.Errorf("missing follower_id or following_id in event data")
		}

		data = queue.FollowEventData{
			FollowerID:  followerID,
			FollowingID: followingID,
		}
	}

	w.logger.WithFields(map[string]interface{}{
		"follower_id":  data.FollowerID,
		"following_id": data.FollowingID,
	}).Info("Handling follow created event")

	followerUUID, err := uuid.Parse(data.FollowerID)
	if err != nil {
		return fmt.Errorf("invalid follower ID: %w", err)
	}

	followingUUID, err := uuid.Parse(data.FollowingID)
	if err != nil {
		return fmt.Errorf("invalid following ID: %w", err)
	}

	// 获取被关注者的最新帖子
	posts, err := w.postRepo.GetByUserID(ctx, followingUUID, 0, 10)
	if err != nil {
		return fmt.Errorf("failed to get following's posts: %w", err)
	}

	// 将帖子添加到关注者的timeline
	var timelines []*models.Timeline
	for _, post := range posts {
		timeline := &models.Timeline{
			UserID:    followerUUID,
			PostID:    post.ID,
			Score:     post.Score,
			CreatedAt: post.CreatedAt,
		}
		timelines = append(timelines, timeline)
	}

	if len(timelines) > 0 {
		if err := w.timelineRepo.CreateBatch(ctx, timelines); err != nil {
			return fmt.Errorf("failed to create timelines: %w", err)
		}
	}

	// 清除关注者的feed缓存
	if err := w.clearUserFeedCache(ctx, data.FollowerID); err != nil {
		w.logger.WithError(err).Error("Failed to clear follower feed cache")
	}

	return nil
}

func (w *FeedWorker) handleFollowDeleted(ctx context.Context, event queue.Event) error {
	data, ok := event.Data.(queue.FollowEventData)
	if !ok {
		// 尝试解析为map
		mapData, mapOk := event.Data.(map[string]interface{})
		if !mapOk {
			return fmt.Errorf("invalid follow deleted event data")
		}

		followerID, ok1 := mapData["follower_id"].(string)
		followingID, ok2 := mapData["following_id"].(string)
		if !ok1 || !ok2 {
			return fmt.Errorf("missing follower_id or following_id in event data")
		}

		data = queue.FollowEventData{
			FollowerID:  followerID,
			FollowingID: followingID,
		}
	}

	w.logger.WithFields(map[string]interface{}{
		"follower_id":  data.FollowerID,
		"following_id": data.FollowingID,
	}).Info("Handling follow deleted event")

	followerUUID, err := uuid.Parse(data.FollowerID)
	if err != nil {
		return fmt.Errorf("invalid follower ID: %w", err)
	}

	followingUUID, err := uuid.Parse(data.FollowingID)
	if err != nil {
		return fmt.Errorf("invalid following ID: %w", err)
	}

	// 获取被关注者的帖子
	posts, err := w.postRepo.GetByUserID(ctx, followingUUID, 0, 100)
	if err != nil {
		return fmt.Errorf("failed to get following's posts: %w", err)
	}

	// 从关注者的timeline中删除这些帖子
	for _, post := range posts {
		if err := w.timelineRepo.DeleteByUserIDAndPostID(ctx, followerUUID, post.ID); err != nil {
			w.logger.WithError(err).WithFields(map[string]interface{}{
				"user_id": data.FollowerID,
				"post_id": post.ID,
			}).Error("Failed to delete timeline entry")
		}
	}

	// 清除关注者的feed缓存
	if err := w.clearUserFeedCache(ctx, data.FollowerID); err != nil {
		w.logger.WithError(err).Error("Failed to clear follower feed cache")
	}

	return nil
}

func (w *FeedWorker) handleLikeCreated(ctx context.Context, event queue.Event) error {
	data, ok := event.Data.(queue.LikeEventData)
	if !ok {
		// 尝试解析为map
		mapData, mapOk := event.Data.(map[string]interface{})
		if !mapOk {
			return fmt.Errorf("invalid like created event data")
		}

		userID, ok1 := mapData["user_id"].(string)
		postID, ok2 := mapData["post_id"].(string)
		if !ok1 || !ok2 {
			return fmt.Errorf("missing user_id or post_id in event data")
		}

		data = queue.LikeEventData{
			UserID: userID,
			PostID: postID,
		}
	}

	w.logger.WithFields(map[string]interface{}{
		"user_id": data.UserID,
		"post_id": data.PostID,
	}).Info("Handling like created event")

	// 这里可以添加点赞相关的处理逻辑，比如更新帖子的热度分数
	// 清除相关缓存
	if err := w.clearPostCache(ctx, data.PostID); err != nil {
		w.logger.WithError(err).Error("Failed to clear post cache")
	}

	return nil
}

func (w *FeedWorker) handleLikeDeleted(ctx context.Context, event queue.Event) error {
	data, ok := event.Data.(queue.LikeEventData)
	if !ok {
		// 尝试解析为map
		mapData, mapOk := event.Data.(map[string]interface{})
		if !mapOk {
			return fmt.Errorf("invalid like deleted event data")
		}

		userID, ok1 := mapData["user_id"].(string)
		postID, ok2 := mapData["post_id"].(string)
		if !ok1 || !ok2 {
			return fmt.Errorf("missing user_id or post_id in event data")
		}

		data = queue.LikeEventData{
			UserID: userID,
			PostID: postID,
		}
	}

	w.logger.WithFields(map[string]interface{}{
		"user_id": data.UserID,
		"post_id": data.PostID,
	}).Info("Handling like deleted event")

	// 清除相关缓存
	if err := w.clearPostCache(ctx, data.PostID); err != nil {
		w.logger.WithError(err).Error("Failed to clear post cache")
	}

	return nil
}

func (w *FeedWorker) handleCommentCreated(ctx context.Context, event queue.Event) error {
	data, ok := event.Data.(queue.CommentEventData)
	if !ok {
		// 尝试解析为map
		mapData, mapOk := event.Data.(map[string]interface{})
		if !mapOk {
			return fmt.Errorf("invalid comment created event data")
		}

		userID, ok1 := mapData["user_id"].(string)
		postID, ok2 := mapData["post_id"].(string)
		if !ok1 || !ok2 {
			return fmt.Errorf("missing user_id or post_id in event data")
		}

		data = queue.CommentEventData{
			UserID: userID,
			PostID: postID,
		}
	}

	w.logger.WithFields(map[string]interface{}{
		"user_id": data.UserID,
		"post_id": data.PostID,
	}).Info("Handling comment created event")

	// 清除相关缓存
	if err := w.clearPostCache(ctx, data.PostID); err != nil {
		w.logger.WithError(err).Error("Failed to clear post cache")
	}

	return nil
}

func (w *FeedWorker) clearUserFeedCache(ctx context.Context, userID string) error {
	// 清除用户的feed缓存
	pattern := fmt.Sprintf("feed:%s:*", userID)
	// 这里需要实现pattern匹配删除
	w.logger.WithField("pattern", pattern).Info("Clearing user feed cache")
	return nil
}

func (w *FeedWorker) clearPostCache(ctx context.Context, postID string) error {
	// 清除帖子相关的缓存
	cacheKey := fmt.Sprintf("post:%s", postID)
	if err := w.cache.Delete(ctx, cacheKey); err != nil {
		return fmt.Errorf("failed to delete post cache: %w", err)
	}
	return nil
}

func (w *FeedWorker) Stop() error {
	w.logger.Info("Stopping feed worker...")
	return w.consumer.Close()
}