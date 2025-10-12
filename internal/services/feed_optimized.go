package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/feed-system/feed-system/internal/config"
	"github.com/feed-system/feed-system/internal/models"
	"github.com/feed-system/feed-system/internal/repository"
	"github.com/feed-system/feed-system/pkg/cache"
	"github.com/feed-system/feed-system/pkg/logger"
	"github.com/feed-system/feed-system/pkg/queue"
	"github.com/google/uuid"
)

// OptimizedFeedService 优化版的Feed服务
type OptimizedFeedService struct {
	postRepo     *repository.PostRepository
	timelineRepo *repository.TimelineRepository
	userRepo     *repository.UserRepository
	followRepo   *repository.FollowRepository
	likeRepo     *repository.LikeRepository
	commentRepo  *repository.CommentRepository
	cache        *cache.RedisClient
	producer     *queue.KafkaProducer
	config       *config.FeedConfig
	logger       *logger.Logger

	// 新增的服务
	activityService      *ActivityService
	timelineCacheService *TimelineCacheService
}

func NewOptimizedFeedService(
	postRepo *repository.PostRepository,
	timelineRepo *repository.TimelineRepository,
	userRepo *repository.UserRepository,
	followRepo *repository.FollowRepository,
	likeRepo *repository.LikeRepository,
	commentRepo *repository.CommentRepository,
	cache *cache.RedisClient,
	producer *queue.KafkaProducer,
	config *config.FeedConfig,
	logger *logger.Logger,
	activityService *ActivityService,
	timelineCacheService *TimelineCacheService,
) *OptimizedFeedService {
	return &OptimizedFeedService{
		postRepo:             postRepo,
		timelineRepo:         timelineRepo,
		userRepo:             userRepo,
		followRepo:           followRepo,
		likeRepo:             likeRepo,
		commentRepo:          commentRepo,
		cache:                cache,
		producer:             producer,
		config:               config,
		logger:               logger,
		activityService:      activityService,
		timelineCacheService: timelineCacheService,
	}
}

// CreatePost 创建帖子 (优化版)
func (s *OptimizedFeedService) CreatePost(ctx context.Context, userID string, req *CreatePostRequest) (*models.Post, error) {
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	// 更新用户活跃度
	if err := s.activityService.UpdateUserActivity(ctx, userUUID, "post"); err != nil {
		s.logger.WithError(err).Error("Failed to update user activity")
	}

	// 获取用户信息
	user, err := s.userRepo.GetByID(ctx, userUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		return nil, errors.New("user not found")
	}

	// 创建帖子
	post := &models.Post{
		UserID:    userUUID,
		Content:   req.Content,
		ImageURLs: req.ImageURLs,
		Score:     s.calculateInitialScore(user),
		CreatedAt: time.Now(),
	}

	if err := s.postRepo.Create(ctx, post); err != nil {
		return nil, fmt.Errorf("failed to create post: %w", err)
	}

	// 计算帖子分数
	post.Score = s.calculatePostScore(post, user)
	if err := s.postRepo.Update(ctx, post); err != nil {
		s.logger.WithError(err).Error("Failed to update post score")
	}

	// 使用优化的分发策略
	if err := s.distributePostOptimized(ctx, post, user); err != nil {
		s.logger.WithError(err).Error("Failed to distribute post")
	}

	// 发送帖子创建事件
	event := queue.Event{
		Type:      queue.EventPostCreated,
		Timestamp: post.CreatedAt,
		Data: queue.PostEventData{
			PostID:    post.ID.String(),
			UserID:    userID,
			Content:   post.Content,
			CreatedAt: post.CreatedAt.Format("2006-01-02T15:04:05Z"),
		},
	}
	if err := s.producer.Publish(ctx, userID, event); err != nil {
		s.logger.WithError(err).Error("Failed to publish post created event")
	}

	s.logger.WithFields(map[string]interface{}{
		"post_id": post.ID,
		"user_id": userID,
	}).Info("Post created successfully")

	return post, nil
}

// GetFeed 获取Feed (优化版 - 使用游标分页)
func (s *OptimizedFeedService) GetFeed(ctx context.Context, userID string, cursor string, limit int) (*FeedResponse, error) {
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	// 更新用户活跃度
	if err := s.activityService.UpdateUserActivity(ctx, userUUID, "view_feed"); err != nil {
		s.logger.WithError(err).Error("Failed to update user activity")
	}

	// 首先尝试从Redis缓存获取Timeline
	timelineItems, nextCursor, hasMore, err := s.timelineCacheService.GetTimeline(ctx, userUUID, cursor, limit)
	if err != nil {
		s.logger.WithError(err).Error("Failed to get timeline from cache")
	}

	var posts []*models.Post

	if len(timelineItems) > 0 {
		// 从缓存获取到数据，根据PostID获取完整的Post信息
		posts, err = s.getPostsByIDs(ctx, timelineItems)
		if err != nil {
			s.logger.WithError(err).Error("Failed to get posts by IDs")
			// 如果获取失败，回退到拉模式
			return s.getFeedByPullMode(ctx, userUUID, cursor, limit)
		}
	} else {
		// 缓存中没有数据，使用拉模式重建Timeline
		return s.getFeedByPullMode(ctx, userUUID, cursor, limit)
	}

	// 更新动态数据
	s.updateDynamicData(ctx, posts, userUUID)

	response := &FeedResponse{
		Posts:      posts,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}

	return response, nil
}

