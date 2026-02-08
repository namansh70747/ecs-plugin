package audit

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"
)

type AuditEventType string

const (
	EventDeploymentStarted   AuditEventType = "deployment.started"
	EventDeploymentCompleted AuditEventType = "deployment.completed"
	EventDeploymentFailed    AuditEventType = "deployment.failed"
	EventDeploymentCancelled AuditEventType = "deployment.cancelled"
	EventDeploymentRollback  AuditEventType = "deployment.rollback"
	EventApprovalRequested   AuditEventType = "approval.requested"
	EventApprovalGranted     AuditEventType = "approval.granted"
	EventApprovalRejected    AuditEventType = "approval.rejected"
	EventDriftDetected       AuditEventType = "drift.detected"
	EventDriftReconciled     AuditEventType = "drift.reconciled"
)

type AuditEvent struct {
	Timestamp    time.Time              `json:"timestamp"`
	EventType    AuditEventType         `json:"event_type"`
	DeploymentID string                 `json:"deployment_id"`
	User         string                 `json:"user,omitempty"`
	ClusterARN   string                 `json:"cluster_arn,omitempty"`
	ServiceName  string                 `json:"service_name,omitempty"`
	Strategy     string                 `json:"strategy,omitempty"`
	Status       string                 `json:"status,omitempty"`
	ErrorCode    string                 `json:"error_code,omitempty"`
	ErrorMessage string                 `json:"error_message,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

type AuditLogger struct {
	mu      sync.Mutex
	file    *os.File
	events  []AuditEvent
	maxSize int
}

func NewAuditLogger(logPath string) (*AuditLogger, error) {
	if logPath == "" {
		logPath = "/var/log/ecs-plugin/audit.log"
	}

	// Create directory if it doesn't exist
	dir := "/var/log/ecs-plugin"
	if err := os.MkdirAll(dir, 0755); err != nil {
		// Fallback to temp directory
		dir = os.TempDir()
		logPath = fmt.Sprintf("%s/ecs-plugin-audit.log", dir)
		log.Printf("[AUDIT] Using fallback log path: %s", logPath)
	}

	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open audit log file: %w", err)
	}

	log.Printf("[AUDIT] Audit logging initialized: %s", logPath)

	return &AuditLogger{
		file:    file,
		events:  []AuditEvent{},
		maxSize: 10000,
	}, nil
}

func (al *AuditLogger) Log(event AuditEvent) error {
	al.mu.Lock()
	defer al.mu.Unlock()

	event.Timestamp = time.Now()

	// Write to file
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal audit event: %w", err)
	}

	if _, err := al.file.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("failed to write audit event: %w", err)
	}

	// Also log to standard logger
	log.Printf("[AUDIT] %s | %s | %s | %s", event.EventType, event.DeploymentID, event.Status, event.User)

	// Keep in memory for queries
	al.events = append(al.events, event)
	if len(al.events) > al.maxSize {
		al.events = al.events[len(al.events)-al.maxSize:]
	}

	return nil
}

func (al *AuditLogger) LogDeploymentStarted(deploymentID, cluster, service, strategy, user string) error {
	return al.Log(AuditEvent{
		EventType:    EventDeploymentStarted,
		DeploymentID: deploymentID,
		User:         user,
		ClusterARN:   cluster,
		ServiceName:  service,
		Strategy:     strategy,
		Status:       "started",
	})
}

func (al *AuditLogger) LogDeploymentCompleted(deploymentID, user string, duration time.Duration) error {
	return al.Log(AuditEvent{
		EventType:    EventDeploymentCompleted,
		DeploymentID: deploymentID,
		User:         user,
		Status:       "completed",
		Metadata: map[string]interface{}{
			"duration_seconds": duration.Seconds(),
		},
	})
}

func (al *AuditLogger) LogDeploymentFailed(deploymentID, user, errorCode, errorMsg string) error {
	return al.Log(AuditEvent{
		EventType:    EventDeploymentFailed,
		DeploymentID: deploymentID,
		User:         user,
		Status:       "failed",
		ErrorCode:    errorCode,
		ErrorMessage: errorMsg,
	})
}

func (al *AuditLogger) LogApprovalGranted(deploymentID, approver, reason string) error {
	return al.Log(AuditEvent{
		EventType:    EventApprovalGranted,
		DeploymentID: deploymentID,
		User:         approver,
		Status:       "approved",
		Metadata: map[string]interface{}{
			"reason": reason,
		},
	})
}

func (al *AuditLogger) GetEvents(limit int) []AuditEvent {
	al.mu.Lock()
	defer al.mu.Unlock()

	if limit <= 0 || limit > len(al.events) {
		limit = len(al.events)
	}

	start := len(al.events) - limit
	if start < 0 {
		start = 0
	}

	result := make([]AuditEvent, limit)
	copy(result, al.events[start:])
	return result
}

func (al *AuditLogger) Close() error {
	al.mu.Lock()
	defer al.mu.Unlock()

	if al.file != nil {
		return al.file.Close()
	}
	return nil
}

var globalAuditLogger *AuditLogger
var auditOnce sync.Once

func GetGlobalAuditLogger() *AuditLogger {
	auditOnce.Do(func() {
		logger, err := NewAuditLogger("")
		if err != nil {
			log.Printf("[AUDIT] Failed to initialize audit logger: %v", err)
			return
		}
		globalAuditLogger = logger
	})
	return globalAuditLogger
}
