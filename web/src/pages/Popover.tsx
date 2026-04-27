import { useEffect } from "react";
import { Link } from "react-router-dom";
import { useTodoStore } from "../lib/store";
import { api } from "../lib/api";
import { TimerBadge } from "../components/TimerBadge";
import { AddTodoForm } from "../components/AddTodoForm";
import type { Todo } from "../lib/types";
import { EventStream } from "../lib/events";

const stream = new EventStream();

export function Popover() {
  const {
    todos,
    activeTimer,
    load,
    loadActiveTimer,
    applyEvent,
  } = useTodoStore();

  useEffect(() => {
    void load();
    void loadActiveTimer();
    stream.start();
    const off = stream.on(applyEvent);
    return () => {
      off();
      stream.stop();
    };
  }, [load, loadActiveTimer, applyEvent]);

  const open = todos.filter((t) => t.status !== "done" && t.status !== "cancelled");
  const top = open.slice(0, 5);
  const activeTodo = activeTimer ? todos.find((t) => t.id === activeTimer.todoId) : null;

  async function startOn(t: Todo) {
    await api.startTimer(t.id);
  }
  async function stopActive() {
    await api.stopTimer();
  }

  return (
    <div className="bg-bg text-fg p-3 flex flex-col gap-3 max-w-sm">
      {activeTimer && (
        <div className="flex items-center justify-between bg-bgsub rounded p-2">
          <div className="flex flex-col">
            <span className="text-xs text-fgmute">Active timer</span>
            <span className="text-sm">{activeTodo ? activeTodo.title : "(unknown todo)"}</span>
          </div>
          <div className="flex items-center gap-2">
            <TimerBadge startedAt={activeTimer.startedAt} />
            <button onClick={stopActive} className="text-urgent hover:text-fg text-sm">
              stop
            </button>
          </div>
        </div>
      )}

      <div className="flex flex-col gap-1">
        <div className="flex items-center justify-between">
          <span className="text-fgmute text-xs uppercase">Today</span>
          <span className="text-fgmute text-xs">
            {open.length} open
          </span>
        </div>
        {top.length === 0 && <div className="text-fgmute text-sm py-2">all clear</div>}
        {top.map((t) => (
          <button
            key={t.id}
            onClick={() => startOn(t)}
            className="flex items-center justify-between text-left p-2 rounded hover:bg-bgsub"
            disabled={!!activeTimer && activeTimer.todoId === t.id}
          >
            <span className="truncate">{t.title}</span>
            {!!activeTimer && activeTimer.todoId === t.id ? (
              <span className="text-accent text-xs">running</span>
            ) : (
              <span className="text-fgmute text-xs">&#9654; start</span>
            )}
          </button>
        ))}
      </div>

      <AddTodoForm />

      <Link to="/" className="text-fgmute text-xs underline self-start">
        Open full window &#8594;
      </Link>
    </div>
  );
}
