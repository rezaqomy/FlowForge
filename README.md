# FlowForge

FlowForge is a Go workflow automation prototype. It loads declarative YAML workflows, validates them against a plugin catalog, executes registered operations, and emits structured execution events.

The current project is an MVP focused on the workflow kernel and local execution model. The HTTP API, persistent stores, and workflow generation service are intentionally thin placeholders for future implementation.

## What It Does

- Defines workflows as declarative YAML resources.
- Registers trigger and operation plugins in a runtime registry.
- Validates workflow references, required operation inputs, expressions, and side-effect warnings.
- Resolves `$path.to.value` references from the workflow scope before invoking operations.
- Executes conditional, nested steps with `then` and `else` branches.
- Supports `dry_run` and `live` run modes.
- Emits run, step, and operation lifecycle events.
- Renders workflows into human-readable pseudocode.

## Project Layout

```text
cmd/server/                 Demo executable entry point
examples/                   Example workflow and integration-style test
internal/api/               Reserved HTTP API package
internal/app/               Application service layer
internal/catalog/           Plugin manifests and schema registry
internal/kernel/            Workflow model, expression evaluator, resolver, engine
internal/plugins/           Built-in example plugins
internal/render/            Workflow pseudocode renderer
internal/runner/            Execution wrapper and event collection
internal/secrets/           Native secret model, validation, encryption, and store boundary
internal/store/             Store interfaces and future SQLite package
internal/validation/        Workflow validator
docs/                       Detailed project documentation
```

## Requirements

- Go 1.25.9 or newer, matching `go.mod`

## Quick Start

Run the tests:

```sh
go test ./...
```

Run the demo:

```sh
go run ./cmd/server
```

The demo loads `examples/important_messages.yaml`, prints pseudocode for the workflow, and dry-runs it with a sample Telegram message event.

## Example Workflow

```yaml
apiVersion: flowforge/v1alpha1
kind: Workflow

metadata:
  name: important-messages

spec:
  trigger:
    type: telegram.message
    as: message

  steps:
    - id: check_sender
      if: "message.sender_id in inputs.target_contacts"
      then:
        - id: analyze
          do: ai.importance
          as: analysis
          with:
            text: "$message.text"
            context: "$inputs.business_context"

        - id: send_admin
          if: "analysis.important"
          do: telegram.send
          with:
            to: "$inputs.admin"
            text: "$message.text"
```

## Built-In Plugins

| Plugin | Type | Purpose |
| --- | --- | --- |
| Telegram | `telegram.message` trigger | Represents an incoming Telegram message event. |
| Telegram | `telegram.send` operation | Sends a Telegram message in `live` mode and returns a message id. |
| AI | `ai.importance` operation | Classifies a message as important using a deterministic keyword-based implementation. |
| Storage | `storage.save` operation | Saves a value to in-memory storage in `live` mode. |

## Documentation

- [Architecture](docs/architecture.md)
- [Workflow Specification](docs/workflow-spec.md)
- [Plugin Guide](docs/plugins.md)
- [Secret Management](docs/secrets.md)
- [Development Guide](docs/development.md)

## Current Limitations

- `cmd/server` is a demo command, not a production HTTP server.
- `internal/api` contains placeholders only.
- `internal/store/sqlite` is reserved for a future SQLite-backed implementation.
- Built-in plugins are local examples, not production integrations.
- `dry_run` prevents side effects in the provided side-effecting plugins, but operation implementations are responsible for honoring the run mode.
