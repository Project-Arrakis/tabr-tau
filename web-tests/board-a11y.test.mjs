// Accessibility and touch fixes of the Landsraad board and the header (audit #659, #619, #889, #924). The page code runs in jsdom over the generated fixtures; what only a layout engine can
// show (sizes, the scroll under a sticky header) is checked by scripts/lcc-m7-sweep.mjs, and the style sheets are read here for the rules that must not come back.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { JSDOM } from 'jsdom';
import { boot, closeAll } from './boot.mjs';

afterEach(closeAll);
const here = path.dirname(fileURLToPath(import.meta.url));
const read = (...p) => fs.readFileSync(path.join(here, '..', ...p), 'utf8').replace(/\r\n/g, '\n');
const appCss = read('internal', 'web', 'static', 'app.css');
const boardCss = read('internal', 'web', 'static', 'lcc', 'lcc-board.css');
const lccCss = read('internal', 'web', 'static', 'lcc', 'lcc.css');

const TH = [1200, 2400, 3600, 4800, 6000];
const OWN = ['SolarisCoin', 'T6ArmorPlating', 'T6RangeFinder', 'T6HoltzmanActuator', 'T6DiamodineBladeParts'];
const house = (id, name, mine, extra = {}) => ({
  id, board_index: id - 900, house_name: 'DA_House' + name, goal_amount: 4000, completed: 0, winning_faction_id: null, completion_time: null,
  contributions: [], player_contributions: [], my_contribution: mine, my_recorded: mine > 0, swatch_held: false, location: { known: false },
  rewards: TH.map((threshold, i) => ({ threshold, amount: 1, template_id: OWN[i], reached: mine >= threshold, delivered: false, shown_template_id: OWN[i], shown_amount: 1, swatch: i === 4, reward_state: mine >= threshold ? 'no_row' : 'not_reached' })),
  ...extra,
});
const board = () => ({
  term: { term_id: 7, start_time: 100, end_time: 200, reigning_faction_id: 1, active_decree_id: 1, elected_decree_id: 1 }, terms: [{ term_id: 7, start_time: 100, end_time: 200 }], earlier_terms: [],
  factions: [{ id: 1, name: 'Atreides' }, { id: 2, name: 'Harkonnen' }], character: { player_id: 1, name: 'Tester', faction_id: 1 }, decrees: [{ id: 1, decree_name: 'Decree A', disabled: 0 }], rotation: [],
  tasks: [house(900, 'Alexin', 0), house(901, 'Thorvald', 4700), house(902, 'Hurata', 1300), house(903, 'Wayku', 6500, { completed: 1, winning_faction_id: 1 })],
});
async function classic() {
  const ui = await boot({ get: { '/api/landsraad': board() } });
  ui.click('[data-tab="landsraad"]'); await ui.settle();
  return ui;
}

test('the Classic board region is named by its own heading, not by itself (#659 G-034)', async () => {
  const ui = await classic();
  const sec = ui.doc.querySelector('section#lsb-board');
  assert.equal(sec.getAttribute('aria-labelledby'), 'lsb-board-h');
  assert.equal(ui.doc.getElementById('lsb-board-h').textContent.replace(/\s+/g, ' ').trim(), 'Task board 4 houses');
});

test('the Classic list is a table with rows and cells: each House is a row with its name as the row header (#659 G-035)', async () => {
  const ui = await classic();
  const rows = [...ui.doc.querySelectorAll('#lsb-rows > li')];
  assert.equal(rows.length, 4);
  for (const r of rows) {
    assert.equal(r.getAttribute('role'), 'row');
    assert.equal(r.querySelectorAll('[role="rowheader"]').length, 1, 'one row header');
    assert.ok(r.querySelectorAll('[role="cell"]').length >= 4, 'the other columns are cells');
    assert.ok(r.querySelector('[role="rowheader"] .hn b'), 'the House name is in the row header');
  }
  assert.equal(ui.doc.querySelector('[role="table"] > [role="rowgroup"] > [role="row"] [role="columnheader"]') !== null, true, 'the header row is there');
});

test('a glance tile is named by its visible words first, and the source marks are not read inside headings (#659 G-038, G-039)', async () => {
  const ui = await classic();
  const tiles = [...ui.doc.querySelectorAll('a.tile')];
  assert.equal(tiles.length, 3);
  for (const t of tiles) {
    const visible = t.querySelector('.l').textContent.replace(/[^\p{L}\s]/gu, '').replace(/\s+/g, ' ').trim().toLowerCase();
    const n = t.querySelector('.n').textContent.trim();
    const label = t.getAttribute('aria-label').toLowerCase();
    assert.ok(label.startsWith(n + ' ' + visible), `the name starts with the visible words: "${label}" vs "${n} ${visible}"`);
  }
  // the source labels (save, derived) and the table and field names in their tooltips are gone (#1057); Help says where the figures come from
  assert.equal(ui.doc.querySelectorAll('.lsb .src').length, 0, 'no source mark is drawn');
  const text = ui.doc.querySelector('#lsb').textContent.replace(/\s+/g, ' ');
  assert.ok(!text.includes('The small marks beside a heading'), 'no sentence about marks that are not there');
  const tips = [...ui.doc.querySelectorAll('#lsb [title]')].map((e) => e.getAttribute('title')).join(' | ');
  assert.doesNotMatch(tips, /landsraad_|Read from the save|Worked out by the editor from fields|winning_faction_id/, 'no table or field name in a tooltip');
  assert.ok(ui.doc.querySelector('details.about[data-about="lsb.counts"] .abt'), 'the About text of the counts stays');
});

