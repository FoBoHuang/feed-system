package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/feed-system/feed-system/internal/config"
	"github.com/feed-system/feed-system/internal/handlers"
	"github.com/feed-system/feed-system/internal/middleware"
	"github.com/feed-system/feed-system/internal/repository"
	"github.com/feed-system/feed-system/internal/services"
	"github.com/feed-system/feed-system/internal/workers"
	"github.com/feed-system/feed-system/pkg/cache"
	"github.com/feed-system/feed-system/pkg/logger"
	"github.com/feed-system/feed-system/pkg/queue"
	"github.com/gin-gonic/gin"
)

func main() {
	// 加载配置
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 初始化日志
	logger := logger.NewLogger()
	logger.Info("Starting Feed System API server...")

	// 初始化数据库
	db, err := repository.NewDatabase(&cfg.Database)
	if err != nil {
		logger.WithError(err).Fatal("Failed to connect to database")
	}
	defer db.Close()

	// 自动迁移数据库表
	if err := db.AutoMigrate(); err != nil {
		logger.WithError(err).Fatal("Failed to migrate database")
	}

	// 初始化Redis缓存
	redisClient := cache.NewRedisClient(
		cfg.Redis.Addr(),
		cfg.Redis.Password,
		cfg.Redis.DB,
		cfg.Redis.PoolSize,
		cfg.Redis.MinIdleConns,
	)
	defer redisClient.Close()

	// 检查Redis连接
	ctx := context.Background()
	if err := redisClient.Ping(ctx); err != nil {
		logger.WithError(err).Fatal("Failed to connect to Redis")
	}

	// 初始化Kafka生产者
	feedEventsProducer := queue.NewKafkaProducer(cfg.Kafka.Brokers, cfg.Kafka.Topics.FeedEvents)
	defer feedEventsProducer.Close()

	userEventsProducer := queue.NewKafkaProducer(cfg.Kafka.Brokers, cfg.Kafka.Topics.UserEvents)
	defer userEventsProducer.Close()

	// 初始化Kafka消费者
	feedEventsConsumer := queue.NewKafkaConsumer(cfg.Kafka.Brokers, cfg.Kafka.Topics.FeedEvents, "feed-worker-group")
	defer feedEventsConsumer.Close()

	// 初始化仓库
	userRepo := repository.NewUserRepository(db.DB)
	followRepo := repository.NewFollowRepository(db.DB)
	postRepo := repository.NewPostRepository(db.DB)
	timelineRepo := repository.NewTimelineRepository(db.DB)
	likeRepo := repository.NewLikeRepository(db.DB)
	commentRepo := repository.NewCommentRepository(db.DB)

	// 初始化服务
	userService := services.NewUserService(userRepo, followRepo, userEventsProducer, logger)
	feedService := services.NewFeedService(postRepo, timelineRepo, userRepo, followRepo, likeRepo, commentRepo, redisClient, feedEventsProducer, &cfg.Feed, logger)
	likeService := services.NewLikeService(postRepo, likeRepo, userRepo, feedEventsProducer, logger)
	commentService := services.NewCommentService(postRepo, commentRepo, userRepo, feedEventsProducer, logger)

	// 初始化优化版服务（新增）
	activityService := services.NewActivityService(userRepo, redisClient, logger)
	timelineCacheService := services.NewTimelineCacheService(redisClient, logger)
	cacheStrategyService := services.NewCacheStrategyService(redisClient, &cfg.Feed, logger, activityService, timelineCacheService)
	recoveryService := services.NewRecoveryService(postRepo, userRepo, followRepo, redisClient, logger, activityService, timelineCacheService)
	optimizedFeedService := services.NewOptimizedFeedService(postRepo, timelineRepo, userRepo, followRepo, likeRepo, commentRepo, redisClient, feedEventsProducer, &cfg.Feed, logger, activityService, timelineCacheService)

	// 初始化工作处理器（原版）
	feedWorker := workers.NewFeedWorker(feedService, userService, postRepo, timelineRepo, followRepo, userRepo, redisClient, feedEventsConsumer, logger)

	// 初始化优化版工作处理器（新增）
	optimizedFeedWorker := workers.NewOptimizedFeedWorker(feedEventsConsumer, logger, cfg, activityService, timelineCacheService, cacheStrategyService, recoveryService, optimizedFeedService)

	// 启动工作处理器
	go func() {
		if err := feedWorker.Start(ctx); err != nil {
			logger.WithError(err).Error("Feed worker stopped with error")
		}
	}()

	// 启动优化版工作处理器（新增）
	go func() {
		if err := optimizedFeedWorker.Start(ctx); err != nil {
			logger.WithError(err).Error("Optimized feed worker stopped with error")
		}
	}()

	// 初始化处理器
	userHandler := handlers.NewUserHandler(userService, cfg.JWT.Secret)
	feedHandler := handlers.NewFeedHandler(feedService, likeService, commentService)

	// 初始化优化版处理器（新增）
	optimizedFeedHandler := handlers.NewOptimizedFeedHandler(optimizedFeedService, activityService, cacheStrategyService, recoveryService, logger)

	// 设置Gin模式
	if cfg.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	// 创建路由
	router := gin.New()
	router.Use(gin.Recovery())

	// 添加CORS中间件
	router.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// 健康检查
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "healthy",
			"time":   time.Now().Unix(),
		})
	})

	// API路由
	api := router.Group("/api/v1")
	{
		// 用户相关路由
		users := api.Group("/users")
		{
			users.POST("/register", userHandler.Register)
			users.POST("/login", userHandler.Login)
			users.GET("/search", userHandler.SearchUsers)
			users.GET("/:id", userHandler.GetProfile)
			users.GET("/:id/followers", userHandler.GetFollowers)
			users.GET("/:id/following", userHandler.GetFollowing)
		}

		// 需要认证的路由（原版API）
		protected := api.Group("")
		protected.Use(middleware.NewJWTAuth(&middleware.JWTConfig{Secret: cfg.JWT.Secret}))
		{
			// 用户相关
			protected.PUT("/users/profile", userHandler.UpdateProfile)
			protected.POST("/users/follow", userHandler.Follow)
			protected.DELETE("/users/unfollow/:id", userHandler.Unfollow)

			// Feed相关（原版）
			protected.POST("/posts", feedHandler.CreatePost)
			protected.GET("/feed", feedHandler.GetFeed)
			protected.GET("/users/:id/posts", feedHandler.GetUserPosts)
			protected.GET("/posts/:id", feedHandler.GetPost)
			protected.DELETE("/posts/:id", feedHandler.DeletePost)
			protected.POST("/posts/:id/like", feedHandler.LikePost)
			protected.DELETE("/posts/:id/like", feedHandler.UnlikePost)
			protected.GET("/posts/:id/likes", feedHandler.GetPostLikes)
			protected.POST("/posts/:id/comments", feedHandler.CreateComment)
			protected.GET("/posts/:id/comments", feedHandler.GetPostComments)
			protected.DELETE("/comments/:id", feedHandler.DeleteComment)
			protected.GET("/posts/search", feedHandler.SearchPosts)
		}
	}

	// 优化版API路由（新增）
	apiV2 := router.Group("/api/v2")
	{
		jwtConfig := &middleware.JWTConfig{Secret: cfg.JWT.Secret}
		optimizedFeedHandler.RegisterRoutes(apiV2, jwtConfig)
	}

	// 创建HTTP服务器
	srv := &http.Server{
		Addr:         cfg.Server.Port,
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	// 启动服务器
	go func() {
		logger.WithField("port", cfg.Server.Port).Info("Starting HTTP server")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.WithError(err).Fatal("Failed to start HTTP server")
		}
	}()

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	// 优雅关闭
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.WithError(err).Error("Server forced to shutdown")
	}

	if err := feedWorker.Stop(); err != nil {
		logger.WithError(err).Error("Failed to stop feed worker")
	}

	if err := optimizedFeedWorker.Stop(ctx); err != nil {
		logger.WithError(err).Error("Failed to stop optimized feed worker")
	}

	logger.Info("Server exited")
}

