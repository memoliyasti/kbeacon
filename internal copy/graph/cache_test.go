package graph

import (
	"testing"
	"time"
)

func TestApplySnapshotDerivesSecretMetadataFromAffectedWorkload(t *testing.T) {
	cache := NewCache("minikube")
	now := time.Unix(1000, 0)

	workload := WorkloadRef{
		Cluster:    "minikube",
		Namespace:  "kbeacon-demo",
		APIVersion: "apps/v1",
		Kind:       "Deployment",
		Name:       "api",
	}

	secret := SecretRef{
		Cluster:   "minikube",
		Namespace: "kbeacon-demo",
		Name:      "app-db-secret",
	}

	cache.ApplySnapshot(
		[]SecretInput{
			{
				Ref:               secret,
				Type:              "Opaque",
				ResourceVersion:   "1",
				CreationTimestamp: now,
			},
		},
		[]WorkloadInput{
			{
				Ref:           workload,
				OwnerTeam:     "platform",
				Criticality:   "high",
				DiscoveryMode: DiscoveryModeHybrid,
				Edges: []DependencyEdge{
					{
						Workload:      workload,
						Secret:        secret,
						DiscoveryMode: DiscoveryModeInfer,
						Sources: []DependencySource{
							{
								Type: "env.secretKeyRef",
								Path: "env[DB_PASSWORD].valueFrom.secretKeyRef",
							},
						},
					},
				},
			},
		},
		now,
	)

	impact, ok := cache.GetSecretImpact("kbeacon-demo", "app-db-secret")
	if !ok {
		t.Fatal("expected secret impact to be found")
	}

	if impact.Secret.OwnerTeam != "platform" {
		t.Fatalf("expected derived ownerTeam platform, got %q", impact.Secret.OwnerTeam)
	}

	if impact.Secret.Criticality != "high" {
		t.Fatalf("expected derived criticality high, got %q", impact.Secret.Criticality)
	}

	if impact.Secret.AffectedWorkloadCount != 1 {
		t.Fatalf("expected one affected workload, got %d", impact.Secret.AffectedWorkloadCount)
	}

	if impact.Secret.AffectedTeamCount != 1 {
		t.Fatalf("expected one affected team, got %d", impact.Secret.AffectedTeamCount)
	}

	if impact.Secret.AffectedNamespaceCount != 1 {
		t.Fatalf("expected one affected namespace, got %d", impact.Secret.AffectedNamespaceCount)
	}

	if impact.Secret.ImpactScore != 24 {
		t.Fatalf("expected impact score 24, got %v", impact.Secret.ImpactScore)
	}
}

func TestApplySnapshotTracksUnresolvedSecretReference(t *testing.T) {
	cache := NewCache("minikube")
	now := time.Unix(1000, 0)

	workload := WorkloadRef{
		Cluster:    "minikube",
		Namespace:  "kbeacon-demo",
		APIVersion: "apps/v1",
		Kind:       "Deployment",
		Name:       "api",
	}

	missingSecret := SecretRef{
		Cluster:   "minikube",
		Namespace: "kbeacon-demo",
		Name:      "missing-secret",
	}

	cache.ApplySnapshot(
		nil,
		[]WorkloadInput{
			{
				Ref:           workload,
				OwnerTeam:     "platform",
				Criticality:   "medium",
				DiscoveryMode: DiscoveryModeHybrid,
				Edges: []DependencyEdge{
					{
						Workload:      workload,
						Secret:        missingSecret,
						DiscoveryMode: DiscoveryModeInfer,
						Sources: []DependencySource{
							{
								Type: "envFrom.secretRef",
								Path: "envFrom.secretRef[missing-secret]",
							},
						},
					},
				},
			},
		},
		now,
	)

	impact, ok := cache.GetSecretImpact("kbeacon-demo", "missing-secret")
	if !ok {
		t.Fatal("expected unresolved secret impact to be present")
	}

	if impact.Secret.Exists {
		t.Fatal("expected unresolved secret to have exists=false")
	}

	if impact.Secret.UnresolvedReferenceCount != 1 {
		t.Fatalf("expected unresolved count 1, got %d", impact.Secret.UnresolvedReferenceCount)
	}
}
