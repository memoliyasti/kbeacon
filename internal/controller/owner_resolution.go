package controller

import (
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type workloadPresence map[string]struct{}

type controllerOwnerIndex struct {
	deployments  workloadPresence
	statefulSets workloadPresence
	daemonSets   workloadPresence
	cronJobs     workloadPresence
	jobWorkloads workloadPresence
	jobs         map[string]*batchv1.Job
	replicaSets  map[string]*appsv1.ReplicaSet
}

func newControllerOwnerIndex() controllerOwnerIndex {
	return controllerOwnerIndex{
		deployments:  workloadPresence{},
		statefulSets: workloadPresence{},
		daemonSets:   workloadPresence{},
		cronJobs:     workloadPresence{},
		jobWorkloads: workloadPresence{},
		jobs:         map[string]*batchv1.Job{},
		replicaSets:  map[string]*appsv1.ReplicaSet{},
	}
}

func objectKey(namespace, name string) string {
	return namespace + "/" + name
}

func (i controllerOwnerIndex) addDeployment(deployment *appsv1.Deployment) {
	if deployment == nil {
		return
	}
	i.deployments[objectKey(deployment.Namespace, deployment.Name)] = struct{}{}
}

func (i controllerOwnerIndex) addStatefulSet(statefulSet *appsv1.StatefulSet) {
	if statefulSet == nil {
		return
	}
	i.statefulSets[objectKey(statefulSet.Namespace, statefulSet.Name)] = struct{}{}
}

func (i controllerOwnerIndex) addDaemonSet(daemonSet *appsv1.DaemonSet) {
	if daemonSet == nil {
		return
	}
	i.daemonSets[objectKey(daemonSet.Namespace, daemonSet.Name)] = struct{}{}
}

func (i controllerOwnerIndex) addCronJob(cronJob *batchv1.CronJob) {
	if cronJob == nil {
		return
	}
	i.cronJobs[objectKey(cronJob.Namespace, cronJob.Name)] = struct{}{}
}

func (i controllerOwnerIndex) addJob(job *batchv1.Job, asWorkload bool) {
	if job == nil {
		return
	}

	key := objectKey(job.Namespace, job.Name)
	i.jobs[key] = job

	if asWorkload {
		i.jobWorkloads[key] = struct{}{}
	}
}

func (i controllerOwnerIndex) addReplicaSet(replicaSet *appsv1.ReplicaSet) {
	if replicaSet == nil {
		return
	}
	i.replicaSets[objectKey(replicaSet.Namespace, replicaSet.Name)] = replicaSet
}

func (i controllerOwnerIndex) hasDeployment(namespace, name string) bool {
	_, ok := i.deployments[objectKey(namespace, name)]
	return ok
}

func (i controllerOwnerIndex) hasStatefulSet(namespace, name string) bool {
	_, ok := i.statefulSets[objectKey(namespace, name)]
	return ok
}

func (i controllerOwnerIndex) hasDaemonSet(namespace, name string) bool {
	_, ok := i.daemonSets[objectKey(namespace, name)]
	return ok
}

func (i controllerOwnerIndex) hasCronJob(namespace, name string) bool {
	_, ok := i.cronJobs[objectKey(namespace, name)]
	return ok
}

func (i controllerOwnerIndex) hasJobWorkload(namespace, name string) bool {
	_, ok := i.jobWorkloads[objectKey(namespace, name)]
	return ok
}

func controllerOwnerReference(refs []metav1.OwnerReference) (metav1.OwnerReference, bool) {
	for _, ref := range refs {
		if ref.Controller == nil || !*ref.Controller {
			continue
		}
		if ref.Kind == "" || ref.Name == "" {
			continue
		}
		return ref, true
	}

	return metav1.OwnerReference{}, false
}

func shouldIncludeJobWorkload(job *batchv1.Job, owners controllerOwnerIndex) bool {
	owner, ok := controllerOwnerReference(job.OwnerReferences)
	if !ok {
		return true
	}

	if owner.Kind == "CronJob" && owners.hasCronJob(job.Namespace, owner.Name) {
		return false
	}

	return true
}

func shouldIncludePodWorkload(pod *corev1.Pod, owners controllerOwnerIndex) bool {
	return !podControllerResolvedToWorkload(pod, owners)
}

func podControllerResolvedToWorkload(pod *corev1.Pod, owners controllerOwnerIndex) bool {
	owner, ok := controllerOwnerReference(pod.OwnerReferences)
	if !ok {
		return false
	}

	switch owner.Kind {
	case "Deployment":
		return owners.hasDeployment(pod.Namespace, owner.Name)
	case "ReplicaSet":
		replicaSet := owners.replicaSets[objectKey(pod.Namespace, owner.Name)]
		if replicaSet == nil {
			return false
		}

		replicaSetOwner, ok := controllerOwnerReference(replicaSet.OwnerReferences)
		if !ok {
			return false
		}

		if replicaSetOwner.Kind == "Deployment" {
			return owners.hasDeployment(replicaSet.Namespace, replicaSetOwner.Name)
		}

		return false
	case "StatefulSet":
		return owners.hasStatefulSet(pod.Namespace, owner.Name)
	case "DaemonSet":
		return owners.hasDaemonSet(pod.Namespace, owner.Name)
	case "Job":
		if owners.hasJobWorkload(pod.Namespace, owner.Name) {
			return true
		}

		job := owners.jobs[objectKey(pod.Namespace, owner.Name)]
		if job == nil {
			return false
		}

		jobOwner, ok := controllerOwnerReference(job.OwnerReferences)
		if !ok {
			return false
		}

		return jobOwner.Kind == "CronJob" && owners.hasCronJob(job.Namespace, jobOwner.Name)
	case "CronJob":
		return owners.hasCronJob(pod.Namespace, owner.Name)
	default:
		return false
	}
}
