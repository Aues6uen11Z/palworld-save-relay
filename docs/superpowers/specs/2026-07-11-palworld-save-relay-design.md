# Pal Save Relay 设计文档

- 日期：2026-07-11
- 状态：待评审
- 关联需求：`PRD.md`
- 参考项目：`cheahjs/palworld-save-tools`（852★，存档解析上游）、`deafdudecomputers/PalworldSaveTools`（fork，房主切换逻辑来源）、`zaigie/palworld-server-tool`（Go，数据模型参考；其存档解析依赖外部 sav_cli，不直接复用）

## 一、项目目标与范围

做一款 **Windows 单文件、零安装** 的桌面工具，实现 Palworld 合作模式「存档接力」：多个玩家轮流当房主，共享同一个世界进度，基于七牛云对象存储中转。

### 本期范围（IN）
- 自动检测本地存档、世界/玩家列表、别名、隐藏
- 房主切换（UID 全局互换），切换前自动备份、可回滚
- 七牛云上传/下载/版本历史/断点续传
- 提示性游玩锁（TTL）
- 导入导出（兜底单文件）
- 本地备份管理（保留最近 N 份）

### 本期不做（OUT）
- 托盘图标 / 自动监听游戏关闭 / 自动上传（Wails v3 托盘已预留，留待后续）
- 专用服务器存档管理（聚焦合作主机接力）
- 帕鲁编辑 / 地图查看（只做接力）

## 二、技术栈

| 层级 | 选型 | 说明 |
|------|------|------|
| 桌面框架 | Wails v3（锁定某 alpha 版，不追每日新版） | 原生托盘为未来自动上传铺路；用户已确认接受 alpha 风险 |
| 后端 | Go 1.22+ | |
| 前端 | React 18 + TypeScript + Vite | 工具链支持最强（shadcn/react 技能、Playwright 测试） |
| UI 组件 | shadcn/ui + Tailwind CSS | 干净现代、依赖轻 |
| 状态管理 | Zustand | 轻量 |
| 云存储 | 七牛云 Kodo（`github.com/qiniu/go-sdk/v7`） | |
| 存档解析 | 自研 Go 移植（来自 cheahjs） | 无现成 Go 库 |
| Oodle 压缩 | `oo2core_9_win64.dll`，syscall 加载，免 CGo | 内嵌于 exe |
| 打包 | Windows 单 exe（DLL 内嵌 go:embed） | |

## 三、领域知识：Palworld 存档格式

> 通过对本机真实存档（`%LOCALAPPDATA%\Pal\Saved\SaveGames\<SteamID>\<WorldGUID>\`）的解析得出。

### 3.1 目录结构
```
<SteamID>/
  GlobalPalStorage.sav          # 全局帕鲁箱（跨世界，本期不纳入接力包）
  <WorldGUID>/
    Level.sav                   # 世界存档（角色/帕鲁/据点/公会）
    LevelMeta.sav
    LocalData.sav
    WorldOption.sav
    Players/
      00000000000000000000000000000001.sav   # 主机玩家（UID 哨兵）
      <GUID>.sav                            # 客人玩家
    backup/                      # 游戏自带备份（接力包排除）
