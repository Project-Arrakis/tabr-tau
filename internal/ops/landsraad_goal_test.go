package ops

import "testing"

// Fill faction goal as a completion at the top tier (#601), on the world of landsraad_mine_test.go (character 1 in Harkonnen, 2) plus the third house of manyWorld and a play
// clock of 150 seconds inside the current term's window (100 to 200). House 174 (Maros) is complete with the faction at 4,250 and the character at 7,850 (above the top tier, 6,000);
// 175 (Kenola) is open with the faction at 1,000 and the character at 1,300; 176 (Vernius) is open with no row of any kind; 149 belongs to the earlier term 6.
const goalWorld = manyWorld + `
insert into farm_variables(one_row, farm_id, universe_time_timestamp, universe_lastactive_timestamp, down_time_accumulation) values (1, null, 1000000000000000, 1000000150000000, 0);
`

func goalOps(t *testing.T) *Ops { return mineOps(t, goalWorld) }

func houseRow(t *testing.T, o *Ops, id int64) map[string]any {
	t.Helper()
	return mustOne(t, o, `select completed c, winning_faction_id w, completion_time ct from landsraad_tasks where id=?`, id)
}
