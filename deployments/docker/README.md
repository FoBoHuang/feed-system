# Docker 部署指南

## 快速开始

### 1. 构建镜像
```bash
# 构建API服务镜像
docker build -f deployments/docker/Dockerfile -t feed-system/api:latest .

# 构建工作进程镜像
docker build -f deployments/docker/Dockerfile.worker -t feed-system/worker:latest .
```

### 2. 启动服务
```bash
cd deployments/docker
docker-compose up -d
```

### 3. 验证服务
```bash
# 检查服务状态
docker-compose ps

# 查看日志
docker-compose logs -f feed-api

# 健康检查
curl http://localhost:8080/health
```

## 服务组件

### PostgreSQL
- 版本：15-alpine
- 端口：5432
- 数据库：feedsystem
- 用户：feeduser
- 密码：feedpass

### Redis
- 版本：7-alpine
- 端口：6379
- 用途：缓存和会话存储

### Kafka + Zookeeper
- Kafka版本：7.4.0
- Zookeeper端口：2181
- Kafka端口：9092
- 自动创建topic：启用

### Feed API
- 端口：8080
- 健康检查：/health
- 重启策略：unless-stopped

### Feed Worker
- 工作进程数量：2个副本
- 处理消息队列事件
- 重启策略：unless-stopped

## 环境变量

查看 `.env` 文件了解所有环境变量配置。

## 数据持久化

- PostgreSQL数据：postgres_data卷
- Redis数据：redis_data卷
- 应用日志：./logs目录

## 监控和日志

### 查看日志
```bash
# 查看所有服务日志
docker-compose logs -f

# 查看特定服务日志
docker-compose logs -f feed-api
docker-compose logs -f feed-worker
```

### 监控容器状态
```bash
docker-compose ps
docker stats
```

## 扩展和缩容

### 扩展API服务
```bash
docker-compose up -d --scale feed-api=3
```

### 扩展Worker服务
```bash
docker-compose up -d --scale feed-worker=5
```

## 停止和清理

### 停止服务
```bash
docker-compose down
```

### 停止并删除数据
```bash
docker-compose down -v
```

### 清理镜像
```bash
docker image prune -f
```

## 常见问题

### 1. Kafka连接失败
确保Zookeeper完全启动后再启动Kafka。

### 2. 数据库连接失败
等待PostgreSQL完全启动，检查健康检查状态。

### 3. 端口冲突
修改docker-compose.yml中的端口映射。

### 4. 内存不足
减少worker副本数量或增加Docker内存限制。

## 性能调优

### 数据库调优
- 调整max_connections
- 配置shared_buffers
- 启用查询缓存

### Redis调优
- 调整maxmemory
- 配置合适的淘汰策略
- 启用持久化

### Kafka调优
- 调整分区数量
- 配置合适的副本因子
- 优化生产者/消费者配置

## 安全配置

### 1. 修改默认密码
在生产环境中修改所有默认密码。

### 2. 使用TLS
配置TLS证书进行加密通信。

### 3. 网络隔离
使用Docker网络隔离不同服务。

### 4. 资源限制
为容器设置CPU和内存限制。