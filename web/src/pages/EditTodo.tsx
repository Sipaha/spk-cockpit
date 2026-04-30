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

interface Snapshot {
  value: string;
  selStart: number;
  selEnd: number;
}

export function EditTodo() {
  const [params] = useSearchParams();
  const todoId = params.get("id") ?? "";

  const [tags, setTags] = useState<string[]>([]);
  const [priority, setPriority] = useState<P>(Priority.Normal);
  const [tagSuggestions, setTagSuggestions] = useState<string[]>([]);
  const [saving, setSaving] = useState(false);
  // initialText drives BOTH gating (don't render textarea until known) and
  // the textarea's defaultValue when it finally mounts. Keeping it null
  // until either (a) create flow starts (set to "") or (b) edit-fetch
  // resolves means the textarea mounts ONCE with the right defaultValue,
  // so React's uncontrolled-input semantics actually work. Setting
  // ref.current.value AFTER an empty mount loses the initial value as soon
  // as the user touches it (and was the cause of "subsequent opens show
  // empty content").
  const [initialText, setInitialText] = useState<string | null>(todoId ? null : "");
  const [err, setErr] = useState<string | null>(null);

  const ref = useRef<HTMLTextAreaElement>(null);

  // Manual undo/redo stack. WebKit2GTK in the v3 alpha.78 webview doesn't
  // honour Ctrl+Z / Ctrl+Y on plain <textarea> — neither the native binding
  // nor document.execCommand("undo") drives the input element's history.
  // We snapshot the value+selection on every meaningful edit and replay
  // them on the shortcut.
  const undoRef = useRef<Snapshot[]>([]);
  const redoRef = useRef<Snapshot[]>([]);

  // Load tag suggestions once on mount (independent of shared store state).
  useEffect(() => {
    api.listTags().then((ts) => setTagSuggestions(ts.map((t) => t.name))).catch(() => {});
  }, []);

  // If editing, fetch the existing todo. Pre-populate state ONLY — the
  // textarea reads initialText at mount via defaultValue (see above).
  useEffect(() => {
    if (!todoId) return;
    api
      .getTodo(todoId)
      .then((todo) => {
        const txt = todo.title + (todo.notes ? "\n" + todo.notes : "");
        setTags(todo.tags ?? []);
        setPriority(todo.priority);
        setInitialText(txt);
      })
      .catch((e) => {
        setErr(e instanceof Error ? e.message : String(e));
        setInitialText(""); // unblock the editor in error state
      });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Once the textarea exists with a known initialText, focus and place
  // cursor at end of title.
  useEffect(() => {
    if (initialText === null) return;
    const el = ref.current;
    if (!el) return;
    el.focus();
    const titleEnd = initialText.indexOf("\n");
    const pos = titleEnd === -1 ? initialText.length : titleEnd;
    el.setSelectionRange(pos, pos);
  }, [initialText]);

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
    // Custom undo: pop pre-edit snapshot from undoRef, push current state
    // to redoRef so Ctrl+Y can return.
    if ((e.ctrlKey || e.metaKey) && !e.shiftKey && (e.key === "z" || e.key === "Z")) {
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
      ((e.shiftKey && (e.key === "z" || e.key === "Z")) || e.key === "y" || e.key === "Y")
    ) {
      e.preventDefault();
      const snap = redoRef.current.pop();
      if (snap) {
        snapshot(undoRef);
        restore(snap);
      }
      return;
    }
    // For any value-mutating keystroke, snapshot the PRE-edit state so a
    // subsequent Ctrl+Z can return to it. We don't try to coalesce
    // consecutive characters into a single undo entry (vscode-style) —
    // char-by-char is acceptable for this small editor.
    const isPrintable = e.key.length === 1 && !e.ctrlKey && !e.metaKey && !e.altKey;
    const isEdit = isPrintable || e.key === "Backspace" || e.key === "Delete" || e.key === "Enter" || e.key === "Tab";
    if (isEdit) {
      snapshot(undoRef);
      redoRef.current = []; // any new edit invalidates redo history
    }
  }

  if (initialText === null) {
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
        defaultValue={initialText}
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
