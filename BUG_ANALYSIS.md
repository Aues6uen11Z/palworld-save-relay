# 鸟举帕鲁 Bug 根因分析与修复

## 问题现象

A 房主的存档通过工具转换为中间态，B 下载后切换为房主，进入游戏后**无法举起帕鲁**（lift）。

## 根因

### 核心原因：Guild `group_name` 字段未被转换

Guild 的 RawData 解码后有一个 `group_name` 字段，值是**房主哨兵 UID 的字符串格式**（如 `"00000000000000000000000000000001"`）。

游戏引擎通过此字段识别"谁是当前房主"。转换时 `deepReplace` 只能处理 UUID 类型的字段，而 `group_name` 是 string 类型，所以**完全被遗漏了**。

转换后 `group_name` 仍然指向旧房主的哨兵 UID，导致游戏引擎在验证帕鲁举起权限时，认为当前玩家不是房主，从而拒绝举起操作。

### 次要因素（已验证不需要额外修复）

- **`_u8_flag`**（host=1/guest=2）：guild RawData 的 encode/decode 循环已经自动处理了交换
- **CSPM Key PlayerUId**：需要被 `deepReplace` 转换（哨兵→真实UID），游戏引擎会正确处理
- **NickName / FilteredNickName**：字符串类型，跟随 player_uid，不需要转换

## 修复方法

### 新增 `fixGuildAfterConversion` 函数

在 `convertFile`（deepReplace）之后调用，读取 Level.sav，修正 `group_name`：

```
group_name: 从 hex(fromUID) → hex(toUID)
```

其中 hex 使用的是 Palworld 的混合端序 UUID 字符串格式（去掉横杠）。

### 代码位置

- `internal/palworld/hostswap.go`: `fixGuildAfterConversion` 函数
- `internal/palworld/hostswap.go`: `convertHostImpl` 中调用

### 修复涉及的完整字段清单

| 字段 | 类型 | 转换方式 | 是否已修复 |
|------|------|---------|-----------|
| PlayerUId (CSPM key) | *UUID | deepReplace | ✅ 已有 |
| OwnerPlayerUId | *UUID | deepReplace | ✅ 已有 |
| guid (ICH) | *UUID | deepReplace | ✅ 已有 |
| OldOwnerPlayerUIds | []UUID | deepReplace | ✅ 已有 |
| LastNickNameModifierPlayerUid | *UUID | deepReplace | ✅ 已有 |
| admin_player_uid | *UUID | deepReplace | ✅ 已有 |
| player_uid | *UUID | deepReplace | ✅ 已有 |
| last_guild_name_modifier_player_uid | *UUID | deepReplace | ✅ 已有 |
| **group_name** | **string** | **fixGuildAfterConversion** | **✅ 本次修复** |
| _u8_flag | uint8 | guild encode 自动处理 | ✅ 无需修复 |

## 验证方法

使用 `正常初始存档.zip`（2人小存档）模拟转换，验证：
1. `group_name` 从哨兵 hex → 真实 UID hex ✅
2. `_u8_flag` 自动交换（旧房主=2，新房主=1）✅
3. CSPM key PlayerUId 分布正确 ✅
4. ICH 分布正确 ✅

使用 `正常存档.zip`（4人存档）生成中间态，用户导入后测试举起帕鲁 → **成功** ✅
