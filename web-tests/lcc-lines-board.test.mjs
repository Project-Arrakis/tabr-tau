// The twelve lines on the live board (M4): the POTENTIAL LINES figure, the outline of a line of five and of a faction with 13 or more Houses, the Imperial Victory Analysis states, and the words.
// The page is the real app.js with the real lcc scripts over the synthetic answer of M1 with the owners set by the test (no game save is read). The evaluator itself is tested in lcc-lines.test.mjs.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import { boot, closeAll } from './boot.mjs';
import { synthetic } from './lcc-fixture.mjs';

afterEach(closeAll);

const text = (el) => el.textContent.replace(/\s+/g, ' ').trim();
// the synthetic answer with the owners of the 25 board positions given as letters: A, H, O open, ? taken with no winner (not readable)
function board(letters, mutate) {
  const a = synthetic();
  for (const t of a.tasks) {
    const c = letters[t.board_index];
    t.completed = c === 'A' || c === 'H' || c === '?' ? 1 : 0;
    t.winning_faction_id = c === 'A' ? 1 : c === 'H' ? 2 : null;
  }
  if (mutate) mutate(a);
  return a;
}
async function open(answer, { motion, layout } = {}) {
  const local = { tab: 'landsraad', 'lsb.view': 'board' };
  if (layout) local['lsb.layout'] = layout;
  const ui = await boot({ get: { '/api/landsraad': answer }, local, pre: motion ? (w) => w.document.documentElement.setAttribute('data-motion', motion) : null });
  await ui.settle();
  return ui;
}
const cards = (ui) => [...ui.doc.querySelectorAll('.lcc-bd-cell .lcc-hc')];
const outlined = (ui, kind) => cards(ui).map((c, i) => [i, c.getAttribute('data-victory')]).filter((x) => (kind ? x[1] === kind : x[1])).map((x) => x[0]);
const metric = (ui, k) => text(ui.doc.querySelector('[data-metric="' + k + '"]'));
const panel = (ui) => ui.doc.querySelector('.lcc-bd-victory');
// every word the board says about a result: the figures, the panel, the cards (their words, names and tips) and the legend
function words(ui) {
  const out = [];
  for (const sel of ['.lcc-bd-metrics', '.lcc-bd-victory', '.lcc-bd-tiles', '.lcc-bd-headlines']) {
    const root = ui.doc.querySelector(sel);
    if (!root) continue;
    out.push(text(root));
    for (const e of root.querySelectorAll('[title],[aria-label]')) out.push(e.getAttribute('title') || '', e.getAttribute('aria-label') || '');
  }
  return out.join(' | ');
}
// the words that must never appear: a win, a won vote, a confirmed outcome (the only "confirmed" allowed are the two honest negatives)
const BANNED = /\bwins?\b|\bwon\b|\bwinners?\b(?! confirmed)|winner confirmed|victory confirmed|vote (was )?won|dominion confirmed|secured/i;
const honest = (s) => s.replace(/No confirmed winner/g, '').replace(/cannot be confirmed/g, '');
const noConfirm = (s) => assert.doesNotMatch(honest(s), /confirmed/i, 'a result is called confirmed: ' + s.slice(0, 300));

const ROW3_A = 'OOOOOOOOOOAAAAAOOOOOOOOOO'; // row 3 (positions 11 to 15)
const letters = (n, ...marks) => { const a = Array(n).fill('O'); for (const [i, c] of marks) a[i] = c; return a.join(''); };

