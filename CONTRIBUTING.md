# Contributing to KBeacon

KBeacon welcomes contributions that preserve the project principles: lightweight agent, Prometheus/Mimir storage, Grafana UI, read-only Kubernetes access, and no Secret value export.

## Development workflow

1. Open an issue for non-trivial changes.
2. Add tests for extractors, annotations, graph behavior, metrics, and API responses.
3. Keep metric label cardinality bounded and documented.
4. Update `docs/technical-design.md` when behavior changes.
5. Run:

```bash
make fmt
make test
make helm-template
```

## Adding resource support

New resource support should be implemented as a dedicated extractor. See the extractor design in `docs/technical-design.md#24-extensibility`.

## Documentation standard

User-facing behavior must be documented with examples. Metrics and annotations are part of the public contract and require special care.
