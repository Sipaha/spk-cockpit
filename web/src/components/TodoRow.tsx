import { Inbox, Pencil, Trash2, X } from "lucide-react";
import type { Todo } from "../lib/types";
import { Priority } from "../lib/types";
import { TagPill } from "./TagPill";
import { TimerBadge } from "./TimerBadge";
import { renderSmart } from "../lib/smartText";
import type { TaskPattern } from "../lib/smartText";

// Only High/Urgent actually need the eye-catching edge strip — Normal and
// Low make up the bulk of cards and a strip on every one would just add
// noise. The strip uses inline styles so it paints regardless of which
// color tokens Tailwind has emitted utilities for.
const priorityColor: Record<number, string | null> = {
  [Priority.Urgent]: "var(--color-urgent)",
  [Priority.High]: "var(--color-high)",
  [Priority.Normal]: null,
  [Priority.Low]: null,
};

const priorityLabel: Record<number, string> = {
  [Priority.Urgent]: "Urgent",
  [Priority.High]: "High",
  [Priority.Normal]: "Normal",
  [Priority.Low]: "Low",
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
  // Park a To Do card in the backlog. Only wired up by the parent for
  // status === "open" cards.
  onBacklog?: (todo: Todo) => void;
}

// Trello/Linear-ish card. The left-edge priority strip gives an at-a-glance
// urgency cue without taking horizontal space; tags sit at the top so the
// most filterable axis is read first; the body line-clamps the title to two
// lines and the notes preview to one. Action icons fade in on hover so a
// quiet column doesn't look busy.
export function TodoRow({
  todo,
  activeTimerStartedAt,
  taskPatterns,
  onDelete,
  onView,
  onEdit,
  onHide,
  onBacklog,
}: TodoRowProps) {
  const isDone = todo.status === "done";
  const hasTimer = activeTimerStartedAt !== null;
  const canDelete = todo.status === "open";
  const canBacklog = todo.status === "open" && !!onBacklog;

  const titleLine = firstLine(todo.title);
  const notesLine = todo.notes ? firstLine(todo.notes) : "";

  return (
    <div className="relative flex items-stretch group">
      {/* Priority accent strip on the left edge — color encodes urgency
          without an extra glyph competing with the title. */}
      {priorityColor[todo.priority] && (
        <div
          className="w-1.5 shrink-0 self-stretch"
          style={{ backgroundColor: priorityColor[todo.priority]! }}
          aria-label={`Priority: ${priorityLabel[todo.priority]}`}
          title={priorityLabel[todo.priority]}
        />
      )}
      <div className="flex-1 min-w-0 px-3 py-2.5 flex flex-col gap-1.5">
        {todo.tags && todo.tags.length > 0 && (
          <div className="flex flex-wrap gap-1">
            {todo.tags.map((t) => (
              <TagPill key={t} name={t} />
            ))}
          </div>
        )}
        <div
          className="cursor-pointer"
          onClick={(e) => {
            e.stopPropagation();
            onView(todo);
          }}
          title="Click to view"
        >
          <div
            className={`text-sm font-medium leading-snug line-clamp-2 ${
              isDone ? "line-through text-fgmute" : "text-fg"
            }`}
          >
            {renderSmart(titleLine, taskPatterns)}
          </div>
          {notesLine && (
            <div className="text-xs text-fgmute leading-snug line-clamp-1 mt-0.5">
              {renderSmart(notesLine, taskPatterns)}
            </div>
          )}
        </div>
        {hasTimer && (
          <div className="pt-0.5">
            <TimerBadge startedAt={activeTimerStartedAt!} />
          </div>
        )}
      </div>

      {/* Action rail. Icons fade in on card hover; absolute keeps them out
          of the body's flex math so the title's line-clamp count is stable. */}
      <div className="absolute right-1.5 top-1.5 flex items-center gap-0.5 opacity-0 group-hover:opacity-100 transition-opacity bg-bg/90 backdrop-blur-sm rounded px-0.5">
        <IconBtn
          icon={<Pencil size={14} />}
          label="Edit"
          onClick={() => onEdit(todo)}
          tone="accent"
        />
        {canBacklog && (
          <IconBtn
            icon={<Inbox size={14} />}
            label="Move to backlog"
            onClick={() => onBacklog!(todo)}
          />
        )}
        {canDelete && (
          <IconBtn
            icon={<Trash2 size={14} />}
            label="Delete"
            onClick={() => onDelete(todo)}
            tone="urgent"
          />
        )}
        {isDone && onHide && (
          <IconBtn
            icon={<X size={14} />}
            label="Hide from Done"
            onClick={() => onHide(todo)}
          />
        )}
      </div>
    </div>
  );
}

function IconBtn({
  icon,
  label,
  onClick,
  tone = "neutral",
}: {
  icon: React.ReactNode;
  label: string;
  onClick: () => void;
  tone?: "neutral" | "accent" | "urgent";
}) {
  const hoverColor =
    tone === "accent"
      ? "hover:text-accent"
      : tone === "urgent"
        ? "hover:text-urgent"
        : "hover:text-fg";
  return (
    <button
      type="button"
      onClick={(e) => {
        e.stopPropagation();
        onClick();
      }}
      className={`p-1 rounded text-fgmute ${hoverColor} hover:bg-bgmute`}
      aria-label={label}
      title={label}
    >
      {icon}
    </button>
  );
}
