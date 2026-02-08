// internal/strategy/bluegreen.go
package strategy

import (
    "context"
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
    if err := s.executor.RegisterTaskDefinition(ctx, dctx.TaskDefinition); err != nil {
        return err
    }

    if err := s.executor.CreateTaskSet(ctx, dctx.ClusterARN, dctx.ServiceName, dctx.TaskDefinition, 100); err != nil {
        return err
    }

    time.Sleep(30 * time.Second)

    if err := s.executor.UpdateTraffic(ctx, dctx.ClusterARN, dctx.ServiceName, 100, 0); err != nil {
        return err
    }

    time.Sleep(1 * time.Minute)

    return s.executor.DeleteTaskSet(ctx, dctx.ClusterARN, dctx.ServiceName, "PRIMARY")
}