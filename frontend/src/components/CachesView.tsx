import { useMemo, useState } from "react";
import { DatabaseIcon, Loader2Icon, LockIcon } from "lucide-react";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Switch } from "@/components/ui/switch";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { CleanBar } from "@/components/CleanBar";
import { ConfirmClean } from "@/components/ConfirmClean";
import { Khali, errorMessage, type CacheItem, type CleanProgress, type CleanReport } from "@/lib/api";
import { formatBytes } from "@/lib/format";
import { cn } from "@/lib/utils";

const STATUS: Record<string, { label: string; variant: "default" | "secondary" | "outline" | "destructive" }> = {
  unused: { label: "Unused version", variant: "default" },
  maybe: { label: "Probably unused", variant: "outline" },
  cache: { label: "Cache", variant: "secondary" },
  inUse: { label: "In use", variant: "outline" },
};

interface Props {
  trash: boolean;
  projectsScanning: boolean;
  onClean: (ids: string[]) => Promise<CleanReport | null>;
  cleanProgress: CleanProgress | null;
}

export function CachesView({ trash, projectsScanning, onClean, cleanProgress }: Props) {
  const [items, setItems] = useState<CacheItem[] | null>(null);
  const [loading, setLoading] = useState(false);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [showInUse, setShowInUse] = useState(false);
  const [confirming, setConfirming] = useState(false);
  const [cleaning, setCleaning] = useState(false);

  const load = async () => {
    setLoading(true);
    try {
      setItems((await Khali.ScanCaches()) ?? []);
      setSelected(new Set());
    } catch (e) {
      toast.error(errorMessage(e));
    } finally {
      setLoading(false);
    }
  };

  const groups = useMemo(() => {
    const out = new Map<string, CacheItem[]>();
    for (const it of items ?? []) {
      if (it.bytes < 1e6) continue; // empty folders aren't worth a row
      if (it.status === "inUse" && !showInUse) continue;
      out.set(it.group, [...(out.get(it.group) ?? []), it]);
    }
    for (const list of out.values()) list.sort((a, b) => b.bytes - a.bytes);
    return [...out.entries()].sort((a, b) => total(b[1]) - total(a[1]));
  }, [items, showInUse]);

  const chosen = (items ?? []).filter((it) => selected.has(it.id));
  const unusedIds = (items ?? []).filter((it) => it.status === "unused" && it.bytes >= 1e6).map((it) => it.id);

  const toggle = (ids: string[], on: boolean) => {
    const next = new Set(selected);
    for (const id of ids) {
      if (on) next.add(id);
      else next.delete(id);
    }
    setSelected(next);
  };

  const runClean = async () => {
    setConfirming(false);
    setCleaning(true);
    const rep = await onClean([...selected]);
    if (rep) {
      const gone = new Set(rep.removed ?? []);
      setItems((cur) => (cur ?? []).filter((it) => !gone.has(it.id)));
      setSelected(new Set());
    }
    setCleaning(false);
  };

  if (!items) {
    return (
      <div className="flex h-full flex-col items-center justify-center gap-3 text-center">
        <DatabaseIcon className="size-8 text-muted-foreground" />
        <div className="max-w-md text-sm text-muted-foreground">
          npm, Gradle, Android SDK versions, Xcode, CocoaPods, Go and other shared caches. Versions your projects
          still need are marked in use and can't be selected.
        </div>
        <Button onClick={load} disabled={loading || projectsScanning}>
          {loading && <Loader2Icon className="animate-spin" />}
          {loading ? "Measuring caches…" : "Scan caches"}
        </Button>
      </div>
    );
  }

  return (
    <div className="flex h-full flex-col">
      <div className="flex items-center gap-3 px-5 py-3">
        <span className="text-sm text-muted-foreground">
          <span className="font-medium text-foreground">{formatBytes(total(groups.flatMap(([, l]) => l.filter((i) => i.status !== "inUse"))))}</span>{" "}
          in shared caches
        </span>
        <label className="flex items-center gap-2 text-xs text-muted-foreground">
          <Switch size="sm" checked={showInUse} onCheckedChange={setShowInUse} />
          Show in use
        </label>
        <div className="flex-1" />
        <Button variant="outline" size="sm" disabled={unusedIds.length === 0} onClick={() => setSelected(new Set(unusedIds))}>
          Select unused versions
        </Button>
        <Button variant="outline" size="sm" onClick={load} disabled={loading}>
          {loading && <Loader2Icon className="animate-spin" />}
          Rescan
        </Button>
      </div>

      <div className="min-h-0 flex-1 space-y-4 overflow-y-auto px-5 pb-4">
        {groups.map(([group, list]) => (
          <section key={group}>
            <h3 className="mb-1.5 flex items-baseline justify-between px-1 text-xs font-medium tracking-wide text-muted-foreground uppercase">
              {group}
              <span className="tabular-nums normal-case">{formatBytes(total(list))}</span>
            </h3>
            <ul className="divide-y rounded-xl border bg-card">
              {list.map((it) => (
                <CacheRow key={it.id} item={it} checked={selected.has(it.id)} onToggle={(on) => toggle([it.id], on)} />
              ))}
            </ul>
          </section>
        ))}
      </div>

      <CleanBar
        count={chosen.length}
        bytes={total(chosen)}
        trash={trash}
        cleaning={cleaning}
        progress={cleanProgress}
        onClear={() => setSelected(new Set())}
        onClean={() => setConfirming(true)}
      />
      <ConfirmClean
        open={confirming}
        onOpenChange={setConfirming}
        items={chosen.map((it) => ({
          key: it.id,
          title: `${it.group}: ${it.label}`,
          detail: it.command?.length ? `runs ${it.command.join(" ")}` : it.path,
          bytes: it.bytes,
        }))}
        trash={trash}
        onConfirm={runClean}
      />
    </div>
  );
}

function CacheRow({ item: it, checked, onToggle }: { item: CacheItem; checked: boolean; onToggle: (on: boolean) => void }) {
  const status = STATUS[it.status] ?? STATUS.cache;
  const inUse = it.status === "inUse";
  return (
    <li className={cn("group flex items-center gap-3 px-3 py-2", inUse && "opacity-60")}>
      {inUse ? (
        <Tooltip>
          <TooltipTrigger render={<span className="flex size-4 items-center justify-center text-muted-foreground" />}>
            <LockIcon className="size-3.5" />
          </TooltipTrigger>
          <TooltipContent>{it.note}</TooltipContent>
        </Tooltip>
      ) : (
        <Checkbox checked={checked} onCheckedChange={onToggle} aria-label={`Select ${it.label}`} />
      )}
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <span className="truncate text-sm font-medium">{it.label}</span>
          <Badge variant={status.variant}>{status.label}</Badge>
        </div>
        <div className="truncate text-xs text-muted-foreground">{it.note || it.path}</div>
      </div>
      {it.path && (
        <Button variant="ghost" size="xs" className="opacity-0 group-hover:opacity-100" onClick={() => Khali.Reveal(it.path).catch(() => {})}>
          Show in Finder
        </Button>
      )}
      <span className="w-16 text-right text-sm text-muted-foreground tabular-nums">{formatBytes(it.bytes)}</span>
    </li>
  );
}

function total(items: CacheItem[]): number {
  return items.reduce((s, i) => s + i.bytes, 0);
}
