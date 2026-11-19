// The low-severity look and feel leftovers of the Command Center board (tau #882, #883, #890, #896, #919, #920, #921, #947, #998), in the page's own code (jsdom) with the synthetic answer
// and no game save. Display, wording and focus only: nothing here changes a request. The measures that need a layout engine (the text floor, the forced-colours render) are in
// scripts/lcc-m7-sweep.mjs; what a style sheet says is read from the file.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { createRequire } from 'node:module';
import { boot, closeAll } from './boot.mjs';
import { synthetic } from './lcc-fixture.mjs';
import { lccDir } from './lcc-load.mjs';

afterEach(closeAll);

const require = createRequire(import.meta.url);
const M = require(path.join(lccDir, 'lcc-model.js'));
const strip = (s) => s.replace(/\r/g, '').replace(/\/\*[\s\S]*?\*\//g, '');
const css = (f) => strip(fs.readFileSync(path.join(lccDir, f), 'utf8'));
const rule = (sheet, selector) => { const m = sheet.match(new RegExp('(^|\\n|\\})\\s*' + selector.replace(/[.*+?^${}()|[\]\\]/g, '\\$&') + '\\s*\\{([^}]*)\\}')); return m ? m[2] : null; };
const text = (el) => el.textContent.replace(/\s+/g, ' ').trim();
const OK = { ok: true, changed: 1, houses: 1, skipped: 0, contribution_raised: 1, contribution_lowered: 0, completed: 0, reopened: 0, reward_rows: 0, outcome: 'written' };
const click = (ui, el) => { if (typeof el === 'string') el = ui.doc.querySelector(el); assert.ok(el, 'no element'); el.focus(); el.dispatchEvent(new ui.w.MouseEvent('click', { bubbles: true, cancelable: true })); };
const idOf = (a, name) => a.tasks.find((t) => t.house_name === 'DA_House' + name).id;
async function open(view = 'board', answer = synthetic(), post = { '/api/landsraad/apply': OK }) {
  const ui = await boot({ get: { '/api/landsraad': answer }, post, local: { tab: 'landsraad', 'lsb.view': view, 'lsb.layout': 'grid' } });
  await ui.settle();
  return ui;
}

// ---- #882: the panel's selects while a write runs ------------------------------------------------------------------------------------------------------------------------
test('#882: a redraw while a write is running draws the panel with its Faction and Set tier selects off, and a faction chosen then goes back to the one in use', async () => {
  const a = synthetic();
  const ui = await open('board', a);
  click(ui, '[data-fk="ht:' + idOf(a, 'Sor') + '"]');
  assert.equal(ui.doc.querySelector('#lcc-bd-fac').disabled, false, 'idle: usable');
  // hold the write: the answer to the POST is not given until the test lets it go
  let release;
  const gate = new Promise((r) => { release = r; });
  const realFetch = ui.w.fetch;
  ui.w.fetch = async (p, opt = {}) => { if ((opt.method || 'GET') === 'POST') await gate; return realFetch(p, opt); };
  click(ui, '.lcc-bd-insp .lcc-bd-act[data-do="fill"]');
  await new Promise((r) => setTimeout(r, 20));
  assert.equal(ui.doc.querySelector('#lsb').getAttribute('aria-busy'), 'true', 'the write is running');
  ui.w.LCCB.redraw(); // the board is drawn again while the write runs
  const fac = ui.doc.querySelector('#lcc-bd-fac'), ts = ui.doc.querySelector('#lcc-bd-tierset');
  assert.equal(fac.disabled, true, 'the faction box of the panel is built off');
  assert.equal(ts.disabled, true, 'the Set tier box of the panel is built off');
  // a choice that reaches the handler anyway (the board's own box is off, so it is not applied): the box shows the faction that is used again
  const used = fac.getAttribute('data-was');
  const other = [...fac.options].map((o) => o.value).find((v) => v !== used);
  fac.disabled = false; fac.value = other;
  fac.dispatchEvent(new ui.w.Event('change', { bubbles: true }));
  assert.equal(fac.value, used, 'the panel does not show a faction that was not applied');
  release();
  await ui.settle();
});

test('#882: with nothing running, a faction chosen in the panel is taken by the board\'s own box and stays', async () => {
  const a = synthetic();
  const ui = await open('board', a);
  click(ui, '[data-fk="ht:' + idOf(a, 'Sor') + '"]');
  const fac = ui.doc.querySelector('#lcc-bd-fac');
  const other = [...fac.options].map((o) => o.value).find((v) => v !== fac.value);
  fac.value = other;
  fac.dispatchEvent(new ui.w.Event('change', { bubbles: true }));
  await ui.settle();
  assert.equal(ui.doc.querySelector('#lsb-fac').value, other, 'the board\'s box took it');
  assert.equal(ui.doc.querySelector('#lcc-bd-fac').value, other, 'and the panel shows it');
  assert.equal(ui.w.LCC_HOST.setFac('1') === true || ui.w.LCC_HOST.setFac('1') === false, true, 'setFac says whether it applied');
});

// ---- #883: focus after Apply tier, a House id as a number ----------------------------------------------------------------------------------------------------------------
test('#883: after Apply tier the focus is on the Apply tier button, not on the ledger row it pressed', async () => {
  const a = synthetic();
  const ui = await open('board', a);
  const id = idOf(a, 'Sor');
  click(ui, '[data-fk="ht:' + id + '"]');
  const sel = ui.doc.querySelector('#lcc-bd-tierset');
  sel.value = '1';
  click(ui, '.lcc-bd-insp [data-lcb="settier"]');
  await ui.settle();
  await new Promise((r) => setTimeout(r, 30));
  assert.ok(ui.posted.some((p) => p.path === '/api/landsraad/apply'), 'the write was sent');
  const f = ui.doc.activeElement;
  assert.equal(f && f.getAttribute('data-fk'), 'at:' + id, 'the focus is on Apply tier');
  assert.equal(ui.doc.querySelector('.lcc-bd-tier[data-focusfk]'), null, 'and no marker is left on the box');
});

test('#883: a House id that arrives as a digit string is read as the number', () => {
  const a = synthetic();
  const id = a.tasks[3].id;
  a.tasks[3].id = String(id);
  const b = M.readBoard(a);
  const h = b.houses.find((x) => x.raw === a.tasks[3]);
  assert.equal(h.id, id);
  assert.equal(typeof h.id, 'number');
});

// ---- #890: no win claims in the folded text ------------------------------------------------------------------------------------------------------------------------------
test('#890: the Classic list\'s folded help and the Reopen mouse-overs speak of the completion and the faction that holds the favor, not of a win or a winner', async () => {
  const ui = await open('classic');
  const root = ui.doc.querySelector('#lsb');
  const all = [text(root)];
  for (const n of root.querySelectorAll('[title]')) all.push(n.getAttribute('title'));
  const s = all.join(' | ');
  assert.match(s, /clears the completion and the faction that holds/);
  assert.doesNotMatch(s, /faction[’']s win\b|the completion and the winner|the winner and the completion/);
});

// ---- #919: unnamed contributors ---------------------------------------------------------------------------------------------------------------------------------------
test('#919: a contributor the save names no guild for is "Unnamed contributor", its save number only in the mouse-over text', async () => {
  const a = synthetic();
  for (const t of a.tasks) t.player_contributions = [];
  a.tasks[0].player_contributions.push({ player_id: 73, faction_id: 1, amount: 900, mine: false }, { player_id: 74, faction_id: 1, amount: 400, mine: false, guild: 'house_of_fortune_hunters_incorporated' });
  const b = M.readBoard(a);
  assert.deepEqual(b.contributors.a.map((c) => [c.label, c.unnamed]), [['Unnamed contributor', true], ['House Of Fortune Hunters Incorporated', false]]);
  assert.equal(b.contributors.a[0].id, 73);
  const ui = await open('board', a);
  const std = ui.doc.querySelector('.lcc-plaque .lcc-standard.lcc-f-a');
  const names = [...std.querySelectorAll('.lcc-standard-name')].map(text);
  assert.deepEqual(names, ['Unnamed contributor', 'House Of Fortune Hunters Incorporated']);
  assert.doesNotMatch(text(std), /\b73\b/, 'the save number is not drawn');
  const tip = std.querySelector('.lcc-standard-h').getAttribute('title');
  assert.match(tip, /Unnamed contributor: the save gives no player or guild name for it \(save number 73\)\./);
  assert.match(tip, /Top 2: 1\. Unnamed contributor 900/);
});

test('#919: the House panel says a guild is shown by its identifier with capitals added', async () => {
  const a = synthetic();
  const ui = await open('board', a);
  const id = a.tasks.find((t) => (t.player_contributions || []).length).id;
  click(ui, '[data-fk="ht:' + id + '"]');
  const p = ui.doc.querySelector('.lcc-bd-insp .lcc-bd-contrib');
  assert.ok(p, 'the House has contributors');
  assert.match(p.getAttribute('title'), /A guild is shown by the identifier the save gives it, with capital letters added here/);
});

// ---- #921: the Choose boxes ignore the browser's saved form state ----------------------------------------------------------------------------------------------------------
test('#921: the Choose boxes of the board and of the Classic list, and the Classic list\'s detail boxes, are drawn with autocomplete off', async () => {
  for (const view of ['board', 'classic']) {
    const ui = await open(view);
    const boxes = [...ui.doc.querySelectorAll('#lsb .lcc-bd-pick, #lsb input.pick, #lsb input.xc')];
    assert.ok(boxes.length >= 25, view + ': boxes found');
    for (const b of boxes) assert.equal(b.getAttribute('autocomplete'), 'off', view + ': ' + (b.getAttribute('aria-label') || b.className));
  }
});

// ---- what the style sheets say (the render measures are in the sweep) ------------------------------------------------------------------------------------------------------------
test('#896: the key to the board has 12 px labels, and the dead legend rules are gone', () => {
  const board = css('lcc-board.css');
  const all = [...board.matchAll(/\.lcc-bd-headlines li\s*\{([^}]*)\}/g)].map((m) => m[1]);
  assert.match(all[all.length - 1], /font-size:\s*12px/);
  assert.doesNotMatch(board, /\.lcc-bd-legend\s*\{[^}]*font-size:\s*(9|11)px/);
});

test('#947: the last line of the victory panel is 12 px in the normal text colour', () => {
  const r = rule(css('lcc-board.css'), '.lcc-bd-vcount');
  assert.match(r, /font:\s*12px/);
  assert.match(r, /color:\s*var\(--lcc-text\)/);
});

test('#920: the two banners of the narrow plaque end level: the cloth takes the height that is left', () => {
  const lcc = css('lcc.css');
  assert.match(lcc, /\.lcc-pl-std-a, \.lcc-pl-std-h \{ display: flex; flex-direction: column; \}/);
  assert.match(lcc, /\.lcc-pl-std-a \.lcc-standard-cloth, \.lcc-pl-std-h \.lcc-standard-cloth \{ flex: 1 1 auto; \}/);
});

test('#998: a selected chip, layout button and view button keep a readable system-colour label in forced colours', () => {
  const board = css('lcc-board.css'), spec = css('lcc-spec.css');
  const b = board.match(/\.lcc-bd-chip\[aria-pressed="true"\], \.lcc-bd-layout button\[aria-pressed="true"\] \{([^}]*forced-color-adjust: none[^}]*)\}/);
  assert.ok(b, 'the board rule takes the control out of the automatic adjustment');
  assert.match(b[1], /background: Highlight/);
  assert.match(b[1], /color: HighlightText/);
  assert.match(b[1], /outline: 2px solid HighlightText/, 'and marks the selection by an outline, not by the fill alone');
  const s = spec.match(/\.lcc-sp-vbtn\[aria-pressed="true"\] \{([^}]*forced-color-adjust: none[^}]*)\}/);
  assert.ok(s, 'the view button of the Specializations too');
  assert.match(s[1], /color: HighlightText/);
});
