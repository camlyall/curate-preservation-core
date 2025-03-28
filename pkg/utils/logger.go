package utils

import "slices"

var ValidLogLevels = []string{"debug", "info", "warn", "error", "Debug", "Info", "Warn", "Error", "DEBUG", "INFO", "WARN", "ERROR"}

func ValidateLogLevel(level string) bool {
	return slices.Contains(ValidLogLevels, level)
}
