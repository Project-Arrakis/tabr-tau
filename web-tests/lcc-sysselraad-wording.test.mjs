// Who has won the Sysselraad, and the one wording table (#929, #926; the operator's decision of 2026-11-18). The save stores no winner and no vote: the game works the winner out from the Houses
// (a line of five, or 13 or more Houses), so the Command Center does the same, in one function and one table of words (WIN in lcc-model.js). This file asserts (1) the winner function against a small
// reading of the rules written here, (2) that every surface of the board shows the same phrase for the same state, and (3) that no surface builds a phrase of its own.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { createRequire } from 'node:module';
import { boot, closeAll } from './boot.mjs';
import { synthetic } from './lcc-fixture.mjs';
import { lccDir, load } from './lcc-load.mjs';

afterEach(closeAll);
const M = createRequire(import.meta.url)(path.join(lccDir, 'lcc-model.js'));
const OWN = { A: 'a', H: 'h', O: 'open', '?': 'unknown' };
const owners = (letters) => [...letters].map((c) => OWN[c]);
const text = (el) => (el ? el.textContent.replace(/\s+/g, ' ').trim() : '');
const pad = (n, ...marks) => { const a = Array(n).fill('O'); for (const [i, c] of marks) a[i] = c; return a.join(''); };

// ---- the board states of the brief, as the letters of the 25 positions --------------------------------------------------------------------------------------------------------------------------
const ROW3_A = 'OOOOOOOOOOAAAAAOOOOOOOOOO'; // Atreides hold row 3: a line of five, 5 Houses
const A13 = 'HAHAHAHAHAHAAAHAHAHAHAHAH'; // 13 Atreides, 12 Harkonnen, no line
const A_BOTH = 'AAAAAAAAAAAAAOHOHOHOHOHOH'; // Atreides: rows 1 and 2 and 3 Houses of row 3 = 13 Houses and two lines; Harkonnen 6
const TIE = 'HAHAHAHAHAHAOAHAHAHAHAHAH'; // 12 each, one open
const NOTHING = 'O'.repeat(25);
const BOTH_LINES = 'AAAAA' + 'O'.repeat(15) + 'HHHHH'; // a line each
const H13 = A13.replace(/[AH]/g, (c) => (c === 'A' ? 'H' : 'A')); // 13 Harkonnen, 12 Atreides, no line
const A_LINE_H_13 = 'AAAAA' + 'HHHHOHHHHOHHHHOHOOOOO'.slice(0, 20); // Atreides row 1; Harkonnen 13 Houses (positions 5-8, 10-13, 15-18, 20), no line of theirs
const UNREADABLE = pad(25, [12, '?']); // one House that cannot be read, nothing else taken
const UNREADABLE_LINE = pad(25, [12, '?'], ...[20, 21, 22, 23, 24].map((i) => [i, 'H'])); // a line is held elsewhere

