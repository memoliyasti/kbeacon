package server

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/kbeacon/kbeacon/internal/graph"
)

type Options struct {
	Cluster        string
	Version        string
	Commit         string
	Now            func() time.Time
	Graph          *graph.Cache
	MetricsHandler http.Handler
	Readiness      func() ([]map[string]any, bool)
}

type Server struct {
	options Options
	mux     *http.ServeMux
}

func New(options Options) http.Handler {
	if options.Now == nil {
		options.Now = time.Now
	}
	if options.Graph == nil {
		options.Graph = graph.NewCache(options.Cluster)
	}
	if options.MetricsHandler == nil {
		options.MetricsHandler = http.NotFoundHandler()
	}

	s := &Server{options: options, mux: http.NewServeMux()}
	s.routes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.mux.HandleFunc("/healthz", s.healthz)
	s.mux.HandleFunc("/readyz", s.readyz)
	s.mux.Handle("/metrics", s.options.MetricsHandler)

	s.mux.HandleFunc("/api/v1", s.discovery)
	s.mux.HandleFunc("/api/v1/config", s.config)
	s.mux.HandleFunc("/api/v1/secrets", s.listSecrets)
	s.mux.HandleFunc("/api/v1/secrets/", s.secretSubresource)
	s.mux.HandleFunc("/api/v1/workloads", s.listWorkloads)
	s.mux.HandleFunc("/api/v1/workloads/", s.workloadSubresource)
	s.mux.HandleFunc("/api/v1/dependency-map", s.dependencyMap)

	// Compatibility aliases.
	s.mux.HandleFunc("/api/secrets", s.listSecrets)
	s.mux.HandleFunc("/api/secrets/", s.secretSubresource)
	s.mux.HandleFunc("/api/workloads", s.listWorkloads)
	s.mux.HandleFunc("/api/workloads/", s.workloadSubresource)
	s.mux.HandleFunc("/api/dependency-map", s.dependencyMap)
}

func (s *Server) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) readyz(w http.ResponseWriter, _ *http.Request) {
	caches := []map[string]any{}
	ready := true

	if s.options.Readiness != nil {
		caches, ready = s.options.Readiness()
	}

	status := http.StatusOK
	statusText := "ready"
	if !ready {
		status = http.StatusServiceUnavailable
		statusText = "not ready"
	}

	writeJSON(w, status, map[string]any{
		"status":  statusText,
		"cluster": s.options.Cluster,
		"caches":  caches,
	})
}

func (s *Server) discovery(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"apiVersion": "kbeacon.io/v1",
		"cluster":    s.options.Cluster,
		"resources":  []string{"secrets", "workloads", "dependency-map", "config"},
	})
}

func (s *Server) listSecrets(w http.ResponseWriter, r *http.Request) {
	items := s.options.Graph.ListSecrets()
	q := r.URL.Query()

	filtered := make([]graph.SecretSummary, 0, len(items))
	for _, item := range items {
		if v := q.Get("namespace"); v != "" && item.Ref.Namespace != v {
			continue
		}
		if v := q.Get("ownerTeam"); v != "" && item.OwnerTeam != v {
			continue
		}
		if v := q.Get("criticality"); v != "" && item.Criticality != v {
			continue
		}
		filtered = append(filtered, item)
	}

	s.writeEnvelope(w, filtered)
}

func (s *Server) listWorkloads(w http.ResponseWriter, r *http.Request) {
	items := s.options.Graph.ListWorkloads()
	q := r.URL.Query()

	filtered := make([]graph.WorkloadSummary, 0, len(items))
	for _, item := range items {
		if v := q.Get("namespace"); v != "" && item.Ref.Namespace != v {
			continue
		}
		if v := q.Get("ownerTeam"); v != "" && item.OwnerTeam != v {
			continue
		}
		if v := q.Get("criticality"); v != "" && item.Criticality != v {
			continue
		}
		if v := q.Get("workloadKind"); v != "" && !strings.EqualFold(item.Ref.Kind, v) {
			continue
		}
		filtered = append(filtered, item)
	}

	s.writeEnvelope(w, filtered)
}

func (s *Server) dependencyMap(w http.ResponseWriter, _ *http.Request) {
	s.writeEnvelope(w, s.options.Graph.DependencyMap())
}

func (s *Server) secretSubresource(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	path = strings.TrimPrefix(path, "/api/v1/secrets/")
	path = strings.TrimPrefix(path, "/api/secrets/")
	parts := splitPath(path)

	if len(parts) != 3 || parts[2] != "impact" {
		writeError(w, http.StatusNotFound, "not_found", "unknown Secret API path")
		return
	}

	namespace := parts[0]
	name := parts[1]

	impact, ok := s.options.Graph.GetSecretImpact(namespace, name)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "Secret not found in dependency graph")
		return
	}

	s.writeEnvelope(w, impact)
}

func (s *Server) workloadSubresource(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	path = strings.TrimPrefix(path, "/api/v1/workloads/")
	path = strings.TrimPrefix(path, "/api/workloads/")
	parts := splitPath(path)

	var namespace, kind, name string

	switch {
	case len(parts) == 3 && parts[2] == "dependencies":
		namespace = parts[0]
		name = parts[1]
	case len(parts) == 4 && parts[3] == "dependencies":
		namespace = parts[0]
		kind = parts[1]
		name = parts[2]
	default:
		writeError(w, http.StatusNotFound, "not_found", "unknown Workload API path")
		return
	}

	deps, ok := s.options.Graph.GetWorkloadDependencies(namespace, kind, name)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "Workload not found in dependency graph")
		return
	}

	s.writeEnvelope(w, deps)
}

func (s *Server) config(w http.ResponseWriter, _ *http.Request) {
	snapshot := s.options.Graph.Snapshot()
	s.writeEnvelope(w, map[string]any{
		"cluster": map[string]string{"name": s.options.Cluster},
		"graph": map[string]int{
			"secrets":   len(snapshot.Secrets),
			"workloads": len(snapshot.Workloads),
			"edges":     len(snapshot.Edges),
		},
	})
}

func (s *Server) writeEnvelope(w http.ResponseWriter, data any) {
	writeJSON(w, http.StatusOK, map[string]any{
		"apiVersion":  "kbeacon.io/v1",
		"cluster":     s.options.Cluster,
		"generatedAt": s.options.Now().UTC().Format(time.RFC3339),
		"data":        data,
	})
}

func splitPath(path string) []string {
	raw := strings.Split(strings.Trim(path, "/"), "/")
	out := make([]string, 0, len(raw))

	for _, item := range raw {
		if item == "" {
			continue
		}
		decoded, err := url.PathUnescape(item)
		if err != nil {
			out = append(out, item)
			continue
		}
		out = append(out, decoded)
	}

	return out
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{
		"apiVersion": "kbeacon.io/v1",
		"error": map[string]any{
			"code":    code,
			"message": message,
		},
	})
}
