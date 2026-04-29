import type { TaskPattern } from "./smartText";

// parseTaskPatterns is defensive against malformed KV blobs — a single bad
// row shouldn't take down the whole rendering pipeline. Used by both the
// Settings editor and the Zustand store that hydrates from /api/kv.
export function parseTaskPatterns(raw: string | undefined | null): TaskPattern[] {
  if (!raw) return [];
  try {
    const parsed = JSON.parse(raw);
    if (!Array.isArray(parsed)) return [];
    return parsed
      .filter(
        (e): e is TaskPattern =>
          typeof e === "object" &&
          e !== null &&
          typeof e.pattern === "string" &&
          typeof e.urlTemplate === "string",
      )
      .map((e) => ({ pattern: e.pattern, urlTemplate: e.urlTemplate, name: e.name }));
  } catch {
    return [];
  }
}
