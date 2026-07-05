package controller

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/memoliyasti/kbeacon/internal/discovery"
	"github.com/memoliyasti/kbeacon/internal/graph"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers"
	appsinformers "k8s.io/client-go/informers/apps/v1"
	batchinformers "k8s.io/client-go/informers/batch/v1"
	coreinformers "k8s.io/client-go/informers/core/v1"
	networkinginformers "k8s.io/client-go/informers/networking/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type Recorder interface {
	ObserveWatchEvent(resource, event string)
	ObserveGraphUpdate(reason string, duration time.Duration)
}

type ResourceConfig struct {
	Secrets               bool
	ServiceAccounts       bool
	Pods                  bool
	Deployments           bool
	ReplicaSets           bool
	StatefulSets          bool
	DaemonSets            bool
	Ingresses             bool
	Jobs                  bool
	CronJobs              bool
	Certificates          bool
	ExternalSecrets       bool
	SecretProviderClasses bool
	KafkaConnectors       bool
	ConfluentConnectors   bool
}

func DefaultResourceConfig() ResourceConfig {
	return ResourceConfig{
		Secrets:         true,
		ServiceAccounts: true,
		Pods:            true,
		Deployments:     true,
		ReplicaSets:     true,
		StatefulSets:    true,
		DaemonSets:      true,
		Ingresses:       true,
		Jobs:            true,
		CronJobs:        true,
	}
}

func (r ResourceConfig) enabled(resource string) bool {
	switch resource {
	case "Secret":
		return r.Secrets
	case "ServiceAccount":
		return r.ServiceAccounts
	case "Pod":
		return r.Pods
	case "Deployment":
		return r.Deployments
	case "ReplicaSet":
		return r.ReplicaSets
	case "StatefulSet":
		return r.StatefulSets
	case "DaemonSet":
		return r.DaemonSets
	case "Ingress":
		return r.Ingresses
	case "Job":
		return r.Jobs
	case "CronJob":
		return r.CronJobs
	case "Certificate":
		return r.Certificates
	case "ExternalSecret":
		return r.ExternalSecrets
	case "SecretProviderClass":
		return r.SecretProviderClasses
	case "KafkaConnector":
		return r.KafkaConnectors
	case "Connector":
		return r.ConfluentConnectors
	default:
		return false
	}
}

type Options struct {
	Cluster           string
	Resync            time.Duration
	RebuildDebounce   time.Duration
	IncludeNamespaces []string
	ExcludeNamespaces []string
	Resources         ResourceConfig
	ResourcesSet      bool
	Recorder          Recorder
	DynamicClient     dynamic.Interface
	Discovery         discovery.Options
	Logger            *slog.Logger
}

type NamespaceFilter struct {
	include map[string]struct{}
	exclude map[string]struct{}
}

type Controller struct {
	client kubernetes.Interface
	graph  *graph.Cache
	logger *slog.Logger

	cluster        string
	resync         time.Duration
	watchNamespace string

	resources        ResourceConfig
	recorder         Recorder
	discoveryOptions discovery.Options
	namespaceFilter  NamespaceFilter
	factory          informers.SharedInformerFactory
	dynamicClient    dynamic.Interface
	dynamicFactory   dynamicinformer.DynamicSharedInformerFactory

	secretInformer              coreinformers.SecretInformer
	serviceAccountInformer      coreinformers.ServiceAccountInformer
	podInformer                 coreinformers.PodInformer
	deploymentInformer          appsinformers.DeploymentInformer
	replicaSetInformer          appsinformers.ReplicaSetInformer
	statefulSetInformer         appsinformers.StatefulSetInformer
	daemonSetInformer           appsinformers.DaemonSetInformer
	ingressInformer             networkinginformers.IngressInformer
	jobInformer                 batchinformers.JobInformer
	cronJobInformer             batchinformers.CronJobInformer
	certificateInformer         cache.SharedIndexInformer
	externalSecretInformer      cache.SharedIndexInformer
	secretProviderClassInformer cache.SharedIndexInformer
	kafkaConnectorInformer      cache.SharedIndexInformer
	confluentConnectorInformer  cache.SharedIndexInformer

	rebuildDebounce time.Duration
	rebuildCh       chan string

	mu                sync.RWMutex
	synced            map[string]bool
	lastRebuildAt     time.Time
	lastRebuildReason string
	lastGraphCounts   map[string]int
}

