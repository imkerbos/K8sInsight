package config

import (
	"fmt"
	"time"
)

// Config 应用顶层配置
type Config struct {
	Server  ServerConfig  `mapstructure:"server"`
	Watch   WatchConfig   `mapstructure:"watch"`
	DB      DBConfig      `mapstructure:"db"`
	Log     LogConfig     `mapstructure:"log"`
	Collect CollectConfig `mapstructure:"collect"`
	Notify  NotifyConfig  `mapstructure:"notify"`
}

// ServerConfig HTTP 服务配置
type ServerConfig struct {
	Port                 int           `mapstructure:"port"`
	ReadTimeout          time.Duration `mapstructure:"readTimeout"`
	WriteTimeout         time.Duration `mapstructure:"writeTimeout"`
	Timezone             string        `mapstructure:"timezone"`
	JWTSecret            string        `mapstructure:"jwtSecret"`
	AccessTokenTTL       time.Duration `mapstructure:"accessTokenTTL"`
	RefreshTokenTTL      time.Duration `mapstructure:"refreshTokenTTL"`
	DefaultAdminPassword string        `mapstructure:"defaultAdminPassword"`
}

// WatchConfig 状态感知层配置
type WatchConfig struct {
	// 监控范围: cluster | namespaces
	Scope         string            `mapstructure:"scope"`
	Namespaces    NamespaceFilter   `mapstructure:"namespaces"`
	LabelSelector string            `mapstructure:"labelSelector"`
	ExcludePods   []PodExcludeRule  `mapstructure:"excludePods"`
	ResyncPeriod  time.Duration     `mapstructure:"resyncPeriod"`
	Kubeconfig    string            `mapstructure:"kubeconfig"`
	Aggregation   AggregationConfig `mapstructure:"aggregation"`
}

// NamespaceFilter 命名空间过滤
type NamespaceFilter struct {
	Include []string `mapstructure:"include"`
	Exclude []string `mapstructure:"exclude"`
}

// PodExcludeRule 明确排除的 Pod 规则
type PodExcludeRule struct {
	Namespace string `mapstructure:"namespace"`
	Name      string `mapstructure:"name"` // 支持通配符
}

// AggregationConfig 去重聚合配置
type AggregationConfig struct {
	GroupWait    time.Duration `mapstructure:"groupWait"`
	ActiveWindow time.Duration `mapstructure:"activeWindow"`
	ResolveWait  time.Duration `mapstructure:"resolveWait"`
}

// CollectConfig 证据采集配置
type CollectConfig struct {
	LogTailLines   int           `mapstructure:"logTailLines"`
	TimeoutPerItem time.Duration `mapstructure:"timeoutPerItem"`
	EnableMetrics  bool          `mapstructure:"enableMetrics"`
	PrometheusURL  string        `mapstructure:"prometheusURL"`
	PromQueryRange time.Duration `mapstructure:"promQueryRange"`
}

// DBConfig 数据库配置
type DBConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbname"`
	SSLMode  string `mapstructure:"sslMode"`
	MaxConns int    `mapstructure:"maxConns"`
	MinConns int    `mapstructure:"minConns"`
}

// DSN 返回 PostgreSQL 连接字符串
func (c DBConfig) DSN() string {
	return "host=" + c.Host +
		" port=" + itoa(c.Port) +
		" user=" + c.User +
		" password=" + c.Password +
		" dbname=" + c.DBName +
		" sslmode=" + c.SSLMode
}

func itoa(i int) string {
	return fmt.Sprintf("%d", i)
}

// LogConfig 日志配置
type LogConfig struct {
	Level string `mapstructure:"level"`
}

// NotifyConfig 通知配置
type NotifyConfig struct {
	Enabled   bool           `mapstructure:"enabled"`
	Webhooks  []WebhookSink  `mapstructure:"webhooks"`
	Larks     []LarkSink     `mapstructure:"larks"`
	Telegrams []TelegramSink `mapstructure:"telegrams"`
}

// WebhookSink Webhook 通知配置
type WebhookSink struct {
	Name    string            `mapstructure:"name"`
	URL     string            `mapstructure:"url"`
	Headers map[string]string `mapstructure:"headers"`
}

// LarkSink 飞书机器人通知配置
type LarkSink struct {
	Name   string `mapstructure:"name"`
	URL    string `mapstructure:"url"`
	Secret string `mapstructure:"secret"`
}

// TelegramSink Telegram 机器人通知配置
type TelegramSink struct {
	Name      string `mapstructure:"name"`
	BotToken  string `mapstructure:"botToken"`
	ChatID    string `mapstructure:"chatId"`
	ParseMode string `mapstructure:"parseMode"`
}
