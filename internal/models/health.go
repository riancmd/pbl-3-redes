package models

import "time"

// Representa o resultado de um checkHealth
type CheckResult struct {
	Endpoint   string
	StatusCode int
	Duration   time.Duration
	Err        error
}
