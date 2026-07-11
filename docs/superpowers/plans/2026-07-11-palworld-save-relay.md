# Pal Save Relay Implementation Plan

> **执行方式**：按任务顺序实现，每个任务先写测试再写实现（TDD），代码在实现阶段写、不预先写进本计划。移植任务对照 Python 源（已 clone 在 `D:\Code\PalworldSaveTools`）逐函数翻译。

**Goal:** Windows 单 exe 的 Palworld 合作存档接力工具，UID 切换房主 + 七牛云同步。

**Architecture:** 纯 Go 存档引擎（移植 cheahjs）解析 SAV/GVAS；Oodle 经 go-oodle（purego 免 CGo）+ 内嵌 DLL；七牛对象存储做无服务器 P2P 接力；Wails v3 绑定 Go 方法到 React 前端。

**Tech Stack:** Go 1.22+ / Wails v3(锁 alpha) / React18+TS+Vite+shadcn/ui+Tailwind+Zustand / `qiniu/go-sdk/v7` / `new-world-tools/go-oodle`。

**Spec:** `docs/superpowers/specs/2026-07-11-palworld-save-relay-design.md`

---

## 关键设计决策（贯穿全程）

1. **属性用有序 `PropertyList`**（slice 保序），不用 `map`——Go map 无序，无法保证 sav→gvas→sav 字节一致往返（Python 靠 dict 插入序）。
2. **GUID 全程用原始 16 字节 `UUID`**，房主切换按原始字节比较——绕开 Palworld 非标准混合端序；混合端序 `String()` 仅用于 UI 展示。
3. **SAV 写回保留原格式**（magic+type），不跨 PlZ/PlM 转换，避免游戏拒载坏档。
4. **未知 RawData 原样字节透传**（skip_decode/encode），只解析房主切换需要的 CharacterSaveParameterMap 和 GroupSaveDataMap。
5. **所有改档操作事务化**：备份→临时副本改→校验→原子 rename 落盘，失败回滚。

## 文件结构

```
palworld-save-relay/
  main.go                  # Wails 入口、服务/托盘注册(预留)
  app.go                   # Wails bindings 层
  go.mod
  internal/
    sav/                   # 存档引擎(Phase 1)
      container.go oodle.go gvas.go archive.go properties.go
      rawdata.go rawdata_character.go rawdata_group.go paltypes.go
      assets/oo2core_9_win64.dll   testdata/*.sav
    palworld/              # 域逻辑(Phase 2): detect.go hostswap.go backup.go pack.go
    storage/               # 云存储(Phase 3): storage.go qiniu.go lock.go
    config/config.go       # 配置(Phase 4)
  frontend/                # React(Phase 5)
  resources/               # 图标等
  docs/superpowers/{specs,plans}/
```

## 测试夹具（真实存档，Phase 1 Task 1 拷入 `internal/sav/testdata/`）
- `level_plz.sav` / `player_plz.sav` ← `6E8DEA2A` 世界（PlZ/zlib）
- `level_plm.sav` / `player_plm.sav` ← `47CB2F89/backup/world/2026.07.11-15.47.46/`（PlM/Oodle，3 玩家，从 backup 快照取避免游戏占用）
- `localdata_plm.sav` ← `6424B6CA/backup/local/2026.07.11-16.12.28/LocalData.sav`（PlM 小样本）

---

# Phase 1：存档引擎（internal/sav/，纯 Go，`go test` 独立可测）

