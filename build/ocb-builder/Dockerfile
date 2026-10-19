# syntax=docker/dockerfile:1.7

# GO_IMAGE is deliberately required and has no mutable default. Supply an
# official Go image pinned as repository@sha256:<digest>.
ARG GO_IMAGE
FROM ${GO_IMAGE} AS prepare

ARG COLLECTOR_VERSION
ENV GOMODCACHE=/opt/fleetamp/gomodcache \
    GOCACHE=/root/.cache/go-build \
    GOTOOLCHAIN=local \
    CGO_ENABLED=0

WORKDIR /src
COPY go.mod go.sum ./
COPY internal/distributions ./internal/distributions
COPY cmd/ocb-cache-manifest ./cmd/ocb-cache-manifest

RUN mkdir -p /cache "${GOMODCACHE}" \
    && go run ./cmd/ocb-cache-manifest \
         -version "${COLLECTOR_VERSION}" \
         -output /cache/go.mod

WORKDIR /cache
RUN --mount=type=cache,target=/root/.cache/go-build \
    go mod download all \
    && GOPROXY=off GOSUMDB=off \
       go install "go.opentelemetry.io/collector/cmd/builder@v${COLLECTOR_VERSION}"

COPY build/ocb-builder/smoke-builder.yaml.tmpl /tmp/smoke-builder.yaml.tmpl
RUN sed "s/VERSION/${COLLECTOR_VERSION}/g" \
      /tmp/smoke-builder.yaml.tmpl > /tmp/smoke-builder.yaml \
    && HOME=/tmp GOCACHE=/tmp/go-build GOPROXY=off GOSUMDB=off \
       /go/bin/builder --config /tmp/smoke-builder.yaml \
    && test -x /tmp/fleetamp-ocb-smoke/fleetamp-collector \
    && rm -rf /tmp/fleetamp-ocb-smoke /tmp/go-build \
    && chmod -R a-w "${GOMODCACHE}"

ARG GO_IMAGE
FROM ${GO_IMAGE}

ARG COLLECTOR_VERSION
ARG REVISION
LABEL org.opencontainers.image.title="FleetAMP offline OCB builder" \
      org.opencontainers.image.description="Digest-pinned OpenTelemetry Collector Builder with the FleetAMP reviewed module cache" \
      org.opencontainers.image.source="https://github.com/marellasunil/FleetAMP" \
      org.opencontainers.image.version="${COLLECTOR_VERSION}" \
      org.opencontainers.image.revision="${REVISION}"

ENV GOMODCACHE=/opt/fleetamp/gomodcache \
    GOCACHE=/tmp/go-build \
    GOPROXY=off \
    GOSUMDB=off \
    GOTOOLCHAIN=local \
    CGO_ENABLED=0 \
    HOME=/tmp

COPY --from=prepare /go/bin/builder /usr/local/bin/ocb
COPY --from=prepare /opt/fleetamp/gomodcache /opt/fleetamp/gomodcache

WORKDIR /workspace
USER 65532:65532
ENTRYPOINT ["/usr/local/bin/ocb"]
