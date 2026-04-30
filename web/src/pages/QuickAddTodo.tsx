import { useEffect, useRef, useState } from "react";
import type { KeyboardEvent } from "react";
import { api } from "../lib/api";
import { Priority } from "../lib/types";
import { closeWindow } from "../lib/wails";

export function QuickAddTodo() {
  const [saving, setSaving] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const ref = useRef<HTMLTextAreaElement>(null);

  // Uncontrolled textarea — preserves browser-native undo/redo (Ctrl+Z /
  // Ctrl+Y). Going controlled (value + onChange) breaks the WebKit2GTK
  // undo stack because React rewrites the DOM value on every keystroke.

  useEffect(() => {
    ref.current?.focus();
  }, []);

  async function save() {
    const text = ref.current?.value ?? "";
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
    // Same shortcut shape as the in-board editor modal: Ctrl/Cmd+Enter
    // commits, plain Enter inserts a newline, Esc dismisses.
    if (e.key === "Enter" && (e.ctrlKey || e.metaKey)) {
      e.preventDefault();
      void save();
      return;
    }
    if (e.key === "Escape") {
      e.preventDefault();
      closeWindow();
      return;
    }
    // WebKit2GTK doesn't bind Ctrl+Z / Ctrl+Y to native undo in embedded
    // webviews; forward via execCommand (still supported by the engine).
    if ((e.ctrlKey || e.metaKey) && !e.shiftKey && (e.key === "z" || e.key === "Z")) {
      e.preventDefault();
      document.execCommand("undo");
      return;
    }
    if (
      (e.ctrlKey || e.metaKey) &&
      ((e.shiftKey && (e.key === "z" || e.key === "Z")) || e.key === "y" || e.key === "Y")
    ) {
      e.preventDefault();
      document.execCommand("redo");
    }
  }

  return (
    <div className="bg-bg text-fg min-h-screen flex flex-col p-4 gap-3">
      <div className="text-fgmute text-xs">
        New todo · Ctrl+Enter to save · Esc to close
      </div>
      <textarea
        ref={ref}
        defaultValue=""
        onKeyDown={onKeyDown}
        disabled={saving}
        placeholder="Title… (next lines become notes)"
        className="flex-1 min-h-32 bg-bgsub border border-bgmute rounded p-3 text-fg font-mono text-sm focus:outline-none focus:border-accent resize-none"
      />
      {err && <div className="text-urgent text-sm">{err}</div>}
    </div>
  );
}
