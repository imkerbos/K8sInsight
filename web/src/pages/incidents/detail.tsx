import {
  Card,
  Descriptions,
  Divider,
  Empty,
  Select,
  Spin,
  Switch,
  Tabs,
  Tag,
  Timeline,
  Typography,
} from 'antd'
import { useParams } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { useMemo, useState } from 'react'
import dayjs from '../../utils/dayjs'
import { getIncident, getIncidentEvidences } from '../../api/incidents'
import type { Evidence } from '../../types/incident'
import './detail.css'

const { Text, Title } = Typography

/** 证据类型的中文标签 */
const evidenceTypeLabel: Record<string, string> = {
  PodEvents: 'Pod 事件',
  CurrentLogs: '当前日志',
  PreviousLogs: '上次日志',
  PodSnapshot: 'Pod 快照',
  Metrics: '资源指标',
}

/** 证据类型排序权重 */
const evidenceTypeOrder: Record<string, number> = {
  PodEvents: 0,
  CurrentLogs: 1,
  PreviousLogs: 2,
  PodSnapshot: 3,
  Metrics: 4,
}

/** 尝试格式化 JSON 内容 */
function formatContent(content: string): { formatted: string; isJson: boolean } {
  if (!content) return { formatted: '(无内容)', isJson: false }
  try {
    const parsed = JSON.parse(content)
    return { formatted: JSON.stringify(parsed, null, 2), isJson: true }
  } catch {
    return { formatted: content, isJson: false }
  }
}

type SnapshotContainer = {
  name: string
  restartCount: number
  lastState?: string
  exitCode?: number
  resources?: {
    requestsMemory?: string
    limitsMemory?: string
  }
}

type PodSnapshotDigest = {
  containers?: SnapshotContainer[]
}

type PodEventSummary = {
  type: string
  reason: string
  message: string
  lastTimestamp?: string
}

type Diagnosis = {
  title: string
  confidence: '高' | '中' | '低'
  evidenceHits: string[]
  findings: string[]
  nextActions: string[]
}

type PromSample = [number | string, string]
type PromSeries = { metric: Record<string, string>; values: PromSample[] }
type PromMetricsBundle = {
  source: string
  series?: {
    memory?: PromSeries[]
    cpu?: PromSeries[]
  }
}

function parseJsonSafe<T>(raw?: string): T | null {
  if (!raw) return null
  try {
    return JSON.parse(raw) as T
  } catch {
    return null
  }
}

function getLatestEvidenceByType(grouped: [string, Evidence[]][]) {
  const latest: Record<string, Evidence | undefined> = {}
  grouped.forEach(([type, list]) => {
    latest[type] = list[0]
  })
  return latest
}

