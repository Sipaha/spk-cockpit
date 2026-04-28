import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { api } from "../lib/api";

export function Settings() {
  const [defaultNotifyMin, setDefaultNotifyMin] = useState("5");
  const [defaultPopupMin, setDefaultPopupMin] = useState("1");
  const [trackerTemplate, setTrackerTemplate] = useState("");
  const [savingNotify, setSavingNotify] = useState(false);
  const [savingPopup, setSavingPopup] = useState(false);
  const [savingTracker, setSavingTracker] = useState(false);
  const [savedAt, setSavedAt] = useState<string | null>(null);

  useEffect(() => {
    void api.getKv("meeting.default_notify_min").then((r) => r.value && setDefaultNotifyMin(r.value));
    void api.getKv("meeting.default_popup_min").then((r) => r.value && setDefaultPopupMin(r.value));
    void api.getKv("tracker.url_template").then((r) => setTrackerTemplate(r.value ?? ""));
  }, []);

  async function saveTracker() {
    setSavingTracker(true);
    try {
      await api.setKv("tracker.url_template", trackerTemplate.trim());
      setSavedAt(new Date().toLocaleTimeString());
    } finally {
      setSavingTracker(false);
    }
  }

  async function saveNotify() {
    setSavingNotify(true);
    try {
      await api.setKv("meeting.default_notify_min", defaultNotifyMin);
      setSavedAt(new Date().toLocaleTimeString());
    } finally {
      setSavingNotify(false);
    }
  }

  async function savePopup() {
    setSavingPopup(true);
    try {
      await api.setKv("meeting.default_popup_min", defaultPopupMin);
      setSavedAt(new Date().toLocaleTimeString());
    } finally {
      setSavingPopup(false);
    }
  }

  return (
    <div className="flex flex-col gap-8 max-w-2xl">
      <h2 className="text-xl font-semibold">Settings</h2>

      <section className="flex flex-col gap-3">
        <h3 className="text-fgmute uppercase text-xs">Calendar</h3>
        <p className="text-fgmute text-sm">
          CalDAV credentials and sync controls live on the{" "}
          <Link to="/calendar" className="text-accent hover:underline">
            Calendar page
          </Link>{" "}
          (gear icon in the header).
        </p>
      </section>

      <section className="flex flex-col gap-3">
        <h3 className="text-fgmute uppercase text-xs">Notifications</h3>
        <label className="flex flex-col gap-1 max-w-xs">
          <span className="text-sm">DBus notification — minutes before meeting</span>
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
            onClick={saveNotify}
            disabled={savingNotify}
            className="px-3 py-1 bg-accent text-bg rounded text-sm"
          >
            {savingNotify ? "saving…" : "save"}
          </button>
        </div>

        <label className="flex flex-col gap-1 max-w-xs mt-3">
          <span className="text-sm">Popup window — minutes before meeting</span>
          <input
            type="number"
            min={0}
            value={defaultPopupMin}
            onChange={(e) => setDefaultPopupMin(e.target.value)}
            className="bg-bgsub border border-bgmute rounded px-3 py-2 focus:outline-none focus:border-accent text-fg"
          />
        </label>
        <div>
          <button
            onClick={savePopup}
            disabled={savingPopup}
            className="px-3 py-1 bg-accent text-bg rounded text-sm"
          >
            {savingPopup ? "saving…" : "save"}
          </button>
        </div>
      </section>

      <section className="flex flex-col gap-3">
        <h3 className="text-fgmute uppercase text-xs">Task tracker</h3>
        <p className="text-fgmute text-sm">
          URL template that turns ticket ids in todo cards into clickable links.
          Use{" "}
          <code className="text-fg">{"{id}"}</code> for the full ticket
          (e.g. COREDEV-197) and{" "}
          <code className="text-fg">{"{project}"}</code> for the project prefix
          (COREDEV).
        </p>
        <label className="flex flex-col gap-1">
          <span className="text-sm">Ticket URL template</span>
          <input
            type="text"
            value={trackerTemplate}
            onChange={(e) => setTrackerTemplate(e.target.value)}
            placeholder="https://jira.example.com/browse/{id}"
            className="bg-bgsub border border-bgmute rounded px-3 py-2 focus:outline-none focus:border-accent text-fg font-mono text-sm"
          />
        </label>
        <p className="text-fgmute text-xs">
          Example for Jira: <code>https://jira.example.com/browse/{"{id}"}</code>
          {" · "}
          for Citeck: <code>https://citeck.ecos24.ru/v2/dashboard?ws={"{project}"}&recordRef=emodel/ept-issue@{"{id}"}</code>
          {" · "}leave empty to disable.
        </p>
        <div>
          <button
            onClick={saveTracker}
            disabled={savingTracker}
            className="px-3 py-1 bg-accent text-bg rounded text-sm"
          >
            {savingTracker ? "saving…" : "save"}
          </button>
        </div>
      </section>

      {savedAt && <div className="text-fgmute text-xs">saved at {savedAt}</div>}
    </div>
  );
}
