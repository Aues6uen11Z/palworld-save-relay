# 幻兽帕鲁存档完全解析指南

> 基于 PalworldSaveTools、palworld-save-relay 及 cheahjs/palworld-save-tools 源码逆向分析

---

## 一、存档文件格式总览

### 1.1 目录结构

Palworld 合作模式存档位于：

```
%LocalAppData%\Pal\Saved\SaveGames\{SteamID64}\{WorldGUID}\
```

专用服务器存档位于：

```
steamapps\common\Palworld\Pal\Saved\SaveGames\0\{ServerGUID}\
```

一个完整的世界存档目录包含：

| 文件 | 作用 |
|------|------|
| `Level.sav` | **世界核心数据**：所有玩家/帕鲁参数、公会、据点、物品容器、地图对象、任务状态等 |
| `LevelMeta.sav` | 世界元数据：世界名称 (`SaveData.WorldName`) |
| `LocalData.sav` | **个人数据**：本地玩家的任务进度、地图解锁、肖像收集等，不随世界传输 |
| `Players/{UID}.sav` | 每个玩家的个人存档（背包、装备、科技解锁、属性点等） |
| `Players/{UID}_dps.sav` | **次元仓库**（Pal Dimension Storage）：玩家存储在次元仓库中的帕鲁数据 |

### 1.2 SAV 容器格式

每个 `.sav` 文件都是一个 **SAV 容器**，内含压缩的 GVAS 数据。容器头固定 12 字节：

```
偏移  大小  字段
0     4     UncompressedLen  (uint32 LE) — 解压后数据长度
4     4     CompressedLen    (uint32 LE) — 压缩数据长度
8     3     Magic            — 压缩算法标识
11    1     SaveType         — 变体标识
```

三种 Magic 标识：

| Magic | 算法 | 说明 |
|-------|------|------|
| `PlZ` | zlib | SaveType=50 时双重 zlib 压缩，49 时单层 |
| `PlM` | Oodle Kraken | Epic 的专有压缩编解码器（Palworld 默认） |
| `CNK` | 双头部 | 外层包裹第二个 SAV 头部 |

### 1.3 GVAS 内部格式

解压后的数据是 **GVAS** 文件（Unreal Engine 4/5 存档格式），魔数为 `0x53415647`（"GVAS" 小端序）。

GVAS 文件结构：

```
┌─ GvasHeader ─────────────────────────┐
│ Magic: uint32 (GVAS)                 │
│ SaveGameVersion: int32               │
│ PackageFileVersionUE4: int32         │
│ PackageFileVersionUE5: int32         │
│ EngineVersion: Major/Minor/Patch/CL  │
│ EngineVersionBranch: FString         │
│ CustomVersionFormat: int32           │
│ CustomVersions: [(GUID, version)]    │
│ SaveGameClassName: FString           │
├─ Properties ─────────────────────────┤
│ 属性序列 (PropertyList)，以 "None" 结尾│
├─ Trailer ────────────────────────────┤
│ 尾部原始字节                          │
└──────────────────────────────────────┘
```

### 1.4 UE 属性系统

GVAS 文件的核心是 **PropertyList**——一个有序的命名属性序列。每个属性包含：

```
Name:   FString  — 属性名
Type:   FString  — 属性类型
Size:   uint64   — 数据大小
Value:  动态类型  — 根据 Type 解析
```

支持的属性类型：

| 类型 | 存储方式 |
|------|----------|
| `IntProperty` | int32 |
| `UInt32Property` | uint32 |
| `UInt64Property` | uint64 |
| `Int64Property` | int64 |
| `FloatProperty` | float32 |
| `BoolProperty` | byte (0/1) |
| `StrProperty` / `NameProperty` | FString (长度前缀字符串) |
| `EnumProperty` | 枚举类型 + FString 值 |
| `ByteProperty` | 枚举类型 + byte 或 FString |
| `StructProperty` | 结构体类型 + GUID + 值（内嵌属性序列） |
| `ArrayProperty` | 元素类型 + uint32 计数 + 元素数据 |
| `MapProperty` | 键类型 + 值类型 + 键值对数组 |
| `SetProperty` | 元素类型 + 属性列表数组 |

结构体的特殊类型：`Vector` (x,y,z double)、`Guid` (16字节)、`DateTime` (uint64)、`Quat` (四元数)、`LinearColor` (RGBA float)。

