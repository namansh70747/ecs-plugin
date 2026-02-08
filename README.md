# ECS Deployment Plugin

A production-ready gRPC-based deployment orchestration system for AWS ECS services with support for multiple deployment strategies. This plugin enables safe, controlled application updates across ECS clusters with zero-downtime deployments.

## Overview

The ECS Deployment Plugin provides a flexible framework for managing containerized application deployments on AWS ECS. It supports three distinct deployment strategies that cater to different organizational needs and risk tolerances, from instant deployments to gradual rollouts with traffic shifting capabilities.

### Key Features

- **Multiple Deployment Strategies**: Choose from Quicksync (instant), Canary (gradual with monitoring), or Blue-Green (complete traffic switch)
- **Mock Mode Support**: Full system testing without AWS credentials or live infrastructure
- **Comprehensive Validation**: Request validation at multiple layers prevents invalid deployments
- **Thread-Safe Registry**: Plugin architecture allows easy strategy extension
- **Graceful Shutdown**: Clean server termination with signal handling
- **gRPC Interface**: High-performance RPC communication for deployment operations
- **Deployment Tracking**: Real-time status monitoring for active deployments

## Project Structure

```
ecs-plugin-dev/
├── cmd/                          # Entry points
│   ├── grpc-server/main.go       # Server startup with graceful shutdown
│   └── grpc-client/main.go       # CLI client for testing and operations
├── internal/
│   ├── aws/                      # AWS SDK integration
│   │   ├── client.go             # AWS client configuration and setup
│   │   ├── ecs.go                # ECS service operations (register tasks, update services)
│   │   └── elb.go                # Load balancer traffic management
│   ├── executor/                 # Operation executors
│   │   ├── executor.go           # Main orchestrator
│   │   ├── service.go            # Service validation and stability checks
│   │   ├── taskdef.go            # Task definition validation and parsing
│   │   └── traffic.go            # Traffic shifting and weight validation
│   ├── grpc/
│   │   └── server.go             # gRPC service implementation with request validation
│   ├── plugin/                   # Plugin architecture
│   │   ├── registry.go           # Thread-safe strategy registry
│   │   └── router.go             # Deployment routing and strategy selection
│   └── strategy/                 # Deployment strategies
│       ├── types.go              # Common types and interfaces
│       ├── quicksync.go          # Instant deployment strategy
│       ├── canary.go             # Gradual rollout with traffic shifting
│       └── bluegreen.go          # Complete environment switch strategy
├── proto/
│   ├── deployment.proto          # gRPC service definitions
│   ├── deployment.pb.go          # Generated protobuf messages
│   └── deployment_grpc.pb.go     # Generated gRPC service code
├── go.mod                        # Go module dependencies
├── Makefile                      # Build automation
├── docker-compose.yml            # LocalStack for local testing
└── README.md                     # This file
```

## Getting Started

### Prerequisites

- Go 1.21 or later
- protoc (protocol buffer compiler) for proto compilation
- Docker and Docker Compose (for LocalStack testing)

### Installation and Setup

Clone the repository and navigate to the project directory:

```bash
cd ecs-plugin-dev
```

Install dependencies:

```bash
go mod tidy
```

Build the project:

```bash
make build
```

This creates two executables in the `bin/` directory:

- `grpc-server`: The deployment orchestration server
- `grpc-client`: The CLI client for testing and deployment operations

### Running the Server

Start the gRPC server in mock mode (no AWS credentials needed):

```bash
MOCK_MODE=true ./bin/grpc-server
```

The server will listen on port 50051 by default. To use a different port:

```bash
MOCK_MODE=true GRPC_PORT=9999 ./bin/grpc-server
```

The server will log its startup status:

```
2026/02/08 13:58:25 [MOCK] ECS client in mock mode
2026/02/08 13:58:25 [MOCK] ELB client in mock mode
2026/02/08 13:58:25 gRPC server listening on port 50051
```

### Testing with the CLI Client

In a separate terminal, use the client to interact with the server:

```bash
./bin/grpc-client -help
```

#### Available Operations

**Deploy with Quicksync strategy (instant):**

```bash
./bin/grpc-client \
  -id deploy1 \
  -cluster arn:aws:ecs:us-east-1:123456789:cluster/prod \
  -service web-service \
  -taskdef task-v1 \
  -strategy quicksync \
  -action deploy
```

**Deploy with Canary strategy (gradual rollout):**

```bash
./bin/grpc-client \
  -id deploy2 \
  -cluster arn:aws:ecs:us-east-1:123456789:cluster/prod \
  -service web-service \
  -taskdef task-v2 \
  -strategy canary \
  -config '{"canary_percent":"30"}' \
  -action deploy
```

**Deploy with Blue-Green strategy (complete switch):**

```bash
./bin/grpc-client \
  -id deploy3 \
  -cluster arn:aws:ecs:us-east-1:123456789:cluster/prod \
  -service web-service \
  -taskdef task-v3 \
  -strategy bluegreen \
  -action deploy
```