// distributePostOptimized 优化的帖子分发策略
func (s *OptimizedFeedService) distributePostOptimized(ctx context.Context, post *models.Post, author *models.User) error {
	// 判断是否为头部用户（粉丝数超过阈值）
	if author.Followers > int64(s.config.PushThreshold) {
		// 头部用户：使用"在线推、离线拉"策略
		return s.distributeForInfluencer(ctx, post, author)
	} else {
		// 普通用户：使用推模式
		return s.distributeForRegularUser(ctx, post, author)
	}
}

// distributeForInfluencer 头部用户的分发策略
func (s *OptimizedFeedService) distributeForInfluencer(ctx context.Context, post *models.Post, author *models.User) error {
	// 1. 获取活跃的关注者（在线推）
	activeFollowers, err := s.activityService.GetActiveFollowers(ctx, author.ID, 1000) // 限制推送给前1000个活跃用户
	if err != nil {
		s.logger.WithError(err).Error("Failed to get active followers")
		activeFollowers = []uuid.UUID{} // 继续执行，但不推送给任何人
	}

	// 2. 推送给活跃用户的Timeline缓存
	if len(activeFollowers) > 0 {
		if err := s.timelineCacheService.BatchAddToTimeline(ctx, activeFollowers, post.ID, post.Score, post.CreatedAt); err != nil {
			s.logger.WithError(err).Error("Failed to batch add to active followers timeline")
		}
	}

	// 3. 记录推送状态，用于崩溃恢复
	if err := s.recordDistributionStatus(ctx, post.ID, author.ID, "influencer_push_completed"); err != nil {
		s.logger.WithError(err).Error("Failed to record distribution status")
	}

	// 4. 发送异步任务处理非活跃用户（离线拉模式会在用户活跃时处理）
	event := queue.Event{
		Type:      "post_distribution_completed",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"post_id":           post.ID.String(),
			"author_id":         author.ID.String(),
			"active_followers":  len(activeFollowers),
			"distribution_type": "influencer",
		},
	}
	if err := s.producer.Publish(ctx, author.ID.String(), event); err != nil {
		s.logger.WithError(err).Error("Failed to publish distribution event")
	}

	s.logger.WithFields(map[string]interface{}{
		"post_id":          post.ID,
		"author_id":        author.ID,
		"active_followers": len(activeFollowers),
	}).Info("Influencer post distributed to active followers")

	return nil
}

// distributeForRegularUser 普通用户的分发策略
func (s *OptimizedFeedService) distributeForRegularUser(ctx context.Context, post *models.Post, author *models.User) error {
	// 获取所有关注者
	followers, err := s.followRepo.GetFollowers(ctx, author.ID, 0, int(s.config.MaxFeedSize))
	if err != nil {
		return fmt.Errorf("failed to get followers: %w", err)
	}

	var followerIDs []uuid.UUID
	for _, follower := range followers {
		followerIDs = append(followerIDs, follower.ID)
	}

	// 推送到所有关注者的Timeline缓存
	if len(followerIDs) > 0 {
		if err := s.timelineCacheService.BatchAddToTimeline(ctx, followerIDs, post.ID, post.Score, post.CreatedAt); err != nil {
			s.logger.WithError(err).Error("Failed to batch add to followers timeline")
		}
	}

	// 也添加到作者自己的timeline
	if err := s.timelineCacheService.AddToTimeline(ctx, author.ID, post.ID, post.Score, post.CreatedAt); err != nil {
		s.logger.WithError(err).Error("Failed to add to author timeline")
	}

	s.logger.WithFields(map[string]interface{}{
		"post_id":   post.ID,
		"author_id": author.ID,
		"followers": len(followerIDs),
	}).Info("Regular user post distributed to all followers")

	return nil
}

