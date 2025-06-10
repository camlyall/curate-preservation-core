// Package processor provides functions to preprocess a package for preservation submission.
// It handles moving files, extracting ZIP archives, and generating PREMIS metadata.
package processor

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/penwern/curate-preservation-core/pkg/premis"
	"github.com/penwern/curate-preservation-core/pkg/utils"
	"github.com/pydio/cells-sdk-go/v4/models"

	"github.com/penwern/curate-preservation-core/pkg/version"
)

// PreprocessPackage prepares a package for preservation submission and returns the path to the preprocessed package path.
// It MOVES the package to a new directory and extracts it if it's a ZIP file.
// It also creates the metadata and premis files.
// NodesCollection is the collection of cells nodes for the package, using the cells SDK.
func PreprocessPackage(ctx context.Context, packagePath, preprocessingDir string, nodesCollection *models.RestNodesCollection, userData *models.IdmUser, organization string) (string, error) {

	packageName := filepath.Base(strings.TrimSuffix(packagePath, filepath.Ext(packagePath)))

	// Create transfer package directory
	transferDir := filepath.Join(preprocessingDir, packageName)
	if err := utils.CreateDir(transferDir); err != nil {
		return "", fmt.Errorf("error creating transfer directory: %w", err)
	}

	// Create data subdirectory
	dataDir := filepath.Join(transferDir, "data")
	if err := utils.CreateDir(dataDir); err != nil {
		return "", fmt.Errorf("error creating data directory: %w", err)
	}

	// Get file info
	fileInfo, err := os.Stat(packagePath)
	if err != nil {
		return "", fmt.Errorf("error checking path: %w", err)
	}

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	// TODO: Support other file types - e.g. tar, gzip, etc.
	if fileInfo.Mode().IsRegular() && utils.IsZipFile(packagePath) {
		// If it's a ZIP file, extract it
		if _, err := utils.ExtractZip(ctx, packagePath, filepath.Join(dataDir, packageName)); err != nil {
			return "", fmt.Errorf("error extracting zip: %w", err)
		}
	} else if fileInfo.Mode().IsRegular() {
		// If it's a regular file, move it
		if err := os.Rename(packagePath, filepath.Join(dataDir, filepath.Base(packagePath))); err != nil {
			return "", fmt.Errorf("error moving file: %w", err)
		}
	} else if fileInfo.IsDir() {
		// If it's a directory, move it
		if err := os.Rename(packagePath, filepath.Join(dataDir, filepath.Base(packagePath))); err != nil {
			return "", fmt.Errorf("error moving directory: %w", err)
		}
	} else {
		return "", fmt.Errorf("file type not supported: %s", packagePath)
	}

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	// Create metadata subdirectory
	metadataDir := filepath.Join(transferDir, "metadata")
	if err = utils.CreateDir(metadataDir); err != nil {
		return "", fmt.Errorf("error creating metadata directory: %w", err)
	}

	// Construct Metadata
	premisObj, metadataArray, err := constructMetadataFromNodesCollection(nodesCollection, userData, organization)
	if err != nil {
		return "", fmt.Errorf("error constructing PREMIS XML: %w", err)
	}

	if len(premisObj.Objects) != 0 {
		// Validate PREMIS XML
		if err = premis.ValidatePremis(premisObj); err != nil {
			return "", fmt.Errorf("error validating PREMIS XML: %w", err)
		}
		if err = premis.WritePremis(premisObj, filepath.Join(metadataDir, "premis.xml")); err != nil {
			return "", fmt.Errorf("error writing PREMIS XML: %w", err)
		}
	}

	// Write metadata JSON if its not empty
	if len(metadataArray) > 0 {
		metadataJSON, err := json.Marshal(metadataArray)
		if err != nil {
			return "", fmt.Errorf("error marshaling metadata JSON array: %w", err)
		}
		if err = os.WriteFile(filepath.Join(metadataDir, "metadata.json"), metadataJSON, 0600); err != nil {
			return "", fmt.Errorf("error writing metadata JSON: %w", err)
		}
	}

	return transferDir, nil
}

