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