test('the winner function: a line of five or 13 Houses wins; both factions qualifying cannot be decided; nothing qualifying is contested', () => {
  const d = (s) => M.WIN.decide(owners(s));
  let r = d(ROW3_A);
  assert.deepEqual([r.state, r.winner, r.routes, r.reason, r.lines], ['won', 'a', ['line'], 'five in a line', ['row2']]);
  r = d(A13);
  assert.deepEqual([r.state, r.winner, r.routes, r.reason, r.lines], ['won', 'a', ['houses'], '13 Houses', []]);
  r = d(A_BOTH);
  assert.deepEqual([r.state, r.winner, r.routes, r.reason], ['won', 'a', ['line', 'houses'], 'five in a line and 13 Houses']);
  assert.deepEqual(r.lines, ['row0', 'row1']);
  r = d(H13);
  assert.deepEqual([r.state, r.winner, r.reason], ['won', 'h', '13 Houses'], '13 exactly is a winning count');
  assert.equal(d(TIE.replace('A', 'O')).state, 'contested', '12 Houses to 11 is a lead and nothing more');
  assert.equal(d(TIE).state, 'contested');
  assert.equal(d(NOTHING).state, 'contested');
  assert.equal(d(synthLetters()).state, 'contested', 'a lead of 12 to 4');
  r = d(BOTH_LINES);
  assert.deepEqual([r.state, r.winner, r.why], ['undecidable', null, 'both']);
  r = d(A_LINE_H_13);
  assert.deepEqual([r.state, r.winner, r.why, r.qualifiers.map((q) => [q.faction, q.reason])], ['undecidable', null, 'both', [['a', 'five in a line'], ['h', '13 Houses']]]);
  r = d(UNREADABLE);
  assert.deepEqual([r.state, r.why], ['undecidable', 'unreadable'], 'a House that cannot be read, and nobody qualifies: it is not called contested');
  r = d(UNREADABLE_LINE);
  assert.deepEqual([r.state, r.winner, r.reason], ['won', 'h', 'five in a line'], 'a line held elsewhere still wins');
  assert.deepEqual([M.WIN.decide(null).state, M.WIN.decide(null).why, M.WIN.decide(['a', 'h']).why], ['undecidable', 'board', 'board'], 'not a board of 25');
  // the same reading of the evaluator and of the function over many boards
  const rnd = (() => { let s = 929; return () => { s = (Math.imul(s, 1664525) + 1013904223) >>> 0; return s / 4294967296; }; })();
  for (let b = 0; b < 600; b++) {
    const mix = [.4, .4, .2, 0][b % 4] ? [.45, .45, .1] : [.3, .3, .4];
    const letters = Array.from({ length: 25 }, () => { const x = rnd(); return x < mix[0] ? 'A' : x < mix[0] + mix[1] ? 'H' : 'O'; }).join('');
    const ev = M.evaluateOwners(owners(letters), null, { termId: 1 });
    const g = M.WIN.decide(owners(letters));
    assert.deepEqual(ev.win, g, 'the evaluator carries the same reading: ' + letters);
    const lineSides = new Set(ev.held.map((l) => l.side));
    const qa = lineSides.has('a') || ev.counts.a >= 13, qh = lineSides.has('h') || ev.counts.h >= 13;
    assert.equal(g.state, qa && qh ? 'undecidable' : qa || qh ? 'won' : 'contested', letters);
  }
});

function synthLetters() { const a = synthetic(); const out = Array(25).fill('O'); for (const t of a.tasks) out[t.board_index] = t.completed && t.winning_faction_id === 1 ? 'A' : t.completed && t.winning_faction_id === 2 ? 'H' : 'O'; return out.join(''); }

