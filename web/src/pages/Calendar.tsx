import { useEffect, useMemo, useRef, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { RefreshCw, Settings as SettingsIcon } from "lucide-react";
import { useTodoStore } from "../lib/store";
import { api } from "../lib/api";
import { MeetingCard } from "../components/MeetingCard";
import { CalDAVSettings } from "../components/CalDAVSettings";
import { linkify } from "../lib/linkify";
import type { Meeting } from "../lib/types";

function startOfDay(d: Date): Date {
  const x = new Date(d);
  x.setHours(0, 0, 0, 0);
  return x;
}

// Sync window upper bound — anything past this isn't loaded into the store, so
// "Show more" caps here.
const SYNC_WINDOW_DAYS = 30;
// Initial visible horizon for the Later section, in days from today.
const INITIAL_LATER_DAYS = 5;
const SHOW_MORE_STEP_DAYS = 7;

export function Calendar() {
  const { meetings, meetingsLoading, loadMeetings } = useTodoStore();
  const [selected, setSelected] = useState<Meeting | null>(null);
  const [noteBody, setNoteBody] = useState("");
  const [savingNote, setSavingNote] = useState(false);
  const [params] = useSearchParams();
  const focusId = params.get("focus");
  const cardRefs = useRef<Map<string, HTMLDivElement>>(new Map());

  const [settingsOpen, setSettingsOpen] = useState(false);
  const [refreshing, setRefreshing] = useState(false);
  const [configured, setConfigured] = useState<boolean | null>(null);
  const [laterDays, setLaterDays] = useState(INITIAL_LATER_DAYS);

  const reload = () => {
    const now = new Date();
    const from = Math.floor(startOfDay(now).getTime() / 1000) - 24 * 3600;
    const to = Math.floor(startOfDay(now).getTime() / 1000) + 30 * 24 * 3600;
    return loadMeetings(from, to);
  };

  useEffect(() => {
    void reload();
    Promise.all([api.getKv("caldav.url"), api.getKv("caldav.username")]).then(([u, n]) =>
      setConfigured(Boolean(u.value && n.value)),
    );
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Auto-select + scroll-into-view when arriving with ?focus=<meetingId>. Used
  // by the pre-meeting popup trigger and tray deep links.
  useEffect(() => {
    if (!focusId || meetings.length === 0) return;
    const m = meetings.find((x) => x.id === focusId);
    if (!m) return;
    setSelected(m);
    const el = cardRefs.current.get(m.id);
    if (el) el.scrollIntoView({ behavior: "smooth", block: "center" });
  }, [focusId, meetings]);

  useEffect(() => {
    if (!selected) {
      setNoteBody("");
      return;
    }
    void api.meetingNote(selected.id).then((n) => setNoteBody(n?.body ?? ""));
  }, [selected]);

  const { sections, hiddenLater } = useMemo(() => {
    const today = startOfDay(new Date()).getTime() / 1000;
    const tomorrow = today + 24 * 3600;
    const dayAfter = today + 2 * 24 * 3600;
    const laterCutoff = today + laterDays * 24 * 3600;
    const allLater = meetings.filter((m) => m.startAt >= dayAfter);
    return {
      sections: [
        { label: "Today", items: meetings.filter((m) => m.startAt >= today && m.startAt < tomorrow) },
        { label: "Tomorrow", items: meetings.filter((m) => m.startAt >= tomorrow && m.startAt < dayAfter) },
        { label: "Later", items: allLater.filter((m) => m.startAt < laterCutoff) },
      ],
      hiddenLater: allLater.filter((m) => m.startAt >= laterCutoff).length,
    };
  }, [meetings, laterDays]);

  const canShowMore = laterDays < SYNC_WINDOW_DAYS && hiddenLater > 0;

  async function saveNote() {
    if (!selected) return;
    setSavingNote(true);
    try {
      await api.upsertNote({ meetingId: selected.id, body: noteBody });
    } finally {
      setSavingNote(false);
    }
  }

  async function refresh() {
    setRefreshing(true);
    try {
      await api.triggerSync("caldav").catch(() => undefined);
      // Give the syncer a beat to upsert before re-reading.
      await new Promise((r) => setTimeout(r, 1200));
      await reload();
    } finally {
      setRefreshing(false);
    }
  }

  return (
    <div className="flex gap-6 h-full">
      <div
        className={`flex flex-col gap-4 min-w-0 ${
          selected ? "w-96 shrink-0" : "flex-1"
        }`}
      >
        <div className="flex items-center justify-between">
          <h2 className="text-xl font-semibold">Calendar</h2>
          <div className="flex items-center gap-2">
            <button
              onClick={refresh}
              disabled={refreshing}
              title="Refresh"
              className="p-2 rounded hover:bg-bgsub disabled:opacity-50"
            >
              <RefreshCw size={16} className={refreshing ? "animate-spin" : ""} />
            </button>
            <button
              onClick={() => setSettingsOpen((v) => !v)}
              title="CalDAV settings"
              className={`p-2 rounded hover:bg-bgsub ${settingsOpen ? "bg-bgsub" : ""}`}
            >
              <SettingsIcon size={16} />
            </button>
          </div>
        </div>

        {settingsOpen && (
          <section className="border border-bgmute rounded p-4 bg-bgsub/50">
            <CalDAVSettings
              onSaved={() => {
                Promise.all([api.getKv("caldav.url"), api.getKv("caldav.username")]).then(([u, n]) =>
                  setConfigured(Boolean(u.value && n.value)),
                );
                void reload();
              }}
            />
          </section>
        )}

        {meetingsLoading && <div className="text-fgmute">loading…</div>}

        {!meetingsLoading && meetings.length === 0 && configured === false && !settingsOpen && (
          <div className="border border-bgmute rounded p-6 flex flex-col items-center gap-3 text-center">
            <div className="text-fgmute text-sm">
              CalDAV is not configured yet. Connect a calendar to see your meetings here.
            </div>
            <button
              onClick={() => setSettingsOpen(true)}
              className="px-3 py-1 bg-accent text-bg rounded text-sm"
            >
              Configure CalDAV
            </button>
          </div>
        )}

        {!meetingsLoading && meetings.length === 0 && configured && (
          <div className="text-fgmute py-8 text-center">no meetings in window</div>
        )}

        {sections.map((section) =>
          section.items.length > 0 ? (
            <section key={section.label} className="flex flex-col gap-2">
              <h3 className="text-fgmute text-xs uppercase">{section.label}</h3>
              {section.items.map((m) => (
                <div
                  key={m.id}
                  ref={(el) => {
                    if (el) cardRefs.current.set(m.id, el);
                    else cardRefs.current.delete(m.id);
                  }}
                >
                  <MeetingCard
                    meeting={m}
                    selected={selected?.id === m.id}
                    onClick={setSelected}
                  />
                </div>
              ))}
              {section.label === "Later" && canShowMore && (
                <button
                  onClick={() =>
                    setLaterDays((d) =>
                      Math.min(SYNC_WINDOW_DAYS, d + SHOW_MORE_STEP_DAYS),
                    )
                  }
                  className="text-fgmute hover:text-fg text-sm py-2"
                >
                  Show more ({hiddenLater} hidden)
                </button>
              )}
            </section>
          ) : null,
        )}
      </div>

      {selected && (
        <aside className="flex-1 min-w-0 flex flex-col gap-3 border-l border-bgmute pl-4">
          <h3 className="font-semibold">{selected.title}</h3>
          {selected.description && (
            <p className="text-fgmute text-sm whitespace-pre-wrap">
              {linkify(selected.description)}
            </p>
          )}
          {selected.location && (
            <p className="text-fgmute text-sm">📍 {linkify(selected.location)}</p>
          )}
          <div className="text-fgmute text-xs">
            {new Date(selected.startAt * 1000).toLocaleString()}
          </div>
          <textarea
            value={noteBody}
            onChange={(e) => setNoteBody(e.target.value)}
            placeholder="Notes (markdown)"
            className="w-full h-64 resize-y bg-bgsub border border-bgmute rounded p-3 text-fg font-mono text-sm focus:outline-none focus:border-accent"
          />
          <div className="flex gap-2">
            <button
              onClick={saveNote}
              disabled={savingNote}
              className="px-3 py-1 bg-accent text-bg rounded text-sm"
            >
              {savingNote ? "saving…" : "save note"}
            </button>
            <button
              onClick={() => setSelected(null)}
              className="px-3 py-1 text-fgmute hover:text-fg text-sm"
            >
              close
            </button>
          </div>
        </aside>
      )}
    </div>
  );
}
