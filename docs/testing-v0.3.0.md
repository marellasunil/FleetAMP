# FleetAMP v0.3.0 local release verification

This is the automated release gate. It complements, but does not replace, the
[manual v0.3.0 E2E checklist](e2e-v0.3.0.md).

## Verification coverage

| Check | Purpose |
| --- | --- |
| `git diff --check` | Reject whitespace errors |
| `go test ./...` | Run unit and integration tests |
| `go vet ./...` | Run Go static analysis |
| `go test -race ./...` | Detect data races |
| Versioned binary build | Verify the release candidate compiles reproducibly |
| `/health` and `/ready` | Verify runtime health and readiness |
| SQLite start and restart | Verify schema initialization and persistence |
| Optional existing database | Verify an upgrade using a copied v0.2 database |
| Optional Docker build | Verify the release container can be built |

## Quick start

Run the release gate from a clean release branch before pushing it:

```bash
bash scripts/verify-v0.3.0.sh
```

This runs the Go test suite, `go vet`, the race detector, a reproducible
`v0.3.0-rc1` binary build, HTTP health/readiness checks and a stop/start cycle
against the same SQLite database. GitOps workers and real GitHub writes are
forced off during the smoke test.

Use `--quick` while iterating and run the full command before opening the PR:

```bash
bash scripts/verify-v0.3.0.sh --quick
bash scripts/verify-v0.3.0.sh
```

Include the Docker build when Docker is available:

```bash
bash scripts/verify-v0.3.0.sh --docker
```

## Expected result

The final line must report:

```text
[verify-v0.3.0] PASS: tests, vet, binary build, health, readiness, database migration and restart
```

The script uses a temporary data directory, binds only to loopback and forces
the GitOps worker and GitHub writes off. It removes its temporary files when it
finishes.

## Existing v0.2 database upgrade

Never test an upgrade against the only copy of a production or personal
database. Stop the existing FleetAMP instance, make a backup, restart the
instance, and pass the backup to the verifier. The script copies it into its
own temporary directory before opening it.

```bash
systemctl --user cat fleetamp | grep FLEETAMP_DATABASE_PATH

SOURCE_DB=/path/shown/by/FLEETAMP_DATABASE_PATH
cp "$SOURCE_DB" \
  ~/Desktop/Sunil/FleetAMP-E2E-backup/fleetamp-v0.2-before-v0.3.db

FLEETAMP_VERIFY_EXISTING_DB=\
~/Desktop/Sunil/FleetAMP-E2E-backup/fleetamp-v0.2-before-v0.3.db \
  bash scripts/verify-v0.3.0.sh
```

The verifier checks that FleetAMP starts, applies idempotent schema changes,
responds on `/health` and `/ready`, stops cleanly, and starts again using the
same upgraded temporary copy.

## Port overrides

The default verification ports are `127.0.0.1:18082` for HTTP and
`127.0.0.1:14322` for OpAMP. They intentionally do not replace the normal
service on `localhost:8080`.

```bash
FLEETAMP_VERIFY_HTTP_ADDR=127.0.0.1:18083 \
FLEETAMP_VERIFY_OPAMP_ADDR=127.0.0.1:14323 \
  bash scripts/verify-v0.3.0.sh
```

Only loopback addresses are accepted by the verification script.

## Release sequence after verification

1. Complete the [manual E2E checklist](e2e-v0.3.0.md).
2. Fix release-blocking defects and rerun the affected checks.
3. Rerun this full verifier from the final release commit.
4. Open and merge the release PR.
5. Tag `v0.3.0` and publish release archives and checksums.
