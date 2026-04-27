// Package fakerepo provides an in-memory secret.SecretRepo for tests.
package fakerepo

import (
	"context"
	"sort"
	"sync"

	"github.com/spk/spk-cockpit/internal/secret"
)

// Secret is an in-memory secret.SecretRepo.
type Secret struct {
	mu   sync.Mutex
	rows map[string]secret.EncryptedSecret
}

// NewSecret constructs an empty in-memory secret repo.
func NewSecret() *Secret { return &Secret{rows: map[string]secret.EncryptedSecret{}} }

// Get returns the encrypted secret.
func (r *Secret) Get(_ context.Context, name string) (secret.EncryptedSecret, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	v, ok := r.rows[name]
	if !ok {
		return secret.EncryptedSecret{}, secret.ErrNotFound
	}
	return v, nil
}

// Set upserts the secret.
func (r *Secret) Set(_ context.Context, s secret.EncryptedSecret) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rows[s.Name] = s
	return nil
}

// Delete removes the secret.
func (r *Secret) Delete(_ context.Context, name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.rows, name)
	return nil
}

// ListNames returns sorted secret names.
func (r *Secret) ListNames(_ context.Context) ([]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, 0, len(r.rows))
	for k := range r.rows {
		out = append(out, k)
	}
	sort.Strings(out)
	return out, nil
}
