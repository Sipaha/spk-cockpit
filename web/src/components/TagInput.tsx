import { useMemo, useRef, useState } from "react";
import type { KeyboardEvent } from "react";
import { X } from "lucide-react";
import { TagPill } from "./TagPill";

const NAME_RE = /^[a-z0-9][a-z0-9_-]*$/i;
const MAX_SUGGESTIONS = 6;

export interface TagInputProps {
  value: string[];
  onChange: (next: string[]) => void;
  suggestions: string[];
  placeholder?: string;
}

// Compact chips-with-input control. Each tag is a removable TagPill; typing
// in the trailing input filters suggestions from `suggestions` and submits a
// tag on Enter / Tab / comma. Backspace on an empty input pops the last
// chip — standard chips-input UX.
export function TagInput({ value, onChange, suggestions, placeholder }: TagInputProps) {
  const [draft, setDraft] = useState("");
  const [highlight, setHighlight] = useState(0);
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
      if (filtered.length > 0 && draft) {
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
    }
  }

  return (
    <div className="flex flex-col gap-1 relative">
      <div className="flex flex-wrap items-center gap-1 bg-bg border border-bgmute rounded p-2 min-h-10 focus-within:border-accent">
        {value.map((name, i) => (
          <span key={name} className="inline-flex items-center gap-1">
            <TagPill name={name} />
            <button
              type="button"
              onClick={() => removeAt(i)}
              className="text-fgmute hover:text-urgent"
              aria-label={`Remove ${name}`}
            >
              <X size={12} />
            </button>
          </span>
        ))}
        <input
          ref={ref}
          value={draft}
          onChange={(e) => {
            setDraft(e.target.value);
            setHighlight(0);
          }}
          onKeyDown={onKeyDown}
          placeholder={placeholder ?? (value.length === 0 ? "add tag…" : "")}
          className="flex-1 min-w-24 bg-transparent text-fg text-sm focus:outline-none"
        />
      </div>
      {draft && filtered.length > 0 && (
        <ul className="absolute left-0 right-0 top-full mt-1 z-30 bg-bgsub border border-bgmute rounded shadow-lg max-h-48 overflow-auto">
          {filtered.map((name, i) => (
            <li
              key={name}
              role="option"
              aria-selected={i === safeHighlight}
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
