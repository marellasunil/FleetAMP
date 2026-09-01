---
title: Upgrade the OpAMP Dependency
---

# Upgrade the OpAMP Dependency

FleetAMP consumes the upstream OpenTelemetry `opamp-go` project as a Go module dependency. It is not installed as an RPM, DEB, or other Linux package.

The dependency is declared in `go.mod`:

```go
require github.com/open-telemetry/opamp-go <version>
```

FleetAMP keeps OpAMP-specific code isolated under `internal/opamp/`. Upgrading `opamp-go` should therefore be treated as an adapter compatibility change, not just a dependency version bump.

## Before upgrading

1. Check the currently used version:

```bash
grep 'open-telemetry/opamp-go' go.mod
```

2. Review the upstream `opamp-go` release notes and the OpAMP specification version included by that release.
3. Pay particular attention to breaking API changes, callback changes, transport behavior, capability handling, remote configuration, AgentDescription, and reconnect behavior.

4. Perform the upgrade on a branch rather than directly on the production branch.

## Upgrade the Go module

Replace `<new-version>` with the selected upstream release:

```bash
go get github.com/open-telemetry/opamp-go@<new-version>
go mod tidy
```

Confirm the dependency change:

```bash
git diff -- go.mod go.sum
```

Do not manually edit `go.sum`.

## Compile and unit-test

Run the complete test suite:

```bash
go test ./...
```

For additional concurrency validation:

```bash
go test -race ./...
```
## Review the FleetAMP OpAMP adapter

Inspect changes affecting:

```text
internal/opamp/
```

The most important file today is:

```text
internal/opamp/server.go
```

If upstream types, callbacks, protobuf fields, connection behavior, or server APIs changed, adapt only the protocol boundary where possible. The rest of FleetAMP should continue to depend on protocol-neutral domain models.

## Mandatory integration validation

A successful `go test ./...` is necessary but not sufficient. Start FleetAMP with a real OpenTelemetry OpAMP Supervisor and verify:

- Supervisor connects successfully.
- FleetAMP receives the AgentDescription and stable instance UID.
- Health and capabilities are reported correctly.
- Remote configuration can be sent.
- Delivery transitions through expected states such as `sent`, `applying`, and `applied`.
- Effective configuration is reported back.
- Disconnect and reconnect work correctly.
- Existing configuration assignments and deployment status still reconcile correctly.
- FleetAMP restart does not break Supervisor reconnection.
## Acceptance policy

FleetAMP should not automatically follow every new `opamp-go` release. Adopt a release only after compatibility testing has passed.

Recommended policy:

> FleetAMP tracks supported upstream `opamp-go` releases after source review, unit tests, race tests, and real Supervisor compatibility validation.

If the upstream release introduces a breaking change that requires broader FleetAMP changes, document that migration in the release notes before merging it.

## Roll back an unsuccessful dependency upgrade

If validation fails, restore the previous dependency version:

```bash
go get github.com/open-telemetry/opamp-go@<previous-version>
go mod tidy
go test ./...
```

Then inspect `go.mod` and `go.sum` to confirm the previous dependency has been restored.

## Upgrade flow

```text
Review upstream release/spec
        ↓
Update opamp-go dependency
        ↓
go mod tidy
        ↓
go test ./... + go test -race ./...
        ↓
Review internal/opamp adapter
        ↓
Real Supervisor integration test
        ↓
Accept or roll back upgrade
```