// Constructs the PREMIS XML from the nodes in the package
// This function is a bit janky as it contructs Premis, Dublin Core and ISAD(G) metadata to avoid looping through the nodes repeatedly
func constructMetadataFromNodesCollection(nodesCollection *models.RestNodesCollection, userData *models.IdmUser, organization string) (premis.Premis, []map[string]any, error) {

	// Initialize the PREMIS XML
	premisRoot := premis.Premis{
		XMLNS:   "http://www.loc.gov/premis/v3",
		XSI:     "http://www.w3.org/2001/XMLSchema-instance",
		Version: "3.0",
		Schema:  "http://www.loc.gov/premis/v3 https://www.loc.gov/standards/premis/premis.xsd",
	}
	premisAgents := []premis.Agent{
		{
			AgentIdentifier: premis.AgentIdentifier{
				IdentifierType:  "Preservation System",
				IdentifierValue: version.Identifier(),
			},
			AgentType: "Software",
			AgentName: "Curate Preservation System",
		},
		{
			AgentIdentifier: premis.AgentIdentifier{
				IdentifierType:  "Cells User UUID",
				IdentifierValue: userData.UUID,
			},
			AgentType: "Curate User",
			AgentName: fmt.Sprintf("Login=%s, GroupPath=%s", userData.Login, userData.GroupPath)},
	}

	// If the premis organization is not empty, add it to the PREMIS agents
	if organization != "" {
		premisAgents = append(premisAgents, premis.Agent{
			AgentIdentifier: premis.AgentIdentifier{
				IdentifierType:  "Organization Name",
				IdentifierValue: organization,
			},
			AgentType: "Organization",
			AgentName: organization,
		})
	}

	// Initialize the Metadata Json Array (Dublin Core and ISAD(G))
	metadataArray := make([]map[string]any, 0)

	nodePrefix := filepath.Dir(nodesCollection.Parent.Path)
	// For each node in the package
	for _, node := range append(nodesCollection.Children, nodesCollection.Parent) {

		objectPath := strings.Replace(node.Path, nodePrefix, "objects/data", 1)

		// Create the PREMIS object
		premisObject, premisEvents, err := constructPremisObjectsFromNode(premisAgents, node, objectPath)
		if err != nil {
			return premis.Premis{}, []map[string]any{}, fmt.Errorf("error constructing PREMIS object: %w", err)
		}
		if premisEvents != nil {
			// Append PREMIS object to PREMIS XML
			premisRoot.Objects = append(premisRoot.Objects, premisObject)
			// Append PREMIS events to PREMIS XML
			premisRoot.Events = append(premisRoot.Events, premisEvents...)
		}

		// Create this node's metadata JSON
		metadataMap := constructMetadataJSONFromNode(node, objectPath)
		// If the metadata JSON is not empty, append it to the array
		if metadataMap != nil {
			metadataArray = append(metadataArray, metadataMap)
		}
	}
	// Append PREMIS agents to PREMIS XML
	if len(premisRoot.Events) != 0 {
		premisRoot.Agents = append(premisRoot.Agents, premisAgents...)
	}

	return premisRoot, metadataArray, nil
}

