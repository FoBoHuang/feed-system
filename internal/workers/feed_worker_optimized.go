package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/feed-system/feed-system/internal/config"
	"github.com/feed-system/feed-system/internal/services"
	"github.com/feed-system/feed-system/pkg/logger"
	"github.com/feed-system/feed-system/pkg/queue"
)

// OptimizedFeedWorker 优化版的Feed Worker
type OptimizedFeedWorker struct {
	consumer *queue.KafkaConsumer
	logger   *logger.Logger
	config   *config.Config

	// 优化服务
	activityService      *services.ActivityService
	timelineCacheService *services.TimelineCacheService
	cacheStrategyService *services.CacheStrategyService
	recoveryService      *services.RecoveryService
	optimizedFeedService *services.OptimizedFeedService
}

func NewOptimizedFeedWorker(
	consumer *queue.KafkaConsumer,
	logger *logger.Logger,
	config *config.Config,
	activityService *services.ActivityService,
	timelineCacheService *services.TimelineCacheService,
	cacheStrategyService *services.CacheStrategyService,
	recoveryService *services.RecoveryService,
	optimizedFeedService *services.OptimizedFeedService,
) *OptimizedFeedWorker {
	return &OptimizedFeedWorker{
		consumer:             consumer,
		logger:               logger,
		config:               config,
		activityService:      activityService,
		timelineCacheService: timelineCacheService,
		cacheStrategyService: cacheStrategyService,
		recoveryService:      recoveryService,
		optimizedFeedService: optimizedFeedService,
	}
}

// Start 启动优化版Worker
func (w *OptimizedFeedWorker) Start(ctx context.Context) error {
	w.logger.Info("Starting optimized feed worker")

	// 启动后台任务
	go w.startBackgroundJobs(ctx)

	// 处理消息
	return w.consumer.Subscribe(ctx, w.handleMessage)
}

// startBackgroundJobs 启动后台任务
func (w *OptimizedFeedWorker) startBackgroundJobs(ctx context.Context) {
	// 启动缓存清理任务（每小时执行一次）
	go w.cacheStrategyService.StartCacheCleanupJob(ctx, 1*time.Hour)

	// 启动崩溃恢复任务（每5分钟执行一次）
	go w.recoveryService.StartRecoveryJob(ctx, 5*time.Minute)

	// 启动Timeline清理任务（每天执行一次）
	go w.startTimelineCleanupJob(ctx)

	// 启动用户活跃度衰减任务（每天执行一次）
	go w.startActivityDecayJob(ctx)

	w.logger.Info("Background jobs started")
}

// handleMessage 处理消息
func (w *OptimizedFeedWorker) handleMessage(message queue.Message) error {
	ctx := context.Background()
	var event queue.Event
	if messageBytes, ok := message.Value.([]byte); ok {
		if err := json.Unmarshal(messageBytes, &event); err != nil {
			w.logger.WithError(err).Error("Failed to unmarshal event")
			return err
		}
	} else if err := json.Unmarshal([]byte(fmt.Sprintf("%v", message.Value)), &event); err != nil {
		w.logger.WithError(err).Error("Failed to unmarshal event")
		return err
	}

	w.logger.WithFields(map[string]interface{}{
		"event_type": event.Type,
		"topic":      message.Topic,
	}).Info("Processing event")

	switch event.Type {
	case queue.EventPostCreated:
		return w.handlePostCreated(ctx, event)
	case queue.EventPostDeleted:
		return w.handlePostDeleted(ctx, event)
	case queue.EventFollowCreated:
		return w.handleUserFollowed(ctx, event)
	case queue.EventFollowDeleted:
		return w.handleUserUnfollowed(ctx, event)
	case "post_distribution_completed":
		return w.handlePostDistributionCompleted(ctx, event)
	case "user_activity_updated":
		return w.handleUserActivityUpdated(ctx, event)
	default:
		w.logger.WithField("event_type", event.Type).Warn("Unknown event type")
		return nil
	}
}

// handlePostCreated 处理帖子创建事件
func (w *OptimizedFeedWorker) handlePostCreated(ctx context.Context, event queue.Event) error {
	data, ok := event.Data.(queue.PostEventData)
	if !ok {
		return fmt.Errorf("invalid post event data")
	}

	w.logger.WithFields(map[string]interface{}{
		"post_id": data.PostID,
		"user_id": data.UserID,
	}).Info("Handling post created event")

	// 这里可以执行一些后续处理，比如：
	// 1. 更新用户活跃度
	// 2. 触发推荐算法更新
	// 3. 发送通知等

	return nil
}

// handlePostDeleted 处理帖子删除事件
func (w *OptimizedFeedWorker) handlePostDeleted(ctx context.Context, event queue.Event) error {
	data, ok := event.Data.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid post deleted event data")
	}

	postID, ok := data["post_id"].(string)
	if !ok {
		return fmt.Errorf("missing post_id in event data")
	}

	w.logger.WithField("post_id", postID).Info("Handling post deleted event")

	// 从所有Timeline缓存中删除该帖子
	// 这里需要扫描所有timeline:*的key并删除对应的帖子
	// 实际实现可能需要维护一个反向索引

	return nil
}

