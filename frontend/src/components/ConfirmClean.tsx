import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { formatBytes, plural } from "@/lib/format";

export interface ConfirmItem {
  key: string;
  title: string;
  detail: string;
  bytes: number;
}

interface Props {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  items: ConfirmItem[];
  trash: boolean;
  onConfirm: () => void;
}

/** Lists exactly what will go before anything is removed. */
export function ConfirmClean({ open, onOpenChange, items, trash, onConfirm }: Props) {
  const total = items.reduce((s, i) => s + i.bytes, 0);
  const verb = trash ? "Move to Trash" : "Delete";
  return (
    <AlertDialog open={open} onOpenChange={onOpenChange}>
      <AlertDialogContent className="sm:max-w-xl">
        <AlertDialogHeader>
          <AlertDialogTitle>
            {verb} {plural(items.length, "item")} ({formatBytes(total)})?
          </AlertDialogTitle>
          <AlertDialogDescription>
            {trash
              ? "They go to the Trash; space is freed once you empty it."
              : "They are deleted right away. Everything listed can be regenerated with the command shown."}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <ul className="selectable max-h-72 divide-y overflow-y-auto rounded-lg border text-sm">
          {items.map((i) => (
            <li key={i.key} className="flex items-start justify-between gap-3 px-3 py-2">
              <div className="min-w-0">
                <div className="truncate font-medium">{i.title}</div>
                <div className="truncate font-mono text-xs text-muted-foreground">{i.detail}</div>
              </div>
              <span className="shrink-0 text-muted-foreground tabular-nums">{formatBytes(i.bytes)}</span>
            </li>
          ))}
        </ul>
        <AlertDialogFooter>
          <AlertDialogCancel>Cancel</AlertDialogCancel>
          <AlertDialogAction variant="destructive" onClick={onConfirm}>
            {verb} {formatBytes(total)}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
