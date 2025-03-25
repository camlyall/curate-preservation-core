// Unused

package metadata

import (
	"encoding/json"
	"fmt"
)

// DublinCore represents the Dublin Core metadata.
type DublinCore struct {
	Title       string `json:"dc.title,omitempty"`
	Creator     string `json:"dc.creator,omitempty"`
	Subject     string `json:"dc.subject,omitempty"`
	Description string `json:"dc.description,omitempty"`
	Publisher   string `json:"dc.publisher,omitempty"`
	Contributor string `json:"dc.contributor,omitempty"`
	Date        string `json:"dc.date,omitempty"`
	Type        string `json:"dc.type,omitempty"`
	Format      string `json:"dc.format,omitempty"`
	Identifier  string `json:"dc.identifier,omitempty"`
	Source      string `json:"dc.source,omitempty"`
	Language    string `json:"dc.language,omitempty"`
	Relation    string `json:"dc.relation,omitempty"`
	Coverage    string `json:"dc.coverage,omitempty"`
	Rights      string `json:"dc.rights,omitempty"`
}

// GetDublinCore creates a sample Dublin Core metadata record.
// func GetDublinCore() DublinCore {
// 	return DublinCore{
// 		Title:       "Sample Title",
// 		Creator:     "Sample Creator",
// 		Subject:     "Sample Subject",
// 		Description: "Sample Description",
// 		Publisher:   "Sample Publisher",
// 		Contributor: "Sample Contributor",
// 		Date:        "2025-03-11",
// 		Type:        "Text",
// 		Format:      "PDF",
// 		Identifier:  "sample-identifier",
// 		Source:      "Sample Source",
// 		Language:    "en",
// 		Relation:    "Sample Relation",
// 		Coverage:    "Sample Coverage",
// 		Rights:      "Sample Rights",
// 	}
// }

// PrintDublinCore prints the Dublin Core metadata to the console.
func (dc DublinCore) AsJson() ([]byte, error) {
	jsonData, err := json.MarshalIndent(dc, "", "  ")
	if err != nil {
		return []byte{}, fmt.Errorf("error marshaling JSON: %w", err)
	}
	return jsonData, nil
}

// WriteDublinCore writes the Dublin Core metadata to a file.
// func (dc DublinCore) Write(filePath string) error {
// 	// Marshal the Dublin Core record to JSON
// 	jsonData, err := json.MarshalIndent(dc, "", "    ")
// 	if err != nil {
// 		return fmt.Errorf("error marshaling JSON: %w", err)
// 	}
// 	// Write the JSON to the file
// 	err = os.WriteFile(filePath, jsonData, 0644)
// 	if err != nil {
// 		return fmt.Errorf("error writing Dublin Core JSON to file: %w", err)
// 	}
// 	return nil
// }
