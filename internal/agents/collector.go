package agents

import "time"

type Collector struct {
	InstanceUID string    `json:"instance_uid"`
	Hostname    string    `json:"hostname,omitempty"`
	Version     string    `json:"version,omitempty"`
	Healthy     bool      `json:"healthy"`
	Connected   bool      `json:"connected"`
	LastSeen    time.Time `json:"last_seen"`
}
