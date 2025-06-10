// Package internal provides the HTTP server implementation for the preservation service.
// It includes the handler for processing preservation requests, recovery middleware, and request ID generation.
// It also manages active requests to prevent duplicate processing.
package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/penwern/curate-preservation-core/pkg/config"
	"github.com/penwern/curate-preservation-core/pkg/logger"
)

// Global map to track active requests
var (
	activeRequests sync.Map
)

// ServiceRunner is an interface that defines the methods required by the HTTP handler
type ServiceRunner interface {
	RunArgs(context.Context, *ServiceArgs) error
}

// recoveryMiddleware wraps an http.HandlerFunc with panic recovery
func recoveryMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				logger.Error(fmt.Sprintf("Panic recovered: %v", err))
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()
		next(w, r)
	}
}

// Handler creates a new HTTP handler for the preservation service
func Handler(svc ServiceRunner) http.HandlerFunc {
	handler := func(w http.ResponseWriter, r *http.Request) {
		// Defaults
		req := ServiceArgs{
			Cleanup: true,
		}

		// Log basic request info
		logger.Debug(fmt.Sprintf("Received request - Method: %s, Path: %s, Remote: %s, Agent: %s",
			r.Method,
			r.URL.Path,
			r.RemoteAddr,
			r.UserAgent(),
		))

		// Read and log request body
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			logger.Error(fmt.Sprintf("Failed to read request body: %v", err))
			http.Error(w, "Failed to read request body", http.StatusBadRequest)
			return
		}
		// Restore the body for later use
		r.Body = io.NopCloser(bytes.NewReader(bodyBytes))

		// Pretty print the JSON body for logging
		var prettyJSON bytes.Buffer
		if err := json.Indent(&prettyJSON, bodyBytes, "", "  "); err == nil {
			logger.Debug(fmt.Sprintf("Request body:\n%s", prettyJSON.String()))
		} else {
			logger.Debug(fmt.Sprintf("Request body (raw): %s", string(bodyBytes)))
		}

		// Decode request args
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			logger.Error(fmt.Sprintf("Failed to decode request body: %v", err))
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Handle preservation config defaults
		if req.PreservationCfg == nil {
			preservationCfg := config.DefaultPreservationConfig()
			req.PreservationCfg = &preservationCfg
		} else {
			// Merge with defaults
			preservationCfg := req.PreservationCfg.MergeWithDefaults()
			req.PreservationCfg = &preservationCfg
		}

		if req.CellsUsername == "" {
			logger.Error("Received request with no username")
			http.Error(w, "no username provided", http.StatusBadRequest)
			return
		}

		// If theres no paths, we're going to look for nodes from Cells
		req.PathsResolved = false
		if len(req.CellsPaths) == 0 {
			if len(req.CellsNodes) == 0 {
				logger.Error("Received request with no paths or nodes")
				http.Error(w, "no paths or nodes provided", http.StatusBadRequest)
				return
			}
			for _, node := range req.CellsNodes {
				req.CellsPaths = append(req.CellsPaths, node.Path)
			}
			// We set true here because paths coming from cells aren't templated. e.g. personal/user/file, not persoanl-files/file
			req.PathsResolved = true
		}

		// Generate a unique request ID
		requestID := generateRequestID(req)

		// Check if identical request is already being processed
		if _, exists := activeRequests.LoadOrStore(requestID, true); exists {
			http.Error(w, "identical request already being processed", http.StatusConflict)
			return
		}
		defer activeRequests.Delete(requestID)

		logger.Debug(fmt.Sprintf("Processing request with ID: %s", requestID))
		if err := svc.RunArgs(r.Context(), &req); err != nil {
			logger.Error(fmt.Sprintf("Preserve error: %v", err))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
	return recoveryMiddleware(handler)
}

// generateRequestID creates a unique identifier for a request based on its contents
func generateRequestID(req ServiceArgs) string {
	// Create a simple hash based on username and path combination
	// Might want to use a more robust hashing method
	id := req.CellsUsername
	for _, path := range req.CellsPaths {
		id += ":" + path
	}
	for _, node := range req.CellsNodes {
		id += ":" + node.Path
	}
	return id
}

// Serve starts the HTTP server for the preservation service.
func Serve(svc *Service, addr string) error {
	http.HandleFunc("/preserve", Handler(svc))
	logger.Info(fmt.Sprintf("Server listening on %s", addr))

	// Create server with proper timeouts to address gosec G114
	server := &http.Server{
		Addr:         addr,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return server.ListenAndServe()
}
