package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/feed-system/feed-system/internal/models"
	"github.com/feed-system/feed-system/internal/repository"
	"github.com/feed-system/feed-system/pkg/cache"
	"github.com/feed-system/feed-system/pkg/logger"
	"github.com/google/uuid"
)

// RecoveryService 崩溃恢复服务
type RecoveryService struct {
	postRepo             *repository.PostRepository
	userRepo             *repository.UserRepository
	followRepo           *repository.FollowRepository
	cache                *cache.RedisClient
	logger               *logger.Logger
	activityService      *ActivityService
	timelineCacheService *TimelineCacheService
}

func NewRecoveryService(
	postRepo *repository.PostRepository,
	userRepo *repository.UserRepository,
	followRepo *repository.FollowRepository,
	cache *cache.RedisClient,
	logger *logger.Logger,
	activityService *ActivityService,
	timelineCacheService *TimelineCacheService,
) *RecoveryService {
	return &RecoveryService{
		postRepo:             postRepo,
		userRepo:             userRepo,
		followRepo:           followRepo,
		cache:                cache,
		logger:               logger,
		activityService:      activityService,
		timelineCacheService: timelineCacheService,
	}
}

// DistributionStatus 分发状态
type DistributionStatus struct {
	PostID    string `json:"post_id"`
	AuthorID  string `json:"author_id"`
	Status    string `json:"status"`
	Timestamp int64  `json:"timestamp"`
}

// todo
// RecoverPendingDistributions 恢复待处理的分发任务
func (s *RecoveryService) RecoverPendingDistributions(ctx context.Context) error {
	s.logger.Info("Starting recovery of pending distributions")

	// 扫描所有待恢复的分发状态
	pattern := "distribution_status:*"
	keys, err := s.scanKeys(ctx, pattern)
	if err != nil {
		return fmt.Errorf("failed to scan distribution status keys: %w", err)
	}

	recoveredCount := 0
	for _, key := range keys {
		if err := s.recoverSingleDistribution(ctx, key); err != nil {
			s.logger.WithError(err).WithField("key", key).Error("Failed to recover distribution")
			continue
		}
		recoveredCount++
	}

	s.logger.WithField("recovered_count", recoveredCount).Info("Distribution recovery completed")
	return nil
}

// recoverSingleDistribution 恢复单个分发任务
func (s *RecoveryService) recoverSingleDistribution(ctx context.Context, key string) error {
	// 获取分发状态
	statusData, err := s.cache.Get(ctx, key)
	if err != nil {
		return fmt.Errorf("failed to get distribution status: %w", err)
	}

	var status DistributionStatus
	if err := json.Unmarshal([]byte(statusData), &status); err != nil {
		return fmt.Errorf("failed to unmarshal distribution status: %w", err)
	}

	// 检查是否需要恢复（超过5分钟未完成的任务）
	if time.Now().Unix()-status.Timestamp < 300 {
		return nil // 任务太新，不需要恢复
	}

	postID, err := uuid.Parse(status.PostID)
	if err != nil {
		return fmt.Errorf("invalid post ID: %w", err)
	}

	authorID, err := uuid.Parse(status.AuthorID)
	if err != nil {
		return fmt.Errorf("invalid author ID: %w", err)
	}

	// 获取帖子和作者信息
	post, err := s.postRepo.GetByID(ctx, postID)
	if err != nil {
		return fmt.Errorf("failed to get post: %w", err)
	}
	if post == nil {
		// 帖子不存在，清理状态
		s.cache.Delete(ctx, key)
		return nil
	}

	author, err := s.userRepo.GetByID(ctx, authorID)
	if err != nil {
		return fmt.Errorf("failed to get author: %w", err)
	}
	if author == nil {
		// 作者不存在，清理状态
		s.cache.Delete(ctx, key)
		return nil
	}

	// 根据状态进行恢复
	switch status.Status {
	case "influencer_push_started":
		// 头部用户推送未完成，重新执行
		if err := s.recoverInfluencerDistribution(ctx, post, author); err != nil {
			return fmt.Errorf("failed to recover influencer distribution: %w", err)
		}
	case "regular_push_started":
		// 普通用户推送未完成，重新执行
		if err := s.recoverRegularDistribution(ctx, post, author); err != nil {
			return fmt.Errorf("failed to recover regular distribution: %w", err)
		}
	default:
		// 未知状态或已完成，清理
		s.cache.Delete(ctx, key)
	}

	s.logger.WithFields(map[string]interface{}{
		"post_id":   status.PostID,
		"author_id": status.AuthorID,
		"status":    status.Status,
	}).Info("Distribution recovered successfully")

	return nil
}

// recoverInfluencerDistribution 恢复头部用户分发
func (s *RecoveryService) recoverInfluencerDistribution(ctx context.Context, post *models.Post, author *models.User) error {
	// 获取活跃的关注者
	activeFollowers, err := s.activityService.GetActiveFollowers(ctx, author.ID, 1000)
	if err != nil {
		s.logger.WithError(err).Error("Failed to get active followers during recovery")
		activeFollowers = []uuid.UUID{}
	}

	// 检查哪些用户的Timeline中还没有这个帖子
	var needDistribution []uuid.UUID
	for _, followerID := range activeFollowers {
		exists, err := s.checkPostInTimeline(ctx, followerID, post.ID)
		if err != nil {
			s.logger.WithError(err).Error("Failed to check post in timeline")
			continue
		}
		if !exists {
			needDistribution = append(needDistribution, followerID)
		}
	}

	// 推送给需要的用户
	if len(needDistribution) > 0 {
		if err := s.timelineCacheService.BatchAddToTimeline(ctx, needDistribution, post.ID, post.Score, post.CreatedAt); err != nil {
			return fmt.Errorf("failed to batch add to timeline during recovery: %w", err)
		}
	}

	// 更新状态为已完成
	key := fmt.Sprintf("distribution_status:%s", post.ID.String())
	if err := s.updateDistributionStatus(ctx, key, "influencer_push_completed"); err != nil {
		s.logger.WithError(err).Error("Failed to update distribution status")
	}

	return nil
}

