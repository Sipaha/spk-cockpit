import { useEffect, useRef, useState } from "react";
import type { KeyboardEvent } from "react";
import { api } from "../lib/api";
import { Priority } from "../lib/types";

// Closes the standalone Wails subprocess window. In the embedded webview the
// runtime bridge exposes Quit; in a plain browser tab (vite dev) we fall back
// to window.close().
function closeWindow() {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const rt = (window as any).runtime;
  if (rt && typeof rt.Quit === "function") {
    rt.Quit();
    return;
  }
  window.close();
}

export function QuickAddTodo() {
  const [text, setText] = useState("");
  const [saving, setSaving] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const ref = useRef<HTMLTextAreaElement>(null);

  useEffect(() => {
    ref.current?.focus();
  }, []);

  async function save() {
    const lines = text.split("\n");
    const title = lines[0].trim();
    if (!title) return;
    const notes = lines.slice(1).join("\n").trim();
    setSaving(true);
    setErr(null);
    try {
      await api.createTodo({
        title,
        notes: notes || undefined,
        priority: Priority.Normal,
      });
      closeWindow();
    } catch (e) {
      setSaving(false);
      setErr(e instanceof Error ? e.message : String(e));
    }
  }

  function onKeyDown(e: KeyboardEvent<HTMLTextAreaElement>) {
    // Plain Enter saves; Shift/Ctrl/Cmd+Enter inserts a newline (default
    // textarea behavior). Esc dismisses the window without saving.
    if (e.key === "Enter" && !e.shiftKey && !e.ctrlKey && !e.metaKey) {
      e.preventDefault();
      void save();
      return;
    }
    if (e.key === "Escape") {
      e.preventDefault();
      closeWindow();
    }
  }

  return (
    <div className="bg-bg text-fg min-h-screen flex flex-col p-4 gap-3">
      <div className="text-fgmute text-xs">
        New todo · Enter to save · Ctrl+Enter for newline · Esc to close
      </div>
      <textarea
        ref={ref}
        value={text}
        onChange={(e) => setText(e.target.value)}
        onKeyDown={onKeyDown}
        disabled={saving}
        placeholder="Title… (next lines become notes)"
        className="flex-1 min-h-32 bg-bgsub border border-bgmute rounded p-3 text-fg font-mono text-sm focus:outline-none focus:border-accent resize-none"
      />
      {err && <div className="text-urgent text-sm">{err}</div>}
    </div>
  );
}