> 移植源：`D:\Code\PalworldSaveTools\src\palsav\palsav\`（archive.py / gvas.py / compressor/ / rawdata/ / paltypes.py）+ `src/palobject.py`(skip_decode)。

### Task 1.1：初始化模块 + 拷贝夹具
- **建**：`go.mod`（module `palworld-save-relay`）、`internal/sav/testdata/*.sav`
- **做**：`go mod init`；按上文路径拷贝 5 个夹具；写个一次性脚本验证 magic（PlZ/PlM 各覆盖）
- **验收**：testdata 下 5 文件，magic 正确

### Task 1.2：SAV 容器 + Oodle
- **建**：`internal/sav/container.go`、`oodle.go`
- **移植**：`compressor/__init__.py`(_parse_sav_header/build_sav/check_sav_format)、`compressor/zlib.py`、`compressor/oozlib.py`、`compressor/enums.py`
- **Oodle**：依赖 `new-world-tools/go-oodle`；`//go:embed assets/oo2core_9_win64.dll`，首次用释放到 `os.TempDir()/go-oodle/`（go-oodle 默认查找路径）；DLL 经 `go-oodle.Download()` 获取
- **API**：`ParseSAVHeader`、`Decompress([]byte)(gvas,hdr,err)`、`Compress(gvas,hdr)([]byte,err)`，保留原 magic+type
- **验收**：PlZ 与 PlM 夹具各做 `解压→压缩→再解压 == 原 gvas 字节`（压缩输出不必字节相同，内容须一致）

### Task 1.3：FArchive 原语 + UUID + GVAS 头 + 有序 PropertyList
- **建**：`archive.go`、`gvas.go`
- **移植**：`archive.py` 的 FArchiveReader/Writer 原语（fstring 含 UTF-16、i32/u32/i64/u64/float/double/bool/byte/u16/i16/guid/optional_guid/tarray/read/eof）、`gvas.py` 的 GvasHeader/GvasFile
- **决策**：UUID=[16]byte+混合端序 String()；PropertyList=有序 slice；CustomProperty=(Decode,Encode) 注册表
- **验收**：原语往返（ascii/utf16 字符串、整数、GUID）；真实夹具解压后读 GVAS 头→写回头→字节一致

### Task 1.4：UE 属性系统
- **建**：`properties.go`；改 `gvas.go` 加 ReadGvasFile/WriteGvasFile
- **移植**：`archive.py` 的 property/properties_until_end/struct/struct_value/array_property/array_value/map_property/set_property/prop_value 及对应 writer + `_READ/_WRITE_PROPERTY_DISPATCH`
- **验收**：合成属性树（含 Int/Str/Bool/Struct(Guid)/Array/Map）`序列化→解析→序列化` 字节一致

### Task 1.5：RawData 解析 + 注册表
- **建**：`rawdata.go`(skip 透传)、`rawdata_character.go`、`rawdata_group.go`、`paltypes.go`
- **移植**：`palobject.py` 的 skip_decode/encode（透传原字节）；`rawdata/character.py`(decode/encode_bytes)；`rawdata/group.py`(decode/decode_bytes/encode_bytes，含 Guild/IndependentGuild/Organization 分支与 V1_MARKER)；`paltypes.py` 的 PALWORLD_TYPE_HINTS 与自定义属性注册表（character/group 用真解析，其余路径注册 skip）
- **验收**：真实 `level_plm.sav`（含 RawData）`解压→解析→序列化→压缩` 与原文件经 `解压` 后的 gvas 字节一致

### Task 1.6：金标准往返测试
- **建**：`internal/sav/roundtrip_test.go`
- **做**：所有 5 个夹具做 `sav→Decompress→ReadGvasFile→Write→gvas2`，断言 `gvas==gvas2`（字节级，覆盖 PlZ+PlM）；另做 `sav→解析→序列化→sav2→解析→序列化` 两跳稳定性
- **验收**：全部通过——这是「不坏档」的核心保证

---

# Phase 2：Palworld 域逻辑（internal/palworld/）

> 移植源：`PalworldSaveTools/src/palworld_toolsets/fix_host_save.py`（fix_save）；`zaigie/palworld-server-tool` 的数据模型可参考。

### Task 2.1：检测与列表（detect.go）
- **做**：存档根 `%LOCALAPPDATA%\Pal\Saved\SaveGames\`；列 SteamID→WorldGUID 世界（含 Level.sav 即世界）；每世界展示 GUID/修改时间/玩家数/别名/隐藏(存 config)；列玩家：解析 Level.sav 的 CharacterSaveParameterMap 取 IsPlayer（UID/昵称/等级/最后在线），标记当前房主(UID 0000…0001)
- **API**：`DetectSaveRoot`、`ListWorlds`、`ListPlayers(worldDir)`
- **验收**：对本机 3 个世界正确列出世界与玩家

### Task 2.2：房主切换（hostswap.go，核心）
- **移植**：`fix_host_save.fix_save`（UID 全局互换）
- **做**：备份→解析 Level.sav+主机玩家.sav+目标玩家.sav→互换(玩家存档 PlayerUId/IndividualId；CSPM 按 InstanceId 换 PlayerUId；公会 GroupSaveDataMap 换 admin/player/handle guid；深度换 OwnerPlayerUId 等 4 字段)→处理 _dps + 重命名玩家 .sav→序列化压缩→落盘前重新解压校验→原子 rename；失败回滚
- **old/new UID 来源**：从玩家 .sav 的 SaveData.PlayerUId 读原始 16 字节（不从文件名推），按原始字节比较
- **前置约束**：目标玩家须已存在于存档，否则 UI 提示「先以客人身份进一次世界」
- **验收**：在 `47CB2F89` 快照副本上切换，重新解析断言 UID 已互换、文件仍能往返、属性数量无丢失

### Task 2.3：备份管理（backup.go）
- **做**：触发(切换/下载覆盖/导入覆盖前)；存 `%APPDATA%/PalSaveRelay/backups/<worldGUID>/<ts>.zip`；保留最近 N(默认5)；列表/回滚/删除
- **验收**：切换前后自动产生备份，回滚能还原

### Task 2.4：打包/解包（pack.go）
- **做**：`PackWorld(dir)->zip` / `UnpackWorld(zip,dest)`，打包整个世界文件夹排除 `backup/`；云同步与导入导出共用
- **验收**：打包→解包→结构与原一致（排除 backup/）

---

# Phase 3：云存储（internal/storage/）

### Task 3.1：Storage 接口（storage.go）
- **API**：`Upload/Download/List/Delete/Get/Put`；Object{Key,Uploader,Size,LastModified}
- **验收**：mock 实现跑通接口契约测试

### Task 3.2：七牛实现（qiniu.go）
- **做**：`qiniu/go-sdk/v7`；分片续传上传、range 分块续传下载(临时 .part 记偏移)；key 方案 `saves/<worldGUID>/<unixmillis>__<uploader>.zip`，不维护 latest 指针（List 取最新）；重试退避
- **验收**：真连测试桶（env 凭证开关）上传/下载/列举/删除

### Task 3.3：游玩锁（lock.go）
- **做**：`saves/<worldGUID>/lock.json`={player,acquiredAt}；抢占前查，TTL(默认6h)内提示「X 在玩，强制接管?」；过期视为空闲；玩完 Delete；提示性(七牛无 CAS)
- **验收**：抢锁/查锁/释放/过期判定逻辑测试

---

# Phase 4：应用外壳（config + Wails v3）

### Task 4.1：配置（config/config.go）
- **做**：`%APPDATA%/PalSaveRelay/config.json`（七牛 AK/SK/Bucket/Region、别名、隐藏存档、上传者名、版本保留数、锁TTL）；SK 可选 DPAPI
- **验收**：读写往返、字段齐全

### Task 4.2：Wails v3 脚手架（main.go, app.go）
- **做**：`wails3` 初始化（锁某 alpha 版）；`app.go` bindings 暴露高层方法（DetectSaves/ListPlayers/SwapHost/Upload/Download/ListVersions/ClaimLock/ReleaseLock/ListBackups/RestoreBackup/Import/Export/SaveConfig）；长操作事件回传进度；托盘注册预留(空)
- **验收**：`wails3 dev` 起窗，前端能调一个 binding 拿到世界列表

### Task 4.3：游戏占用守护 + DLL 运行时释放
- **做**：写入前查 `Palworld.exe`/`PalServer.exe` 进程，运行中阻止并提示；启动释放内嵌 Oodle DLL 到 temp
- **验收**：游戏运行时切换被拦截

---

# Phase 5：前端（React + shadcn/ui）

### Task 5.1：脚手架
- **做**：Vite+React+TS+shadcn/ui+Tailwind+Zustand；Wails bindings 的 TS 封装
- **验收**：`wails3 dev` 显示带路由的空壳

### Task 5.2：页面（逐页）
- 引导页（七牛配置+确认存档目录）/ 主页（世界信息+锁状态+三按钮）/ 房主切换（世界列表→玩家列表→选我→进度）/ 云同步（上传下载+版本历史+锁）/ 备份管理 / 导入导出 / 设置（测试连接）
- **验收**：每页接通对应 binding，长操作显示进度

---

# Phase 6：集成与打包

### Task 6.1：单 exe 打包
- **做**：`wails3 build` 出单 exe；确认 Oodle DLL 内嵌(generate 包含)；资源/图标
- **验收**：裸机双击 exe 可运行

### Task 6.2：端到端接力流
- **做**：A 上传→B 下载→B 切换房主→B 上传 全链路在真桶跑通；游玩锁正确提示
- **验收**：接力后存档可正常进游戏、房主已切换

### Task 6.3：文档
- **做**：README 使用说明、首次引导、常见问题（新玩家不在存档等）
- **验收**：非技术玩家按文档能完成接力

---

## 自查（实现前对照 spec）
- [ ] 房主切换覆盖 CSPM/公会/深度 4 字段/玩家存档重命名 (spec 七) — Task 2.2
- [ ] PlZ+PlM 双格式保留 (spec 三.3) — Task 1.2/1.6
- [ ] 断点续传 (spec 九.4) — Task 3.2
- [ ] 提示性游玩锁+TTL (spec 十) — Task 3.3
- [ ] 事务化+落盘前校验+游戏占用守护 (spec 十四) — Task 2.2/4.3
- [ ] 导入导出兜底 (spec 十二) — Task 2.4/5.2
- [ ] 单 exe 零安装 (spec 二) — Task 6.1