test('the table: the headline and the tally for each state', () => {
  const W = M.WIN;
  const row = (s) => { const d = W.decide(owners(s)); return [W.headline(d), W.tally(d)]; };
  assert.deepEqual(row(ROW3_A), ['SYSSELRAAD WON · ATREIDES', 'ATREIDES · BY FIVE IN A LINE · 5 TO 0 · 20 HOUSES OPEN']);
  assert.deepEqual(row(A13), ['SYSSELRAAD WON · ATREIDES', 'ATREIDES · BY 13 HOUSES · 13 TO 12 · 0 HOUSES OPEN']);
  assert.deepEqual(row(A_BOTH), ['SYSSELRAAD WON · ATREIDES', 'ATREIDES · BY FIVE IN A LINE AND 13 HOUSES · 13 TO 6 · 6 HOUSES OPEN']);
  assert.deepEqual(row(H13), ['SYSSELRAAD WON · HARKONNEN', 'HARKONNEN · BY 13 HOUSES · 13 TO 12 · 0 HOUSES OPEN']);
  assert.deepEqual(row(synthLetters()), ['SYSSELRAAD CONTESTED', 'ATREIDES LEAD 12 TO 4 · 9 HOUSES OPEN']);
  assert.deepEqual(row(TIE), ['SYSSELRAAD CONTESTED', 'HOUSE COUNT EVEN AT 12 · 1 HOUSE OPEN']);
  assert.deepEqual(row(NOTHING), ['SYSSELRAAD CONTESTED', 'HOUSE COUNT EVEN AT 0 · 25 HOUSES OPEN']);
  assert.deepEqual(row(BOTH_LINES), ['SYSSELRAAD UNDECIDABLE', 'ATREIDES BY FIVE IN A LINE · HARKONNEN BY FIVE IN A LINE · CANNOT BE DECIDED FROM THIS SAVE']);
  assert.deepEqual(row(A_LINE_H_13), ['SYSSELRAAD UNDECIDABLE', 'ATREIDES BY FIVE IN A LINE · HARKONNEN BY 13 HOUSES · CANNOT BE DECIDED FROM THIS SAVE']);
  assert.deepEqual(row(UNREADABLE), ['SYSSELRAAD UNDECIDABLE', 'COUNT INCOMPLETE · 1 HOUSE UNREADABLE']);
  assert.deepEqual(row(UNREADABLE_LINE), ['SYSSELRAAD WON · HARKONNEN', 'HARKONNEN · BY FIVE IN A LINE · 5 TO 0 · 19 HOUSES OPEN']);
  assert.deepEqual([W.headline(W.decide(null)), W.tally(W.decide(null))], ['SYSSELRAAD UNDECIDABLE', 'HOUSE COUNT UNAVAILABLE · BOARD NOT COMPLETE']);
  assert.equal(W.TEXT.facts, 'The save does not record the vote; the game works the winner out from the Houses.');
});

// ---- every surface of the page ------------------------------------------------------------------------------------------------------------------------------------------------------------------
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
async function open(answer, local = {}) {
  const ui = await boot({ get: { '/api/landsraad': answer }, local: Object.assign({ tab: 'landsraad', 'lsb.view': 'board', 'lsb.layout': 'grid' }, local) });
  await ui.settle();
  return ui;
}
const click = (ui, el) => { if (typeof el === 'string') el = ui.doc.querySelector(el); assert.ok(el, 'no element'); el.focus(); el.dispatchEvent(new ui.w.MouseEvent('click', { bubbles: true, cancelable: true })); };

