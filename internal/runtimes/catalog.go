// Package runtimes defines component types used by OTelPlane lifecycle requests.
package runtimes

// Type identifies a managed OpenTelemetry component kind.
type Type string

const (
	OTelCollector           Type = "otel-collector"
	OTelCollectorKubernetes Type = "otel-collector-kubernetes"
	OTelOperator            Type = "otel-operator"
)
