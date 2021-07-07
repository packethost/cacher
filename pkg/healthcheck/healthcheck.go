// Package healthcheck provides service to get grpc server health status.
package healthcheck

import (
	"context"

	"google.golang.org/grpc/health/grpc_health_v1"
)

// HealthChecker for grpc server.
type HealthChecker struct{}

// Check status and returns grpc healthcheckresponse
func (s *HealthChecker) Check(ctx context.Context, req *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {
	return &grpc_health_v1.HealthCheckResponse{
		Status: grpc_health_v1.HealthCheckResponse_SERVING,
	}, nil
}

// Watch streams the server status change.
func (s *HealthChecker) Watch(req *grpc_health_v1.HealthCheckRequest, server grpc_health_v1.Health_WatchServer) error {
	return server.Send(&grpc_health_v1.HealthCheckResponse{
		Status: grpc_health_v1.HealthCheckResponse_SERVING,
	})
}

// GrpcHealthChecker requests to check the grpc server health.
func GrpcHealthChecker() *HealthChecker {
	return &HealthChecker{}
}