func constructPremisObjectsFromNode(premisAgents []premis.Agent, node *models.TreeNode, objectPath string) (premis.Object, []premis.Event, error) {
	// Create the PREMIS object
	premisObject := premis.Object{
		XSIType: "premis:file",
		ObjectIdentifier: premis.ObjectIdentifier{
			IdentifierType:  "UUID",
			IdentifierValue: node.UUID,
		},
		ObjectCharacteristics: premis.ObjectCharacteristics{
			Format: premis.Format{
				FormatDesignation: premis.FormatDesignation{
					FormatName: strings.Trim(node.MetaStore["mime"], "\""),
				},
			},
		},
		OriginalName: objectPath,
	}

	// Get the json PREMIS events from the cells PREMIS metadata
	nodePremisMetaStore := node.MetaStore["premis"]
	jsonPremisEvents := make([]map[string]any, 0)
	if nodePremisMetaStore != "" {
		if err := json.Unmarshal([]byte(nodePremisMetaStore), &jsonPremisEvents); err != nil {
			return premis.Object{}, nil, fmt.Errorf("error unmarshalling premis json array: %v", err)
		}
	}

	// Get the json PREMIS events from the cells PREMIS metadata
	// TODO: Remove this once "usermeta-premis-data" is phased out
	nodePremisMetaStoreOther := node.MetaStore["usermeta-premis-data"]
	var jsonPremisEventsOther []map[string]any
	if nodePremisMetaStoreOther != "" {
		if err := json.Unmarshal([]byte(nodePremisMetaStoreOther), &jsonPremisEventsOther); err != nil {
			return premis.Object{}, nil, fmt.Errorf("error unmarshalling premis json array: %v", err)
		}
	}

	// Combine the json PREMIS events
	jsonPremisEvents = append(jsonPremisEvents, jsonPremisEventsOther...)

	// If there are no PREMIS events, return empty object and events
	if len(jsonPremisEvents) == 0 {
		return premis.Object{}, nil, nil
	}

	// Create the PREMIS events
	premisEvents := make([]premis.Event, len(jsonPremisEvents))
	// For each event in the json array
	for i, premisEventJSON := range jsonPremisEvents {
		// Create the PREMIS event
		eventIdentifier, ok := premisEventJSON["event_identifier"].(map[string]any)
		if !ok {
			return premis.Object{}, nil, fmt.Errorf("invalid event_identifier format")
		}

		identifierType, ok := eventIdentifier["event_identifier_type"].(string)
		if !ok {
			return premis.Object{}, nil, fmt.Errorf("invalid event_identifier_type format")
		}

		identifierValue, ok := eventIdentifier["event_identifier_value"].(string)
		if !ok {
			return premis.Object{}, nil, fmt.Errorf("invalid event_identifier_value format")
		}

		eventType, ok := premisEventJSON["event_type"].(string)
		if !ok {
			return premis.Object{}, nil, fmt.Errorf("invalid event_type format")
		}

		eventDateTime, ok := premisEventJSON["event_date_time"].(string)
		if !ok {
			return premis.Object{}, nil, fmt.Errorf("invalid event_date_time format")
		}

		eventDetailInfo, ok := premisEventJSON["event_detail_information"].(map[string]any)
		if !ok {
			return premis.Object{}, nil, fmt.Errorf("invalid event_detail_information format")
		}

		eventDetail, ok := eventDetailInfo["event_detail"].(string)
		if !ok {
			return premis.Object{}, nil, fmt.Errorf("invalid event_detail format")
		}

		eventOutcomeInfo, ok := premisEventJSON["event_outcome_information"].(map[string]any)
		if !ok {
			return premis.Object{}, nil, fmt.Errorf("invalid event_outcome_information format")
		}

		eventOutcome, ok := eventOutcomeInfo["event_outcome"].(string)
		if !ok {
			return premis.Object{}, nil, fmt.Errorf("invalid event_outcome format")
		}

		eventOutcomeDetail, ok := eventOutcomeInfo["event_outcome_detail"].(map[string]any)
		if !ok {
			return premis.Object{}, nil, fmt.Errorf("invalid event_outcome_detail format")
		}

		eventOutcomeDetailNote, ok := eventOutcomeDetail["event_outcome_detail_note"].(string)
		if !ok {
			return premis.Object{}, nil, fmt.Errorf("invalid event_outcome_detail_note format")
		}

		premisEvent := premis.Event{
			EventIdentifier: premis.EventIdentifier{
				IdentifierType:  identifierType,
				IdentifierValue: identifierValue,
			},
			EventType:     eventType,
			EventDateTime: eventDateTime,
			EventDetailInformation: premis.EventDetailInformation{
				EventDetail: eventDetail,
			},
			EventOutcomeInformation: premis.EventOutcomeInformation{
				EventOutcome: eventOutcome,
				EventOutcomeDetail: premis.EventOutcomeDetail{
					EventOutcomeDetailNote: eventOutcomeDetailNote,
				},
			},
		}
		// Append linking agent identifier to event for every agent
		for _, premisAgent := range premisAgents {
			premisEvent.LinkingAgentIdentifiers = append(premisEvent.LinkingAgentIdentifiers, premis.LinkingAgentIdentifier(premisAgent.AgentIdentifier))
		}

		// Append linking event identifier to object
		premisObject.LinkingEventIdentifiers = append(premisObject.LinkingEventIdentifiers, premis.LinkingEventIdentifier(premisEvent.EventIdentifier))
		// Append event to events
		premisEvents[i] = premisEvent
	}
	return premisObject, premisEvents, nil
}

