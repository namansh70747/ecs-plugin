// internal/plugin/router.go
package plugin

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"ecs-plugin-dev/internal/executor"
	"ecs-plugin-dev/internal/metrics"
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
	Status    string
	Message   string
	Progress  int32
	StartTime time.Time
	EndTime   time.Time
}

type Router struct {
	strategies      map[string]strategy.Strategy
	executor        *executor.Executor
	statuses        sync.Map
	serviceQueue    sync.Map // Tracks active deployments per service
	hooks           *executor.HookRegistry
	cancelFuncs     sync.Map // Tracks cancel functions for active deployments
	approvalManager *executor.ApprovalManager
}

func NewRouter() *Router {
	exec := executor.NewExecutor()
	hooks := executor.NewHookRegistry()

	// Register default hooks
	hooks.RegisterHook(executor.PreDeployHook, executor.Hook{
		Name: "validation",
		Fn:   executor.ValidationHook,
	})
	hooks.RegisterHook(executor.PostDeployHook, executor.Hook{
		Name: "health-check",
		Fn:   executor.HealthCheckHook,
	})
	hooks.RegisterHook(executor.PostDeployHook, executor.Hook{
		Name: "notification",
		Fn:   executor.NotificationHook,
	})

	return &Router{
		strategies: map[string]strategy.Strategy{
			"quicksync": strategy.NewQuickSyncStrategy(exec),
			"canary":    strategy.NewCanaryStrategy(exec),
			"bluegreen": strategy.NewBlueGreenStrategy(exec),
			"rolling":   strategy.NewRollingStrategy(exec),
		},
		executor:        exec,
		hooks:           hooks,
		approvalManager: executor.NewApprovalManager(),
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

	// Check for concurrent deployments to same service
	serviceKey := fmt.Sprintf("%s/%s", req.ClusterARN, req.ServiceName)
	if _, loaded := r.serviceQueue.LoadOrStore(serviceKey, req.DeploymentID); loaded {
		return &DeploymentResult{
			Success: false,
			Message: "deployment already in progress for this service",
		}, fmt.Errorf("concurrent deployment detected")
	}

	strat, ok := r.strategies[req.Strategy]
	if !ok {
		r.serviceQueue.Delete(serviceKey)
		return nil, fmt.Errorf("unknown strategy: %s", req.Strategy)
	}

	startTime := time.Now()
	r.statuses.Store(req.DeploymentID, &DeploymentStatus{
		Status:    "RUNNING",
		Message:   "deployment started",
		Progress:  0,
		StartTime: startTime,
	})

	metrics.IncrementInProgress()

	// Create cancellable context for this deployment
	deployCtx, cancel := context.WithCancel(ctx)
	r.cancelFuncs.Store(req.DeploymentID, cancel)

	go func() {
		defer func() {
			r.serviceQueue.Delete(serviceKey)
			r.cancelFuncs.Delete(req.DeploymentID)
			metrics.DecrementInProgress()
			cancel() // Ensure context is cancelled
		}()

		// Execute pre-deploy hooks
		if err := r.hooks.ExecutePreDeployHooks(deployCtx, req.DeploymentID, req.ClusterARN, req.ServiceName); err != nil {
			r.statuses.Store(req.DeploymentID, &DeploymentStatus{
				Status:    "FAILED",
				Message:   fmt.Sprintf("pre-deploy hook failed: %v", err),
				Progress:  100,
				StartTime: startTime,
				EndTime:   time.Now(),
			})
			metrics.RecordDeployment(req.Strategy, "failed", time.Since(startTime))
			return
		}

		// Check if deployment was cancelled before execution
		select {
		case <-deployCtx.Done():
			r.statuses.Store(req.DeploymentID, &DeploymentStatus{
				Status:    "CANCELLED",
				Message:   "deployment cancelled before execution",
				Progress:  100,
				StartTime: startTime,
				EndTime:   time.Now(),
			})
			metrics.RecordDeployment(req.Strategy, "cancelled", time.Since(startTime))
			return
		default:
		}

		err := strat.Execute(deployCtx, &strategy.DeploymentContext{
			DeploymentID:   req.DeploymentID,
			ClusterARN:     req.ClusterARN,
			ServiceName:    req.ServiceName,
			TaskDefinition: req.TaskDefinition,
			Config:         req.Config,
		})

		endTime := time.Now()
		duration := endTime.Sub(startTime)

		if err != nil {
			status := "FAILED"
			if err == context.Canceled {
				status = "CANCELLED"
			}
			r.statuses.Store(req.DeploymentID, &DeploymentStatus{
				Status:    status,
				Message:   err.Error(),
				Progress:  100,
				StartTime: startTime,
				EndTime:   endTime,
			})
			metrics.RecordDeployment(req.Strategy, status, duration)
		} else {
			// Execute post-deploy hooks
			if hookErr := r.hooks.ExecutePostDeployHooks(deployCtx, req.DeploymentID, req.ClusterARN, req.ServiceName); hookErr != nil {
				r.statuses.Store(req.DeploymentID, &DeploymentStatus{
					Status:    "FAILED",
					Message:   fmt.Sprintf("post-deploy hook failed: %v", hookErr),
					Progress:  100,
					StartTime: startTime,
					EndTime:   time.Now(),
				})
				metrics.RecordDeployment(req.Strategy, "failed", duration)
				return
			}

			r.statuses.Store(req.DeploymentID, &DeploymentStatus{
				Status:    "SUCCESS",
				Message:   "deployment completed",
				Progress:  100,
				StartTime: startTime,
				EndTime:   endTime,
			})
			metrics.RecordDeployment(req.Strategy, "success", duration)
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

// CancelDeployment cancels an in-progress deployment
func (r *Router) CancelDeployment(deploymentID string) error {
	// Get deployment status
	val, ok := r.statuses.Load(deploymentID)
	if !ok {
		return fmt.Errorf("deployment not found: %s", deploymentID)
	}

	status := val.(*DeploymentStatus)
	if status.Status != "RUNNING" {
		return fmt.Errorf("deployment %s is not running (status: %s)", deploymentID, status.Status)
	}

	// Cancel the deployment context
	if cancelFunc, ok := r.cancelFuncs.Load(deploymentID); ok {
		cancel := cancelFunc.(context.CancelFunc)
		cancel()
		log.Printf("[ROUTER] Cancellation requested for deployment %s", deploymentID)
		return nil
	}

	return fmt.Errorf("cancel function not found for deployment %s", deploymentID)
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

// ApproveDeployment approves or rejects a deployment
func (r *Router) ApproveDeployment(ctx context.Context, deploymentID string, approved bool, approver, reason string) error {
	if approved {
		return r.approvalManager.ApproveDeployment(ctx, deploymentID, approver, reason)
	}
	return r.approvalManager.RejectDeployment(ctx, deploymentID, approver, reason)
}
