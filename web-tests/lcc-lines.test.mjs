// The Sysselraad evaluator of the Command Center (M4): the twelve lines of the fixed 5 by 5 board, which of them one faction holds, which are blocked or one House away, how many are still possible,
// the House counts and the 13-House threshold, and the verdict that says which fact decides. It is a pure reading of 25 owners (lcc-model.js): no DOM, no write. It is checked against every board case of
// M1 (docs/design/landsraad-command-center/fixtures/board-victory-cases.json), against one board per line and faction, and against boards that must fail closed. No game save is read.
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
const cases = () => clean(JSON.parse(fs.readFileSync(path.join(fixDir, 'board-victory-cases.json'), 'utf8'))).cases;
const lineDefs = () => clean(JSON.parse(fs.readFileSync(path.join(fixDir, 'board-victory-cases.json'), 'utf8'))).lines;

// owner letters (A, H, O open, ? unreadable) -> the 25 owners the evaluator reads
const owners = (s) => [...s].map((c) => ({ A: 'a', H: 'h', O: 'open' })[c] || 'unknown');
const ev = (s, evidence) => M.evaluateOwners(owners(s), evidence || null, { termId: 7 });
const status = (r) => r.lines.map((l) => l.status.replace(/-a$/, '-A').replace(/-h$/, '-H'));
const letters = (n, ...marks) => { const a = Array(n).fill('O'); for (const [i, c] of marks) a[i] = c; return a.join(''); };
const ids = (xs) => xs.map((l) => l.id);

test('the twelve lines are the five rows, the five columns and the two diagonals of the fixed board, in that order, and they cannot be changed', () => {
  assert.equal(M.LINES.length, 12);
  assert.deepEqual(M.LINES.map((l) => ({ id: l.id, indexes: [...l.indexes] })), lineDefs(), 'the same twelve as the fixture file, in the same order');
  assert.deepEqual([...M.LINES[7].indexes], [2, 7, 12, 17, 22], 'column 3');
  assert.deepEqual([...M.LINES[10].indexes], [0, 6, 12, 18, 24], 'the main diagonal');
  assert.deepEqual([...M.LINES[11].indexes], [4, 8, 12, 16, 20], 'the opposite diagonal');
  assert.deepEqual(M.LINES.map((l) => l.name), ['Row 1', 'Row 2', 'Row 3', 'Row 4', 'Row 5', 'Column 1', 'Column 2', 'Column 3', 'Column 4', 'Column 5', 'Main diagonal', 'Opposite diagonal']);
  const onLines = Array(25).fill(0);
  for (const l of M.LINES) { assert.equal(new Set(l.indexes).size, 5); for (const i of l.indexes) onLines[i]++; }
  assert.deepEqual(onLines.filter((n) => n < 2 || n > 4), [], 'every House is on two, three or four lines');
  assert.equal(onLines[12], 4, 'the middle House is on four lines');
  assert.ok(Object.isFrozen(M.LINES) && Object.isFrozen(M.LINES[0]) && Object.isFrozen(M.LINES[0].indexes));
  assert.equal(M.MAJORITY, 13);
});

test('each of the twelve lines is held when one faction has all five Houses of it, for each faction', () => {
  for (const [li, line] of M.LINES.entries()) {
    for (const f of ['A', 'H']) {
      const s = letters(25, ...line.indexes.map((i) => [i, f]));
      const r = ev(s);
      assert.deepEqual(ids(r.held), [line.id], line.id + ' ' + f + ': exactly that line is held (the lines that cross it hold only one House of it)');
      assert.equal(r.held[0].side, f.toLowerCase());
      assert.equal(r.lines[li].status, 'complete-' + f.toLowerCase());
      assert.equal(r.verdict, 'line_held', line.id);
      assert.equal(r.winner, f.toLowerCase());
      assert.equal(r.counts[f.toLowerCase()], 5);
      assert.equal(r.conflict, false);
    }
  }
});

