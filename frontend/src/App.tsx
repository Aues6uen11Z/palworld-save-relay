import { useEffect, useState, useCallback } from "react";
import { Dialogs } from "@wailsio/runtime";
import { App } from "../bindings/palworld-save-relay";
import type { World, BackupRecord } from "../bindings/palworld-save-relay/models";
import type { Player } from "../bindings/palworld-save-relay/internal/palworld/models";
import type { Config } from "../bindings/palworld-save-relay/internal/config/models";

type View = "worlds" | "cloud" | "backups" | "settings";

function defaultConfig(): Config {
  return {
    qiniu: { access_key: "", secret_key: "", bucket: "", region: "", domain: "" },
    uploader: "",
    save_root: "",
    world_aliases: {},
    hidden_worlds: {},
    backup_keep: 5,
    lock_ttl: 0,
  } as unknown as Config;
}

export default function AppView() {
  const [view, setView] = useState<View>("worlds");
  const [cfg, setCfg] = useState<Config>(defaultConfig());
  const [worlds, setWorlds] = useState<World[]>([]);
  const [selWorld, setSelWorld] = useState<World | null>(null);
  const [players, setPlayers] = useState<Player[]>([]);
  const [busy, setBusy] = useState(false);
  const [toast, setToast] = useState<{ kind: "ok" | "err"; msg: string } | null>(null);
  const [saveRoot, setSaveRoot] = useState("");
  const [detectErr, setDetectErr] = useState("");

  const flash = (kind: "ok" | "err", msg: string) => {
    setToast({ kind, msg });
    setTimeout(() => setToast(null), 4000);
  };

  const refreshWorlds = useCallback(async () => {
    setDetectErr("");
    try {
      const ws = await App.DetectWorlds();
      setWorlds(ws || []);
      if (ws && ws.length && !selWorld) setSelWorld(ws[0]);
    } catch (e: any) {
      setDetectErr(String(e?.message || e));
      flash("err", "检测世界失败: " + (e?.message || e));
    }
    try { setSaveRoot(await App.ResolvedSaveRoot()); } catch {}
  }, [selWorld]);

  useEffect(() => {
    (async () => {
      try {
        const c = await App.GetConfig();
        if (c) setCfg(c);
      } catch (e: any) {
        flash("err", "加载配置失败: " + (e?.message || e));
      }
      await refreshWorlds();
    })();
  }, [refreshWorlds]);

  const selectWorld = async (w: World) => {
    setSelWorld(w);
    try {
      setPlayers((await App.ListPlayers(w.Path)) || []);
    } catch (e: any) {
      flash("err", String(e?.message || e));
    }
  };

  useEffect(() => {
    if (selWorld) selectWorld(selWorld);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selWorld?.Path]);

  const run = async (label: string, fn: () => Promise<void>) => {
    setBusy(true);
    try {
      await fn();
      flash("ok", label + " 完成");
      if (selWorld) await selectWorld(selWorld);
    } catch (e: any) {
      flash("err", label + " 失败: " + (e?.message || e));
    } finally {
      setBusy(false);
    }
  };

  const needsConfig = !cfg?.qiniu?.access_key || !cfg?.qiniu?.bucket;

  return (
    <div className="flex h-full">
      <Sidebar view={view} setView={setView} />
      <main className="flex-1 overflow-auto">
        <header className="px-6 py-4 border-b border-gray-200 bg-white flex items-center justify-between">
          <div>
            <h1 className="text-lg font-semibold">{titleFor(view)}</h1>
            {selWorld && view !== "settings" && (
              <p className="text-xs text-gray-500">{selWorld.alias || selWorld.GUID}</p>
            )}
          </div>
        </header>

        <div className="p-6 max-w-4xl">
          {needsConfig && view !== "settings" && (
            <div className="card p-4 mb-4 border-amber-200 bg-amber-50">
              <p className="text-sm text-amber-800">
                还没配置云服务。可到「设置」配置云同步；或直接用下方的「导出 / 导入存档」手动传输。
              </p>
            </div>
          )}

          {view === "worlds" && (
            <WorldsView
              worlds={worlds}
              sel={selWorld}
              onSelect={selectWorld}
              players={players}
              busy={busy}
              saveRoot={saveRoot}
              detectErr={detectErr}
              onUpload={() => run("上传存档", () => App.UploadWorld(selWorld!.Path))}
              onDownload={() => run("下载最新", () => App.DownloadLatest(selWorld!.Path))}
              onActivate={() => run("接手当房主", () => App.ActivateHost(selWorld!.Path))}
              onExport={async () => {
                if (!selWorld) return;
                setBusy(true);
                try {
                  const p = await App.ExportWorld(selWorld!.Path);
                  flash("ok", "已导出到：" + p);
                } catch (e: any) { flash("err", "导出失败: " + (e?.message || e)); }
                finally { setBusy(false); }
              }}
              onImport={async () => {
                if (!selWorld) return;
                try {
                  const res = await Dialogs.OpenFile({
                    Title: "导入存档",
                    AllowsMultipleSelection: true,
                    CanChooseFiles: true,
                    Filters: [{ DisplayName: "存档包", Pattern: "*.palrelay.zip;*.zip" }],
                  });
                  const inPath = Array.isArray(res) ? res[0] : res;
                  if (!inPath) return;
                  await run("导入存档", async () => {
                    await App.ImportWorld(inPath, selWorld!.Path);
                    await App.ActivateHost(selWorld!.Path);
                  });
                } catch (e: any) { flash("err", "导入失败: " + (e?.message || e)); }
              }}
            />
          )}
          {view === "cloud" && <CloudView world={selWorld} busy={busy} flash={flash} />}
          {view === "backups" && <BackupsView world={selWorld} busy={busy} flash={flash} />}
          {view === "settings" && (
            <SettingsView cfg={cfg} autoRoot={saveRoot} onSaved={(c) => { setCfg(c); flash("ok", "配置已保存"); refreshWorlds(); }} />
          )}
        </div>
      </main>

      {toast && (
        <div className={`fixed bottom-5 right-5 px-4 py-3 rounded-lg shadow-lg text-sm text-white ${toast.kind === "ok" ? "bg-emerald-600" : "bg-red-600"}`}>
          {toast.msg}
        </div>
      )}
    </div>
  );
}

