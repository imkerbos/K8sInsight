package sink

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"time"

	"github.com/kerbos/k8sinsight/internal/detector"
)

// Telegram Telegram 机器人通知
type Telegram struct {
	name      string
	botToken  string
	chatID    string
	parseMode string
	client    *http.Client
}

// NewTelegram 创建 Telegram 通知
func NewTelegram(name, botToken, chatID, parseMode string) *Telegram {
	if parseMode == "" {
		parseMode = "HTML"
	}
	return &Telegram{
		name:      name,
		botToken:  botToken,
		chatID:    chatID,
		parseMode: parseMode,
		client:    &http.Client{Timeout: 10 * time.Second},
	}
}

func (t *Telegram) Name() string { return t.name }

func (t *Telegram) Send(ctx context.Context, event detector.AnomalyEvent) error {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.botToken)
	rc := inferRootCause(event)
	summaryMsg, evidenceMsg := splitEvidenceMessage(event.Message)

	text := fmt.Sprintf(
		"<b>K8sInsight 异常事件通知</b>\n\n"+
			"类型: <code>%s</code>\n"+
			"集群: <code>%s</code>\n"+
			"命名空间: <code>%s</code>\n"+
			"Pod: <code>%s</code>\n"+
			"容器: <code>%s</code>\n"+
			"原因: <code>%s</code>\n"+
			"摘要: %s\n\n"+
			"%s"+
			"<b>结论:</b> %s\n"+
			"<b>建议:</b> %s\n\n"+
			"<i>%s</i>",
		html.EscapeString(string(event.Type)),
		html.EscapeString(safe(event.ClusterID)),
		html.EscapeString(event.Namespace),
		html.EscapeString(event.PodName),
		html.EscapeString(safe(event.ContainerName)),
		html.EscapeString(safe(event.Reason)),
		html.EscapeString(summaryMsg),
		telegramEvidenceBlock(evidenceMsg),
		html.EscapeString(rc.Summary),
		html.EscapeString(firstSuggestion(rc.Suggestions)),
		html.EscapeString(event.Timestamp.Format(time.RFC3339)),
	)

	reqBody := map[string]any{
		"chat_id":    t.chatID,
		"text":       text,
		"parse_mode": t.parseMode,
		"link_preview_options": map[string]any{
			"is_disabled": true,
		},
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("序列化 Telegram 消息失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("创建 Telegram 请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("发送 Telegram 通知失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	var result struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(respBody, &result); err == nil && !result.OK {
		return fmt.Errorf("Telegram API 错误: %s", result.Description)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Telegram 返回 HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

func telegramEvidenceBlock(evidence string) string {
	if evidence == "" {
		return ""
	}
	return "<b>证据支撑:</b>\n<pre>" + html.EscapeString(evidence) + "</pre>\n"
}
