package store

import (
	"sort"
	"sync"

	"flowforge/internal/kernel"
)

type MemoryWorkflowStore struct {
	mu        sync.RWMutex
	workflows map[string]kernel.WorkflowResource
}

func NewMemoryWorkflowStore() *MemoryWorkflowStore {
	return &MemoryWorkflowStore{workflows: make(map[string]kernel.WorkflowResource)}
}

func (s *MemoryWorkflowStore) Create(workflow kernel.WorkflowResource) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.workflows[workflow.Metadata.Name]; exists {
		return ErrWorkflowExists
	}
	s.workflows[workflow.Metadata.Name] = workflow
	return nil
}

func (s *MemoryWorkflowStore) Get(name string) (kernel.WorkflowResource, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	workflow, exists := s.workflows[name]
	if !exists {
		return kernel.WorkflowResource{}, ErrWorkflowNotFound
	}
	return workflow, nil
}

func (s *MemoryWorkflowStore) Save(workflow kernel.WorkflowResource) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.workflows[workflow.Metadata.Name] = workflow
	return nil
}

func (s *MemoryWorkflowStore) Update(workflow kernel.WorkflowResource) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.workflows[workflow.Metadata.Name]; !exists {
		return ErrWorkflowNotFound
	}
	s.workflows[workflow.Metadata.Name] = workflow
	return nil
}

func (s *MemoryWorkflowStore) Delete(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.workflows[name]; !exists {
		return ErrWorkflowNotFound
	}
	delete(s.workflows, name)
	return nil
}

func (s *MemoryWorkflowStore) List() ([]kernel.WorkflowResource, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]kernel.WorkflowResource, 0, len(s.workflows))
	for _, workflow := range s.workflows {
		out = append(out, workflow)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Metadata.Name < out[j].Metadata.Name
	})
	return out, nil
}
