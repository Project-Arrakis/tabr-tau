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
// the lines each faction can still make: one line of the Imperial Victory Analysis (#935; it was the POTENTIAL LINES figure of the row of four figures)
const linesLeft = (ui) => { const e = ui.doc.querySelector('.lcc-bd-vlines'); return e ? text(e) : null; };
const panel = (ui) => ui.doc.querySelector('.lcc-bd-victory');
// every word the board says about a result: the panel, the cards (their words, names and tips) and the legend
function words(ui) {
  const out = [];
  for (const sel of ['.lcc-bd-victory', '.lcc-bd-tiles', '.lcc-bd-headlines']) {
    const root = ui.doc.querySelector(sel);
    if (!root) continue;
    const bare = root.cloneNode(true); // the House hover card (#975) is about the place and the next reward, not about a result: its words are read by their own tests
    bare.querySelectorAll('.hcard').forEach((n) => n.remove());
    out.push(text(bare));
    for (const e of root.querySelectorAll('[title],[aria-label],[data-says]')) if (!e.closest('.hcard')) out.push(e.getAttribute('title') || '', e.getAttribute('aria-label') || '', e.getAttribute('data-says') || '');
  }
  return out.join(' | ');
}
// the words that must never appear: a won vote, a confirmed Dominion or a confirmed outcome (since 2026-11-18 the game's winner, worked out from the Houses, is said as SYSSELRAAD WON; the save still holds no vote)
const BANNED = /vote (was )?won|dominion (is )?confirmed|victory confirmed|winner confirmed|secured/i;
const honest = (s) => s.replace(/is not confirmed/g, '').replace(/cannot be confirmed/g, '');
const noConfirm = (s) => assert.doesNotMatch(honest(s), /confirmed/i, 'a result is called confirmed: ' + s.slice(0, 300));

const ROW3_A = 'OOOOOOOOOOAAAAAOOOOOOOOOO'; // row 3 (positions 11 to 15)
const letters = (n, ...marks) => { const a = Array(n).fill('O'); for (const [i, c] of marks) a[i] = c; return a.join(''); };

test('no line held: the figure is the number of lines still possible, no House is outlined, and the panel says contested', async () => {
  const ui = await open(synthetic());
  assert.equal(linesLeft(ui), 'Lines of five still possible: Atreides 4 · Harkonnen 0');
  assert.equal(ui.doc.querySelector('.lcc-bd-metrics'), null, 'the row of four figures is gone (#935)');
  assert.equal(outlined(ui).length, 0);
  assert.equal(panel(ui).getAttribute('data-state'), 'contested');
  assert.match(text(panel(ui)), /^IMPERIAL VICTORY ANALYSISSYSSELRAAD CONTESTEDATREIDES LEAD 12 TO 4 · 9 HOUSES OPENSysselraad contested: no faction holds a line of five or 13 Houses\./);
  assert.equal(panel(ui).querySelector('.lcc-bd-emblem'), null, 'no winner: no icon, not even an empty diamond');
  assert.doesNotMatch(text(panel(ui)), /Dominion|DOMINION|not verified/);
  assert.equal(panel(ui).querySelector('.lcc-bd-vcount'), null, 'no second House count line (#1052)');
  assert.equal(panel(ui).querySelector('.lcc-bd-well'), null, 'no second row of the three counts (#1052)');
  assert.equal(panel(ui).querySelectorAll('details, .lcc-bd-lines, .lcc-bd-line').length, 0, 'the folded list of the twelve lines is no longer in the victory panel (#931)');
  assert.doesNotMatch(honest(words(ui)), BANNED);
  noConfirm(words(ui));
});

