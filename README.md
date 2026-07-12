# 幻兽帕鲁换房主

**中文** | [English](README.en.md)

![license](https://img.shields.io/badge/license-MIT-blue) ![platform](https://img.shields.io/badge/platform-Windows-blue) ![go](https://img.shields.io/badge/Go-1.25-00ADD8)

> Palworld 联机「换房主」桌面工具。让多个玩家轮流当房主、共享同一个世界进度，**单文件 exe、零安装**。


---

## 下载

从 [Releases](https://github.com/Aues6uen11Z/palworld-save-relay/releases) 页面下载 `palworld-save-relay.exe`，双击运行即可，无需安装、无需配环境。

> 首次运行自动检测本地 Palworld 存档目录（`%LocalAppData%\Pal\Saved\SaveGames`）。

> ⚠️ **务必关闭 Palworld 的 Steam 云存档**（Steam -> 游戏属性 -> 通用 -> 取消勾选「保持游戏存档在 Steam 云中」）。Steam 云会与本工具的存档替换冲突，导致换房主 / 回滚后存档被覆盖回旧版本。

## 使用方法

在「换房主」页选一个世界，然后二选一：

### 方式一：云同步（推荐）

1. **（仅首次）** 到「设置」配置七牛云：填 AccessKey / SecretKey / Bucket；区域自动识别，下载域名留空自动获取。
2. **当前房主**：点「⬆ 上传存档」——存档上传后，本机存档会转为访客模式（只剩个人数据，不能继续游玩；可在「备份」页回滚恢复）。
3. **接手方**：点「⬇ 下载最新」，再点「🎯 换我当房主」，开游戏即可。

### 方式二：手动传输（不配云也能用）

1. **房主**：点「📤 导出存档」，选位置存成单文件，发给对方。
2. **接手方**：点「📥 导入存档」，选收到的文件，自动成为新房主，开游戏即可。

> 每次上传 / 下载 / 导入 / 换房主前都会自动本地备份，可在「备份」页随时回滚。

## 功能

- 自动检测本地房主存档、世界 / 玩家列表，按 UID 识别房主
- 一键换房主（SteamID → UID via cityhash64）
- 云同步：七牛云上传 / 下载 / 版本历史
- 手动导入导出：单文件中转，不依赖云服务
- 本地备份管理，可一键回滚
- 中英双语界面，一键切换
- 全链路日志（`%AppData%\PalSaveRelay\app.log`）
- Windows 单 exe（Oodle DLL 内嵌），零安装

## 工作原理

> 不关心原理可直接跳过，按上面「使用方法」操作即可。

合作模式存档默认只存在房主本地，房主离线后其他人无法继续。本工具通过「房主 UID 转换 + 云端 / 文件中转」实现换房主。

存档里房主自己的 UID 是哨兵 `00000000-0000-0000-0000-000000000001`，其他人（客人）的 UID 是各自 SteamID 派生的真实 UID（`cityhash64`）。

- **中间态**：把房主哨兵 `0000…0001` 换成房主自己的真实 UID，得到「所有人都是真实 UID、无人是房主」的状态——这是用来传输的格式。
- **上传**：在**临时副本**上做上述转换再打包上传；上传成功后**本机存档裁剪为访客模式**（只留 LocalData.sav），防止原房主继续游玩产生冲突存档（已自动备份，可回滚）。**导出**则只打包临时副本，本机不动。
- **下载 / 导入**：把中间态覆盖进本机（先备份），再把自己的真实 UID 换成 `0000…0001`，即成为新房主。

核心操作 ConvertHost(fromUID → toUID) 是**单向全局 UID 替换**（不是两人互换），配合中间态实现换房主。每个人的存档数据始终挂在各自真实 UID 下，跨传输不丢。

> LocalData.sav 是个人任务 / 地图进度，属于本机，**不随世界传输**（上传 / 下载都不带它），各人保留各人的；但本地备份会包含它以便完整回滚。

## 从源码构建

需要 Go 1.25+、Node.js、[Wails v3 CLI](https://wails.io)、mingw-w64 gcc（go-oodle 依赖 CGO）。

```powershell
.\build.ps1            # 一键构建：前端 + 图标 + syso + go build -> palworld-save-relay.exe
```

或手动：

```bash
cd frontend && npm install && npm run build && cd ..
go build -o palworld-save-relay.exe .
```

> `@wailsio/runtime` 跟着 Wails 版本走，别手动锁老版本，否则前后端协议不一致会报 Invalid runtime call。

## 开发

```bash
wails3 dev                 # 热重载开发
go test ./internal/...     # 后端测试（含真实存档往返）
```

项目结构：

```
internal/
  sav/        存档引擎（移植自 cheahjs/palworld-save-tools）
  palworld/   域逻辑：检测、ConvertHost、SteamID->UID、备份、打包
  storage/    云存储抽象 + 七牛实现
  config/     应用配置
  logger/     进程级文件日志
frontend/     React UI（双语 i18n）
```

测试夹具是真实 Palworld 存档（PlZ/zlib 与 PlM/Oodle 各覆盖），已 gitignore（隐私）。首次运行拷贝：

```powershell
cd internal/sav/testdata; ./fetch.ps1
```

## 已知限制

- 房主真实 UID 可能已存在于存档（如 OldOwnerPlayerUIds）。单次「交出」会产生重复引用，**不影响换房主**。
- 公会 individual_character_handle_ids 的 guid 当前未纳入替换；如实测公会成员归属异常再补。
- 上传后本机存档裁剪为访客模式（上传前自动备份，可回滚恢复）；导出只动临时副本、不改本机；下载 / 导入 / 回滚会完整替换本机世界（先备份再覆盖），「换我当房主」会改本机（先备份）。

## 致谢

- [cheahjs/palworld-save-tools](https://github.com/cheahjs/palworld-save-tools) - 存档格式解析（SAV/GVAS/属性系统）的上游。
- [deafdudecomputers/PalworldSaveTools](https://github.com/deafdudecomputers/PalworldSaveTools) - 房主切换与公会 / 角色解析参考。
- [new-world-tools/go-oodle](https://github.com/new-world-tools/go-oodle) - Go 的 Oodle 绑定。

## License

MIT