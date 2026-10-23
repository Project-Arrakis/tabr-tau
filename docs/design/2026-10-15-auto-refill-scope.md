# Scope of the refill, repair and start-up auto-refill operations

> Note (2026-10-22): the review pane this document describes ("Review & save", "Save to game", Save reachable only from the pane) was removed in #238. Every edit is now written to the game's file as it is made, or queued while the file cannot be written; see `SECURITY.md` and the E5 group of `audit/2026-10-22-L2-findings-register.md` for what stands in its place. The text below is kept as the record of the design at its date.

Date: 2026-10-15. Issue: #165 (audit L2 G08, tracking issue #157). Status: record of what the code does today; one point is waiting for the owner.

## What the operations touch

The refill and repair buttons are not per base. They work on the whole save:

| Operation | Rows it can change |
|---|---|
| Refill base water (`RefillBaseWater`) | every built (not hologram) water cistern, windtrap and blood purifier in the save |
| Refill generators (`RefillGenerators`) | the fuel inventory of every built generator, spice generator, wind turbine and windtrap in the save |
| Repair all to max (`RepairBuildings`) | every building piece and placeable: raised to the best health seen for its type |
| Clear sand (`ClearSand`) | every building piece |

A single-player save has one player and one world, and the base list is a view of it, not a boundary: whole-save is the intended behaviour, and the buttons and docs say "all bases". True per-base scoping is deferred, with that reason. The Power and Water tabs of one base only read; they show the same devices the refill writes (a device's fuel inventory is one shared definition, the first inventory of type 3, `powerInventorySQL`).

## The start-up auto-refill

The opt-in setting "Automatic refill when the editor opens" (`autoRefillOnOpen`, off by default) runs `RefillBaseWater` and `RefillGenerators` at start-up and saves the result straight away, without the Review & save step. That is the one exception to "Save only from the review pane". It is the operator decision of 2026-10-08, recorded in the architecture plan, section "Exception to Save only from the review pane". It keeps every other protection of the write pipeline: the single-player check, the changed-on-disk check, the integrity check, a backup first and the read-back. It is skipped while a game session runs and when unsaved edits are waiting.

What the decision covered and what the code now does:

- Covered on 2026-10-08: base water (cisterns, windtraps, blood purifiers) and generators (fuel and spice).
- Added later by #141 (PR #142) under the same setting, with no separate recorded decision: wind turbines and windtraps (their filters and lubricant, five stacks of 100 for a turbine). **Owner to confirm** that this extension is accepted under the 2026-10-08 exception, or to ask for the turbines and windtraps to leave the start-up path while the buttons keep them.
- Not part of it: Repair all to max and Clear sand. They are buttons only and never run at start-up.

Every pass is one transaction that reads the devices and writes the fill together (#166), so it cannot fill a device twice, and a pass that finds everything full writes nothing and leaves no edit behind.

## Still open on #165

The pieces that live in the web layer and are not done by the operations change: the confirmation that states the consequence when the setting is switched on, keeping the start-up result on screen and across restarts instead of a 3.5 s toast and an in-memory "Last automatic save", and the "all bases" wording on the buttons.
