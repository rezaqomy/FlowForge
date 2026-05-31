package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
)

type MasterKeyProvider interface {
	MasterKey() ([]byte, error)
}

type StaticKeyProvider struct {
	Key []byte
}

func (p StaticKeyProvider) MasterKey() ([]byte, error) {
	if len(p.Key) != 32 {
		return nil, fmt.Errorf("master key must be 32 bytes")
	}
	return append([]byte(nil), p.Key...), nil
}

func GenerateMasterKey() ([]byte, error) {
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, err
	}
	return key, nil
}

type EnvelopeCipher struct {
	provider MasterKeyProvider
}

func NewEnvelopeCipher(provider MasterKeyProvider) *EnvelopeCipher {
	if provider == nil {
		panic("nil MasterKeyProvider passed to NewEnvelopeCipher")
	}
	return &EnvelopeCipher{provider: provider}
}

type EncryptedSecret struct {
	APIVersion          string     `json:"apiVersion"`
	Kind                string     `json:"kind"`
	Metadata            Metadata   `json:"metadata"`
	Type                SecretType `json:"type"`
	Immutable           bool       `json:"immutable,omitempty"`
	EncryptedDataKey    []byte     `json:"encryptedDataKey"`
	DataKeyNonce        []byte     `json:"dataKeyNonce"`
	DataNonce           []byte     `json:"dataNonce"`
	Ciphertext          []byte     `json:"ciphertext"`
	EncryptionAlgorithm string     `json:"encryptionAlgorithm"`
}

func (c *EnvelopeCipher) Encrypt(secret SecretResource) (EncryptedSecret, error) {
	normalized := secret.Normalized()
	plaintext, err := json.Marshal(normalized.Data)
	if err != nil {
		return EncryptedSecret{}, err
	}
	dataKey, err := GenerateMasterKey()
	if err != nil {
		return EncryptedSecret{}, err
	}
	masterKey, err := c.provider.MasterKey()
	if err != nil {
		return EncryptedSecret{}, err
	}
	dataAEAD, err := aead(dataKey)
	if err != nil {
		return EncryptedSecret{}, err
	}
	masterAEAD, err := aead(masterKey)
	if err != nil {
		return EncryptedSecret{}, err
	}
	dataNonce, err := nonce(dataAEAD)
	if err != nil {
		return EncryptedSecret{}, err
	}
	keyNonce, err := nonce(masterAEAD)
	if err != nil {
		return EncryptedSecret{}, err
	}
	aad, err := encryptionAAD(normalized.APIVersion, normalized.Kind, normalized.Metadata, normalized.Type, normalized.Immutable)
	if err != nil {
		return EncryptedSecret{}, err
	}
	return EncryptedSecret{
		APIVersion:          normalized.APIVersion,
		Kind:                normalized.Kind,
		Metadata:            normalized.Metadata,
		Type:                normalized.Type,
		Immutable:           normalized.Immutable,
		EncryptedDataKey:    masterAEAD.Seal(nil, keyNonce, dataKey, aad),
		DataKeyNonce:        keyNonce,
		DataNonce:           dataNonce,
		Ciphertext:          dataAEAD.Seal(nil, dataNonce, plaintext, aad),
		EncryptionAlgorithm: "AES-256-GCM envelope",
	}, nil
}

func (c *EnvelopeCipher) Decrypt(encrypted EncryptedSecret) (SecretResource, error) {
	masterKey, err := c.provider.MasterKey()
	if err != nil {
		return SecretResource{}, err
	}
	masterAEAD, err := aead(masterKey)
	if err != nil {
		return SecretResource{}, err
	}
	aad, err := encryptionAAD(encrypted.APIVersion, encrypted.Kind, encrypted.Metadata, encrypted.Type, encrypted.Immutable)
	if err != nil {
		return SecretResource{}, err
	}
	dataKey, err := masterAEAD.Open(nil, encrypted.DataKeyNonce, encrypted.EncryptedDataKey, aad)
	if err != nil {
		return SecretResource{}, fmt.Errorf("decrypt data key: %w", err)
	}
	dataAEAD, err := aead(dataKey)
	if err != nil {
		return SecretResource{}, err
	}
	plaintext, err := dataAEAD.Open(nil, encrypted.DataNonce, encrypted.Ciphertext, aad)
	if err != nil {
		return SecretResource{}, fmt.Errorf("decrypt secret data: %w", err)
	}
	var data map[string][]byte
	if err := json.Unmarshal(plaintext, &data); err != nil {
		return SecretResource{}, err
	}
	return SecretResource{
		APIVersion: encrypted.APIVersion,
		Kind:       encrypted.Kind,
		Metadata:   encrypted.Metadata,
		Type:       encrypted.Type,
		Data:       data,
		Immutable:  encrypted.Immutable,
	}, nil
}

func encryptionAAD(apiVersion, kind string, metadata Metadata, secretType SecretType, immutable bool) ([]byte, error) {
	return json.Marshal(struct {
		APIVersion string     `json:"apiVersion"`
		Kind       string     `json:"kind"`
		Metadata   Metadata   `json:"metadata"`
		Type       SecretType `json:"type"`
		Immutable  bool       `json:"immutable"`
	}{
		APIVersion: apiVersion,
		Kind:       kind,
		Metadata:   metadata,
		Type:       secretType,
		Immutable:  immutable,
	})
}

func aead(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func nonce(aead cipher.AEAD) ([]byte, error) {
	out := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, out); err != nil {
		return nil, err
	}
	return out, nil
}
