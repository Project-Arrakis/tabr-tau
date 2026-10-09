# Player tabs: what the console shows, and what tabr-tau builds

Date: 2026-10-08. Issue: #96 (with #94 for Bases). Status: spec; slices are built one PR at a time.

Operator decision: the Player tab mirrors the Dune Docker console's **Players > Player Name** view, tab for tab. Each tab holds what the
console's tab holds. The console is for a dedicated server and tabr-tau is for a single-player save, so some actions drop out; the
default is to keep a control, not cut it.

Source: console v1.4.47 (`console/web/src/features/players/*`, `features/bases/*`, `features/vehicles/*`, API in `console/api/src`),
read from its code and rendered locally against fake data. Rule for "applies to single-player" below: the console itself talks only
to Postgres and runs live actions over RabbitMQ, neither of which exists in a single-player save. What matters is whether the **same
data lives in the SQLite save**, which was checked read-only against a real save on 2026-10-08 (counts in the last column).

Status: **Have** (in tabr-tau), **Build** (data is in the save; direct edit), **Cut** (server or multiplayer only), **Later** (applies,
needs more research or a live check in game).

## Player Summary header (shown above the tabs)

| Console shows | Source in the save | Real save check | tabr-tau |
|---|---|---|---|
| Character name, status (Online / Offline), map, guild | `player_state`, `actors`; guild does not exist in single-player | name, map present | Build (guild shown as "-") |
| Level, XP | `fgl_entities.components` `FLevelComponent[1].TotalXPEarned`, slot `DuneCharacter`; level from the console's 201-entry XP table | XP 41,221 | Build |
| Skill points (unspent / total) | `FLevelComponent[1].UnspentSkillPoints`, `TotalSkillPoints` | 9 / 70 | Build |
| Available Intel (cap 2,779) | `actors.properties` `TechKnowledgePlayerComponent.m_TechKnowledgePoints` | 333 | Build |
| Health (current / estimated max), Hydration, Spice Addiction | `FHealthComponent[1].m_CurrentHealth`; `actors.gas_attributes` Hydration and SpiceAddiction sets; max health = 150 + Vitality tiers from the Combat specialization level (always shown as estimated) | health 112.3 | Build |
| Faction alignment, faction standings (reputation, estimated rank, story limit) | `player_faction`, `player_faction_reputation`, `factions`, `journey_story_node`, `player_tags` | tables present | Have reputation; Build alignment; rank estimate Later |
| Platform, Funcom, FLS ID | `accounts` | present | Have (shown today) |
| DB player, account, controller, player-state ids | `actors`, `player_state` | present | Build |
| Currency tiles: Solari Credit, Solari Coin, secondary currency | `player_virtual_currency_balances` (credit); `SolarisCoin` item stacks (coin) | credit 591,227; coin 95,470 | Build (both tiles; they are different balances) |

## Character

| Console | Single-player data | tabr-tau |
|---|---|---|
| Quick Rewards: Give Water (10 water packs), Refill Container | give the item row; fill the container fill-state (have) | Have refill containers; Build give water |
| Give XP | edit `FLevelComponent` XP (console does it live over RabbitMQ; here a save edit) | Build (needs in-game verification) |
| Give Currency (Solari Credit / secondary) | `player_virtual_currency_balances` | Build |
| Give Intel (clamped to the 2,779 cap) | `actors.properties` path above | Build |
| Give Faction Reputation (0 to 12,474) | `player_faction_reputation` | Have (set reputation) |
| Give Items: picker, quantity, grade 0-5, augments + augment grade, a queue, "Give Package" | item rows; catalog picker | Have single give; Build queue, augments |
| Inventory (N): group tabs Backpack / Character / Loadout / Unique schematics; filter; columns id, name, stack, grade, slot, durability, max durability, augments; edit, apply augments, delete | `inventories`, `items` (inventory types 0 / 1 / 15 / 30) | Have list and edit / delete; Build group tabs, filter, augments column and apply |
| Character Action Log (last 25, client-side only) | none | Have the equivalent: Review & save lists every edit |

## Crafting, Research, Building Sets, Customizations

| Tab | Console | Single-player data | tabr-tau |
|---|---|---|---|
| Crafting | Crafting Schematics table (recipe, recipe id, source, grade, Unlock), filter, category rail (Essentials, Water Discipline, Combat, Construction, Exploration, Vehicles), reload; 383+ schematics from the console's item catalog | `actors.properties` `CraftingRecipesLibraryActorComponent.m_KnownItemRecipes` (173 known) | Build |
| Research | Research table (name, item key, type, product group, Research / Repair Unlock), filters, category rail incl. Augmentations / Uniques | `TechKnowledgePlayerComponent.m_TechKnowledge.m_TechKnowledgeData` (365 entries, 229 Purchased); unlock also adds the linked recipe or building set | Build |
| Building Sets | Table of 213 building-set unlocks with group, requires, status; the console delivers a patent token item rather than writing the unlock | learned building sets / buildable pieces tables | Have the learned list; Build the Grant (item delivery or direct unlock, to decide) |
| Customizations | 342 cosmetics in 5 sets (Atreides, Harkonnen, Smuggler, Dune Man, Filmic Archive); Grant / Grant Set / Grant All; delivers token items | item rows (none held now) | Build as item delivery; entitlement or DLC cosmetics are only delivered tokens, ownership cannot be verified |

Catalogs used by the console (MIT, RedBlink) and already embedded in tabr-tau's notices model: `admin-items.json` (items, schematics,
building sets, customizations), `admin-skill-modules.json` (145 skill modules), `journey-tags.json`. Names, categories and groups are
the console's own regex heuristics, not game data; keep that note.

## Skills, Specialization, Journey, Blueprints

