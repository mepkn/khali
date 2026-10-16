import { useMemo, useState } from "react";
import { ChevronRightIcon, FolderSearchIcon, LockIcon, PinIcon, PinOffIcon, SparklesIcon } from "lucide-react";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { CleanBar } from "@/components/CleanBar";
import { ConfirmClean, type ConfirmItem } from "@/components/ConfirmClean";
import { Khali, errorMessage, type CleanProgress, type CleanReport, type Project, type ScanResult, type Settings, type Target } from "@/lib/api";
import { formatAgo, formatBytes, parentRel, plural } from "@/lib/format";
import { idleSelection, isRecent, matchesKind, pruneSelection, selectable, selectionTotals, type KindFilter } from "@/lib/selection";
import { cn } from "@/lib/utils";

const KINDS: { value: KindFilter; label: string }[] = [
  { value: "all", label: "All" },
  { value: "deps", label: "node_modules" },
  { value: "build", label: "Build output" },
  { value: "native", label: "Native" },
  { value: "artifact", label: "App files" },
];

interface Props {
  scan: ScanResult | null;
  scanning: boolean;
  settings: Settings | null;
  onSettings: (s: Settings) => void;
  onClean: (paths: string[]) => Promise<CleanReport | null>;
  onScanned: (r: ScanResult | null) => void;
  cleanProgress: CleanProgress | null;
}

