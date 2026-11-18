// The Command Center's source adapter (M3, stage A): it reads the answer of GET /api/landsraad into one value per House and the totals of the same pass. It is checked against the synthetic
// answer of M1 and against every board case of M1 (no game save is read), and against boards that must not be trusted. It has no DOM, no network and no write.
import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { createRequire } from 'node:module';
import { lccDir, root } from './lcc-load.mjs';

const require = createRequire(import.meta.url);
const M = require(path.join(lccDir, 'lcc-model.js'));
const fixDir = path.join(root, 'docs', 'design', 'landsraad-command-center', 'fixtures');
const clean = (o) => JSON.parse(JSON.stringify(o, (k, v) => (k.startsWith('_') ? undefined : v)));
const answer = () => clean(JSON.parse(fs.readFileSync(path.join(fixDir, 'api-landsraad.synthetic.json'), 'utf8')));
const cases = () => clean(JSON.parse(fs.readFileSync(path.join(fixDir, 'board-victory-cases.json'), 'utf8'))).cases;
const NL = String.fromCharCode(10);
const byName = (b, n) => b.houses.find((h) => h.name === n);

test('the synthetic answer is a ready board of 25 Houses by board position, with numbers 01 to 25 and one count that adds up', () => {
  const b = M.readBoard(answer());
  assert.equal(b.ready, true, b.problems.join(' | '));
  assert.equal(b.houses.length, 25);
  assert.deepEqual(b.houses.map((h) => h.index), [...Array(25).keys()]);
  assert.deepEqual(b.houses.map((h) => h.number), [...Array(25).keys()].map((i) => String(i + 1).padStart(2, '0')));
  assert.equal(b.grid.length, 25);
  assert.deepEqual([b.counts.a, b.counts.h, b.counts.open, b.counts.unknown], [12, 4, 9, 0]);
  assert.equal(b.counts.a + b.counts.h + b.counts.open + b.counts.unknown, 25);
  assert.equal(b.countComplete, true);
  assert.equal(b.term.id, 7);
  assert.equal(b.term.reigning, 'a');
  assert.equal(b.me.key, 'a');
  assert.equal(byName(b, 'Vernius').pos.words, 'position 1 of 25, row 1, column 1');
  assert.equal(byName(b, 'Ecaz').pos.words, 'position 25 of 25, row 5, column 5');
  assert.equal(byName(b, 'Mutelli').pos.words, 'position 13 of 25, row 3, column 3');
});

test('the House name is the save\'s own (the prefix off) and the alias of Mikkarol is not applied here: the art table owns it', () => {
  const b = M.readBoard(answer());
  assert.ok(byName(b, 'Mikkarol'));
  assert.equal(byName(b, 'Mikkarol').key, 'mikkarol');
});

test('the owner is read strictly: completed with a winner the Landsraad has, open, or unreadable', () => {
  const b = M.readBoard(answer());
  assert.equal(byName(b, 'Vernius').owner, 'h');
  assert.equal(byName(b, 'Richese').owner, 'a');
  assert.equal(byName(b, 'Kenola').owner, 'open');
  const t = answer();
  const set = (name, patch) => Object.assign(t.tasks.find((x) => x.house_name === 'DA_House' + name), patch);
  set('Kenola', { completed: 1, winning_faction_id: null }); // completed with no winner
  set('Sor', { completed: 0, winning_faction_id: 1 });      // a winner with no completion
  set('Lindaren', { completed: 1, winning_faction_id: 3 }); // a faction the Landsraad does not have
  set('Argosaz', { completed: 2 });                         // not a boolean
  const c = M.readBoard(t);
  for (const n of ['Kenola', 'Sor', 'Lindaren', 'Argosaz']) assert.equal(byName(c, n).owner, 'unknown', n);
  assert.equal(c.counts.unknown, 4);
  assert.equal(c.countComplete, false, 'a count with an unreadable House is not offered as complete');
  assert.equal(c.counts.a + c.counts.h + c.counts.open + c.counts.unknown, 25);
});

