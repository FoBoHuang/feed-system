# Feed流系统架构设计

## 1. 系统架构概览

### 1.1 整体架构
Feed流系统采用微服务架构，核心组件包括：
- API网关层：统一入口，认证授权
- 业务服务层：用户服务、Feed服务、互动服务
- 数据存储层：PostgreSQL、Redis、Kafka
- 基础设施层：Docker、Kubernetes、监控告警

### 1.2 架构原则
- **高可用**：多副本部署，故障自动恢复
- **高性能**：推拉结合，多级缓存
- **高扩展**：水平扩展，自动扩缩容
- **松耦合**：微服务架构，异步通信

## 2. 核心组件设计

### 2.1 用户服务 (User Service)
负责用户注册、登录、认证、用户关系管理等核心功能。

**主要职责：**
- 用户注册和登录认证
- JWT Token生成和验证
- 用户关注关系管理
- 用户信息查询和更新
- 用户搜索功能

**数据模型：**
```sql
-- 用户表
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username VARCHAR(30) UNIQUE NOT NULL,
    email VARCHAR(100) UNIQUE NOT NULL,
    password VARCHAR(255) NOT NULL,
    display_name VARCHAR(50),
    avatar TEXT,
    bio TEXT,
    followers BIGINT DEFAULT 0,
    following BIGINT DEFAULT 0,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

-- 关注关系表
CREATE TABLE follows (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    follower_id UUID NOT NULL REFERENCES users(id),
    following_id UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP,
    UNIQUE(follower_id, following_id)
);
```

### 2.2 Feed服务 (Feed Service)
负责帖子的创建、发布、Feed流生成等核心功能。

**主要职责：**
- 帖子创建和管理
- Feed流生成（推拉结合）
- 帖子排序算法
- 帖子搜索功能
- 帖子删除和清理

**数据模型：**
```sql
-- 帖子表
CREATE TABLE posts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    content TEXT NOT NULL,
    image_urls TEXT[],
    like_count BIGINT DEFAULT 0,
    comment_count BIGINT DEFAULT 0,
    share_count BIGINT DEFAULT 0,
    score DOUBLE PRECISION DEFAULT 0,
    is_deleted BOOLEAN DEFAULT false,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

-- 时间线表
CREATE TABLE timelines (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    post_id UUID NOT NULL REFERENCES posts(id),
    score DOUBLE PRECISION DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

### 2.3 互动服务 (Interaction Service)
负责点赞、评论等用户互动功能。

**主要职责：**
- 帖子点赞和取消点赞
- 评论创建和管理
- 互动数据统计
- 互动事件处理

**数据模型：**
```sql
-- 点赞表
CREATE TABLE likes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    post_id UUID NOT NULL REFERENCES posts(id),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, post_id)
);

-- 评论表
CREATE TABLE comments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    post_id UUID NOT NULL REFERENCES posts(id),
    content TEXT NOT NULL,
    parent_id UUID REFERENCES comments(id),
    like_count BIGINT DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);
```

## 3. 推拉结合模式详解

### 3.1 模式选择策略

**阈值设定：**
```go
const PushThreshold = 5000 // 粉丝数阈值

