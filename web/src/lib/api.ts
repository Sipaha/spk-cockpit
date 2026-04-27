import type { Todo, Tag, CreateTodoRequest, UpdateTodoRequest } from "./types";

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
  listTags: () => request<Tag[]>("/api/tags"),
};
