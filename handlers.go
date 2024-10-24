package main

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net/http"
	"strconv"

	"go.uber.org/zap"
)

// ThrottleResponse represents the JSON response structure
type ThrottleResponse struct {
	Rate    int    `json:"rate"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
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

func handleTraceSegments(w http.ResponseWriter, r *http.Request) {
	logger.Info("Received request",
		zap.String("method", r.Method),
		zap.String("path", r.URL.Path),
		zap.String("user_agent", r.UserAgent()),
	)

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	status, rate := statusManager.GetStatus()

	if status == StatusThrottled {
		// generate a random number
		prob := rand.IntN(100)

		if prob < rate {
			response := ThrottledException{
				Message: "Rate exceeded",
				Type:    "ThrottlingException",
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(response)
			return
		}

	}

	response := Response{
		UnprocessedTraceSegments: []TraceSegment{
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
}
