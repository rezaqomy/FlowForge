package secrets

import "testing"

func TestEnvelopeCipherRoundTrip(t *testing.T) {
	key := make([]byte, 32)
	key[0] = 1
	cipher := NewEnvelopeCipher(StaticKeyProvider{Key: key})
	secret := SecretResource{
		APIVersion: "flowforge/v1alpha1",
		Kind:       "Secret",
		Metadata:   Metadata{Name: "service-credential"},
		Type:       TypeAPIKey,
		Data:       map[string][]byte{"api-key": []byte("secret-token")},
		Immutable:  true,
	}

	encrypted, err := cipher.Encrypt(secret)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	if string(encrypted.Ciphertext) == "secret-token" {
		t.Fatalf("ciphertext contains plaintext token")
	}

	decrypted, err := cipher.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}
	if string(decrypted.Data["api-key"]) != "secret-token" {
		t.Fatalf("decrypted api-key = %q", decrypted.Data["api-key"])
	}
	if !decrypted.Immutable {
		t.Fatalf("immutable flag was not preserved")
	}
}

func TestEnvelopeCipherRejectsAuthenticatedMetadataTampering(t *testing.T) {
	key := make([]byte, 32)
	key[0] = 2
	cipher := NewEnvelopeCipher(StaticKeyProvider{Key: key})
	encrypted, err := cipher.Encrypt(SecretResource{
		Metadata: Metadata{Name: "service-credential"},
		Data:     map[string][]byte{"token": []byte("secret-token")},
	})
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	encrypted.Metadata.Name = "other"
	if _, err := cipher.Decrypt(encrypted); err == nil {
		t.Fatalf("Decrypt() error = nil, want authentication error")
	}
}

func TestStaticKeyProviderRequiresThirtyTwoBytes(t *testing.T) {
	if _, err := (StaticKeyProvider{Key: []byte("short")}).MasterKey(); err == nil {
		t.Fatalf("MasterKey() error = nil, want error")
	}
}