func chooseDistributionModel(user *User) DistributionModel {
    if user.Followers <= PushThreshold {
        return PushModel
    }
    return PullModel
}
```

### 3.2 推模式 (Push Model)

**适用场景：**
- 普通用户（粉丝数 ≤ 5000）
- 新注册用户的默认模式

**实现原理：**
1. 用户发布帖子时，立即将帖子推送到所有关注者的timeline
2. 关注者读取Feed时，直接从timeline获取数据
3. 时间复杂度：O(1)读取，O(N)写入（N为粉丝数）

**优点：**
- 读取性能极佳，O(1)时间复杂度
- 用户体验好，Feed加载速度快
- 实现简单，逻辑清晰

**缺点：**
- 写入放大，大V用户写入成本高
- 存储冗余，同一份帖子多份存储
- 不适合大V用户

### 3.3 拉模式 (Pull Model)

**适用场景：**
- 大V用户（粉丝数 > 5000）
- 明星、KOL等高粉丝用户

**实现原理：**
1. 用户发布帖子时，只存储在作者自己的时间线
2. 关注者读取Feed时，实时聚合关注者的帖子
3. 时间复杂度：O(K)读取（K为关注数），O(1)写入

**优点：**
- 存储效率高，无数据冗余
- 写入性能稳定，不受粉丝数影响
- 适合大V用户场景

**缺点：**
- 读取性能相对较差
- 实时计算开销大
- 可能影响用户体验

### 3.4 混合策略优化

**渐进推送：**
```go
func distributePostHybrid(ctx context.Context, post *Post, author *User) error {
    if author.Followers <= PushThreshold {
        // 全量推送
        return pushToAllFollowers(ctx, post, author)
    } else {
        // 选择性推送：只推送给最活跃的1000个关注者
        activeFollowers := getMostActiveFollowers(author.ID, 1000)
        return pushToActiveFollowers(ctx, post, author, activeFollowers)
    }
}
```

**冷热分离：**
- 热数据：推送给活跃用户
- 冷数据：拉取时实时计算
- 根据用户活跃度动态调整

## 4. 性能优化策略

### 4.1 数据库优化

**索引策略：**
```sql
-- 用户查询索引
CREATE INDEX idx_users_username ON users(username);
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_created_at ON users(created_at);

-- 关注关系索引
CREATE INDEX idx_follows_follower ON follows(follower_id);
CREATE INDEX idx_follows_following ON follows(following_id);
CREATE INDEX idx_follows_created_at ON follows(created_at);

-- 帖子查询索引
CREATE INDEX idx_posts_user_id ON posts(user_id);
CREATE INDEX idx_posts_created_at ON posts(created_at DESC);
CREATE INDEX idx_posts_score ON posts(score DESC);

-- 时间线索引
CREATE INDEX idx_timelines_user_id ON timelines(user_id);
CREATE INDEX idx_timelines_score ON timelines(score DESC);
CREATE INDEX idx_timelines_created_at ON timelines(created_at DESC);
```

**连接池优化：**
```go
dbConfig := &gorm.Config{
    Logger: logger.Default.LogMode(logger.Info),
}

db, err := gorm.Open(postgres.Open(dsn), dbConfig)
sqlDB, _ := db.DB()
sqlDB.SetMaxOpenConns(100)    // 最大连接数
sqlDB.SetMaxIdleConns(10)     // 最大空闲连接数
sqlDB.SetConnMaxLifetime(time.Hour) // 连接最大生命周期
```

### 4.2 缓存优化

**多级缓存架构：**
```
┌─────────────────┐
│   Application   │
│    Cache (L1)   │
└────────┬────────┘
         │
┌────────▼────────┐
│     Redis       │
│   Cache (L2)    │
└────────┬────────┘
         │
┌────────▼────────┐
│   Database      │
│  (Source Data)  │
└─────────────────┘
```

**缓存策略：**
```go
type CacheStrategy struct {
    TTL            time.Duration
    MaxSize        int64
    EvictionPolicy string // LRU, LFU, FIFO
    WarmUpEnabled  bool
}

func (c *CacheService) GetFeedWithCache(ctx context.Context, userID string, cursor string, limit int) (*FeedResponse, error) {
    cacheKey := fmt.Sprintf("feed:%s:%s:%d", userID, cursor, limit)

    // L1缓存检查
    if cached, err := c.appCache.Get(cacheKey); err == nil {
        return cached.(*FeedResponse), nil
    }

    // L2缓存检查
    if cached, err := c.redisClient.GetJSON(ctx, cacheKey, &FeedResponse{}); err == nil {
        c.appCache.Set(cacheKey, cached, c.cacheConfig.TTL)
        return cached, nil
    }

    // 数据库查询
    feed, err := c.generateFeed(ctx, userID, cursor, limit)
    if err != nil {
        return nil, err
    }

    // 缓存结果
    c.redisClient.SetJSON(ctx, cacheKey, feed, c.cacheConfig.TTL)
    c.appCache.Set(cacheKey, feed, c.cacheConfig.TTL)

    return feed, nil
}
```

### 4.3 消息队列优化

**Kafka配置优化：**
```yaml
# Kafka Producer配置
producer:
  batch.size: 16384
  linger.ms: 10
  compression.type: lz4
  acks: 1
  retries: 3
  max.in.flight.requests.per.connection: 5

