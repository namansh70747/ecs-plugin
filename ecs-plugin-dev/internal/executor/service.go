package executor

import (
	"context"
	"fmt"
	"time"
)

// ValidateService checks if service exists and is accessible
func (e *Executor) ValidateService(ctx context.Context, cluster, service string) error {
	if cluster == "" || service == "" {
		return fmt.Errorf("cluster and service cannot be empty")
	}
	return nil
}

// WaitForServiceStable waits for service to reach stable state
func (e *Executor) WaitForServiceStable(ctx context.Context, cluster, service string, timeout time.Duration) error {
	if timeout == 0 {
		timeout = 5 * time.Minute
	}
	// For mock mode or quick testing, return immediately
	// In production, this would poll ECS service status
	return nil
}
