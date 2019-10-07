package microtls

import (
	"crypto/tls"
	"crypto/x509"
	"reflect"
	"testing"
)

func TestSetKeyAndCertPaths(t *testing.T) {
	type args struct {
		keyPath  string
		certPath string
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetKeyAndCertPaths(tt.args.keyPath, tt.args.certPath)
		})
	}
}

func TestGetCert(t *testing.T) {
	tests := []struct {
		name    string
		want    *tls.Certificate
		want1   *x509.CertPool
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1, err := GetCert()
			if (err != nil) != tt.wantErr {
				t.Errorf("GetCert() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetCert() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("GetCert() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestClientConfig(t *testing.T) {
	tests := []struct {
		name    string
		want    *tls.Config
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ClientConfig()
			if (err != nil) != tt.wantErr {
				t.Errorf("ClientConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ClientConfig() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGRPCServerConfig(t *testing.T) {
	tests := []struct {
		name    string
		want    *tls.Config
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GRPCServerConfig()
			if (err != nil) != tt.wantErr {
				t.Errorf("GRPCServerConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GRPCServerConfig() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHTTPServerConfig(t *testing.T) {
	tests := []struct {
		name    string
		want    *tls.Config
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := HTTPServerConfig()
			if (err != nil) != tt.wantErr {
				t.Errorf("HTTPServerConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("HTTPServerConfig() = %v, want %v", got, tt.want)
			}
		})
	}
}
