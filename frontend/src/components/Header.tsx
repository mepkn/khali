import { RefreshCwIcon } from "lucide-react";
import { Button } from "@/components/ui/button";
import type { Disk, Progress } from "@/lib/api";
import { formatBytes } from "@/lib/format";
import { cn } from "@/lib/utils";

interface Props {
  disk: Disk | null;
  scanning: boolean;
  progress: Progress | null;
  onRescan: () => void;
}

/** Title bar: the window is dragged by it, and it shows the disk's free space. */
export function Header({ disk, scanning, progress, onRescan }: Props) {
  const used = disk && disk.total > 0 ? (disk.total - disk.free) / disk.total : 0;
  const low = disk ? disk.free < 20e9 : false;

  return (
    <header className="drag flex h-14 shrink-0 items-center gap-4 border-b pr-4 pl-22">
      <div className="flex items-center gap-2">
        <img src="/favicon.svg?v=1" alt="" className="size-6" />
        <span className="font-semibold tracking-tight">Khali</span>
      </div>

      <div className="flex min-w-0 flex-1 items-center justify-end gap-3">
        {disk && (
          <div className="flex w-64 flex-col gap-1">
            <div className="flex items-baseline justify-between text-xs">
              <span className={cn("font-medium", low ? "text-destructive" : "text-foreground")}>
                {formatBytes(disk.free)} free
              </span>
              <span className="text-muted-foreground">of {formatBytes(disk.total)}</span>
            </div>
            <div className="h-1.5 overflow-hidden rounded-full bg-muted">
              <div
                className={cn("h-full rounded-full transition-all", low ? "bg-destructive" : "bg-primary")}
                style={{ width: `${Math.round(used * 100)}%` }}
              />
            </div>
          </div>
        )}
        <Button variant="outline" size="sm" className="no-drag min-w-28" onClick={onRescan} disabled={scanning}>
          <RefreshCwIcon className={cn(scanning && "animate-spin")} />
          {scanning ? scanLabel(progress) : "Rescan"}
        </Button>
      </div>
    </header>
  );
}

function scanLabel(p: Progress | null): string {
  if (!p) return "Scanning…";
  if (p.phase === "finding") return "Finding…";
  return p.total ? `Sizing ${Math.round((p.done / p.total) * 100)}%` : "Sizing…";
}
