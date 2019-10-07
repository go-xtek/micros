package micros

import (
	"context"
	"database/sql"
	"net/http"
	"testing"

	"github.com/RediSearch/redisearch-go/redisearch"
	"github.com/gidyon/account/pkg/api/admin"
	"github.com/gidyon/account/pkg/api/user"
	"github.com/gidyon/config"
	"github.com/gidyon/notification/pkg/api"
	"github.com/go-redis/redis"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/jinzhu/gorm"
	"google.golang.org/grpc"
)

func TestAPIs_Run(t *testing.T) {
	type fields struct {
		cfg                 *config.Config
		db                  *gorm.DB
		sqlDB               *sql.DB
		redisClient         *redis.Client
		notificationClient  notification.NotificationServiceClient
		adminAccountsClient admin.AdminAPIClient
		userAccountsClient  user.UserAPIClient
		rediSearchClient    *redisearch.Client
		httpMux             *http.ServeMux
		runtimeMux          *runtime.ServeMux
		gRPCServer          *grpc.Server
		clientConn          *grpc.ClientConn
	}
	type args struct {
		ctx context.Context
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			microsAPI := &APIs{
				cfg:                 tt.fields.cfg,
				db:                  tt.fields.db,
				sqlDB:               tt.fields.sqlDB,
				redisClient:         tt.fields.redisClient,
				notificationClient:  tt.fields.notificationClient,
				adminAccountsClient: tt.fields.adminAccountsClient,
				userAccountsClient:  tt.fields.userAccountsClient,
				rediSearchClient:    tt.fields.rediSearchClient,
				httpMux:             tt.fields.httpMux,
				runtimeMux:          tt.fields.runtimeMux,
				gRPCServer:          tt.fields.gRPCServer,
				clientConn:          tt.fields.clientConn,
			}
			if err := microsAPI.Run(tt.args.ctx); (err != nil) != tt.wantErr {
				t.Errorf("APIs.Run() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
