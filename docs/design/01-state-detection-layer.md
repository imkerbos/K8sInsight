# 状态感知层设计文档（实现对齐版）

> 模块代号：Watcher / Detector  
> 状态：与当前代码一致  
> 更新日期：2026-03-03

---

## 1. 概述

状态感知层负责监听 Kubernetes Pod 与 Warning Event，检测异常后产出 `AnomalyEvent`，并分发给：

- `collector`：采集日志/事件/快照/指标证据
- `aggregator`：按 `dedupKey` 聚合事件并维护状态机
- `notify`：通知分发

核心入口：
- `internal/watcher/watcher.go`
- `internal/detector/detector.go`

---

## 2. 当前实现架构

### 2.1 双通道

1. Pod 状态通道（主）
- Informer：`Pods`
- 处理器：`internal/watcher/pod_handler.go`
- 触发：`Add/Update` 且容器状态有意义变化

2. K8s Event 通道（补充）
- Informer：`Events`
- 处理器：`internal/watcher/event_handler.go`
- 仅处理 `type=Warning` 且 `involvedObject.kind=Pod`

### 2.2 处理链路

`Watcher -> Detector -> (Collector, Aggregator, Notifier)`

---

## 3. 异常类型：定义 vs 实际检测

定义位置：`internal/detector/types.go`

### 3.1 已实现检测规则

来自 Pod 状态规则（`internal/detector/rules.go`）：
- `CrashLoopBackOff`
- `OOMKilled`
- `ErrorExit`
- `ImagePullBackOff`
- `CreateContainerConfigError`
- `Evicted`

来自 Event 通道（`internal/detector/detector.go`）：
- `FailedScheduling`（Warning Event reason）

### 3.2 已定义但当前未实现检测规则

- `RestartIncrement`
- `StateOscillation`

说明：前端与类型定义已包含这两类，但后端默认规则集中尚未注册对应 Rule。

---

## 4. Dedup 与状态机（当前实现）

### 4.1 DedupKey

生成逻辑：`internal/detector/types.go`

格式：
`{clusterID}/{namespace}/Pod/{podIdentity}/{anomalyType}`

其中 `podIdentity` 优先 `PodUID`，缺失时回退 `PodName`。

这意味着聚合粒度是 **Pod 级别**，而非 Owner 级别。

### 4.2 状态机

状态定义：`Detecting -> Active -> Resolved`（`Archived` 仅枚举）

- `Detecting`：新建事件后进入聚合等待窗口（`groupWait`，默认 30s）
- `Active`：窗口结束后转活跃
- `Resolved`：`activeWindow` 超时无新事件自动转已恢复（默认 6h）

当前未实现：
- 自动/手动转 `Archived` 的业务路径
- `resolveWait` 参数在聚合器中未使用

---

## 5. 证据采集（当前实现）

实现位置：`internal/collector/collector.go`

采集项：
- `PreviousLogs`
- `CurrentLogs`
- `PodEvents`
- `PodSnapshot`
- `Metrics`（可配置开关）

策略：
- 并发采集
- 每项独立超时（`timeoutPerItem`）
- 单项失败不阻断整体
- 采集后通过 channel 交给 aggregator 入库

---

## 6. 配置与过滤

配置结构：`internal/config/config.go` + `configs/config.yaml`

支持：
- `scope`（cluster/namespaces）
- 命名空间 include/exclude
- `labelSelector`
- `excludePods`
- `resyncPeriod`

监控规则动态覆盖（当前仅部分生效）：
- 在 `cluster/manager.go` 中会将规则里的 `watchScope`、`labelSelector` 覆盖到 watcher
- `watchNamespaces`、`anomalyTypes` 目前未进入实际过滤链路

---

## 7. 已知限制与风险

1. 规则覆盖不完整：`RestartIncrement/StateOscillation` 未实现。
2. `Archived` 无状态流转实现。
3. `monitor_rule.anomalyTypes/watchNamespaces` 未生效到检测流程。
4. 聚合配置 `resolveWait` 尚未使用。

---

## 8. 后续改进建议

1. 增加 `RestartIncrement`、`StateOscillation` Rule 并纳入默认注册。
2. 补齐 `Archived` 归档流程（自动或 API 手动归档）。
3. 将 `monitor_rule` 的 `watchNamespaces/anomalyTypes` 下推到 Filter/Detector。
4. 实现 `resolveWait`，避免抖动场景过早 `Resolved`。
