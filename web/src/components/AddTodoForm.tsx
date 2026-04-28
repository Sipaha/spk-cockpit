import { useMemo, useRef, useState } from "react";
import type { ChangeEvent, KeyboardEvent } from "react";
import { Priority } from "../lib/types";
import { api } from "../lib/api";
import { parseQuickAdd } from "../lib/parser";
import { useTodoStore } from "../lib/store";

const MAX_SUGGESTIONS = 8;

// Find the whitespace-bounded token that contains `pos`. The autocomplete
// uses this to detect when the cursor is sitting inside a #tag fragment.
function tokenAtCursor(
  s: string,
  pos: number,
): { start: number; end: number; text: string } {
  let start = pos;
  while (start > 0 && !/\s/.test(s[start - 1])) start--;
  let end = pos;
  while (end < s.length && !/\s/.test(s[end])) end++;
  return { start, end, text: s.slice(start, end) };
}

export function AddTodoForm() {
  const [input, setInput] = useState("");
  const [busy, setBusy] = useState(false);
  const [cursor, setCursor] = useState(0);
  const [highlight, setHighlight] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);

  const knownTags = useTodoStore((s) => s.tags);

  const preview = input ? parseQuickAdd(input) : null;
  const isValid = preview !== null && preview.title.length > 0;

  const tok = tokenAtCursor(input, cursor);
  const partial = tok.text.startsWith("#") ? tok.text.slice(1).toLowerCase() : null;
  const alreadyChosen = new Set((preview?.tags ?? []).map((t) => t.toLowerCase()));
  const matches = useMemo(() => {
    if (partial === null) return [];
    return knownTags
      .map((t) => t.name)
      .filter((n) => n.length > 0)
      .filter(
        (n) =>
          n.toLowerCase() !== partial && // skip exact-match: nothing to complete
          (partial === "" || n.toLowerCase().includes(partial)) &&
          !alreadyChosen.has(n.toLowerCase()),
      )
      .slice(0, MAX_SUGGESTIONS);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [partial, knownTags, input]);
  const isOpen = matches.length > 0 && partial !== null;
  const safeHighlight = matches.length === 0 ? 0 : Math.min(highlight, matches.length - 1);

  function syncCursor(el: HTMLInputElement) {
    setCursor(el.selectionStart ?? el.value.length);
  }

  function onChange(e: ChangeEvent<HTMLInputElement>) {
    setInput(e.target.value);
    syncCursor(e.target);
    setHighlight(0);
  }

  function complete(name: string) {
    const before = input.slice(0, tok.start);
    const after = input.slice(tok.end);
    const replacement = "#" + name;
    // Drop a trailing space when there isn't already whitespace after, so the
    // cursor lands ready for the next token.
    const sep = after.startsWith(" ") || after === "" ? "" : " ";
    const trail = after === "" ? " " : "";
    const next = before + replacement + sep + trail + after;
    const newCursor = tok.start + replacement.length + (sep ? 1 : trail ? 1 : 0);
    setInput(next);
    setHighlight(0);
    requestAnimationFrame(() => {
      const el = inputRef.current;
      if (!el) return;
      el.focus();
      el.setSelectionRange(newCursor, newCursor);
      setCursor(newCursor);
    });
  }

  function onKeyDown(e: KeyboardEvent<HTMLInputElement>) {
    if (!isOpen) return;
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setHighlight((h) => (h + 1) % matches.length);
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setHighlight((h) => (h - 1 + matches.length) % matches.length);
    } else if (e.key === "Enter" || e.key === "Tab") {
      e.preventDefault();
      complete(matches[safeHighlight]);
    } else if (e.key === "Escape") {
      e.preventDefault();
      // Close by moving the cursor outside the #tag token: append a space.
      setInput((v) => v + " ");
      setHighlight(0);
    }
  }

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!isValid) return;
    setBusy(true);
    try {
      await api.createTodo({
        title: preview!.title,
        priority: preview!.priority ?? Priority.Normal,
        tags: preview!.tags.length > 0 ? preview!.tags : undefined,
        dueAt: preview!.dueAt,
      });
      setInput("");
    } finally {
      setBusy(false);
    }
  }

  return (
    <form onSubmit={submit} className="flex flex-col gap-1 relative">
      <input
        ref={inputRef}
        type="text"
        value={input}
        onChange={onChange}
        onKeyUp={(e) => syncCursor(e.currentTarget)}
        onClick={(e) => syncCursor(e.currentTarget)}
        onKeyDown={onKeyDown}
        placeholder='+ Add todo (e.g. "Fix bug !urgent #backend due:tomorrow")'
        className="flex-1 bg-bgsub border border-bgmute rounded px-3 py-2 focus:outline-none focus:border-accent text-fg"
        disabled={busy}
        autoComplete="off"
      />
      {isOpen && (
        <ul
          className="absolute left-0 right-0 top-full mt-1 z-10 bg-bgsub border border-bgmute rounded shadow-lg max-h-56 overflow-auto"
          role="listbox"
        >
          {matches.map((name, i) => {
            const tag = knownTags.find((t) => t.name === name);
            const valid = tag && /^#[0-9a-fA-F]{6}$/.test(tag.color);
            return (
              <li
                key={name}
                role="option"
                aria-selected={i === safeHighlight}
                onMouseDown={(e) => {
                  // mousedown so we run before input loses focus
                  e.preventDefault();
                  complete(name);
                }}
                onMouseEnter={() => setHighlight(i)}
                className={`flex items-center gap-2 px-3 py-1.5 cursor-pointer text-sm ${
                  i === safeHighlight ? "bg-bgmute" : ""
                }`}
              >
                <span
                  className="w-2.5 h-2.5 rounded-full inline-block border border-bgmute"
                  style={{ backgroundColor: valid ? tag!.color : "transparent" }}
                />
                <span>#{name}</span>
              </li>
            );
          })}
        </ul>
      )}
      {preview && input.length > 0 && (
        <div className="text-xs text-fgmute pl-2 flex gap-3 flex-wrap">
          <span>title: <span className="text-fg">{preview.title || "(empty)"}</span></span>
          {preview.priority !== Priority.Normal && (
            <span>!{labelFor(preview.priority)}</span>
          )}
          {preview.tags.length > 0 && <span>tags: {preview.tags.map((t) => `#${t}`).join(" ")}</span>}
          {preview.dueAt && <span>due: {new Date(preview.dueAt * 1000).toLocaleString()}</span>}
        </div>
      )}
    </form>
  );
}

function labelFor(p: number): string {
  switch (p) {
    case Priority.Low:
      return "low";
    case Priority.High:
      return "high";
    case Priority.Urgent:
      return "urgent";
    default:
      return "normal";
  }
}