function buildDiagnosis(anomalyType: string, grouped: [string, Evidence[]][]): Diagnosis | null {
  const latestByType = getLatestEvidenceByType(grouped)
  const snapshot = parseJsonSafe<PodSnapshotDigest>(latestByType.PodSnapshot?.content)
  const podEvents = parseJsonSafe<PodEventSummary[]>(latestByType.PodEvents?.content) ?? []
  const previousLogs = latestByType.PreviousLogs?.content || ''
  const metricsError = latestByType.Metrics?.error
  const container = snapshot?.containers?.[0]
  const restartCount = container?.restartCount ?? 0
  const exitCode = container?.exitCode
  const limitMem = container?.resources?.limitsMemory || '未知'
  const reqMem = container?.resources?.requestsMemory || '未知'
  const eventReasonSet = new Set(podEvents.map(e => e.reason))
  const eventMessageText = podEvents.map(e => e.message).join('\n')
  const hasReason = (reason: string) => eventReasonSet.has(reason)
  const hasMessage = (keyword: string) =>
    podEvents.some(e => e.message.toLowerCase().includes(keyword.toLowerCase()))

  if (anomalyType === 'OOMKilled') {
    const oomExitCode = exitCode ?? 137
    const oomEvent = podEvents.find(e => e.reason === 'OOMKilled')

    const leakHintMatch = previousLogs.match(/Memory usage ~(\d+)MB/g)
    const lastLeakHint = leakHintMatch?.[leakHintMatch.length - 1]
    const hasLeakHint = previousLogs.includes('memory leak') || !!lastLeakHint
    const evidenceHits = [
      '异常类型: OOMKilled',
      `资源配置: request=${reqMem} / limit=${limitMem}`,
    ]
    if (oomEvent) evidenceHits.push(`事件命中: ${oomEvent.reason}`)
    if (hasLeakHint) evidenceHits.push(`日志命中: ${lastLeakHint || 'memory leak'}`)
    if (metricsError) evidenceHits.push(`指标异常: ${metricsError}`)

    const findings = [
      `容器被 OOMKilled，退出码 ${oomExitCode}，说明进程因内存不足被系统杀死。`,
      `容器内存配置为 request=${reqMem} / limit=${limitMem}。`,
      oomEvent?.message || 'Pod 事件中出现 OOMKilled 记录。',
    ]
    if (hasLeakHint) {
      findings.push(`日志出现内存持续增长迹象（${lastLeakHint || 'possible memory leak'}）。`)
    }
    if (metricsError) {
      findings.push(`实时指标未采集成功（${metricsError}），峰值内存曲线缺失。`)
    }

    return {
      title: 'OOM 根因判断',
      confidence: hasLeakHint ? '高' : '中',
      evidenceHits,
      findings,
      nextActions: [
        '提高内存 limit 或降低单次分配量，先让服务恢复稳定。',
        '检查近期代码中的缓存/对象生命周期，重点排查未释放引用。',
        '补齐 metrics-server（或接入 Prometheus）以保留 OOM 前内存曲线。',
      ],
    }
  }

  if (anomalyType === 'CrashLoopBackOff') {
    const backOffEvent = podEvents.find(e => e.reason === 'BackOff' || e.reason === 'CrashLoopBackOff')
    const hasProbeHint = hasMessage('probe') || hasMessage('failed liveness probe') || hasMessage('readiness probe')
    const hasConfigHint = hasMessage('config') || hasMessage('secret') || hasMessage('not found')
    const hasDepHint = hasMessage('connection refused') || hasMessage('timeout') || hasMessage('dial tcp')
    const evidenceHits = ['异常类型: CrashLoopBackOff', `重启次数: ${restartCount}`]
    if (backOffEvent) evidenceHits.push(`事件命中: ${backOffEvent.reason}`)
    if (hasProbeHint) evidenceHits.push('事件命中: probe/unhealthy')
    if (hasConfigHint) evidenceHits.push('事件命中: config/secret/not found')
    if (hasDepHint) evidenceHits.push('事件/日志命中: 依赖连接失败')
    if (typeof exitCode === 'number') evidenceHits.push(`退出码: ${exitCode}`)

    const findings = [
      `容器持续重启并进入 CrashLoopBackOff，当前重启次数约 ${restartCount} 次。`,
      typeof exitCode === 'number' ? `最近一次退出码为 ${exitCode}，建议结合业务程序退出语义排查。` : '容器退出码缺失，建议补充容器终止状态采集。',
      backOffEvent?.message || 'Pod 事件中出现 BackOff/CrashLoopBackOff 迹象。',
    ]
    if (hasProbeHint) findings.push('事件中出现健康检查失败线索，可能由启动慢、探针阈值过严或应用未就绪导致。')
    if (hasConfigHint) findings.push('事件中包含配置/依赖对象缺失线索，建议优先核查 ConfigMap/Secret/环境变量。')
    if (hasDepHint) findings.push('事件或日志中出现依赖连接失败迹象，可能触发应用启动后快速退出。')
    if (metricsError) findings.push(`实时指标采集异常（${metricsError}），无法联合资源曲线判断是否资源抖动触发。`)

    return {
      title: 'CrashLoopBackOff 根因判断',
      confidence: hasProbeHint || hasConfigHint || hasDepHint ? '中' : '低',
      evidenceHits,
      findings,
      nextActions: [
        '优先查看容器最近 1-3 次崩溃前日志，定位首次报错栈。',
        '核查 liveness/readiness 探针参数是否与应用启动时长匹配。',
        '检查依赖服务、ConfigMap、Secret、环境变量与镜像启动命令是否一致。',
      ],
    }
  }

  if (anomalyType === 'ErrorExit') {
    const failedEvent = podEvents.find(e => e.reason === 'Failed')
    const hasPanicHint = /panic|fatal|exception/i.test(previousLogs)
    const hasPermissionHint = /permission denied|operation not permitted/i.test(previousLogs)
    const hasFileHint = /no such file|not found/i.test(previousLogs)
    const evidenceHits = ['异常类型: ErrorExit']
    if (typeof exitCode === 'number') evidenceHits.push(`退出码: ${exitCode}`)
    if (failedEvent) evidenceHits.push(`事件命中: ${failedEvent.reason}`)
    if (hasPanicHint) evidenceHits.push('日志命中: panic/fatal/exception')
    if (hasPermissionHint) evidenceHits.push('日志命中: permission denied')
    if (hasFileHint) evidenceHits.push('日志命中: no such file/not found')

    const findings = [
      typeof exitCode === 'number'
        ? `容器异常退出（ErrorExit），最近一次退出码 ${exitCode}。`
        : '容器异常退出（ErrorExit），但当前证据缺少明确退出码。',
      failedEvent?.message || 'Pod 事件包含 Failed/异常退出线索。',
      previousLogs ? '已采集到崩溃前日志，可用于还原退出前最后调用路径。' : '缺少 PreviousLogs，无法确认应用退出前最后行为。',
    ]
    if (hasPanicHint) findings.push('日志中出现 panic/fatal/exception 关键词，倾向于应用内部异常导致退出。')
    if (hasPermissionHint) findings.push('日志出现权限相关报错，可能是容器权限、挂载目录或安全策略导致进程退出。')
    if (hasFileHint) findings.push('日志出现文件或依赖缺失线索，可能是启动参数或镜像内容不一致。')

    return {
      title: 'ErrorExit 根因判断',
      confidence: hasPanicHint || hasPermissionHint || hasFileHint ? '中' : '低',
      evidenceHits,
      findings,
      nextActions: [
        '按退出码与最后报错日志定位应用代码或启动脚本问题。',
        '核查镜像版本、启动命令与运行时配置是否匹配。',
        '必要时将关键启动日志持久化，避免重启后证据丢失。',
      ],
    }
  }

  if (anomalyType === 'ImagePullBackOff') {
    const pullBackOff = podEvents.find(e => e.reason === 'ImagePullBackOff' || e.reason === 'ErrImagePull')
    const hasAuthHint = hasMessage('unauthorized') || hasMessage('authentication') || hasMessage('denied')
    const hasNotFoundHint = hasMessage('not found') || hasMessage('manifest unknown')
    const hasNetworkHint = hasMessage('i/o timeout') || hasMessage('connection refused') || hasMessage('tls')
    const evidenceHits = ['异常类型: ImagePullBackOff']
    if (pullBackOff) evidenceHits.push(`事件命中: ${pullBackOff.reason}`)
    if (hasAuthHint) evidenceHits.push('事件命中: unauthorized/denied')
    if (hasNotFoundHint) evidenceHits.push('事件命中: image/tag not found')
    if (hasNetworkHint) evidenceHits.push('事件命中: network/tls timeout')

    const findings = [
      'Pod 在拉取镜像阶段失败并进入 ImagePullBackOff，工作负载无法完成启动。',
      pullBackOff?.message || 'Pod 事件包含 ErrImagePull/ImagePullBackOff 记录。',
      hasAuthHint
        ? '事件提示镜像仓库鉴权失败，重点检查 imagePullSecrets 与仓库权限。'
        : hasNotFoundHint
          ? '事件提示镜像或 tag 不存在，需确认镜像名称/tag 是否发布正确。'
          : hasNetworkHint
            ? '事件提示网络或 TLS 异常，需检查节点到镜像仓库的连通性。'
            : '当前事件未给出明确失败类别，建议补充 kubelet 拉取日志。',
    ]

    return {
      title: 'ImagePullBackOff 根因判断',
      confidence: hasAuthHint || hasNotFoundHint || hasNetworkHint ? '高' : '中',
      evidenceHits,
      findings,
      nextActions: [
        '验证镜像地址、tag、仓库可见性以及 imagePullSecrets 引用是否正确。',
        '在节点侧检查到仓库的 DNS、网络与 TLS 证书链。',
        '固定发布流程，避免 tag 漂移或镜像尚未推送即部署。',
      ],
    }
  }

  if (anomalyType === 'CreateContainerConfigError') {
    const cfgErrEvent = podEvents.find(e => e.reason === 'CreateContainerConfigError' || e.reason === 'Failed')
    const hasSecretHint = hasMessage('secret') && hasMessage('not found')
    const hasConfigMapHint = hasMessage('configmap') && hasMessage('not found')
    const hasEnvHint = hasMessage('env') || hasMessage('invalid')
    const evidenceHits = ['异常类型: CreateContainerConfigError']
    if (cfgErrEvent) evidenceHits.push(`事件命中: ${cfgErrEvent.reason}`)
    if (hasSecretHint) evidenceHits.push('事件命中: secret not found')
    if (hasConfigMapHint) evidenceHits.push('事件命中: configmap not found')
    if (hasEnvHint) evidenceHits.push('事件命中: env invalid')

    const findings = [
      '容器创建前配置校验失败（CreateContainerConfigError），未进入正常启动阶段。',
      cfgErrEvent?.message || 'Pod 事件中包含配置错误线索。',
      hasSecretHint
        ? '发现 Secret 缺失线索，可能导致环境变量或挂载注入失败。'
        : hasConfigMapHint
          ? '发现 ConfigMap 缺失线索，可能导致配置文件或环境变量无法解析。'
          : hasEnvHint
            ? '事件包含环境变量或字段不合法线索。'
            : '暂未识别到具体配置对象，需回看完整事件详情。',
    ]

    return {
      title: 'CreateContainerConfigError 根因判断',
      confidence: hasSecretHint || hasConfigMapHint || hasEnvHint ? '高' : '中',
      evidenceHits,
      findings,
      nextActions: [
        '逐项核对 Deployment/StatefulSet 中引用的 Secret、ConfigMap、env 与 volume 声明。',
        '确认目标 namespace 下对象存在且 key 正确。',
        '将关键配置做发布前校验，避免运行时才暴露引用错误。',
      ],
    }
  }

  if (anomalyType === 'FailedScheduling') {
    const scheduleEvent = podEvents.find(e => e.reason === 'FailedScheduling')
    const hasResourceHint = hasMessage('insufficient cpu') || hasMessage('insufficient memory')
    const hasTaintHint = hasMessage('taint') || hasMessage('didn\'t tolerate')
    const hasAffinityHint = hasMessage('affinity') || hasMessage('selector')
    const evidenceHits = ['异常类型: FailedScheduling']
    if (scheduleEvent) evidenceHits.push(`事件命中: ${scheduleEvent.reason}`)
    if (hasResourceHint) evidenceHits.push('事件命中: insufficient cpu/memory')
    if (hasTaintHint) evidenceHits.push('事件命中: taint/toleration mismatch')
    if (hasAffinityHint) evidenceHits.push('事件命中: affinity/selector conflict')

    const findings = [
      'Pod 调度失败（FailedScheduling），尚未分配到可用节点。',
      scheduleEvent?.message || 'Pod 事件中存在 FailedScheduling 记录。',
      hasResourceHint
        ? '当前线索指向集群可用资源不足（CPU/内存不足）导致无法调度。'
        : hasTaintHint
          ? '当前线索指向 taint/toleration 不匹配。'
          : hasAffinityHint
            ? '当前线索指向节点亲和性或选择器约束过严。'
            : '当前仅确认调度失败，需查看调度器完整决策信息。',
    ]

    return {
      title: 'FailedScheduling 根因判断',
      confidence: hasResourceHint || hasTaintHint || hasAffinityHint ? '高' : '中',
      evidenceHits,
      findings,
      nextActions: [
        '核查 requests/limits 与节点剩余资源，必要时扩容节点池。',
        '检查 taints/tolerations、nodeSelector、affinity 规则是否冲突。',
        '排查 PDB/拓扑约束是否过严导致可调度节点集合过小。',
      ],
    }
  }

  if (anomalyType === 'Evicted') {
    const evictedEvent = podEvents.find(e => e.reason === 'Evicted')
    const hasMemoryPressure = hasMessage('memory pressure')
    const hasDiskPressure = hasMessage('disk pressure') || hasMessage('ephemeral-storage')
    const evidenceHits = ['异常类型: Evicted', `资源配置: request=${reqMem} / limit=${limitMem}`]
    if (evictedEvent) evidenceHits.push(`事件命中: ${evictedEvent.reason}`)
    if (hasMemoryPressure) evidenceHits.push('事件命中: memory pressure')
    if (hasDiskPressure) evidenceHits.push('事件命中: disk/ephemeral-storage pressure')

    const findings = [
      'Pod 被节点驱逐（Evicted），通常由节点资源压力触发。',
      evictedEvent?.message || 'Pod 事件中包含 Evicted 记录。',
      hasMemoryPressure
        ? '线索指向节点内存压力，需重点排查节点级内存争用。'
        : hasDiskPressure
          ? '线索指向节点磁盘/临时存储压力。'
          : '当前未识别具体压力维度，建议补充节点状态与 kubelet 事件。',
      `容器历史资源配置为 request=${reqMem} / limit=${limitMem}。`,
    ]

    return {
      title: 'Evicted 根因判断',
      confidence: hasMemoryPressure || hasDiskPressure ? '中' : '低',
      evidenceHits,
      findings,
      nextActions: [
        '排查节点资源压力来源，重点关注高波动工作负载与临时存储占用。',
        '优化 Pod requests/limits 与优先级，降低被驱逐概率。',
        '必要时启用集群自动扩缩容并设置合理资源保障策略。',
      ],
    }
  }

  if (anomalyType === 'RestartIncrement') {
    const hasBackOff = hasReason('BackOff') || hasReason('CrashLoopBackOff')
    const hasOOM = hasReason('OOMKilled') || /oom/i.test(eventMessageText) || /oom/i.test(previousLogs)
    const evidenceHits = ['异常类型: RestartIncrement', `重启次数: ${restartCount}`]
    if (hasBackOff) evidenceHits.push('事件命中: BackOff/CrashLoopBackOff')
    if (hasOOM) evidenceHits.push('事件/日志命中: OOM')
    if (typeof exitCode === 'number') evidenceHits.push(`退出码: ${exitCode}`)

    const findings = [
      `检测到重启次数持续增加（当前约 ${restartCount} 次），存在稳定性退化风险。`,
      hasBackOff ? '事件中存在 BackOff 迹象，说明容器可能处于“启动-崩溃”循环。' : '未观测到明显 BackOff，可能是间歇性失败触发重启。',
      hasOOM ? '发现 OOM 相关线索，建议与内存峰值和限制配置联合分析。' : '当前未识别 OOM 线索，需结合退出码与应用日志继续定位。',
    ]

    return {
      title: 'RestartIncrement 根因判断',
      confidence: hasBackOff || hasOOM ? '中' : '低',
      evidenceHits,
      findings,
      nextActions: [
        '按时间线对齐重启点、事件与错误日志，识别触发重启的共性条件。',
        '若伴随 OOM/探针失败，优先处理资源与健康检查参数。',
        '对关键工作负载设置重启阈值告警，提前拦截故障放大。',
      ],
    }
  }

  if (anomalyType === 'StateOscillation') {
    const hasProbeHint = hasMessage('probe') || hasMessage('unhealthy')
    const hasNodeFlapHint = hasMessage('node not ready') || hasMessage('network')
    const evidenceHits = ['异常类型: StateOscillation']
    if (hasProbeHint) evidenceHits.push('事件命中: probe/unhealthy')
    if (hasNodeFlapHint) evidenceHits.push('事件命中: node/network flap')
    if (!hasProbeHint && !hasNodeFlapHint) evidenceHits.push('证据不足: 仅识别到状态震荡')

    const findings = [
      '检测到 Pod 状态频繁震荡（StateOscillation），表现为短周期状态切换。',
      hasProbeHint
        ? '事件中出现探针不稳定线索，可能导致 Ready/NotReady 来回切换。'
        : '当前未识别明显探针异常，需要结合事件时间线进一步确认震荡触发器。',
      hasNodeFlapHint
        ? '存在节点或网络抖动线索，可能导致状态同步不稳定。'
        : '尚未发现明确节点抖动证据。',
    ]

    return {
      title: 'StateOscillation 根因判断',
      confidence: hasProbeHint || hasNodeFlapHint ? '中' : '低',
      evidenceHits,
      findings,
      nextActions: [
        '按分钟级查看 Pod 事件时间线，定位首次震荡触发点。',
        '校准探针阈值与超时时间，避免瞬时抖动导致状态翻转。',
        '排查节点网络与 kubelet 健康状态，确认是否存在底层抖动。',
      ],
    }
  }

  return null
}

