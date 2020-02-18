package grpc

import (
	microtls "github.com/gidyon/micros/utils/tls"
	"github.com/grpc-ecosystem/go-grpc-middleware"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// ServerParams contains parameters when calling NewServer
type ServerParams struct {
	ServerOptions      []grpc.ServerOption
	UnaryInterceptors  []grpc.UnaryServerInterceptor
	StreamInterceptors []grpc.StreamServerInterceptor
}

// NewServer creates a new grpc server
func NewServer(opt *ServerParams) (*grpc.Server, error) {
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

	opts = append(opts, opt.ServerOptions...)

	// Append other server options
	opts = append(
		opts,
		grpc_middleware.WithUnaryServerChain(
			grpc_middleware.ChainUnaryServer(opt.UnaryInterceptors...),
		),
		grpc_middleware.WithStreamServerChain(
			grpc_middleware.ChainStreamServer(opt.StreamInterceptors...),
		),
	)

	return grpc.NewServer(opts...), nil
}
