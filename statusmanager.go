package main

import (
	"sync"

	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

var StatusOK = "OK"
var StatusThrottled = "Throttled"

type StatusManager struct {
	mutex        sync.RWMutex
	status       string
	throttleRate int
	Limiter      *rate.Limiter
}

func NewStatusManager() *StatusManager {
	return &StatusManager{
		status: StatusOK,
	}
}

func (sm *StatusManager) GetStatus() (string, int) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	return sm.status, sm.throttleRate
}

func (sm *StatusManager) SetThrottled(rate int) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	sm.status = StatusThrottled
	sm.throttleRate = rate
	logger.Info("status changed",
		zap.String("status", sm.status),
		zap.Int("throttle_rate", sm.throttleRate),
	)
}

func (sm *StatusManager) SetOK() {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	sm.status = StatusOK
	logger.Info("status changed", zap.String("status", sm.status))
}