// each state: the phrase every surface must show. `panelTag`/`panelHead`: the panel.
// panelHead: the title slot of the panel; panelFoot: the how-line (a won state) or the count lead (contested) under the counts
const STATES = [
  { name: 'won by a line of five', answer: () => board(ROW3_A), won: true, tag: 'SYSSELRAAD WON', head: 'SYSSELRAAD WON · ATREIDES', tally: 'ATREIDES · BY FIVE IN A LINE · 5 TO 0 · 20 HOUSES OPEN', panelHead: 'SYSSELRAAD WON · ATREIDES', panelFoot: 'BY FIVE IN A LINE', icon: 'a' },
  { name: 'won by 13 Houses', answer: () => board(A13), won: true, tag: 'SYSSELRAAD WON', head: 'SYSSELRAAD WON · ATREIDES', tally: 'ATREIDES · BY 13 HOUSES · 13 TO 12 · 0 HOUSES OPEN', panelHead: 'SYSSELRAAD WON · ATREIDES', panelFoot: 'BY 13 HOUSES', icon: 'a' },
  { name: 'won by a line and by 13 Houses', answer: () => board(A_BOTH), won: true, tag: 'SYSSELRAAD WON', head: 'SYSSELRAAD WON · ATREIDES', tally: 'ATREIDES · BY FIVE IN A LINE AND 13 HOUSES · 13 TO 6 · 6 HOUSES OPEN', panelHead: 'SYSSELRAAD WON · ATREIDES', panelFoot: 'BY FIVE IN A LINE AND 13 HOUSES', icon: 'a' },
  { name: 'contested, a lead', answer: () => synthetic(), tag: 'SYSSELRAAD CONTESTED', head: 'SYSSELRAAD CONTESTED', tally: 'ATREIDES LEAD 12 TO 4 · 9 HOUSES OPEN', panelHead: 'ATREIDES LEAD 12 TO 4 · 9 HOUSES OPEN', panelFoot: 'House count: Atreides ahead 12 to 4.' },
  { name: 'contested, a tie', answer: () => board(TIE), tag: 'SYSSELRAAD CONTESTED', head: 'SYSSELRAAD CONTESTED', tally: 'HOUSE COUNT EVEN AT 12 · 1 HOUSE OPEN', panelHead: 'HOUSE COUNT EVEN AT 12 · 1 HOUSE OPEN', panelFoot: 'House count: even at 12 each.' },
  { name: 'contested, nothing taken', answer: () => board(NOTHING), tag: 'SYSSELRAAD CONTESTED', head: 'SYSSELRAAD CONTESTED', tally: 'HOUSE COUNT EVEN AT 0 · 25 HOUSES OPEN', panelHead: 'HOUSE COUNT EVEN AT 0 · 25 HOUSES OPEN', panelFoot: 'House count: even at 0 each.' },
  { name: 'undecidable, both factions qualify', answer: () => board(BOTH_LINES), tag: 'SYSSELRAAD UNDECIDABLE', head: 'SYSSELRAAD UNDECIDABLE', tally: 'ATREIDES BY FIVE IN A LINE · HARKONNEN BY FIVE IN A LINE · CANNOT BE DECIDED FROM THIS SAVE', panelHead: 'ATREIDES BY FIVE IN A LINE · HARKONNEN BY FIVE IN A LINE · CANNOT BE DECIDED FROM THIS SAVE' },
  { name: 'unreadable owners, nobody qualifies', answer: () => board(UNREADABLE), tag: 'SYSSELRAAD UNDECIDABLE', head: 'SYSSELRAAD UNDECIDABLE', tally: 'COUNT INCOMPLETE · 1 HOUSE UNREADABLE', panelHead: 'COUNT INCOMPLETE · 1 HOUSE UNREADABLE', panelFoot: 'House count incomplete: 1 House could not be read, so no comparison is made.' },
  { name: 'unreadable owners, a line elsewhere', answer: () => board(UNREADABLE_LINE), won: true, tag: 'SYSSELRAAD WON', head: 'SYSSELRAAD WON · HARKONNEN', tally: 'HARKONNEN · BY FIVE IN A LINE · 5 TO 0 · 19 HOUSES OPEN', panelHead: 'SYSSELRAAD WON · HARKONNEN', panelFoot: 'BY FIVE IN A LINE', icon: 'h' },
  { name: 'a board that is not a board', answer: () => { const a = board(ROW3_A); a.tasks.pop(); return a; }, tag: 'SYSSELRAAD UNDECIDABLE', head: 'SYSSELRAAD UNDECIDABLE', tally: 'HOUSE COUNT UNAVAILABLE · BOARD NOT COMPLETE', panelHead: 'HOUSE COUNT UNAVAILABLE · BOARD NOT COMPLETE' },
];

