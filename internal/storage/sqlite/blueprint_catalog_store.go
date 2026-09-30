package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/marellasunil/FleetAMP/internal/blueprints"
	"github.com/marellasunil/FleetAMP/internal/storage"
)

func catalogStrings(values []string) string { b, _ := json.Marshal(values); return string(b) }
func scanCatalogStrings(raw string) []string { var values []string; _ = json.Unmarshal([]byte(raw), &values); return values }

func (s *DestinationProfileStore) CreatePattern(ctx context.Context, p *blueprints.Pattern) error {
	if p == nil { return fmt.Errorf("blueprint pattern is required") }
	_, err := s.db.ExecContext(ctx, `INSERT INTO blueprint_patterns(id,name,description,platform,receiver_id,receiver_config,signals,enabled,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?)`,
		p.ID,p.Name,p.Description,p.Platform,p.ReceiverID,p.ReceiverConfig,catalogStrings(p.Signals),boolToInt(p.Enabled),p.CreatedAt.Format(time.RFC3339Nano),p.UpdatedAt.Format(time.RFC3339Nano))
	return err
}
func (s *DestinationProfileStore) GetPattern(ctx context.Context,id string)(*blueprints.Pattern,error){
	return scanPattern(s.db.QueryRowContext(ctx, patternSelect+` WHERE id=?`,id))
}
func (s *DestinationProfileStore) ListPatterns(ctx context.Context)([]*blueprints.Pattern,error){
	rows,err:=s.db.QueryContext(ctx,patternSelect+` ORDER BY name`); if err!=nil{return nil,err}; defer rows.Close()
	var out []*blueprints.Pattern; for rows.Next(){p,e:=scanPattern(rows);if e!=nil{return nil,e};out=append(out,p)};return out,rows.Err()
}
func (s *DestinationProfileStore) DeletePattern(ctx context.Context,id string)error{
	r,e:=s.db.ExecContext(ctx,`DELETE FROM blueprint_patterns WHERE id=?`,id);if e!=nil{return e};n,_:=r.RowsAffected();if n==0{return storage.ErrBlueprintPatternNotFound};return nil
}
const patternSelect=`SELECT id,name,description,platform,receiver_id,receiver_config,signals,enabled,created_at,updated_at FROM blueprint_patterns`
func scanPattern(s scanner)(*blueprints.Pattern,error){
	var p blueprints.Pattern;var signals,created,updated string;var enabled int
	if e:=s.Scan(&p.ID,&p.Name,&p.Description,&p.Platform,&p.ReceiverID,&p.ReceiverConfig,&signals,&enabled,&created,&updated);e!=nil{if errors.Is(e,sql.ErrNoRows){return nil,storage.ErrBlueprintPatternNotFound};return nil,e}
	p.Signals=scanCatalogStrings(signals);p.Enabled=enabled!=0;p.CreatedAt,_=time.Parse(time.RFC3339Nano,created);p.UpdatedAt,_=time.Parse(time.RFC3339Nano,updated);return &p,nil
}

func (s *DestinationProfileStore) CreateBlock(ctx context.Context,b *blueprints.Block)error{
	if b==nil{return fmt.Errorf("blueprint block is required")}
	_,err:=s.db.ExecContext(ctx,`INSERT INTO blueprint_blocks(id,name,description,kind,component_id,config_yaml,signals,platforms,required,locked,enabled,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		b.ID,b.Name,b.Description,b.Kind,b.ComponentID,b.ConfigYAML,catalogStrings(b.Signals),catalogStrings(b.Platforms),boolToInt(b.Required),boolToInt(b.Locked),boolToInt(b.Enabled),b.CreatedAt.Format(time.RFC3339Nano),b.UpdatedAt.Format(time.RFC3339Nano));return err
}
func (s *DestinationProfileStore) GetBlock(ctx context.Context,id string)(*blueprints.Block,error){return scanBlock(s.db.QueryRowContext(ctx,blockSelect+` WHERE id=?`,id))}
func (s *DestinationProfileStore) ListBlocks(ctx context.Context)([]*blueprints.Block,error){
	rows,err:=s.db.QueryContext(ctx,blockSelect+` ORDER BY kind,name`);if err!=nil{return nil,err};defer rows.Close();var out []*blueprints.Block
	for rows.Next(){b,e:=scanBlock(rows);if e!=nil{return nil,e};out=append(out,b)};return out,rows.Err()
}
func (s *DestinationProfileStore) DeleteBlock(ctx context.Context,id string)error{
	r,e:=s.db.ExecContext(ctx,`DELETE FROM blueprint_blocks WHERE id=?`,id);if e!=nil{return e};n,_:=r.RowsAffected();if n==0{return storage.ErrBlueprintBlockNotFound};return nil
}
const blockSelect=`SELECT id,name,description,kind,component_id,config_yaml,signals,platforms,required,locked,enabled,created_at,updated_at FROM blueprint_blocks`
func scanBlock(s scanner)(*blueprints.Block,error){
	var b blueprints.Block;var signals,platforms,created,updated string;var required,locked,enabled int
	if e:=s.Scan(&b.ID,&b.Name,&b.Description,&b.Kind,&b.ComponentID,&b.ConfigYAML,&signals,&platforms,&required,&locked,&enabled,&created,&updated);e!=nil{if errors.Is(e,sql.ErrNoRows){return nil,storage.ErrBlueprintBlockNotFound};return nil,e}
	b.Signals=scanCatalogStrings(signals);b.Platforms=scanCatalogStrings(platforms);b.Required=required!=0;b.Locked=locked!=0;b.Enabled=enabled!=0;b.CreatedAt,_=time.Parse(time.RFC3339Nano,created);b.UpdatedAt,_=time.Parse(time.RFC3339Nano,updated);return &b,nil
}
