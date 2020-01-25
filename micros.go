package micros

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/RediSearch/redisearch-go/redisearch"
	"github.com/gidyon/config"
	"github.com/gidyon/logger"
	"github.com/gidyon/micros/pkg/conn"
	microtls "github.com/gidyon/micros/utils/tls"
	"github.com/go-redis/redis"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"
	"strings"

	"net/http"

	"google.golang.org/grpc"

	"github.com/grpc-ecosystem/grpc-gateway/runtime"
)

// Service contains API clients, connections and options for bootstrapping a micro-service
type Service struct {
	cfg                          *config.Config
	db                           *gorm.DB // uses gorm
	sqlDB                        *sql.DB  // uses database/sql driver
	redisClient                  *redis.Client
	rediSearchClient             *redisearch.Client
	baseEndpoint                 string
	httpMiddlewares              []httpMiddleware
	httpMux                      *http.ServeMux
	runtimeMux                   *runtime.ServeMux
	clientConn                   *grpc.ClientConn
	gRPCServer                   *grpc.Server
	externalServicesConn         map[string]*grpc.ClientConn
	gRPCUnaryInterceptors        []grpc.UnaryServerInterceptor
	gRPCStreamInterceptors       []grpc.StreamServerInterceptor
	serverOption                 []grpc.ServerOption
	gRPCUnaryClientInterceptors  []grpc.UnaryClientInterceptor
	grpcStreamClientInterceptors []grpc.StreamClientInterceptor
	dialOptions                  []grpc.DialOption
	enableCORS                   bool
}

// NewService create a new micro-service based on the options passed in config
func NewService(ctx context.Context, cfg *config.Config) (*Service, error) {

	// Initialize paths to cert and key
	microtls.SetKeyAndCertPaths(cfg.ServiceTLSKeyFile(), cfg.ServiceTLSCertFile())

	if cfg.Logging() {
		// Initialize logger
		err := logger.Init(cfg.LogLevel(), cfg.LogTimeFormat())
		if err != nil {
			return nil, errors.Wrap(err, "failed to initialize logger")
		}
	}

	var (
		err              error
		db               *gorm.DB
		sqlDB            *sql.DB
		redisClient      *redis.Client
		rediSearchClient *redisearch.Client
		externalServices = make(map[string]*grpc.ClientConn)
	)

	if cfg.UseSQLDatabase() {
		sqlDBInfo := cfg.SQLDatabase()
		if sqlDBInfo.UseGorm() {
			// Create a *sql.DB instance
			db, err = conn.ToSQLDBUsingORM(&conn.DBOptions{
				Dialect:  sqlDBInfo.SQLDatabaseDialect(),
				Host:     sqlDBInfo.Host(),
				Port:     fmt.Sprintf("%d", sqlDBInfo.Port()),
				User:     sqlDBInfo.User(),
				Password: sqlDBInfo.Password(),
				Schema:   sqlDBInfo.Schema(),
			})
			if err != nil {
				return nil, err
			}
			sqlDB = nil
		} else {
			// Create a *sql.DB instance
			sqlDB, err = conn.ToSQLDB(&conn.DBOptions{
				Dialect:  sqlDBInfo.SQLDatabaseDialect(),
				Host:     sqlDBInfo.Host(),
				Port:     fmt.Sprintf("%d", sqlDBInfo.Port()),
				User:     sqlDBInfo.User(),
				Password: sqlDBInfo.Password(),
				Schema:   sqlDBInfo.Schema(),
			})
			if err != nil {
				return nil, err
			}
			db = nil
		}
	}

	if cfg.UseRedis() {
		redisDBInfo := cfg.RedisDatabase()

		// Creates a redis client
		redisClient = conn.NewRedisClient(&conn.RedisOptions{
			Address: redisDBInfo.Host(),
			Port:    fmt.Sprintf("%d", redisDBInfo.Port()),
		})

		if cfg.UseRediSearch() {
			// Create a redisearch client
			rediSearchClient = redisearch.NewClient(
				redisDBInfo.Address(), cfg.ServiceName()+":index",
			)
		}
	}

	// Remote services
	for _, srv := range cfg.ExternalServices() {
		if !srv.Available() {
			continue
		}
		externalServices[strings.ToLower(srv.Name())], err = conn.DialService(ctx, &conn.GRPCDialOptions{
			ServiceName: srv.Name(),
			Address:     srv.Address(),
			TLSCertFile: srv.TLSCertFile(),
			ServerName:  srv.ServerName(),
			WithBlock:   false,
			K8Service:   srv.K8Service(),
		})
		if err != nil {
			return nil, errors.Wrapf(err, "failed to create connection to service %s", srv.Name())
		}
	}

	// Update the rest client
	runtimeMux := newRuntimeMux()

	return &Service{
		cfg:                          cfg,
		db:                           db,
		sqlDB:                        sqlDB,
		redisClient:                  redisClient,
		rediSearchClient:             rediSearchClient,
		httpMiddlewares:              make([]httpMiddleware, 0),
		runtimeMux:                   runtimeMux,
		externalServicesConn:         externalServices,
		gRPCUnaryInterceptors:        make([]grpc.UnaryServerInterceptor, 0),
		gRPCStreamInterceptors:       make([]grpc.StreamServerInterceptor, 0),
		serverOption:                 make([]grpc.ServerOption, 0),
		gRPCUnaryClientInterceptors:  make([]grpc.UnaryClientInterceptor, 0),
		grpcStreamClientInterceptors: make([]grpc.StreamClientInterceptor, 0),
		dialOptions:                  make([]grpc.DialOption, 0),
	}, nil
}

