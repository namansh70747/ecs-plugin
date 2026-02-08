// internal/aws/elb.go
package aws

import (
	"context"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
)

type ELBClient struct {
	client *elasticloadbalancingv2.Client
	mock   bool
}

func NewELBClient() *ELBClient {
	if isMock() {
		log.Println("[MOCK] ELB client in mock mode")
		return &ELBClient{mock: true}
	}
	cfg, err := loadConfig(context.Background())
	if err != nil {
		panic(fmt.Sprintf("failed to create ELB client: %v", err))
	}
	return &ELBClient{
		client: elasticloadbalancingv2.NewFromConfig(cfg),
	}
}

func (c *ELBClient) UpdateTargetGroupWeights(ctx context.Context, cluster, service string, canaryWeight, primaryWeight int) error {
	if c.mock {
		log.Printf("[MOCK] UpdateTargetGroupWeights: canary=%d%%, primary=%d%%", canaryWeight, primaryWeight)
		return nil
	}

	// Discover listener ARN from service tags or configuration
	listenerArn, err := c.discoverListenerArn(ctx, cluster, service)
	if err != nil {
		return fmt.Errorf("failed to discover listener ARN: %w", err)
	}

	// Get target groups for this listener
	canaryTG, primaryTG, err := c.getTargetGroups(ctx, listenerArn)
	if err != nil {
		return fmt.Errorf("failed to get target groups: %w", err)
	}

	// Validate target group health before shifting traffic
	if err := c.validateTargetGroupHealth(ctx, canaryTG, primaryTG); err != nil {
		log.Printf("[WARN] Target group health validation failed: %v", err)
	}

	_, err = c.client.ModifyListener(ctx, &elasticloadbalancingv2.ModifyListenerInput{
		ListenerArn: aws.String(listenerArn),
		DefaultActions: []types.Action{
			{
				Type: types.ActionTypeEnumForward,
				ForwardConfig: &types.ForwardActionConfig{
					TargetGroups: []types.TargetGroupTuple{
						{
							TargetGroupArn: aws.String(canaryTG),
							Weight:         aws.Int32(int32(canaryWeight)),
						},
						{
							TargetGroupArn: aws.String(primaryTG),
							Weight:         aws.Int32(int32(primaryWeight)),
						},
					},
				},
			},
		},
	})
	return err
}

// discoverListenerArn discovers the ALB listener ARN from service configuration
func (c *ELBClient) discoverListenerArn(ctx context.Context, cluster, service string) (string, error) {
	if c.mock {
		log.Println("[ELB] Mock mode: returning test listener ARN")
		return "arn:aws:elasticloadbalancing:us-east-1:123456789012:listener/app/my-load-balancer/50dc6c495c0c9188/f2f7dc8efc522ab2", nil
	}

	log.Printf("[ELB] Discovering listener ARN for service %s", service)

	// Get ECS service to find load balancers
	ecsClient := NewECSClient()
	svc, err := ecsClient.DescribeService(ctx, cluster, service)
	if err != nil {
		return "", fmt.Errorf("failed to describe service: %w", err)
	}

	if len(svc.LoadBalancers) == 0 {
		return "", fmt.Errorf("no load balancers found for service %s", service)
	}

	// Get target group ARN from service
	targetGroupArn := *svc.LoadBalancers[0].TargetGroupArn
	log.Printf("[ELB] Found target group: %s", targetGroupArn)

	// Describe target group to get load balancer ARN
	tgResp, err := c.client.DescribeTargetGroups(ctx, &elasticloadbalancingv2.DescribeTargetGroupsInput{
		TargetGroupArns: []string{targetGroupArn},
	})
	if err != nil {
		return "", fmt.Errorf("failed to describe target groups: %w", err)
	}

	if len(tgResp.TargetGroups) == 0 || len(tgResp.TargetGroups[0].LoadBalancerArns) == 0 {
		return "", fmt.Errorf("no load balancer found for target group")
	}

	lbArn := tgResp.TargetGroups[0].LoadBalancerArns[0]
	log.Printf("[ELB] Found load balancer: %s", lbArn)

	// Get listeners for the load balancer
	listenersResp, err := c.client.DescribeListeners(ctx, &elasticloadbalancingv2.DescribeListenersInput{
		LoadBalancerArn: &lbArn,
	})
	if err != nil {
		return "", fmt.Errorf("failed to describe listeners: %w", err)
	}

	if len(listenersResp.Listeners) == 0 {
		return "", fmt.Errorf("no listeners found for load balancer")
	}

	listenerArn := *listenersResp.Listeners[0].ListenerArn
	log.Printf("[ELB] Discovered listener ARN: %s", listenerArn)

	return listenerArn, nil
}

// getTargetGroups retrieves target group ARNs for canary and primary
func (c *ELBClient) getTargetGroups(ctx context.Context, listenerArn string) (string, string, error) {
	// Query listener to get current target groups
	result, err := c.client.DescribeListeners(ctx, &elasticloadbalancingv2.DescribeListenersInput{
		ListenerArns: []string{listenerArn},
	})

	if err != nil {
		return "", "", err
	}

	if len(result.Listeners) == 0 {
		return "", "", fmt.Errorf("listener not found")
	}

	// Extract target groups from listener actions
	if len(result.Listeners[0].DefaultActions) > 0 {
		action := result.Listeners[0].DefaultActions[0]
		if action.ForwardConfig != nil && len(action.ForwardConfig.TargetGroups) >= 2 {
			return *action.ForwardConfig.TargetGroups[0].TargetGroupArn,
				*action.ForwardConfig.TargetGroups[1].TargetGroupArn,
				nil
		}
	}

	return "", "", fmt.Errorf("target groups not found in listener configuration")
}

// validateTargetGroupHealth checks target group health before traffic shift
func (c *ELBClient) validateTargetGroupHealth(ctx context.Context, canaryTG, primaryTG string) error {
	for _, tgArn := range []string{canaryTG, primaryTG} {
		healthResult, err := c.client.DescribeTargetHealth(ctx, &elasticloadbalancingv2.DescribeTargetHealthInput{
			TargetGroupArn: aws.String(tgArn),
		})

		if err != nil {
			return fmt.Errorf("failed to describe target health for %s: %w", tgArn, err)
		}

		// Check if at least one target is healthy
		healthyCount := 0
		for _, target := range healthResult.TargetHealthDescriptions {
			if target.TargetHealth != nil && target.TargetHealth.State == types.TargetHealthStateEnumHealthy {
				healthyCount++
			}
		}

		if healthyCount == 0 {
			return fmt.Errorf("no healthy targets in target group %s", tgArn)
		}

		log.Printf("[ELB] Target group %s has %d healthy targets", tgArn, healthyCount)
	}

	return nil
}
