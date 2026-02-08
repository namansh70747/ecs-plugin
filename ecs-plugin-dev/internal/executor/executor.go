// internal/executor/executor.go
package executor

import (
    "context"
    "fmt"

    "ecs-plugin-dev/internal/aws"
)

type Executor struct {
    ecsClient *aws.ECSClient
    elbClient *aws.ELBClient
}

func NewExecutor() *Executor {
    return &Executor{
        ecsClient: aws.NewECSClient(),
        elbClient: aws.NewELBClient(),
    }
}

func (e *Executor) RegisterTaskDefinition(ctx context.Context, taskDefJSON string) error {
    return e.ecsClient.RegisterTaskDefinition(ctx, taskDefJSON)
}

func (e *Executor) UpdateService(ctx context.Context, cluster, service, taskDef string) error {
    return e.ecsClient.UpdateService(ctx, cluster, service, taskDef)
}

func (e *Executor) CreateTaskSet(ctx context.Context, cluster, service, taskDef string, weight int) error {
    return e.ecsClient.CreateTaskSet(ctx, cluster, service, taskDef, weight)
}

func (e *Executor) UpdateTraffic(ctx context.Context, cluster, service string, canaryWeight, primaryWeight int) error {
    return e.elbClient.UpdateTargetGroupWeights(ctx, cluster, service, canaryWeight, primaryWeight)
}

func (e *Executor) DeleteTaskSet(ctx context.Context, cluster, service, taskSetID string) error {
    return e.ecsClient.DeleteTaskSet(ctx, cluster, service, taskSetID)
}

func (e *Executor) RollbackService(ctx context.Context, cluster, service string) error {
    taskDef, err := e.ecsClient.GetPreviousTaskDefinition(ctx, cluster, service)
    if err != nil {
        return fmt.Errorf("rollback failed: %w", err)
    }
    return e.UpdateService(ctx, cluster, service, taskDef)
}