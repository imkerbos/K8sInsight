# K8sInsight

K8sInsight 是一个面向 Kubernetes 集群的 **异常监测与问题根因分析系统**。  
它以 Pod 运行异常为切入点，自动采集运行现场信息，关联多维度上下文数据，并通过统一界面呈现问题全貌，帮助运维与 SRE 更快理解 **“发生了什么、为什么发生、应该如何处理”**。

K8sInsight 的目标不是简单告警，而是 **让问题变得可解释、可复盘、可追溯**。

---

## 项目背景

在实际生产环境中，常见但难以解决的问题包括：

- Pod 频繁重启，却无法确定真正原因
- OOM 发生后容器迅速拉起，现场信息丢失
- 同类问题反复出现，但无法总结规律
- 监控系统能发现异常，却无法回答“为什么”
- 问题排查高度依赖个人经验，难以沉淀

K8sInsight 旨在解决 **“问题被发现了，但无法被理解”** 的核心痛点。

---

## 核心目标

- 持续感知 Kubernetes 集群中的运行异常
- 在异常发生的第一时间自动介入
- 系统化采集与保留关键运行证据
- 对异常事件进行关联、归档与分析
- 通过 Web 界面提供完整的问题视图
- 为根因分析与后续改进提供事实基础

---

## 使用场景

K8sInsight 适用于以下场景：

- Pod 重启 / OOM 问题定位
- CrashLoopBackOff 辅助分析
- 发布后异常回溯
- 偶发性、难复现问题排查
- 生产环境运行质量监测
- 运维与 SRE 事后复盘支持

---

## 系统功能

### 1. 异常事件监测

- 持续监测集群中 Pod 及容器的运行状态变化
- 识别异常行为，包括但不限于：
  - Pod 重启
  - 容器 OOM
  - 异常退出
  - 状态频繁波动
- 以事件为中心触发后续分析流程

---

### 2. 运行现场自动采集

在异常发生时，系统会自动采集与问题相关的关键上下文信息，包括但不限于：

- Pod 与容器基础信息
- 容器退出状态与原因
- 重启历史与时间线
- 相关运行事件记录
- 资源使用与限制相关信息

目标是 **最大程度还原异常发生前后的运行状态**。

---

### 3. 异常事件归档与时间线

- 将异常事件进行统一归档
- 构建清晰的事件时间线
- 支持查看异常发生前后的上下文变化
- 支持对同一 Pod / 应用的多次异常进行对比

---

### 4. 问题关联与分析视图

- 将异常事件与相关运行信息进行关联
- 从多个维度呈现问题全貌
- 帮助用户理解：
  - 问题发生的背景
  - 可能的触发条件
  - 重复出现的模式

---

### 5. Web 可视化界面

- 提供统一的 Web 页面
- 以问题为中心展示异常详情
- 支持从集群、命名空间、应用等维度进行查看
- 降低问题分析门槛，减少对命令行与经验的依赖

---

### 6. 通知与系统联动

- 支持异常事件触发通知
- 将异常信息推送到外部系统
- 可用于：
  - 告警平台
  - 工单系统
  - 事件中心
  - 自动化处理流程

---

## 技术栈

### 后端

| 组件 | 技术选型 | 说明 |
|------|---------|------|
| 语言 | Go 1.25.5 | |
| Web 框架 | Gin (gin-gonic/gin) | HTTP API 服务 |
| 认证 | JWT (golang-jwt/jwt/v5) | RS256 签名 + Refresh Token |
| 日志 | Zap (uber-go/zap) | 结构化 JSON 日志 |
| 配置管理 | Viper (spf13/viper) | 支持 YAML / ENV / ConfigMap |
| 数据库 | PostgreSQL 16+ | |
| 数据库驱动 | pgx (jackc/pgx/v5) | |
| ORM | GORM (gorm.io/gorm) | |
| 数据库迁移 | golang-migrate | Schema 版本管理 |
| K8s 客户端 | client-go | Informer / Watch |
| K8s 类型 | k8s.io/api + apimachinery | |
| API 文档 | Swagger (swaggo/swag) | 自动生成 |
| 参数校验 | validator (go-playground/validator) | Gin 内置集成 |
| 跨域 | gin-contrib/cors | |
| Prometheus 指标 | prometheus/client_golang | 暴露 /metrics |
| 健康检查 | /healthz + /readyz | K8s 探针 |
| 优雅关停 | os/signal + context | SIGTERM 处理 |
| 限流 | ulule/limiter | API 限流保护 |
| 请求追踪 | gin-contrib/requestid | 请求 ID 关联日志 |
| 测试 | testify (stretchr/testify) | 断言 / Mock |

