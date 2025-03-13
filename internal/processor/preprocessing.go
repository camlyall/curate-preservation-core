// Preprocessor for preparing packages downloaded from Cells for submission to A3M

package processor

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/penwern/preservation-go/internal/cells"
	"github.com/penwern/preservation-go/pkg/premis"
	"github.com/penwern/preservation-go/pkg/utils"
)

// PreprocessPackage prepares a package for preservation submission and returns the path to the preprocessed package path.
func PreprocessPackage(ctx context.Context, packagePath, preprocessingDir string, nodeCollection cells.NodeCollection, userData cells.UserData) (string, error) {

	packageName := filepath.Base(strings.TrimSuffix(packagePath, filepath.Ext(packagePath)))

	// Create transfer package directory
	transferDir := filepath.Join(preprocessingDir, packageName)
	if err := os.Mkdir(transferDir, 0755); err != nil {
		return "", fmt.Errorf("error creating transfer directory: %w", err)
	}

	// Create data subdirectory
	dataDir := filepath.Join(transferDir, "data")
	if err := os.Mkdir(dataDir, 0755); err != nil {
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

	if fileInfo.Mode().IsRegular() && utils.IsZipFile(packagePath) {
		// If it's a ZIP file, extract it
		if _, err := utils.ExtractZip(packagePath, filepath.Join(dataDir, packageName)); err != nil {
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
	if err = os.Mkdir(metadataDir, 0755); err != nil {
		return "", fmt.Errorf("error creating metadata directory: %w", err)
	}

	combinedNodes := []cells.NodeData{nodeCollection.Parent}
	combinedNodes = append(combinedNodes, nodeCollection.Children...)

	premisXml, err := constructPremisFromNodes(combinedNodes, userData, filepath.Dir(nodeCollection.Parent.Path))
	if err != nil {
		return "", fmt.Errorf("error constructing PREMIS XML: %w", err)
	}

	if err = premis.ValidatePremis(premisXml); err != nil {
		return "", fmt.Errorf("error validating PREMIS XML: %w", err)
	}
	if err = premis.WritePremis(premisXml, filepath.Join(metadataDir, "premis.xml")); err != nil {
		return "", fmt.Errorf("error writing PREMIS XML: %w", err)
	}

	// TODO: Write dc and isadg json metadata

	return transferDir, nil
}

func constructPremisFromNodes(nodes []cells.NodeData, userData cells.UserData, nodePrefix string) (premis.Premis, error) {
	premisXml := premis.Premis{
		XMLNS:   "http://www.loc.gov/premis/v3",
		XSI:     "http://www.w3.org/2001/XMLSchema-instance",
		Version: "3.0",
		Schema:  "http://www.loc.gov/premis/v3 https://www.loc.gov/standards/premis/premis.xsd",
	}

	premisAgents := []premis.Agent{
		{
			AgentIdentifier: premis.AgentIdentifier{
				IdentifierType:  "Organisation Name",
				IdentifierValue: "Penwern Limited",
			},
			AgentType: "Organisation",
			AgentName: "Penwern Limited",
		},
		{
			AgentIdentifier: premis.AgentIdentifier{
				IdentifierType:  "Preservation System",
				IdentifierValue: "Penwern Curate",
			},
			AgentType: "Software",
			AgentName: "Curate",
		},
		{
			AgentIdentifier: premis.AgentIdentifier{
				IdentifierType:  "Curate User UUID",
				IdentifierValue: userData.Uuid,
			},
			AgentType: "Curate User",
			AgentName: fmt.Sprintf("Username=%s, Display Name=%s, Group=%s", userData.Login, userData.Attributes.DisplayName, userData.GroupPath)},
	}

	// For each node in the package
	for _, node := range nodes {
		// Create the PREMIS object
		premisObject := premis.Object{
			XSIType: "premis:file",
			ObjectIdentifier: premis.ObjectIdentifier{
				IdentifierType:  "UUID",
				IdentifierValue: node.Uuid,
			},
			ObjectCharacteristics: premis.ObjectCharacteristics{
				Format: premis.Format{
					FormatDesignation: premis.FormatDesignation{
						FormatName: strings.Trim(node.MetaStore["mime"], "\""),
					},
				},
			},
			OriginalName: strings.Replace(node.Path, nodePrefix, "objects/data", 1),
		}

		// Get the json PREMIS events from the cells PREMIS metadata
		nodePremisMetaStore := node.MetaStore["Premis"]
		var jsonPremisEvents []map[string]any
		if nodePremisMetaStore != "" {
			if err := json.Unmarshal([]byte(nodePremisMetaStore), &jsonPremisEvents); err != nil {
				return premisXml, fmt.Errorf("error unmarshalling premis json array: %v", err)
			}
		}

		// Get the json PREMIS events from the cells PREMIS metadata
		// TODO: Remove this once "usermeta-premis-data" is phased out
		nodePremisMetaStore_Other := node.MetaStore["usermeta-premis-data"]
		var jsonPremisEvents_Other []map[string]any
		if nodePremisMetaStore_Other != "" {
			if err := json.Unmarshal([]byte(nodePremisMetaStore_Other), &jsonPremisEvents_Other); err != nil {
				return premisXml, fmt.Errorf("error unmarshalling premis json array: %v", err)
			}
		}

		// Combine the json PREMIS events
		jsonPremisEvents = append(jsonPremisEvents, jsonPremisEvents_Other...)

		// Create the PREMIS events
		premisEvents := make([]premis.Event, len(jsonPremisEvents))
		// For each event in the json array
		for i, premisEventJson := range jsonPremisEvents {
			// Create the PREMIS event
			premisEvent := premis.Event{
				EventIdentifier: premis.EventIdentifier{
					IdentifierType:  premisEventJson["event_identifier"].(map[string]any)["event_identifier_type"].(string),
					IdentifierValue: premisEventJson["event_identifier"].(map[string]any)["event_identifier_value"].(string),
				},
				EventType:     premisEventJson["event_type"].(string),
				EventDateTime: premisEventJson["event_date_time"].(string),
				EventDetailInformation: premis.EventDetailInformation{
					EventDetail: premisEventJson["event_detail_information"].(map[string]any)["event_detail"].(string),
				},
				EventOutcomeInformation: premis.EventOutcomeInformation{
					EventOutcome: premisEventJson["event_outcome_information"].(map[string]any)["event_outcome"].(string),
					EventOutcomeDetail: premis.EventOutcomeDetail{
						EventOutcomeDetailNote: premisEventJson["event_outcome_information"].(map[string]any)["event_outcome_detail"].(map[string]any)["event_outcome_detail_note"].(string),
					},
				},
			}
			// // Append linking agent identifier to event for every agent
			for _, premisAgent := range premisAgents {
				premisEvent.LinkingAgentIdentifiers = append(premisEvent.LinkingAgentIdentifiers, premis.LinkingAgentIdentifier(premisAgent.AgentIdentifier))
			}

			// Append linking event identifier to object
			premisObject.LinkingEventIdentifiers = append(premisObject.LinkingEventIdentifiers, premis.LinkingEventIdentifier(premisEvent.EventIdentifier))
			// Append event to events
			premisEvents[i] = premisEvent
		}
		// Append PREMIS object to PREMIS XML
		premisXml.Objects = append(premisXml.Objects, premisObject)
		// Append PREMIS events to PREMIS XML
		premisXml.Events = append(premisXml.Events, premisEvents...)
	}
	// Append PREMIS agents to PREMIS XML
	premisXml.Agents = append(premisXml.Agents, premisAgents...)

	return premisXml, nil
}
