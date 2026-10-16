import { useState } from "react";
import { FolderPlusIcon, XIcon } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Khali, errorMessage, type Settings } from "@/lib/api";

interface Props {
  settings: Settings;
  onSaved: (s: Settings) => void;
  onRescan: () => void;
}

export function SettingsView({ settings, onSaved, onRescan }: Props) {
  const [draft, setDraft] = useState<Settings>(settings);
  // Pins change from the projects list while this page is open, so only the
  // fields edited here are compared and saved.
  const editable = (s: Settings) => JSON.stringify([s.roots, s.trash, s.recentDays]);
  const dirty = editable(draft) !== editable(settings);
  const rootsChanged = JSON.stringify(draft.roots) !== JSON.stringify(settings.roots);

  const addRoot = async () => {
    try {
      const dir = await Khali.PickFolder();
      if (dir && !(draft.roots ?? []).includes(dir)) setDraft({ ...draft, roots: [...(draft.roots ?? []), dir] });
    } catch (e) {
      toast.error(errorMessage(e));
    }
  };

  const save = async () => {
    try {
      await Khali.SaveSettings({ ...settings, roots: draft.roots, trash: draft.trash, recentDays: draft.recentDays });
      const saved = await Khali.GetSettings();
      setDraft(saved);
      onSaved(saved);
      if (rootsChanged) onRescan();
    } catch (e) {
      toast.error(errorMessage(e));
    }
  };

  return (
    <div className="h-full overflow-y-auto">
      <div className="mx-auto max-w-2xl space-y-8 px-5 py-6">
        <section className="space-y-3">
          <div>
            <h3 className="font-medium">Project folders</h3>
            <p className="text-sm text-muted-foreground">
              Searched up to four levels deep for git repos and folders with a package.json, go.mod or app.json.
            </p>
          </div>
          <ul className="divide-y rounded-xl border bg-card">
            {(draft.roots ?? []).map((r) => (
              <li key={r} className="flex items-center gap-2 px-3 py-2">
                <span className="selectable min-w-0 flex-1 truncate font-mono text-[0.8rem]">{r}</span>
                <Button
                  variant="ghost"
                  size="icon-xs"
                  aria-label={`Remove ${r}`}
                  onClick={() => setDraft({ ...draft, roots: (draft.roots ?? []).filter((x) => x !== r) })}
                >
                  <XIcon />
                </Button>
              </li>
            ))}
            {(draft.roots ?? []).length === 0 && <li className="px-3 py-2 text-sm text-muted-foreground">No folders yet.</li>}
          </ul>
          <Button variant="outline" size="sm" onClick={addRoot}>
            <FolderPlusIcon />
            Add folder
          </Button>
        </section>

        <section className="space-y-3">
          <h3 className="font-medium">Cleaning</h3>
          <div className="flex items-start justify-between gap-6 rounded-xl border bg-card p-3">
            <div>
              <Label htmlFor="trash">Move to Trash instead of deleting</Label>
              <p className="text-sm text-muted-foreground">Safer, but no space is freed until you empty the Trash.</p>
            </div>
            <Switch id="trash" checked={draft.trash} onCheckedChange={(trash) => setDraft({ ...draft, trash })} />
          </div>
          <div className="flex items-start justify-between gap-6 rounded-xl border bg-card p-3">
            <div>
              <Label htmlFor="recent">Active project window (days)</Label>
              <p className="text-sm text-muted-foreground">
                Projects with edits or commits in this many days are marked active and skipped by “Select idle”.
              </p>
            </div>
            <Input
              id="recent"
              type="number"
              min={1}
              max={60}
              className="w-20"
              value={draft.recentDays}
              onChange={(e) => setDraft({ ...draft, recentDays: Math.max(1, Number(e.target.value) || 1) })}
            />
          </div>
        </section>

        <div className="flex justify-end gap-2">
          <Button variant="ghost" disabled={!dirty} onClick={() => setDraft(settings)}>
            Reset
          </Button>
          <Button disabled={!dirty} onClick={save}>
            Save
          </Button>
        </div>
      </div>
    </div>
  );
}
