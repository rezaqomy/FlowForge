package secrets

import "errors"

var (
	ErrInvalidSecret    = errors.New("invalid secret")
	ErrSecretNotFound   = errors.New("secret not found")
	ErrSecretExists     = errors.New("secret already exists")
	ErrSecretImmutable  = errors.New("secret is immutable")
	ErrSecretKeyMissing = errors.New("secret key missing")
)
