package sqlite

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/marellasunil/FleetAMP/internal/guides"
)

func TestGuideCatalogPublishAndRollback(t *testing.T) {
	ctx:=context.Background();db,err:=Open(ctx,filepath.Join(t.TempDir(),"guide.db"));if err!=nil{t.Fatal(err)};defer db.Close();store:=db.GuideCatalog()
	active,err:=store.Active(ctx);if err!=nil{t.Fatal(err)};if active.Version!=1{t.Fatalf("initial version=%d, want 1",active.Version)}
	draft,err:=store.Draft(ctx);if err!=nil{t.Fatal(err)};draft.Nodes=append(draft.Nodes,guides.Node{ID:"mobile",Stage:"platform",Name:"Mobile",ParentIDs:[]string{"frontend"},Enabled:true,SortOrder:999})
	if err:=store.SaveDraft(ctx,draft,"admin");err!=nil{t.Fatal(err)};published,err:=store.Publish(ctx,"admin");if err!=nil{t.Fatal(err)};if published.Version!=2{t.Fatalf("published version=%d, want 2",published.Version)}
	rolled,err:=store.Rollback(ctx,1,"admin");if err!=nil{t.Fatal(err)};if rolled.Version!=3{t.Fatalf("rollback version=%d, want 3",rolled.Version)};for _,n:=range rolled.Catalog.Nodes{if n.ID=="mobile"{t.Fatal("rollback retained node added in version 2")}}
}
