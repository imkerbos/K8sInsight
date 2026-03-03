package sink

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/kerbos/k8sinsight/internal/detector"
)

// Lark 飞书机器人通知（interactive card）
type Lark struct {
	name   string
	url    string
	secret string
	client *http.Client
}

// NewLark 创建飞书通知
func NewLark(name, url, secret string) *Lark {
	return &Lark{
		name:   name,
		url:    url,
		secret: secret,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (l *Lark) Name() string { return l.name }

func (l *Lark) Send(ctx context.Context, event detector.AnomalyEvent) error {
	rc := inferRootCause(event)
	summaryMsg, evidenceMsg := splitEvidenceMessage(event.Message)

	elements := []any{
		// 摘要行：快速定位问题资源
		map[string]any{
			"tag": "div",
			"text": map[string]any{
				"tag":     "plain_text",
				"content": fmt.Sprintf("📍 %s / %s / %s", event.Namespace, event.PodName, safe(event.ContainerName)),
			},
		},
		// 分割线
		map[string]any{"tag": "hr"},
		// 资源信息：双栏布局
		map[string]any{
			"tag": "div",
			"fields": []any{
				larkField("集群", safe(event.ClusterID)),
				larkField("命名空间", event.Namespace),
				larkField("Pod", event.PodName),
				larkField("容器", safe(event.ContainerName)),
				larkField("节点", safe(event.NodeName)),
				larkField("Owner", ownerInfo(event.OwnerKind, event.OwnerName)),
			},
		},
		// 分割线
		map[string]any{"tag": "hr"},
	}

	// 异常详情
	detail := fmt.Sprintf("异常类型：%s\n原因：%s", event.Type, safe(event.Reason))
	if event.ExitCode != 0 {
		detail += fmt.Sprintf("\n退出码：%d", event.ExitCode)
	}
	if event.RestartCount > 0 {
		detail += fmt.Sprintf("\n重启次数：%d", event.RestartCount)
	}
	detail += fmt.Sprintf("\n摘要：%s", summaryMsg)

	elements = append(elements, map[string]any{
		"tag": "div",
		"text": map[string]any{
			"tag":     "plain_text",
			"content": detail,
		},
	})

	if evidenceMsg != "" {
		elements = append(elements,
			map[string]any{"tag": "hr"},
			map[string]any{
				"tag": "div",
				"text": map[string]any{
					"tag":     "plain_text",
					"content": "证据支撑：\n" + evidenceMsg,
				},
			},
		)
	}
	elements = append(elements,
		map[string]any{"tag": "hr"},
		map[string]any{
			"tag": "div",
			"text": map[string]any{
				"tag":     "plain_text",
				"content": "结论：" + rc.Summary,
			},
		},
		map[string]any{
			"tag": "div",
			"text": map[string]any{
				"tag":     "plain_text",
				"content": "建议：" + firstSuggestion(rc.Suggestions),
			},
		},
	)

	// 底部时间戳
	elements = append(elements, map[string]any{
		"tag": "note",
		"elements": []any{
			map[string]any{
				"tag":     "plain_text",
				"content": "🕐 " + event.Timestamp.Format("2006-01-02 15:04:05 MST"),
			},
		},
	})

	card := map[string]any{
		"config": map[string]any{
			"wide_screen_mode": true,
		},
		"header": map[string]any{
			"title": map[string]any{
				"tag":     "plain_text",
				"content": fmt.Sprintf("%s %s", anomalyIcon(event.Type), event.Type),
			},
			"template": l.templateByType(event.Type),
		},
		"elements": elements,
	}

	body := map[string]any{
		"msg_type": "interactive",
		"card":     card,
	}

	if l.secret != "" {
		ts := time.Now().Unix()
		sign, err := l.sign(ts)
		if err != nil {
			return fmt.Errorf("生成飞书签名失败: %w", err)
		}
		body["timestamp"] = fmt.Sprintf("%d", ts)
		body["sign"] = sign
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("序列化飞书通知失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, l.url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("创建飞书请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := l.client.Do(req)
	if err != nil {
		return fmt.Errorf("发送飞书通知失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(respBody, &result); err == nil && result.Code != 0 {
		return fmt.Errorf("飞书返回错误: code=%d, msg=%s", result.Code, result.Msg)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("飞书返回 HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

func (l *Lark) sign(ts int64) (string, error) {
	stringToSign := fmt.Sprintf("%d\n%s", ts, l.secret)
	h := hmac.New(sha256.New, []byte(stringToSign))
	if _, err := h.Write([]byte{}); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(h.Sum(nil)), nil
}

func (l *Lark) templateByType(t detector.AnomalyType) string {
	switch t {
	case detector.AnomalyOOMKilled, detector.AnomalyCrashLoopBackOff, detector.AnomalyErrorExit:
		return "red"
	case detector.AnomalyEvicted, detector.AnomalyFailedScheduling:
		return "orange"
	default:
		return "blue"
	}
}

func safe(v string) string {
	if v == "" {
		return "-"
	}
	return v
}

// larkField 创建飞书卡片双栏字段
func larkField(label, value string) map[string]any {
	return map[string]any{
		"is_short": true,
		"text": map[string]any{
			"tag":     "plain_text",
			"content": fmt.Sprintf("%s\n%s", label, value),
		},
	}
}

// ownerInfo 格式化 Owner 信息
func ownerInfo(kind, name string) string {
	if kind == "" && name == "" {
		return "-"
	}
	if kind == "" {
		return name
	}
	return kind + "/" + name
}

// anomalyIcon 根据异常类型返回对应图标
func anomalyIcon(t detector.AnomalyType) string {
	switch t {
	case detector.AnomalyOOMKilled:
		return "💥"
	case detector.AnomalyCrashLoopBackOff:
		return "🔄"
	case detector.AnomalyErrorExit:
		return "❌"
	case detector.AnomalyRestartIncrement:
		return "🔁"
	case detector.AnomalyImagePullBackOff:
		return "📦"
	case detector.AnomalyCreateContainerConfigError:
		return "⚙️"
	case detector.AnomalyFailedScheduling:
		return "📋"
	case detector.AnomalyEvicted:
		return "🚫"
	case detector.AnomalyStateOscillation:
		return "📈"
	default:
		return "⚠️"
	}
}
