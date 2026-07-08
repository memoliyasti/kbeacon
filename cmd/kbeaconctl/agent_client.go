package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	defaultKBeaconNamespace   = "kbeacon-system"
	defaultKBeaconService     = "kbeacon"
	defaultKBeaconServicePort = "http"
)

type agentClient interface {
	Get(ctx context.Context, endpoint string, query url.Values) ([]byte, string, int, error)
	Description() string
}

type clientOptions struct {
	Server      string
	Namespace   string
	Service     string
	ServicePort string
	Kubeconfig  string
	Context     string
	Timeout     time.Duration
}

type directAgentClient struct {
	client  *http.Client
	baseURL *url.URL
}

func newDirectAgentClient(server string, timeout time.Duration) (*directAgentClient, error) {
	baseURL, err := parseDirectBaseURL(server)
	if err != nil {
		return nil, err
	}

	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	return &directAgentClient{
		client:  &http.Client{Timeout: timeout},
		baseURL: baseURL,
	}, nil
}

func (c *directAgentClient) Get(ctx context.Context, endpoint string, query url.Values) ([]byte, string, int, error) {
	requestURL := joinEndpointURL(c.baseURL, endpoint, query)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, requestURL, 0, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, requestURL, 0, err
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, requestURL, resp.StatusCode, fmt.Errorf("read response: %w", readErr)
	}

	return body, requestURL, resp.StatusCode, nil
}

func (c *directAgentClient) Description() string {
	return c.baseURL.String()
}

type kubeServiceProxyClient struct {
	client      *http.Client
	baseURL     *url.URL
	namespace   string
	service     string
	servicePort string
	context     string
}

func newKubeServiceProxyClient(opts clientOptions) (*kubeServiceProxyClient, error) {
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	namespace := nonEmpty(opts.Namespace, defaultKBeaconNamespace)
	service := nonEmpty(opts.Service, defaultKBeaconService)
	servicePort := nonEmpty(opts.ServicePort, defaultKBeaconServicePort)

	restConfig, err := kubeRESTConfig(opts.Kubeconfig, opts.Context)
	if err != nil {
		return nil, fmt.Errorf("load Kubernetes client config: %w", err)
	}

	transport, err := rest.TransportFor(restConfig)
	if err != nil {
		return nil, fmt.Errorf("build Kubernetes API transport: %w", err)
	}

	baseURL, err := kubeServiceProxyURL(restConfig.Host, namespace, service, servicePort)
	if err != nil {
		return nil, err
	}

	return &kubeServiceProxyClient{
		client:      &http.Client{Transport: transport, Timeout: timeout},
		baseURL:     baseURL,
		namespace:   namespace,
		service:     service,
		servicePort: servicePort,
		context:     opts.Context,
	}, nil
}

func (c *kubeServiceProxyClient) Get(ctx context.Context, endpoint string, query url.Values) ([]byte, string, int, error) {
	requestURL := joinEndpointURL(c.baseURL, endpoint, query)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, requestURL, 0, fmt.Errorf("build Kubernetes proxy request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, requestURL, 0, err
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, requestURL, resp.StatusCode, fmt.Errorf("read Kubernetes proxy response: %w", readErr)
	}

	return body, requestURL, resp.StatusCode, nil
}

func (c *kubeServiceProxyClient) Description() string {
	contextName := nonEmpty(c.context, "current-context")
	return fmt.Sprintf("kube://%s/%s/services/%s:%s/proxy", contextName, c.namespace, c.service, c.servicePort)
}

func kubeRESTConfig(kubeconfigPath, contextName string) (*rest.Config, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if strings.TrimSpace(kubeconfigPath) != "" {
		loadingRules.ExplicitPath = strings.TrimSpace(kubeconfigPath)
	}

	overrides := &clientcmd.ConfigOverrides{}
	if strings.TrimSpace(contextName) != "" {
		overrides.CurrentContext = strings.TrimSpace(contextName)
	}

	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides).ClientConfig()
}

func kubeServiceProxyURL(host, namespace, service, servicePort string) (*url.URL, error) {
	host = strings.TrimSpace(host)
	namespace = strings.TrimSpace(namespace)
	service = strings.TrimSpace(service)
	servicePort = strings.TrimSpace(servicePort)

	if host == "" {
		return nil, fmt.Errorf("Kubernetes API server host is empty")
	}
	if namespace == "" {
		return nil, fmt.Errorf("KBeacon namespace is empty")
	}
	if service == "" {
		return nil, fmt.Errorf("KBeacon service name is empty")
	}
	if servicePort == "" {
		return nil, fmt.Errorf("KBeacon service port is empty")
	}

	u, err := url.Parse(host)
	if err != nil {
		return nil, fmt.Errorf("parse Kubernetes API server host %q: %w", host, err)
	}
	if u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("Kubernetes API server host must include scheme and host")
	}

	u.Path = strings.TrimRight(u.Path, "/") +
		"/api/v1/namespaces/" + url.PathEscape(namespace) +
		"/services/" + url.PathEscape(service) + ":" + url.PathEscape(servicePort) +
		"/proxy"
	u.RawQuery = ""
	u.Fragment = ""

	return u, nil
}

func parseDirectBaseURL(server string) (*url.URL, error) {
	server = strings.TrimSpace(server)
	if server == "" {
		return nil, fmt.Errorf("server URL is empty")
	}

	if !strings.Contains(server, "://") {
		server = "http://" + server
	}

	u, err := url.Parse(server)
	if err != nil {
		return nil, fmt.Errorf("parse server URL %q: %w", server, err)
	}
	if u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("server URL must include scheme and host")
	}

	u.Path = strings.TrimRight(u.Path, "/")
	u.RawQuery = ""
	u.Fragment = ""

	return u, nil
}

func joinEndpointURL(baseURL *url.URL, endpoint string, query url.Values) string {
	u := *baseURL

	endpoint = "/" + strings.TrimLeft(endpoint, "/")
	u.Path = strings.TrimRight(u.Path, "/") + endpoint

	if query != nil {
		u.RawQuery = query.Encode()
	} else {
		u.RawQuery = ""
	}
	u.Fragment = ""

	return u.String()
}

func nonEmpty(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}
