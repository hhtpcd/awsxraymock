package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"
)

func Test_handleSetThrottled(t *testing.T) {
	tests := []struct {
		name          string
		method        string
		url           string
		expectedCode  int
		expectedState string
		expectedRate  int
	}{
		{
			name:          "Success with valid rate",
			method:        http.MethodPost,
			url:           "/SetThrottled?rate=50",
			expectedCode:  http.StatusOK,
			expectedState: StatusThrottled,
			expectedRate:  50,
		},
		{
			name:          "Invalid rate",
			method:        http.MethodPost,
			url:           "/SetThrottled?rate=DOGS",
			expectedCode:  http.StatusBadRequest,
			expectedState: StatusOK,
			expectedRate:  0,
		},
		{
			name:          "Default rate without rate parameter",
			method:        http.MethodPost,
			url:           "/SetThrottled",
			expectedCode:  http.StatusOK,
			expectedState: StatusThrottled,
			expectedRate:  100,
		},
		{
			name:          "Rate is 101",
			method:        http.MethodPost,
			url:           "/SetThrottled?rate=101",
			expectedCode:  http.StatusBadRequest,
			expectedState: StatusOK,
			expectedRate:  0,
		},
		{
			name:          "Bad method",
			method:        http.MethodGet,
			url:           "/SetThrottled",
			expectedCode:  http.StatusMethodNotAllowed,
			expectedState: StatusOK,
			expectedRate:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger = zap.NewNop()
			statusManager = NewStatusManager()
			req := httptest.NewRequest(tt.method, tt.url, nil)
			w := httptest.NewRecorder()

			handleSetThrottled(w, req)

			if w.Code != tt.expectedCode {
				t.Errorf("handleSetThrottled() = %v, want %v", w.Code, tt.expectedCode)
			}

			status, rate := statusManager.GetStatus()
			if status != tt.expectedState {
				t.Errorf("statusManager.GetStatus() = %v, want %v", status, tt.expectedState)
			}

			if rate != tt.expectedRate {
				t.Errorf("statusManager.GetStatus() = %v, want %v", rate, tt.expectedRate)
			}
		})
	}
}

func Test_handleSetOK(t *testing.T) {
	tests := []struct {
		name          string
		method        string
		url           string
		expectedCode  int
		expectedState string
	}{
		{
			name:          "Success",
			method:        http.MethodPost,
			url:           "/SetOK",
			expectedCode:  http.StatusOK,
			expectedState: StatusOK,
		},
		{
			name:          "Bad method",
			method:        http.MethodGet,
			url:           "/SetOK",
			expectedCode:  http.StatusMethodNotAllowed,
			expectedState: StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger = zap.NewNop()
			statusManager = NewStatusManager()
			req := httptest.NewRequest(tt.method, tt.url, nil)
			w := httptest.NewRecorder()

			handleSetOK(w, req)

			if w.Code != tt.expectedCode {
				t.Errorf("handleSetOK() = %v, want %v", w.Code, tt.expectedCode)
			}

			status, _ := statusManager.GetStatus()
			if status != tt.expectedState {
				t.Errorf("statusManager.GetStatus() = %v, want %v", status, tt.expectedState)
			}
		})
	}
}

func Test_handleTraceSegments(t *testing.T) {
	tests := []struct {
		name         string
		method       string
		url          string
		throttleRate int
		expectedCode int
	}{
		{
			name:         "Success",
			method:       http.MethodPost,
			url:          "/TraceSegments",
			throttleRate: 0,
			expectedCode: http.StatusOK,
		},
		{
			name:         "Bad method",
			method:       http.MethodGet,
			url:          "/TraceSegments",
			throttleRate: 0,
			expectedCode: http.StatusMethodNotAllowed,
		},
		{
			name:         "Throttled",
			method:       http.MethodPost,
			url:          "/TraceSegments",
			throttleRate: 100,
			expectedCode: http.StatusTooManyRequests,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger = zap.NewNop()
			statusManager = NewStatusManager()
			req := httptest.NewRequest(tt.method, tt.url, nil)
			w := httptest.NewRecorder()

			statusManager.SetThrottled(tt.throttleRate)

			handleTraceSegments(w, req)

			if w.Code != tt.expectedCode {
				t.Errorf("handleTraceSegments() = %v, want %v", w.Code, tt.expectedCode)
			}
		})
	}
}
