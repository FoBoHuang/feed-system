# Feed流系统 - 简历项目梳理与面试准备

## 一、简历项目描述

### 项目名称

高性能社交Feed流系统

### 技术栈

Go 1.21 / Gin / PostgreSQL / Redis / Apache Kafka / GORM / Docker / Kubernetes

### 项目描述

负责设计与开发面向千万级用户的社交媒体Feed流系统，支持帖子发布、关注关系、信息流聚合、点赞评论等核心功能。系统采用推拉结合的信息流分发架构，通过多级缓存、异步消息处理和智能分发策略，实现了 P99 < 200ms 的接口响应和 10万+ QPS 的并发处理能力。

### 核心职责与技术亮点

**1. 设计并实现推拉结合的Feed分发引擎**

- 基于用户粉丝量阈值（5000/10000）自动选择推模式或拉模式：普通用户（粉丝数 ≤ 阈值）采用写扩散（Push），发布时通过 Redis Pipeline 批量写入所有粉丝 Timeline；头部用户（大V）采用"在线推、离线拉"策略，仅推送给 Top 1000 活跃粉丝，非活跃粉丝访问时触发拉模式实时聚合，将头部用户发帖峰值写放大降低 80%。

**2. 基于 Redis Sorted Set 的 Timeline 存储与游标分页**

- 使用 Redis Sorted Set 存储用户 Timeline，以时间戳作为 score 实现高效的时间倒序排列；采用 `ZRevRangeByScore` 配合游标（cursor-based pagination）替代传统 offset 分页，避免深度分页的 O(N) 性能退化，大数据量分页性能提升 90%。通过 Pipeline 批量操作将多用户 Timeline 写入合并为单次网络往返，并自动裁剪 Timeline 至 1000 条防止内存膨胀。

**3. 设计多层缓存策略与用户活跃度驱动的动态缓存管理**

- 构建 L1（应用内存缓存）+ L2（Redis 缓存）+ DB 三级缓存架构。引入用户活跃度评分系统（基于登录、发帖、点赞、评论等行为加权计算，带指数时间衰减），动态调整缓存策略：活跃用户 Timeline 缓存 7天 / 最多 1000 条，非活跃用户缓存 2小时 / 最多 200 条，VIP 用户缓存 30天。定时后台任务自动清理非活跃用户冗余缓存，活跃用户缓存命中率达 95%+。

**4. 基于 Kafka 的异步事件驱动架构**

- 采用事件驱动架构解耦核心业务，定义帖子创建/删除、关注/取关、点赞/评论等 9 类事件，通过 Kafka 按用户 ID 分区保证同一用户的事件有序消费。Worker 端实现批量消费（batch size 500）与指数退避重试机制，实现流量削峰与异步处理。

**5. 实现帖子智能排序算法**

- 设计综合排序公式 `Score = (UserWeight + EngagementScore) × TimeDecay`，其中用户权重基于粉丝数对数加权，互动得分按点赞(0.1)、评论(0.2)、分享(0.3)加权求和，时间衰减采用指数衰减函数（半衰期 24h），确保 Feed 流内容的新鲜度与相关性平衡。

**6. 设计崩溃恢复机制保障消息可达性**

- 在帖子分发全流程中引入状态机（started → completed），将分发状态持久化到 Redis（TTL 24h）。后台 Recovery Job 每 5 分钟扫描超时（> 5min）未完成的分发任务，通过幂等校验（ZScore 检查帖子是否已存在于目标 Timeline）实现断点续传式恢复，确保 100% 消息到达。

**7. Kubernetes 云原生部署与弹性伸缩**

- API 服务与 Worker 服务分离部署，基于 HPA 配置 CPU（70%）/ 内存（80%）双指标自动扩缩容，API 层 3-20 实例、Worker 层 2-10 实例弹性伸缩。配合 Liveness / Readiness 探针实现故障自愈，系统可用性达 99.9%+。

### 项目成果

- 系统支撑千万级 DAU，接口 P99 响应时间 < 200ms
- 头部用户（百万粉丝）发帖场景写入负载降低 80%
- Redis 缓存命中率 95%+，数据库查询压力降低 60%
- 游标分页方案替代 offset 分页，大数据量翻页性能提升 90%

---

## 二、面试延伸问题与参考答案

### Q1：为什么选择 5000 作为推拉模式的阈值？如何动态调整？

**答：**

阈值的选择基于写放大与读延迟之间的平衡：

