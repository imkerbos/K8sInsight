package core

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

const (
	defaultClientTimeout = 10 * time.Second
	testConnTimeout      = 5 * time.Second
	testConnRetries      = 3
	testConnRetryDelay   = 1 * time.Second
)

// NewKubeClient 创建 Kubernetes 客户端
// 优先使用 InCluster 配置（Pod 内运行），否则回退到 kubeconfig（本地开发）
func NewKubeClient(kubeconfigPath string) (kubernetes.Interface, error) {
	config, err := buildConfig(kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("构建 K8s 配置失败: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("创建 K8s 客户端失败: %w", err)
	}

	return clientset, nil
}

// NewKubeClientFromContent 从 kubeconfig 文本内容创建 Kubernetes 客户端（默认 10s 超时）
func NewKubeClientFromContent(content string) (kubernetes.Interface, error) {
	restConfig, err := clientcmd.RESTConfigFromKubeConfig([]byte(content))
	if err != nil {
		return nil, fmt.Errorf("解析 kubeconfig 内容失败: %w", err)
	}

	restConfig.Timeout = defaultClientTimeout

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("创建 K8s 客户端失败: %w", err)
	}

	return clientset, nil
}

// newKubeClientWithTimeout 从 kubeconfig 文本创建指定超时的客户端
func newKubeClientWithTimeout(content string, timeout time.Duration) (kubernetes.Interface, error) {
	restConfig, err := clientcmd.RESTConfigFromKubeConfig([]byte(content))
	if err != nil {
		return nil, fmt.Errorf("解析 kubeconfig 内容失败: %w", err)
	}

	restConfig.Timeout = timeout

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("创建 K8s 客户端失败: %w", err)
	}

	return clientset, nil
}

// ClusterInfo 集群基础信息
type ClusterInfo struct {
	Version   string `json:"version"`
	NodeCount int    `json:"nodeCount"`
}

// TestKubeConnection 测试 kubeconfig 连通性（5s 超时，自动重试 3 次，间隔 1s）
func TestKubeConnection(content string) (*ClusterInfo, error) {
	clientset, err := newKubeClientWithTimeout(content, testConnTimeout)
	if err != nil {
		return nil, err
	}

	var lastErr error
	for i := 0; i < testConnRetries; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), testConnTimeout)

		sv, err := clientset.Discovery().ServerVersion()
		if err == nil {
			// 连接成功，顺便获取节点数
			nodes, _ := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
			nodeCount := 0
			if nodes != nil {
				nodeCount = len(nodes.Items)
			}
			cancel()
			return &ClusterInfo{
				Version:   sv.GitVersion,
				NodeCount: nodeCount,
			}, nil
		}

		cancel()
		lastErr = err

		if i < testConnRetries-1 {
			time.Sleep(testConnRetryDelay)
		}
	}

	return nil, fmt.Errorf("集群连接测试失败（重试 %d 次）: %w", testConnRetries, lastErr)
}

// GetClusterInfo 获取集群基础信息（版本、节点数���，使用默认 10s 超时
func GetClusterInfo(content string) (*ClusterInfo, error) {
	clientset, err := NewKubeClientFromContent(content)
	if err != nil {
		return nil, err
	}

	sv, err := clientset.Discovery().ServerVersion()
	if err != nil {
		return nil, fmt.Errorf("获取集群版本失败: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultClientTimeout)
	defer cancel()

	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	nodeCount := 0
	if err == nil {
		nodeCount = len(nodes.Items)
	}

	return &ClusterInfo{
		Version:   sv.GitVersion,
		NodeCount: nodeCount,
	}, nil
}

func buildConfig(kubeconfigPath string) (*rest.Config, error) {
	// 优先尝试 InCluster
	if config, err := rest.InClusterConfig(); err == nil {
		return config, nil
	}

	// 回退到 kubeconfig 文件
	if kubeconfigPath == "" {
		if home := homedir.HomeDir(); home != "" {
			kubeconfigPath = filepath.Join(home, ".kube", "config")
		}
	}

	return clientcmd.BuildConfigFromFlags("", kubeconfigPath)
}
