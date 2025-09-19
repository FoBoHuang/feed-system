package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/feed-system/feed-system/internal/config"
	"github.com/feed-system/feed-system/internal/repository"
	"github.com/feed-system/feed-system/internal/services"
	"github.com/feed-system/feed-system/internal/workers"
	"github.com/feed-system/feed-system/pkg/cache"
	"github.com/feed-system/feed-system/pkg/logger"
	"github.com/feed-system/feed-system/pkg/queue"
)

func main() {
	// 加载配置
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 初始化日志
	logger := logger.NewLogger()
	logger.Info("Starting Feed System Worker...")

	// 初始化数据库
	db, err := repository.NewDatabase(&cfg.Database)
	if err != nil {
		logger.WithError(err).Fatal("Failed to connect to database")
	}
	defer db.Close()

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

	// 初始化Kafka消费者
	feedEventsConsumer := queue.NewKafkaConsumer(cfg.Kafka.Brokers, cfg.Kafka.Topics.FeedEvents, "feed-worker-group")
	defer feedEventsConsumer.Close()

	// 初始化Kafka生产者（用于处理过程中的事件发布）
	feedEventsProducer := queue.NewKafkaProducer(cfg.Kafka.Brokers, cfg.Kafka.Topics.FeedEvents)
	defer feedEventsProducer.Close()

	// 初始化仓库
	userRepo := repository.NewUserRepository(db.DB)
	followRepo := repository.NewFollowRepository(db.DB)
	postRepo := repository.NewPostRepository(db.DB)
	timelineRepo := repository.NewTimelineRepository(db.DB)
	likeRepo := repository.NewLikeRepository(db.DB)
	commentRepo := repository.NewCommentRepository(db.DB)

	// 初始化服务
	userService := services.NewUserService(userRepo, followRepo, feedEventsProducer, logger)
	feedService := services.NewFeedService(postRepo, timelineRepo, userRepo, followRepo, likeRepo, commentRepo, redisClient, feedEventsProducer, &cfg.Feed, logger)

	// 初始化工作处理器
	feedWorker := workers.NewFeedWorker(feedService, userService, postRepo, timelineRepo, followRepo, userRepo, redisClient, feedEventsConsumer, logger)

	// 启动工作处理器
	logger.Info("Starting feed worker...")
	go func() {
		if err := feedWorker.Start(ctx); err != nil {
			logger.WithError(err).Error("Feed worker stopped with error")
		}
	}()

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down worker...")

	// 优雅关闭
	_, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := feedWorker.Stop(); err != nil {
		logger.WithError(err).Error("Failed to stop feed worker")
	}

	logger.Info("Worker exited")
}