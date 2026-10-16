import { describe, expect, it } from "vitest";
import type { Project, Target } from "./api";
import { formatAgo, formatBytes, parentRel } from "./format";
import { idleSelection, isRecent, pruneSelection, selectionTotals } from "./selection";

const DAY = 86400;
const now = 1_800_000_000 * 1000;

function target(path: string, kind: string, bytes: number, safe = true): Target {
  return { path, rel: path, kind, label: "", regen: "", bytes, files: 1, safe, note: "" };
}

function project(path: string, daysAgo: number, targets: Target[]): Project {
  return { path, name: path, git: true, lastActive: now / 1000 - daysAgo * DAY, targets, bytes: 0, android: [] };
}

describe("format", () => {
  it("formats bytes in decimal units", () => {
    expect(formatBytes(512)).toBe("512 B");
    expect(formatBytes(1_500_000)).toBe("1.5 MB");
    expect(formatBytes(12_400_000_000)).toBe("12.4 GB");
    expect(formatBytes(240_000_000)).toBe("240 MB");
  });

  it("formats relative days", () => {
    expect(formatAgo(now / 1000 - 3600, now)).toBe("today");
    expect(formatAgo(now / 1000 - DAY - 10, now)).toBe("yesterday");
    expect(formatAgo(now / 1000 - 10 * DAY, now)).toBe("10 days ago");
    expect(formatAgo(now / 1000 - 70 * DAY, now)).toBe("2 months ago");
    expect(formatAgo(0, now)).toBe("never");
  });

  it("finds the folder a target lives in", () => {
    expect(parentRel("mobile/node_modules")).toBe("mobile");
    expect(parentRel("node_modules")).toBe(".");
  });
});

describe("selection", () => {
  const active = project("/p/active", 0, [target("/p/active/node_modules", "deps", 100)]);
  const old = project("/p/old", 20, [
    target("/p/old/node_modules", "deps", 200),
    target("/p/old/mobile/android", "native", 300),
    target("/p/old/android/.kotlin", "native", 5, false),
  ]);
  const pinnedOld = project("/p/pinned", 40, [target("/p/pinned/node_modules", "deps", 400)]);
  const projects = [active, old, pinnedOld];
  const pinned = new Set(["/p/pinned"]);
  const state = (p: Project) => ({ pinned: pinned.has(p.path), recent: isRecent(p, 2, now) });

  it("selects safe targets of idle, unpinned projects", () => {
    expect([...idleSelection(projects, state, "all")].sort()).toEqual(["/p/old/mobile/android", "/p/old/node_modules"]);
    expect([...idleSelection(projects, state, "deps")]).toEqual(["/p/old/node_modules"]);
  });

  it("totals the selection", () => {
    const sel = new Set(["/p/old/node_modules", "/p/old/mobile/android"]);
    expect(selectionTotals(projects, sel)).toEqual({ bytes: 500, count: 2 });
  });

  it("prunes paths that became unselectable", () => {
    const sel = new Set(["/p/pinned/node_modules", "/p/old/android/.kotlin", "/p/old/node_modules", "/gone"]);
    expect([...pruneSelection(projects, pinned, sel)]).toEqual(["/p/old/node_modules"]);
  });
});
