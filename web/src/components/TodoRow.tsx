import { Pencil, Trash2, X } from "lucide-react";
import type { Todo } from "../lib/types";
import { Priority } from "../lib/types";
import { TagPill } from "./TagPill";
import { TimerBadge } from "./TimerBadge";
import { renderSmart } from "../lib/smartText";
import type { TaskPattern } from "../lib/smartText";

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

function firstLine(s: string): string {
  const nl = s.indexOf("\n");
  return nl === -1 ? s : s.slice(0, nl);
}

export interface TodoRowProps {
  todo: Todo;
  activeTimerStartedAt: number | null;
  taskPatterns: TaskPattern[];
  onDelete: (todo: Todo) => void;
  onView: (todo: Todo) => void;
  onEdit: (todo: Todo) => void;
  onHide?: (todo: Todo) => void;
}

export function TodoRow({
  todo,
  activeTimerStartedAt,
  taskPatterns,
  onDelete,
  onView,
  onEdit,
  onHide,
}: TodoRowProps) {
  const isDone = todo.status === "done";
  const hasTimer = activeTimerStartedAt !== null;
  const canDelete = todo.status === "open";

  const titleLine = firstLine(todo.title);
  const notesLine = todo.notes ? firstLine(todo.notes) : "";

  return (
    <div className="flex items-start gap-3 p-3 rounded hover:bg-bgsub group">
      <span className={`${priorityClass[todo.priority]} pt-0.5`}>
        {priorityGlyph[todo.priority]}
      </span>
      <div
        className="flex-1 min-w-0 cursor-pointer flex flex-col gap-0.5"
        onClick={(e) => {
          e.stopPropagation();
          onView(todo);
        }}
        title="Click to view"
      >
        <div className={`truncate ${isDone ? "line-through text-fgmute" : ""}`}>
          {renderSmart(titleLine, taskPatterns)}
        </div>
        {notesLine && (
          <div className="truncate text-fgmute text-xs">
            {renderSmart(notesLine, taskPatterns)}
          </div>
        )}
      </div>
      {hasTimer && <TimerBadge startedAt={activeTimerStartedAt!} />}
      <div className="flex flex-wrap gap-1 max-w-32 justify-end">
        {(todo.tags ?? []).map((t) => (
          <TagPill key={t} name={t} />
        ))}
      </div>
      <div className="flex flex-col gap-1 self-stretch">
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
    </div>
  );
}
