package secrets

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
)

func TestEncryptedFileStorePersistsEncryptedSecret(t *testing.T) {
	store := NewEncryptedFileStore(t.TempDir(), testCipher())
	if err := store.Create(context.Background(), SecretResource{
		Metadata: Metadata{Name: "service-credential"},
		Type:     TypeAPIKey,
		Data:     map[string][]byte{"api-key": []byte("secret-token")},
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	got, err := store.Get(context.Background(), "service-credential")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if string(got.Data["api-key"]) != "secret-token" {
		t.Fatalf("api-key = %q, want secret-token", got.Data["api-key"])
	}

	raw, err := os.ReadFile(store.path("service-credential"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if strings.Contains(string(raw), "secret-token") {
		t.Fatalf("file store wrote plaintext secret")
	}
}

func TestEncryptedFileStoreUpdateAndDelete(t *testing.T) {
	store := NewEncryptedFileStore(t.TempDir(), testCipher())
	if err := store.Create(context.Background(), SecretResource{
		Metadata: Metadata{Name: "service-credential"},
		Data:     map[string][]byte{"token": []byte("old")},
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := store.Update(context.Background(), SecretResource{
		Metadata: Metadata{Name: "service-credential"},
		Data:     map[string][]byte{"token": []byte("new")},
	}); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	resolved, err := store.Resolve(context.Background(), SecretRef{Name: "service-credential", Key: "token"})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if string(resolved) != "new" {
		t.Fatalf("Resolve() = %q, want new", resolved)
	}
	if err := store.Delete(context.Background(), "service-credential"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	_, err = store.Get(context.Background(), "service-credential")
	if !errors.Is(err, ErrSecretNotFound) {
		t.Fatalf("Get() error = %v, want ErrSecretNotFound", err)
	}
}

func TestEncryptedFileStoreListsSecretsByName(t *testing.T) {
	store := NewEncryptedFileStore(t.TempDir(), testCipher())
	for _, name := range []string{"second", "first"} {
		if err := store.Create(context.Background(), SecretResource{
			Metadata: Metadata{Name: name},
			Data:     map[string][]byte{"token": []byte(name)},
		}); err != nil {
			t.Fatalf("Create(%q) error = %v", name, err)
		}
	}
	list, err := store.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != 2 || list[0].Metadata.Name != "first" || list[1].Metadata.Name != "second" {
		t.Fatalf("List() returned unexpected order: %#v", list)
	}
}

func TestEncryptedFileStoreExplicitlyReplacesImmutableSecret(t *testing.T) {
	store := NewEncryptedFileStore(t.TempDir(), testCipher())
	if err := store.Create(context.Background(), SecretResource{
		Metadata:  Metadata{Name: "service-credential"},
		Data:      map[string][]byte{"token": []byte("old")},
		Immutable: true,
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := store.Replace(context.Background(), SecretResource{
		Metadata:  Metadata{Name: "service-credential"},
		Data:      map[string][]byte{"token": []byte("new")},
		Immutable: true,
	}); err != nil {
		t.Fatalf("Replace() error = %v", err)
	}
	value, err := store.Resolve(context.Background(), SecretRef{Name: "service-credential", Key: "token"})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if string(value) != "new" {
		t.Fatalf("Resolve() = %q, want new", value)
	}
}

func TestEncryptedFileStoreRejectsInvalidLookupName(t *testing.T) {
	store := NewEncryptedFileStore(t.TempDir(), testCipher())

	_, err := store.Get(context.Background(), "../service-credential")
	if !errors.Is(err, ErrInvalidSecret) {
		t.Fatalf("Get() error = %v, want ErrInvalidSecret", err)
	}
}

func testCipher() *EnvelopeCipher {
	key := make([]byte, 32)
	key[0] = 11
	return NewEnvelopeCipher(StaticKeyProvider{Key: key})
}
