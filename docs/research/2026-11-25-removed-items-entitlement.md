# Dune: Awakening single-player: building sets and emotes removed by the entitlement check

Timeline and evidence from one single-player character. Compiled 2026-11-25. Unofficial; not affiliated with or endorsed by Funcom.

## Summary

- **Reason:** the account did not own these items (no entitlement), and the game's entitlement check removed them. This is the conclusion from the evidence below; no log line says that an item was removed.
- On 2026-11-05, **77 building sets and 11 emotes** disappeared from the character in one event. All 77 set ids begin with `MTX_`. No sets or emotes were added at the same time.
- The items had been given earlier with a save editor (tabr-tau), according to the first report. The save file itself does not record who granted an item.
- It happened around a world load. The game's log from that session showed its own reward check failing (`Entitlement check failed`) for building sets and customizations.
- After the items were given again, **all 77 sets and all 11 emotes were still there on 2026-11-25**.
- **Not known:** what triggered the check at that load, or why 92 other `MTX_` sets were kept.

## Timeline

Local time is UTC-8. Source "Backups" means re-checked for this document; "record" means notes written at the time, which can no longer be checked.

| Date and time (local) | What happened | Source |
|---|---|---|
| 10-20 | Two cosmetics added straight to the customization library as a test. After the game loaded the save, the cosmetic `MTX_Atreides_Battlerifle` was gone (159 ids became 158). The plain swatch `MWpnDyepackAtre` stayed. | [tabr-tau #217](https://github.com/Project-Arrakis/tabr-tau/issues/217) |
| 11-02 20:39 to 11-05 12:24 | 19 backup copies checked: every one holds 236 building sets (169 `MTX_`) and 24 emotes. | Backups |
| 11-05 11:34 | Start of the game session in which the loss happened. | Game log (record) |
| 11-05 12:24:30 | Last backup with everything present. | Backups |
| 11-05, between 12:24 and 13:09 | A world load, then the loss. The failed-check lines began about 1 minute 15 seconds after the load and ran for about 90 seconds. | Backups, record |
| 11-05 13:09 | Loss reported. | Record |
| 11-05 13:11:11 | First backup after the loss: **159 sets (92 `MTX_`), 13 emotes**. | Backups |
| 11-05 13:13 | Emotes given again with the editor, game closed. | Record |
| 11-05 14:53:58 | Backup shows 236 sets and 24 emotes again. | Backups |
| 11-06 | Several later backups still hold 236 sets and 24 emotes. One more failed check was logged, for a set the character did not have; nothing was removed. | Backups, log (record) |
| 11-16 | Review of later backups: all 77 sets and 11 emotes still present. | Record |
| 11-25 20:49 | Latest backup: 245 sets (178 `MTX_`), 24 emotes, **77 of 77 and 11 of 11 still present**. | Backups |

## Evidence

### 1. Save backups (re-checked 2026-11-25)

Each backup is a copy of the single-player save. Counts come from the table `building_progression_learned_building_sets` and from `items` rows whose `template_id` starts with `Emote`. Only copies were read.

| Backup file (name is UTC) | Sets | `MTX_` sets | Emotes |
|---|---|---|---|
| `game-20261105-202430Z-137566900-auto.db` (last before) | 236 | 169 | 24 |
| `game-20261105-211111Z-854508300-auto.db` (first after) | 159 | 92 | 13 |
| `game-20261105-225358Z-015593200-auto.db` (after re-give) | 236 | 169 | 24 |
| `game-20261126-044942Z-753975100-auto.db` (latest) | 245 | 178 | 24 |

SHA-256 of those files, in the same order:

```
11fb4607fa62f8b4bb19cbffb3b5dfcf8057d2c11dec291efb9fca19cfb5a8a4  game-20261105-202430Z-137566900-auto.db
9e35e018e1a6bb898cdaade5276a6ca30b259acdd097aa239d793ba0adf09a0f  game-20261105-211111Z-854508300-auto.db
f58090116ce214bf5e12cc612e1cc7ea8c2160a3decfb7c062411725beccefb2  game-20261105-225358Z-015593200-auto.db
56607ee367fe906a1657ea7452d0a05cfe6c0ebca64e57583b6d52f9c972ba49  game-20261126-044942Z-753975100-auto.db
```

### 2. The game's own save copy (record)

The first report says the copy the game takes at the start of a world load (`game_prepatch.db`) still held all 236 sets, and the game's own save a few minutes later held 159. The editor's last write had been about 40 minutes earlier, and its backup from that moment also had all 236. That copy has since been overwritten.

### 3. Game log (record)

The log of the 11-05 session is **no longer on disk**. The lines and counts below were recorded from it on 11-05 and 11-06 and cannot be re-checked.

- `LogPlayerRewards: Warning: Entitlement check failed ... RewardType: ERewardAction::BuildingSet`: 506 lines over the whole session, naming 7 ids; 478 of them named `MTX_Neut_HarvesterCaptainChair_Placeable`. (The first report, written during the session, counted 420.)
- The same check with `ERewardAction::Customization`: 34 lines naming 17 ids.
- In the same session: `Get Player Rewards call failed ... 403 ACCESS_DENIED`, naming the local single-player world.
- The eight game logs from 11-02 up to that session had no `Entitlement check failed` line.
- Game version: 1.5.15.0 (Dreamworld 2173753), the same as in the session before.

### 4. The refused rewards call in later sessions (checked 2026-11-25)

Logs still on disk show this line (shortened; the local world name is removed):

```
LogPlayerRewards: Error: Get Player Rewards call failed: GetPlayerRewardsServer Failed:
https://sb-retail.fls.funcom.com/api/PlayerRewards_GetPlayerRewardsServer_V2
"HttpCode":403, "ErrorName":"ACCESS_DENIED" ... Could not find service authorization
information for Battlegroup: LOCAL-<removed>
```

It appears in four sessions (11-24 22:33, 11-25 12:09, 13:32 and 20:12 local). None of the 11 logs on disk has an `Entitlement check failed` line, and the 11-25 20:49 backup still holds all 77 sets and 11 emotes. The logs do not show whether the entitlement check ran in those sessions.

## What this shows, and what it does not

**Shown by the save copies**

- The loss happened once in the window checked, within a 47 minute gap between two backups.
- It removed only `MTX_` building sets (77 of 169) and 11 of 24 emotes.
- Giving them again held for at least 20 days.

**Conclusion (inferred)**

- The account was not entitled to the removed items, and the game's entitlement check removed them. A save editor can add an item to a save, but it cannot make the account own it. The log shows the check failing; it does not show a line that says an item was removed.

**Unknown**

- What part, if any, the refused rewards call played on 11-05.
- Whether the check will remove items again. A failed check does not always remove anything.
- Why 92 `MTX_` sets were kept. Nothing in the save separates them from the removed ones: no entitlement column, no flag, no record of the grant source.
- Whether cosmetics in the customization library were removed on 11-05 (34 checks failed; the library was not compared).
- Whether the same happens to other accounts or on other game versions.

## If this happens to you

- Keep a backup of your save before giving yourself anything.
- A save editor cannot grant a DLC or an account entitlement. The game can take such items back.
- To report it, note the time, your game version and the `Entitlement check failed` lines from `DuneSandbox.log` **before** the game rotates the log away. Remove account ids and your Windows user name before sharing a log.

Related: [tabr-tau #801](https://github.com/Project-Arrakis/tabr-tau/issues/801) (mark these ids in the editor; open).

## Appendix: removed ids

### Building sets (77)

```
MTX_Atre_BreakfastRoomSet_Patent
MTX_Atre_Hologram_CaladanEmblem_Patent
MTX_Atre_Movie_Bench_Patent
MTX_Atre_Movie_Bonsai_Patent
MTX_Atre_Movie_BullStatue_Patent
MTX_Atre_Movie_Cabinet_Patent
MTX_Atre_Movie_CaladanCastle_Hologram_Patent
MTX_Atre_Movie_Chair_Patent
MTX_Atre_Movie_DecorativeBox_Patent
MTX_Atre_Movie_DecorativePlate_Patent
MTX_Atre_Movie_DecorativeWineBottle_Patent
MTX_Atre_Movie_Dresser_Patent
MTX_Atre_Movie_FoodPlatter_01_Patent
MTX_Atre_Movie_Glass_01_Patent
MTX_Atre_Movie_Glass_02_Patent
MTX_Atre_Movie_LargeCarpet_Patent
MTX_Atre_Movie_Painting_Patent
MTX_Atre_Movie_PlateCutlery_Patent
MTX_Atre_Movie_Table_Patent
MTX_Atre_Movie_Vase_01_Patent
MTX_Atre_Movie_WaterPitcher_Patent
MTX_Atre_Movie_WindChimes_Patent
MTX_Atre_TeaSet_Patent
MTX_Atre_TeaSet_Patent2
MTX_Atre_TeaSet_Patent3
MTX_Atre_TeaSet_Patent4
MTX_Atre_Troopship_Relief_Placeable_Patent
MTX_Atreides_MovieGlowglobe_Patent
MTX_ChoamExtention_Column_Patent
MTX_ChoamExtention_Floor_01_Patent
MTX_ChoamExtention_Passageway_01_Patent
MTX_ChoamExtention_Roof_01_Patent
MTX_ChoamExtention_WallRoundCorner_Patent
MTX_ChoamExtention_Wall_01_Patent
MTX_ChoamExtention_Wall_02_Patent
MTX_ChoamExtention_Wall_03_Patent
MTX_Neut_ChristmasTree_Hologram_Patent
MTX_Neut_DesertMechanicSet02_Patent
MTX_Neut_DesertMechanicSet_Glowglobe_Patent
MTX_Neut_DesertMechanicSet_Patent
MTX_Neut_DesertMechanicSet_Projector_Patent
MTX_Neut_DesertMechanicSet_Toolbox_Patent
MTX_Neut_GunnerSet_Patent
MTX_Neut_GunnerSet_Patent_02
MTX_Neut_HarvesterCaptainChair_Placeable
MTX_Neut_Miniature_AtreidesTroops_Patent
MTX_Neut_Miniature_HarkonnenTroops_Patent
MTX_Neut_Miniature_Harvester_Patent
MTX_Neut_Miniature_Ornithopter_Patent
MTX_Neut_Miniature_Sandbike_Patent
MTX_Neut_Miniature_SpiceField_Patent
MTX_Neut_Miniature_StrategyFlag_Patent
MTX_Neut_Miniature_Transport_Patent
MTX_Neut_Miniature_Worm_Patent
MTX_Neut_MuadDibCage_Patent
MTX_Neut_MuralPainting_01_Patent
MTX_Neut_MuralPainting_02_Patent
MTX_Neut_Statue_Bat_Patent
MTX_Neut_Statues_DesertMouse_Patent
MTX_Neut_Statues_KulonDonkey_Patent
MTX_Neut_Statues_LargeSandworm_Patent
MTX_Neut_StrategyRoomCarpet_Patent
MTX_Neut_StrategyTable_Patent
MTX_Neut_Strategy_RoomSign_Patent
MTX_Neut_TeleporterDevice_Placeable
MTX_ReverendMotherRoom_Banner_Patent
MTX_ReverendMotherRoom_Carpet_Patent
MTX_ReverendMotherRoom_Chair_01_Patent
MTX_ReverendMotherRoom_Chair_02_Patent
MTX_ReverendMotherRoom_TableRound_Patent
MTX_ReverendMotherRoom_Table_Patent
MTX_SardaukarDecorationSet_Patent
MTX_Sardaukar_BuildingSet_Patent
MTX_Smug_BuildingSet_Patent
MTX_Smug_BuildingSet_Placeables_Patent
MTX_WaterShippers_BuildingSet_Patent
MTX_Watershippers_Placeables_Patent
```

### Emotes (11)

```
Emote_ExamineGround
Emote_Sardaukar_BloodWipe
Emote_Sardaukar_Kneel
Emote_ScanHorizon
Emote_ThumperBack
Emote_ThumperForward
Emote_ThumperLeft
Emote_ThumperRight
Emote_WatershipperFreeze
Emote_WatershipperSalute
Emote_WormSign
```
