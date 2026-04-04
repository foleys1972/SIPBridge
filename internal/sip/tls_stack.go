package sip

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"sipbridge/internal/config"
)

// NewTLSClientConfig builds a tls.Config for dialing Oracle / AudioCodes SBCs (TLS, optional mTLS, custom trust).
func NewTLSClientConfig(cfg config.SIPConfig) (*tls.Config, error) {
	t := &tls.Config{
		MinVersion: tls.VersionTLS12,
		ServerName: cfg.TLSServerName,
	}
	if cfg.TLSInsecureSkipVerify {
		t.InsecureSkipVerify = true
	}
	if cfg.TLSRootCAFile != "" {
		pem, err := os.ReadFile(cfg.TLSRootCAFile)
		if err != nil {
			return nil, fmt.Errorf("read TLS root CA: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("parse TLS root CA PEM")
		}
		t.RootCAs = pool
	}
	if cfg.TLSClientCertFile != "" && cfg.TLSClientKeyFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.TLSClientCertFile, cfg.TLSClientKeyFile)
		if err != nil {
			return nil, fmt.Errorf("load client cert: %w", err)
		}
		t.Certificates = []tls.Certificate{cert}
	}
	return t, nil
}
