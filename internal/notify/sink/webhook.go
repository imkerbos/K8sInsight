package sink

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/kerbos/k8sinsight/internal/detector"
)

// Webhook 通用 Webhook 通知
type Webhook struct {
	name    string
	url     string
	headers map[string]string
	client  *http.Client
}

// NewWebhook 创建 Webhook 通知
func NewWebhook(name, url string, headers map[string]string) *Webhook {
	return &Webhook{
		name:    name,
		url:     url,
		headers: headers,
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

func (w *Webhook) Name() string { return w.name }

func (w *Webhook) Send(ctx context.Context, event detector.AnomalyEvent) error {
	rc := inferRootCause(event)
	summary, evidence := splitEvidenceMessage(event.Message)

	payload := map[string]interface{}{
		"type":       event.Type,
		"pod":        event.PodName,
		"namespace":  event.Namespace,
		"message":    summary,
		"evidence":   evidence,
		"timestamp":  event.Timestamp,
		"dedupKey":   event.DedupKey(),
		"rootCause":  rc.Summary,
		"suggestion": firstSuggestion(rc.Suggestions),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("序列化通知内容失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	for k, v := range w.headers {
		req.Header.Set(k, v)
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("发送 Webhook 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("Webhook 返回错误状态码: %d", resp.StatusCode)
	}

	return nil
}
