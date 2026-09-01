# FleetAMP systemd deployment

This directory contains the reference systemd deployment for running FleetAMP as a long-running Linux service on a VM or bare-metal host.

## Runtime layout

```text
/opt/fleetamp/bin/fleetamp       FleetAMP binary
/etc/fleetamp/fleetamp.env       Service configuration
/var/lib/fleetamp/               Persistent FleetAMP state
/etc/systemd/system/fleetamp.service
```

FleetAMP writes application output to stdout/stderr. Under systemd this is captured by journald, so no FleetAMP-specific log file or log-rotation process is required.

## Build and install

```bash
go build -o fleetamp ./cmd/fleetamp
sudo useradd --system --home-dir /var/lib/fleetamp --shell /sbin/nologin fleetamp
sudo install -d -o fleetamp -g fleetamp /opt/fleetamp/bin /etc/fleetamp /var/lib/fleetamp
sudo install -m 0755 fleetamp /opt/fleetamp/bin/fleetamp
sudo install -m 0644 deploy/systemd/fleetamp.env.example /etc/fleetamp/fleetamp.env
sudo install -m 0644 deploy/systemd/fleetamp.service /etc/systemd/system/fleetamp.service
sudo systemctl daemon-reload
sudo systemctl enable --now fleetamp
```

## Verify

```bash
systemctl status fleetamp
curl http://127.0.0.1:8080/health
journalctl -u fleetamp -f
```

The OpAMP endpoint uses the address configured by `FLEETAMP_OPAMP_ADDR` (default `:4320`) and the `/v1/opamp` WebSocket path.

## Upgrade

Build or download the replacement binary, stop the service, replace `/opt/fleetamp/bin/fleetamp`, then start the service again. FleetAMP state remains under `/var/lib/fleetamp`.

```bash
sudo systemctl stop fleetamp
sudo install -m 0755 fleetamp /opt/fleetamp/bin/fleetamp
sudo systemctl start fleetamp
```

## Logging and retention

Use journald to view FleetAMP logs:

```bash
journalctl -u fleetamp
journalctl -u fleetamp --since "1 hour ago"
journalctl -u fleetamp -f
```

Log retention is an operating-system concern. Configure journald limits such as `SystemMaxUse`, `SystemMaxFileSize`, and `MaxRetentionSec` according to the host policy rather than implementing log deletion inside FleetAMP.

## Structured logs and rotation

FleetAMP can mirror structured logs to `/var/log/fleetamp/fleetamp.log` while continuing to emit the same records to stdout for journald.

The supplied `fleetamp-logrotate` policy rotates the file daily or when it reaches 10 MB, keeps three rotated backups, and removes backups older than three days. The optional `fleetamp-logrotate.timer` checks the policy hourly so a high-volume log does not wait for the normal daily logrotate timer.

Install the rotation files as root:

```bash
cp deploy/systemd/fleetamp-logrotate /etc/logrotate.d/fleetamp
cp deploy/systemd/fleetamp-logrotate.service /etc/systemd/system/
cp deploy/systemd/fleetamp-logrotate.timer /etc/systemd/system/
systemctl daemon-reload
systemctl enable --now fleetamp-logrotate.timer
```
