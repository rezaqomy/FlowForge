package secrets

type SecretType string

const (
	TypeOpaque SecretType = "Opaque"
	TypeAPIKey SecretType = "flowforge.io/api-key"

	MaxSecretSize = 1 * 1024 * 1024
)

type Metadata struct {
	Name        string            `json:"name" yaml:"name"`
	Labels      map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
}

type SecretResource struct {
	APIVersion string            `json:"apiVersion" yaml:"apiVersion"`
	Kind       string            `json:"kind" yaml:"kind"`
	Metadata   Metadata          `json:"metadata" yaml:"metadata"`
	Type       SecretType        `json:"type,omitempty" yaml:"type,omitempty"`
	Data       map[string][]byte `json:"data,omitempty" yaml:"data,omitempty"`
	StringData map[string]string `json:"stringData,omitempty" yaml:"stringData,omitempty"`
	Immutable  bool              `json:"immutable,omitempty" yaml:"immutable,omitempty"`
}

type SecretRef struct {
	Name string `json:"name" yaml:"name"`
	Key  string `json:"key" yaml:"key"`
}

func (s SecretResource) Normalized() SecretResource {
	out := s.deepCopy()
	if out.Type == "" {
		out.Type = TypeOpaque
	}
	if out.Data == nil {
		out.Data = make(map[string][]byte)
	}
	for key, value := range out.StringData {
		out.Data[key] = []byte(value)
	}
	out.StringData = nil
	return out
}

func (s SecretResource) deepCopy() SecretResource {
	out := s
	out.Metadata.Labels = copyStringMap(s.Metadata.Labels)
	out.Metadata.Annotations = copyStringMap(s.Metadata.Annotations)
	out.Data = copyBytesMap(s.Data)
	out.StringData = copyStringMap(s.StringData)
	return out
}

func copyStringMap(input map[string]string) map[string]string {
	if input == nil {
		return nil
	}
	out := make(map[string]string, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func copyBytesMap(input map[string][]byte) map[string][]byte {
	if input == nil {
		return nil
	}
	out := make(map[string][]byte, len(input))
	for key, value := range input {
		out[key] = append([]byte(nil), value...)
	}
	return out
}