test('every board case of M1: the status of each line, the held and one-away lines, the leader, the margin, the 13-House flag and the verdict', () => {
  let n = 0;
  for (const c of cases()) {
    const e = c.expected;
    if (!e.boardValid) continue;
    const letters_ = c.owners;
    const evidence = c.source.cutoffVerified ? { verified: true, owners: owners(c.source.cutoffOwners) } : null;
    const r = M.evaluateOwners(owners(letters_), evidence, { termId: 1 });
    n++;
    assert.equal(r.valid, true, c.id);
    assert.deepEqual(r.counts, { a: e.counts.A, h: e.counts.H, open: e.counts.O, unknown: e.counts.unknown }, c.id);
    assert.deepEqual(status(r), e.lineStatus, c.id + ': the twelve line statuses');
    assert.deepEqual(ids(r.held), e.completeLines, c.id + ': held lines');
    assert.deepEqual(r.oneAway.map((l) => [l.id, l.side.toUpperCase(), l.missing[0]]), e.oneAwayLines.map((x) => [x.line, x.owner, x.missing_index]), c.id + ': one House away');
    assert.equal(r.leader, e.countLeader === null ? null : e.countLeader.toLowerCase(), c.id + ': leader');
    assert.equal(r.margin, e.margin, c.id + ': margin');
    assert.equal(r.thresholdReached, e.thresholdReached, c.id + ': 13 Houses or more');
    // the fixture file names the two results line_confirmed and majority_confirmed; the page's own names say less (a line is held, a count is read at a cutoff)
    assert.equal(r.verdict, { line_confirmed: 'line_held', majority_confirmed: 'majority_at_cutoff' }[e.verdict] || e.verdict, c.id + ': verdict');
    assert.equal(r.winner, e.winner === null ? null : e.winner.toLowerCase(), c.id + ': the faction the verdict names');
    assert.equal(r.vote.verified, e.majorityAtCutoff.verified, c.id);
    assert.equal(r.vote.winner, e.majorityAtCutoff.winner === null ? null : e.majorityAtCutoff.winner.toLowerCase(), c.id + ': the cutoff majority');
    if (e.majorityAtCutoff.verified) assert.deepEqual(r.vote.counts, { a: e.majorityAtCutoff.cutoffCounts.A, h: e.majorityAtCutoff.cutoffCounts.H, open: e.majorityAtCutoff.cutoffCounts.O }, c.id + ': counts at the cutoff');
    // the board is never ordered by anything else than its positions
    assert.deepEqual(r.lines.map((l) => l.id), lineDefs().map((l) => l.id), c.id);
  }
  assert.ok(n >= 24, 'valid cases read: ' + n);
});

test('the boards of M1 that are not boards have no counts, no lines and the verdict unknown, and each says why through the reader', () => {
  let n = 0;
  for (const c of cases()) {
    if (c.expected.boardValid) continue;
    const tasks = (c.tasks || [...c.owners].map((o, i) => ({ board_index: i, house_name: 'DEMO_HOUSE_' + (i + 1), completed: o === 'A' || o === 'H' ? 1 : 0, winning_faction_id: o === 'A' ? 1 : o === 'H' ? 2 : null }))).map((t, i) => ({ id: 1000 + i, goal_amount: 1, contributions: [], rewards: [], ...t }));
    const b = M.readBoard({ term: { term_id: 1 }, tasks, factions: [{ id: 1, name: 'Atreides' }, { id: 2, name: 'Harkonnen' }] });
    const r = M.evaluate(b, null);
    n++;
    assert.equal(r.valid, false, c.id);
    assert.equal(r.counts, null, c.id);
    assert.equal(r.verdict, 'unknown', c.id);
    assert.equal(r.winner, null);
    assert.equal(r.held.length, 0);
    assert.equal(r.potential.total, null, c.id + ': no number of potential lines is made up');
    assert.equal(r.majority, null);
    assert.ok(r.why.length > 0, c.id);
    assert.deepEqual(status(r), Array(12).fill('unknown'), c.id);
  }
  assert.ok(n >= 4);
  for (const bad of [null, undefined, [], Array(24).fill('a'), Array(26).fill('a'), 'x'.repeat(25)]) assert.equal(M.evaluateOwners(bad, null).valid, false);
  assert.equal(M.evaluate(null, null).valid, false);
  assert.equal(M.evaluate({}, null).valid, false);
});

