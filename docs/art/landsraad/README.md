# Landsraad pictures (reference copies)

Pictures for designing the Landsraad tab, kept in one place so anyone working on the page can use them. They are Dune: Awakening game artwork that belongs to Funcom, taken from the game's own interface pictures. They are **not** covered by this repository's MIT licence; see [`docs/art-notice.md`](../../art-notice.md). The editor does not load anything from this folder: the pictures the page serves are in `internal/web/static/` (the house symbols in `internal/web/static/houses/`, 64 px). The Command Center parts have their own optimized copies in `internal/web/static/lcc/art/` (webp, listed with hashes in [`art-manifest.json`](../../design/landsraad-command-center/art-manifest.json)); this folder stays the full-size reference.

All files are transparent PNG, one picture each.

| Folder | What | Size | Files |
|---|---|---|---|
| `houses/` | the symbol of each Landsraad house | 128 x 128 | 25: `alexin`, `argosaz`, `dyvetz`, `ecaz`, `hagal`, `hurata`, `imota`, `kenola`, `lindaren`, `maros`, `mikarrol`, `moritani`, `mutelli`, `novebruns`, `richese`, `sor`, `spinette`, `taligari`, `thorvald`, `tseida`, `varota`, `vernius`, `wallach`, `wayku`, `wydras` |
| `lanes/` | the scroll icon of each mission lane (specialization) | 256 x 256 | 5: `combat`, `crafting`, `exploration`, `gathering`, `sabotage` |
| `banners/` | the specialization path banner icon, same five names | 256 x 256 | 5: `combat`, `crafting`, `exploration`, `gathering`, `sabotage` |
| `factions/` | the two faction crests: the white crest and the round medallion of each | white 512 x 512; medallions 100 x 100 (Atreides) and 104 x 104 (Harkonnen) | 4: `atreides`, `harkonnen`, `atreides-medallion`, `harkonnen-medallion` |
| `decrees/` | the picture of each decree kind | 256 x 256 | 15: `choam`, `dagger`, `exclusive-import-rights`, `honor-of-the-fallen`, `loot`, `production-contract`, `repair`, `repair-cost-reduction`, `scholarly-collaboration`, `vendor-armor`, `vendor-utilities`, `vendor-vehicles`, `vendor-weapons`, `weapon-rifle`, `xp` |

Notes:
- The house names follow the game's spelling (`mikarrol`); the shipped 64 px copy of that house is `mikkarol.webp`.
- The decree pictures are named by what the decree gives, not by the game's file names, so a decree's picture has to be matched by hand.
- There is no `vendor-special` picture: the plain special-vendor decree has none in the game's interface pictures, so the pages show a neutral picture for it (`internal/web/static/lcc/art/decrees/fallback.svg`, authored for this repository, not game art).
- The faction crests are a separate set from the houses and the path icons and are never mixed with them: the white crest is used on the plaque's shields, standards and a claimed 13th seat, the medallion on a claimed seat. Which is used where is in [`05-m2-visual-foundation.md`](../../design/landsraad-command-center/05-m2-visual-foundation.md).
- Everything is at most the size the game stores it at, scaled down to the sizes above.
