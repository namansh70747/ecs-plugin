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
	_, err := c.client.ModifyListener(ctx, &elasticloadbalancingv2.ModifyListenerInput{
		DefaultActions: []types.Action{
			{
				Type: types.ActionTypeEnumForward,
				ForwardConfig: &types.ForwardActionConfig{
					TargetGroups: []types.TargetGroupTuple{
						{
							TargetGroupArn: aws.String("canary-tg-arn"),
							Weight:         aws.Int32(int32(canaryWeight)),
						},
						{
							TargetGroupArn: aws.String("primary-tg-arn"),
							Weight:         aws.Int32(int32(primaryWeight)),
						},
					},
				},
			},
		},
	})
	return err
}
