FROM golang:1.25.13-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /fleetamp ./cmd/fleetamp

FROM alpine:3.22
RUN apk add --no-cache ca-certificates && mkdir -p /var/lib/fleetamp && chown 10001:10001 /var/lib/fleetamp
COPY --from=build /fleetamp /usr/local/bin/fleetamp
USER 10001:10001
WORKDIR /var/lib/fleetamp
EXPOSE 8080 4320
ENTRYPOINT ["/usr/local/bin/fleetamp"]
