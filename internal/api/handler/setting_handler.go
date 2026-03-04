package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/kerbos/k8sinsight/internal/detector"
	"github.com/kerbos/k8sinsight/internal/notify/sink"
	"github.com/kerbos/k8sinsight/internal/store/model"
	"github.com/kerbos/k8sinsight/internal/store/repository"
)

// 产品设置支持的 key 列表
var brandingKeys = []string{"site_title", "site_logo", "site_slogan"}

// 安全设置支持的 key 列表及默认值
var securityKeys = map[string]string{
	"security_min_password_length": "6",
	"security_access_token_ttl":    "2h",
	"security_refresh_token_ttl":   "168h",
}

// SSO 设置支持的 key 列表及默认值
var ssoKeys = map[string]string{
	"sso_enabled":          "false",
	"sso_provider_name":    "",
	"sso_client_id":        "",
	"sso_client_secret":    "",
	"sso_issuer_url":       "",
	"sso_redirect_uri":     "",
	"sso_scopes":           "openid,profile,email",
	"sso_auto_create_user": "true",
	"sso_default_role":     "viewer",
}

// 通知设置支持的 key 列表及默认值
var notifyKeys = map[string]string{
	"notify_enabled":   "false",
	"notify_channel":   "webhook",
	"notify_webhooks":  "[]",
	"notify_larks":     "[]",
	"notify_telegrams": "[]",
}

// 资源采集设置支持的 key 列表及默认值
var collectKeys = map[string]string{
	"collect_enable_metrics":   "true",
	"collect_prometheus_url":   "",
	"collect_prom_query_range": "10m",
}

// SettingHandler 系统设置 API 处理器
type SettingHandler struct {
	settingRepo  repository.SettingRepository
	incidentRepo repository.IncidentRepository
	evidenceRepo repository.EvidenceRepository
	logger       *zap.Logger
}

type NotifyTestRequest struct {
	Channel    string            `json:"channel" binding:"required,oneof=webhook lark telegram"`
	IncidentID string            `json:"incidentId"`
	Name       string            `json:"name"`
	URL        string            `json:"url"`
	Headers    map[string]string `json:"headers"`
	Secret     string            `json:"secret"`
	BotToken   string            `json:"botToken"`
	ChatID     string            `json:"chatId"`
	ParseMode  string            `json:"parseMode"`
}

