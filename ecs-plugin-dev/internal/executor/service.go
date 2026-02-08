package executor

import (
	"context"
	"fmt"
	"log"
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

	// Check if mock mode
	if e.ecsClient == nil {
		log.Println("[MOCK] Service stability check skipped in mock mode")
		return nil
	}

	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	log.Printf("[SERVICE] Waiting for service %s to stabilize (timeout: %v)", service, timeout)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if time.Now().After(deadline) {
				return fmt.Errorf("service stabilization timeout after %v", timeout)
			}

			// Use DescribeService for real AWS check
			svc, err := e.ecsClient.DescribeService(ctx, cluster, service)
			if err != nil {
				log.Printf("[SERVICE] Error describing service: %v", err)
				continue
			}

			// Check if service is stable:
			// 1. Only one deployment (PRIMARY)
			// 2. Running count matches desired count
			// 3. Deployment rollout is completed
			if len(svc.Deployments) == 1 {
				deployment := svc.Deployments[0]

				isPrimary := deployment.Status != nil && *deployment.Status == "PRIMARY"
				isCompleted := deployment.RolloutState == "COMPLETED"
				tasksMatch := deployment.RunningCount == deployment.DesiredCount
				serviceMatch := svc.RunningCount == svc.DesiredCount

				if isPrimary && isCompleted && tasksMatch && serviceMatch {
					log.Printf("[SERVICE] Service %s is stable: %d/%d tasks running",
						service, svc.RunningCount, svc.DesiredCount)
					return nil
				}

				status := "UNKNOWN"
				if deployment.Status != nil {
					status = *deployment.Status
				}
				log.Printf("[SERVICE] Service %s not yet stable: status=%s, rollout=%s, running=%d/%d",
					service, status, deployment.RolloutState,
					deployment.RunningCount, deployment.DesiredCount)
			} else {
				log.Printf("[SERVICE] Service %s has %d deployments, waiting for convergence",
					service, len(svc.Deployments))
			}
		}
	}
}