var metadataMap = map[string]string{
	"usermeta-dc-title":                   "dc.title",
	"usermeta-dc-creator":                 "dc.creator",
	"usermeta-dc-subject":                 "dc.subject",
	"usermeta-dc-description":             "dc.description",
	"usermeta-dc-publisher":               "dc.publisher",
	"usermeta-dc-contributor":             "dc.contributor",
	"usermeta-dc-date":                    "dc.date",
	"usermeta-dc-type":                    "dc.type",
	"usermeta-dc-format":                  "dc.format",
	"usermeta-dc-identifier":              "dc.identifier",
	"usermeta-dc-source":                  "dc.source",
	"usermeta-dc-language":                "dc.language",
	"usermeta-dc-relation":                "dc.relation",
	"usermeta-dc-coverage":                "dc.coverage",
	"usermeta-dc-rights":                  "dc.rights",
	"usermeta-isadg-title":                "isadg.title",
	"usermeta-isadg-date":                 "isadg.date",
	"usermeta-isadg-level-of-description": "isadg.level-of-description",
	"usermeta-isadg-extent-and-medium-of-the-unit-of-description":        "isadg.extent-and-medium-of-the-unit-of-description",
	"usermeta-isadg-alternative-identifiers":                             "isadg.alternative-identifiers", // ICL Custom Field
	"usermeta-isadg-name-of-creators":                                    "isadg.name-of-creators",
	"usermeta-isadg-administrativebiographical-history":                  "isadg.administrativebiographical-history",
	"usermeta-isadg-archival-history":                                    "isadg.archival-history",
	"usermeta-isadg-immediate-source-of-acquisition-or-transfer":         "isadg.immediate-source-of-acquisition-or-transfer",
	"usermeta-isadg-scope-and-content":                                   "isadg.scope-and-content",
	"usermeta-isadg-appraisal-destruction-and-scheduling-information":    "isadg.appraisal-destruction-and-scheduling-information",
	"usermeta-isadg-accruals":                                            "isadg.accruals",
	"usermeta-isadg-system-of-arrangement":                               "isadg.system-of-arrangement",
	"usermeta-isadg-conditions-governing-access":                         "isadg.conditions-governing-access",
	"usermeta-isadg-conditions-governing-reproduction":                   "isadg.conditions-governing-reproduction",
	"usermeta-isadg-languagescripts-of-material":                         "isadg.languagescripts-of-material",
	"usermeta-isadg-physical-characteristics-and-technical-requirements": "isadg.physical-characteristics-and-technical-requirements",
	"usermeta-isadg-finding-aids":                                        "isadg.finding-aids",
	"usermeta-isadg-existence-and-location-of-originals":                 "isadg.existence-and-location-of-originals",
	"usermeta-isadg-existence-and-location-of-copies":                    "isadg.existence-and-location-of-copies",
	"usermeta-isadg-related-units-of-description":                        "isadg.related-units-of-description",
	"usermeta-isadg-publication-note":                                    "isadg.publication-note",
	"usermeta-isadg-note":                                                "isadg.note",
	"usermeta-isadg-archivists-note":                                     "isadg.archivists-note",
	"usermeta-isadg-rules-or-conventions":                                "isadg.rules-or-conventions",
	"usermeta-isadg-dates-of-descriptions":                               "isadg.dates-of-descriptions",
}

// constructMetadataJSONFromNode constructs the DC and ISADG metadata JSON from the node
func constructMetadataJSONFromNode(node *models.TreeNode, objectPath string) map[string]any {
	metadata := make(map[string]any)
	for cellsKey, metaKey := range metadataMap {
		if value, exists := node.MetaStore[cellsKey]; exists && value != "" {
			metadata[metaKey] = value
		}
	}
	if len(metadata) == 0 {
		return nil
	}
	metadata["filename"] = objectPath
	return metadata
}
