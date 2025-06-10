package utils

import "slices"

// ValidLogLevels is a list of valid log levels.
var ValidLogLevels = []string{"debug", "info", "warn", "error", "Debug", "Info", "Warn", "Error", "DEBUG", "INFO", "WARN", "ERROR"}

// ValidateLogLevel checks if a log level is valid.
func ValidateLogLevel(level string) bool {
	return slices.Contains(ValidLogLevels, level)
}
