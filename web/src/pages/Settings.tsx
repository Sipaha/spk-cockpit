import { useEffect, useState } from "react";
import { useTodoStore } from "../lib/store";
import { api } from "../lib/api";
import { SyncStatusBadge } from "../components/SyncStatusBadge";

export function Settings() {
  const { syncStates, loadSyncStatus } = useTodoStore();

  const [caldavUrl, setCaldavUrl] = useState("https://caldav.yandex.ru/");
  const [caldavUser, setCaldavUser] = useState("");
  const [caldavPass, setCaldavPass] = useState("");
  const [defaultNotifyMin, setDefaultNotifyMin] = useState("5");
  const [savingCaldav, setSavingCaldav] = useState(false);
  const [savingNotifyMin, setSavingNotifyMin] = useState(false);
  const [savedAt, setSavedAt] = useState<string | null>(null);

  useEffect(() => {
    void loadSyncStatus();
    void api.getKv("caldav.url").then((r) => r.value && setCaldavUrl(r.value));
    void api.getKv("caldav.username").then((r) => r.value && setCaldavUser(r.value));
    void api.getKv("meeting.default_notify_min").then((r) => r.value && setDefaultNotifyMin(r.value));
  }, [loadSyncStatus]);

  async function saveCaldav() {
    setSavingCaldav(true);
    try {
      await api.setKv("caldav.url", caldavUrl);
      await api.setKv("caldav.username", caldavUser);
      if (caldavPass) {
        await api.setSecret("yandex_caldav", caldavPass);
        setCaldavPass("");
      }
      setSavedAt(new Date().toLocaleTimeString());
    } finally {
      setSavingCaldav(false);
    }
  }

  async function saveNotifyMin() {
    setSavingNotifyMin(true);
    try {
      await api.setKv("meeting.default_notify_min", defaultNotifyMin);
      setSavedAt(new Date().toLocaleTimeString());
    } finally {
      setSavingNotifyMin(false);
    }
  }

  async function syncNow() {
    await api.triggerSync("caldav");
    setTimeout(() => void loadSyncStatus(), 1000);
  }

  const caldavState = syncStates.find((s) => s.source === "caldav");

  return (
    <div className="flex flex-col gap-8 max-w-2xl">
      <h2 className="text-xl font-semibold">Settings</h2>

      <section className="flex flex-col gap-3">
        <h3 className="text-fgmute uppercase text-xs">Yandex CalDAV</h3>
        <label className="flex flex-col gap-1">
          <span className="text-sm">URL</span>
          <input
            type="text"
            value={caldavUrl}
            onChange={(e) => setCaldavUrl(e.target.value)}
            className="bg-bgsub border border-bgmute rounded px-3 py-2 focus:outline-none focus:border-accent text-fg"
          />
        </label>
        <label className="flex flex-col gap-1">
          <span className="text-sm">Username</span>
          <input
            type="text"
            value={caldavUser}
            onChange={(e) => setCaldavUser(e.target.value)}
            className="bg-bgsub border border-bgmute rounded px-3 py-2 focus:outline-none focus:border-accent text-fg"
          />
        </label>
        <label className="flex flex-col gap-1">
          <span className="text-sm">Password (leave blank to keep existing)</span>
          <input
            type="password"
            value={caldavPass}
            onChange={(e) => setCaldavPass(e.target.value)}
            className="bg-bgsub border border-bgmute rounded px-3 py-2 focus:outline-none focus:border-accent text-fg"
          />
        </label>
        <div className="flex items-center gap-3">
          <button
            onClick={saveCaldav}
            disabled={savingCaldav}
            className="px-3 py-1 bg-accent text-bg rounded text-sm"
          >
            {savingCaldav ? "saving…" : "save credentials"}
          </button>
          <button
            onClick={syncNow}
            className="px-3 py-1 bg-bgsub border border-bgmute rounded text-sm hover:border-fgmute"
          >
            sync now
          </button>
          {caldavState && <SyncStatusBadge state={caldavState} />}
        </div>
      </section>

      <section className="flex flex-col gap-3">
        <h3 className="text-fgmute uppercase text-xs">Notifications</h3>
        <label className="flex flex-col gap-1 max-w-xs">
          <span className="text-sm">Default minutes before meeting</span>
          <input
            type="number"
            min={0}
            value={defaultNotifyMin}
            onChange={(e) => setDefaultNotifyMin(e.target.value)}
            className="bg-bgsub border border-bgmute rounded px-3 py-2 focus:outline-none focus:border-accent text-fg"
          />
        </label>
        <div>
          <button
            onClick={saveNotifyMin}
            disabled={savingNotifyMin}
            className="px-3 py-1 bg-accent text-bg rounded text-sm"
          >
            {savingNotifyMin ? "saving…" : "save"}
          </button>
        </div>
      </section>

      {savedAt && <div className="text-fgmute text-xs">saved at {savedAt}</div>}
    </div>
  );
}
