// internal/strategy/canary.go
package strategy

import (
    "context"
    "fmt"
    "strconv"
    "time"

    "ecs-plugin-dev/internal/executor"
)

type CanaryStrategy struct {
    executor *executor.Executor
}

func NewCanaryStrategy(exec *executor.Executor) Strategy {
    return &CanaryStrategy{executor: exec}
}

func (s *CanaryStrategy) Execute(ctx context.Context, dctx *DeploymentContext) error {
    canaryPercent, _ := strconv.Atoi(dctx.Config["canary_percent"])
    if canaryPercent == 0 {
        canaryPercent = 20
    }

    if err := s.executor.RegisterTaskDefinition(ctx, dctx.TaskDefinition); err != nil {
        return err
    }

    if err := s.executor.CreateTaskSet(ctx, dctx.ClusterARN, dctx.ServiceName, dctx.TaskDefinition, canaryPercent); err != nil {
        return err
    }

    time.Sleep(2 * time.Minute)

    if err := s.executor.UpdateTraffic(ctx, dctx.ClusterARN, dctx.ServiceName, 0, 100); err != nil {
        return err
    }

    if err := s.executor.DeleteTaskSet(ctx, dctx.ClusterARN, dctx.ServiceName, "PRIMARY"); err != nil {
        return fmt.Errorf("cleanup failed: %w", err)
    }

    return nil
}