test('mixed ownership: a line with a House of each faction is blocked for both and is not counted as possible', () => {
  const r = ev(letters(25, [0, 'A'], [1, 'H']));
  assert.equal(r.lines[0].status, 'blocked', 'row 1 has Atreides and Harkonnen');
  assert.equal(r.blockedLines, 1);
  assert.equal(r.potential.total, 11, 'twelve lines less the blocked one');
  assert.equal(r.potential.a, 10, 'the eleven less the column that only Harkonnen has touched');
  assert.equal(r.potential.h, 9, 'the eleven less the column and the diagonal that only Atreides has touched');
  // the lines that cross the two Houses are open for one faction only
  assert.equal(r.lines.find((l) => l.id === 'col0').status, 'open-a');
  assert.equal(r.lines.find((l) => l.id === 'col1').status, 'open-h');
  assert.equal(r.lines.find((l) => l.id === 'diag-main').status, 'open-a');
  assert.deepEqual(r.lines.find((l) => l.id === 'col0').by.a, [0]);
  assert.deepEqual(r.lines.find((l) => l.id === 'col0').missing, [5, 10, 15, 20]);
  // potential for a faction: a line only the other faction has touched is not its own
  const t = ev(letters(25, [0, 'A'], [5, 'A'], [10, 'H'], [11, 'H']));
  assert.equal(t.lines.find((l) => l.id === 'col0').status, 'blocked');
  assert.equal(t.lines.find((l) => l.id === 'row2').status, 'open-h');
  assert.equal(t.potential.a, t.lines.filter((l) => l.status === 'open-a' || l.status === 'open-any').length);
  assert.equal(t.potential.h, t.lines.filter((l) => l.status === 'open-h' || l.status === 'open-any').length);
  assert.ok(t.potential.a < t.potential.total || t.potential.h < t.potential.total, 'a line only one faction can use is not counted for the other');
});

test('open Houses: a line with nothing claimed counts for both factions and once in the total; a board of only open Houses has twelve', () => {
  const r = ev('O'.repeat(25));
  assert.deepEqual(status(r), Array(12).fill('open-any'));
  assert.deepEqual(r.potential, { total: 12, a: 12, h: 12, exact: true });
  assert.equal(r.verdict, 'none');
  assert.equal(r.leader, null);
  assert.equal(r.margin, 0);
  assert.equal(r.majority, null);
  assert.equal(r.thresholdReached, false);
  assert.equal(r.held.length, 0);
  assert.equal(r.oneAway.length, 0);
  assert.deepEqual(r.counts, { a: 0, h: 0, open: 25, unknown: 0 });
  assert.equal(r.countComplete, true);
});

test('one House away: four Houses of one faction and one open; the missing House is named by its position', () => {
  const r = ev(letters(25, [0, 'A'], [1, 'A'], [2, 'A'], [3, 'A']));
  const row1 = r.lines[0];
  assert.equal(row1.status, 'one-away-a');
  assert.deepEqual(row1.missing, [4]);
  assert.deepEqual(r.oneAway.map((l) => l.id), ['row0']);
  assert.equal(r.verdict, 'pending_vote', 'a lead before a verified cutoff is a lead');
  const four = ev(letters(25, [0, 'A'], [1, 'A'], [2, 'A'], [3, 'A'], [4, 'H']));
  assert.equal(four.lines[0].status, 'blocked', 'the fifth House is taken by the other faction: no longer one away');
  assert.equal(four.oneAway.length, 0);
});

test('the 12 and 13 boundary: 13 Houses is the count that cannot be overtaken, 12 is a lead only, and neither is a vote result', () => {
  const run = (a, h) => ev('A'.repeat(a) + 'H'.repeat(h) + 'O'.repeat(25 - a - h));
  const r12 = run(12, 12);
  assert.equal(r12.leader, null, 'even');
  assert.equal(r12.thresholdReached, false);
  assert.equal(r12.majority, null);
  const r12b = run(12, 13);
  assert.equal(r12b.leader, 'h');
  assert.equal(r12b.majority, 'h', '13 of 25');
  const a13 = run(13, 0);
  assert.equal(a13.majority, 'a');
  assert.equal(a13.thresholdReached, true);
  const a12 = run(12, 0);
  assert.equal(a12.leader, 'a');
  assert.equal(a12.majority, null, '12 of 25 is not more than half');
  assert.equal(a12.thresholdReached, false);
  // with 13, the verdict is never a majority result: there is no verified start of the Voting Session. Thirteen Houses laid so that no line is complete are only pending.
  for (const r of [a13, run(13, 12), r12b, a12]) assert.notEqual(r.verdict, 'majority_at_cutoff');
  assert.equal(a13.verdict, 'line_held', 'thirteen Houses filled in order complete two rows: that is a line, and it is said as a line');
  const scattered = ev('HAHAHAHAHAHAAAHAHAHAHAHAH'); // the case of M1: 13 Atreides, 12 Harkonnen, no line
  assert.equal(scattered.counts.a, 13);
  assert.equal(scattered.held.length, 0);
  assert.equal(scattered.majority, 'a');
  assert.equal(scattered.verdict, 'pending_vote');
  assert.equal(scattered.vote.verified, false);
  assert.equal(scattered.vote.winner, null);
  assert.equal(scattered.winner, null);
  // 12 Houses of one faction and 12 of the other, one open: nobody leads, nobody has the threshold
  assert.equal(ev('HAHAHAHAHAHAOAHAHAHAHAHAH').majority, null);
});