type TimeValue = {
  ts: number
  value: number
}

function toTimeValues(values: PromSample[] | undefined): TimeValue[] {
  if (!values) return []
  return values
    .map((v) => ({ ts: Number(v[0]), value: Number(v[1]) }))
    .filter((p) => Number.isFinite(p.ts) && Number.isFinite(p.value))
}

function calcStats(values: number[]) {
  if (!values.length) return null
  const min = Math.min(...values)
  const max = Math.max(...values)
  const current = values[values.length - 1]
  const avg = values.reduce((a, b) => a + b, 0) / values.length
  return { min, max, current, avg }
}

function fmtNumber(n: number, digits = 2) {
  return n.toFixed(digits)
}

function parseK8sMemoryToMiB(raw?: string): number | null {
  if (!raw) return null
  const text = raw.trim()
  const m = text.match(/^([0-9]*\.?[0-9]+)\s*([KMGTP]i?|[kmgpt]i?)?$/)
  if (!m) return null

  const value = Number(m[1])
  if (!Number.isFinite(value)) return null
  const unit = (m[2] || '').toUpperCase()

  const bin: Record<string, number> = {
    KI: 1 / 1024,
    MI: 1,
    GI: 1024,
    TI: 1024 * 1024,
    PI: 1024 * 1024 * 1024,
  }
  const dec: Record<string, number> = {
    K: 1000 / (1024 * 1024),
    M: 1000 * 1000 / (1024 * 1024),
    G: 1000 * 1000 * 1000 / (1024 * 1024),
    T: 1000 * 1000 * 1000 * 1000 / (1024 * 1024),
    P: 1000 * 1000 * 1000 * 1000 * 1000 / (1024 * 1024),
  }

  if (!unit) return value / (1024 * 1024) // 视为 bytes
  if (bin[unit] !== undefined) return value * bin[unit]
  if (dec[unit] !== undefined) return value * dec[unit]
  return null
}

