# Help snippet for the board controls (mock for tau #974, options E2 and E3)

This is the Help text that goes with the control area when every explanatory sentence has been taken off the screen. It is written in the style of `docs/help/landsraad-board.md` and is not part of that page yet: the build would add it as the section 'Fill, Complete and Reopen' and extend 'Filters and the list'. Fixture-free: it describes the controls, not data.

## Fill, Complete and Reopen

- A House is chosen with the box on its card, or with **Choose shown (N)**, which chooses the Houses the board shows at that moment (N is how many). **Clear** drops the choice. Nothing is written by choosing.
- **Show**, **House** and the drawer (Specialization, Jump to House, Group) change only which Houses are shown. The boxes, Choose shown and Clear change which Houses are chosen. A House that a filter hides is never acted on: the number on the count medallion and on Fill, Complete and Reopen counts only the Houses that are chosen and shown, and a hidden House cannot be ticked.
- **Fill**, **Complete** and **Reopen** each carry a number: how many of the chosen Houses that action would change (for example 'Fill 21'). Hover the key, or put the keyboard focus on it, for the sentence.
- **Fill** and **Complete** act for the faction shown by the two crests, Atreides or Harkonnen. It starts on your own faction. Choosing a crest writes nothing.
- Pressing an action asks one short question, for example 'Fill 21 Houses for Harkonnen?', with **Yes** and **Cancel**. Hover or focus **Yes** to see the totals: the missions written, the Solari, House Scrip and experience, and the rewards that will be waiting at the House representative. Everything is written at once, with a backup first. Escape cancels and nothing is written.
- With nothing chosen the plate is a thin strip: the count 0, the words 'Choose Houses' and Choose shown (N). The keys for the actions appear when at least one House is chosen and shown.
- In option E3 the same strip shows the filters until a House is chosen and the actions after; the Filters drawer is not available while Houses are chosen, and **Clear** or Escape brings the filters back.

## Filters and the list

- **House** has three words: Any, **Undecided** (no faction has the favor of the House yet; **Wavering** is an Undecided House that both factions have progress on) and **Pledged** (the House is pledged to a faction: 'Pledged to Atreides' or 'Pledged to Harkonnen').
- The number '25 / 25' beside the filters is how many Houses are shown out of 25.
- The Filters key opens the drawer: Specialization, Jump to House and, in the list layout, Group. Its number says how many of them are not at their default.
- Grid shows the Houses as a 5 by 5 board; List shows the same Houses as a sortable list, and Group is then in the drawer.

## The board header

- 'Term 7 · 5 × 5' means the board is the fixed 5 by 5 of the term; the game positions are read from `board_index` (the same sentence as the first paragraph of The Landsraad board). When a faction has secured the Sysselraad, the victory panel's headline reads SYSSELRAAD SECURED; that panel is not part of the control area.

## Where each removed sentence went

| Removed from the screen | Now |
|---|---|
| 'Term 7 · Fixed 5 × 5 board · game positions read from board_index' (under the title) | Short line 'Term 7 · 5 × 5'; tooltip on it; this Help, 'The board header' |
| 'Fill changes 21 of the 25 chosen Houses. A question shows the totals before anything is written.' (the mouse-over text of Fill; the same for Complete and Reopen) | Tooltip on each key; this Help, 'Fill, Complete and Reopen' |
| The question's paragraph: 'Writes 107 missions: +642,000 Solari, +107,000 House Scrip, ... 92 rewards waiting at the House representative. Houses: Alexin, Dyvetz, ... 4 other chosen Houses are left alone: nothing to change for them.' | Tooltip on Yes (the totals); Help, 'Fill, Complete and Reopen'. The names of the Houses are on the cards' boxes |
| 'Written at once, with a backup first.' | Tooltip on Yes; Help |
| 'Fill and Complete act for this faction. It starts on your own faction. Choosing it writes nothing.' (read to screen readers only, never drawn) | Tooltip on each crest, still the key's accessible description; Help |
| Today's summary line 'Chosen: 25 houses. Fill chosen changes 21, Complete chosen changes 14 for Atreides, ... Reopen chosen changes 16.' | The count medallion and the numbers on the keys; tooltip on the medallion; Help |
| 'Choose all houses' (a label that sounded like the Show tab All) | 'Choose shown (N)'; tooltip 'Chooses the Houses that are shown now...'; Help |
| House chips 'Not won' and 'Favor won' (option E) | Undecided and Pledged (#1079); tooltips; Help, 'Filters and the list' |
| The Filters drawer being away while Houses are chosen (E3) | Tooltip on Clear; Help |
