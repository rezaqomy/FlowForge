package kernel

import "context"

type RunMode string

const (
	RunModeLive   RunMode = "live"
	RunModeDryRun RunMode = "dry_run"
)

type RunStatus string

const (
	RunStatusCompleted RunStatus = "completed"
	RunStatusFailed    RunStatus = "failed"
)

type Event struct {
	Type    string         `json:"type"`
	Payload map[string]any `json:"payload"`
}

type RunRequest struct {
	Workflow WorkflowResource
	Event    Event
	Inputs   map[string]any
	Mode     RunMode
	Sink     EventSink
}

type RunResult struct {
	Status RunStatus
	Vars   map[string]any
	Events []ExecutionEvent
}

type Operation interface {
	Run(ctx context.Context, input map[string]any, meta OperationMeta) (OperationResult, error)
}

type OperationMeta struct {
	RunID    string
	Workflow string
	StepID   string
	Mode     RunMode
}

type OperationResult struct {
	Output map[string]any
}

type OperationRegistry interface {
	GetOperation(name string) (Operation, bool)
}

type Registry struct {
	ops map[string]Operation
}

func NewRegistry() *Registry {
	return &Registry{ops: make(map[string]Operation)}
}

func (r *Registry) Register(name string, op Operation) {
	r.ops[name] = op
}

func (r *Registry) GetOperation(name string) (Operation, bool) {
	op, ok := r.ops[name]
	return op, ok
}
