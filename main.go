package main

import (
	"encoding/json"
	"flag"
	"os"
	"sync"

	"net/http"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var logger *zap.Logger
var statusManager = NewStatusManager()
var StatusOK = "OK"
var StatusThrottled = "Throttled"
var crtPath, keyPath string

type StatusManager struct {
	mutex  sync.RWMutex
	status string
}

func NewStatusManager() *StatusManager {
	return &StatusManager{
		status: StatusOK,
	}
}

func (sm *StatusManager) GetStatus() string {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	return sm.status
}

func (sm *StatusManager) SetThrottled(status string) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	sm.status = StatusThrottled
	logger.Info("status changed", zap.String("status", sm.status))
}

func (sm *StatusManager) SetOK() {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	sm.status = StatusOK
	logger.Info("status changed", zap.String("status", sm.status))
}

// Response represents the structure of our JSON response
type Response struct {
	UnprocessedTraceSegments []TraceSegment `json:"UnprocessedTraceSegments"`
}

// TraceSegment represents each segment in the response
type TraceSegment struct {
	Id        string `json:"Id"`
	ErrorCode string `json:"ErrorCode"`
	Message   string `json:"Message"`
}

// ThrottledException represents the structure of the error response
type ThrottledException struct {
	Message string `json:"message"`
	Type    string `json:"__type"`
}

func handleSetOK(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	statusManager.SetOK()
	w.WriteHeader(http.StatusOK)
}

func handleSetThrottled(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	statusManager.SetThrottled(StatusThrottled)
	w.WriteHeader(http.StatusOK)
}

func handleTraceSegments(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	status := statusManager.GetStatus()

	if status == StatusThrottled {
		response := ThrottledException{
			Message: "Rate exceeded",
			Type:    "ThrottlingException",
		}

		logger.Info("Received request",
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.String("status_code", "429"),
			zap.String("message", "Rate Exceeded"),
			zap.String("user_agent", r.UserAgent()),
		)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(response)
		return
	} else {
		response := Response{
			UnprocessedTraceSegments: []TraceSegment{
				{
					Id:        "",
					ErrorCode: "",
					Message:   "",
				},
			},
		}

		logger.Info("Received request",
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.String("status_code", "200"),
			zap.String("message", "ok"),
			zap.String("user_agent", r.UserAgent()),
		)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}
}

func main() {
	// Define flags for certificate and key file paths
	certPath := flag.String("cert", "/path/to/cert.pem", "Path to the TLS certificate file")
	keyPath := flag.String("key", "/path/to/key.pem", "Path to the TLS key file")
	flag.Parse()

	// Register handler for /TraceSegments path
	http.HandleFunc("/TraceSegments", handleTraceSegments)
	http.HandleFunc("/SetOK", handleSetOK)
	http.HandleFunc("/SetThrottled", handleSetThrottled)

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
