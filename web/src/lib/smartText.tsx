import type { MouseEvent, ReactNode } from "react";

// Universal task-tracker rendering driven by two user-supplied regexes:
//
//   tickerPattern — matches a ticket reference in any text (captures the
//                   parts that should be reusable in the URL/display).
//                   Default: \b([A-Z][A-Z0-9_]*-\d+)\b — captures things
//                   like COREDEV-197 or PROJ_2-5.
//   urlTemplate   — string with $1, $2, $0 backrefs that resolves to a
//                   browseable URL when applied to a tickerPattern match.
//                   Empty disables tracker linkification entirely.
//
// Detection runs in two passes:
//   1. Greedy URL spans: we find https://… runs first and run tickerPattern
//      on each of them. If a URL contains a ticket, the whole URL collapses
//      to a single [$1] anchor pointing at the resolved tracker URL.
//      If it doesn't, the URL stays as a plain external link.
//   2. The remaining (non-URL) text gets a second tickerPattern pass for
//      bare references like "fix COREDEV-197 by Friday".
//
// Anchor click handlers stop propagation so opening a ticket from inside a
// clickable card body doesn't also fire the card's edit handler.

const URL_RE = /https?:\/\/[^\s<>"\\]+/g;
export const DEFAULT_TICKET_PATTERN = String.raw`\b([A-Z][A-Z0-9_]*-\d+)\b`;

function openExternal(url: string, e: MouseEvent<HTMLAnchorElement>) {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const rt = (window as any).runtime;
  if (rt && typeof rt.BrowserOpenURL === "function") {
    e.preventDefault();
    rt.BrowserOpenURL(url);
  }
}

// applyBackrefs replaces $0, $1, $2, … in `template` with the corresponding
// regex match groups. $0 is the full match; everything past the captures
// list is left untouched. $$ keeps a literal dollar.
function applyBackrefs(template: string, match: RegExpMatchArray): string {
  return template.replace(/\$(\d+)|\$\$/g, (whole, num) => {
    if (whole === "$$") return "$";
    const i = Number(num);
    if (i === 0) return match[0];
    return match[i] ?? "";
  });
}

function compilePattern(src: string): RegExp | null {
  if (!src) return null;
  try {
    // Force a global flag so matchAll finds every occurrence; user-supplied
    // flags like /i are honored otherwise.
    return new RegExp(src, "g");
  } catch {
    return null;
  }
}

// firstMatch returns the first match of pattern in text without disturbing
// matchAll callers — re-compiles a non-global copy of the pattern.
function firstMatch(pattern: RegExp, text: string): RegExpMatchArray | null {
  const single = new RegExp(pattern.source, pattern.flags.replace("g", ""));
  return text.match(single);
}

export interface TrackerConfig {
  pattern: string; // regex source
  urlTemplate: string;
}

export function renderSmart(text: string, cfg: TrackerConfig): ReactNode[] {
  if (!text) return [];
  const trackerOn = !!cfg.urlTemplate;
  const ticket = trackerOn
    ? compilePattern(cfg.pattern || DEFAULT_TICKET_PATTERN)
    : null;

  type Range = { start: number; end: number; node: ReactNode };
  const ranges: Range[] = [];

  // Pass 1: URLs.
  for (const m of text.matchAll(URL_RE)) {
    const start = m.index ?? 0;
    const url = m[0];
    let collapsed: { url: string; label: string } | null = null;
    if (ticket) {
      const inUrl = firstMatch(ticket, url);
      if (inUrl) {
        const built = applyBackrefs(cfg.urlTemplate, inUrl);
        collapsed = { url: built, label: `[${inUrl[1] ?? inUrl[0]}]` };
      }
    }
    ranges.push({
      start,
      end: start + url.length,
      node: collapsed
        ? anchor(start, collapsed.url, collapsed.label)
        : anchor(start, url, url),
    });
  }

  // Pass 2: bare ticket references in non-URL spans.
  if (ticket) {
    for (const m of text.matchAll(ticket)) {
      const start = m.index ?? 0;
      const end = start + m[0].length;
      if (ranges.some((r) => start < r.end && end > r.start)) continue;
      const url = applyBackrefs(cfg.urlTemplate, m);
      ranges.push({
        start,
        end,
        node: anchor(start, url, `[${m[1] ?? m[0]}]`),
      });
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
