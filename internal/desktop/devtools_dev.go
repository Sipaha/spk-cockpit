//go:build wails && !production

package desktop

// devToolsEnabled controls whether the embedded webview ships with DevTools
// open to F12 / right-click → Inspect. Dev builds enable it; production
// builds (built with the `production` tag) flip this in devtools_prod.go.
const devToolsEnabled = true
