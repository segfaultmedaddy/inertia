package inertiabase

type Page struct {
	Props          map[string]any      `json:"props"`
	DeferredProps  map[string][]string `json:"deferredProps,omitempty"`
	Component      string              `json:"component"`
	URL            string              `json:"url"`
	Version        string              `json:"version"`
	MergeProps     []string            `json:"mergeProps,omitempty"`
	EncryptHistory bool                `json:"encryptHistory"`
	ClearHistory   bool                `json:"clearHistory"`
}
