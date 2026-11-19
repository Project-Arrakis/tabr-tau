# Fill, Complete and tiers

## The buttons

- **Tier boxes** (the numbered boxes of a House): click a tier to give your character that tier. Your contribution is raised to the tier's amount. Contribution adds up, so clicking tier 3 gives tiers 1 to 3. Click the highest earned tier again to remove it. Removing a tier also takes back its reward if the reward still waits at the House representative (see below).
- **Set tier and Apply tier** do the same as clicking the box of that tier.
- **Fill** raises your own contribution to the top tier (it is never lowered) and writes what the missions that reach it pay.
- **Fill faction goal** does Fill and also takes your faction to its goal in one click.
- **Complete House** records the favor of the House as won by the chosen faction and sets that faction's progress to the House goal.
- **Reopen** clears the win and the completion. Progress and contribution stay as they are, so it does not fully undo a Complete: the progress that Complete raised stays, and a House another faction won is left open, not given back.
- **Choose** boxes pick Houses for **Fill chosen**, **Complete chosen** and **Reopen chosen**. Several Houses are one edit: either every House changes or none.
- **Faction for Fill and Complete** starts on your own faction. Choosing a faction writes nothing. While another faction is chosen, Fill and the tier boxes do nothing, because your contribution counts for your own faction.

## How each click is written

- At once, with a backup of the save first.
- It is refused if the House is no longer as you were shown it.
- A click that writes ten missions or more on one House asks first, and so does every button for several Houses. The question says what it writes.
- Removing a tier asks first when it takes back a reward, or when the reward of the tier stays and the question has something to say about it. The question names the reward.
- Complete on a House another faction won replaces that win. The question says so first.

## What a raise writes

In one edit:

- Your contribution, raised to the tier.
- The same ground added to your faction's progress while the House is open. The House is won when the progress reaches the goal.
- One reward for each tier crossed, waiting at the House representative.
- For each mission, what a mission with a Recollection turned in pays: 6,000 Solari and 1,000 House Scrip in your wallet, and 1,500 XP on the specialization of the House. Whether a mission without a Recollection pays the House Scrip and the XP is not known.

A mission is 1,000 contribution times the setting `LandsraadContributionMultiplier`, and the XP is scaled by `LandsraadSpecializationXpMultiplier`. Both are read from `ServerCustomSettings.ini` on this computer (1 when the key is absent). A click is refused, with the reason, when a value cannot be used or the file cannot be read. The daily and weekly mission limits of the game are not applied.

## What it does not write

- Your character's own XP and your faction standing.
- No progress event. The editor leaves the game's progress cursor where it is. Whether the game keeps a contribution raised here at its next step is not checked in the game.
- Removing a tier does not take back the Solari, House Scrip and XP that its missions paid, and it never touches the progress of your faction or the House's win.

## What removing a tier takes back

Removing a tier lowers your contribution and takes back the reward of each removed tier that **still waits** at the House representative: the row the raise wrote for that tier. In detail:

- A reward you **already collected** stays. The result says how many.
- A waiting reward is taken back only when the editor can tie it to the removed tier with certainty. It stays, and the result says so, when the same item is also the reward of a tier that stays, when less waits than the tier paid, when the waiting reward was written in an earlier term, or when your own missions in the game cover the tier.
- The House swatch (the reward of the top tier) is taken back only when the editor's own record says the editor wrote it. Taking it back also removes your one-time swatch row of this term, so the swatch can be won again.
- Only your own rows for that House are looked at. Other Houses, other players and the rest of the save are not touched, and a backup is made first.
- A reward that is not collected and is not tied to a removed tier (for example one left by an earlier removal) is not touched.

Raising the tier again pays it again: the reward is written again, together with its missions (their Solari, House Scrip and XP were not taken back, so they are paid a second time).

## Paid once

The editor remembers the ground it paid in a small file of its own, next to its other files (never in the game's save). Raising a removed tier again pays nothing again while the save still shows its reward (a reward you collected), and a raise above what was paid pays only the new tiers. A second Fill with nothing above your contribution writes nothing. If the file is missing or belongs to another save, nothing is taken as paid.

## Going back

A backup of the save is taken before the change is written. Restore on the Database tab (Save backups) puts it back.

## Not checked in the game

The pay-out of a mission (Solari, House Scrip, specialization XP and rewards) has not been checked in the game yet. See [Checked in the game](checked-in-the-game.md).
