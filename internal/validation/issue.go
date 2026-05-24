package validation

type IssueLevel string

const (
	IssueLevelError   IssueLevel = "error"
	IssueLevelWarning IssueLevel = "warning"
)

type Issue struct {
	Code    string     `json:"code"`
	Level   IssueLevel `json:"level"`
	Message string     `json:"message"`
	Path    string     `json:"path"`
	Hint    string     `json:"hint,omitempty"`
}
