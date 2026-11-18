# Command Center M2: the visual foundation

Milestone M2 of the Landsraad and Specialization Command Center: the approved V20/V21 look as scoped styles, small parts, local art and a fixture gallery. Nothing here is wired to the editor. The Landsraad and Specialization tabs are unchanged, no route, setting, save schema or write path was added, and no demo value reaches production code.

Read with `DECISIONS.md` (D05, D10, D11, D14, D16, D20), `art-manifest.json` and `docs/art-notice.md`.

## 1. What was built

| Part | Where |
|---|---|
| Helpers: art lookups, seat model, parts, three renderers | `internal/web/static/lcc/lcc.js` (no network, no storage, DOM built with `createElement` and `textContent` only) |
| Scoped styles | `internal/web/static/lcc/lcc.css` (every selector starts at a `lcc` class) |
| Optimized local art | `internal/web/static/lcc/art/{houses,lanes,banners,decrees,factions}/` (webp, about 430 KiB in all) |
| Art manifest (hashes, sizes, source, decree matching) | `docs/design/landsraad-command-center/art-manifest.json` |
| Component gallery (fixture data) | `docs/design/landsraad-command-center/gallery/` |
| Serving | two file names and five art folders added to the fixed lists in `internal/web/server.go` (see 5) |
| Capture script | `scripts/lcc-gallery-shots.mjs` |
| Tests | `internal/web/lcc_assets_test.go`, `web-tests/lcc-*.test.mjs` |

The shared `design/` and `kit/` modules were not touched: they hold no game art, and nothing in M2 needs a change to them (D14, D17).

## 2. Component token map

Every colour, face and motion value is named once, in the alias block at the top of `lcc.css`, on `.lcc, .lcc-scope`. Nothing outside that block names a design token, and no custom property outside `--lcc-*` is defined.

