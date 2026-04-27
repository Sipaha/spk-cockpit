package secret

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"

	"github.com/spk/spk-cockpit/internal/clock"
)

// Service encrypts/decrypts secrets using AES-256-GCM.
type Service struct {
	repo  SecretRepo
	clock clock.Clock
	gcm   cipher.AEAD
}

// NewService constructs a Service. masterKey must be 32 bytes (AES-256).
func NewService(r SecretRepo, c clock.Clock, masterKey []byte) (*Service, error) {
	if len(masterKey) != 32 {
		return nil, errors.New("master key must be 32 bytes")
	}
	block, err := aes.NewCipher(masterKey)
	if err != nil {
		return nil, fmt.Errorf("aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm: %w", err)
	}
	return &Service{repo: r, clock: c, gcm: gcm}, nil
}

// Set encrypts and stores a secret value.
func (s *Service) Set(ctx context.Context, name, value string) error {
	if name == "" {
		return errors.New("name is required")
	}
	nonce := make([]byte, s.gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return fmt.Errorf("nonce: %w", err)
	}
	ct := s.gcm.Seal(nil, nonce, []byte(value), []byte(name))
	return s.repo.Set(ctx, EncryptedSecret{
		Name: name, Ciphertext: ct, Nonce: nonce, UpdatedAt: s.clock.Now().Unix(),
	})
}

// Get decrypts and returns a secret value.
func (s *Service) Get(ctx context.Context, name string) (string, error) {
	row, err := s.repo.Get(ctx, name)
	if err != nil {
		return "", err
	}
	pt, err := s.gcm.Open(nil, row.Nonce, row.Ciphertext, []byte(row.Name))
	if err != nil {
		return "", fmt.Errorf("decrypt %s: %w", name, err)
	}
	return string(pt), nil
}

// Delete removes a secret.
func (s *Service) Delete(ctx context.Context, name string) error {
	return s.repo.Delete(ctx, name)
}

// ListNames returns sorted secret names (no values).
func (s *Service) ListNames(ctx context.Context) ([]string, error) {
	return s.repo.ListNames(ctx)
}
