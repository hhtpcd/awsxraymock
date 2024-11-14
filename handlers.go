package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

// ThrottleResponse represents the JSON response structure
type ThrottleResponse struct {
	Rate    int    `json:"rate"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

// Response represents the structure of our JSON response
type Response struct {
	UnprocessedTraceSegments []UnprocessedSegmentDocuments `json:"UnprocessedTraceSegments"`
}

// UnprocessedSegmentDocuments represents each segment in the response
type UnprocessedSegmentDocuments struct {
	Id        string `json:"Id"`
	ErrorCode string `json:"ErrorCode"`
	Message   string `json:"Message"`
}

// ExceptionResponse represents the structure of the error response
type ExceptionResponse struct {
	Message string `json:"message"`
	Type    string `json:"__type"`
}

type TraceSegments struct {
	TraceSegmentDocuments []string `json:"TraceSegmentDocuments"`
}

func handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
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

	w.Header().Set("Content-Type", "application/json")

	// Get and validate rate parameter
	rate := r.URL.Query().Get("rate")
	rateValue := 100 // Default value

	if rate != "" {
		var err error
		rateValue, err = strconv.Atoi(rate)
		if err != nil {
			logger.Warn("rate is not a number")
			http.Error(w, "rate is not a number", http.StatusBadRequest)
			return
		}

		if rateValue < 0 || rateValue > 100 {
			logger.Info("rate out of range",
				zap.Int("rate", rateValue),
			)
			logger.Warn("rate out of range")
			http.Error(w, "rate out of range", http.StatusBadRequest)
			return
		}
	}

	response := ThrottleResponse{
		Rate:    rateValue,
		Message: fmt.Sprintf("throttle rate set to: %d%%", rateValue),
	}

	statusManager.SetThrottled(rateValue)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// RateLimitHandler is a middleware that applies rate limiting to a handler
// The rate limit will be per server instance and will not work well across
// multiple instances of the server.
func RateLimitHandler(next http.Handler, limit rate.Limit, burst int) http.Handler {
	limiter := rate.NewLimiter(limit, burst)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !limiter.Allow() {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(ExceptionResponse{
				Message: "Rate exceeded",
				Type:    "ThrottlingException",
			})
			return
		}

		next.ServeHTTP(w, r)
	})
}

func handleTraceSegments(w http.ResponseWriter, r *http.Request) {
	// TODO: add metrics for requets
	// - histogram for request document count
	// - counter for request types (throttled, not)
	// - counter for JSON decode errors
	// - counter for rate limit exceeded

	// Move to main.go, and increment here instead.
	logger.Info("Received request",
		zap.String("method", r.Method),
		zap.String("path", r.URL.Path),
		zap.String("user_agent", r.UserAgent()),
	)

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		opsProcessed.WithLabelValues(
			r.Method,
			string(http.StatusMethodNotAllowed),
			r.URL.String(),
			"method_not_allowed",
		).Inc()
		return
	}

	// Read JSON post body
	var traceSegments TraceSegments
	err := json.NewDecoder(r.Body).Decode(&traceSegments)
	if err != nil {
		logger.Error("Failed to decode JSON",
			zap.Error(err),
		)
		http.Error(w, "Failed to decode JSON", http.StatusBadRequest)
		opsProcessed.WithLabelValues(
			r.Method,
			string(http.StatusBadRequest),
			r.URL.String(),
			"bad_request",
		).Inc()
		return
	}

	defer r.Body.Close()

	// Count documents in the submission.
	docCount := len(traceSegments.TraceSegmentDocuments)
	docSum.WithLabelValues().Observe(float64(docCount))

	if !statusManager.Limiter.AllowN(time.Now(), docCount) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(ExceptionResponse{
			Message: "Rate exceeded",
			Type:    "ThrottlingException",
		})

		// Record metrics
		opsProcessed.WithLabelValues(
			r.Method,
			string(http.StatusTooManyRequests),
			r.URL.String(),
			"too_many_requests",
		).Inc()
		docProc.WithLabelValues(
			"rate_limited",
		).Add(float64(docCount))

		return
	}

	response := Response{
		UnprocessedTraceSegments: []UnprocessedSegmentDocuments{
			{
				Id:        "",
				ErrorCode: "",
				Message:   "",
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
	// Record Metrics
	opsProcessed.WithLabelValues(
		r.Method,
		string(http.StatusOK),
		r.URL.String(),
		"",
	).Inc()
	docProc.WithLabelValues(
		"ok",
	).Add(float64(docCount))
}