```
本机实测：主机玩家文件名为 `0000...0001`，客人为真实 GUID。

### 3.2 SAV 容器格式
头：`[4B uncompressed_len LE][4B compressed_len LE][3B magic][1B save_type]`，数据起始于 offset 12。CNK 多一层 12B 头。

magic 字节决定压缩算法：
- `PlZ` -> zlib。`save_type==50` 为双层 zlib（解压两次）；`==49` 为单层
- `PlM` -> Oodle（Epic 私有，需 `oo2core` DLL）
- `CNK` -> 双层头，内层再判 PlZ/PlM

### 3.3 为什么有两种格式（实测结论）
格式**不按文件类型分**（同一个 Level.sav 在不同世界可能是 PlZ 或 PlM），而是按**游戏版本/存盘时机**分：Palworld 某次引擎升级后把压缩从 zlib 换成 Oodle；旧世界保持 PlZ，新世界为 PlM，游戏两套解码器都能读。

**设计影响（关键）**：
1. 读文件头 magic 判断走 zlib 还是 Oodle，不可假设
2. 写回**保留原格式**，绝不跨格式转换（避免游戏拒载坏档）
3. Oodle 读+写都必须支持 -> 需内嵌 DLL

### 3.4 GVAS 格式（解压后的载荷）
- 头：magic(0x53415647, 即 ASCII "GVAS" 的 LE)、save_game_version=3、包版本、引擎版本、分支串、custom_version_format=3、custom_versions 数组、save_game_class_name 串
- 属性树：`{name:FString, type:FString, size:u64, value:<类型特定>}` 序列，到 name=="None" 结束
- 尾：4 字节（通常 0x00000000）
- UE 属性类型：Int/Int64/Float/Double/Str/Name/Enum/Array/Struct/Map/Byte/Bool 等
- RawData 自定义属性：`CharacterSaveParameterMap`、`GroupSaveDataMap` 等为原始字节块，需各自解析

### 3.5 房主切换本质
在 `Level.sav` 与两个玩家 `.sav` 之间全局互换 `oldHostUID ↔ newHostUID`：
- 玩家 .sav：`SaveData.PlayerUId`、`IndividualId.PlayerUId`
- Level.sav `CharacterSaveParameterMap`：按 InstanceId 定位两个角色条目，互换 PlayerUId
- Level.sav `GroupSaveDataMap`（公会）：互换 admin_player_uid、players[].player_uid、individual_character_handle_ids[].guid（按 instance_id 定位）
- Level.sav 深度互换：OwnerPlayerUId / owner_player_uid / build_player_uid / private_lock_player_uid
- 重命名玩家 .sav 文件（按 GUID 命名）；处理 `_dps.sav`（若存在）与 PalStorageContainerId

**前置约束**：目标玩家必须已存在于该存档（至少以客人身份进过一次世界），否则无法切换为房主。UI 须提示。

## 四、总体架构

```
┌──────────────────────────────────────────────┐
│  Wails v3 单进程                              │
│  ┌────────────────┐    ┌──────────────────┐  │
│  │ Go 后端 bindings│◄──►│ React 前端        │  │
│  │ (app.go)        │ 事件│ (shadcn/Tailwind) │  │
│  └──────┬─────────┘    └──────────────────┘  │
│  ┌──────▼───────────────────────────────┐    │
│  │ internal/                             │    │
│  │  sav/    存档引擎(移植 cheahjs)        │    │
│  │  palworld/  检测/房主切换/备份/打包     │    │
│  │  storage/   七牛云+游玩锁(抽象接口)     │    │
│  │  config/    配置                       │    │
│  └───────────────────────────────────────┘    │
│  资源: oo2core_9_win64.dll (go:embed 内嵌)    │
└──────────────────────────────────────────────┘
        │ HTTP
   ┌────▼─────┐
   │ 七牛云 Kodo │ (共享桶，无中心服务器)
   └──────────┘
```

- Wails bindings 暴露高层方法；长操作（解析大 Level.sav、上传下载）通过事件回传进度
- Storage 接口抽象，便于将来接 OSS/COS
- 纯 Go 工具链构建（Oodle 用 syscall 加载 DLL，免 CGo）

## 五、项目结构

```
palworld-save-relay/
  main.go                # Wails app 入口、服务注册（托盘预留）
  app.go                 # bindings 层：前端可调用方法 + 事件发射
  internal/
    sav/                 # 存档引擎（逐行对照 cheahjs 移植）
      container.go       # SAV 容器：头解析 + 解压/压缩路由(PlZ/PlM/CNK)
      oodle.go           # Oodle DLL syscall 加载(解压/压缩)
      gvas.go            # GVAS 头 + 文件读写
      archive.go         # FArchiveReader/Writer 原语
      properties.go      # UE 属性类型读写
      rawdata.go         # RawData 分发(已知解析，未知原样字节往返)
      rawdata_character.go  # CharacterSaveParameterMap 解析
      rawdata_group.go      # GroupSaveDataMap(公会)解析
      paltypes.go         # 类型提示/自定义属性注册表
    palworld/
      detect.go          # 检测存档根、列世界、列玩家
      hostswap.go        # 房主切换：UID 全局互换(核心)
      backup.go          # 本地备份(保留最近N份)
      pack.go            # 存档目录打包/解包(zip)，云同步+导入导出复用
    storage/
      storage.go         # Storage 接口
      qiniu.go           # 七牛实现(上传/下载/列举/删除, 断点续传)
      lock.go            # 游玩锁(提示性+TTL)
    config/config.go     # 配置读写(%APPDATA%/PalSaveRelay/config.json)
  frontend/              # React + Vite + TS
    src/{pages,components,stores,api}
  resources/oo2core_9_win64.dll  # 内嵌资源
  docs/superpowers/specs/
