// cmd/grpc-server/main.go
package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ecs-plugin-dev/internal/config"
	server "ecs-plugin-dev/internal/grpc"
	pb "ecs-plugin-dev/proto"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/reflection"
)

func main() {
	// Load configuration
	cfg, err := config.LoadConfig(os.Getenv("CONFIG_FILE"))
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// Start metrics server if enabled
	var metricsServer *http.Server
	if cfg.Server.EnableMetrics {
		metricsServer = startMetricsServer(cfg.Server.MetricsPort)
	}

	// Start gRPC server
	port := fmt.Sprintf("%d", cfg.Server.Port)
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	// Configure gRPC server options
	serverOpts := []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(
			server.LoggingInterceptor(),
			server.MetricsInterceptor(),
			server.RecoveryInterceptor(),
		),
	}

	// Add TLS if certificates are provided
	certFile := os.Getenv("TLS_CERT_FILE")
	keyFile := os.Getenv("TLS_KEY_FILE")
	if certFile != "" && keyFile != "" {
		log.Printf("Loading TLS certificates: cert=%s, key=%s", certFile, keyFile)
		tlsCreds, err := credentials.NewServerTLSFromFile(certFile, keyFile)
		if err != nil {
			log.Fatalf("failed to load TLS credentials: %v", err)
		}
		serverOpts = append(serverOpts, grpc.Creds(tlsCreds))
		log.Println("TLS enabled for gRPC server")
	} else {
		log.Println("Running without TLS (insecure mode)")
	}

	grpcServer := grpc.NewServer(serverOpts...)

	deploymentServer := server.NewDeploymentServer()
	pb.RegisterDeploymentServiceServer(grpcServer, deploymentServer)
	reflection.Register(grpcServer)

	// Graceful shutdown with timeout
	shutdownCh := make(chan struct{})
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
		sig := <-sigCh
		log.Printf("Received signal: %v, initiating graceful shutdown", sig)

		// Shutdown metrics server
		if metricsServer != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := metricsServer.Shutdown(ctx); err != nil {
				log.Printf("Metrics server shutdown error: %v", err)
			}
		}

		// Graceful stop with timeout
		stopped := make(chan struct{})
		go func() {
			grpcServer.GracefulStop()
			close(stopped)
		}()

		// Force stop after timeout
		select {
		case <-stopped:
			log.Println("Server stopped gracefully")
		case <-time.After(cfg.Server.GracefulTimeout):
			log.Println("Graceful shutdown timeout, forcing stop")
			grpcServer.Stop()
		}

		close(shutdownCh)
	}()

	log.Printf("gRPC server listening on port %s", port)
	if cfg.Server.EnableMetrics {
		log.Printf("Metrics available at http://localhost:%d/metrics", cfg.Server.MetricsPort)
	}

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}

	<-shutdownCh
	log.Println("Server shutdown complete")
}

func startMetricsServer(port int) *http.Server {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Printf("Metrics server listening on %s", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("metrics server error: %v", err)
		}
	}()

	return server
}
