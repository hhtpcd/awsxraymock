package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/time/rate"
)

var logger *zap.Logger
var statusManager *StatusManager

func main() {
	certPath := flag.String("cert", "", "Path to the TLS certificate file")
	keyPath := flag.String("key", "", "Path to the TLS key file")
	rateLimit := flag.Float64("rate-limit", 2500, "Rate limit for the /TraceSegments endpoint")
	rateBurst := flag.Int("rate-burst", 2500, "Rate burst for the /TraceSegments endpoint")
	flag.Parse()

	statusManager = NewStatusManager()

	// Create a separate ServeMux for the healthz endpoint
	healthMux := http.NewServeMux()
	healthMux.HandleFunc("/healthz", handleHealthz)

	config := zap.NewProductionEncoderConfig()
	config.EncodeTime = zapcore.ISO8601TimeEncoder
	encoder := zapcore.NewJSONEncoder(config)
	core := zapcore.NewTee(
		zapcore.NewCore(encoder, zapcore.AddSync(zapcore.AddSync(zapcore.Lock(os.Stdout))), zapcore.DebugLevel),
	)

	logger = zap.New(core)
	defer logger.Sync()

	apiMux := http.NewServeMux()
	apiMux.Handle("/TraceSegments", RateLimitHandler(http.HandlerFunc(handleTraceSegments), rate.Limit(*rateLimit), *rateBurst))
	apiMux.HandleFunc("/SetOK", handleSetOK)
	apiMux.HandleFunc("/SetThrottled", handleSetThrottled)

	apiServer := newHTTPServer(":8443", apiMux)
	healthServer := newHTTPServer(":8080", healthMux)

	errCh := make(chan error, 2)

	logger.Info("starting server for health endpoints on :8080")
	go func() {
		if err := healthServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("health server failed: %w", err)
		}
	}()

	logger.Info("Starting server on :8443")
	logger.Info("Rate limit configured", zap.Float64("rate-limit", *rateLimit), zap.Int("rate-burst", *rateBurst))

	go func() {
		if err := serveTLSServer(apiServer, *certPath, *keyPath); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("api server failed: %w", err)
		}
	}()

	signalCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	var runErr error
	select {
	case <-signalCtx.Done():
		logger.Info("shutdown signal received")
	case err := <-errCh:
		runErr = err
		logger.Error("server exited unexpectedly", zap.Error(err))
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := healthServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("health server shutdown failed", zap.Error(err))
		if runErr == nil {
			runErr = err
		}
	}

	if err := apiServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("api server shutdown failed", zap.Error(err))
		if runErr == nil {
			runErr = err
		}
	}

	if runErr != nil {
		os.Exit(1)
	}
}

func newHTTPServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}
}

func serveTLSServer(server *http.Server, certPath, keyPath string) error {
	certificate, err := loadTLSCertificate(certPath, keyPath)
	if err != nil {
		return err
	}

	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		return err
	}

	tlsConfig := server.TLSConfig
	if tlsConfig == nil {
		tlsConfig = defaultTLSConfig()
	} else {
		tlsConfig = tlsConfig.Clone()
	}
	tlsConfig.Certificates = []tls.Certificate{certificate}

	return server.Serve(tls.NewListener(listener, tlsConfig))
}
