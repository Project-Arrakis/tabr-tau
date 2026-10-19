# UI/UX review of the editor, 2026-10-18

Scope: every tab and sub-view, the header and banners, the review-and-save flow, confirmations, empty/error/loading states, accessibility and the CSS
(`internal/web/static/`). Method: read the UI and the code behind each control (`internal/ops`, `internal/save`), computed contrast ratios from the CSS colours,
and rendered the real stylesheet in a headless browser. Nothing was changed. Every claim of inaccuracy was checked against the code.

## Verdict

The editor is functionally rich and unusually careful about safety: every save-affecting edit is queued and reviewed, risky actions are confirmed, typed-word
gates guard SQL writes and backup restore, output is escaped by construction, and the explanatory text mostly matches the code (refill amounts, the 2,779 Intel
cap, the level 200 cap, the 0 to 12474 reputation range, repair scope and the autorefill behaviour were all checked and agree). It is let down by a thin
generic brown-and-orange skin, accessibility gaps the 2026-10-15 audit already named (UX-07, UX-08, UX-09) that are still open, developer voice in the text,
and a handful of concrete defects.

| Area | Score /10 |
|---|---|
| Accuracy | 7.5 |
| User-friendliness | 5.5 |
| Polish | 4.5 |
| Professional | 5 |
| Redundancy | 5.5 |
| Dune theme | 3.5 |

## Findings (most important first)

1. **[Accuracy, high]** "Reset all vendors" asks the wrong question. `app.js:483` and `:1002`: for all vendors (`d.v` empty) the confirm says "Resets the purchase limits
   of this vendor", but `ResetVendors` (`ops/world.go:166-177`) deletes every vendor's rows. Branch on `d.v`; say "all N vendors"; name the vendor for one.
2. **[Friendly, high]** The header shows the full save path, including the FLS id that the Summary card hides ("so a screenshot does not share them") and the Windows
   user name (`index.html:11`, `app.js:235`). Database > accounts and the CSV export also show ids unmasked. Show the file name and parent folder, full path in a
   `title` or a Copy path control, and note under Database that raw tables are unmasked.
3. **[Polish, high]** The sticky warning banner cannot stick: the header is `sticky; top:0; z-index:5` (`app.css:4`) and the banner `sticky; top:0; z-index:4`
   (`app.css:47`), so after scrolling the header covers "Saving is blocked", "The game saved since this editor opened" and "Lost contact". Offset the banner by a
   `--hdr-h` token or put both in one sticky wrapper.
4. **[Polish, high]** Primary buttons fail contrast: white on `--acc` is 2.63:1 (dark, `#fff` on `#d9902f`) and 4.07:1 (light, `#fff` on `#b56d10`); `.rankbar` uses an
   undefined `--accent` fallback `#c9a227` at 2.33:1 on the light panel (`app.css:13, 25, 63`). Dark text on spice in dark mode, darker spice with white text in light.
5. **[Polish, high]** Control borders are about 1.5:1 (`--line` on the input background), failing WCAG 1.4.11. Add a `--line-strong` token for control borders.
6. **[Polish, high]** No hover, active or focus-visible rules anywhere (the only `:hover` is `tr:hover td`), and no `color-scheme`, so native scrollbars and popups ignore the
   dark theme. Add `:focus-visible`, hover and active states, and `color-scheme: dark light`.
7. **[Friendly, high]** The review dialog is not a proper modal (audit UX-07): `role=dialog aria-modal` without `aria-labelledby`, the Tab trap covers only the ask
   dialog, `render()` drops focus after each action, and toasts have no `role=status` / `role=alert`. Add labels, extend the trap, set `inert` on the page behind, restore focus.
8. **[Friendly, high]** Config edits write to the game's .ini files the moment a field loses focus (`app.js:1091`, `:808`), bypassing Review and Discard, which every other tab
   uses. Add a per-row Save or Apply and Revert, or a persistent note that changes here are written immediately.
9. **[Accuracy, high]** The SQL write confirmation shows only the first 400 characters in a `<p>` that collapses whitespace (`app.js:1024`) while the whole statement runs.
   Show the full statement in a scrollable `<pre>` and say how many characters were cut.
10. **[Accuracy, med]** Read-only SQL silently stops at 1,000 rows (`ops/db.go:264`); a result of exactly 1,000 looks complete. Return a `truncated` flag and say so.
11. **[Friendly, med]** Filter boxes ignore Enter and typing (Inventory, Journey, Research, Crafting, Customizations, Database; `app.js:393, 507, 556, 571, 594, 826`), and
    Inventory and Database refetch and lose focus. Filter on input (debounced) with a Clear control and refilter client-side.
