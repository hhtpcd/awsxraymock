package main

import (
	"flag"
	"os"

	"net/http"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var logger *zap.Logger
var statusManager *StatusManager

func main() {
	// Define flags for certificate and key file paths
	certPath := flag.String("cert", "/path/to/cert.pem", "Path to the TLS certificate file")
	keyPath := flag.String("key", "/path/to/key.pem", "Path to the TLS key file")
	flag.Parse()

	statusManager = NewStatusManager()

	// Register handler for /TraceSegments path
	http.HandleFunc("/TraceSegments", handleTraceSegments)
	http.HandleFunc("/SetOK", handleSetOK)
	http.HandleFunc("/SetThrottled", handleSetThrottled)
	http.HandleFunc("/healthz", handleHealthz)

	config := zap.NewProductionEncoderConfig()
	config.EncodeTime = zapcore.ISO8601TimeEncoder
	encoder := zapcore.NewJSONEncoder(config)
	core := zapcore.NewTee(
		zapcore.NewCore(encoder, zapcore.AddSync(zapcore.AddSync(zapcore.Lock(os.Stdout))), zapcore.DebugLevel),
	)

	logger = zap.New(core)

	// Start server on port 8080
	logger.Info("Starting server on :8443")
	if err := http.ListenAndServeTLS(":8443", *certPath, *keyPath, nil); err != nil {
		logger.Fatal("Server failed to start: %v", zap.Error(err))
	}
}