// recoverRegularDistribution 恢复普通用户分发
func (s *RecoveryService) recoverRegularDistribution(ctx context.Context, post *models.Post, author *models.User) error {
	// 获取所有关注者
	followers, err := s.followRepo.GetFollowers(ctx, author.ID, 0, 10000)
	if err != nil {
		return fmt.Errorf("failed to get followers: %w", err)
	}

	var followerIDs []uuid.UUID
	for _, follower := range followers {
		followerIDs = append(followerIDs, follower.ID)
	}

	// 检查哪些用户的Timeline中还没有这个帖子
	var needDistribution []uuid.UUID
	for _, followerID := range followerIDs {
		exists, err := s.checkPostInTimeline(ctx, followerID, post.ID)
		if err != nil {
			s.logger.WithError(err).Error("Failed to check post in timeline")
			continue
		}
		if !exists {
			needDistribution = append(needDistribution, followerID)
		}
	}

	// 推送给需要的用户
	if len(needDistribution) > 0 {
		if err := s.timelineCacheService.BatchAddToTimeline(ctx, needDistribution, post.ID, post.Score, post.CreatedAt); err != nil {
			return fmt.Errorf("failed to batch add to timeline during recovery: %w", err)
		}
	}

	// 更新状态为已完成
	key := fmt.Sprintf("distribution_status:%s", post.ID.String())
	if err := s.updateDistributionStatus(ctx, key, "regular_push_completed"); err != nil {
		s.logger.WithError(err).Error("Failed to update distribution status")
	}

	return nil
}

// checkPostInTimeline 检查帖子是否在用户Timeline中
func (s *RecoveryService) checkPostInTimeline(ctx context.Context, userID, postID uuid.UUID) (bool, error) {
	timelineKey := fmt.Sprintf("timeline:%s", userID.String())

	// 使用ZScore检查成员是否存在
	_, err := s.cache.ZScore(ctx, timelineKey, postID.String())
	if err != nil {
		// 如果是"member not found"错误，表示不存在
		if err.Error() == "redis: nil" {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

// updateDistributionStatus 更新分发状态
func (s *RecoveryService) updateDistributionStatus(ctx context.Context, key, status string) error {
	// 获取现有状态
	statusData, err := s.cache.Get(ctx, key)
	if err != nil {
		return err
	}

	var distributionStatus DistributionStatus
	if err := json.Unmarshal([]byte(statusData), &distributionStatus); err != nil {
		return err
	}

	// 更新状态
	distributionStatus.Status = status
	distributionStatus.Timestamp = time.Now().Unix()

	// 保存更新后的状态
	jsonData, err := json.Marshal(distributionStatus)
	if err != nil {
		return err
	}

	// 已完成的任务保存较短时间，用于监控
	ttl := 1 * time.Hour
	if status == "completed" {
		ttl = 1 * time.Hour
	} else {
		ttl = 24 * time.Hour
	}

	return s.cache.Set(ctx, key, jsonData, ttl)
}

// scanKeys 扫描Redis中匹配模式的keys
func (s *RecoveryService) scanKeys(ctx context.Context, pattern string) ([]string, error) {
	// 这里需要实现Redis SCAN命令
	// 由于当前的RedisClient没有SCAN方法，我们需要添加它
	// 暂时返回空列表，实际实现需要添加SCAN支持
	return []string{}, nil
}

// StartRecoveryJob 启动定期恢复任务
func (s *RecoveryService) StartRecoveryJob(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("Recovery job stopped")
			return
		case <-ticker.C:
			if err := s.RecoverPendingDistributions(ctx); err != nil {
				s.logger.WithError(err).Error("Recovery job failed")
			}
		}
	}
}

// GetDistributionStats 获取分发统计信息
func (s *RecoveryService) GetDistributionStats(ctx context.Context) (map[string]int, error) {
	stats := map[string]int{
		"pending":   0,
		"completed": 0,
		"failed":    0,
	}

	// 扫描所有分发状态
	pattern := "distribution_status:*"
	keys, err := s.scanKeys(ctx, pattern)
	if err != nil {
		return stats, fmt.Errorf("failed to scan keys: %w", err)
	}

	for _, key := range keys {
		statusData, err := s.cache.Get(ctx, key)
		if err != nil {
			continue
		}

		var status DistributionStatus
		if err := json.Unmarshal([]byte(statusData), &status); err != nil {
			continue
		}

		switch status.Status {
		case "influencer_push_completed", "regular_push_completed":
			stats["completed"]++
		case "influencer_push_started", "regular_push_started":
			// 检查是否超时
			if time.Now().Unix()-status.Timestamp > 300 {
				stats["failed"]++
			} else {
				stats["pending"]++
			}
		default:
			stats["pending"]++
		}
	}

	return stats, nil
}
