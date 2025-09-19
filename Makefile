.PHONY: build run test clean docker-build docker-run k8s-deploy

# 构建变量
BINARY_NAME=feed-api
WORKER_NAME=feed-worker
DOCKER_REGISTRY=your-registry
VERSION=latest

# Go相关变量
GOPATH=$(shell go env GOPATH)
GOBIN=$(GOPATH)/bin
GO=go
GOFLAGS=-v

# 构建API服务
build-api:
	@echo "Building API service..."
	$(GO) build $(GOFLAGS) -o bin/$(BINARY_NAME) ./cmd/api

# 构建Worker服务
build-worker:
	@echo "Building Worker service..."
	$(GO) build $(GOFLAGS) -o bin/$(WORKER_NAME) ./cmd/worker

# 构建所有服务
build: build-api build-worker

# 运行API服务
run-api: build-api
	@echo "Running API service..."
	./bin/$(BINARY_NAME)

# 运行Worker服务
run-worker: build-worker
	@echo "Running Worker service..."
	./bin/$(WORKER_NAME)

# 运行所有服务
run: build
	@echo "Running all services..."
	./bin/$(BINARY_NAME) &
	./bin/$(WORKER_NAME) &
	@wait

# 下载依赖
deps:
	@echo "Downloading dependencies..."
	$(GO) mod download
	$(GO) mod tidy

# 运行测试
test:
	@echo "Running tests..."
	$(GO) test -v ./...

# 运行测试覆盖率
test-coverage:
	@echo "Running tests with coverage..."
	$(GO) test -v -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html

# 代码格式化
fmt:
	@echo "Formatting code..."
	$(GO) fmt ./...

# 代码检查
lint:
	@echo "Running linter..."
	golangci-lint run

# 清理构建文件
clean:
	@echo "Cleaning build files..."
	rm -rf bin/
	rm -f coverage.out coverage.html

# Docker构建
docker-build:
	@echo "Building Docker images..."
	docker build -f deployments/docker/Dockerfile -t $(DOCKER_REGISTRY)/feed-system/api:$(VERSION) .
	docker build -f deployments/docker/Dockerfile.worker -t $(DOCKER_REGISTRY)/feed-system/worker:$(VERSION) .

# Docker推送
docker-push: docker-build
	@echo "Pushing Docker images..."
	docker push $(DOCKER_REGISTRY)/feed-system/api:$(VERSION)
	docker push $(DOCKER_REGISTRY)/feed-system/worker:$(VERSION)

# Docker运行
docker-run:
	@echo "Running with Docker Compose..."
	cd deployments/docker && docker-compose up -d

# Docker停止
docker-stop:
	@echo "Stopping Docker containers..."
	cd deployments/docker && docker-compose down

# Docker清理
docker-clean:
	@echo "Cleaning Docker resources..."
	cd deployments/docker && docker-compose down -v
	docker system prune -f

# Kubernetes部署
k8s-deploy:
	@echo "Deploying to Kubernetes..."
	cd deployments/k8s && kubectl apply -k .

# Kubernetes删除
k8s-delete:
	@echo "Deleting from Kubernetes..."
	cd deployments/k8s && kubectl delete -k .

# Kubernetes状态
k8s-status:
	@echo "Checking Kubernetes status..."
	kubectl get all -n feed-system

# 数据库迁移
migrate:
	@echo "Running database migrations..."
	$(GO) run cmd/migrate/main.go

# 生成数据库模型
model:
	@echo "Generating database models..."
	go run github.com/xxjwxc/gormt@latest -H=127.0.0.1 -d=feedsystem -p=5432 -u=feeduser -a=feedpass -s=public

# 生成API文档
docs:
	@echo "Generating API documentation..."
	go run cmd/docs/main.go

# 性能测试
benchmark:
	@echo "Running benchmarks..."
	$(GO) test -bench=. -benchmem ./...

# 启动开发环境
dev:
	@echo "Starting development environment..."
	make docker-run
	@echo "Waiting for services to start..."
	@sleep 10
	@echo "Development environment is ready!"
	@echo "API Server: http://localhost:8080"
	@echo "Health Check: http://localhost:8080/health"

# 停止开发环境
dev-stop:
	@echo "Stopping development environment..."
	make docker-stop

# 完整开发流程
dev-full: deps fmt lint test build
	@echo "Development cycle completed!"

# 发布版本
release: clean test docker-build docker-push k8s-deploy
	@echo "Release completed!"

# 帮助信息
help:
	@echo "Feed System Makefile"
	@echo ""
	@echo "Usage:"
	@echo "  make build          - Build all services"
	@echo "  make run            - Run all services"
	@echo "  make test           - Run tests"
	@echo "  make docker-build   - Build Docker images"
	@echo "  make docker-run     - Run with Docker"
	@echo "  make k8s-deploy     - Deploy to Kubernetes"
	@echo "  make dev            - Start development environment"
	@echo "  make clean          - Clean build files"
	@echo "  make help           - Show this help message"
	@echo ""
	@echo "Available targets:"
	@awk '/^[a-zA-Z_-]+:.*?##/ { printf "  %-20s %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

# 默认目标
.DEFAULT_GOAL := help