// HTTPHandler returns the handler for the service
func (service *Service) HTTPHandler() http.Handler {
	return service.httpMux
}

// HTTPMuxer returns the internal ServeMux of the service
func (service *Service) HTTPMuxer() *http.ServeMux {
	return service.httpMux
}

// EnableCORS enables coss origin requests to service
func (service *Service) EnableCORS() {
	service.enableCORS = true
}

// DisableCORS disables cross origin requests on service
func (service *Service) DisableCORS() {
	service.enableCORS = false
}

// ServiceEndpoint ...
func (service *Service) ServiceEndpoint(path string) {
	service.baseEndpoint = path
}

// AddEndpoint ...
func (service *Service) AddEndpoint(path string, handler http.Handler) {
	if service.httpMux == nil {
		service.httpMux = http.NewServeMux()
	}
	service.httpMux.Handle(path, handler)
}

// AddEndpointFunc ...
func (service *Service) AddEndpointFunc(path string, handleFunc http.HandlerFunc) {
	if service.httpMux == nil {
		service.httpMux = http.NewServeMux()
	}
	service.httpMux.HandleFunc(path, handleFunc)
}

type httpMiddleware func(http.Handler) http.Handler

// AddHTTPMiddlewares adds middlewares to the service http handler
func (service *Service) AddHTTPMiddlewares(middlewares ...httpMiddleware) {
	service.httpMiddlewares = append(service.httpMiddlewares, middlewares...)
}

// AddGRPCDialOptions adds dial options to gRPC reverse proxy client
func (service *Service) AddGRPCDialOptions(dialOptions ...grpc.DialOption) {
	for _, dialOption := range dialOptions {
		service.dialOptions = append(service.dialOptions, dialOption)
	}
}

// AddGRPCServerOptions adds server options to gRPC server
func (service *Service) AddGRPCServerOptions(serverOptions ...grpc.ServerOption) {
	for _, serverOption := range serverOptions {
		service.serverOption = append(service.serverOption, serverOption)
	}
}

// AddGRPCStreamServerInterceptors adds stream interceptors to the gRPC server
func (service *Service) AddGRPCStreamServerInterceptors(
	streamInterceptors ...grpc.StreamServerInterceptor,
) {
	for _, streamInterceptor := range streamInterceptors {
		service.gRPCStreamInterceptors = append(
			service.gRPCStreamInterceptors, streamInterceptor,
		)
	}
}

// AddGRPCUnaryServerInterceptors adds unary interceptors to the gRPC server
func (service *Service) AddGRPCUnaryServerInterceptors(
	unaryInterceptors ...grpc.UnaryServerInterceptor,
) {
	for _, unaryInterceptor := range unaryInterceptors {
		service.gRPCUnaryInterceptors = append(
			service.gRPCUnaryInterceptors, unaryInterceptor,
		)
	}
}

// AddGRPCStreamClientInterceptors adds stream interceptors to the gRPC reverse proxy client
func (service *Service) AddGRPCStreamClientInterceptors(
	streamInterceptors ...grpc.StreamClientInterceptor,
) {
	for _, streamInterceptor := range streamInterceptors {
		service.grpcStreamClientInterceptors = append(
			service.grpcStreamClientInterceptors, streamInterceptor,
		)
	}
}

// AddGRPCUnaryClientInterceptors adds unary interceptors to the gRPC reverse proxy client
func (service *Service) AddGRPCUnaryClientInterceptors(
	unaryInterceptors ...grpc.UnaryClientInterceptor,
) {
	for _, unaryInterceptor := range unaryInterceptors {
		service.gRPCUnaryClientInterceptors = append(
			service.gRPCUnaryClientInterceptors, unaryInterceptor,
		)
	}
}

// Config returns the config for the service
func (service *Service) Config() *config.Config {
	return service.cfg
}

// RuntimeMux returns the runtime muxer for the service
func (service *Service) RuntimeMux() *runtime.ServeMux {
	return service.runtimeMux
}

// ClientConn returns the underlying client connection to grpc server used by reverse proxy
func (service *Service) ClientConn() *grpc.ClientConn {
	return service.clientConn
}

// GRPCServer returns the grpc server
func (service *Service) GRPCServer() *grpc.Server {
	return service.gRPCServer
}

// DB returns a gorm db instance
func (service *Service) DB() *gorm.DB {
	return service.db
}

// SQLDB returns database/sql db instance
func (service *Service) SQLDB() *sql.DB {
	return service.sqlDB
}

// RedisClient returns a redis client
func (service *Service) RedisClient() *redis.Client {
	return service.redisClient
}

// RediSearchClient returns redisearch client
func (service *Service) RediSearchClient() *redisearch.Client {
	return service.rediSearchClient
}

// ExternalServiceConn returns the underlying grpc connection to the external service
func (service *Service) ExternalServiceConn(serviceName string) (*grpc.ClientConn, error) {
	cc, ok := service.externalServicesConn[strings.ToLower(serviceName)]
	if !ok {
		return nil, errors.Errorf("no service exists with name: %s", serviceName)
	}
	return cc, nil
}

// creates a http Muxer using runtime.NewServeMux
func newRuntimeMux() *runtime.ServeMux {
	return runtime.NewServeMux(
		runtime.WithMarshalerOption(
			runtime.MIMEWildcard,
			&runtime.JSONPb{
				OrigName:     true,
				EmitDefaults: true,
			},
		),
	)
}
