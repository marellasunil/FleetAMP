package runtimes

// InstallationGuide describes a supported installation decision without
// coupling FleetAMP to a package manager, Kubernetes client, or Git provider.
// Guides are informational until a later lifecycle request is approved.
type InstallationGuide struct {
	ID               string
	Name             string
	RuntimeType      Type
	Summary          string
	SuitableFor      []string
	DeploymentMethod string
	Owner            string
	Prerequisites    []string
	Steps            []string
	Verification     []string
}

// DefaultInstallationGuides returns the built-in, vendor-neutral installation
// decision guide. External vendor collectors remain in the add-on catalog.
func DefaultInstallationGuides() []InstallationGuide {
	guides := []InstallationGuide{
		{
			ID: "linux-service", Name: "Linux system service", RuntimeType: OTelCollector,
			Summary: "Run an OpAMP-supervised Collector as a host service with a stable identity and persistent supervisor state.",
			SuitableFor: []string{"Host metrics and logs", "VM agent", "Bare-metal agent", "Gateway"},
			DeploymentMethod: "Package or binary + systemd", Owner: "Platform or SRE team",
			Prerequisites: []string{"Supported Linux host and service account", "FleetAMP OpAMP endpoint and trust material", "Persistent directories for supervisor identity and Collector configuration"},
			Steps: []string{"Install the Supervisor and Collector binaries", "Configure OpAMP endpoint, authentication and stable identity", "Register the system service and start it", "Confirm health and effective configuration in FleetAMP"},
			Verification: []string{"Service is active", "Collector appears in Installed inventory", "Compatibility checks report required capabilities"},
		},
		{
			ID: "container", Name: "Container runtime", RuntimeType: OTelCollector,
			Summary: "Run a supervised Collector in a container while preserving identity and configuration state outside the container filesystem.",
			SuitableFor: []string{"Local lab", "Container host", "Portable gateway"},
			DeploymentMethod: "Docker or compatible runtime", Owner: "Platform or container operations team",
			Prerequisites: []string{"Pinned Supervisor and Collector images", "Persistent volume for supervisor identity", "Secret injection for OpAMP authentication", "Outbound connectivity to FleetAMP"},
			Steps: []string{"Create persistent identity storage", "Mount supervisor configuration and trust material", "Start the supervised Collector", "Confirm registration and reported capabilities"},
			Verification: []string{"Container is healthy", "Identity survives restart", "FleetAMP reports effective configuration"},
		},
		{
			ID: "kubernetes-workload", Name: "Kubernetes workload", RuntimeType: OTelCollectorKubernetes,
			Summary: "Deploy Collector agents or gateways as Kubernetes workloads while the cluster platform remains responsible for scheduling and replicas.",
			SuitableFor: []string{"DaemonSet agent", "Deployment gateway", "Stateful gateway", "Sidecar"},
			DeploymentMethod: "Helm, Kustomize or manifests", Owner: "Kubernetes platform team",
			Prerequisites: []string{"Namespace and workload ownership", "ServiceAccount and minimum RBAC", "Kubernetes Secret for credentials", "Network policy allowing FleetAMP and telemetry endpoints"},
			Steps: []string{"Choose agent or gateway topology", "Render workload, service and security resources", "Deploy through the organization's delivery mechanism", "Confirm every Collector identity in FleetAMP"},
			Verification: []string{"Workload is ready", "Expected replicas are registered", "Cluster, namespace and workload metadata are reported"},
		},
		{
			ID: "otel-operator", Name: "OpenTelemetry Operator", RuntimeType: OTelOperator,
			Summary: "Use the upstream Operator and Collector custom resources when the organization accepts an in-cluster lifecycle controller.",
			SuitableFor: []string{"Operator-managed Collector", "Auto-instrumentation resources", "Kubernetes-native lifecycle"},
			DeploymentMethod: "Helm or Operator manifests", Owner: "Kubernetes platform team",
			Prerequisites: []string{"Approved Operator and CRD versions", "Cluster-scoped RBAC review", "Defined ownership for upgrades and CRD changes", "Compatibility validation before rollout"},
			Steps: []string{"Approve Operator and CRD versions", "Install the Operator through the platform delivery path", "Create governed Collector resources", "Verify Operator and Collector health separately"},
			Verification: []string{"Operator deployment is healthy", "Collector custom resources are reconciled", "Managed Collectors register with FleetAMP"},
		},
		{
			ID: "gitops", Name: "GitOps-managed Kubernetes", RuntimeType: OTelCollectorKubernetes,
			Summary: "Store approved desired state in Git and let the organization's existing GitOps controller apply it to selected clusters.",
			SuitableFor: []string{"Regulated environments", "Multi-cluster rollout", "Repository-owned desired state"},
			DeploymentMethod: "Git pull request + GitOps controller", Owner: "Application owner and platform approver",
			Prerequisites: []string{"Connected Git provider", "Repository path and branch policy", "Existing GitOps controller", "FleetAMP group or label target mapping"},
			Steps: []string{"Generate an immutable desired-state proposal", "Validate target, policy and compatibility", "Approve and merge through repository controls", "Observe reconciliation and verify registered Collectors"},
			Verification: []string{"Approved commit is reconciled", "Target workload is healthy", "FleetAMP records expected identities and configuration state"},
		},
	}
	return cloneInstallationGuides(guides)
}

func cloneInstallationGuides(source []InstallationGuide) []InstallationGuide {
	result := make([]InstallationGuide, len(source))
	for index, guide := range source {
		result[index] = guide
		result[index].SuitableFor = append([]string(nil), guide.SuitableFor...)
		result[index].Prerequisites = append([]string(nil), guide.Prerequisites...)
		result[index].Steps = append([]string(nil), guide.Steps...)
		result[index].Verification = append([]string(nil), guide.Verification...)
	}
	return result
}
