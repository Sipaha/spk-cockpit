import { create } from "zustand";
import { api } from "./api";
import type { Todo, ApiEvent, TimerSession } from "./types";

interface TodoState {
  todos: Todo[];
  loading: boolean;
  includeDone: boolean;
  error: string | null;

  activeTimer: TimerSession | null;

  load: () => Promise<void>;
  setIncludeDone: (v: boolean) => void;
  applyEvent: (e: ApiEvent) => void;
  loadActiveTimer: () => Promise<void>;
}

export const useTodoStore = create<TodoState>((set, get) => ({
  todos: [],
  loading: false,
  includeDone: false,
  error: null,
  activeTimer: null,

  async load() {
    set({ loading: true, error: null });
    try {
      const todos = await api.listTodos(get().includeDone);
      set({ todos, loading: false });
    } catch (e) {
      set({ error: (e as Error).message, loading: false });
    }
  },
  setIncludeDone(v) {
    set({ includeDone: v });
    void get().load();
  },
  async loadActiveTimer() {
    try {
      const t = await api.activeTimer();
      set({ activeTimer: t });
    } catch {
      set({ activeTimer: null });
    }
  },
  applyEvent(e) {
    if (e.type === "todo.created") {
      const { todo } = e.data as { todo: Todo };
      set({ todos: [todo, ...get().todos] });
    } else if (e.type === "todo.updated") {
      const { todo } = e.data as { todo: Todo };
      set({ todos: get().todos.map((t) => (t.id === todo.id ? todo : t)) });
    } else if (e.type === "todo.deleted") {
      const { todoId } = e.data as { todoId: string };
      set({ todos: get().todos.filter((t) => t.id !== todoId) });
    } else if (e.type === "timer.started") {
      const d = e.data as { todoId: string; sessionId: number; startedAt: number };
      set({
        activeTimer: {
          id: d.sessionId,
          todoId: d.todoId,
          startedAt: d.startedAt,
          source: "manual",
        },
      });
    } else if (e.type === "timer.stopped") {
      set({ activeTimer: null });
    }
  },
}));