export function ProjectsView({ scan, scanning, settings, onSettings, onClean, onScanned, cleanProgress }: Props) {
  const [kind, setKind] = useState<KindFilter>("all");
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [expanded, setExpanded] = useState<Set<string>>(new Set());
  const [confirming, setConfirming] = useState(false);
  const [cleaning, setCleaning] = useState(false);

  const pinned = useMemo(() => new Set(settings?.pinned ?? []), [settings]);
  const recentDays = settings?.recentDays ?? 2;
  const projects = useMemo(() => scan?.projects ?? [], [scan]);
  // Selection is pruned as projects get pinned or cleaned.
  const sel = useMemo(() => pruneSelection(projects, pinned, selected), [projects, pinned, selected]);

  const visible = useMemo(
    () =>
      projects
        .map((p) => ({ p, targets: (p.targets ?? []).filter((t) => matchesKind(t, kind)) }))
        .filter((v) => v.targets.length > 0),
    [projects, kind],
  );
  const emptyCount = projects.length - visible.length;
  const maxBytes = Math.max(1, ...visible.map((v) => sum(v.targets.filter((t) => t.safe))));
  const totalSafe = sum(visible.flatMap((v) => v.targets.filter((t) => t.safe)));
  const totals = selectionTotals(projects, sel);

  const toggle = (paths: string[], on: boolean) => {
    const next = new Set(sel);
    for (const p of paths) {
      if (on) next.add(p);
      else next.delete(p);
    }
    setSelected(next);
  };

  const setPin = async (p: Project, pin: boolean) => {
    try {
      onSettings(await Khali.SetPinned(p.path, pin));
    } catch (e) {
      toast.error(errorMessage(e));
    }
  };

  const selectIdle = () => {
    const next = idleSelection(projects, (p) => ({ pinned: pinned.has(p.path), recent: isRecent(p, recentDays) }), kind);
    setSelected(next);
    if (next.size === 0) toast.info("Nothing idle to select", { description: `Projects used in the last ${plural(recentDays, "day")} and pinned ones are skipped.` });
  };

  const confirmItems: ConfirmItem[] = projects.flatMap((p) =>
    (p.targets ?? [])
      .filter((t) => sel.has(t.path))
      .map((t) => ({ key: t.path, title: `${p.name}/${t.rel}`, detail: regenText(t), bytes: t.bytes })),
  );

  const runClean = async () => {
    setConfirming(false);
    setCleaning(true);
    const rep = await onClean([...sel]);
    if (rep) {
      setSelected(new Set());
      onScanned(await Khali.LastScan().catch(() => scan));
    }
    setCleaning(false);
  };

  if (!scan) {
    return (
      <div className="flex h-full flex-col items-center justify-center gap-2 text-muted-foreground">
        <FolderSearchIcon className="size-8" />
        <p className="text-sm">{scanning ? "Looking through your projects…" : "No scan yet."}</p>
      </div>
    );
  }

  return (
    <div className="flex h-full flex-col">
      <div className="flex flex-wrap items-center gap-2 px-5 py-3">
        <div className="flex rounded-lg border p-0.5">
          {KINDS.map((k) => (
            <button
              key={k.value}
              onClick={() => setKind(k.value)}
              className={cn(
                "rounded-md px-2.5 py-1 text-xs font-medium transition-colors",
                kind === k.value ? "bg-secondary text-foreground" : "text-muted-foreground hover:text-foreground",
              )}
            >
              {k.label}
            </button>
          ))}
        </div>
        <span className="text-sm text-muted-foreground">
          <span className="font-medium text-foreground">{formatBytes(totalSafe)}</span> can be cleaned in{" "}
          {plural(visible.length, "project")}
        </span>
        <div className="flex-1" />
        <Tooltip>
          <TooltipTrigger render={<Button variant="outline" size="sm" onClick={selectIdle} />}>
            <SparklesIcon />
            Select idle
          </TooltipTrigger>
          <TooltipContent>Everything in projects not used for {plural(recentDays, "day")}, except pinned ones</TooltipContent>
        </Tooltip>
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto px-5 pb-4">
        <div className="divide-y rounded-xl border bg-card">
          {visible.map(({ p, targets }) => (
            <ProjectRow
              key={p.path}
              project={p}
              targets={targets}
              pinned={pinned.has(p.path)}
              recent={isRecent(p, recentDays)}
              expanded={expanded.has(p.path)}
              onExpand={() => {
                const next = new Set(expanded);
                if (next.has(p.path)) next.delete(p.path);
                else next.add(p.path);
                setExpanded(next);
              }}
              selected={sel}
              onToggle={toggle}
              onPin={(on) => setPin(p, on)}
              share={sum(targets.filter((t) => t.safe)) / maxBytes}
            />
          ))}
          {visible.length === 0 && <p className="p-6 text-center text-sm text-muted-foreground">Nothing to clean here.</p>}
        </div>
        {emptyCount > 0 && (
          <p className="mt-3 text-center text-xs text-muted-foreground">
            {plural(emptyCount, "other project")} {emptyCount === 1 ? "has" : "have"} nothing to clean
            {kind !== "all" ? " of this kind" : ""}.
          </p>
        )}
      </div>

      <CleanBar
        count={totals.count}
        bytes={totals.bytes}
        trash={settings?.trash ?? false}
        cleaning={cleaning}
        progress={cleanProgress}
        onClear={() => setSelected(new Set())}
        onClean={() => setConfirming(true)}
      />
      <ConfirmClean
        open={confirming}
        onOpenChange={setConfirming}
        items={confirmItems}
        trash={settings?.trash ?? false}
        onConfirm={runClean}
      />
    </div>
  );
}

interface RowProps {
  project: Project;
  targets: Target[];
  pinned: boolean;
  recent: boolean;
  expanded: boolean;
  onExpand: () => void;
  selected: Set<string>;
  onToggle: (paths: string[], on: boolean) => void;
  onPin: (on: boolean) => void;
  share: number;
}

