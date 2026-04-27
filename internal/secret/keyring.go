package secret

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"

	"github.com/zalando/go-keyring"
)

// MasterKeyName is the keyring entry name (under service "spk-cockpit").
const MasterKeyName = "master_key"

// keyringService is the OS keyring service identifier.
const keyringService = "spk-cockpit"

// KeyResolver provides the 32-byte AES-256 master key.
type KeyResolver interface {
	Resolve() ([]byte, error)
}

// EnvResolver reads a base64-encoded 32-byte key from an environment variable.
type EnvResolver struct {
	EnvVar string
}

// NewEnvResolver constructs an EnvResolver. Empty envVar uses SPK_COCKPIT_MASTER_KEY.
func NewEnvResolver(envVar string) *EnvResolver {
	if envVar == "" {
		envVar = "SPK_COCKPIT_MASTER_KEY"
	}
	return &EnvResolver{EnvVar: envVar}
}

// Resolve decodes the base64 env var; missing => error.
func (e *EnvResolver) Resolve() ([]byte, error) {
	raw := os.Getenv(e.EnvVar)
	if raw == "" {
		return nil, fmt.Errorf("env var %s not set", e.EnvVar)
	}
	key, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", e.EnvVar, err)
	}
	if len(key) != 32 {
		return nil, errors.New("master key must be 32 bytes")
	}
	return key, nil
}

// KeyringResolver fetches the master key from the OS keyring; generates one on first use.
type KeyringResolver struct{}

// NewKeyringResolver constructs a KeyringResolver.
func NewKeyringResolver() *KeyringResolver { return &KeyringResolver{} }

// Resolve returns the master key, generating + storing one if absent.
func (k *KeyringResolver) Resolve() ([]byte, error) {
	raw, err := keyring.Get(keyringService, MasterKeyName)
	if err == nil {
		key, decErr := base64.StdEncoding.DecodeString(raw)
		if decErr != nil {
			return nil, fmt.Errorf("decode keyring entry: %w", decErr)
		}
		if len(key) != 32 {
			return nil, errors.New("keyring master key not 32 bytes")
		}
		return key, nil
	}
	if !errors.Is(err, keyring.ErrNotFound) {
		return nil, fmt.Errorf("keyring access: %w", err)
	}
	// Generate a new key.
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate key: %w", err)
	}
	if err := keyring.Set(keyringService, MasterKeyName, base64.StdEncoding.EncodeToString(key)); err != nil {
		return nil, fmt.Errorf("store keyring: %w", err)
	}
	return key, nil
}

// ResolveOrFallback tries primary, then fallback. Useful for "keyring with env-var override".
func ResolveOrFallback(primary, fallback KeyResolver) ([]byte, error) {
	if primary != nil {
		key, err := primary.Resolve()
		if err == nil {
			return key, nil
		}
	}
	if fallback == nil {
		return nil, errors.New("no resolver available")
	}
	return fallback.Resolve()
}
