import { useState } from "react";
import { Priority } from "../lib/types";
import { api } from "../lib/api";
import { parseQuickAdd } from "../lib/parser";

export function AddTodoForm() {
  const [input, setInput] = useState("");
  const [busy, setBusy] = useState(false);

  const preview = input ? parseQuickAdd(input) : null;
  const isValid = preview !== null && preview.title.length > 0;

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!isValid) return;
    setBusy(true);
    try {
      await api.createTodo({
        title: preview!.title,
        priority: preview!.priority ?? Priority.Normal,
        tags: preview!.tags.length > 0 ? preview!.tags : undefined,
        dueAt: preview!.dueAt,
      });
      setInput("");
    } finally {
      setBusy(false);
    }
  }

  return (
    <form onSubmit={submit} className="flex flex-col gap-1">
      <input
        type="text"
        value={input}
        onChange={(e) => setInput(e.target.value)}
        placeholder='+ Add todo (e.g. "Fix bug !urgent #backend due:tomorrow")'
        className="flex-1 bg-bgsub border border-bgmute rounded px-3 py-2 focus:outline-none focus:border-accent text-fg"
        disabled={busy}
      />
      {preview && input.length > 0 && (
        <div className="text-xs text-fgmute pl-2 flex gap-3 flex-wrap">
          <span>title: <span className="text-fg">{preview.title || "(empty)"}</span></span>
          {preview.priority !== Priority.Normal && (
            <span>!{labelFor(preview.priority)}</span>
          )}
          {preview.tags.length > 0 && <span>tags: {preview.tags.map((t) => `#${t}`).join(" ")}</span>}
          {preview.dueAt && <span>due: {new Date(preview.dueAt * 1000).toLocaleString()}</span>}
        </div>
      )}
    </form>
  );
}

function labelFor(p: number): string {
  switch (p) {
    case Priority.Low:
      return "low";
    case Priority.High:
      return "high";
    case Priority.Urgent:
      return "urgent";
    default:
      return "normal";
  }
}
