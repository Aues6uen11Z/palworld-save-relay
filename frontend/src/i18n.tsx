import { createContext, useCallback, useContext, useState, type ReactNode } from "react";

export type Lang = "zh" | "en";

type Entry = { zh: string; en: string };
type Dict = Record<string, Entry>;

const dict: Dict = {
  "app.title": { zh: "幻兽帕鲁换房主", en: "Palworld Host Swap" },

  "nav.worlds": { zh: "换房主", en: "Host Swap" },
  "nav.cloud": { zh: "云同步", en: "Cloud Sync" },
  "nav.backups": { zh: "备份", en: "Backups" },
  "nav.settings": { zh: "设置", en: "Settings" },

  "title.worlds": { zh: "换房主", en: "Host Swap" },
  "title.cloud": { zh: "云同步", en: "Cloud Sync" },
  "title.backups": { zh: "备份管理", en: "Backup Manager" },
  "title.settings": { zh: "设置", en: "Settings" },

  "win.min": { zh: "最小化", en: "Minimize" },
  "win.max": { zh: "最大化", en: "Maximize" },
  "win.restore": { zh: "还原", en: "Restore" },
  "win.close": { zh: "关闭", en: "Close" },

  "common.done": { zh: "{0} 完成", en: "{0} done" },
  "common.failed": { zh: "{0} 失败: {1}", en: "{0} failed: {1}" },

  "err.detectWorlds": { zh: "检测世界失败: {0}", en: "Failed to detect worlds: {0}" },
  "err.loadConfig": { zh: "加载配置失败: {0}", en: "Failed to load config: {0}" },
  "err.export": { zh: "导出失败: {0}", en: "Export failed: {0}" },
  "err.import": { zh: "导入失败: {0}", en: "Import failed: {0}" },
  "err.rename": { zh: "改名失败: {0}", en: "Rename failed: {0}" },

  "toast.configSaved": { zh: "配置已保存", en: "Config saved" },
  "toast.versionDownloaded": { zh: "已下载该版本", en: "Version downloaded" },
  "toast.rolledBack": { zh: "已回滚", en: "Rolled back" },
  "toast.uploaded": { zh: "存档已上传", en: "Save uploaded" },
  "toast.activated": { zh: "已切换为房主", en: "Switched to host" },

  "label.upload": { zh: "上传存档", en: "Upload Save" },
  "label.downloadActivate": { zh: "下载并成为房主", en: "Download & Become Host" },

  "dialog.exportTitle": { zh: "导出存档", en: "Export Save" },
  "dialog.importTitle": { zh: "导入存档", en: "Import Save" },
  "dialog.savePkg": { zh: "存档包", en: "Save Package" },
  "dialog.uploadTitle": { zh: "上传存档", en: "Upload Save" },
  "dialog.uploadConfirm": {
    zh: "上传后，本机存档将从「房主」转为「房客」——仅保留个人进度数据，不能继续游玩。\n\n云端将保存最新存档供其他玩家下载。你可以随时在「备份」页回滚恢复。\n\n确定要上传吗？",
    en: "After uploading, your local save will switch from Host to Guest — only personal progress is kept, and you cannot continue playing.\n\nThe latest save will be stored in the cloud for others to download. You can restore from the Backups page at any time.\n\nAre you sure you want to upload?",
  },
  "dialog.confirmUpload": { zh: "确认上传", en: "Confirm Upload" },
  "dialog.downloadTitle": { zh: "下载并成为房主", en: "Download & Become Host" },
  "dialog.downloadConfirm": {
    zh: "将下载云端存档并切换为房主。\n\n当前本地存档会先自动备份（可在「备份」页回滚）。下载完成后，你可以启动游戏。\n\n确定要继续吗？",
    en: "This will download the cloud save and switch you to host.\n\nYour current local save will be backed up first (restore from Backups page). After download, you can launch the game.\n\nAre you sure you want to continue?",
  },
  "dialog.downloadHostWarning": {
    zh: "⚠️ 你当前是房主，下载将覆盖你的本地存档！\n\n",
    en: "⚠️ You are currently the host. Downloading will overwrite your local save!\n\n",
  },
  "dialog.confirmDownload": { zh: "确认下载", en: "Confirm Download" },
  "dialog.cancel": { zh: "取消", en: "Cancel" },
  "dialog.activatedTitle": { zh: "已切换为房主", en: "You Are Now the Host" },
  "dialog.activatedMsg": {
    zh: "已成功下载存档并切换为房主！\n\n你现在可以启动 Palworld 游戏了。",
    en: "Save downloaded and you are now the host!\n\nYou can now launch Palworld.",
  },
  "dialog.gotIt": { zh: "知道了", en: "Got It" },

  "warn.noCloud": {
    zh: "还没配置云服务。可到「设置」配置云同步；或直接用下方的「导出 / 导入存档」手动传输。",
    en: "Cloud service not configured yet. Go to Settings to set up cloud sync, or use Export / Import below to transfer manually.",
  },

  "worlds.selectWorld": { zh: "选择世界", en: "Select World" },
  "worlds.notFound": { zh: "未检测到 Palworld 存档。", en: "No Palworld save detected." },
  "worlds.saveDir": { zh: "存档目录：", en: "Save directory: " },
  "worlds.saveDirNA": { zh: "(未获取)", en: "(not available)" },
  "worlds.detectErr": { zh: "错误：{0}", en: "Error: {0}" },
  "worlds.fixPath": { zh: "路径不对？可在「设置」里手动指定存档目录。", en: "Wrong path? You can set the save directory manually in Settings." },
  "worlds.playerCount": { zh: "{0} 玩家", en: "{0} players" },
  "worlds.worldName": { zh: "世界名", en: "World Name" },
  "worlds.aliasPlaceholder": { zh: "存档备注", en: "Save note" },
  "worlds.openFolder": { zh: "📂 打开存档位置", en: "📂 Open Save Folder" },
  "worlds.playersTitle": { zh: "玩家", en: "Players" },
  "worlds.noPlayers": { zh: "无玩家数据", en: "No player data" },
  "worlds.unnamed": { zh: "(未命名)", en: "(unnamed)" },
  "worlds.host": { zh: "房主", en: "Host" },
  "worlds.guest": { zh: "客机", en: "Guest" },
  "worlds.guestHint": { zh: "你当前不是此世界的房主。点击下方「下载并成为房主」即可接手。", en: "You are not the host of this world. Click Download & Become Host below to take over." },
  "worlds.guestOnly": { zh: "非房主不可用：先点「下载并成为房主」接手", en: "Guest-only: click Download & Become Host first" },
  "worlds.swapHost": { zh: "换房主", en: "Swap Host" },
  "worlds.swapHostDesc": {
    zh: "当前房主点「上传存档」把存档发到云端，本机存档会转为房客模式（只剩个人数据，不能继续游玩；可在「备份」页回滚恢复）。接手方点「下载并成为房主」即可成为新房主。",
    en: "The current host clicks Upload Save to push the save to the cloud; the local save is then reduced to guest-only (personal data only - restore from Backups to play again). The person taking over clicks Download & Become Host.",
  },
  "worlds.btnUpload": { zh: "⬆ 上传存档", en: "⬆ Upload Save" },
  "worlds.btnDownloadActivate": { zh: "🎯 下载并成为房主", en: "🎯 Download & Become Host" },
  "worlds.manualTransfer": { zh: "手动传输", en: "Manual Transfer" },
  "worlds.manualDesc": {
    zh: "没配云服务也能用：导出方点「导出存档」选位置存成单文件，把文件发给对方；对方点「导入存档」选该文件即自动成为新房主。",
    en: "No cloud needed: the exporter clicks Export Save to create a single file and sends it; the recipient clicks Import Save and picks that file, automatically becoming the new host.",
  },
  "worlds.btnExport": { zh: "📤 导出存档", en: "📤 Export Save" },
  "worlds.btnImport": { zh: "📥 导入存档", en: "📥 Import Save" },

  "cloud.selectFirst": { zh: "请先在「世界」里选择一个世界。", en: "Please select a world under Host Swap first." },
  "cloud.versions": { zh: "云端版本 · {0}", en: "Cloud Versions · {0}" },
  "cloud.empty": { zh: "云端暂无版本。", en: "No cloud versions yet." },
  "cloud.latest": { zh: "最新", en: "Latest" },
  "cloud.download": { zh: "下载", en: "Download" },

  "backups.selectFirst": { zh: "请先选择一个世界。", en: "Please select a world first." },
  "backups.title": { zh: "本地备份 · {0}", en: "Local Backups · {0}" },
  "backups.empty": { zh: "暂无备份。每次切换/下载/导入会自动备份。", en: "No backups yet. A backup is made automatically before every host swap / download / import." },
  "backups.restore": { zh: "回滚", en: "Restore" },
  "backups.hostLabel": { zh: "房主存档", en: "Host Save" },
  "backups.guestLabel": { zh: "房客存档", en: "Guest Save" },

  "settings.qiniu": { zh: "七牛云 Kodo", en: "Qiniu Kodo" },
  "settings.accessKey": { zh: "AccessKey", en: "AccessKey" },
  "settings.secretKey": { zh: "SecretKey", en: "SecretKey" },
  "settings.bucket": { zh: "Bucket", en: "Bucket" },
  "settings.domain": { zh: "下载域名（留空自动获取）", en: "Download domain (leave blank for auto)" },
  "settings.general": { zh: "通用", en: "General" },
  "settings.uploader": { zh: "上传者名（标识版本）", en: "Uploader name (identifies versions)" },
  "settings.saveRoot": { zh: "存档目录（留空自动检测）", en: "Save directory (leave blank for auto-detect)" },
  "settings.autoDetect": { zh: "自动检测：{0}", en: "Auto-detected: {0}" },
  "settings.save": { zh: "保存配置", en: "Save Config" },

  "lang.switch": { zh: "English", en: "中文" },
};