test('a verified cutoff decides the count, below 13 as well, and a tie decides nothing; evidence that is not exactly a snapshot is no evidence', () => {
  const cur = 'AOAHOOAOHAOOHOAHOHAOAAHOO'; // A 8, H 6
  assert.equal(ev(cur).verdict, 'pending_vote');
  const ok = ev(cur, { verified: true, owners: owners(cur) });
  assert.equal(ok.verdict, 'majority_at_cutoff');
  assert.equal(ok.winner, 'a');
  assert.equal(ok.vote.phase, 'cutoff_verified');
  assert.deepEqual(ok.vote.counts, { a: 8, h: 6, open: 11 });
  const tie = ev(cur, { verified: true, owners: owners('AAAAHHHHOOOOOOOOOOOOOOOOO') });
  assert.equal(tie.verdict, 'none');
  assert.equal(tie.winner, null);
  for (const bad of [{ verified: false, owners: owners(cur) }, { owners: owners(cur) }, { verified: true }, { verified: true, owners: owners(cur).slice(1) }, { verified: true, owners: [...owners(cur).slice(1), 'unknown'] }, { verified: 'yes', owners: owners(cur) }, 'AOAH', 7, {}]) {
    const r = ev(cur, bad);
    assert.equal(r.vote.verified, false, JSON.stringify(bad));
    assert.equal(r.verdict, 'pending_vote');
    assert.equal(r.winner, null);
  }
});

test('both factions with a line is a conflict and nobody is chosen; a line against the other side\'s verified majority is a conflict too; the same faction is both', () => {
  const both = ev('AAAAA' + 'O'.repeat(15) + 'HHHHH');
  assert.equal(both.verdict, 'conflict');
  assert.equal(both.conflict, true);
  assert.equal(both.winner, null);
  assert.deepEqual(ids(both.held), ['row0', 'row4']);
  assert.match(both.conflictReason, /Both factions hold a line/);
  const reversed = ev('HHHHH' + 'O'.repeat(15) + 'AAAAA');
  assert.equal(reversed.verdict, 'conflict');
  assert.equal(reversed.winner, null, 'the order of the lines never decides');
  const line = 'AAAAAOHOOOOOHOOOOOOOHOOOO';
  const other = ev(line, { verified: true, owners: owners('OOOOOHHOHOOHHOOHOOOOHOOOO') });
  assert.equal(other.verdict, 'conflict');
  assert.equal(other.winner, null);
  const same = ev(line, { verified: true, owners: owners(line) });
  assert.equal(same.verdict, 'line_held');
  assert.equal(same.winner, 'a');
  assert.equal(same.majorityAlsoRecorded, true);
  assert.equal(ev(line).majorityAlsoRecorded, false);
});

test('two lines of one faction are both kept, and a board of 25 of one faction holds all twelve', () => {
  const r = ev('AAAAAAOOOOAOOOOAOOOOAOOHH');
  assert.deepEqual(ids(r.held), ['row0', 'col0']);
  assert.equal(r.verdict, 'line_held');
  assert.equal(r.winner, 'a');
  const all = ev('A'.repeat(25));
  assert.equal(all.held.length, 12);
  assert.equal(all.potential.total, 0);
  assert.equal(all.majority, 'a');
});

