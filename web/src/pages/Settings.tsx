import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { api } from "../lib/api";

export function Settings() {
  const [defaultNotifyMin, setDefaultNotifyMin] = useState("5");
  const [defaultPopupMin, setDefaultPopupMin] = useState("1");
  const [trackerTemplate, setTrackerTemplate] = useState("");
  const [trackerPattern, setTrackerPattern] = useState("");
  const [savingNotify, setSavingNotify] = useState(false);
  const [savingPopup, setSavingPopup] = useState(false);
  const [savingTracker, setSavingTracker] = useState(false);
  const [savedAt, setSavedAt] = useState<string | null>(null);

  useEffect(() => {
    void api.getKv("meeting.default_notify_min").then((r) => r.value && setDefaultNotifyMin(r.value));
    void api.getKv("meeting.default_popup_min").then((r) => r.value && setDefaultPopupMin(r.value));
    void api.getKv("tracker.url_template").then((r) => setTrackerTemplate(r.value ?? ""));
    void api.getKv("tracker.ticket_pattern").then((r) => setTrackerPattern(r.value ?? ""));
  }, []);

  async function saveTracker() {
    setSavingTracker(true);
    try {
      await Promise.all([
        api.setKv("tracker.url_template", trackerTemplate.trim()),
        api.setKv("tracker.ticket_pattern", trackerPattern.trim()),
      ]);
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
          Two regex-driven knobs that turn ticket references in todo cards
          into clickable links: a pattern that detects ticket ids in any
          text and a URL template that builds a browseable URL from the
          captured groups.
        </p>
        <label className="flex flex-col gap-1">
          <span className="text-sm">Ticket regex (capture groups feed the URL template)</span>
          <input
            type="text"
            value={trackerPattern}
            onChange={(e) => setTrackerPattern(e.target.value)}
            placeholder={String.raw`\b([A-Z][A-Z0-9_]*-\d+)\b`}
            className="bg-bgsub border border-bgmute rounded px-3 py-2 focus:outline-none focus:border-accent text-fg font-mono text-sm"
          />
        </label>
        <label className="flex flex-col gap-1">
          <span className="text-sm">URL template ($0 = full match, $1, $2… = capture groups)</span>
          <input
            type="text"
            value={trackerTemplate}
            onChange={(e) => setTrackerTemplate(e.target.value)}
            placeholder="https://jira.example.com/browse/$1"
            className="bg-bgsub border border-bgmute rounded px-3 py-2 focus:outline-none focus:border-accent text-fg font-mono text-sm"
          />
        </label>
        <p className="text-fgmute text-xs leading-relaxed">
          Defaults: pattern <code>{String.raw`\b([A-Z][A-Z0-9_]*-\d+)\b`}</code>
          {" "}captures common ticket ids (COREDEV-197, PROJ_2-5).
          {" "}Examples for the URL template:
          {" "}<code>https://jira.example.com/browse/$1</code> (Jira),
          {" "}<code>https://citeck.ecos24.ru/v2/dashboard?ws=COREDEV&recordRef=emodel/ept-issue@$1</code> (Citeck, single workspace).
          {" "}Leave the URL template empty to disable.
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
