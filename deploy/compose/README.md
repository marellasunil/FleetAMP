# FleetAMP + OpenTelemetry Collector container lab

This local Docker Compose lab builds FleetAMP from the current checkout and
connects one OpenTelemetry Collector through an OpAMP supervisor. It uses
separate named volumes for FleetAMP state and the supervisor's stable identity.
It does not use the Fedora systemd service, its database, or its ports.
A small web proxy shares FleetAMP's container network and forwards browser
requests over loopback. This also supports Fedora browsers that send
`Origin: null` on a same-origin request. Docker publishes the proxy only on
`127.0.0.1:18080`; FleetAMP and OpAMP listen on container loopback.

## Requirements

- Docker Engine and Docker Compose
- A local `fleetamp-opamp-supervisor:0.149.0` image containing
  `/opampsupervisor` and `/otelcol-contrib`. This is the supervisor image
  already built on the Fedora test laptop. To use a different supervisor
  image, change `collector.image` in `compose.yaml` and verify both binary
  paths in `supervisor.yaml`.
- Enough memory for FleetAMP, the Go build, and the Collector.

## Run

From the FleetAMP repository root:

```bash
docker compose -f deploy/compose/compose.yaml up -d --build
docker compose -f deploy/compose/compose.yaml ps
curl -fsS http://127.0.0.1:18080/health
docker compose -f deploy/compose/compose.yaml logs collector
```

Open <http://localhost:18080/setup> on Fedora for the isolated first-time
administrator setup. Retrieve its temporary setup token from
`docker compose -f deploy/compose/compose.yaml logs fleetamp`; do not paste
the token into an issue or Git commit. After setup, open
<http://localhost:18080/agents> and check that the collector appears.

The lab uses plaintext OpAMP only on container loopback. Do not expose the
web proxy beyond the laptop. Use the documented TLS, OpAMP token, and server
pepper configuration for remote deployments.

To verify persistence, restart the containers and check the same collector
identity and any assigned group:

```bash
docker compose -f deploy/compose/compose.yaml restart
docker compose -f deploy/compose/compose.yaml ps
```

To stop without deleting stored state:

```bash
docker compose -f deploy/compose/compose.yaml down
```

`down -v` also deletes the two lab volumes, including the FleetAMP database,
administrator, and supervisor identity. Use it only when you want to reset
the lab.