```

## 六、存档引擎（internal/sav，移植 cheahjs）

### 6.1 流水线
`SAV文件 ->[容器头]->[解压]-> GVAS字节 ->[GVAS头+属性树]-> 内存属性树 ->[改]->[序列化]-> GVAS字节 ->[压缩]-> SAV文件`

### 6.2 SAV 容器 (container.go)
- 解压：PlZ -> `zlib.decompress(data[12:])`，type==50 再解一次；PlM -> Oodle 解压 `data[12:12+compLen]` 到 uncompLen
- 压缩：按原 magic+type 反向，**保留原格式**
- 校验：解压后长度须匹配 uncompLen

### 6.3 Oodle (oodle.go)
- DLL 内嵌 `go:embed` -> 释放到工作目录 -> Windows syscall（LoadLibrary/GetProcAddress）加载
- 两个函数：`OodleLZ_Decompress(comp, compLen, out, outLen, ...)`、`OodleLZ_Compress(Kraken, Normal, src, srcLen, dst, ...)`
- 免 CGo，保证纯 Go 工具链构建
- 备选：syscall 加载若受阻，回退 go-oodle（CGo）

### 6.4 GVAS + 属性系统 (gvas.go/archive.go/properties.go)
- 移植 UE 属性读写器；每种类型读/写必须互为精确逆运算
- FArchive 原语：i8/u8/i16/u16/i32/u32/i64/u64/float/double/bool/fstring/guid/bytes
- 属性类型：Int/Int64/Float/Double/Str/Name/Enum/Array/Struct/Map/Byte/Bool

### 6.5 RawData (rawdata*.go)
- 已知类型（`CharacterSaveParameterMap`、`GroupSaveDataMap`）解析为结构体
- **未知类型保留原始字节原样往返**（不坏档的关键）
- 房主切换只需改这两个 + 命名 UID 字段，其余原样穿过

### 6.6 正确性保证
本机真实存档做「sav->gvas->sav 字节一致」往返测试，PlZ 与 PlM 各覆盖。

## 七、房主切换（palworld/hostswap.go，核心）

移植 `fix_host_save.fix_save`，事务化执行（在副本上操作，成功才落盘）：

1. **备份**：复制 `Level.sav` + `Players/` 到本地备份目录
2. **解析**：解压+解析 `Level.sav`、主机玩家 .sav、目标玩家 .sav
3. **互换玩家存档内 UID**：`SaveData.PlayerUId`、`IndividualId.PlayerUId`（old↔new）；记录 InstanceId、PalStorageContainerId
4. **Level.sav · CharacterSaveParameterMap**：按 InstanceId 定位两个角色条目，互换 PlayerUId
5. **Level.sav · GroupSaveDataMap（公会）**：互换 admin_player_uid、players[].player_uid、individual_character_handle_ids[].guid（按 instance_id 定位）
6. **Level.sav · 深度互换**：递归 OwnerPlayerUId / owner_player_uid / build_player_uid / private_lock_player_uid（old↔new）
7. **`_dps.sav`**（若存在）：按 PalStorageContainerId 互换并重命名
8. **重命名玩家 .sav 文件**：主机文件->newUid.sav，目标文件->oldUid.sav
9. **落盘前校验**：重新解压刚写入的三个文件确认可解析，再原子 rename（.tmp->正式）；任一步失败回滚备份
10. **游戏占用检测**：写入前检查 Palworld.exe 是否运行，运行中阻止并提示

## 八、检测与列表（palworld/detect.go）

- 存档根：`%LOCALAPPDATA%\Pal\Saved\SaveGames\`
- 列 SteamID 文件夹（通常一个）；其下列 WorldGUID 文件夹（每个含 Level.sav 即一个世界）
- 每个世界展示：GUID、最后修改时间、玩家数、别名（用户定义，存配置）、隐藏标记（用户定义）
- 列玩家：解析 Level.sav 的 CharacterSaveParameterMap 取 IsPlayer 条目（UID/昵称/等级/最后在线）；标记当前房主（UID 0000...0001）

## 九、云同步（storage/）

### 9.1 Storage 接口
```go
type Object struct {
    Key, Uploader string
    Size int64
    LastModified time.Time
}
type Storage interface {
    Upload(key string, r io.Reader, size int64) error
    Download(key string, w io.Writer, prog func(int64,int64)) error
    List(prefix string) ([]Object, error)
    Delete(key string) error
    Get(key string) ([]byte, error)
    Put(key string, data []byte) error
}
```

### 9.2 云端 key 方案（共享桶，无中心服务器）
- 版本包：`saves/<worldGUID>/<unixmillis>__<uploader>.zip`
- 元信息编码进文件名，**不维护单独 latest 指针**（避免指针与实际版本不一致）
- 「下载最新」= List 前缀取时间戳最大者

### 9.3 打包内容
整个世界文件夹（Level.sav + LevelMeta.sav + WorldOption.sav + Players/*.sav + LocalData.sav），**排除游戏自带 backup/ 子目录**。还原时整体覆盖到本机 `<SteamID>/<WorldGUID>/`。GlobalPalStorage.sav 本期不纳入。

### 9.4 断点续传
- 上传：七牛分片/续传上传器
- 下载：range 分块（~2MB/块）+ 临时 .part 文件记录偏移，失败可续

## 十、游玩锁（storage/lock.go，提示性 + TTL）

`lock.json` = `{player, acquiredAt}`，存 `saves/<worldGUID>/lock.json`。
- 抢占前先查：存在且 `now - acquiredAt < TTL`（默认 6h）-> 弹「X 正在游玩（自 Y），是否强制接管？」确认后覆盖
- 超过 TTL 视为空闲，仅提示可接管
- 玩完释放：Delete(lock.json)
- 七牛无原子 CAS，故为提示性（符合 PRD）；TTL 可配

## 十一、备份管理（palworld/backup.go）

- 触发：每次房主切换前、每次云下载覆盖前、每次导入覆盖前
- 位置：`%APPDATA%/PalSaveRelay/backups/<worldGUID>/<yyyy-mm-dd_HHMMss>.zip`（与游戏自带 backup/ 分开）
- 保留：默认最近 5 份（可配），超出删旧
- UI：列表（时间、大小）、回滚（回滚前再备一份当前）、删除

## 十二、导入导出（palworld/pack.go 复用）

- `PackWorld(dir)->zip` / `UnpackWorld(zip,dest)`，云同步与导入导出共用
- 导出：打包成用户选择的 `.palrelay.zip`，手动传输
- 导入：选文件 -> 解包到临时目录 -> 提示选「我是谁」做房主切换 -> 备份当前 -> 写入 `<SteamID>/<WorldGUID>/`

## 十三、前端（React 18 + TS + Vite + shadcn/ui + Tailwind + Zustand）

| 页面 | 内容 |
|------|------|
| 引导页 | 首次：填七牛配置、确认存档目录 |
| 主页 | 当前接力世界（别名/GUID/房主/人数）、游玩锁状态、三大按钮：上传/下载最新/切换我为房主 |
| 房主切换 | 世界列表（别名+GUID+修改时间+人数，隐藏/显示、改别名）-> 选世界 -> 玩家列表（昵称/等级/UID/最后在线，标记当前房主）-> 选「我是谁」-> 确认 -> 进度 -> 成功 |
| 云同步 | 上传(进度)、下载最新、版本历史列表(时间/上传者/大小，可单独下载)、游玩锁抢占/释放 |
| 备份管理 | 本地备份列表、回滚、删除 |
| 导入导出 | 导出文件、导入文件(可选切换房主) |
| 设置 | 七牛配置(测试连接)、存档目录覆盖、默认上传者名、版本保留数、锁TTL、隐藏存档管理 |

长操作通过 Wails 事件回传进度，前端显示进度条。

## 十四、错误处理

- **事务化**：所有改存档操作 = 备份 -> 临时副本操作 -> 校验 -> 原子 rename 落盘；失败回滚备份
- **原子写**：写 .tmp -> fsync -> rename
- **落盘前校验**：写完 .sav 重新解压+解析确认可读才提交
- **游戏占用守护**：写入前检查 Palworld.exe/PalServer.exe 进程，运行中阻止并提示
- **网络**：指数退避重试（3-5 次）、断点续传；区分鉴权/配额/网络错误给清晰中文提示
- **密钥存储**：七牛 SK 存 %APPDATA%/PalSaveRelay/config.json；可选 Windows DPAPI 加密（默认明文，用户独占目录）

## 十五、测试策略

- **存档引擎往返测试（金标准）**：本机真实 .sav 夹具，`解压->解析->序列化->压缩` 须字节一致；PlZ 与 PlM 各覆盖。抓住几乎所有移植 bug
- **属性类型单元测试**：每种属性读写互逆
- **房主切换测试**：真实多人存档副本上切换，重新解析断言 UID 已互换、文件仍能往返、属性数量无丢失
- **Storage 测试**：mock 接口；可选七牛真连集成测试（env 凭证开关）
- **打包/解包测试**：zip 往返
- **前端测试**：Playwright 浏览器自动化（build-web-apps 插件支持）
- **夹具**：提交小体积真实 .sav（玩家存档 3-34KB）；较大 Level.sav 用小样本或本地放置

## 十六、数据流（主流程）

### 16.1 接力：A 上传、B 下载并切换
```
玩家A: 主页「上传存档」-> pack 世界文件夹 -> 断点续传上传 saves/<GUID>/<ts>__A.zip
        -> 抢占 lock.json