function ProjectRow({ project: p, targets, pinned, recent, expanded, onExpand, selected, onToggle, onPin, share }: RowProps) {
  const allowed = selectable(p, pinned)
    .filter((t) => targets.includes(t))
    .map((t) => t.path);
  const picked = allowed.filter((path) => selected.has(path)).length;
  const bytes = sum(targets.filter((t) => t.safe));

  return (
    <div className={cn(pinned && "bg-muted/40")}>
      <div className="flex items-center gap-3 px-3 py-2.5">
        <button onClick={onExpand} className="rounded p-0.5 text-muted-foreground hover:text-foreground" aria-label="Show folders">
          <ChevronRightIcon className={cn("size-4 transition-transform", expanded && "rotate-90")} />
        </button>
        <Checkbox
          checked={allowed.length > 0 && picked === allowed.length}
          indeterminate={picked > 0 && picked < allowed.length}
          disabled={allowed.length === 0}
          onCheckedChange={(on) => onToggle(allowed, on)}
          aria-label={`Select ${p.name}`}
        />
        <button onClick={onExpand} className="flex min-w-0 flex-1 items-center gap-2 text-left">
          <span className="truncate font-medium">{p.name}</span>
          {pinned && <Badge variant="secondary">Pinned</Badge>}
          {!pinned && recent && <Badge variant="outline">Active</Badge>}
          {!p.git && <Badge variant="outline">No git</Badge>}
        </button>
        <span className="hidden w-28 text-right text-xs text-muted-foreground md:block">{formatAgo(p.lastActive)}</span>
        <div className="flex w-36 items-center gap-2">
          <div className="h-1.5 flex-1 overflow-hidden rounded-full bg-muted">
            <div className="h-full rounded-full bg-primary/70" style={{ width: `${Math.max(2, share * 100)}%` }} />
          </div>
          <span className="w-16 text-right text-sm tabular-nums">{formatBytes(bytes)}</span>
        </div>
        <Tooltip>
          <TooltipTrigger render={<Button variant="ghost" size="icon-sm" onClick={() => onPin(!pinned)} />}>
            {pinned ? <PinOffIcon /> : <PinIcon />}
          </TooltipTrigger>
          <TooltipContent>{pinned ? "Unpin" : "Pin: the project you're working on is never cleaned"}</TooltipContent>
        </Tooltip>
      </div>

      {expanded && (
        <ul className="pb-2 pl-14">
          {targets.map((t) => (
            <TargetRow
              key={t.path}
              target={t}
              disabled={pinned || !t.safe}
              checked={selected.has(t.path)}
              onToggle={(on) => onToggle([t.path], on)}
            />
          ))}
        </ul>
      )}
    </div>
  );
}

function TargetRow({ target: t, disabled, checked, onToggle }: { target: Target; disabled: boolean; checked: boolean; onToggle: (on: boolean) => void }) {
  return (
    <li className="group flex items-center gap-3 py-1.5 pr-3">
      {t.safe ? (
        <Checkbox checked={checked} disabled={disabled} onCheckedChange={onToggle} aria-label={`Select ${t.rel}`} />
      ) : (
        <Tooltip>
          <TooltipTrigger render={<span className="flex size-4 items-center justify-center text-muted-foreground" />}>
            <LockIcon className="size-3.5" />
          </TooltipTrigger>
          <TooltipContent>Kept: {t.note}</TooltipContent>
        </Tooltip>
      )}
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <span className="truncate font-mono text-[0.8rem]">{t.rel}</span>
          <span className="shrink-0 text-xs text-muted-foreground">{t.label}</span>
        </div>
        <div className="truncate text-xs text-muted-foreground">{t.safe ? regenText(t) : `Kept: ${t.note}`}</div>
      </div>
      <Button
        variant="ghost"
        size="xs"
        className="opacity-0 group-hover:opacity-100"
        onClick={() => Khali.Reveal(t.path).catch(() => {})}
      >
        Show in Finder
      </Button>
      <span className="w-16 text-right text-sm text-muted-foreground tabular-nums">{formatBytes(t.bytes)}</span>
    </li>
  );
}

function regenText(t: Target): string {
  const dir = parentRel(t.rel);
  const where = dir === "." ? "" : ` in ${dir}/`;
  if (t.regen.includes(" ") && /^(npm|npx|pnpm|yarn|bun|pod)\b/.test(t.regen)) return `Get back: ${t.regen}${where}`;
  return `Get back: ${t.regen}`;
}

function sum(ts: Target[]): number {
  return ts.reduce((s, t) => s + t.bytes, 0);
}
