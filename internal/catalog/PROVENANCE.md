# Where the data in this folder comes from

| File | What it is | Source | Added |
|---|---|---|---|
| `items.json` | Template id, in-game name, category, source and required DLC of each item | The item catalog of the Dune Docker console (RedBlink, MIT licence), `runtime/data/admin-items.json`; see `internal/notices/THIRD-PARTY-NOTICES.md` ("Item catalog") for the commit, the blob and the licence text | #69, #122 |
| `stacks.json` | For 221 item template ids, the most of the item one inventory slot holds (`{"max": {"<template id>": n}}`) | The game's own item tables: the base item rows, `StackAndDurability.MaxStackSize`. Only items that stack are listed | #240, 2026-10-22 |
| `customization_unlocks.json` | For 392 cosmetic item template ids, the customization ids that using the item unlocks (625 entries, 614 distinct ids) | The game's own item table `DT_ItemTableCustomizations`, `CustomizationItemStats.UnlockableCustomizations` | #216, 2026-10-20 |

`stacks.json` and `customization_unlocks.json` contain only the game's identifiers and integers: no names, text, images or code. They are
read by `catalog.go` (`MaxStack`, `Unlocks`) and nothing else.

**Game build.** The installed game's Steam manifest (`appmanifest_1172710.acf`, read on 2026-10-22) names build `20502675`, last updated by
Steam on 2025-10-23 09:07 UTC, with the target build equal to the installed one. Both files were added after that date (2026-10-20 and
2026-10-22), and the game had no later update, so they were read from build `20502675`. The tables are read from the installed game's content
files, read-only; nothing in the game is changed by reading them.

**Not recorded in the repository** (so not stated here): the tool and the exact files used to read the tables, and any licence or terms-of-service
position on using them. The notice for these two files is in `internal/notices/THIRD-PARTY-NOTICES.md` ("Stack sizes and cosmetic unlock ids").
The game and its data belong to Funcom and this project is unofficial. Whoever refreshes either file should add the game build, the tool and the
files used to this table in the same pull request.
