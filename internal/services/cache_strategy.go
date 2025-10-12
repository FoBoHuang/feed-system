package services

import (
	"context"
	"fmt"
	"time"

	"github.com/feed-system/feed-system/internal/config"
	"github.com/feed-system/feed-system/pkg/cache"
	"github.com/feed-system/feed-system/pkg/logger"
	"github.com/google/uuid"
)

// CacheStrategyService 缓存策略管理服务
type CacheStrategyService struct {
	cache                *cache.RedisClient
	config               *config.FeedConfig
	logger               *logger.Logger
	activityService      *ActivityService
	timelineCacheService *TimelineCacheService
}

func NewCacheStrategyService(
	cache *cache.RedisClient,
	config *config.FeedConfig,
	logger *logger.Logger,
	activityService *ActivityService,
	timelineCacheService *TimelineCacheService,
) *CacheStrategyService {
	return &CacheStrategyService{
		cache:                cache,
		config:               config,
		logger:               logger,
		activityService:      activityService,
		timelineCacheService: timelineCacheService,
	}
}

const (
	// 缓存策略配置
	ActiveUserCacheHours     = 7 * 24  // 活跃用户缓存7天
	InactiveUserCacheHours   = 2       // 非活跃用户缓存2小时
	VIPUserCacheHours        = 30 * 24 // VIP用户缓存30天
	MaxTimelineItemsActive   = 1000    // 活跃用户最大Timeline条数
	MaxTimelineItemsInactive = 200     // 非活跃用户最大Timeline条数
)

// UserCacheStrategy 用户缓存策略
type UserCacheStrategy struct {
	UserID           uuid.UUID     `json:"user_id"`
	IsActive         bool          `json:"is_active"`
	IsVIP            bool          `json:"is_vip"`
	CacheTTL         time.Duration `json:"cache_ttl"`
	MaxTimelineItems int           `json:"max_timeline_items"`
	LastUpdated      time.Time     `json:"last_updated"`
}

// DetermineUserCacheStrategy 确定用户的缓存策略
func (s *CacheStrategyService) DetermineUserCacheStrategy(ctx context.Context, userID uuid.UUID) (*UserCacheStrategy, error) {
	// 检查用户是否活跃
	isActive, err := s.activityService.IsUserActive(ctx, userID)
	if err != nil {
		s.logger.WithError(err).Error("Failed to check user activity")
		isActive = false
	}

	// 检查是否为VIP用户（这里简化为粉丝数超过10万的用户）
	isVIP := false
	// TODO: 实现VIP用户判断逻辑

	// 确定缓存策略
	var cacheTTL time.Duration
	var maxItems int

	if isVIP {
		cacheTTL = VIPUserCacheHours * time.Hour
		maxItems = MaxTimelineItemsActive * 2 // VIP用户更多缓存
	} else if isActive {
		cacheTTL = ActiveUserCacheHours * time.Hour
		maxItems = MaxTimelineItemsActive
	} else {
		cacheTTL = InactiveUserCacheHours * time.Hour
		maxItems = MaxTimelineItemsInactive
	}

	strategy := &UserCacheStrategy{
		UserID:           userID,
		IsActive:         isActive,
		IsVIP:            isVIP,
		CacheTTL:         cacheTTL,
		MaxTimelineItems: maxItems,
		LastUpdated:      time.Now(),
	}

	return strategy, nil
}

// ApplyCacheStrategy 应用缓存策略
func (s *CacheStrategyService) ApplyCacheStrategy(ctx context.Context, userID uuid.UUID) error {
	strategy, err := s.DetermineUserCacheStrategy(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to determine cache strategy: %w", err)
	}

	// 设置Timeline缓存过期时间
	if err := s.timelineCacheService.SetTimelineExpiration(ctx, userID, strategy.IsActive); err != nil {
		s.logger.WithError(err).Error("Failed to set timeline expiration")
	}

	// 如果是非活跃用户且Timeline过大，进行裁剪
	if !strategy.IsActive {
		timelineSize, err := s.timelineCacheService.GetTimelineSize(ctx, userID)
		if err != nil {
			s.logger.WithError(err).Error("Failed to get timeline size")
		} else if timelineSize > int64(strategy.MaxTimelineItems) {
			if err := s.trimUserTimeline(ctx, userID, strategy.MaxTimelineItems); err != nil {
				s.logger.WithError(err).Error("Failed to trim user timeline")
			}
		}
	}

	// 缓存策略信息
	if err := s.cacheUserStrategy(ctx, userID, strategy); err != nil {
		s.logger.WithError(err).Error("Failed to cache user strategy")
	}

	return nil
}

// trimUserTimeline 裁剪用户Timeline
func (s *CacheStrategyService) trimUserTimeline(ctx context.Context, userID uuid.UUID, maxItems int) error {
	timelineKey := fmt.Sprintf("timeline:%s", userID.String())

	// 保留最新的maxItems条记录，删除其余的
	// 使用ZRemRangeByRank删除最旧的记录
	return s.cache.ZRemRangeByRank(ctx, timelineKey, 0, -int64(maxItems)-1)
}

// cacheUserStrategy 缓存用户策略
func (s *CacheStrategyService) cacheUserStrategy(ctx context.Context, userID uuid.UUID, strategy *UserCacheStrategy) error {
	key := fmt.Sprintf("user_cache_strategy:%s", userID.String())
	return s.cache.SetJSON(ctx, key, strategy, 24*time.Hour)
}

