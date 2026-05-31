package secrets

import (
	"errors"
	"testing"
)

func TestValidateCreateMergesStringDataAndDefaultsType(t *testing.T) {
	secret := SecretResource{
		APIVersion: "flowforge/v1alpha1",
		Kind:       "Secret",
		Metadata:   Metadata{Name: "service-credential"},
		StringData: map[string]string{"api-key": "token"},
	}

	if err := ValidateCreate(secret); err != nil {
		t.Fatalf("ValidateCreate() error = %v", err)
	}
	normalized := secret.Normalized()
	if normalized.Type != TypeOpaque {
		t.Fatalf("Type = %q, want %q", normalized.Type, TypeOpaque)
	}
	if string(normalized.Data["api-key"]) != "token" {
		t.Fatalf("StringData was not merged into Data")
	}
	if normalized.StringData != nil {
		t.Fatalf("StringData should be cleared after normalization")
	}
}

func TestValidateCreateRejectsInvalidName(t *testing.T) {
	err := ValidateCreate(SecretResource{
		Metadata: Metadata{Name: "Service_Credential"},
		Data:     map[string][]byte{"token": []byte("value")},
	})
	if !errors.Is(err, ErrInvalidSecret) {
		t.Fatalf("ValidateCreate() error = %v, want ErrInvalidSecret", err)
	}
}

func TestValidateCreateRejectsInvalidKey(t *testing.T) {
	err := ValidateCreate(SecretResource{
		Metadata: Metadata{Name: "service-credential"},
		Data:     map[string][]byte{"bad/key": []byte("value")},
	})
	if !errors.Is(err, ErrInvalidSecret) {
		t.Fatalf("ValidateCreate() error = %v, want ErrInvalidSecret", err)
	}
}

func TestValidateCreateEnforcesTypedSecretRequiredKeys(t *testing.T) {
	err := ValidateCreate(SecretResource{
		Metadata: Metadata{Name: "service-credential"},
		Type:     TypeAPIKey,
		Data:     map[string][]byte{"other": []byte("value")},
	})
	if !errors.Is(err, ErrInvalidSecret) {
		t.Fatalf("ValidateCreate() error = %v, want ErrInvalidSecret", err)
	}
}

func TestValidateUpdateRejectsImmutableDataChange(t *testing.T) {
	oldSecret := SecretResource{
		Metadata:  Metadata{Name: "service-credential"},
		Data:      map[string][]byte{"api-key": []byte("old")},
		Immutable: true,
	}
	newSecret := SecretResource{
		Metadata:  Metadata{Name: "service-credential"},
		Data:      map[string][]byte{"api-key": []byte("new")},
		Immutable: true,
	}

	err := ValidateUpdate(oldSecret, newSecret)
	if !errors.Is(err, ErrSecretImmutable) {
		t.Fatalf("ValidateUpdate() error = %v, want ErrSecretImmutable", err)
	}
}
