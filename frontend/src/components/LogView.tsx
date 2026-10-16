import { useEffect, useState } from "react";
import { Khali, type LogEntry } from "@/lib/api";
import { formatBytes } from "@/lib/format";

const MODE: Record<string, string> = { delete: "Deleted", trash: "Trashed", command: "Ran command" };

/** What khali removed, newest first, with how to get it back. */
export function LogView({ version }: { version: number }) {
  const [entries, setEntries] = useState<LogEntry[]>([]);

  useEffect(() => {
    Khali.Log()
      .then((l) => setEntries(l ?? []))
      .catch(() => {});
  }, [version]);

  if (entries.length === 0) {
    return <p className="p-10 text-center text-sm text-muted-foreground">Nothing removed yet.</p>;
  }
  const freed = entries.reduce((s, e) => s + e.bytes, 0);

  return (
    <div className="h-full overflow-y-auto px-5 py-3">
      <p className="mb-3 text-sm text-muted-foreground">
        <span className="font-medium text-foreground">{formatBytes(freed)}</span> removed in the last {entries.length} entries
      </p>
      <ul className="selectable divide-y rounded-xl border bg-card">
        {entries.map((e, i) => (
          <li key={i} className="flex items-center gap-3 px-3 py-2">
            <div className="min-w-0 flex-1">
              <div className="truncate font-mono text-[0.8rem]">{e.path}</div>
              <div className="truncate text-xs text-muted-foreground">
                {MODE[e.mode] ?? e.mode} · {e.label}
                {e.regen ? ` · ${e.regen}` : ""}
              </div>
            </div>
            <span className="shrink-0 text-xs text-muted-foreground">{new Date(e.time * 1000).toLocaleString()}</span>
            <span className="w-16 text-right text-sm text-muted-foreground tabular-nums">{formatBytes(e.bytes)}</span>
          </li>
        ))}
      </ul>
    </div>
  );
}
