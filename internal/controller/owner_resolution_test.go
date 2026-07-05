package controller

import (
	"testing"

	"github.com/memoliyasti/kbeacon/internal/graph"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	kcache "k8s.io/client-go/tools/cache"
)

func TestControllerRebuildSkipsReplicaSetOwnedPodWhenDeploymentResolved(t *testing.T) {
	graphCache := graph.NewCache("test-cluster")
	ctrl := New(fake.NewSimpleClientset(), graphCache, Options{
		Cluster: "test-cluster",
		Resources: ResourceConfig{
			Pods:        true,
			Deployments: true,
			ReplicaSets: true,
		},
		ResourcesSet: true,
	})

	deployment := ownerResolutionTestDeployment("payments", "api", "api-secret")
	replicaSet := ownerResolutionTestReplicaSet("payments", "api-756d4d9f96", "api")
	pod := ownerResolutionTestPod("payments", "api-756d4d9f96-kc2zd", "api-756d4d9f96", "api-secret")

	addOwnerResolutionObjectToInformerStore(t, ctrl.deploymentInformer.Informer(), deployment)
	addOwnerResolutionObjectToInformerStore(t, ctrl.replicaSetInformer.Informer(), replicaSet)
	addOwnerResolutionObjectToInformerStore(t, ctrl.podInformer.Informer(), pod)
	markOwnerResolutionSyncedForTest(ctrl)

	ctrl.rebuild("test")

	snapshot := graphCache.Snapshot()
	if !ownerResolutionHasWorkload(snapshot.Workloads, "Deployment", "api") {
		t.Fatalf("expected Deployment workload, got %#v", snapshot.Workloads)
	}
	if ownerResolutionHasWorkload(snapshot.Workloads, "Pod", "api-756d4d9f96-kc2zd") {
		t.Fatalf("did not expect ReplicaSet-owned Pod when Deployment owner is resolved, got %#v", snapshot.Workloads)
	}
}

func TestControllerRebuildKeepsReplicaSetOwnedPodWhenReplicaSetUnresolved(t *testing.T) {
	graphCache := graph.NewCache("test-cluster")
	ctrl := New(fake.NewSimpleClientset(), graphCache, Options{
		Cluster: "test-cluster",
		Resources: ResourceConfig{
			Pods:        true,
			Deployments: true,
			ReplicaSets: false,
		},
		ResourcesSet: true,
	})

	deployment := ownerResolutionTestDeployment("payments", "api", "api-secret")
	pod := ownerResolutionTestPod("payments", "api-756d4d9f96-kc2zd", "api-756d4d9f96", "api-secret")

	addOwnerResolutionObjectToInformerStore(t, ctrl.deploymentInformer.Informer(), deployment)
	addOwnerResolutionObjectToInformerStore(t, ctrl.podInformer.Informer(), pod)
	markOwnerResolutionSyncedForTest(ctrl)

	ctrl.rebuild("test")

	snapshot := graphCache.Snapshot()
	if !ownerResolutionHasWorkload(snapshot.Workloads, "Deployment", "api") {
		t.Fatalf("expected Deployment workload, got %#v", snapshot.Workloads)
	}
	if !ownerResolutionHasWorkload(snapshot.Workloads, "Pod", "api-756d4d9f96-kc2zd") {
		t.Fatalf("expected Pod fallback when ReplicaSet owner is unresolved, got %#v", snapshot.Workloads)
	}
}

