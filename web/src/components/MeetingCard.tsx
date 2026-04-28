import { Calendar, MapPin, Bell } from "lucide-react";
import type { Meeting } from "../lib/types";

export interface MeetingCardProps {
  meeting: Meeting;
  onClick?: (m: Meeting) => void;
  selected?: boolean;
}

function formatTime(unix: number): string {
  const d = new Date(unix * 1000);
  return d.toLocaleTimeString(undefined, { hour: "2-digit", minute: "2-digit" });
}

function relTime(startAt: number, endAt: number): string {
  const now = Date.now();
  // Meeting that's already finished — flag it once and stop nagging the
  // user with a "started" tag that lingers forever.
  if (endAt * 1000 <= now) return "ended";
  // Currently happening: between start and end.
  if (startAt * 1000 <= now) return "now";
  const ms = startAt * 1000 - now;
  const min = Math.round(ms / 60000);
  if (min < 60) return `in ${min}m`;
  const hr = Math.round(min / 60);
  if (hr < 24) return `in ${hr}h`;
  const day = Math.round(hr / 24);
  return `in ${day}d`;
}

export function MeetingCard({ meeting, onClick, selected }: MeetingCardProps) {
  const cls = `flex flex-col gap-1 p-3 rounded border cursor-pointer ${
    selected ? "bg-bgsub border-accent" : "bg-bg border-bgmute hover:border-fgmute"
  } ${meeting.cancelled ? "opacity-50 line-through" : ""}`;
  return (
    <div className={cls} onClick={() => onClick?.(meeting)}>
      <div className="flex items-center justify-between">
        <span className="font-medium">{meeting.title}</span>
        <span className="text-fgmute text-xs">
          {relTime(meeting.startAt, meeting.endAt)}
        </span>
      </div>
      <div className="flex items-center gap-3 text-fgmute text-xs">
        <span className="inline-flex items-center gap-1">
          <Calendar size={12} />
          {formatTime(meeting.startAt)} – {formatTime(meeting.endAt)}
        </span>
        {meeting.location && (
          <span className="inline-flex items-center gap-1">
            <MapPin size={12} />
            {meeting.location}
          </span>
        )}
        {meeting.notifyMin !== undefined && (
          <span className="inline-flex items-center gap-1">
            <Bell size={12} />
            {meeting.notifyMin}m
          </span>
        )}
        {meeting.source === "caldav" && (
          <span className="text-low text-[10px] uppercase">caldav</span>
        )}
      </div>
    </div>
  );
}
