package secrets

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
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
	encrypted, err := s.cipher.Encrypt(normalized)
	if err != nil {
		return err
	}
	return s.writeExclusive(s.path(normalized.Metadata.Name), encrypted)
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

func (s *EncryptedFileStore) List(_ context.Context) ([]SecretResource, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []SecretResource{}, nil
		}
		return nil, err
	}
	out := make([]SecretResource, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".json")
		encrypted, err := s.read(name)
		if err != nil {
			return nil, err
		}
		secret, err := s.cipher.Decrypt(encrypted)
		if err != nil {
			return nil, err
		}
		out = append(out, secret.deepCopy())
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Metadata.Name < out[j].Metadata.Name
	})
	return out, nil
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

func (s *EncryptedFileStore) Replace(_ context.Context, secret SecretResource) error {
	normalized := secret.Normalized()
	if err := ValidateCreate(normalized); err != nil {
		return err
	}
	if _, err := s.read(normalized.Metadata.Name); err != nil {
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

func (s *EncryptedFileStore) writeExclusive(path string, secret EncryptedSecret) error {
	if err := os.MkdirAll(s.dir, 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(secret, "", "  ")
	if err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return ErrSecretExists
		}
		return err
	}
	defer file.Close()
	if _, err := file.Write(data); err != nil {
		os.Remove(path)
		return err
	}
	if err := file.Close(); err != nil {
		os.Remove(path)
		return err
	}
	return nil
}

func (s *EncryptedFileStore) path(name string) string {
	return filepath.Join(s.dir, name+".json")
}
