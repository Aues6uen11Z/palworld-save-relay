# AGENTS.md — palworld-save-relay

## Project Overview

Wails v3 桌面应用（Go + React/TypeScript），用于幻兽帕鲁的多人存档房主转移。前端 React + Tailwind CSS，后端 Go，通过 Wails 绑定通信。

## Tech Stack

- **Runtime**: Wails v3 (`github.com/wailsapp/wails/v3`)
- **Frontend**: React 19 + TypeScript + Vite + Tailwind CSS
- **Backend**: Go (CGO_ENABLED=1, Windows only)
- **State**: 本地 JSON config (`~/.config/palworld-save-relay/config.json`)
- **Cloud**: 七牛云对象存储

## Project Structure

```
├── app.go                  # Wails service（所有前端可调用的方法）
├── main.go                 # 入口，窗口配置
├── version.txt             # 当前版本号（如 v0.5.8），release 时自动更新
├── frontend/
│   ├── src/App.tsx          # 主组件（所有页面都在这一个文件里）
│   ├── src/i18n.tsx         # 中英文翻译
│   └── src/index.css        # Tailwind 入口
├── internal/
│   ├── palworld/            # 存档解析、Steam ID、主机转换
│   ├── storage/             # 七牛云存储、版本管理
│   ├── updater/             # 自动更新（下载 + bat 替换重启）
│   ├── config/              # 配置读写
│   └── sav/                 # .sav 文件底层解析
├── build/                   # 图标资源（appicon.png → icon.ico）
├── build.ps1                # 本地构建脚本
└── release.ps1              # 一键发布脚本（GitHub + Gitee）
```

## Coding Conventions

- **二进制输出到根目录**: 所有构建（调试和发布）都将 `palworld-save-relay.exe` 输出到项目根目录，不是 `dist/`。
- **单文件组件**: 所有页面视图都在 `frontend/src/App.tsx` 中，不拆分文件。保持简单。
- **i18n**: 所有用户可见文本必须在 `i18n.tsx` 中定义中英文，通过 `t("key")` 调用。
- **Wails 绑定**: 前端调用后端方法通过 `App.MethodName()`（自动生成的绑定），不要直接 fetch。
- **Go 错误处理**: 用 `logger.Errorf` 记录错误，返回 `fmt.Errorf` 给前端。不 panic。
- **Windows only**: 这是纯 Windows 应用，不需要考虑跨平台。

## Build & Release Workflow

### 日常开发构建：`build.ps1`

```powershell
.\build.ps1              # 默认 amd64
.\build.ps1 -Arch arm64  # ARM64
```

流程：
1. `npm run build`（tsc 类型检查 + vite 构建 → `frontend/dist/`）
2. `wails3 generate icons`（`build/appicon.png` → `.ico` / `.icns`）
3. `wails3 generate syso`（图标 + manifest → `wails.syso`）
4. `go build`（嵌入 `frontend/dist` + syso → 根目录 `palworld-save-relay.exe`）

产出：根目录下的 `palworld-save-relay.exe`

### 发布流程：`release.ps1`

```powershell
.\release.ps1 v0.5.9 "修复了 XX 问题"
```

自动化流程：
1. 构建二进制（带版本号 ldflags + `-H windowsgui`）
2. 更新 `version.txt`，git commit
3. git tag + push 到 GitHub 和 Gitee
4. 创建 GitHub release（附带二进制）
5. 创建 Gitee release（附带二进制）
6. 验证 `version.txt` 在两个平台都已更新

前提：
- `gh` CLI 已登录（`gh auth login`）
- `.gitee_token` 文件包含 Gitee API token
- `git push` 到 `origin master` 有权限

### 提交规范

- 提交信息用中文或英文均可，前缀用 conventional commits 风格：
  - `fix:` 修复 bug
  - `feat:` 新功能
  - `refactor:` 重构
  - `chore:` 版本号、构建配置等
- 一次提交做一个事情，不要混合不相关的改动
- 提交前确保 `npm run build` 和 `go build` 都通过

### 手动构建（不用 build.ps1）

```powershell
# 1. 前端
cd frontend && npm run build && cd ..

# 2. Go（开发调试版，带版本号）
go build -ldflags "-X main.Version=v0.5.8" -o palworld-save-relay.exe .

# 3. Go（发布版，带 windowsgui 标志）
go build -ldflags="-w -s -H windowsgui -X main.Version=v0.5.8" -trimpath -o palworld-save-relay.exe .
```

## Important Notes

- `version.txt` 是自动更新的版本源，release 时由 `release.ps1` 自动更新
- 自动更新机制：Go 后端下载新二进制 → 写 bat 脚本 → `os.Exit(0)` 退出 → bat 替换 exe 并重启
- bat 脚本中 `%` 必须转义为 `%%`（因为 `fmt.Sprintf` 会解析 `%d`/`%s`）
- 前端组件全在 App.tsx 里，超过 700 行是正常的，不需要拆分
