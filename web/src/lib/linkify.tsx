import type { ReactNode } from "react";

// Linkify scans text for http(s) URLs and returns a React fragment list with
// clickable <a> tags for each URL, preserving newlines as <br>. Anything not a
// URL is rendered as plain text. Used in meeting descriptions where we don't
// want a full markdown parser.
export function linkify(text: string): ReactNode[] {
  if (!text) return [];
  const out: ReactNode[] = [];
  const lines = text.split("\n");
  lines.forEach((line, lineIdx) => {
    const matches = [...line.matchAll(/https?:\/\/[^\s<>"]+/g)];
    let last = 0;
    for (const m of matches) {
      const idx = m.index ?? 0;
      if (idx > last) {
        out.push(line.slice(last, idx));
      }
      const url = m[0];
      out.push(
        <a
          key={`${lineIdx}-${idx}`}
          href={url}
          target="_blank"
          rel="noreferrer"
          className="text-accent hover:underline break-all"
        >
          {url}
        </a>,
      );
      last = idx + url.length;
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