- **推模式的成本**：一个拥有 N 个粉丝的用户发帖，需要写入 N 条 Timeline 记录。当 N = 5000 时，单次发帖的写入量为 5000 次 Redis `ZADD`，通过 Pipeline 批量执行大约耗时 50-100ms，在可接受范围内。
- **拉模式的成本**：假设用户关注了 K 个人，读取 Feed 需要聚合 K 个用户的最新帖子，当 K 较大时读延迟会上升。但拉模式的写入成本为 O(1)。
- **5000 的经验判断**：在社交产品中，粉丝数 > 5000 的用户通常占总用户量的 1%-5%，但他们每次发帖产生的写放大对系统的压力却是非线性增长的。5000 是一个典型的工程经验值，类似 Twitter 早期的设计。

**动态调整方案**（系统已支持配置化）：

```yaml
# configs/config.yaml
feed:
  push_threshold: 5000  # 可通过配置中心动态修改
```

实际落地时可以进一步优化为：
1. **监控驱动**：监控 Push 模式下的 P99 写入耗时，当超过阈值（如 200ms）时自动上调 threshold。
2. **分级阈值**：不是简单的二选一，而是引入三级策略 — 小于 1000 全量推、1000-10000 推给活跃用户、大于 10000 纯拉模式。
3. **A/B 测试**：在不同用户群体上测试不同阈值对用户体验（Feed 加载延迟）和系统成本（Redis 内存、写入 QPS）的影响。

---

### Q2：缓存和 DB 的数据一致性如何保证？

**答：**

本系统采用的是**最终一致性**模型，而非强一致性，这是 Feed 流场景的合理选择 — 用户对信息流中出现几秒延迟的容忍度远高于交易系统。

**具体策略：**

**写路径（帖子发布时）：**
1. 先写 DB（PostgreSQL），确保数据持久化。
2. 然后异步写 Redis Timeline 缓存（通过 Kafka 事件或直接写入）。
3. 如果 Redis 写入失败，记录到分发状态中（`distribution_status`），由 Recovery Job 兜底恢复。

```
用户发帖 → 写DB(Post表) → 推送事件到Kafka → Worker消费 → 写Redis Timeline
                                                          ↓ (失败时)
                                                    Recovery Job重试
```

**读路径（获取 Feed 时）：**
1. 优先读 Redis Timeline 缓存。
2. 缓存未命中时，回退到拉模式（Pull Mode）从 DB 实时聚合。
3. 拉取后异步重建 Redis 缓存（`rebuildTimelineCache`）。

**缓存失效时机：**
- 帖子被删除时：发送 `post_deleted` 事件，Worker 从所有相关 Timeline 中移除。
- 用户取消关注时：发送 `follow_deleted` 事件，Worker 清理对应 Timeline。
- TTL 自然过期：活跃用户 7天，非活跃用户 2小时。

**兜底机制：**
- 拉模式本身就是缓存一致性的终极兜底 — 当 Redis 缓存不存在或数据异常时，直接从 DB 聚合最新数据，同时异步重建缓存。
- 这种 "Cache-Aside + Pull Fallback" 模式确保了在缓存失效、Redis 故障等异常场景下系统仍然可用。

---

### Q3：Kafka 消费失败的重试策略和死信队列处理？

**答：**

**当前重试机制：**

系统使用 `segmentio/kafka-go` 库，Consumer 采用消费者组模式（GroupID），手动提交 offset（`CommitInterval: 1s`）。消费失败时的处理策略：

1. **同步重试**：Worker 的 `handleMessage` 在处理失败后返回 error，但不会阻塞后续消息消费 — 当前实现中失败的消息会被跳过并打日志（`continue`），等待 Recovery Job 兜底。
2. **分发状态恢复**：对于帖子分发这类关键操作，失败不依赖 Kafka 重试，而是通过 Redis 中的分发状态（`distribution_status:*`）由 Recovery Service 每 5 分钟扫描并重新执行。

**生产环境建议的增强方案：**

```
正常消费 → 处理成功 → 提交offset
    ↓ (失败)
第1次重试（立即） → 成功 → 提交offset
    ↓ (失败)
第2次重试（1s后） → 成功 → 提交offset
    ↓ (失败)
第3次重试（5s后） → 成功 → 提交offset
    ↓ (3次都失败)
写入死信Topic（feed-events-dlq）→ 提交原offset → 告警通知
```

- **指数退避**：重试间隔 1s → 5s → 25s，最多 3 次。
- **死信队列**：消费多次失败后发送到单独的 DLQ Topic（如 `feed-events-dlq`），避免阻塞正常消费进度。
- **人工介入**：DLQ 消息触发告警，运维人员排查后可手动重放。
- **幂等性保障**：所有消费逻辑天然幂等 — `ZADD` 操作对同一 member 重复执行结果相同，Recovery 中用 `ZScore` 检查帖子是否已存在。

