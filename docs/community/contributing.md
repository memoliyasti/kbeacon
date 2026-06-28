# Contributing

See the repository-level `CONTRIBUTING.md` for the full contributor guide.

Before opening a pull request, run:

    go fmt ./...
    go test ./...
    helm lint ./charts/kbeacon --set cluster.name=ci
    mkdocs build --strict

Never include Secret values, tokens, kubeconfigs, or registry credentials in issues, pull requests, logs, examples, or screenshots.
