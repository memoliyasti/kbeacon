package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/memoliyasti/kbeacon/internal/graph"
)

const (
	defaultLimit = 100
	maxLimit     = 1000
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

type paginationRequest struct {
	Limit  int
	Offset int
}

type paginationResponse struct {
	Limit      int  `json:"limit"`
	Offset     int  `json:"offset"`
	Total      int  `json:"total"`
	Returned   int  `json:"returned"`
	NextOffset *int `json:"nextOffset,omitempty"`
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
	q := r.URL.Query()

	pageRequest, err := parsePagination(q)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_query", err.Error())
		return
	}

	exists, hasExists, err := boolQueryParam(q, "exists")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_query", err.Error())
		return
	}

	items := s.options.Graph.ListSecrets()
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
		if v := q.Get("secretName"); v != "" && item.Ref.Name != v {
			continue
		}
		if hasExists && item.Exists != exists {
			continue
		}

		filtered = append(filtered, item)
	}

	paged, pagination := paginate(filtered, pageRequest)
	s.writeEnvelopeWithPagination(w, paged, pagination)
}

func (s *Server) listWorkloads(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	pageRequest, err := parsePagination(q)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_query", err.Error())
		return
	}

	items := s.options.Graph.ListWorkloads()
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
		if v := q.Get("workloadName"); v != "" && item.Ref.Name != v {
			continue
		}
		if v := q.Get("discoveryMode"); v != "" && !strings.EqualFold(string(item.DiscoveryMode), v) {
			continue
		}

		filtered = append(filtered, item)
	}

	paged, pagination := paginate(filtered, pageRequest)
	s.writeEnvelopeWithPagination(w, paged, pagination)
}

func (s *Server) dependencyMap(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	pageRequest, err := parsePagination(q)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_query", err.Error())
		return
	}

	resolved, hasResolved, err := boolQueryParam(q, "resolved")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_query", err.Error())
		return
	}

	depMap := s.options.Graph.DependencyMap()
	filteredEdges := make([]graph.DependencyEdge, 0, len(depMap.Edges))

	for _, edge := range depMap.Edges {
		if v := q.Get("namespace"); v != "" && edge.Workload.Namespace != v && edge.Secret.Namespace != v {
			continue
		}
		if v := q.Get("workloadNamespace"); v != "" && edge.Workload.Namespace != v {
			continue
		}
		if v := q.Get("secretNamespace"); v != "" && edge.Secret.Namespace != v {
			continue
		}
		if v := q.Get("workloadKind"); v != "" && !strings.EqualFold(edge.Workload.Kind, v) {
			continue
		}
		if v := q.Get("workloadName"); v != "" && edge.Workload.Name != v {
			continue
		}
		if v := q.Get("secretName"); v != "" && edge.Secret.Name != v {
			continue
		}
		if v := q.Get("ownerTeam"); v != "" && edge.OwnerTeam != v {
			continue
		}
		if v := q.Get("criticality"); v != "" && edge.Criticality != v {
			continue
		}
		if v := q.Get("discoveryMode"); v != "" && !strings.EqualFold(string(edge.DiscoveryMode), v) {
			continue
		}
		if hasResolved && edge.Resolved != resolved {
			continue
		}

		filteredEdges = append(filteredEdges, edge)
	}

	pagedEdges, pagination := paginate(filteredEdges, pageRequest)

	out := graph.DependencyMap{
		Nodes: nodesForEdges(depMap.Nodes, pagedEdges),
		Edges: pagedEdges,
	}

	s.writeEnvelopeWithPagination(w, out, pagination)
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

func (s *Server) writeEnvelopeWithPagination(w http.ResponseWriter, data any, pagination paginationResponse) {
	writeJSON(w, http.StatusOK, map[string]any{
		"apiVersion":  "kbeacon.io/v1",
		"cluster":     s.options.Cluster,
		"generatedAt": s.options.Now().UTC().Format(time.RFC3339),
		"pagination":  pagination,
		"data":        data,
	})
}

func parsePagination(values url.Values) (paginationRequest, error) {
	out := paginationRequest{
		Limit:  defaultLimit,
		Offset: 0,
	}

	if raw := strings.TrimSpace(values.Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 {
			return out, fmt.Errorf("limit must be a positive integer")
		}
		if parsed > maxLimit {
			parsed = maxLimit
		}
		out.Limit = parsed
	}

	if raw := strings.TrimSpace(values.Get("offset")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 {
			return out, fmt.Errorf("offset must be a non-negative integer")
		}
		out.Offset = parsed
	}

	return out, nil
}

func boolQueryParam(values url.Values, key string) (bool, bool, error) {
	raw := strings.TrimSpace(values.Get(key))
	if raw == "" {
		return false, false, nil
	}

	switch strings.ToLower(raw) {
	case "true", "1", "yes", "y", "on":
		return true, true, nil
	case "false", "0", "no", "n", "off":
		return false, true, nil
	default:
		return false, true, fmt.Errorf("%s must be a boolean", key)
	}
}

func paginate[T any](items []T, request paginationRequest) ([]T, paginationResponse) {
	total := len(items)

	start := request.Offset
	if start > total {
		start = total
	}

	end := start + request.Limit
	if end > total {
		end = total
	}

	var nextOffset *int
	if end < total {
		next := end
		nextOffset = &next
	}

	return items[start:end], paginationResponse{
		Limit:      request.Limit,
		Offset:     request.Offset,
		Total:      total,
		Returned:   end - start,
		NextOffset: nextOffset,
	}
}

func nodesForEdges(nodes []graph.DependencyMapNode, edges []graph.DependencyEdge) []graph.DependencyMapNode {
	needed := map[string]struct{}{}

	for _, edge := range edges {
		needed["workload:"+graph.WorkloadID(edge.Workload)] = struct{}{}
		needed["secret:"+graph.SecretID(edge.Secret)] = struct{}{}
	}

	out := make([]graph.DependencyMapNode, 0, len(needed))
	for _, node := range nodes {
		if _, ok := needed[node.ID]; ok {
			out = append(out, node)
		}
	}

	return out
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