---

## 二、Level.sav — 世界数据详解

Level.sav 是存档的核心，包含整个世界的全部状态。其顶层属性 `worldSaveData` 下存储了所有关键数据。

### 2.1 CharacterSaveParameterMap（CSPM）

**角色/帕鲁参数映射表**，是存档中最重要的数据结构。

```
MapProperty: CharacterSaveParameterMap
├── Key: {PlayerUId: GUID, InstanceId: GUID}
└── Value: {
      RawData: ArrayProperty<Byte>
        → 解码后: {
             object: PropertyList  (PalWorldSaveParameter)
             unknown_bytes: [4]
             group_id: GUID
             trailing_bytes: [4]
           }
    }
```

**Key 的两个 GUID：**
- `PlayerUId`：拥有者 UID（玩家或"世界"）
- `InstanceId`：唯一实例标识符

**Value.RawData 解码后的结构：**

| 字段 | 类型 | 含义 |
|------|------|------|
| `object` | PropertyList | 帕鲁/角色的完整参数 |
| `unknown_bytes` | [4]byte | 未知保留字节 |
| `group_id` | GUID | 所属公会 ID |
| `trailing_bytes` | [4]byte | 尾部保留字节 |

**CSPM entry 如何区分玩家和帕鲁？**

通过 `object` 内 `SaveParameter.IsPlayer` 布尔字段判断：
- `IsPlayer = true` → 该条目是玩家角色
- `IsPlayer = false` → 该条目是帕鲁

### 2.2 GroupSaveDataMap（公会/组数据）

公会和组信息存储在 `GroupSaveDataMap` 中，RawData 按 GroupType 分支解析：

#### EPalGroupType::Guild（多人公会）

```python
{
    "group_id": GUID,                          # 公会唯一 ID
    "group_name": FString,                     # 公会名称
    "individual_character_handle_ids": [        # 公会成员列表
        {"guid": UID, "instance_id": InstanceId},
        ...
    ],
    "org_type": byte,                           # 组织类型
    "base_ids": [GUID, ...],                    # 关联据点 ID
    "unknown_1": int32,
    "base_camp_level": int32,                   # 公会据点等级
    "map_object_instance_ids_base_camp_points": [GUID, ...],  # 据点地图对象
    "guild_name": FString,                      # 公会名（重复字段）
    "last_guild_name_modifier_player_uid": GUID,# 最后修改公会名的玩家
    "admin_player_uid": GUID,                   # 管理员/房主 UID
    "players": [                                # 成员详细信息
        {
            "player_uid": GUID,
            "player_info": {
                "last_online_real_time": int64, # 最后在线时间
                "player_name": FString          # 玩家名
            }
        },
        ...
    ]
}
```

#### EPalGroupType::IndependentGuild（独立公会/单人公会）

```python
{
    "group_id": GUID,
    "group_name": FString,
    "individual_character_handle_ids": [...],
    "org_type": byte,
    "base_camp_level": int32,                   # 据点等级
    "guild_name": FString,                      # 公会名
    "player_uid": GUID,                         # 唯一玩家 UID
    "player_info": {
        "last_online_real_time": int64,
        "player_name": FString
    }
}
```

#### EPalGroupType::Organization（组织/临时组）

```python
{
    "group_id": GUID,
    "group_name": FString,
    "individual_character_handle_ids": [...],
    "org_type": byte,
    "trailing_bytes": [12]
}
```

### 2.3 BaseCampSaveData（据点营地数据）

每个据点存储为 MapProperty，Key 为 GUID，Value 为 StructProperty 包含：

| 字段 | 含义 |
|------|------|
| RawData | 据点核心数据（不透明二进制） |
| WorkerDirector.RawData | 工人调度数据 |
| WorkCollection.RawData | 工作收集数据 |
| ModuleMap | 功能模块映射 |

### 2.4 ItemContainerSaveData（物品容器数据）

存储背包、箱子、据点仓库等所有容器：

```
MapProperty: ItemContainerSaveData
├── Key: StructProperty (容器标识)
└── Value: {
      RawData: 不透明字节 (容器内容)
      Slots.Slots.RawData: 槽位详细数据
    }
```

### 2.5 CharacterContainerSaveData（角色容器数据）