// GetUserCacheStrategy 获取用户缓存策略
func (s *CacheStrategyService) GetUserCacheStrategy(ctx context.Context, userID uuid.UUID) (*UserCacheStrategy, error) {
	key := fmt.Sprintf("user_cache_strategy:%s", userID.String())
	var strategy UserCacheStrategy
	if err := s.cache.GetJSON(ctx, key, &strategy); err != nil {
		// 缓存中没有，重新计算
		return s.DetermineUserCacheStrategy(ctx, userID)
	}
	return &strategy, nil
}

// CleanupInactiveUserCaches 清理非活跃用户的缓存
func (s *CacheStrategyService) CleanupInactiveUserCaches(ctx context.Context) error {
	s.logger.Info("Starting cleanup of inactive user caches")

	// 扫描所有Timeline缓存
	pattern := "timeline:*"
	keys, err := s.scanTimelineKeys(ctx, pattern)
	if err != nil {
		return fmt.Errorf("failed to scan timeline keys: %w", err)
	}

	cleanedCount := 0
	for _, key := range keys {
		// 提取用户ID
		userID, err := s.extractUserIDFromTimelineKey(key)
		if err != nil {
			continue
		}

		// 检查用户是否活跃
		isActive, err := s.activityService.IsUserActive(ctx, userID)
		if err != nil {
			s.logger.WithError(err).Error("Failed to check user activity during cleanup")
			continue
		}

		// 如果用户不活跃，检查Timeline大小并进行清理
		if !isActive {
			timelineSize, err := s.timelineCacheService.GetTimelineSize(ctx, userID)
			if err != nil {
				continue
			}

			// 如果Timeline过大，进行裁剪
			if timelineSize > MaxTimelineItemsInactive {
				if err := s.trimUserTimeline(ctx, userID, MaxTimelineItemsInactive); err != nil {
					s.logger.WithError(err).Error("Failed to trim inactive user timeline")
					continue
				}
				cleanedCount++
			}

			// 设置较短的过期时间
			if err := s.cache.Expire(ctx, key, InactiveUserCacheHours*time.Hour); err != nil {
				s.logger.WithError(err).Error("Failed to set expiration for inactive user timeline")
			}
		}
	}

	s.logger.WithField("cleaned_count", cleanedCount).Info("Inactive user cache cleanup completed")
	return nil
}

// StartCacheCleanupJob 启动缓存清理任务
func (s *CacheStrategyService) StartCacheCleanupJob(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("Cache cleanup job stopped")
			return
		case <-ticker.C:
			if err := s.CleanupInactiveUserCaches(ctx); err != nil {
				s.logger.WithError(err).Error("Cache cleanup job failed")
			}
		}
	}
}

// GetCacheStats 获取缓存统计信息
func (s *CacheStrategyService) GetCacheStats(ctx context.Context) (map[string]interface{}, error) {
	stats := map[string]interface{}{
		"total_timelines": 0,
		"active_users":    0,
		"inactive_users":  0,
		"vip_users":       0,
		"cache_hit_rate":  0.0,
		"memory_usage_mb": 0.0,
	}

	// 扫描所有Timeline
	pattern := "timeline:*"
	keys, err := s.scanTimelineKeys(ctx, pattern)
	if err != nil {
		return stats, fmt.Errorf("failed to scan timeline keys: %w", err)
	}

	stats["total_timelines"] = len(keys)

	// 统计不同类型用户
	activeCount := 0
	inactiveCount := 0
	vipCount := 0

	for _, key := range keys {
		userID, err := s.extractUserIDFromTimelineKey(key)
		if err != nil {
			continue
		}

		strategy, err := s.GetUserCacheStrategy(ctx, userID)
		if err != nil {
			continue
		}

		if strategy.IsVIP {
			vipCount++
		} else if strategy.IsActive {
			activeCount++
		} else {
			inactiveCount++
		}
	}

	stats["active_users"] = activeCount
	stats["inactive_users"] = inactiveCount
	stats["vip_users"] = vipCount

	return stats, nil
}

// scanTimelineKeys 扫描Timeline相关的keys
func (s *CacheStrategyService) scanTimelineKeys(ctx context.Context, pattern string) ([]string, error) {
	// 这里需要实现Redis SCAN命令
	// 暂时返回空列表，实际实现需要添加SCAN支持
	return []string{}, nil
}

// extractUserIDFromTimelineKey 从Timeline key中提取用户ID
func (s *CacheStrategyService) extractUserIDFromTimelineKey(key string) (uuid.UUID, error) {
	// timeline:uuid格式
	if len(key) < 10 || key[:9] != "timeline:" {
		return uuid.Nil, fmt.Errorf("invalid timeline key format")
	}

	userIDStr := key[9:] // 去掉"timeline:"前缀
	return uuid.Parse(userIDStr)
}

// PrewarmCache 预热缓存（为活跃用户预先构建Timeline）
func (s *CacheStrategyService) PrewarmCache(ctx context.Context, userIDs []uuid.UUID) error {
	s.logger.WithField("user_count", len(userIDs)).Info("Starting cache prewarm")

	for _, userID := range userIDs {
		// 检查用户是否活跃
		isActive, err := s.activityService.IsUserActive(ctx, userID)
		if err != nil {
			s.logger.WithError(err).Error("Failed to check user activity during prewarm")
			continue
		}

		// 只为活跃用户预热缓存
		if isActive {
			// 检查Timeline是否已存在
			exists, err := s.timelineCacheService.IsTimelineCached(ctx, userID)
			if err != nil {
				s.logger.WithError(err).Error("Failed to check timeline cache")
				continue
			}

			if !exists {
				// Timeline不存在，需要构建
				// 这里可以触发拉模式来构建Timeline
				s.logger.WithField("user_id", userID).Info("Prewarming timeline for active user")
				// TODO: 实现Timeline预热逻辑
			}
		}
	}

	s.logger.Info("Cache prewarm completed")
	return nil
}
