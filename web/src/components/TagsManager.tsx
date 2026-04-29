import { useEffect, useRef, useState } from "react";
import { Pencil, Trash2 } from "lucide-react";
import { useTodoStore } from "../lib/store";
import { api } from "../lib/api";
import { TagPill } from "./TagPill";

export function TagsManager() {
  const tags = useTodoStore((s) => s.tags);
  const loadTags = useTodoStore((s) => s.loadTags);
  const loadTodos = useTodoStore((s) => s.load);
  const [creating, setCreating] = useState("");
  const [busy, setBusy] = useState<string | null>(null);
  const [editingName, setEditingName] = useState<string | null>(null);
  const [draftName, setDraftName] = useState("");
  const editRef = useRef<HTMLInputElement>(null);

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
      // Removing a tag also detaches it from every todo via FK CASCADE,
      // so refresh both caches.
      await Promise.all([loadTags(), loadTodos()]);
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

  function startRename(name: string) {
    setEditingName(name);
    setDraftName(name);
    requestAnimationFrame(() => editRef.current?.select());
  }

  async function commitRename() {
    if (!editingName) return;
    const next = draftName.trim().replace(/^#/, "");
    setEditingName(null);
    if (!next || next === editingName) return;
    setBusy(editingName);
    try {
      await api.renameTag(editingName, next);
      await Promise.all([loadTags(), loadTodos()]);
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
            const isEditing = editingName === t.name;
            return (
              <li
                key={t.name}
                className="flex items-center gap-3 py-1.5 px-2 rounded hover:bg-bgsub group"
              >
                <span className="flex-1 min-w-0 flex items-center gap-2">
                  {isEditing ? (
                    <input
                      ref={editRef}
                      value={draftName}
                      onChange={(e) => setDraftName(e.target.value)}
                      onBlur={() => void commitRename()}
                      onKeyDown={(e) => {
                        if (e.key === "Enter") {
                          e.preventDefault();
                          void commitRename();
                        } else if (e.key === "Escape") {
                          e.preventDefault();
                          setEditingName(null);
                        }
                      }}
                      className="bg-bg border border-bgmute rounded px-2 py-0.5 text-fg text-sm focus:outline-none focus:border-accent"
                    />
                  ) : (
                    <TagPill name={t.name} color={c} />
                  )}
                </span>
                <button
                  onClick={() => startRename(t.name)}
                  disabled={busy === t.name}
                  className="opacity-0 group-hover:opacity-100 text-fgmute hover:text-accent"
                  aria-label={`Rename ${t.name}`}
                  title="Rename"
                >
                  <Pencil size={14} />
                </button>
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
