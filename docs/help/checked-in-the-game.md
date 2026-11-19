# Checked in the game

Every write has automated tests on a synthetic save built from the game's real schema. This page says what was tried beyond that. The full list, with dates and where each result is recorded, is the table "Which write features were checked in the game" in the project README.

| Feature | Checked how |
|---|---|
| Landsraad: Fill to a tier, Fill chosen | Tests, and run on a copy of a real save. Not in the game. Whether the game hands out the tier after it is not known |
| Landsraad: Complete and Reopen | Tests only |
| Landsraad: Complete and fill chosen, Unfill chosen (#932) | Tests only, on copies of a test save; not in the game |
| Landsraad: the completion time written by Complete | Tests only. Based on the game's play clock inside the term's window |
| Landsraad: what a mission pays (Solari, House Scrip, specialization XP, rewards) | Not checked in the game yet |
| Landsraad: whether the game keeps a contribution raised here at its next progress step | Not checked in the game |
| Landsraad: Term and Decrees controls (Apply, Make active, Set flag) | Tests only |
| Specialization: Add XP, Grant LV 100, Reset, keystones | Tests only |

"Tests only" means the rows written are exactly the expected ones and the save passes its integrity check, but the game itself has not been run on the result.

Nothing on this page is a promise about how the game behaves. If you try a feature in the game and see something different, the backup taken before each write puts the save back.