test('no line held: the figure is the number of lines still possible, no House is outlined, and the panel says the vote is awaited', async () => {
  const ui = await open(synthetic());
  assert.match(metric(ui, 'lines'), /^POTENTIAL LINES4Unblocked five-House alignments · Atreides 4 · Harkonnen 0$/);
  assert.equal(outlined(ui).length, 0);
  assert.equal(panel(ui).getAttribute('data-state'), 'contested');
  assert.match(text(panel(ui)), /AWAITING VOTING SESSION.*SYSSELRAAD CONTESTED.*No confirmed winner/);
  assert.match(text(panel(ui).querySelector('.lcc-bd-vcount')), /^House count: Atreides ahead 12 to 4 · not verified: the save does not record the start of the Voting Session\.$/);
  assert.match(metric(ui, 'term'), /^TERM STATUSCONTESTED/);
  assert.equal(ui.doc.querySelectorAll('.lcc-bd-line').length, 12, 'all twelve lines are listed, in the board\'s own order');
  assert.deepEqual([...ui.doc.querySelectorAll('.lcc-bd-line')].map((l) => l.getAttribute('data-line')), ['row0', 'row1', 'row2', 'row3', 'row4', 'col0', 'col1', 'col2', 'col3', 'col4', 'diag-main', 'diag-anti']);
  assert.equal(ui.doc.querySelector('.lcc-bd-lines').closest('details').open, false, 'the list is folded away until asked for');
  assert.doesNotMatch(honest(words(ui)), BANNED);
  noConfirm(words(ui));
});

test('one held row: its five Houses, and only those, have the circuit outline and the words LINE OF FIVE', async () => {
  const ui = await open(board(ROW3_A));
  assert.deepEqual(outlined(ui), [10, 11, 12, 13, 14]);
  assert.deepEqual(outlined(ui, 'circuit'), [10, 11, 12, 13, 14]);
  for (const i of [10, 11, 12, 13, 14]) {
    const c = cards(ui)[i];
    assert.equal(c.getAttribute('data-lines'), 'row2');
    assert.equal(text(c.querySelector('.lcc-flag-line')), 'LINE OF FIVE');
    assert.match(c.querySelector('.lcc-hc-select').getAttribute('aria-label'), /part of a line of five held by Atreides: row 3/);
    assert.match(c.querySelector('.lcc-hc-select').getAttribute('title'), /Atreides hold this House in a line of five: row 3\. The Houses as they stand now, not a vote result/);
  }
  assert.equal(ui.doc.querySelectorAll('.lcc-bd-cell .lcc-hc .lcc-flag-line').length, 5);
  assert.equal(ui.doc.querySelectorAll('.lcc-bd-cell .lcc-hc .lcc-flag-dom').length, 0, 'five Houses are not 13: no count outline');
  const p = panel(ui);
  assert.equal(p.getAttribute('data-state'), 'line');
  assert.equal(p.getAttribute('data-faction'), 'a');
  assert.match(text(p), /FIVE IN A LINE.*Atreides · FIVE IN A LINE.*Atreides hold five Houses in one line: row 3\. Read from the Houses as they stand now; the save does not record the result of a vote\./);
  assert.match(metric(ui, 'term'), /^TERM STATUSFIVE IN A LINEAtreides · row 3 · vote not recorded$/);
  assert.match(metric(ui, 'lines'), /^POTENTIAL LINES\d+Unblocked five-House alignments · Atreides \d+ · Harkonnen \d+ · 1 held, not counted$/);
  assert.match(text(ui.doc.querySelector('.lcc-bd-line[data-line="row2"]')), /^Row 3 · Houses 11, 12, 13, 14, 15Atreides hold all five Houses: five in a line$/);
  assert.match(text(p.querySelector('summary')), /^The twelve lines \(1 held, \d+ still possible\)$/);
  assert.doesNotMatch(honest(words(ui)), BANNED);
  noConfirm(words(ui));
});

test('a held diagonal and a held column are outlined by their own positions, for either faction', async () => {
  const anti = board(letters(25, ...[4, 8, 12, 16, 20].map((i) => [i, 'H'])));
  let ui = await open(anti);
  assert.deepEqual(outlined(ui, 'circuit'), [4, 8, 12, 16, 20]);
  assert.equal(panel(ui).getAttribute('data-faction'), 'h');
  assert.match(text(panel(ui)), /Harkonnen · FIVE IN A LINE.*Harkonnen hold five Houses in one line: opposite diagonal/);
  assert.match(metric(ui, 'term'), /Harkonnen · opposite diagonal · vote not recorded/);
  closeAll();
  ui = await open(board(letters(25, ...[2, 7, 12, 17, 22].map((i) => [i, 'A']))));
  assert.deepEqual(outlined(ui, 'circuit'), [2, 7, 12, 17, 22]);
  assert.match(text(panel(ui)), /Atreides hold five Houses in one line: column 3/);
  assert.equal(text(ui.doc.querySelector('.lcc-bd-line[data-line="col2"] span')), 'Atreides hold all five Houses: five in a line');
});

