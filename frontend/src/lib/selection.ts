import type { Project, Target } from "./api";

export type KindFilter = "all" | "deps" | "build" | "native" | "artifact";

export interface ProjectState {
  pinned: boolean;
  recent: boolean;
}

/** Whether a project was touched within `days` days. */
export function isRecent(p: Project, days: number, now = Date.now()): boolean {
  return now / 1000 - p.lastActive < days * 86400;
}

export function matchesKind(t: Target, kind: KindFilter): boolean {
  return kind === "all" || t.kind === kind;
}

/** Targets that may be selected: safe ones in projects that aren't pinned. */
export function selectable(p: Project, pinned: boolean): Target[] {
  if (pinned) return [];
  return (p.targets ?? []).filter((t) => t.safe);
}

/**
 * "Select idle": every safe target of the chosen kind in projects that are
 * neither pinned nor recently active.
 */
export function idleSelection(
  projects: Project[],
  state: (p: Project) => ProjectState,
  kind: KindFilter,
): Set<string> {
  const out = new Set<string>();
  for (const p of projects) {
    const s = state(p);
    if (s.recent) continue;
    for (const t of selectable(p, s.pinned)) {
      if (matchesKind(t, kind)) out.add(t.path);
    }
  }
  return out;
}

/** Total bytes and count of the selected targets. */
export function selectionTotals(projects: Project[], selected: Set<string>): { bytes: number; count: number } {
  let bytes = 0;
  let count = 0;
  for (const p of projects) {
    for (const t of p.targets ?? []) {
      if (selected.has(t.path)) {
        bytes += t.bytes;
        count++;
      }
    }
  }
  return { bytes, count };
}

/** Drops selected paths that are no longer selectable (pinned, cleaned, rescanned). */
export function pruneSelection(projects: Project[], pinned: Set<string>, selected: Set<string>): Set<string> {
  const ok = new Set<string>();
  for (const p of projects) {
    for (const t of selectable(p, pinned.has(p.path))) {
      if (selected.has(t.path)) ok.add(t.path);
    }
  }
  return ok;
}
