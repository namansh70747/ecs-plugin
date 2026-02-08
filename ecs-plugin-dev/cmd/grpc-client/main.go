// cmd/grpc-client/main.go
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"time"

	pb "ecs-plugin-dev/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	var (
		server     = flag.String("server", "localhost:50051", "gRPC server address")
		action     = flag.String("action", "deploy", "Action: deploy, status, rollback")
		deployID   = flag.String("id", "", "Deployment ID")
		cluster    = flag.String("cluster", "", "ECS Cluster ARN")
		service    = flag.String("service", "", "ECS Service Name")
		taskDef    = flag.String("taskdef", "", "Task Definition JSON file")
		strategy   = flag.String("strategy", "quicksync", "Deployment strategy")
		configJSON = flag.String("config", "{}", "Config JSON")
	)
	flag.Parse()

	conn, err := grpc.Dial(*server, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("connection failed: %v", err)
	}
	defer conn.Close()

	client := pb.NewDeploymentServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	switch *action {
	case "deploy":
		var config map[string]string
		json.Unmarshal([]byte(*configJSON), &config)

		resp, err := client.Deploy(ctx, &pb.DeployRequest{
			DeploymentId:   *deployID,
			ClusterArn:     *cluster,
			ServiceName:    *service,
			TaskDefinition: *taskDef,
			Strategy:       *strategy,
			Config:         config,
		})
		if err != nil {
			log.Fatalf("deploy failed: %v", err)
		}
		fmt.Printf("Success: %v\nMessage: %s\nDeployment ID: %s\n",
			resp.Success, resp.Message, resp.DeploymentId)

	case "status":
		resp, err := client.GetStatus(ctx, &pb.StatusRequest{
			DeploymentId: *deployID,
		})
		if err != nil {
			log.Fatalf("status check failed: %v", err)
		}
		fmt.Printf("Status: %s\nProgress: %d%%\nMessage: %s\n",
			resp.Status, resp.Progress, resp.Message)

	case "rollback":
		resp, err := client.Rollback(ctx, &pb.RollbackRequest{
			DeploymentId: *deployID,
			ClusterArn:   *cluster,
			ServiceName:  *service,
		})
		if err != nil {
			log.Fatalf("rollback failed: %v", err)
		}
		fmt.Printf("Success: %v\nMessage: %s\n", resp.Success, resp.Message)

	case "list-strategies":
		fmt.Println("Available deployment strategies:")
		fmt.Println("  - quicksync   : Instant deployment")
		fmt.Println("  - canary      : Gradual rollout (configurable %)")
		fmt.Println("  - bluegreen   : Complete traffic switch")

	default:
		log.Fatalf("unknown action: %s (available: deploy, status, rollback, list-strategies)", *action)
	}
}
