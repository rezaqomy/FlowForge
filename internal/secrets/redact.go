package secrets

const Redacted = "[REDACTED]"

type Redactor struct {
	values map[string]struct{}
}

func NewRedactor(values ...[]byte) Redactor {
	redactor := Redactor{values: make(map[string]struct{}, len(values))}
	for _, value := range values {
		if len(value) == 0 {
			continue
		}
		redactor.values[string(value)] = struct{}{}
	}
	return redactor
}

func (r Redactor) String(value string) string {
	if _, ok := r.values[value]; ok {
		return Redacted
	}
	return value
}
