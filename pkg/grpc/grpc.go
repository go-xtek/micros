package grpc

import (
	"context"
	"github.com/gidyon/config"
	"github.com/gidyon/logger"
	"github.com/gidyon/micros/pkg/grpc/middleware"
	microtls "github.com/gidyon/micros/utils/tls"
	"github.com/grpc-ecosystem/go-grpc-middleware"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// NewGRPCServer creates a new grpc server
func NewGRPCServer(
	ctx context.Context,
	cfg *config.Config,
	unaryInterceptors []grpc.UnaryServerInterceptor,
	streamInterceptors []grpc.StreamServerInterceptor,
) (*grpc.Server, error) {
	// Create TLS object for gRPC server
	tlsConfig, err := microtls.GRPCServerConfig()
	if err != nil {
		return nil, err
	}

	creds := credentials.NewTLS(tlsConfig)

	// Create serveroption
	opts := []grpc.ServerOption{
		grpc.Creds(creds),
	}

	var unaryLoggerInterceptors []grpc.UnaryServerInterceptor
	var streamLoggerInterceptors []grpc.StreamServerInterceptor
	if cfg.Logging() {
		// add logging middleware
		unaryLoggerInterceptors, streamLoggerInterceptors = middleware.AddLogging(logger.Log)
	}

	// add recovery from panic middleware
	unaryRecoveryInterceptors, streamRecoveryInterceptors := middleware.AddRecovery()

	// Append other server options
	opts = append(
		opts,
		grpc_middleware.WithUnaryServerChain(
			chainUnaryInterceptors(
				unaryLoggerInterceptors,
				unaryInterceptors,
				unaryRecoveryInterceptors,
			)...,
		),
		grpc_middleware.WithStreamServerChain(
			chainStreamInterceptors(
				streamLoggerInterceptors,
				streamInterceptors,
				streamRecoveryInterceptors,
			)...,
		),
	)

	return grpc.NewServer(opts...), nil
}

type grpcUnaryInterceptorsSlice []grpc.UnaryServerInterceptor

func chainUnaryInterceptors(
	unaryInterceptorsSlice ...grpcUnaryInterceptorsSlice,
) []grpc.UnaryServerInterceptor {
	unaryInterceptors := make([]grpc.UnaryServerInterceptor, 0, len(unaryInterceptorsSlice))

	for _, unaryInterceptorSlice := range unaryInterceptorsSlice {
		for _, unaryInterceptor := range unaryInterceptorSlice {
			unaryInterceptors = append(unaryInterceptors, unaryInterceptor)
		}
	}

	return unaryInterceptors
}

type grpcStreamInterceptorsSlice []grpc.StreamServerInterceptor

func chainStreamInterceptors(
	streamInterceptorsSlice ...grpcStreamInterceptorsSlice,
) []grpc.StreamServerInterceptor {
	streamInterceptors := make([]grpc.StreamServerInterceptor, 0, len(streamInterceptorsSlice))

	for _, streamInterceptorSlice := range streamInterceptorsSlice {
		for _, streamInterceptor := range streamInterceptorSlice {
			streamInterceptors = append(streamInterceptors, streamInterceptor)
		}
	}

	return streamInterceptors
}
