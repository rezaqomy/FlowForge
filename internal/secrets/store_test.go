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

func TestEncryptedMemoryStoreListsSecretsByName(t *testing.T) {
	store := testStore()
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
		t.Fatalf("List() names = %v, want [first second]", []string{list[0].Metadata.Name, list[1].Metadata.Name})
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

func TestEncryptedMemoryStoreExplicitlyReplacesImmutableSecret(t *testing.T) {
	store := testStore()
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
