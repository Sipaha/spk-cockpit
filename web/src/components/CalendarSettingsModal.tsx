import { useEffect, useState } from "react";
import { api } from "../lib/api";
import { CalDAVSettings } from "./CalDAVSettings";

export interface CalendarSettingsModalProps {
  onClose: () => void;
  onSaved: () => void;
}

// One-stop modal for everything Calendar-related: CalDAV connection
// settings (host/login/password) plus the meeting-notification minute
// defaults that previously lived on the global Settings page.
export function CalendarSettingsModal({ onClose, onSaved }: CalendarSettingsModalProps) {
  const [defaultNotifyMin, setDefaultNotifyMin] = useState("5");
  const [defaultPopupMin, setDefaultPopupMin] = useState("1");
  const [savingNotify, setSavingNotify] = useState(false);
  const [notifySaved, setNotifySaved] = useState<string | null>(null);

  useEffect(() => {
    void api.getKv("meeting.default_notify_min").then((r) => r.value && setDefaultNotifyMin(r.value));
    void api.getKv("meeting.default_popup_min").then((r) => r.value && setDefaultPopupMin(r.value));

    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") onClose();
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  async function saveNotify() {
    setSavingNotify(true);
    try {
      await Promise.all([
        api.setKv("meeting.default_notify_min", defaultNotifyMin),
        api.setKv("meeting.default_popup_min", defaultPopupMin),
      ]);
      setNotifySaved(new Date().toLocaleTimeString());
    } finally {
      setSavingNotify(false);
    }
  }

  return (
    <div className="fixed inset-0 z-40 bg-black/60 flex items-center justify-center p-6">
      <div className="bg-bgsub border border-bgmute rounded shadow-2xl w-full max-w-xl flex flex-col gap-4 p-4 max-h-[85vh] overflow-auto">
        <div className="flex items-center justify-between">
          <div className="text-fgmute text-xs uppercase tracking-wide">
            Calendar settings
          </div>
          <button
            onClick={onClose}
            className="text-fgmute hover:text-fg text-sm"
          >
            Close
          </button>
        </div>

        <section className="flex flex-col gap-2">
          <h3 className="text-fg text-sm font-semibold">CalDAV</h3>
          <CalDAVSettings onSaved={onSaved} />
        </section>

        <section className="flex flex-col gap-2 pt-2 border-t border-bgmute">
          <h3 className="text-fg text-sm font-semibold">Notifications</h3>
          <p className="text-fgmute text-xs">
            Defaults applied to every meeting; per-meeting overrides still
            win. Setting either to 0 disables that channel for the default.
          </p>
          <label className="flex items-center justify-between gap-3 max-w-sm">
            <span className="text-sm">DBus notification, minutes before</span>
            <input
              type="number"
              min={0}
              value={defaultNotifyMin}
              onChange={(e) => setDefaultNotifyMin(e.target.value)}
              className="w-24 bg-bg border border-bgmute rounded px-2 py-1 focus:outline-none focus:border-accent text-fg text-sm"
            />
          </label>
          <label className="flex items-center justify-between gap-3 max-w-sm">
            <span className="text-sm">Popup window, minutes before</span>
            <input
              type="number"
              min={0}
              value={defaultPopupMin}
              onChange={(e) => setDefaultPopupMin(e.target.value)}
              className="w-24 bg-bg border border-bgmute rounded px-2 py-1 focus:outline-none focus:border-accent text-fg text-sm"
            />
          </label>
          <div className="flex items-center gap-3">
            <button
              onClick={saveNotify}
              disabled={savingNotify}
              className="px-3 py-1 bg-accent text-bg rounded text-sm"
            >
              {savingNotify ? "saving…" : "Save notifications"}
            </button>
            {notifySaved && (
              <span className="text-fgmute text-xs">saved at {notifySaved}</span>
            )}
          </div>
        </section>
      </div>
    </div>
  );
}
