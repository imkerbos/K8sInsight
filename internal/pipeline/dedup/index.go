package dedup

import (
	"sync"
	"time"

	"github.com/kerbos/k8sinsight/internal/domain"
)

// Index 内存中的活跃事件去重索引
type Index struct {
	mu        sync.RWMutex
	incidents map[string]*domain.Incident
	maxSize   int
}

// NewIndex 创建去重索引
func NewIndex(maxSize int) *Index {
	if maxSize <= 0 {
		maxSize = 10000
	}
	return &Index{
		incidents: make(map[string]*domain.Incident),
		maxSize:   maxSize,
	}
}

// FindActive 查找活跃事件
func (idx *Index) FindActive(dedupKey string) (*domain.Incident, bool) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	inc, ok := idx.incidents[dedupKey]
	if !ok {
		return nil, false
	}
	if inc.State == domain.StateResolved || inc.State == domain.StateArchived {
		return nil, false
	}
	return inc, true
}

// Upsert 插入或更新事件
func (idx *Index) Upsert(inc *domain.Incident) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.incidents[inc.DedupKey] = inc

	// 超出 maxSize 时驱逐最旧的 Resolved 条目
	if len(idx.incidents) > idx.maxSize {
		idx.evictOldestResolved()
	}
}

// Remove 移除事件
func (idx *Index) Remove(dedupKey string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	delete(idx.incidents, dedupKey)
}

// AllActive 返回所有活跃事件
func (idx *Index) AllActive() []*domain.Incident {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	var result []*domain.Incident
	for _, inc := range idx.incidents {
		if inc.State == domain.StateDetecting || inc.State == domain.StateActive {
			result = append(result, inc)
		}
	}
	return result
}

// Sweep 清理过期条目，返回需要 resolve 的事件列表
func (idx *Index) Sweep(activeWindow time.Duration) []*domain.Incident {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	var toResolve []*domain.Incident
	for key, inc := range idx.incidents {
		switch inc.State {
		case domain.StateDetecting, domain.StateActive:
			if time.Since(inc.LastSeen) > activeWindow {
				toResolve = append(toResolve, inc)
			}
		case domain.StateResolved, domain.StateArchived:
			// 已解决的条目超过 2 倍活跃窗口后自动清理
			if time.Since(inc.LastSeen) > 2*activeWindow {
				delete(idx.incidents, key)
			}
		}
	}
	return toResolve
}

// Len 返回索引中的条目数
func (idx *Index) Len() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return len(idx.incidents)
}

// evictOldestResolved 驱逐最旧的 Resolved 条目（调用方需持锁）
func (idx *Index) evictOldestResolved() {
	var oldestKey string
	var oldestTime time.Time

	for key, inc := range idx.incidents {
		if inc.State != domain.StateResolved && inc.State != domain.StateArchived {
			continue
		}
		if oldestKey == "" || inc.LastSeen.Before(oldestTime) {
			oldestKey = key
			oldestTime = inc.LastSeen
		}
	}

	if oldestKey != "" {
		delete(idx.incidents, oldestKey)
	}
}