for (const st of STATES) {
  test('every surface shows the same phrase: ' + st.name, async () => {
    const ui = await open(st.answer());
    const doc = ui.doc;
    // the plaque: headline and tally
    assert.equal(text(doc.querySelector('.lcc-pl-session')), st.head);
    assert.equal(text(doc.querySelector('.lcc-pl-standing')), st.tally);
    assert.doesNotMatch(text(doc.querySelector('.lcc-plaque-host')), /VOTING SESSION|MAJORITY AT 13/, 'the old headline is gone');
    // the victory panel: tag (the phrase), title and body
    const p = doc.querySelector('.lcc-bd-victory');
    assert.equal(text(p.querySelector('.lcc-bd-vtag')), st.tag);
    assert.equal(text(p.querySelector('.lcc-bd-vhead')), st.panelHead);
    assert.equal(text(p.querySelector('.lcc-bd-vcount')), st.panelFoot || '', 'the how-line of a win, or the count lead');
    const emblem = p.querySelector('.lcc-bd-emblem');
    if (st.icon) assert.equal(emblem.getAttribute('data-faction'), st.icon, 'the winner\'s crest'); else assert.equal(emblem, null, 'no winner: no icon');
    assert.equal(p.querySelector('.lcc-bd-vbody'), null, 'the explanatory paragraph is gone');
    const sr = text(p.querySelector('.lcc-bd-sr'));
    if (st.won) assert.match(sr, /The save does not record the vote; the game works the winner out from the Houses\./, 'the facts line is for a screen reader and the tooltip where a winner is');
    assert.doesNotMatch(text(p) + ' ' + sr, /Dominion|DOMINION|not verified|NOT VERIFIED|AWAITING VOTING SESSION|LINE OF FIVE|not a vote result/);
    assert.equal(p.querySelectorAll('details, .lcc-bd-lines').length, 0, 'no list of the twelve lines in the panel (#931)');
    // the House cards: the gold outline on a House that makes up a win is drawn by lcc-lines-board.test.mjs; no text flag on any card (#961); the aria-label and the tooltip of a card that is part of a win say it in the same words
    assert.equal(doc.querySelectorAll('.lcc-bd-cell .lcc-hc .lcc-flag').length, 0, 'no text flag on a card');
    if (st.won) {
      const lead = doc.querySelector('.lcc-bd-cell .lcc-hc[data-victory] .lcc-hc-select');
      assert.ok(lead, 'a House of the win');
      for (const attr of ['aria-label', 'title']) assert.match(lead.getAttribute(attr), /Sysselraad won by (Atreides|Harkonnen), by (five in a line|13 Houses|five in a line and 13 Houses)\. The save does not record the vote; the game works the winner out from the Houses\./, attr);
    } else {
      for (const c of doc.querySelectorAll('.lcc-bd-cell .lcc-hc .lcc-hc-select')) assert.doesNotMatch((c.getAttribute('aria-label') || '') + ' ' + (c.getAttribute('title') || ''), /Sysselraad won/);
    }
    // the plaque and the panel never disagree with the function the model exports
    const d = M.WIN.decide(owners([...(st.answer().tasks)].sort((x, y) => x.board_index - y.board_index).map((t) => (t.completed && t.winning_faction_id === 1 ? 'A' : t.completed && t.winning_faction_id === 2 ? 'H' : t.completed ? '?' : 'O')).join('')));
    if (st.name !== 'a board that is not a board') { assert.equal(text(doc.querySelector('.lcc-pl-session')), M.WIN.headline(d)); assert.equal(text(doc.querySelector('.lcc-pl-standing')), M.WIN.tally(d)); }
    assert.deepEqual(ui.posted, [], 'looking sends nothing');
  });
}

test('the twelve-lines block is gone from the victory panel, and so is the line chooser (#931, #958)', async () => {
  const ui = await open(board(ROW3_A));
  assert.equal(ui.doc.querySelectorAll('.lcc-bd-victory details').length, 0);
  for (const sel of ['[data-lcb="line"]', '[data-lcb="strategy"]', '.lcc-bd-strat-panel', '.lcc-hc[data-route]']) assert.equal(ui.doc.querySelector(sel), null, sel);
  assert.equal(ui.doc.querySelectorAll('.lcc-bd-cell .lcc-hc[data-victory="win"]').length, 5, 'the win outline of the five Houses is still drawn');
});

