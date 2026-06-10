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
internal/api/               HTTP resource and webhook handlers
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
| Telegram | `telegram.send` operation | Sends a Telegram Bot API `sendMessage` request in `live` mode and returns a message id. |
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
- `internal/store/sqlite` is reserved for a future SQLite-backed implementation.
- Some built-in plugins are local examples rather than production integrations.
- `dry_run` prevents side effects in the provided side-effecting plugins, but operation implementations are responsible for honoring the run mode.

## Resource Management

Use the same manifest command to create a resource or update an existing one:

```sh
go run ./cmd/flowforge apply -f examples/plugins/telegram.yaml
go run ./cmd/flowforge apply -f examples/secrets/telegram-proxy.yaml
```

Get a single resource:

```sh
go run ./cmd/flowforge get workflow telegram-echo
go run ./cmd/flowforge get secret telegram-proxy
```

List resources:

```sh
go run ./cmd/flowforge get workflows
go run ./cmd/flowforge get secrets
```

The default output is formatted JSON. Use `-o yaml` when YAML is preferred:

```sh
go run ./cmd/flowforge get workflow telegram-echo -o yaml
```

Delete a workflow or secret by kind and name:

```sh
go run ./cmd/flowforge delete workflow telegram-echo
go run ./cmd/flowforge delete secret telegram-proxy
```

You can also identify the resource from its manifest. Both forms are accepted:

```sh
go run ./cmd/flowforge delete -f examples/secrets/telegram-proxy.yaml
go run ./cmd/flowforge delete secret examples/secrets/telegram-proxy.yaml
```

`Workflow` and `Secret` resources support create, list, get, update, and delete through their `/v1` endpoints. Workflows and secrets are persisted under the server's `--data-dir`. Secret responses never return stored values.

Direct updates to a secret with `immutable: true` are rejected. The `apply` command uses an explicit atomic replacement when its manifest changes, so declarative updates still take effect without a delete/create gap.

### CLI Completion

Build the CLI as an executable. The `cmd/flowforge` directory itself cannot be executed:

```sh
mkdir -p bin
go build -o bin/flowforge ./cmd/flowforge
```

Install fish completion permanently:

```fish
bin/flowforge completion install fish
```

Start a new fish session, or load it immediately:

```fish
source ~/.config/fish/completions/flowforge.fish
```

The fish completion suggests commands, resource kinds, output formats, manifest paths, and resource names returned by the running server. Add `bin` to `PATH` if you want to invoke the command as `flowforge`. Completion generators are also available for bash and zsh:

```sh
flowforge completion bash
flowforge completion zsh
```

## Live Telegram Sends

`telegram.send` can read the bot token from the encrypted secret store with `SecretRef{Name: "telegram-bot", Key: "api-key"}` when a workflow runs in `live` mode. `TELEGRAM_BOT_TOKEN` remains available as a fallback. Keep the token out of workflow YAML and pass chat ids through runtime inputs, for example `inputs.admin`.

The optional proxy can also come from a secret, for example `SecretRef{Name: "telegram-proxy", Key: "url"}`. `TELEGRAM_PROXY_URL` remains available as a fallback. Supported schemes are `http`, `https`, and `socks5`; keep proxy credentials out of workflow YAML.

## Telegram Polling Echo

Start the HTTP server. Telegram polling is enabled by default and waits until the bot token secret exists:

```sh
go run ./cmd/server -addr 127.0.0.1:8080 -data-dir .flowforge
```

Apply the echo workflow and encrypted bot token from manifest files:

```sh
go run ./cmd/flowforge apply -f examples/plugins/telegram.yaml
go run ./cmd/flowforge apply -f examples/secrets/telegram-bot.yaml
```

If you need a proxy:

```sh
go run ./cmd/flowforge apply -f examples/secrets/telegram-proxy.yaml
```

Send a text message to the bot. FlowForge polls Telegram with `getUpdates`, converts text messages into `telegram.message` events, and runs matching workflows in `live` mode.
