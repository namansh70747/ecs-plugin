// internal/strategy/quicksync.go
package strategy

import (
    "context"
    "ecs-plugin-dev/internal/executor"
)

type QuickSyncStrategy struct {
    executor *executor.Executor
}

func NewQuickSyncStrategy(exec *executor.Executor) Strategy {
    return &QuickSyncStrategy{executor: exec}
}

func (s *QuickSyncStrategy) Execute(ctx context.Context, dctx *DeploymentContext) error {
    if err := s.executor.RegisterTaskDefinition(ctx, dctx.TaskDefinition); err != nil {
        return err
    }
    return s.executor.UpdateService(ctx, dctx.ClusterARN, dctx.ServiceName, dctx.TaskDefinition)
}