package catalog

type Catalog interface {
	GetOperationManifest(name string) (OperationManifest, bool)
	GetTriggerManifest(name string) (TriggerManifest, bool)
	ListOperations() []OperationManifest
	ListTriggers() []TriggerManifest
}

type PluginManifest struct {
	Name        string              `json:"name"`
	Description string              `json:"description,omitempty"`
	Operations  []OperationManifest `json:"operations"`
	Triggers    []TriggerManifest   `json:"triggers,omitempty"`
}

type TriggerManifest struct {
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
	DefaultAs   string `json:"defaultAs,omitempty"`
	EventSchema Schema `json:"eventSchema"`
}

type OperationManifest struct {
	Type         string       `json:"type"`
	Description  string       `json:"description,omitempty"`
	InputSchema  Schema       `json:"inputSchema,omitempty"`
	OutputSchema Schema       `json:"outputSchema,omitempty"`
	SideEffect   bool         `json:"sideEffect"`
	Display      DisplayHints `json:"display,omitempty"`
}

type DisplayHints struct {
	Label string `json:"label,omitempty"`
}