func NewNamespaceFilter(include, exclude []string) NamespaceFilter {
	return NamespaceFilter{
		include: stringSet(include),
		exclude: stringSet(exclude),
	}
}

func (f NamespaceFilter) Include(namespace string) bool {
	if namespace == "" {
		return false
	}

	if len(f.include) > 0 {
		if _, ok := f.include[namespace]; !ok {
			return false
		}
	}

	if _, ok := f.exclude[namespace]; ok {
		return false
	}

	return true
}

func newInformerFactory(client kubernetes.Interface, resync time.Duration, includeNamespaces []string, filter NamespaceFilter) (informers.SharedInformerFactory, string) {
	if len(includeNamespaces) == 1 {
		namespace := strings.TrimSpace(includeNamespaces[0])
		if filter.Include(namespace) {
			return informers.NewSharedInformerFactoryWithOptions(
				client,
				resync,
				informers.WithNamespace(namespace),
			), namespace
		}
	}

	return informers.NewSharedInformerFactory(client, resync), ""
}

var certManagerCertificateGVR = schema.GroupVersionResource{
	Group:    "cert-manager.io",
	Version:  "v1",
	Resource: "certificates",
}

var externalSecretsExternalSecretGVR = schema.GroupVersionResource{
	Group:    "external-secrets.io",
	Version:  "v1",
	Resource: "externalsecrets",
}

var secretsStoreSecretProviderClassGVR = schema.GroupVersionResource{
	Group:    "secrets-store.csi.x-k8s.io",
	Version:  "v1",
	Resource: "secretproviderclasses",
}

var strimziKafkaConnectorGVR = schema.GroupVersionResource{
	Group:    "kafka.strimzi.io",
	Version:  "v1",
	Resource: "kafkaconnectors",
}

var confluentConnectorGVR = schema.GroupVersionResource{
	Group:    "platform.confluent.io",
	Version:  "v1beta1",
	Resource: "connectors",
}

