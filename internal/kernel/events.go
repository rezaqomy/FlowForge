package kernel

import "time"

type EventSink interface {
	Emit(event ExecutionEvent)
}

type ExecutionEvent struct {
	Type      string         `json:"type"`
	RunID     string         `json:"runId"`
	StepID    string         `json:"stepId,omitempty"`
	Message   string         `json:"message,omitempty"`
	Timestamp time.Time      `json:"timestamp"`
	Data      map[string]any `json:"data,omitempty"`
}

type CollectingSink struct {
	Events []ExecutionEvent
}

func (s *CollectingSink) Emit(event ExecutionEvent) {
	s.Events = append(s.Events, event)
}

type FanoutSink struct {
	sinks []EventSink
}

func NewFanoutSink(sinks ...EventSink) *FanoutSink {
	filtered := make([]EventSink, 0, len(sinks))
	for _, sink := range sinks {
		if sink != nil {
			filtered = append(filtered, sink)
		}
	}
	return &FanoutSink{sinks: filtered}
}

func (s *FanoutSink) Emit(event ExecutionEvent) {
	for _, sink := range s.sinks {
		sink.Emit(event)
	}
}
