// Pick a readable text color (black or white) for an arbitrary background hex.
// Uses the YIQ luma trick — good enough for rough contrast without pulling in a
// color library. Returns the CSS fgmute custom-property when the input isn't a
// 6-digit hex, so callers get a sane default for unrecognised values.
export function readableText(hex: string): string {
  if (!/^#[0-9a-fA-F]{6}$/.test(hex)) return "var(--color-fgmute)";
  const r = parseInt(hex.slice(1, 3), 16);
  const g = parseInt(hex.slice(3, 5), 16);
  const b = parseInt(hex.slice(5, 7), 16);
  const yiq = (r * 299 + g * 587 + b * 114) / 1000;
  return yiq >= 140 ? "#11111b" : "#cdd6f4";
}
