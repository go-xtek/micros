package microtls

import (
	"crypto/tls"
	"crypto/x509"
	"github.com/pkg/errors"
	"io/ioutil"
	"strings"
)

var (
	crt = "certs/cert.pem"
	key = "certs/key.pem"
)

// SetKeyAndCertPaths initializes path to private key and certificate
func SetKeyAndCertPaths(keyPath, certPath string) {
	if strings.TrimSpace(keyPath) != "" {
		crt = certPath
	}
	if strings.TrimSpace(certPath) != "" {
		key = keyPath
	}
}

// GetCert returns a certificate pair, pool and an error
func GetCert() (*tls.Certificate, *x509.CertPool, error) {
	// Read certificate
	serverCrt, err := ioutil.ReadFile(crt)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to read cert file")
	}

	// Read private key
	serverKey, err := ioutil.ReadFile(key)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to read key file")
	}

	cert, err := tls.X509KeyPair(serverCrt, serverKey)
	if err != nil {
		return nil, nil, errors.Wrap(err, "could not load server key pair")
	}

	cp := x509.NewCertPool()
	ok := cp.AppendCertsFromPEM(serverCrt)
	if !ok {
		return nil, nil, errors.New("failed to append cert to pool")
	}

	return &cert, cp, nil
}

// ClientConfig creates a tls config object for client
func ClientConfig() (*tls.Config, error) {
	cert, certPool, err := GetCert()
	if err != nil {
		return nil, err
	}

	tlsConfig := &tls.Config{
		RootCAs:            certPool,
		Certificates:       []tls.Certificate{*cert},
		InsecureSkipVerify: true,
	}

	return tlsConfig, nil
}

// GRPCServerConfig creates a tls config object for grpc server
func GRPCServerConfig() (*tls.Config, error) {
	cert, certPool, err := GetCert()
	if err != nil {
		return nil, err
	}

	tlsConfig := &tls.Config{
		ClientAuth:         tls.VerifyClientCertIfGiven,
		Certificates:       []tls.Certificate{*cert},
		ClientCAs:          certPool,
		InsecureSkipVerify: true,
	}

	return tlsConfig, nil
}

// HTTPServerConfig creates a tls config object for http server
func HTTPServerConfig() (*tls.Config, error) {
	cert, certPool, err := GetCert()
	if err != nil {
		return nil, err
	}

	tlsConfig := &tls.Config{
		ClientAuth:   tls.VerifyClientCertIfGiven,
		Certificates: []tls.Certificate{*cert},
		ClientCAs:    certPool,
		NextProtos:   []string{"h2"},
	}

	return tlsConfig, nil
}
