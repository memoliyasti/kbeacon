package controller

import (
	"testing"
	"time"
)

func TestNamespaceFilterIncludeList(t *testing.T) {
	filter := NewNamespaceFilter(
		[]string{"team-a", "team-b"},
		[]string{"team-b"},
	)

	if !filter.Include("team-a") {
		t.Fatal("expected team-a to be included")
	}

	if filter.Include("team-b") {
		t.Fatal("expected explicit exclude to override include")
	}

	if filter.Include("team-c") {
		t.Fatal("expected team-c to be excluded because include list is restrictive")
	}
}

func TestNamespaceFilterExcludeOnly(t *testing.T) {
	filter := NewNamespaceFilter(
		nil,
		[]string{"kube-system", "kube-public"},
	)

	if !filter.Include("default") {
		t.Fatal("expected default namespace to be included")
	}

	if filter.Include("kube-system") {
		t.Fatal("expected kube-system namespace to be excluded")
	}

	if filter.Include("") {
		t.Fatal("expected empty namespace to be excluded")
	}
}

func TestControllerStatusMarksDisabledResourcesOptional(t *testing.T) {
	ctrl := New(
		nil,
		nil,
		Options{
			Cluster: "minikube",
			Resources: ResourceConfig{
				Secrets:     true,
				Deployments: true,
			},
			ResourcesSet: true,
		},
	)

	status, ready := ctrl.Status()
	if ready {
		t.Fatal("expected controller not to be ready because enabled caches are not synced")
	}

	byResource := map[string]map[string]any{}
	for _, item := range status {
		byResource[item["resource"].(string)] = item
	}

	if byResource["Secret"]["optional"] != nil {
		t.Fatalf("expected enabled Secret not to be optional, got %#v", byResource["Secret"])
	}

	if byResource["Pod"]["optional"] != true {
		t.Fatalf("expected disabled Pod to be optional, got %#v", byResource["Pod"])
	}

	if byResource["Pod"]["reason"] != "disabled" {
		t.Fatalf("expected disabled Pod reason, got %#v", byResource["Pod"])
	}

	if byResource["Deployment"]["optional"] != nil {
		t.Fatalf("expected enabled Deployment not to be optional, got %#v", byResource["Deployment"])
	}
}

type fakeRecorder struct {
	watchEvents  int
	graphUpdates int
}

func (f *fakeRecorder) ObserveWatchEvent(resource, event string) {
	f.watchEvents++
}

func (f *fakeRecorder) ObserveGraphUpdate(reason string, duration time.Duration) {
	f.graphUpdates++
}

func TestControllerStoresRecorder(t *testing.T) {
	recorder := &fakeRecorder{}

	ctrl := New(
		nil,
		nil,
		Options{
			Cluster: "minikube",
			Resources: ResourceConfig{
				Secrets: true,
			},
			ResourcesSet: true,
			Recorder:     recorder,
		},
	)

	if ctrl.recorder != recorder {
		t.Fatal("expected controller to store recorder from options")
	}
}
