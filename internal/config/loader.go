package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Load 加载配置文件，支持 YAML / ENV 覆盖
func Load(configPath string) (*Config, error) {
	v := viper.New()

	// 默认值
	setDefaults(v)

	// 配置文件
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath("./configs")
		v.AddConfigPath("/etc/k8sinsight")
	}

	// 环境变量覆盖：K8SINSIGHT_SERVER_PORT → server.port
	v.SetEnvPrefix("K8SINSIGHT")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("读取配置文件失败: %w", err)
		}
		// 配置文件不存在时使用默认值
	}

	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}

	return cfg, nil
}

func setDefaults(v *viper.Viper) {
	// Server
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.readTimeout", 30*time.Second)
	v.SetDefault("server.writeTimeout", 30*time.Second)
	v.SetDefault("server.timezone", "Asia/Shanghai")
	v.SetDefault("server.jwtSecret", "")
	v.SetDefault("server.accessTokenTTL", 2*time.Hour)
	v.SetDefault("server.refreshTokenTTL", 168*time.Hour) // 7 days
	v.SetDefault("server.defaultAdminPassword", "k8sinsight")

	// Watch
	v.SetDefault("watch.scope", "cluster")
	v.SetDefault("watch.resyncPeriod", 30*time.Minute)
	v.SetDefault("watch.namespaces.exclude", []string{"kube-system", "kube-public", "kube-node-lease"})
	v.SetDefault("watch.aggregation.groupWait", 30*time.Second)
	v.SetDefault("watch.aggregation.activeWindow", 6*time.Hour)
	v.SetDefault("watch.aggregation.resolveWait", 5*time.Minute)

	// Collect
	v.SetDefault("collect.logTailLines", 200)
	v.SetDefault("collect.timeoutPerItem", 10*time.Second)
	v.SetDefault("collect.enableMetrics", true)
	v.SetDefault("collect.prometheusURL", "")
	v.SetDefault("collect.promQueryRange", 10*time.Minute)

	// DB
	v.SetDefault("db.host", "localhost")
	v.SetDefault("db.port", 5432)
	v.SetDefault("db.user", "k8sinsight")
	v.SetDefault("db.dbname", "k8sinsight")
	v.SetDefault("db.sslMode", "disable")
	v.SetDefault("db.maxConns", 20)
	v.SetDefault("db.minConns", 5)

	// Log
	v.SetDefault("log.level", "info")

	// Notify
	v.SetDefault("notify.enabled", false)
	v.SetDefault("notify.webhooks", []any{})
	v.SetDefault("notify.larks", []any{})
	v.SetDefault("notify.telegrams", []any{})
}
