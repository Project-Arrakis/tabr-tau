// #893: a held line of five (or 13 Houses) is not final while Houses that cannot be read could still give the other faction a line or 13 Houses. The evaluator carries conflictPossible and the
// wording table says so in one sentence; verdictSide (once called winner) is the side the verdict names, not a result. Pure reading of 25 owners, no DOM, no write, no game save.
import test from 'node:test';
import assert from 'node:assert/strict';
import path from 'node:path';
import { createRequire } from 'node:module';
import { lccDir } from './lcc-load.mjs';

const require = createRequire(import.meta.url);
const M = require(path.join(lccDir, 'lcc-model.js'));
const owners = (s) => [...s].map((c) => ({ A: 'a', H: 'h', O: 'open' })[c] || 'unknown');
const put = (marks) => { const a = Array(25).fill('O'); for (const [i, c] of marks) a[i] = c; return a.join(''); };
const range = (from, to, c) => Array.from({ length: to - from + 1 }, (_, i) => [from + i, c]);
const ev = (s) => M.evaluateOwners(owners(s), null, { termId: 7 });

// the example of the issue: Atreides hold row 1, Harkonnen hold four Houses of row 5 and the fifth cannot be read
const ISSUE = put([...range(0, 4, 'A'), ...range(20, 23, 'H'), [24, '?']]);

test('the issue example: Atreides hold a line, Harkonnen hold four Houses of a line and the fifth is unreadable -> conflict possible, not final', () => {
  const r = ev(ISSUE);
  assert.equal(r.verdict, 'line_held');
  assert.equal(r.verdictSide, 'a');
  assert.equal(r.conflict, false, 'nothing is read as a conflict: only that one could exist');
  assert.equal(r.conflictPossible, true);
  assert.equal(r.win.state, 'won');
  assert.equal(r.win.winner, 'a');
  assert.equal(r.win.conflictPossible, true);
  assert.equal(M.WIN.maybeNote(r.win), '1 House could not be read, so Harkonnen may also qualify, so this may not be final.');
  assert.match(M.WIN.say(r.win), /Sysselraad secured by Atreides, by five in a line\..*1 House could not be read, so Harkonnen may also qualify/);
  assert.equal(M.WIN.tally(r.win), 'ATREIDES · BY FIVE IN A LINE · 5 TO 4 · 15 HOUSES OPEN · MAY NOT BE FINAL');
  assert.equal(M.WIN.headline(r.win), 'SYSSELRAAD SECURED · ATREIDES', 'the headline wording decisions are unchanged');
  assert.equal(Object.prototype.hasOwnProperty.call(r, 'winner'), false, 'the evaluator has no field called winner any more');
});

test('Harkonnen hold the line and Atreides could complete one: the sentence names the other faction, and counts the Houses', () => {
  const r = ev(put([...range(20, 24, 'H'), ...range(0, 3, 'A'), [4, '?'], [9, '?']]));
  assert.equal(r.verdictSide, 'h');
  assert.equal(r.conflictPossible, true);
  assert.equal(M.WIN.maybeNote(r.win), '2 Houses could not be read, so Atreides may also qualify, so this may not be final.');
});

test('no conflict is possible: every House readable, however many lines are held', () => {
  for (const s of [put(range(0, 4, 'A')), put([...range(0, 4, 'A'), ...range(20, 23, 'H')]), put([...range(0, 4, 'A'), ...range(5, 9, 'A')])]) {
    const r = ev(s);
    assert.equal(r.conflictPossible, false);
    assert.equal(M.WIN.maybeNote(r.win), '');
    assert.doesNotMatch(M.WIN.say(r.win), /could not be read|final/);
    assert.doesNotMatch(M.WIN.tally(r.win), /FINAL/);
  }
});

test('no conflict is possible: the unreadable House sits only on lines the other faction has blocked or holds nothing on', () => {
  // Atreides row 1; the unreadable House 24 lies on row 5 (22 is Atreides), column 5 (4 is Atreides) and the main diagonal (0 is Atreides): Harkonnen cannot complete any
  let r = ev(put([...range(0, 4, 'A'), [20, 'H'], [21, 'H'], [22, 'A'], [23, 'H'], [24, '?']]));
  assert.equal(r.verdict, 'line_held');
  assert.equal(r.conflictPossible, false);
  // Harkonnen hold a line; the unreadable centre lies only on lines where Atreides hold nothing, and an open House is not an Atreides House
  r = ev(put([...range(20, 24, 'H'), [12, '?']]));
  assert.equal(r.verdictSide, 'h');
  assert.equal(r.conflictPossible, false);
  assert.equal(M.WIN.say(r.win).includes('could not be read'), false);
});

