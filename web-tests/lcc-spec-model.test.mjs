// The Specializations model of the Command Center (M5): the level table is the editor's own, the level is read from it and never from a linear formula, the number typed for Add XP is checked
// the way the editor checks it, and every state of the synthetic cases of M1 (docs/design/landsraad-command-center/fixtures/api-player-specs.cases.json) reads as that document expects.
import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { createRequire } from 'node:module';
import { lccDir, root, read } from './lcc-load.mjs';

const S = createRequire(import.meta.url)(path.join(lccDir, 'lcc-spec.js'));
const CASES = JSON.parse(read('docs', 'design', 'landsraad-command-center', 'fixtures', 'api-player-specs.cases.json')).cases;
const byId = (id) => CASES.find((c) => c.id === id);
const rowOf = (vm, key) => vm.rows.find((r) => r.key === key);

test('the level table is the editor\'s own: 101 starts, the same numbers as internal/ops/specialization.go, strictly rising, ending on the largest XP', () => {
  const go = fs.readFileSync(path.join(root, 'internal', 'ops', 'specialization.go'), 'utf8').replace(/\r/g, '');
  const body = go.slice(go.indexOf('var specCumulativeXP = []int64{') + 'var specCumulativeXP = []int64{'.length);
  const goTable = body.slice(0, body.indexOf('\n}')).split(/[\s,]+/).filter(Boolean).map(Number);
  assert.equal(goTable.length, 101);
  assert.deepEqual([...S.LEVEL_XP], goTable);
  for (let i = 1; i < S.LEVEL_XP.length; i++) assert.ok(S.LEVEL_XP[i] > S.LEVEL_XP[i - 1]);
  assert.equal(S.LEVEL_XP[0], 0);
  assert.equal(S.LEVEL_XP[100], 44182);
});

test('the level comes from the table: the boundary values, and nothing above the cap', () => {
  const want = [[0, 0], [1, 0], [99, 0], [100, 1], [101, 1], [204, 1], [205, 2], [1106, 8], [1107, 9], [43557, 98], [43558, 99], [44181, 99], [44182, 100], [99999, 100], [-5, 0]];
  for (const [xp, lv] of want) assert.equal(S.levelOf(xp), lv, 'xp ' + xp);
  assert.equal(S.levelOf(NaN), 0);
  assert.equal(S.levelOf(null), 0);
  // a straight line (xp / 44182 * 100) would put 22,091 XP at level 50; the table puts it at level 64
  assert.equal(S.levelOf(22091), 64);
  assert.equal(S.levelOf(S.LEVEL_XP[50] - 1), 49);
  assert.equal(S.levelOf(S.LEVEL_XP[50]), 50);
});

test('XP to the next level is the distance to the next start, and 0 at the last level', () => {
  assert.equal(S.xpToNext(0), 100);
  assert.equal(S.xpToNext(1), 99);
  assert.equal(S.xpToNext(100), 105);
  assert.equal(S.xpToNext(44181), 1);
  assert.equal(S.xpToNext(44182), 0);
});

test('Add XP: a whole number from 1 to 44,182; zero, a minus sign and everything else is stopped before it is sent (#1032)', () => {
  // the number that is judged is the number the page sends (+value), so the spellings of a whole number that the classic list accepts pass too: 1e3 is 1000 and 1.0 is 1 (#899)
  const ok = ['1', '1000', '+5', ' 7 ', '44182', '00012', '1e3', '1E3', '+1e3', '1.0', '1.', '1000.00', '5e0', '4.4182e4', '150e-1'];
  for (const t of ok) { assert.equal(S.amountProblem(t, 44182), '', JSON.stringify(t)); assert.ok(Number.isSafeInteger(+t) && +t >= 1 && +t <= 44182, 'what is sent for ' + t + ' is a whole number in range'); }
  const bad = ['-1', '-1000', '-44182', '-44183', '-1e3', '-0', '-5.0', '-99999999999999999999', '', '   ', '0', '-0', '+0', '0.0', '0e5', '1.5', '0.5', '1e-1', '1e5', '1e400', '-1e400', '1,000', 'NaN', 'Infinity', 'abc', '0x10', '1e', 'e3', '.', '+', '44183', '-44183', '99999999999999999999', '--5', '5-', '1 000', null, undefined, 12];
  for (const t of bad) assert.match(S.amountProblem(t, 44182), /whole number of XP from 1 to 44,182\. XP is added here, never removed \(use Reset to take it back\)\. Nothing was sent\./, JSON.stringify(t));
  // the limit is the one the save reports
  assert.equal(S.amountProblem('50000', 60000), '');
  assert.match(S.amountProblem('50000', 44182), /Nothing was sent/);
});