interface I18nCtx {
  lang: Lang;
  setLang: (l: Lang) => void;
  t: (key: string, ...args: (string | number)[]) => string;
}

const Ctx = createContext<I18nCtx | null>(null);

const STORAGE_KEY = "palrelay.lang";

function detectInitialLang(): Lang {
  try {
    const saved = localStorage.getItem(STORAGE_KEY);
    if (saved === "en" || saved === "zh") return saved;
  } catch {}
  // Fall back to the browser/UI language.
  const nav = (typeof navigator !== "undefined" && navigator.language) || "";
  return nav.toLowerCase().startsWith("zh") ? "zh" : "en";
}

export function LangProvider({ children }: { children: ReactNode }) {
  const [lang, setLangState] = useState<Lang>(detectInitialLang);

  const setLang = useCallback((l: Lang) => {
    setLangState(l);
    try { localStorage.setItem(STORAGE_KEY, l); } catch {}
  }, []);

  const t = useCallback((key: string, ...args: (string | number)[]) => {
    const entry = dict[key];
    let s = entry ? (lang === "en" ? entry.en : entry.zh) : key;
    if (args.length) {
      args.forEach((a, i) => { s = s.split(`{${i}}`).join(String(a)); });
    }
    return s;
  }, [lang]);

  return <Ctx.Provider value={{ lang, setLang, t }}>{children}</Ctx.Provider>;
}

export function useI18n(): I18nCtx {
  const ctx = useContext(Ctx);
  if (!ctx) throw new Error("useI18n must be used within LangProvider");
  return ctx;
}




