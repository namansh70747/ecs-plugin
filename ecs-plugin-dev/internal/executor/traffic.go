package executor

import (
	"context"
	"fmt"
)

// ShiftTraffic gradually shifts traffic between task sets
func (e *Executor) ShiftTraffic(ctx context.Context, cluster, service string, canaryPercent int) error {
	if canaryPercent < 0 || canaryPercent > 100 {
		return fmt.Errorf("canary percent must be between 0 and 100")
	}

	primaryPercent := 100 - canaryPercent
	return e.UpdateTraffic(ctx, cluster, service, canaryPercent, primaryPercent)
}

// ValidateTrafficConfig validates traffic configuration
func (e *Executor) ValidateTrafficConfig(canaryWeight, primaryWeight int) error {
	if canaryWeight < 0 || primaryWeight < 0 {
		return fmt.Errorf("weights cannot be negative")
	}
	if canaryWeight+primaryWeight != 100 {
		return fmt.Errorf("weights must sum to 100, got %d", canaryWeight+primaryWeight)
	}
	return nil
}
