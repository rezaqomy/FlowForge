package store

import (
	"errors"
	"testing"
)

func TestFileWorkflowStorePersistsLifecycle(t *testing.T) {
	dir := t.TempDir()
	first := NewFileWorkflowStore(dir)
	workflow := testWorkflow("notify", "telegram.message")
	if err := first.Create(workflow); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	second := NewFileWorkflowStore(dir)
	got, err := second.Get("notify")
	if err != nil {
		t.Fatalf("Get() after reopen error = %v", err)
	}
	if got.Spec.Trigger.Type != "telegram.message" {
		t.Fatalf("Get().Spec.Trigger.Type = %q", got.Spec.Trigger.Type)
	}

	workflow.Spec.Trigger.Type = "message.received"
	if err := second.Update(workflow); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	third := NewFileWorkflowStore(dir)
	got, err = third.Get("notify")
	if err != nil {
		t.Fatalf("Get() after update error = %v", err)
	}
	if got.Spec.Trigger.Type != "message.received" {
		t.Fatalf("persisted trigger = %q, want message.received", got.Spec.Trigger.Type)
	}

	if err := third.Delete("notify"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := NewFileWorkflowStore(dir).Get("notify"); !errors.Is(err, ErrWorkflowNotFound) {
		t.Fatalf("Get() after delete error = %v, want ErrWorkflowNotFound", err)
	}
}
