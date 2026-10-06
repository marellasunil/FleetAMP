package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/marellasunil/FleetAMP/internal/lifecycle"
	"github.com/marellasunil/FleetAMP/internal/storage"
)

type ComponentGitOpsPreviewStore struct{ db *sql.DB }

func (s *ComponentGitOpsPreviewStore) Create(ctx context.Context, v *lifecycle.GitOpsPreview) error {
	files, e := json.Marshal(v.Files)
	if e != nil {
		return e
	}
	_, e = s.db.ExecContext(ctx, `INSERT INTO component_gitops_previews(id,plan_id,plan_hash,repository_path,files,diff,preview_hash,prepared_by,created_at) VALUES(?,?,?,?,?,?,?,?,?)`, v.ID, v.PlanID, v.PlanHash, v.RepositoryPath, string(files), v.Diff, v.PreviewHash, v.PreparedBy, formatTime(v.CreatedAt))
	if e != nil {
		return fmt.Errorf("create component GitOps preview: %w", e)
	}
	return nil
}
func (s *ComponentGitOpsPreviewStore) GetByPlan(ctx context.Context, id string) (*lifecycle.GitOpsPreview, error) {
	v, e := scanGitOpsPreview(s.db.QueryRowContext(ctx, gitOpsPreviewSelect+` WHERE plan_id=?`, id))
	if errors.Is(e, sql.ErrNoRows) {
		return nil, storage.ErrComponentGitOpsPreviewNotFound
	}
	return v, e
}
func (s *ComponentGitOpsPreviewStore) List(ctx context.Context, n int) ([]*lifecycle.GitOpsPreview, error) {
	if n <= 0 || n > 500 {
		n = 100
	}
	rows, e := s.db.QueryContext(ctx, gitOpsPreviewSelect+` ORDER BY created_at DESC LIMIT ?`, n)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	r := []*lifecycle.GitOpsPreview{}
	for rows.Next() {
		v, e := scanGitOpsPreview(rows)
		if e != nil {
			return nil, e
		}
		r = append(r, v)
	}
	return r, rows.Err()
}

const gitOpsPreviewSelect = `SELECT id,plan_id,plan_hash,repository_path,files,diff,preview_hash,prepared_by,created_at FROM component_gitops_previews`

type gitOpsPreviewScanner interface{ Scan(...any) error }

func scanGitOpsPreview(s gitOpsPreviewScanner) (*lifecycle.GitOpsPreview, error) {
	v := &lifecycle.GitOpsPreview{}
	var files, created string
	if e := s.Scan(&v.ID, &v.PlanID, &v.PlanHash, &v.RepositoryPath, &files, &v.Diff, &v.PreviewHash, &v.PreparedBy, &created); e != nil {
		return nil, e
	}
	if e := json.Unmarshal([]byte(files), &v.Files); e != nil {
		return nil, e
	}
	v.CreatedAt, _ = parseTime(created)
	return v, nil
}

var _ storage.ComponentGitOpsPreviewStore = (*ComponentGitOpsPreviewStore)(nil)
