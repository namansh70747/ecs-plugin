package executor

import (
	"context"
	"fmt"
	"log"
	"time"
)

type DriftStatus string

const (
	DriftNone     DriftStatus = "none"
	DriftDetected DriftStatus = "detected"
	DriftFixed    DriftStatus = "fixed"
)

type DriftResult struct {
	Status          DriftStatus
	Drifts          []string
	DetectedAt      time.Time
	ReconciledAt    time.Time
	ReconcileAction string
}

func (e *Executor) DetectDrift(ctx context.Context, cluster, service, expectedTaskDef string) (*DriftResult, error) {
	log.Printf("[DRIFT] Detecting drift for service %s", service)

	result := &DriftResult{
		Status:     DriftNone,
		Drifts:     []string{},
		DetectedAt: time.Now(),
	}

	// Get current service state
	currentSvc, err := e.ecsClient.DescribeService(ctx, cluster, service)
	if err != nil {
		return nil, fmt.Errorf("failed to describe service: %w", err)
	}

	currentTaskDef := *currentSvc.TaskDefinition

	// Check task definition drift
	if currentTaskDef != expectedTaskDef {
		result.Status = DriftDetected
		result.Drifts = append(result.Drifts, fmt.Sprintf("Task definition drift: expected %s, found %s", expectedTaskDef, currentTaskDef))
		log.Printf("[DRIFT] Task definition drift detected: expected %s, found %s", expectedTaskDef, currentTaskDef)
	}

	// Check desired count drift (if configured)
	if currentSvc.DesiredCount == 0 {
		result.Status = DriftDetected
		result.Drifts = append(result.Drifts, fmt.Sprintf("Service scaled to zero: desired count is %d", currentSvc.DesiredCount))
		log.Printf("[DRIFT] Service scaled to zero unexpectedly")
	}

	// Check running count vs desired
	if currentSvc.RunningCount < currentSvc.DesiredCount {
		result.Status = DriftDetected
		result.Drifts = append(result.Drifts, fmt.Sprintf("Running count (%d) less than desired (%d)", currentSvc.RunningCount, currentSvc.DesiredCount))
		log.Printf("[DRIFT] Running count drift: running=%d, desired=%d", currentSvc.RunningCount, currentSvc.DesiredCount)
	}

	if result.Status == DriftNone {
		log.Printf("[DRIFT] No drift detected for service %s", service)
	} else {
		log.Printf("[DRIFT] Detected %d drift(s) for service %s", len(result.Drifts), service)
	}

	return result, nil
}

func (e *Executor) ReconcileDrift(ctx context.Context, cluster, service, expectedTaskDef string) error {
	log.Printf("[DRIFT] Reconciling drift for service %s", service)

	// Detect drift first
	drift, err := e.DetectDrift(ctx, cluster, service, expectedTaskDef)
	if err != nil {
		return fmt.Errorf("failed to detect drift: %w", err)
	}

	if drift.Status == DriftNone {
		log.Printf("[DRIFT] No drift to reconcile")
		return nil
	}

	log.Printf("[DRIFT] Found %d drift(s), reconciling...", len(drift.Drifts))

	// Reconcile by updating service to expected task definition
	err = e.UpdateService(ctx, cluster, service, expectedTaskDef)
	if err != nil {
		return fmt.Errorf("failed to reconcile drift: %w", err)
	}

	// Wait for service to stabilize
	err = e.WaitForServiceStable(ctx, cluster, service, 5*time.Minute)
	if err != nil {
		return fmt.Errorf("service failed to stabilize after reconciliation: %w", err)
	}

	log.Printf("[DRIFT] Successfully reconciled drift for service %s", service)
	return nil
}

func (e *Executor) MonitorDrift(ctx context.Context, cluster, service, expectedTaskDef string, interval time.Duration) error {
	if interval == 0 {
		interval = 5 * time.Minute
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log.Printf("[DRIFT] Starting drift monitoring for service %s (interval: %v)", service, interval)

	for {
		select {
		case <-ctx.Done():
			log.Printf("[DRIFT] Drift monitoring stopped for service %s", service)
			return ctx.Err()
		case <-ticker.C:
			drift, err := e.DetectDrift(ctx, cluster, service, expectedTaskDef)
			if err != nil {
				log.Printf("[DRIFT] Error detecting drift: %v", err)
				continue
			}

			if drift.Status == DriftDetected {
				log.Printf("[DRIFT] Drift detected, auto-reconciling...")
				err = e.ReconcileDrift(ctx, cluster, service, expectedTaskDef)
				if err != nil {
					log.Printf("[DRIFT] Failed to auto-reconcile: %v", err)
				}
			}
		}
	}
}
