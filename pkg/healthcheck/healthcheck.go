// Package healthcheck provides service to get grpc server health status.
package healthcheck

import (
	"context"

	"google.golang.org/grpc/health/grpc_health_v1"
)

// HealthChecker for grpc server.
type HealthChecker struct{}

// Check status and return a GRPC health response.
func (s *HealthChecker) Check(_ context.Context, _ *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {
	return &grpc_health_v1.HealthCheckResponse{
		Status: grpc_health_v1.HealthCheckResponse_SERVING,
	}, nil
}

// Watch streams the server status change.
func (s *HealthChecker) Watch(_ *grpc_health_v1.HealthCheckRequest, server grpc_health_v1.Health_WatchServer) error {
	return server.Send(&grpc_health_v1.HealthCheckResponse{
		Status: grpc_health_v1.HealthCheckResponse_SERVING,
	})
}

// GRPCHealthChecker requests to check the grpc server health.
func GRPCHealthChecker() *HealthChecker {
	return &HealthChecker{}
}
