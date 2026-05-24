package kernel

type WorkflowResource struct {
	APIVersion string       `json:"apiVersion" yaml:"apiVersion"`
	Kind       string       `json:"kind" yaml:"kind"`
	Metadata   Metadata     `json:"metadata" yaml:"metadata"`
	Spec       WorkflowSpec `json:"spec" yaml:"spec"`
}

type Metadata struct {
	Name        string            `json:"name" yaml:"name"`
	Labels      map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
}

type WorkflowSpec struct {
	Trigger TriggerDef `json:"trigger" yaml:"trigger"`
	Steps   []StepDef  `json:"steps" yaml:"steps"`
}

type TriggerDef struct {
	Type string         `json:"type" yaml:"type"`
	As   string         `json:"as" yaml:"as"`
	With map[string]any `json:"with,omitempty" yaml:"with,omitempty"`
}

type StepDef struct {
	ID   string         `json:"id" yaml:"id"`
	If   string         `json:"if,omitempty" yaml:"if,omitempty"`
	Do   string         `json:"do,omitempty" yaml:"do,omitempty"`
	As   string         `json:"as,omitempty" yaml:"as,omitempty"`
	With map[string]any `json:"with,omitempty" yaml:"with,omitempty"`
	Then []StepDef      `json:"then,omitempty" yaml:"then,omitempty"`
	Else []StepDef      `json:"else,omitempty" yaml:"else,omitempty"`
}