function titleFor(v: View) {
  return { worlds: "存档转换", cloud: "云同步", backups: "备份管理", settings: "设置" }[v];
}

function Sidebar({ view, setView }: { view: View; setView: (v: View) => void }) {
  const items: [View, string][] = [
    ["worlds", "存档转换"],
    ["cloud", "云同步"],
    ["backups", "备份"],
    ["settings", "设置"],
  ];
  return (
    <aside className="w-52 bg-gray-900 text-gray-300 flex flex-col">
      <div className="px-5 py-5 text-white font-bold flex items-center gap-2">
        <span className="text-xl">🪄</span> Palworld 存档转换
      </div>
      <nav className="flex-1 px-2 space-y-1">
        {items.map(([v, label]) => (
          <button
            key={v}
            onClick={() => setView(v)}
            className={`w-full text-left px-3 py-2 rounded-lg text-sm transition ${view === v ? "bg-brand text-white" : "hover:bg-gray-800"}`}
          >
            {label}
          </button>
        ))}
      </nav>
    </aside>
  );
}

function WorldsView(props: {
  worlds: World[]; sel: World | null; onSelect: (w: World) => void; players: Player[];
  busy: boolean;
  saveRoot: string; detectErr: string;
  onUpload: () => void; onDownload: () => void; onActivate: () => void;
  onExport: () => void; onImport: () => void;
}) {
  const { worlds, sel, onSelect, players, busy, saveRoot, detectErr } = props;
  return (
    <div className="space-y-5">
      <div className="card p-4">
        <h2 className="font-semibold mb-3">选择世界</h2>
        {worlds.length === 0 ? (
          <div className="text-sm text-gray-500"><p>未检测到 Palworld 存档。</p><p className="mt-1">存档目录：<span className="font-mono text-gray-700 break-all">{saveRoot || "(未获取)"}</span></p>{detectErr && <p className="mt-1 text-red-500">错误：{detectErr}</p>}<p className="mt-1">路径不对？可在「设置」里手动指定存档目录。</p></div>
        ) : (
          <div className="grid gap-2">
            {worlds.map((w) => (
              <button
                key={w.GUID}
                onClick={() => onSelect(w)}
                className={`text-left px-3 py-2 rounded-lg border transition ${sel?.GUID === w.GUID ? "border-brand bg-brand/5" : "border-gray-200 hover:bg-gray-50"}`}
              >
                <div className="flex items-center justify-between">
                  <span className="font-medium text-sm">{w.alias || w.GUID}</span>
                  <span className="text-xs text-gray-400">{w.PlayerCount} 玩家</span>
                </div>
                <span className="text-xs text-gray-400">{new Date(w.ModTime).toLocaleString()}</span>
              </button>
            ))}
          </div>
        )}
      </div>

      {sel && (
        <>
          <div className="card p-4">
            <h2 className="font-semibold mb-3">玩家</h2>
            <div className="space-y-1">
              {players.length === 0 ? (
                <p className="text-sm text-gray-500">无玩家数据</p>
              ) : (
                players.map((p) => (
                  <div key={p.InstanceID} className="flex items-center justify-between text-sm py-1">
                    <span>{p.NickName || "(未命名)"} <span className="text-gray-400">Lv.{p.Level}</span></span>
                    {p.IsHost ? <span className="pill bg-indigo-100 text-indigo-700">房主</span> : null}
                  </div>
                ))
              )}
            </div>
          </div>

          <div className="card p-4">
            <h2 className="font-semibold mb-1">换房主</h2>
            <p className="text-xs text-gray-500 mb-3">当前房主点「上传存档」把存档发到云端（不影响本机，你仍是房主）。接手方先「下载最新」再「换我当房主」即可成为新房主。</p>
            <div className="flex flex-wrap gap-2">
              <button className="btn-primary" disabled={busy} onClick={props.onUpload}>⬆ 上传存档</button>
              <button className="btn-ghost" disabled={busy} onClick={props.onDownload}>⬇ 下载最新</button>
              <button className="btn-primary" disabled={busy} onClick={props.onActivate}>🎯 换我当房主</button>
            </div>
          </div>

          <div className="card p-4">
            <h2 className="font-semibold mb-1">手动传输</h2>
            <p className="text-xs text-gray-500 mb-3">没配云服务也能用：点「导出存档」会存到桌面（不影响本机，你仍是房主），把文件发给对方；对方点「导入存档」选该文件即自动成为新房主。</p>
            <div className="flex flex-wrap gap-2">
              <button className="btn-ghost" disabled={busy} onClick={props.onExport}>📤 导出存档</button>
              <button className="btn-ghost" disabled={busy} onClick={props.onImport}>📥 导入存档</button>
            </div>
          </div>
        </>
      )}
    </div>
  );
}