func New(client kubernetes.Interface, graphCache *graph.Cache, options Options) *Controller {
	if options.Resync == 0 {
		options.Resync = 10 * time.Hour
	}
	if options.RebuildDebounce == 0 {
		options.RebuildDebounce = 250 * time.Millisecond
	}
	if options.Logger == nil {
		options.Logger = slog.Default()
	}
	if options.Discovery.Cluster == "" {
		options.Discovery = discovery.DefaultOptions(options.Cluster)
	}

	resources := options.Resources
	if !options.ResourcesSet {
		resources = DefaultResourceConfig()
	}

	namespaceFilter := NewNamespaceFilter(options.IncludeNamespaces, options.ExcludeNamespaces)
	factory, watchNamespace := newInformerFactory(client, options.Resync, options.IncludeNamespaces, namespaceFilter)

	c := &Controller{
		client:           client,
		graph:            graphCache,
		logger:           options.Logger,
		cluster:          options.Cluster,
		resync:           options.Resync,
		resources:        resources,
		recorder:         options.Recorder,
		discoveryOptions: options.Discovery,
		namespaceFilter:  namespaceFilter,
		factory:          factory,
		dynamicClient:    options.DynamicClient,
		watchNamespace:   watchNamespace,
		rebuildDebounce:  options.RebuildDebounce,
		rebuildCh:        make(chan string, 1),
		lastGraphCounts:  map[string]int{},
		synced:           map[string]bool{},
	}

	if resources.Secrets {
		c.secretInformer = factory.Core().V1().Secrets()
		c.synced["Secret"] = false
	}
	if resources.ServiceAccounts {
		c.serviceAccountInformer = factory.Core().V1().ServiceAccounts()
		c.synced["ServiceAccount"] = false
	}
	if resources.Pods {
		c.podInformer = factory.Core().V1().Pods()
		c.synced["Pod"] = false
	}
	if resources.Deployments {
		c.deploymentInformer = factory.Apps().V1().Deployments()
		c.synced["Deployment"] = false
	}
	if resources.ReplicaSets {
		c.replicaSetInformer = factory.Apps().V1().ReplicaSets()
		c.synced["ReplicaSet"] = false
	}
	if resources.StatefulSets {
		c.statefulSetInformer = factory.Apps().V1().StatefulSets()
		c.synced["StatefulSet"] = false
	}
	if resources.DaemonSets {
		c.daemonSetInformer = factory.Apps().V1().DaemonSets()
		c.synced["DaemonSet"] = false
	}
	if resources.Ingresses {
		c.ingressInformer = factory.Networking().V1().Ingresses()
		c.synced["Ingress"] = false
	}
	if resources.Jobs {
		c.jobInformer = factory.Batch().V1().Jobs()
		c.synced["Job"] = false
	}
	if resources.CronJobs {
		c.cronJobInformer = factory.Batch().V1().CronJobs()
		c.synced["CronJob"] = false
	}
	if options.DynamicClient != nil && (resources.Certificates || resources.ExternalSecrets || resources.SecretProviderClasses || resources.KafkaConnectors || resources.ConfluentConnectors) {
		c.dynamicFactory = dynamicinformer.NewFilteredDynamicSharedInformerFactory(
			options.DynamicClient,
			options.Resync,
			watchNamespace,
			nil,
		)

		if resources.Certificates {
			c.certificateInformer = c.dynamicFactory.ForResource(certManagerCertificateGVR).Informer()
			c.synced["Certificate"] = false
		}

		if resources.ExternalSecrets {
			c.externalSecretInformer = c.dynamicFactory.ForResource(externalSecretsExternalSecretGVR).Informer()
			c.synced["ExternalSecret"] = false
		}

		if resources.SecretProviderClasses {
			c.secretProviderClassInformer = c.dynamicFactory.ForResource(secretsStoreSecretProviderClassGVR).Informer()
			c.synced["SecretProviderClass"] = false
		}

		if resources.KafkaConnectors {
			c.kafkaConnectorInformer = c.dynamicFactory.ForResource(strimziKafkaConnectorGVR).Informer()
			c.synced["KafkaConnector"] = false
		}

		if resources.ConfluentConnectors {
			c.confluentConnectorInformer = c.dynamicFactory.ForResource(confluentConnectorGVR).Informer()
			c.synced["Connector"] = false
		}
	}

	c.registerHandlers()
	return c
}

func (c *Controller) Start(ctx context.Context) error {
	c.logger.Info(
		"starting kubernetes informers",
		"cluster", c.cluster,
		"resync", c.resync.String(),
		"rebuildDebounce", c.rebuildDebounce.String(),
	)

	go c.runRebuildLoop(ctx)

	c.factory.Start(ctx.Done())
	if c.dynamicFactory != nil {
		c.dynamicFactory.Start(ctx.Done())
	}

	syncs := c.enabledSyncs()

	for _, item := range syncs {
		ok := cache.WaitForCacheSync(ctx.Done(), item.synced)
		c.setSynced(item.name, ok)
		if !ok {
			return fmt.Errorf("cache sync failed for %s", item.name)
		}
		c.logger.Info("cache synced", "resource", item.name)
	}

	c.requestRebuild("initial-sync")

	<-ctx.Done()
	return nil
}

func (c *Controller) Status() ([]map[string]any, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	out := make([]map[string]any, 0, len(resourceOrder()))
	all := true

	for _, name := range resourceOrder() {
		if !c.resources.enabled(name) {
			out = append(out, map[string]any{
				"resource": name,
				"synced":   true,
				"optional": true,
				"reason":   "disabled",
			})
			continue
		}

		synced := c.synced[name]
		if !synced {
			all = false
		}

		out = append(out, map[string]any{
			"resource": name,
			"synced":   synced,
		})
	}

	return out, all
}

func (c *Controller) CacheSyncStatus() map[string]bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	out := make(map[string]bool, len(c.synced))
	for _, name := range resourceOrder() {
		if !c.resources.enabled(name) {
			continue
		}
		out[name] = c.synced[name]
	}

	return out
}

