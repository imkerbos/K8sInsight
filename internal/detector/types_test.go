package detector

import "testing"

func TestAnomalyEventDedupKey_PodScopedByUID(t *testing.T) {
	e1 := AnomalyEvent{
		ClusterID: "c1",
		Namespace: "default",
		PodUID:    "uid-a",
		PodName:   "demo-a",
		Type:      AnomalyOOMKilled,
		OwnerKind: "Deployment",
		OwnerName: "shared-workload",
	}
	e2 := AnomalyEvent{
		ClusterID: "c1",
		Namespace: "default",
		PodUID:    "uid-b",
		PodName:   "demo-b",
		Type:      AnomalyOOMKilled,
		OwnerKind: "Deployment",
		OwnerName: "shared-workload",
	}

	if e1.DedupKey() == e2.DedupKey() {
		t.Fatalf("expected different dedup keys for different pod UIDs, got same: %s", e1.DedupKey())
	}
}

func TestAnomalyEventDedupKey_FallbackToPodName(t *testing.T) {
	e := AnomalyEvent{
		ClusterID: "c1",
		Namespace: "default",
		PodName:   "demo-a",
		Type:      AnomalyCrashLoopBackOff,
	}

	got := e.DedupKey()
	want := "c1/default/Pod/demo-a/CrashLoopBackOff"
	if got != want {
		t.Fatalf("unexpected dedup key: got %q, want %q", got, want)
	}
}
