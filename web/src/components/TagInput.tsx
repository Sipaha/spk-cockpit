import { useMemo, useRef, useState } from "react";
import type { KeyboardEvent } from "react";
import { X } from "lucide-react";
import { useTodoStore } from "../lib/store";

// Cyrillic + Latin + digits + a couple separators. The tag list is
// user-driven, so anything that's not whitespace or a structural char is
// allowed (rejecting "/", "?", etc. that confuse URL templates).
const NAME_RE = /^[\p{L}\p{N}][\p{L}\p{N}_-]*$/u;
const MAX_SUGGESTIONS = 8;

export interface TagInputProps {
  value: string[];
  onChange: (next: string[]) => void;
  suggestions: string[];
  placeholder?: string;
}

function readableText(hex: string): string {
  if (!/^#[0-9a-fA-F]{6}$/.test(hex)) return "var(--color-fgmute)";
  const r = parseInt(hex.slice(1, 3), 16);
  const g = parseInt(hex.slice(3, 5), 16);
  const b = parseInt(hex.slice(5, 7), 16);
  const yiq = (r * 299 + g * 587 + b * 114) / 1000;
  return yiq >= 140 ? "#11111b" : "#cdd6f4";
}

// Chip that is a single colored capsule with the X tucked inside, so the
// background color and the remove control read as one widget.
function Chip({ name, onRemove }: { name: string; onRemove: () => void }) {
  const c = useTodoStore((s) => s.tags.find((t) => t.name === name)?.color) ?? "";
  const colored = /^#[0-9a-fA-F]{6}$/.test(c);
  return (
    <span
      className={`inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs ${
        colored ? "" : "bg-bgmute text-fgmute"
      }`}
      style={colored ? { backgroundColor: c, color: readableText(c) } : undefined}
    >
      <span>{name}</span>
      <button
        type="button"
        onClick={onRemove}
        aria-label={`Remove ${name}`}
        className="opacity-70 hover:opacity-100"
      >
        <X size={10} />
      </button>
    </span>
  );
}

// Chips-with-input control. Each tag is a removable Chip; the input
// underneath drops a suggestions dropdown — empty input shows every
// known tag, typing filters by substring. Enter / Tab / comma submit the
// highlighted suggestion (or the typed text), Backspace on empty input
// pops the last chip, Esc clears the draft.
export function TagInput({ value, onChange, suggestions, placeholder }: TagInputProps) {
  const [draft, setDraft] = useState("");
  const [highlight, setHighlight] = useState(0);
  const [focused, setFocused] = useState(false);
  const ref = useRef<HTMLInputElement>(null);

  const lower = draft.toLowerCase().replace(/^#/, "");
  const filtered = useMemo(() => {
    const taken = new Set(value.map((v) => v.toLowerCase()));
    return suggestions
      .filter((n) => !taken.has(n.toLowerCase()))
      .filter((n) => lower === "" || n.toLowerCase().includes(lower))
      .slice(0, MAX_SUGGESTIONS);
  }, [value, suggestions, lower]);
  const safeHighlight = filtered.length === 0 ? 0 : Math.min(highlight, filtered.length - 1);

  function add(raw: string) {
    const name = raw.trim().replace(/^#/, "");
    if (!name) return;
    if (!NAME_RE.test(name)) return;
    if (value.some((v) => v.toLowerCase() === name.toLowerCase())) return;
    onChange([...value, name]);
    setDraft("");
    setHighlight(0);
  }

  function removeAt(i: number) {
    onChange(value.filter((_, idx) => idx !== i));
  }

  function onKeyDown(e: KeyboardEvent<HTMLInputElement>) {
    if (e.key === "Enter" || e.key === "Tab" || e.key === ",") {
      if (filtered.length > 0) {
        e.preventDefault();
        add(filtered[safeHighlight]);
        return;
      }
      if (draft.trim()) {
        e.preventDefault();
        add(draft);
        return;
      }
    } else if (e.key === "ArrowDown" && filtered.length > 0) {
      e.preventDefault();
      setHighlight((h) => (h + 1) % filtered.length);
    } else if (e.key === "ArrowUp" && filtered.length > 0) {
      e.preventDefault();
      setHighlight((h) => (h - 1 + filtered.length) % filtered.length);
    } else if (e.key === "Backspace" && draft === "" && value.length > 0) {
      e.preventDefault();
      removeAt(value.length - 1);
    } else if (e.key === "Escape") {
      e.preventDefault();
      setDraft("");
      setFocused(false);
    }
  }

  const isOpen = focused && filtered.length > 0;

  return (
    <div className="flex flex-col gap-1 relative">
      <div
        className="flex flex-wrap items-center gap-1 bg-bg border border-bgmute rounded p-2 min-h-10 focus-within:border-accent cursor-text"
        onClick={() => ref.current?.focus()}
      >
        {value.map((name, i) => (
          <Chip key={name} name={name} onRemove={() => removeAt(i)} />
        ))}
        <input
          ref={ref}
          value={draft}
          onChange={(e) => {
            setDraft(e.target.value);
            setHighlight(0);
          }}
          onFocus={() => setFocused(true)}
          // Delay blur so a click on a suggestion's mousedown handler runs
          // before focused flips to false (otherwise the dropdown unmounts
          // before the click fires).
          onBlur={() => setTimeout(() => setFocused(false), 120)}
          onKeyDown={onKeyDown}
          placeholder={placeholder ?? (value.length === 0 ? "add tag…" : "")}
          className="flex-1 min-w-24 bg-transparent text-fg text-sm focus:outline-none"
        />
      </div>
      {isOpen && (
        <ul className="absolute left-0 right-0 top-full mt-1 z-30 bg-bgsub border border-bgmute rounded shadow-lg max-h-56 overflow-auto">
          {filtered.map((name, i) => (
            <li
              key={name}
              role="option"
              aria-selected={i === safeHighlight}
              // Use mousedown (not click) so the action runs before the
              // input's blur handler fires and tears down the dropdown.
              onMouseDown={(e) => {
                e.preventDefault();
                add(name);
                ref.current?.focus();
              }}
              onMouseEnter={() => setHighlight(i)}
              className={`flex items-center gap-2 px-3 py-1.5 cursor-pointer text-sm ${
                i === safeHighlight ? "bg-bgmute" : ""
              }`}
            >
              <span>{name}</span>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
