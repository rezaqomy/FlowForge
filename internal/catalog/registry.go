package catalog

import "sort"

type Registry struct {
	operations map[string]OperationManifest
	triggers   map[string]TriggerManifest
}

func New() *Registry {
	return &Registry{
		operations: make(map[string]OperationManifest),
		triggers:   make(map[string]TriggerManifest),
	}
}

func (r *Registry) RegisterManifest(manifest PluginManifest) {
	for _, op := range manifest.Operations {
		r.operations[op.Type] = op
	}
	for _, trigger := range manifest.Triggers {
		r.triggers[trigger.Type] = trigger
	}
}

func (r *Registry) GetOperationManifest(name string) (OperationManifest, bool) {
	manifest, ok := r.operations[name]
	return manifest, ok
}

func (r *Registry) GetTriggerManifest(name string) (TriggerManifest, bool) {
	manifest, ok := r.triggers[name]
	return manifest, ok
}

func (r *Registry) ListOperations() []OperationManifest {
	out := make([]OperationManifest, 0, len(r.operations))
	for _, manifest := range r.operations {
		out = append(out, manifest)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Type < out[j].Type })
	return out
}

func (r *Registry) ListTriggers() []TriggerManifest {
	out := make([]TriggerManifest, 0, len(r.triggers))
	for _, manifest := range r.triggers {
		out = append(out, manifest)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Type < out[j].Type })
	return out
}
