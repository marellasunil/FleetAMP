FROM ghcr.io/open-telemetry/opentelemetry-collector-releases/opentelemetry-collector-opampsupervisor:0.149.0 AS supervisor
FROM otel/opentelemetry-collector-contrib:0.149.0 AS collector
FROM alpine:3.22
RUN apk add --no-cache ca-certificates
COPY --from=supervisor /usr/local/bin/opampsupervisor /opampsupervisor
COPY --from=collector /otelcol-contrib /otelcol-contrib
RUN mkdir -p /var/lib/otel-supervisor && chmod 0777 /var/lib/otel-supervisor
ENTRYPOINT ["/opampsupervisor"]
CMD ["--config", "/etc/fleetamp-supervisor/supervisor.yaml"]
