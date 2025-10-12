package services

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/feed-system/feed-system/internal/models"
	"github.com/feed-system/feed-system/pkg/cache"
	"github.com/feed-system/feed-system/pkg/logger"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
)

// TimelineCacheService Redis Timeline缓存服务
type TimelineCacheService struct {
	cache  *cache.RedisClient
	logger *logger.Logger
}

func NewTimelineCacheService(cache *cache.RedisClient, logger *logger.Logger) *TimelineCacheService {
	return &TimelineCacheService{
		cache:  cache,
		logger: logger,
	}
}

const (
	// Timeline缓存配置
	TimelineCacheTTL     = 24 * time.Hour     // Timeline缓存过期时间
	MaxTimelineSize      = 1000               // 每个用户Timeline最大条数
	ActiveUserCacheTTL   = 7 * 24 * time.Hour // 活跃用户缓存时间更长
	InactiveUserCacheTTL = 2 * time.Hour      // 非活跃用户缓存时间较短
)

// TimelineItem Timeline条目
type TimelineItem struct {
	PostID    string    `json:"post_id"`
	Score     float64   `json:"score"`
	Timestamp time.Time `json:"timestamp"`
}

// AddToTimeline 添加帖子到用户Timeline
func (s *TimelineCacheService) AddToTimeline(ctx context.Context, userID uuid.UUID, postID uuid.UUID, score float64, timestamp time.Time) error {
	key := s.getTimelineKey(userID)

	// 使用时间戳作为score，确保时间顺序
	scoreValue := float64(timestamp.Unix())

	// 添加到SortedSet
	if err := s.cache.ZAdd(ctx, key, &redis.Z{
		Score:  scoreValue,
		Member: postID.String(),
	}); err != nil {
		return fmt.Errorf("failed to add to timeline: %w", err)
	}

	// 限制Timeline大小，删除最旧的条目
	if err := s.cache.ZRemRangeByRank(ctx, key, 0, -MaxTimelineSize-1); err != nil {
		s.logger.WithError(err).Error("Failed to trim timeline")
	}

	// 设置过期时间
	if err := s.cache.Expire(ctx, key, TimelineCacheTTL); err != nil {
		s.logger.WithError(err).Error("Failed to set timeline expiration")
	}

	return nil
}

// GetTimeline 获取用户Timeline (基于游标分页)
func (s *TimelineCacheService) GetTimeline(ctx context.Context, userID uuid.UUID, cursor string, limit int) ([]TimelineItem, string, bool, error) {
	key := s.getTimelineKey(userID)

	// 解析游标
	var maxScore float64 = float64(time.Now().Unix()) // 默认从当前时间开始
	if cursor != "" {
		if score, err := strconv.ParseFloat(cursor, 64); err == nil {
			maxScore = score
		}
	}

	// 使用ZRevRangeByScore获取数据，按时间倒序
	results, err := s.cache.ZRevRangeByScoreWithScores(ctx, key, &redis.ZRangeBy{
		Min:   "-inf",
		Max:   fmt.Sprintf("(%f", maxScore), // 不包含cursor本身
		Count: int64(limit + 1),             // 多获取一个判断是否还有更多
	})
	if err != nil {
		return nil, "", false, fmt.Errorf("failed to get timeline: %w", err)
	}

	var items []TimelineItem
	var nextCursor string
	hasMore := false

	// 处理结果
	if len(results) > limit {
		hasMore = true
		results = results[:limit]
	}

	for _, result := range results {
		item := TimelineItem{
			PostID:    result.Member.(string),
			Score:     result.Score,
			Timestamp: time.Unix(int64(result.Score), 0),
		}
		items = append(items, item)
		nextCursor = fmt.Sprintf("%.0f", result.Score)
	}

	return items, nextCursor, hasMore, nil
}

// RemoveFromTimeline 从Timeline移除帖子
func (s *TimelineCacheService) RemoveFromTimeline(ctx context.Context, userID uuid.UUID, postID uuid.UUID) error {
	key := s.getTimelineKey(userID)

	if err := s.cache.ZRem(ctx, key, postID.String()); err != nil {
		return fmt.Errorf("failed to remove from timeline: %w", err)
	}

	return nil
}

