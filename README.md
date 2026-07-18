# 幻兽帕鲁换房主

**中文** | [English](README.en.md)

![license](https://img.shields.io/badge/license-MIT-blue) ![platform](https://img.shields.io/badge/platform-Windows-blue) ![go](https://img.shields.io/badge/Go-1.25-00ADD8)

> Palworld 联机「换房主」桌面工具。多人轮流当房主、共享同一个世界进度，**单文件 exe、零安装**。

## 下载

从 [Releases](https://github.com/Aues6uen11Z/palworld-save-relay/releases) 下载 `palworld-save-relay.exe`，双击运行。

> 首次运行自动检测本地存档目录（`%LocalAppData%\Pal\Saved\SaveGames`）。

> ⚠️ **关闭 Palworld 的 Steam 云存档**（Steam -> 属性 -> 通用 -> 取消「保持游戏存档在 Steam 云中」），否则 Steam 云会把换房主后的存档覆盖回旧版。

## 使用方法

在「换房主」页选一个世界，二选一：

**云同步（推荐）**：当前房主点「上传存档」（本机转为访客模式，已自动备份）；接手方点「下载最新」→「换我当房主」→ 开游戏。

**手动传输**：房主点「导出存档」发文件给对方；接手方点「导入存档」→ 自动成为新房主 → 开游戏。

> 每次操作前自动本地备份，可在「备份」页回滚。

## 功能

- 自动检测存档目录、世界/玩家列表，按 UID 识别房主
- 一键换房主（SteamID → UID，cityhash64）
- 云同步（七牛云）：上传/下载/版本历史/游玩锁
- 手动导入导出：单文件中转，不依赖云
- **自动修复历史坏档**：下载/导入时自动重建被截断的公会 ICH、把散落的帕鲁收拢回房主槽（修复「据点帕鲁无法举起」bug）
- 本地备份管理，一键回滚
- 中英双语，全链路日志（`%AppData%\PalSaveRelay\app.log`）

## 工作原理

合作模式存档只存在房主本地。存档里房主 UID 是哨兵 `0000…0001`，客人是各自 SteamID 派生的真实 UID。

换房主只搬**房主玩家的身份**（玩家条目、公会成员引用、帕鲁所有权、玩家文件）从 `0001` 挪到他的真实 UID；**世界数据**（所有帕鲁、建筑等）留在 `0001` 房主槽。下一个房主激活时把自己的真实 UID 换成 `0001`，直接继承全部世界数据——换房主后的存档与官方房主存档结构一致。

- **上传/导出**：在临时副本上做上述转换再打包；上传成功后本机裁剪为访客模式（只剩个人数据，已备份可回滚）；导出只动临时副本，本机不改。
- **下载/导入**：覆盖本机（先备份），自动修复历史坏档，再激活成房主（真实 UID → `0001`）。
- **LocalData.sav** 是个人任务/地图进度，不随世界传输，各人保留各人。

## 从源码构建

需要 Go 1.25+、Node.js、[Wails v3 CLI](https://wails.io)、mingw-w64 gcc（CGO）。

```powershell
.\build.ps1            # 前端 + 图标 + syso + go build
```

## 开发

```bash
wails3 dev              # 热重载
go test ./internal/...  # 后端测试（含真实存档往返）
```

```
internal/
  sav/        存档引擎（移植自 cheahjs/palworld-save-tools）
  palworld/   域逻辑：检测、换房主、SteamID->UID、备份、打包、坏档修复
  storage/    云存储抽象 + 七牛实现
  config/     应用配置
  logger/     进程级文件日志
frontend/     React UI（双语 i18n）
```

测试夹具是真实存档（PlZ/zlib 与 PlM/Oodle 各覆盖），已 gitignore。`cd internal/sav/testdata; ./fetch.ps1` 拉取。

## 已知限制

- 上传后本机裁剪为访客模式（上传前自动备份，可回滚）；导出不动本机；下载/导入/回滚/换房主都会先备份再改本机。
- 房主真实 UID 可能已存在于 OldOwnerPlayerUIds 等历史字段，产生重复引用，不影响换房主。

## 致谢

- [cheahjs/palworld-save-tools](https://github.com/cheahjs/palworld-save-tools) — 存档格式解析（SAV/GVAS/属性系统）上游。
- [deafdudecomputers/PalworldSaveTools](https://github.com/deafdudecomputers/PalworldSaveTools) — 房主切换与公会/角色解析参考。
- [new-world-tools/go-oodle](https://github.com/new-world-tools/go-oodle) — Go 的 Oodle 绑定。

## License

MIT
