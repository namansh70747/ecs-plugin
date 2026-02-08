package executor

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

type ApprovalStatus string

const (
	ApprovalPending  ApprovalStatus = "pending"
	ApprovalApproved ApprovalStatus = "approved"
	ApprovalRejected ApprovalStatus = "rejected"
)

type ApprovalRequest struct {
	DeploymentID string
	ClusterARN   string
	ServiceName  string
	Strategy     string
	RequestedAt  time.Time
	Status       ApprovalStatus
	Approver     string
	Reason       string
}

type ApprovalManager struct {
	mu       sync.RWMutex
	requests map[string]*ApprovalRequest
}

func NewApprovalManager() *ApprovalManager {
	return &ApprovalManager{
		requests: make(map[string]*ApprovalRequest),
	}
}

func (am *ApprovalManager) RequestApproval(ctx context.Context, deploymentID, cluster, service, strategy string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	req := &ApprovalRequest{
		DeploymentID: deploymentID,
		ClusterARN:   cluster,
		ServiceName:  service,
		Strategy:     strategy,
		RequestedAt:  time.Now(),
		Status:       ApprovalPending,
	}
	am.requests[deploymentID] = req

	log.Printf("[APPROVAL] Deployment %s requires approval (cluster: %s, service: %s, strategy: %s)",
		deploymentID, cluster, service, strategy)

	return nil
}

func (am *ApprovalManager) ApproveDeployment(ctx context.Context, deploymentID, approver, reason string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	req, exists := am.requests[deploymentID]
	if !exists {
		return fmt.Errorf("approval request not found for deployment %s", deploymentID)
	}

	if req.Status != ApprovalPending {
		return fmt.Errorf("deployment %s already %s", deploymentID, req.Status)
	}

	req.Status = ApprovalApproved
	req.Approver = approver
	req.Reason = reason

	log.Printf("[APPROVAL] Deployment %s approved by %s: %s", deploymentID, approver, reason)
	return nil
}

func (am *ApprovalManager) RejectDeployment(ctx context.Context, deploymentID, approver, reason string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	req, exists := am.requests[deploymentID]
	if !exists {
		return fmt.Errorf("approval request not found for deployment %s", deploymentID)
	}

	if req.Status != ApprovalPending {
		return fmt.Errorf("deployment %s already %s", deploymentID, req.Status)
	}

	req.Status = ApprovalRejected
	req.Approver = approver
	req.Reason = reason

	log.Printf("[APPROVAL] Deployment %s rejected by %s: %s", deploymentID, approver, reason)
	return nil
}

func (am *ApprovalManager) GetApprovalStatus(deploymentID string) (ApprovalStatus, error) {
	am.mu.RLock()
	defer am.mu.RUnlock()

	req, exists := am.requests[deploymentID]
	if !exists {
		return "", fmt.Errorf("approval request not found for deployment %s", deploymentID)
	}

	return req.Status, nil
}

func (am *ApprovalManager) WaitForApproval(ctx context.Context, deploymentID string, timeout time.Duration) error {
	if timeout == 0 {
		timeout = 30 * time.Minute
	}

	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	log.Printf("[APPROVAL] Waiting for approval of deployment %s (timeout: %v)", deploymentID, timeout)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if time.Now().After(deadline) {
				return fmt.Errorf("approval timeout for deployment %s", deploymentID)
			}

			status, err := am.GetApprovalStatus(deploymentID)
			if err != nil {
				return err
			}

			switch status {
			case ApprovalApproved:
				log.Printf("[APPROVAL] Deployment %s approved, proceeding", deploymentID)
				return nil
			case ApprovalRejected:
				return fmt.Errorf("deployment %s rejected", deploymentID)
			}
		}
	}
}
