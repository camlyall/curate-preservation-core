package internal

import (
	"encoding/json"
	"net/http"
	"sync"

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
		// Decode request args
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if req.Username == "" {
			logger.Error("received request with no username")
			http.Error(w, "no username provided", http.StatusBadRequest)
			return
		}

		// If theres no paths, we're going to look for nodes from Cells
		req.PathsResolved = false
		if len(req.Paths) == 0 {
			if len(req.Nodes) == 0 {
				logger.Error("received request with no paths or nodes")
				http.Error(w, "no paths or nodes provided", http.StatusBadRequest)
				return
			}
			for _, node := range req.Nodes {
				req.Paths = append(req.Paths, node.Path)
			}
			// We set true here becauce paths coming from cells aren't templated. e.g. personal/user/file, not persoanl-files/file
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
	// In a production system, you might want to use a more robust hashing method
	id := req.Username
	for _, path := range req.Paths {
		id += ":" + path
	}
	return id
}

func Serve(svc *Service, addr string) error {
	http.HandleFunc("/preserve", Handler(svc))
	logger.Info("Listening on %s", addr)
	return http.ListenAndServe(addr, nil)
}