// getFeedByPullMode 使用拉模式获取Feed
func (s *OptimizedFeedService) getFeedByPullMode(ctx context.Context, userID uuid.UUID, cursor string, limit int) (*FeedResponse, error) {
	// 获取关注的用户
	following, err := s.followRepo.GetFollowing(ctx, userID, 0, 1000) // 限制关注数量
	if err != nil {
		return nil, fmt.Errorf("failed to get following users: %w", err)
	}

	var followingIDs []uuid.UUID
	for _, user := range following {
		followingIDs = append(followingIDs, user.ID)
	}
	// 包含自己的帖子
	followingIDs = append(followingIDs, userID)

	// 从数据库拉取最新的帖子
	posts, err := s.postRepo.GetPostsByUserIDs(ctx, followingIDs, cursor, limit+1)
	if err != nil {
		return nil, fmt.Errorf("failed to get posts by user IDs: %w", err)
	}

	// 处理分页
	hasMore := len(posts) > limit
	if hasMore {
		posts = posts[:limit]
	}

	var nextCursor string
	if len(posts) > 0 {
		nextCursor = posts[len(posts)-1].CreatedAt.Format(time.RFC3339Nano)
	}

	// 重建Timeline缓存（异步）
	go func() {
		s.rebuildTimelineCache(context.Background(), userID, posts)
	}()

	// 更新动态数据
	s.updateDynamicData(ctx, posts, userID)

	response := &FeedResponse{
		Posts:      posts,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}

	return response, nil
}

// getPostsByIDs 根据Timeline项获取完整的Post信息
func (s *OptimizedFeedService) getPostsByIDs(ctx context.Context, timelineItems []TimelineItem) ([]*models.Post, error) {
	var postIDs []uuid.UUID
	for _, item := range timelineItems {
		if postID, err := uuid.Parse(item.PostID); err == nil {
			postIDs = append(postIDs, postID)
		}
	}

	if len(postIDs) == 0 {
		return []*models.Post{}, nil
	}

	posts, err := s.postRepo.GetByIDs(ctx, postIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get posts by IDs: %w", err)
	}

	// 按照timeline的顺序重新排序
	postMap := make(map[uuid.UUID]*models.Post)
	for _, post := range posts {
		postMap[post.ID] = post
	}

	var orderedPosts []*models.Post
	for _, item := range timelineItems {
		if postID, err := uuid.Parse(item.PostID); err == nil {
			if post, exists := postMap[postID]; exists && !post.IsDeleted {
				orderedPosts = append(orderedPosts, post)
			}
		}
	}

	return orderedPosts, nil
}

// rebuildTimelineCache 重建Timeline缓存
func (s *OptimizedFeedService) rebuildTimelineCache(ctx context.Context, userID uuid.UUID, posts []*models.Post) {
	var timelines []*models.Timeline
	for _, post := range posts {
		timeline := &models.Timeline{
			UserID:    userID,
			PostID:    post.ID,
			Score:     post.Score,
			CreatedAt: post.CreatedAt,
		}
		timelines = append(timelines, timeline)
	}

	if err := s.timelineCacheService.RebuildTimelineFromDB(ctx, userID, timelines); err != nil {
		s.logger.WithError(err).Error("Failed to rebuild timeline cache")
	}

	// 根据用户活跃度设置过期时间
	isActive, _ := s.activityService.IsUserActive(ctx, userID)
	if err := s.timelineCacheService.SetTimelineExpiration(ctx, userID, isActive); err != nil {
		s.logger.WithError(err).Error("Failed to set timeline expiration")
	}
}

// recordDistributionStatus 记录分发状态（用于崩溃恢复）
func (s *OptimizedFeedService) recordDistributionStatus(ctx context.Context, postID, authorID uuid.UUID, status string) error {
	key := fmt.Sprintf("distribution_status:%s", postID.String())
	data := map[string]interface{}{
		"post_id":   postID.String(),
		"author_id": authorID.String(),
		"status":    status,
		"timestamp": time.Now().Unix(),
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	// 保存24小时，用于崩溃恢复
	return s.cache.Set(ctx, key, jsonData, 24*time.Hour)
}

// Helper methods (保持与原版本相同)
func (s *OptimizedFeedService) calculateInitialScore(user *models.User) float64 {
	score := 1.0
	if user.Followers > 0 {
		score += math.Log10(float64(user.Followers)+1) * 0.5
	}
	score += float64(user.Following) * 0.01
	return score
}

func (s *OptimizedFeedService) calculatePostScore(post *models.Post, user *models.User) float64 {
	score := s.calculateInitialScore(user)
	hoursSinceCreated := time.Since(post.CreatedAt).Hours()
	timeDecay := math.Exp(-hoursSinceCreated / 24.0)
	engagementScore := float64(post.LikeCount)*0.1 + float64(post.CommentCount)*0.2 + float64(post.ShareCount)*0.3
	finalScore := (score + engagementScore) * timeDecay
	return finalScore
}

func (s *OptimizedFeedService) updateDynamicData(ctx context.Context, posts []*models.Post, viewerID uuid.UUID) {
	for _, post := range posts {
		isLiked, err := s.likeRepo.IsLiked(ctx, viewerID, post.ID)
		if err != nil {
			s.logger.WithError(err).Error("Failed to check like status")
			continue
		}
		_ = isLiked
	}
}