存储帕鲁箱（Palbox）、出战帕鲁（Otomo）等容器的**槽位引用**：

```
MapProperty: CharacterContainerSaveData
├── Key: {ID: GUID}  — 容器唯一标识
└── Value: {
      bReferenceSlot: bool
      SlotNum: int       — 最大槽位数
      Slots: [
        {
          SlotIndex: int  — 槽位编号
          RawData: [42]byte — 不透明 blob：
            [0:4]   uint32  数据长度
            [4:20]  GUID    PlayerUId（所属玩家）
            [20:36] GUID    InstanceId（帕鲁实例 ID）
            [36:42] 未知字节
          CustomVersionData: [...]
        },
        ...
      ]
      RawData: [] — 容器级数据（大部分为空）
    }
```

**注意：** CharacterContainerSaveData 只存储**槽位引用**（PlayerUId + InstanceId），实际帕鲁数据在 CSPM 中。游戏通过 `(PlayerUId, InstanceId)` 匹配来解析容器中的帕鲁。

### 2.6 其他世界数据

| 数据结构 | 作用 |
|----------|------|
| `FoliageGridSaveDataMap` | 采集过的植被网格 |
| `MapObjectSaveData` | 地图上所有可交互对象 |
| `MapObjectSpawnerInStageSaveData` | 地图对象生成器状态 |
| `WorkSaveData` | 工作分配数据 |
| `DungeonSaveData` | 地城数据（宝箱、奖励） |
| `InvaderSaveData` | 入侵者事件数据 |
| `OilrigSaveData` | 石油钻井平台数据 |
| `SupplySaveData` | 空投补给数据 |
| `GuildExtraSaveDataMap` | 公会额外数据（实验室研究等） |
| `EnemyCampSaveData` | 敌人营地状态 |

---

## 三、玩家存档（Players/UID.sav）

每个玩家的个人存档保存在 `Players/{UID}.sav` 文件中，结构与 Level.sav 相同（SAV → GVAS → PropertyList），但内容不同。

UID 的命名规则是 `SteamID64` 的十六进制大写表示（如 `76561198012345678.sav`）。

**特殊 UID — 房主哨兵：**

```
00000000-0000-0000-0000-000000000001
（16字节中只有第12字节=1，其余全零）
```

合作模式中，房主始终使用 `0001.sav`，客人使用各自 SteamID 派生的 UID 文件。

### 3.1 次元仓库（Pal Dimension Storage）

次元仓库数据存储在独立的 `_dps.sav` 文件中，**不在 Level.sav 的 CharacterContainerSaveData 里**。

```
Players/{PlayerUID}_dps.sav
```

**数据结构：**

```
SaveParameterArray (ArrayProperty)
  type_name: PalDimensionPalStorageSaveParameter
  └── 每个 entry:
        SaveParameter: {
          CharacterID: NameProperty     // 帕鲁种类（如 "Anubis", "GrassBoss"）
          Level: ByteProperty           // 等级
          OwnerPlayerUId: StructProperty // 拥有者 UID
          IsPlayer: BoolProperty        // false
          ... (与 CSPM 中帕鲁完全相同的字段)
        }
        InstanceId: {
          PlayerUId: GUID               // 所属玩家 UID
          InstanceId: GUID              // 唯一实例 ID
          DebugName: StrProperty
        }
```

**关键点：**
- 每个玩家有自己的 `_dps.sav`，次元仓库是**按玩家独立存储**的
- 最大 9600 个 slot（96×100）
- 空 slot 的 `CharacterID` 为 `None`，所有字段为零值
- 与 Level.sav 中的 `CharacterContainerSaveData`（帕鲁箱、Otomo 等容器）是**完全独立**的数据结构

**与 Level.sav 容器的关系：**

Level.sav 中的 `CharacterContainerSaveData` 存储帕鲁箱（Palbox）、出战帕鲁（Otomo）等容器的**槽位引用**（PlayerUId + InstanceId 的 RawData blob），而次元仓库的实际帕鲁数据在 `_dps.sav` 中。玩家存档中的 `PalStorageContainerId` 指向 Level.sav 中的容器 ID，但次元仓库不使用这个机制。

### 3.2 玩家存档关键字段