function TimeSeriesChart({
  samples,
  color,
  height = 220,
  unit,
  seriesLabel,
  limitValue,
  limitLabel,
}: {
  samples: TimeValue[]
  color: string
  height?: number
  unit: string
  seriesLabel: string
  limitValue?: number | null
  limitLabel?: string
}) {
  const width = 760
  const padding = { top: 20, right: 16, bottom: 34, left: 52 }
  const plotW = width - padding.left - padding.right
  const plotH = height - padding.top - padding.bottom

  if (!samples.length) {
    return <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无曲线数据" />
  }

  const values = samples.map((s) => s.value)
  const sourceMin = Math.min(...values)
  const sourceMax = Math.max(...values)
  const max = limitValue && Number.isFinite(limitValue) ? Math.max(sourceMax, limitValue) : sourceMax
  const min = Math.min(0, sourceMin)
  const range = max - min || 1
  const stepX = samples.length > 1 ? plotW / (samples.length - 1) : plotW

  const pointPairs = samples.map((v, i) => {
    const x = padding.left + i * stepX
    const y = padding.top + plotH - ((v.value - min) / range) * plotH
    return { x, y }
  })

  const linePoints = pointPairs.map((p) => `${p.x},${p.y}`).join(' ')
  const areaPoints = [
    `${padding.left},${padding.top + plotH}`,
    ...pointPairs.map((p) => `${p.x},${p.y}`),
    `${padding.left + plotW},${padding.top + plotH}`,
  ].join(' ')

  const firstTs = dayjs.unix(samples[0].ts).format('HH:mm:ss')
  const lastTs = dayjs.unix(samples[samples.length - 1].ts).format('HH:mm:ss')
  const stepSec = samples.length > 1 ? Math.round(samples[1].ts - samples[0].ts) : 0

  const yTickValues = [max, min + (range * 2) / 3, min + range / 3, min]
  const xTicks = [0, 0.25, 0.5, 0.75, 1].map((r) => {
    const idx = Math.min(samples.length - 1, Math.round((samples.length - 1) * r))
    const x = padding.left + plotW * r
    const label = dayjs.unix(samples[idx].ts).format('HH:mm:ss')
    return { x, label }
  })

  const limitY = limitValue && Number.isFinite(limitValue)
    ? padding.top + plotH - (((limitValue as number) - min) / range) * plotH
    : null

  const gradientId = `trend-${seriesLabel.replace(/\s+/g, '-').toLowerCase()}`

  return (
    <div className="incident-chart-wrap">
      <div className="incident-chart-legend">
        <span className="incident-chart-legend-item">
          <span className="incident-chart-dot" style={{ backgroundColor: color }} />
          {seriesLabel}
        </span>
        {limitY !== null && (
          <span className="incident-chart-legend-item danger">Limit: {limitLabel || `${fmtNumber(limitValue as number)} ${unit}`}</span>
        )}
      </div>

      <svg width="100%" viewBox={`0 0 ${width} ${height}`} className="incident-chart-svg" role="img" aria-label={seriesLabel}>
        <defs>
          <linearGradient id={gradientId} x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stopColor={color} stopOpacity="0.3" />
            <stop offset="100%" stopColor={color} stopOpacity="0" />
          </linearGradient>
        </defs>

        {yTickValues.map((v, i) => {
          const y = padding.top + (plotH * i) / (yTickValues.length - 1)
          return (
            <g key={`y-${i}`}>
              <line x1={padding.left} y1={y} x2={padding.left + plotW} y2={y} stroke="#e6edf7" strokeWidth="1" />
              <text x={8} y={y + 4} fontSize="10" fill="#7b8ca7">{fmtNumber(v)} {unit}</text>
            </g>
          )
        })}

        {xTicks.map((t, i) => (
          <g key={`x-${i}`}>
            <line x1={t.x} y1={padding.top} x2={t.x} y2={padding.top + plotH} stroke="#f1f5fa" strokeWidth="1" />
            <text x={t.x - 18} y={height - 10} fontSize="10" fill="#7b8ca7">{t.label}</text>
          </g>
        ))}

        {limitY !== null && (
          <>
            <line
              x1={padding.left}
              y1={limitY}
              x2={padding.left + plotW}
              y2={limitY}
              stroke="#ff4d4f"
              strokeWidth="1.4"
              strokeDasharray="5 4"
            />
            <text x={padding.left + plotW - 120} y={limitY - 5} fontSize="10" fill="#cf1322">
              limit {limitLabel || `${fmtNumber(limitValue as number)} ${unit}`}
            </text>
          </>
        )}

        <polyline fill={`url(#${gradientId})`} stroke="none" points={areaPoints} />
        <polyline fill="none" stroke={color} strokeWidth="2.2" points={linePoints} />
      </svg>

      <Text type="secondary" className="incident-chart-meta">
        时间范围: {firstTs} - {lastTs}，采样点: {samples.length}，间隔: {stepSec > 0 ? `${stepSec}s` : '-'}
      </Text>
    </div>
  )
}

