package secrets

import (
	"bytes"
	"fmt"
	"regexp"
)

var (
	secretNamePattern = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)
	secretKeyPattern  = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
)

func ValidateCreate(secret SecretResource) error {
	normalized := secret.Normalized()
	if normalized.APIVersion != "" && normalized.APIVersion != "flowforge/v1alpha1" {
		return fmt.Errorf("%w: unsupported apiVersion %q", ErrInvalidSecret, normalized.APIVersion)
	}
	if normalized.Kind != "" && normalized.Kind != "Secret" {
		return fmt.Errorf("%w: unsupported kind %q", ErrInvalidSecret, normalized.Kind)
	}
	if err := validateSecretName(normalized.Metadata.Name); err != nil {
		return err
	}
	if err := validateData(normalized); err != nil {
		return err
	}
	return validateType(normalized)
}

func validateSecretName(name string) error {
	if !secretNamePattern.MatchString(name) {
		return fmt.Errorf("%w: metadata.name must be a DNS label", ErrInvalidSecret)
	}
	return nil
}

func ValidateUpdate(oldSecret, newSecret SecretResource) error {
	oldNormalized := oldSecret.Normalized()
	if oldNormalized.Immutable {
		newNormalized := newSecret.Normalized()
		if !equalBytesMap(oldNormalized.Data, newNormalized.Data) || oldNormalized.Type != newNormalized.Type || !newNormalized.Immutable {
			return ErrSecretImmutable
		}
	}
	return ValidateCreate(newSecret)
}

func validateData(secret SecretResource) error {
	total := 0
	for key, value := range secret.Data {
		if !secretKeyPattern.MatchString(key) {
			return fmt.Errorf("%w: invalid data key %q", ErrInvalidSecret, key)
		}
		total += len(key) + len(value)
		if total > MaxSecretSize {
			return fmt.Errorf("%w: total data size exceeds %d bytes", ErrInvalidSecret, MaxSecretSize)
		}
	}
	return nil
}

func validateType(secret SecretResource) error {
	switch secret.Type {
	case TypeOpaque:
		return nil
	case TypeAPIKey:
		if _, ok := secret.Data["api-key"]; !ok {
			return fmt.Errorf("%w: %s requires key %q", ErrInvalidSecret, TypeAPIKey, "api-key")
		}
		return nil
	default:
		return fmt.Errorf("%w: unsupported secret type %q", ErrInvalidSecret, secret.Type)
	}
}

func equalBytesMap(left, right map[string][]byte) bool {
	if len(left) != len(right) {
		return false
	}
	for key, leftValue := range left {
		rightValue, ok := right[key]
		if !ok || !bytes.Equal(leftValue, rightValue) {
			return false
		}
	}
	return true
}
