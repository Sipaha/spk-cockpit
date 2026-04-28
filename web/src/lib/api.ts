import type {
  Todo, Tag, CreateTodoRequest, UpdateTodoRequest,
  TimerSession, TodoTimeTotal,
  Meeting, CreateMeetingRequest, UpdateMeetingRequest,
  Note, UpsertNoteRequest,
  Secret, SyncStateEntry,
  StandupReport,
} from "./types";

const BASE = "";

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const resp = await fetch(BASE + path, {
    headers: { "Content-Type": "application/json" },
    ...init,
  });
  if (!resp.ok) {
    let msg = `HTTP ${resp.status}`;
    try {
      const body = await resp.json();
      if (body?.error?.message) msg = body.error.message;
    } catch {
      // ignore
    }
    throw new Error(msg);
  }
  if (resp.status === 204) return undefined as T;
  return (await resp.json()) as T;
}

export const api = {
  listTodos: (includeDone = false) =>
    request<Todo[]>(`/api/todos${includeDone ? "?includeDone=1" : ""}`),
  createTodo: (req: CreateTodoRequest) =>
    request<Todo>("/api/todos", { method: "POST", body: JSON.stringify(req) }),
  updateTodo: (id: string, req: UpdateTodoRequest) =>
    request<Todo>(`/api/todos/${id}`, { method: "PATCH", body: JSON.stringify(req) }),
  deleteTodo: (id: string) =>
    request<void>(`/api/todos/${id}`, { method: "DELETE" }),
  restoreTodo: (id: string) =>
    request<Todo>(`/api/todos/${id}/restore`, { method: "POST" }),
  dismissTodo: (id: string) =>
    request<Todo>(`/api/todos/${id}/dismiss`, { method: "POST" }),
  listDeletedTodos: () => request<Todo[]>(`/api/todos/deleted`),
  startTimer: (todoId: string) =>
    request<TimerSession>("/api/timer/start", {
      method: "POST",
      body: JSON.stringify({ todoId }),
    }),
  stopTimer: (todoId: string) =>
    request<TimerSession>("/api/timer/stop", {
      method: "POST",
      body: JSON.stringify({ todoId }),
    }),
  stopAllTimers: () =>
    request<TimerSession[]>("/api/timer/stop", { method: "POST", body: "{}" }),
  activeTimers: () => request<TimerSession[]>("/api/timer/active"),
  todoTime: (id: string, sinceUnix = 0) =>
    request<TodoTimeTotal>(`/api/todos/${id}/time?since=${sinceUnix}`),
  listTags: () => request<Tag[]>("/api/tags"),
  upsertTag: (name: string, color: string) =>
    request<Tag>(`/api/tags/${encodeURIComponent(name)}`, {
      method: "PUT",
      body: JSON.stringify({ color }),
    }),
  renameTag: (oldName: string, newName: string) =>
    request<void>(`/api/tags/${encodeURIComponent(oldName)}/rename`, {
      method: "POST",
      body: JSON.stringify({ newName }),
    }),
  deleteTag: (name: string) =>
    request<void>(`/api/tags/${encodeURIComponent(name)}`, { method: "DELETE" }),

  listMeetings: (fromUnix: number, toUnix: number, includeCancelled = false) =>
    request<Meeting[]>(
      `/api/meetings?from=${fromUnix}&to=${toUnix}${includeCancelled ? "&includeCancelled=1" : ""}`,
    ),
  nextMeeting: () => request<Meeting | null>("/api/meetings/next"),
  createMeeting: (req: CreateMeetingRequest) =>
    request<Meeting>("/api/meetings", { method: "POST", body: JSON.stringify(req) }),
  updateMeeting: (id: string, req: UpdateMeetingRequest) =>
    request<Meeting>(`/api/meetings/${id}`, { method: "PATCH", body: JSON.stringify(req) }),
  deleteMeeting: (id: string) =>
    request<void>(`/api/meetings/${id}`, { method: "DELETE" }),
  meetingNote: (id: string) => request<Note | null>(`/api/meetings/${id}/note`),
  upsertNote: (req: UpsertNoteRequest) =>
    request<Note>("/api/notes", { method: "PUT", body: JSON.stringify(req) }),
  deleteNote: (id: string) =>
    request<void>(`/api/notes/${id}`, { method: "DELETE" }),
  todoNote: (id: string) => request<Note | null>(`/api/todos/${id}/note`),

  listSecrets: () => request<Secret[]>("/api/secrets"),
  setSecret: (name: string, value: string) =>
    request<void>(`/api/secrets/${encodeURIComponent(name)}`, {
      method: "PUT",
      body: JSON.stringify({ value }),
    }),
  deleteSecret: (name: string) =>
    request<void>(`/api/secrets/${encodeURIComponent(name)}`, { method: "DELETE" }),

  syncStatus: () => request<SyncStateEntry[]>("/api/sync"),
  triggerSync: (source: string) =>
    request<void>(`/api/sync/${encodeURIComponent(source)}`, { method: "POST" }),

  standup: (date?: string) =>
    request<StandupReport>(`/api/standup${date ? `?date=${encodeURIComponent(date)}` : ""}`),

  getKv: (key: string) =>
    request<{ key: string; value: string | null }>(`/api/kv/${encodeURIComponent(key)}`),
  setKv: (key: string, value: string) =>
    request<void>(`/api/kv/${encodeURIComponent(key)}`, {
      method: "PUT",
      body: JSON.stringify({ value }),
    }),
};
