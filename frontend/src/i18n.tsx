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
  "nav.help": { zh: "原理", en: "How It Works" },

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

  // User-friendly error messages keyed by backend error codes.
  // parseErr() extracts [CODE] from the error string and looks up "errcode.CODE".
  "errcode.GAME_RUNNING": { zh: "游戏正在运行中，请先关闭 Palworld", en: "Palworld is running. Please close the game first." },
  "errcode.QINIU_CONFIG": { zh: "云服务未配置，请前往「设置」页填写七牛云配置", en: "Cloud service not configured. Set up Qiniu in Settings." },
  "errcode.NO_CLOUD_VERSIONS": { zh: "云端没有可用的存档版本", en: "No cloud save versions available." },
  "errcode.VALIDATION_FAILED": { zh: "存档文件校验失败，可能已损坏。请尝试重新下载或导入", en: "Save file validation failed, it may be corrupted. Try re-downloading or re-importing." },
  "errcode.RAW_HOST_SAVE": { zh: "这是完整的主机存档或备份，不是可中转的存档。它仍带有房主哨兵标识，导入后激活会覆盖原房主数据。请让对方用「导出存档」或「上传」生成 _relay.zip，再导入那个文件", en: "This is a raw host save/backup, not a relay intermediate. It still carries the host sentinel; activating after import would overwrite the former host's data. Have the host use Export/Upload to produce a _relay.zip and import that instead." },
  "errcode.WORLD_MISMATCH": { zh: "这个存档属于另一个世界，和当前世界不一致。请确认双方操作的是同一个世界的存档", en: "This save belongs to a different world than the selected one. Make sure both sides are operating on the same world." },
  "errcode.WORLD_UNKNOWN": { zh: "无法确认存档所属世界（缺少中转记录）。请让对方重新用「导出」或「上传」生成存档", en: "Cannot verify which world this save belongs to (no relay record). Ask the other side to re-export or re-upload the save." },
  "errcode.STEAMID_PARSE": { zh: "无法识别 Steam 账号信息，请检查存档目录是否正确", en: "Cannot identify Steam account. Please verify the save directory." },
  "errcode.BACKUP_FAILED": { zh: "创建备份失败，操作已中止。可能是磁盘空间不足或文件被占用", en: "Backup creation failed, operation aborted. Disk may be full or files locked." },
  "errcode.PACK_FAILED": { zh: "打包存档失败，存档文件可能损坏或缺失", en: "Failed to pack save. Save files may be corrupted or missing." },
  "errcode.DUPLICATE_PLAYER_DATA": { zh: "检测到重复玩家数据：主机存档(0001)和你的客机存档同时存在。通常是因为拿到存档后没激活就直接玩了，或直接用了别人的存档。请从备份恢复后正确激活，再上传/导出", en: "Duplicate player data: both the host save (0001) and your guest save exist. Usually caused by playing without activating after downloading, or directly using a save from another player. Restore from a backup, activate as host, then upload/export." },
  "errcode.NOT_HOST": { zh: "还不是房主（缺少主机玩家存档）。请先「激活为主机」", en: "Not the host yet (host player save missing). Please activate as host first." },
  "errcode.NOT_HOST_WORLD": { zh: "不是主机世界（缺少 Level.sav）。请先下载最新存档并激活为主机", en: "Not a host world (Level.sav missing). Please download the latest save and activate as host first." },
  "errcode.UPLOAD_FAILED": { zh: "上传到云端失败，请检查网络连接和云服务配置", en: "Upload to cloud failed. Check network connection and cloud config." },
  "errcode.DOWNLOAD_FAILED": { zh: "从云端下载失败，请检查网络连接", en: "Download from cloud failed. Check your network connection." },
  "errcode.CONVERT_FAILED": { zh: "房主转换失败，请重试。可从「备份」页回滚", en: "Host conversion failed. You can rollback from the Backups page." },
  "errcode.STRIP_FAILED": { zh: "转为客机失败，已自动回滚，存档未受影响", en: "Failed to switch to guest. Automatically rolled back, save is unchanged." },
  "errcode.STRIP_FATAL": { zh: "转为客机失败且回滚也失败，请从「备份」页手动恢复", en: "Failed to switch to guest and rollback also failed. Please restore manually from Backups." },
  "errcode.REPLACE_FAILED": { zh: "替换存档失败，已自动回滚，存档未受影响", en: "Failed to replace save. Automatically rolled back, save is unchanged." },
  "errcode.REPLACE_FATAL": { zh: "替换存档失败且回滚也失败，请从「备份」页手动恢复", en: "Failed to replace save and rollback also failed. Please restore manually from Backups." },
  "errcode.RESTORE_FAILED": { zh: "回滚失败，已自动恢复到操作前状态", en: "Restore failed. Automatically recovered to pre-operation state." },
  "errcode.RESTORE_FATAL": { zh: "回滚失败且自动恢复也失败，请联系支持并保留备份文件", en: "Restore failed and auto-recovery also failed. Please contact support and keep backup files." },
  "errcode.FILE_WRITE": { zh: "写入文件失败，请检查磁盘空间和路径权限", en: "Failed to write file. Check disk space and path permissions." },
  "errcode.FILE_READ": { zh: "读取文件失败，请确认文件存在且未损坏", en: "Failed to read file. Confirm the file exists and is not corrupted." },

  "toast.configSaved": { zh: "配置已保存", en: "Config saved" },
  "toast.versionDownloaded": { zh: "已下载该版本", en: "Version downloaded" },
  "toast.rolledBack": { zh: "已回滚", en: "Rolled back" },
  "toast.uploaded": { zh: "存档已上传", en: "Save uploaded" },
  "toast.activated": { zh: "已切换为房主", en: "Switched to host" },

  "label.upload": { zh: "上传存档", en: "Upload Save" },
  "label.downloadActivate": { zh: "下载存档", en: "Download Save" },

  "dialog.exportTitle": { zh: "导出存档", en: "Export Save" },
  "dialog.importTitle": { zh: "导入存档", en: "Import Save" },
  "dialog.savePkg": { zh: "存档包", en: "Save Package" },
  "dialog.uploadTitle": { zh: "上传存档", en: "Upload Save" },
  "dialog.uploadConfirm": {
    zh: "上传后，本机存档将从「房主」转为「客机」——仅保留个人进度数据，不能继续游玩。\n\n云端将保存最新存档供其他玩家下载。你可以随时在「备份」页回滚恢复。\n\n确定要上传吗？",
    en: "After uploading, your local save will switch from Host to Guest — only personal progress is kept, and you cannot continue playing.\n\nThe latest save will be stored in the cloud for others to download. You can restore from the Backups page at any time.\n\nAre you sure you want to upload?",
  },
  "dialog.confirmUpload": { zh: "确认上传", en: "Confirm Upload" },
  "dialog.downloadTitle": { zh: "下载存档", en: "Download Save" },
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
  "worlds.selectAccount": { zh: "Steam 账号", en: "Steam Account" },
  "worlds.accountAuto": { zh: "自动检测", en: "Auto-detect" },
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
  "worlds.guestHint": { zh: "你当前不是此世界的房主。点击下方「下载存档」即可接手。", en: "You are not the host of this world. Click Download Save below to take over." },
  "worlds.guestOnly": { zh: "非房主不可用：先点「下载存档」接手", en: "Guest-only: click Download Save first" },
  "worlds.swapHost": { zh: "云端换房主", en: "Cloud Host Swap" },
  "worlds.swapHostDesc": {
    zh: "通过云端传输存档。当前房主上传存档，接手方下载后自动成为新房主。",
    en: "Transfer the save via cloud. The current host uploads the save; the recipient downloads it and automatically becomes the new host.",
  },
  "worlds.btnUpload": { zh: "⬆ 上传存档", en: "⬆ Upload Save" },
  "worlds.btnDownloadActivate": { zh: "📥 下载存档", en: "📥 Download Save" },
  "worlds.manualTransfer": { zh: "手动换房主", en: "Manual Host Swap" },
  "worlds.manualDesc": {
    zh: "通过文件传输存档。当前房主导出存档文件发给对方；对方导入后自动成为新房主。",
    en: "Transfer the save via file. The current host exports a save file and sends it; the recipient imports it and automatically becomes the new host.",
  },
  "worlds.btnExport": { zh: "📤 导出存档", en: "📤 Export Save" },
  "worlds.btnImport": { zh: "📥 导入存档", en: "📥 Import Save" },
  "worlds.exportDiag": { zh: "🩺 导出诊断包", en: "🩺 Export Diagnostics" },
  "worlds.diagTitle": { zh: "诊断", en: "Diagnostics" },
  "worlds.diagDesc": { zh: "导出当前存档、备份和日志，便于排查问题", en: "Export current save, backups, and log for troubleshooting" },

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
  "backups.guestLabel": { zh: "客机存档", en: "Guest Save" },
  "backups.openFolder": { zh: "打开备份目录", en: "Open Backup Folder" },

  "settings.qiniu": { zh: "七牛云 Kodo", en: "Qiniu Kodo" },
  "settings.accessKey": { zh: "AccessKey", en: "AccessKey" },
  "settings.secretKey": { zh: "SecretKey", en: "SecretKey" },
  "settings.bucket": { zh: "空间名称", en: "Bucket" },
  "settings.domain": { zh: "下载域名（留空自动获取）", en: "Download domain (leave blank for auto)" },
  "settings.general": { zh: "通用", en: "General" },
  "settings.uploader": { zh: "上传者名（标识版本）", en: "Uploader name (identifies versions)" },
  "settings.saveRoot": { zh: "存档目录（留空自动检测）", en: "Save directory (leave blank for auto-detect)" },
  "settings.autoDetect": { zh: "自动检测：{0}", en: "Auto-detected: {0}" },
  "settings.save": { zh: "保存配置", en: "Save Config" },
  "settings.about": { zh: "关于", en: "About" },
  "settings.checkUpdate": { zh: "检查更新", en: "Check for Updates" },
  "settings.checking": { zh: "检查中...", en: "Checking..." },
  "settings.newVersion": { zh: "发现新版本 v{0}", en: "New version v{0} available" },
  "settings.updateNow": { zh: "立即更新", en: "Update Now" },
  "settings.updating": { zh: "下载中...", en: "Downloading..." },
  "settings.updateHint": { zh: "下载完成后将自动重启", en: "Will restart automatically after download" },
  "settings.upToDate": { zh: "已是最新版本", en: "Up to date" },
  "settings.updateMessage": { zh: "发现新版本，建议立即更新。", en: "A new version is available. Update recommended." },
  "settings.later": { zh: "稍后", en: "Later" },
  "toast.diagExported": { zh: "诊断包已导出", en: "Diagnostic bundle exported" },
  "help.title": { zh: "原理说明", en: "How It Works" },
  "help.flowTitle": { zh: "房主转移流程", en: "Host Transfer Flow" },
  "help.flowDesc": { zh: "房主转移分两步：当前房主导出中转包（本地转为客机），新房主导入中转包（自动成为新房主）。注意：直接拷别人存档文件夹不行，必须走导出-导入流程，否则会出现重复玩家数据等问题。", en: "Host transfer has two steps: the current host exports an intermediate (becomes a guest machine), the new host imports it (automatically becomes the new host). Note: directly copying another player save folder does not work - you must use the export-import flow, otherwise duplicate player data and other issues will occur." },
  "help.uidTitle": { zh: "UID 转换原理", en: "UID Transformation" },
  "help.uidDesc": { zh: "存档里房主用 0001 哨兵标识。导出时把 0001 转成房主的真实 UID（变成客机），导入后激活把新房主的 UID 转成 0001（变成房主）。直接拷别人存档不行——0001 还是别人的角色，你进去就顶着别人的角色玩，导出也会被拦。", en: "The host is identified by the 0001 sentinel. Export converts 0001 to the host real UID (becomes guest); activation converts the new host UID to 0001 (becomes host). Copying another player save directly does not work - 0001 still holds their character, so you would play as them and export would be blocked." },
  "help.conceptsTitle": { zh: "核心概念", en: "Key Concepts" },
  "help.c1Title": { zh: "世界 GUID", en: "World GUID" },
  "help.c1Desc": { zh: "：每个世界唯一，工具只允许同世界之间转移", en: ": Each world is unique; transfers only work within the same world" },
  "help.c2Title": { zh: "中转包 ≠ 备份", en: "Intermediate != Backup" },
  "help.c2Desc": { zh: "：中转包（_relay.zip）给别人，备份（_host.zip）给自己回滚，不要混发", en: ": Intermediate (_relay.zip) is for others; backup (_host.zip) is for your own rollback - do not mix them" },
  "help.c3Title": { zh: "玩家 UID", en: "Player UID" },
  "help.c3Desc": { zh: "：由 Steam 账号派生的唯一标识，每个玩家不同。存档里房主用 0001 哨兵代替真实 UID，工具通过转换它实现房主切换", en: ": A unique identifier derived from the Steam account, different for each player. The host uses 0001 sentinel instead of the real UID in the save; the tool swaps them to transfer host." },
  "help.c4Title": { zh: "LocalData", en: "LocalData" },
  "help.c4Desc": { zh: "：地图进度等个人数据，不随中转包传输，各自保留", en: ": Personal data like map progress, not transferred with the intermediate" },

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

// parseErr extracts a backend error code ([CODE] detail) and maps it to a
// localized user-friendly message. If no code is found or the code is unknown,
// falls back to the raw error string.
export function parseErr(e: unknown, t: (key: string, ...args: (string | number)[]) => string): string {
  const raw = String((e as any)?.message || e);
  const m = raw.match(/^\[(\w+)\]\s*(.*)/s);
  if (m) {
    const key = "errcode." + m[1];
    const friendly = t(key);
    if (friendly !== key) return friendly;
    return m[2] || raw;
  }
  return raw;
}
