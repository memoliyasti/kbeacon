# Releases

KBeacon releases are produced from semantic version tags.

Create a release tag:

    git tag -a v0.2.4 -m "KBeacon v0.2.4"
    git push origin v0.2.4

The release workflow publishes:

- Linux binaries.
- macOS binaries.
- Helm chart package.
- SHA256 checksums.
- Multi-arch container images for linux/amd64 and linux/arm64.

Images are published to:

    ghcr.io/memoliyasti/kbeacon

## Image visibility

Official KBeacon GHCR packages are public by default. Kubernetes clusters do not need an image pull Secret for the default image. Use imagePullSecrets only for private forks or private registries.
