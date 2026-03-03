package sink

import (
	"fmt"

	"github.com/kerbos/k8sinsight/internal/detector"
)

type rootCauseResult struct {
	Summary     string
	Suggestions []string
}

func inferRootCause(event detector.AnomalyEvent) rootCauseResult {
	switch event.Type {
	case detector.AnomalyOOMKilled:
		return rootCauseResult{
			Summary: "容器触发 OOMKill，内存上限不足或存在内存泄漏。",
			Suggestions: []string{
				"提高容器 memory limit/request，避免被 cgroup 杀死。",
				"排查应用内存增长路径，重点检查缓存和大对象分配。",
				"结合 GC/heap profile 与业务高峰时段进行对比分析。",
			},
		}
	case detector.AnomalyCrashLoopBackOff:
		return rootCauseResult{
			Summary: "容器反复启动失败，进入 CrashLoopBackOff。",
			Suggestions: []string{
				"检查容器启动日志与启动命令/参数是否正确。",
				"确认依赖配置、密钥、下游服务连通性是否满足启动条件。",
				"必要时先放宽探针阈值，避免应用尚未就绪被过早重启。",
			},
		}
	case detector.AnomalyErrorExit:
		msg := "容器异常退出（非 OOM）。"
		if event.ExitCode != 0 {
			msg = fmt.Sprintf("容器异常退出（exitCode=%d）。", event.ExitCode)
		}
		return rootCauseResult{
			Summary: msg,
			Suggestions: []string{
				"对照退出码和应用日志定位失败阶段。",
				"检查配置与环境变量是否完整（连接串、凭据、端口）。",
				"回溯最近发布变更，确认是否引入不兼容行为。",
			},
		}
	case detector.AnomalyImagePullBackOff:
		return rootCauseResult{
			Summary: "镜像拉取失败，镜像不可达或认证异常。",
			Suggestions: []string{
				"确认镜像名/tag 存在且可访问。",
				"检查 imagePullSecrets 与镜像仓库权限。",
				"排查节点到仓库网络与 DNS 连通性。",
			},
		}
	case detector.AnomalyCreateContainerConfigError:
		return rootCauseResult{
			Summary: "容器配置错误导致创建失败。",
			Suggestions: []string{
				"检查引用的 ConfigMap/Secret 是否存在且键名正确。",
				"核对 volumeMount、envFrom、命令参数配置。",
				"比对工作负载模板与最近变更记录。",
			},
		}
	case detector.AnomalyFailedScheduling:
		return rootCauseResult{
			Summary: "Pod 调度失败，资源或调度约束不满足。",
			Suggestions: []string{
				"检查节点资源是否足够（CPU/内存/ephemeral-storage）。",
				"检查 nodeSelector/affinity/taint-toleration 约束。",
				"必要时扩容节点或调整请求资源规格。",
			},
		}
	case detector.AnomalyEvicted:
		return rootCauseResult{
			Summary: "Pod 被驱逐，节点资源压力过高。",
			Suggestions: []string{
				"检查节点内存/磁盘压力与 kubelet 驱逐阈值。",
				"提高关键工作负载优先级，优化 request/limit。",
				"清理节点无效镜像、日志与临时文件。",
			},
		}
	case detector.AnomalyStateOscillation:
		return rootCauseResult{
			Summary: "Pod 状态频繁振荡，存在不稳定依赖或探针抖动。",
			Suggestions: []string{
				"核对健康探针阈值与应用启动耗时匹配。",
				"检查下游依赖是否波动导致频繁失败恢复。",
				"结合事件时间线定位首次异常触发点。",
			},
		}
	default:
		return rootCauseResult{
			Summary: "检测到异常事件，需结合日志与事件进一步确认根因。",
			Suggestions: []string{
				"优先查看事件详情与证据采集结果。",
				"对照最近变更与集群资源状态进行排查。",
			},
		}
	}
}
