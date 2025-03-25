// Unused

package metadata

import (
	"encoding/json"
	"fmt"
)

// ISADG represents the General International Standard Archival Description metadata.
// https://en.wikipedia.org/wiki/ISAD(G)
type ISADG struct {
	// Identity Statement
	Title                                 string `json:"isadg:title,omitempty"`
	ReferenceCodes                        string `json:"isadg:reference-codes,omitempty"`
	Date                                  string `json:"isadg:date,omitempty"`
	LevelOfDescription                    string `json:"isadg:level-of-description,omitempty"`
	ExtentAndMediumOfTheUnitOfDescription string `json:"isadg:extent-and-medium-of-the-unit-of-description,omitempty"`

	// Context
	NameOfCreators                         string `json:"isadg:name-of-creators,omitempty"` // TODO: Should this just be "Name"?
	AdministrativeBiographicalHistory      string `json:"isadg:administrativebiographical-history,omitempty"`
	ArchivalHistory                        string `json:"isadg:archival-history,omitempty"`
	ImmediateSourceOfAcquisitionOrTransfer string `json:"isadg:immediate-source-of-acquisition-or-transfer,omitempty"`

	// Content and Structure
	ScopeAndContent                              string `json:"isadg:scope-and-content,omitempty"`
	AppraisalDestructionAndSchedulingInformation string `json:"isadg:appraisal-destruction-and-scheduling-information,omitempty"`
	Accruals                                     string `json:"isadg:accruals,omitempty"`
	SystemOfArrangement                          string `json:"isadg:system-of-arrangement,omitempty"`

	// Conditions of Access and Use
	ConditionsGoverningAccess                       string `json:"isadg:conditions-governing-access,omitempty"`
	ConditionsGoverningReproduction                 string `json:"isadg:conditions-governing-reproduction,omitempty"`
	LanguagesScriptsOfMaterial                      string `json:"isadg:languagescripts-of-material,omitempty"`
	PhysicalCharacteristicsAndTechnicalRequirements string `json:"isadg:physical-characteristics-and-technical-requirements,omitempty"`
	FindingAids                                     string `json:"isadg:finding-aids,omitempty"`

	// Allied Materials
	ExistenceAndLocationOfOriginals string `json:"isadg:existence-and-location-of-originals,omitempty"`
	ExistenceAndLocationOfCopies    string `json:"isadg:existence-and-location-of-copies,omitempty"`
	RelatedUnitsOfDescription       string `json:"isadg:related-units-of-description,omitempty"`
	PublicationNote                 string `json:"isadg:publication-note,omitempty"`

	// Notes
	Note string `json:"isadg:note,omitempty"`

	// Description Control
	ArchivistsNote      string `json:"isadg:archivists-note,omitempty"`
	RulesOrConventions  string `json:"isadg:rules-or-conventions,omitempty"`
	DatesOfDescriptions string `json:"isadg:dates-of-descriptions,omitempty"`
}

// func NewISADG(title, date, levelOfDescription, extentAndMedium string) *ISADG {
// 	return &ISADG{
// 		Title:                                  title,
// 		ReferenceCodes:                         "ISAD(G)",
// 		Date:                                   date,
// 		LevelOfDescription:                     levelOfDescription,
// 		ExtentAndMediumOfTheUnitOfDescription:  extentAndMedium,
// 		NameOfCreators:                         "Name",
// 		AdministrativeBiographicalHistory:      "Administrative Biographical History",
// 		ArchivalHistory:                        "Archival History",
// 		ImmediateSourceOfAcquisitionOrTransfer: "Immediate Source of Acquisition or Transfer",
// 		ScopeAndContent:                        "Scope and Content",
// 		AppraisalDestructionAndSchedulingInformation: "Appraisal, Destruction, and Scheduling Information",
// 		Accruals:                                        "Accruals",
// 		SystemOfArrangement:                             "System of Arrangement",
// 		ConditionsGoverningAccess:                       "Conditions Governing Access",
// 		ConditionsGoverningReproduction:                 "Conditions Governing Reproduction",
// 		LanguagesScriptsOfMaterial:                      "Languages, Scripts of Material",
// 		PhysicalCharacteristicsAndTechnicalRequirements: "Physical Characteristics and Technical Requirements",
// 		FindingAids:                                     "Finding Aids",
// 		ExistenceAndLocationOfOriginals:                 "Existence and Location of Originals",
// 		ExistenceAndLocationOfCopies:                    "Existence and Location of Copies",
// 		RelatedUnitsOfDescription:                       "Related Units of Description",
// 		PublicationNote:                                 "Publication Note",
// 		Note:                                            "Note",
// 		ArchivistsNote:                                  "Archivists Note",
// 		RulesOrConventions:                              "Rules or Conventions",
// 		DatesOfDescriptions:                             "Dates of Descriptions",
// 	}
// }

// PrintDublinCore prints the Dublin Core metadata to the console.
func (isadg ISADG) AsJson() ([]byte, error) {
	jsonData, err := json.MarshalIndent(isadg, "", "  ")
	if err != nil {
		return []byte{}, fmt.Errorf("error marshaling JSON: %w", err)
	}
	return jsonData, nil
}
