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
	return NewTLSClientConfigValues(
		cfg.TLSServerName,
		cfg.TLSRootCAFile,
		cfg.TLSClientCertFile,
		cfg.TLSClientKeyFile,
		cfg.TLSInsecureSkipVerify,
	)
}

// NewTLSClientConfigValues builds a tls.Config from explicit fields (used by per-trunk TLS profiles).
func NewTLSClientConfigValues(serverName, rootCAFile, clientCertFile, clientKeyFile string, insecureSkipVerify bool) (*tls.Config, error) {
	t := &tls.Config{
		MinVersion: tls.VersionTLS12,
		ServerName: serverName,
	}
	if insecureSkipVerify {
		t.InsecureSkipVerify = true
	}
	if rootCAFile != "" {
		pem, err := os.ReadFile(rootCAFile)
		if err != nil {
			return nil, fmt.Errorf("read TLS root CA: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("parse TLS root CA PEM")
		}
		t.RootCAs = pool
	}
	if clientCertFile != "" && clientKeyFile != "" {
		cert, err := tls.LoadX509KeyPair(clientCertFile, clientKeyFile)
		if err != nil {
			return nil, fmt.Errorf("load client cert: %w", err)
		}
		t.Certificates = []tls.Certificate{cert}
	}
	return t, nil
}