func init() {
	// 创建必要的目录
	dirs := []string{"logs", "uploads", "configs"}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Printf("Failed to create directory %s: %v", dir, err)
		}
	}

	// 创建默认配置文件（如果不存在）
	configPath := "configs/config.yaml"
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		if err := createDefaultConfig(configPath); err != nil {
			log.Printf("Failed to create default config: %v", err)
		}
	}
}

func createDefaultConfig(path string) error {
	defaultConfig := `server:
  port: ":8080"
  mode: "debug"
  read_timeout: 30s
  write_timeout: 30s

database:
  host: "localhost"
  port: 5432
  user: "feeduser"
  password: "feedpass"
  dbname: "feedsystem"
  sslmode: "disable"
  max_open_conns: 100
  max_idle_conns: 10

redis:
  host: "localhost"
  port: 6379
  password: ""
  db: 0
  pool_size: 100
  min_idle_conns: 10

kafka:
  brokers:
    - "localhost:9092"
  topics:
    user_events: "user-events"
    feed_events: "feed-events"
    feed_updates: "feed-updates"

jwt:
  secret: "your-secret-key-change-in-production"
  expire_time: 24h

feed:
  push_threshold: 5000  # 小于5000粉丝使用推模式，大于使用拉模式
  cache_ttl: 1h
  max_feed_size: 1000   # 单个用户feed最大容量
  rank_update_interval: 5m`

	return os.WriteFile(path, []byte(defaultConfig), 0644)
}