test('a read that cannot be used says so and draws nothing from it', () => {
  for (const bad of [null, undefined, {}, { rows: 'x', unassigned: [] }, { rows: [], unassigned: null, maxXp: 44182 }]) { const vm = S.readSpecs(bad, null); assert.equal(vm.ok, false); assert.ok(vm.problem); }
  assert.equal(S.readSpecs({ rows: [], unassigned: [], maxXp: 0 }, null).ok, false);
  assert.equal(S.readSpecs({ rows: [], unassigned: [], maxXp: 44182 }, null).ok, false, 'no tracks');
  const vm = S.readSpecs({ rows: [{ track: 'Combat', number: 4, xp: 5, level: 0, keystoneOwned: 0, keystoneTotal: 41 }, { track: 'combat', number: 4, xp: 9, level: 0 }, null, { number: 1 }], unassigned: [], maxXp: 44182 }, null);
  assert.equal(vm.rows.length, 1, 'a repeated or nameless row is left out');
  assert.equal(vm.rows[0].xp, 5);
});

test('the lanes come in the approved order, whatever order the save answers in', () => {
  const vm = S.readSpecs(byId('mixed-five-tracks').response, null);
  assert.deepEqual(vm.rows.map((r) => r.key), ['crafting', 'gathering', 'exploration', 'combat', 'sabotage']);
});

test('XP cap and keystones are separate: level 100 with no keystone is not complete, all keystones at level 0 is not level 100, only both is the full path', () => {
  const cap = rowOf(S.readSpecs(byId('xp-44182-cap-zero-keystones').response, null), 'combat');
  assert.equal(cap.capped, true); assert.equal(cap.keyComplete, false); assert.equal(cap.kind, 'cap'); assert.equal(cap.level, 100);
  assert.equal(S.masteryText(cap), 'XP CAP REACHED');
  assert.equal(S.laneBadge(cap), '✦ XP CAP REACHED');
  assert.ok(!/KEYSTONES COMPLETE|FULL/.test(S.masteryText(cap)));
  const keys = rowOf(S.readSpecs(byId('xp-0-all-keystones-41-of-41').response, null), 'combat');
  assert.equal(keys.capped, false); assert.equal(keys.keyComplete, true); assert.equal(keys.kind, 'keys'); assert.equal(keys.level, 0);
  assert.equal(S.masteryText(keys), 'KEYSTONES COMPLETE');
  assert.ok(!/CAP REACHED|LEVEL 100/.test(S.masteryText(keys)));
  const both = rowOf(S.readSpecs(byId('both-complete').response, null), 'combat');
  assert.equal(both.kind, 'full');
  assert.equal(S.masteryText(both), 'FULL PATH COMPLETE');
  assert.equal(S.laneBadge(both), '✦ FULL PATH COMPLETE');
  const fresh = rowOf(S.readSpecs(byId('xp-0-keystones-0-of-41').response, null), 'combat');
  assert.equal(fresh.kind, 'progress'); assert.equal(S.masteryText(fresh), 'XP CAP NOT REACHED');
});

test('a save with no keystone map is never complete: the total 0 is "Not in the save", not 0 of 0', () => {
  const vm = S.readSpecs(byId('keystone-total-zero-no-map').response, null);
  const r = rowOf(vm, 'combat');
  assert.equal(r.keyKnown, false); assert.equal(r.keyComplete, false); assert.equal(r.kind, 'cap');
  assert.equal(vm.totals.total, 0);
  assert.equal(vm.totals.keysPartial, true);
});

test('every case of M1 reads as the document expects', () => {
  for (const c of CASES) {
    const vm = S.readSpecs(c.response, null);
    assert.equal(vm.ok, true, c.id);
    const e = c.expect.combat, r = rowOf(vm, 'combat');
    assert.equal(r.capped, e.xpCap, c.id + ' cap');
    assert.equal(r.keyComplete, e.keystonesComplete, c.id + ' keystones');
    assert.equal(r.known, e.xpEditingEnabled, c.id + ' xp editing');
    assert.equal(vm.resetCount, c.response.resetCount, c.id + ' reset count comes from the save, not the row count');
    assert.equal(vm.unassigned.length, c.response.unassigned.length, c.id);
    if (c.expect.resetKnownOnly !== undefined) assert.equal(vm.resetCount, c.expect.resetKnownOnly);
  }
});