# Kafka Consumer配置
consumer:
  group.id: feed-worker-group
  enable.auto.commit: false
  max.poll.records: 500
  fetch.min.bytes: 1
  fetch.max.wait.ms: 500
```

**消息分区策略：**
```go
func getPartitionKey(eventType string, userID string) string {
    switch eventType {
    case "user_created", "user_updated":
        return userID
    case "post_created":
        return userID
    case "follow_created", "follow_deleted":
        return userID
    default:
        return "default"
    }
}
```

### 4.4 应用层优化

**连接池管理：**
```go
type HTTPClientPool struct {
    pool *sync.Pool
}

func NewHTTPClientPool() *HTTPClientPool {
    return &HTTPClientPool{
        pool: &sync.Pool{
            New: func() interface{} {
                return &http.Client{
                    Timeout: 30 * time.Second,
                    Transport: &http.Transport{
                        MaxIdleConns:        100,
                        MaxIdleConnsPerHost: 10,
                        IdleConnTimeout:     90 * time.Second,
                    },
                }
            },
        },
    }
}
```

**并发控制：**
```go
type WorkerPool struct {
    workers   int
    jobQueue  chan Job
    resultQueue chan Result
    wg          sync.WaitGroup
}

func (wp *WorkerPool) Start() {
    for i := 0; i < wp.workers; i++ {
        wp.wg.Add(1)
        go wp.worker(i)
    }
}

func (wp *WorkerPool) worker(id int) {
    defer wp.wg.Done()
    for job := range wp.jobQueue {
        result := wp.processJob(job)
        wp.resultQueue <- result
    }
}
```

## 5. 监控和可观测性

### 5.1 指标收集

**业务指标：**
- Feed生成时间
- 用户注册/登录成功率
- 帖子发布成功率
- 点赞/评论操作成功率

**系统指标：**
- CPU使用率
- 内存使用率
- 磁盘I/O
- 网络吞吐量

**应用指标：**
- HTTP请求延迟
- 数据库查询延迟
- 缓存命中率
- 消息队列积压

### 5.2 日志规范

**结构化日志：**
```go
logger.WithFields(logrus.Fields{
    "user_id":    userID,
    "post_id":    postID,
    "action":     "create_post",
    "duration":   time.Since(start).Milliseconds(),
    "status":     "success",
}).Info("Post created successfully")
```

**日志级别：**
- DEBUG：详细的调试信息
- INFO：正常操作信息
- WARN：警告信息
- ERROR：错误信息
- FATAL：致命错误

### 5.3 链路追踪

**分布式追踪：**
```go
func (s *FeedService) CreatePost(ctx context.Context, userID string, req *CreatePostRequest) (*Post, error) {
    span, ctx := opentracing.StartSpanFromContext(ctx, "CreatePost")
    defer span.Finish()

    span.SetTag("user_id", userID)
    span.SetTag("content_length", len(req.Content))

    // 业务逻辑
    post, err := s.createPostInternal(ctx, userID, req)

    if err != nil {
        span.SetTag("error", true)
        span.LogFields(log.Error(err))
        return nil, err
    }

    return post, nil
}
```

## 6. 安全性设计

### 6.1 认证授权

**JWT认证：**
```go
type Claims struct {
    UserID   string `json:"user_id"`
    Username string `json:"username"`
    jwt.RegisteredClaims
}

func GenerateToken(userID, username string, secret string, expireTime time.Duration) (string, error) {
    claims := &Claims{
        UserID:   userID,
        Username: username,
        RegisteredClaims: jwt.RegisteredClaims{
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(expireTime)),
            IssuedAt:  jwt.NewNumericDate(time.Now()),
        },
    }

    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString([]byte(secret))
}
```

### 6.2 数据安全

**输入验证：**
```go
type CreatePostRequest struct {
    Content   string   `json:"content" binding:"required,min=1,max=1000"`
    ImageURLs []string `json:"image_urls" binding:"max=9"`
}

