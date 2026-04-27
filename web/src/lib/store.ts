import { create } from "zustand";
import { api } from "./api";
import type { Todo, ApiEvent, TimerSession, Meeting, SyncStateEntry } from "./types";

interface AppState {
  todos: Todo[];
  loading: boolean;
  includeDone: boolean;
  error: string | null;
  activeTimer: TimerSession | null;

  meetings: Meeting[];
  meetingsLoading: boolean;

  syncStates: SyncStateEntry[];

  load: () => Promise<void>;
  setIncludeDone: (v: boolean) => void;
  loadActiveTimer: () => Promise<void>;
  loadMeetings: (fromUnix: number, toUnix: number) => Promise<void>;
  loadSyncStatus: () => Promise<void>;
  applyEvent: (e: ApiEvent) => void;
}

export const useTodoStore = create<AppState>((set, get) => ({
  todos: [],
  loading: false,
  includeDone: false,
  error: null,
  activeTimer: null,
  meetings: [],
  meetingsLoading: false,
  syncStates: [],

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
  async loadMeetings(fromUnix, toUnix) {
    set({ meetingsLoading: true });
    try {
      const list = await api.listMeetings(fromUnix, toUnix);
      set({ meetings: list, meetingsLoading: false });
    } catch {
      set({ meetingsLoading: false });
    }
  },
  async loadSyncStatus() {
    try {
      const list = await api.syncStatus();
      set({ syncStates: list });
    } catch {
      // ignore
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
        activeTimer: { id: d.sessionId, todoId: d.todoId, startedAt: d.startedAt, source: "manual" },
      });
    } else if (e.type === "timer.stopped") {
      set({ activeTimer: null });
    } else if (e.type === "meeting.upserted") {
      const { meeting } = e.data as { meeting: Meeting };
      const others = get().meetings.filter((m) => m.id !== meeting.id);
      set({ meetings: [...others, meeting].sort((a, b) => a.startAt - b.startAt) });
    } else if (e.type === "meeting.deleted") {
      const { meetingId } = e.data as { meetingId: string };
      set({ meetings: get().meetings.filter((m) => m.id !== meetingId) });
    } else if (e.type === "sync.state_changed") {
      void get().loadSyncStatus();
    }
  },
}));
