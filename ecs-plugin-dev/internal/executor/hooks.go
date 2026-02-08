package executor

import (
	"context"
	"fmt"
	"log"
)

// HookType defines the type of deployment hook
type HookType string

const (
	PreDeployHook  HookType = "pre-deploy"
	PostDeployHook HookType = "post-deploy"
)

// Hook represents a deployment hook
type Hook struct {
	Name string
	Fn   func(ctx context.Context, deploymentID, cluster, service string) error
}

// HookRegistry stores registered hooks
type HookRegistry struct {
	preDeployHooks  []Hook
	postDeployHooks []Hook
}

// NewHookRegistry creates a new hook registry
func NewHookRegistry() *HookRegistry {
	return &HookRegistry{
		preDeployHooks:  []Hook{},
		postDeployHooks: []Hook{},
	}
}

// RegisterHook registers a new hook
func (h *HookRegistry) RegisterHook(hookType HookType, hook Hook) {
	switch hookType {
	case PreDeployHook:
		h.preDeployHooks = append(h.preDeployHooks, hook)
	case PostDeployHook:
		h.postDeployHooks = append(h.postDeployHooks, hook)
	}
}

// ExecutePreDeployHooks executes all pre-deployment hooks
func (h *HookRegistry) ExecutePreDeployHooks(ctx context.Context, deploymentID, cluster, service string) error {
	log.Printf("[HOOKS] Executing %d pre-deploy hooks", len(h.preDeployHooks))
	for _, hook := range h.preDeployHooks {
		log.Printf("[HOOK] Running pre-deploy hook: %s", hook.Name)
		if err := hook.Fn(ctx, deploymentID, cluster, service); err != nil {
			return fmt.Errorf("pre-deploy hook %s failed: %w", hook.Name, err)
		}
	}
	return nil
}

// ExecutePostDeployHooks executes all post-deployment hooks
func (h *HookRegistry) ExecutePostDeployHooks(ctx context.Context, deploymentID, cluster, service string) error {
	log.Printf("[HOOKS] Executing %d post-deploy hooks", len(h.postDeployHooks))
	for _, hook := range h.postDeployHooks {
		log.Printf("[HOOK] Running post-deploy hook: %s", hook.Name)
		if err := hook.Fn(ctx, deploymentID, cluster, service); err != nil {
			return fmt.Errorf("post-deploy hook %s failed: %w", hook.Name, err)
		}
	}
	return nil
}

// Default hooks
func ValidationHook(ctx context.Context, deploymentID, cluster, service string) error {
	log.Printf("[HOOK] Validating deployment: %s", deploymentID)
	if deploymentID == "" || cluster == "" || service == "" {
		return fmt.Errorf("invalid deployment parameters")
	}
	return nil
}

func HealthCheckHook(ctx context.Context, deploymentID, cluster, service string) error {
	log.Printf("[HOOK] Running health check for deployment: %s", deploymentID)
	// In production, this would check service health metrics
	return nil
}

func NotificationHook(ctx context.Context, deploymentID, cluster, service string) error {
	log.Printf("[HOOK] Sending notification for deployment: %s", deploymentID)
	// In production, this would send notifications (Slack, email, etc.)
	return nil
}
