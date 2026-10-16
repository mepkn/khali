// One place for the generated Go bindings and the types the UI uses.
export { Khali } from "../../bindings/khali";
export type { CleanReport, CleanProgress, Disk } from "../../bindings/khali";
export type { Item as CacheItem } from "../../bindings/khali/internal/caches";
export type { Project, Target, Progress, Result as ScanResult } from "../../bindings/khali/internal/scan";
export type { Settings, LogEntry } from "../../bindings/khali/internal/store";

/** Turns a rejected Wails call into a readable message. */
export function errorMessage(e: unknown): string {
  if (e instanceof Error) return e.message;
  if (typeof e === "string") return e;
  if (e && typeof e === "object" && "message" in e) return String((e as { message: unknown }).message);
  return "Something went wrong";
}
