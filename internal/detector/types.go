package detector

// 所有核心领域类型已迁移到 domain 包
// 这里通过 type alias 保持向后兼容，后续逐步迁移直接引用

import "github.com/kerbos/k8sinsight/internal/domain"

// AnomalyType 异常类型枚举
type AnomalyType = domain.AnomalyType

const (
	AnomalyCrashLoopBackOff           = domain.AnomalyCrashLoopBackOff
	AnomalyOOMKilled                  = domain.AnomalyOOMKilled
	AnomalyErrorExit                  = domain.AnomalyErrorExit
	AnomalyRestartIncrement           = domain.AnomalyRestartIncrement
	AnomalyImagePullBackOff           = domain.AnomalyImagePullBackOff
	AnomalyCreateContainerConfigError = domain.AnomalyCreateContainerConfigError
	AnomalyFailedScheduling           = domain.AnomalyFailedScheduling
	AnomalyEvicted                    = domain.AnomalyEvicted
	AnomalyStateOscillation           = domain.AnomalyStateOscillation
)

// DetectionSource 检测来源
type DetectionSource = domain.DetectionSource

const (
	SourcePodState = domain.SourcePodState
	SourceK8sEvent = domain.SourceK8sEvent
)

// AnomalyEvent 异常事件
type AnomalyEvent = domain.AnomalyEvent
