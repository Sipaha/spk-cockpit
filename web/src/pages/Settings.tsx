import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { Plus, Trash2 } from "lucide-react";
import { api } from "../lib/api";
import { DEFAULT_TASK_PATTERN } from "../lib/smartText";
import type { TaskPattern } from "../lib/smartText";
import { parseTaskPatterns } from "../lib/patternUtils";

const TASK_PATTERNS_KEY = "tracker.patterns";

export function Settings() {
  const [taskPatterns, setTaskPatterns] = useState<TaskPattern[]>([]);
  const [savingTracker, setSavingTracker] = useState(false);
  const [savedAt, setSavedAt] = useState<string | null>(null);

  useEffect(() => {
    void api.getKv(TASK_PATTERNS_KEY).then((r) => setTaskPatterns(parseTaskPatterns(r.value)));
  }, []);

  async function saveTracker() {
    setSavingTracker(true);
    try {
      // Drop fully-empty rows so a forgotten editor leaves no garbage in KV.
      const cleaned = taskPatterns
        .map((p) => ({
          pattern: p.pattern.trim(),
          urlTemplate: p.urlTemplate.trim(),
          name: p.name?.trim() || undefined,
        }))
        .filter((p) => p.urlTemplate);
      await api.setKv(TASK_PATTERNS_KEY, JSON.stringify(cleaned));
      setSavedAt(new Date().toLocaleTimeString());
    } finally {
      setSavingTracker(false);
    }
  }

  function patchRow(i: number, patch: Partial<TaskPattern>) {
    setTaskPatterns((rows) =>
      rows.map((r, idx) => (idx === i ? { ...r, ...patch } : r)),
    );
  }
  function removeRow(i: number) {
    setTaskPatterns((rows) => rows.filter((_, idx) => idx !== i));
  }
  function addRow() {
    setTaskPatterns((rows) => [
      ...rows,
      { pattern: DEFAULT_TASK_PATTERN, urlTemplate: "", name: "" },
    ]);
  }

  return (
    <div className="flex flex-col gap-8 max-w-2xl">
      <h2 className="text-xl font-semibold">Settings</h2>

      <section className="flex flex-col gap-3">
        <h3 className="text-fgmute uppercase text-xs">Calendar</h3>
        <p className="text-fgmute text-sm">
          CalDAV credentials, sync controls, and meeting notification
          defaults all live on the{" "}
          <Link to="/calendar" className="text-accent hover:underline">
            Calendar page
          </Link>{" "}
          (gear icon in the header).
        </p>
      </section>

      <section className="flex flex-col gap-3">
        <h3 className="text-fgmute uppercase text-xs">Task trackers</h3>
        <p className="text-fgmute text-sm">
          Each row is a (regex, URL template) pair that turns task references
          in todo cards into clickable links. Patterns are tried in order;
          the first one that matches wins. Capture groups feed{" "}
          <code className="text-fg">$1</code>, <code className="text-fg">$2</code>…
          in the URL template; <code className="text-fg">$0</code> is the full
          match. <code className="text-fg">$$</code> is a literal dollar.
        </p>
        {taskPatterns.length === 0 && (
          <div className="text-fgmute text-sm">no task trackers configured</div>
        )}
        <ul className="flex flex-col gap-3">
          {taskPatterns.map((row, i) => (
            <li
              key={i}
              className="flex flex-col gap-2 p-3 bg-bgsub rounded border border-bgmute"
            >
              <div className="flex gap-2 items-center">
                <input
                  type="text"
                  value={row.name ?? ""}
                  onChange={(e) => patchRow(i, { name: e.target.value })}
                  placeholder="label (optional, e.g. Jira)"
                  className="flex-1 bg-bg border border-bgmute rounded px-3 py-1.5 text-fg text-sm focus:outline-none focus:border-accent"
                />
                <button
                  onClick={() => removeRow(i)}
                  className="text-fgmute hover:text-urgent"
                  aria-label="Remove tracker"
                >
                  <Trash2 size={16} />
                </button>
              </div>
              <label className="flex flex-col gap-1">
                <span className="text-xs text-fgmute">Regex</span>
                <input
                  type="text"
                  value={row.pattern}
                  onChange={(e) => patchRow(i, { pattern: e.target.value })}
                  placeholder={DEFAULT_TASK_PATTERN}
                  className="bg-bg border border-bgmute rounded px-3 py-1.5 text-fg font-mono text-sm focus:outline-none focus:border-accent"
                />
              </label>
              <label className="flex flex-col gap-1">
                <span className="text-xs text-fgmute">URL template</span>
                <input
                  type="text"
                  value={row.urlTemplate}
                  onChange={(e) => patchRow(i, { urlTemplate: e.target.value })}
                  placeholder="https://example.com/browse/$1"
                  className="bg-bg border border-bgmute rounded px-3 py-1.5 text-fg font-mono text-sm focus:outline-none focus:border-accent"
                />
              </label>
            </li>
          ))}
        </ul>
        <div className="flex gap-2">
          <button
            onClick={addRow}
            className="flex items-center gap-1 px-3 py-1.5 text-fgmute hover:text-fg rounded text-sm border border-bgmute hover:border-fgmute"
          >
            <Plus size={14} /> Add tracker
          </button>
          <button
            onClick={saveTracker}
            disabled={savingTracker}
            className="px-3 py-1.5 bg-accent text-bg rounded text-sm"
          >
            {savingTracker ? "saving…" : "save"}
          </button>
        </div>
        <p className="text-fgmute text-xs leading-relaxed">
          Example — Jira-style:{" "}
          <code>https://tracker.example.com/browse/$1</code>. Default regex{" "}
          <code>{DEFAULT_TASK_PATTERN}</code> matches ids like PROJ-1, ABC_2-50.
        </p>
      </section>

      {savedAt && <div className="text-fgmute text-xs">saved at {savedAt}</div>}
    </div>
  );
}
