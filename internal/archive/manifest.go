package archive

import "time"

type Manifest struct {
	Version             int                 `json:"version"`
	Profile             string              `json:"profile"`
	ExportedAt          time.Time           `json:"exported_at"`
	SourcePlatform      string              `json:"source_platform"`
	IncludesCredentials bool                `json:"includes_credentials"`
	IncludesHistory     bool                `json:"includes_history"`
	Fields              map[string][]string `json:"field_classification,omitempty"`
}