test('every board case of M1: the readable ones give the case\'s counts, the others are not a board', () => {
  let ready = 0, notReady = 0;
  for (const c of cases()) {
    const tasks = (c.tasks || [...c.owners].map((o, i) => ({ board_index: i, house_name: 'DEMO_HOUSE_' + String(i + 1).padStart(2, '0'), completed: o === 'A' || o === 'H' ? 1 : 0, winning_faction_id: o === 'A' ? 1 : o === 'H' ? 2 : null, sysselraad: 0 })))
      .map((t, i) => ({ id: 1000 + i, goal_amount: 4000, contributions: [], rewards: [], my_contribution: 0, ...t }));
    const b = M.readBoard({ term: { term_id: 1 }, tasks, factions: [{ id: 1, name: 'Atreides' }, { id: 2, name: 'Harkonnen' }], character: null });
    if (c.expected.boardValid) {
      ready++;
      assert.equal(b.ready, true, c.id + ': ' + b.problems.join(' | '));
      assert.equal(b.counts.a, c.expected.counts.A, c.id);
      assert.equal(b.counts.h, c.expected.counts.H, c.id);
      assert.equal(b.counts.open, c.expected.counts.O, c.id);
      assert.equal(b.counts.unknown, c.expected.counts.unknown, c.id);
    } else {
      notReady++;
      assert.equal(b.ready, false, c.id + ' must not be a board');
      assert.equal(b.grid, null, c.id + ': no grid is drawn');
      assert.ok(b.problems.length > 0, c.id);
      assert.ok(b.houses.every((h) => h.pos === null && h.number === ''), c.id + ': no number is drawn from a board that failed its checks');
    }
  }
  assert.ok(ready >= 20 && notReady >= 4, `ready ${ready}, not ready ${notReady}`);
});

test('the sysselraad column is carried and never read as a result: the case with it set has the same counts as without', () => {
  const c = cases().find((x) => x.id === 'sysselraad-flag-set-no-evidence');
  const tasks = c.tasks.map((t, i) => ({ id: 1000 + i, goal_amount: 4000, contributions: [], rewards: [], ...t, sysselraad: i < 9 ? 1 : 0 }));
  const b = M.readBoard({ term: { term_id: 1 }, tasks, factions: [{ id: 1, name: 'Atreides' }, { id: 2, name: 'Harkonnen' }] });
  assert.equal(b.ready, true);
  assert.equal(b.houses[0].sysselraad, 1);
  assert.deepEqual([b.counts.a, b.counts.h, b.counts.open], [12, 4, 9]);
  const code = fs.readFileSync(path.join(lccDir, 'lcc-model.js'), 'utf8').split(NL).filter((l) => !l.trim().startsWith('//'));
  assert.equal(code.filter((l) => l.includes('sysselraad')).length, 1, 'the column is named in one line of code: where it is carried');
});