| 字段 | 类型 | 说明 |
|------|------|------|
| `PlayerUId` | GUID | 玩家唯一 ID |
| `IndividualId` | Struct | 玩家实例标识（PlayerUId + InstanceId） |
| `OtomoCharacterContainerId` | Struct | 出战帕鲁容器 ID（指向 Level.sav） |
| `PalStorageContainerId` | Struct | 帕鲁箱容器 ID（指向 Level.sav） |
| `InventoryInfo` | Struct | 背包容器 ID（武器、装备、食物等多个容器） |
| `UnlockedRecipeTechnologyNames` | Array | 已解锁的科技列表 |
| `RecordData` | Struct | 游戏记录（捕获数、帕鲁图鉴、传送点解锁等） |

---

## 四、UID 系统与 SteamID 转换

### 4.1 SteamID → PlayerUID

Palworld 使用 Google CityHash64 从 SteamID64 派生玩家 UID：

```
1. 将 SteamID64 转为十进制字符串
2. 编码为 UTF-16-LE 字节
3. 计算 CityHash64 哈希
4. 取低 32 位和高 32 位: lo = hash & 0xFFFFFFFF, hi = hash >> 32
5. 混合: r = lo + hi * 23
6. 将 r 写入 16 字节 UUID 的前 4 字节，后 12 字节填充 0
```

### 4.2 GUID 格式

Palworld 使用**混合端序（mixed-endian）**格式显示 GUID：

```
标准 UUID:   12345678-1234-1234-1234-123456789ABC
Palworld:    87654321-4321-4321-4321-CBA987654321
```

### 4.3 房主 UID

合作模式的房主使用固定的哨兵 UID：

```
00000000-0000-0000-0000-000000000001
（16字节中只有第12字节=1，其余全零）
```

所有世界数据（帕鲁、建筑等）在合作模式下都归属于这个哨兵槽位。

---

## 五、帕鲁 (Pal) 完整字段解析

每个帕鲁的数据存储在 CSPM 的一个条目中，IsPlayer=false。通过解码 RawData → object → SaveParameter 可获取所有字段：

### 5.1 基础属性

| 字段 | 类型 | 范围 | 说明 |
|------|------|------|------|
| `Level` | byte | 1-80 | 帕鲁等级 |
| `HP` / `MaxHP` | int | — | 当前/最大生命值 |
| `Attack` | int | — | 攻击力 |
| `Defense` | int | — | 防御力 |
| `Stamina` | int | — | 耐力 |
| `WorkSpeed` | int | — | 工作速度 |
| `Weight` | int | — | 负重 |
| `IsPlayer` | bool | — | 始终为 false |

### 5.2 IVs（个体值）

| 字段 | 范围 | 说明 |
|------|------|------|
| `Talent_HP` | 0-100 | 生命值个体值 |
| `Talent_Attack` | 0-100 | 攻击力个体值 |
| `Talent_Defense` | 0-100 | 防御力个体值 |

### 5.3 灵魂 (Souls)

通过「神秘力量」系统提升的额外属性：

| 字段 | 范围 | 说明 |
|------|------|------|
| `RideAbility_Lv_HP` | 0-20 | 生命值灵魂等级 |
| `RideAbility_Lv_Attack` | 0-20 | 攻击力灵魂等级 |
| `RideAbility_Lv_Defense` | 0-20 | 防御力灵魂等级 |
| `RideAbility_Lv_WorkSpeed` | 0-20 | 工作速度灵魂等级 |

### 5.4 技能 (Skills)

| 字段 | 说明 |
|------|------|
| `EquipWaza` | 装备的主动技能列表（最多 4 个） |
| `HaveWaza` | 已学会的技能列表 |

### 5.5 被动特性 (Passives)

| 字段 | 说明 |
|------|------|
| `PassiveDataList` | 被动特性列表（如 "ElementBoost_Bug_Ice"） |

常见被动特性包括：
- 攻击加成类：`Attack_up_lv1/2/3`
- 属性加成类：`ElementBoost_Fire_Ice` 等
- 速度类：`MoveSpeed_up_lv1/2`
- 工作类：`WorkSpeed_up_lv1/2`

### 5.6 工作适应性 (Work Suitability)

| 字段 | 范围 | 说明 |
|------|------|------|
| `WorkSuitability` | 0-10 | 各项工作能力等级 |

工作类型：手工制作、采伐、种植、发电、制药、冷却、搬运、采矿、牧场

