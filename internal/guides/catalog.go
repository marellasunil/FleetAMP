// Package guides defines the organization-managed Instrumentation Guide graph.
package guides

import (
	"fmt"
	"strings"
	"time"
)

var Stages = []string{"capability", "platform", "technology", "method", "topology"}

const CurrentSchemaVersion = 2

type Node struct {
	ID string `json:"id"`
	Stage string `json:"stage"`
	Name string `json:"name"`
	Description string `json:"description"`
	ParentIDs []string `json:"parent_ids,omitempty"`
	NextStageLabel string `json:"next_stage_label,omitempty"`
	Advantages string `json:"advantages,omitempty"`
	Requirements string `json:"requirements,omitempty"`
	Considerations string `json:"considerations,omitempty"`
	Steps []string `json:"steps,omitempty"`
	Enabled bool `json:"enabled"`
	SortOrder int `json:"sort_order"`
}

type Catalog struct {
	SchemaVersion int `json:"schema_version,omitempty"`
	Nodes []Node `json:"nodes"`
}

type Version struct { Version int; Catalog Catalog; CreatedBy string; CreatedAt time.Time; PublishedAt time.Time }

func stageIndex(stage string) int { for i, value := range Stages { if stage == value { return i } }; return -1 }

func (c Catalog) Validate() error {
	if len(c.Nodes) == 0 { return fmt.Errorf("guide must contain at least one node") }
	seen := map[string]Node{}
	for _, node := range c.Nodes {
		node.ID = strings.TrimSpace(node.ID); node.Name = strings.TrimSpace(node.Name); node.Stage = strings.TrimSpace(node.Stage)
		if node.ID == "" || node.Name == "" || stageIndex(node.Stage) < 0 { return fmt.Errorf("every node needs a valid ID, name and stage") }
		if _, exists := seen[node.ID]; exists { return fmt.Errorf("duplicate node ID %q", node.ID) }
		seen[node.ID] = node
	}
	for _, node := range c.Nodes {
		index := stageIndex(node.Stage)
		if index == 0 && len(node.ParentIDs) > 0 { return fmt.Errorf("capability %q cannot have a parent", node.Name) }
		if index > 0 && node.Enabled && len(node.ParentIDs) == 0 { return fmt.Errorf("%s %q needs at least one parent", node.Stage, node.Name) }
		for _, parentID := range node.ParentIDs {
			parent, exists := seen[parentID]
			if !exists { return fmt.Errorf("%q references missing parent %q", node.Name, parentID) }
			if stageIndex(parent.Stage) != index-1 { return fmt.Errorf("%q parent %q must be in the preceding stage", node.Name, parent.Name) }
		}
	}
	return nil
}

