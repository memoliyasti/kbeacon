# Release Process

KBeacon releases are created from Git tags.

## Versioning

KBeacon uses semantic version tags:

    vMAJOR.MINOR.PATCH

Example:

    v0.3.10

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

    git tag -a v0.3.10 -m "KBeacon v0.3.10"
    git push origin v0.3.10

The release workflow publishes:

- GitHub Release;
- Linux binaries;
- macOS binaries;
- Helm chart package;
- SHA256 checksums;
- multi-arch container image for `linux/amd64` and `linux/arm64`.
- keyless Sigstore Cosign signature for the published container image.

## Container image tags

For release `v0.3.10`, the workflow publishes:

    ghcr.io/memoliyasti/kbeacon:v0.3.10
    ghcr.io/memoliyasti/kbeacon:0.3.10
    ghcr.io/memoliyasti/kbeacon:latest
    ghcr.io/memoliyasti/kbeacon:sha-<short-sha>

## Official GHCR package visibility

Official KBeacon GHCR packages for this repository are expected to be public. Kubernetes clusters do not need an image pull Secret for the default image. Use `imagePullSecrets` only for private forks or private registries.

## SBOMs and attestations

Release builds publish SPDX JSON SBOM files together with the normal release artifacts.

The release workflow also creates GitHub artifact attestations for the release artifact checksums. The release image build enables BuildKit provenance and SBOM metadata for the pushed multi-arch GHCR image.

Release consumers can verify artifact checksums from `checksums.txt`. GitHub artifact attestations can be verified with GitHub CLI when available for the repository and account plan.

## kbeaconctl release assets

Release assets include kbeaconctl binaries for Linux and macOS alongside the kbeacon-agent binaries. Verify downloaded CLI binaries with checksums.txt.
