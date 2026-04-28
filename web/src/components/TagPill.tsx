import { useTodoStore } from "../lib/store";

// Pick a readable text color (black or white) for an arbitrary background hex.
// Uses the YIQ luma trick — good enough for rough contrast without pulling in
// a color library.
function readableText(hex: string): string {
  if (!/^#[0-9a-fA-F]{6}$/.test(hex)) return "var(--color-fgmute)";
  const r = parseInt(hex.slice(1, 3), 16);
  const g = parseInt(hex.slice(3, 5), 16);
  const b = parseInt(hex.slice(5, 7), 16);
  const yiq = (r * 299 + g * 587 + b * 114) / 1000;
  return yiq >= 140 ? "#11111b" : "#cdd6f4";
}

// TagPill renders a tag as a colored chip. The colored capsule itself is the
// "this is a tag" cue, so we drop the '#' prefix that used to live here —
// it added noise without information.
export function TagPill({ name, color }: { name: string; color?: string }) {
  // Resolve color from the global tags cache when not passed explicitly so
  // every render of a tag picks up Settings-page edits without prop drilling.
  const fromStore = useTodoStore((s) => s.tags.find((t) => t.name === name)?.color);
  const c = color ?? fromStore ?? "";
  if (c && /^#[0-9a-fA-F]{6}$/.test(c)) {
    return (
      <span
        className="inline-block px-2 py-0.5 rounded-full text-xs font-medium"
        style={{ backgroundColor: c, color: readableText(c) }}
      >
        {name}
      </span>
    );
  }
  return (
    <span className="inline-block px-2 py-0.5 rounded-full bg-bgmute text-fgmute text-xs font-medium">
      {name}
    </span>
  );
}
