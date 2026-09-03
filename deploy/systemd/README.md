# FleetAMP systemd deployment

This directory contains the reference systemd deployment for running FleetAMP as a long-running Linux service on a VM or bare-metal host.

## Runtime layout

```text
/opt/fleetamp/bin/fleetamp       FleetAMP binary
/etc/fleetamp/fleetamp.env       Service configuration
/etc/fleetamp/tls/                Default TLS certificate directory
/var/lib/fleetamp/               Persistent FleetAMP state
/etc/systemd/system/fleetamp.service
```

FleetAMP writes application output to stdout/stderr. Under systemd this is captured by journald, so no FleetAMP-specific log file or log-rotation process is required.

## Build and install

```bash
go build -o fleetamp ./cmd/fleetamp
sudo useradd --system --home-dir /var/lib/fleetamp --shell /sbin/nologin fleetamp
sudo install -d -o fleetamp -g fleetamp /opt/fleetamp/bin /etc/fleetamp /var/lib/fleetamp
sudo install -d -o fleetamp -g fleetamp -m 0700 /etc/fleetamp/tls
sudo install -m 0755 fleetamp /opt/fleetamp/bin/fleetamp
sudo install -m 0600 deploy/systemd/fleetamp.env.example /etc/fleetamp/fleetamp.env
sudo install -d -m 0700 /etc/credstore.encrypted
head -c 48 /dev/urandom | base64 | sudo systemd-creds encrypt \
  --with-key=host+tpm2 --name=fleetamp-server-pepper \
  - /etc/credstore.encrypted/fleetamp-server-pepper
sudo chmod 0600 /etc/credstore.encrypted/fleetamp-server-pepper
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

Install or upgrade the binary and unit with the repository script:

```bash
./scripts/install-user.sh install
# For later releases:
./scripts/install-user.sh upgrade
```

The script runs the Go tests and build before stopping the current service. It
then creates a consistent data backup, installs the new binary and unit,
restarts FleetAMP, and checks the HTTP health endpoint. A failed health check
automatically restores the previous binary and service files.

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

The OpAMP endpoint uses `FLEETAMP_OPAMP_ADDR` (secure default `127.0.0.1:4320`) and the `/v1/opamp` WebSocket path.

## Security configuration

FleetAMP listens only on localhost by default. On its first start it creates a
short-lived, one-time administrator setup token. Retrieve it and open `/setup`:

```bash
journalctl -u fleetamp -g bootstrap_token --since "15 minutes ago"
```

The submitted password is converted to an Argon2id verifier using a unique salt
and the server pepper. FleetAMP never stores or decrypts the password. After a
successful setup, the token is erased and `/setup` cannot create another admin.
Unused tokens are held only in memory, so a restart invalidates the previous
token and creates a replacement. Sessions use `HttpOnly`, `SameSite=Strict`
cookies; set `FLEETAMP_SECURE_COOKIES=true` whenever the public endpoint uses
HTTPS.

On Fedora, a local browser or forwarding layer can sometimes submit
`Origin: null`. FleetAMP accepts that value only for a loopback target, a
loopback network peer, and a browser request reported as same-origin or
same-site. Diagnose rejected requests without exposing secrets:

```bash
journalctl --user -u fleetamp --since "10 minutes ago" -o cat | \
  grep origin_rejected | tail
```

The encrypted `fleetamp-server-pepper` credential is bound to the local systemd
host key and TPM2. Copying it and the SQLite database to another server will not
produce a usable login. If TPM2 is unavailable, use `--with-key=host`; this is
still OS-installation-specific but does not provide hardware binding. Never
copy the plaintext pepper or include it in a backup with the database.

Configure the independent OpAMP bearer token and browser origin in the mode
`0600` environment file:

```bash
FLEETAMP_OPAMP_TOKEN=<at-least-32-random-characters>
FLEETAMP_ALLOWED_ORIGINS=https://fleetamp.example.com
FLEETAMP_SECURE_COOKIES=true
```

Use FleetAMP native TLS or terminate HTTPS/WSS at a trusted reverse proxy or
load balancer. When TLS is terminated upstream, explicitly set
`FLEETAMP_HTTP_TLS_TERMINATED=true` and `FLEETAMP_OPAMP_TLS_TERMINATED=true`.
For native TLS, use the certificate options documented in
`fleetamp.env.example`. Keep `/health` and `/ready` restricted by firewall rules
even though they intentionally do not require login. `FLEETAMP_ALLOW_INSECURE=true` remains a development-only escape
hatch. Environment-based Basic Auth is retained temporarily for migration and
should not be used for a new installation.

## Upgrade and rollback

For a user-level installation, pull or check out the intended source revision
and run:

```bash
./scripts/install-user.sh upgrade
```

Backups are stored under
`~/.local/state/fleetamp/backups/<UTC-timestamp>/`. To restore the newest
backup, including the earlier binary and service files:

```bash
./scripts/install-user.sh rollback
```

Pass a backup directory as the second argument to restore a specific backup.
The data archive is retained for disaster recovery and is not automatically
restored during a binary rollback, avoiding unintended overwrites of live data.

For a system-wide installation, build or download the replacement binary,
back up `/var/lib/fleetamp`, replace `/opt/fleetamp/bin/fleetamp`, restart
the service, and verify `/health`.

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
