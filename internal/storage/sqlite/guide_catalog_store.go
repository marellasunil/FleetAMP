package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/marellasunil/FleetAMP/internal/guides"
)

type GuideCatalogStore struct{ db *sql.DB }

func catalogJSON(c guides.Catalog) (string, error) { b, e := json.Marshal(c); return string(b), e }
func parseCatalog(raw string) (guides.Catalog, error) { var c guides.Catalog; e := json.Unmarshal([]byte(raw), &c); return c, e }

func (s *GuideCatalogStore) seed(ctx context.Context) error {
	defaultCatalog := guides.DefaultCatalog()
	raw, err := catalogJSON(defaultCatalog)
	if err != nil { return err }
	if _, err = s.db.ExecContext(ctx, `INSERT OR IGNORE INTO guide_catalog_versions(version,content,created_by,created_at,published_at) VALUES(1,?,'system',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`, raw); err != nil { return err }
	if _, err = s.db.ExecContext(ctx, `INSERT OR IGNORE INTO guide_catalog_state(singleton,active_version) VALUES(1,1)`); err != nil { return err }
	if _, err = s.db.ExecContext(ctx, `INSERT OR IGNORE INTO guide_catalog_draft(singleton,content,base_version,updated_by,updated_at) VALUES(1,?,1,'system',CURRENT_TIMESTAMP)`, raw); err != nil { return err }

	var activeVersion int
	var activeRaw, activeCreator, draftUpdater string
	if err = s.db.QueryRowContext(ctx, `SELECT v.version,v.content,v.created_by,d.updated_by FROM guide_catalog_state s JOIN guide_catalog_versions v ON v.version=s.active_version JOIN guide_catalog_draft d ON d.singleton=1 WHERE s.singleton=1`).Scan(&activeVersion, &activeRaw, &activeCreator, &draftUpdater); err != nil { return err }
	activeCatalog, err := parseCatalog(activeRaw)
	if err != nil { return err }
	if activeCatalog.SchemaVersion >= guides.CurrentSchemaVersion || activeCreator != "system" || draftUpdater != "system" { return nil }

	// Upgrade untouched built-in catalogs without overwriting an organization's
	// published or in-progress custom guide.
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil { return err }
	defer tx.Rollback()
	var next int
	if err = tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(version),0)+1 FROM guide_catalog_versions`).Scan(&next); err != nil { return err }
	if _, err = tx.ExecContext(ctx, `INSERT INTO guide_catalog_versions(version,content,created_by,created_at,published_at) VALUES(?,?,'system',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`, next, raw); err != nil { return err }
	if _, err = tx.ExecContext(ctx, `UPDATE guide_catalog_state SET active_version=? WHERE singleton=1`, next); err != nil { return err }
	if _, err = tx.ExecContext(ctx, `UPDATE guide_catalog_draft SET content=?,base_version=?,updated_by='system',updated_at=CURRENT_TIMESTAMP WHERE singleton=1`, raw, next); err != nil { return err }
	return tx.Commit()
}

func (s *GuideCatalogStore) Draft(ctx context.Context) (guides.Catalog, error) {
	if err := s.seed(ctx); err != nil { return guides.Catalog{}, err }
	var raw string
	if err := s.db.QueryRowContext(ctx, `SELECT content FROM guide_catalog_draft WHERE singleton=1`).Scan(&raw); err != nil { return guides.Catalog{}, err }
	return parseCatalog(raw)
}

func (s *GuideCatalogStore) SaveDraft(ctx context.Context, c guides.Catalog, actor string) error {
	if err := c.Validate(); err != nil { return err }
	if c.SchemaVersion == 0 { c.SchemaVersion = guides.CurrentSchemaVersion }
	raw, err := catalogJSON(c)
	if err != nil { return err }
	_, err = s.db.ExecContext(ctx, `UPDATE guide_catalog_draft SET content=?,updated_by=?,updated_at=CURRENT_TIMESTAMP WHERE singleton=1`, raw, actor)
	return err
}

func scanGuideVersion(row interface{ Scan(...any) error }) (guides.Version, error) {
	var v guides.Version
	var raw, created, published string
	err := row.Scan(&v.Version, &raw, &v.CreatedBy, &created, &published)
	if err != nil { return v, err }
	v.Catalog, err = parseCatalog(raw)
	if err != nil { return v, err }
	v.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", created)
	v.PublishedAt, _ = time.Parse("2006-01-02 15:04:05", published)
	return v, nil
}

func (s *GuideCatalogStore) Active(ctx context.Context) (guides.Version, error) {
	if err := s.seed(ctx); err != nil { return guides.Version{}, err }
	return scanGuideVersion(s.db.QueryRowContext(ctx, `SELECT v.version,v.content,v.created_by,v.created_at,v.published_at FROM guide_catalog_versions v JOIN guide_catalog_state s ON s.active_version=v.version WHERE s.singleton=1`))
}

func (s *GuideCatalogStore) Publish(ctx context.Context, actor string) (guides.Version, error) {
	catalog, err := s.Draft(ctx)
	if err != nil { return guides.Version{}, err }
	if err = catalog.Validate(); err != nil { return guides.Version{}, err }
	raw, _ := catalogJSON(catalog)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil { return guides.Version{}, err }
	defer tx.Rollback()
	var next int
	if err = tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(version),0)+1 FROM guide_catalog_versions`).Scan(&next); err != nil { return guides.Version{}, err }
	if _, err = tx.ExecContext(ctx, `INSERT INTO guide_catalog_versions(version,content,created_by,created_at,published_at) VALUES(?,?,?,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`, next, raw, actor); err != nil { return guides.Version{}, err }
	if _, err = tx.ExecContext(ctx, `UPDATE guide_catalog_state SET active_version=? WHERE singleton=1`, next); err != nil { return guides.Version{}, err }
	if _, err = tx.ExecContext(ctx, `UPDATE guide_catalog_draft SET base_version=?,updated_by=?,updated_at=CURRENT_TIMESTAMP WHERE singleton=1`, next, actor); err != nil { return guides.Version{}, err }
	if err = tx.Commit(); err != nil { return guides.Version{}, err }
	return s.Active(ctx)
}

func (s *GuideCatalogStore) Versions(ctx context.Context) ([]guides.Version, error) {
	if err := s.seed(ctx); err != nil { return nil, err }
	rows, err := s.db.QueryContext(ctx, `SELECT version,content,created_by,created_at,published_at FROM guide_catalog_versions ORDER BY version DESC`)
	if err != nil { return nil, err }
	defer rows.Close()
	var result []guides.Version
	for rows.Next() { version, scanErr := scanGuideVersion(rows); if scanErr != nil { return nil, scanErr }; result = append(result, version) }
	return result, rows.Err()
}

func (s *GuideCatalogStore) Rollback(ctx context.Context, version int, actor string) (guides.Version, error) {
	var raw string
	if err := s.db.QueryRowContext(ctx, `SELECT content FROM guide_catalog_versions WHERE version=?`, version).Scan(&raw); err != nil { return guides.Version{}, fmt.Errorf("guide version %d not found", version) }
	catalog, err := parseCatalog(raw)
	if err != nil { return guides.Version{}, err }
	if err = s.SaveDraft(ctx, catalog, actor); err != nil { return guides.Version{}, err }
	return s.Publish(ctx, actor)
}
