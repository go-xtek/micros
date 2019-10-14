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

	"net/http"

	"google.golang.org/grpc"

	"github.com/gidyon/account/pkg/api/admin"
	"github.com/gidyon/account/pkg/api/user"
	"github.com/gidyon/notification/pkg/api"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
)

// Service contains API clients, connections and options for bootstrapping a micro-service
type Service struct {
	cfg                          *config.Config
	db                           *gorm.DB // uses gorm
	sqlDB                        *sql.DB  // uses database/sql driver
	redisClient                  *redis.Client
	rediSearchClient             *redisearch.Client
	notificationClient           notification.NotificationServiceClient
	adminAccountsClient          admin.AdminAPIClient
	userAccountsClient           user.UserAPIClient
	httpMux                      *http.ServeMux
	runtimeMux                   *runtime.ServeMux
	clientConn                   *grpc.ClientConn
	gRPCServer                   *grpc.Server
	gRPCUnaryInterceptors        []grpc.UnaryServerInterceptor
	gRPCStreamInterceptors       []grpc.StreamServerInterceptor
	serverOption                 []grpc.ServerOption
	gRPCUnaryClientInterceptors  []grpc.UnaryClientInterceptor
	grpcStreamClientInterceptors []grpc.StreamClientInterceptor
	dialOptions                  []grpc.DialOption
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
		err                     error
		db                      *gorm.DB
		sqlDB                   *sql.DB
		redisClient             *redis.Client
		rediSearchClient        *redisearch.Client
		accountsServiceConn     *grpc.ClientConn
		notificationServiceConn *grpc.ClientConn
	)

	if cfg.UseSQLDatabase() {
		if cfg.UseGorm() {
			// Create a *sql.DB instance
			db, err = conn.ToSQLDBUsingORM(&conn.DBOptions{
				Dialect:  cfg.SQLDatabaseDialect(),
				Host:     cfg.SQLDatabaseHost(),
				Port:     fmt.Sprintf("%d", cfg.SQLDatabasePort()),
				User:     cfg.SQLDatabaseUser(),
				Password: cfg.SQLDatabasePassword(),
				Schema:   cfg.SQLDatabaseSchema(),
			})
			if err != nil {
				return nil, err
			}
			sqlDB = nil
		} else {
			// Create a *sql.DB instance
			sqlDB, err = conn.ToSQLDB(&conn.DBOptions{
				Dialect:  cfg.SQLDatabaseDialect(),
				Host:     cfg.SQLDatabaseHost(),
				Port:     fmt.Sprintf("%d", cfg.SQLDatabasePort()),
				User:     cfg.SQLDatabaseUser(),
				Password: cfg.SQLDatabasePassword(),
				Schema:   cfg.SQLDatabaseSchema(),
			})
			if err != nil {
				return nil, err
			}
			db = nil
		}

	}

	if cfg.UseRedis() {
		// Creates a redis client
		redisClient = conn.NewRedisClient(&conn.RedisOptions{
			Address: cfg.RedisHost(),
			Port:    fmt.Sprintf("%d", cfg.RedisPort()),
		})
	}

	if cfg.UseRediSearch() {
		// Create a redisearch client
		rediSearchClient = redisearch.NewClient(cfg.RedisURL(), cfg.ServiceName()+":index")
	}

	// Remote services
	if cfg.RequireAuthentication() {
		accountsServiceConn, err = conn.DialAccountService(ctx, &conn.GRPCDialOptions{
			Address:     cfg.AuthenticationAddress(),
			TLSCertFile: cfg.AuthenticationTLSCertFile(),
			ServerName:  cfg.AuthenticationTLSServerName(),
			WithBlock:   false,
		})
		if err != nil {
			return nil, errors.Wrap(err, "failed to create connection to accounts service")
		}
	}

	if cfg.UseNotifications() {
		notificationServiceConn, err = conn.DialNotificationService(ctx, &conn.GRPCDialOptions{
			Address:     cfg.NotificationAddress(),
			TLSCertFile: cfg.NotificationTLSCertFile(),
			ServerName:  cfg.NotificationTLSServerName(),
			WithBlock:   false,
		})
		if err != nil {
			return nil, errors.Wrap(err, "failed to connect to notification service")
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
		adminAccountsClient:          admin.NewAdminAPIClient(accountsServiceConn),
		userAccountsClient:           user.NewUserAPIClient(accountsServiceConn),
		notificationClient:           notification.NewNotificationServiceClient(notificationServiceConn),
		runtimeMux:                   runtimeMux,
		gRPCUnaryInterceptors:        make([]grpc.UnaryServerInterceptor, 0),
		gRPCStreamInterceptors:       make([]grpc.StreamServerInterceptor, 0),
		serverOption:                 make([]grpc.ServerOption, 0),
		gRPCUnaryClientInterceptors:  make([]grpc.UnaryClientInterceptor, 0),
		grpcStreamClientInterceptors: make([]grpc.StreamClientInterceptor, 0),
		dialOptions:                  make([]grpc.DialOption, 0),
	}, nil
}

