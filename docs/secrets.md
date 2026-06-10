# Secret Management

FlowForge has a native secret management package for storing, validating, encrypting, and resolving application secrets.

## Goals

- Keep raw secret values out of workflow definitions, run events, logs, and rendered output.
- Store secret data encrypted at rest.
- Resolve individual secret keys by explicit reference.
- Support immutable secrets for production credentials that should not drift.
- Keep the storage boundary replaceable for future backends.

## Secret Resource

Secrets are represented as FlowForge resources:

```yaml
apiVersion: flowforge/v1alpha1
kind: Secret
metadata:
  name: service-credential
type: flowforge.io/api-key
immutable: true
stringData:
  api-key: "<api-key-placeholder>"
```

`stringData` is write-only input. The package normalizes it into binary-safe `data` and clears `stringData`.

## Secret Types

| Type | Required keys |
| --- | --- |
| `Opaque` | None |
| `flowforge.io/api-key` | `api-key` |

Unknown secret types are rejected. New integration-specific types should add explicit validation.

## Validation

Validation checks:

- `metadata.name` is a DNS-style label.
- data keys use alphanumeric characters, `.`, `_`, or `-`.
- total data size does not exceed `MaxSecretSize`.
- type-specific required keys are present.
- immutable secrets cannot change data or type.

## Encryption

The current implementation uses envelope encryption:

1. Generate a random data encryption key.
2. Encrypt secret data with AES-256-GCM.
3. Encrypt the data key with a master key using AES-256-GCM.
4. Store only ciphertext, nonces, metadata, type, and immutable flag.

The package includes two backends:

| Backend | Purpose |
| --- | --- |
| `EncryptedMemoryStore` | Runtime/testing backend that stores encrypted records in memory. |
| `EncryptedFileStore` | Local persistent backend that writes envelope-encrypted JSON records with `0600` file permissions and atomic rename. |

## Resolution

Callers resolve a single key with:

```go
SecretRef{Name: "service-credential", Key: "api-key"}
```

Resolved values are returned as bytes and should be used just-in-time by integrations. They should not be copied into workflow scope or operation results.

## Telegram Credentials

Telegram credentials can be stored as encrypted `Secret` resources instead of process environment variables:

```yaml
apiVersion: flowforge/v1alpha1
kind: Secret
metadata:
  name: telegram-bot
type: flowforge.io/api-key
immutable: true
stringData:
  api-key: "<telegram-bot-token>"
```

Optional proxy credentials can be stored separately:

```yaml
apiVersion: flowforge/v1alpha1
kind: Secret
metadata:
  name: telegram-proxy
type: Opaque
immutable: true
stringData:
  url: "socks5://127.0.0.1:1080"
```

Wire the stored values into the Telegram plugin when registering it:

```go
store := secrets.NewEncryptedFileStore(secretDir, cipher)
send := telegram.NewSendOperationWithOptions(telegram.SendOptions{
    SecretResolver: store,
    BotTokenRef:    &secrets.SecretRef{Name: "telegram-bot", Key: "api-key"},
    ProxyURLRef:    &secrets.SecretRef{Name: "telegram-proxy", Key: "url"},
})
reg.Register("telegram.send", send)
```

The encrypted file backend writes ciphertext to disk with `0600` file permissions. Keep the master key outside the repository and back it up separately; without it, saved secrets cannot be decrypted.
