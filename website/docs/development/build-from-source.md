---
title: Build from Source
---

# Build from Source

Clone the repository and run the standard Go checks before submitting changes.

```bash
git clone https://github.com/marellasunil/FleetAMP.git
cd FleetAMP
gofmt -w ./cmd ./internal
go test ./...
go vet ./...
```

## Project boundaries

Contributions should preserve the layered design:

- Protocol-specific types remain in management adapters.
- Core packages use FleetAMP domain models.
- Storage is accessed through interfaces.
- Git, CMDB/CSDM, identity, and other external systems are implemented as providers/adapters rather than core dependencies.

See the repository `CONTRIBUTING.md` for contribution guidance.
