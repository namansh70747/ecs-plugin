// internal/strategy/bluegreen.go
package strategy

import (
	"context"
	"fmt"
	"log"
	"time"

	"ecs-plugin-dev/internal/executor"
)

type BlueGreenStrategy struct {
	executor *executor.Executor
}

func NewBlueGreenStrategy(exec *executor.Executor) Strategy {
	return &BlueGreenStrategy{executor: exec}
}

func (s *BlueGreenStrategy) Execute(ctx context.Context, dctx *DeploymentContext) error {
	log.Println("[BLUEGREEN] Starting blue-green deployment")

	// Save previous task definition for rollback
	if err := s.executor.RollbackService(ctx, dctx.ClusterARN, dctx.ServiceName); err != nil {
		log.Printf("[BLUEGREEN] Warning: Could not fetch previous task definition: %v", err)
	}

	// Register new task definition (green)
	if err := s.executor.RegisterTaskDefinition(ctx, dctx.TaskDefinition); err != nil {
		return fmt.Errorf("failed to register green task definition: %w", err)
	}

	// Create green task set at 100% weight
	log.Println("[BLUEGREEN] Creating green environment")
	if err := s.executor.CreateTaskSet(ctx, dctx.ClusterARN, dctx.ServiceName, dctx.TaskDefinition, 100); err != nil {
		return fmt.Errorf("failed to create green task set: %w", err)
	}

	// Wait for green environment to stabilize
	stabilizationTime := 30 * time.Second
	if timeoutStr, ok := dctx.Config["stabilization_time"]; ok {
		if duration, err := time.ParseDuration(timeoutStr); err == nil {
			stabilizationTime = duration
		}
	}

	log.Printf("[BLUEGREEN] Waiting %v for green environment to stabilize", stabilizationTime)
	stabilizeCtx, cancel := context.WithTimeout(ctx, stabilizationTime+time.Minute)
	defer cancel()

	if err := s.executor.WaitForServiceStable(stabilizeCtx, dctx.ClusterARN, dctx.ServiceName, stabilizationTime+time.Minute); err != nil {
		log.Printf("[BLUEGREEN] Green environment failed to stabilize: %v, initiating rollback", err)
		s.rollback(ctx, dctx)
		return fmt.Errorf("green environment stabilization failed: %w", err)
	}

	log.Println("[BLUEGREEN] Green environment is stable")

	// Shift traffic to green (100% to new, 0% to old)
	log.Println("[BLUEGREEN] Shifting traffic to green environment")
	if err := s.executor.UpdateTraffic(ctx, dctx.ClusterARN, dctx.ServiceName, 100, 0); err != nil {
		log.Printf("[BLUEGREEN] Traffic shift failed: %v, initiating rollback", err)
		s.rollback(ctx, dctx)
		return fmt.Errorf("traffic shift failed: %w", err)
	}

	// Wait before cleanup
	cleanupDelay := 1 * time.Minute
	if delayStr, ok := dctx.Config["cleanup_delay"]; ok {
		if duration, err := time.ParseDuration(delayStr); err == nil {
			cleanupDelay = duration
		}
	}

	log.Printf("[BLUEGREEN] Waiting %v before cleanup", cleanupDelay)
	time.Sleep(cleanupDelay)

	// Cleanup blue environment
	log.Println("[BLUEGREEN] Cleaning up blue environment")
	if err := s.executor.DeleteTaskSet(ctx, dctx.ClusterARN, dctx.ServiceName, "PRIMARY"); err != nil {
		log.Printf("[BLUEGREEN] Warning: cleanup failed: %v", err)
		// Don't fail deployment on cleanup error
	}

	log.Println("[BLUEGREEN] Deployment completed successfully")
	return nil
}

// rollback reverts to blue environment
func (s *BlueGreenStrategy) rollback(ctx context.Context, dctx *DeploymentContext) {
	log.Println("[BLUEGREEN ROLLBACK] Starting automatic rollback to blue environment")

	// Shift traffic back to blue (0% to new, 100% to old)
	if err := s.executor.UpdateTraffic(ctx, dctx.ClusterARN, dctx.ServiceName, 0, 100); err != nil {
		log.Printf("[BLUEGREEN ROLLBACK] Failed to shift traffic back: %v", err)
	}

	// Delete green task set
	if err := s.executor.DeleteTaskSet(ctx, dctx.ClusterARN, dctx.ServiceName, "GREEN"); err != nil {
		log.Printf("[BLUEGREEN ROLLBACK] Failed to delete green task set: %v", err)
	}

	log.Println("[BLUEGREEN ROLLBACK] Rollback completed")
}