/** 单条证据渲染 */
function EvidenceContent({ evidence }: { evidence: Evidence }) {
  const [showRaw, setShowRaw] = useState(false)
  const { formatted, isJson } = useMemo(() => formatContent(evidence.content), [evidence.content])

  return (
    <Card size="small" className="incident-evidence-card">
      <div className="incident-evidence-header">
        <Text type="secondary" style={{ fontSize: 12 }}>
          采集时间: {dayjs(evidence.collectedAt).format('YYYY-MM-DD HH:mm:ss')}
          {evidence.error && <Tag color="red" style={{ marginLeft: 8 }}>采集异常: {evidence.error}</Tag>}
        </Text>
        {isJson && (
          <Switch
            checkedChildren="原始"
            unCheckedChildren="格式化"
            checked={showRaw}
            onChange={setShowRaw}
            size="small"
          />
        )}
      </div>
      <pre className="incident-evidence-pre">
        {showRaw ? (evidence.content || '(无内容)') : formatted}
      </pre>
    </Card>
  )
}

/** 按类型分组，每组内按采集时间倒序，只展示最新一条，可展开历史 */
function groupEvidences(evidences: Evidence[]) {
  const groups = new Map<string, Evidence[]>()
  for (const e of evidences) {
    const list = groups.get(e.type) || []
    list.push(e)
    groups.set(e.type, list)
  }
  // 组内按时间倒序
  for (const list of groups.values()) {
    list.sort((a, b) => dayjs(b.collectedAt).valueOf() - dayjs(a.collectedAt).valueOf())
  }
  // 按类型排序
  return [...groups.entries()].sort(
    ([a], [b]) => (evidenceTypeOrder[a] ?? 99) - (evidenceTypeOrder[b] ?? 99)
  )
}

