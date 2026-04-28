import { useEffect } from "react";
import { Pencil } from "lucide-react";
import type { Todo } from "../lib/types";
import { TagPill } from "./TagPill";
import { renderSmart } from "../lib/smartText";
import type { TaskPattern } from "../lib/smartText";

const STATUS_LABEL: Record<string, string> = {
  open: "To Do",
  in_progress: "In Progress",
  done: "Done",
  cancelled: "Cancelled",
};

export interface TodoViewModalProps {
  todo: Todo;
  taskPatterns: TaskPattern[];
  onClose: () => void;
  onEdit: () => void;
}

// Read-only popup that opens on a plain card click. Renders title + notes
// through renderSmart so embedded tracker references stay clickable, and
// hands the user off to the editor modal via the Edit button or pencil
// icon.
export function TodoViewModal({ todo, taskPatterns, onClose, onEdit }: TodoViewModalProps) {
  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") onClose();
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  const created = new Date(todo.createdAt * 1000);
  const updated = new Date(todo.updatedAt * 1000);

  return (
    <div className="fixed inset-0 z-40 bg-black/60 flex items-center justify-center p-6">
      <div className="bg-bgsub border border-bgmute rounded shadow-2xl w-full max-w-2xl flex flex-col gap-3 p-4">
        <div className="flex items-center justify-between">
          <span className="text-fgmute text-xs uppercase tracking-wide">
            {STATUS_LABEL[todo.status] ?? todo.status}
          </span>
          <button
            onClick={onEdit}
            title="Edit"
            aria-label="Edit"
            className="text-fgmute hover:text-accent"
          >
            <Pencil size={16} />
          </button>
        </div>
        <h3 className="text-fg text-lg font-semibold whitespace-pre-wrap break-words">
          {renderSmart(todo.title, taskPatterns)}
        </h3>
        {todo.notes && (
          <div className="text-fg text-sm whitespace-pre-wrap break-words bg-bg border border-bgmute rounded p-3 max-h-72 overflow-auto">
            {renderSmart(todo.notes, taskPatterns)}
          </div>
        )}
        {todo.tags && todo.tags.length > 0 && (
          <div className="flex flex-wrap gap-1">
            {todo.tags.map((t) => (
              <TagPill key={t} name={t} />
            ))}
          </div>
        )}
        <div className="flex flex-wrap gap-x-4 gap-y-1 text-fgmute text-xs pt-2 border-t border-bgmute">
          {todo.dueAt && (
            <span>due: {new Date(todo.dueAt * 1000).toLocaleDateString()}</span>
          )}
          <span>created: {created.toLocaleString()}</span>
          {Math.abs(updated.getTime() - created.getTime()) > 1000 && (
            <span>updated: {updated.toLocaleString()}</span>
          )}
        </div>
        <div className="flex justify-end gap-2">
          <button
            onClick={onClose}
            className="px-3 py-1 text-fgmute hover:text-fg text-sm"
          >
            Close
          </button>
          <button
            onClick={onEdit}
            className="px-3 py-1 bg-accent text-bg rounded text-sm"
          >
            Edit
          </button>
        </div>
      </div>
    </div>
  );
}