test('13 Houses held: those Houses have the majority outline, labelled as a count that is not verified, and the Dominion is never confirmed', async () => {
  const a13 = 'HAHAHAHAHAHAAAHAHAHAHAHAH'; // 13 Atreides, 12 Harkonnen, no line
  const ui = await open(board(a13));
  const a = [...a13].map((c, i) => (c === 'A' ? i : -1)).filter((i) => i >= 0);
  assert.deepEqual(outlined(ui, 'dominion'), a);
  assert.deepEqual(outlined(ui, 'circuit'), []);
  const p = panel(ui);
  assert.equal(p.getAttribute('data-state'), 'count');
  assert.match(text(p), /IMPERIAL DOMINION · NOT VERIFIED.*Atreides · 13 HOUSES HELD.*A count of Houses now, not a vote result\. The save does not record when the Voting Session starts, so a House-count lead cannot be confirmed here\./);
  assert.match(text(p.querySelector('.lcc-bd-vcount')), /Atreides ahead 13 to 12 · not verified/);
  for (const i of a) assert.match(cards(ui)[i].querySelector('.lcc-hc-select').getAttribute('title'), /a count of Houses, not a vote result, and not verified: the save does not record the start of the Voting Session/);
  assert.match(text(ui.doc.querySelector('.lcc-bd-headlines')), /MAJORITY HOUSE · COUNT NOT VERIFIED/);
  assert.doesNotMatch(honest(words(ui)), BANNED);
  noConfirm(words(ui));
});

test('12 Houses held is a lead and nothing more', async () => {
  const ui = await open(board('HAHAHAHAHAHAOAHAHAHAHAHAH')); // 12 each, one open
  assert.equal(outlined(ui).length, 0);
  assert.equal(panel(ui).getAttribute('data-state'), 'contested');
  assert.match(text(panel(ui).querySelector('.lcc-bd-vcount')), /^House count: even at 12 each · not verified/);
  assert.doesNotMatch(text(panel(ui)), /DOMINION/);
});

test('a line of five and 13 Houses of the same faction: the line Houses have the circuit outline, the rest of the 13 the majority outline, and the panel says both honestly', async () => {
  const l = 'AAAAAAAAAAAAAOHOHOHOHOHOH'; // positions 0 to 12 Atreides (row 1, row 2 and 3 Houses of row 3), a few Harkonnen Houses that hold no line
  const ui = await open(board(l));
  assert.deepEqual(outlined(ui, 'circuit'), [0, 1, 2, 3, 4, 5, 6, 7, 8, 9]);
  assert.deepEqual(outlined(ui, 'dominion'), [10, 11, 12]);
  assert.match(text(panel(ui)), /Atreides hold five Houses in 2 lines: row 1 and row 2\..*Atreides also hold 13 Houses: a count, not verified/);
  assert.equal(ui.doc.querySelectorAll('.lcc-bd-cell .lcc-hc[data-victory="circuit"] .lcc-flag-dom').length, 0, 'a House of a line carries the line badge only');
  assert.doesNotMatch(honest(words(ui)), BANNED);
  noConfirm(words(ui));
});

test('a line for each faction is a conflict: both lines are outlined, nobody is chosen', async () => {
  const ui = await open(board('AAAAA' + 'O'.repeat(15) + 'HHHHH'));
  assert.deepEqual(outlined(ui, 'circuit'), [0, 1, 2, 3, 4, 20, 21, 22, 23, 24]);
  const p = panel(ui);
  assert.equal(p.getAttribute('data-state'), 'conflict');
  assert.equal(p.getAttribute('data-faction'), null, 'no faction is named for the panel');
  assert.match(text(p), /CONFLICT.*BOTH FACTIONS HOLD A LINE.*Atreides hold row 1 and Harkonnen hold row 5\. The save does not record which came first or the result of a vote, so neither faction is chosen\./);
  assert.match(metric(ui, 'term'), /^TERM STATUSCONFLICT/);
  assert.doesNotMatch(honest(words(ui)), BANNED);
  noConfirm(words(ui));
});