test('a read that is not a board says why, in sentences, and keeps the Houses that can be read for the list', () => {
  const dup = answer(); dup.tasks[8].board_index = 7;
  let b = M.readBoard(dup);
  assert.equal(b.ready, false);
  assert.match(b.problems.join(' '), /position 7 is used by more than one/);
  assert.equal(b.houses.length, 25, 'the list still has every House');
  assert.deepEqual(b.houses.map((h) => h.name), b.houses.map((h) => h.name).slice().sort((x, y) => x.localeCompare(y)), 'a failed board lists by name, not by a position that cannot be trusted');
  const few = answer(); few.tasks.pop();
  b = M.readBoard(few); assert.equal(b.ready, false); assert.match(b.problems.join(' '), /24 of 25/);
  const many = answer(); many.tasks.push({ ...many.tasks[0], id: 1 });
  b = M.readBoard(many); assert.equal(b.ready, false); assert.match(b.problems.join(' '), /26 Houses/);
  const out = answer(); out.tasks[3].board_index = 25;
  b = M.readBoard(out); assert.equal(b.ready, false); assert.match(b.problems.join(' '), /outside 0 to 24/);
  const frac = answer(); frac.tasks[3].board_index = 3.5;
  assert.equal(M.readBoard(frac).ready, false);
  const noTerm = answer(); noTerm.term = null;
  b = M.readBoard(noTerm); assert.equal(b.ready, false); assert.match(b.problems.join(' '), /term could not be read/);
  const dupName = answer(); dupName.tasks[2].house_name = dupName.tasks[1].house_name;
  b = M.readBoard(dupName); assert.equal(b.ready, false); assert.match(b.problems.join(' '), /name appears twice/);
  const oneFac = answer(); oneFac.factions = [{ id: 1, name: 'Atreides' }];
  assert.equal(M.readBoard(oneFac).ready, false);
  for (const junk of [null, undefined, 5, 'x', [], {}, { tasks: 'no' }, { tasks: [null, 3, 'a'] }]) {
    const j = M.readBoard(junk);
    assert.equal(j.ready, false);
    assert.equal(j.grid, null);
  }
});

test('the reward ledger: four states from the server, never one for another; no_row is "delivery unknown", never collected', () => {
  const b = M.readBoard(answer());
  const rich = byName(b, 'Richese');
  assert.deepEqual([rich.reached, rich.collected, rich.waiting, rich.unknownDelivery], [1, 1, 0, 0]);
  const ken = byName(b, 'Kenola');
  assert.deepEqual([ken.reached, ken.collected, ken.waiting, ken.unknownDelivery], [1, 0, 0, 1]);
  assert.equal(ken.tiers[0].state, 'no_row');
  assert.match(ken.tiers[0].word, /delivery unknown/);
  assert.doesNotMatch(ken.tiers[0].word, /collected/);
  const alex = byName(b, 'Alexin');
  assert.deepEqual([alex.reached, alex.collected, alex.waiting], [2, 1, 1]);
  const thor = byName(b, 'Thorvald');
  assert.deepEqual([thor.reached, thor.collected, thor.waiting], [5, 4, 1]);
  assert.equal(thor.tiers[4].swatch, true);
  assert.equal(thor.state, 'done');
  const mor = byName(b, 'Moritani');
  assert.deepEqual(mor.tiers.slice(0, 4).map((x) => x.state), ['delivered', 'waiting', 'delivered', 'no_row']);
  assert.equal(mor.unknownDelivery, 1);
  const none = byName(b, 'Ecaz'); // a House with no reward rows at all
  assert.equal(none.tierCount, 0);
  assert.equal(none.top, null);
  assert.equal(none.next, null);
  assert.equal(none.state, 'new');
  for (const h of b.houses) assert.equal(h.reached, h.collected + h.waiting + h.unknownDelivery, h.name + ': reached splits into collected, waiting and unknown');
  const r = b.rollup;
  assert.equal(r.reached, b.houses.reduce((s, h) => s + h.reached, 0));
  assert.equal(r.tiers, r.reached + r.notReached);
});

test('a state the page does not know, or one that contradicts the contribution, reads as delivery unknown and is never trusted', () => {
  const t = answer();
  const task = t.tasks.find((x) => x.house_name === 'DA_HouseRichese');
  task.rewards[0].reward_state = 'paid_in_full';
  task.rewards[3].reward_state = 'delivered'; // not reached, yet "delivered"
  const h = byName(M.readBoard(t), 'Richese');
  assert.equal(h.tiers[0].state, 'unknown');
  assert.equal(h.tiers[3].state, 'unknown');
  assert.equal(h.collected, 0);
  assert.equal(h.unknownDelivery, 1, 'the reached tier is delivery unknown; the tier not reached is not counted as reached');
  assert.equal(h.reached, 1);
});