test('an unreadable House fails closed: it is neither side\'s nor open, the count is incomplete, no number of potential lines is made up, and a line held elsewhere still stands', () => {
  const r = ev(letters(25, [12, '?']));
  assert.deepEqual(r.counts, { a: 0, h: 0, open: 24, unknown: 1 });
  assert.equal(r.countComplete, false);
  assert.equal(r.potential.exact, false, 'no exact count of potential lines');
  assert.equal(r.verdict, 'unknown');
  const centre = r.lines.filter((l) => l.indexes.includes(12));
  assert.equal(centre.length, 4);
  assert.ok(centre.every((l) => l.status === 'unknown'), 'the four lines through the unreadable House are unknown, not open');
  assert.deepEqual(r.lines.find((l) => l.id === 'row0').status, 'open-any');
  assert.equal(r.unknownLines, 4);
  assert.deepEqual(centre[0].by.unknown, [12]);
  assert.equal(centre[0].missing.length, 0, 'an unknown line names no missing House');
  // a line held away from the unreadable House stands, and is the verdict
  const held = ev(letters(25, [12, '?'], ...[20, 21, 22, 23, 24].map((i) => [i, 'H'])));
  assert.equal(held.verdict, 'line_held');
  assert.equal(held.winner, 'h');
  assert.equal(held.countComplete, false);
  assert.equal(held.potential.exact, false);
  // an unreadable House is never read as an open House that could complete a line
  const four = ev(letters(25, [0, 'A'], [1, 'A'], [2, 'A'], [3, 'A'], [4, '?']));
  assert.equal(four.lines[0].status, 'unknown');
  assert.equal(four.oneAway.length, 0);
  // the reader gives the same: a taken House with no winner is unreadable
  const tasks = [...Array(25).keys()].map((i) => ({ id: 1 + i, board_index: i, house_name: 'DA_HouseDemo' + i, completed: i === 12 ? 1 : 0, winning_faction_id: null, goal_amount: 1, contributions: [], rewards: [] }));
  const b = M.readBoard({ term: { term_id: 3 }, tasks, factions: [{ id: 1, name: 'Atreides' }, { id: 2, name: 'Harkonnen' }] });
  const e = M.evaluate(b, null);
  assert.equal(e.valid, true);
  assert.equal(e.counts.unknown, 1);
  assert.equal(e.potential.exact, false);
});

test('the reading follows board_index, never the order of the list it was given, and it carries the term and a key that changes with the Houses', () => {
  const mk = (rev) => {
    const tasks = [...Array(25).keys()].map((i) => ({ id: 100 + i, board_index: i, house_name: 'DA_HouseDemo' + String.fromCharCode(97 + i), completed: i % 5 === 0 ? 1 : 0, winning_faction_id: i % 5 === 0 ? 1 : null, goal_amount: 1, contributions: [], rewards: [] }));
    if (rev) tasks.reverse();
    return M.readBoard({ term: { term_id: 9 }, tasks, factions: [{ id: 1, name: 'Atreides' }, { id: 2, name: 'Harkonnen' }] });
  };
  const a = M.evaluate(mk(false), null), b = M.evaluate(mk(true), null);
  assert.deepEqual(a, b, 'the same board in another order is the same reading');
  assert.deepEqual(ids(a.held), ['col0'], 'the first column: positions 0, 5, 10, 15, 20');
  assert.equal(a.termId, 9);
  assert.equal(a.key.startsWith('9:'), true);
  assert.equal(a.key.length, 2 + 25);
  const c = M.evaluateOwners(owners('A'.repeat(25)), null, { termId: 10 });
  assert.notEqual(c.key, a.key);
  assert.notEqual(M.evaluateOwners(owners('A'.repeat(25)), null, { termId: 9 }).key, M.evaluateOwners(owners('A'.repeat(24) + 'H'), null, { termId: 9 }).key);
});

test('no word of the evaluator or its sentences says that a vote was won; the sysselraad column is still not read', () => {
  // the evaluator's own strings: the winner function and its wording table (WIN) are the one place that says a Sysselraad is won, and are tested in lcc-win-wording.test.mjs
  const all = fs.readFileSync(path.join(lccDir, 'lcc-model.js'), 'utf8').replace(/\r/g, '');
  const src = all.slice(0, all.indexOf('// ---- who won the Sysselraad')) + all.slice(all.indexOf('// ---- filters, sort, groups'));
  // every string the file can say
  const strings = [...src.matchAll(/'((?:[^'\\\n]|\\.)*)'/g)].map((m) => m[1]);
  assert.ok(strings.length > 40);
  for (const s of strings) assert.doesNotMatch(s, /\bwins?\b|\bwon\b|\bwinner\b|victory|confirmed/i, 'a string of the reader says a result: ' + s);
  // the reasons the evaluator gives for a verified cutoff, a conflict and an absent record
  const reasons = [ev('O'.repeat(25)).vote.reason, ev('AAAAA' + 'O'.repeat(15) + 'HHHHH').conflictReason, ev('A'.repeat(25), { verified: true, owners: owners('A'.repeat(25)) }).vote.reason, ev('A'.repeat(25), { verified: true, owners: owners('A'.repeat(12) + 'H'.repeat(12) + 'O') }).vote.reason];
  for (const s of reasons) { assert.ok(s.length > 10); assert.doesNotMatch(s, /\bwins?\b|\bwon\b/i); }
});