玩家B: 主页「下载最新」-> List 取最新 -> 断点续传下载 -> 备份本地 -> UnpackWorld 覆盖本地
        -> 查 lock.json（A 在玩，提示）-> 确认接管
        -> 房主切换页 -> 选「我是B」-> SwapHost(备份/解析/互换/校验/落盘)
        -> 释放/抢占 lock.json -> 启动游戏
玩家B 玩完: 上传存档 -> 释放 lock.json
```

### 16.2 房主切换内部
```
SwapHost(worldDir, oldUid, newUid):
  backup(worldDir)
  parse Level.sav, oldPlayer.sav, newPlayer.sav
  swap UIDs (玩家存档/CSPM/公会/深度)
  handle _dps + rename
  serialize + compress -> .tmp
  validate (re-decompress+parse)
  guard: Palworld.exe not running
  atomic rename -> commit
  on any error: rollback backup
```

## 十七、开放问题与风险

| 项 | 说明 | 缓解 |
|----|------|------|
| Wails v3 alpha | API 可能随版本变 | 锁定某 alpha 版不追新；核心风险在存档引擎非 Wails |
| Oodle syscall 加载 | C 函数签名需精确 | 先实现+往返测试验证；回退 go-oodle(CGo) |
| 游玩锁非原子 | 七牛无 CAS，可能同时抢占 | 提示性 + TTL，UI 强提示；可接受 |
| 新玩家不在存档 | 无法直接切换为房主 | UI 提示「先以客人身份进一次世界」 |
| GlobalPalStorage 是否需接力 | 本期排除 | 测试验证；如需再加 |
| 存档格式未来变更 | 游戏更新可能改格式 | 引擎往返测试 + magic 路由设计已隔离变化 |

