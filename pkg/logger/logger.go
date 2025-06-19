// Package logger provides a simple logging utility using Uber's Zap library.
// It supports different log levels and structured logging.
// It is designed to be used across the application for consistent logging.
package logger

import (
	"os"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Global logger instance
var log *zap.SugaredLogger

const logFilePath = "/var/log/curate/curate-preservation-core.log"

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
	case "fatal", "Fatal", "FATAL":
		zapLevel = zapcore.FatalLevel
	case "panic", "Panic", "PANIC":
		zapLevel = zapcore.PanicLevel
	default:
		zapLevel = zapcore.InfoLevel
	}

	// Console encoder config (minimal fields for journald)
	consoleEncoderConfig := zap.NewProductionEncoderConfig()
	consoleEncoderConfig.TimeKey = ""
	consoleEncoderConfig.LevelKey = ""
	consoleEncoderConfig.CallerKey = ""
	consoleEncoder := zapcore.NewConsoleEncoder(consoleEncoderConfig)

	// File encoder config (full fields)
	fileEncoderConfig := zap.NewProductionEncoderConfig()
	fileEncoderConfig.TimeKey = "timestamp"
	fileEncoderConfig.EncodeTime = func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString(t.Format("2006-01-02 15:04:05.00"))
	}
	fileEncoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	fileEncoderConfig.EncodeCaller = zapcore.ShortCallerEncoder
	fileEncoder := zapcore.NewConsoleEncoder(fileEncoderConfig)

	// Outputs
	consoleSyncer := zapcore.AddSync(os.Stdout)
	file, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o6)
	if err != nil {
		panic("failed to open log file: " + err.Error())
	}
	fileSyncer := zapcore.AddSync(file)

	// Cores
	consoleCore := zapcore.NewCore(consoleEncoder, consoleSyncer, zapLevel)
	fileCore := zapcore.NewCore(fileEncoder, fileSyncer, zapLevel)

	// Tee core
	core := zapcore.NewTee(consoleCore, fileCore)

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

// Panic logs a panic message and exits
func Panic(msg string, args ...any) {
	GetLogger().Panicf(msg, args...)
}

// With returns a logger with added structured context
func With(fields ...any) *zap.SugaredLogger {
	return GetLogger().With(fields...)
}
