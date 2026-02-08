// internal/strategy/canary.go
package strategy

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"ecs-plugin-dev/internal/executor"
	"ecs-plugin-dev/internal/metrics"
)

type CanaryStrategy struct {
	executor *executor.Executor
}

func NewCanaryStrategy(exec *executor.Executor) Strategy {
	return &CanaryStrategy{executor: exec}
}

func (s *CanaryStrategy) Execute(ctx context.Context, dctx *DeploymentContext) error {
	// Parse canary configuration
	stages := parseCanaryStages(dctx.Config)
	stageTimeout := parseStageTimeout(dctx.Config)
	enableRollback := parseRollbackEnabled(dctx.Config)

	log.Printf("[CANARY] Starting multi-stage deployment with stages: %v (rollback: %v)", stages, enableRollback)

	// Save previous task definition for rollback
	if err := s.executor.RollbackService(ctx, dctx.ClusterARN, dctx.ServiceName); err != nil {
		log.Printf("[CANARY] Warning: Could not fetch previous task definition: %v", err)
	}

	if err := s.executor.RegisterTaskDefinition(ctx, dctx.TaskDefinition); err != nil {
		return err
	}

	// Execute each canary stage
	for i, percent := range stages {
		stage := fmt.Sprintf("%d%%", percent)
		log.Printf("[CANARY] Stage %d/%d: %s", i+1, len(stages), stage)

		if err := s.executor.CreateTaskSet(ctx, dctx.ClusterARN, dctx.ServiceName, dctx.TaskDefinition, percent); err != nil {
			metrics.CanaryStagesTotal.WithLabelValues(stage, "failed").Inc()
			if enableRollback {
				log.Printf("[CANARY] Stage %s failed, initiating rollback", stage)
				s.rollback(ctx, dctx)
			}
			return fmt.Errorf("stage %s failed: %w", stage, err)
		}

		// Wait for stage stabilization
		log.Printf("[CANARY] Waiting %v for stage %s to stabilize", stageTimeout, stage)
		select {
		case <-time.After(stageTimeout):
			// Validate stage health
			if err := s.validateStageHealth(ctx, dctx, percent); err != nil {
				metrics.CanaryStagesTotal.WithLabelValues(stage, "failed").Inc()
				if enableRollback {
					log.Printf("[CANARY] Stage %s health check failed: %v, initiating rollback", stage, err)
					s.rollback(ctx, dctx)
				}
				return fmt.Errorf("stage %s health check failed: %w", stage, err)
			}
			metrics.CanaryStagesTotal.WithLabelValues(stage, "success").Inc()
			log.Printf("[CANARY] Stage %s completed successfully", stage)
		case <-ctx.Done():
			if enableRollback {
				log.Println("[CANARY] Context canceled, initiating rollback")
				s.rollback(ctx, dctx)
			}
			return ctx.Err()
		}
	}

	// Final traffic shift to 100%
	log.Println("[CANARY] Shifting all traffic to new version")
	if err := s.executor.UpdateTraffic(ctx, dctx.ClusterARN, dctx.ServiceName, 0, 100); err != nil {
		metrics.TrafficShiftsTotal.WithLabelValues("canary", "failed").Inc()
		if enableRollback {
			log.Println("[CANARY] Traffic shift failed, initiating rollback")
			s.rollback(ctx, dctx)
		}
		return err
	}
	metrics.TrafficShiftsTotal.WithLabelValues("canary", "success").Inc()

	// Cleanup old task set
	if err := s.executor.DeleteTaskSet(ctx, dctx.ClusterARN, dctx.ServiceName, "PRIMARY"); err != nil {
		return fmt.Errorf("cleanup failed: %w", err)
	}

	log.Println("[CANARY] Deployment completed successfully")
	return nil
}

// validateStageHealth checks service health at current canary stage
func (s *CanaryStrategy) validateStageHealth(ctx context.Context, dctx *DeploymentContext, percent int) error {
	log.Printf("[CANARY] Validating health for stage %d%%", percent)

	// Wait for service to stabilize at this stage
	stabilizeCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	if err := s.executor.WaitForServiceStable(stabilizeCtx, dctx.ClusterARN, dctx.ServiceName, 2*time.Minute); err != nil {
		return fmt.Errorf("service did not stabilize: %w", err)
	}

	log.Printf("[CANARY] Health check passed for stage %d%%", percent)
	return nil
}

// rollback reverts to previous task definition
func (s *CanaryStrategy) rollback(ctx context.Context, dctx *DeploymentContext) {
	log.Println("[CANARY ROLLBACK] Starting automatic rollback")

	// Shift traffic back to 100% primary
	if err := s.executor.UpdateTraffic(ctx, dctx.ClusterARN, dctx.ServiceName, 0, 100); err != nil {
		log.Printf("[CANARY ROLLBACK] Failed to shift traffic back: %v", err)
	}

	// Delete canary task set
	if err := s.executor.DeleteTaskSet(ctx, dctx.ClusterARN, dctx.ServiceName, "CANARY"); err != nil {
		log.Printf("[CANARY ROLLBACK] Failed to delete canary task set: %v", err)
	}

	log.Println("[CANARY ROLLBACK] Rollback completed")
	metrics.RecordError("strategy", "canary_rollback")
}

// parseCanaryStages extracts canary stages from config
func parseCanaryStages(config map[string]string) []int {
	if stagesStr, ok := config["canary_stages"]; ok {
		parts := strings.Split(stagesStr, ",")
		stages := make([]int, 0, len(parts))
		for _, part := range parts {
			if percent, err := strconv.Atoi(strings.TrimSpace(part)); err == nil {
				stages = append(stages, percent)
			}
		}
		if len(stages) > 0 {
			return stages
		}
	}

	// Fallback to single stage
	if percentStr, ok := config["canary_percent"]; ok {
		if percent, err := strconv.Atoi(percentStr); err == nil {
			return []int{percent, 100}
		}
	}

	// Default multi-stage canary
	return []int{20, 50, 100}
}

// parseStageTimeout extracts stage timeout from config
func parseStageTimeout(config map[string]string) time.Duration {
	if timeoutStr, ok := config["stage_timeout"]; ok {
		if duration, err := time.ParseDuration(timeoutStr); err == nil {
			return duration
		}
	}
	return 2 * time.Minute
}

// parseRollbackEnabled checks if automatic rollback is enabled
func parseRollbackEnabled(config map[string]string) bool {
	if rollbackStr, ok := config["enable_rollback"]; ok {
		return rollbackStr == "true" || rollbackStr == "1"
	}
	return true // Default: rollback enabled
}
