import { useEffect, useState, useCallback } from "react";
import { Dialogs, Window } from "@wailsio/runtime";
import { App } from "../bindings/palworld-save-relay";
import type { World, BackupRecord } from "../bindings/palworld-save-relay/models";
import type { Player } from "../bindings/palworld-save-relay/internal/palworld/models";
import type { Config } from "../bindings/palworld-save-relay/internal/config/models";
import { useI18n, type Lang } from "./i18n";

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
  const { t, lang } = useI18n();
  const [view, setView] = useState<View>("worlds");
  const [cfg, setCfg] = useState<Config>(defaultConfig());
  const [worlds, setWorlds] = useState<World[]>([]);
  const [selWorld, setSelWorld] = useState<World | null>(null);
  const [players, setPlayers] = useState<Player[]>([]);
  const [busy, setBusy] = useState(false);
  const [toast, setToast] = useState<{ kind: "ok" | "err"; msg: string } | null>(null);
  const [saveRoot, setSaveRoot] = useState("");
  const [detectErr, setDetectErr] = useState("");
  const [maximised, setMaximised] = useState(false);

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
      flash("err", t("err.detectWorlds", String(e?.message || e)));
    }
    try { setSaveRoot(await App.ResolvedSaveRoot()); } catch {}
  }, [selWorld, t]);

  useEffect(() => {
    (async () => {
      try {
        const c = await App.GetConfig();
        if (c) setCfg(c);
      } catch (e: any) {
        flash("err", t("err.loadConfig", String(e?.message || e)));
      }
      await refreshWorlds();
    })();
  }, [refreshWorlds]);

  // Keep the native window title in sync with the selected language.
  useEffect(() => {
    Window.SetTitle(t("app.title")).catch(() => {});
    document.documentElement.lang = lang;
  }, [lang, t]);

  // Keep the maximise/restore icon in sync (covers Win+Up, snap, etc.)
  useEffect(() => {
    let alive = true;
    const sync = () => { Window.IsMaximised().then((m) => { if (alive) setMaximised(!!m); }).catch(() => {}); };
    sync();
    window.addEventListener("resize", sync);
    return () => { alive = false; window.removeEventListener("resize", sync); };
  }, []);

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
      flash("ok", t("common.done", label));
      if (selWorld) await selectWorld(selWorld);
    } catch (e: any) {
      flash("err", t("common.failed", label, String(e?.message || e)));
    } finally {
      setBusy(false);
    }
  };

  const needsConfig = !cfg?.qiniu?.access_key || !cfg?.qiniu?.bucket;

  const titleFor = (v: View) =>
    ({ worlds: t("title.worlds"), cloud: t("title.cloud"), backups: t("title.backups"), settings: t("title.settings") }[v]);

  return (
    <div className="flex h-full">
      <Sidebar view={view} setView={setView} />
      <main className="flex-1 flex flex-col min-w-0 overflow-hidden">
        <header
          className="drag px-6 py-4 border-b border-gray-200 bg-white flex items-center justify-between flex-shrink-0"
          onDoubleClick={() => Window.ToggleMaximise()}
        >
          <div>
            <h1 className="text-lg font-semibold">{titleFor(view)}</h1>
            {selWorld && view !== "settings" && (
              <p className="text-xs text-gray-500">{selWorld.alias || selWorld.GUID}</p>
            )}
          </div>
          <WindowControls maximised={maximised} />
        </header>

        <div className="flex-1 overflow-auto">
          <div className="p-6 max-w-4xl">
          {needsConfig && view !== "settings" && (
            <div className="card p-4 mb-4 border-amber-200 bg-amber-50">
              <p className="text-sm text-amber-800">
                {t("warn.noCloud")}
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
              onUpload={() => run(t("label.upload"), () => App.UploadWorld(selWorld!.Path))}
              onDownload={() => run(t("label.downloadLatest"), () => App.DownloadLatest(selWorld!.Path))}
              onActivate={() => run(t("label.activate"), () => App.ActivateHost(selWorld!.Path))}
              onExport={async () => {
                if (!selWorld) return;
                try {
                  const out = await Dialogs.SaveFile({
                    Title: t("dialog.exportTitle"),
                    Filename: `${selWorld.GUID}.palrelay.zip`,
                    Filters: [{ DisplayName: t("dialog.savePkg"), Pattern: "*.palrelay.zip" }],
                  });
                  if (!out) return;
                  await run(t("dialog.exportTitle"), () => App.ExportWorld(selWorld!.Path, out));
                } catch (e: any) { flash("err", t("err.export", String(e?.message || e))); }
              }}
              onImport={async () => {
                if (!selWorld) return;
                try {
                  const res = await Dialogs.OpenFile({
                    Title: t("dialog.importTitle"),
                    Filters: [{ DisplayName: t("dialog.savePkg"), Pattern: "*.palrelay.zip;*.zip" }],
                  });
                  const inPath = Array.isArray(res) ? (res[0] || "") : (res || "");
                  if (!inPath) return;
                  await run(t("dialog.importTitle"), async () => {
                    await App.ImportWorld(inPath, selWorld!.Path);
                    await App.ActivateHost(selWorld!.Path);
                  });
                } catch (e: any) { flash("err", t("err.import", String(e?.message || e))); }
              }}
              onAlias={(guid, alias) => { App.SetWorldMeta(guid, alias, selWorld?.hidden ?? false).then(() => refreshWorlds()).catch((e: any) => flash("err", t("err.rename", String(e?.message || e)))); }}
            />
          )}
          {view === "cloud" && <CloudView world={selWorld} busy={busy} flash={flash} />}
          {view === "backups" && <BackupsView world={selWorld} busy={busy} flash={flash} />}
          {view === "settings" && (
            <SettingsView cfg={cfg} autoRoot={saveRoot} onSaved={(c) => { setCfg(c); flash("ok", t("toast.configSaved")); refreshWorlds(); }} />
          )}
          </div>
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

function WindowControls({ maximised }: { maximised: boolean }) {
  const { t } = useI18n();
  return (
    <div className="no-drag flex items-center -mr-2">
      <button className="win-btn" title={t("win.min")} onClick={() => Window.Minimise()}>
        <svg width="12" height="12" viewBox="0 0 12 12" aria-hidden="true">
          <rect y="5.5" width="12" height="1" fill="currentColor" />
        </svg>
      </button>
      <button className="win-btn" title={maximised ? t("win.restore") : t("win.max")} onClick={() => Window.ToggleMaximise()}>
        {maximised ? (
          <svg width="12" height="12" viewBox="0 0 12 12" fill="none" stroke="currentColor" strokeWidth="1" aria-hidden="true">
            <rect x="2.5" y="3.5" width="6" height="6" />
            <path d="M4.5 3.5 V1.5 H10.5 V7.5 H8.5" />
          </svg>
        ) : (
          <svg width="12" height="12" viewBox="0 0 12 12" fill="none" stroke="currentColor" strokeWidth="1" aria-hidden="true">
            <rect x="2.5" y="2.5" width="7" height="7" />
          </svg>
        )}
      </button>
      <button className="win-btn close" title={t("win.close")} onClick={() => Window.Close()}>
        <svg width="12" height="12" viewBox="0 0 12 12" stroke="currentColor" strokeWidth="1.2" aria-hidden="true">
          <path d="M2 2 L10 10 M10 2 L2 10" />
        </svg>
      </button>
    </div>
  );
}

function Sidebar({ view, setView }: { view: View; setView: (v: View) => void }) {
  const { lang, setLang, t } = useI18n();
  const items: [View, string][] = [
    ["worlds", t("nav.worlds")],
    ["cloud", t("nav.cloud")],
    ["backups", t("nav.backups")],
    ["settings", t("nav.settings")],
  ];
  return (
    <aside className="w-52 bg-gray-900 text-gray-300 flex flex-col">
      <div
        className="drag px-5 py-5 text-white font-bold flex items-center gap-2"
        onDoubleClick={() => Window.ToggleMaximise()}
      >
        <span className="text-xl">🪄</span> {t("app.title")}
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
      <div className="px-2 py-3 border-t border-gray-800">
        <LangSwitch lang={lang} setLang={setLang} />
      </div>
    </aside>
  );
}

function LangSwitch({ lang, setLang }: { lang: Lang; setLang: (l: Lang) => void }) {
  const { t } = useI18n();
  return (
    <button
      onClick={() => setLang(lang === "zh" ? "en" : "zh")}
      className="w-full text-left px-3 py-2 rounded-lg text-sm text-gray-400 hover:bg-gray-800 hover:text-gray-200 transition"
      title={t("lang.switch")}
    >
      🌐 {t("lang.switch")}
    </button>
  );
}

function WorldsView(props: {
  worlds: World[]; sel: World | null; onSelect: (w: World) => void; players: Player[];
  busy: boolean;
  saveRoot: string; detectErr: string;
  onUpload: () => void; onDownload: () => void; onActivate: () => void;
  onExport: () => void; onImport: () => void;
  onAlias: (guid: string, alias: string) => void;
}) {
  const { t } = useI18n();
  const { worlds, sel, onSelect, players, busy, saveRoot, detectErr, onAlias } = props;
  const [aliasInput, setAliasInput] = useState(sel?.alias || "");
  useEffect(() => { setAliasInput(sel?.alias || ""); }, [sel?.GUID, sel?.alias]);
  return (
    <div className="space-y-5">
      <div className="card p-4">
        <h2 className="font-semibold mb-3">{t("worlds.selectWorld")}</h2>
        {worlds.length === 0 ? (
          <div className="text-sm text-gray-500"><p>{t("worlds.notFound")}</p><p className="mt-1">{t("worlds.saveDir")}<span className="font-mono text-gray-700 break-all">{saveRoot || t("worlds.saveDirNA")}</span></p>{detectErr && <p className="mt-1 text-red-500">{t("worlds.detectErr", detectErr)}</p>}<p className="mt-1">{t("worlds.fixPath")}</p></div>
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
                  <span className="text-xs text-gray-400">{t("worlds.playerCount", w.PlayerCount)}</span>
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
            <div className="flex items-center justify-between mb-2">
              <h2 className="font-semibold">{t("worlds.worldName")}</h2>
              <span className="text-xs text-gray-400 font-mono break-all">{sel.GUID}</span>
            </div>
            <input className="input" value={aliasInput} placeholder={t("worlds.aliasPlaceholder")} onChange={(e) => setAliasInput(e.target.value)} onBlur={() => onAlias(sel.GUID, aliasInput)} />
          </div>

          <div className="card p-4">
            <h2 className="font-semibold mb-3">{t("worlds.playersTitle")}</h2>
            <div className="space-y-1">
              {players.length === 0 ? (
                <p className="text-sm text-gray-500">{t("worlds.noPlayers")}</p>
              ) : (
                players.map((p) => (
                  <div key={p.InstanceID} className="flex items-center justify-between text-sm py-1">
                    <span>{p.NickName || t("worlds.unnamed")} <span className="text-gray-400">Lv.{p.Level}</span></span>
                    {p.IsHost ? <span className="pill bg-indigo-100 text-indigo-700">{t("worlds.host")}</span> : null}
                  </div>
                ))
              )}
            </div>
          </div>

          <div className="card p-4">
            <h2 className="font-semibold mb-1">{t("worlds.swapHost")}</h2>
            <p className="text-xs text-gray-500 mb-3">{t("worlds.swapHostDesc")}</p>
            <div className="flex flex-wrap gap-2">
              <button className="btn-primary" disabled={busy} onClick={props.onUpload}>{t("worlds.btnUpload")}</button>
              <button className="btn-ghost" disabled={busy} onClick={props.onDownload}>{t("worlds.btnDownload")}</button>
              <button className="btn-primary" disabled={busy} onClick={props.onActivate}>{t("worlds.btnActivate")}</button>
            </div>
          </div>

          <div className="card p-4">
            <h2 className="font-semibold mb-1">{t("worlds.manualTransfer")}</h2>
            <p className="text-xs text-gray-500 mb-3">{t("worlds.manualDesc")}</p>
            <div className="flex flex-wrap gap-2">
              <button className="btn-ghost" disabled={busy} onClick={props.onExport}>{t("worlds.btnExport")}</button>
              <button className="btn-ghost" disabled={busy} onClick={props.onImport}>{t("worlds.btnImport")}</button>
            </div>
          </div>
        </>
      )}
    </div>
  );
}

function CloudView({ world, busy, flash }: { world: World | null; busy: boolean; flash: (k: "ok" | "err", m: string) => void }) {
  const { t } = useI18n();
  const [versions, setVersions] = useState<any[]>([]);
  useEffect(() => {
    if (!world) return;
    App.ListVersions(world.GUID).then(setVersions).catch((e) => flash("err", String(e)));
  }, [world?.GUID]);
  if (!world) return <p className="text-sm text-gray-500">{t("cloud.selectFirst")}</p>;
  return (
    <div className="card p-4">
      <h2 className="font-semibold mb-3">{t("cloud.versions", world.alias || world.GUID)}</h2>
      {versions.length === 0 ? (
        <p className="text-sm text-gray-500">{t("cloud.empty")}</p>
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
                {i === 0 ? <span className="pill bg-emerald-100 text-emerald-700">{t("cloud.latest")}</span> : (
                  <button className="btn-ghost" disabled={busy} onClick={() =>
                    App.DownloadVersion(world.Path, v.Key).then(() => flash("ok", t("toast.versionDownloaded"))).catch((e) => flash("err", String(e)))
                  }>{t("cloud.download")}</button>
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
  const { t } = useI18n();
  const [backups, setBackups] = useState<BackupRecord[]>([]);
  const load = () => { if (world) App.ListBackups(world.Path).then(setBackups).catch((e) => flash("err", String(e))); };
  useEffect(load, [world?.Path]);
  if (!world) return <p className="text-sm text-gray-500">{t("backups.selectFirst")}</p>;
  return (
    <div className="card p-4">
      <h2 className="font-semibold mb-3">{t("backups.title", world.alias || world.GUID)}</h2>
      {backups.length === 0 ? (
        <p className="text-sm text-gray-500">{t("backups.empty")}</p>
      ) : (
        <div className="divide-y divide-gray-100">
          {backups.map((b) => (
            <div key={b.name} className="py-2 flex items-center justify-between">
              <div className="text-sm">
                <div className="font-medium">{b.name}</div>
                <div className="text-xs text-gray-400">{new Date(b.time).toLocaleString()} · {(b.size / 1024).toFixed(0)} KB</div>
              </div>
              <button className="btn-danger" disabled={busy} onClick={() =>
                App.RestoreBackup(world.Path, b.name).then(() => { flash("ok", t("toast.rolledBack")); load(); }).catch((e) => flash("err", String(e)))
              }>{t("backups.restore")}</button>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

function SettingsView({ cfg, autoRoot, onSaved }: { cfg: Config; autoRoot: string; onSaved: (c: Config) => void }) {
  const { t } = useI18n();
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
        <h2 className="font-semibold">{t("settings.qiniu")}</h2>
        <div className="grid grid-cols-2 gap-3">
          <div><label className="label">{t("settings.accessKey")}</label><input className="input" value={q.access_key || ""} onChange={(e) => set("access_key", e.target.value)} /></div>
          <div><label className="label">{t("settings.secretKey")}</label><input className="input" type="password" value={q.secret_key || ""} onChange={(e) => set("secret_key", e.target.value)} /></div>
          <div><label className="label">{t("settings.bucket")}</label><input className="input" value={q.bucket || ""} onChange={(e) => set("bucket", e.target.value)} /></div>
          <div className="col-span-2"><label className="label">{t("settings.domain")}</label><input className="input" value={q.domain || ""} onChange={(e) => set("domain", e.target.value)} /></div>
        </div>
      </div>
      <div className="card p-4 space-y-3">
        <h2 className="font-semibold">{t("settings.general")}</h2>
        <div><label className="label">{t("settings.uploader")}</label><input className="input" value={uploader} onChange={(e) => setUploader(e.target.value)} /></div>
        <div><label className="label">{t("settings.saveRoot")}</label><input className="input" value={root} onChange={(e) => setRoot(e.target.value)} />{!root && autoRoot && <p className="text-xs text-gray-400 mt-1 font-mono break-all">{t("settings.autoDetect", autoRoot)}</p>}</div>
      </div>
      <button className="btn-primary" onClick={save}>{t("settings.save")}</button>
    </div>
  );
}