package controller

import (
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
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
			Cluster: "test-cluster",
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

func TestControllerUsesNamespaceScopedInformerFactoryForSingleIncludedNamespace(t *testing.T) {
	ctrl := New(
		nil,
		nil,
		Options{
			Cluster:           "test-cluster",
			IncludeNamespaces: []string{"payments"},
			ExcludeNamespaces: []string{"kube-system"},
			Resources: ResourceConfig{
				Pods: true,
			},
			ResourcesSet: true,
		},
	)

	if ctrl.watchNamespace != "payments" {
		t.Fatalf("expected namespace-scoped informer factory for payments, got %q", ctrl.watchNamespace)
	}
}

func TestControllerUsesClusterScopedInformerFactoryWithoutSingleIncludedNamespace(t *testing.T) {
	ctrl := New(
		nil,
		nil,
		Options{
			Cluster:           "test-cluster",
			IncludeNamespaces: []string{"payments", "reports"},
			Resources: ResourceConfig{
				Pods: true,
			},
			ResourcesSet: true,
		},
	)

	if ctrl.watchNamespace != "" {
		t.Fatalf("expected cluster-scoped informer factory, got namespace %q", ctrl.watchNamespace)
	}
}

func TestControllerStoresCertificateInformerWhenEnabled(t *testing.T) {
	ctrl := New(
		nil,
		nil,
		Options{
			Cluster:       "test-cluster",
			DynamicClient: dynamicfake.NewSimpleDynamicClient(runtime.NewScheme()),
			Resources: ResourceConfig{
				Certificates: true,
			},
			ResourcesSet: true,
		},
	)

	if ctrl.certificateInformer == nil {
		t.Fatal("expected cert-manager Certificate informer when enabled")
	}

	if _, ok := ctrl.synced["Certificate"]; !ok {
		t.Fatalf("expected Certificate sync status to be registered, got %#v", ctrl.synced)
	}
}

func TestControllerEnabledSyncsIncludeCertificate(t *testing.T) {
	ctrl := New(nil, nil, Options{
		Cluster:       "test-cluster",
		DynamicClient: dynamicfake.NewSimpleDynamicClient(runtime.NewScheme()),
		Resources: ResourceConfig{
			Certificates: true,
		},
		ResourcesSet: true,
	})

	names := map[string]bool{}
	for _, item := range ctrl.enabledSyncs() {
		names[item.name] = true
	}

	if !names["Certificate"] {
		t.Fatalf("expected Certificate informer in enabled syncs, got %#v", names)
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
			Cluster: "test-cluster",
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

func TestControllerLowPrivilegeModeMarksSecretInformerOptional(t *testing.T) {
	ctrl := New(
		nil,
		nil,
		Options{
			Cluster: "test-cluster",
			Resources: ResourceConfig{
				Secrets:     false,
				Deployments: true,
			},
			ResourcesSet: true,
		},
	)

	status, ready := ctrl.Status()
	if ready {
		t.Fatal("expected controller not to be ready because Deployment cache is enabled but not synced")
	}

	byResource := map[string]map[string]any{}
	for _, item := range status {
		byResource[item["resource"].(string)] = item
	}

	if byResource["Secret"]["optional"] != true {
		t.Fatalf("expected disabled Secret informer to be optional, got %#v", byResource["Secret"])
	}

	if byResource["Secret"]["reason"] != "disabled" {
		t.Fatalf("expected disabled Secret reason, got %#v", byResource["Secret"])
	}

	cacheStatus := ctrl.CacheSyncStatus()
	if _, ok := cacheStatus["Secret"]; ok {
		t.Fatalf("expected disabled Secret informer to be omitted from cache sync status, got %#v", cacheStatus)
	}

	if _, ok := cacheStatus["Deployment"]; !ok {
		t.Fatalf("expected enabled Deployment informer in cache sync status, got %#v", cacheStatus)
	}
}

func TestControllerEnabledSyncsIncludeServiceAccountAndIngress(t *testing.T) {
	ctrl := New(nil, nil, Options{
		Cluster: "test-cluster",
		Resources: ResourceConfig{
			ServiceAccounts: true,
			Ingresses:       true,
		},
		ResourcesSet: true,
	})

	names := map[string]bool{}
	for _, item := range ctrl.enabledSyncs() {
		names[item.name] = true
	}

	if !names["ServiceAccount"] {
		t.Fatalf("expected ServiceAccount informer in enabled syncs, got %#v", names)
	}
	if !names["Ingress"] {
		t.Fatalf("expected Ingress informer in enabled syncs, got %#v", names)
	}
}

func TestControllerStoresExternalSecretInformerWhenEnabled(t *testing.T) {
	ctrl := New(
		nil,
		nil,
		Options{
			Cluster:       "test-cluster",
			DynamicClient: dynamicfake.NewSimpleDynamicClient(runtime.NewScheme()),
			Resources: ResourceConfig{
				ExternalSecrets: true,
			},
			ResourcesSet: true,
		},
	)

	if ctrl.externalSecretInformer == nil {
		t.Fatal("expected ExternalSecret informer when enabled")
	}

	if _, ok := ctrl.synced["ExternalSecret"]; !ok {
		t.Fatalf("expected ExternalSecret sync status to be registered, got %#v", ctrl.synced)
	}
}

func TestControllerEnabledSyncsIncludeExternalSecret(t *testing.T) {
	ctrl := New(nil, nil, Options{
		Cluster:       "test-cluster",
		DynamicClient: dynamicfake.NewSimpleDynamicClient(runtime.NewScheme()),
		Resources: ResourceConfig{
			ExternalSecrets: true,
		},
		ResourcesSet: true,
	})

	names := map[string]bool{}
	for _, item := range ctrl.enabledSyncs() {
		names[item.name] = true
	}

	if !names["ExternalSecret"] {
		t.Fatalf("expected ExternalSecret informer in enabled syncs, got %#v", names)
	}
}

func TestControllerStoresSecretProviderClassInformerWhenEnabled(t *testing.T) {
	ctrl := New(
		nil,
		nil,
		Options{
			Cluster:       "test-cluster",
			DynamicClient: dynamicfake.NewSimpleDynamicClient(runtime.NewScheme()),
			Resources: ResourceConfig{
				SecretProviderClasses: true,
			},
			ResourcesSet: true,
		},
	)

	if ctrl.secretProviderClassInformer == nil {
		t.Fatal("expected SecretProviderClass informer when enabled")
	}

	if _, ok := ctrl.synced["SecretProviderClass"]; !ok {
		t.Fatalf("expected SecretProviderClass sync status to be registered, got %#v", ctrl.synced)
	}
}

func TestControllerEnabledSyncsIncludeSecretProviderClass(t *testing.T) {
	ctrl := New(nil, nil, Options{
		Cluster:       "test-cluster",
		DynamicClient: dynamicfake.NewSimpleDynamicClient(runtime.NewScheme()),
		Resources: ResourceConfig{
			SecretProviderClasses: true,
		},
		ResourcesSet: true,
	})

	names := map[string]bool{}
	for _, item := range ctrl.enabledSyncs() {
		names[item.name] = true
	}

	if !names["SecretProviderClass"] {
		t.Fatalf("expected SecretProviderClass informer in enabled syncs, got %#v", names)
	}
}

func TestControllerStoresKafkaConnectorInformerWhenEnabled(t *testing.T) {
	ctrl := New(
		nil,
		nil,
		Options{
			Cluster:       "test-cluster",
			DynamicClient: dynamicfake.NewSimpleDynamicClient(runtime.NewScheme()),
			Resources: ResourceConfig{
				KafkaConnectors: true,
			},
			ResourcesSet: true,
		},
	)

	if ctrl.kafkaConnectorInformer == nil {
		t.Fatal("expected KafkaConnector informer when enabled")
	}

	if _, ok := ctrl.synced["KafkaConnector"]; !ok {
		t.Fatalf("expected KafkaConnector sync status to be registered, got %#v", ctrl.synced)
	}
}

func TestControllerEnabledSyncsIncludeKafkaConnector(t *testing.T) {
	ctrl := New(nil, nil, Options{
		Cluster:       "test-cluster",
		DynamicClient: dynamicfake.NewSimpleDynamicClient(runtime.NewScheme()),
		Resources: ResourceConfig{
			KafkaConnectors: true,
		},
		ResourcesSet: true,
	})

	names := map[string]bool{}
	for _, item := range ctrl.enabledSyncs() {
		names[item.name] = true
	}

	if !names["KafkaConnector"] {
		t.Fatalf("expected KafkaConnector informer in enabled syncs, got %#v", names)
	}
}

func TestControllerStoresConfluentConnectorInformerWhenEnabled(t *testing.T) {
	ctrl := New(
		nil,
		nil,
		Options{
			Cluster:       "test-cluster",
			DynamicClient: dynamicfake.NewSimpleDynamicClient(runtime.NewScheme()),
			Resources: ResourceConfig{
				ConfluentConnectors: true,
			},
			ResourcesSet: true,
		},
	)

	if ctrl.confluentConnectorInformer == nil {
		t.Fatal("expected Connector informer when enabled")
	}

	if _, ok := ctrl.synced["Connector"]; !ok {
		t.Fatalf("expected Connector sync status to be registered, got %#v", ctrl.synced)
	}
}

func TestControllerEnabledSyncsIncludeConfluentConnector(t *testing.T) {
	ctrl := New(nil, nil, Options{
		Cluster:       "test-cluster",
		DynamicClient: dynamicfake.NewSimpleDynamicClient(runtime.NewScheme()),
		Resources: ResourceConfig{
			ConfluentConnectors: true,
		},
		ResourcesSet: true,
	})

	names := map[string]bool{}
	for _, item := range ctrl.enabledSyncs() {
		names[item.name] = true
	}

	if !names["Connector"] {
		t.Fatalf("expected Connector informer in enabled syncs, got %#v", names)
	}
}

func TestOptionalCRDGVRVersionsMatchSupportedResources(t *testing.T) {
	tests := []struct {
		name         string
		group        string
		version      string
		resource     string
		wantGroup    string
		wantVersion  string
		wantResource string
	}{
		{
			name:         "Strimzi KafkaConnector",
			group:        strimziKafkaConnectorGVR.Group,
			version:      strimziKafkaConnectorGVR.Version,
			resource:     strimziKafkaConnectorGVR.Resource,
			wantGroup:    "kafka.strimzi.io",
			wantVersion:  "v1beta2",
			wantResource: "kafkaconnectors",
		},
		{
			name:         "Confluent Connector",
			group:        confluentConnectorGVR.Group,
			version:      confluentConnectorGVR.Version,
			resource:     confluentConnectorGVR.Resource,
			wantGroup:    "platform.confluent.io",
			wantVersion:  "v1beta1",
			wantResource: "connectors",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.group != tt.wantGroup || tt.version != tt.wantVersion || tt.resource != tt.wantResource {
				t.Fatalf(
					"unexpected GVR for %s: %s/%s/%s",
					tt.name,
					tt.group,
					tt.version,
					tt.resource,
				)
			}
		})
	}
}
