package guides

import "testing"

func TestDefaultCatalogIsValid(t *testing.T) {
	if err := DefaultCatalog().Validate(); err != nil { t.Fatal(err) }
}

func TestCatalogRejectsBrokenParent(t *testing.T) {
	catalog := DefaultCatalog()
	catalog.Nodes = append(catalog.Nodes, Node{ID:"mobile", Stage:"platform", Name:"Mobile", ParentIDs:[]string{"missing"}, Enabled:true})
	if err := catalog.Validate(); err == nil { t.Fatal("expected missing parent validation error") }
}

func TestCatalogRequiresPrecedingStageParent(t *testing.T) {
	catalog := DefaultCatalog()
	catalog.Nodes = append(catalog.Nodes, Node{ID:"bad", Stage:"technology", Name:"Bad path", ParentIDs:[]string{"apm"}, Enabled:true})
	if err := catalog.Validate(); err == nil { t.Fatal("expected stage validation error") }
}

func TestInfrastructurePathStopsAtRelevantMetrics(t *testing.T) {
	catalog := DefaultCatalog()
	byID := map[string]Node{}
	for _, node := range catalog.Nodes { byID[node.ID] = node }
	if byID["infra-linux"].NextStageLabel != "Infrastructure metrics" { t.Fatalf("infrastructure platform next label=%q", byID["infra-linux"].NextStageLabel) }
	for _, id := range []string{"infra-hostmetrics", "infra-process", "infra-kubelet", "infra-cloudmetrics"} {
		if byID[id].Stage != "technology" || byID[id].NextStageLabel != "" { t.Fatalf("%s is not a terminal infrastructure metric: %#v", id, byID[id]) }
	}
	for _, id := range []string{"apm-java", "apm-python", "apm-go"} {
		for _, parent := range byID[id].ParentIDs {
			if parent == "infra-linux" || parent == "infra-windows" || parent == "infra-kubernetes" || parent == "infra-cloud" { t.Fatalf("application technology %s leaked into infrastructure", id) }
		}
	}
}

func TestDatabasePathUsesDatabaseSpecificDecisions(t *testing.T) {
	catalog := DefaultCatalog()
	byID := map[string]Node{}
	for _, node := range catalog.Nodes { byID[node.ID] = node }
	if byID["db-host"].NextStageLabel != "Database engine" { t.Fatalf("database platform next label=%q", byID["db-host"].NextStageLabel) }
	if byID["db-postgresql"].NextStageLabel != "Collection approach" { t.Fatalf("database engine next label=%q", byID["db-postgresql"].NextStageLabel) }
	for _, parent := range byID["db-cloudmetrics"].ParentIDs { if parent == "db-postgresql" || parent == "db-mysql" || parent == "db-other" { t.Fatalf("cloud database metrics leaked into a self-managed database path: %#v", byID["db-cloudmetrics"] ) } }
}
