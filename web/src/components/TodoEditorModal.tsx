import { useEffect, useRef, useState } from "react";
import { Priority } from "../lib/types";
import type { Priority as P } from "../lib/types";
import { TagInput } from "./TagInput";

export interface TodoEditorModalProps {
  heading: string;
  initialText: string;
  initialTags?: string[];
  initialPriority?: P;
  tagSuggestions?: string[];
  onClose: () => void;
  // First line is the title, rest is notes. The parent decides whether this
  // becomes a create or update; the modal only owns the text editing UX.
  onSave: (
    title: string,
    notes: string,
    tags: string[],
    priority: P,
  ) => Promise<void> | void;
}

function splitTitleNotes(text: string): { title: string; notes: string } {
  const idx = text.indexOf("\n");
  if (idx === -1) return { title: text.trim(), notes: "" };
  return { title: text.slice(0, idx).trim(), notes: text.slice(idx + 1) };
}

const PRIORITY_OPTIONS: { value: P; label: string; color: string }[] = [
  { value: Priority.Low, label: "Low", color: "var(--color-low)" },
  { value: Priority.Normal, label: "Normal", color: "var(--color-fgmute)" },
  { value: Priority.High, label: "High", color: "var(--color-high)" },
  { value: Priority.Urgent, label: "Urgent", color: "var(--color-urgent)" },
];

export function TodoEditorModal({
  heading,
  initialText,
  initialTags = [],
  initialPriority = Priority.Normal,
  tagSuggestions = [],
  onClose,
  onSave,
}: TodoEditorModalProps) {
  const [text, setText] = useState(initialText);
  const [tags, setTags] = useState<string[]>(initialTags);
  const [priority, setPriority] = useState<P>(initialPriority);
  const [saving, setSaving] = useState(false);
  const ref = useRef<HTMLTextAreaElement>(null);

  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    el.focus();
    // Cursor at end of title so the user can keep typing without clearing.
    const titleEnd = initialText.indexOf("\n");
    const pos = titleEnd === -1 ? initialText.length : titleEnd;
    el.setSelectionRange(pos, pos);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  async function commit() {
    const { title, notes } = splitTitleNotes(text);
    if (!title) return;
    setSaving(true);
    try {
      await onSave(title, notes, tags, priority);
      onClose();
    } finally {
      setSaving(false);
    }
  }

  return (
    // The backdrop deliberately does NOT close on click: text selection
    // inside the textarea can release the pointer outside the dialog, and
    // we don't want that to dismiss what the user was editing. Only Esc,
    // Cancel, or Save close the modal.
    <div className="fixed inset-0 z-40 bg-black/60 flex items-center justify-center p-6">
      <div className="bg-bgsub border border-bgmute rounded shadow-2xl w-full max-w-2xl flex flex-col gap-3 p-4">
        <div className="text-fgmute text-xs uppercase tracking-wide">{heading}</div>
        <textarea
          ref={ref}
          value={text}
          onChange={(e) => setText(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Escape") {
              e.preventDefault();
              onClose();
              return;
            }
            if (e.key === "Enter" && (e.ctrlKey || e.metaKey)) {
              e.preventDefault();
              void commit();
            }
          }}
          disabled={saving}
          placeholder="Title… (next lines become notes)"
          className="bg-bg border border-bgmute rounded p-3 text-fg font-mono text-sm h-72 focus:outline-none focus:border-accent resize-y"
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
        <div className="text-fgmute text-xs">
          First line is the title; rest becomes notes. Ctrl+Enter saves, Esc cancels.
        </div>
        <div className="flex justify-end gap-2">
          <button
            onClick={onClose}
            disabled={saving}
            className="px-3 py-1 text-fgmute hover:text-fg text-sm"
          >
            Cancel
          </button>
          <button
            onClick={commit}
            disabled={saving}
            className="px-3 py-1 bg-accent text-bg rounded text-sm"
          >
            {saving ? "saving…" : "Save"}
          </button>
        </div>
      </div>
    </div>
  );
}
