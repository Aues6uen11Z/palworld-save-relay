# Pal Save Relay

Palworld 联机「存档接力」桌面工具。让多个玩家轮流当房主、共享同一个世界进度，基于七牛云对象存储中转，**单文件 exe、零安装**。

> 合作模式存档默认只存在房主本地，房主离线后其他人无法继续。本工具通过「房主 UID 转换 + 云端中转」实现接力。

## 接力原理

存档里房主自己的 UID 是哨兵 `00000000-0000-0000-0000-000000000001`，其他人（客人）的 UID 是各自 SteamID 派生的真实 UID（`cityhash64`）。

1. **房主交出**：当前房主 A 用本机 SteamID 算出自己的真实 UID，把存档里所有 `0000…0001` 替换成自己的真实 UID（得到一个**中间态**：所有人都是真实 UID、无房主哨兵），打包上传到七牛云。
2. **接班人接手**：B 下载中间态，把自己的真实 UID 替换成 `0000…0001`，B 即成为房主，开游戏继续。

核心操作 `ConvertHost(fromUID → toUID)` 是**单向全局 UID 替换**（不是两人互换），配合云端中间态实现接力。

## 功能

- 自动检测本地存档、世界/玩家列表，按 UID 识别房主
- 一键房主转换（`SteamID→UID` via cityhash64），切换前自动备份、可回滚
- 七牛云上传/下载/版本历史/游玩锁（提示性 + TTL）
- 本地备份管理（保留最近 N 份）、导入导出（兜底单文件）
- Windows 单 exe（Oodle DLL 内嵌）

## 技术栈

Go 1.22+ · Wails v3 (alpha) · React 18 + TypeScript + Vite + Tailwind · 七牛云 Kodo · `go-oodle`（Oodle，purego 免 CGo）

## 项目结构

```
internal/
  sav/        存档引擎（移植自 cheahjs/palworld-save-tools：SAV 容器/Oodle/GVAS/属性/RawData）
  palworld/   域逻辑：检测、ConvertHost、SteamID->UID、备份、打包
  storage/    云存储抽象 + 七牛实现 + 游玩锁
  config/     应用配置
main.go app.go   Wails v3 入口与 bindings（19 个方法）
frontend/        React UI
docs/superpowers/{specs,plans}/   设计文档与实现计划
```

## 构建

需要 Go 1.22+、Node.js、[Wails v3 CLI](https://wails.io)：

```bash
wails3 task build          # 生产构建（含 npm build + go build），输出 bin/palworld-save-relay.exe
# 或手动：
cd frontend && npm install && npm run build && cd ..
go build -o bin/palworld-save-relay.exe .
```

## 开发

```bash
wails3 dev                 # 热重载开发
go test ./...              # 后端测试（含真实存档往返）
```

测试夹具是真实 Palworld 存档（PlZ/zlib 与 PlM/Oodle 各覆盖），已 gitignore（隐私）。首次运行拷贝：

```powershell
cd internal/sav/testdata; ./fetch.ps1
```

## 使用

1. 首次启动到「设置」填七牛云（AccessKey/SecretKey/Bucket/区域；下载域名留空自动获取）。
2. 「世界与接力」选要接力的世界。
3. 房主：点「上传交出」→（玩完）「释放锁」。
4. 接班人：「下载最新」→「接手当房主」→「占锁」→ 开游戏。

## 已知限制 / 注意

- 房主真实 UID 可能已存在于存档（如 `OldOwnerPlayerUIds`）。单次「交出」会产生重复引用，**不影响接力**（单向流，中间态里多几处引用无害）。
- 公会 `individual_character_handle_ids` 的 `guid` 当前未纳入替换；如实测公会成员归属异常再补。
- 游玩锁为**提示性**（对象存储无原子 CAS），配合 TTL。
- 托盘图标 / 自动监听游戏关闭 / 自动上传为后续计划（Wails v3 托盘已预留）。

## 致谢

- [cheahjs/palworld-save-tools](https://github.com/cheahjs/palworld-save-tools) — 存档格式解析（SAV/GVAS/属性系统）的上游。
- [deafdudecomputers/PalworldSaveTools](https://github.com/deafdudecomputers/PalworldSaveTools) — 房主切换与公会/角色解析参考。
- [new-world-tools/go-oodle](https://github.com/new-world-tools/go-oodle) — Go 的 Oodle 绑定。

## License

MIT
