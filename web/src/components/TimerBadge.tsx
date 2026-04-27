import { useEffect, useState } from "react";
import { Timer as TimerIcon } from "lucide-react";

export interface TimerBadgeProps {
  startedAt: number;
  label?: string;
}

function formatElapsed(sec: number): string {
  if (sec < 60) return `${sec}s`;
  const m = Math.floor(sec / 60);
  const s = sec % 60;
  if (m < 60) return `${m}:${String(s).padStart(2, "0")}`;
  const h = Math.floor(m / 60);
  const mm = m % 60;
  return `${h}:${String(mm).padStart(2, "0")}:${String(s).padStart(2, "0")}`;
}

export function TimerBadge({ startedAt, label }: TimerBadgeProps) {
  const [now, setNow] = useState(() => Math.floor(Date.now() / 1000));

  useEffect(() => {
    const id = window.setInterval(() => {
      setNow(Math.floor(Date.now() / 1000));
    }, 1000);
    return () => window.clearInterval(id);
  }, []);

  const elapsed = Math.max(0, now - startedAt);
  return (
    <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded bg-accent/20 text-accent text-xs font-mono">
      <TimerIcon size={12} />
      {formatElapsed(elapsed)}
      {label ? <span className="text-fgmute ml-1">{label}</span> : null}
    </span>
  );
}