test('the two scales stay apart, no goal or threshold is built in, and a missing goal is null', () => {
  const t = answer();
  t.tasks[0].goal_amount = null;
  t.tasks[1].goal_amount = 'abc';
  const b = M.readBoard(t);
  assert.equal(b.houses[0].goal, null);
  assert.equal(b.houses[1].goal, null);
  const a = byName(M.readBoard(answer()), 'Alexin');
  assert.equal(a.goal, 4000, 'the goal is the one the answer gives');
  assert.equal(a.mine, 2500);
  assert.equal(a.top, 6000);
  assert.deepEqual(a.next, { n: 3, threshold: 3600, remaining: 1100 });
  const th = byName(M.readBoard(answer()), 'Thorvald');
  assert.equal(th.mine, 6500, 'a contribution above the top tier is kept as it is');
  assert.equal(th.next, null);
  const src = fs.readFileSync(path.join(lccDir, 'lcc-model.js'), 'utf8').replace(/\/\/[^\n]*/g, '');
  for (const n of ['70000', '70,000', '4000', '4,000', '1200', '1,200', '2400', '3600', '4800', '6000', '700', '3500', '7000', '10500', '14000']) assert.ok(!src.includes(n), 'a goal or a threshold is written in the adapter: ' + n);
});

test('the progress of each faction is read by the faction\'s side and a leader is one side ahead, never a win', () => {
  const b = M.readBoard(answer());
  const sor = byName(b, 'Sor');
  assert.equal(typeof sor.prog.a, 'number');
  const t = answer();
  const x = t.tasks.find((k) => k.house_name === 'DA_HouseSor');
  x.contributions = [{ faction_id: 1, amount: 100 }, { faction_id: 2, amount: 300 }, { faction_id: 9, amount: 50 }];
  x.completed = 0; x.winning_faction_id = null;
  let h = byName(M.readBoard(t), 'Sor');
  assert.equal(h.leader, 'h');
  assert.equal(h.owner, 'open', 'a lead is not a claim');
  assert.deepEqual(h.progOther, [50], 'a faction the Landsraad does not have is kept apart');
  x.contributions = [{ faction_id: 1, amount: 300 }, { faction_id: 2, amount: 300 }];
  h = byName(M.readBoard(t), 'Sor'); assert.equal(h.leader, null);
  x.contributions = null; h = byName(M.readBoard(t), 'Sor'); assert.equal(h.leader, null);
});

test('the specialization is the path of the answer\'s own line, five at most, in lower case; an unknown line is none', () => {
  const b = M.readBoard(answer());
  const lines = new Set(b.houses.map((h) => h.track));
  assert.deepEqual([...lines].sort(), ['combat', 'crafting', 'exploration', 'gathering', 'sabotage']);
  const t = answer(); t.tasks[0].line = 'Spice'; t.tasks[1].line = null;
  const c = M.readBoard(t);
  assert.equal(c.houses[0].track, '');
  assert.equal(c.houses[1].track, '');
});

test('filters: Show, House and Specialization together, outside the fixed lists the filter is open and never throws', () => {
  const b = M.readBoard(answer());
  const n = (f) => b.houses.filter((h) => M.matches(h, f)).length;
  assert.equal(n({}), 25);
  assert.equal(n({ show: 'all', hs: 'any', spec: 'all' }), 25);
  assert.equal(n({ show: 'new' }) + n({ show: 'part' }) + n({ show: 'done' }), 25);
  assert.equal(n({ show: 'left' }), 25 - n({ show: 'done' }));
  assert.equal(n({ hs: 'open' }) + n({ hs: 'taken' }), 25);
  assert.equal(n({ hs: 'taken' }), 16, 'taken: 12 by Atreides and 4 by Harkonnen');
  assert.equal(n({ spec: 'combat' }), 5);
  assert.equal(n({ spec: 'combat', hs: 'open' }), b.houses.filter((h) => h.track === 'combat' && !h.taken).length);
  assert.equal(n({ show: 'nonsense', hs: 'nonsense', spec: 'nonsense' }), 25);
  assert.equal(n({ show: '__proto__', spec: 'constructor' }), 25);
  assert.equal(M.matches(b.houses[0], null), true);
});

