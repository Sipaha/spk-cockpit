import { Pencil, Trash2, X } from "lucide-react";
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

const TITLE_MAX = 100;

// firstTitleLine returns the first newline-delimited line of `s`, truncated to
// TITLE_MAX with an ellipsis. Used for the kanban card preview where notes
// and long titles must collapse to a single readable strip.
function firstTitleLine(s: string): string {
  const nl = s.indexOf("\n");
  const line = nl === -1 ? s : s.slice(0, nl);
  return line.length > TITLE_MAX ? line.slice(0, TITLE_MAX) + "…" : line;
}

export interface TodoRowProps {
  todo: Todo;
  activeTimerStartedAt: number | null;
  onDelete: (todo: Todo) => void;
  onEdit: (todo: Todo) => void;
  // Hides a Done card from the board immediately (vs waiting for the
  // 3-day Done-column timeout). Optional; only the Done-column wires it up.
  onHide?: (todo: Todo) => void;
}

export function TodoRow({
  todo,
  activeTimerStartedAt,
  onDelete,
  onEdit,
  onHide,
}: TodoRowProps) {
  const isDone = todo.status === "done";
  const hasTimer = activeTimerStartedAt !== null;
  // Delete only from the "To Do" column. Once a todo's been picked up
  // (in_progress) or finished, dropping it should be a deliberate Trash-page
  // action, not a one-click hover gesture from a busy board.
  const canDelete = todo.status === "open";

  return (
    <div className="flex items-center gap-3 p-3 rounded hover:bg-bgsub group">
      <span className={priorityClass[todo.priority]}>{priorityGlyph[todo.priority]}</span>
      <span
        className={`flex-1 cursor-pointer truncate ${isDone ? "line-through text-fgmute" : ""}`}
        onClick={(e) => {
          e.stopPropagation();
          onEdit(todo);
        }}
        title="Click to edit"
      >
        {firstTitleLine(todo.title)}
      </span>
      {hasTimer && <TimerBadge startedAt={activeTimerStartedAt!} />}
      <div className="flex gap-1">
        {(todo.tags ?? []).map((t) => (
          <TagPill key={t} name={t} />
        ))}
      </div>
      <button
        onClick={(e) => {
          e.stopPropagation();
          onEdit(todo);
        }}
        className="opacity-0 group-hover:opacity-100 text-fgmute hover:text-accent"
        aria-label="Edit"
      >
        <Pencil size={16} />
      </button>
      {canDelete && (
        <button
          onClick={(e) => {
            e.stopPropagation();
            onDelete(todo);
          }}
          className="opacity-0 group-hover:opacity-100 text-fgmute hover:text-urgent"
          aria-label="Delete"
        >
          <Trash2 size={16} />
        </button>
      )}
      {isDone && onHide && (
        <button
          onClick={(e) => {
            e.stopPropagation();
            onHide(todo);
          }}
          className="opacity-0 group-hover:opacity-100 text-fgmute hover:text-fg"
          aria-label="Hide from Done"
          title="Hide from Done"
        >
          <X size={16} />
        </button>
      )}
    </div>
  );
}
