import type { MouseEvent, ReactNode } from "react";

// Universal "task tracker" rendering. The user configures a single URL
// template like https://jira.example.com/browse/{id} (Jira-style) or
// https://citeck.ecos24.ru/v2/dashboard?ws={project}&recordRef=emodel/ept-issue@{id}
// (Citeck-style) in Settings. {id} expands to the full ticket id
// (e.g. COREDEV-197) and {project} to the part before the dash.
//
// Given that template, both:
//   1) full URLs that share the template's host AND contain a ticket-shaped
//      substring,
//   2) bare ticket tokens like COREDEV-197 in plain text,
// render as a single clickable [TICKET-ID] anchor pointing at the resolved
// tracker URL. Other URLs become normal clickable anchors. Plain text falls
// through unchanged.

const URL_RE = /https?:\/\/[^\s<>"\\]+/g;
const TICKET_RE = /\b([A-Z][A-Z0-9_]*-\d+)\b/g;

function openExternal(url: string, e: MouseEvent<HTMLAnchorElement>) {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const rt = (window as any).runtime;
  if (rt && typeof rt.BrowserOpenURL === "function") {
    e.preventDefault();
    rt.BrowserOpenURL(url);
  }
}

export function resolveTracker(template: string, ticket: string): string | null {
  if (!template) return null;
  const dash = ticket.indexOf("-");
  if (dash <= 0) return null;
  const project = ticket.slice(0, dash);
  return template.replaceAll("{id}", ticket).replaceAll("{project}", project);
}

function templateHost(template: string): string | null {
  try {
    return new URL(
      template.replaceAll("{id}", "X-1").replaceAll("{project}", "X"),
    ).host.toLowerCase();
  } catch {
    return null;
  }
}

function urlMatchesTemplate(url: string, host: string | null): string | null {
  if (!host) return null;
  let parsed: URL;
  try {
    parsed = new URL(url);
  } catch {
    return null;
  }
  if (parsed.host.toLowerCase() !== host) return null;
  // Greedy ticket lookup inside the URL — works whether the id sits in the
  // path (Jira) or inside a query value (Citeck recordRef=…@TICKET).
  const m = url.match(/[A-Z][A-Z0-9_]*-\d+/);
  return m ? m[0] : null;
}

export function renderSmart(text: string, template: string): ReactNode[] {
  if (!text) return [];

  const host = template ? templateHost(template) : null;

  type Range = { start: number; end: number; node: ReactNode };
  const ranges: Range[] = [];

  // Full URLs first — they own their span so a contained ticket pattern
  // doesn't get linkified twice.
  for (const m of text.matchAll(URL_RE)) {
    const start = m.index ?? 0;
    const url = m[0];
    const ticketInUrl = urlMatchesTemplate(url, host);
    if (ticketInUrl && template) {
      const resolved = resolveTracker(template, ticketInUrl) ?? url;
      ranges.push({
        start,
        end: start + url.length,
        node: anchor(start, resolved, `[${ticketInUrl}]`),
      });
    } else {
      ranges.push({
        start,
        end: start + url.length,
        node: anchor(start, url, url),
      });
    }
  }

  // Bare ticket ids — only when the user has configured a template.
  if (template) {
    for (const m of text.matchAll(TICKET_RE)) {
      const start = m.index ?? 0;
      const end = start + m[0].length;
      if (ranges.some((r) => start < r.end && end > r.start)) continue;
      const url = resolveTracker(template, m[1]);
      if (!url) continue;
      ranges.push({ start, end, node: anchor(start, url, `[${m[1]}]`) });
    }
  }

  ranges.sort((a, b) => a.start - b.start);

  const out: ReactNode[] = [];
  let cursor = 0;
  for (const r of ranges) {
    if (r.start > cursor) out.push(text.slice(cursor, r.start));
    out.push(r.node);
    cursor = r.end;
  }
  if (cursor < text.length) out.push(text.slice(cursor));
  return out;
}

function anchor(key: number, url: string, label: string): ReactNode {
  return (
    <a
      key={`a-${key}`}
      href={url}
      target="_blank"
      rel="noreferrer"
      onClick={(e) => {
        e.stopPropagation();
        openExternal(url, e);
      }}
      className="text-accent hover:underline break-all"
    >
      {label}
    </a>
  );
}