test('a track whose number is not known has no XP, no level and no cap; it is flagged, and the other four are untouched', () => {
  const vm = S.readSpecs(byId('conflicting-learned-number-leaves-combat-unresolved').response, null);
  const c = rowOf(vm, 'combat');
  assert.equal(c.known, false); assert.equal(c.xp, null); assert.equal(c.level, null); assert.equal(c.kind, 'unresolved'); assert.equal(c.capped, false);
  assert.equal(S.laneBadge(c), 'TRACK NOT IDENTIFIED');
  assert.equal(vm.anyUnidentified, true);
  assert.ok(vm.unassigned.length >= 1);
  assert.equal(vm.rows.filter((r) => r.known).length, 4);
  assert.equal(vm.totals.atCap, vm.rows.filter((r) => r.known && r.xp >= vm.maxXp).length);
});

test('a row with a number but no XP row is a track at 0 XP, editable, not hidden', () => {
  const vm = S.readSpecs(byId('xp-0-keystones-0-of-41').response, null);
  assert.equal(vm.rows.length, 5);
  for (const r of vm.rows) { assert.equal(r.known, true); assert.equal(r.xp, 0); assert.equal(r.level, 0); }
});

test('the level the save stores and the level of its XP are shown apart, and a difference is flagged, not corrected', () => {
  const vm = S.readSpecs(byId('raw-row-xp-44182-level-10-mismatch').response, null);
  const r = rowOf(vm, 'combat');
  assert.equal(r.level, 100); assert.equal(r.storedLevel, 10); assert.equal(r.levelMismatch, true);
  // the editor's own fractional levels are not a mismatch
  const ok = S.readSpecs(byId('xp-1').response, null);
  assert.equal(rowOf(ok, 'combat').levelMismatch, false);
  const ok2 = S.readSpecs(byId('xp-44181-near-cap-keystones-20-of-41').response, null);
  assert.equal(rowOf(ok2, 'combat').levelMismatch, false);
});

test('if the save\'s largest XP is not the one this table ends on, no level is worked out from it', () => {
  const resp = JSON.parse(JSON.stringify(byId('mixed-five-tracks').response));
  resp.maxXp = 50000;
  const vm = S.readSpecs(resp, null);
  assert.equal(vm.levelsTrusted, false);
  const r = vm.rows.find((x) => x.known && x.storedLevel !== null);
  assert.equal(r.level, Math.floor(r.storedLevel));
  assert.equal(r.toNext, null);
});

test('the Houses of a path come from the houses given, by their line, and are not made up when there are none', () => {
  const houses = [{ id: 1, name: 'DA_HouseEcaz', line: 'Crafting' }, { id: 2, name: 'DA_HouseAlexin', line: 'crafting' }, { id: 3, name: 'DA_HouseSor', line: 'Combat' }, { id: 4, name: 'DA_HouseX', line: '' }];
  const vm = S.readSpecs(byId('mixed-five-tracks').response, houses);
  assert.deepEqual(rowOf(vm, 'crafting').houses.map((h) => h.id), [2, 1], 'by name');
  assert.deepEqual(rowOf(vm, 'combat').houses.map((h) => h.id), [3]);
  assert.deepEqual(rowOf(vm, 'gathering').houses, []);
  const none = S.readSpecs(byId('mixed-five-tracks').response, null);
  assert.equal(rowOf(none, 'crafting').houses, null);
  assert.equal(none.housesKnown, false);
});

test('the words of the remaining XP are exact at the edges', () => {
  const vm = S.readSpecs(byId('xp-44181-near-cap-keystones-20-of-41').response, null);
  const r = rowOf(vm, 'combat');
  assert.equal(S.xpRemainText(vm, r), '1 XP TO LEVEL 100 · 1 XP TO CAP');
  const cap = S.readSpecs(byId('both-complete').response, null);
  assert.equal(S.xpRemainText(cap, rowOf(cap, 'combat')), 'XP CAP REACHED — NO FURTHER XP LEVELS');
  const fresh = S.readSpecs(byId('xp-0-keystones-0-of-41').response, null);
  assert.equal(S.xpRemainText(fresh, rowOf(fresh, 'combat')), '100 XP TO LEVEL 1 · 44,182 XP TO CAP');
});

test('the hero totals: tracks at the cap, keystones bought and the largest XP, all from the save', () => {
  const vm = S.readSpecs(byId('mixed-five-tracks').response, null);
  assert.equal(vm.totals.tracks, 5);
  assert.equal(vm.totals.atCap, vm.rows.filter((r) => r.xp >= vm.maxXp).length);
  assert.equal(vm.totals.owned, vm.rows.reduce((s, r) => s + r.keyOwned, 0));
  assert.equal(vm.totals.total, 5 * 41);
});
