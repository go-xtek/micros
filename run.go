package micros

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/gidyon/logger"
	service_grpc "github.com/gidyon/micros/pkg/grpc"
	http_middleware "github.com/gidyon/micros/pkg/http"
	micro_tls "github.com/gidyon/micros/utils/tls"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"google.golang.org/grpc/reflection"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"google.golang.org/grpc"
)

// Run multiplexes GRPC and HTTP server on the same port
func (service *Service) Run(ctx context.Context, insecure bool) error {
	if service.baseEndpoint == "" {
		service.baseEndpoint = "/"
	}
	// Registration of service endpoint
	service.httpMux.Handle(service.baseEndpoint, service.runtimeMux)

	// Apply middlewares
	handler := http_middleware.Apply(service.Handler(), service.httpMiddlewares...)

	// the grpcHandlerFunc takes an grpc server and a http muxer and will
	// route the request to the right place at runtime.
	// handler := grpcHandlerFunc(service.GRPCServer(), service.HTTPMux())
	ghandler := grpcHandlerFunc(service.GRPCServer(), handler)

	// HTTP server configuration
	httpServer := &http.Server{
		Addr:              fmt.Sprintf(":%d", service.cfg.ServicePort()),
		Handler:           ghandler,
		ReadTimeout:       time.Duration(5 * time.Second),
		ReadHeaderTimeout: time.Duration(5 * time.Second),
		WriteTimeout:      time.Duration(5 * time.Second),
	}

	// Graceful shutdown of server
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for range c {
			if service.cfg.Logging() {
				logger.Log.Warn(
					"shutting service...", zap.String("service name", service.cfg.ServiceName()),
				)
			}
			httpServer.Shutdown(ctx)

			<-ctx.Done()
		}
	}()

	// Create TCP listener
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", service.cfg.ServicePort()))
	if err != nil {
		return errors.Wrap(err, "failed to create TCP listener")
	}

	logMsgFn := func() {
		if service.cfg.Logging() {
			logger.Log.Info(
				"<gRPC and REST> server for service running",
				zap.String("service name", service.cfg.ServiceName()),
				zap.Int("gRPC Port", service.cfg.ServicePort()),
			)
		} else {
			logrus.Infof(
				"<gRPC and REST> server for service running service: %s port: %d",
				service.cfg.ServiceName(), service.cfg.ServicePort(),
			)
		}
	}

	logMsgFn()

	if insecure {
		return httpServer.Serve(lis)
	}

	// Parse HTTP server TLS config
	serverTLSsConfig, err := micro_tls.HTTPServerConfig()
	if err != nil {
		return errors.Wrap(err, "failed to create TLS config for HTTP server")
	}

	return httpServer.Serve(tls.NewListener(lis, serverTLSsConfig))
}

// grpcHandlerFunc returns an http.Handler that delegates to grpcServer on incoming gRPC
// connections or otherHandler otherwise. Copied from cockroachdb.
func grpcHandlerFunc(grpcServer *grpc.Server, otherHandler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO(tamird): point to merged gRPC code rather than a PR.
		// This is a partial recreation of gRPC's internal checks https://github.com/grpc/grpc-go/pull/514/files#diff-95e9a25b738459a2d3030e1e6fa2a718R61
		if r.ProtoMajor == 2 && strings.Contains(r.Header.Get("Content-Type"), "application/grpc") {
			grpcServer.ServeHTTP(w, r)
		} else {
			otherHandler.ServeHTTP(w, r)
		}
	})
}

// InitGRPC initialize gRPC server and client with registered client and server interceptors and options.
// The method must be called before registering anything on the gRPC server or gRPC client connection.
// When this method has been called, subsequent calls to add interceptors and/or options will not update the service
func (service *Service) InitGRPC(ctx context.Context) error {
	// client connection for the reverse gateway
	clientConn, err := service_grpc.NewClientConn(
		service.cfg,
		service.dialOptions,
		service.gRPCUnaryClientInterceptors,
		service.grpcStreamClientInterceptors,
	)
	if err != nil {
		return err
	}

	service.clientConn = clientConn

	// create gRPC server for service
	grpcSrv, err := service_grpc.NewServer(
		service.cfg,
		service.serverOptions,
		service.gRPCUnaryInterceptors,
		service.gRPCStreamInterceptors,
	)
	if err != nil {
		return err
	}

	// register reflection on the gRPC server
	reflection.Register(grpcSrv)

	service.gRPCServer = grpcSrv

	return nil
}
