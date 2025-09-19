# Kubernetes 部署指南

## 快速开始

### 1. 前提条件
- Kubernetes 集群 (1.20+)
- kubectl 配置完成
- Kustomize 安装完成
- 容器镜像仓库访问权限

### 2. 构建镜像
```bash
# 构建API服务镜像
docker build -f ../docker/Dockerfile -t your-registry/feed-system/api:latest ../..

# 构建工作进程镜像
docker build -f ../docker/Dockerfile.worker -t your-registry/feed-system/worker:latest ../..

# 推送到镜像仓库
docker push your-registry/feed-system/api:latest
docker push your-registry/feed-system/worker:latest
```

### 3. 修改镜像地址
编辑 `kustomization.yaml` 文件，更新镜像地址：
```yaml
images:
- name: feed-system/api
  newName: your-registry/feed-system/api
  newTag: latest
- name: feed-system/worker
  newName: your-registry/feed-system/worker
  newTag: latest
```

### 4. 部署应用
```bash
# 应用所有资源配置
kubectl apply -k .

# 检查部署状态
kubectl get all -n feed-system

# 查看Pod状态
kubectl get pods -n feed-system

# 查看服务状态
kubectl get svc -n feed-system
```

## 服务组件

### 1. Namespace
- 名称：feed-system
- 隔离应用资源

### 2. ConfigMap
- 名称：feed-config
- 包含应用配置文件
- 挂载到容器内部

### 3. PostgreSQL
- 部署：单实例
- 存储：10Gi PVC
- 服务：postgres-service
- 端口：5432

### 4. Redis
- 部署：单实例
- 存储：5Gi PVC
- 服务：redis-service
- 端口：6379

### 5. Kafka + Zookeeper
- Zookeeper：单实例
- Kafka：单实例
- 自动创建topic
- 服务：kafka-service (9092)

### 6. Feed API
- 部署：3副本
- 服务：ClusterIP
- 端口：80 -> 8080
- HPA：自动扩缩容
- Ingress：feed-system.local

### 7. Feed Worker
- 部署：3副本
- HPA：自动扩缩容
- 处理消息队列事件

## 扩缩容

### 手动扩缩容
```bash
# 扩展API服务到5个副本
kubectl scale deployment feed-api -n feed-system --replicas=5

# 扩展Worker到8个副本
kubectl scale deployment feed-worker -n feed-system --replicas=8
```

### 自动扩缩容
HPA配置：
- API服务：最小3个，最大20个副本
- Worker：最小2个，最大10个副本
- 触发条件：CPU 70%，内存 80%

## 监控和日志

### Prometheus监控
- ServiceMonitor配置
- 指标收集：/metrics
- 告警规则定义

### Grafana仪表板
- Feed System Dashboard
- 关键指标可视化
- 自定义图表

### 告警规则
- API错误率高
- Worker队列积压
- 数据库连接失败
- Redis连接失败
- Kafka连接失败

### 日志查看
```bash
# 查看API服务日志
kubectl logs -f deployment/feed-api -n feed-system

# 查看Worker日志
kubectl logs -f deployment/feed-worker -n feed-system

# 查看所有Pod日志
kubectl logs -f -l app.kubernetes.io/name=feed-system -n feed-system
```

## 配置管理

### 更新配置
```bash
# 编辑配置文件
kubectl edit configmap feed-config -n feed-system

# 重启Pod使配置生效
kubectl rollout restart deployment/feed-api -n feed-system
kubectl rollout restart deployment/feed-worker -n feed-system
```

### 环境变量
- CONFIG_PATH：配置文件路径
- 数据库连接信息
- Redis连接信息
- Kafka连接信息

## 存储管理

### 持久化存储
- PostgreSQL：10Gi PVC
- Redis：5Gi PVC
- 存储类：standard

### 备份策略
```bash
# 备份PostgreSQL数据
kubectl exec -it deployment/postgres -n feed-system -- pg_dump -U feeduser feedsystem > backup.sql

# 备份Redis数据
kubectl exec -it deployment/redis -n feed-system -- redis-cli save
kubectl cp feed-system/redis-xxx:/data/dump.rdb ./dump.rdb
```

## 网络配置

### Ingress
- 类：nginx
- 主机：feed-system.local
- 路径：/
- SSL：可配置

### Service
- ClusterIP：内部服务发现
- 端口映射：80 -> 8080

## 安全考虑

### 1. 镜像安全
- 使用官方基础镜像
- 定期更新镜像
- 扫描镜像漏洞

### 2. 网络隔离
- 使用NetworkPolicy
- 限制Pod间通信
- 服务间认证

### 3. RBAC
- 最小权限原则
- 服务账户配置
- 权限审计

### 4. 数据安全
- 数据库密码管理
- 传输加密
- 数据备份加密

## 性能调优

### 资源限制
- CPU限制：防止资源耗尽
- 内存限制：防止OOM
- 合理请求值设置

### 调度策略
- 反亲和性规则
- 节点亲和性
- Pod反亲和性

### 缓存优化
- Redis内存配置
- 缓存过期策略
- 缓存命中率监控

## 故障排查

### Pod故障
```bash
# 查看Pod事件
kubectl describe pod <pod-name> -n feed-system

# 查看Pod日志
kubectl logs <pod-name> -n feed-system

# 进入Pod调试
kubectl exec -it <pod-name> -n feed-system -- /bin/sh
```

### 服务故障
```bash
# 查看服务状态
kubectl get svc -n feed-system

# 查看Endpoint
kubectl get endpoints -n feed-system

# 测试服务连接
kubectl run test-pod --image=busybox -it --rm -- wget -O- http://feed-api-service/health
```

### 存储故障
```bash
# 查看PVC状态
kubectl get pvc -n feed-system

# 查看PV状态
kubectl get pv

# 查看存储类
kubectl get storageclass
```

## 升级策略

### 滚动升级
```bash
# 更新镜像
kubectl set image deployment/feed-api feed-api=your-registry/feed-system/api:new-tag -n feed-system

# 查看升级状态
kubectl rollout status deployment/feed-api -n feed-system

# 回滚升级
kubectl rollout undo deployment/feed-api -n feed-system
```

### 蓝绿部署
- 创建新版本部署
- 验证新版本
- 切换流量
- 删除旧版本

## 清理资源

```bash
# 删除所有资源
kubectl delete -k .

# 删除命名空间
kubectl delete namespace feed-system

# 清理PVC（可选）
kubectl delete pvc --all -n feed-system
```