package conn

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/grpc-ecosystem/go-grpc-middleware"
	"google.golang.org/grpc/balancer/roundrobin"

	"github.com/go-redis/redis"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"strings"

	// Imports mysql driver
	_ "github.com/go-sql-driver/mysql"
)

// DBOptions contains information for connecting to SQL database
type DBOptions struct {
	Dialect  string
	Host     string
	Port     string
	User     string
	Password string
	Schema   string
}

// PortNumber return port with any colon(:) removed
func (opt *DBOptions) PortNumber() string {
	return strings.TrimPrefix(opt.Port, ":")
}

// ToSQLDBUsingORM opens a connection to a SQL database using gorm
func ToSQLDBUsingORM(opt *DBOptions) (*gorm.DB, error) {
	// add MySQL driver specific parameter to parse date/time
	param := "charset=utf8&parseTime=true"

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?%s",
		opt.User,
		opt.Password,
		opt.Host,
		opt.PortNumber(),
		opt.Schema,
		param,
	)

	dialect := func() string {
		if opt.Dialect == "" {
			return "mysql"
		}
		return opt.Dialect
	}()

	db, err := gorm.Open(dialect, dsn)
	if err != nil {
		return nil, errors.Wrap(err, "(gorm) failed to open connection to database")
	}

	return db, nil
}

// ToSQLDB opens a connection to a SQL database using database/sql API
func ToSQLDB(opt *DBOptions) (*sql.DB, error) {
	// add MySQL driver specific parameter to parse date/time
	param := "charset=utf8&parseTime=true"

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?%s",
		opt.User,
		opt.Password,
		opt.Host,
		opt.PortNumber(),
		opt.Schema,
		param,
	)

	dialect := func() string {
		if opt.Dialect == "" {
			return "mysql"
		}
		return opt.Dialect
	}()

	sqlDB, err := sql.Open(dialect, dsn)
	if err != nil {
		return nil, errors.Wrap(err, "(sql) failed to open connection to database")
	}

	return sqlDB, nil
}

// RedisOptions contains information for connecting to redis server
type RedisOptions struct {
	Address string
	Port    string
}

// NewRedisClient creates a pool of connections to redis database.
func NewRedisClient(opt *RedisOptions) *redis.Client {
	redisURL := func(a, b string) string {
		if a == "" {
			return b
		}
		return a
	}

	uri := fmt.Sprintf("%s:%s", opt.Address, strings.TrimPrefix(opt.Port, ":"))
	return redis.NewClient(&redis.Options{
		Network: "tcp",
		Addr:    redisURL(uri, ":6379"),
	})
}

// GRPCDialOptions contains options for dialing a remote connection
type GRPCDialOptions struct {
	ServiceName string
	Address     string
	TLSCertFile string
	ServerName  string
	WithBlock   bool
}

// DialAccountService dials to authentication service and returns the grpc client connection
func DialAccountService(ctx context.Context, opt *GRPCDialOptions) (*grpc.ClientConn, error) {
	return DialService(ctx, opt)
}

// DialNotificationService dials to notification service and returns the grpc client connection
func DialNotificationService(ctx context.Context, opt *GRPCDialOptions) (*grpc.ClientConn, error) {
	return DialService(ctx, opt)
}

// DialService dials to any remote service and returns the grpc client connection
func DialService(ctx context.Context, opt *GRPCDialOptions) (*grpc.ClientConn, error) {
	creds, err := credentials.NewClientTLSFromFile(opt.TLSCertFile, opt.ServerName)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create tls config for %s service", opt.ServerName)
	}

	dopts := []grpc.DialOption{
		// Transport TLS
		grpc.WithTransportCredentials(creds),
		// Load balancer scheme
		grpc.WithBalancerName(roundrobin.Name),
		// Other interceptors
		grpc.WithUnaryInterceptor(
			grpc_middleware.ChainUnaryClient(
				waitForReadyInterceptor,
			),
		),
	}

	if opt.WithBlock {
		dopts = append(dopts, grpc.WithBlock())
	}

	return grpc.DialContext(ctx, opt.Address, dopts...)
}

// wait for ready call option for all client call
func waitForReadyInterceptor(
	ctx context.Context,
	method string,
	req, reply interface{},
	cc *grpc.ClientConn,
	invoker grpc.UnaryInvoker,
	opts ...grpc.CallOption,
) error {
	return invoker(ctx, method, req, reply, cc, grpc.WaitForReady(true))
}
