package grpc

import (
	"context"
	"fmt"
	"strings"

	"ecs-plugin-dev/internal/audit"
	"ecs-plugin-dev/internal/plugin"
	pb "ecs-plugin-dev/proto"
)

type DeploymentServer struct {
	pb.UnimplementedDeploymentServiceServer
	router *plugin.Router
}

func NewDeploymentServer() *DeploymentServer {
	return &DeploymentServer{
		router: plugin.NewRouter(),
	}
}

func (s *DeploymentServer) Deploy(ctx context.Context, req *pb.DeployRequest) (*pb.DeployResponse, error) {
	// Validate request
	if err := s.validateDeployRequest(req); err != nil {
		return &pb.DeployResponse{
			Success: false,
			Message: fmt.Sprintf("invalid request: %v", err),
		}, nil
	}

	result, err := s.router.RouteDeployment(ctx, &plugin.DeploymentRequest{
		DeploymentID:   req.DeploymentId,
		ClusterARN:     req.ClusterArn,
		ServiceName:    req.ServiceName,
		TaskDefinition: req.TaskDefinition,
		Strategy:       req.Strategy,
		Config:         req.Config,
	})

	if err != nil {
		return &pb.DeployResponse{
			Success: false,
			Message: fmt.Sprintf("deployment failed: %v", err),
		}, nil
	}

	return &pb.DeployResponse{
		Success:      result.Success,
		Message:      result.Message,
		DeploymentId: result.DeploymentID,
	}, nil
}

func (s *DeploymentServer) GetStatus(ctx context.Context, req *pb.StatusRequest) (*pb.StatusResponse, error) {
	status, err := s.router.GetDeploymentStatus(ctx, req.DeploymentId)
	if err != nil {
		return &pb.StatusResponse{
			Status:  "UNKNOWN",
			Message: err.Error(),
		}, nil
	}

	return &pb.StatusResponse{
		Status:   status.Status,
		Message:  status.Message,
		Progress: status.Progress,
	}, nil
}

func (s *DeploymentServer) Rollback(ctx context.Context, req *pb.RollbackRequest) (*pb.RollbackResponse, error) {
	err := s.router.Rollback(ctx, req.DeploymentId, req.ClusterArn, req.ServiceName)
	if err != nil {
		return &pb.RollbackResponse{
			Success: false,
			Message: fmt.Sprintf("rollback failed: %v", err),
		}, nil
	}

	return &pb.RollbackResponse{
		Success: true,
		Message: "rollback initiated successfully",
	}, nil
}

func classifyError(err error) (string, string) {
	if err == nil {
		return "", ""
	}

	errMsg := err.Error()

	// Validation errors
	if strings.Contains(errMsg, "cannot be empty") || strings.Contains(errMsg, "invalid") {
		return "VALIDATION_ERROR", "Request validation failed"
	}

	// AWS errors
	if strings.Contains(errMsg, "failed to") && (strings.Contains(errMsg, "describe") || strings.Contains(errMsg, "update") || strings.Contains(errMsg, "register")) {
		return "AWS_API_ERROR", "AWS API call failed"
	}

	// Timeout errors
	if strings.Contains(errMsg, "timeout") || strings.Contains(errMsg, "deadline") {
		return "TIMEOUT_ERROR", "Operation timed out"
	}

	// Context errors
	if strings.Contains(errMsg, "context canceled") {
		return "CANCELLED_ERROR", "Deployment was cancelled"
	}

	// Health check errors
	if strings.Contains(errMsg, "health") || strings.Contains(errMsg, "unhealthy") {
		return "HEALTH_CHECK_ERROR", "Service health check failed"
	}

	// Default
	return "INTERNAL_ERROR", "Internal server error"
}

func (s *DeploymentServer) ApproveDeployment(ctx context.Context, req *pb.ApprovalRequest) (*pb.ApprovalResponse, error) {
	if req.DeploymentId == "" {
		return &pb.ApprovalResponse{
			Success: false,
			Message: "deployment_id is required",
		}, nil
	}

	err := s.router.ApproveDeployment(ctx, req.DeploymentId, req.Approved, req.Approver, req.Reason)
	if err != nil {
		return &pb.ApprovalResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	if auditLogger := audit.GetGlobalAuditLogger(); auditLogger != nil {
		if req.Approved {
			auditLogger.LogApprovalGranted(req.DeploymentId, req.Approver, req.Reason)
		} else {
			auditLogger.Log(audit.AuditEvent{
				EventType:    audit.EventApprovalRejected,
				DeploymentID: req.DeploymentId,
				User:         req.Approver,
				Status:       "rejected",
			})
		}
	}

	return &pb.ApprovalResponse{
		Success: true,
		Message: fmt.Sprintf("Deployment %s", map[bool]string{true: "approved", false: "rejected"}[req.Approved]),
	}, nil
}

// validateDeployRequest validates deploy request fields
func (s *DeploymentServer) validateDeployRequest(req *pb.DeployRequest) error {
	if req.DeploymentId == "" {
		return fmt.Errorf("deployment_id is required")
	}
	if req.ClusterArn == "" {
		return fmt.Errorf("cluster_arn is required")
	}
	if req.ServiceName == "" {
		return fmt.Errorf("service_name is required")
	}
	if req.TaskDefinition == "" {
		return fmt.Errorf("task_definition is required")
	}
	if req.Strategy == "" {
		return fmt.Errorf("strategy is required")
	}
	return nil
}