// handleUserFollowed 处理用户关注事件
func (w *OptimizedFeedWorker) handleUserFollowed(ctx context.Context, event queue.Event) error {
	data, ok := event.Data.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid user followed event data")
	}

	followerID, ok := data["follower_id"].(string)
	if !ok {
		return fmt.Errorf("missing follower_id in event data")
	}

	followingID, ok := data["following_id"].(string)
	if !ok {
		return fmt.Errorf("missing following_id in event data")
	}

	w.logger.WithFields(map[string]interface{}{
		"follower_id":  followerID,
		"following_id": followingID,
	}).Info("Handling user followed event")

	// 用户关注后，可能需要：
	// 1. 清除关注者的Timeline缓存，让其重新构建
	// 2. 如果被关注者是普通用户，将其最近的帖子推送给关注者
	// 3. 更新缓存策略

	return nil
}

// handleUserUnfollowed 处理用户取消关注事件
func (w *OptimizedFeedWorker) handleUserUnfollowed(ctx context.Context, event queue.Event) error {
	data, ok := event.Data.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid user unfollowed event data")
	}

	followerID, ok := data["follower_id"].(string)
	if !ok {
		return fmt.Errorf("missing follower_id in event data")
	}

	followingID, ok := data["following_id"].(string)
	if !ok {
		return fmt.Errorf("missing following_id in event data")
	}

	w.logger.WithFields(map[string]interface{}{
		"follower_id":  followerID,
		"following_id": followingID,
	}).Info("Handling user unfollowed event")

	// 用户取消关注后，需要：
	// 1. 从关注者的Timeline缓存中删除被取消关注用户的帖子
	// 2. 重新构建Timeline

	return nil
}

// handlePostDistributionCompleted 处理帖子分发完成事件
func (w *OptimizedFeedWorker) handlePostDistributionCompleted(ctx context.Context, event queue.Event) error {
	data, ok := event.Data.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid distribution completed event data")
	}

	postID, ok := data["post_id"].(string)
	if !ok {
		return fmt.Errorf("missing post_id in event data")
	}

	distributionType, ok := data["distribution_type"].(string)
	if !ok {
		distributionType = "unknown"
	}

	w.logger.WithFields(map[string]interface{}{
		"post_id":           postID,
		"distribution_type": distributionType,
	}).Info("Handling post distribution completed event")

	// 分发完成后的处理：
	// 1. 更新分发状态
	// 2. 记录统计信息
	// 3. 清理临时数据

	return nil
}

// handleUserActivityUpdated 处理用户活跃度更新事件
func (w *OptimizedFeedWorker) handleUserActivityUpdated(ctx context.Context, event queue.Event) error {
	data, ok := event.Data.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid user activity event data")
	}

	userID, ok := data["user_id"].(string)
	if !ok {
		return fmt.Errorf("missing user_id in event data")
	}

	w.logger.WithField("user_id", userID).Info("Handling user activity updated event")

	// 用户活跃度更新后：
	// 1. 重新评估缓存策略
	// 2. 调整Timeline缓存大小和过期时间

	return nil
}

// startTimelineCleanupJob 启动Timeline清理任务
func (w *OptimizedFeedWorker) startTimelineCleanupJob(ctx context.Context) {
	ticker := time.NewTicker(24 * time.Hour) // 每天执行一次
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("Timeline cleanup job stopped")
			return
		case <-ticker.C:
			if err := w.timelineCacheService.CleanupExpiredTimelines(ctx); err != nil {
				w.logger.WithError(err).Error("Timeline cleanup job failed")
			}
		}
	}
}

// startActivityDecayJob 启动用户活跃度衰减任务
func (w *OptimizedFeedWorker) startActivityDecayJob(ctx context.Context) {
	ticker := time.NewTicker(24 * time.Hour) // 每天执行一次
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("Activity decay job stopped")
			return
		case <-ticker.C:
			w.runActivityDecayJob(ctx)
		}
	}
}

// runActivityDecayJob 执行活跃度衰减任务
func (w *OptimizedFeedWorker) runActivityDecayJob(ctx context.Context) {
	w.logger.Info("Starting activity decay job")

	// 这里需要扫描所有用户并应用活跃度衰减
	// 实际实现可能需要分批处理以避免对数据库造成过大压力

	w.logger.Info("Activity decay job completed")
}

// GetWorkerStats 获取Worker统计信息
func (w *OptimizedFeedWorker) GetWorkerStats(ctx context.Context) (map[string]interface{}, error) {
	stats := map[string]interface{}{
		"worker_type":        "optimized_feed_worker",
		"start_time":         time.Now().Format(time.RFC3339),
		"processed_messages": 0, // 这里需要实际统计
	}

	// 获取各种服务的统计信息
	if cacheStats, err := w.cacheStrategyService.GetCacheStats(ctx); err == nil {
		stats["cache_stats"] = cacheStats
	}

	if distributionStats, err := w.recoveryService.GetDistributionStats(ctx); err == nil {
		stats["distribution_stats"] = distributionStats
	}

	return stats, nil
}

// Stop 停止Worker
func (w *OptimizedFeedWorker) Stop(ctx context.Context) error {
	w.logger.Info("Stopping optimized feed worker")
	return w.consumer.Close()
}