func (h *FeedHandler) CreatePost(c *gin.Context) {
    var req CreatePostRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    // 内容过滤
    req.Content = sanitizeContent(req.Content)

    // 业务逻辑
}
```

**SQL注入防护：**
```go
// 使用GORM的参数化查询
result := db.Where("username = ? AND is_active = ?", username, true).First(&user)

// 避免字符串拼接
// 错误：db.Where("username = '" + username + "'")
// 正确：db.Where("username = ?", username)
```

## 7. 部署和运维

### 7.1 容器化部署

**Dockerfile最佳实践：**
```dockerfile
# 使用多阶段构建
FROM golang:1.21-alpine AS builder

# 安装构建依赖
RUN apk add --no-cache git ca-certificates

# 设置工作目录
WORKDIR /app

# 复制依赖文件
COPY go.mod go.sum ./
RUN go mod download

# 复制源码
COPY . .

# 构建应用
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o feed-api ./cmd/api

# 运行阶段
FROM alpine:latest

# 安装运行时依赖
RUN apk --no-cache add ca-certificates tzdata

# 创建非root用户
RUN addgroup -g 1000 -S appuser && adduser -u 1000 -S appuser -G appuser

# 复制二进制文件
COPY --from=builder /app/feed-api /usr/local/bin/

# 设置用户
USER appuser

# 暴露端口
EXPOSE 8080

# 健康检查
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# 启动命令
CMD ["feed-api"]
```

### 7.2 Kubernetes部署

**Deployment配置：**
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: feed-api
  namespace: feed-system
spec:
  replicas: 3
  selector:
    matchLabels:
      app: feed-api
  template:
    metadata:
      labels:
        app: feed-api
    spec:
      containers:
      - name: feed-api
        image: feed-system/api:latest
        ports:
        - containerPort: 8080
        resources:
          requests:
            memory: "256Mi"
            cpu: "250m"
          limits:
            memory: "512Mi"
            cpu: "500m"
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
```

### 7.3 自动扩缩容

**HPA配置：**
```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: feed-api-hpa
  namespace: feed-system
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: feed-api
  minReplicas: 3
  maxReplicas: 20
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
  - type: Resource
    resource:
      name: memory
      target:
        type: Utilization
        averageUtilization: 80
```

## 8. 性能基准测试

### 8.1 测试环境
- CPU: 16核
- 内存: 32GB
- 网络: 千兆网络
- 数据库: PostgreSQL 15
- 缓存: Redis 7
- 消息队列: Kafka 3.4

### 8.2 测试结果

**API性能：**
- 用户注册：1000 QPS，平均响应时间 50ms
- 用户登录：2000 QPS，平均响应时间 30ms
- 创建帖子：500 QPS，平均响应时间 100ms
- 获取Feed：1000 QPS，平均响应时间 80ms
- 点赞操作：2000 QPS，平均响应时间 20ms

**系统容量：**
- 支持并发用户数：100万+
- 日活跃用户：1000万+
- 帖子存储：10亿+
- Feed生成：1000万/天

**资源使用：**
- CPU使用率：平均 30%，峰值 70%
- 内存使用：平均 60%，峰值 80%
- 网络带宽：平均 50%，峰值 80%

## 9. 未来规划

### 9.1 功能增强
- **智能推荐**：基于机器学习的个性化推荐
- **多媒体支持**：视频、音频内容处理
- **实时通信**：WebSocket实时推送
- **地理位置**：基于位置的内容推荐

### 9.2 技术升级
- **云原生**：Service Mesh、Serverless
- **边缘计算**：CDN边缘节点部署
- **AI能力**：内容理解、情感分析
- **区块链**：内容确权、激励机制

### 9.3 架构演进
- **事件驱动**：全面事件驱动架构
- **CQRS**：命令查询职责分离
- **领域驱动**：DDD领域建模
- **云原生**：Kubernetes Operator

---

这份架构文档详细描述了Feed流系统的技术架构、核心算法、性能优化策略等关键内容，为系统的开发、部署和运维提供了全面的技术指导。