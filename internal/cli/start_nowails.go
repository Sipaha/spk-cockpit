//go:build !wails

package cli

import (
	"context"
	"fmt"
)

func runStart(_ context.Context) error {
	return fmt.Errorf("desktop mode requires building with: go build -tags wails ./cmd/cockpit")
}
