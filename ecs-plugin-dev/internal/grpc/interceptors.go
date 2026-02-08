package grpc

import (
	"context"
	"log"
	"time"

	"ecs-plugin-dev/internal/metrics"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// LoggingInterceptor logs all gRPC calls
func LoggingInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()

		log.Printf("[gRPC] %s started", info.FullMethod)

		resp, err := handler(ctx, req)

		duration := time.Since(start)
		if err != nil {
			log.Printf("[gRPC] %s failed: %v (duration: %v)", info.FullMethod, err, duration)
		} else {
			log.Printf("[gRPC] %s completed (duration: %v)", info.FullMethod, duration)
		}

		return resp, err
	}
}

// MetricsInterceptor collects metrics for all gRPC calls
func MetricsInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()

		resp, err := handler(ctx, req)

		duration := time.Since(start)
		status := "success"
		if err != nil {
			status = "error"
		}

		// Record gRPC call metrics
		metrics.AWSAPICallsTotal.WithLabelValues("grpc", info.FullMethod, status).Inc()
		metrics.AWSAPICallDuration.WithLabelValues("grpc", info.FullMethod).Observe(duration.Seconds())

		return resp, err
	}
}

// RecoveryInterceptor recovers from panics
func RecoveryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[PANIC] %s: %v", info.FullMethod, r)
				metrics.RecordError("grpc_server", "panic")
				err = status.Errorf(codes.Internal, "internal server error")
			}
		}()

		return handler(ctx, req)
	}
}
