import { describe, it, expect } from "vitest";
import { parseQuickAdd } from "./parser";
import { Priority } from "./types";

describe("parseQuickAdd", () => {
  it("returns plain title when there are no tokens", () => {
    const r = parseQuickAdd("Buy milk");
    expect(r.title).toBe("Buy milk");
    expect(r.priority).toBe(Priority.Normal);
    expect(r.tags).toEqual([]);
    expect(r.dueAt).toBeUndefined();
  });

  it("parses priority", () => {
    const r = parseQuickAdd("Fix bug !urgent");
    expect(r.title).toBe("Fix bug");
    expect(r.priority).toBe(Priority.Urgent);
  });

  it("parses multiple tags", () => {
    const r = parseQuickAdd("Review MR #backend #review");
    expect(r.title).toBe("Review MR");
    expect(r.tags).toEqual(["backend", "review"]);
  });

  it("parses due:YYYY-MM-DD as 18:00 local that day", () => {
    const r = parseQuickAdd("Ship release due:2026-05-01");
    expect(r.title).toBe("Ship release");
    expect(r.dueAt).toBeDefined();
    const d = new Date(r.dueAt! * 1000);
    expect(d.getFullYear()).toBe(2026);
    expect(d.getMonth()).toBe(4);
    expect(d.getDate()).toBe(1);
    expect(d.getHours()).toBe(18);
  });

  it("parses due:today/tomorrow", () => {
    const r1 = parseQuickAdd("X due:today");
    const r2 = parseQuickAdd("Y due:tomorrow");
    expect(r1.dueAt).toBeDefined();
    expect(r2.dueAt).toBeDefined();
    expect(r2.dueAt!).toBeGreaterThan(r1.dueAt!);
  });

  it("handles all tokens together", () => {
    const r = parseQuickAdd("Fix login bug !urgent #backend #review due:2026-05-01");
    expect(r.title).toBe("Fix login bug");
    expect(r.priority).toBe(Priority.Urgent);
    expect(r.tags).toEqual(["backend", "review"]);
    expect(r.dueAt).toBeDefined();
  });

  it("ignores unknown ! / # / due: tokens (passes them through as title)", () => {
    const r = parseQuickAdd("Try !nope #with-dash due:badformat");
    expect(r.title).toBe("Try !nope due:badformat");
    expect(r.tags).toEqual(["with-dash"]);
  });

  it("trims and collapses spaces in remaining title", () => {
    const r = parseQuickAdd("  Fix    !high   #x   bug  ");
    expect(r.title).toBe("Fix bug");
    expect(r.priority).toBe(Priority.High);
  });

  it("returns empty title if input is only tokens", () => {
    const r = parseQuickAdd("!high #only");
    expect(r.title).toBe("");
    expect(r.priority).toBe(Priority.High);
    expect(r.tags).toEqual(["only"]);
  });
});
