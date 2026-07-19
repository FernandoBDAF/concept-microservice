package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Minimal shapes for `kubectl get pods -o json`.
type podList struct {
	Items []pod `json:"items"`
}

type pod struct {
	Metadata podMetadata `json:"metadata"`
	Status   podStatus   `json:"status"`
}

type podMetadata struct {
	Name            string           `json:"name"`
	Namespace       string           `json:"namespace"`
	OwnerReferences []ownerReference `json:"ownerReferences"`
}

type ownerReference struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}

type podStatus struct {
	Phase             string            `json:"phase"`
	ContainerStatuses []containerStatus `json:"containerStatuses"`
}

type containerStatus struct {
	Ready bool `json:"ready"`
}

// KindWorkload is the workload-level summary for the kind target.
type KindWorkload struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Ready     string `json:"ready"` // "n/m": ready pods / total pods
	Status    string `json:"status"`
}

// workloadName collapses a pod name to its owning workload's name.
//   - ReplicaSet owner: strip the trailing pod-template-hash segment ->
//     Deployment name. A ReplicaSet name always ends in "-<hash>", and the
//     hash alphabet is not plain hex, so segment-stripping beats a regex.
//   - StatefulSet/DaemonSet/Job owner: the owner name as-is.
//   - No owner: the pod name as-is (bare pods).
func workloadName(p pod) string {
	for _, ref := range p.Metadata.OwnerReferences {
		switch ref.Kind {
		case "ReplicaSet":
			if i := strings.LastIndex(ref.Name, "-"); i > 0 {
				return ref.Name[:i]
			}
			return ref.Name
		case "StatefulSet", "DaemonSet", "Job":
			return ref.Name
		}
	}
	return p.Metadata.Name
}

// podReady reports whether every container in the pod is ready.
// Pods with no reported container statuses count as not ready.
func podReady(p pod) bool {
	if len(p.Status.ContainerStatuses) == 0 {
		return false
	}
	for _, cs := range p.Status.ContainerStatuses {
		if !cs.Ready {
			return false
		}
	}
	return true
}

// summarizeKindPods aggregates a kubectl pod list into workload-level rows.
// Ready is "readyPods/totalPods". Status is "Running" when all pods are
// running, otherwise the first non-Running phase encountered.
func summarizeKindPods(raw []byte, namespace string) ([]KindWorkload, error) {
	var list podList
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, fmt.Errorf("parse kubectl pod list: %w", err)
	}

	type agg struct {
		total    int
		ready    int
		badPhase string // first non-Running phase seen, if any
	}
	byWorkload := map[string]*agg{}
	for _, p := range list.Items {
		name := workloadName(p)
		a := byWorkload[name]
		if a == nil {
			a = &agg{}
			byWorkload[name] = a
		}
		a.total++
		if podReady(p) {
			a.ready++
		}
		if p.Status.Phase != "Running" && a.badPhase == "" {
			a.badPhase = p.Status.Phase
		}
	}

	names := make([]string, 0, len(byWorkload))
	for name := range byWorkload {
		names = append(names, name)
	}
	sort.Strings(names)

	out := make([]KindWorkload, 0, len(names))
	for _, name := range names {
		a := byWorkload[name]
		status := "Running"
		if a.badPhase != "" {
			status = a.badPhase
		}
		out = append(out, KindWorkload{
			Namespace: namespace,
			Name:      name,
			Ready:     fmt.Sprintf("%d/%d", a.ready, a.total),
			Status:    status,
		})
	}
	return out, nil
}

// kindHealthFromPods derives per-service health for the kind target from pod
// readiness in lab-core (no HTTP probing in-cluster).
func kindHealthFromPods(raw []byte) ([]HealthResult, error) {
	workloads, err := summarizeKindPods(raw, "lab-core")
	if err != nil {
		return nil, err
	}
	out := make([]HealthResult, 0, len(workloads))
	for _, w := range workloads {
		parts := strings.SplitN(w.Ready, "/", 2)
		ok := len(parts) == 2 && parts[0] == parts[1] && parts[0] != "0"
		hr := HealthResult{Service: w.Name, OK: ok}
		if !ok {
			hr.Error = fmt.Sprintf("pods ready %s, status %s", w.Ready, w.Status)
		}
		out = append(out, hr)
	}
	return out, nil
}

// kindClusterListed reports whether `kind get clusters` output contains the
// given cluster name (one name per line).
func kindClusterListed(raw []byte, cluster string) bool {
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(line) == cluster {
			return true
		}
	}
	return false
}
