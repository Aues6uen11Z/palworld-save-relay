import { useEffect, useState, useCallback } from "react";
import { App } from "../bindings/palworld-save-relay";
import type { World, BackupRecord } from "../bindings/palworld-save-relay/models";
import type { Player } from "../bindings/palworld-save-relay/internal/palworld/models";
import type { Config } from "../bindings/palworld-save-relay/internal/config/models";
import type { LockStatus } from "../bindings/palworld-save-relay/internal/storage/models";

type View = "worlds" | "cloud" | "backups" | "settings";

export default function AppView() {
  const [view, setView] = useState<View>("worlds");
  const [cfg, setCfg] = useState<Config | null>(null);
  const [worlds, setWorlds] = useState<World[]>([]);
  const [selWorld, setSelWorld] = useState<World | null>(null);
  const [players, setPlayers] = useState<Player[]>([]);
  const [lock, setLock] = useState<LockStatus | null>(null);
  const [busy, setBusy] = useState(false);
  const [toast, setToast] = useState<{ kind: "ok" | "err"; msg: string } | null>(null);

  const flash = (kind: "ok" | "err", msg: string) => {
    setToast({ kind, msg });
    setTimeout(() => setToast(null), 4000);
  };

  const refreshWorlds = useCallback(async () => {
    try {
      const ws = await App.DetectWorlds();
      setWorlds(ws || []);
      if (ws && ws.length && !selWorld) setSelWorld(ws[0]);
    } catch (e: any) {
      flash("err", "检测世界失败: " + (e?.message || e));
    }
  }, [selWorld]);

  useEffect(() => {
    (async () => {
      const c = await App.GetConfig();
      setCfg(c);
      await refreshWorlds();
    })();
  }, [refreshWorlds]);

  const selectWorld = async (w: World) => {
    setSelWorld(w);
    try {
      setPlayers((await App.ListPlayers(w.Path)) || []);
      setLock(await App.LockStatus(w.GUID));
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
          {selWorld && lock && (view === "worlds" || view === "cloud") && (
            <LockBadge lock={lock} />
          )}
        </header>

        <div className="p-6 max-w-4xl">
          {needsConfig && view !== "settings" && (
            <div className="card p-4 mb-4 border-amber-200 bg-amber-50">
              <p className="text-sm text-amber-800">
                还没配置七牛云，请先到「设置」填写 AccessKey / SecretKey / Bucket / 区域。
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
              onUpload={() => run("上传交出", async () => {
                await App.PrepareUpload(selWorld!.Path);
                await App.UploadWorld(selWorld!.Path);
              })}
              onDownload={() => run("下载最新", () => App.DownloadLatest(selWorld!.Path))}
              onActivate={() => run("接手当房主", () => App.ActivateHost(selWorld!.Path))}
              onAcquire={() => run("占锁", () => App.AcquireLock(selWorld!.GUID, cfg?.uploader || "player"))}
              onRelease={() => run("释放锁", () => App.ReleaseLock(selWorld!.GUID))}
            />
          )}
          {view === "cloud" && <CloudView world={selWorld} busy={busy} flash={flash} />}
          {view === "backups" && <BackupsView world={selWorld} busy={busy} flash={flash} />}
          {view === "settings" && cfg && (
            <SettingsView cfg={cfg} onSaved={(c) => { setCfg(c); flash("ok", "配置已保存"); }} />
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
  return { worlds: "世界与接力", cloud: "云同步", backups: "备份管理", settings: "设置" }[v];
}

function Sidebar({ view, setView }: { view: View; setView: (v: View) => void }) {
  const items: [View, string][] = [
    ["worlds", "世界与接力"],
    ["cloud", "云同步"],
    ["backups", "备份"],
    ["settings", "设置"],
  ];
  return (
    <aside className="w-52 bg-gray-900 text-gray-300 flex flex-col">
      <div className="px-5 py-5 text-white font-bold flex items-center gap-2">
        <span className="text-xl">🪄</span> Pal Save Relay
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
      <p className="px-5 py-4 text-xs text-gray-500">存档接力 · 开源</p>
    </aside>
  );
}

function LockBadge({ lock }: { lock: LockStatus }) {
  if (!lock.Held)
    return <span className="pill bg-emerald-100 text-emerald-700">空闲</span>;
  return (
    <span className={`pill ${lock.Stale ? "bg-amber-100 text-amber-700" : "bg-rose-100 text-rose-700"}`}>
      {lock.Stale ? "锁过期" : "有人游玩"} · {lock.Lock?.player}
    </span>
  );
}

function WorldsView(props: {
  worlds: World[]; sel: World | null; onSelect: (w: World) => void; players: Player[];
  busy: boolean;
  onUpload: () => void; onDownload: () => void; onActivate: () => void;
  onAcquire: () => void; onRelease: () => void;
}) {
  const { worlds, sel, onSelect, players, busy } = props;
  return (
    <div className="space-y-5">
      <div className="card p-4">
        <h2 className="font-semibold mb-3">选择世界</h2>
        {worlds.length === 0 ? (
          <p className="text-sm text-gray-500">未检测到 Palworld 存档。可在「设置」里手动指定存档目录。</p>
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
            <h2 className="font-semibold mb-1">接力操作</h2>
            <p className="text-xs text-gray-500 mb-3">房主交出：先「上传交出」再释放锁。接手者：先「下载最新」再「接手当房主」并占锁。</p>
            <div className="flex flex-wrap gap-2">
              <button className="btn-primary" disabled={busy} onClick={props.onUpload}>⬆ 上传交出</button>
              <button className="btn-ghost" disabled={busy} onClick={props.onDownload}>⬇ 下载最新</button>
              <button className="btn-primary" disabled={busy} onClick={props.onActivate}>🎯 接手当房主</button>
              <button className="btn-ghost" disabled={busy} onClick={props.onAcquire}>🔒 占锁</button>
              <button className="btn-ghost" disabled={busy} onClick={props.onRelease}>🔓 释放锁</button>
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

function SettingsView({ cfg, onSaved }: { cfg: Config; onSaved: (c: Config) => void }) {
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
        <div><label className="label">存档目录（留空自动检测）</label><input className="input" value={root} onChange={(e) => setRoot(e.target.value)} /></div>
      </div>
      <button className="btn-primary" onClick={save}>保存配置</button>
    </div>
  );
}


