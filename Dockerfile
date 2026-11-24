FROM golang:1.26.0-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /otelplane ./cmd/otelplane

FROM alpine:3.22
RUN apk add --no-cache ca-certificates && mkdir -p /var/lib/otelplane && chown 10001:10001 /var/lib/otelplane
COPY --from=build /otelplane /usr/local/bin/otelplane
USER 10001:10001
WORKDIR /var/lib/otelplane
EXPOSE 8080 4320
ENTRYPOINT ["/usr/local/bin/otelplane"]
