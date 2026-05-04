import { useEffect, useRef, useState } from "react";
import type { FormEvent, KeyboardEvent } from "react";
import { api } from "../lib/api";
import { Priority } from "../lib/types";
import { closeWindow } from "../lib/wails";

interface Snapshot {
  value: string;
  selStart: number;
  selEnd: number;
}

export function QuickAddTodo() {
  const [saving, setSaving] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const ref = useRef<HTMLTextAreaElement>(null);

  // Uncontrolled textarea — preserves browser-native undo/redo (Ctrl+Z /
  // Ctrl+Y). Going controlled (value + onChange) breaks the WebKit2GTK
  // undo stack because React rewrites the DOM value on every keystroke.

  // Manual undo/redo stack (same model as EditTodo): WebKit2GTK in v3
  // alpha.78 doesn't honour Ctrl+Z on plain <textarea>, neither natively
  // nor via execCommand("undo") (which collapses to "delete everything"
  // in this webview). beforeInput captures all value mutations including
  // paste/IME. Coalescing rule: consecutive plain character insertions
  // within COALESCE_MS share one undo entry (vscode-style).
  const undoRef = useRef<Snapshot[]>([]);
  const redoRef = useRef<Snapshot[]>([]);
  const lastEditAtRef = useRef(0);
  const COALESCE_MS = 500;

  useEffect(() => {
    ref.current?.focus();
  }, []);

  function snapshot(stack: React.MutableRefObject<Snapshot[]>) {
    const el = ref.current;
    if (!el) return;
    stack.current.push({
      value: el.value,
      selStart: el.selectionStart ?? 0,
      selEnd: el.selectionEnd ?? 0,
    });
    if (stack.current.length > 200) stack.current.shift();
  }

  function restore(snap: Snapshot) {
    const el = ref.current;
    if (!el) return;
    el.value = snap.value;
    el.setSelectionRange(snap.selStart, snap.selEnd);
  }

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
    // Custom undo / redo. e.code (physical key) is layout-independent —
    // e.key gives "я" for Ctrl+Z on Russian Cyrillic and never matches.
    if ((e.ctrlKey || e.metaKey) && !e.shiftKey && e.code === "KeyZ") {
      e.preventDefault();
      const snap = undoRef.current.pop();
      if (snap) {
        snapshot(redoRef);
        restore(snap);
      }
      return;
    }
    if (
      (e.ctrlKey || e.metaKey) &&
      ((e.shiftKey && e.code === "KeyZ") || e.code === "KeyY")
    ) {
      e.preventDefault();
      const snap = redoRef.current.pop();
      if (snap) {
        snapshot(undoRef);
        restore(snap);
      }
    }
  }

  function onBeforeInput(e: FormEvent<HTMLTextAreaElement>) {
    const native = e.nativeEvent as InputEvent;
    const now = performance.now();
    // WebKit2GTK leaves inputType empty for plain typing — see EditTodo
    // comment. Heuristic: a non-empty, non-"insertText" inputType (paste,
    // delete-line, drag, etc.) breaks coalescing; otherwise short-interval
    // inputs share one undo entry.
    const inputType = native.inputType ?? "";
    const breakingInputType = inputType !== "" && inputType !== "insertText";
    const recent = now - lastEditAtRef.current < COALESCE_MS;
    if (breakingInputType || !recent || undoRef.current.length === 0) {
      snapshot(undoRef);
      redoRef.current = [];
    }
    lastEditAtRef.current = now;
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
        onBeforeInput={onBeforeInput}
        disabled={saving}
        placeholder="Title… (next lines become notes)"
        className="flex-1 min-h-32 bg-bgsub border border-bgmute rounded p-3 text-fg font-mono text-sm focus:outline-none focus:border-accent resize-none"
      />
      {err && <div className="text-urgent text-sm">{err}</div>}
    </div>
  );
}