function CloudView({ world, busy, flash }: { world: World | null; busy: boolean; flash: (k: "ok" | "err", m: string) => void }) {
  const [versions, setVersions] = useState<any[]>([]);
  useEffect(() => {
    if (!world) return;
    App.ListVersions(world.GUID).then(setVersions).catch((e) => flash("err", String(e)));
  }, [world?.GUID]);
  if (!world) return <p className="text-sm text-gray-500">请先在「世界」里选择一个世界。</p>;
  return (
    <div className="card p-4">
      <h2 className="font-semibold mb-3">云端版本 · {world.alias || world.GUID}</h2>
      {versions.length === 0 ? (
        <p className="text-sm text-gray-500">云端暂无版本。</p>
      ) : (
        <div className="divide-y divide-gray-100">
          {versions.map((v, i) => {
            const [, , up] = (v.Key || "").split("/").pop()?.split("__") || [];
            return (
              <div key={v.Key} className="py-2 flex items-center justify-between">
                <div className="text-sm">
                  <div className="font-medium">{new Date(v.LastModified).toLocaleString()}</div>
                  <div className="text-xs text-gray-400">{v.Uploader || up || ""} · {(v.Size / 1024).toFixed(0)} KB</div>
                </div>
                {i === 0 ? <span className="pill bg-emerald-100 text-emerald-700">最新</span> : (
                  <button className="btn-ghost" disabled={busy} onClick={() =>
                    App.DownloadVersion(world.Path, v.Key).then(() => flash("ok", "已下载该版本")).catch((e) => flash("err", String(e)))
                  }>下载</button>
                )}
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}

function BackupsView({ world, busy, flash }: { world: World | null; busy: boolean; flash: (k: "ok" | "err", m: string) => void }) {
  const [backups, setBackups] = useState<BackupRecord[]>([]);
  const load = () => { if (world) App.ListBackups(world.Path).then(setBackups).catch((e) => flash("err", String(e))); };
  useEffect(load, [world?.Path]);
  if (!world) return <p className="text-sm text-gray-500">请先选择一个世界。</p>;
  return (
    <div className="card p-4">
      <h2 className="font-semibold mb-3">本地备份 · {world.alias || world.GUID}</h2>
      {backups.length === 0 ? (
        <p className="text-sm text-gray-500">暂无备份。每次切换/下载/导入会自动备份。</p>
      ) : (
        <div className="divide-y divide-gray-100">
          {backups.map((b) => (
            <div key={b.name} className="py-2 flex items-center justify-between">
              <div className="text-sm">
                <div className="font-medium">{b.name}</div>
                <div className="text-xs text-gray-400">{new Date(b.time).toLocaleString()} · {(b.size / 1024).toFixed(0)} KB</div>
              </div>
              <button className="btn-danger" disabled={busy} onClick={() =>
                App.RestoreBackup(world.Path, b.name).then(() => { flash("ok", "已回滚"); load(); }).catch((e) => flash("err", String(e)))
              }>回滚</button>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

function SettingsView({ cfg, autoRoot, onSaved }: { cfg: Config; autoRoot: string; onSaved: (c: Config) => void }) {
  const [q, setQ] = useState(cfg.qiniu || ({} as any));
  const [uploader, setUploader] = useState(cfg.uploader || "");
  const [root, setRoot] = useState(cfg.save_root || "");
  const set = (k: string, v: string) => setQ((p: any) => ({ ...p, [k]: v }));
  const save = async () => {
    const next = { ...cfg, qiniu: q, uploader, save_root: root } as Config;
    await App.SaveConfig(next);
    onSaved(next);
  };
  return (
    <div className="space-y-4">
      <div className="card p-4 space-y-3">
        <h2 className="font-semibold">七牛云 Kodo</h2>
        <div className="grid grid-cols-2 gap-3">
          <div><label className="label">AccessKey</label><input className="input" value={q.access_key || ""} onChange={(e) => set("access_key", e.target.value)} /></div>
          <div><label className="label">SecretKey</label><input className="input" type="password" value={q.secret_key || ""} onChange={(e) => set("secret_key", e.target.value)} /></div>
          <div><label className="label">Bucket</label><input className="input" value={q.bucket || ""} onChange={(e) => set("bucket", e.target.value)} /></div>
          <div><label className="label">区域 (z0/z1/z2/na0)</label><input className="input" value={q.region || ""} onChange={(e) => set("region", e.target.value)} /></div>
          <div className="col-span-2"><label className="label">下载域名（留空自动获取）</label><input className="input" value={q.domain || ""} onChange={(e) => set("domain", e.target.value)} /></div>
        </div>
      </div>
      <div className="card p-4 space-y-3">
        <h2 className="font-semibold">通用</h2>
        <div><label className="label">上传者名（标识版本）</label><input className="input" value={uploader} onChange={(e) => setUploader(e.target.value)} /></div>
        <div><label className="label">存档目录（留空自动检测）</label><input className="input" value={root} onChange={(e) => setRoot(e.target.value)} />{!root && autoRoot && <p className="text-xs text-gray-400 mt-1 font-mono break-all">自动检测：{autoRoot}</p>}</div>
      </div>
      <button className="btn-primary" onClick={save}>保存配置</button>
    </div>
  );
}