12. **[Friendly, med]** Specialization is a dead end for new users (`app.js:423-433`): Add XP, Grant Max and Reset are disabled until you earn XP in game and reload, explained in a
    90-word paragraph, and "Advanced: set a raw row" is open by default with no confirm. Collapse both into `<details>` and require a typed confirm for the raw row.
13. **[Accuracy, med]** Landsraad Start and End are raw integers (`app.js:774`). Format as local date and time and show time left.
14. **[Professional, med]** Landsraad reward lines and the Vendor table show raw template and vendor ids (`app.js:781`, `:484`). Look names up in the catalog.
15. **[Professional, med]** The review's "Edits you made" list mixes casing and raw ids ("xp +1000", "edit item 4521", "faction 3 reputation = 500", "landsraad task 12 faction 2 progress = 5000",
    "decree 4 disabled=1", "skill <id> rank 3"). One sentence-case template per operation with names and before to after (UX-04 is only partly fixed).
16. **[Redundancy, med]** Repair exists in three places for gear and two for vehicles (Inventory and Admin; Vehicles and Admin). Keep one home each and link from Admin.
17. **[Friendly, med]** Disabled "not in tabr-tau yet" controls fill Admin and Player (Repair Faction, Repair Quests, Wipe Inventory, Reset Progression, Spawn) under a "Danger
    Zone" card with nothing usable; the Blueprints stub says "planned (#96)" while the README says #111 (`app.js:467-481, 138-141, 487`). Remove dead controls and the stub pill; keep the roadmap in the README.
18. **[Professional, med]** "The console" is never defined and appears 6+ times in visible text (`app.js:403, 408, 428, 468, 483, 487, 694`). Remove the asides or define it once.
19. **[Professional, med]** Raw snake_case and bare column headers (`time_stored`, `reason`, `id`, `vendor_id`, `name`, `size`, `modified`; `app.js:485, 716, 761, 777, 813, 843`);
    "id", "Id", "ID" and "Item id" are inconsistent. Label every column and use "ID".
20. **[Friendly, med]** Both backup lists show size in bytes and an ISO timestamp, with no delete or prune (`save.go:648`; `app.js:813, 843`). Show "12.4 MB" and a local date; add keep-last-N and delete.
21. **[Friendly, med]** The safety net is hard to find: restoring a backup is under Database > Save backups next to raw SQL, and the post-save toast with the backup path vanishes after 15 s. Add a visible Backups entry and a link in the review dialog and the toast.
22. **[Polish, med]** The post-save toast's long path overflows (`app.js:890`, `app.css:32`). `overflow-wrap:anywhere`, show the file name, offer Copy path.
23. **[Polish, med]** The Live Map canvas distorts on resize and is blurry on HiDPI (`app.css:67`, `livemap.js:240-241`). Size the bitmap from the client size times the device pixel ratio with a `ResizeObserver`.
24. **[Friendly, med]** The Live Map has no keyboard or touch path, no zoom or reset buttons, no `aria-label`; toggling a layer or the auto-refresh re-mounts it and resets pan and zoom (`livemap.js:249-272, 301-304`).
25. **[Professional, med]** Live Map marker labels are raw type ids (`HouseRepresentativeArgosaz`, `AzuritePickup`, `EnemyLaborOutpost`) and coordinates have no unit (`livemap.js:260, 295`; audit UX-11).
26. **[Accuracy, low]** The tab name "Live Map" suggests a live feed; the page says "as of your last save". Rename to "Map".
27. **[Polish, med]** `.warn` is reused as an inline pill on `<span>`s in table cells (`app.js:509, 558, 742`) and renders as a heavy dark block. Add a `.badge.warn` and keep `.warn` for banners.
28. **[Polish, med]** On narrow windows the six-item sticky header wraps to about three rows and there are no `@media` rules (`app.css:4`). Collapse the path and auto-refresh into a menu, un-stick on small screens, add breakpoints.
29. **[Polish, med]** Weak typographic hierarchy: card titles are 13px uppercase muted, the h1 is 16px, there is no h2 (`app.css:15, 52`). Use a 15 to 16px heading in the text colour, a real h2 per view, and the type pairing below.
30. **[Friendly, med]** The Summary card (about 19 tiles including database ids) is re-fetched above all 12 Player sub-tabs and pushes every tab below the fold (`app.js:360-369`). Compact strip with a Details expander.
31. **[Accuracy, med]** Solari appears under three names in four places (Summary tiles "Solari Credit", "Solari Coin", "House Credit"; Overview "Solari in backpack"; Quick rewards "Give currency"). The code is right but a player cannot tell which to edit. One Currency card with three labelled rows.
32. **[Friendly, med]** With saving blocked ("You can keep editing", `app.js:245`) edits will later hit "changed on disk" (audit UX-01); auto-refresh also drops a half-made Land Claim selection (`LC.sel`). Dim edit controls when blocked and do not auto-refresh while a claim selection is open.
33. **[Friendly, med]** Config dead end in window mode: "pass `--config <folder>`" with no command line and no folder picker (`app.js:790`; `locked.html:13`). Add a folder chooser or explain the shortcut.
34. **[Professional, med]** Casing and wording are inconsistent: Title Case ("Specialization Tracks", "Grant All Keystones", "Player Admin Actions") against sentence case ("Unlock all research", "Quick rewards"); toasts mix "Hydration 70.1 -> 100." and "XP 1 to 2"; "row(s)" (`app.js:924, 1027`). Sentence case, one arrow style, a short style guide.
35. **[Redundancy, med]** Land Claim has two overlapping editors (the grid and "Resize by square", `app.js:660-673, 722-729`) and drifting terms ("Horizontal segments" `cells - 1` against "N cells"). Merge them; use "cells" consistently.
36. **[Redundancy, med]** The Automatic refill card appears on both the Power and Water sub-tabs of every base (`app.js:606-610, 695, 701`) and is the one write that bypasses Review. A single Settings home.
37. **[Friendly, med]** Landsraad actions are inconsistent: the faction select feeds Fill goal and Complete, only Complete confirms, Term Apply does not, and "Reigning", "Active decree" and "Elected decree" are not explained (`app.js:774-782`). Label the selector "Act as faction", put the faction on each button, apply one confirmation policy.
38. **[Friendly, low]** Database browse: "0 rows - showing 1-0", every cell is an editable input that commits on blur, the search placeholder is just "search", paging has no page number (`app.js:832, 1092`).
39. **[Friendly, low]** The review pane's Save is at the bottom of a long scroll (`app.js:302`, `app.css:50-52`). Sticky footer with the edit count.
40. **[Polish, low]** Tables: click-only sorting with no `tabindex` or `aria-sort`, stacked `.scroll` regions trap the wheel, unlabeled inline number inputs (`app.js:1034-1036`).
41. **[Polish, low]** Rank bars are repeated bullet characters with only a title (`app.js:535`). `role=img aria-label="Rank 2 of 5"` and a compact "2/5".
42. **[Professional, low]** Internal ids appear in visible columns and tooltips (Research "Key", Crafting "Recipe id", "Item id", "FLS ID", "DB player", "Player controller"). Hide behind a "Show ids" toggle.
43. **[Friendly, low]** "Give item" mixes Give, Add to queue, Give water, Fill hydration and Refill Literjons in one row (`app.js:408`). Split into Give item and Top-ups.
44. **[Polish, low]** Leftovers: unused `A.spec` (`app.js:927`); the map icon list repeated in `livemap.js`, `app.css` (about 60 `.lm-i-*` rules) and a server-side listing; hard-coded `.warn` and `.info` colours with a blue that clashes with the palette; no favicon; `<title>` does not change per tab.
45. **[Accuracy, low]** Wording nits: "Auto-refresh" does not say what it refreshes; "Health 85.0 (max at least 150)" reads awkwardly; "Skill points 9 / 70" is not labelled unspent over total; the Skills "Charge for changes" box defaults off, so rank changes do not spend points unless ticked.

## Quick wins (each under an hour)

Banner offset (3); vendor reset text (1); button colours, `--line-strong`, focus, hover and `color-scheme` (4 to 6); format Landsraad times and names (13, 14); label every table column and use "ID" (19);
sentence case and one arrow style (34); the full SQL in a scrollable `<pre>` and a truncation note (9, 10); `overflow-wrap` on toasts and `role=status` / `role=alert` (7, 22);
Enter-to-filter and the empty-table state (11, 38); remove "the console" asides and the dead Admin buttons (17, 18); header path to file name and folder (2); a badge class instead of `.warn` on spans and a defined `--accent` (4, 27); format backup size and date (20).

## Larger changes

A real theme layer (tokens, fonts, ornaments, badge and button variants, banners and map legend on tokens); properly modal dialogs with `inert`, focus return and an accessibility pass on tables, rank bars, toggles and the canvas; a Live Map rework (resize and DPR, keyboard and touch, persistent view across refresh, readable names); a navigation restructure (compact Summary strip, an Advanced group for Config and Database with a visible Backups entry, one Settings home for autorefill, one Land Claim editor, one repair home, one Currency card); one commit model for Config; a first-use flow for Specialization; responsive breakpoints.

## Proposed Dune theme

All colour pairs were computed. Dark: body text 13 to 15:1, muted at least 6.85:1, dark text on spice 7.26:1, spice-as-text on the panel 9.11:1, control border 3.76:1.
Light: body at least 12.4:1, muted at least 5.86:1, white on spice 5.42:1, control border 4.31:1. Every status colour is at least 4.5:1 on its panel in both themes.
The CSP allows only same-origin fonts (`font-src 'self'`): use the system stacks below (Windows always has Palatino Linotype, Segoe UI Variable and Cascadia Mono) or self-host open fonts
(Cinzel or Marcellus for display, IBM Plex Sans for body, IBM Plex Mono for ids) and register them in `server.go`. Do not use Funcom logos or fonts. Original ornament ideas: a thin double rule with a small
diamond as the card-title divider, a dune-ridge underline on the active tab, Fremen blue (Ibad) for links and info, faction colours as small pips only.

```css
:root{
  /* Night on Arrakis (default) */
  --bg:#14100b; --panel:#1c1610; --raised:#271e14; --inset:#0f0c08;
  --line:#3b2f21; --line-strong:#7d6a4e;
  --text:#f0e6d0; --muted:#b5a68a;
  --spice:#e8892a; --on-spice:#1a0e03; --spice-text:#f4a95a;
  --ibad:#5fb0d6; --ok:#7ec886; --warn:#eab25a; --bad:#f27766;
  --atreides:#5cae78; --harkonnen:#e0584c; --choam:#d9b84e; --focus:#f4a95a;
  --banner-warn-bg:#3a2810; --banner-warn-text:#ffe2b8;
  --banner-info-bg:#14303d; --banner-info-text:#cfeaf7;
  --map-bg:#0d0f12;
  --font-display:"Cinzel","Palatino Linotype",Palatino,"Book Antiqua",Georgia,serif;
  --font-body:"IBM Plex Sans","Segoe UI Variable Text","Segoe UI",system-ui,sans-serif;
  --font-mono:"IBM Plex Mono","Cascadia Mono",Consolas,monospace;
  --fs-xs:12px; --fs-sm:13px; --fs-md:14px; --fs-lg:16px; --fs-h2:20px; --fs-h1:24px;
  --lh-body:1.5; --tracking-caps:.08em;
  --sp-1:4px; --sp-2:8px; --sp-3:12px; --sp-4:16px; --sp-5:24px; --sp-6:32px;
  --radius-sm:4px; --radius:8px; --radius-pill:999px;
  --shadow-1:0 1px 0 rgba(0,0,0,.4); --shadow-2:0 8px 24px rgba(0,0,0,.45);
  --ring:0 0 0 2px var(--bg),0 0 0 4px var(--focus);
  --hdr-h:56px; /* offset for sticky banners */
  color-scheme:dark light;
}
@media (prefers-color-scheme:light){:root:not([data-theme="dark"]){
  /* Daylight over the Erg */
  --bg:#f4ebd8; --panel:#fcf6e8; --raised:#ede1c5; --inset:#ffffff;
  --line:#d5c6a3; --line-strong:#85724f;
  --text:#2a1f12; --muted:#605238;
  --spice:#a85208; --on-spice:#ffffff; --spice-text:#8a4205;
  --ibad:#1d6a8c; --ok:#276b33; --warn:#7a5200; --bad:#a52f1d;
  --atreides:#2a7347; --harkonnen:#a5271d; --choam:#7a6210; --focus:#8a4205;
  --banner-warn-bg:#f6e2bd; --banner-warn-text:#4a2f05;
  --banner-info-bg:#d9edf6; --banner-info-text:#0f3a4d;
  --shadow-1:0 1px 0 rgba(60,40,10,.15); --shadow-2:0 8px 24px rgba(60,40,10,.25);
}}
```

Microcopy in a restrained in-universe voice (each keeps the plain meaning in the same sentence):

1. Save success: "Saved to the game. Backup kept as game-20261018-1203.db."
2. Discard confirm: "Every edit since the file was loaded is dropped, as if the wind had covered your tracks. The file on disk is not touched."
3. Game-running banner: "The game is running in single-player, so saving is paused. Close the game or return to its menu, then save."
4. Water refill: "Water is life: fills every cistern, windtrap and Blood Purifier to capacity."
5. Map hint: "Hover a marker to read the sand. Drag to pan, scroll to zoom."
6. Empty base list: "No base founded in this save yet."
7. Lost connection: "Lost contact with the editor. Saving is paused until it answers."
8. Landsraad: eyebrow "Landsraad term 14"; "Reigning" relabelled "Ruling faction".
9. Config warning: "Close Dune before editing: the game rewrites these files when it exits."
10. Locked page: "This tab is not signed in. Open the one-time link tabr-tau printed when it started; it opens the door for a few minutes."
