package detector

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestErrorExitRule_IgnoreExitCode143(t *testing.T) {
	r := NewErrorExitRule()
	newPod := podWithLastTermination("app", 143, "Error", 1)

	events := r.Evaluate(nil, newPod)
	if len(events) != 0 {
		t.Fatalf("expected no events for exit code 143, got %d", len(events))
	}
}

func TestErrorExitRule_IgnoreExitCode0(t *testing.T) {
	r := NewErrorExitRule()
	newPod := podWithLastTermination("app", 0, "Completed", 1)

	events := r.Evaluate(nil, newPod)
	if len(events) != 0 {
		t.Fatalf("expected no events for exit code 0, got %d", len(events))
	}
}

func TestErrorExitRule_RecordNonZeroExit(t *testing.T) {
	r := NewErrorExitRule()
	newPod := podWithLastTermination("app", 1, "Error", 1)

	events := r.Evaluate(nil, newPod)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != AnomalyErrorExit {
		t.Fatalf("expected AnomalyErrorExit, got %s", events[0].Type)
	}
	if events[0].ExitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", events[0].ExitCode)
	}
}

func TestOOMKilledRule_IgnoreExitCode143(t *testing.T) {
	r := NewOOMKilledRule()
	newPod := podWithLastTermination("app", 143, "OOMKilled", 1)

	events := r.Evaluate(nil, newPod)
	if len(events) != 0 {
		t.Fatalf("expected no oom events for exit code 143, got %d", len(events))
	}
}

func TestOOMKilledRule_RecordNonNormalExit(t *testing.T) {
	r := NewOOMKilledRule()
	newPod := podWithLastTermination("app", 137, "OOMKilled", 1)

	events := r.Evaluate(nil, newPod)
	if len(events) != 1 {
		t.Fatalf("expected 1 oom event, got %d", len(events))
	}
	if events[0].Type != AnomalyOOMKilled {
		t.Fatalf("expected AnomalyOOMKilled, got %s", events[0].Type)
	}
	if events[0].ExitCode != 137 {
		t.Fatalf("expected exit code 137, got %d", events[0].ExitCode)
	}
}

func podWithLastTermination(container string, exitCode int32, reason string, restartCount int32) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo-pod",
			Namespace: "default",
			UID:       "pod-uid-1",
		},
		Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:         container,
					RestartCount: restartCount,
					LastTerminationState: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode: exitCode,
							Reason:   reason,
						},
					},
				},
			},
		},
	}
}
