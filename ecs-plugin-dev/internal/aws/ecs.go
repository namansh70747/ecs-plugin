// internal/aws/ecs.go
package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"ecs-plugin-dev/internal/metrics"
	"ecs-plugin-dev/internal/util"

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

	start := time.Now()
	var err error

	retryErr := util.ExponentialBackoff(ctx, util.DefaultRetryConfig(), func() error {
		var taskDef ecs.RegisterTaskDefinitionInput
		if jsonErr := json.Unmarshal([]byte(taskDefJSON), &taskDef); jsonErr != nil {
			return fmt.Errorf("invalid task definition: %w", jsonErr)
		}

		_, err = c.client.RegisterTaskDefinition(ctx, &taskDef)
		return err
	})

	status := "success"
	if retryErr != nil {
		status = "error"
		metrics.RecordError("ecs_client", "register_task_definition")
	}
	metrics.RecordAWSCall("ecs", "RegisterTaskDefinition", status, time.Since(start))

	return retryErr
}

func (c *ECSClient) UpdateService(ctx context.Context, cluster, service, taskDef string) error {
	if c.mock {
		log.Printf("[MOCK] UpdateService: cluster=%s, service=%s", cluster, service)
		return nil
	}

	start := time.Now()
	var err error

	retryErr := util.ExponentialBackoff(ctx, util.DefaultRetryConfig(), func() error {
		_, err = c.client.UpdateService(ctx, &ecs.UpdateServiceInput{
			Cluster:            aws.String(cluster),
			Service:            aws.String(service),
			TaskDefinition:     aws.String(taskDef),
			ForceNewDeployment: true,
		})
		return err
	})

	status := "success"
	if retryErr != nil {
		status = "error"
		metrics.RecordError("ecs_client", "update_service")
	}
	metrics.RecordAWSCall("ecs", "UpdateService", status, time.Since(start))

	return retryErr
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

// DescribeService retrieves service details with retry and metrics
func (c *ECSClient) DescribeService(ctx context.Context, cluster, service string) (*types.Service, error) {
	if c.mock {
		desiredCount := int32(2)
		runningCount := int32(2)
		return &types.Service{
			ServiceName:  aws.String(service),
			Status:       aws.String("ACTIVE"),
			DesiredCount: desiredCount,
			RunningCount: runningCount,
			Deployments: []types.Deployment{
				{
					Status:       aws.String("PRIMARY"),
					RolloutState: types.DeploymentRolloutStateCompleted,
					RunningCount: runningCount,
					DesiredCount: desiredCount,
				},
			},
		}, nil
	}

	start := time.Now()
	var result *ecs.DescribeServicesOutput

	err := util.ExponentialBackoff(ctx, util.DefaultRetryConfig(), func() error {
		var e error
		result, e = c.client.DescribeServices(ctx, &ecs.DescribeServicesInput{
			Cluster:  aws.String(cluster),
			Services: []string{service},
		})
		return e
	})

	metrics.RecordAWSCall("ecs", "DescribeServices", "success", time.Since(start))

	if err != nil {
		metrics.RecordAWSCall("ecs", "DescribeServices", "error", time.Since(start))
		metrics.RecordError("aws", "DescribeServices")
		return nil, fmt.Errorf("describe services failed: %w", err)
	}

	if len(result.Services) == 0 {
		metrics.RecordError("aws", "ServiceNotFound")
		return nil, fmt.Errorf("service %s not found in cluster %s", service, cluster)
	}

	return &result.Services[0], nil
}

// DescribeTaskDefinition retrieves task definition details
func (c *ECSClient) DescribeTaskDefinition(ctx context.Context, taskDef string) (*types.TaskDefinition, error) {
	if c.mock {
		family := "mock-task"
		return &types.TaskDefinition{
			Family: &family,
		}, nil
	}

	start := time.Now()
	var result *ecs.DescribeTaskDefinitionOutput

	err := util.ExponentialBackoff(ctx, util.DefaultRetryConfig(), func() error {
		var e error
		result, e = c.client.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
			TaskDefinition: aws.String(taskDef),
		})
		return e
	})

	metrics.RecordAWSCall("ecs", "DescribeTaskDefinition", "success", time.Since(start))

	if err != nil {
		metrics.RecordAWSCall("ecs", "DescribeTaskDefinition", "error", time.Since(start))
		metrics.RecordError("aws", "DescribeTaskDefinition")
		return nil, fmt.Errorf("describe task definition failed: %w", err)
	}

	return result.TaskDefinition, nil
}
