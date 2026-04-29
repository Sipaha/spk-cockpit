import type { MouseEvent } from "react";
import * as wails from "@wailsio/runtime";

// In Wails v3 the embedded webview's location.protocol is `wails:`. In dev
// (vite, plain browser tab) it's `http:` and the runtime calls are no-ops or
// undefined. Detect once and reuse.
function isWails(): boolean {
  return typeof window !== "undefined" && window.location.protocol === "wails:";
}

// openExternal routes a URL to the system browser. v3 exposes Browser.OpenURL
// directly on the runtime module. In a plain browser tab the regular href
// follow-through still works, so callers don't need to special-case dev vs
// production.
export function openExternal(url: string, e?: MouseEvent<HTMLAnchorElement>) {
  if (isWails() && wails.Browser?.OpenURL) {
    e?.preventDefault();
    wails.Browser.OpenURL(url);
    return;
  }
  // Browser fallback: the anchor's default navigation handles it.
}

// closeWindow asks the current Wails window to close. v3 exposes the active
// window methods on `wails.Window` (the runtime resolves the active window
// automatically). Falls back to `window.close()` for a plain browser tab —
// useful only for the dev server.
export function closeWindow() {
  if (isWails() && wails.Window?.Close) {
    wails.Window.Close();
    return;
  }
  window.close();
}
