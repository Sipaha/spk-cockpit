import { useEffect, useState } from "react";
import { api } from "../lib/api";
import { useTodoStore } from "../lib/store";
import { SyncStatusBadge } from "./SyncStatusBadge";

export interface CalDAVSettingsProps {
  onSaved?: () => void;
}

// CalDAVSettings is the embeddable form for editing CalDAV URL, username,
// password and triggering an immediate sync. Lives in the Calendar page's
// gear panel; also reused as a standalone configure-CTA from the empty state.
export function CalDAVSettings({ onSaved }: CalDAVSettingsProps) {
  const syncStates = useTodoStore((s) => s.syncStates);
  const loadSyncStatus = useTodoStore((s) => s.loadSyncStatus);
  const [url, setUrl] = useState("");
  const [user, setUser] = useState("");
  const [pass, setPass] = useState("");
  const [saving, setSaving] = useState(false);
  const [savedAt, setSavedAt] = useState<string | null>(null);

  useEffect(() => {
    void loadSyncStatus();
    void api.getKv("caldav.url").then((r) => r.value && setUrl(r.value));
    void api.getKv("caldav.username").then((r) => r.value && setUser(r.value));
  }, [loadSyncStatus]);

  async function save() {
    setSaving(true);
    try {
      await api.setKv("caldav.url", url);
      await api.setKv("caldav.username", user);
      if (pass) {
        await api.setSecret("caldav_password", pass);
        setPass("");
      }
      setSavedAt(new Date().toLocaleTimeString());
      onSaved?.();
    } finally {
      setSaving(false);
    }
  }

  async function syncNow() {
    await api.triggerSync("caldav");
    setTimeout(() => void loadSyncStatus(), 1000);
    onSaved?.();
  }

  const state = syncStates.find((s) => s.source === "caldav");

  return (
    <div className="flex flex-col gap-3">
      <label className="flex flex-col gap-1">
        <span className="text-sm">URL</span>
        <input
          type="text"
          value={url}
          onChange={(e) => setUrl(e.target.value)}
          placeholder="https://caldav.example.com/calendars/you/"
          className="bg-bgsub border border-bgmute rounded px-3 py-2 focus:outline-none focus:border-accent text-fg"
        />
      </label>
      <label className="flex flex-col gap-1">
        <span className="text-sm">Username</span>
        <input
          type="text"
          value={user}
          onChange={(e) => setUser(e.target.value)}
          className="bg-bgsub border border-bgmute rounded px-3 py-2 focus:outline-none focus:border-accent text-fg"
        />
      </label>
      <label className="flex flex-col gap-1">
        <span className="text-sm">Password (leave blank to keep existing)</span>
        <input
          type="password"
          value={pass}
          onChange={(e) => setPass(e.target.value)}
          className="bg-bgsub border border-bgmute rounded px-3 py-2 focus:outline-none focus:border-accent text-fg"
        />
      </label>
      <div className="flex items-center gap-3">
        <button
          onClick={save}
          disabled={saving}
          className="px-3 py-1 bg-accent text-bg rounded text-sm"
        >
          {saving ? "saving…" : "save"}
        </button>
        <button
          onClick={syncNow}
          className="px-3 py-1 bg-bgsub border border-bgmute rounded text-sm hover:border-fgmute"
        >
          sync now
        </button>
        {state && <SyncStatusBadge state={state} />}
        {savedAt && <span className="text-fgmute text-xs">saved at {savedAt}</span>}
      </div>
    </div>
  );
}
