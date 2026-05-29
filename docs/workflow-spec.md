# Workflow Specification

FlowForge workflows are YAML resources with `apiVersion`, `kind`, `metadata`, and `spec` fields.

## Top-Level Resource

```yaml
apiVersion: flowforge/v1alpha1
kind: Workflow

metadata:
  name: example
  labels:
    team: support
  annotations:
    description: Demo workflow

spec:
  trigger:
    type: telegram.message
    as: message

  steps: []
```

### Fields

| Field | Required | Description |
| --- | --- | --- |
| `apiVersion` | Recommended | Workflow API version. Current examples use `flowforge/v1alpha1`. |
| `kind` | Recommended | Resource kind. Current examples use `Workflow`. |
| `metadata.name` | Recommended | Human-readable workflow name used in run metadata. |
| `metadata.labels` | No | String labels for categorization. |
| `metadata.annotations` | No | String annotations for extra metadata. |
| `spec.trigger` | Yes | Event definition that starts the workflow. |
| `spec.steps` | Yes | Ordered list of workflow steps. |

The engine currently requires `spec.trigger.as` during execution. Validation also depends on a registered trigger type to understand trigger payload fields.

## Trigger

```yaml
trigger:
  type: telegram.message
  as: message
  with:
    optional: value
```

| Field | Required | Description |
| --- | --- | --- |
| `type` | Yes | Trigger type registered in the catalog. |
| `as` | Yes | Alias used to expose the event payload in expressions and references. |
| `with` | No | Reserved trigger configuration map. The current engine does not consume it. |

When the run starts, the trigger payload is stored under the alias. If `as: message`, the event field `text` is available as `message.text`.

## Steps

```yaml
- id: analyze
  if: "message.sender_id in inputs.target_contacts"
  do: ai.importance
  as: analysis
  with:
    text: "$message.text"
    context: "$inputs.business_context"
  then: []
  else: []
```

| Field | Required | Description |
| --- | --- | --- |
| `id` | Yes at runtime | Unique step identifier used in errors and events. |
| `if` | No | Boolean expression controlling whether the step runs. |
| `do` | No | Operation type to execute. |
| `as` | No | Alias used to store operation output in scope. |
| `with` | No | Operation input map. Values can include `$...` references. |
| `then` | No | Nested steps to run after this step succeeds and its condition is true. |
| `else` | No | Nested steps to run when this step's condition evaluates false. |

A step can be a condition-only container when it has `if` and nested branches but no `do`.

## Scope and References

The workflow scope contains:

- Trigger payload under `spec.trigger.as`.
- Runtime inputs under `inputs`.
- Operation outputs under each step's `as` alias.

Use plain paths in expressions:

```yaml
if: "message.sender_id in inputs.target_contacts"
```

Use `$`-prefixed paths in `with` values:

```yaml
with:
  text: "$message.text"
  to: "$inputs.admin"
```

The resolver recursively processes strings, maps, and arrays. Strings that do not start with `$` are treated as literals.

## Expressions

Conditions must evaluate to a boolean.

Supported literals:

- Strings with double quotes, for example `"urgent"`.
- Numbers, parsed as floating-point values.
- Booleans: `true`, `false`.
- Identifiers such as `message.text` or `analysis.important`.

Supported operators:

| Operator | Meaning |
| --- | --- |
| `and` | Boolean conjunction. |
| `or` | Boolean disjunction. |
| `not` | Boolean negation. |
| `==` | Equality comparison. |
| `!=` | Inequality comparison. |
| `>` | Numeric greater-than comparison. |
| `<` | Numeric less-than comparison. |
| `>=` | Numeric greater-than-or-equal comparison. |
| `<=` | Numeric less-than-or-equal comparison. |
| `in` | Checks whether the left value exists in a list. |
| `contains` | Checks whether the string form of the left value contains the string form of the right value. |

Parentheses are supported:

```yaml
if: "(analysis.important and analysis.confidence >= 0.8) or message.text contains \"urgent\""
```

## Branching Behavior

If a step has an `if` expression:

- When the condition is true, the step operation runs, followed by its `then` steps.
- When the condition is false, the engine emits `step.skipped` and runs `else` steps if they exist.

If a step has no `if`, it is treated as true.

## Run Modes

FlowForge defines two run modes:

- `dry_run`
- `live`

The engine passes the mode to every operation as `OperationMeta.Mode`. Operations are responsible for honoring it. The built-in `telegram.send` and `storage.save` operations only perform their side effects in `live` mode.

## Validation Notes

The validator catches common workflow issues before execution:

- Unknown trigger or operation.
- Missing required operation inputs.
- Invalid condition expression syntax.
- Unknown trigger or alias paths.
- Duplicate output aliases in a validation branch.
- Side-effecting operations, reported as warnings.

Runtime execution can still fail if input values have the wrong type or if a required runtime input is missing.
