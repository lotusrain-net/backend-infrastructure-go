package cache

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
)

func NewTLSConfig(enabled bool, serverName, caFile string) (*tls.Config, error) {
	if !enabled {
		return nil, nil
	}
	config := &tls.Config{
		MinVersion: tls.VersionTLS12,
		ServerName: serverName,
	}
	if caFile == "" {
		return config, nil
	}
	encoded, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("read Redis TLS CA: %w", err)
	}
	roots, err := x509.SystemCertPool()
	if err != nil || roots == nil {
		roots = x509.NewCertPool()
	}
	if !roots.AppendCertsFromPEM(encoded) {
		return nil, fmt.Errorf("redis TLS CA file contains no certificates")
	}
	config.RootCAs = roots
	return config, nil
}
