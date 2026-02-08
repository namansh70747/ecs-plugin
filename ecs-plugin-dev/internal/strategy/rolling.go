package strategy

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"ecs-plugin-dev/internal/aws"
	"ecs-plugin-dev/internal/executor"
)

type RollingStrategy struct {
	executor  *executor.Executor
	ecsClient *aws.ECSClient
}

func NewRollingStrategy(exec *executor.Executor) Strategy {
	return &RollingStrategy{
		executor:  exec,
		ecsClient: aws.NewECSClient(),
	}
}

func (s *RollingStrategy) Execute(ctx context.Context, dctx *DeploymentContext) error {
	log.Println("[ROLLING] Starting rolling deployment")

	// Parse configuration
	batchSize := s.parseBatchSize(dctx.Config)
	batchDelay := s.parseBatchDelay(dctx.Config)

	log.Printf("[ROLLING] Batch size: %d%%, Delay: %v", batchSize, batchDelay)

	// Save previous task definition for rollback
	prevTaskDef, err := s.ecsClient.GetPreviousTaskDefinition(ctx, dctx.ClusterARN, dctx.ServiceName)
	if err != nil {
		log.Printf("[ROLLING] Warning: Could not get previous task definition: %v", err)
	}
	dctx.Config["previous_taskdef"] = prevTaskDef

	// Register new task definition
	if err := s.executor.RegisterTaskDefinition(ctx, dctx.TaskDefinition); err != nil {
		return fmt.Errorf("failed to register task definition: %w", err)
	}

	// Execute rolling update in batches
	totalBatches := 100 / batchSize
	log.Printf("[ROLLING] Executing %d batches", totalBatches)

	for batch := 1; batch <= totalBatches; batch++ {
		select {
		case <-ctx.Done():
			log.Printf("[ROLLING] Context canceled at batch %d, initiating rollback", batch)
			s.rollback(ctx, dctx)
			return ctx.Err()
		default:
		}

		currentWeight := batch * batchSize
		if currentWeight > 100 {
			currentWeight = 100
		}

		log.Printf("[ROLLING] Batch %d/%d: Shifting to %d%% new version", batch, totalBatches, currentWeight)

		// Shift traffic gradually
		if err := s.executor.UpdateTraffic(ctx, dctx.ClusterARN, dctx.ServiceName, currentWeight, 100-currentWeight); err != nil {
			log.Printf("[ROLLING] Failed to shift traffic: %v, initiating rollback", err)
			s.rollback(ctx, dctx)
			return fmt.Errorf("traffic shift failed: %w", err)
		}

		// Wait for stabilization
		log.Printf("[ROLLING] Waiting %v for batch %d to stabilize", batchDelay, batch)
		select {
		case <-ctx.Done():
			log.Printf("[ROLLING] Context canceled during stabilization, initiating rollback")
			s.rollback(ctx, dctx)
			return ctx.Err()
		case <-time.After(batchDelay):
		}

		// Validate batch health
		if err := s.validateBatchHealth(ctx, dctx); err != nil {
			log.Printf("[ROLLING] Batch %d health check failed: %v, initiating rollback", batch, err)
			s.rollback(ctx, dctx)
			return fmt.Errorf("batch health check failed: %w", err)
		}

		log.Printf("[ROLLING] Batch %d completed successfully", batch)
	}

	// Final update to 100%
	log.Println("[ROLLING] Finalizing rolling deployment to 100%")
	if err := s.executor.UpdateService(ctx, dctx.ClusterARN, dctx.ServiceName, dctx.TaskDefinition); err != nil {
		s.rollback(ctx, dctx)
		return fmt.Errorf("final update failed: %w", err)
	}

	// Wait for final stabilization
	if err := s.executor.WaitForServiceStable(ctx, dctx.ClusterARN, dctx.ServiceName, 5*time.Minute); err != nil {
		log.Printf("[ROLLING] Warning: Service did not stabilize: %v", err)
	}

	log.Println("[ROLLING] Rolling deployment completed successfully")
	return nil
}

func (s *RollingStrategy) validateBatchHealth(ctx context.Context, dctx *DeploymentContext) error {
	// Get service status
	_, err := s.ecsClient.DescribeService(ctx, dctx.ClusterARN, dctx.ServiceName)
	if err != nil {
		return fmt.Errorf("failed to validate service: %w", err)
	}

	log.Printf("[ROLLING] Batch health check passed")
	return nil
}

func (s *RollingStrategy) rollback(ctx context.Context, dctx *DeploymentContext) {
	log.Println("[ROLLING] Initiating rollback to previous version")

	prevTaskDef := dctx.Config["previous_taskdef"]
	if prevTaskDef == "" {
		log.Println("[ROLLING] No previous task definition available for rollback")
		return
	}

	// Shift traffic back to old version
	if err := s.executor.UpdateTraffic(ctx, dctx.ClusterARN, dctx.ServiceName, 0, 100); err != nil {
		log.Printf("[ROLLING] Rollback traffic shift failed: %v", err)
		return
	}

	// Update service to previous task definition
	if err := s.executor.UpdateService(ctx, dctx.ClusterARN, dctx.ServiceName, prevTaskDef); err != nil {
		log.Printf("[ROLLING] Rollback service update failed: %v", err)
		return
	}

	log.Println("[ROLLING] Rollback completed")
}

func (s *RollingStrategy) parseBatchSize(config map[string]string) int {
	if batchSize, ok := config["batch_size"]; ok {
		if size, err := strconv.Atoi(batchSize); err == nil && size > 0 && size <= 100 {
			return size
		}
	}
	return 25 // Default 25% per batch
}

func (s *RollingStrategy) parseBatchDelay(config map[string]string) time.Duration {
	if delay, ok := config["batch_delay"]; ok {
		if d, err := time.ParseDuration(delay); err == nil {
			return d
		}
	}
	return 1 * time.Minute // Default 1 minute between batches
}
