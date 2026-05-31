package secrets

import (
	"context"
	"errors"
	"testing"
)

func TestEncryptedMemoryStoreCreateGetResolve(t *testing.T) {
	store := testStore()
	secret := SecretResource{
		Metadata: Metadata{Name: "service-credential"},
		Type:     TypeAPIKey,
		Data:     map[string][]byte{"api-key": []byte("token")},
	}

	if err := store.Create(context.Background(), secret); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	got, err := store.Get(context.Background(), "service-credential")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if string(got.Data["api-key"]) != "token" {
		t.Fatalf("api-key = %q, want token", got.Data["api-key"])
	}
	resolved, err := store.Resolve(context.Background(), SecretRef{Name: "service-credential", Key: "api-key"})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if string(resolved) != "token" {
		t.Fatalf("Resolve() = %q, want token", resolved)
	}
}

func TestEncryptedMemoryStoreRejectsDuplicateCreate(t *testing.T) {
	store := testStore()
	secret := SecretResource{
		Metadata: Metadata{Name: "service-credential"},
		Data:     map[string][]byte{"token": []byte("value")},
	}
	if err := store.Create(context.Background(), secret); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	err := store.Create(context.Background(), secret)
	if !errors.Is(err, ErrSecretExists) {
		t.Fatalf("Create() error = %v, want ErrSecretExists", err)
	}
}

func TestEncryptedMemoryStoreProtectsInternalDataCopies(t *testing.T) {
	store := testStore()
	secret := SecretResource{
		Metadata: Metadata{Name: "service-credential"},
		Data:     map[string][]byte{"token": []byte("value")},
	}
	if err := store.Create(context.Background(), secret); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	got, err := store.Get(context.Background(), "service-credential")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	got.Data["token"][0] = 'X'
	gotAgain, err := store.Get(context.Background(), "service-credential")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if string(gotAgain.Data["token"]) != "value" {
		t.Fatalf("store leaked mutable internal data: %q", gotAgain.Data["token"])
	}
}

func TestEncryptedMemoryStoreHonorsImmutableSecrets(t *testing.T) {
	store := testStore()
	if err := store.Create(context.Background(), SecretResource{
		Metadata:  Metadata{Name: "service-credential"},
		Data:      map[string][]byte{"token": []byte("old")},
		Immutable: true,
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	err := store.Update(context.Background(), SecretResource{
		Metadata:  Metadata{Name: "service-credential"},
		Data:      map[string][]byte{"token": []byte("new")},
		Immutable: true,
	})
	if !errors.Is(err, ErrSecretImmutable) {
		t.Fatalf("Update() error = %v, want ErrSecretImmutable", err)
	}
}

func TestEncryptedMemoryStoreResolveMissingKey(t *testing.T) {
	store := testStore()
	if err := store.Create(context.Background(), SecretResource{
		Metadata: Metadata{Name: "service-credential"},
		Data:     map[string][]byte{"token": []byte("value")},
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	_, err := store.Resolve(context.Background(), SecretRef{Name: "service-credential", Key: "missing"})
	if !errors.Is(err, ErrSecretKeyMissing) {
		t.Fatalf("Resolve() error = %v, want ErrSecretKeyMissing", err)
	}
}

func testStore() *EncryptedMemoryStore {
	return NewEncryptedMemoryStore(testCipher())
}
