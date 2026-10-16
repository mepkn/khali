import { Loader2Icon, Trash2Icon } from "lucide-react";
import { Button } from "@/components/ui/button";
import type { CleanProgress } from "@/lib/api";
import { formatBytes, plural } from "@/lib/format";

interface Props {
  count: number;
  bytes: number;
  trash: boolean;
  cleaning: boolean;
  progress: CleanProgress | null;
  onClear: () => void;
  onClean: () => void;
  children?: React.ReactNode;
}

/** Bottom bar with the selection total and the clean button. */
export function CleanBar({ count, bytes, trash, cleaning, progress, onClear, onClean, children }: Props) {
  return (
    <div className="flex h-14 shrink-0 items-center gap-3 border-t bg-card px-5">
      <div className="flex min-w-0 flex-1 items-center gap-3 text-sm">
        {cleaning ? (
          <span className="flex min-w-0 items-center gap-2 text-muted-foreground">
            <Loader2Icon className="size-4 shrink-0 animate-spin" />
            <span className="truncate">
              Removing {progress ? `${Math.min(progress.done + 1, progress.total)} of ${progress.total}` : "…"}
              {progress?.current ? ` · ${progress.current}` : ""}
            </span>
          </span>
        ) : count > 0 ? (
          <>
            <span>
              <span className="font-semibold tabular-nums">{formatBytes(bytes)}</span>
              <span className="text-muted-foreground"> in {plural(count, "item")}</span>
            </span>
            <Button variant="ghost" size="xs" onClick={onClear}>
              Clear
            </Button>
          </>
        ) : (
          <span className="text-muted-foreground">Nothing selected</span>
        )}
        {children}
      </div>
      <Button variant="destructive" disabled={count === 0 || cleaning} onClick={onClean}>
        <Trash2Icon />
        {trash ? "Move to Trash" : "Delete"}
      </Button>
    </div>
  );
}
