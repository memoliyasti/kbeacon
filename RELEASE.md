# Release Process

KBeacon releases are created from Git tags.

## Versioning

KBeacon uses semantic version tags:

    vMAJOR.MINOR.PATCH

Example:

    v0.1.2

## Release checklist

Before tagging:

    git checkout main
    git pull --ff-only origin main
    git status --short
    go fmt ./...
    go test ./...
    go build -o ./bin/kbeacon-agent ./cmd/kbeacon-agent
    helm lint ./charts/kbeacon --set cluster.name=release
    helm template kbeacon ./charts/kbeacon --namespace kbeacon-system --set cluster.name=release --set dashboards.enabled=true > /tmp/kbeacon-rendered.yaml
    docker run --rm -i --entrypoint=promtool prom/prometheus:v3.1.0 check rules /dev/stdin < examples/prometheus/rules.yaml
    python3 -m pip install -r requirements-docs.txt
    mkdocs build --strict

## Tagging

    git tag -a v0.1.2 -m "KBeacon v0.1.2"
    git push origin v0.1.2

The release workflow publishes:

- GitHub Release;
- Linux binaries;
- macOS binaries;
- Helm chart package;
- SHA256 checksums;
- multi-arch container image for `linux/amd64` and `linux/arm64`.

## Container image tags

For release `v0.1.2`, the workflow publishes:

    ghcr.io/memoliyasti/kbeacon:v0.1.2
    ghcr.io/memoliyasti/kbeacon:0.1.2
    ghcr.io/memoliyasti/kbeacon:latest
    ghcr.io/memoliyasti/kbeacon:sha-<short-sha>

## Private GHCR packages

If the package is private, Kubernetes clusters need an image pull secret with `read:packages` permission.
