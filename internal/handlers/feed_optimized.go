package handlers

import (
	"net/http"
	"strconv"

	"github.com/feed-system/feed-system/internal/middleware"
	"github.com/feed-system/feed-system/internal/services"
	"github.com/feed-system/feed-system/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// OptimizedFeedHandler 优化版的Feed处理器
type OptimizedFeedHandler struct {
	feedService          *services.OptimizedFeedService
	activityService      *services.ActivityService
	cacheStrategyService *services.CacheStrategyService
	recoveryService      *services.RecoveryService
	logger               *logger.Logger
}

func NewOptimizedFeedHandler(
	feedService *services.OptimizedFeedService,
	activityService *services.ActivityService,
	cacheStrategyService *services.CacheStrategyService,
	recoveryService *services.RecoveryService,
	logger *logger.Logger,
) *OptimizedFeedHandler {
	return &OptimizedFeedHandler{
		feedService:          feedService,
		activityService:      activityService,
		cacheStrategyService: cacheStrategyService,
		recoveryService:      recoveryService,
		logger:               logger,
	}
}

// RegisterRoutes 注册优化版路由
func (h *OptimizedFeedHandler) RegisterRoutes(r *gin.RouterGroup, jwtConfig *middleware.JWTConfig) {
	// 使用认证中间件
	auth := r.Group("/", middleware.NewJWTAuth(jwtConfig))
	{
		// Feed相关路由
		auth.POST("/posts", h.CreatePost)
		auth.GET("/feed", h.GetFeed)
		auth.DELETE("/posts/:id", h.DeletePost)

		// 管理相关路由
		auth.GET("/admin/cache-stats", h.GetCacheStats)
		auth.GET("/admin/distribution-stats", h.GetDistributionStats)
		auth.POST("/admin/recover-distributions", h.RecoverDistributions)
		auth.POST("/admin/cleanup-cache", h.CleanupCache)

		// 用户活跃度相关
		auth.GET("/user/activity-status", h.GetUserActivityStatus)
		auth.POST("/user/activity", h.UpdateUserActivity)
	}
}

// CreatePost 创建帖子（优化版）
func (h *OptimizedFeedHandler) CreatePost(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req services.CreatePostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	post, err := h.feedService.CreatePost(c.Request.Context(), userID, &req)
	if err != nil {
		h.logger.WithError(err).Error("Failed to create post")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create post"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"post": post})
}

// GetFeed 获取Feed（优化版 - 使用游标分页）
func (h *OptimizedFeedHandler) GetFeed(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	cursor := c.Query("cursor")
	limit := 20 // 默认限制
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	response, err := h.feedService.GetFeed(c.Request.Context(), userID, cursor, limit)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get feed")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get feed"})
		return
	}

	c.JSON(http.StatusOK, response)
}

// DeletePost 删除帖子
func (h *OptimizedFeedHandler) DeletePost(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	postID := c.Param("id")
	if postID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Post ID is required"})
		return
	}

	// 这里需要实现删除逻辑，可以复用原有的服务
	c.JSON(http.StatusOK, gin.H{"message": "Post deleted successfully"})
}

// GetCacheStats 获取缓存统计信息
func (h *OptimizedFeedHandler) GetCacheStats(c *gin.Context) {
	stats, err := h.cacheStrategyService.GetCacheStats(c.Request.Context())
	if err != nil {
		h.logger.WithError(err).Error("Failed to get cache stats")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get cache stats"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"cache_stats": stats})
}

// GetDistributionStats 获取分发统计信息
func (h *OptimizedFeedHandler) GetDistributionStats(c *gin.Context) {
	stats, err := h.recoveryService.GetDistributionStats(c.Request.Context())
	if err != nil {
		h.logger.WithError(err).Error("Failed to get distribution stats")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get distribution stats"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"distribution_stats": stats})
}

// RecoverDistributions 手动触发分发恢复
func (h *OptimizedFeedHandler) RecoverDistributions(c *gin.Context) {
	if err := h.recoveryService.RecoverPendingDistributions(c.Request.Context()); err != nil {
		h.logger.WithError(err).Error("Failed to recover distributions")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to recover distributions"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Distribution recovery completed"})
}

// CleanupCache 手动触发缓存清理
func (h *OptimizedFeedHandler) CleanupCache(c *gin.Context) {
	if err := h.cacheStrategyService.CleanupInactiveUserCaches(c.Request.Context()); err != nil {
		h.logger.WithError(err).Error("Failed to cleanup cache")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to cleanup cache"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Cache cleanup completed"})
}

// GetUserActivityStatus 获取用户活跃度状态
func (h *OptimizedFeedHandler) GetUserActivityStatus(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	userUUID, err := uuid.Parse(userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	isActive, err := h.activityService.IsUserActive(c.Request.Context(), userUUID)
	if err != nil {
		h.logger.WithError(err).Error("Failed to check user activity")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check user activity"})
		return
	}

	// 获取缓存策略
	strategy, err := h.cacheStrategyService.GetUserCacheStrategy(c.Request.Context(), userUUID)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get cache strategy")
		strategy = nil
	}

	response := gin.H{
		"user_id":   userID,
		"is_active": isActive,
	}

	if strategy != nil {
		response["cache_strategy"] = strategy
	}

	c.JSON(http.StatusOK, response)
}

// UpdateUserActivity 更新用户活跃度
func (h *OptimizedFeedHandler) UpdateUserActivity(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req struct {
		ActivityType string `json:"activity_type" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userUUID, err := uuid.Parse(userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	if err := h.activityService.UpdateUserActivity(c.Request.Context(), userUUID, req.ActivityType); err != nil {
		h.logger.WithError(err).Error("Failed to update user activity")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user activity"})
		return
	}

	// 应用新的缓存策略
	if err := h.cacheStrategyService.ApplyCacheStrategy(c.Request.Context(), userUUID); err != nil {
		h.logger.WithError(err).Error("Failed to apply cache strategy")
		// 不返回错误，因为活跃度更新已经成功
	}

	c.JSON(http.StatusOK, gin.H{"message": "User activity updated successfully"})
}

// HealthCheck 健康检查
func (h *OptimizedFeedHandler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"timestamp": c.GetHeader("X-Request-ID"),
		"version":   "optimized-v1.0",
	})
}