### 5.7 外观/特殊标志

| 字段 | 类型 | 说明 |
|------|------|------|
| `IsBoss` / `IsAlpha` | bool | Boss/Alpha 帕鲁标志 |
| `IsLucky` | bool | 幸运（闪光）帕鲁 |
| `IsPredator` | bool | 掠食者标志 |
| `IsAwakened` | bool | 觉醒状态 |
| `IsImported` / `IsDNA` | bool | 导入/DNA 标志 |

### 5.8 其他字段

| 字段 | 说明 |
|------|------|
| `NickName` | 自定义昵称 |
| `OwnerPlayerUId` | 拥有者玩家 UID |
| `OldOwnerPlayerUIds` | 历史拥有者 UID 数组 |
| `LastNickNameModifierPlayerUid` | 最后修改昵称的玩家 |
| `IndividualCharacterHandleId` | 个体角色句柄（guid + instance_id） |
| `group_id` | 所属公会 ID（在 RawData 解码后） |
| `CondenserRank` | 浓缩等级（0-4） |
| `Rank` | 帕鲁等级（0-3） |
| `FavoriteLockLevel` | 收藏锁定等级（0-3） |

---

## 六、玩家角色字段解析

玩家角色 IsPlayer=true，同样在 CSPM 中存储，关键字段：

### 6.1 玩家状态

| 字段 | 说明 |
|------|------|
| `NickName` | 角色昵称 |
| `Level` | 角色等级 |
| `HP` / `MaxHP` | 生命值 |
| `MaxStamina` | 最大耐力 |
| `Attack` | 攻击力 |
| `Defense` | 防御力 |
| `WorkSpeed` | 工作速度 |
| `MaxInventoryWeight` | 最大负重 |

### 6.2 玩家统计

| 字段 | 说明 |
|------|------|
| `CaptureCount` | 捕获帕鲁总数 |
| `CaptureCountUnique` | 捕获独立种类数 |
| `DeathCount` | 死亡次数 |
| `PlayTime` | 游玩时间 |

### 6.3 玩家背包

玩家背包数据存储在 `ItemContainerSaveData` 中，关联到玩家的 CSPM 条目。包含：

- 主背包物品列表
- 装备槽位（武器、护甲、饰品、食物、盾牌、滑翔伞、模块）
- 关键物品（肖像、科技解锁）

---

## 七、常见问题

### Q: `struct.error` 解析错误

存档格式过期。在游戏内加载一次存档触发自动格式升级后重试。

### Q: 为什么 LocalData.sav 不传输？

LocalData.sav 是个人任务/地图进度，每个玩家独立维护。合作模式传输时故意排除，各人保留各人的进度。

### Q: 次元仓库和帕鲁箱有什么区别？

- **帕鲁箱（Palbox）**：数据在 Level.sav 的 `CharacterContainerSaveData` 中，是世界数据的一部分，随世界传输
- **次元仓库**：数据在 `Players/{UID}_dps.sav` 中，是玩家个人数据，按玩家独立存储
- **出战帕鲁（Otomo）**：数据在 Level.sav 的 `CharacterContainerSaveData` 中，也是世界数据

### Q: 合作模式下各玩家的数据边界是什么？

| 数据类型 | 存储位置 | 是否随世界传输 |
|----------|----------|----------------|
| 帕鲁参数（CSPM） | Level.sav | 是 |
| 公会数据 | Level.sav | 是 |
| 据点/建筑 | Level.sav | 是 |
| 帕鲁箱/Otomo 槽位 | Level.sav | 是 |
| 次元仓库 | Players/{UID}_dps.sav | 否（按玩家独立） |
| 玩家背包/装备 | Players/{UID}.sav | 否（按玩家独立） |
| 地图解锁/任务进度 | LocalData.sav | 否（按玩家独立） |

---

## 参考资料

- [PalworldSaveTools](https://github.com/deafdudecomputers/PalworldSaveTools) — 综合存档编辑工具
- [palworld-save-relay](https://github.com/Aues6uen11Z/palworld-save-relay) — 换房主专用工具
- [cheahjs/palworld-save-tools](https://github.com/cheahjs/palworld-save-tools) — 原始存档格式解析器
- [new-world-tools/go-oodle](https://github.com/new-world-tools/go-oodle) — Go Oodle 绑定
