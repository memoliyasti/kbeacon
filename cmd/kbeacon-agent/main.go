package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/memoliyasti/kbeacon/internal/config"
	"github.com/memoliyasti/kbeacon/internal/controller"
	"github.com/memoliyasti/kbeacon/internal/discovery"
	"github.com/memoliyasti/kbeacon/internal/graph"
	"github.com/memoliyasti/kbeacon/internal/kube"
	kmetrics "github.com/memoliyasti/kbeacon/internal/metrics"
	"github.com/memoliyasti/kbeacon/internal/server"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	version = "dev"
	commit  = "unknown"
)

func main() {
	var (
		bindAddressRaw          = flag.String("http-bind-address", getenv("KBEACON_HTTP_BIND_ADDRESS", ""), "HTTP bind address")
		clusterNameRaw          = flag.String("cluster-name", "", "KBeacon cluster name")
		logLevelRaw             = flag.String("log-level", "", "Log level")
		configPath              = flag.String("config", "", "Path to Agent config file")
		kubeconfig              = flag.String("kubeconfig", getenv("KUBECONFIG", ""), "Path to kubeconfig file")
		resyncIntervalRaw       = flag.String("resync-interval", "", "Kubernetes informer resync interval")
		rebuildDebounceRaw      = flag.String("rebuild-debounce", "", "Graph rebuild debounce duration")
		includeNamespacesRaw    = flag.String("include-namespaces", "", "Comma-separated namespace allow-list")
		excludeNamespacesRaw    = flag.String("exclude-namespaces", "", "Comma-separated namespace deny-list")
		includeImagePullSecrets = flag.Bool("include-image-pull-secrets", true, "Discover imagePullSecrets dependencies")
	)
	flag.Parse()

	flagSet := map[string]bool{}
	flag.Visit(func(f *flag.Flag) {
		flagSet[f.Name] = true
	})

	cfg, err := config.Load(*configPath)
	if err != nil {
		logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
		logger.Error("failed to load config", "config", *configPath, "error", err)
		os.Exit(1)
	}

	if flagSet["cluster-name"] {
		cfg.Cluster.Name = *clusterNameRaw
	}
	if flagSet["log-level"] {
		cfg.Log.Level = *logLevelRaw
	}
	if flagSet["resync-interval"] {
		cfg.Discovery.ResyncInterval = *resyncIntervalRaw
	}
	if flagSet["rebuild-debounce"] {
		cfg.Discovery.Reconcile.Debounce = *rebuildDebounceRaw
	}
	if flagSet["include-namespaces"] {
		cfg.Discovery.Namespaces.Include = config.SplitCSV(*includeNamespacesRaw)
	}
	if flagSet["exclude-namespaces"] {
		cfg.Discovery.Namespaces.Exclude = config.SplitCSV(*excludeNamespacesRaw)
	}
	if flagSet["include-image-pull-secrets"] {
		cfg.Discovery.IncludeImagePullSecrets = *includeImagePullSecrets
	}

	cfg.Normalize()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: parseLevel(cfg.Log.Level)}))
	logger.Info(
		"starting kbeacon-agent",
		"version", version,
		"commit", commit,
		"cluster", cfg.Cluster.Name,
		"config", *configPath,
	)

	resyncInterval, err := cfg.ResyncInterval()
	if err != nil {
		logger.Error("invalid resync interval", "value", cfg.Discovery.ResyncInterval, "error", err)
		os.Exit(1)
	}

	rebuildDebounce, err := cfg.RebuildDebounce()
	if err != nil {
		logger.Error("invalid rebuild debounce", "value", cfg.Discovery.Reconcile.Debounce, "error", err)
		os.Exit(1)
	}

	shutdownGracePeriod, err := cfg.ShutdownGracePeriod()
	if err != nil {
		logger.Error("invalid shutdown grace period", "value", cfg.Agent.ShutdownGracePeriod, "error", err)
		os.Exit(1)
	}

	client, _, err := kube.NewClient(*kubeconfig)
	if err != nil {
		logger.Error("failed to create kubernetes client", "error", err)
		os.Exit(1)
	}

	graphCache := graph.NewCache(cfg.Cluster.Name)

	var runtimeRecorder *kmetrics.RuntimeRecorder
	if cfg.Metrics.Runtime.Enabled {
		runtimeRecorder = kmetrics.NewRuntimeRecorder(cfg.Cluster.Name)
	}

	discoveryOptions := discovery.DefaultOptions(cfg.Cluster.Name)
	discoveryOptions.DefaultMode = cfg.DiscoveryMode()
	discoveryOptions.IncludeImagePullSecrets = cfg.Discovery.IncludeImagePullSecrets
	discoveryOptions.IncludeInitContainers = cfg.Discovery.IncludeInitContainers
	discoveryOptions.IncludeEphemeralContainers = cfg.Discovery.IncludeEphemeralContainers
	discoveryOptions.ReadPodTemplateAnnotations = cfg.Discovery.ReadPodTemplateAnnotations
	discoveryOptions.MetadataLabelsEnabled = cfg.Discovery.MetadataLabels.Enabled
	discoveryOptions.MetadataLabelKeys = discovery.MetadataLabelKeyConfig{
		OwnerTeam:   cfg.Discovery.MetadataLabels.OwnerTeam,
		Service:     cfg.Discovery.MetadataLabels.Service,
		Environment: cfg.Discovery.MetadataLabels.Environment,
		Criticality: cfg.Discovery.MetadataLabels.Criticality,
	}

	ctrl := controller.New(client, graphCache, controller.Options{
		Cluster:           cfg.Cluster.Name,
		Resync:            resyncInterval,
		RebuildDebounce:   rebuildDebounce,
		IncludeNamespaces: cfg.Discovery.Namespaces.Include,
		ExcludeNamespaces: cfg.Discovery.Namespaces.Exclude,
		Resources: controller.ResourceConfig{
			Secrets:         cfg.ResourcesToWatch.Core.Secrets,
			ServiceAccounts: cfg.ResourcesToWatch.Core.ServiceAccounts,
			Pods:            cfg.ResourcesToWatch.Core.Pods,
			Deployments:     cfg.ResourcesToWatch.Apps.Deployments,
			StatefulSets:    cfg.ResourcesToWatch.Apps.StatefulSets,
			DaemonSets:      cfg.ResourcesToWatch.Apps.DaemonSets,
			Jobs:            cfg.ResourcesToWatch.Batch.Jobs,
			CronJobs:        cfg.ResourcesToWatch.Batch.CronJobs,
		},
		ResourcesSet: true,
		Recorder:     runtimeRecorder,
		Discovery:    discoveryOptions,
		Logger:       logger,
	})

	registry := prometheus.NewRegistry()
	registry.MustRegister(kmetrics.NewGraphCollectorWithOptions(
		graphCache,
		cfg.Cluster.Name,
		version,
		commit,
		kmetrics.GraphCollectorOptions{
			EmitEdges: cfg.Metrics.Edge.Enabled,
		},
	))
	if runtimeRecorder != nil {
		registry.MustRegister(runtimeRecorder)
	}
	if cfg.Metrics.Runtime.Enabled {
		registry.MustRegister(kmetrics.NewRuntimeCollector(
			graphCache,
			cfg.Cluster.Name,
			version,
			commit,
			ctrl.CacheSyncStatus,
			ctrl.LastGraphCounts,
		))
	}

	bindAddress := cfg.HTTPBindAddress()
	if *bindAddressRaw != "" {
		bindAddress = *bindAddressRaw
	}

	h := server.New(server.Options{
		Cluster: cfg.Cluster.Name,
		Version: version,
		Commit:  commit,
		Now:     time.Now,
		Graph:   graphCache,
		MetricsHandler: promhttp.HandlerFor(registry, promhttp.HandlerOpts{
			EnableOpenMetrics: false,
		}),
		Readiness: ctrl.Status,
	})

	srv := &http.Server{
		Addr:              bindAddress,
		Handler:           h,
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 2)

	go func() {
		if err := ctrl.Start(ctx); err != nil && ctx.Err() == nil {
			errCh <- err
		}
	}()

	go func() {
		logger.Info("serving", "address", bindAddress)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	exitCode := 0

	select {
	case <-ctx.Done():
		logger.Info("shutdown requested")
	case err := <-errCh:
		if err != nil {
			logger.Error("agent failed", "error", err)
			exitCode = 1
		}
		stop()
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownGracePeriod)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("http shutdown failed", "error", err)
		exitCode = 1
	}

	logger.Info("kbeacon-agent stopped")
	os.Exit(exitCode)
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func parseLevel(level string) slog.Leveler {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
