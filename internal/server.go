package internal

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/penwern/preservation-go/pkg/config"
	"github.com/penwern/preservation-go/pkg/logger"
)

// Global map to track active requests
var (
	activeRequests sync.Map
)

func Handler(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		// Defaults
		req := ServiceArgs{
			Cleanup: true,
		}

		logger.Debug("Received request: %+v", r)

		// Decode request args
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
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
			logger.Error("received request with no username")
			http.Error(w, "no username provided", http.StatusBadRequest)
			return
		}

		// If theres no paths, we're going to look for nodes from Cells
		req.PathsResolved = false
		if len(req.CellsPaths) == 0 {
			if len(req.CellsNodes) == 0 {
				logger.Error("received request with no paths or nodes")
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

		logger.Debug("Request ID: %s", requestID)
		if err := svc.RunArgs(r.Context(), &req); err != nil {
			logger.Error("preserve error: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}

// generateRequestID creates a unique identifier for a request based on its contents
func generateRequestID(req ServiceArgs) string {
	// Create a simple hash based on username and path combination
	// Might want to use a more robust hashing method
	id := req.CellsUsername
	for _, path := range req.CellsPaths {
		id += ":" + path
	}
	return id
}

func Serve(svc *Service, addr string) error {
	http.HandleFunc("/preserve", Handler(svc))
	logger.Info("Listening on %s", addr)
	return http.ListenAndServe(addr, nil)
}
