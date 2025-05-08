package config

import transferservice "github.com/penwern/preservation-go/common/proto/a3m/gen/go/a3m/api/transferservice/v1beta1"

// User stored variables
type PreservationConfig struct {
	// ProcessType            string // eark or standard
	// ImageNormalizationTiff bool // Unused yet?
	CompressAip bool // TODO: Change this to AIP Compression Algo and Level (with algo option None)
	A3mConfig   *transferservice.ProcessingConfig
	AtomConfig  *AtomConfig
}

// // GetDefault returns a default preservation configuration
func DefaultPreservationConfig() PreservationConfig {
	a3mConfig := DefaultA3mConfig()
	atomConfig := DefaultAtomConfig()
	return PreservationConfig{
		CompressAip: false,
		A3mConfig:   a3mConfig,
		AtomConfig:  atomConfig,
	}
}

func DefaultA3mConfig() *transferservice.ProcessingConfig {
	return &transferservice.ProcessingConfig{
		AssignUuidsToDirectories:                     true,
		ExamineContents:                              false,
		GenerateTransferStructureReport:              true,
		DocumentEmptyDirectories:                     true,
		ExtractPackages:                              true,
		DeletePackagesAfterExtraction:                false,
		IdentifyTransfer:                             true,
		IdentifySubmissionAndMetadata:                true,
		IdentifyBeforeNormalization:                  true,
		Normalize:                                    true,
		TranscribeFiles:                              true,
		PerformPolicyChecksOnOriginals:               true,
		PerformPolicyChecksOnPreservationDerivatives: true,
		AipCompressionLevel:                          1,
		AipCompressionAlgorithm:                      transferservice.ProcessingConfig_AIP_COMPRESSION_ALGORITHM_S7_COPY,
	}
}
