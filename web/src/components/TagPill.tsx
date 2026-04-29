import { useTodoStore } from "../lib/store";
import { readableText } from "../lib/colorUtils";

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
