// internal/aws/ecs.go
package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

type ECSClient struct {
	client *ecs.Client
	mock   bool
}

func NewECSClient() *ECSClient {
	if isMock() {
		log.Println("[MOCK] ECS client in mock mode")
		return &ECSClient{mock: true}
	}
	cfg, err := loadConfig(context.Background())
	if err != nil {
		panic(fmt.Sprintf("failed to create ECS client: %v", err))
	}
	return &ECSClient{
		client: ecs.NewFromConfig(cfg),
	}
}

func (c *ECSClient) RegisterTaskDefinition(ctx context.Context, taskDefJSON string) error {
	if c.mock {
		log.Printf("[MOCK] RegisterTaskDefinition: %s", taskDefJSON)
		return nil
	}
	var taskDef ecs.RegisterTaskDefinitionInput
	if err := json.Unmarshal([]byte(taskDefJSON), &taskDef); err != nil {
		return fmt.Errorf("invalid task definition: %w", err)
	}

	_, err := c.client.RegisterTaskDefinition(ctx, &taskDef)
	return err
}

func (c *ECSClient) UpdateService(ctx context.Context, cluster, service, taskDef string) error {
	if c.mock {
		log.Printf("[MOCK] UpdateService: cluster=%s, service=%s", cluster, service)
		return nil
	}
	_, err := c.client.UpdateService(ctx, &ecs.UpdateServiceInput{
		Cluster:            aws.String(cluster),
		Service:            aws.String(service),
		TaskDefinition:     aws.String(taskDef),
		ForceNewDeployment: true,
	})
	return err
}

func (c *ECSClient) CreateTaskSet(ctx context.Context, cluster, service, taskDef string, weight int) error {
	if c.mock {
		log.Printf("[MOCK] CreateTaskSet: cluster=%s, service=%s, weight=%d%%", cluster, service, weight)
		return nil
	}
	_, err := c.client.CreateTaskSet(ctx, &ecs.CreateTaskSetInput{
		Cluster:        aws.String(cluster),
		Service:        aws.String(service),
		TaskDefinition: aws.String(taskDef),
		Scale: &types.Scale{
			Unit:  types.ScaleUnitPercent,
			Value: float64(weight),
		},
	})
	return err
}

func (c *ECSClient) DeleteTaskSet(ctx context.Context, cluster, service, taskSetID string) error {
	if c.mock {
		log.Printf("[MOCK] DeleteTaskSet: cluster=%s, service=%s, taskSetID=%s", cluster, service, taskSetID)
		return nil
	}
	_, err := c.client.DeleteTaskSet(ctx, &ecs.DeleteTaskSetInput{
		Cluster: aws.String(cluster),
		Service: aws.String(service),
		TaskSet: aws.String(taskSetID),
		Force:   aws.Bool(true),
	})
	return err
}

func (c *ECSClient) GetPreviousTaskDefinition(ctx context.Context, cluster, service string) (string, error) {
	if c.mock {
		log.Printf("[MOCK] GetPreviousTaskDefinition: cluster=%s, service=%s", cluster, service)
		return "arn:aws:ecs:us-east-1:123456789:task-definition/previous:1", nil
	}
	resp, err := c.client.DescribeServices(ctx, &ecs.DescribeServicesInput{
		Cluster:  aws.String(cluster),
		Services: []string{service},
	})
	if err != nil {
		return "", err
	}

	if len(resp.Services) == 0 {
		return "", fmt.Errorf("service not found")
	}

	deployments := resp.Services[0].Deployments
	if len(deployments) < 2 {
		return "", fmt.Errorf("no previous deployment found")
	}

	return *deployments[1].TaskDefinition, nil
}
