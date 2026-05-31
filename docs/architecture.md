# Architecture

FlowForge is organized around a small workflow kernel plus supporting packages for plugin metadata, validation, rendering, and application orchestration.

## Runtime Model

A run starts with a `kernel.RunRequest`:

- `Workflow`: the parsed `WorkflowResource`.
- `Event`: the trigger event type and payload.
- `Inputs`: caller-provided workflow inputs.
- `Mode`: `dry_run` or `live`.
- `Sink`: optional event sink for observing execution.

The engine creates a workflow scope with two root values:

- The trigger alias from `spec.trigger.as`, populated from `Event.Payload`.
- `inputs`, populated from `RunRequest.Inputs`.

For example, a trigger alias of `message` exposes `message.text`, while external inputs are available as `inputs.admin`.

## Main Packages

### `internal/kernel`

The kernel contains the core workflow runtime:

- `WorkflowResource`, `WorkflowSpec`, `TriggerDef`, and `StepDef` model the YAML resource.
- `Engine` executes steps, branches, and operations.
- `ExpressionEvaluator` parses and evaluates boolean `if` expressions.
- `PathResolver` resolves `$...` references in operation inputs.
- `Scope` stores trigger data, inputs, and operation output aliases.
- `ExecutionEvent` and event sinks expose runtime progress.

### `internal/catalog`

The catalog stores plugin manifests independently from executable operation instances. It is used by validation and rendering to understand registered triggers and operations.

Operation manifests describe:

- Type name, such as `telegram.send`.
- Input and output schemas.
- Whether the operation can have side effects.
- Optional display hints.

Trigger manifests describe:

- Event type, such as `telegram.message`.
- Default alias.
- Event payload schema.

### `internal/validation`

The validator performs static checks before execution:

- Unknown trigger types.
- Unknown operation types.
- Missing required operation inputs.
- Invalid expressions.
- Unknown variables in expressions and `$...` references.
- Duplicate output aliases in the same validation path.
- Warnings for operations marked as side-effecting.

The validator allows arbitrary `inputs.*` paths because runtime inputs are caller-defined.

### `internal/runner`

The runner wraps the kernel engine and adds an in-memory event sink. It returns both the kernel run result and the collected events.

### `internal/render`

The pseudocode renderer converts workflow YAML into a readable outline. It uses catalog display hints when available and strips the `inputs.` prefix from displayed references for readability.

### `internal/secrets`

The secrets package defines a FlowForge-native secret resource model. It includes validation, immutable secret checks, explicit key references, redaction helpers, envelope encryption, and encrypted memory/file store implementations.

### `internal/plugins`

Built-in example plugins register both executable operations and catalog manifests:

- `ai.importance`
- `telegram.message`
- `telegram.send`
- `storage.save`

### `internal/app`, `internal/api`, and `internal/store`

These packages define early application boundaries:

- `app.RunService` delegates execution to `runner.Runner`.
- `app.WorkflowService` currently returns workflows unchanged.
- `app.GenerationService` is a placeholder.
- `api.Server` is a placeholder.
- `store.WorkflowStore` and `store.RunStore` are interfaces.
- `store/sqlite` is reserved for a future SQLite-backed store.

## Execution Flow

1. Register plugins into the operation registry and catalog.
2. Load YAML into `kernel.WorkflowResource`.
3. Optionally validate the workflow using `validation.Validator`.
4. Build a `kernel.RunRequest`.
5. Run through `runner.Runner` or directly through `kernel.Engine`.
6. The engine initializes scope from the trigger payload and inputs.
7. Each step emits `step.started`.
8. If `if` is set, the expression evaluator decides whether the step runs.
9. If the condition is false, the engine emits `step.skipped` and runs `else` steps when present.
10. If `do` is set, the resolver resolves `with` inputs and the registered operation runs.
11. If `as` is set, operation output is stored under that alias.
12. Nested `then` steps run after a successful step.
13. The engine emits `run.completed` or `run.failed`.

## Event Types

The engine currently emits:

- `run.started`
- `run.completed`
- `run.failed`
- `step.started`
- `step.skipped`
- `step.failed`
- `step.completed`
- `operation.started`
- `operation.completed`

Each event includes a run id, timestamp, message, optional step id, and optional data.
