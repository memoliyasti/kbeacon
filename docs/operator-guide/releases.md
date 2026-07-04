# Releases

KBeacon releases are produced from semantic version tags.

Create a release tag:

    git tag -a v0.3.2 -m "KBeacon v0.3.2"
    git push origin v0.3.2

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

## SBOMs and attestations

KBeacon releases include SPDX JSON SBOM files for the source tree and generated release artifacts.

The release workflow creates artifact provenance attestations with GitHub artifact attestations and enables BuildKit provenance and SBOM metadata for release container images.

Use `checksums.txt` to verify downloaded release assets. Use GitHub CLI artifact attestation verification when you need provenance validation in addition to checksum validation.

## kbeaconctl release assets

Release assets include kbeaconctl binaries for Linux and macOS alongside the kbeacon-agent binaries. Verify downloaded CLI binaries with checksums.txt.
