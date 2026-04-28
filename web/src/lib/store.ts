import { create } from "zustand";
import { api } from "./api";
import type { Todo, Tag, ApiEvent, TimerSession, Meeting, SyncStateEntry } from "./types";

interface AppState {
  todos: Todo[];
  loading: boolean;
  includeDone: boolean;
  error: string | null;
  activeTimers: TimerSession[];

  meetings: Meeting[];
  meetingsLoading: boolean;

  syncStates: SyncStateEntry[];

  tags: Tag[];

  trackerUrlTemplate: string;
  trackerTicketPattern: string;

  load: () => Promise<void>;
  setIncludeDone: (v: boolean) => void;
  loadActiveTimer: () => Promise<void>;
  loadMeetings: (fromUnix: number, toUnix: number) => Promise<void>;
  loadSyncStatus: () => Promise<void>;
  loadTags: () => Promise<void>;
  loadTrackerTemplate: () => Promise<void>;
  applyEvent: (e: ApiEvent) => void;
}

export const useTodoStore = create<AppState>((set, get) => ({
  todos: [],
  loading: false,
  includeDone: false,
  error: null,
  activeTimers: [],
  meetings: [],
  meetingsLoading: false,
  syncStates: [],
  tags: [],
  trackerUrlTemplate: "",
  trackerTicketPattern: "",

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
      const list = await api.activeTimers();
      set({ activeTimers: list ?? [] });
    } catch {
      set({ activeTimers: [] });
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
  async loadTags() {
    try {
      const tags = await api.listTags();
      set({ tags });
    } catch {
      // ignore — UI just shows no colors
    }
  },
  async loadTrackerTemplate() {
    try {
      const [tpl, pat] = await Promise.all([
        api.getKv("tracker.url_template"),
        api.getKv("tracker.ticket_pattern"),
      ]);
      set({
        trackerUrlTemplate: tpl.value ?? "",
        trackerTicketPattern: pat.value ?? "",
      });
    } catch {
      set({ trackerUrlTemplate: "", trackerTicketPattern: "" });
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
      const others = get().activeTimers.filter((t) => t.todoId !== d.todoId);
      set({
        activeTimers: [
          ...others,
          { id: d.sessionId, todoId: d.todoId, startedAt: d.startedAt, source: "manual" },
        ],
      });
    } else if (e.type === "timer.stopped") {
      const d = e.data as { todoId: string };
      set({ activeTimers: get().activeTimers.filter((t) => t.todoId !== d.todoId) });
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
