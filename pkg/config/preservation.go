package config

import transferservice "github.com/penwern/preservation-go/common/proto/a3m/gen/go/a3m/api/transferservice/v1beta1"

// User stored variables
type PreservationConfig struct {
	// ID                     uint16 // Unused
	// Name                   string // Unused
	// Description            string // Unused
	// ProcessType            string // eark or standard
	// ImageNormalizationTiff bool // Unused yet?
	CompressAip bool
	A3mConfig   transferservice.ProcessingConfig
}