test('one held row: its five Houses, and only those, have the win outline and no text flag (#961)', async () => {
  const ui = await open(board(ROW3_A));
  assert.deepEqual(outlined(ui), [10, 11, 12, 13, 14]);
  assert.deepEqual(outlined(ui, 'win'), [10, 11, 12, 13, 14]);
  for (const i of [10, 11, 12, 13, 14]) {
    const c = cards(ui)[i];
    assert.equal(c.getAttribute('data-lines'), 'row2');
    assert.equal(c.querySelector('.lcc-flag, .lcc-flag-win, .lcc-hc-badge'), null, 'no text flag on a winning card: the outline carries the win');
    assert.match(c.querySelector('.lcc-hc-select').getAttribute('aria-label'), /part of a line of five held by Atreides: row 3, Sysselraad won by Atreides, by five in a line\. The save does not record the vote; the game works the winner out from the Houses\./);
    assert.match(c.querySelector('.lcc-hc-select').getAttribute('data-says'), /Atreides hold this House in a line of five: row 3\. Sysselraad won by Atreides, by five in a line\. The save does not record the vote/);
  }
  assert.equal(ui.doc.querySelectorAll('.lcc-bd-cell .lcc-hc .lcc-flag').length, 0);
  assert.equal(outlined(ui, 'win').length, 5, 'five Houses are not 13: only the line is outlined');
  const p = panel(ui);
  assert.equal(p.getAttribute('data-state'), 'line');
  assert.equal(p.getAttribute('data-faction'), 'a');
  assert.match(text(p), /SYSSELRAAD WON.*SYSSELRAAD WON · ATREIDES.*Sysselraad won by Atreides, by five in a line\. The save does not record the vote; the game works the winner out from the Houses\. Atreides hold five Houses in one line: row 3\./);
  assert.equal(text(p.querySelector('.lcc-bd-vcount')), 'BY FIVE IN A LINE');
  assert.equal(p.querySelector('.lcc-bd-emblem').getAttribute('data-faction'), 'a', 'the winner\'s crest');
  assert.match(linesLeft(ui), /^Lines of five still possible: Atreides \d+ · Harkonnen \d+$/);
  assert.doesNotMatch(honest(words(ui)), BANNED);
  noConfirm(words(ui));
});

test('a held diagonal and a held column are outlined by their own positions, for either faction', async () => {
  const anti = board(letters(25, ...[4, 8, 12, 16, 20].map((i) => [i, 'H'])));
  let ui = await open(anti);
  assert.deepEqual(outlined(ui, 'win'), [4, 8, 12, 16, 20]);
  assert.equal(panel(ui).getAttribute('data-faction'), 'h');
  assert.match(text(panel(ui)), /SYSSELRAAD WON · HARKONNEN.*Harkonnen hold five Houses in one line: opposite diagonal/);
  closeAll();
  ui = await open(board(letters(25, ...[2, 7, 12, 17, 22].map((i) => [i, 'A']))));
  assert.deepEqual(outlined(ui, 'win'), [2, 7, 12, 17, 22]);
  assert.match(text(panel(ui)), /Atreides hold five Houses in one line: column 3/);
});

test('13 Houses held: those Houses have the same win outline and flag as a line, the panel and the headline say the Sysselraad is won by 13 Houses, and the Dominion is never confirmed', async () => {
  const a13 = 'HAHAHAHAHAHAAAHAHAHAHAHAH'; // 13 Atreides, 12 Harkonnen, no line
  const ui = await open(board(a13));
  const a = [...a13].map((c, i) => (c === 'A' ? i : -1)).filter((i) => i >= 0);
  assert.deepEqual(outlined(ui, 'win'), a);
  const p = panel(ui);
  assert.equal(p.getAttribute('data-state'), 'count');
  assert.match(text(p), /SYSSELRAAD WON.*SYSSELRAAD WON · ATREIDES.*The save does not record the vote.*Atreides hold 13 of the 25 Houses\./);
  assert.equal(text(p.querySelector('.lcc-bd-vcount')), 'BY 13 HOUSES');
  assert.doesNotMatch(text(p), /Dominion|DOMINION|not verified|not a vote result/);
  for (const i of a) assert.match(cards(ui)[i].querySelector('.lcc-hc-select').getAttribute('data-says'), /One of the Atreides Houses, who hold 13 or more Houses\. Sysselraad won by Atreides, by 13 Houses\./);
  assert.equal(ui.doc.querySelectorAll('.lcc-bd-cell .lcc-hc .lcc-flag').length, 0, 'the Houses of a 13-House win have the outline and no flag either');
  assert.match(text(ui.doc.querySelector('.lcc-bd-headlines')), /WINNING HOUSES · SYSSELRAAD WON/);
  assert.doesNotMatch(honest(words(ui)), BANNED);
  noConfirm(words(ui));
});

test('12 Houses held is a lead and nothing more', async () => {
  const ui = await open(board('HAHAHAHAHAHAOAHAHAHAHAHAH')); // 12 each, one open
  assert.equal(outlined(ui).length, 0);
  assert.equal(panel(ui).getAttribute('data-state'), 'contested');
  assert.equal(panel(ui).querySelector('.lcc-bd-vcount'), null, 'the tie is in the headline only (#1052)');
  assert.match(text(panel(ui).querySelector('.lcc-bd-vhead')), /^HOUSE COUNT EVEN AT 12/);
  assert.doesNotMatch(text(panel(ui)), /DOMINION/);
});