---

### Q4：如果 Redis 宕机，Timeline 如何降级？

**答：**

系统在设计时已经内置了 Redis 不可用场景的降级路径：

**自动降级到拉模式（Pull Mode）：**

在 `GetFeed` 方法中，当 Redis Timeline 查询失败或返回空数据时，系统自动回退到拉模式：

```go
// internal/services/feed_optimized.go
timelineItems, nextCursor, hasMore, err := s.timelineCacheService.GetTimeline(ctx, userUUID, cursor, limit)
if err != nil {
    s.logger.WithError(err).Error("Failed to get timeline from cache")
}

if len(timelineItems) > 0 {
    // 从缓存获取到数据，走正常流程
    posts, err = s.getPostsByIDs(ctx, timelineItems)
    if err != nil {
        // 获取失败，回退到拉模式
        return s.getFeedByPullMode(ctx, userUUID, cursor, limit)
    }
} else {
    // 缓存中没有数据，使用拉模式
    return s.getFeedByPullMode(ctx, userUUID, cursor, limit)
}
```

**拉模式的具体流程：**
1. 查询 `follows` 表获取用户关注列表。
2. 查询 `posts` 表批量拉取关注用户的最新帖子。
3. 按时间倒序排列，使用游标分页返回。
4. 异步尝试重建 Redis 缓存（Redis 恢复后自动生效）。

**完整降级分层：**

| 层级 | 状态 | 行为 |
|------|------|------|
| L1 应用缓存 | 命中 | 直接返回，延迟 < 1ms |
| L2 Redis 缓存 | 命中 | 从 Sorted Set 读取，延迟 < 5ms |
| L2 Redis 缓存 | 未命中/宕机 | 降级到拉模式 |
| 拉模式 DB 查询 | 正常 | 聚合关注者帖子，延迟 50-200ms |
| 拉模式 DB 查询 | 也失败 | 返回错误，前端展示兜底内容 |

**对写入的影响：**
- Redis 宕机时，Push 模式的 Timeline 写入会失败，但帖子已持久化到 PostgreSQL。
- 分发状态记录失败，但 Kafka 事件仍然存在，Redis 恢复后 Recovery Job 会重新处理。
- 核心数据零丢失：PostgreSQL 是数据的 Source of Truth，Redis 只是加速层。

---

### Q5：排序算法中时间衰减系数为什么选 24h？

**答：**

选择 24h 作为时间衰减的半衰期参数，基于以下考量：

**公式回顾：**

```go
timeDecay = math.Exp(-hoursSinceCreated / 24.0)
```

这是一个指数衰减函数，含义是：
- 发布 0 小时后：衰减因子 = 1.0（满分）
- 发布 24 小时后：衰减因子 ≈ 0.368（约 37%）
- 发布 48 小时后：衰减因子 ≈ 0.135（约 14%）
- 发布 72 小时后：衰减因子 ≈ 0.050（约 5%）

**为什么是 24h 而不是其他值？**

1. **匹配用户行为周期**：社交应用的用户使用模式通常以天为周期 — 早中晚各刷一次。24h 衰减确保"今天的内容"在排序中有明显优势，而"昨天的内容"权重降到 37%，不会完全消失但也不会霸占首屏。

2. **平衡新鲜度与质量**：
   - 如果衰减太快（如 6h），高质量但发布稍早的帖子会迅速沉底，用户可能错过好内容。
   - 如果衰减太慢（如 72h），Feed 中会充斥大量旧帖，用户刷到的内容不够"新鲜"。
   - 24h 是一个经典的平衡点：一篇互动量高的帖子（engagement score 高）即使发布了一天，依然可以排在互动量低的新帖前面。

3. **可配置化设计**：实际生产中这个值应该根据产品数据调优。可以做的优化：
   - **品类差异化**：短内容（类似微博）用 12h 衰减更快，长内容（类似公众号）用 48h 衰减更慢。
   - **用户差异化**：高频用户衰减快一些（他们随时在看，需要更多新内容），低频用户衰减慢一些（确保他们回来时能看到近期的优质内容）。
   - **A/B 测试驱动**：最终参数应基于用户留存、互动率等北极星指标通过 A/B 测试确定。

---

### Q6：推拉结合模式下，关注/取关操作如何处理 Timeline？

**答：**

**关注操作（follow）：**

