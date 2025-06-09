package utils

import (
	"errors"
	"net"
	"strings"
	"time"

	"github.com/penwern/curate-preservation-core/pkg/logger"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	DefaultRetryAttempts = 3
	DefaultInitialDelay  = 1 * time.Second
)

// IsTransientError checks if an error is transient (e.g., network issues).
func IsTransientError(err error) bool {
	if err == nil {
		return false
	}

	// Check for network-related errors
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}

	// Check for timeout error messages
	if strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "temporary unavailable") {
		return true
	}

	// Check for DNS resolution errors
	if strings.Contains(err.Error(), "no such host") || strings.Contains(err.Error(), "server misbehaving") {
		return true
	}

	// Check for specific HTTP status codes (e.g., 500, 502, 503, 504)
	if strings.Contains(err.Error(), "500") || strings.Contains(err.Error(), "502") ||
		strings.Contains(err.Error(), "503") || strings.Contains(err.Error(), "504") {
		return true
	}

	// Check for gRPC transient errors
	if s, ok := status.FromError(err); ok {
		switch s.Code() {
		case codes.Unavailable, codes.DeadlineExceeded, codes.ResourceExhausted:
			return true
		case codes.Unknown: // Because we want to retry when a3m fails
			return true
		}
	}

	return false
}

func WithRetry(operation func() error) error {
	return Retry(DefaultRetryAttempts, DefaultInitialDelay, operation, IsTransientError)
}

// Retry retries a function on transient errors with exponential backoff.
func Retry(attempts int, delay time.Duration, operation func() error, isTransient func(error) bool) error {
	var err error
	for i := range attempts {
		err = operation()
		if err == nil {
			return nil // Success
		}

		// Check if the error is transient
		if !isTransient(err) {
			logger.Debug("Non-transient error occurred: %v", err)
			return err // Non-transient error, stop retrying
		}

		logger.Error("Transient error occurred: %v. Retrying (%d/%d)...", err, i+1, attempts)
		time.Sleep(delay)
		delay *= 2 // Exponential backoff
	}
	logger.Error("Failed after %d attempts: %v", attempts, err)
	return err // Return the last error after exhausting retries
}