// DefaultCatalog is organized by observability intent. IDs are deliberately
// branch-specific even when labels repeat, preventing application-only choices
// from leaking into infrastructure, database, log, or frontend paths.
func DefaultCatalog() Catalog {
	n := func(id, stage, name, nextLabel string, parents ...string) Node { return Node{ID:id, Stage:stage, Name:name, NextStageLabel:nextLabel, ParentIDs:parents, Enabled:true} }
	nodes := []Node{
		n("apm","capability","Application performance","Platform"), n("infrastructure","capability","Infrastructure","Platform"), n("kubernetes","capability","Kubernetes","Cluster environment"), n("logs","capability","Logs","Platform"), n("database","capability","Database","Database platform"), n("frontend","capability","Frontend","Client platform"), n("custom","capability","Custom telemetry","Telemetry platform"),

		n("apm-linux","platform","Linux / VM","Application technology","apm"), n("apm-windows","platform","Windows / VM","Application technology","apm"), n("apm-kubernetes","platform","Kubernetes","Application technology","apm"), n("apm-serverless","platform","Serverless","Application technology","apm"),
		n("infra-linux","platform","Linux host / VM","Infrastructure metrics","infrastructure"), n("infra-windows","platform","Windows host / VM","Infrastructure metrics","infrastructure"), n("infra-kubernetes","platform","Kubernetes node","Infrastructure metrics","infrastructure"), n("infra-cloud","platform","Cloud infrastructure","Infrastructure metrics","infrastructure"),
		n("k8s-cluster","platform","Kubernetes cluster","Kubernetes telemetry","kubernetes"),
		n("logs-linux","platform","Linux / VM","Log source","logs"), n("logs-windows","platform","Windows / VM","Log source","logs"), n("logs-kubernetes","platform","Kubernetes","Log source","logs"), n("logs-cloud","platform","Cloud service","Log source","logs"),
		n("db-host","platform","Self-managed host / VM","Database engine","database"), n("db-kubernetes","platform","Kubernetes","Database engine","database"), n("db-managed","platform","Managed cloud database","Database engine","database"),
		n("frontend-browser","platform","Web browser","Frontend technology","frontend"), n("frontend-mobile","platform","Mobile application","Frontend technology","frontend"),
		n("custom-host","platform","Host / VM","Telemetry source","custom"), n("custom-kubernetes","platform","Kubernetes","Telemetry source","custom"), n("custom-external","platform","External / managed service","Telemetry source","custom"),

		n("apm-java","technology","Java","Instrumentation","apm-linux","apm-windows","apm-kubernetes","apm-serverless"), n("apm-dotnet","technology",".NET","Instrumentation","apm-linux","apm-windows","apm-kubernetes","apm-serverless"), n("apm-python","technology","Python","Instrumentation","apm-linux","apm-windows","apm-kubernetes","apm-serverless"), n("apm-node","technology","Node.js","Instrumentation","apm-linux","apm-windows","apm-kubernetes","apm-serverless"), n("apm-go","technology","Go","Instrumentation","apm-linux","apm-windows","apm-kubernetes","apm-serverless"), n("apm-php","technology","PHP","Instrumentation","apm-linux","apm-kubernetes","apm-serverless"), n("apm-other","technology","Other OTLP-capable runtime","Instrumentation","apm-linux","apm-windows","apm-kubernetes","apm-serverless"),
		n("infra-hostmetrics","technology","Host system metrics","","infra-linux","infra-windows"), n("infra-process","technology","Process metrics","","infra-linux","infra-windows"), n("infra-kubelet","technology","Node, pod and container metrics","","infra-kubernetes"), n("infra-cloudmetrics","technology","Cloud provider infrastructure metrics","","infra-cloud"),
		n("k8s-node-workload","technology","Node, pod and container metrics","Collection component","k8s-cluster"), n("k8s-cluster-state","technology","Cluster state and inventory","Collection component","k8s-cluster"), n("k8s-events","technology","Kubernetes events","Collection component","k8s-cluster"),
		n("logs-files","technology","Application log files","Collection approach","logs-linux","logs-windows","logs-kubernetes"), n("logs-journal","technology","systemd journal","Collection approach","logs-linux"), n("logs-syslog","technology","Syslog","Collection approach","logs-linux","logs-cloud"), n("logs-winevent","technology","Windows Event Log","Collection approach","logs-windows"), n("logs-container","technology","Container stdout / stderr","Collection approach","logs-kubernetes"), n("logs-otlp","technology","OTLP application logs","Collection approach","logs-linux","logs-windows","logs-kubernetes","logs-cloud"),
		n("db-postgresql","technology","PostgreSQL","Collection approach","db-host","db-kubernetes"), n("db-mysql","technology","MySQL / MariaDB","Collection approach","db-host","db-kubernetes"), n("db-mongodb","technology","MongoDB","Collection approach","db-host","db-kubernetes"), n("db-redis","technology","Redis","Collection approach","db-host","db-kubernetes"), n("db-sqlserver","technology","SQL Server","Collection approach","db-host","db-kubernetes"), n("db-other","technology","Other database","Collection approach","db-host","db-kubernetes"),
		n("db-managed-postgresql","technology","Managed PostgreSQL","Collection approach","db-managed"), n("db-managed-mysql","technology","Managed MySQL / MariaDB","Collection approach","db-managed"), n("db-managed-mongodb","technology","Managed MongoDB","Collection approach","db-managed"), n("db-managed-redis","technology","Managed Redis","Collection approach","db-managed"), n("db-managed-sqlserver","technology","Managed SQL Server","Collection approach","db-managed"), n("db-managed-other","technology","Other managed database","Collection approach","db-managed"),
		n("frontend-js","technology","Browser JavaScript","Instrumentation","frontend-browser"), n("frontend-android","technology","Android","Instrumentation","frontend-mobile"), n("frontend-ios","technology","iOS","Instrumentation","frontend-mobile"),
		n("custom-otlp","technology","OTLP source","Collection approach","custom-host","custom-kubernetes","custom-external"), n("custom-prometheus","technology","Prometheus endpoint","Collection approach","custom-host","custom-kubernetes","custom-external"), n("custom-statsd","technology","StatsD source","Collection approach","custom-host","custom-kubernetes"),

		n("apm-auto","method","Zero-code auto-instrumentation","Deployment topology","apm-java","apm-dotnet","apm-python","apm-node","apm-php"), n("apm-sdk","method","OpenTelemetry SDK","Deployment topology","apm-java","apm-dotnet","apm-python","apm-node","apm-go","apm-php","apm-other"), n("apm-go-compile","method","Go compile-time instrumentation","Deployment topology","apm-go"), n("apm-ebpf","method","OpenTelemetry eBPF Instrumentation","Deployment topology","apm-go","apm-other"),
		n("k8s-kubeletstats","method","Kubelet Stats receiver","Collector placement","k8s-node-workload"), n("k8s-clusterreceiver","method","Kubernetes Cluster receiver","Collector placement","k8s-cluster-state"), n("k8s-objects","method","Kubernetes Objects receiver","Collector placement","k8s-events"),
		n("logs-filelog","method","File Log receiver","Collector placement","logs-files","logs-container"), n("logs-journald","method","Journald receiver","Collector placement","logs-journal"), n("logs-syslogreceiver","method","Syslog receiver","Collector placement","logs-syslog"), n("logs-windowsevent","method","Windows Event Log receiver","Collector placement","logs-winevent"), n("logs-otlpreceiver","method","OTLP receiver","Collector placement","logs-otlp"),
		n("db-receiver","method","Collector database receiver","Collector placement","db-postgresql","db-mysql","db-mongodb","db-redis","db-sqlserver","db-other","db-managed-postgresql","db-managed-mysql","db-managed-mongodb","db-managed-redis","db-managed-sqlserver","db-managed-other"), n("db-app-spans","method","Application database spans","Collector placement","db-postgresql","db-mysql","db-mongodb","db-redis","db-sqlserver","db-other","db-managed-postgresql","db-managed-mysql","db-managed-mongodb","db-managed-redis","db-managed-sqlserver","db-managed-other"), n("db-cloudmetrics","method","Cloud provider metrics","Collector placement","db-managed-postgresql","db-managed-mysql","db-managed-mongodb","db-managed-redis","db-managed-sqlserver","db-managed-other"),
		n("frontend-websdk","method","OpenTelemetry Web SDK","Export topology","frontend-js"), n("frontend-mobile-sdk","method","OpenTelemetry mobile SDK","Export topology","frontend-android","frontend-ios"),
		n("custom-otlpreceiver","method","OTLP receiver","Collector placement","custom-otlp"), n("custom-promreceiver","method","Prometheus receiver","Collector placement","custom-prometheus"), n("custom-statsdreceiver","method","StatsD receiver","Collector placement","custom-statsd"),

		n("topology-host-agent","topology","Host Collector agent","","apm-auto","apm-sdk","apm-go-compile","apm-ebpf","logs-filelog","logs-journald","logs-syslogreceiver","logs-windowsevent","logs-otlpreceiver","db-receiver","db-app-spans","custom-otlpreceiver","custom-promreceiver","custom-statsdreceiver"), n("topology-operator","topology","OpenTelemetry Operator","","apm-auto","apm-sdk","apm-go-compile"), n("topology-daemonset","topology","Collector DaemonSet","","k8s-kubeletstats","logs-filelog","logs-otlpreceiver","custom-otlpreceiver","custom-promreceiver","custom-statsdreceiver"), n("topology-singleton","topology","Single cluster Collector","","k8s-clusterreceiver","k8s-objects"), n("topology-sidecar","topology","Collector sidecar","","apm-sdk","logs-filelog","logs-otlpreceiver","db-receiver","db-app-spans","custom-otlpreceiver","custom-promreceiver","custom-statsdreceiver"), n("topology-gateway","topology","Central Collector gateway","","apm-auto","apm-sdk","apm-go-compile","apm-ebpf","logs-syslogreceiver","logs-otlpreceiver","db-receiver","db-app-spans","db-cloudmetrics","frontend-websdk","frontend-mobile-sdk","custom-otlpreceiver","custom-promreceiver","custom-statsdreceiver"),
	}
	details := map[string][4]string{
		"infra-hostmetrics":{"Host system metrics","Collects CPU, memory, load, disk, filesystem, network and paging metrics without application instrumentation.","OpenTelemetry Collector with the hostmetrics receiver running with host access.","Deploy as an agent and review filesystem mounts, process permissions and collection interval."},
		"infra-process":{"Process metrics","Shows resource consumption and process health without requiring a language agent.","Host process visibility and permissions appropriate to the operating system.","Process-level labels can create high cardinality; limit collection to required processes."},
		"infra-kubelet":{"Kubernetes node and workload metrics","Collects node, pod and container metrics directly from kubelet endpoints.","Kubelet Stats receiver, service account permissions and node-local network access.","Use a DaemonSet and verify authentication, TLS and metric volume."},
		"infra-cloudmetrics":{"Cloud infrastructure metrics","Uses provider APIs for managed compute, load balancers, storage and related resources.","Cloud receiver or API integration with least-privilege credentials.","Provider APIs can introduce delay, quotas and additional cost."},
		"apm-auto":{"Zero-code auto-instrumentation","Fast onboarding with framework-aware traces and runtime metrics.","A supported runtime, compatible libraries and an application restart.","Verify current language support and measure startup and runtime overhead."},
		"apm-sdk":{"OpenTelemetry SDK","Provides custom spans, metrics and business context with the most control.","Application code, build and release changes.","Application teams own SDK lifecycle and semantic quality."},
		"apm-go-compile":{"Go compile-time instrumentation","Adds stable zero-code instrumentation during build without a runtime attach step.","Control of the Go build pipeline and supported instrumentation rules.","Rebuild binaries for instrumentation updates and validate third-party package coverage."},
		"apm-ebpf":{"OpenTelemetry eBPF Instrumentation","Offers low-friction service discovery and baseline tracing for supported Linux workloads.","Compatible Linux kernel and the required elevated capabilities.","Coverage and business context are more limited than code-based instrumentation."},
		"db-receiver":{"Collector database receiver","Collects database health and performance metrics without changing application code.","A receiver supported by the chosen Collector distribution, network access and read-only monitoring credentials.","Receiver maturity and metric coverage vary by database; verify against the deployed Collector version."},
		"db-app-spans":{"Application database spans","Connects database calls to end-to-end application traces.","Supported client-library instrumentation in the calling application.","This complements database metrics; it does not replace database server monitoring."},
		"db-cloudmetrics":{"Cloud provider database metrics","Uses the managed-service monitoring API without database host access.","Provider integration and least-privilege cloud permissions.","Sampling intervals, dimensions and API costs are controlled by the provider."},
	}
	for i := range nodes {
		nodes[i].SortOrder=i
		if d, ok := details[nodes[i].ID]; ok { nodes[i].Description=d[0];nodes[i].Advantages=d[1];nodes[i].Requirements=d[2];nodes[i].Considerations=d[3] }
	}
	steps := map[string][]string{
		"infra-hostmetrics":{"Deploy the Collector as an agent on each target host.","Enable only the required hostmetrics scrapers and resource detection.","Validate host identity, permissions, metric volume and Collector self-observability."},
		"infra-process":{"Define which process metrics are operationally useful.","Grant only the process visibility required by the Collector.","Filter noisy processes and validate label cardinality before broad rollout."},
		"infra-kubelet":{"Deploy a Collector DaemonSet with one instance per selected node.","Configure kubelet authentication, TLS and the required metric groups.","Validate node, pod and container identity before scaling cluster-wide."},
		"infra-cloudmetrics":{"Select the provider receiver or approved cloud integration.","Create least-privilege credentials and store them as managed secrets.","Validate API limits, metric delay, dimensions and cost before production rollout."},
		"topology-host-agent":{"Install a managed Collector on each target host.","Send OTLP to an approved gateway or destination.","Validate telemetry and Collector self-observability."},
		"topology-operator":{"Install the OpenTelemetry Operator with required cluster permissions.","Define Instrumentation resources for selected namespaces and workloads.","Roll out gradually and validate injected applications."},
		"topology-daemonset":{"Deploy one Collector per selected Kubernetes node.","Configure node-local collection, enrichment and secure export.","Validate RBAC, resource usage and scheduling coverage."},
		"topology-singleton":{"Deploy one or a small elected set of cluster Collectors.","Grant only the Kubernetes API permissions required by the receivers.","Prevent duplicate cluster-wide collection and monitor availability."},
		"topology-sidecar":{"Add a Collector sidecar to the workload template.","Route workload telemetry to its local Collector.","Account for per-pod resources and coordinated rollouts."},
		"topology-gateway":{"Deploy redundant Collector gateways behind a stable endpoint.","Configure TLS, memory protection, batching and routing.","Load-test capacity and monitor queues and export failures."},
	}
	for i := range nodes { if v := steps[nodes[i].ID]; v != nil { nodes[i].Steps=v } }
	return Catalog{SchemaVersion:CurrentSchemaVersion, Nodes:nodes}
}