func TestControllerRebuildSkipsCronJobOwnedJobWhenCronJobResolved(t *testing.T) {
	graphCache := graph.NewCache("test-cluster")
	ctrl := New(fake.NewSimpleClientset(), graphCache, Options{
		Cluster: "test-cluster",
		Resources: ResourceConfig{
			Jobs:     true,
			CronJobs: true,
		},
		ResourcesSet: true,
	})

	cronJob := ownerResolutionTestCronJob("payments", "nightly")
	job := ownerResolutionTestJob("payments", "nightly-28700000", "nightly")

	addOwnerResolutionObjectToInformerStore(t, ctrl.cronJobInformer.Informer(), cronJob)
	addOwnerResolutionObjectToInformerStore(t, ctrl.jobInformer.Informer(), job)
	markOwnerResolutionSyncedForTest(ctrl)

	ctrl.rebuild("test")

	snapshot := graphCache.Snapshot()
	if !ownerResolutionHasWorkload(snapshot.Workloads, "CronJob", "nightly") {
		t.Fatalf("expected CronJob workload, got %#v", snapshot.Workloads)
	}
	if ownerResolutionHasWorkload(snapshot.Workloads, "Job", "nightly-28700000") {
		t.Fatalf("did not expect CronJob-owned Job when CronJob owner is resolved, got %#v", snapshot.Workloads)
	}
}

func addOwnerResolutionObjectToInformerStore(t *testing.T, informer kcache.SharedIndexInformer, obj any) {
	t.Helper()
	if err := informer.GetStore().Add(obj); err != nil {
		t.Fatalf("add object to informer store: %v", err)
	}
}

func markOwnerResolutionSyncedForTest(ctrl *Controller) {
	for name := range ctrl.synced {
		ctrl.synced[name] = true
	}
}

func ownerResolutionHasWorkload(workloads []graph.WorkloadSummary, kind, name string) bool {
	for _, workload := range workloads {
		if workload.Ref.Kind == kind && workload.Ref.Name == name {
			return true
		}
	}
	return false
}

func ownerResolutionTestDeployment(namespace, name, secretName string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			UID:       types.UID(name + "-uid"),
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: ownerResolutionTestPodSpec(secretName),
			},
		},
	}
}

func ownerResolutionTestReplicaSet(namespace, name, deploymentName string) *appsv1.ReplicaSet {
	return &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Namespace:       namespace,
			UID:             types.UID(name + "-uid"),
			OwnerReferences: []metav1.OwnerReference{ownerResolutionTestOwnerReference("apps/v1", "Deployment", deploymentName, deploymentName+"-uid")},
		},
	}
}

func ownerResolutionTestPod(namespace, name, replicaSetName, secretName string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Namespace:       namespace,
			UID:             types.UID(name + "-uid"),
			OwnerReferences: []metav1.OwnerReference{ownerResolutionTestOwnerReference("apps/v1", "ReplicaSet", replicaSetName, replicaSetName+"-uid")},
		},
		Spec: ownerResolutionTestPodSpec(secretName),
	}
}

func ownerResolutionTestCronJob(namespace, name string) *batchv1.CronJob {
	return &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			UID:       types.UID(name + "-uid"),
		},
		Spec: batchv1.CronJobSpec{
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: ownerResolutionTestPodSpec("nightly-secret"),
					},
				},
			},
		},
	}
}

func ownerResolutionTestJob(namespace, name, cronJobName string) *batchv1.Job {
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Namespace:       namespace,
			UID:             types.UID(name + "-uid"),
			OwnerReferences: []metav1.OwnerReference{ownerResolutionTestOwnerReference("batch/v1", "CronJob", cronJobName, cronJobName+"-uid")},
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: ownerResolutionTestPodSpec("nightly-secret"),
			},
		},
	}
}

func ownerResolutionTestPodSpec(secretName string) corev1.PodSpec {
	return corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name: "app",
				Env: []corev1.EnvVar{
					{
						Name: "SECRET_VALUE",
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
								Key:                  "value",
							},
						},
					},
				},
			},
		},
	}
}

func ownerResolutionTestOwnerReference(apiVersion, kind, name, uid string) metav1.OwnerReference {
	controller := true
	return metav1.OwnerReference{
		APIVersion: apiVersion,
		Kind:       kind,
		Name:       name,
		UID:        types.UID(uid),
		Controller: &controller,
	}
}
