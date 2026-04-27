// Package secret stores AES-256-GCM-encrypted credentials with a master key from
// the OS keyring (production) or an env var (tests).
package secret

import (
	"context"
	"errors"
)

// ErrNotFound is returned when a secret name is unknown.
var ErrNotFound = errors.New("secret: not found")

// EncryptedSecret is the persisted form: ciphertext + nonce, never the plaintext.
type EncryptedSecret struct {
	Name       string
	Ciphertext []byte
	Nonce      []byte
	UpdatedAt  int64
}

// SecretRepo persists encrypted secrets. //nolint:revive // domain naming intentional
type SecretRepo interface { //nolint:revive // domain naming intentional
	Get(ctx context.Context, name string) (EncryptedSecret, error)
	Set(ctx context.Context, s EncryptedSecret) error
	Delete(ctx context.Context, name string) error
	ListNames(ctx context.Context) ([]string, error)
}
