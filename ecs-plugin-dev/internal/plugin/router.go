// internal/plugin/router.go
package plugin

import (
	"context"
	"fmt"
	"sync"

	"ecs-plugin-dev/internal/executor"
	"ecs-plugin-dev/internal/strategy"
)

type DeploymentRequest struct {
	DeploymentID   string
	ClusterARN     string
	ServiceName    string
	TaskDefinition string
	Strategy       string
	Config         map[string]string
}

type DeploymentResult struct {
	Success      bool
	Message      string
	DeploymentID string
}

type DeploymentStatus struct {
	Status   string
	Message  string
	Progress int32
}

type Router struct {
	strategies map[string]strategy.Strategy
	executor   *executor.Executor
	statuses   sync.Map
}

func NewRouter() *Router {
	exec := executor.NewExecutor()
	return &Router{
		strategies: map[string]strategy.Strategy{
			"quicksync": strategy.NewQuickSyncStrategy(exec),
			"canary":    strategy.NewCanaryStrategy(exec),
			"bluegreen": strategy.NewBlueGreenStrategy(exec),
		},
		executor: exec,
	}
}

func (r *Router) RouteDeployment(ctx context.Context, req *DeploymentRequest) (*DeploymentResult, error) {
	// Validate request first
	if err := r.ValidateRequest(req); err != nil {
		return &DeploymentResult{
			Success: false,
			Message: fmt.Sprintf("validation failed: %v", err),
		}, err
	}

	strat, ok := r.strategies[req.Strategy]
	if !ok {
		return nil, fmt.Errorf("unknown strategy: %s", req.Strategy)
	}

	r.statuses.Store(req.DeploymentID, &DeploymentStatus{
		Status:   "RUNNING",
		Message:  "deployment started",
		Progress: 0,
	})

	go func() {
		err := strat.Execute(ctx, &strategy.DeploymentContext{
			DeploymentID:   req.DeploymentID,
			ClusterARN:     req.ClusterARN,
			ServiceName:    req.ServiceName,
			TaskDefinition: req.TaskDefinition,
			Config:         req.Config,
		})

		if err != nil {
			r.statuses.Store(req.DeploymentID, &DeploymentStatus{
				Status:   "FAILED",
				Message:  err.Error(),
				Progress: 100,
			})
		} else {
			r.statuses.Store(req.DeploymentID, &DeploymentStatus{
				Status:   "SUCCESS",
				Message:  "deployment completed",
				Progress: 100,
			})
		}
	}()

	return &DeploymentResult{
		Success:      true,
		Message:      "deployment initiated",
		DeploymentID: req.DeploymentID,
	}, nil
}

func (r *Router) GetDeploymentStatus(ctx context.Context, deploymentID string) (*DeploymentStatus, error) {
	val, ok := r.statuses.Load(deploymentID)
	if !ok {
		return nil, fmt.Errorf("deployment not found: %s", deploymentID)
	}
	return val.(*DeploymentStatus), nil
}

func (r *Router) Rollback(ctx context.Context, deploymentID, clusterARN, serviceName string) error {
	return r.executor.RollbackService(ctx, clusterARN, serviceName)
}

// ValidateRequest validates deployment request
func (r *Router) ValidateRequest(req *DeploymentRequest) error {
	if req.DeploymentID == "" {
		return fmt.Errorf("deployment ID is required")
	}
	if req.ClusterARN == "" {
		return fmt.Errorf("cluster ARN is required")
	}
	if req.ServiceName == "" {
		return fmt.Errorf("service name is required")
	}
	if req.TaskDefinition == "" {
		return fmt.Errorf("task definition is required")
	}
	if req.Strategy == "" {
		return fmt.Errorf("strategy is required")
	}

	// Validate strategy exists
	if _, ok := r.strategies[req.Strategy]; !ok {
		return fmt.Errorf("unknown strategy: %s", req.Strategy)
	}

	return nil
}

// ListStrategies returns available deployment strategies
func (r *Router) ListStrategies() []string {
	strategies := make([]string, 0, len(r.strategies))
	for name := range r.strategies {
		strategies = append(strategies, name)
	}
	return strategies
}