export default function IncidentDetail() {
  const { id } = useParams<{ id: string }>()
  const [selectedRound, setSelectedRound] = useState<string>('latest')

  const { data: incident, isLoading } = useQuery({
    queryKey: ['incident', id],
    queryFn: () => getIncident(id!),
    enabled: !!id,
  })

  const { data: evidences } = useQuery({
    queryKey: ['incident', id, 'evidences'],
    queryFn: () => getIncidentEvidences(id!),
    enabled: !!id,
  })

  const grouped = useMemo(() => groupEvidences(evidences ?? []), [evidences])
  const diagnosis = useMemo(() => buildDiagnosis(incident?.anomalyType || '', grouped), [incident?.anomalyType, grouped])
  const latestByType = useMemo(() => getLatestEvidenceByType(grouped), [grouped])
  const promMetrics = useMemo(
    () => parseJsonSafe<PromMetricsBundle>(latestByType.Metrics?.content),
    [latestByType.Metrics?.content]
  )
  const latestSnapshot = useMemo(
    () => parseJsonSafe<PodSnapshotDigest>(latestByType.PodSnapshot?.content),
    [latestByType.PodSnapshot?.content]
  )
  const memorySeries = useMemo(
    () => toTimeValues(promMetrics?.series?.memory?.[0]?.values).map((p) => ({ ...p, value: p.value / (1024 * 1024) })),
    [promMetrics]
  )
  const cpuSeries = useMemo(
    () => toTimeValues(promMetrics?.series?.cpu?.[0]?.values),
    [promMetrics]
  )
  const memoryStats = useMemo(() => calcStats(memorySeries.map((p) => p.value)), [memorySeries])
  const cpuStats = useMemo(() => calcStats(cpuSeries.map((p) => p.value)), [cpuSeries])
  const memoryLimitRaw = latestSnapshot?.containers?.[0]?.resources?.limitsMemory
  const memoryLimitMiB = useMemo(() => parseK8sMemoryToMiB(memoryLimitRaw), [memoryLimitRaw])
  const hasPrometheusMetrics = promMetrics?.source === 'prometheus'

  // 采集轮次：按 collectedAt 聚合（同一秒算一轮）
  const rounds = useMemo(() => {
    const roundSet = new Map<string, string>()
    for (const e of evidences ?? []) {
      const key = dayjs(e.collectedAt).format('YYYY-MM-DD HH:mm:ss')
      if (!roundSet.has(key)) {
        roundSet.set(key, key)
      }
    }
    return [...roundSet.keys()].sort().reverse()
  }, [evidences])

  // 根据轮次筛选证据
  const filteredGrouped = useMemo(() => {
    if (selectedRound === 'latest') {
      return grouped.map(([type, list]) => [type, [list[0]]] as [string, Evidence[]])
    }
    if (selectedRound === 'all') {
      return grouped
    }
    return grouped
      .map(([type, list]) => {
        const filtered = list.filter(e => dayjs(e.collectedAt).format('YYYY-MM-DD HH:mm:ss') === selectedRound)
        return [type, filtered] as [string, Evidence[]]
      })
      .filter(([, list]) => list.length > 0)
  }, [grouped, selectedRound])

  const podNames: string[] = useMemo(() => {
    if (!incident) return []
    try { return JSON.parse(incident.podNames) } catch { return [] }
  }, [incident])

  if (isLoading) return <Spin size="large" style={{ display: 'block', marginTop: 100, textAlign: 'center' }} />
  if (!incident) return <Empty description="事件未找到" />

  const tabItems = filteredGrouped.map(([type, list]) => ({
    key: type,
    label: `${evidenceTypeLabel[type] || type}${list.length > 1 ? ` (${list.length})` : ''}`,
    children: (
      <div>
        {list.length > 1 ? (
          <Timeline
            style={{ marginTop: 12 }}
            items={list.map(e => ({
              children: <EvidenceContent key={e.id} evidence={e} />,
            }))}
          />
        ) : (
          <EvidenceContent evidence={list[0]} />
        )}
      </div>
    ),
  }))

  return (
    <div className="incident-detail-page">
      <Card className="incident-hero" bordered={false}>
        <div className="incident-hero-head">
          <div>
            <Title level={4} style={{ margin: 0 }}>事件详情</Title>
            <Text type="secondary">{incident.namespace} / {incident.ownerKind}/{incident.ownerName}</Text>
          </div>
          <div className="incident-hero-tags">
            <Tag color={incident.state === 'Active' ? 'red' : incident.state === 'Resolved' ? 'green' : 'orange'}>
              {incident.state}
            </Tag>
            <Tag color="processing">{incident.anomalyType}</Tag>
            <Tag color="default">发生 {incident.count} 次</Tag>
          </div>
        </div>
        <div className="incident-hero-msg">{incident.message}</div>
      </Card>

      <Card title="基础信息" className="incident-base-card">
        <Descriptions
          size="small"
          labelStyle={{ width: 100 }}
          column={{ xs: 1, sm: 1, md: 2, xl: 3 }}
        >
          <Descriptions.Item label="命名空间">{incident.namespace}</Descriptions.Item>
          <Descriptions.Item label="Owner">{incident.ownerKind}/{incident.ownerName}</Descriptions.Item>
          <Descriptions.Item label="首次发现">{dayjs(incident.firstSeen).format('YYYY-MM-DD HH:mm:ss')}</Descriptions.Item>
          <Descriptions.Item label="最后发现">{dayjs(incident.lastSeen).format('YYYY-MM-DD HH:mm:ss')}</Descriptions.Item>
          <Descriptions.Item label="Dedup Key" span={2}>
            <code className="incident-dedup-key">{incident.dedupKey}</code>
          </Descriptions.Item>
          <Descriptions.Item label="涉及 Pod" span={3}>
            <div className="incident-pods-wrap">
              {podNames.length ? podNames.map(name => <Tag key={name}>{name}</Tag>) : <Text type="secondary">无 Pod 信息</Text>}
            </div>
          </Descriptions.Item>
        </Descriptions>
      </Card>

      {diagnosis && (
        <Card
          title={diagnosis.title}
          className="incident-diagnosis"
          extra={<Tag color={diagnosis.confidence === '高' ? 'green' : diagnosis.confidence === '中' ? 'orange' : 'red'}>置信度: {diagnosis.confidence}</Tag>}
        >
          <div className="incident-diagnosis-hits">
            {diagnosis.evidenceHits.map(hit => (
              <Tag key={hit} color="blue">{hit}</Tag>
            ))}
          </div>
          <div className="incident-diagnosis-section">
            <Text strong>关键判断</Text>
            <ul>
              {diagnosis.findings.map((f) => <li key={f}>{f}</li>)}
            </ul>
          </div>
          <Divider style={{ margin: '14px 0' }} />
          <div className="incident-diagnosis-section">
            <Text strong>建议动作</Text>
            <ul>
              {diagnosis.nextActions.map((a) => <li key={a}>{a}</li>)}
            </ul>
          </div>
        </Card>
      )}

      {hasPrometheusMetrics && (
        <Card title="资源趋势（Prometheus）" className="incident-metrics-card">
          <div className="incident-chart-block">
            <Text strong>内存工作集趋势（MiB）</Text>
            <TimeSeriesChart
              samples={memorySeries}
              color="#52c41a"
              unit="MiB"
              seriesLabel="Working Set"
              limitValue={memoryLimitMiB}
              limitLabel={memoryLimitRaw || undefined}
            />
            {memoryStats && (
              <div className="incident-stats-row">
                <span>当前 {fmtNumber(memoryStats.current)} MiB</span>
                <span>峰值 {fmtNumber(memoryStats.max)} MiB</span>
                <span>均值 {fmtNumber(memoryStats.avg)} MiB</span>
                <span>最小 {fmtNumber(memoryStats.min)} MiB</span>
              </div>
            )}
          </div>

          <div className="incident-chart-block">
            <Text strong>CPU 使用趋势（cores）</Text>
            <TimeSeriesChart samples={cpuSeries} color="#1677ff" unit="cores" seriesLabel="CPU Usage" />
            {cpuStats && (
              <div className="incident-stats-row">
                <span>当前 {fmtNumber(cpuStats.current, 3)} cores</span>
                <span>峰值 {fmtNumber(cpuStats.max, 3)} cores</span>
                <span>均值 {fmtNumber(cpuStats.avg, 3)} cores</span>
                <span>最小 {fmtNumber(cpuStats.min, 3)} cores</span>
              </div>
            )}
          </div>
        </Card>
      )}

      <Card
        title="采集证据"
        extra={rounds.length > 1 ? (
          <Select
            value={selectedRound}
            onChange={setSelectedRound}
            style={{ width: 240 }}
            options={[
              { label: '最新一次采集', value: 'latest' },
              { label: '全部采集记录', value: 'all' },
              ...rounds.map(r => ({ label: `采集轮次: ${r}`, value: r })),
            ]}
          />
        ) : null}
        className="incident-evidence-tabs"
      >
        {tabItems.length > 0 ? (
          <Tabs items={tabItems} />
        ) : (
          <Empty description="暂无证据" />
        )}
      </Card>
    </div>
  )
}
