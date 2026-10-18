package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
	"github.com/marellasunil/FleetAMP/internal/distributions"
)

type DistributionStore struct{ db *sql.DB }

func (s *DistributionStore) Publish(ctx context.Context, d *distributions.Distribution) error {
	if d == nil { return fmt.Errorf("distribution is required") }
	if err := d.Validate(); err != nil { return err }
	components, _ := json.Marshal(d.Components); locked, _ := json.Marshal(d.LockedSections); editable, _ := json.Marshal(d.EditableSections)
	_, err := s.db.ExecContext(ctx, `INSERT INTO collector_distributions(id,name,collector_version,source_repository,source_ref,artifact_uri,artifact_sha256,components,locked_sections,editable_sections,spec_hash,published_by,published_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`, d.ID,d.Name,d.CollectorVersion,d.SourceRepository,d.SourceRef,d.ArtifactURI,d.ArtifactSHA256,string(components),string(locked),string(editable),d.SpecHash,d.PublishedBy,d.PublishedAt.Format(time.RFC3339Nano))
	if err != nil { return fmt.Errorf("publish Collector distribution: %w", err) }
	return nil
}

func scanDistribution(row interface{ Scan(...any) error }) (*distributions.Distribution,error) {
	d:=&distributions.Distribution{}; var components,locked,editable,published string
	if err:=row.Scan(&d.ID,&d.Name,&d.CollectorVersion,&d.SourceRepository,&d.SourceRef,&d.ArtifactURI,&d.ArtifactSHA256,&components,&locked,&editable,&d.SpecHash,&d.PublishedBy,&published); err!=nil{return nil,err}
	if err:=json.Unmarshal([]byte(components),&d.Components);err!=nil{return nil,err}; _=json.Unmarshal([]byte(locked),&d.LockedSections); _=json.Unmarshal([]byte(editable),&d.EditableSections); d.PublishedAt,_=time.Parse(time.RFC3339Nano,published); return d,nil
}

func (s *DistributionStore) List(ctx context.Context)([]*distributions.Distribution,error){ rows,err:=s.db.QueryContext(ctx,`SELECT id,name,collector_version,source_repository,source_ref,artifact_uri,artifact_sha256,components,locked_sections,editable_sections,spec_hash,published_by,published_at FROM collector_distributions ORDER BY published_at DESC`);if err!=nil{return nil,err};defer rows.Close();var out []*distributions.Distribution;for rows.Next(){d,e:=scanDistribution(rows);if e!=nil{return nil,e};out=append(out,d)};return out,rows.Err() }
func (s *DistributionStore) Get(ctx context.Context,id string)(*distributions.Distribution,error){ d,err:=scanDistribution(s.db.QueryRowContext(ctx,`SELECT id,name,collector_version,source_repository,source_ref,artifact_uri,artifact_sha256,components,locked_sections,editable_sections,spec_hash,published_by,published_at FROM collector_distributions WHERE id=?`,id));if err==sql.ErrNoRows{return nil,fmt.Errorf("distribution %q not found",id)};return d,err }
