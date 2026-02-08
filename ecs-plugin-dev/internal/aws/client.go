package aws

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
)

// isMock checks if running in mock mode
func isMock() bool {
	return os.Getenv("MOCK_MODE") == "true"
}

// loadConfig creates AWS config for LocalStack or real AWS
func loadConfig(ctx context.Context) (aws.Config, error) {
	opts := []func(*config.LoadOptions) error{}

	// Static credentials (for LocalStack fake env)
	if ak := os.Getenv("AWS_ACCESS_KEY_ID"); ak != "" {
		sk := os.Getenv("AWS_SECRET_ACCESS_KEY")
		token := os.Getenv("AWS_SESSION_TOKEN")
		opts = append(opts, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(ak, sk, token),
		))
	}

	// Region
	if r := os.Getenv("AWS_REGION"); r != "" {
		opts = append(opts, config.WithRegion(r))
	} else {
		opts = append(opts, config.WithRegion("us-east-1"))
	}

	// LocalStack endpoint
	if ep := os.Getenv("AWS_ENDPOINT_URL"); ep != "" {
		log.Printf("[AWS] Using custom endpoint: %s (LocalStack mode)", ep)
		opts = append(opts, config.WithEndpointResolverWithOptions(
			aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
				return aws.Endpoint{
					URL:               ep,
					HostnameImmutable: true,
					SigningRegion:     region,
				}, nil
			}),
		))
	}

	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return aws.Config{}, fmt.Errorf("failed to load AWS config: %w", err)
	}

	return cfg, nil
}