type NotifyWebhookSetting struct {
	Name    string            `json:"name"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
}

type NotifyLarkSetting struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	Secret string `json:"secret"`
}

type NotifyTelegramSetting struct {
	Name      string `json:"name"`
	BotToken  string `json:"botToken"`
	ChatID    string `json:"chatId"`
	ParseMode string `json:"parseMode"`
}

type NotifySettings struct {
	Enabled   bool                    `json:"enabled"`
	Channel   string                  `json:"channel"`
	Webhooks  []NotifyWebhookSetting  `json:"webhooks"`
	Larks     []NotifyLarkSetting     `json:"larks"`
	Telegrams []NotifyTelegramSetting `json:"telegrams"`
}

type CollectSettings struct {
	EnableMetrics  bool   `json:"enableMetrics"`
	PrometheusURL  string `json:"prometheusURL"`
	PromQueryRange string `json:"promQueryRange"`
}

type CollectTestRequest struct {
	PrometheusURL string `json:"prometheusURL" binding:"required"`
}

type promInstantResponse struct {
	Status string `json:"status"`
	Data   struct {
		Result []json.RawMessage `json:"result"`
	} `json:"data"`
	Error string `json:"error"`
}

func NewSettingHandler(
	settingRepo repository.SettingRepository,
	incidentRepo repository.IncidentRepository,
	evidenceRepo repository.EvidenceRepository,
	logger *zap.Logger,
) *SettingHandler {
	return &SettingHandler{
		settingRepo:  settingRepo,
		incidentRepo: incidentRepo,
		evidenceRepo: evidenceRepo,
		logger:       logger.Named("api.setting"),
	}
}

// GetBranding 获取产品设置
func (h *SettingHandler) GetBranding(c *gin.Context) {
	settings, err := h.settingRepo.BatchGet(c.Request.Context(), brandingKeys)
	if err != nil {
		h.logger.Error("获取产品设置失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	c.JSON(http.StatusOK, settings)
}

// UpdateBranding 更新产品设置
func (h *SettingHandler) UpdateBranding(c *gin.Context) {
	var req map[string]string
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	allowed := make(map[string]bool, len(brandingKeys))
	for _, k := range brandingKeys {
		allowed[k] = true
	}
	filtered := make(map[string]string)
	for k, v := range req {
		if allowed[k] {
			filtered[k] = v
		}
	}
	if len(filtered) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无有效设置项"})
		return
	}

	if err := h.settingRepo.BatchSet(c.Request.Context(), filtered); err != nil {
		h.logger.Error("更新产品设置失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "保存成功"})
}

// GetSecurity 获取安全设置
func (h *SettingHandler) GetSecurity(c *gin.Context) {
	keys := make([]string, 0, len(securityKeys))
	for k := range securityKeys {
		keys = append(keys, k)
	}

	settings, err := h.settingRepo.BatchGet(c.Request.Context(), keys)
	if err != nil {
		h.logger.Error("获取安全设置失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	for k, def := range securityKeys {
		if settings[k] == "" {
			settings[k] = def
		}
	}
	c.JSON(http.StatusOK, settings)
}

// UpdateSecurity 更新安全设置
func (h *SettingHandler) UpdateSecurity(c *gin.Context) {
	var req map[string]string
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	filtered := make(map[string]string)
	for k, v := range req {
		if _, ok := securityKeys[k]; ok {
			filtered[k] = v
		}
	}
	if len(filtered) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无有效设置项"})
		return
	}

	if v, ok := filtered["security_min_password_length"]; ok {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "密码最小长度必须为正整数"})
			return
		}
	}

	if err := h.settingRepo.BatchSet(c.Request.Context(), filtered); err != nil {
		h.logger.Error("更新安全设置失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "保存成功"})
}

// GetSSO 获取 SSO 设置
func (h *SettingHandler) GetSSO(c *gin.Context) {
	keys := make([]string, 0, len(ssoKeys))
	for k := range ssoKeys {
		keys = append(keys, k)
	}

	settings, err := h.settingRepo.BatchGet(c.Request.Context(), keys)
	if err != nil {
		h.logger.Error("获取 SSO 设置失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	for k, def := range ssoKeys {
		if settings[k] == "" {
			settings[k] = def
		}
	}

	// 掩码 client_secret，不泄露明文
	if settings["sso_client_secret"] != "" {
		settings["sso_client_secret"] = "***"
	}

	c.JSON(http.StatusOK, settings)
}

// UpdateSSO 更新 SSO 设置
func (h *SettingHandler) UpdateSSO(c *gin.Context) {
	var req map[string]string
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	filtered := make(map[string]string)
	for k, v := range req {
		if _, ok := ssoKeys[k]; ok {
			filtered[k] = v
		}
	}
	if len(filtered) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无有效设置项"})
		return
	}

	// 如果 client_secret 为掩码值，不更新该字段
	if filtered["sso_client_secret"] == "***" {
		delete(filtered, "sso_client_secret")
	}

	if err := h.settingRepo.BatchSet(c.Request.Context(), filtered); err != nil {
		h.logger.Error("更新 SSO 设置失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "保存成功"})
}

// GetCollect 获取资源采集设置
func (h *SettingHandler) GetCollect(c *gin.Context) {
	keys := make([]string, 0, len(collectKeys))
	for k := range collectKeys {
		keys = append(keys, k)
	}

	settings, err := h.settingRepo.BatchGet(c.Request.Context(), keys)
	if err != nil {
		h.logger.Error("获取资源采集设置失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	for k, def := range collectKeys {
		if settings[k] == "" {
			settings[k] = def
		}
	}

	enableMetrics := settings["collect_enable_metrics"] == "true"
	promURL := strings.TrimSpace(settings["collect_prometheus_url"])
	promRange := strings.TrimSpace(settings["collect_prom_query_range"])
	if promRange == "" {
		promRange = collectKeys["collect_prom_query_range"]
	}

	c.JSON(http.StatusOK, CollectSettings{
		EnableMetrics:  enableMetrics,
		PrometheusURL:  promURL,
		PromQueryRange: promRange,
	})
}

// UpdateCollect 更新资源采集设置
func (h *SettingHandler) UpdateCollect(c *gin.Context) {
	var req CollectSettings
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	req.PrometheusURL = strings.TrimSpace(req.PrometheusURL)
	req.PromQueryRange = strings.TrimSpace(req.PromQueryRange)
	if req.PromQueryRange == "" {
		req.PromQueryRange = collectKeys["collect_prom_query_range"]
	}
	if _, err := time.ParseDuration(req.PromQueryRange); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "promQueryRange 格式无效（示例: 30s, 5m, 1h）"})
		return
	}

	payload := map[string]string{
		"collect_enable_metrics":   strconv.FormatBool(req.EnableMetrics),
		"collect_prometheus_url":   req.PrometheusURL,
		"collect_prom_query_range": req.PromQueryRange,
	}

	if err := h.settingRepo.BatchSet(c.Request.Context(), payload); err != nil {
		h.logger.Error("更新资源采集设置失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "保存成功"})
}

// TestCollectConnection 测试 Prometheus 连接
func (h *SettingHandler) TestCollectConnection(c *gin.Context) {
	var req CollectTestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	promURL := strings.TrimSpace(req.PrometheusURL)
	if promURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Prometheus 地址不能为空"})
		return
	}
	if _, err := url.ParseRequestURI(promURL); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Prometheus 地址格式无效"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 8*time.Second)
	defer cancel()

	resultCount, err := testPrometheusConnection(ctx, promURL)
	if err != nil {
		h.logger.Warn("Prometheus 连接测试失败", zap.String("url", promURL), zap.Error(err))
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     "Prometheus 连接成功",
		"resultCount": resultCount,
	})
}

func testPrometheusConnection(ctx context.Context, baseURL string) (int, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return 0, fmt.Errorf("Prometheus 地址格式无效")
	}

	path := strings.TrimRight(u.Path, "/")
	u.Path = path + "/api/v1/query"
	q := u.Query()
	q.Set("query", "up")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return 0, fmt.Errorf("构造测试请求失败: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("连接 Prometheus 失败: %w", err)
	}
	defer resp.Body.Close()

	var payload promInstantResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return 0, fmt.Errorf("Prometheus 响应解析失败: %w", err)
	}

	if resp.StatusCode >= http.StatusBadRequest || payload.Status != "success" {
		errText := payload.Error
		if errText == "" {
			errText = resp.Status
		}
		return 0, fmt.Errorf("Prometheus 查询失败: %s", errText)
	}

	return len(payload.Data.Result), nil
}

// GetNotify 获取通知设置
func (h *SettingHandler) GetNotify(c *gin.Context) {
	keys := make([]string, 0, len(notifyKeys))
	for k := range notifyKeys {
		keys = append(keys, k)
	}
	settings, err := h.settingRepo.BatchGet(c.Request.Context(), keys)
	if err != nil {
		h.logger.Error("获取通知设置失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	for k, def := range notifyKeys {
		if settings[k] == "" {
			settings[k] = def
		}
	}

	resp := NotifySettings{Enabled: settings["notify_enabled"] == "true", Channel: settings["notify_channel"]}
	if resp.Channel == "" {
		resp.Channel = "webhook"
	}
	_ = json.Unmarshal([]byte(settings["notify_webhooks"]), &resp.Webhooks)
	_ = json.Unmarshal([]byte(settings["notify_larks"]), &resp.Larks)
	_ = json.Unmarshal([]byte(settings["notify_telegrams"]), &resp.Telegrams)
	if resp.Webhooks == nil {
		resp.Webhooks = []NotifyWebhookSetting{}
	}
	if resp.Larks == nil {
		resp.Larks = []NotifyLarkSetting{}
	}
	if resp.Telegrams == nil {
		resp.Telegrams = []NotifyTelegramSetting{}
	}
	c.JSON(http.StatusOK, resp)
}

// UpdateNotify 更新通知设置
func (h *SettingHandler) UpdateNotify(c *gin.Context) {
	var req NotifySettings
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}
	if req.Channel == "" {
		req.Channel = "webhook"
	}
	if req.Channel != "webhook" && req.Channel != "lark" && req.Channel != "telegram" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "通知通道必须是 webhook/lark/telegram"})
		return
	}

	switch req.Channel {
	case "webhook":
		if req.Enabled && len(req.Webhooks) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "已启用通知时，Webhook 配置不能为空"})
			return
		}
		for _, wh := range req.Webhooks {
			if wh.URL == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Webhook URL 不能为空"})
				return
			}
		}
		req.Larks = []NotifyLarkSetting{}
		req.Telegrams = []NotifyTelegramSetting{}
	case "lark":
		if req.Enabled && len(req.Larks) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "已启用通知时，Lark 配置不能为空"})
			return
		}
		for _, l := range req.Larks {
			if l.URL == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Lark URL 不能为空"})
				return
			}
		}
		req.Webhooks = []NotifyWebhookSetting{}
		req.Telegrams = []NotifyTelegramSetting{}
	case "telegram":
		if req.Enabled && len(req.Telegrams) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "已启用通知时，Telegram 配置不能为空"})
			return
		}
		for _, t := range req.Telegrams {
			if t.BotToken == "" || t.ChatID == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Telegram BotToken/ChatID 不能为空"})
				return
			}
		}
		req.Webhooks = []NotifyWebhookSetting{}
		req.Larks = []NotifyLarkSetting{}
	}

	webhooksJSON, _ := json.Marshal(req.Webhooks)
	larksJSON, _ := json.Marshal(req.Larks)
	telegramsJSON, _ := json.Marshal(req.Telegrams)

	payload := map[string]string{
		"notify_enabled":   strconv.FormatBool(req.Enabled),
		"notify_channel":   req.Channel,
		"notify_webhooks":  string(webhooksJSON),
		"notify_larks":     string(larksJSON),
		"notify_telegrams": string(telegramsJSON),
	}
	if err := h.settingRepo.BatchSet(c.Request.Context(), payload); err != nil {
		h.logger.Error("更新通知设置失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "保存成功"})
}

// TestNotify 测试通知发送（Webhook/Lark/Telegram）
func (h *SettingHandler) TestNotify(c *gin.Context) {
	var req NotifyTestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	testEvent := detector.AnomalyEvent{
		ID:            "test-notify",
		Timestamp:     time.Now(),
		ClusterID:     "test-cluster",
		Type:          detector.AnomalyOOMKilled,
		Source:        detector.SourcePodState,
		Message:       "这是一条测试通知，请忽略。",
		PodUID:        "test-pod-uid",
		PodName:       "test-pod",
		Namespace:     "default",
		ContainerName: "app",
		Reason:        "OOMKilled",
		RestartCount:  1,
	}
	if req.IncidentID != "" {
		ev, err := h.buildEventFromIncident(c.Request.Context(), req.IncidentID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		testEvent = ev
	}

	h.fillNotifyCredentialFromSettings(c.Request.Context(), &req)

	switch req.Channel {
	case "webhook":
		if req.URL == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Webhook URL 不能为空"})
			return
		}
		w := sink.NewWebhook(req.NameOrDefault("test-webhook"), req.URL, req.Headers)
		if err := w.Send(c.Request.Context(), testEvent); err != nil {
			h.logger.Error("Webhook 测试通知失败", zap.Error(err))
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}
	case "lark":
		if req.URL == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Lark Webhook URL 不能为空"})
			return
		}
		l := sink.NewLark(req.NameOrDefault("test-lark"), req.URL, req.Secret)
		if err := l.Send(c.Request.Context(), testEvent); err != nil {
			h.logger.Error("Lark 测试通知失败", zap.Error(err))
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}
	case "telegram":
		if req.BotToken == "" || req.ChatID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Telegram BotToken/ChatID 不能为空"})
			return
		}
		t := sink.NewTelegram(req.NameOrDefault("test-telegram"), req.BotToken, req.ChatID, req.ParseMode)
		if err := t.Send(c.Request.Context(), testEvent); err != nil {
			h.logger.Error("Telegram 测试通知失败", zap.Error(err))
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "不支持的通知通道"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "测试通知发送成功"})
}

func (r NotifyTestRequest) NameOrDefault(def string) string {
	if r.Name != "" {
		return r.Name
	}
	return def
}

func (h *SettingHandler) fillNotifyCredentialFromSettings(ctx context.Context, req *NotifyTestRequest) {
	settings, err := h.settingRepo.BatchGet(ctx, []string{"notify_webhooks", "notify_larks", "notify_telegrams"})
	if err != nil {
		return
	}
	switch req.Channel {
	case "webhook":
		if req.URL != "" {
			return
		}
		var list []NotifyWebhookSetting
		if err := json.Unmarshal([]byte(settings["notify_webhooks"]), &list); err == nil && len(list) > 0 {
			req.Name = req.NameOrDefault(list[0].Name)
			req.URL = list[0].URL
			if req.Headers == nil {
				req.Headers = list[0].Headers
			}
		}
	case "lark":
		if req.URL != "" {
			return
		}
		var list []NotifyLarkSetting
		if err := json.Unmarshal([]byte(settings["notify_larks"]), &list); err == nil && len(list) > 0 {
			req.Name = req.NameOrDefault(list[0].Name)
			req.URL = list[0].URL
			if req.Secret == "" {
				req.Secret = list[0].Secret
			}
		}
	case "telegram":
		if req.BotToken != "" && req.ChatID != "" {
			return
		}
		var list []NotifyTelegramSetting
		if err := json.Unmarshal([]byte(settings["notify_telegrams"]), &list); err == nil && len(list) > 0 {
			req.Name = req.NameOrDefault(list[0].Name)
			if req.BotToken == "" {
				req.BotToken = list[0].BotToken
			}
			if req.ChatID == "" {
				req.ChatID = list[0].ChatID
			}
			if req.ParseMode == "" {
				req.ParseMode = list[0].ParseMode
			}
		}
	}
}

func (h *SettingHandler) buildEventFromIncident(ctx context.Context, incidentID string) (detector.AnomalyEvent, error) {
	if h.incidentRepo == nil || h.evidenceRepo == nil {
		return detector.AnomalyEvent{}, fmt.Errorf("事件证据仓储未初始化")
	}
	inc, err := h.incidentRepo.FindByID(ctx, incidentID)
	if err != nil {
		return detector.AnomalyEvent{}, fmt.Errorf("查询事件失败: %w", err)
	}
	evidences, err := h.evidenceRepo.FindByIncidentID(ctx, incidentID)
	if err != nil {
		return detector.AnomalyEvent{}, fmt.Errorf("查询证据失败: %w", err)
	}

	podName := ""
	var pods []string
	_ = json.Unmarshal([]byte(inc.PodNames), &pods)
	if len(pods) > 0 {
		podName = pods[0]
	}

	lines := []string{inc.Message, "", "[证据分析]"}
	lines = append(lines, summarizeEvidence(evidences)...)

	event := detector.AnomalyEvent{
		ID:        "incident-" + inc.ID,
		Timestamp: time.Now(),
		Type:      detector.AnomalyType(inc.AnomalyType),
		Message:   strings.Join(lines, "\n"),
		PodName:   podName,
		Namespace: inc.Namespace,
		OwnerKind: inc.OwnerKind,
		OwnerName: inc.OwnerName,
	}
	if inc.ClusterID != nil {
		event.ClusterID = *inc.ClusterID
	}
	return event, nil
}

func summarizeEvidence(evidences []model.Evidence) []string {
	lines := []string{}
	for _, e := range evidences {
		switch e.Type {
		case "PodSnapshot":
			var pod map[string]any
			if json.Unmarshal([]byte(e.Content), &pod) != nil {
				continue
			}
			if cs, ok := pod["containers"].([]any); ok && len(cs) > 0 {
				if c0, ok := cs[0].(map[string]any); ok {
					restart := int64FromAny(c0["restartCount"])
					lastState := strFromAny(c0["lastState"])
					exitCode := int64FromAny(c0["exitCode"])
					lines = append(lines, fmt.Sprintf("container=%s restartCount=%d lastState=%s exitCode=%d", strFromAny(c0["name"]), restart, safeText(lastState), exitCode))
					if res, ok := c0["resources"].(map[string]any); ok {
						lines = append(lines, fmt.Sprintf("memory request/limit=%s/%s", safeText(strFromAny(res["requestsMemory"])), safeText(strFromAny(res["limitsMemory"]))))
					}
				}
			}
		case "PodEvents":
			var items []map[string]any
			if json.Unmarshal([]byte(e.Content), &items) != nil || len(items) == 0 {
				continue
			}
			last := items[len(items)-1]
			lines = append(lines, "latestWarningEvent="+strings.TrimSpace(strFromAny(last["reason"])+" "+strFromAny(last["message"])))
		case "Metrics":
			peak, latest := parseMemoryFromMetrics(e.Content)
			if peak > 0 || latest > 0 {
				lines = append(lines, fmt.Sprintf("memory(oom前窗口) peak=%s latest=%s", humanMi(peak), humanMi(latest)))
			}
		}
	}
	if len(lines) == 0 {
		lines = append(lines, "证据不足：当前事件无有效 PodSnapshot/Metrics/PodEvents 数据")
	}
	return lines
}

func parseMemoryFromMetrics(content string) (peak float64, latest float64) {
	type series struct {
		Values [][]any `json:"values"`
	}
	var payload struct {
		Source string `json:"source"`
		Series struct {
			Memory []series `json:"memory"`
		} `json:"series"`
	}
	if json.Unmarshal([]byte(content), &payload) != nil || payload.Source != "prometheus" {
		return
	}
	for _, s := range payload.Series.Memory {
		for _, p := range s.Values {
			if len(p) < 2 {
				continue
			}
			val, _ := strconv.ParseFloat(strFromAny(p[1]), 64)
			if val > peak {
				peak = val
			}
			latest = val
		}
	}
	return
}

func int64FromAny(v any) int64 {
	switch x := v.(type) {
	case float64:
		return int64(x)
	case int64:
		return x
	case int:
		return int64(x)
	case string:
		n, _ := strconv.ParseInt(x, 10, 64)
		return n
	default:
		return 0
	}
}

func strFromAny(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	b, _ := json.Marshal(v)
	return strings.Trim(string(b), "\"")
}

func humanMi(v float64) string {
	if v <= 0 {
		return "-"
	}
	return fmt.Sprintf("%.1fMi", v/1024.0/1024.0)
}

func safeText(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// GetMinPasswordLength 从设置中获取密码最小长度（供其他 handler 调用）
func GetMinPasswordLength(settingRepo repository.SettingRepository, c *gin.Context) int {
	val, err := settingRepo.Get(c.Request.Context(), "security_min_password_length")
	if err != nil || val == "" {
		return 6
	}
	n, err := strconv.Atoi(val)
	if err != nil || n < 1 {
		return 6
	}
	return n
}
