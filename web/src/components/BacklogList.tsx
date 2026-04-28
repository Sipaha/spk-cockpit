import { useMemo, useState } from "react";
import { ArrowUp } from "lucide-react";
import { useTodoStore } from "../lib/store";
import { api } from "../lib/api";
import { TagPill } from "./TagPill";
import { renderSmart } from "../lib/smartText";
import type { Todo } from "../lib/types";

const TITLE_MAX = 100;
function firstLine(s: string): string {
  const nl = s.indexOf("\n");
  const line = nl === -1 ? s : s.slice(0, nl);
  return line.length > TITLE_MAX ? line.slice(0, TITLE_MAX) + "…" : line;
}

// Renders the list of todos parked in the backlog (status = "backlog") and
// gives each row a one-click "Promote to To Do" action that flips status
// back to open. Reads from the global todos store so the SSE event stream
// keeps the list live without a refetch.
export function BacklogList() {
  const todos = useTodoStore((s) => s.todos);
  const taskPatterns = useTodoStore((s) => s.taskPatterns);
  const [busyId, setBusyId] = useState<string | null>(null);

  const items = useMemo(
    () =>
      todos
        .filter((t) => t.status === "backlog")
        .sort((a, b) => b.sortOrder - a.sortOrder),
    [todos],
  );

  async function promote(t: Todo) {
    setBusyId(t.id);
    try {
      await api.updateTodo(t.id, { status: "open" });
    } finally {
      setBusyId(null);
    }
  }

  if (items.length === 0) {
    return <div className="text-fgmute py-8 text-center">backlog is empty</div>;
  }
  return (
    <ul className="flex flex-col gap-1 max-h-96 overflow-auto">
      {items.map((t) => (
        <li
          key={t.id}
          className="flex items-center gap-3 p-2 rounded hover:bg-bgsub group"
        >
          <span className="flex-1 min-w-0 truncate">
            {renderSmart(firstLine(t.title), taskPatterns)}
          </span>
          <div className="flex gap-1">
            {(t.tags ?? []).map((tag) => (
              <TagPill key={tag} name={tag} />
            ))}
          </div>
          <button
            onClick={() => void promote(t)}
            disabled={busyId === t.id}
            className="opacity-0 group-hover:opacity-100 text-accent hover:underline text-sm flex items-center gap-1 shrink-0"
            title="Move to To Do"
          >
            <ArrowUp size={14} /> To Do
          </button>
        </li>
      ))}
    </ul>
  );
}
