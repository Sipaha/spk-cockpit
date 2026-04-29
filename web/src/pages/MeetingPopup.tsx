import { useEffect, useState } from "react";
import { useSearchParams } from "react-router-dom";
import type { Meeting } from "../lib/types";
import { linkify } from "../lib/linkify";
import { closeWindow } from "../lib/wails";

function formatTime(unix: number): string {
  return new Date(unix * 1000).toLocaleTimeString(undefined, { hour: "2-digit", minute: "2-digit" });
}

function relTime(unix: number): string {
  const ms = unix * 1000 - Date.now();
  if (ms < 0) return "started";
  const min = Math.round(ms / 60000);
  if (min < 60) return `in ${min}m`;
  const hr = Math.round(min / 60);
  return `in ${hr}h`;
}

// MeetingPopup is the bare standalone view rendered in a native v3 child
// window. It carries no sidebar/nav — only the meeting essentials and a
// Dismiss button. Esc and the button both call closeWindow() (Wails-aware
// close that works in the v3 webview where raw window.close() is a no-op).
export function MeetingPopup() {
  const [params] = useSearchParams();
  const id = params.get("id");
  const [meeting, setMeeting] = useState<Meeting | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!id) {
      setError("missing meeting id");
      return;
    }
    let cancelled = false;
    fetch(`/api/meetings/${encodeURIComponent(id)}`)
      .then(async (r) => {
        if (!r.ok) throw new Error(`HTTP ${r.status}`);
        return (await r.json()) as Meeting;
      })
      .then((m) => {
        if (!cancelled) setMeeting(m);
      })
      .catch((e) => {
        if (!cancelled) setError(String(e));
      });
    return () => {
      cancelled = true;
    };
  }, [id]);

  // Esc closes the popup. The popup has no focused input by default, so
  // listen at the document level rather than on a specific element.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") closeWindow();
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, []);

  const dismiss = () => closeWindow();

  return (
    <div className="min-h-screen bg-bg text-fg p-5 flex flex-col">
      {error && <div className="text-red-400 text-sm">Failed to load: {error}</div>}
      {!error && !meeting && <div className="text-fgmute">Loading…</div>}
      {meeting && (
        <>
          <div className="flex-1 flex flex-col gap-3">
            <h1 className="text-xl font-semibold">{meeting.title}</h1>
            <div className="text-fgmute text-sm">
              {formatTime(meeting.startAt)} – {formatTime(meeting.endAt)}{" "}
              <span className="ml-1 text-accent">{relTime(meeting.startAt)}</span>
            </div>
            {meeting.location && (
              <div className="text-fgmute text-sm">📍 {linkify(meeting.location)}</div>
            )}
            {meeting.description && (
              <div className="text-sm whitespace-pre-wrap mt-2 leading-relaxed">
                {linkify(meeting.description)}
              </div>
            )}
          </div>
          <div className="flex justify-end gap-2 pt-3 border-t border-bgmute">
            <button
              onClick={dismiss}
              className="px-3 py-1 rounded bg-accent text-bg text-sm font-medium"
            >
              Dismiss
            </button>
          </div>
        </>
      )}
    </div>
  );
}
