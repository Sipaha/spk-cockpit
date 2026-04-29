import type { ReactNode } from "react";
import { openExternal } from "./wails";

// Trailing chars that get glued to URLs by aggressive matching but virtually
// never belong (sentence-ending punctuation, closing brackets/quotes).
const TRAILING_TRIM = /[.,!?;:)\]}>"'»\\]+$/;

// Linkify scans text for http(s) URLs and returns a React fragment list with
// clickable <a> tags for each URL, preserving newlines as <br>. Anything not a
// URL is rendered as plain text. Used in meeting descriptions where we don't
// want a full markdown parser.
export function linkify(text: string): ReactNode[] {
  if (!text) return [];
  const out: ReactNode[] = [];
  const lines = text.split("\n");
  lines.forEach((line, lineIdx) => {
    const matches = [...line.matchAll(/https?:\/\/[^\s<>"\\]+/g)];
    let last = 0;
    for (const m of matches) {
      const idx = m.index ?? 0;
      if (idx > last) {
        out.push(line.slice(last, idx));
      }
      let url = m[0];
      const trail = url.match(TRAILING_TRIM);
      const trailing = trail ? trail[0] : "";
      if (trailing) url = url.slice(0, url.length - trailing.length);
      out.push(
        <a
          key={`${lineIdx}-${idx}`}
          href={url}
          target="_blank"
          rel="noreferrer"
          onClick={(e) => openExternal(url, e)}
          className="text-accent hover:underline break-all"
        >
          {url}
        </a>,
      );
      if (trailing) out.push(trailing);
      last = idx + m[0].length;
    }
    if (last < line.length) {
      out.push(line.slice(last));
    }
    if (lineIdx < lines.length - 1) {
      out.push(<br key={`br-${lineIdx}`} />);
    }
  });
  return out;
}
