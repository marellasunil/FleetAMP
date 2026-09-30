package blueprints

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"
)

// Pattern is an administrator-approved starting point for a guided Blueprint.
type Pattern struct {
	ID, Name, Description, Platform, ReceiverID, ReceiverConfig string
	Signals []string
	Enabled bool
	CreatedAt, UpdatedAt time.Time
}

// Block is a reusable Collector component offered by the Blueprint builder.
type Block struct {
	ID, Name, Description, Kind, ComponentID, ConfigYAML string
	Signals, Platforms []string
	Required, Locked, Enabled bool
	CreatedAt, UpdatedAt time.Time
}

func catalogID(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(strings.Join(parts, "\x00"))))
	return hex.EncodeToString(sum[:12])
}

func NewPattern(name, description, platform, receiverID, receiverConfig string, signals []string) *Pattern {
	now := time.Now().UTC()
	name = strings.TrimSpace(name)
	return &Pattern{ID: catalogID("pattern", name, platform), Name: name, Description: strings.TrimSpace(description),
		Platform: strings.TrimSpace(platform), ReceiverID: strings.TrimSpace(receiverID),
		ReceiverConfig: strings.TrimSpace(receiverConfig), Signals: signals, Enabled: true, CreatedAt: now, UpdatedAt: now}
}

func NewBlock(name, description, kind, componentID, configYAML string, signals, platforms []string, required, locked bool) *Block {
	now := time.Now().UTC()
	name = strings.TrimSpace(name)
	return &Block{ID: catalogID("block", kind, componentID), Name: name, Description: strings.TrimSpace(description),
		Kind: strings.TrimSpace(kind), ComponentID: strings.TrimSpace(componentID), ConfigYAML: strings.TrimSpace(configYAML),
		Signals: signals, Platforms: platforms, Required: required, Locked: locked, Enabled: true, CreatedAt: now, UpdatedAt: now}
}
