// Immutable FleetAMP configuration artifact model.
//
// Purpose:
//   Represents a versioned configuration payload that can be assigned to a
//   managed agent and correlated with OpAMP remote-config status/effective state.
//
// Identity:
//   Content receives a SHA-256 hash; artifact ID is derived from name, version,
//   and content so configuration objects are deterministic and immutable.
//
// Dependencies:
//   Go crypto/sha256, encoding/hex, strings, and time only.

package configs

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"
)

// Configuration is an immutable configuration artifact managed by FleetAMP.
type Configuration struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Version     string    `json:"version"`
	Content     string    `json:"content"`
	ContentType string    `json:"content_type"`
	Hash        string    `json:"hash"`
	CreatedAt   time.Time `json:"created_at"`
}

// NewConfiguration builds an immutable artifact with deterministic identity and
// content hashes used for assignment and OpAMP status correlation.
func NewConfiguration(name, version, content, contentType string) *Configuration {
	if contentType == "" {
		contentType = "text/yaml"
	}
	contentHash := sha256.Sum256([]byte(content))
	identityHash := sha256.Sum256([]byte(strings.Join([]string{name, version, content}, "\x00")))
	return &Configuration{
		ID: hex.EncodeToString(identityHash[:]), Name: name, Version: version,
		Content: content, ContentType: contentType,
		Hash: hex.EncodeToString(contentHash[:]), CreatedAt: time.Now().UTC(),
	}
}
