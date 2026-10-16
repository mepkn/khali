import { useCallback, useEffect, useRef, useState } from "react";
import { Events } from "@wailsio/runtime";
import { toast } from "sonner";
import { Toaster } from "@/components/ui/sonner";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { TooltipProvider } from "@/components/ui/tooltip";
import { Header } from "@/components/Header";
import { ProjectsView } from "@/components/ProjectsView";
import { CachesView } from "@/components/CachesView";
import { LogView } from "@/components/LogView";
import { SettingsView } from "@/components/SettingsView";
import { Khali, errorMessage, type CleanProgress, type CleanReport, type Disk, type Progress, type ScanResult, type Settings } from "@/lib/api";
import { formatBytes, plural } from "@/lib/format";

export default function App() {
  const [settings, setSettings] = useState<Settings | null>(null);
  const [disk, setDisk] = useState<Disk | null>(null);
  const [scan, setScan] = useState<ScanResult | null>(null);
  const [scanning, setScanning] = useState(false);
  const [progress, setProgress] = useState<Progress | null>(null);
  const [cleanProgress, setCleanProgress] = useState<CleanProgress | null>(null);
  const [tab, setTab] = useState("projects");
  const [logVersion, setLogVersion] = useState(0);
  const started = useRef(false);

  const refreshDisk = useCallback(() => {
    Khali.Disk().then(setDisk).catch(() => {});
  }, []);

  const runScan = useCallback(async () => {
    setScanning(true);
    setProgress(null);
    try {
      setScan(await Khali.Scan());
      refreshDisk();
    } catch (e) {
      toast.error(errorMessage(e));
    } finally {
      setScanning(false);
      setProgress(null);
    }
  }, [refreshDisk]);

  useEffect(() => {
    const offScan = Events.On("scan:progress", (e) => setProgress(e.data as Progress));
    const offClean = Events.On("clean:progress", (e) => setCleanProgress(e.data as CleanProgress));
    Khali.GetSettings().then(setSettings).catch((e) => toast.error(errorMessage(e)));
    refreshDisk();
    // Reuse this session's scan when the window is reloaded; scan otherwise.
    // The ref keeps StrictMode's second effect run from scanning twice.
    if (!started.current) {
      started.current = true;
      Khali.LastScan()
        .then((last) => (last ? setScan(last) : runScan()))
        .catch(() => runScan());
    }
    const timer = setInterval(refreshDisk, 30_000);
    return () => {
      offScan();
      offClean();
      clearInterval(timer);
    };
  }, [refreshDisk, runScan]);

  // Shared by projects and caches: run a clean, then report what it freed.
  const clean = async (run: () => Promise<CleanReport>): Promise<CleanReport | null> => {
    setCleanProgress(null);
    try {
      const rep = await run();
      setDisk(rep.after);
      setLogVersion((v) => v + 1);
      const removed = rep.removed?.length ?? 0;
      const gained = rep.after.free - rep.before.free;
      if (removed > 0) {
        toast.success(`Removed ${plural(removed, "item")}`, {
          description: `${formatBytes(rep.freed)} cleaned${gained > 0 ? `, disk has ${formatBytes(gained)} more free` : ""}${settings?.trash ? " (empty the Trash to free it)" : ""}.`,
        });
      }
      for (const err of rep.errors ?? []) {
        toast.error(`Couldn't remove ${err.path}`, { description: err.error });
      }
      return rep;
    } catch (e) {
      toast.error(errorMessage(e));
      return null;
    } finally {
      setCleanProgress(null);
    }
  };

  return (
    <TooltipProvider>
      <div className="flex h-screen flex-col">
        <Header disk={disk} scanning={scanning} progress={progress} onRescan={runScan} />
        <Tabs value={tab} onValueChange={(v) => setTab(String(v))} className="flex min-h-0 flex-1 flex-col gap-0">
          <div className="border-b px-5">
            <TabsList variant="line">
              <TabsTrigger value="projects">Projects</TabsTrigger>
              <TabsTrigger value="caches">Caches</TabsTrigger>
              <TabsTrigger value="log">Log</TabsTrigger>
              <TabsTrigger value="settings">Settings</TabsTrigger>
            </TabsList>
          </div>
          <TabsContent value="projects" className="min-h-0 flex-1">
            <ProjectsView
              scan={scan}
              scanning={scanning}
              settings={settings}
              onSettings={setSettings}
              onClean={(paths) => clean(() => Khali.CleanTargets(paths))}
              onScanned={setScan}
              cleanProgress={cleanProgress}
            />
          </TabsContent>
          <TabsContent value="caches" className="min-h-0 flex-1">
            <CachesView
              trash={settings?.trash ?? false}
              projectsScanning={scanning}
              onClean={(ids) => clean(() => Khali.CleanCaches(ids))}
              cleanProgress={cleanProgress}
            />
          </TabsContent>
          <TabsContent value="log" className="min-h-0 flex-1">
            <LogView version={logVersion} />
          </TabsContent>
          <TabsContent value="settings" className="min-h-0 flex-1">
            {settings && (
              <SettingsView
                settings={settings}
                onSaved={(s) => {
                  setSettings(s);
                  toast.success("Settings saved");
                }}
                onRescan={runScan}
              />
            )}
          </TabsContent>
        </Tabs>
      </div>
      <Toaster position="bottom-right" />
    </TooltipProvider>
  );
}
