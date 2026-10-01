package storage
import("context";"github.com/marellasunil/FleetAMP/internal/guides")
type GuideCatalogStore interface{Draft(context.Context)(guides.Catalog,error);SaveDraft(context.Context,guides.Catalog,string)error;Active(context.Context)(guides.Version,error);Publish(context.Context,string)(guides.Version,error);Versions(context.Context)([]guides.Version,error);Rollback(context.Context,int,string)(guides.Version,error)}
