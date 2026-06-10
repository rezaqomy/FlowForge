package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"flowforge/internal/kernel"
)

type FileWorkflowStore struct {
	mu  sync.RWMutex
	dir string
}

func NewFileWorkflowStore(dir string) *FileWorkflowStore {
	return &FileWorkflowStore{dir: dir}
}

func (s *FileWorkflowStore) Create(workflow kernel.WorkflowResource) error {
	if err := validateWorkflowName(workflow.Metadata.Name); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writeExclusive(workflow)
}

func (s *FileWorkflowStore) Get(name string) (kernel.WorkflowResource, error) {
	if err := validateWorkflowName(name); err != nil {
		return kernel.WorkflowResource{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.read(name)
}

func (s *FileWorkflowStore) Save(workflow kernel.WorkflowResource) error {
	if err := validateWorkflowName(workflow.Metadata.Name); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.write(workflow)
}

func (s *FileWorkflowStore) Update(workflow kernel.WorkflowResource) error {
	if err := validateWorkflowName(workflow.Metadata.Name); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.read(workflow.Metadata.Name); err != nil {
		return err
	}
	return s.write(workflow)
}

func (s *FileWorkflowStore) Delete(name string) error {
	if err := validateWorkflowName(name); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.Remove(s.path(name)); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ErrWorkflowNotFound
		}
		return err
	}
	return nil
}

func (s *FileWorkflowStore) List() ([]kernel.WorkflowResource, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []kernel.WorkflowResource{}, nil
		}
		return nil, err
	}
	out := make([]kernel.WorkflowResource, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		workflow, err := s.read(strings.TrimSuffix(entry.Name(), ".json"))
		if err != nil {
			return nil, err
		}
		out = append(out, workflow)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Metadata.Name < out[j].Metadata.Name
	})
	return out, nil
}

func (s *FileWorkflowStore) read(name string) (kernel.WorkflowResource, error) {
	data, err := os.ReadFile(s.path(name))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return kernel.WorkflowResource{}, ErrWorkflowNotFound
		}
		return kernel.WorkflowResource{}, err
	}
	var workflow kernel.WorkflowResource
	if err := json.Unmarshal(data, &workflow); err != nil {
		return kernel.WorkflowResource{}, fmt.Errorf("decode workflow %q: %w", name, err)
	}
	return workflow, nil
}

func (s *FileWorkflowStore) write(workflow kernel.WorkflowResource) error {
	if err := os.MkdirAll(s.dir, 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(workflow, "", "  ")
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(s.dir, ".workflow-*.tmp")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Chmod(0600); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempName, s.path(workflow.Metadata.Name))
}

func (s *FileWorkflowStore) writeExclusive(workflow kernel.WorkflowResource) error {
	if err := os.MkdirAll(s.dir, 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(workflow, "", "  ")
	if err != nil {
		return err
	}
	file, err := os.OpenFile(s.path(workflow.Metadata.Name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return ErrWorkflowExists
		}
		return err
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		os.Remove(file.Name())
		return err
	}
	return file.Close()
}

func (s *FileWorkflowStore) path(name string) string {
	return filepath.Join(s.dir, name+".json")
}

func validateWorkflowName(name string) error {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("invalid workflow name %q", name)
	}
	return nil
}
