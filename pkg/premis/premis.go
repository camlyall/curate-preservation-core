// Package premis provides structures and functions for handling PREMIS metadata.
// It includes definitions for PREMIS objects, events, agents, and methods to marshal, write, and validate PREMIS records.
package premis

import (
	_ "embed"
	"encoding/xml"
	"fmt"
	"os"

	"github.com/lestrrat-go/libxml2"
	"github.com/lestrrat-go/libxml2/xsd"
	"github.com/penwern/curate-preservation-core/pkg/logger"
)

// Embed the premis.xsd file.
//
//go:embed premis.xsd
var premisSchema []byte

// Premis is the root element for a PREMIS record.
type Premis struct {
	XMLName xml.Name `xml:"premis:premis"`
	XMLNS   string   `xml:"xmlns:premis,attr"`
	XSI     string   `xml:"xmlns:xsi,attr"`
	Version string   `xml:"version,attr"`
	Schema  string   `xml:"xsi:schemaLocation,attr"`

	Objects []Object `xml:"premis:object"`
	Events  []Event  `xml:"premis:event"`
	Agents  []Agent  `xml:"premis:agent"`
}

// Object represents a digital object.
type Object struct {
	XSIType                 string                   `xml:"xsi:type,attr"`
	ObjectIdentifier        ObjectIdentifier         `xml:"premis:objectIdentifier"`
	ObjectCharacteristics   ObjectCharacteristics    `xml:"premis:objectCharacteristics"`
	OriginalName            string                   `xml:"premis:originalName"`
	LinkingEventIdentifiers []LinkingEventIdentifier `xml:"premis:linkingEventIdentifier"`
}

// ObjectIdentifier uniquely identifies an object.
type ObjectIdentifier struct {
	IdentifierType  string `xml:"premis:objectIdentifierType"`
	IdentifierValue string `xml:"premis:objectIdentifierValue"`
}

// ObjectCharacteristics holds technical and descriptive details.
type ObjectCharacteristics struct {
	Format Format `xml:"premis:format"`
}

// FormatDesignation ...
type FormatDesignation struct {
	FormatName    string `xml:"premis:formatName"`
	FormatVersion string `xml:"premis:formatVersion,omitempty"` // TODO: Is this optional?
}

// Format ...
type Format struct {
	FormatDesignation FormatDesignation `xml:"premis:formatDesignation,omitempty"`
}

// LinkingEventIdentifier links an event to an object.
type LinkingEventIdentifier struct {
	IdentifierType  string `xml:"premis:linkingEventIdentifierType"`
	IdentifierValue string `xml:"premis:linkingEventIdentifierValue"`
}

// Event represents an action or change affecting one or more objects.
type Event struct {
	EventIdentifier          EventIdentifier           `xml:"premis:eventIdentifier"`
	EventType                string                    `xml:"premis:eventType"`
	EventDateTime            string                    `xml:"premis:eventDateTime"`
	EventDetailInformation   EventDetailInformation    `xml:"premis:eventDetailInformation"`
	EventOutcomeInformation  EventOutcomeInformation   `xml:"premis:eventOutcomeInformation"`
	LinkingAgentIdentifiers  []LinkingAgentIdentifier  `xml:"premis:linkingAgentIdentifier"`
	LinkingObjectIdentifiers []LinkingObjectIdentifier `xml:"premis:linkingObjectIdentifier,omitempty"`
}

// EventIdentifier uniquely identifies an event.
type EventIdentifier struct {
	IdentifierType  string `xml:"premis:eventIdentifierType"`
	IdentifierValue string `xml:"premis:eventIdentifierValue"`
}

// EventDetailInformation contains details about the event.
type EventDetailInformation struct {
	EventDetail string `xml:"premis:eventDetail"`
}

// EventOutcomeInformation contains details about the result of an event.
type EventOutcomeInformation struct {
	EventOutcome       string             `xml:"premis:eventOutcome"`
	EventOutcomeDetail EventOutcomeDetail `xml:"premis:eventOutcomeDetail"`
}

// EventOutcomeDetail ...
type EventOutcomeDetail struct {
	EventOutcomeDetailNote string `xml:"premis:eventOutcomeDetailNote"`
}

// LinkingAgentIdentifier links an agent to an event.
type LinkingAgentIdentifier struct {
	IdentifierType  string `xml:"premis:linkingAgentIdentifierType"`
	IdentifierValue string `xml:"premis:linkingAgentIdentifierValue"`
}

// LinkingObjectIdentifier links an object to an event.
type LinkingObjectIdentifier struct {
	ObjectIdentifierType  string `xml:"premis:linkingObjectIdentifierType"`
	ObjectIdentifierValue string `xml:"premis:linkingObjectIdentifierValue"`
}

// Agent represents an entity (person, organization, or software) responsible for events.
type Agent struct {
	AgentIdentifier AgentIdentifier `xml:"premis:agentIdentifier"`
	AgentName       string          `xml:"premis:agentName"`
	AgentType       string          `xml:"premis:agentType"`
}

// AgentIdentifier uniquely identifies an agent.
type AgentIdentifier struct {
	IdentifierType  string `xml:"premis:agentIdentifierType"`
	IdentifierValue string `xml:"premis:agentIdentifierValue"`
}

