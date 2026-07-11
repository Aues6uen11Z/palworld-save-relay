package sav

// PalWorldTypeHints maps property paths to their struct type, so MapProperty
// keys/values parse correctly. Ported from cheahjs palsav/paltypes.py.
var PalWorldTypeHints = map[string]string{
	".worldSaveData.CharacterContainerSaveData.Key":                                                                    "StructProperty",
	".worldSaveData.CharacterSaveParameterMap.Key":                                                                     "StructProperty",
	".worldSaveData.CharacterSaveParameterMap.Value":                                                                   "StructProperty",
	".worldSaveData.FoliageGridSaveDataMap.Key":                                                                        "StructProperty",
	".worldSaveData.FoliageGridSaveDataMap.Value.ModelMap.Value":                                                       "StructProperty",
	".worldSaveData.FoliageGridSaveDataMap.Value.ModelMap.Value.InstanceDataMap.Key":                                   "StructProperty",
	".worldSaveData.FoliageGridSaveDataMap.Value.ModelMap.Value.InstanceDataMap.Value":                                 "StructProperty",
	".worldSaveData.FoliageGridSaveDataMap.Value":                                                                      "StructProperty",
	".worldSaveData.ItemContainerSaveData.Key":                                                                         "StructProperty",
	".worldSaveData.MapObjectSaveData.MapObjectSaveData.ConcreteModel.ModuleMap.Value":                                 "StructProperty",
	".worldSaveData.MapObjectSaveData.MapObjectSaveData.Model.EffectMap.Value":                                         "StructProperty",
	".worldSaveData.MapObjectSpawnerInStageSaveData.Key":                                                               "StructProperty",
	".worldSaveData.MapObjectSpawnerInStageSaveData.Value":                                                             "StructProperty",
	".worldSaveData.MapObjectSpawnerInStageSaveData.Value.SpawnerDataMapByLevelObjectInstanceId.Key":                   "Guid",
	".worldSaveData.MapObjectSpawnerInStageSaveData.Value.SpawnerDataMapByLevelObjectInstanceId.Value":                 "StructProperty",
	".worldSaveData.MapObjectSpawnerInStageSaveData.Value.SpawnerDataMapByLevelObjectInstanceId.Value.ItemMap.Value":   "StructProperty",
	".worldSaveData.WorkSaveData.WorkSaveData.WorkAssignMap.Value":                                                     "StructProperty",
	".worldSaveData.BaseCampSaveData.Key":                                                                              "Guid",
	".worldSaveData.BaseCampSaveData.Value":                                                                            "StructProperty",
	".worldSaveData.BaseCampSaveData.Value.ModuleMap.Value":                                                            "StructProperty",
	".worldSaveData.ItemContainerSaveData.Value":                                                                       "StructProperty",
	".worldSaveData.CharacterContainerSaveData.Value":                                                                  "StructProperty",
	".worldSaveData.GroupSaveDataMap.Key":                                                                              "Guid",
	".worldSaveData.GroupSaveDataMap.Value":                                                                            "StructProperty",
	".worldSaveData.EnemyCampSaveData.EnemyCampStatusMap.Value":                                                        "StructProperty",
	".worldSaveData.DungeonSaveData.DungeonSaveData.MapObjectSaveData.MapObjectSaveData.Model.EffectMap.Value":         "StructProperty",
	".worldSaveData.DungeonSaveData.DungeonSaveData.MapObjectSaveData.MapObjectSaveData.ConcreteModel.ModuleMap.Value": "StructProperty",
	".worldSaveData.InvaderSaveData.Key":                                                                               "Guid",
	".worldSaveData.InvaderSaveData.Value":                                                                             "StructProperty",
	".worldSaveData.OilrigSaveData.OilrigMap.Value":                                                                    "StructProperty",
	".worldSaveData.SupplySaveData.SupplyInfos.Key":                                                                    "Guid",
	".worldSaveData.SupplySaveData.SupplyInfos.Value":                                                                  "StructProperty",
	".worldSaveData.GuildExtraSaveDataMap.Key":                                                                         "Guid",
	".worldSaveData.GuildExtraSaveDataMap.Value":                                                                       "StructProperty",
	".worldSaveData.EnemyCampSaveData.EnemyCampStatusMap.Value.TreasureBoxInfoMapBySpawnerName.Value":                  "StructProperty",
	".worldSaveData.DungeonSaveData.DungeonSaveData.RewardSaveDataMap.Key":                                             "Guid",
	".worldSaveData.DungeonSaveData.DungeonSaveData.RewardSaveDataMap.Value":                                           "StructProperty",
	".SaveData.Local_MaxFriendshipPalIds.Key":                                                                          "PalWorldPlayerUId",
	".SaveData.Local_MaxFriendshipPalIds.Value":                                                                        "StructProperty",
}

// rawdataPaths are the custom RawData property paths. All are passed through
// opaquely here; Phase 2 swaps register real parsers for the ones the
// host-swap needs (CharacterSaveParameterMap.Value.RawData, GroupSaveDataMap).
var rawdataPaths = []string{
	".worldSaveData.GroupSaveDataMap",
	".worldSaveData.CharacterSaveParameterMap.Value.RawData",
	".worldSaveData.ItemContainerSaveData.Value.RawData",
	".worldSaveData.ItemContainerSaveData.Value.Slots.Slots.RawData",
	".worldSaveData.CharacterContainerSaveData.Value.Slots.Slots.RawData",
	".worldSaveData.DynamicItemSaveData.DynamicItemSaveData.RawData",
	".worldSaveData.FoliageGridSaveDataMap.Value.ModelMap.Value.RawData",
	".worldSaveData.FoliageGridSaveDataMap.Value.ModelMap.Value.InstanceDataMap.Value.RawData",
	".worldSaveData.BaseCampSaveData.Value.RawData",
	".worldSaveData.BaseCampSaveData.Value.WorkerDirector.RawData",
	".worldSaveData.BaseCampSaveData.Value.WorkCollection.RawData",
	".worldSaveData.BaseCampSaveData.Value.ModuleMap",
	".worldSaveData.WorkSaveData",
	".worldSaveData.MapObjectSaveData",
	".worldSaveData.GuildExtraSaveDataMap.Value.GuildItemStorage.RawData",
	".worldSaveData.GuildExtraSaveDataMap.Value.Lab.RawData",
}

// PalWorldConfig returns the type hints and custom-property handlers used to
// read/write Palworld saves with all RawData passed through opaquely.
func PalWorldConfig() (map[string]string, map[string]CustomProperty) {
	custom := make(map[string]CustomProperty, len(rawdataPaths))
	skip := CustomProperty{Decode: skipDecode, Encode: skipEncode}
	for _, p := range rawdataPaths {
		custom[p] = skip
	}
	return PalWorldTypeHints, custom
}
