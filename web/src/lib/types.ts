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