// GetPremis returns a sample PREMIS record.
func GetPremis() Premis {
	return Premis{
		XMLNS:   "http://www.loc.gov/premis/v3",
		XSI:     "http://www.w3.org/2001/XMLSchema-instance",
		Version: "3.0",
		Schema:  "http://www.loc.gov/premis/v3 https://www.loc.gov/standards/premis/premis.xsd",
		Objects: []Object{
			{
				XSIType: "premis:file",
				ObjectIdentifier: ObjectIdentifier{
					IdentifierType:  "UUID",
					IdentifierValue: "object-123",
				},
				ObjectCharacteristics: ObjectCharacteristics{
					Format: Format{
						FormatDesignation: FormatDesignation{
							FormatName: "PDF",
						},
					},
				},
				OriginalName: "objects/data/ingest/sample.pdf",
				LinkingEventIdentifiers: []LinkingEventIdentifier{
					{
						IdentifierType:  "UUID",
						IdentifierValue: "event-456",
					},
					{
						IdentifierType:  "UUID",
						IdentifierValue: "event-789",
					},
				},
			},
		},
		Events: []Event{
			{
				EventIdentifier: EventIdentifier{
					IdentifierType:  "UUID",
					IdentifierValue: "event-456",
				},
				EventType:     "ingestion",
				EventDateTime: "2025-03-11T12:34:56Z",
				EventDetailInformation: EventDetailInformation{
					EventDetail: "Object ingested successfully.",
				},
				EventOutcomeInformation: EventOutcomeInformation{
					EventOutcome: "success",
					EventOutcomeDetail: EventOutcomeDetail{
						EventOutcomeDetailNote: "Object ingested successfully.",
					},
				},
				LinkingAgentIdentifiers: []LinkingAgentIdentifier{
					{
						IdentifierType:  "UUID",
						IdentifierValue: "agent-789",
					},
					{
						IdentifierType:  "Name",
						IdentifierValue: "agent name",
					},
				},
				LinkingObjectIdentifiers: []LinkingObjectIdentifier{
					{
						ObjectIdentifierType:  "UUID",
						ObjectIdentifierValue: "object-123",
					},
					{
						ObjectIdentifierType:  "name",
						ObjectIdentifierValue: "object anem",
					},
				},
			},
			{
				EventIdentifier: EventIdentifier{
					IdentifierType:  "UUID",
					IdentifierValue: "event-789",
				},
				EventType:     "ingestion",
				EventDateTime: "2025-03-11T12:34:56Z",
				EventDetailInformation: EventDetailInformation{
					EventDetail: "Object ingested successfully.",
				},
				EventOutcomeInformation: EventOutcomeInformation{
					EventOutcome: "success",
					EventOutcomeDetail: EventOutcomeDetail{
						EventOutcomeDetailNote: "Object ingested successfully.",
					},
				},
				LinkingAgentIdentifiers: []LinkingAgentIdentifier{
					{
						IdentifierType:  "UUID",
						IdentifierValue: "agent-789",
					},
					{
						IdentifierType:  "Name",
						IdentifierValue: "agent name",
					},
				},
			},
		},
		Agents: []Agent{
			{
				AgentIdentifier: AgentIdentifier{
					IdentifierType:  "UUID",
					IdentifierValue: "agent-789",
				},
				AgentName: "Archivist",
				AgentType: "human",
			},
			{
				AgentIdentifier: AgentIdentifier{
					IdentifierType:  "name",
					IdentifierValue: "agent-name",
				},
				AgentName: "Archivist",
				AgentType: "human",
			},
		},
	}
}

// ToString marshals the PREMIS record to XML and returns it as a string.
func ToString(premisRecord Premis) (string, error) {
	xmlData, err := xml.MarshalIndent(premisRecord, "", "  ")
	if err != nil {
		return "", fmt.Errorf("error marshaling XML: %v", err)
	}
	logger.Debug(xml.Header + string(xmlData))
	return string(xmlData), nil
}

// WritePremis writes the PREMIS record to a file.
func WritePremis(premisRecord Premis, filePath string) error {
	// Marshal the PREMIS record to XML
	xmlData, err := xml.MarshalIndent(premisRecord, "", "    ")
	if err != nil {
		return fmt.Errorf("error marshaling XML: %w", err)
	}
	// Add XML header
	xmlData = append([]byte(xml.Header), xmlData...)
	// Write the XML to the file
	err = os.WriteFile(filePath, xmlData, 0600)
	if err != nil {
		return fmt.Errorf("error writing PREMIS XML to file: %w", err)
	}
	logger.Debug("PREMIS XML written to file: %s", filePath)
	return nil
}

// ValidatePremis validates the PREMIS metadata against the schema.
func ValidatePremis(premisRecord Premis) error {
	// Marshal the PREMIS record to XML
	xmlData, err := xml.Marshal(premisRecord)
	if err != nil {
		return fmt.Errorf("error marshaling PREMIS record: %w", err)
	}

	// Parse the XML schema from the embedded data
	schema, err := xsd.Parse(premisSchema)
	if err != nil {
		return fmt.Errorf("error parsing embedded XML schema: %w", err)
	}
	defer schema.Free()

	// Parse the XML
	doc, err := libxml2.Parse(xmlData)
	if err != nil {
		return fmt.Errorf("error parsing XML document: %w", err)
	}
	defer doc.Free()

	// Validate the XML document against the schema
	if err := schema.Validate(doc); err != nil {
		return fmt.Errorf("error validating XML document against schema: %w", err)
	}

	return nil
}