test('an unreadable House: the count of potential lines says it is incomplete and shows no number; a line held elsewhere is still outlined', async () => {
  let ui = await open(board(letters(25, [12, '?'])));
  assert.match(metric(ui, 'lines'), /^POTENTIAL LINES—Count incomplete · 1 House unreadable$/);
  assert.equal(ui.doc.querySelector('[data-metric="lines"]').getAttribute('data-incomplete'), 'true');
  assert.match(text(ui.doc.querySelector('.lcc-bd-victory summary')), /count incomplete/);
  assert.match(text(panel(ui).querySelector('.lcc-bd-vcount')), /^House count incomplete: 1 House could not be read, so no comparison is made\.$/);
  assert.equal(outlined(ui).length, 0);
  closeAll();
  ui = await open(board(letters(25, [12, '?'], ...[20, 21, 22, 23, 24].map((i) => [i, 'H']))));
  assert.deepEqual(outlined(ui, 'circuit'), [20, 21, 22, 23, 24]);
  assert.match(metric(ui, 'lines'), /Count incomplete/);
  assert.equal(panel(ui).getAttribute('data-state'), 'line');
});

test('a board that is not a board draws no outline and no number of potential lines', async () => {
  const a = board(ROW3_A);
  a.tasks.pop();
  const ui = await open(a);
  assert.equal(ui.doc.querySelectorAll('[data-victory]').length, 0);
  assert.match(metric(ui, 'lines'), /^POTENTIAL LINES—Not counted: the board could not be checked$/);
  assert.equal(panel(ui).getAttribute('data-state'), 'unavailable');
  assert.match(text(panel(ui)), /COUNT UNAVAILABLE.*NO COUNT TO SHOW/);
  assert.equal(ui.doc.querySelectorAll('.lcc-bd-lines').length, 0);
});

test('the list layout keeps the figure and the panel, and the table shows the same Houses', async () => {
  const ui = await open(board(ROW3_A), { layout: 'list' });
  const list = ui.doc.querySelector('.lcc-bd-list') ? true : false;
  assert.ok(list || ui.doc.querySelector('.lcc-bd-listwrap'), 'the list is drawn');
  assert.equal(panel(ui).getAttribute('data-state'), 'line');
  assert.match(metric(ui, 'lines'), /1 held, not counted/);
});

test('the animated border follows the Motion setting: Full adds it to the Houses of a line, Calm and Off leave the still outline', async () => {
  const sweeps = (ui) => ui.doc.querySelectorAll('.lcc-bd-cell .lcc-hc .lcc-win-sweep').length;
  let ui = await open(board(ROW3_A), { motion: 'full' });
  assert.equal(sweeps(ui), 5);
  assert.equal(ui.doc.querySelectorAll('.lcc-bd-cell .lcc-hc.lcc-win-anim').length, 5);
  assert.equal(ui.doc.querySelector('.lcc-win-sweep').getAttribute('aria-hidden'), 'true');
  closeAll();
  for (const m of ['calm', 'off']) {
    ui = await open(board(ROW3_A), { motion: m });
    assert.equal(sweeps(ui), 0, m);
    assert.equal(outlined(ui, 'circuit').length, 5, m + ': the still outline stays');
    closeAll();
  }
  ui = await open(board('HAHAHAHAHAHAAAHAHAHAHAHAH'), { motion: 'full' });
  assert.equal(sweeps(ui), 13, 'the 13 Houses have the same border');
});

test('clicking a House of a line only selects it: nothing is sent and the outline stays', async () => {
  const ui = await open(board(ROW3_A));
  const pick = cards(ui)[11].querySelector('.lcc-hc-select');
  pick.dispatchEvent(new ui.w.MouseEvent('click', { bubbles: true, cancelable: true }));
  await ui.settle();
  assert.equal(ui.posted.length, 0);
  assert.deepEqual(outlined(ui, 'circuit'), [10, 11, 12, 13, 14]);
  assert.equal(ui.doc.querySelectorAll('.lcc-bd-cell .lcc-hc[data-selected="true"]').length, 1);
});
