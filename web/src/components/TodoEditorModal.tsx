import { useEffect, useRef, useState } from "react";

export interface TodoEditorModalProps {
  heading: string;
  initialText: string;
  onClose: () => void;
  // First line is the title, rest is notes. The parent decides whether this
  // becomes a create or update; the modal only owns the text editing UX.
  onSave: (title: string, notes: string) => Promise<void> | void;
}

function splitTitleNotes(text: string): { title: string; notes: string } {
  const idx = text.indexOf("\n");
  if (idx === -1) return { title: text.trim(), notes: "" };
  return { title: text.slice(0, idx).trim(), notes: text.slice(idx + 1) };
}

export function TodoEditorModal({
  heading,
  initialText,
  onClose,
  onSave,
}: TodoEditorModalProps) {
  const [text, setText] = useState(initialText);
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
      await onSave(title, notes);
      onClose();
    } finally {
      setSaving(false);
    }
  }

  return (
    <div
      className="fixed inset-0 z-40 bg-black/60 flex items-center justify-center p-6"
      onClick={onClose}
    >
      <div
        className="bg-bgsub border border-bgmute rounded shadow-2xl w-full max-w-2xl flex flex-col gap-3 p-4"
        onClick={(e) => e.stopPropagation()}
      >
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