func (c *Controller) LastGraphCounts() map[string]int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	out := make(map[string]int, len(c.lastGraphCounts))
	for k, v := range c.lastGraphCounts {
		out[k] = v
	}
	return out
}

func (c *Controller) Ready() bool {
	_, ready := c.Status()
	return ready
}

func (c *Controller) registerHandlers() {
	if c.secretInformer != nil {
		c.registerResourceHandler("Secret", c.secretInformer.Informer())
	}
	if c.serviceAccountInformer != nil {
		c.registerResourceHandler("ServiceAccount", c.serviceAccountInformer.Informer())
	}
	if c.podInformer != nil {
		c.registerResourceHandler("Pod", c.podInformer.Informer())
	}
	if c.deploymentInformer != nil {
		c.registerResourceHandler("Deployment", c.deploymentInformer.Informer())
	}
	if c.replicaSetInformer != nil {
		c.registerResourceHandler("ReplicaSet", c.replicaSetInformer.Informer())
	}
	if c.statefulSetInformer != nil {
		c.registerResourceHandler("StatefulSet", c.statefulSetInformer.Informer())
	}
	if c.daemonSetInformer != nil {
		c.registerResourceHandler("DaemonSet", c.daemonSetInformer.Informer())
	}
	if c.ingressInformer != nil {
		c.registerResourceHandler("Ingress", c.ingressInformer.Informer())
	}
	if c.jobInformer != nil {
		c.registerResourceHandler("Job", c.jobInformer.Informer())
	}
	if c.cronJobInformer != nil {
		c.registerResourceHandler("CronJob", c.cronJobInformer.Informer())
	}
	if c.certificateInformer != nil {
		c.registerResourceHandler("Certificate", c.certificateInformer)
	}
	if c.externalSecretInformer != nil {
		c.registerResourceHandler("ExternalSecret", c.externalSecretInformer)
	}
	if c.secretProviderClassInformer != nil {
		c.registerResourceHandler("SecretProviderClass", c.secretProviderClassInformer)
	}
	if c.kafkaConnectorInformer != nil {
		c.registerResourceHandler("KafkaConnector", c.kafkaConnectorInformer)
	}
	if c.confluentConnectorInformer != nil {
		c.registerResourceHandler("Connector", c.confluentConnectorInformer)
	}
}

func (c *Controller) registerResourceHandler(resource string, informer cache.SharedIndexInformer) {
	handler := cache.ResourceEventHandlerFuncs{
		AddFunc: func(any) {
			c.observeWatchEvent(resource, "add")
			c.requestRebuild("add")
		},
		UpdateFunc: func(any, any) {
			c.observeWatchEvent(resource, "update")
			c.requestRebuild("update")
		},
		DeleteFunc: func(any) {
			c.observeWatchEvent(resource, "delete")
			c.requestRebuild("delete")
		},
	}

	informer.AddEventHandler(handler)
}

func (c *Controller) observeWatchEvent(resource, event string) {
	if c.recorder == nil {
		return
	}
	c.recorder.ObserveWatchEvent(resource, event)
}

func (c *Controller) requestRebuild(reason string) {
	select {
	case c.rebuildCh <- reason:
	default:
	}
}

func (c *Controller) runRebuildLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case reason := <-c.rebuildCh:
			timer := time.NewTimer(c.rebuildDebounce)
			latestReason := reason

		Drain:
			for {
				select {
				case <-ctx.Done():
					timer.Stop()
					return
				case latestReason = <-c.rebuildCh:
					continue
				case <-timer.C:
					break Drain
				}
			}

			c.rebuild(latestReason)
		}
	}
}

