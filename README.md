# Palworld 存档转换

**中文** | [English](README.en.md)

Palworld 联机「存档换房主」桌面工具。让多个玩家轮流当房主、共享同一个世界进度，可走云同步或手动导入导出，**单文件 exe、零安装**。

> 合作模式存档默认只存在房主本地，房主离线后其他人无法继续。本工具通过「房主 UID 转换 + 云端/文件中转」实现换房主。

## 换房主原理

存档里房主自己的 UID 是哨兵 `00000000-0000-0000-0000-000000000001`，其他人（客人）的 UID 是各自 SteamID 派生的真实 UID（`cityhash64`）。

- **中间态**：把房主哨兵 `0000…0001` 换成房主自己的真实 UID，得到「所有人都是真实 UID、无人是房主」的状态。这是用来传输的格式。
- **上传 / 导出**：在**临时副本**上做上述转换再打包，**本机存档不动**（你上传完仍是房主）。
- **下载 / 导入**：把中间态覆盖进本机（先备份），再把自己的真实 UID 换成 `0000…0001`，即成为新房主。

核心操作 ConvertHost(fromUID -> toUID) 是**单向全局 UID 替换**（不是两人互换），配合中间态实现换房主。每个人的存档数据始终挂在各自真实 UID 下，跨传输不丢。

> LocalData.sav 是个人任务/地图进度，属于本机，**不随世界传输**（上传/下载都不带它），各人保留各人的；但本地备份会包含它以便完整回滚。

## 功能

- 自动检测本地房主存档（有 Level.sav 的完整世界）、世界/玩家列表，按 UID 识别房主；客人存档（仅 LocalData.sav）不检测
- 一键换房主（SteamID->UID via cityhash64），操作前自动备份、可回滚
- 云同步：七牛云上传/下载/版本历史
- 手动导入导出：没配云服务也能用，导出成单文件发给对方、对方导入即成为房主
- 本地备份管理（每次换房主/下载/导入前自动备份）
- 全链路日志（%APPDATA%\PalSaveRelay\app.log），启动/配置/检测/换房主/云/备份/导入导出均有记录
- Windows 单 exe（Oodle DLL 内嵌）

## 技术栈

Go 1.25 · Wails v3 (alpha2) · React 18 + TypeScript + Vite + Tailwind · 七牛云 Kodo · go-oodle（Oodle，CGO 调用 oo2core DLL，需 mingw gcc）

## 项目结构

```
internal/
  sav/        存档引擎（移植自 cheahjs/palworld-save-tools：SAV 容器/Oodle/GVAS/属性/RawData）
  palworld/   域逻辑：检测、ConvertHost、PackIntermediate、SteamID->UID、备份、打包
  storage/    云存储抽象 + 七牛实现
  config/     应用配置
  logger/     进程级文件日志
main.go app.go   Wails v3 入口与 bindings
frontend/        React UI
docs/superpowers/{specs,plans}/   设计文档与实现计划
```

## 构建

需要 Go 1.25+、Node.js、[Wails v3 CLI](https://wails.io)、mingw-w64 gcc（go-oodle 依赖 CGO）：

```bash
wails3 task build          # 生产构建（含 npm build + go build），输出 bin/palworld-save-relay.exe
# 或手动：
cd frontend && npm install && npm run build && cd ..
go build -o bin/palworld-save-relay.exe .
```

> 注意：frontend/package.json 里 @wailsio/runtime 跟着 Wails 版本走，别手动锁老版本，否则前后端协议不一致会报 Invalid runtime call。

## 开发

```bash
wails3 dev                 # 热重载开发
go test ./internal/...     # 后端测试（含真实存档往返）
```

测试夹具是真实 Palworld 存档（PlZ/zlib 与 PlM/Oodle 各覆盖），已 gitignore（隐私）。首次运行拷贝：

```powershell
cd internal/sav/testdata; ./fetch.ps1
```

## 使用

1. （可选）到「设置」配云服务（七牛云 AccessKey/SecretKey/Bucket；区域自动识别，下载域名留空自动获取）。不配也能用导入导出。
2. 在「换房主」里选要换房主的世界。
3. **云同步方式**：当前房主点「上传存档」（本机不变，仍是房主）；接手方点「下载最新」再「换我当房主」，开游戏。
4. **手动方式**：房主点「导出存档」选位置存成单文件，发给对方；对方点「导入存档」选该文件，自动成为新房主。

## 已知限制 / 注意

- 房主真实 UID 可能已存在于存档（如 OldOwnerPlayerUIds）。单次「交出」会产生重复引用，**不影响换房主**（单向流，中间态里多几处引用无害）。
- 公会 individual_character_handle_ids 的 guid 当前未纳入替换；如实测公会成员归属异常再补。
- 上传/导出只动临时副本、不改本机；下载/导入会覆盖本机世界（先备份），「换我当房主」会改本机（先备份）。
- 托盘图标 / 自动监听游戏关闭 / 自动上传为后续计划。

## 致谢

- [cheahjs/palworld-save-tools](https://github.com/cheahjs/palworld-save-tools) - 存档格式解析（SAV/GVAS/属性系统）的上游。
- [deafdudecomputers/PalworldSaveTools](https://github.com/deafdudecomputers/PalworldSaveTools) - 房主切换与公会/角色解析参考。
- [new-world-tools/go-oodle](https://github.com/new-world-tools/go-oodle) - Go 的 Oodle 绑定。

## License

MIT