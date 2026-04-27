import { useEffect } from "react";
import { useTodoStore } from "../lib/store";
import { api } from "../lib/api";
import { TodoRow } from "./TodoRow";
import type { Todo } from "../lib/types";

export function TodoList() {
  const { todos, loading, error, load, includeDone, setIncludeDone } = useTodoStore();

  useEffect(() => {
    void load();
  }, [load]);

  async function toggleDone(t: Todo) {
    const next = t.status === "done" ? "open" : "done";
    await api.updateTodo(t.id, { status: next });
  }

  async function remove(t: Todo) {
    if (!confirm(`Delete "${t.title}"?`)) return;
    await api.deleteTodo(t.id);
  }

  return (
    <div className="flex flex-col gap-3">
      <div className="flex justify-between items-center">
        <h2 className="text-xl font-semibold">Todos</h2>
        <label className="flex items-center gap-2 text-fgmute text-sm">
          <input
            type="checkbox"
            checked={includeDone}
            onChange={(e) => setIncludeDone(e.target.checked)}
          />
          show done
        </label>
      </div>
      {loading && <div className="text-fgmute">loading…</div>}
      {error && <div className="text-urgent">error: {error}</div>}
      <div className="flex flex-col">
        {todos.map((t) => (
          <TodoRow key={t.id} todo={t} onToggleDone={toggleDone} onDelete={remove} />
        ))}
        {!loading && todos.length === 0 && (
          <div className="text-fgmute py-8 text-center">no todos yet</div>
        )}
      </div>
    </div>
  );
}
