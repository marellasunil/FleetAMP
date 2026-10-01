// Package guides defines the organization-managed Instrumentation Guide graph.
package guides

import (
	"fmt"
	"strings"
	"time"
)

var Stages = []string{"capability", "platform", "technology", "method", "topology"}

type Node struct {
	ID string `json:"id"`; Stage string `json:"stage"`; Name string `json:"name"`; Description string `json:"description"`
	ParentIDs []string `json:"parent_ids,omitempty"`; Advantages string `json:"advantages,omitempty"`; Requirements string `json:"requirements,omitempty"`; Considerations string `json:"considerations,omitempty"`; Steps []string `json:"steps,omitempty"`
	Enabled bool `json:"enabled"`; SortOrder int `json:"sort_order"`
}
type Catalog struct { Nodes []Node `json:"nodes"` }
type Version struct { Version int; Catalog Catalog; CreatedBy string; CreatedAt time.Time; PublishedAt time.Time }

func stageIndex(stage string) int { for i,v:=range Stages { if stage==v{return i} }; return -1 }
func (c Catalog) Validate() error {
	if len(c.Nodes)==0{return fmt.Errorf("guide must contain at least one node")}; seen:=map[string]Node{}
	for _,n:=range c.Nodes { n.ID=strings.TrimSpace(n.ID);n.Name=strings.TrimSpace(n.Name);n.Stage=strings.TrimSpace(n.Stage);if n.ID==""||n.Name==""||stageIndex(n.Stage)<0{return fmt.Errorf("every node needs a valid ID, name and stage")};if _,ok:=seen[n.ID];ok{return fmt.Errorf("duplicate node ID %q",n.ID)};seen[n.ID]=n }
	for _,n:=range c.Nodes { i:=stageIndex(n.Stage);if i==0&&len(n.ParentIDs)>0{return fmt.Errorf("capability %q cannot have a parent",n.Name)};if i>0&&n.Enabled&&len(n.ParentIDs)==0{return fmt.Errorf("%s %q needs at least one parent",n.Stage,n.Name)};for _,pid:=range n.ParentIDs { p,ok:=seen[pid];if !ok{return fmt.Errorf("%q references missing parent %q",n.Name,pid)};if stageIndex(p.Stage)!=i-1{return fmt.Errorf("%q parent %q must be in the preceding stage",n.Name,p.Name)} } }
	return nil
}

func DefaultCatalog() Catalog {
	n:=func(id,stage,name string,parents ...string)Node{return Node{ID:id,Stage:stage,Name:name,ParentIDs:parents,Enabled:true}}
	nodes:=[]Node{
		n("apm","capability","Application performance"),n("infrastructure","capability","Infrastructure"),n("kubernetes","capability","Kubernetes"),n("logs","capability","Logs"),n("database","capability","Database"),n("frontend","capability","Frontend"),n("custom","capability","Custom telemetry"),
		n("linux","platform","Linux / VM","apm","infrastructure","logs","database","custom"),n("k8s-platform","platform","Kubernetes","apm","infrastructure","kubernetes","logs","database","custom"),n("serverless","platform","Serverless","apm","logs","custom"),n("browser","platform","Browser","frontend"),n("other","platform","Other / OTLP","apm","infrastructure","logs","database","custom"),
		n("java","technology","Java","linux","k8s-platform","serverless"),n("dotnet","technology",".NET","linux","k8s-platform","serverless"),n("python","technology","Python","linux","k8s-platform","serverless"),n("javascript","technology","JavaScript / Node.js","linux","k8s-platform","serverless","browser"),n("go","technology","Go","linux","k8s-platform","serverless"),n("host","technology","Host metrics","linux","other"),n("cluster","technology","Cluster and workloads","k8s-platform"),n("filelog","technology","File / syslog / journald","linux","k8s-platform","other"),n("sql","technology","SQL / database receiver","linux","k8s-platform","other"),n("generic","technology","Generic OTLP source","linux","k8s-platform","serverless","browser","other"),
		n("auto","method","Auto-instrumentation","java","dotnet","python","javascript"),n("sdk","method","OpenTelemetry SDK","java","dotnet","python","javascript","go","generic"),n("receiver","method","Collector receiver","host","cluster","filelog","sql","generic"),n("ebpf","method","eBPF / OBI","host","cluster"),
		n("agent","topology","Host agent","auto","sdk","receiver","ebpf"),n("operator","topology","OTel Operator","auto","sdk","receiver"),n("daemonset","topology","DaemonSet","receiver","ebpf"),n("sidecar","topology","Sidecar","auto","sdk","receiver"),n("gateway","topology","Central gateway","auto","sdk","receiver","ebpf"),n("direct","topology","Direct OTLP export","auto","sdk"),
	}
	details:=map[string][4]string{"auto":{"Automatic instrumentation","Fast onboarding and framework-aware traces.","Supported runtime, injection method and application restart.","Check runtime compatibility and measure overhead."},"sdk":{"OpenTelemetry SDK","Maximum control and rich business context.","Application code, build and release changes.","Teams own SDK lifecycle and semantic quality."},"receiver":{"Collector receiver","Reusable collection with centralized processing.","Source connectivity, credentials and receiver permissions.","Plan interval, volume, cardinality and availability."},"ebpf":{"eBPF / OBI","Low-friction discovery and broad baseline visibility.","Compatible Linux kernel and elevated capabilities.","Coverage varies and business context is more limited."}}
	for i:=range nodes { nodes[i].SortOrder=i;if d,ok:=details[nodes[i].ID];ok{nodes[i].Description=d[0];nodes[i].Advantages=d[1];nodes[i].Requirements=d[2];nodes[i].Considerations=d[3]} }
	steps:=map[string][]string{"agent":{"Install a managed Collector or language agent on the host.","Send OTLP to an approved gateway or backend.","Validate telemetry and Collector self-observability."},"operator":{"Install the OpenTelemetry Operator with required cluster permissions.","Define Instrumentation and Collector resources.","Roll out gradually and validate injected workloads."},"daemonset":{"Deploy a Collector DaemonSet on selected nodes.","Configure node-local receivers, enrichment and secure export.","Validate RBAC, resource usage and scheduling coverage."},"sidecar":{"Add a Collector sidecar to the workload template.","Route application telemetry to the local sidecar.","Account for per-pod resources and joint rollouts."},"gateway":{"Deploy redundant Collector gateways behind a stable endpoint.","Configure TLS, batching, memory protection and routing.","Load-test capacity and monitor queues and export failures."},"direct":{"Configure the SDK or runtime for direct OTLP export.","Use TLS and managed endpoint credentials.","Add a gateway when shared processing or buffering is required."}}
	for i:=range nodes { if v:=steps[nodes[i].ID];v!=nil{nodes[i].Steps=v} };return Catalog{Nodes:nodes}
}