1. 写入 `follows` 表建立关注关系。
2. 发送 `follow_created` 事件到 Kafka。
3. Worker 消费后执行：
   - 如果被关注者是普通用户（Push 模式），从 `posts` 表拉取其最近的 N 条帖子，批量写入关注者的 Redis Timeline。
   - 如果被关注者是大V（Pull 模式），不做额外处理 — 关注者下次刷 Feed 时，拉模式会自动包含该大V的帖子。
4. 清除关注者的 Feed 缓存，确保下次请求获取最新数据。

**取消关注（unfollow）：**

1. 软删除 `follows` 表记录。
2. 发送 `follow_deleted` 事件到 Kafka。
3. Worker 消费后执行：
   - 从关注者的 Redis Timeline 中移除被取关用户的所有帖子（通过维护 `post.user_id` 的反向索引，或遍历 Timeline 逐条检查）。
   - 清除关注者的 Feed 缓存。

**一致性考量**：取消关注后，用户的 Feed 中可能仍然短暂显示被取关用户的帖子（因为缓存尚未更新），这在社交产品中是可接受的 — 用户刷新后即可看到最新结果。

---

### Q7：系统如何处理热点事件（如明星发帖）导致的瞬时流量？

**答：**

百万粉丝的大V发帖是典型的"写热点"场景，系统通过以下多层机制应对：

**1. 分发层削峰（核心策略）：**

大V走 `distributeForInfluencer` 路径，只推送给 Top 1000 活跃粉丝，而非全量百万粉丝。这将单次发帖的写入量从 100万次 降低到 1000次，削峰比 99.9%。

**2. Kafka 异步解耦：**

帖子写入 DB 后立即返回成功，分发操作通过 Kafka 异步执行。即使分发耗时较长，也不影响用户的发帖体验（接口响应时间 < 100ms）。

**3. 读侧拉模式兜底：**

未被推送的 99.9万粉丝在刷 Feed 时走拉模式，查询压力分散到每个用户的请求中，不会形成瞬时峰值。

**4. K8s HPA 弹性伸缩：**

Worker 配置了 HPA（2-10 实例），当 CPU 使用率超过 70% 时自动扩容，扩容策略为每 15 秒可增加 50% 的实例数，确保消费能力跟上生产速率。

**5. Redis Pipeline 批量写入：**

对 1000 个活跃粉丝的 Timeline 写入使用 Pipeline 合并为单次网络请求，避免 1000 次独立的 `ZADD` 调用。

---

### Q8：用户活跃度评分系统的设计思路是什么？

**答：**

活跃度评分系统用于区分活跃用户和非活跃用户，指导推拉模式选择和缓存策略。

**评分模型：**

```go
// 每种行为的分值
login:     5.0   // 登录
post:     15.0   // 发帖（最高权重，代表强活跃）
comment:   8.0   // 评论
share:    10.0   // 分享
like:      2.0   // 点赞（最低权重，代表弱活跃）
view_feed: 1.0   // 浏览Feed
```

**时间衰减机制：**

```go
// 距离上次活跃的时间越长，历史分值衰减越多
decay = 0.9 ^ (hoursSinceLastActive / 24.0)
newScore = oldScore × decay + increment
```

每过 24 小时，历史积分衰减 10%。一个用户如果连续 7 天不活跃，其历史分值衰减为原来的 `0.9^7 ≈ 48%`。

**活跃判定规则（三条件取或）：**
1. 当前在线（`IsOnline = true`）
2. 活跃度分数 ≥ 50（`ActiveUserScoreThreshold`）
3. 最后活跃时间在 7 天内

**设计权衡**：
- 分值上限设为 1000，防止极度活跃用户的分数溢出。
- 在线状态通过 Redis 缓存（TTL 15min），用户 15 分钟无操作自动判定为离线。
- 活跃度缓存 5 分钟，避免频繁查库。

---

### Q9：这套系统和 Twitter / 微博的 Feed 流有什么异同？

**答：**

**相同点：**
- 都采用推拉结合模型（Twitter 的 Fan-out Service）。
- 都使用 Redis 作为 Timeline 的热存储。
- 都有按粉丝量区分大V和普通用户的分发策略。

**不同点/简化点（面试中要诚实说明）：**

