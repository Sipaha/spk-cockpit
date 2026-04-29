//go:build wails && production

package desktop

// See devtools_dev.go for the rationale. Production builds disable DevTools.
const devToolsEnabled = false
