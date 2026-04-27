import { useState } from "react";
import { Priority } from "../lib/types";
import { api } from "../lib/api";

export function AddTodoForm() {
  const [title, setTitle] = useState("");
  const [busy, setBusy] = useState(false);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!title.trim()) return;
    setBusy(true);
    try {
      await api.createTodo({ title: title.trim(), priority: Priority.Normal });
      setTitle("");
    } finally {
      setBusy(false);
    }
  }

  return (
    <form onSubmit={submit} className="flex gap-2 items-center">
      <input
        type="text"
        value={title}
        onChange={(e) => setTitle(e.target.value)}
        placeholder="+ Add todo (Enter to submit)"
        className="flex-1 bg-bgsub border border-bgmute rounded px-3 py-2 focus:outline-none focus:border-accent text-fg"
        disabled={busy}
      />
    </form>
  );
}