// RegisterHTTPMux registers the http muxer to the service.
func (service *Service) RegisterHTTPMux(mux *http.ServeMux) {
	mux.Handle("/", service.runtimeMux)
	service.httpMux = mux
}

type httpMiddleware func(http.Handler) http.Handler

// AddHTTPMiddlewares adds middlewares to the service http handler
func (service *Service) AddHTTPMiddlewares(middlewares ...httpMiddleware) {
	for _, httpMiddleware := range middlewares {
		httpMiddleware(service.HTTPMux())
	}
}

// AddDialOptionsGRPC adds dial options to gRPC reverse proxy client
func (service *Service) AddDialOptionsGRPC(dialOptions ...grpc.DialOption) {
	for _, dialOption := range dialOptions {
		service.dialOptions = append(service.dialOptions, dialOption)
	}
}

// AddServerOptionsGRPC adds server options to gRPC server
func (service *Service) AddServerOptionsGRPC(serverOptions ...grpc.ServerOption) {
	for _, serverOption := range serverOptions {
		service.serverOption = append(service.serverOption, serverOption)
	}
}

// AddStreamServerInterceptorsGRPC adds stream interceptors to the gRPC server
func (service *Service) AddStreamServerInterceptorsGRPC(
	streamInterceptors ...grpc.StreamServerInterceptor,
) {
	for _, streamInterceptor := range streamInterceptors {
		service.gRPCStreamInterceptors = append(
			service.gRPCStreamInterceptors, streamInterceptor,
		)
	}
}

// AddUnaryServerInterceptorsGRPC adds unary interceptors to the gRPC server
func (service *Service) AddUnaryServerInterceptorsGRPC(
	unaryInterceptors ...grpc.UnaryServerInterceptor,
) {
	for _, unaryInterceptor := range unaryInterceptors {
		service.gRPCUnaryInterceptors = append(
			service.gRPCUnaryInterceptors, unaryInterceptor,
		)
	}
}

// AddStreamClientInterceptorsGRPC adds stream interceptors to the gRPC reverse proxy client
func (service *Service) AddStreamClientInterceptorsGRPC(
	streamInterceptors ...grpc.StreamClientInterceptor,
) {
	for _, streamInterceptor := range streamInterceptors {
		service.grpcStreamClientInterceptors = append(
			service.grpcStreamClientInterceptors, streamInterceptor,
		)
	}
}

// AddUnaryClientInterceptorsGRPC adds unary interceptors to the gRPC reverse proxy client
func (service *Service) AddUnaryClientInterceptorsGRPC(
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

// HTTPMux returns the http muxer for the service
func (service *Service) HTTPMux() *http.ServeMux {
	return service.httpMux
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

// AdminAccountClient returns admin account API for authentication
func (service *Service) AdminAccountClient() admin.AdminAPIClient {
	return service.adminAccountsClient
}

// NotificationClient returns the notification client
func (service *Service) NotificationClient() notification.NotificationServiceClient {
	return service.notificationClient
}

// UserAccountClient returns user account client API for authentication
func (service *Service) UserAccountClient() user.UserAPIClient {
	return service.userAccountsClient
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