// BatchAddToTimeline 批量添加到多个用户的Timeline
func (s *TimelineCacheService) BatchAddToTimeline(ctx context.Context, userIDs []uuid.UUID, postID uuid.UUID, score float64, timestamp time.Time) error {
	scoreValue := float64(timestamp.Unix())

	// 使用Pipeline批量操作
	pipe := s.cache.Pipeline()

	for _, userID := range userIDs {
		key := s.getTimelineKey(userID)
		pipe.ZAdd(ctx, key, &redis.Z{
			Score:  scoreValue,
			Member: postID.String(),
		})
		// 限制大小
		pipe.ZRemRangeByRank(ctx, key, 0, -MaxTimelineSize-1)
		// 设置过期时间
		pipe.Expire(ctx, key, TimelineCacheTTL)
	}

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("failed to batch add to timelines: %w", err)
	}

	return nil
}

// ClearUserTimeline 清空用户Timeline
func (s *TimelineCacheService) ClearUserTimeline(ctx context.Context, userID uuid.UUID) error {
	key := s.getTimelineKey(userID)
	return s.cache.Delete(ctx, key)
}

// IsTimelineCached 检查用户Timeline是否已缓存
func (s *TimelineCacheService) IsTimelineCached(ctx context.Context, userID uuid.UUID) (bool, error) {
	key := s.getTimelineKey(userID)
	count, err := s.cache.Exists(ctx, key)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetTimelineSize 获取Timeline大小
func (s *TimelineCacheService) GetTimelineSize(ctx context.Context, userID uuid.UUID) (int64, error) {
	key := s.getTimelineKey(userID)
	return s.cache.ZCard(ctx, key)
}

// SetTimelineExpiration 设置Timeline过期时间（根据用户活跃度）
func (s *TimelineCacheService) SetTimelineExpiration(ctx context.Context, userID uuid.UUID, isActiveUser bool) error {
	key := s.getTimelineKey(userID)

	var ttl time.Duration
	if isActiveUser {
		ttl = ActiveUserCacheTTL
	} else {
		ttl = InactiveUserCacheTTL
	}

	return s.cache.Expire(ctx, key, ttl)
}

// RebuildTimelineFromDB 从数据库重建Timeline缓存
func (s *TimelineCacheService) RebuildTimelineFromDB(ctx context.Context, userID uuid.UUID, timelines []*models.Timeline) error {
	key := s.getTimelineKey(userID)

	// 先清空现有缓存
	if err := s.cache.Delete(ctx, key); err != nil {
		s.logger.WithError(err).Error("Failed to clear timeline cache")
	}

	if len(timelines) == 0 {
		return nil
	}

	// 批量添加
	pipe := s.cache.Pipeline()
	for _, timeline := range timelines {
		scoreValue := float64(timeline.CreatedAt.Unix())
		pipe.ZAdd(ctx, key, &redis.Z{
			Score:  scoreValue,
			Member: timeline.PostID.String(),
		})
	}

	// 设置过期时间
	pipe.Expire(ctx, key, TimelineCacheTTL)

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("failed to rebuild timeline cache: %w", err)
	}

	return nil
}

// getTimelineKey 获取Timeline的Redis key
func (s *TimelineCacheService) getTimelineKey(userID uuid.UUID) string {
	return fmt.Sprintf("timeline:%s", userID.String())
}

// GetOldestPostScore 获取Timeline中最旧帖子的分数
func (s *TimelineCacheService) GetOldestPostScore(ctx context.Context, userID uuid.UUID) (float64, error) {
	key := s.getTimelineKey(userID)

	// 获取分数最小的一个元素
	results, err := s.cache.ZRangeWithScores(ctx, key, 0, 0)
	if err != nil {
		return 0, err
	}

	if len(results) == 0 {
		return 0, nil
	}

	return results[0].Score, nil
}

// CleanupExpiredTimelines 清理过期的Timeline缓存
func (s *TimelineCacheService) CleanupExpiredTimelines(ctx context.Context) error {
	// 这个方法可以通过定时任务调用，清理过期的Timeline
	// 实际实现可能需要扫描所有timeline:*的key并检查其TTL
	// 这里只是一个示例框架
	s.logger.Info("Timeline cleanup job started")

	// TODO: 实现具体的清理逻辑
	// 可以使用SCAN命令扫描timeline:*模式的key
	// 然后检查其TTL，删除过期的缓存

	return nil
}
