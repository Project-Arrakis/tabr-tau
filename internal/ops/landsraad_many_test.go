package ops

// The world of several houses (#364), on the world of landsraad_mine_test.go plus a third house of the current term (176, Vernius) where the character has no row.
const manyWorld = `
insert into landsraad_tasks(id, term_id, board_index, house_name, goal_amount) values (176, 7, 2, 'DA_HouseVernius', 4000);
insert into landsraad_task_rewards(task_id, threshold, template_id, amount) values
  (176, 1200, 'SolarisCoin', 10000), (176, 2400, 'Schematic.V', 1), (176, 3600, 'T6Machinery', 50), (176, 4800, 'Schematic.W', 1), (176, 6000, 'Swatch.Vernius', 1);
`
