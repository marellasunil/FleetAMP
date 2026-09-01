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

## Local development as a user service

For a single-user Fedora/Linux development machine, FleetAMP can run through the
user's systemd manager. This does not require a root-owned unit or a dedicated
system account. The example in `fleetamp-user.service` is based on the
service validated on the FleetAMP Fedora development laptop.

The example assumes:

```text
%h/FleetAMP                         repository and working directory
%h/.local/bin/fleetamp              installed FleetAMP binary
%h/.local/state/fleetamp/log/        application log directory
```

`%h` is a systemd specifier for the current user's home directory. Change the
working directory and data paths if the repository is stored elsewhere.

Build and install the binary and unit:

```bash
cd ~/FleetAMP
go build -o fleetamp ./cmd/fleetamp
install -D -m 0755 fleetamp ~/.local/bin/fleetamp
install -d ~/.config/systemd/user ~/.local/state/fleetamp/log
install -m 0644 deploy/systemd/fleetamp-user.service \
  ~/.config/systemd/user/fleetamp.service
systemctl --user daemon-reload
systemctl --user enable --now fleetamp
```

Verify and manage the user service without `sudo`:

```bash
systemctl --user status fleetamp
systemctl --user is-enabled fleetamp
systemctl --user restart fleetamp
journalctl --user -u fleetamp -f
curl http://127.0.0.1:8080/health
```

Inspect which unit file systemd loaded:

```bash
systemctl --user show fleetamp -p FragmentPath
systemctl --user cat fleetamp
```

A command such as `sudo systemctl status fleetamp` checks the system service
manager and will report that the unit is missing when FleetAMP is installed only
as a user service.

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