**Check deployment status:**

```bash
./bin/grpc-client -id deploy1 -action status
```

Output example:

```
Status: SUCCESS
Progress: 100%
Message: deployment completed
```

**Rollback a deployment:**

```bash
./bin/grpc-client -action rollback
```

**List available deployment strategies:**

```bash
./bin/grpc-client -action list-strategies
```

Output:

```
Available deployment strategies:
  - quicksync   : Instant deployment
  - canary      : Gradual rollout (configurable %)
  - bluegreen   : Complete traffic switch
```

## Deployment Strategies Explained

### Quicksync

Instantly deploys a new task definition to the ECS service. All traffic switches to the new version immediately. Best for non-critical services or when you have confidence in the deployment.

**Execution flow:**

1. Register new task definition
2. Update service to use new task definition
3. Deployment complete

### Canary

Gradually shifts traffic to a new version while monitoring the canary deployment. Typically starts with a small percentage of traffic and increases over time if the deployment appears healthy.

**Execution flow:**

1. Register new task definition
2. Create a canary task set with specified weight (default 20%)
3. Monitor for 2 minutes
4. Shift remaining traffic to new version
5. Clean up old task set

**Configuration options:**

- `canary_percent`: Initial percentage of traffic for canary (0-100, default 20)

### Blue-Green

Maintains two identical complete environments. Traffic switches completely from the blue environment to the green environment, allowing instant rollback if needed.

**Execution flow:**

1. Register new task definition
2. Create green environment with 100% weight
3. Wait 30 seconds for stabilization
4. Shift all traffic to green
5. Wait 1 minute for verification
6. Remove old blue environment

## AWS Integration

The system supports both real AWS environments and mock mode for testing.

### Real AWS Environment

Set the required AWS environment variables:

```bash
export AWS_ACCESS_KEY_ID=your_access_key
export AWS_SECRET_ACCESS_KEY=your_secret_key
export AWS_REGION=us-east-1
export AWS_ENDPOINT_URL=https://ecs.us-east-1.amazonaws.com  # Optional, for custom endpoints

./bin/grpc-server
```

The ECS client will:

- Register new task definitions
- Update ECS service configurations
- Create and manage task sets for canary deployments
- Handle traffic shifting through load balancers

The ELB client will:

- Manage target group weights for traffic distribution
- Support both instant and gradual traffic switching

### Mock Mode

Mock mode simulates AWS operations without requiring credentials or infrastructure. All operations return success with mock logs showing what would have been executed.

```bash
MOCK_MODE=true ./bin/grpc-server
```

Example mock output:

```
[MOCK] RegisterTaskDefinition: task-v1
[MOCK] UpdateService: cluster=arn:aws:ecs:us-east-1:123456789:cluster/prod, service=web-service
[MOCK] CreateTaskSet: cluster=arn:aws:ecs:us-east-1:123456789:cluster/prod, service=web-service, weight=30%
[MOCK] UpdateTargetGroupWeights: canary=30%, primary=70%
```

## Validation and Error Handling

The system performs validation at multiple levels to ensure deployment safety:

### Request Validation

All deployment requests are validated for required fields:

- `deployment_id`: Unique identifier for tracking the deployment
- `cluster_arn`: AWS resource ARN for the ECS cluster
- `service_name`: Name of the ECS service to update
- `task_definition`: Task definition name or ARN
- `strategy`: One of: quicksync, canary, bluegreen

**Invalid request example:**

```bash
./bin/grpc-client \
  -cluster arn:aws:ecs:us-east-1:123456789:cluster/prod \
  -service web-service \
  -taskdef task-v1 \
  -strategy quicksync \
  -action deploy
```

Response:

```
Success: false
Message: invalid request: deployment_id is required
```

### Strategy Validation

Only registered strategies are accepted. Requesting an unregistered strategy returns an error:

```bash
./bin/grpc-client \
  -id deploy1 \
  -cluster arn:aws:ecs:us-east-1:123456789:cluster/prod \
  -service web-service \
  -taskdef task-v1 \
  -strategy invalid_strategy \
  -action deploy
```

Response:

```
Success: false
Message: deployment failed: unknown strategy: invalid_strategy
```

### Configuration Validation

Canary deployments validate the canary percentage:

- Must be between 0 and 100
- Primary and canary weights must sum to 100

Traffic weight validation ensures load balancer configurations are correct.

## System Architecture

### Plugin Registry

The plugin registry enables extensible strategy management with thread-safe operations:

```go
type Registry struct {
    strategies map[string]strategy.Strategy
    mu         sync.RWMutex  // Ensures thread safety
}
```

Benefits:

- Add new strategies without modifying core code
- Safe concurrent access from multiple goroutines
- Easy strategy discovery and listing

### Router Architecture

The router acts as a dispatcher, routing each deployment request to the appropriate strategy:

1. Receives deployment request
2. Validates all required fields
3. Looks up strategy in registry
4. Executes strategy-specific deployment logic
5. Stores and returns deployment status

