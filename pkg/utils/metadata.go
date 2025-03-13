package utils

type DcMetadata struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Creator     string `json:"creator"`
	Subject     string `json:"subject"`
	Publisher   string `json:"publisher"`
	Contributor string `json:"contributor"`
	Date        string `json:"date"`
	Type        string `json:"type"`
	Format      string `json:"format"`
	Identifier  string `json:"identifier"`
	Source      string `json:"source"`
	Language    string `json:"language"`
	Relation    string `json:"relation"`
	Coverage    string `json:"coverage"`
	Rights      string `json:"rights"`
}
