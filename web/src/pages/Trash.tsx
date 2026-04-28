import { useEffect, useState } from "react";
import { RotateCcw } from "lucide-react";
import { api } from "../lib/api";
import { TagPill } from "../components/TagPill";
import type { Todo } from "../lib/types";

const TITLE_MAX = 100;
function firstLine(s: string): string {
  const nl = s.indexOf("\n");
  const line = nl === -1 ? s : s.slice(0, nl);
  return line.length > TITLE_MAX ? line.slice(0, TITLE_MAX) + "…" : line;
}

export function Trash() {
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

  return (
    <div className="flex flex-col gap-3 max-w-3xl">
      <h2 className="text-xl font-semibold">Trash</h2>
      <p className="text-fgmute text-sm">
        Recently deleted todos. Click Restore to bring one back to the board.
      </p>
      {loading ? (
        <div className="text-fgmute">loading…</div>
      ) : items.length === 0 ? (
        <div className="text-fgmute py-8 text-center">no deleted todos</div>
      ) : (
        <ul className="flex flex-col gap-1">
          {items.map((t) => (
            <li
              key={t.id}
              className="flex items-center gap-3 p-2 rounded hover:bg-bgsub group"
            >
              <span className="flex-1 truncate text-fgmute">{firstLine(t.title)}</span>
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
      )}
    </div>
  );
}