### 前端

| 组件 | 技术选型 | 说明 |
|------|---------|------|
| 框架 | React 18+ / TypeScript | |
| 构建工具 | Vite | |
| UI 组件库 | Ant Design 5.x | |
| 请求管理 | TanStack Query | API 状态管理 |
| 图表 | ECharts | 可视化 |
| 时间处理 | dayjs | |
| 路由 | React Router | |

### 部署与运维

| 组件 | 技术选型 | 说明 |
|------|---------|------|
| 容器化 | Docker multi-stage | distroless / alpine 基础镜像 |
| 部署 | Helm Chart | K8s 原生部署 |
| 通知集成 | Webhook + 插件式 | 钉钉 / 飞书 / Slack 等 |

---

## 部署策略（推荐）

### Dev 环境（Air + Docker Compose）

- 运行模式：后端 `Air` 热更新 + 前端 `Vite HMR`
- 数据库策略：**Dev 环境不在 compose 内维护 PostgreSQL**，连接外部已存在 PG（本机或远程）
- 适用目标：本地联调、快速开发、调试异常链路

关键文件：
- `deploy/docker/docker-compose.yml`
- `deploy/docker/docker-compose.dev.yml`
- `deploy/docker/.env`

启动命令：

```bash
make dev-docker-up-d
```

### Production 环境（必须支持两种方式）

#### 1) Docker Compose 方式（单机/小规模）

- 要求：**生产 compose 栈内包含 PostgreSQL 服务**
- 建议：
  - `postgres` 使用独立持久化卷
  - 后端和数据库分开资源限制
  - 开启定时备份（全量 + WAL）

已提供文件：
- `deploy/docker/docker-compose.prod.yml`
- `deploy/docker/.env.prod.example`

使用方式：

```bash
cp deploy/docker/.env.prod.example deploy/docker/.env.prod
# 修改镜像、密码、JWT 等生产参数
make prod-up-d
```

#### 2) Helm 方式（Kubernetes）

- 要求：**生产集群内集成 PostgreSQL（StatefulSet）**
- 推荐：
  - 使用内置 PostgreSQL StatefulSet（默认 `postgres:18`）
  - PG 使用 PVC，配置备份与监控
  - 后端通过 ClusterIP Service 访问 PG

已提供 Chart：
- `deploy/helm/k8sinsight/Chart.yaml`
- `deploy/helm/k8sinsight/values.yaml`
- `deploy/helm/k8sinsight/templates/*`

安装示例：

```bash
helm upgrade --install k8sinsight deploy/helm/k8sinsight \
  -n k8sinsight --create-namespace \
  --set auth.jwtSecret='change-me-super-secret' \
  --set auth.defaultAdminPassword='change-me-admin-password' \
  --set postgresql.auth.password='change-me-strong-password'

# 如果使用 Istio（仅创建 VirtualService）
helm upgrade --install k8sinsight deploy/helm/k8sinsight \
  -n k8sinsight --create-namespace \
  --set exposure.mode=istio \
  --set istio.enabled=true
```

说明：
- Chart 默认包含 PostgreSQL（`postgresql.enabled=true`）。
- Chart 默认镜像仓库：
  - `ghcr.io/imkerbos/k8sinsight-backend`
  - `ghcr.io/imkerbos/k8sinsight-frontend`
- Chart 默认拉取策略：`Always`
- 如需接入外部数据库，可在 values 中关闭内置 PG，并覆盖后端 DB 环境变量。
- 暴露方式支持二选一：
  - `exposure.mode=gateway`：使用 K8s Ingress（`gateway.enabled=true` 时生效）
  - `exposure.mode=istio`：默认只创建 `VirtualService`（不创建 Gateway 资源）

