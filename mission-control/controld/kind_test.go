package main

import "testing"

// Trimmed capture shape of `kubectl get pods -n lab-core -o json`:
// deployment-owned pods (ReplicaSet owner with pod-template-hash), one
// scaled deployment, one not-ready pod.
const kindCorePodsFixture = `{
  "apiVersion": "v1",
  "kind": "List",
  "items": [
    {
      "metadata": {
        "name": "api-service-7d4b9c8f6d-abcde",
        "namespace": "lab-core",
        "ownerReferences": [{"kind": "ReplicaSet", "name": "api-service-7d4b9c8f6d"}]
      },
      "status": {"phase": "Running", "containerStatuses": [{"ready": true}]}
    },
    {
      "metadata": {
        "name": "api-service-7d4b9c8f6d-fghij",
        "namespace": "lab-core",
        "ownerReferences": [{"kind": "ReplicaSet", "name": "api-service-7d4b9c8f6d"}]
      },
      "status": {"phase": "Running", "containerStatuses": [{"ready": true}]}
    },
    {
      "metadata": {
        "name": "auth-service-5f6a7b8c9d-klmno",
        "namespace": "lab-core",
        "ownerReferences": [{"kind": "ReplicaSet", "name": "auth-service-5f6a7b8c9d"}]
      },
      "status": {"phase": "Running", "containerStatuses": [{"ready": false}]}
    },
    {
      "metadata": {
        "name": "graphrag-service-1a2b3c4d5e-pqrst",
        "namespace": "lab-core",
        "ownerReferences": [{"kind": "ReplicaSet", "name": "graphrag-service-1a2b3c4d5e"}]
      },
      "status": {"phase": "Pending", "containerStatuses": []}
    }
  ]
}`

// Trimmed capture shape for lab-infra: statefulset-owned pods with ordinal
// suffixes, plus a deployment (redis).
const kindInfraPodsFixture = `{
  "apiVersion": "v1",
  "kind": "List",
  "items": [
    {
      "metadata": {
        "name": "postgres-0",
        "namespace": "lab-infra",
        "ownerReferences": [{"kind": "StatefulSet", "name": "postgres"}]
      },
      "status": {"phase": "Running", "containerStatuses": [{"ready": true}]}
    },
    {
      "metadata": {
        "name": "rabbitmq-0",
        "namespace": "lab-infra",
        "ownerReferences": [{"kind": "StatefulSet", "name": "rabbitmq"}]
      },
      "status": {"phase": "Running", "containerStatuses": [{"ready": true}]}
    },
    {
      "metadata": {
        "name": "redis-6c7d8e9f0a-uvwxy",
        "namespace": "lab-infra",
        "ownerReferences": [{"kind": "ReplicaSet", "name": "redis-6c7d8e9f0a"}]
      },
      "status": {"phase": "Running", "containerStatuses": [{"ready": true}]}
    }
  ]
}`

func TestSummarizeKindPodsCore(t *testing.T) {
	workloads, err := summarizeKindPods([]byte(kindCorePodsFixture), "lab-core")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []KindWorkload{
		{Namespace: "lab-core", Name: "api-service", Ready: "2/2", Status: "Running"},
		{Namespace: "lab-core", Name: "auth-service", Ready: "0/1", Status: "Running"},
		{Namespace: "lab-core", Name: "graphrag-service", Ready: "0/1", Status: "Pending"},
	}
	if len(workloads) != len(want) {
		t.Fatalf("expected %d workloads, got %d: %+v", len(want), len(workloads), workloads)
	}
	for i, w := range want {
		if workloads[i] != w {
			t.Errorf("workload %d: got %+v, want %+v", i, workloads[i], w)
		}
	}
}

func TestSummarizeKindPodsInfra(t *testing.T) {
	workloads, err := summarizeKindPods([]byte(kindInfraPodsFixture), "lab-infra")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []KindWorkload{
		{Namespace: "lab-infra", Name: "postgres", Ready: "1/1", Status: "Running"},
		{Namespace: "lab-infra", Name: "rabbitmq", Ready: "1/1", Status: "Running"},
		{Namespace: "lab-infra", Name: "redis", Ready: "1/1", Status: "Running"},
	}
	if len(workloads) != len(want) {
		t.Fatalf("expected %d workloads, got %d: %+v", len(want), len(workloads), workloads)
	}
	for i, w := range want {
		if workloads[i] != w {
			t.Errorf("workload %d: got %+v, want %+v", i, workloads[i], w)
		}
	}
}

func TestSummarizeKindPodsEmpty(t *testing.T) {
	workloads, err := summarizeKindPods([]byte(`{"items": []}`), "lab-obs")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(workloads) != 0 {
		t.Fatalf("expected no workloads, got %+v", workloads)
	}
}

func TestSummarizeKindPodsMalformed(t *testing.T) {
	if _, err := summarizeKindPods([]byte("not json"), "lab-core"); err == nil {
		t.Fatal("expected error for malformed input")
	}
}

func TestKindHealthFromPods(t *testing.T) {
	results, err := kindHealthFromPods([]byte(kindCorePodsFixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	byService := map[string]HealthResult{}
	for _, r := range results {
		byService[r.Service] = r
	}
	if !byService["api-service"].OK {
		t.Errorf("api-service should be ok: %+v", byService["api-service"])
	}
	if byService["auth-service"].OK {
		t.Errorf("auth-service should not be ok: %+v", byService["auth-service"])
	}
	if byService["auth-service"].Error == "" {
		t.Error("auth-service should carry an error message")
	}
	if byService["graphrag-service"].OK {
		t.Errorf("graphrag-service should not be ok: %+v", byService["graphrag-service"])
	}
}

func TestWorkloadNameNonHexHash(t *testing.T) {
	// pod-template-hash alphabet is not plain hex (e.g. "5tqxv9b").
	p := pod{Metadata: podMetadata{
		Name:            "profile-worker-5tqxv9b-zw4kp",
		OwnerReferences: []ownerReference{{Kind: "ReplicaSet", Name: "profile-worker-5tqxv9b"}},
	}}
	if got := workloadName(p); got != "profile-worker" {
		t.Errorf("got %q, want profile-worker", got)
	}
	bare := pod{Metadata: podMetadata{Name: "debug-pod"}}
	if got := workloadName(bare); got != "debug-pod" {
		t.Errorf("bare pod: got %q, want debug-pod", got)
	}
}

func TestKindClusterListed(t *testing.T) {
	if !kindClusterListed([]byte("lab\n"), "lab") {
		t.Error("expected lab cluster to be listed")
	}
	if !kindClusterListed([]byte("other\nlab\n"), "lab") {
		t.Error("expected lab cluster among multiple")
	}
	if kindClusterListed([]byte("No kind clusters found.\n"), "lab") {
		t.Error("did not expect lab in empty listing")
	}
	if kindClusterListed([]byte("laboratory\n"), "lab") {
		t.Error("did not expect substring match")
	}
}
