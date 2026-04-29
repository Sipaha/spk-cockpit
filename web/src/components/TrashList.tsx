import { useEffect, useState } from "react";
import { RotateCcw } from "lucide-react";
import { api } from "../lib/api";
import { TagPill } from "./TagPill";
import type { Todo } from "../lib/types";
import { firstLine } from "../lib/textUtils";

const TITLE_MAX = 100;

// Shared list-of-deleted-todos with per-row Restore. Used both by the
// Trash page and the Trash modal opened from the Todos board.
export function TrashList() {
  const [items, setItems] = useState<Todo[]>([]);
  const [loading, setLoading] = useState(true);
  const [busyId, setBusyId] = useState<string | null>(null);

  async function reload() {
    setLoading(true);
    try {
      setItems(await api.listDeletedTodos());
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void reload();
  }, []);

  async function restore(id: string) {
    setBusyId(id);
    try {
      await api.restoreTodo(id);
      setItems((xs) => xs.filter((t) => t.id !== id));
    } finally {
      setBusyId(null);
    }
  }

  if (loading) return <div className="text-fgmute">loading…</div>;
  if (items.length === 0) {
    return <div className="text-fgmute py-8 text-center">no deleted todos</div>;
  }
  return (
    <ul className="flex flex-col gap-1 max-h-96 overflow-auto">
      {items.map((t) => (
        <li
          key={t.id}
          className="flex items-center gap-3 p-2 rounded hover:bg-bgsub group"
        >
          <span className="flex-1 truncate text-fgmute">{firstLine(t.title, TITLE_MAX)}</span>
          <div className="flex gap-1">
            {(t.tags ?? []).map((tag) => (
              <TagPill key={tag} name={tag} />
            ))}
          </div>
          <button
            onClick={() => void restore(t.id)}
            disabled={busyId === t.id}
            className="opacity-0 group-hover:opacity-100 text-accent hover:underline text-sm flex items-center gap-1"
          >
            <RotateCcw size={14} /> Restore
          </button>
        </li>
      ))}
    </ul>
  );
}
