// Package secrets models group-scoped secret metadata. Secret values are
// encrypted before they reach the persistence layer and are never returned by
// FleetAMP's user-facing APIs.
package secrets

import "time"

type GroupSecret struct {
	GroupID, Key, Ciphertext, UpdatedBy string
	CreatedAt, UpdatedAt                time.Time
}