| Tab | Console | Single-player data | tabr-tau |
|---|---|---|---|
| Skills | Skill Point Controls (set unspent points); Skill Browser by school (Trooper, Mentat, Planetologist, Bene Gesserit, Swordmaster) and tree, rank bars per skill, Save / Discard, Restore Starter Skills presets | `FLevelComponent[1].ModuleData` (146 modules; `SkillPointsSpent` is cumulative, rank comes from the point ladder in `admin-skill-modules.json`) | Build |
| Specialization | Table Track, XP, Level, Keystone, Add XP, Grant Max, Reset; Grant / Reset All Keystones | `specialization_tracks` (empty in this save), `specialization_keystones_map`, `purchased_specialization_keystones` | Have track set; Build keystones, Add XP, Grant Max, Reset |
| Journey | Journey Browser: Story, Contracts, Codex, Tutorial trees, filter, Complete / Reset per node, status, tags, depends-on | `journey_story_node` (2,221 nodes), `player_tags`, `tutorials` (118), `tutorial_per_player` | Have complete / reset; Build tree view, depends-on, contracts, codex |
| Blueprints | Player Blueprints (import, export, delete, select all) and Browse community blueprints (online catalog) | `building_blueprints` and 3 child tables (empty in this save) | Build player blueprints; Cut community browse (needs the internet; revisit) |

## Bases (console: embedded Bases panel, issue #94)

Base list columns: ID, Base Name, Base Type, Owner, Shared With, Map, Generators, Building Pieces, Placeables, Coordinates. Row actions:
Refill Generators, Refill Water (Download, Delete and permission actions: **cut**). Expanded row tabs:

| Tab | Console | tabr-tau |
|---|---|---|
| Power | per generator / windtrap type: count, fuel queued, lowest reserve, none-queued count; Auto-Refill switch | Have refill; Build the table and the auto-refill switch (have, at editor start) |
| Water | per container type: count, water stored / capacity, fill %, blood volume; auto-refill | Have refill; Build the table |
| Inventory | summary tiles, group chips (Storage, Refining, Crafting, Other), items / containers view, filter, container contents (list / grid), add, delete, give, fill | Storage list done (#75); Build the rest |
| Land Claim Editor | clickable grid of cells, vertical level, apply | Have expand and shrink by size; Build the grid |
| Sub-Fief Permissions, Base Permissions | roster editing | **Cut** (single owner) |

tabr-tau has base features the console's row does not (structure health, repair all, clear sand, placeables, piece types): they go to
the **Extras** tab, not into a console tab.

## Vehicles

Console: list of the player's vehicles (Vehicle, Type, Lowest Condition %, Fuel %, Location), expandable component cards, **View
Contents** (cargo hold, slot list or grid, delete stack / selected / all). Access filter and Owner column are moot for one owner.

tabr-tau: owned vehicles only (#92, owner = rank 1 in `permission_actor_rank`; the real save has 13 vehicles, 1 owned). Have bring, repair,
chassis durability. Build: Type, condition %, fuel %, location columns; components; cargo hold view and delete (`inventory_type = 0`, never
the module link). Refuel exists in the console API but has no screen there: `actors.properties[<class>].m_InitialFuel = 1.0`.

## Admin

| Console | tabr-tau |
|---|---|
| Faction Assignment (Neutral / Atreides / Harkonnen) | Build (`player_faction`) |
| Repair Faction, Repair Landsraad Quests | Later (verify the nodes exist in a single-player save) |
| Repair Gear | Have (moved to Admin) |
| Repair Vehicle Durability with "repair below %" | Have repair all; Build the threshold |
| Wipe Inventory, Reset Progression | Later (the console runs these live; a save edit needs a design) |
| Recover Deleted Character, Repair Login Queue, Kick, Ban / Unban | **Cut** (server, accounts, RabbitMQ) |
| Teleport To: Coordinates / Another Player / Player Base | Have coordinates, base, respawn point (#67); other player is moot |
| Spawn Vehicle (vehicle + template tiers) | Later (the game must create the vehicle correctly) |

## Live Map (its own console page, requested 2026-10-08)

Console: pannable, zoomable square maps of Hagga Basin and the Deep Desert with markers read straight from the database and refreshed every
5 seconds (static atlas once a minute): players, vehicles, bases, storage, spice fields, resources and points of interest; a partition
select; layer toggles and filters; a compass; teleport the player from the map.

For tabr-tau the data comes from the **save file**, so it shows the character, the player's own vehicles, the base, its storage
containers and any spice fields recorded in the save, as of the last save (not live positions). Two open questions:

- **Map images.** The console ships about 82 MB of map images and terrain meshes made from the game's own files, and its NOTICE says not
  to redistribute Funcom's proprietary assets. tabr-tau (public) will not bundle them. Options: draw markers on a plain coordinate grid
  with the map bounds; or read the map imagery from the player's own installed game at run time (read-only, nothing copied into the repo;
  needs a licence / ToS check first, same as #61).
- **Static atlas of resources and points of interest.** Where its coordinates come from (the console's data files) needs the same check.

Build after the Player tabs. Not part of the Player tabs.

## Order of work

1. Player Summary header and Character (read-only first, then Give XP / Currency / Intel, queue, augments).
2. Admin to match (Faction Assignment, repair threshold, then the Later items).
3. Bases (#94): Power, Water, Inventory, Land Claim grid.
4. Vehicles: columns, components, cargo.
5. Specialization, Journey, Skills.
6. Crafting, Research, Building Sets, Customizations.
7. Blueprints.
8. Live Map (from the save; map imagery decision first).

Every write is verified the same way as before: the value read back from the save after the game's next flush, and a check in game. New
writes go through the review pane and the existing pipeline (single-player check, backup, integrity check, read-back).
