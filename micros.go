package micros

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/RediSearch/redisearch-go/redisearch"
	"github.com/gidyon/config"
	"github.com/gidyon/micros/pkg/conn"
	microtls "github.com/gidyon/micros/pkg/tls"
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

// APIs contains API clients, connections and options for bootstrapping a micro-service
type APIs struct {
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
func NewService(ctx context.Context, cfg *config.Config) (*APIs, error) {

	// Initialize paths to cert and key
	microtls.SetKeyAndCertPaths(cfg.ServiceTLSKeyFile(), cfg.ServiceTLSCertFile())

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
			WithBlock:   true,
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
			WithBlock:   true,
		})
		if err != nil {
			return nil, errors.Wrap(err, "failed to connect to notification service")
		}
	}

	// Update the rest client
	runtimeMux := newRuntimeMux()

	return &APIs{
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
func (microsAPI *APIs) RegisterHTTPMux(mux *http.ServeMux) {
	mux.Handle("/", microsAPI.runtimeMux)
	microsAPI.httpMux = mux
}

type httpMiddleware func(http.Handler) http.Handler

// AddHTTPMiddlewares adds middlewares to the service http handler
func (microsAPI *APIs) AddHTTPMiddlewares(middlewares ...httpMiddleware) {
	for _, httpMiddleware := range middlewares {
		httpMiddleware(microsAPI.HTTPMux())
	}
}

// AddDialOptionsGRPC adds dial options to gRPC reverse proxy client
func (microsAPI *APIs) AddDialOptionsGRPC(dialOptions ...grpc.DialOption) {
	for _, dialOption := range dialOptions {
		microsAPI.dialOptions = append(microsAPI.dialOptions, dialOption)
	}
}

// AddServerOptionsGRPC adds server options to gRPC server
func (microsAPI *APIs) AddServerOptionsGRPC(serverOptions ...grpc.ServerOption) {
	for _, serverOption := range serverOptions {
		microsAPI.serverOption = append(microsAPI.serverOption, serverOption)
	}
}

// AddStreamServerInterceptorsGRPC adds stream interceptors to the gRPC server
func (microsAPI *APIs) AddStreamServerInterceptorsGRPC(
	streamInterceptors ...grpc.StreamServerInterceptor,
) {
	for _, streamInterceptor := range streamInterceptors {
		microsAPI.gRPCStreamInterceptors = append(
			microsAPI.gRPCStreamInterceptors, streamInterceptor,
		)
	}
}

// AddUnaryServerInterceptorsGRPC adds unary interceptors to the gRPC server
func (microsAPI *APIs) AddUnaryServerInterceptorsGRPC(
	unaryInterceptors ...grpc.UnaryServerInterceptor,
) {
	for _, unaryInterceptor := range unaryInterceptors {
		microsAPI.gRPCUnaryInterceptors = append(
			microsAPI.gRPCUnaryInterceptors, unaryInterceptor,
		)
	}
}

// AddStreamClientInterceptorsGRPC adds stream interceptors to the gRPC reverse proxy client
func (microsAPI *APIs) AddStreamClientInterceptorsGRPC(
	streamInterceptors ...grpc.StreamClientInterceptor,
) {
	for _, streamInterceptor := range streamInterceptors {
		microsAPI.grpcStreamClientInterceptors = append(
			microsAPI.grpcStreamClientInterceptors, streamInterceptor,
		)
	}
}

// AddUnaryClientInterceptorsGRPC adds unary interceptors to the gRPC reverse proxy client
func (microsAPI *APIs) AddUnaryClientInterceptorsGRPC(
	unaryInterceptors ...grpc.UnaryClientInterceptor,
) {
	for _, unaryInterceptor := range unaryInterceptors {
		microsAPI.gRPCUnaryClientInterceptors = append(
			microsAPI.gRPCUnaryClientInterceptors, unaryInterceptor,
		)
	}
}

// HTTPMux returns the http muxer for the service
func (microsAPI *APIs) HTTPMux() *http.ServeMux {
	return microsAPI.httpMux
}

// RuntimeMux returns the runtime muxer for the service
func (microsAPI *APIs) RuntimeMux() *runtime.ServeMux {
	return microsAPI.runtimeMux
}

// ClientConn returns the underlying client connection to grpc server used by reverse proxy
func (microsAPI *APIs) ClientConn() *grpc.ClientConn {
	return microsAPI.clientConn
}

// GRPCServer returns the grpc server
func (microsAPI *APIs) GRPCServer() *grpc.Server {
	return microsAPI.gRPCServer
}

// DB returns a gorm db instance
func (microsAPI *APIs) DB() *gorm.DB {
	return microsAPI.db
}

// SQLDB returns database/sql db instance
func (microsAPI *APIs) SQLDB() *sql.DB {
	return microsAPI.sqlDB
}

// RedisClient returns a redis client
func (microsAPI *APIs) RedisClient() *redis.Client {
	return microsAPI.redisClient
}

// RediSearchClient returns redisearch client
func (microsAPI *APIs) RediSearchClient() *redisearch.Client {
	return microsAPI.rediSearchClient
}

// AdminAccountClient returns admin account API for authentication
func (microsAPI *APIs) AdminAccountClient() admin.AdminAPIClient {
	return microsAPI.adminAccountsClient
}

// NotificationClient returns the notification client
func (microsAPI *APIs) NotificationClient() notification.NotificationServiceClient {
	return microsAPI.notificationClient
}

// UserAccountClient returns user account client API for authentication
func (microsAPI *APIs) UserAccountClient() user.UserAPIClient {
	return microsAPI.userAccountsClient
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
