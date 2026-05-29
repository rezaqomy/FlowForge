# Development Guide

## Common Commands

Run all tests:

```sh
go test ./...
```

Run the demo command:

```sh
go run ./cmd/server
```

Format Go code:

```sh
gofmt -w .
```

## Adding a Workflow Example

1. Create a YAML file under `examples/`.
2. Use `apiVersion: flowforge/v1alpha1` and `kind: Workflow`.
3. Register the required plugins in the test or command that loads it.
4. Add a test that unmarshals the YAML and performs a dry run.
5. Include representative `RunRequest.Inputs` and event payload data.

## Adding an Operation

1. Create or update a package under `internal/plugins/`.
2. Implement `kernel.Operation`.
3. Register the operation in `kernel.Registry`.
4. Register the manifest in `catalog.Registry`.
5. Add unit tests for input validation and output.
6. Add dry-run/live tests if the operation has side effects.
7. Update [Plugin Guide](plugins.md) when the operation becomes part of the built-in catalog.

## Adding a Trigger

1. Add a `catalog.TriggerManifest` with the trigger type and event schema.
2. Choose a useful `DefaultAs` alias.
3. Make sure workflows set `spec.trigger.as`; the current engine requires it.
4. Add validation tests for known and unknown trigger fields.

## Validation Behavior

Validation is static and schema-driven. It can confirm known trigger fields and operation outputs, but runtime `inputs.*` values are intentionally open-ended. This lets callers pass custom input maps without a separate input schema.

Validator output uses `Issue`:

- `code`
- `level`
- `message`
- `path`
- `hint`

Errors should block execution in callers that need strict behavior. Warnings, such as `side_effect_operation`, are advisory.

## Testing Strategy

The existing tests cover:

- Kernel execution, aliases, branching, run modes, and events.
- Expression evaluation and path resolution.
- Workflow validation issues.
- Built-in plugin behavior.
- Pseudocode rendering.
- Example workflow loading and dry-run execution.

For new behavior, prefer small package-level tests. Add an example-level test when a change affects a full workflow path.

## Known Extension Points

### HTTP API

`internal/api` currently contains placeholders. A future API should likely expose:

- Workflow creation and retrieval.
- Workflow validation.
- Dry-run execution.
- Live execution.
- Run event retrieval.
- Plugin catalog inspection.

### Persistence

`internal/store` defines `WorkflowStore` and `RunStore` interfaces. `internal/store/sqlite` is reserved for a future SQLite implementation.

### Workflow Generation

`app.GenerationService` is reserved for future workflow generation behavior. It does not currently implement generation logic.

## Documentation Rules

Keep documentation aligned with code behavior:

- Document placeholders as planned or reserved, not implemented.
- Update operation schemas when manifests change.
- Update workflow syntax docs when `kernel.WorkflowResource` changes.
- Update expression docs when evaluator syntax changes.
