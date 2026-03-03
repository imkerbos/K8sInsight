# 项目目录结构设计（实现对齐版）

> 状态：与当前仓库一致  
> 更新日期：2026-03-03

---

## 1. 设计原则

- 后端遵循 `cmd + internal` 结构。
- `internal` 按业务模块拆分（watcher/detector/collector/aggregator/store/api）。
- 前后端同仓库，构建与运行分离。
- 数据库 schema 通过 `migrations/` 管理。

---

## 2. 当前目录结构

```
K8sInsight/
├── cmd/
│   └── k8sinsight/
│       └── main.go
├── internal/
│   ├── aggregator/
│   │   ├── aggregator.go
│   │   └── dedup.go
│   ├── api/
│   │   ├── router.go
│   │   ├── middleware/
│   │   │   └── auth.go
│   │   └── handler/
│   │       ├── auth_handler.go
│   │       ├── cluster_handler.go
│   │       ├── health_handler.go
│   │       ├── incident_handler.go
│   │       ├── local_cache.go
│   │       ├── monitor_rule_handler.go
│   │       ├── role_handler.go
│   │       ├── setting_handler.go
│   │       └── user_handler.go
│   ├── auth/
│   │   ├── password.go
│   │   ├── seed.go
│   │   └── token.go
│   ├── cluster/
│   │   └── manager.go
│   ├── collector/
│   │   ├── collector.go
│   │   ├── events.go
│   │   ├── logs.go
│   │   ├── metrics.go
│   │   └── pod_snapshot.go
│   ├── config/
│   │   ├── config.go
│   │   └── loader.go
│   ├── core/
│   │   ├── errx.go
│   │   ├── k8s.go
│   │   └── logger.go
│   ├── detector/
│   │   ├── detector.go
│   │   ├── rules.go
│   │   ├── types.go
│   │   └── types_test.go
│   ├── notify/
│   │   ├── dispatcher.go
│   │   ├── notifier.go
│   │   └── sink/
│   │       └── webhook.go
│   ├── store/
│   │   ├── db.go
│   │   ├── model/
│   │   │   ├── cluster.go
│   │   │   ├── evidence.go
│   │   │   ├── incident.go
│   │   │   ├── monitor_rule.go
│   │   │   ├── role.go
│   │   │   ├── system_setting.go
│   │   │   └── user.go
│   │   └── repository/
│   │       ├── cluster_repo.go
│   │       ├── evidence_repo.go
│   │       ├── incident_repo.go
│   │       ├── monitor_rule_repo.go
│   │       ├── role_repo.go
│   │       ├── setting_repo.go
│   │       └── user_repo.go
│   └── watcher/
│       ├── event_handler.go
│       ├── filter.go
│       ├── pod_handler.go
│       └── watcher.go
├── web/
│   ├── src/
│   │   ├── api/
│   │   ├── assets/
│   │   ├── components/
│   │   ├── contexts/
│   │   ├── hooks/
│   │   ├── pages/
│   │   │   ├── clusters/
│   │   │   ├── dashboard/
│   │   │   ├── incidents/
│   │   │   ├── login/
│   │   │   ├── monitor-rules/
│   │   │   ├── settings/
│   │   │   └── timeline/
│   │   ├── types/
│   │   └── utils/
│   ├── package.json
│   └── vite.config.ts
├── configs/
│   └── config.yaml
├── migrations/
│   ├── 000001_create_incidents.*
│   ├── 000002_create_evidences.*
│   ├── 000003_create_timeline_entries.*
│   ├── 000004_create_clusters.*
│   ├── 000005_create_users.*
│   └── 000006_optimize_incidents_queries.*
├── deploy/
│   ├── docker/
│   └── test-pods/
├── docs/
│   └── design/
├── scripts/                  # 当前为空（按需求已清空）
├── Makefile
├── go.mod
├── go.sum
└── README.md
```

---

## 3. 模块职责

| 模块 | 主要职责 | 关键文件 |
|---|---|---|
| `watcher` | 监听 Pod/Event 变化并过滤 | `watcher.go`, `pod_handler.go`, `event_handler.go` |
| `detector` | 异常规则匹配，产出 `AnomalyEvent` | `detector.go`, `rules.go`, `types.go` |
| `collector` | 异常证据并发采集 | `collector.go`, `logs.go`, `events.go`, `pod_snapshot.go`, `metrics.go` |
| `aggregator` | 事件去重聚合与状态流转 | `aggregator.go`, `dedup.go` |
| `store` | PostgreSQL 持久化（模型+仓储） | `store/model/*`, `store/repository/*` |
| `api` | HTTP API（认证、用户、角色、集群、事件） | `router.go`, `handler/*` |
| `cluster` | 多集群管道生命周期管理 | `manager.go` |
| `notify` | 通知分发与 sink | `dispatcher.go`, `sink/webhook.go` |
| `web` | 前端页面与 API 调用 | `web/src/pages/*`, `web/src/api/*` |

---

## 4. 关键数据流

`Watcher -> Detector -> Collector/Aggregator/Notify -> Store -> API -> Web`

---

## 5. 当前实现备注

1. `scripts/` 目录目前为空（已按要求删除测试脚本）。
2. `docs/acceptance/` 已删除，仅保留 `docs/design/`。
3. 性能相关查询优化已落在：
   - `internal/store/repository/incident_repo.go`
   - `internal/api/handler/incident_handler.go`
   - `migrations/000006_optimize_incidents_queries.*`
