package config

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Redis    RedisConfig    `mapstructure:"redis"`
	Kafka    KafkaConfig    `mapstructure:"kafka"`
	JWT      JWTConfig      `mapstructure:"jwt"`
	Feed     FeedConfig     `mapstructure:"feed"`
}

type ServerConfig struct {
	Port         string        `mapstructure:"port"`
	Mode         string        `mapstructure:"mode"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
}

type DatabaseConfig struct {
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	User         string `mapstructure:"user"`
	Password     string `mapstructure:"password"`
	DBName       string `mapstructure:"dbname"`
	SSLMode      string `mapstructure:"sslmode"`
	MaxOpenConns int    `mapstructure:"max_open_conns"`
	MaxIdleConns int    `mapstructure:"max_idle_conns"`
}

type RedisConfig struct {
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	Password     string `mapstructure:"password"`
	DB           int    `mapstructure:"db"`
	PoolSize     int    `mapstructure:"pool_size"`
	MinIdleConns int    `mapstructure:"min_idle_conns"`
}

type KafkaConfig struct {
	Brokers []string `mapstructure:"brokers"`
	Topics  Topics   `mapstructure:"topics"`
}

type Topics struct {
	UserEvents  string `mapstructure:"user_events"`
	FeedEvents  string `mapstructure:"feed_events"`
	FeedUpdates string `mapstructure:"feed_updates"`
}

type JWTConfig struct {
	Secret     string        `mapstructure:"secret"`
	ExpireTime time.Duration `mapstructure:"expire_time"`
}

type FeedConfig struct {
	PushThreshold      int                `mapstructure:"push_threshold"` // 推模式阈值
	CacheTTL           time.Duration      `mapstructure:"cache_ttl"`
	MaxFeedSize        int                `mapstructure:"max_feed_size"`
	RankUpdateInterval time.Duration      `mapstructure:"rank_update_interval"`
	Optimization       OptimizationConfig `mapstructure:"optimization"` // 优化配置
}

// OptimizationConfig 优化配置
type OptimizationConfig struct {
	ActiveUser    UserCacheConfig `mapstructure:"active_user"`
	InactiveUser  UserCacheConfig `mapstructure:"inactive_user"`
	VIPUser       UserCacheConfig `mapstructure:"vip_user"`
	Recovery      RecoveryConfig  `mapstructure:"recovery"`
	CacheCleanup  CleanupConfig   `mapstructure:"cache_cleanup"`
	ActivityDecay DecayConfig     `mapstructure:"activity_decay"`
	Timeline      TimelineConfig  `mapstructure:"timeline"`
}

// UserCacheConfig 用户缓存配置
type UserCacheConfig struct {
	ScoreThreshold   float64 `mapstructure:"score_threshold"`
	CacheHours       int     `mapstructure:"cache_hours"`
	MaxTimelineItems int     `mapstructure:"max_timeline_items"`
}

// RecoveryConfig 崩溃恢复配置
type RecoveryConfig struct {
	CheckInterval int `mapstructure:"check_interval"`
	TaskTimeout   int `mapstructure:"task_timeout"`
}

// CleanupConfig 缓存清理配置
type CleanupConfig struct {
	Interval  int `mapstructure:"interval"`
	BatchSize int `mapstructure:"batch_size"`
}

// DecayConfig 活跃度衰减配置
type DecayConfig struct {
	DecayFactor float64 `mapstructure:"decay_factor"`
	Interval    int     `mapstructure:"interval"`
	MaxScore    float64 `mapstructure:"max_score"`
}

// TimelineConfig Timeline配置
type TimelineConfig struct {
	DefaultTTL      int `mapstructure:"default_ttl"`
	MaxItems        int `mapstructure:"max_items"`
	CleanupInterval int `mapstructure:"cleanup_interval"`
}

func LoadConfig() (*Config, error) {
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "configs/config.yaml"
	}

	viper.SetConfigFile(configPath)
	viper.SetConfigType("yaml")

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &config, nil
}

func (c *DatabaseConfig) DSN() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.DBName, c.SSLMode)
}

func (c *RedisConfig) Addr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}
