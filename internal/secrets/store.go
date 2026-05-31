package secrets

import (
	"context"
	"fmt"
	"sync"
)

type Store interface {
	Create(ctx context.Context, secret SecretResource) error
	Get(ctx context.Context, name string) (SecretResource, error)
	Update(ctx context.Context, secret SecretResource) error
	Delete(ctx context.Context, name string) error
	Resolve(ctx context.Context, ref SecretRef) ([]byte, error)
}

type EncryptedMemoryStore struct {
	mu      sync.RWMutex
	cipher  *EnvelopeCipher
	secrets map[string]EncryptedSecret
}

func NewEncryptedMemoryStore(cipher *EnvelopeCipher) *EncryptedMemoryStore {
	return &EncryptedMemoryStore{
		cipher:  cipher,
		secrets: make(map[string]EncryptedSecret),
	}
}

func (s *EncryptedMemoryStore) Create(_ context.Context, secret SecretResource) error {
	normalized := secret.Normalized()
	if err := ValidateCreate(normalized); err != nil {
		return err
	}
	encrypted, err := s.cipher.Encrypt(normalized)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.secrets[normalized.Metadata.Name]; exists {
		return ErrSecretExists
	}
	s.secrets[normalized.Metadata.Name] = encrypted
	return nil
}

func (s *EncryptedMemoryStore) Get(_ context.Context, name string) (SecretResource, error) {
	if err := validateSecretName(name); err != nil {
		return SecretResource{}, err
	}
	s.mu.RLock()
	encrypted, ok := s.secrets[name]
	s.mu.RUnlock()
	if !ok {
		return SecretResource{}, ErrSecretNotFound
	}
	secret, err := s.cipher.Decrypt(encrypted)
	if err != nil {
		return SecretResource{}, err
	}
	return secret.deepCopy(), nil
}

func (s *EncryptedMemoryStore) Update(_ context.Context, secret SecretResource) error {
	normalized := secret.Normalized()

	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.secrets[normalized.Metadata.Name]
	if !ok {
		return ErrSecretNotFound
	}
	currentSecret, err := s.cipher.Decrypt(current)
	if err != nil {
		return err
	}
	if err := ValidateUpdate(currentSecret, normalized); err != nil {
		return err
	}
	encrypted, err := s.cipher.Encrypt(normalized)
	if err != nil {
		return err
	}
	s.secrets[normalized.Metadata.Name] = encrypted
	return nil
}

func (s *EncryptedMemoryStore) Delete(_ context.Context, name string) error {
	if err := validateSecretName(name); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.secrets[name]; !ok {
		return ErrSecretNotFound
	}
	delete(s.secrets, name)
	return nil
}

func (s *EncryptedMemoryStore) Resolve(ctx context.Context, ref SecretRef) ([]byte, error) {
	secret, err := s.Get(ctx, ref.Name)
	if err != nil {
		return nil, err
	}
	value, ok := secret.Data[ref.Key]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrSecretKeyMissing, ref.Key)
	}
	return append([]byte(nil), value...), nil
}
