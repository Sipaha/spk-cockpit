import { useEffect, useRef, useState } from "react";
import { Pencil, Trash2, Square } from "lucide-react";
import type { Todo } from "../lib/types";
import { Priority } from "../lib/types";
import { TagPill } from "./TagPill";
import { TimerBadge } from "./TimerBadge";

const priorityClass: Record<number, string> = {
  [Priority.Urgent]: "text-urgent",
  [Priority.High]: "text-high",
  [Priority.Normal]: "text-normal",
  [Priority.Low]: "text-low",
};

const priorityGlyph: Record<number, string> = {
  [Priority.Urgent]: "🔥",
  [Priority.High]: "⚡",
  [Priority.Normal]: "•",
  [Priority.Low]: "▫",
};

export interface TodoRowProps {
  todo: Todo;
  activeTimerStartedAt: number | null;
  onDelete: (todo: Todo) => void;
  onStopTimer: (todo: Todo) => void;
  onRenameTitle: (todo: Todo, title: string) => void;
}

export function TodoRow({
  todo,
  activeTimerStartedAt,
  onDelete,
  onStopTimer,
  onRenameTitle,
}: TodoRowProps) {
  const isDone = todo.status === "done";
  const hasTimer = activeTimerStartedAt !== null;

  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(todo.title);
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (editing) {
      setDraft(todo.title);
      requestAnimationFrame(() => inputRef.current?.select());
    }
  }, [editing, todo.title]);

  function commit() {
    const next = draft.trim();
    if (next && next !== todo.title) onRenameTitle(todo, next);
    setEditing(false);
  }

  return (
    <div className="flex items-center gap-3 p-3 rounded hover:bg-bgsub group">
      <span className={priorityClass[todo.priority]}>{priorityGlyph[todo.priority]}</span>
      {editing ? (
        <input
          ref={inputRef}
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          onBlur={commit}
          onKeyDown={(e) => {
            if (e.key === "Enter") {
              e.preventDefault();
              commit();
            } else if (e.key === "Escape") {
              e.preventDefault();
              setEditing(false);
            }
          }}
          className="flex-1 bg-bgsub border border-bgmute rounded px-2 py-1 text-fg focus:outline-none focus:border-accent"
        />
      ) : (
        <span
          className={`flex-1 cursor-text ${isDone ? "line-through text-fgmute" : ""}`}
          onDoubleClick={() => setEditing(true)}
          title="Double-click to edit"
        >
          {todo.title}
        </span>
      )}
      {hasTimer && <TimerBadge startedAt={activeTimerStartedAt!} />}
      <div className="flex gap-1">
        {(todo.tags ?? []).map((t) => (
          <TagPill key={t} name={t} />
        ))}
      </div>
      {!editing && (
        <button
          onClick={() => setEditing(true)}
          className="opacity-0 group-hover:opacity-100 text-fgmute hover:text-accent"
          aria-label="Edit"
        >
          <Pencil size={16} />
        </button>
      )}
      {hasTimer && (
        // Manual stop is still useful for "interrupt the timer without
        // changing column" — auto-stop fires when leaving In Progress, but
        // sometimes the user wants to pause without moving the card.
        <button
          onClick={() => onStopTimer(todo)}
          className="text-urgent hover:text-fg"
          aria-label="Stop timer"
        >
          <Square size={16} />
        </button>
      )}
      <button
        onClick={() => onDelete(todo)}
        className="opacity-0 group-hover:opacity-100 text-fgmute hover:text-urgent"
        aria-label="Delete"
      >
        <Trash2 size={16} />
      </button>
    </div>
  );
}
