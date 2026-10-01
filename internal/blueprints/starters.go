package blueprints

// Starter maps a common observability use case to an approved Pattern and
// deployment approach. Destination settings deliberately remain separate.
type Starter struct {
	ID, Name, Description, Category, Goal, Platform, Technology, Method, Topology string
	Signals, Outcomes, Requirements                                                   []string
	Pattern                                                                           *Pattern
}

// CommonStarters returns conservative starting points useful in most organizations.
func CommonStarters() []*Starter {
	application := NewPattern("OTLP application service", "Receive application metrics, traces and logs over OTLP.", "application", "otlp", "protocols:\n  grpc: {}\n  http: {}", []string{"metrics", "traces", "logs"})
	host := NewPattern("Linux host baseline", "Collect CPU, memory, disk, filesystem, load, network and process metrics.", "linux", "hostmetrics", "collection_interval: 30s\nscrapers:\n  cpu: {}\n  memory: {}\n  disk: {}\n  filesystem: {}\n  load: {}\n  network: {}\n  processes: {}", []string{"metrics"})
	kubernetes := NewPattern("Kubernetes cluster baseline", "Collect Kubernetes cluster inventory and state metrics.", "kubernetes", "k8s_cluster", "collection_interval: 30s\nnode_conditions_to_report: [Ready, MemoryPressure, DiskPressure, PIDPressure]\nallocatable_types_to_report: [cpu, memory]", []string{"metrics"})
	gateway := NewPattern("Central OTLP gateway", "Receive all three telemetry signals through a shared Collector gateway.", "any", "otlp", "protocols:\n  grpc:\n    endpoint: 0.0.0.0:4317\n  http:\n    endpoint: 0.0.0.0:4318", []string{"metrics", "traces", "logs"})
	logs := NewPattern("Application log collection", "Tail structured application logs and preserve file identity across restarts.", "linux", "filelog", "include: [/var/log/apps/*.log]\nstart_at: end\ninclude_file_name: true\ninclude_file_path: true", []string{"logs"})
	prometheus := NewPattern("Prometheus metrics collection", "Scrape an approved Prometheus endpoint through the Collector.", "any", "prometheus", "config:\n  scrape_configs:\n    - job_name: application\n      scrape_interval: 30s\n      static_configs:\n        - targets: [localhost:9090]", []string{"metrics"})
	return []*Starter{
		{ID: "application-apm", Name: "Application APM", Description: "A production baseline for services using an OpenTelemetry SDK or auto-instrumentation.", Category: "Applications", Goal: "apm", Platform: "application", Technology: "OpenTelemetry", Method: "gateway", Topology: "Application → Collector gateway → approved destination", Signals: []string{"metrics", "traces", "logs"}, Outcomes: []string{"Service performance", "Distributed traces", "Correlated logs"}, Requirements: []string{"OTLP-capable application", "Reachable Collector endpoint"}, Pattern: application},
		{ID: "linux-host", Name: "Linux Host Baseline", Description: "Standard infrastructure coverage for virtual machines and physical Linux hosts.", Category: "Infrastructure", Goal: "infrastructure", Platform: "linux", Technology: "Linux", Method: "collector-agent", Topology: "Host Collector → approved destination", Signals: []string{"metrics"}, Outcomes: []string{"Resource saturation", "Filesystem capacity", "Process health"}, Requirements: []string{"Collector installed as an agent", "Host metric permissions"}, Pattern: host},
		{ID: "kubernetes-cluster", Name: "Kubernetes Cluster Baseline", Description: "Cluster inventory and health metrics using one Collector deployment per cluster.", Category: "Kubernetes", Goal: "kubernetes", Platform: "kubernetes", Technology: "Kubernetes", Method: "deployment", Topology: "Cluster Collector → approved destination", Signals: []string{"metrics"}, Outcomes: []string{"Workload inventory", "Node conditions", "Cluster capacity"}, Requirements: []string{"Kubernetes RBAC", "Collector service account"}, Pattern: kubernetes},
		{ID: "central-otlp-gateway", Name: "Central OTLP Gateway", Description: "A shared ingestion tier that decouples workloads from observability backends.", Category: "Platform", Goal: "custom", Platform: "any", Technology: "OpenTelemetry", Method: "gateway", Topology: "Workloads → OTLP gateway → approved destination", Signals: []string{"metrics", "traces", "logs"}, Outcomes: []string{"Central policy point", "Backend isolation", "Consistent batching"}, Requirements: []string{"Load-balanced gateway service", "Network path from workloads"}, Pattern: gateway},
		{ID: "application-logs", Name: "Application Logs", Description: "A safe starting point for collecting file-based application logs from Linux hosts.", Category: "Logs", Goal: "logs", Platform: "linux", Technology: "File logs", Method: "collector-agent", Topology: "Log files → Host Collector → approved destination", Signals: []string{"logs"}, Outcomes: []string{"Central log search", "File source attribution", "Restart-safe collection"}, Requirements: []string{"Readable log path", "Organization-specific include path"}, Pattern: logs},
		{ID: "prometheus-metrics", Name: "Prometheus Metrics", Description: "Bring existing Prometheus-format endpoints into the governed OpenTelemetry pipeline.", Category: "Metrics", Goal: "custom", Platform: "any", Technology: "Prometheus", Method: "gateway", Topology: "Prometheus endpoint → Collector → approved destination", Signals: []string{"metrics"}, Outcomes: []string{"Existing metric reuse", "Governed export", "Central scrape policy"}, Requirements: []string{"Reachable metrics endpoint", "Organization-specific scrape target"}, Pattern: prometheus},
	}
}

func CommonSafetyBlocks() []*Block {
	return []*Block{
		NewBlock("Memory limiter", "Protect the Collector from memory exhaustion.", "processors", "memory_limiter", "check_interval: 1s\nlimit_mib: 512\nspike_limit_mib: 128", []string{"metrics", "traces", "logs"}, []string{"any"}, true, true),
		NewBlock("Batch processor", "Batch telemetry before export.", "processors", "batch", "timeout: 5s", []string{"metrics", "traces", "logs"}, []string{"any"}, true, true),
	}
}
