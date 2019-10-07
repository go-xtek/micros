package grpc

import (
	"context"
	"reflect"
	"testing"

	"github.com/gidyon/config"
	"google.golang.org/grpc"
)

func TestNewGRPCServer(t *testing.T) {
	type args struct {
		ctx context.Context
		cfg *config.Config
	}
	tests := []struct {
		name    string
		args    args
		want    *grpc.Server
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewGRPCServer(tt.args.ctx, tt.args.cfg, nil, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewGRPCServer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewGRPCServer() = %v, want %v", got, tt.want)
			}
		})
	}
}
