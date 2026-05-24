package catalog

type Schema struct {
	Type       string            `json:"type" yaml:"type"`
	Properties map[string]Schema `json:"properties,omitempty" yaml:"properties,omitempty"`
	Items      *Schema           `json:"items,omitempty" yaml:"items,omitempty"`
	Required   []string          `json:"required,omitempty" yaml:"required,omitempty"`
}
