import { useEffect } from "react";
import { Link } from "react-router-dom";
import { useTodoStore } from "../lib/store";
import { api } from "../lib/api";
import { TimerBadge } from "../components/TimerBadge";
import type { Todo } from "../lib/types";
import { EventStream } from "../lib/events";

const stream = new EventStream();

export function Popover() {
  // Per-field selectors so unrelated state slices (meetings, tags, etc.)
  // mutating from SSE events don't re-render this hot tray-popover view.
  const todos = useTodoStore((s) => s.todos);
  const activeTimers = useTodoStore((s) => s.activeTimers);
  const load = useTodoStore((s) => s.load);
  const loadActiveTimer = useTodoStore((s) => s.loadActiveTimer);
  const applyEvent = useTodoStore((s) => s.applyEvent);

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

  const open = todos.filter(
    (t) => t.status !== "done" && t.status !== "cancelled" && t.status !== "backlog",
  );
  const top = open.slice(0, 5);
  const primaryActive = activeTimers.length > 0
    ? activeTimers.reduce((a, b) => (a.startedAt < b.startedAt ? a : b))
    : null;
  const activeTodo = primaryActive ? todos.find((t) => t.id === primaryActive.todoId) : null;

  async function startOn(t: Todo) {
    await api.startTimer(t.id);
  }
  async function stopActive() {
    if (!primaryActive) return;
    await api.stopTimer(primaryActive.todoId);
  }
  function isRunning(todoId: string): boolean {
    return activeTimers.some((s) => s.todoId === todoId);
  }

  return (
    <div className="bg-bg text-fg p-3 flex flex-col gap-3 max-w-sm">
      {primaryActive && (
        <div className="flex items-center justify-between bg-bgsub rounded p-2">
          <div className="flex flex-col">
            <span className="text-xs text-fgmute">
              Active timer{activeTimers.length > 1 ? ` (+${activeTimers.length - 1})` : ""}
            </span>
            <span className="text-sm">{activeTodo ? activeTodo.title : "(unknown todo)"}</span>
          </div>
          <div className="flex items-center gap-2">
            <TimerBadge startedAt={primaryActive.startedAt} />
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
            disabled={isRunning(t.id)}
          >
            <span className="truncate">{t.title}</span>
            {isRunning(t.id) ? (
              <span className="text-accent text-xs">running</span>
            ) : (
              <span className="text-fgmute text-xs">&#9654; start</span>
            )}
          </button>
        ))}
      </div>

      <Link to="/" className="text-fgmute text-xs underline self-start">
        Open full window &#8594;
      </Link>
    </div>
  );
}
