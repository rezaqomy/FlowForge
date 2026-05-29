# Plugin Guide

Plugins connect FlowForge workflow definitions to executable behavior and catalog metadata.

A plugin usually registers two things:

- One or more executable operations in `kernel.Registry`.
- One or more manifests in `catalog.Registry`.

## Operation Interface

Operations implement `kernel.Operation`:

```go
type Operation interface {
	Run(ctx context.Context, input map[string]any, meta OperationMeta) (OperationResult, error)
}
```

`input` contains the resolved `with` map from the workflow step. `meta` contains:

- `RunID`
- `Workflow`
- `StepID`
- `Mode`

Operations return `kernel.OperationResult` with an `Output` map. If the workflow step has `as`, that output is stored in scope under the alias.

## Registering an Operation

```go
func Register(reg *kernel.Registry, cat *catalog.Registry) {
	reg.Register("example.echo", EchoOperation{})
	cat.RegisterManifest(catalog.PluginManifest{
		Name: "example",
		Operations: []catalog.OperationManifest{
			{
				Type:        "example.echo",
				Description: "Returns the provided text.",
				InputSchema: catalog.Schema{
					Type: "object",
					Properties: map[string]catalog.Schema{
						"text": {Type: "string"},
					},
					Required: []string{"text"},
				},
				OutputSchema: catalog.Schema{
					Type: "object",
					Properties: map[string]catalog.Schema{
						"text": {Type: "string"},
					},
					Required: []string{"text"},
				},
				SideEffect: false,
				Display:    catalog.DisplayHints{Label: "example.echo"},
			},
		},
	})
}
```

## Handling Run Modes

Side-effecting operations should check `meta.Mode`.

```go
if meta.Mode == kernel.RunModeLive {
	// perform external write, send message, persist data, etc.
}
```

The validator reports a warning for manifests with `SideEffect: true`, but the engine does not block them. The operation implementation controls what happens in `dry_run`.

## Built-In Plugins

### `ai.importance`

Classifies whether a message is important.

Inputs:

| Field | Type | Required |
| --- | --- | --- |
| `text` | string | Yes |
| `context` | string | Yes |

Outputs:

| Field | Type |
| --- | --- |
| `important` | boolean |
| `reason` | string |
| `confidence` | number |

Current implementation is deterministic and keyword-based. It marks a message important when the normalized text and context contain `urgent` or `payment`.

### `telegram.message`

Represents an incoming Telegram message trigger.

Event payload:

| Field | Type | Required |
| --- | --- | --- |
| `id` | string | Yes |
| `sender_id` | string | Yes |
| `chat_id` | string | Yes |
| `text` | string | Yes |

Default alias: `message`.

### `telegram.send`

Sends a Telegram message.

Inputs:

| Field | Type | Required |
| --- | --- | --- |
| `to` | string | Yes |
| `text` | string | Yes |

Outputs:

| Field | Type |
| --- | --- |
| `message_id` | string |

Side effect: yes.

Current implementation appends to an in-memory `Sent` slice only in `live` mode and always returns a deterministic message id.

### `storage.save`

Saves a value in in-memory storage.

Inputs:

| Field | Type | Required |
| --- | --- | --- |
| `key` | string | Yes |
| `value` | string | Yes in schema |

Outputs:

| Field | Type |
| --- | --- |
| `saved` | boolean |

Side effect: yes.

Current implementation writes to an in-memory map only in `live` mode.

## Plugin Design Checklist

When adding a plugin:

1. Pick stable operation and trigger type names, usually `namespace.action`.
2. Validate input types inside `Run`.
3. Return clear errors because runtime errors are wrapped with the step id.
4. Provide an accurate manifest with required inputs and output fields.
5. Mark side-effecting operations with `SideEffect: true`.
6. Honor `dry_run` for external writes, sends, and persistence.
7. Add focused tests for success, invalid input, and run-mode behavior.
