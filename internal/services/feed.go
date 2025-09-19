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

type FeedService struct {
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
}

func NewFeedService(
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
) *FeedService {
	return &FeedService{
		postRepo:     postRepo,
		timelineRepo: timelineRepo,
		userRepo:     userRepo,
		followRepo:   followRepo,
		likeRepo:     likeRepo,
		commentRepo:  commentRepo,
		cache:        cache,
		producer:     producer,
		config:       config,
		logger:       logger,
	}
}

type CreatePostRequest struct {
	Content   string   `json:"content" binding:"required,min=1,max=1000"`
	ImageURLs []string `json:"image_urls"`
}

type FeedResponse struct {
	Posts      []*models.Post `json:"posts"`
	NextCursor string         `json:"next_cursor"`
	HasMore    bool           `json:"has_more"`
}

func (s *FeedService) CreatePost(ctx context.Context, userID string, req *CreatePostRequest) (*models.Post, error) {
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
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
		UserID:      userUUID,
		Content:     req.Content,
		ImageURLs:   req.ImageURLs,
		Score:       s.calculateInitialScore(user),
		CreatedAt:   time.Now(),
	}

	if err := s.postRepo.Create(ctx, post); err != nil {
		return nil, fmt.Errorf("failed to create post: %w", err)
	}

	// 计算帖子分数
	post.Score = s.calculatePostScore(post, user)
	if err := s.postRepo.Update(ctx, post); err != nil {
		s.logger.WithError(err).Error("Failed to update post score")
	}

	// 分发帖子到关注者的timeline
	if err := s.distributePost(ctx, post, user); err != nil {
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

func (s *FeedService) GetFeed(ctx context.Context, userID string, cursor string, limit int) (*FeedResponse, error) {
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	// 检查缓存
	cacheKey := fmt.Sprintf("feed:%s:%s:%d", userID, cursor, limit)
	if cachedFeed, err := s.getCachedFeed(ctx, cacheKey); err == nil && cachedFeed != nil {
		return cachedFeed, nil
	}

	// 获取用户的timeline
	offset := 0
	if cursor != "" {
		// 解码cursor获取偏移量
		offset = s.decodeCursor(cursor)
	}

	timelines, err := s.timelineRepo.GetByUserID(ctx, userUUID, offset, limit+1)
	if err != nil {
		return nil, fmt.Errorf("failed to get timeline: %w", err)
	}

	var posts []*models.Post
	hasMore := false
	nextCursor := ""

	if len(timelines) > limit {
		hasMore = true
		timelines = timelines[:limit]
		nextCursor = s.encodeCursor(offset + limit)
	}

	// 提取posts
	for _, timeline := range timelines {
		if !timeline.Post.IsDeleted {
			posts = append(posts, &timeline.Post)
		}
	}

	// 更新阅读量等动态数据
	s.updateDynamicData(ctx, posts, userUUID)

	response := &FeedResponse{
		Posts:      posts,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}

	// 缓存结果
	if err := s.cacheFeed(ctx, cacheKey, response); err != nil {
		s.logger.WithError(err).Error("Failed to cache feed")
	}

	return response, nil
}

func (s *FeedService) GetUserPosts(ctx context.Context, targetUserID string, offset, limit int) ([]*models.Post, error) {
	userUUID, err := uuid.Parse(targetUserID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	posts, err := s.postRepo.GetByUserID(ctx, userUUID, offset, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get user posts: %w", err)
	}

	return posts, nil
}

func (s *FeedService) GetPostByID(ctx context.Context, postID string) (*models.Post, error) {
	postUUID, err := uuid.Parse(postID)
	if err != nil {
		return nil, fmt.Errorf("invalid post ID: %w", err)
	}

	post, err := s.postRepo.GetByID(ctx, postUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get post: %w", err)
	}
	if post == nil {
		return nil, errors.New("post not found")
	}

	return post, nil
}

func (s *FeedService) DeletePost(ctx context.Context, userID, postID string) error {
	postUUID, err := uuid.Parse(postID)
	if err != nil {
		return fmt.Errorf("invalid post ID: %w", err)
	}

	// 获取帖子
	post, err := s.postRepo.GetByID(ctx, postUUID)
	if err != nil {
		return fmt.Errorf("failed to get post: %w", err)
	}
	if post == nil {
		return errors.New("post not found")
	}

	// 检查权限
	if post.UserID.String() != userID {
		return errors.New("permission denied")
	}

	// 删除帖子
	if err := s.postRepo.Delete(ctx, postUUID); err != nil {
		return fmt.Errorf("failed to delete post: %w", err)
	}

	// 从所有timeline中删除
	if err := s.timelineRepo.DeleteByPostID(ctx, postUUID); err != nil {
		s.logger.WithError(err).Error("Failed to delete timeline entries")
	}

	// 清除相关缓存
	s.clearFeedCache(ctx, post.UserID.String())

	// 发送帖子删除事件
	event := queue.Event{
		Type:      queue.EventPostDeleted,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"post_id": postID,
			"user_id": userID,
		},
	}
	if err := s.producer.Publish(ctx, userID, event); err != nil {
		s.logger.WithError(err).Error("Failed to publish post deleted event")
	}

	s.logger.WithFields(map[string]interface{}{
		"post_id": postID,
		"user_id": userID,
	}).Info("Post deleted successfully")

	return nil
}

func (s *FeedService) SearchPosts(ctx context.Context, query string, offset, limit int) ([]*models.Post, error) {
	posts, err := s.postRepo.Search(ctx, query, offset, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search posts: %w", err)
	}
	return posts, nil
}

func (s *FeedService) distributePost(ctx context.Context, post *models.Post, author *models.User) error {
	// 根据粉丝数量决定使用推模式还是拉模式
	if author.Followers <= int64(s.config.PushThreshold) {
		return s.pushPost(ctx, post, author)
	} else {
		return s.pullPost(ctx, post, author)
	}
}

func (s *FeedService) pushPost(ctx context.Context, post *models.Post, author *models.User) error {
	// 推模式：将帖子推送给所有关注者
	followers, err := s.followRepo.GetFollowers(ctx, author.ID, 0, int(s.config.MaxFeedSize))
	if err != nil {
		return fmt.Errorf("failed to get followers: %w", err)
	}

	var timelines []*models.Timeline
	for _, follower := range followers {
		timeline := &models.Timeline{
			UserID:    follower.ID,
			PostID:    post.ID,
			Score:     post.Score,
			CreatedAt: post.CreatedAt,
		}
		timelines = append(timelines, timeline)
	}

	if len(timelines) > 0 {
		if err := s.timelineRepo.CreateBatch(ctx, timelines); err != nil {
			return fmt.Errorf("failed to create timelines: %w", err)
		}
	}

	// 也添加到作者自己的timeline
	authorTimeline := &models.Timeline{
		UserID:    author.ID,
		PostID:    post.ID,
		Score:     post.Score,
		CreatedAt: post.CreatedAt,
	}
	if err := s.timelineRepo.Create(ctx, authorTimeline); err != nil {
		s.logger.WithError(err).Error("Failed to create author timeline")
	}

	return nil
}

func (s *FeedService) pullPost(ctx context.Context, post *models.Post, author *models.User) error {
	// 拉模式：只将帖子推送给最活跃的一部分关注者
	// 这里简化为只推送给前1000个关注者
	followers, err := s.followRepo.GetFollowers(ctx, author.ID, 0, 1000)
	if err != nil {
		return fmt.Errorf("failed to get followers: %w", err)
	}

	var timelines []*models.Timeline
	for _, follower := range followers {
		timeline := &models.Timeline{
			UserID:    follower.ID,
			PostID:    post.ID,
			Score:     post.Score,
			CreatedAt: post.CreatedAt,
		}
		timelines = append(timelines, timeline)
	}

	if len(timelines) > 0 {
		if err := s.timelineRepo.CreateBatch(ctx, timelines); err != nil {
			return fmt.Errorf("failed to create timelines: %w", err)
		}
	}

	// 也添加到作者自己的timeline
	authorTimeline := &models.Timeline{
		UserID:    author.ID,
		PostID:    post.ID,
		Score:     post.Score,
		CreatedAt: post.CreatedAt,
	}
	if err := s.timelineRepo.Create(ctx, authorTimeline); err != nil {
		s.logger.WithError(err).Error("Failed to create author timeline")
	}

	return nil
}

func (s *FeedService) calculateInitialScore(user *models.User) float64 {
	// 基于用户的活跃度计算初始分数
	score := 1.0

	// 粉丝数影响
	if user.Followers > 0 {
		score += math.Log10(float64(user.Followers) + 1) * 0.5
	}

	// 活跃度影响（简化计算）
	score += float64(user.Following) * 0.01

	return score
}

func (s *FeedService) calculatePostScore(post *models.Post, user *models.User) float64 {
	// 计算帖子的综合得分，用于排序
	score := s.calculateInitialScore(user)

	// 时间衰减
	hoursSinceCreated := time.Since(post.CreatedAt).Hours()
	timeDecay := math.Exp(-hoursSinceCreated / 24.0) // 24小时衰减

	// 互动分数
	engagementScore := float64(post.LikeCount)*0.1 + float64(post.CommentCount)*0.2 + float64(post.ShareCount)*0.3

	// 综合分数
	finalScore := (score + engagementScore) * timeDecay

	return finalScore
}

func (s *FeedService) getCachedFeed(ctx context.Context, key string) (*FeedResponse, error) {
	var response FeedResponse
	if err := s.cache.GetJSON(ctx, key, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

func (s *FeedService) cacheFeed(ctx context.Context, key string, response *FeedResponse) error {
	return s.cache.SetJSON(ctx, key, response, s.config.CacheTTL)
}

func (s *FeedService) clearFeedCache(ctx context.Context, userID string) error {
	// 清除用户的所有feed缓存
	// pattern := fmt.Sprintf("feed:%s:*", userID)
	// 这里需要实现keys命令或扫描删除
	return nil
}

func (s *FeedService) updateDynamicData(ctx context.Context, posts []*models.Post, viewerID uuid.UUID) {
	// 更新阅读量等动态数据（这里简化处理）
	for _, post := range posts {
		// 检查是否已点赞
		isLiked, err := s.likeRepo.IsLiked(ctx, viewerID, post.ID)
		if err != nil {
			s.logger.WithError(err).Error("Failed to check like status")
			continue
		}
		// 这里可以添加更多动态数据
		_ = isLiked
	}
}

func (s *FeedService) encodeCursor(offset int) string {
	data := map[string]int{"offset": offset}
	jsonData, _ := json.Marshal(data)
	return string(jsonData)
}

func (s *FeedService) decodeCursor(cursor string) int {
	var data map[string]int
	if err := json.Unmarshal([]byte(cursor), &data); err != nil {
		return 0
	}
	if offset, ok := data["offset"]; ok {
		return offset
	}
	return 0
}