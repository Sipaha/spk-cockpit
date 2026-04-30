import { useEffect, useRef, useState } from "react";
import type { KeyboardEvent } from "react";
import { useSearchParams } from "react-router-dom";
import { api } from "../lib/api";
import { Priority } from "../lib/types";
import type { Priority as P } from "../lib/types";
import { TagInput } from "../components/TagInput";
import { closeWindow } from "../lib/wails";

const PRIORITY_OPTIONS: { value: P; label: string; color: string }[] = [
  { value: Priority.Low, label: "Low", color: "var(--color-low)" },
  { value: Priority.Normal, label: "Normal", color: "var(--color-fgmute)" },
  { value: Priority.High, label: "High", color: "var(--color-high)" },
  { value: Priority.Urgent, label: "Urgent", color: "var(--color-urgent)" },
];

function splitTitleNotes(text: string): { title: string; notes: string } {
  const idx = text.indexOf("\n");
  if (idx === -1) return { title: text.trim(), notes: "" };
  return { title: text.slice(0, idx).trim(), notes: text.slice(idx + 1) };
}

export function EditTodo() {
  const [params] = useSearchParams();
  const todoId = params.get("id") ?? "";

  const [tags, setTags] = useState<string[]>([]);
  const [priority, setPriority] = useState<P>(Priority.Normal);
  const [tagSuggestions, setTagSuggestions] = useState<string[]>([]);
  const [saving, setSaving] = useState(false);
  const [loading, setLoading] = useState(!!todoId);
  const [err, setErr] = useState<string | null>(null);

  // Uncontrolled textarea — preserves browser-native undo/redo (Ctrl+Z /
  // Ctrl+Y). Going controlled (value + onChange) breaks the WebKit2GTK
  // undo stack because React rewrites the DOM value on every keystroke.
  const ref = useRef<HTMLTextAreaElement>(null);

  // Load tag suggestions once on mount (independent of shared store state).
  useEffect(() => {
    api.listTags().then((ts) => setTagSuggestions(ts.map((t) => t.name))).catch(() => {});
  }, []);

  // If editing, fetch the existing todo and pre-populate form fields.
  useEffect(() => {
    if (!todoId) {
      // Create flow: focus the textarea immediately.
      ref.current?.focus();
      return;
    }
    setLoading(true);
    api
      .getTodo(todoId)
      .then((todo) => {
        if (ref.current) {
          const initialText = todo.title + (todo.notes ? "\n" + todo.notes : "");
          ref.current.value = initialText;
          ref.current.focus();
          // Cursor at end of title so user can keep typing without clearing.
          const titleEnd = initialText.indexOf("\n");
          const pos = titleEnd === -1 ? initialText.length : titleEnd;
          ref.current.setSelectionRange(pos, pos);
        }
        setTags(todo.tags ?? []);
        setPriority(todo.priority);
      })
      .catch((e) => {
        setErr(e instanceof Error ? e.message : String(e));
      })
      .finally(() => setLoading(false));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  async function save() {
    const text = ref.current?.value ?? "";
    const { title, notes } = splitTitleNotes(text);
    if (!title) return;
    setSaving(true);
    setErr(null);
    try {
      if (todoId) {
        await api.updateTodo(todoId, { title, notes, tags, priority });
      } else {
        await api.createTodo({
          title,
          notes: notes || undefined,
          tags: tags.length ? tags : undefined,
          priority,
        });
      }
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

  if (loading) {
    return (
      <div className="bg-bg text-fg min-h-screen flex flex-col p-4 gap-3">
        <div className="text-fgmute text-sm">loading…</div>
      </div>
    );
  }

  return (
    <div className="bg-bg text-fg min-h-screen flex flex-col p-4 gap-3">
      <div className="text-fgmute text-xs">
        {todoId ? "Edit todo" : "New todo"} · First line is the title; rest becomes notes · Ctrl+Enter to save · Esc to close
      </div>
      <textarea
        ref={ref}
        defaultValue=""
        onKeyDown={onKeyDown}
        disabled={saving}
        placeholder="Title… (next lines become notes)"
        className="flex-1 min-h-48 bg-bgsub border border-bgmute rounded p-3 text-fg font-mono text-sm focus:outline-none focus:border-accent resize-none"
      />
      <TagInput value={tags} onChange={setTags} suggestions={tagSuggestions} />
      <div className="flex items-center gap-2">
        <span className="text-fgmute text-xs uppercase tracking-wide w-16">Priority</span>
        <div className="flex gap-1">
          {PRIORITY_OPTIONS.map((p) => {
            const selected = priority === p.value;
            return (
              <button
                key={p.value}
                type="button"
                onClick={() => setPriority(p.value)}
                className={`px-2.5 py-1 rounded text-xs border transition-colors ${
                  selected
                    ? "border-transparent text-bg font-medium"
                    : "border-bgmute text-fgmute hover:text-fg hover:border-fgmute"
                }`}
                style={selected ? { backgroundColor: p.color } : undefined}
              >
                {p.label}
              </button>
            );
          })}
        </div>
      </div>
      {err && <div className="text-urgent text-sm">{err}</div>}
      <div className="flex justify-end gap-2">
        <button
          onClick={closeWindow}
          disabled={saving}
          className="px-3 py-1 text-fgmute hover:text-fg text-sm"
        >
          Cancel
        </button>
        <button
          onClick={() => void save()}
          disabled={saving}
          className="px-3 py-1 bg-accent text-bg rounded text-sm"
        >
          {saving ? "saving…" : "Save"}
        </button>
      </div>
    </div>
  );
}
