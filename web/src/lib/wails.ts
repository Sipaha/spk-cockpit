import type { MouseEvent } from "react";

// The Wails embedded webview injects window.runtime with native-bridge calls.
// When running outside the bridge (vite dev, a plain browser tab), runtime is
// undefined; callers degrade gracefully.

interface WailsRuntime {
  BrowserOpenURL?: (url: string) => void;
  Quit?: () => void;
}

function bridge(): WailsRuntime | null {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const rt = (window as any).runtime as WailsRuntime | undefined;
  return rt ?? null;
}

// openExternal routes a URL to the system browser when the Wails bridge is
// present. In a plain browser tab the regular href follow-through still works,
// so callers don't need to special-case dev vs production.
export function openExternal(url: string, e?: MouseEvent<HTMLAnchorElement>) {
  const rt = bridge();
  if (rt && typeof rt.BrowserOpenURL === "function") {
    e?.preventDefault();
    rt.BrowserOpenURL(url);
  }
}

// closeWindow asks Wails to quit the current window's process; falls back to
// the browser's own window.close() when the bridge is absent.
export function closeWindow() {
  const rt = bridge();
  if (rt && typeof rt.Quit === "function") {
    rt.Quit();
    return;
  }
  window.close();
}