test('13 Houses: the other faction could also reach 13 with the unreadable Houses (no line needed), and could not with fewer', () => {
  // Atreides hold row 1 and 12; 22 and 0..4 block every line through 24; Harkonnen hold 12 Houses without a line; the one unreadable House would make 13
  const base = [...range(0, 4, 'A'), [12, 'A'], [22, 'A'], ...range(5, 8, 'H'), [10, 'H'], [11, 'H'], [13, 'H'], [14, 'H'], ...range(15, 17, 'H'), [24, '?']];
  const r = ev(put([...base, [18, 'H']]));
  assert.deepEqual([r.counts.a, r.counts.h, r.counts.unknown], [7, 12, 1]);
  assert.equal(r.win.state, 'won');
  assert.equal(r.conflictPossible, true, '12 + 1 unreadable = 13');
  const r2 = ev(put(base)); // Harkonnen at 11 Houses: 11 + 1 cannot reach 13
  assert.equal(r2.counts.h, 11);
  assert.equal(r2.conflictPossible, false);
});

test('a win by 13 Houses is checked the same way (the other faction could still complete a line)', () => {
  const a13 = [0, 5, 6, 7, 8, 10, 11, 13, 14, 15, 16, 17, 18].map((i) => [i, 'A']);
  const r = ev(put([...a13, ...range(20, 23, 'H'), [24, '?']]));
  assert.equal(r.win.winner, 'a');
  assert.deepEqual([...r.win.routes], ['houses']);
  assert.equal(r.conflictPossible, true);
});

test('states with no winner never carry the flag, and the count reading (no board) never does', () => {
  for (const s of [put([]), put([[12, '?']]), put([...range(0, 3, 'A'), ...range(20, 23, 'H'), [24, '?']]), put([...range(0, 4, 'A'), ...range(20, 24, 'H')])]) {
    const r = ev(s);
    if (r.win.state !== 'won') assert.equal(r.conflictPossible, false, s);
  }
  assert.equal(M.WIN.decideCounts(13, 2).conflictPossible, false);
  assert.equal(M.WIN.decideCounts(4, 2).conflictPossible, false);
  assert.equal(M.WIN.decide(null).conflictPossible, false);
  assert.equal(M.WIN.maybeNote(M.WIN.decide(null)), '');
});

test('conflictPossible equals "some way to read the unreadable Houses makes both factions qualify" over seeded random boards', () => {
  let seed = 893;
  const rnd = () => { seed = (Math.imul(seed, 1664525) + 1013904223) >>> 0; return seed / 4294967296; };
  let yes = 0, no = 0, wonWithUnknown = 0;
  for (let b = 0; b < 4000; b++) {
    const mix = [[.45, .35], [.3, .3], [.2, .6]][b % 3];
    const unk = b % 4;
    const own = Array.from({ length: 25 }, () => { const x = rnd(); return x < mix[0] ? 'a' : x < mix[0] + mix[1] ? 'h' : 'open'; });
    const cells = [];
    while (cells.length < unk) { const i = Math.floor(rnd() * 25); if (!cells.includes(i)) cells.push(i); }
    for (const i of cells) own[i] = 'unknown';
    const r = M.evaluateOwners(own, null, { termId: 1 });
    // the oracle: try every reading (a, h or open) of the unreadable Houses
    let both = false;
    const walk = (k, cur) => {
      if (both) return;
      if (k === cells.length) { const d = M.WIN.decide(cur); if (d.state === 'undecidable' && d.why === 'both') both = true; return; }
      for (const v of ['a', 'h', 'open']) { const c = cur.slice(); c[cells[k]] = v; walk(k + 1, c); }
    };
    if (r.win.state === 'won') {
      walk(0, own);
      if (cells.length) wonWithUnknown++;
      assert.equal(r.conflictPossible, both && cells.length > 0, own.join(','));
    } else assert.equal(r.conflictPossible, false);
    if (r.conflictPossible) yes++; else no++;
  }
  assert.ok(yes > 5 && no > 20 && wonWithUnknown > 50, 'the sample reaches both answers: ' + [yes, no, wonWithUnknown]);
});
