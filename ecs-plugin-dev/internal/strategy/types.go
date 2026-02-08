// internal/strategy/types.go
package strategy

import "context"

type DeploymentContext struct {
    DeploymentID   string
    ClusterARN     string
    ServiceName    string
    TaskDefinition string
    Config         map[string]string
}

type Strategy interface {
    Execute(ctx context.Context, dctx *DeploymentContext) error
}