### 配置基线建议

| 配置项 | Dev 推荐 | Production 推荐 |
|---|---|---|
| DB 连接池 `maxConns/minConns` | `20 / 5` | `80 / 20`（按实例规格调优） |
| 后端副本数 | 1 | 2-4 |
| JWT Secret | 开发固定值可接受 | 必须 Secret 注入并轮换 |
| 默认管理员密码 | 可临时默认 | 禁止默认值，首次部署即改密 |
| 日志级别 | `info`/`debug` | `info` |
| 聚合 `groupWait` | `30s` | `30s` |
| 聚合 `activeWindow` | `6h` | `6h`（按业务调整） |

生产配置示例（关键项）：

```yaml
server:
  port: 8080
  jwtSecret: "<from-secret>"
  accessTokenTTL: 2h
  refreshTokenTTL: 168h

db:
  host: "postgresql.k8sinsight.svc.cluster.local"
  port: 5432
  user: "k8sinsight"
  password: "<from-secret>"
  dbname: "k8sinsight"
  sslMode: disable
  maxConns: 80
  minConns: 20

watch:
  resyncPeriod: 30m
  aggregation:
    groupWait: 30s
    activeWindow: 6h
```

---

## 设计原则

- **问题驱动**：以异常问题为核心对象
- **证据优先**：优先保留真实运行现场
- **可解释性**：不仅知道“出问题了”，还要知道“为什么”
- **可复盘性**：支持事后分析与经验沉淀
- **平台化**：为后续能力扩展预留空间

---

## 项目定位说明

K8sInsight 不是：

- 单纯的监控系统
- 指标或日志平台
- 传统告警工具

K8sInsight 专注于：

> **“在 Kubernetes 异常发生后，帮助你看懂问题。”**

---

## 预期价值

通过 K8sInsight，可以实现：

- 明显缩短异常问题的定位时间
- 减少对个人经验的依赖
- 提高问题分析的一致性与可靠性
- 将偶发问题转变为可分析问题
- 为持续改进与自动化分析提供基础数据

---

## 后续可扩展方向

- 异常模式识别与聚类
- 历史问题对比与趋势分析
- 与发布、变更系统的关联分析
- 更智能的问题归因与建议
- 面向平台级 SRE 的问题治理能力

---

## 验收结果（2026-03-03）

以下为历史完整端到端验收结果（覆盖认证、用户、集群、事件生命周期接口）。

| 验收模块 | 用例总数 | 通过 | 失败 | 结果 |
|---|---:|---:|---:|---|
| 认证与会话 | 7 | 7 | 0 | 通过 |
| 角色与用户管理 | 23 | 23 | 0 | 通过 |
| 系统设置（安全参数） | 3 | 3 | 0 | 通过 |
| 集群管理 | 14 | 14 | 0 | 通过 |
| 异常事件与生命周期接口 | 9 | 9 | 0 | 通过 |
| 测试数据清理 | 3 | 3 | 0 | 通过 |
| **合计** | **59** | **59** | **0** | **通过** |

可参考：
- `docs/design/01-state-detection-layer.md`

---

## 性能基准（100万 incidents）

### 测试数据与说明

- 数据库：PostgreSQL
- 数据量：`incidents` 表新增 `namespace=perf-1m` 共 `1,000,000` 行
- 类型分布：9 种异常类型（`CrashLoopBackOff`、`OOMKilled`、`ErrorExit`、`RestartIncrement`、`ImagePullBackOff`、`CreateContainerConfigError`、`FailedScheduling`、`Evicted`、`StateOscillation`）
- 时间：2026-03-03

### 已应用优化

- 列表查询改为游标分页（Keyset）  
- 复合索引：`(last_seen DESC, id DESC)` 及 state/type/namespace/cluster 维度索引  
- `owner_name ILIKE` + `pg_trgm` GIN 索引  
- 列表查询超时与慢查询日志  
- 读接口短 TTL 本地缓存（列表/详情/证据）

### DB 侧查询基准（单位：ms）

