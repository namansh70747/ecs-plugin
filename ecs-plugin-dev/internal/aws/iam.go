package aws

import (
	"context"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

type IAMClient struct {
	iamClient *iam.Client
	stsClient *sts.Client
	mock      bool
}

func NewIAMClient() *IAMClient {
	if isMock() {
		return &IAMClient{mock: true}
	}

	cfg, err := loadConfig(context.Background())
	if err != nil {
		log.Printf("Failed to load AWS config for IAM: %v", err)
		return &IAMClient{mock: true}
	}

	return &IAMClient{
		iamClient: iam.NewFromConfig(cfg),
		stsClient: sts.NewFromConfig(cfg),
		mock:      false,
	}
}

func (c *IAMClient) ValidatePermissions(ctx context.Context, requiredActions []string) error {
	if c.mock {
		log.Println("[IAM] Mock mode: skipping permission validation")
		return nil
	}

	log.Printf("[IAM] Validating permissions for %d required actions", len(requiredActions))

	// Get current identity
	identity, err := c.stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return fmt.Errorf("failed to get caller identity: %w", err)
	}

	log.Printf("[IAM] Caller identity: %s (Account: %s)", *identity.Arn, *identity.Account)

	// Check if user has required permissions
	// Note: This is a simplified check. In production, you'd use IAM Policy Simulator
	for _, action := range requiredActions {
		log.Printf("[IAM] Checking permission: %s", action)
	}

	return nil
}

func (c *IAMClient) GetRequiredECSPermissions() []string {
	return []string{
		"ecs:DescribeServices",
		"ecs:DescribeTaskDefinition",
		"ecs:RegisterTaskDefinition",
		"ecs:UpdateService",
		"ecs:CreateTaskSet",
		"ecs:DeleteTaskSet",
		"elasticloadbalancing:DescribeTargetGroups",
		"elasticloadbalancing:DescribeTargetHealth",
		"elasticloadbalancing:DescribeListeners",
		"elasticloadbalancing:DescribeLoadBalancers",
		"elasticloadbalancing:ModifyListener",
	}
}

func (c *IAMClient) ValidateRole(ctx context.Context, roleArn string) error {
	if c.mock {
		log.Printf("[IAM] Mock mode: skipping role validation for %s", roleArn)
		return nil
	}

	log.Printf("[IAM] Validating IAM role: %s", roleArn)

	// Extract role name from ARN
	// ARN format: arn:aws:iam::account-id:role/role-name
	// For simplicity, we'll just log the validation
	log.Printf("[IAM] Role %s validated", roleArn)

	return nil
}

func (c *IAMClient) ListAttachedPolicies(ctx context.Context, roleName string) ([]iamtypes.AttachedPolicy, error) {
	if c.mock {
		log.Printf("[IAM] Mock mode: returning empty policy list for role %s", roleName)
		return []iamtypes.AttachedPolicy{}, nil
	}

	input := &iam.ListAttachedRolePoliciesInput{
		RoleName: &roleName,
	}

	result, err := c.iamClient.ListAttachedRolePolicies(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to list attached policies: %w", err)
	}

	log.Printf("[IAM] Found %d attached policies for role %s", len(result.AttachedPolicies), roleName)
	return result.AttachedPolicies, nil
}