test('no surface builds a phrase of its own: the words of a win live in lcc-model.js only', () => {
  const read = (f) => fs.readFileSync(f, 'utf8').replace(/\r/g, '');
  const here = path.join(lccDir, '..');
  const files = ['lcc/lcc.js', 'lcc/lcc-board.js', 'lcc/lcc-term.js', 'lcc/lcc-spec.js', 'app.js'].map((f) => [f, read(path.join(here, f))]);
  for (const [f, src] of files) {
    for (const bad of ["'SYSSELRAAD WON", "'SYSSELRAAD CONTESTED", "'SYSSELRAAD UNDECIDABLE", "'LINE OF FIVE'", "'MAJORITY HOUSE'", 'MAJORITY AT 13', 'BY 13 HOUSES', "'FIVE IN A LINE'", 'CANNOT BE DECIDED']) {
      // the TERM STATUS figure of the figures row is removed in its own change (the figures row goes); it is the one place that still has its own words
      const lines = src.split('\n').map((l) => l.replace(/\/\/.*$/, '')).filter((l) => l.includes(bad) && !l.includes("item('TERM STATUS'"));
      assert.equal(lines.length, 0, f + ' builds its own phrase: ' + bad);
    }
  }
  const model = read(path.join(lccDir, 'lcc-model.js'));
  for (const must of ['SYSSELRAAD WON', 'SYSSELRAAD CONTESTED', 'SYSSELRAAD UNDECIDABLE', 'The save does not record the vote; the game works the winner out from the Houses.']) assert.ok(model.includes(must), must);
});

// ---- the gold 13th seat (the centre orb) of the seat track: a ring and the travelling arc only for a win by 13 Houses ------------------------------------------------------------------------
test('the centre orb carries the win ring only when a faction has won by 13 Houses; the arc moves only with Motion Full and no reduced-motion request', () => {
  const orb = (letters, motion) => {
    const { w, L, doc } = load();
    if (motion != null) doc.documentElement.setAttribute('data-motion', motion);
    w.matchMedia = () => ({ matches: false, media: '' });
    const own = owners(letters);
    const win = M.WIN.decide(own);
    const a = own.filter((o) => o === 'a').length, h = own.filter((o) => o === 'h').length;
    const node = L.renderPlaque({ a, h, win });
    const seat = node.querySelector('.lcc-seat-center');
    return { seat, win: seat.getAttribute('data-win13'), sweeps: node.querySelectorAll('.lcc-seat-win-sweep').length, others: node.querySelectorAll('.lcc-seat:not(.lcc-seat-center)[data-win13]').length };
  };
  const cases = [
    ['13 Atreides Houses, no line', A13, 'a'],
    ['13 Harkonnen Houses, no line', H13, 'h'],
    ['a line and 13 Houses', A_BOTH, 'a'],
    ['a line of five only', ROW3_A, null],
    ['contested', TIE, null],
    ['empty board', NOTHING, null],
    ['both qualify', A_LINE_H_13, null],
    ['one House unreadable', UNREADABLE, null],
  ];
  for (const [name, letters, want] of cases) {
    const r = orb(letters, 'full');
    assert.equal(r.win, want, name);
    assert.equal(r.sweeps, want ? 1 : 0, name + ': the ring that the style moves exists only then');
    assert.equal(r.others, 0, name + ': no other seat is marked');
    assert.equal(r.seat.classList.contains('lcc-seat-win13'), !!want, name);
    assert.equal(orb(letters, 'calm').win, want, name + ': the still ring does not depend on Motion');
  }
  // the style: the arc is drawn and turned only with Motion Full and no reduced-motion request; the still ring (the halo, bolder) is always there; forced colours draw no arc
  const bare = fs.readFileSync(path.join(lccDir, 'lcc.css'), 'utf8').replace(/\r/g, '').replace(/\/\*[\s\S]*?\*\//g, '');
  assert.match(bare, /\.lcc-seat-win-sweep \{ display: none;/);
  assert.match(bare, /@media \(prefers-reduced-motion: no-preference\) \{\s*html\[data-motion="full"\] \.lcc-seat-win13 \.lcc-seat-win-sweep \{[^}]*animation: lcc-win-sweep 9s linear infinite;/);
  assert.match(bare, /\.lcc-seat-win13 \.lcc-seat-halo \{ stroke: var\(--lcc-gold-hi\); stroke-width: 6;/);
  assert.match(bare, /@media \(forced-colors: active\)[^]*html\[data-motion\] \.lcc-seat-win13 \.lcc-seat-win-sweep \{ display: none; \}/);
});