test('a line of five and 13 Houses of the same faction: all 13 Houses have the one win outline, and the panel says both routes', async () => {
  const l = 'AAAAAAAAAAAAAOHOHOHOHOHOH'; // positions 0 to 12 Atreides (row 1, row 2 and 3 Houses of row 3), a few Harkonnen Houses that hold no line
  const ui = await open(board(l));
  assert.deepEqual(outlined(ui, 'win'), [0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12]);
  assert.equal(text(panel(ui).querySelector('.lcc-bd-vcount')), 'BY FIVE IN A LINE AND 13 HOUSES');
  assert.match(text(panel(ui)), /Atreides hold five Houses in 2 lines: row 1 and row 2\. Atreides hold 13 of the 25 Houses\./);
  assert.equal(ui.doc.querySelectorAll('.lcc-bd-cell .lcc-hc[data-victory="win"]').length, 13, 'each of the 13 carries the one outline');
  assert.equal(ui.doc.querySelectorAll('.lcc-bd-cell .lcc-hc .lcc-flag').length, 0, 'and no text flag');
  assert.doesNotMatch(honest(words(ui)), BANNED);
  noConfirm(words(ui));
});

test('a line for each faction is a conflict: both lines are outlined, nobody is chosen', async () => {
  const ui = await open(board('AAAAA' + 'O'.repeat(15) + 'HHHHH'));
  assert.deepEqual(outlined(ui, 'win'), [0, 1, 2, 3, 4, 20, 21, 22, 23, 24]);
  const p = panel(ui);
  assert.equal(p.getAttribute('data-state'), 'conflict');
  assert.equal(p.getAttribute('data-faction'), null, 'no faction is named for the panel');
  assert.match(text(p), /SYSSELRAAD UNDECIDABLE.*ATREIDES BY FIVE IN A LINE · HARKONNEN BY FIVE IN A LINE · CANNOT BE DECIDED FROM THIS SAVE.*both factions qualify, so it cannot be decided from this save/);
  assert.equal(p.querySelector('.lcc-bd-emblem'), null, 'no faction has won: no icon');
  assert.equal(ui.doc.querySelectorAll('.lcc-bd-cell .lcc-hc .lcc-flag').length, 0, 'no card carries a text flag');
  assert.doesNotMatch(honest(words(ui)), BANNED);
  noConfirm(words(ui));
});

test('an unreadable House: the count of potential lines says it is incomplete and shows no number; a line held elsewhere is still outlined', async () => {
  let ui = await open(board(letters(25, [12, '?'])));
  assert.equal(linesLeft(ui), null, 'a count that cannot be made shows no line (#1052)');
  assert.equal(panel(ui).getAttribute('data-state'), 'unreadable');
  assert.match(text(panel(ui)), /SYSSELRAAD UNDECIDABLE.*COUNT INCOMPLETE · 1 HOUSE UNREADABLE/);
  assert.equal(panel(ui).querySelector('.lcc-bd-vcount'), null, 'the headline says it, once');
  assert.equal(outlined(ui).length, 0);
  closeAll();
  ui = await open(board(letters(25, [12, '?'], ...[20, 21, 22, 23, 24].map((i) => [i, 'H']))));
  assert.deepEqual(outlined(ui, 'win'), [20, 21, 22, 23, 24]);
  assert.equal(linesLeft(ui), null);
  assert.equal(panel(ui).getAttribute('data-state'), 'line');
});

test('a board that is not a board draws no outline and no number of potential lines', async () => {
  const a = board(ROW3_A);
  a.tasks.pop();
  const ui = await open(a);
  assert.equal(ui.doc.querySelectorAll('[data-victory]').length, 0);
  assert.equal(linesLeft(ui), null, 'no count of lines when the board cannot be checked');
  assert.equal(panel(ui).getAttribute('data-state'), 'unavailable');
  assert.match(text(panel(ui)), /SYSSELRAAD UNDECIDABLE.*HOUSE COUNT UNAVAILABLE · BOARD NOT COMPLETE/);
});

test('the list layout keeps the figure and the panel, and the table shows the same Houses', async () => {
  const ui = await open(board(ROW3_A), { layout: 'list' });
  const list = ui.doc.querySelector('.lcc-bd-list') ? true : false;
  assert.ok(list || ui.doc.querySelector('.lcc-bd-listwrap'), 'the list is drawn');
  assert.equal(panel(ui).getAttribute('data-state'), 'line');
  assert.match(linesLeft(ui), /^Lines of five still possible: Atreides \d+ · Harkonnen \d+$/);
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
    assert.equal(outlined(ui, 'win').length, 5, m + ': the still outline stays');
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
  assert.deepEqual(outlined(ui, 'win'), [10, 11, 12, 13, 14]);
  assert.equal(ui.doc.querySelectorAll('.lcc-bd-cell .lcc-hc[data-selected="true"]').length, 1);
});
