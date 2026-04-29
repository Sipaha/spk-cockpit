import { useEffect, useState } from "react";
import type { StandupReport, StandupItem } from "../lib/types";
import { api } from "../lib/api";
import { safeUrl } from "../lib/smartText";

export function Standup() {
  const [report, setReport] = useState<StandupReport | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    let cancelled = false;
    api.standup()
      .then((r) => {
        if (!cancelled) setReport(r);
      })
      .catch((e) => {
        if (!cancelled) setError(String(e));
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const copy = async () => {
    if (!report) return;
    await navigator.clipboard.writeText(toMarkdown(report));
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  };

  if (error) return <div className="text-red-400">Failed to load standup: {error}</div>;
  if (!report) return <div className="text-fgmute">Loading…</div>;

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h2 className="text-xl font-semibold">Standup — {report.day}</h2>
        <button
          onClick={copy}
          className="px-3 py-1 rounded bg-bgsub border border-bgmute hover:bg-bg text-sm"
        >
          {copied ? "Copied!" : "Copy as markdown"}
        </button>
      </div>
      {report.errors && report.errors.length > 0 && (
        <div className="text-yellow-400 text-sm">
          {report.errors.map((e, i) => (
            <div key={i}>⚠ {e}</div>
          ))}
        </div>
      )}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <Section title="Yesterday" items={report.yesterday} />
        <Section title="Today" items={report.today} />
        <Section title="Blockers" items={report.blockers} />
      </div>
    </div>
  );
}

function Section({ title, items }: { title: string; items: StandupItem[] }) {
  return (
    <div className="bg-bgsub border border-bgmute rounded p-3">
      <h3 className="text-sm uppercase tracking-wide text-fgmute mb-2">{title}</h3>
      {items.length === 0 ? (
        <div className="text-fgmute text-sm">— nothing —</div>
      ) : (
        <ul className="space-y-2">
          {items.map((it, i) => (
            <li key={`${it.source}-${it.refId ?? it.title}-${i}`} className="text-sm">
              <span className="text-fgmute mr-2">[{tagFor(it.source)}]</span>
              {it.url ? (
                <a className="hover:underline" href={safeUrl(it.url)} target="_blank" rel="noreferrer">
                  {it.title}
                </a>
              ) : (
                it.title
              )}
              {it.detail && <span className="text-fgmute"> — {it.detail}</span>}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

function tagFor(s: StandupItem["source"]): string {
  switch (s) {
    case "todo":
      return "todo";
    case "gitlab":
      return "git";
    case "tracker":
      return "pt";
  }
}

function toMarkdown(r: StandupReport): string {
  let s = `# Standup — ${r.day}\n\n`;
  s += sectionMd("Yesterday", r.yesterday);
  s += sectionMd("Today", r.today);
  s += sectionMd("Blockers", r.blockers);
  if (r.errors && r.errors.length > 0) {
    s += "\n_Source errors:_\n";
    for (const e of r.errors) s += `- ${e}\n`;
  }
  return s;
}

function sectionMd(name: string, items: StandupItem[]): string {
  let s = `## ${name}\n`;
  if (items.length === 0) {
    s += "_(none)_\n\n";
    return s;
  }
  for (const it of items) {
    let line = `- [${tagFor(it.source)}] ${it.title}`;
    if (it.detail) line += ` — ${it.detail}`;
    if (it.url) line += ` (${it.url})`;
    s += line + "\n";
  }
  return s + "\n";
}
