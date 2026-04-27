import { useEffect, useMemo, useRef, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { useTodoStore } from "../lib/store";
import { api } from "../lib/api";
import { MeetingCard } from "../components/MeetingCard";
import { linkify } from "../lib/linkify";
import type { Meeting } from "../lib/types";

function startOfDay(d: Date): Date {
  const x = new Date(d);
  x.setHours(0, 0, 0, 0);
  return x;
}

export function Calendar() {
  const { meetings, meetingsLoading, loadMeetings } = useTodoStore();
  const [selected, setSelected] = useState<Meeting | null>(null);
  const [noteBody, setNoteBody] = useState("");
  const [savingNote, setSavingNote] = useState(false);
  const [params] = useSearchParams();
  const focusId = params.get("focus");
  const cardRefs = useRef<Map<string, HTMLDivElement>>(new Map());

  useEffect(() => {
    const now = new Date();
    const from = Math.floor(startOfDay(now).getTime() / 1000) - 24 * 3600;
    const to = Math.floor(startOfDay(now).getTime() / 1000) + 30 * 24 * 3600;
    void loadMeetings(from, to);
  }, [loadMeetings]);

  // Auto-select + scroll-into-view when arriving with ?focus=<meetingId>.
  // Used by the pre-meeting popup trigger and tray "Open standup"-style deep links.
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

  const sections = useMemo(() => {
    const today = startOfDay(new Date()).getTime() / 1000;
    const tomorrow = today + 24 * 3600;
    const dayAfter = today + 2 * 24 * 3600;
    return [
      { label: "Today", items: meetings.filter((m) => m.startAt >= today && m.startAt < tomorrow) },
      { label: "Tomorrow", items: meetings.filter((m) => m.startAt >= tomorrow && m.startAt < dayAfter) },
      { label: "Later", items: meetings.filter((m) => m.startAt >= dayAfter) },
    ];
  }, [meetings]);

  async function saveNote() {
    if (!selected) return;
    setSavingNote(true);
    try {
      await api.upsertNote({ meetingId: selected.id, body: noteBody });
    } finally {
      setSavingNote(false);
    }
  }

  return (
    <div className="flex gap-6 h-full">
      <div className="flex-1 flex flex-col gap-4 max-w-2xl">
        <h2 className="text-xl font-semibold">Calendar</h2>
        {meetingsLoading && <div className="text-fgmute">loading…</div>}
        {!meetingsLoading && meetings.length === 0 && (
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
            </section>
          ) : null,
        )}
      </div>

      {selected && (
        <aside className="w-96 flex flex-col gap-3 border-l border-bgmute pl-4">
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
            className="flex-1 min-h-64 bg-bgsub border border-bgmute rounded p-3 text-fg font-mono text-sm focus:outline-none focus:border-accent"
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
