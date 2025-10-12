package services

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/feed-system/feed-system/internal/models"
	"github.com/feed-system/feed-system/internal/repository"
	"github.com/feed-system/feed-system/pkg/cache"
	"github.com/feed-system/feed-system/pkg/logger"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
)

// ActivityService 用户活跃度服务
type ActivityService struct {
	userRepo *repository.UserRepository
	cache    *cache.RedisClient
	logger   *logger.Logger
}

func NewActivityService(
	userRepo *repository.UserRepository,
	cache *cache.RedisClient,
	logger *logger.Logger,
) *ActivityService {
	return &ActivityService{
		userRepo: userRepo,
		cache:    cache,
		logger:   logger,
	}
}

// 活跃度阈值配置
const (
	// 活跃用户的活跃度分数阈值
	ActiveUserScoreThreshold = 50.0
	// 在线用户缓存TTL
	OnlineUserCacheTTL = 15 * time.Minute
	// 活跃度衰减因子
	ActivityDecayFactor = 0.9
	// 最大活跃度分数
	MaxActivityScore = 1000.0
)

// IsUserActive 判断用户是否活跃
func (s *ActivityService) IsUserActive(ctx context.Context, userID uuid.UUID) (bool, error) {
	// 先从缓存检查
	cacheKey := fmt.Sprintf("user_active:%s", userID.String())
	if active, err := s.cache.Get(ctx, cacheKey); err == nil {
		return active == "1", nil
	}

	// 从数据库获取用户信息
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return false, fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		return false, nil
	}

	// 判断用户是否活跃
	isActive := s.calculateUserActivity(user)

	// 缓存结果
	cacheValue := "0"
	if isActive {
		cacheValue = "1"
	}
	if err := s.cache.Set(ctx, cacheKey, cacheValue, 5*time.Minute); err != nil {
		s.logger.WithError(err).Error("Failed to cache user activity status")
	}

	return isActive, nil
}

// UpdateUserActivity 更新用户活跃度
func (s *ActivityService) UpdateUserActivity(ctx context.Context, userID uuid.UUID, activityType string) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		return fmt.Errorf("user not found")
	}

	now := time.Now()

	// 更新最后活跃时间
	user.LastActiveAt = &now
	user.IsOnline = true

	// 计算活跃度增量
	increment := s.getActivityIncrement(activityType)

	// 应用时间衰减
	if user.LastActiveAt != nil {
		hoursSinceLastActive := now.Sub(*user.LastActiveAt).Hours()
		decay := math.Pow(ActivityDecayFactor, hoursSinceLastActive/24.0)
		user.ActivityScore = user.ActivityScore*decay + increment
	} else {
		user.ActivityScore = increment
	}

	// 限制最大活跃度分数
	if user.ActivityScore > MaxActivityScore {
		user.ActivityScore = MaxActivityScore
	}

	// 更新数据库
	if err := s.userRepo.Update(ctx, user); err != nil {
		return fmt.Errorf("failed to update user activity: %w", err)
	}

	// 更新在线状态缓存
	onlineKey := fmt.Sprintf("user_online:%s", userID.String())
	if err := s.cache.Set(ctx, onlineKey, "1", OnlineUserCacheTTL); err != nil {
		s.logger.WithError(err).Error("Failed to cache user online status")
	}

	// 清除活跃度缓存
	activeKey := fmt.Sprintf("user_active:%s", userID.String())
	if err := s.cache.Delete(ctx, activeKey); err != nil {
		s.logger.WithError(err).Error("Failed to clear user activity cache")
	}

	return nil
}

// GetActiveFollowers 获取活跃的关注者列表
func (s *ActivityService) GetActiveFollowers(ctx context.Context, userID uuid.UUID, limit int) ([]uuid.UUID, error) {
	cacheKey := fmt.Sprintf("active_followers:%s", userID.String())

	// 尝试从缓存获取
	if cachedFollowers, err := s.getActiveFollowersFromCache(ctx, cacheKey); err == nil && len(cachedFollowers) > 0 {
		if len(cachedFollowers) > limit {
			return cachedFollowers[:limit], nil
		}
		return cachedFollowers, nil
	}

	// 从数据库查询所有关注者，然后过滤活跃用户
	// 这里需要添加相应的repository方法
	// 暂时返回空列表，实际实现需要查询follow表并过滤活跃用户
	var activeFollowers []uuid.UUID

	// 缓存结果
	if err := s.cacheActiveFollowers(ctx, cacheKey, activeFollowers); err != nil {
		s.logger.WithError(err).Error("Failed to cache active followers")
	}

	return activeFollowers, nil
}

// SetUserOffline 设置用户离线
func (s *ActivityService) SetUserOffline(ctx context.Context, userID uuid.UUID) error {
	// 更新数据库
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		return nil
	}

	user.IsOnline = false
	if err := s.userRepo.Update(ctx, user); err != nil {
		return fmt.Errorf("failed to update user offline status: %w", err)
	}

	// 删除在线状态缓存
	onlineKey := fmt.Sprintf("user_online:%s", userID.String())
	if err := s.cache.Delete(ctx, onlineKey); err != nil {
		s.logger.WithError(err).Error("Failed to delete user online cache")
	}

	return nil
}

// calculateUserActivity 计算用户是否活跃
func (s *ActivityService) calculateUserActivity(user *models.User) bool {
	// 如果用户在线，直接认为活跃
	if user.IsOnline {
		return true
	}

	// 根据活跃度分数判断
	if user.ActivityScore >= ActiveUserScoreThreshold {
		return true
	}

	// 根据最后活跃时间判断（7天内活跃）
	if user.LastActiveAt != nil {
		return time.Since(*user.LastActiveAt) < 7*24*time.Hour
	}

	return false
}

// getActivityIncrement 根据活动类型获取活跃度增量
func (s *ActivityService) getActivityIncrement(activityType string) float64 {
	switch activityType {
	case "login":
		return 5.0
	case "post":
		return 15.0
	case "like":
		return 2.0
	case "comment":
		return 8.0
	case "share":
		return 10.0
	case "view_feed":
		return 1.0
	default:
		return 1.0
	}
}

// getActiveFollowersFromCache 从缓存获取活跃关注者
func (s *ActivityService) getActiveFollowersFromCache(ctx context.Context, key string) ([]uuid.UUID, error) {
	// 使用Redis Set存储活跃关注者ID
	members, err := s.cache.ZRevRange(ctx, key, 0, -1)
	if err != nil {
		return nil, err
	}

	var followers []uuid.UUID
	for _, member := range members {
		if id, err := uuid.Parse(member); err == nil {
			followers = append(followers, id)
		}
	}

	return followers, nil
}

// cacheActiveFollowers 缓存活跃关注者
func (s *ActivityService) cacheActiveFollowers(ctx context.Context, key string, followers []uuid.UUID) error {
	if len(followers) == 0 {
		return nil
	}

	// 使用ZSet存储，score为当前时间戳
	now := float64(time.Now().Unix())
	for _, follower := range followers {
		if err := s.cache.ZAdd(ctx, key, &redis.Z{
			Score:  now,
			Member: follower.String(),
		}); err != nil {
			return err
		}
	}

	// 设置过期时间
	return s.cache.Expire(ctx, key, 10*time.Minute)
}
