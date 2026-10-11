# FleetAMP v0.3.0 local release verification

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
