export type Priority = 0 | 1 | 2 | 3;
export const Priority = { Low: 0, Normal: 1, High: 2, Urgent: 3 } as const;

export type TodoStatus = "open" | "in_progress" | "done" | "cancelled";

export interface Todo {
  id: string;
  title: string;
  notes: string;
  priority: Priority;
  status: TodoStatus;
  dueAt?: number;
  tags: string[];
  createdAt: number;
  updatedAt: number;
  doneAt?: number;
}

export interface Tag {
  name: string;
  color: string;
  createdAt: number;
}

export interface CreateTodoRequest {
  title: string;
  notes?: string;
  priority: Priority;
  dueAt?: number;
  tags?: string[];
}

export interface UpdateTodoRequest {
  title?: string;
  notes?: string;
  priority?: Priority;
  status?: TodoStatus;
  dueAt?: number;
  tags?: string[];
}

export interface ApiEvent<T = unknown> {
  type: string;
  data: T;
}

export interface TimerSession {
  id: number;
  todoId: string;
  startedAt: number;
  endedAt?: number;
  source: string;
}

export interface TodoTimeTotal {
  todoId: string;
  sinceUnix: number;
  totalSec: number;
  sessionCount: number;
  hasActive: boolean;
}

export interface StartTimerRequest {
  todoId: string;
}

export type MeetingSource = "manual" | "caldav";

export interface Meeting {
  id: string;
  source: MeetingSource;
  externalUid?: string;
  externalEtag?: string;
  title: string;
  description: string;
  location: string;
  startAt: number;
  endAt: number;
  notifyMin?: number;
  notifiedAt?: number;
  cancelled: boolean;
  createdAt: number;
  updatedAt: number;
}

export interface CreateMeetingRequest {
  title: string;
  description?: string;
  location?: string;
  startAt: number;
  endAt: number;
  notifyMin?: number;
}

export interface UpdateMeetingRequest {
  title?: string;
  description?: string;
  location?: string;
  startAt?: number;
  endAt?: number;
  notifyMin?: number;
}

export interface Note {
  id: string;
  meetingId?: string;
  todoId?: string;
  body: string;
  createdAt: number;
  updatedAt: number;
}

export interface UpsertNoteRequest {
  meetingId?: string;
  todoId?: string;
  body: string;
}

export interface Secret {
  name: string;
  updatedAt: number;
}

export interface SyncStateEntry {
  source: string;
  cursor: string;
  lastOkAt?: number;
  lastErr?: string;
}
