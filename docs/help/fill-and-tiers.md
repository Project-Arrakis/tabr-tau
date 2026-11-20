# Fill, Complete and tiers

## The buttons

- **Tier boxes** (the numbered boxes of a House): click a tier to give your character that tier. Your contribution is raised to the tier's amount. Contribution adds up, so clicking tier 3 gives tiers 1 to 3. Click the highest earned tier again to remove it. Removing a tier also takes back its reward if the reward still waits at the House representative (see below).
- **Set tier and Apply tier** do the same as clicking the box of that tier.
- **Fill** raises your own contribution to the top tier (it is never lowered) and writes what the missions that reach it pay.
- **Fill faction goal** does Fill and also takes your faction to its goal in one click.
- **Complete House** pledges the House to the chosen faction and sets that faction's progress to the House goal.
- **Reopen** clears the completion and the faction that holds the House's favor. Progress and contribution stay as they are, so it does not fully undo a Complete: the progress that Complete raised stays, and a House that another faction had claimed is left undecided, not given back.
- **Choose** boxes pick Houses for **Fill chosen**, **Complete chosen**, **Complete and fill chosen**, **Reopen chosen** and **Unfill chosen**. Several Houses are one edit: either every House changes or none.
- **Complete and fill chosen** completes the chosen Houses for the chosen faction and fills your own contribution to the top tier, in one write with one backup and one result line. It pays the missions like Fill does, so the question names them. It is off, with the reason shown, while a faction other than your own is chosen: your contribution counts for your own faction, and **Complete chosen** is the button for another faction.
- **Unfill chosen** sets your own contribution to 0 on each chosen House, so its tiers go back to 0 of 5. It never changes whether a House is taken: a pledged House stays pledged. It takes back the waiting rewards of the removed tiers by the rule below, and the question names how many it takes back and how many collected rewards stay. It always asks first. It is off, with the reason shown, while another faction is chosen. **Reopen** and **Unfill** are the way back from Complete and fill.
- **Faction for Fill and Complete** starts on your own faction (the box says so: "(your faction)"). Choosing a faction writes nothing. While another faction is chosen, Fill, Complete and fill, Unfill and the tier boxes do nothing, because your contribution counts for your own faction.

## How each click is written

- At once, with a backup of the save first.
- It is refused if the House is no longer as you were shown it.
- A click that writes ten missions or more on one House asks first, and so does every button for several Houses. The question says what it writes.
- Unfill chosen always asks first, and it is refused as a conflict, with nothing changed, when the number of waiting rewards it would take back is no longer the number the question named.
- Removing a tier asks first when it takes back a reward, or when the reward of the tier stays and the question has something to say about it. The question names the reward.
- Complete on a House whose favor another faction holds replaces that faction. The question says so first.

## What a raise writes

In one edit:

- Your contribution, raised to the tier.
- The same ground added to your faction's progress while the House is open. The House is pledged to the faction when the progress reaches the goal.
- One reward for each tier crossed, waiting at the House representative.
- For each mission, what a mission with a Recollection turned in pays: 6,000 Solari and 1,000 House Scrip in your wallet, and 1,500 XP on the specialization of the House. Whether a mission without a Recollection pays the House Scrip and the XP is not known.

After a change of faction, what you reached under your earlier faction counts as already paid. A raise up to that ground writes no mission, no reward and no progress for your new faction, so its tiers can read "Reached, no delivery recorded" although nothing was delivered under the new faction; only the ground above it is paid.

A mission is 1,000 contribution times the setting `LandsraadContributionMultiplier`, and the XP is scaled by `LandsraadSpecializationXpMultiplier`. Both are read from `ServerCustomSettings.ini` on this computer (1 when the key is absent). A click is refused, with the reason, when a value cannot be used or the file cannot be read. The daily and weekly mission limits of the game are not applied.

## What it does not write

- Your character's own XP and your faction standing.
- No progress event. The editor leaves the game's progress cursor where it is. Whether the game keeps a contribution raised here at its next step is not checked in the game.
- Removing a tier does not take back the Solari, House Scrip and XP that its missions paid, and it never touches the progress of your faction or which faction the House is pledged to. Raising the tier again does not pay them a second time either.

## What removing a tier takes back

Removing a tier lowers your contribution and takes back the reward of each removed tier that **still waits** at the House representative: the row the raise wrote for that tier. In detail:

- A reward you **already collected** stays. The result says how many.
- A waiting reward is taken back only when the editor's own record says the editor paid that tier and the editor can tie the reward to the removed tier with certainty. It stays, and the result says so, when the same item is also the reward of a tier that stays, when less waits than the tier paid, when the waiting reward was written in an earlier term, or when the editor has no record of paying that tier (for example a reward the game wrote, or one an older version of the editor wrote). The editor remembers where it started paying on a House: a tier at or below that ground may hold a reward the game wrote there, so it stays, and a record kept by an older version of the editor does not say where it started, so every waiting reward stays until the next term. An older version of the editor does not know the ceiling and the starting ground the newer one adds to its record, and drops them when it next writes it: after using an older version, raising a removed tier can pay its missions once more. A waiting reward that your own missions in the game cover stays too, and the result counts it apart. A tier with nothing waiting is not counted at all. The result counts rewards, not tiers: two tiers that give the same item are one reward.
- The House swatch (the reward of the top tier) is taken back only when the editor's own record says the editor wrote it. Taking it back also removes your one-time swatch row of this term, so the swatch can be obtained again.
- Only your own rows for that House are looked at. Other Houses, other players and the rest of the save are not touched, and a backup is made first.
- A reward that is not collected and is not tied to a removed tier (for example one left by an earlier removal) is not touched.

Raising the tier again writes the reward that was taken back once more, and pays nothing else: no Solari, House Scrip or XP and no progress for your faction, however often you remove the tier and raise it again. A reward you collected is not written again. The result and the question before a click say how many missions were not paid because that ground was paid before.

## Paid once

The editor remembers the ground it paid in a small file of its own, next to its other files (never in the game's save). A removal that takes rewards back also writes into that file the tier it took them back up to, so the missions stay paid while the rewards are gone. Raising a removed tier again pays no mission a second time, and a raise above what was paid pays only the new tiers. A second Fill with nothing above your contribution writes nothing. The editor counts the file only as far as the save agrees (the highest tier whose reward the save shows from this term, or the tier a removal took rewards back up to): if you put an older backup back, ground the backup does not show is paid as if the file did not say so, except that a removal's tier stays counted, so such a raise pays less rather than more. If the file is missing, damaged or belongs to another save, nothing is taken as paid, and no waiting reward is taken back by a removal, because nothing says the editor paid it.

## Going back

A backup of the save is taken before the change is written. Restore on the Database tab (Save backups) puts it back.

## Not checked in the game

The pay-out of a mission (Solari, House Scrip, specialization XP and rewards) has not been checked in the game yet. See [Checked in the game](checked-in-the-game.md).
