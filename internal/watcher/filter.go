package watcher

import (
	"path/filepath"

	corev1 "k8s.io/api/core/v1"

	"github.com/kerbos/k8sinsight/internal/config"
)

// Filter 监控范围过滤器
type Filter struct {
	cfg config.WatchConfig
}

// NewFilter 创建过滤器
func NewFilter(cfg config.WatchConfig) *Filter {
	return &Filter{cfg: cfg}
}

// ShouldProcess 判断是否应处理该 Pod
// 过滤优先级: excludePods > namespaces.exclude > namespaces.include > labelSelector
func (f *Filter) ShouldProcess(pod *corev1.Pod) bool {
	// 1. 明确排除列表（最高优先级）
	if f.isExcludedPod(pod) {
		return false
	}

	// 2. Namespace 排除
	if f.isNamespaceExcluded(pod.Namespace) {
		return false
	}

	// 3. Namespace 包含（空列表=不限制）
	if !f.isNamespaceIncluded(pod.Namespace) {
		return false
	}

	return true
}

// ShouldProcessNamespace 判断是否应处理该 Namespace（用于 Event 过滤）
func (f *Filter) ShouldProcessNamespace(namespace string) bool {
	if f.isNamespaceExcluded(namespace) {
		return false
	}
	return f.isNamespaceIncluded(namespace)
}

func (f *Filter) isExcludedPod(pod *corev1.Pod) bool {
	for _, rule := range f.cfg.ExcludePods {
		if rule.Namespace != "" && rule.Namespace != pod.Namespace {
			continue
		}
		if rule.Name != "" {
			matched, _ := filepath.Match(rule.Name, pod.Name)
			if matched {
				return true
			}
		}
	}
	return false
}

func (f *Filter) isNamespaceExcluded(namespace string) bool {
	for _, ns := range f.cfg.Namespaces.Exclude {
		if ns == namespace {
			return true
		}
	}
	return false
}

func (f *Filter) isNamespaceIncluded(namespace string) bool {
	if len(f.cfg.Namespaces.Include) == 0 {
		return true
	}
	for _, ns := range f.cfg.Namespaces.Include {
		if ns == namespace {
			return true
		}
	}
	return false
}
