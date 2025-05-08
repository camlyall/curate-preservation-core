package internal

import (
	"encoding/json"
	"net/http"
	"sync"

	transferservice "github.com/penwern/preservation-go/common/proto/a3m/gen/go/a3m/api/transferservice/v1beta1"
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

		var processingCfg config.PreservationConfig
		if req.PreservationCfg != nil {
			processingCfg = config.PreservationConfig{
				CompressAip: req.PreservationCfg.CompressAip,
				A3mConfig: &transferservice.ProcessingConfig{
					AssignUuidsToDirectories:                     req.PreservationCfg.A3mConfig.AssignUuidsToDirectories,
					ExamineContents:                              req.PreservationCfg.A3mConfig.ExamineContents,
					GenerateTransferStructureReport:              req.PreservationCfg.A3mConfig.GenerateTransferStructureReport,
					DocumentEmptyDirectories:                     req.PreservationCfg.A3mConfig.DocumentEmptyDirectories,
					ExtractPackages:                              req.PreservationCfg.A3mConfig.ExtractPackages,
					DeletePackagesAfterExtraction:                req.PreservationCfg.A3mConfig.DeletePackagesAfterExtraction,
					IdentifyTransfer:                             req.PreservationCfg.A3mConfig.IdentifyTransfer,
					IdentifySubmissionAndMetadata:                req.PreservationCfg.A3mConfig.IdentifySubmissionAndMetadata,
					IdentifyBeforeNormalization:                  req.PreservationCfg.A3mConfig.IdentifyBeforeNormalization,
					Normalize:                                    req.PreservationCfg.A3mConfig.Normalize,
					TranscribeFiles:                              req.PreservationCfg.A3mConfig.TranscribeFiles,
					PerformPolicyChecksOnOriginals:               req.PreservationCfg.A3mConfig.PerformPolicyChecksOnOriginals,
					PerformPolicyChecksOnPreservationDerivatives: req.PreservationCfg.A3mConfig.PerformPolicyChecksOnPreservationDerivatives,
					// Ignored as not seen by user
					// AipCompressionLevel:                          req.PreservationCfg.A3mConfig.AipCompressionLevel,
					// AipCompressionAlgorithm:                      req.PreservationCfg.A3mConfig.AipCompressionAlgorithm,
					AipCompressionLevel:     1,
					AipCompressionAlgorithm: transferservice.ProcessingConfig_AIP_COMPRESSION_ALGORITHM_S7_COPY,
				},
				AtomConfig: &config.AtomConfig{
					Host:          req.PreservationCfg.AtomConfig.Host,
					ApiKey:        req.PreservationCfg.AtomConfig.ApiKey,
					LoginEmail:    req.PreservationCfg.AtomConfig.LoginEmail,
					LoginPassword: req.PreservationCfg.AtomConfig.LoginPassword,
					RsyncTarget:   req.PreservationCfg.AtomConfig.RsyncTarget,
					RsyncCommand:  req.PreservationCfg.AtomConfig.RsyncCommand,
					Slug:          req.PreservationCfg.AtomConfig.Slug,
				},
			}
		} else {
			processingCfg = config.DefaultPreservationConfig()
		}

		logger.Debug("Request ID: %s", requestID)
		if err := svc.RunArgs(r.Context(), &req, &processingCfg); err != nil {
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
