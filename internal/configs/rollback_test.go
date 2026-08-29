// Tests rollback safety rules for immutable configuration history.
package configs

import (
	"errors"
	"testing"
	"time"
)

func TestValidateRollbackTarget(t *testing.T) {
	now := time.Now().UTC()
	current := &Configuration{ID: "v3", Name: "agent.yaml", CreatedAt: now}

	tests := []struct {
		name   string
		target *Configuration
		want   error
	}{
		{"older same lineage", &Configuration{ID: "v2", Name: "agent.yaml", CreatedAt: now.Add(-time.Hour)}, nil},
		{"same artifact", &Configuration{ID: "v3", Name: "agent.yaml", CreatedAt: now}, ErrRollbackSameConfiguration},
		{"different lineage", &Configuration{ID: "v2", Name: "other.yaml", CreatedAt: now.Add(-time.Hour)}, ErrRollbackDifferentLineage},
		{"newer artifact", &Configuration{ID: "v4", Name: "agent.yaml", CreatedAt: now.Add(time.Hour)}, ErrRollbackNotOlder},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRollbackTarget(current, tt.target)
			if tt.want == nil && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.want != nil && !errors.Is(err, tt.want) {
				t.Fatalf("got %v, want %v", err, tt.want)
			}
		})
	}
}