### Executor Pattern

The executor abstraction separates AWS operations from deployment strategies:

- **Strategies** define deployment patterns
- **Executor** handles AWS infrastructure operations
- **AWS Clients** manage EC2, ECS, and ELB API calls

This separation allows:

- Easy testing with mock AWS clients
- Strategy reuse across different infrastructure
- Clear responsibility boundaries

## Testing

### Test Coverage

The system includes comprehensive tests for all deployment scenarios:

1. **Valid Deployments**
   - Quicksync deployment: Succeeds immediately
   - Canary deployment: Creates gradual rollout
   - Blue-Green deployment: Creates isolated environments

2. **Validation Tests**
   - Missing deployment ID: Returns validation error
   - Missing cluster ARN: Returns validation error
   - Invalid strategy: Returns unknown strategy error

3. **Status Monitoring**
   - Check deployment status: Returns current progress
   - Retrieve completed status: Returns success state

4. **Rollback Operations**
   - Initiate rollback: Reverts to previous task definition
   - Verify rollback success: Confirms previous version active

### Running Tests

Build and start the server:

```bash
make build
MOCK_MODE=true ./bin/grpc-server &
```

Run all test scenarios:

```bash
# Test valid quicksync deployment
./bin/grpc-client \
  -id deploy1 \
  -cluster arn:aws:ecs:us-east-1:123456789:cluster/prod \
  -service web-service \
  -taskdef task-v1 \
  -strategy quicksync \
  -action deploy

# Test validation error
./bin/grpc-client \
  -id missing-cluster \
  -service web-service \
  -taskdef task-v1 \
  -strategy quicksync \
  -action deploy

# Test invalid strategy
./bin/grpc-client \
  -id invalid-strat \
  -cluster arn:aws:ecs:us-east-1:123456789:cluster/prod \
  -service web-service \
  -taskdef task-v1 \
  -strategy invalid_strategy \
  -action deploy
```

All test scenarios pass with the mock mode enabled.

## Configuration

### Environment Variables

- `MOCK_MODE`: Set to `true` for testing without AWS credentials
- `GRPC_PORT`: Port for gRPC server (default: 50051)
- `AWS_REGION`: AWS region (default: us-east-1)
- `AWS_ACCESS_KEY_ID`: AWS access key
- `AWS_SECRET_ACCESS_KEY`: AWS secret key
- `AWS_SESSION_TOKEN`: AWS session token (optional)
- `AWS_ENDPOINT_URL`: Custom AWS endpoint (for LocalStack, etc.)

### Proto Configuration

The gRPC service is defined in `proto/deployment.proto`. To update the service definition:

1. Edit `proto/deployment.proto`
2. Run `make proto` to regenerate Go code
3. Rebuild the project with `make build`

## Benefits and Use Cases

### Zero-Downtime Deployments

The plugin eliminates deployment-induced service interruptions:

- **Quicksync**: Suitable for services with health checks that automatically recover
- **Canary**: Detects issues in new versions before full rollout
- **Blue-Green**: Allows instant rollback if problems detected

### Risk Mitigation

Different strategies support different risk profiles:

- **High-risk changes**: Use Blue-Green for instant rollback capability
- **Moderate-risk changes**: Use Canary for gradual validation
- **Low-risk changes**: Use Quicksync for fast deployments

### Operational Efficiency

Automates repetitive deployment tasks:

- Consistent deployment procedures across teams
- Reduced manual intervention and human error
- Clear audit trail of all deployments via status tracking
- Centralized strategy management

### Infrastructure Abstraction

Mock mode enables development and testing without AWS infrastructure:

- CI/CD pipelines can run without live AWS accounts
- Developers can test deployment logic locally
- Faster iteration cycles
- Cost reduction from avoided test infrastructure

## Production Deployment

### Prerequisites for Production

Before running in production:

1. **Ensure AWS credentials are properly configured**

   ```bash
   unset MOCK_MODE  # Disable mock mode
   export AWS_REGION=your-region
   # AWS credentials via environment or ~/.aws/credentials
   ```

2. **Use appropriate timeouts and retry logic**
   - The current timeout is 10 seconds for gRPC operations
   - For long-running deployments, increase context timeouts

3. **Set up monitoring and alerting**
   - Monitor server logs for errors
   - Track deployment statuses
   - Alert on failed deployments

4. **Configure graceful shutdown**
   - The server handles SIGINT and SIGTERM signals
   - Ongoing deployments complete before shutdown

### Deploying the Server

On a production server:

```bash
# Build the release
make build

# Run with systemd or similar process manager
# This ensures the server restarts on failure
systemctl start ecs-deployment-server
```

For Kubernetes deployments:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: ecs-deployment-server
spec:
  containers:
  - name: server
    image: my-registry/ecs-plugin:latest
    ports:
    - containerPort: 50051
    env:
    - name: GRPC_PORT
      value: "50051"
    # AWS credentials via secrets
    envFrom:
    - secretRef:
        name: aws-credentials
```