| Query | Avg | P95 | P99 | Max |
|---|---:|---:|---:|---:|
| list_unfiltered_page1 | 0.005 | 0.008 | 0.011 | 0.012 |
| list_namespace_page1 | 0.005 | 0.006 | 0.007 | 0.007 |
| list_active_page1 | 0.007 | 0.008 | 0.008 | 0.008 |
| cursor_next_page | 0.038 | 0.045 | 0.066 | 0.074 |
| search_owner_ilike | 19.786 | 21.730 | 22.997 | 23.514 |
| type_CrashLoopBackOff | 0.037 | 0.062 | 0.069 | 0.072 |
| type_OOMKilled | 0.027 | 0.034 | 0.036 | 0.037 |
| type_ErrorExit | 0.030 | 0.037 | 0.037 | 0.037 |
| type_RestartIncrement | 0.031 | 0.046 | 0.057 | 0.059 |
| type_ImagePullBackOff | 0.035 | 0.045 | 0.110 | 0.136 |
| type_CreateContainerConfigError | 0.022 | 0.023 | 0.023 | 0.023 |
| type_FailedScheduling | 0.024 | 0.025 | 0.031 | 0.033 |
| type_Evicted | 0.023 | 0.023 | 0.027 | 0.029 |
| type_StateOscillation | 0.024 | 0.024 | 0.030 | 0.033 |

### API 并发压测（`CONCURRENCY=100`, `REQUESTS=1000`，单位：秒）

| API | Avg | P95 | P99 | Max |
|---|---:|---:|---:|---:|
| 列表查询(第一页) | 0.010846 | 0.110375 | 0.170040 | 0.198945 |
| 事件详情 | 0.001182 | 0.002846 | 0.004792 | 0.008441 |
| 事件证据 | 0.000911 | 0.001313 | 0.002520 | 0.010765 |

### 结论

- 在 100 万数据规模下，列表接口在高并发下可达到 **P95 约 110ms**、**P99 约 170ms**（本次环境实测）。
- 详情和证据查询保持在毫秒级。
- 最慢路径仍是 `owner_name ILIKE '%keyword%'`，但在 `pg_trgm` 索引后已降至约 20ms 量级（DB 内核查询）。

可参考：
- `docs/design/01-state-detection-layer.md`
- `docs/design/02-project-structure.md`

---

## GitHub Actions

仓库已提供两条工作流：

- `.github/workflows/ci.yml`
  - 触发：`push tag`（匹配 `v*`）
  - 执行：Go 单测、前端 `lint + build`
- `.github/workflows/release-images.yml`
  - 触发：`push tag`（匹配 `v*`）
  - 执行：构建并推送多架构镜像到 GHCR
    - `ghcr.io/imkerbos/k8sinsight-backend:<tag>`
    - `ghcr.io/imkerbos/k8sinsight-frontend:<tag>`
    - 同时推送 `latest`

打 tag 触发示例：

```bash
git tag v1.0.0
git push origin v1.0.0
```

前置条件：

- 仓库 `Settings -> Actions -> General` 允许 workflow 运行。
- 仓库 `Settings -> Actions -> General -> Workflow permissions` 设置为 `Read and write permissions`（用于 `packages: write` 推送 GHCR）。

---

## 通知配置示例

`K8sInsight` 已支持三类通知通道：`Webhook`、`Lark(飞书卡片)`、`Telegram`。

参考配置（`configs/config.yaml`）：

```yaml
notify:
  enabled: true
  webhooks:
    - name: ops-webhook
      url: https://example.com/webhook
      headers:
        Authorization: "Bearer xxx"
  larks:
    - name: ops-lark
      url: https://open.feishu.cn/open-apis/bot/v2/hook/xxxxx
      secret: "" # 可选
  telegrams:
    - name: ops-telegram
      botToken: "123456:ABCDEF"
      chatId: "-1001234567890"
      parseMode: "HTML"
```

说明：
- `notify.enabled=false` 时不会发出任何通知。
- `Lark` 使用 `interactive` 卡片消息。
- `Telegram` 走 Bot API `sendMessage`。

---

## License

本项目采用 [MIT License](LICENSE) 开源协议。

## Author

- GitHub: [@imkerbos](https://github.com/imkerbos)