test('sorting: each column, both ways, ties by name, and the grid order is never the order of the list', () => {
  const b = M.readBoard(answer());
  const names = (o) => M.sortHouses(b.houses, o, 'a').map((h) => h.name);
  const alpha = names({ sort: 'house' });
  assert.deepEqual(alpha, b.houses.map((h) => h.name).sort((x, y) => x.localeCompare(y)));
  assert.deepEqual(names({ sort: 'house', dir: 'desc' }), alpha.slice().reverse());
  const tiers = M.sortHouses(b.houses, { sort: 'tiers', dir: 'desc' }, 'a');
  assert.ok(tiers.every((h, i) => i === 0 || tiers[i - 1].reached >= h.reached));
  for (let i = 1; i < tiers.length; i++) if (tiers[i - 1].reached === tiers[i].reached) assert.ok(tiers[i - 1].name.localeCompare(tiers[i].name) <= 0, 'a tie is broken by name, ascending');
  const st = M.sortHouses(b.houses, { sort: 'status' }, 'a');
  assert.ok(st[0].owner === 'a');
  const loc = M.sortHouses(b.houses, { sort: 'location' }, 'a');
  assert.equal(loc[loc.length - 1].loc.known, false, 'a House with no place sorts last');
  assert.deepEqual(M.sortHouses(b.houses, { sort: 'nonsense' }, 'a').map((h) => h.name), alpha, 'an unknown column is the House column');
  const before = b.houses.map((h) => h.id).join();
  M.sortHouses(b.houses, { sort: 'tiers', dir: 'desc' }, 'a');
  assert.equal(b.houses.map((h) => h.id).join(), before, 'sorting returns a new array and leaves the grid order alone');
  assert.deepEqual(b.grid.map((h) => h.index), [...Array(25).keys()]);
  const pos = M.sortHouses(b.houses, { sort: 'position' }, 'a');
  assert.deepEqual(pos.map((h) => h.index), [...Array(25).keys()]);
});

test('grouping by region puts the Houses with no place last under one heading; by specialization, one group per path', () => {
  const b = M.readBoard(answer());
  const g = M.sortHouses(b.houses, { sort: 'house', group: 'region' }, 'a');
  const regions = g.map((h) => M.groupOf(h, 'region'));
  const firstEmpty = regions.indexOf('');
  assert.ok(firstEmpty > 0);
  assert.ok(regions.slice(firstEmpty).every((r) => r === ''), 'unplaced Houses are together at the end');
  const s = M.sortHouses(b.houses, { sort: 'house', group: 'spec' }, 'a').map((h) => h.track);
  assert.deepEqual(s, s.slice().sort(), 'one run per specialization');
});

test('positions(): numbers for a board, none for anything else', () => {
  const t = answer().tasks;
  const p = M.positions(t);
  assert.equal(p.size, 25);
  assert.equal(p.get(9001).number, '01');
  assert.equal(p.get(9025).number, '25');
  t[1].board_index = 0;
  assert.equal(M.positions(t).size, 0);
  assert.equal(M.positions(null).size, 0);
  assert.equal(M.position(-1), null);
  assert.equal(M.position(25), null);
  assert.equal(M.position(1.5), null);
});

test('the adapter is pure: no DOM, no network, no storage, no write, no markup', () => {
  const src = fs.readFileSync(path.join(lccDir, 'lcc-model.js'), 'utf8').split(NL).filter((l) => !l.trim().startsWith('//')).join(NL);
  for (const bad of ['document', 'window.', 'fetch(', 'XMLHttpRequest', 'localStorage', 'sessionStorage', 'innerHTML', 'outerHTML', 'insertAdjacent', 'eval(', 'new Function', '/api/', 'RegExp(', 'new RegExp']) assert.ok(!src.includes(bad), 'the adapter uses ' + bad);
});
