package config

import (
	"reflect"

	transferservice "github.com/penwern/curate-preservation-core/common/proto/a3m/gen/go/a3m/api/transferservice/v1beta1"
)

// User stored variables
type PreservationConfig struct {
	// ProcessType            string 	// eark or standard
	// ImageNormalizationTiff bool 		// Unused yet?
	// TODO: Change this to AIP Compression Algo and Level (with algo option None)
	CompressAip bool                              `json:"compress_aip" comment:"Compress AIP"`
	A3mConfig   *transferservice.ProcessingConfig `json:"a3m_config" comment:"A3M processing configuration"`
	AtomConfig  *AtomConfig                       `json:"atom_config" comment:"AtoM configuration"`
}

// DefaultPreservationConfig returns a default configuration for the preservation service.
func DefaultPreservationConfig() PreservationConfig {
	return PreservationConfig{
		CompressAip: false,
		A3mConfig:   defaultA3mConfig(),
		AtomConfig:  defaultAtomConfig(),
	}
}

// defaultA3mConfig returns a default configuration for the A3M service.
func defaultA3mConfig() *transferservice.ProcessingConfig {
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
		PerformPolicyChecksOnAccessDerivatives:       true,
		ThumbnailMode:                                transferservice.ProcessingConfig_THUMBNAIL_MODE_GENERATE,
		AipCompressionLevel:                          1,
		AipCompressionAlgorithm:                      transferservice.ProcessingConfig_AIP_COMPRESSION_ALGORITHM_S7_COPY,
	}
}

// MergeWithDefaults takes a partial config and returns a new config with defaults applied
func (cfg *PreservationConfig) MergeWithDefaults() PreservationConfig {
	defaults := DefaultPreservationConfig()

	// If input config is nil, return defaults
	if cfg == nil {
		return defaults
	}

	result := defaults

	// Handle top level fields
	result.CompressAip = cfg.CompressAip || defaults.CompressAip

	// Handle A3M config
	if cfg.A3mConfig != nil {
		// Use reflection to merge non-zero values
		inputVal := reflect.ValueOf(cfg.A3mConfig).Elem()
		defaultVal := reflect.ValueOf(result.A3mConfig).Elem()

		for i := range inputVal.NumField() {
			field := inputVal.Field(i)
			// For booleans, OR with default
			if field.Kind() == reflect.Bool {
				if field.Bool() {
					defaultVal.Field(i).SetBool(true)
				}
				continue
			}
			// For other types, override if non-zero
			if !field.IsZero() {
				defaultVal.Field(i).Set(field)
			}
		}
	}

	// Handle AtoM config
	if cfg.AtomConfig != nil {
		// Use reflection to merge non-empty strings
		inputVal := reflect.ValueOf(cfg.AtomConfig).Elem()
		defaultVal := reflect.ValueOf(result.AtomConfig).Elem()

		for i := range inputVal.NumField() {
			field := inputVal.Field(i)
			if field.Kind() == reflect.String && field.String() != "" {
				defaultVal.Field(i).SetString(field.String())
			}
		}
	}

	return result
}
