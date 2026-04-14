APP_NAME := k8sinsight
BIN_DIR := bin
GO := go
GOFLAGS := -v
COMPOSE := docker compose -f deploy/docker/docker-compose.yml -f deploy/docker/docker-compose.dev.yml --env-file deploy/docker/.env
COMPOSE_PROD := docker compose -f deploy/docker/docker-compose.prod.yml --env-file deploy/docker/.env.prod

# 数据库连接参数（可通过环境变量 DB_DSN 覆盖）
DB_DSN ?= postgres://k8sinsight:k8sinsight@localhost:5432/k8sinsight?sslmode=disable

.PHONY: build run test lint fmt clean tidy help \
        dev-docker-up dev-docker-up-d dev-docker-down dev-docker-ps dev-docker-logs \
        prod-up prod-up-d prod-down prod-ps prod-logs \
        migrate-up migrate-down migrate-create migrate-status \
        frontend-install frontend-build

# ============================================================
# 构建
# ============================================================

## build: 编译 Go 二进制
build:
	$(GO) build $(GOFLAGS) -o $(BIN_DIR)/$(APP_NAME) ./cmd/k8sinsight

## run: 本地运行
run: build
	./$(BIN_DIR)/$(APP_NAME)

## clean: 清理构建产物
clean:
	rm -rf $(BIN_DIR) coverage.txt cover.html

## tidy: 整理 Go 依赖
tidy:
	$(GO) mod tidy

# ============================================================
# Docker Compose 开发环境（后端 Air + 前端 Vite HMR）
# ============================================================

## dev-docker-up: 前台启动开发环境
dev-docker-up:
	$(COMPOSE) up --build

## dev-docker-up-d: 后台启动开发环境
dev-docker-up-d:
	$(COMPOSE) up -d --build

## dev-docker-down: 停止开发环境
dev-docker-down:
	$(COMPOSE) down

## dev-docker-ps: 查看容器状态
dev-docker-ps:
	$(COMPOSE) ps

## dev-docker-logs: 查看容器日志
dev-docker-logs:
	$(COMPOSE) logs -f

# ============================================================
# Docker Compose 生产环境（内置 PostgreSQL）
# ============================================================

## prod-up: 前台启动生产环境
prod-up:
	$(COMPOSE_PROD) up --build

## prod-up-d: 后台启动生产环境
prod-up-d:
	$(COMPOSE_PROD) up -d --build

## prod-down: 停止生产环境
prod-down:
	$(COMPOSE_PROD) down

## prod-ps: 查看生产环境容器状态
prod-ps:
	$(COMPOSE_PROD) ps

## prod-logs: 查看生产环境日志
prod-logs:
	$(COMPOSE_PROD) logs -f

# ============================================================
# 数据库迁移
# ============================================================

## migrate-up: 执行所有待执行迁移
migrate-up:
	migrate -path migrations -database "$(DB_DSN)" up

## migrate-down: 回滚最近一次迁移
migrate-down:
	migrate -path migrations -database "$(DB_DSN)" down 1

## migrate-down-all: 回滚所有迁移
migrate-down-all:
	migrate -path migrations -database "$(DB_DSN)" down

## migrate-status: 查看迁移状态
migrate-status:
	migrate -path migrations -database "$(DB_DSN)" version

## migrate-create: 创建新迁移文件（用法: make migrate-create NAME=xxx）
migrate-create:
	migrate create -ext sql -dir migrations -seq $(NAME)

# ============================================================
# 前端
# ============================================================

## frontend-install: 安装前端依赖
frontend-install:
	cd web && npm install

## frontend-build: 构建前端生产版本
frontend-build:
	cd web && npm run build

# ============================================================
# 测试 & 质量
# ============================================================

## test: 运行单元测试
test:
	$(GO) test ./... -race -count=1

## test-coverage: 运行测试并生成覆盖率报告
test-coverage:
	$(GO) test ./... -race -coverprofile=coverage.txt -covermode=atomic
	$(GO) tool cover -html=coverage.txt -o cover.html

## lint: 代码检查
lint:
	golangci-lint run ./...

## fmt: 格式化代码
fmt:
	$(GO) fmt ./...
	goimports -w .

# ============================================================
# 帮助
# ============================================================

## help: 显示帮助
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## //' | column -t -s ':'
