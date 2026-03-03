package aggregator

import (
	"sync"
	"time"
)

// IncidentState 事件状态
type IncidentState string

const (
	StateDetecting IncidentState = "Detecting"
	StateActive    IncidentState = "Active"
	StateResolved  IncidentState = "Resolved"
	StateArchived  IncidentState = "Archived"
)

// Incident 聚合后的事件实体（一次完整异常的生命周期）
type Incident struct {
	ID         string        `json:"id"`
	DedupKey   string        `json:"dedupKey"`
	State      IncidentState `json:"state"`
	FirstSeen  time.Time     `json:"firstSeen"`
	LastSeen   time.Time     `json:"lastSeen"`
	Count      int           `json:"count"`       // 聚合的异常事件次数
	Namespace  string        `json:"namespace"`
	OwnerKind  string        `json:"ownerKind"`
	OwnerName  string        `json:"ownerName"`
	AnomalyType string      `json:"anomalyType"`
	Message    string        `json:"message"`     // 最新的消息
	PodNames   []string      `json:"podNames"`    // 涉及的 Pod 列表
	ClusterID  string        `json:"clusterId,omitempty"`
}

// dedupIndex 内存中的活跃事件索引
type dedupIndex struct {
	mu        sync.RWMutex
	incidents map[string]*Incident // key: dedupKey
}

func newDedupIndex() *dedupIndex {
	return &dedupIndex{
		incidents: make(map[string]*Incident),
	}
}

// findActive 查找活跃事件
func (idx *dedupIndex) findActive(dedupKey string) (*Incident, bool) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	inc, ok := idx.incidents[dedupKey]
	if !ok {
		return nil, false
	}
	if inc.State == StateResolved || inc.State == StateArchived {
		return nil, false
	}
	return inc, true
}

// upsert 插入或更新事件
func (idx *dedupIndex) upsert(inc *Incident) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.incidents[inc.DedupKey] = inc
}

// remove 移除事件（Resolved/Archived 后）
func (idx *dedupIndex) remove(dedupKey string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	delete(idx.incidents, dedupKey)
}

// allActive 返回所有活跃事件
func (idx *dedupIndex) allActive() []*Incident {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	var result []*Incident
	for _, inc := range idx.incidents {
		if inc.State == StateDetecting || inc.State == StateActive {
			result = append(result, inc)
		}
	}
	return result
}
