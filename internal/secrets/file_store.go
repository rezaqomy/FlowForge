package secrets

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type EncryptedFileStore struct {
	dir    string
	cipher *EnvelopeCipher
}

func NewEncryptedFileStore(dir string, cipher *EnvelopeCipher) *EncryptedFileStore {
	return &EncryptedFileStore{dir: dir, cipher: cipher}
}

func (s *EncryptedFileStore) Create(_ context.Context, secret SecretResource) error {
	normalized := secret.Normalized()
	if err := ValidateCreate(normalized); err != nil {
		return err
	}
	path := s.path(normalized.Metadata.Name)
	if _, err := os.Stat(path); err == nil {
		return ErrSecretExists
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	encrypted, err := s.cipher.Encrypt(normalized)
	if err != nil {
		return err
	}
	return s.write(path, encrypted)
}

func (s *EncryptedFileStore) Get(_ context.Context, name string) (SecretResource, error) {
	if err := validateSecretName(name); err != nil {
		return SecretResource{}, err
	}
	encrypted, err := s.read(name)
	if err != nil {
		return SecretResource{}, err
	}
	secret, err := s.cipher.Decrypt(encrypted)
	if err != nil {
		return SecretResource{}, err
	}
	return secret.deepCopy(), nil
}

func (s *EncryptedFileStore) Update(_ context.Context, secret SecretResource) error {
	normalized := secret.Normalized()
	current, err := s.read(normalized.Metadata.Name)
	if err != nil {
		return err
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
	return s.write(s.path(normalized.Metadata.Name), encrypted)
}

func (s *EncryptedFileStore) Delete(_ context.Context, name string) error {
	if err := validateSecretName(name); err != nil {
		return err
	}
	if err := os.Remove(s.path(name)); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ErrSecretNotFound
		}
		return err
	}
	return nil
}

func (s *EncryptedFileStore) Resolve(ctx context.Context, ref SecretRef) ([]byte, error) {
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

func (s *EncryptedFileStore) read(name string) (EncryptedSecret, error) {
	if err := validateSecretName(name); err != nil {
		return EncryptedSecret{}, err
	}
	data, err := os.ReadFile(s.path(name))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return EncryptedSecret{}, ErrSecretNotFound
		}
		return EncryptedSecret{}, err
	}
	var encrypted EncryptedSecret
	if err := json.Unmarshal(data, &encrypted); err != nil {
		return EncryptedSecret{}, err
	}
	return encrypted, nil
}

func (s *EncryptedFileStore) write(path string, secret EncryptedSecret) error {
	if err := os.MkdirAll(s.dir, 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(secret, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(s.dir, ".secret-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func (s *EncryptedFileStore) path(name string) string {
	return filepath.Join(s.dir, name+".json")
}
