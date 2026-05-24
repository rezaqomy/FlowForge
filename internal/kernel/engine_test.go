package kernel

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeOperation struct {
	output    map[string]any
	lastInput map[string]any
	lastMeta  OperationMeta
	err       error
}

func (f *fakeOperation) Run(_ context.Context, input map[string]any, meta OperationMeta) (OperationResult, error) {
	f.lastInput = input
	f.lastMeta = meta
	if f.err != nil {
		return OperationResult{}, f.err
	}
	return OperationResult{Output: f.output}, nil
}

func testWorkflow() WorkflowResource {
	return WorkflowResource{
		APIVersion: "flowforge/v1alpha1",
		Kind:       "Workflow",
		Metadata:   Metadata{Name: "important-messages"},
		Spec: WorkflowSpec{
			Trigger: TriggerDef{Type: "telegram.message", As: "message"},
			Steps: []StepDef{
				{
					ID: "check_sender",
					If: "message.sender_id in inputs.target_contacts",
					Then: []StepDef{
						{
							ID: "analyze",
							Do: "ai.importance",
							As: "analysis",
							With: map[string]any{
								"text":    "$message.text",
								"context": "$inputs.business_context",
							},
						},
						{
							ID: "send_admin",
							If: "analysis.important",
							Do: "telegram.send",
							With: map[string]any{
								"to":   "$inputs.admin",
								"text": "$message.text",
							},
						},
					},
					Else: []StepDef{
						{
							ID: "record_skip",
							Do: "storage.save",
							With: map[string]any{
								"key":   "skip",
								"value": "$message.sender_id",
							},
						},
					},
				},
			},
		},
	}
}

func newTestEngine(reg *Registry) *Engine {
	engine := NewEngine(reg, NewEvaluator(), NewResolver())
	engine.now = func() time.Time { return time.Unix(0, 0).UTC() }
	engine.newRunID = func() string { return "run_test" }
	return engine
}

func baseRequest() RunRequest {
	return RunRequest{
		Workflow: testWorkflow(),
		Event: Event{
			Type: "telegram.message",
			Payload: map[string]any{
				"sender_id": "u_123",
				"text":      "urgent payment issue",
			},
		},
		Inputs: map[string]any{
			"target_contacts":  []any{"u_123"},
			"business_context": "support",
			"admin":            "admin_chat",
		},
		Mode: RunModeDryRun,
	}
}

func TestEngineRunsSimpleWorkflow(t *testing.T) {
	reg := NewRegistry()
	analyze := &fakeOperation{output: map[string]any{"important": true}}
	send := &fakeOperation{output: map[string]any{"message_id": "msg_1"}}
	reg.Register("ai.importance", analyze)
	reg.Register("telegram.send", send)

	result, err := newTestEngine(reg).Run(context.Background(), baseRequest())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != RunStatusCompleted {
		t.Fatalf("Run() status = %s, want %s", result.Status, RunStatusCompleted)
	}
	if result.Vars["analysis"] == nil {
		t.Fatalf("Run() did not store analysis output")
	}
	if send.lastInput["to"] != "admin_chat" {
		t.Fatalf("telegram.send input to = %v, want admin_chat", send.lastInput["to"])
	}
}

func TestEngineSkipsStepWhenConditionFalse(t *testing.T) {
	reg := NewRegistry()
	save := &fakeOperation{output: map[string]any{"ok": true}}
	reg.Register("storage.save", save)

	req := baseRequest()
	req.Inputs["target_contacts"] = []any{"u_999"}
	result, err := newTestEngine(reg).Run(context.Background(), req)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if save.lastInput["value"] != "u_123" {
		t.Fatalf("else branch did not execute")
	}
	foundSkip := false
	for _, event := range result.Events {
		if event.Type == "step.skipped" && event.StepID == "check_sender" {
			foundSkip = true
		}
	}
	if !foundSkip {
		t.Fatalf("expected step.skipped event")
	}
}

func TestEngineStoresOperationOutputUsingAlias(t *testing.T) {
	reg := NewRegistry()
	reg.Register("ai.importance", &fakeOperation{output: map[string]any{"important": true, "confidence": 0.9}})
	reg.Register("telegram.send", &fakeOperation{output: map[string]any{"message_id": "msg_1"}})

	result, err := newTestEngine(reg).Run(context.Background(), baseRequest())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	analysis, ok := result.Vars["analysis"].(map[string]any)
	if !ok || analysis["confidence"] != 0.9 {
		t.Fatalf("alias output = %#v", result.Vars["analysis"])
	}
}

func TestEngineUnknownOperationReturnsError(t *testing.T) {
	reg := NewRegistry()
	_, err := newTestEngine(reg).Run(context.Background(), baseRequest())
	if !errors.Is(err, ErrUnknownOperation) {
		t.Fatalf("Run() error = %v, want ErrUnknownOperation", err)
	}
}

func TestEnginePassesDryRunModeToOperations(t *testing.T) {
	reg := NewRegistry()
	analyze := &fakeOperation{output: map[string]any{"important": false}}
	reg.Register("ai.importance", analyze)

	req := baseRequest()
	req.Workflow.Spec.Steps[0].Then = req.Workflow.Spec.Steps[0].Then[:1]
	if _, err := newTestEngine(reg).Run(context.Background(), req); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if analyze.lastMeta.Mode != RunModeDryRun {
		t.Fatalf("operation mode = %s, want %s", analyze.lastMeta.Mode, RunModeDryRun)
	}
}

func TestEngineEmitsExecutionEvents(t *testing.T) {
	reg := NewRegistry()
	reg.Register("ai.importance", &fakeOperation{output: map[string]any{"important": false}})
	req := baseRequest()
	req.Workflow.Spec.Steps[0].Then = req.Workflow.Spec.Steps[0].Then[:1]

	result, err := newTestEngine(reg).Run(context.Background(), req)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(result.Events) == 0 {
		t.Fatalf("expected execution events")
	}
	if result.Events[0].Type != "run.started" {
		t.Fatalf("first event = %s, want run.started", result.Events[0].Type)
	}
	if result.Events[len(result.Events)-1].Type != "run.completed" {
		t.Fatalf("last event = %s, want run.completed", result.Events[len(result.Events)-1].Type)
	}
}