func (c *Controller) rebuild(reason string) {
	if !c.Ready() {
		return
	}

	start := time.Now()

	secrets := []graph.SecretInput{}
	workloads := []graph.WorkloadInput{}
	serviceAccountPullSecrets := map[string][]string{}
	ownerIndex := newControllerOwnerIndex()

	if c.secretInformer != nil {
		secretObjects, err := c.secretInformer.Lister().List(labels.Everything())
		if err != nil {
			c.logger.Error("list secrets failed", "error", err)
			return
		}

		secrets = make([]graph.SecretInput, 0, len(secretObjects))
		for _, secret := range secretObjects {
			if !c.namespaceFilter.Include(secret.Namespace) {
				continue
			}
			secrets = append(secrets, discovery.SecretInputFromSecret(c.cluster, secret))
		}
	}

	if c.serviceAccountInformer != nil {
		serviceAccounts, err := c.serviceAccountInformer.Lister().List(labels.Everything())
		if err != nil {
			c.logger.Error("list serviceaccounts failed", "error", err)
			return
		}

		for _, serviceAccount := range serviceAccounts {
			if !c.namespaceFilter.Include(serviceAccount.Namespace) {
				continue
			}

			names := make([]string, 0, len(serviceAccount.ImagePullSecrets))
			for _, ref := range serviceAccount.ImagePullSecrets {
				if ref.Name == "" {
					continue
				}
				names = append(names, ref.Name)
			}

			if len(names) > 0 {
				serviceAccountPullSecrets[discovery.ServiceAccountKey(serviceAccount.Namespace, serviceAccount.Name)] = names
			}
		}
	}

	discoveryOptions := c.discoveryOptions
	discoveryOptions.ServiceAccountImagePullSecrets = serviceAccountPullSecrets

	if c.deploymentInformer != nil {
		deployments, err := c.deploymentInformer.Lister().List(labels.Everything())
		if err != nil {
			c.logger.Error("list deployments failed", "error", err)
			return
		}

		for _, deployment := range deployments {
			if !c.namespaceFilter.Include(deployment.Namespace) {
				continue
			}
			ownerIndex.addDeployment(deployment)
			workloads = append(workloads, discovery.WorkloadFromDeployment(discoveryOptions, deployment))
		}
	}

	if c.replicaSetInformer != nil {
		replicaSets, err := c.replicaSetInformer.Lister().List(labels.Everything())
		if err != nil {
			c.logger.Error("list replicasets failed", "error", err)
			return
		}

		for _, replicaSet := range replicaSets {
			if !c.namespaceFilter.Include(replicaSet.Namespace) {
				continue
			}
			ownerIndex.addReplicaSet(replicaSet)
		}
	}

	if c.statefulSetInformer != nil {
		statefulSets, err := c.statefulSetInformer.Lister().List(labels.Everything())
		if err != nil {
			c.logger.Error("list statefulsets failed", "error", err)
			return
		}

		for _, statefulSet := range statefulSets {
			if !c.namespaceFilter.Include(statefulSet.Namespace) {
				continue
			}
			ownerIndex.addStatefulSet(statefulSet)
			workloads = append(workloads, discovery.WorkloadFromStatefulSet(discoveryOptions, statefulSet))
		}
	}

	if c.daemonSetInformer != nil {
		daemonSets, err := c.daemonSetInformer.Lister().List(labels.Everything())
		if err != nil {
			c.logger.Error("list daemonsets failed", "error", err)
			return
		}

		for _, daemonSet := range daemonSets {
			if !c.namespaceFilter.Include(daemonSet.Namespace) {
				continue
			}
			ownerIndex.addDaemonSet(daemonSet)
			workloads = append(workloads, discovery.WorkloadFromDaemonSet(discoveryOptions, daemonSet))
		}
	}

	if c.ingressInformer != nil {
		ingresses, err := c.ingressInformer.Lister().List(labels.Everything())
		if err != nil {
			c.logger.Error("list ingresses failed", "error", err)
			return
		}

		for _, ingress := range ingresses {
			if !c.namespaceFilter.Include(ingress.Namespace) {
				continue
			}
			workloads = append(workloads, discovery.WorkloadFromIngress(discoveryOptions, ingress))
		}
	}

	if c.cronJobInformer != nil {
		cronJobs, err := c.cronJobInformer.Lister().List(labels.Everything())
		if err != nil {
			c.logger.Error("list cronjobs failed", "error", err)
			return
		}

		for _, cronJob := range cronJobs {
			if !c.namespaceFilter.Include(cronJob.Namespace) {
				continue
			}
			ownerIndex.addCronJob(cronJob)
			workloads = append(workloads, discovery.WorkloadFromCronJob(discoveryOptions, cronJob))
		}
	}

	if c.jobInformer != nil {
		jobs, err := c.jobInformer.Lister().List(labels.Everything())
		if err != nil {
			c.logger.Error("list jobs failed", "error", err)
			return
		}

		for _, job := range jobs {
			if !c.namespaceFilter.Include(job.Namespace) {
				continue
			}

			includeJob := shouldIncludeJobWorkload(job, ownerIndex)
			ownerIndex.addJob(job, includeJob)

			if !includeJob {
				continue
			}
			workloads = append(workloads, discovery.WorkloadFromJob(discoveryOptions, job))
		}
	}

	if c.podInformer != nil {
		pods, err := c.podInformer.Lister().List(labels.Everything())
		if err != nil {
			c.logger.Error("list pods failed", "error", err)
			return
		}

		for _, pod := range pods {
			if !c.namespaceFilter.Include(pod.Namespace) {
				continue
			}
			if !shouldIncludePodWorkload(pod, ownerIndex) {
				continue
			}
			workloads = append(workloads, discovery.WorkloadFromPod(discoveryOptions, pod))
		}
	}

	if c.certificateInformer != nil {
		for _, item := range c.certificateInformer.GetStore().List() {
			certificate, ok := item.(*unstructured.Unstructured)
			if !ok {
				continue
			}
			if !c.namespaceFilter.Include(certificate.GetNamespace()) {
				continue
			}
			workloads = append(workloads, discovery.WorkloadFromCertificate(discoveryOptions, certificate))
		}
	}

	if c.externalSecretInformer != nil {
		for _, item := range c.externalSecretInformer.GetStore().List() {
			externalSecret, ok := item.(*unstructured.Unstructured)
			if !ok {
				continue
			}
			if !c.namespaceFilter.Include(externalSecret.GetNamespace()) {
				continue
			}
			workloads = append(workloads, discovery.WorkloadFromExternalSecret(discoveryOptions, externalSecret))
		}
	}

	if c.secretProviderClassInformer != nil {
		for _, item := range c.secretProviderClassInformer.GetStore().List() {
			secretProviderClass, ok := item.(*unstructured.Unstructured)
			if !ok {
				continue
			}
			if !c.namespaceFilter.Include(secretProviderClass.GetNamespace()) {
				continue
			}
			workloads = append(workloads, discovery.WorkloadFromSecretProviderClass(discoveryOptions, secretProviderClass))
		}
	}

	if c.kafkaConnectorInformer != nil {
		for _, item := range c.kafkaConnectorInformer.GetStore().List() {
			kafkaConnector, ok := item.(*unstructured.Unstructured)
			if !ok {
				continue
			}
			if !c.namespaceFilter.Include(kafkaConnector.GetNamespace()) {
				continue
			}
			workloads = append(workloads, discovery.WorkloadFromStrimziKafkaConnector(discoveryOptions, kafkaConnector))
		}
	}

	if c.confluentConnectorInformer != nil {
		for _, item := range c.confluentConnectorInformer.GetStore().List() {
			connector, ok := item.(*unstructured.Unstructured)
			if !ok {
				continue
			}
			if !c.namespaceFilter.Include(connector.GetNamespace()) {
				continue
			}
			workloads = append(workloads, discovery.WorkloadFromConfluentConnector(discoveryOptions, connector))
		}
	}

	c.graph.ApplySnapshot(secrets, workloads, time.Now())

	duration := time.Since(start)
	if c.recorder != nil {
		c.recorder.ObserveGraphUpdate(reason, duration)
	}

	snapshot := c.graph.Snapshot()
	c.setRebuildStats(reason, map[string]int{
		"Secret":         len(secrets),
		"Workload":       len(workloads),
		"DependencyEdge": len(snapshot.Edges),
	})

	c.logger.Debug(
		"graph rebuilt",
		"reason", reason,
		"secrets", len(secrets),
		"workloads", len(workloads),
		"edges", len(snapshot.Edges),
		"duration", duration.String(),
	)
}

