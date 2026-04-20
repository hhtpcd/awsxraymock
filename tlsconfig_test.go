package main

import (
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func Test_loadTLSCertificate(t *testing.T) {
	t.Run("generates self signed certificate when paths omitted", func(t *testing.T) {
		certificate, err := loadTLSCertificate("", "")
		if err != nil {
			t.Fatalf("loadTLSCertificate() error = %v", err)
		}

		if len(certificate.Certificate) == 0 {
			t.Fatal("expected generated certificate data")
		}
	})

	t.Run("requires cert and key together", func(t *testing.T) {
		if _, err := loadTLSCertificate("cert.pem", ""); err == nil {
			t.Fatal("expected error when key path is missing")
		}
	})

	t.Run("loads certificate pair from disk", func(t *testing.T) {
		certificate, err := generateSelfSignedCertificate()
		if err != nil {
			t.Fatalf("generateSelfSignedCertificate() error = %v", err)
		}

		dir := t.TempDir()
		certPath := filepath.Join(dir, "server.crt")
		keyPath := filepath.Join(dir, "server.key")

		privateKey, ok := certificate.PrivateKey.(*rsa.PrivateKey)
		if !ok {
			t.Fatalf("unexpected private key type %T", certificate.PrivateKey)
		}

		if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: certificate.Certificate[0],
		}), 0o600); err != nil {
			t.Fatalf("WriteFile(cert) error = %v", err)
		}

		if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
		}), 0o600); err != nil {
			t.Fatalf("WriteFile(key) error = %v", err)
		}

		loadedCertificate, err := loadTLSCertificate(certPath, keyPath)
		if err != nil {
			t.Fatalf("loadTLSCertificate() error = %v", err)
		}

		if len(loadedCertificate.Certificate) == 0 {
			t.Fatal("expected loaded certificate data")
		}
	})
}

func Test_defaultTLSConfig(t *testing.T) {
	config := defaultTLSConfig()
	if config.MinVersion != tls.VersionTLS12 {
		t.Fatalf("defaultTLSConfig().MinVersion = %v, want %v", config.MinVersion, tls.VersionTLS12)
	}
}

func Test_newHTTPServer(t *testing.T) {
	server := newHTTPServer(":8443", http.NewServeMux())

	if server.ReadHeaderTimeout != 5*time.Second {
		t.Fatalf("ReadHeaderTimeout = %v, want %v", server.ReadHeaderTimeout, 5*time.Second)
	}

	if server.ReadTimeout != 10*time.Second {
		t.Fatalf("ReadTimeout = %v, want %v", server.ReadTimeout, 10*time.Second)
	}

	if server.WriteTimeout != 10*time.Second {
		t.Fatalf("WriteTimeout = %v, want %v", server.WriteTimeout, 10*time.Second)
	}

	if server.IdleTimeout != 2*time.Minute {
		t.Fatalf("IdleTimeout = %v, want %v", server.IdleTimeout, 2*time.Minute)
	}
}
