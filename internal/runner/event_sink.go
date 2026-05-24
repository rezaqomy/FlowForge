package runner

import "flowforge/internal/kernel"

type MemoryEventSink struct {
	Events []kernel.ExecutionEvent
}

func (s *MemoryEventSink) Emit(event kernel.ExecutionEvent) {
	s.Events = append(s.Events, event)
}
