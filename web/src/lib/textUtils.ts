// firstLine returns the first line of `s`, optionally truncated to `max`
// characters with an ellipsis. Used wherever a multi-line todo body needs to
// collapse into a single-row preview.
export function firstLine(s: string, max?: number): string {
  const nl = s.indexOf("\n");
  const line = nl === -1 ? s : s.slice(0, nl);
  if (max !== undefined && line.length > max) return line.slice(0, max) + "…";
  return line;
}
