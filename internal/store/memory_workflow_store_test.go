package store

import (
	"errors"
	"testing"

	"flowforge/internal/kernel"
)

func TestMemoryWorkflowStoreLifecycle(t *testing.T) {
	workflowStore := NewMemoryWorkflowStore()
	workflow := testWorkflow("notify", "telegram.message")

	if err := workflowStore.Create(workflow); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := workflowStore.Create(workflow); !errors.Is(err, ErrWorkflowExists) {
		t.Fatalf("Create() error = %v, want ErrWorkflowExists", err)
	}

	workflow.Spec.Trigger.Type = "message.received"
	if err := workflowStore.Update(workflow); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	got, err := workflowStore.Get("notify")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Spec.Trigger.Type != "message.received" {
		t.Fatalf("Get().Spec.Trigger.Type = %q, want message.received", got.Spec.Trigger.Type)
	}

	if err := workflowStore.Delete("notify"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := workflowStore.Get("notify"); !errors.Is(err, ErrWorkflowNotFound) {
		t.Fatalf("Get() after Delete() error = %v, want ErrWorkflowNotFound", err)
	}
}

func TestMemoryWorkflowStoreUpdateMissing(t *testing.T) {
	workflowStore := NewMemoryWorkflowStore()
	if err := workflowStore.Update(testWorkflow("missing", "event")); !errors.Is(err, ErrWorkflowNotFound) {
		t.Fatalf("Update() error = %v, want ErrWorkflowNotFound", err)
	}
}

func testWorkflow(name, trigger string) kernel.WorkflowResource {
	return kernel.WorkflowResource{
		APIVersion: "flowforge/v1alpha1",
		Kind:       "Workflow",
		Metadata:   kernel.Metadata{Name: name},
		Spec: kernel.WorkflowSpec{
			Trigger: kernel.TriggerDef{Type: trigger, As: "event"},
		},
	}
}
