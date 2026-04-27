// Package webembed embeds the built React UI (web/dist) into the cockpit binary.
package webembed

import "embed"

// DistFS is the embedded file system at web/dist. The web project must be
// built (pnpm build) before compiling the Go binary, otherwise this is empty.
//
//go:embed all:dist
var DistFS embed.FS