func (c *Controller) setSynced(resource string, synced bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.synced[resource] = synced
}

func (c *Controller) setRebuildStats(reason string, counts map[string]int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.lastRebuildAt = time.Now()
	c.lastRebuildReason = reason
	c.lastGraphCounts = counts
}

func (c *Controller) enabledSyncs() []struct {
	name   string
	synced cache.InformerSynced
} {
	syncs := []struct {
		name   string
		synced cache.InformerSynced
	}{}

	if c.secretInformer != nil {
		syncs = append(syncs, struct {
			name   string
			synced cache.InformerSynced
		}{name: "Secret", synced: c.secretInformer.Informer().HasSynced})
	}
	if c.serviceAccountInformer != nil {
		syncs = append(syncs, struct {
			name   string
			synced cache.InformerSynced
		}{name: "ServiceAccount", synced: c.serviceAccountInformer.Informer().HasSynced})
	}
	if c.podInformer != nil {
		syncs = append(syncs, struct {
			name   string
			synced cache.InformerSynced
		}{name: "Pod", synced: c.podInformer.Informer().HasSynced})
	}
	if c.deploymentInformer != nil {
		syncs = append(syncs, struct {
			name   string
			synced cache.InformerSynced
		}{name: "Deployment", synced: c.deploymentInformer.Informer().HasSynced})
	}
	if c.replicaSetInformer != nil {
		syncs = append(syncs, struct {
			name   string
			synced cache.InformerSynced
		}{name: "ReplicaSet", synced: c.replicaSetInformer.Informer().HasSynced})
	}
	if c.statefulSetInformer != nil {
		syncs = append(syncs, struct {
			name   string
			synced cache.InformerSynced
		}{name: "StatefulSet", synced: c.statefulSetInformer.Informer().HasSynced})
	}
	if c.daemonSetInformer != nil {
		syncs = append(syncs, struct {
			name   string
			synced cache.InformerSynced
		}{name: "DaemonSet", synced: c.daemonSetInformer.Informer().HasSynced})
	}
	if c.ingressInformer != nil {
		syncs = append(syncs, struct {
			name   string
			synced cache.InformerSynced
		}{name: "Ingress", synced: c.ingressInformer.Informer().HasSynced})
	}
	if c.jobInformer != nil {
		syncs = append(syncs, struct {
			name   string
			synced cache.InformerSynced
		}{name: "Job", synced: c.jobInformer.Informer().HasSynced})
	}
	if c.cronJobInformer != nil {
		syncs = append(syncs, struct {
			name   string
			synced cache.InformerSynced
		}{name: "CronJob", synced: c.cronJobInformer.Informer().HasSynced})
	}
	if c.certificateInformer != nil {
		syncs = append(syncs, struct {
			name   string
			synced cache.InformerSynced
		}{name: "Certificate", synced: c.certificateInformer.HasSynced})
	}
	if c.externalSecretInformer != nil {
		syncs = append(syncs, struct {
			name   string
			synced cache.InformerSynced
		}{name: "ExternalSecret", synced: c.externalSecretInformer.HasSynced})
	}
	if c.secretProviderClassInformer != nil {
		syncs = append(syncs, struct {
			name   string
			synced cache.InformerSynced
		}{name: "SecretProviderClass", synced: c.secretProviderClassInformer.HasSynced})
	}
	if c.kafkaConnectorInformer != nil {
		syncs = append(syncs, struct {
			name   string
			synced cache.InformerSynced
		}{name: "KafkaConnector", synced: c.kafkaConnectorInformer.HasSynced})
	}
	if c.confluentConnectorInformer != nil {
		syncs = append(syncs, struct {
			name   string
			synced cache.InformerSynced
		}{name: "Connector", synced: c.confluentConnectorInformer.HasSynced})
	}

	return syncs
}

func resourceOrder() []string {
	return []string{"Secret", "ServiceAccount", "Pod", "Deployment", "ReplicaSet", "StatefulSet", "DaemonSet", "Ingress", "Job", "CronJob", "Certificate", "ExternalSecret", "SecretProviderClass", "KafkaConnector", "Connector"}
}

func stringSet(values []string) map[string]struct{} {
	out := map[string]struct{}{}

	for _, value := range values {
		if value == "" {
			continue
		}
		out[value] = struct{}{}
	}

	return out
}
