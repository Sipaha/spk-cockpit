import type { ReactNode } from "react";
import { openExternal } from "./wails";

// Universal task-tracker rendering. The user configures one or more
// (pattern, urlTemplate) pairs in Settings. Each pattern is a regex that
// matches a task reference in any text and exposes capture groups for the
// URL template:
//
//   - pattern      — regex source. Capture groups feed the URL.
//                    Default: \b([A-Z][A-Z0-9_]*-\d+)\b — covers
//                    PROJ-1, ABC_2-50, etc.
//   - urlTemplate  — string with $0, $1, $2… backrefs that resolves to a
//                    browseable URL. $$ keeps a literal dollar.
//
// Detection runs in two passes:
//   1. Greedy URL spans. We find https://… runs first and try every
//      configured pattern against each one. If a URL contains a match
//      under any pattern, the whole URL collapses into a single [$1]
//      anchor pointing at the resolved tracker URL. Otherwise the URL
//      stays as a plain external link.
//   2. The remaining (non-URL) text gets a second pass for bare
//      references like "fix PROJ-123 by Friday".
//
// Anchor click handlers stop propagation so opening a task from inside a
// clickable card body doesn't also fire the card's edit handler.

const URL_RE = /https?:\/\/[^\s<>"\\]+/g;
export const DEFAULT_TASK_PATTERN = String.raw`\b([A-Z][A-Z0-9_]*-\d+)\b`;

export interface TaskPattern {
  pattern: string;
  urlTemplate: string;
  name?: string;
}

// safeUrl returns the input unchanged if it parses as http: or https:, and "#"
// otherwise. We render anchors with user-supplied URL templates ("$1" backrefs
// resolved against a captured pattern), so a misconfigured template like
// "javascript:alert($1)" or a tracker server returning a `javascript:` URL must
// not become a click-to-execute sink in the Wails WebView.
export function safeUrl(raw: string): string {
  try {
    const u = new URL(raw, "http://placeholder.invalid/");
    if (u.protocol === "http:" || u.protocol === "https:") return raw;
  } catch {
    // fall through
  }
  return "#";
}

function applyBackrefs(template: string, match: RegExpMatchArray): string {
  return template.replace(/\$(\d+)|\$\$/g, (whole, num) => {
    if (whole === "$$") return "$";
    const i = Number(num);
    if (i === 0) return match[0];
    return match[i] ?? "";
  });
}

interface CompiledPattern {
  global: RegExp;       // for matchAll
  single: RegExp;       // for first-match probe
  urlTemplate: string;
}

// Cap pattern length and reject the most common catastrophic-backtracking
// shapes before compiling. JS lacks a regex engine timeout, so a malicious or
// accidental "(a+)+" pattern applied to every todo title on every render would
// hang the UI thread. This isn't an exhaustive ReDoS detector (consider a
// proper safe-regex library for that) — it catches:
//   - quantified group containing a quantifier:  (a+)+, (.*)+, (ab*){2,}, etc.
//   - adjacent quantifiers in the flat pattern:  a++, b**, etc.
const MAX_PATTERN_LEN = 256;
const NESTED_QUANTIFIER = /\(([^()]*?)([+*?]|\{\d+,?\d*\})([^()]*?)\)\s*([+*?]|\{\d+,?\d*\})/;
const ADJACENT_QUANTIFIER = /([+*?]|\{\d+,?\d*\})\s*([+*?]|\{\d+,?\d*\})/;

function looksRedos(src: string): boolean {
  return NESTED_QUANTIFIER.test(src) || ADJACENT_QUANTIFIER.test(src);
}

function compile(p: TaskPattern): CompiledPattern | null {
  const src = p.pattern || DEFAULT_TASK_PATTERN;
  if (!p.urlTemplate) return null;
  if (src.length > MAX_PATTERN_LEN) return null;
  if (looksRedos(src)) return null;
  try {
    const single = new RegExp(src);
    const flags = single.flags.includes("g") ? single.flags : single.flags + "g";
    return {
      global: new RegExp(src, flags),
      single,
      urlTemplate: p.urlTemplate,
    };
  } catch {
    return null;
  }
}

export function renderSmart(text: string, patterns: TaskPattern[]): ReactNode[] {
  if (!text) return [];
  const compiled: CompiledPattern[] = [];
  for (const p of patterns ?? []) {
    const c = compile(p);
    if (c) compiled.push(c);
  }

  type Range = { start: number; end: number; node: ReactNode };
  const ranges: Range[] = [];

  // Pass 1: URL spans. The first compiled pattern that matches inside
  // a URL wins; URLs that match no pattern stay as plain external links.
  for (const m of text.matchAll(URL_RE)) {
    const start = m.index ?? 0;
    const url = m[0];
    let collapsed: { url: string; label: string } | null = null;
    for (const c of compiled) {
      const inUrl = url.match(c.single);
      if (inUrl) {
        collapsed = {
          url: safeUrl(applyBackrefs(c.urlTemplate, inUrl)),
          label: `[${inUrl[1] ?? inUrl[0]}]`,
        };
        break;
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

  // Pass 2: bare references in non-URL spans. We collect every pattern's
  // matches and skip those that overlap an already-claimed range, so two
  // patterns competing for the same text don't double-render.
  for (const c of compiled) {
    for (const m of text.matchAll(c.global)) {
      const start = m.index ?? 0;
      const end = start + m[0].length;
      if (ranges.some((r) => start < r.end && end > r.start)) continue;
      const url = safeUrl(applyBackrefs(c.urlTemplate, m));
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