test('the queue chip and the Theme button are named by their visible words first (#619 D9-2, D9-3)', async () => {
  const ui = await boot();
  const chip = ui.doc.querySelector('#pending');
  assert.match(chip.textContent, /^\d+ queued$/);
  assert.equal(chip.getAttribute('aria-label'), chip.textContent + ', show the queued changes');
  const dom = new JSDOM(read('internal', 'web', 'static', 'index.html'), { runScripts: 'outside-only', pretendToBeVisual: true, url: 'http://127.0.0.1:8090/' });
  const w = dom.window;
  w.matchMedia = (q) => ({ matches: false, addEventListener() {}, removeEventListener() {} });
  w.eval(read('internal', 'web', 'static', 'motion-boot.js')); w.eval(read('internal', 'web', 'static', 'look-boot.js')); w.eval(read('internal', 'web', 'static', 'theme-menu.js'));
  const btn = w.document.querySelector('#theme-btn');
  assert.ok(btn.textContent.startsWith('Theme: '), btn.textContent);
  assert.ok(btn.getAttribute('aria-label').startsWith(btn.textContent + '. '), btn.getAttribute('aria-label'));
  w.close();
});

test('a control that is off is not faded: the quiet text and edge of the one disabled look tell it, so its label keeps its contrast (#889, #659 G-061, #1049)', () => {
  const ruleOf = (css, sel) => { const i = css.indexOf(sel + ' {'); const j = i < 0 ? css.indexOf(sel + '{') : i; assert.ok(j >= 0, 'no rule for ' + sel); return css.slice(j, css.indexOf('}', j)); };
  // the board's and the classic list's own rules for an off control are gone: the one look is in app.css (disabled-look.test.mjs)
  for (const sel of ['.lcc-bd-act[aria-disabled="true"]', '.lcc-bd-chip[aria-disabled="true"]', '.lcc-bd-layout button[aria-disabled="true"]', '.lcc-bd-select:disabled']) assert.ok(!boardCss.includes(sel + ' {'), sel + ' has no look of its own');
  for (const sel of ['.lsb .qa[aria-disabled="true"]', '.lsb .tc[aria-disabled="true"]', '.lsb .sorth:disabled']) assert.ok(!appCss.includes(sel + '{'), sel + ' has no look of its own');
  assert.ok(!/opacity/.test(ruleOf(appCss, '.lsb .btnl input:disabled+span')), 'a disabled box label must not fade');
  const one = ruleOf(appCss, ':root :is(button,select,input,textarea,[role="tab"],[role="switch"],.d-btn,.d-toggle):is(:disabled,[aria-disabled="true"]):not(.tc,.lcc-bd-tier,.lcc-hc-t)');
  assert.ok(one.includes('{color:var(--dis-fg)'), 'the quiet text colour');
  assert.doesNotMatch(one, /dashed/);
});


test('the not-reached tier box edge in the dark look is at least 3:1 against the card (#889)', () => {
  const edge = /--lcc-tier-edge: (#[0-9a-f]{6});/.exec(lccCss.split('@media (prefers-color-scheme: light)')[0])[1];
  const L = (h) => { const c = [1, 3, 5].map((i) => parseInt(h.substr(i, 2), 16) / 255).map((v) => (v <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4)); return 0.2126 * c[0] + 0.7152 * c[1] + 0.0722 * c[2]; };
  const ratio = (a, b) => (Math.max(L(a), L(b)) + 0.05) / (Math.min(L(a), L(b)) + 0.05);
  const cardA = /--lcc-card-a: (#[0-9a-f]{6})/.exec(lccCss)[1], cardB = /--lcc-card-b: (#[0-9a-f]{6})/.exec(lccCss)[1];
  assert.ok(ratio(edge, cardA) >= 3 && ratio(edge, cardB) >= 3, `${edge} vs ${cardA} ${ratio(edge, cardA).toFixed(2)}, vs ${cardB} ${ratio(edge, cardB).toFixed(2)}`);
});

test('the header and the Classic list reach 44 px on a touch screen or in a window of 640 px or less, after the 32 px rules (#659 G-037)', () => {
  const i = appCss.indexOf('@media (pointer: coarse), (max-width: 640px) {');
  assert.ok(i > appCss.lastIndexOf('min-height:32px'), 'the rule comes after the last 32 px rule, so it wins');
  const block = appCss.slice(i, appCss.indexOf('\n}', i));
  for (const sel of ['header .d-btn', 'header .d-pill', 'header label.mut', '.lsb .hn', '.lsb .qa', '.lsb .loc-t']) assert.ok(block.includes(sel), sel);
  assert.match(block, /min-height: 44px/);
});
