import { Check, Trash2, Play, Square } from "lucide-react";
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
  onToggleDone: (todo: Todo) => void;
  onDelete: (todo: Todo) => void;
  onStartTimer: (todo: Todo) => void;
  onStopTimer: (todo: Todo) => void;
}

export function TodoRow({
  todo,
  activeTimerStartedAt,
  onToggleDone,
  onDelete,
  onStartTimer,
  onStopTimer,
}: TodoRowProps) {
  const isDone = todo.status === "done";
  const hasTimer = activeTimerStartedAt !== null;
  return (
    <div className="flex items-center gap-3 p-3 rounded hover:bg-bgsub group">
      <button
        onClick={() => onToggleDone(todo)}
        className={`w-5 h-5 rounded border ${isDone ? "bg-success border-success" : "border-fgmute"} flex items-center justify-center`}
        aria-label={isDone ? "Mark as open" : "Mark as done"}
      >
        {isDone && <Check size={14} className="text-bg" />}
      </button>
      <span className={priorityClass[todo.priority]}>{priorityGlyph[todo.priority]}</span>
      <span className={`flex-1 ${isDone ? "line-through text-fgmute" : ""}`}>
        {todo.title}
      </span>
      {hasTimer && <TimerBadge startedAt={activeTimerStartedAt!} />}
      <div className="flex gap-1">
        {todo.tags.map((t) => (
          <TagPill key={t} name={t} />
        ))}
      </div>
      {hasTimer ? (
        <button
          onClick={() => onStopTimer(todo)}
          className="text-urgent hover:text-fg"
          aria-label="Stop timer"
        >
          <Square size={16} />
        </button>
      ) : (
        <button
          onClick={() => onStartTimer(todo)}
          className="opacity-0 group-hover:opacity-100 text-fgmute hover:text-accent"
          aria-label="Start timer"
          disabled={isDone}
        >
          <Play size={16} />
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
