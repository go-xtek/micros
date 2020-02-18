package grpc

import (
	"context"
	"fmt"
	microtls "github.com/gidyon/micros/utils/tls"
	"github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"os"
)

// ClientParams contains parameters for NewClientConn
type ClientParams struct {
	Port               int
	DialOptions        []grpc.DialOption
	UnaryInterceptors  []grpc.UnaryClientInterceptor
	StreamInterceptors []grpc.StreamClientInterceptor
}

// NewClientConn dials to a grpc server
func NewClientConn(opt *ClientParams) (*grpc.ClientConn, error) {
	// Parse client TLS config
	clientTLSConfig, err := microtls.ClientConfig()
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse client tls config")
	}

	dopts := []grpc.DialOption{
		grpc.WithTransportCredentials(credentials.NewTLS(clientTLSConfig)),
	}

	for _, dialOption := range opt.DialOptions {
		dopts = append(dopts, dialOption)
	}

	// Enables wait for ready RPCs
	waitForReadyUnaryInterceptor := func(
		ctx context.Context,
		method string,
		req, reply interface{},
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		return invoker(ctx, method, req, reply, cc, grpc.WaitForReady(true))
	}

	unaryClientInterceptors := []grpc.UnaryClientInterceptor{waitForReadyUnaryInterceptor}
	for _, unaryInterceptor := range opt.UnaryInterceptors {
		unaryClientInterceptors = append(unaryClientInterceptors, unaryInterceptor)
	}

	streamClientInterceptors := make([]grpc.StreamClientInterceptor, 0)
	for _, streamInterceptor := range opt.StreamInterceptors {
		streamClientInterceptors = append(streamClientInterceptors, streamInterceptor)
	}

	// Append to dial option
	dopts = append(
		dopts,
		grpc.WithUnaryInterceptor(
			grpc_middleware.ChainUnaryClient(unaryClientInterceptors...),
		),
		grpc.WithStreamInterceptor(
			grpc_middleware.ChainStreamClient(streamClientInterceptors...),
		),
	)

	// Enable Retries
	os.Setenv("GRPC_GO_RETRY", "on")
	address := fmt.Sprintf("localhost:%d", opt.Port)

	clientConn, err := grpc.Dial(address, dopts...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to dial to gRPC server")
	}

	return clientConn, nil
}
