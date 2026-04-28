import { useEffect, useState } from "react";
import { Trash2 } from "lucide-react";
import { useTodoStore } from "../lib/store";
import { api } from "../lib/api";
import { TagPill } from "./TagPill";

export function TagsManager() {
  const { tags, loadTags } = useTodoStore();
  const [creating, setCreating] = useState("");
  const [busy, setBusy] = useState<string | null>(null);

  useEffect(() => {
    void loadTags();
  }, [loadTags]);

  async function setColor(name: string, color: string) {
    setBusy(name);
    try {
      await api.upsertTag(name, color);
      await loadTags();
    } finally {
      setBusy(null);
    }
  }

  async function remove(name: string) {
    if (!confirm(`Delete tag "${name}"? It will also be unlinked from every todo.`)) return;
    setBusy(name);
    try {
      await api.deleteTag(name);
      await loadTags();
    } finally {
      setBusy(null);
    }
  }

  async function create() {
    const name = creating.trim().replace(/^#/, "");
    if (!name) return;
    setBusy(name);
    try {
      await api.upsertTag(name, "#89b4fa");
      setCreating("");
      await loadTags();
    } finally {
      setBusy(null);
    }
  }

  return (
    <div className="flex flex-col gap-2">
      <div className="flex gap-2 items-center">
        <input
          value={creating}
          onChange={(e) => setCreating(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") {
              e.preventDefault();
              void create();
            }
          }}
          placeholder="new tag name"
          className="flex-1 max-w-xs bg-bgsub border border-bgmute rounded px-3 py-1.5 text-fg focus:outline-none focus:border-accent text-sm"
        />
        <button
          onClick={create}
          disabled={!creating.trim() || busy !== null}
          className="px-3 py-1.5 bg-accent text-bg rounded text-sm disabled:opacity-50"
        >
          Add
        </button>
      </div>
      {tags.length === 0 ? (
        <div className="text-fgmute text-sm">no tags yet</div>
      ) : (
        <ul className="flex flex-col gap-1">
          {tags.map((t) => {
            const c = /^#[0-9a-fA-F]{6}$/.test(t.color) ? t.color : "#89b4fa";
            return (
              <li
                key={t.name}
                className="flex items-center gap-3 py-1.5 px-2 rounded hover:bg-bgsub group"
              >
                <span className="flex-1">
                  <TagPill name={t.name} color={c} />
                </span>
                <input
                  type="color"
                  value={c}
                  onChange={(e) => setColor(t.name, e.target.value)}
                  disabled={busy === t.name}
                  className="w-8 h-8 bg-transparent border border-bgmute rounded cursor-pointer"
                  title="Tag color"
                />
                <button
                  onClick={() => remove(t.name)}
                  disabled={busy === t.name}
                  className="opacity-0 group-hover:opacity-100 text-fgmute hover:text-urgent"
                  aria-label={`Delete ${t.name}`}
                >
                  <Trash2 size={16} />
                </button>
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
}