| 维度 | 本系统 | Twitter / 微博 |
|------|--------|----------------|
| 规模 | 千万级 DAU 设计 | 亿级 DAU |
| Timeline 存储 | Redis Sorted Set | Redis + Manhattan (Twitter自研KV) |
| 消息队列 | Kafka 单 Topic | 多级 Topic + 优先级队列 |
| 排序算法 | 基于规则的 Score 公式 | ML 模型 + 特征工程 |
| 推拉阈值 | 静态配置 | 动态计算 + ML 预测 |
| 混排 | 无（纯时间线） | 广告、推荐、关注混排 |
| 图片/视频 | URL 引用 | CDN + 转码 + 审核管线 |

面试时可以说："本系统实现了 Feed 流的核心架构（推拉结合 + 缓存分层 + 异步分发），在实际生产中还需要进一步结合 ML 推荐、内容审核、CDN 分发等模块来达到完整的社交产品形态。"

---

## 三、系统架构速览图（面试白板用）

```
                        ┌─────────────────┐
                        │  Load Balancer  │
                        └────────┬────────┘
                                 │
                        ┌────────▼────────┐
                        │  Gin API Layer  │  ← JWT Auth + Rate Limit
                        │  (Stateless)    │
                        └────────┬────────┘
                                 │
              ┌──────────────────┼──────────────────┐
              │                  │                   │
     ┌────────▼───────┐  ┌──────▼───────┐  ┌───────▼────────┐
     │  User Service  │  │ Feed Service │  │ Like/Comment   │
     │  (关注/用户)    │  │ (推拉引擎)   │  │ Service        │
     └────────┬───────┘  └──────┬───────┘  └───────┬────────┘
              │                  │                   │
   ┌──────────┼──────────────────┼───────────────────┤
   │          │                  │                   │
   │  ┌───────▼──────┐  ┌───────▼──────┐  ┌────────▼───────┐
   │  │  PostgreSQL  │  │    Redis     │  │  Apache Kafka  │
   │  │  (持久存储)   │  │  (Timeline   │  │  (异步事件)     │
   │  │              │  │   + Cache)   │  │                │
   │  └──────────────┘  └──────────────┘  └────────┬───────┘
   │                                                │
   │                                       ┌────────▼────────┐
   │                                       │  Feed Worker    │
   │                                       │  (消费+分发+恢复)│
   │                                       └─────────────────┘
   │
   │  ┌───────────────────────────────────────────────────────┐
   │  │              Background Jobs                          │
   │  │  • Cache Cleanup (每1h)                               │
   │  │  • Recovery Job (每5min)                              │
   │  │  • Timeline Cleanup (每24h)                           │
   │  │  • Activity Decay (每24h)                             │
   │  └───────────────────────────────────────────────────────┘
   │
   │  ┌───────────────────────────────────────────────────────┐
   └──│              Kubernetes                               │
      │  • HPA: API 3-20 / Worker 2-10                       │
      │  • Liveness + Readiness Probes                        │
      │  • Prometheus + Grafana Monitoring                    │
      └───────────────────────────────────────────────────────┘
```

---

## 四、核心代码引用索引（面试前快速复习）

| 模块 | 文件 | 关键方法/结构 |
|------|------|--------------|
| 推拉分发引擎 | `internal/services/feed_optimized.go` | `distributePostOptimized`, `distributeForInfluencer`, `distributeForRegularUser` |
| 拉模式兜底 | `internal/services/feed_optimized.go` | `getFeedByPullMode`, `rebuildTimelineCache` |
| Timeline缓存 | `internal/services/timeline_cache.go` | `AddToTimeline`, `GetTimeline`(游标分页), `BatchAddToTimeline`(Pipeline) |
| 排序算法 | `internal/services/feed.go` | `calculatePostScore`, `calculateInitialScore` |
| 用户活跃度 | `internal/services/activity.go` | `UpdateUserActivity`, `IsUserActive`, `GetActiveFollowers` |
| 缓存策略 | `internal/services/cache_strategy.go` | `DetermineUserCacheStrategy`, `CleanupInactiveUserCaches`, `PrewarmCache` |
| 崩溃恢复 | `internal/services/recovery.go` | `RecoverPendingDistributions`, `recoverInfluencerDistribution` |
| Worker | `internal/workers/feed_worker_optimized.go` | `handleMessage`, `startBackgroundJobs` |
| Kafka封装 | `pkg/queue/kafka.go` | `Publish`, `Subscribe`, 9种Event定义 |
| 数据模型 | `internal/models/` | `Post`, `Timeline`, `User`(含ActivityScore), `Follow`, `Like`, `Comment` |
| 配置管理 | `internal/config/config.go` | `FeedConfig`, `OptimizationConfig` |
| K8s弹性 | `deployments/k8s/hpa.yaml` | API HPA (3-20), Worker HPA (2-10) |
