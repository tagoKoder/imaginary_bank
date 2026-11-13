// bff\internal\client\grpc\conn.go
package grpc

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"github.com/tagoKoder/bff/internal/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

func dial(addr string, cfg config.Config) (*grpc.ClientConn, error) {
	opts, err := dialOptions(cfg)
	if err != nil {
		return nil, err
	}

	// grpc.NewClient is non-blocking; individual RPCs have deadlines.
	return grpc.NewClient(addr, opts...)
}

func dialOptions(cfg config.Config) ([]grpc.DialOption, error) {
	if !cfg.GRPCUseTLS {
		if cfg.Env != "local" {
			return nil, fmt.Errorf("insecure gRPC transport refused outside local environment")
		}
		return []grpc.DialOption{
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithChainUnaryInterceptor(OutgoingHeadersUnaryInterceptor()),
			grpc.WithChainStreamInterceptor(OutgoingHeadersStreamInterceptor()),
		}, nil
	}

	caPem, err := os.ReadFile(cfg.GRPCCACertPath)
	if err != nil {
		return nil, fmt.Errorf("read CA: %w", err)
	}
	cp := x509.NewCertPool()
	if !cp.AppendCertsFromPEM(caPem) {
		return nil, fmt.Errorf("append CA failed")
	}

	tlsCfg := &tls.Config{
		RootCAs:    cp,
		MinVersion: tls.VersionTLS12,
	}

	if cfg.GRPCClientCertPath != "" && cfg.GRPCClientKeyPath != "" {
		cert, err := tls.LoadX509KeyPair(cfg.GRPCClientCertPath, cfg.GRPCClientKeyPath)
		if err != nil {
			return nil, fmt.Errorf("load client cert: %w", err)
		}
		tlsCfg.Certificates = []tls.Certificate{cert}
	}

	return []grpc.DialOption{
		grpc.WithTransportCredentials(credentials.NewTLS(tlsCfg)),
		grpc.WithChainUnaryInterceptor(OutgoingHeadersUnaryInterceptor()),
		grpc.WithChainStreamInterceptor(OutgoingHeadersStreamInterceptor()),
	}, nil
}
