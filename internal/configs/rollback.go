// Rollback target validation for immutable FleetAMP configuration history.
//
// Purpose:
//
//	Ensures rollback selects an older artifact from the same configuration
//	lineage before FleetAMP attempts validation or OpAMP delivery.
package configs

import (
	"errors"
	"fmt"
)

var (
	ErrRollbackSameConfiguration = errors.New("rollback target is already the desired configuration")
	ErrRollbackDifferentLineage  = errors.New("rollback target belongs to a different configuration name")
	ErrRollbackNotOlder          = errors.New("rollback target is not older than the current desired configuration")
)

// ValidateRollbackTarget enforces same-lineage, older-artifact rollback semantics.
func ValidateRollbackTarget(current, target *Configuration) error {
	if current == nil || target == nil {
		return fmt.Errorf("current and target configurations are required")
	}
	if current.ID == target.ID {
		return ErrRollbackSameConfiguration
	}
	if current.Name != target.Name {
		return ErrRollbackDifferentLineage
	}
	if !target.CreatedAt.Before(current.CreatedAt) {
		return ErrRollbackNotOlder
	}
	return nil
}
