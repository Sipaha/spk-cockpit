import { Priority } from "./types";
import type { Priority as P } from "./types";

export interface QuickAddResult {
  title: string;
  priority: P;
  tags: string[];
  dueAt?: number;
}

const TAG_RE = /^#([a-z0-9][a-z0-9_-]*)$/i;
const PRIO_MAP: Record<string, P> = {
  "!low": Priority.Low,
  "!normal": Priority.Normal,
  "!high": Priority.High,
  "!urgent": Priority.Urgent,
};

export function parseQuickAdd(input: string): QuickAddResult {
  const tokens = input.split(/\s+/).filter(Boolean);
  let priority: P = Priority.Normal;
  const tags: string[] = [];
  let dueAt: number | undefined;
  const titleTokens: string[] = [];

  for (const tok of tokens) {
    if (PRIO_MAP[tok.toLowerCase()] !== undefined) {
      priority = PRIO_MAP[tok.toLowerCase()];
      continue;
    }
    const tagMatch = tok.match(TAG_RE);
    if (tagMatch) {
      tags.push(tagMatch[1]);
      continue;
    }
    if (tok.toLowerCase().startsWith("due:")) {
      const v = tok.slice(4);
      const ts = parseDueValue(v);
      if (ts !== null) {
        dueAt = ts;
        continue;
      }
    }
    titleTokens.push(tok);
  }

  return {
    title: titleTokens.join(" ").trim(),
    priority,
    tags,
    dueAt,
  };
}

function parseDueValue(v: string): number | null {
  const lower = v.toLowerCase();
  const today = atSixPM(new Date());

  if (lower === "today") {
    return Math.floor(today.getTime() / 1000);
  }
  if (lower === "tomorrow") {
    const t = new Date(today.getTime());
    t.setDate(t.getDate() + 1);
    return Math.floor(t.getTime() / 1000);
  }
  const m = v.match(/^(\d{4})-(\d{2})-(\d{2})$/);
  if (m) {
    const d = new Date(Number(m[1]), Number(m[2]) - 1, Number(m[3]), 18, 0, 0, 0);
    if (!isNaN(d.getTime())) {
      return Math.floor(d.getTime() / 1000);
    }
  }
  return null;
}

function atSixPM(base: Date): Date {
  const d = new Date(base);
  d.setHours(18, 0, 0, 0);
  return d;
}
