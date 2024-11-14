package main

import (
	"flag"
	"os"

	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/time/rate"
)

var logger *zap.Logger
var statusManager *StatusManager

// Metrics
var (
	opsProcessed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "awsxraymock_requests",
			Help: "A counter of processed requests",
		},
		[]string{"method", "http_code", "path", "error"},
	)
	docSum = promauto.NewSummaryVec(
		prometheus.SummaryOpts{
			Name: "awsxraymock_document_segment_size",
			Help: "The size of the document segments submitted to the API",
		},
		[]string{},
	)
	docProc = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "awsxraymock_documents",
			Help: "Counter for the number of segment documents processed",
		},
		[]string{"status"},
	)
)

func main() {
	// Define flags for certificate and key file paths
	certPath := flag.String("cert", "/path/to/cert.pem", "Path to the TLS certificate file")
	keyPath := flag.String("key", "/path/to/key.pem", "Path to the TLS key file")
	rateLimit := flag.Float64("rate-limit", 2500, "Rate limit for the /TraceSegments endpoint")
	rateBurst := flag.Int("rate-burst", 2500, "Rate burst for the /TraceSegments endpoint")
	flag.Parse()

	statusManager = NewStatusManager()
	statusManager.Limiter = rate.NewLimiter(rate.Limit(*rateLimit), *rateBurst)

	// Register handler for /TraceSegments path
	http.HandleFunc("/TraceSegments", handleTraceSegments)
	http.HandleFunc("/SetOK", handleSetOK)
	http.HandleFunc("/SetThrottled", handleSetThrottled)

	// Create a separate ServeMux for the healthz endpoint
	healthMux := http.NewServeMux()
	healthMux.HandleFunc("/healthz", handleHealthz)
	healthMux.HandleFunc("/metrics", promhttp.Handler())

	config := zap.NewProductionEncoderConfig()
	config.EncodeTime = zapcore.ISO8601TimeEncoder
	encoder := zapcore.NewJSONEncoder(config)
	core := zapcore.NewTee(
		zapcore.NewCore(encoder, zapcore.AddSync(zapcore.AddSync(zapcore.Lock(os.Stdout))), zapcore.DebugLevel),
	)

	logger = zap.New(core)

	logger.Info("starting server for health endpoints on :8080")
	go func() {
		if err := http.ListenAndServe(":8080", healthMux); err != nil {
			logger.Fatal("Health server failed to start: %v", zap.Error(err))
		}
	}()

	// Start server on port 8080
	logger.Info("Starting server on :8443")
	logger.Info("Rate limit configured", zap.Float64("rate-limit", *rateLimit), zap.Int("rate-burst", *rateBurst))
	if err := http.ListenAndServeTLS(":8443", *certPath, *keyPath, nil); err != nil {
		logger.Fatal("Server failed to start: %v", zap.Error(err))
	}
}