| Alias | Value | Used for |
|---|---|---|
| `--lcc-ground`, `-panel`, `-raised`, `-sunken` | `--c-ground`, `--c-panel`, `--c-raised`, `--c-sunken` | card faces, frame inside, tooltip, badge |
| `--lcc-text`, `--lcc-dim` | `--c-text`, `--c-dim` | body and quiet text |
| `--lcc-line` | `--c-bronze`, else `--c-rule` (the Sietch look defines no bronze) | ochre divider, badge edge |
| `--lcc-hair` | `--c-hair` | dividers |
| `--lcc-focus` | `--c-focus` | every focus ring |
| `--lcc-gold-text` | `--c-gold` | gold used as text (the look's own sand, so it passes contrast in every theme) |
| `--lcc-gold`, `-hi`, `-lo` | constants `#e3b24d`, `#ffe9a8`, `#8e5f1c`; Dayside `#8a5a10`, `#b07a1c`, `#5e3c08` | frames, rules, rings, nameplates |
| `--lcc-on-gold`, `--lcc-ink` | `#2a1a05` (Dayside on-gold `#fff6df`) | letters on a gold fill |
| `--lcc-atreides`, `-deep`, `--lcc-harkonnen`, `-deep` | `#12a381`, `#0b5a47`, `#d23a40`, `#7a1118` | faction identity only: never a theme colour |
| `--lcc-on-faction`, `--lcc-cream` | `#fdf6e6`, `#f4e6c3` | words on cloth and on the plaque |
| `--lcc-field` | `#0a0b1e` | the plaque's own field |
| `--lcc-mo` | `--mo-slow` | the one reveal |
| `--lcc-v` | set from script as a percent | the lane bar fill (a CSSOM write, so the content security policy needs no inline style) |

The plaque is the approved dark artwork in both themes, so its golds are pinned on `.lcc-plaque`; the page around it follows Dark or Dayside, the three looks (Fremen, Awakening, Sietch) and the palettes. Motion follows the `data-motion` setting (full, calm, off) and `prefers-reduced-motion`; the only animation is one reveal, and nothing loops.

| Component | Class | Notes |
|---|---|---|
| Cut-corner gold frame | `lcc-frame` | clip-path, no image |
| Gold nameplate | `lcc-pill` | pointed ends |
| HELD BY with its vertical rule | `lcc-heldby`, `lcc-rule-v` | one element for the rule |
| Cloth standard (A, H) | `lcc-standard` | up to five lines, "Not in the save" when there are none |
| Badges | `lcc-badge`, `-demo`, `-ref` | FIXTURE and READ-ONLY are words, not colours |
| Reachable tooltip | `lcc-tt` | real text, `aria-describedby`, shown on hover and focus |
| Voting plaque | `lcc-plaque-host` > `lcc-plaque` | container-query layout, wide from 940 px, narrow below, stacked head under 520 px |
| Lane card | `lcc-lane` | one native button; XP cap and keystones are two separate tags |
| House card | `lcc-house` | one native button; owner in words and a mark, `data-victory` is one outline |

## 3. The plaque

* A vector underlay in a 2172 x 724 coordinate system (the size of the approved artwork), a track, and DOM text over them in percent with `cqw` sizes. It is built in vector form, not from the approved screenshot, so it is crisp at any width and the words are real text.
* 25 seats at an equal pitch with an equal inset at both ends. Seat 13 (index 12) is always the gold middle seat with its own gold border and halo; it takes the colour and mark of its owner when claimed and stays open and gold otherwise (D16). Atreides fills from the left, Harkonnen from the right. A count that cannot be read (negative, fractional, above 25, or together above 25) lights nothing and says "unavailable".
* Wording follows the one wording table (DECISIONS section G; the operator's decision of 2026-11-18 (#929, #926, #931) supersedes the earlier rule that no word says a House or a faction has won, and the #912 and #915 headline): the plaque says "SYSSELRAAD WON · <FACTION>" when a line of five or 13 Houses decides it, "SYSSELRAAD CONTESTED" with a count line such as "ATREIDES LEAD 12 TO 4 · 9 HOUSES OPEN" otherwise, and "SYSSELRAAD UNDECIDABLE" when both qualify. The old headline "VOTING SESSION · MAJORITY AT 13 OF 25 HOUSES" is dropped; seat 13 stays marked on the track.
* The vertical rule between the decree and HELD BY is one element drawn after everything else in the underlay, so nothing paints over it (a test checks that there is exactly one and that it is last). Straight gold lines are thin rectangles, because a gradient sized by the box of a zero-height line does not draw. The line between a nameplate and its count is gold and carries the diamond at its centre (a test checks it).
* The narrow layout is a crop of the same artwork, not a second track: one set of seats in the DOM.

## 4. Art

Inventory (`art-manifest.json` has name, bytes, SHA-256, size and source of each file):

| Role | Files | Size | Note |
|---|---|---|---|
| House symbols | 25 | 128 x 128 | `mikkarol.webp` is the one spelling in the folder |
| Lane scrolls | 5 | 256 x 256 | Specializations |
| Path banners | 5 | **256 x 256** | the reference README said 512; the files are 256, the README is corrected |
| Decree pictures | 9 + 1 | 128 x 128 | hand-matched, see below; plus the neutral `fallback.svg` |
| Faction crests | 2 white + 2 medallions | 256 x 256 (white), 100 and 104 (medallions) | Atreides and Harkonnen, a separate set (see 4.1); reference PNGs in `docs/art/landsraad/factions/` |

Lookups (`houseAsset`, `laneAsset`, `bannerAsset`, `decreeAsset`, `factionAsset`) take a string, lower-case and trim it, and look it up in a fixed table by own-property check. A path, an extension, a traversal, a non-string, a prototype name and anything not in the table gives `null` (or, for a decree, the neutral picture). No file name is built from a name. The art base is checked to be a path on this origin.

Decisions recorded here:

* **Spelling alias (D10).** The save and the tracker say Mikkarol, the reference art says mikarrol. The table has one explicit entry `mikarrol` to `mikkarol`; both spellings, with or without the `DA_House` prefix, find the one file.
* **Missing `vendor-special` (D11).** The reference folder has no such file. The plain special-vendor decree has no picture, and so does an unknown id: both get `decrees/fallback.svg`, an authored neutral diamond (no text, link or script; the only file in the art folder that is not game art). There is never a 404.
* **Decree matching is by hand**, with the basis recorded in the manifest. Four SpecialVendorActive decrees match by name (vehicles, weapons, armor, utilities), as do the experience and crafting-cost decrees; ranged and melee damage, the defeat-drop and the repair decrees match by subject. Five reference pictures (`choam`, `dagger`, `exclusive-import-rights`, `weapon-rifle`, `xp`) match no decree of the editor's table and are not shipped to the page.
* **Licence (D14).** The pictures are Funcom game art outside the repository licence. Optimized local copies are served from the embedded folder only; the page loads nothing from a network. `docs/art-notice.md` names the new folder, the neutral exception and the manifest.

### 4.1 The faction crests

The two faction crests replace the vector marks the first M2 build drew itself (a hawk-like V and a ring with a spur, labelled as original marks). They are the crests of the approved package: a white crest per faction and a round medallion per faction, Dune: Awakening artwork that belongs to Funcom (the same notice as the other art). They are a separate set: `factionAsset(faction, kind)` takes `'a'`, `'h'`, `'Atreides'` or `'Harkonnen'` and the kind `'mark'` (white crest, the default) or `'medal'`, and it always answers; anything else gets `{ key: null, file: null, src: '', fallback: true }` and no picture is drawn. The House and path tables never find a crest and `factionAsset` never finds a House.

Where each picture is used, matched to the approved V20 plaque and its stylesheet (the shields and the standards are in the V20 picture itself, the seats are its `ib-token` circles):

| Place | V20 | This build |
|---|---|---|
| Header shields (left Atreides, right Harkonnen) | white crest | `factions/atreides.webp`, `factions/harkonnen.webp`, an SVG `<image>` in the decorative layer (120 units) |
| Cloth standards | white crest | the same two white crests (76 units in the wide layout; a 44 px `<img>` in the narrow stack) |
| A claimed seat (not the 13th) | the round medallion (ring and white crest) | `factions/atreides-medallion.webp`, `factions/harkonnen-medallion.webp` over the seat circle |
| The 13th seat when claimed | the white crest over the gold hub | the white crest (44 units) over the seat circle, which keeps its gold border and halo |
| An open seat | the round unclaimed medallion | not used: the drawn bronze seat stays (the unclaimed medallion holds no faction mark and is not shipped) |

Fallbacks and cues: every picture is decorative (`alt=""` on the HTML ones, the SVG layer is `aria-hidden`) and the text beside it names the faction. The drawn seat mark (a V for Atreides, a ring for Harkonnen) stays under each claimed seat as the fallback and is hidden by the stylesheet only after the picture has loaded; a picture that fails to load draws nothing, so the shield or cloth colour, the nameplates and the counts stand alone. In forced colours the pictures are dropped and the factions differ by the drawn mark, the seat fill and, on the stacked standards, two plain shapes (not the crests). Colour is never the only cue.

## 5. Serving, and why the gallery is not part of the editor

The embedded `static/lcc/lcc.css` and `lcc.js` and the five art folders are added to the fixed lists in `internal/web/server.go` (a request path is only ever compared with the list built from the embedded folders; `.svg` is added to the folder content types for the one neutral picture). `TestEveryEmbeddedAssetIsServedWithItsOwnBytes` and the new tests check each file is served with its own bytes and a known type, and that traversal and upper-case spellings answer 404.

The gallery lives under `docs/`, so it is not embedded, not routed and not linked from the editor page; a test checks that no editor file mentions it and that none of its fixture values appear in the helpers, the styles or the page script. It is a static page that works from any loopback static server of the repository folder, has its own content security policy (`default-src 'none'; script-src 'self'; style-src 'self'; img-src 'self'; font-src 'self'`), and says FIXTURE / DEMO in the header and in every section heading and plaque. Its settings are read from the address and only ever compared with fixed lists.

To look at it: serve the repository root on 127.0.0.1 with any static server, open `/docs/design/landsraad-command-center/gallery/index.html`; `?scene=plaques|lanes|houses|primitives|art`, `?scene=plaque-a12h4`, `look=fremen|awakening|sietch`, `theme=system|dark|light`, `palette=...`, `motion=full|calm|off`.

## 6. Visual acceptance checklist

Each item is to be looked at on the gallery at the widths 390, 768, 1280, 1600 and at 200 percent zoom, in Dark and Dayside. The boxes are for the reviewer: the builder looked at the headless captures, which is not a sign-off, and no box is ticked here.

Geometry and state

- [ ] 25 seats, equal pitch, equal inset; seat 13 gold with halo in every case, including 0 and 0, and 13 and 12.
- [ ] Atreides from the left, Harkonnen from the right; the 13th takes its owner's colour and mark.
- [ ] Counts that cannot be read light nothing and say so.
- [ ] Cut-corner gold frame, header band, pointed track tips, cloth standards with their rods.
- [ ] Wording: the table of DECISIONS section G (SYSSELRAAD WON / CONTESTED / UNDECIDABLE; was "VOTING SESSION · MAJORITY AT 13 OF 25 HOUSES" until 2026-11-18), "Not in the save" for contributors.

The five risky crops (a side-by-side with the approved reference is captured for each; see 7)

| Crop | What must hold | File |
|---|---|---|
| Left point | clean pointed tip with the gold edge, the inner line, no seat touching the wall | `LCC_M2_crop_left-point_dark_2x`, `LCC_M2_compare_left-point` |
| Right point | the same, mirrored, in the Harkonnen colour | `LCC_M2_crop_right-point_dark_2x`, `LCC_M2_compare_right-point` |
| Center gold circle and diamonds | gold 13th seat with its ring, the diamond above and the pointer below, no overlap with a neighbouring seat | `LCC_M2_crop_center-gold_dark_2x`, `LCC_M2_compare_center-gold` |
| Footer House nameplate, line, diamond, count | the gold line runs **between** the nameplate and the count and carries the hollow diamond at its end; the count sits under the line | `LCC_M2_crop_footer_dark_2x`, `LCC_M2_compare_footer` |
| Top HELD BY vertical divider | one unbroken gold rule between the decree and HELD BY; nothing paints over it | `LCC_M2_crop_held-by-divider_dark_2x`, `LCC_M2_compare_held-by-divider` |

Cards and primitives

- [ ] Lane cards at level 0, 60 and 100, with keystones 0 of 41 and 41 of 41: the XP cap and the keystones are two tags; "XP cap reached" is never "mastered"; an unknown keystone total is never complete.
- [ ] House cards: open, Atreides claimed, Harkonnen claimed, one away, five in a line, majority, owner unreadable; state is text and a mark as well as colour; one outline at most.
- [ ] A missing picture hides itself and leaves the name and the state.

Looks, themes, motion, access

- [ ] Fremen, Awakening, Sietch; Dark and Dayside; the plaque keeps its own field in both.
- [ ] Motion full, calm, off; reduced motion; nothing loops.
- [ ] Forced colours: the decoration is dropped, the factions still differ by fill and by shape, the selected card has a 3 px outline.
- [ ] 200 percent zoom: no horizontal scroll of the page, no clipped text.
- [ ] Contrast of the plaque words 4.5:1 on its field (a test computes it); focus rings visible; every card is a native button with a full name.
- [ ] No console error, no failed request, no request to another host (the capture report records them).

## 7. Result, and what was not done

Automated (all run for the pull request; counts are in its description): the Go asset tests (`internal/web`), 48 node tests in `web-tests/lcc-*.test.mjs` (art lookups and allowlist, alias, traversal and case, seat model and the 25 seats with the gold 13th, cards, CSS isolation and tokens, forced colours, contrast, gallery), and the existing `internal/web`, `internal/hygiene`, `design` and `kit` groups.

Captured headlessly (no window was shown), outside the repository, with `scripts/lcc-gallery-shots.mjs`: 93 images, named `LCC_M2_<part>_<case>_<theme>_<width>.png` and a `LCC_M2_report.json` (viewports, console problems, network problems, anything not run). The last run had no console error, no failed request and no request to another host.

After the real faction crests replaced the drawn marks (4.1) the same 93 captures were taken again, headlessly, with the same script (named `LCC_M2b_*`, report `LCC_M2b_report.json`: no console error, no failed request, nothing not run).

Pixel comparison against the approved V20 picture: made as a side-by-side of each risky crop, at the same scale, with the reference cut at the same fraction of the plaque. It is a visual comparison, not a numeric diff; no pixel-difference score was computed.

Variances from the approved V20 picture, for human approval:

| Item | Reference | This build | Kind |
|---|---|---|---|
| Seat size and pitch | pitch about 47 units, seats about 44 | pitch 52, seats 38 (equal inset at both ends is tested) | not resolved |
| Gold hub | a large dark ring with a heavy glow, offset from the middle | a gold seat with ring and halo, centred | not resolved |
| Bar lighting | neon glow on the rails and tips | flat gold edge and a thin inner line with a soft shadow | not resolved |
| Footer rail | wide glow under the footer | two thin gold rules | not resolved |
| HELD BY divider | glowing vertical line | plain gold rule, unbroken | close |
| Cloth standards and shields | game-drawn crests | the same crest pictures (see 4.1) | resolved |
| Claimed 13th seat | white crest on the gold hub, no colour change | white crest on the gold seat in the owner's colour (the M2 rule that the 13th takes its owner's colour) | intentional |
| Open seats | round unclaimed medallion | drawn bronze seat | not resolved |
| Session wording | "13 HOUSES WIN" | the wording table: "SYSSELRAAD WON · <FACTION>" by five in a line or by 13 Houses, "SYSSELRAAD CONTESTED" otherwise (DECISIONS section G, 2026-11-18; it was "MAJORITY AT 13 OF 25 HOUSES" after #912) | intentional |
| Contributors | demo guild names | "Not in the save" (D07, D15) | intentional |
| Layout below 940 px | none in the reference | a narrow re-layout and a stacked head under 520 px | intentional, needs approval |

Not done in M2: any wiring into the editor, any write path, any live data, the numeric pixel diff, a real-device or real-browser run on a phone, a screen reader run, a high-contrast run on Windows (forced colours was emulated), and a visual check by the operator.

### 7.1 Standards review (the two contributor cloths)

Reviewed at 1280, 768 and 390 wide, dark and light, and in forced colours, against the approved V20 plaque.

Findings:
- The words started right under the crest and the heading wrapped to two lines, so the crest, heading and list read as one cramped stack; V20 has the crest, a one-line heading and then the list, with air between.
- The foot of the cloth ended 4 units above the HOUSE nameplates and the foot ornament sat on the last contributor line when five names were shown.
- The list ran into the foot in the narrow layout (the slant cut the last line), and the foot had no gold edge there, only the sides.
- A long name had no safe rule: it could push a column wider (the two narrow columns became unequal) or overrun the cloth.
- In forced colours, wide layout, the standards were bare words with no outline and no mark.
- The empty state was a lone line at the top of a tall empty cloth.
- Plaque and standards were already mirrors in the drawing; the text boxes were also mirrors. The list was left-aligned against a centred heading, where V20 centres the block.

Changed (display only, `lcc.js` and `lcc.css`): the crest picture is larger (88 units, as in V20); the words box starts below it and ends above the ornament; the heading is one line; the list is a centred block with gold numbers, each name in its own span with the full name as a title, cut with an ellipsis in the wide layout and wrapped (even inside an unbroken string) in the narrow one; the cloth tip stops 10 units above the nameplates and the ornament sits just above the tip; the narrow cloth has a gold edge on the foot and a fixed foot depth with deeper padding; the two narrow columns are equal whatever the names; the empty state is centred in the cloth with a thin gold line above it, still the words "Not in the save"; forced colours, wide, draws an outlined box with the two-shape mark. Tests: `web-tests/lcc-standards.test.mjs` (five lines and a very long name on both standards, hostile text, the mirror of the drawing and of the boxes, the box between crest and ornament, the foot, forced colours, contrast), and a new gallery case `long`. Captured again, headlessly, as `LCC_M2c_*`.

Foot of the cloth, revisited against V20 (captured as `LCC_M2d_*`): the point of each cloth is now the approved picture itself (two PNG crops of the approved plaque in `lcc/art/standards/`, rows without baked-in text, design artwork supplied by the project owner), not a redrawn shape: the thick gold V band, the thin inner line, the darker tone and the wheat ornament are the original pixels, the cloth above it and the words stay live. Wide: placed where the plaque has it and faded into the cloth; narrow: below the cloth, lined up with its edge.

Left as it is, and why: the contributor names are not in a single-player save, so the filled list is shown only for fixture data and the live state is the honest empty one; the names use the page's body face, not the serif-like face of the approved picture (the picture is an image, the face is the page's); the narrow layout keeps its stacked cloths (no V20 equivalent).

## 8. Rollback

Remove `internal/web/static/lcc/`, the two names and five folders added to `internal/web/server.go`, the gallery folder, the manifest, the tests and the script, and restore `docs/art-notice.md` and `docs/art/landsraad/README.md`. Nothing else refers to them.
