package grpc

import (
	"reflect"
	"testing"

	"github.com/gidyon/config"
	"google.golang.org/grpc"
)

func TestNewGRPCClientConn(t *testing.T) {
	type args struct {
		cfg *config.Config
	}
	tests := []struct {
		name    string
		args    args
		want    *grpc.ClientConn
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			got, err := NewGRPCClientConn(tt.args.cfg, nil, nil, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewGRPCClientConn() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewGRPCClientConn() = %v, want %v", got, tt.want)
			}
		})
	}
}
