// Package audit defines append-only FleetAMP control-plane audit events.
package audit

import (
	"strings"
	"time"
)

type Event struct {
	ID           int64     `json:"id"`
	Timestamp    time.Time `json:"timestamp"`
	Actor        string    `json:"actor"`
	Action       string    `json:"action"`
	ResourceType string    `json:"resource_type"`
	ResourceID   string    `json:"resource_id,omitempty"`
	Outcome      string    `json:"outcome"`
	HTTPMethod   string    `json:"http_method"`
	Path         string    `json:"path"`
	StatusCode   int       `json:"status_code"`
}

type Filter struct {
	Actor   string
	Action  string
	Outcome string
	Since   time.Time
	Until   time.Time
	Limit   int
}

func (f Filter) Normalized() Filter {
	f.Actor = strings.TrimSpace(f.Actor)
	f.Action = strings.TrimSpace(f.Action)
	f.Outcome = strings.TrimSpace(f.Outcome)
	if f.Limit <= 0 || f.Limit > 500 {
		f.Limit = 200
	}
	return f
}
