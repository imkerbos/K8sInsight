package collector

import (
	"bytes"
	"context"
	"io"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/kerbos/k8sinsight/internal/detector"
)

// collectPreviousLogs 采集上一个容器的日志（P0 最高优先级）
// CrashLoopBackOff 场景下，前一次容器日志会在下次重启时被覆盖
func collectPreviousLogs(ctx context.Context, client kubernetes.Interface, event detector.AnomalyEvent, timeout time.Duration) Evidence {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	containerName := event.ContainerName
	if containerName == "" {
		return Evidence{
			Type:      EvidencePreviousLogs,
			Timestamp: time.Now(),
			Error:     "容器名称为空，跳过日志采集",
		}
	}

	previous := true
	req := client.CoreV1().Pods(event.Namespace).GetLogs(event.PodName, &corev1.PodLogOptions{
		Container: containerName,
		Previous:  previous,
	})

	stream, err := req.Stream(ctx)
	if err != nil {
		return Evidence{
			Type:      EvidencePreviousLogs,
			Timestamp: time.Now(),
			Error:     err.Error(),
		}
	}
	defer stream.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, stream); err != nil {
		return Evidence{
			Type:      EvidencePreviousLogs,
			Content:   buf.String(), // 返回已读取的部分
			Timestamp: time.Now(),
			Error:     err.Error(),
		}
	}

	return Evidence{
		Type:      EvidencePreviousLogs,
		Content:   buf.String(),
		Timestamp: time.Now(),
	}
}

// collectCurrentLogs 采集当前容器尾部日志
func collectCurrentLogs(ctx context.Context, client kubernetes.Interface, event detector.AnomalyEvent, tailLines int, timeout time.Duration) Evidence {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	containerName := event.ContainerName
	if containerName == "" {
		return Evidence{
			Type:      EvidenceCurrentLogs,
			Timestamp: time.Now(),
			Error:     "容器名称为空，跳过日志采集",
		}
	}

	lines := int64(tailLines)
	if lines == 0 {
		lines = 200
	}
	req := client.CoreV1().Pods(event.Namespace).GetLogs(event.PodName, &corev1.PodLogOptions{
		Container: containerName,
		TailLines: &lines,
	})

	stream, err := req.Stream(ctx)
	if err != nil {
		return Evidence{
			Type:      EvidenceCurrentLogs,
			Timestamp: time.Now(),
			Error:     err.Error(),
		}
	}
	defer stream.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, stream); err != nil {
		return Evidence{
			Type:      EvidenceCurrentLogs,
			Content:   buf.String(),
			Timestamp: time.Now(),
			Error:     err.Error(),
		}
	}

	return Evidence{
		Type:      EvidenceCurrentLogs,
		Content:   buf.String(),
		Timestamp: time.Now(),
	}
}
