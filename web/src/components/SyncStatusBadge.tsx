import { Check, AlertCircle } from "lucide-react";
import type { SyncStateEntry } from "../lib/types";

export function SyncStatusBadge({ state }: { state: SyncStateEntry }) {
  if (state.lastErr) {
    return (
      <span className="inline-flex items-center gap-1 text-urgent text-xs">
        <AlertCircle size={12} />
        {state.lastErr}
      </span>
    );
  }
  if (state.lastOkAt) {
    const ago = Math.max(0, Math.round((Date.now() / 1000 - state.lastOkAt) / 60));
    return (
      <span className="inline-flex items-center gap-1 text-success text-xs">
        <Check size={12} />
        {ago < 1 ? "just synced" : `synced ${ago}m ago`}
      </span>
    );
  }
  return <span className="text-fgmute text-xs">never synced</span>;
}
