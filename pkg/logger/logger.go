package logger

import (
	"os"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Global logger instance
var log *zap.SugaredLogger

// Initialize sets up the logger with the given log level
func Initialize(level string) {
	// Parse log level
	var zapLevel zapcore.Level
	switch level {
	case "debug", "Debug", "DEBUG":
		zapLevel = zapcore.DebugLevel
	case "info", "Info", "INFO":
		zapLevel = zapcore.InfoLevel
	case "warn", "Warn", "WARN":
		zapLevel = zapcore.WarnLevel
	case "error", "Error", "ERROR":
		zapLevel = zapcore.ErrorLevel
	default:
		zapLevel = zapcore.InfoLevel
	}

	// Create encoder config
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.TimeKey = "timestamp"
	// encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder // TODO: Re-enable this for machine-readable logs
	encoderConfig.EncodeTime = func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString(t.Format("2006-01-02 15:04:05.00"))
	}
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	encoderConfig.EncodeCaller = zapcore.ShortCallerEncoder

	// Create the core
	core := zapcore.NewCore(
		zapcore.NewConsoleEncoder(encoderConfig),
		zapcore.AddSync(os.Stdout),
		zapLevel,
	)

	// Create the logger
	logger := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))
	log = logger.Sugar()
}

// GetLogger returns the global logger instance
func GetLogger() *zap.SugaredLogger {
	if log == nil {
		Initialize("info") // Default to info level
	}
	return log
}

// Debug logs a debug message
func Debug(msg string, args ...any) {
	GetLogger().Debugf(msg, args...)
}

// Info logs an info message
func Info(msg string, args ...any) {
	GetLogger().Infof(msg, args...)
}

// Warn logs a warning message
func Warn(msg string, args ...any) {
	GetLogger().Warnf(msg, args...)
}

// Error logs an error message
func Error(msg string, args ...any) {
	GetLogger().Errorf(msg, args...)
}

// Fatal logs a fatal message and exits
func Fatal(msg string, args ...any) {
	GetLogger().Fatalf(msg, args...)
}

// With returns a logger with added structured context
func With(fields ...any) *zap.SugaredLogger {
	return GetLogger().With(fields...)
}
