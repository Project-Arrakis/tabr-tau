// Three read-only additions to the Command Center board: the summary line of the term (#440), the character's level in the specialization of a House (#646), and a list whose Place and Next tier cells
// fit (#1103). Real page code with the synthetic answers of M1; no game save. Nothing here writes, and nothing here adds a request.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import { boot, closeAll } from './boot.mjs';
import { synthetic } from './lcc-fixture.mjs';
import { read } from './lcc-load.mjs';

afterEach(closeAll);

const text = (el) => (el ? el.textContent.replace(/\s+/g, ' ').trim() : null);
const CASES = JSON.parse(read('docs', 'design', 'landsraad-command-center', 'fixtures', 'api-player-specs.cases.json')).cases;
const SPECS = () => JSON.parse(JSON.stringify(CASES.find((c) => c.id === 'mixed-five-tracks').response));
const APPLIED = { ok: true, changed: 1, houses: 1, skipped: 0, contribution_raised: 1, contribution_lowered: 0, completed: 0, reopened: 0, reward_rows: 1, outcome: 'written', missions: 3, solari: 18000, house_scrip: 3000, specialization_xp: { Gathering: 4500 }, rewards_already_given: 0 };
async function open(answer = synthetic(), { layout = 'grid', local = {}, tab = 'landsraad' } = {}) {
  const ui = await boot({ get: { '/api/landsraad': answer, '/api/player/specs': SPECS() }, post: { '/api/landsraad/apply': APPLIED }, local: { tab, 'lsb.view': 'board', 'lsb.layout': layout, ...local } });
  await ui.settle();
  return ui;
}
const click = (ui, el) => { if (typeof el === 'string') el = ui.doc.querySelector(el); assert.ok(el, 'no element'); el.focus(); el.dispatchEvent(new ui.w.MouseEvent('click', { bubbles: true, cancelable: true })); };
const tileOf = (ui, name) => [...ui.doc.querySelectorAll('.lcc-bd-cell')].find((c) => c.querySelector('.lcc-hc-name').textContent.toLowerCase() === 'house ' + name.toLowerCase());
const countsOf = (ui, a = synthetic()) => { const hs = ui.w.LCCM.readBoard(a).houses; const n = (k) => hs.filter((h) => ui.w.LCCM.matches(h, { show: k })).length; return { done: n('done'), part: n('part'), new: n('new') }; };

test('#440: one summary line under the term counts the Houses at top tier, in progress and not started, from the same state the Show tabs filter by', async () => {
  const ui = await open();
  const sum = ui.doc.querySelector('.lcc-bd-panel-head .lcc-bd-sum');
  assert.ok(sum, 'the summary line is in the board header');
  const c = countsOf(ui);
  assert.equal(c.done + c.part + c.new, 25, 'every House is in one of the three');
  assert.ok(c.done > 0 && c.part > 0 && c.new > 0, 'the fixture has Houses in all three states, so the line is not a trivial check');
  const keys = [...sum.querySelectorAll('button')];
  assert.deepEqual(keys.map((b) => text(b)), [c.done + ' top tier', c.part + ' in progress', c.new + ' not started']);
  assert.deepEqual(keys.map((b) => b.dataset.val), ['done', 'part', 'new'], 'each key is a value of the Show filter');
  // the wording is the Show tabs' (one name per thing)
  const tabs = Object.fromEntries([...ui.doc.querySelectorAll('[data-lcb="filter"][data-key="show"]')].filter((b) => b.closest('.lcc-k-tabs')).map((b) => [b.dataset.val, text(b)]));
  keys.forEach((b) => assert.equal(text(b).replace(/^\d+ /, ''), tabs[b.dataset.val].toLowerCase()));
  assert.ok(keys.every((b) => /Show only the \d+ Houses? at/.test(b.title) && /changes nothing in the save/.test(b.title)), 'the mouse-over says what a press does');
  assert.equal(sum.getAttribute('role'), 'group');
  assert.ok(sum.getAttribute('aria-label'));
});

test('#440: pressing a count sets that Show tab, shows exactly those Houses, and the count of Houses shown agrees', async () => {
  const ui = await open();
  const c = countsOf(ui);
  click(ui, '.lcc-bd-sum [data-val="part"]');
  assert.equal(ui.doc.querySelector('.lcc-k-tabs [data-val="part"]').getAttribute('aria-checked'), 'true', 'the Show tab In progress is on');
  assert.equal(ui.doc.querySelector('.lcc-bd-sum [data-val="part"]').getAttribute('aria-pressed'), 'true');
  assert.equal(ui.doc.querySelector('.lcc-bd-sum [data-val="done"]').getAttribute('aria-pressed'), 'false');
  assert.equal(ui.doc.querySelector('[data-lk="shown"]').textContent, String(c.part), 'the Houses shown are the ones counted');
  assert.deepEqual(countsOf(ui), c, 'the counts are of the term, not of what is shown: pressing a count does not change them');
  assert.equal(text(ui.doc.querySelector('.lcc-bd-sum [data-val="done"]')), c.done + ' top tier');
  assert.equal(ui.posted.length, 0, 'nothing is written');
});

test('#440: a character in no faction has no summary line (every House would read as not started)', async () => {
  const a = synthetic();
  a.character.faction_id = null;
  const ui = await open(a);
  assert.ok(ui.doc.querySelector('.lcc-bd-panel-head'), 'the board is drawn');
  assert.equal(ui.doc.querySelector('.lcc-bd-sum'), null);
});

test('#646: the House panel says Level N after the Specializations tab has read the levels, a dash with a title before, and the board makes no request of its own for it', async () => {
  const ui = await open();
  assert.equal(ui.fetched.filter((p) => p === '/api/player/specs').length, 0, 'the Landsraad tab does not read the specializations');
  click(ui, tileOf(ui, 'Kenola').querySelector('.lcc-hc-select'));
  let lv = ui.doc.querySelector('.lcc-bd-insp-title .lcc-bd-lvl');
  assert.ok(lv, 'the level is on the specialization line');
  assert.equal(text(lv), 'Level —', 'not read yet: a dash, never a made-up number');
  assert.match(lv.getAttribute('title'), /Not read yet/);
  assert.equal(ui.doc.querySelector('.lcc-bd-insp-title').lastElementChild, lv, 'it follows the specialization name on the same line');
  assert.equal(ui.w.eval('window.LCC_HOST.specLevels()'), null);
  assert.match(tileOf(ui, 'Kenola').querySelector('.lcc-hc-select').getAttribute('title') || tileOf(ui, 'Kenola').querySelector('.lcc-hc-select').getAttribute('data-says'), /Gathering specialization, Level —\. Click to see/);
  // the Specializations tab reads the answer (either view); the board then shows its levels
  click(ui, 'button[data-tab="player"]'); await ui.settle();
  click(ui, '[data-sub="player:specialization"]'); await ui.settle();
  const levels = ui.w.eval('window.LCC_HOST.specLevels()');
  assert.ok(levels && Number.isInteger(levels.gathering), 'the levels come from the answer the tab read: ' + JSON.stringify(levels));
  const want = levels.gathering;
  click(ui, 'button[data-tab="landsraad"]'); await ui.settle();
  if (!ui.doc.querySelector('.lcc-bd-insp-title')) click(ui, tileOf(ui, 'Kenola').querySelector('.lcc-hc-select')); // a chosen House stays chosen when the tab is left and entered again
  lv = ui.doc.querySelector('.lcc-bd-insp-title .lcc-bd-lvl');
  assert.equal(text(lv), 'Level ' + want);
  assert.equal(lv.getAttribute('title'), null, 'a known level needs no explanation');
  const card = tileOf(ui, 'Kenola');
  assert.match(card.querySelector('.lcc-hc-select').getAttribute('title') || card.querySelector('.lcc-hc-select').getAttribute('data-says'), new RegExp('Gathering specialization, Level ' + want + '\\. Click to see'));
  assert.equal(card.querySelector('.lcc-hc-spec').getAttribute('title'), 'Gathering specialization, Level ' + want);
  assert.equal(ui.fetched.filter((p) => p === '/api/player/specs').length, 1, 'one read, made by the Specializations tab');
  // the level belongs to the line: a House of another line shows that line's level
  const other = ui.w.eval('window.LCC_HOST.specLevels()');
  click(ui, tileOf(ui, 'Vernius').querySelector('.lcc-hc-select'));
  assert.equal(text(ui.doc.querySelector('.lcc-bd-insp-title .lcc-bd-lvl')), other.sabotage === undefined ? 'Level —' : 'Level ' + other.sabotage);
  // a write that gave specialization XP makes the levels stale: the dash comes back, never the old number
  click(ui, tileOf(ui, 'Kenola').querySelector('.lcc-hc-select'));
  click(ui, '.lcc-bd-insp .lcc-bd-tier[data-t="3"]'); await ui.settle();
  assert.equal(ui.posted.filter((p) => p.path === '/api/landsraad/apply').length, 1, 'the raise was sent');
  assert.equal(ui.w.eval('window.LCC_HOST.specLevels()'), null, 'the levels are from before the write');
  if (!ui.doc.querySelector('.lcc-bd-insp-title')) click(ui, tileOf(ui, 'Kenola').querySelector('.lcc-hc-select'));
  assert.equal(text(ui.doc.querySelector('.lcc-bd-insp-title .lcc-bd-lvl')), 'Level —', 'the dash comes back, never the old number');
});

test('#646: a line the save holds no level for, and a House with no specialization, show a dash or nothing and never a number', async () => {
  const ui = await open();
  const b = ui.w.LCCM.readBoard(synthetic(), { gathering: 12, combat: -3, crafting: 1.5, sabotage: '7' });
  const lvl = (n) => b.houses.find((h) => h.name === n).level;
  assert.equal(lvl('Kenola'), 12, 'a whole number is passed through');
  assert.equal(b.houses.filter((h) => h.track === 'combat').every((h) => h.level === null), true, 'a negative number is not a level');
  assert.equal(b.houses.filter((h) => h.track === 'crafting').every((h) => h.level === null), true, 'a fraction is not a level');
  assert.equal(b.houses.filter((h) => h.track === 'sabotage').every((h) => h.level === null), true, 'a text is not a level');
  assert.equal(ui.w.LCCM.readBoard(synthetic()).houses.every((h) => h.level === null), true, 'no levels given: every House is unknown');
  assert.equal(ui.w.LCCM.readBoard(synthetic(), null).houses.every((h) => h.level === null), true);
  const a = synthetic();
  a.tasks[0].line = '';
  assert.equal(ui.w.LCCM.readBoard(a, { gathering: 5 }).houses.find((h) => h.id === a.tasks[0].id).level, null, 'a House with no line has no level');
});

test('#1103: the list cells fit: the Place link is short, the Next tier cell keeps "(N more)" apart from the number, and the full words stay in the label and the mouse-over', async () => {
  const ui = await open(synthetic(), { layout: 'list' });
  const rows = [...ui.doc.querySelectorAll('.lcc-bd-list tbody tr.lcc-bd-row')];
  assert.equal(rows.length, 25);
  const lind = rows.find((r) => text(r.querySelector('.lcc-bd-c-house')).startsWith('Lindaren'));
  const place = lind.querySelector('td.lcc-bd-loc .lcc-bd-linkb, td.lcc-bd-loc .lcc-bd-nopic');
  assert.match(text(place), /^Deep Desert · B8$/, 'no word "cell" in the drawn text');
  const said = (place.getAttribute('aria-label') || '') + ' ' + (place.getAttribute('title') || '');
  assert.match(said, /cell B8/, 'the word is kept for a screen reader and the mouse-over');
  const nextRow = rows.find((r) => r.querySelector('.lcc-bd-c-next .lcc-bd-more'));
  assert.ok(nextRow, 'a House with a next tier');
  const next = nextRow.querySelector('.lcc-bd-c-next');
  const more = next.querySelector('.lcc-bd-more');
  assert.match(text(more), /^\(\d[\d,]* more\)$/);
  assert.match(next.firstChild.textContent, /^\d[\d,]*$/, 'the number stands alone in front of the span');
  const hover = next.getAttribute('title') || ui.w.eval('window.LCC_HOST.tips(' + nextRow.dataset.id + ')').next; // the cell's own title, or the reward hover card's heading (the page's text for it)
  assert.match(hover, /\d[\d,]* more/, 'the mouse-over keeps how many more');
  assert.equal(text(next).includes('more'), true, 'the text is still in the cell for a screen reader');
});

test('#1103: the style sheet hides "(N more)" visually only where the list is crowded (a container query), reads it out, and does not give Place a minimum width', () => {
  const css = read('internal', 'web', 'static', 'lcc', 'lcc-board.css');
  const m = css.match(/@container \(max-width: (\d+)px\) \{\s*\.lcc-bd-more \{([^}]*)\}/);
  assert.ok(m, 'a container query holds the rule');
  assert.match(m[2], /position: absolute/);
  assert.match(m[2], /clip: rect\(0 0 0 0\)/);
  assert.doesNotMatch(m[2], /display: none|visibility: hidden/, 'read out, not removed');
  assert.ok(!/\.lcc-bd-loc[^{]*\{[^}]*min-width/.test(css) && !/lcc-bd-c-loc[^{]*\{[^}]*min-width/.test(css), 'a minimum width on Place cut the Not collected column (#1103)');
});

test('the summary line, the level and the list cells ask for nothing: the board makes the same reads as before', async () => {
  const ui = await open(synthetic(), { layout: 'list' });
  const reads = [...new Set(ui.fetched)].filter((p) => p.startsWith('/api/landsraad') || p.startsWith('/api/player/specs'));
  assert.deepEqual(reads, ['/api/landsraad']);
  assert.equal(ui.posted.length, 0);
});

// #1108: the Landsraad answer carries the level of each line, so the board says Level N at once; read only, no request of its own, no write path
const LEVELS = { Gathering: 12, Sabotage: 40 };
const levelText = (ui) => text(ui.doc.querySelector('.lcc-bd-insp-title .lcc-bd-lvl'));

test('#1108: the House panel and the tile say Level N from the Landsraad answer at once, without the Specializations tab and without a request of its own', async () => {
  const a = synthetic();
  a.specialization_levels = { ...LEVELS };
  const ui = await open(a);
  click(ui, tileOf(ui, 'Kenola').querySelector('.lcc-hc-select'));
  assert.equal(levelText(ui), 'Level 12', 'a Gathering House, no visit to the Specializations tab');
  const lv = ui.doc.querySelector('.lcc-bd-insp-title .lcc-bd-lvl');
  assert.equal(lv.getAttribute('title'), null, 'a known level needs no explanation');
  const card = tileOf(ui, 'Kenola');
  assert.equal(card.querySelector('.lcc-hc-spec').getAttribute('title'), 'Gathering specialization, Level 12');
  assert.deepEqual(JSON.parse(JSON.stringify(ui.w.eval('window.LCC_HOST.specLevels()'))), { gathering: 12, sabotage: 40 });
  // the level belongs to the line; a line the answer holds no level for stays a dash, never a number
  click(ui, tileOf(ui, 'Vernius').querySelector('.lcc-hc-select'));
  assert.equal(levelText(ui), 'Level 40');
  const crafting = [...ui.doc.querySelectorAll('.lcc-bd-cell')].find((c) => /crafting/i.test(c.querySelector('.lcc-hc-spec') ? c.querySelector('.lcc-hc-spec').getAttribute('title') || '' : ''));
  if (crafting) { click(ui, crafting.querySelector('.lcc-hc-select')); assert.equal(levelText(ui), 'Level —'); }
  assert.equal(ui.fetched.filter((p) => p === '/api/player/specs').length, 0, 'no request of the Specializations tab was needed');
  assert.equal(ui.posted.length, 0, 'nothing is written');
});

test('#1108: an old save whose answer has no levels (no track rows) shows the same dash as before, and so does a malformed field', async () => {
  for (const bad of [undefined, null, {}, { Gathering: -1, Sabotage: 2.5, Combat: '9' }, 'x']) {
    const a = synthetic();
    if (bad !== undefined) a.specialization_levels = bad;
    const ui = await open(a);
    click(ui, tileOf(ui, 'Kenola').querySelector('.lcc-hc-select'));
    assert.equal(levelText(ui), 'Level —', 'no number is made up for ' + JSON.stringify(bad));
    assert.equal(ui.w.eval('window.LCC_HOST.specLevels()'), null);
    await closeAll();
  }
});

test('#1108: a write that gave specialization XP drops the old level, and the re-read answer brings the new one; without levels in it the dash stays', async () => {
  const a = synthetic();
  a.specialization_levels = { ...LEVELS };
  const ui = await open(a);
  click(ui, tileOf(ui, 'Kenola').querySelector('.lcc-hc-select'));
  assert.equal(levelText(ui), 'Level 12');
  a.specialization_levels = { Gathering: 15, Sabotage: 40 }; // what the save holds after the write, read again by the page
  click(ui, '.lcc-bd-insp .lcc-bd-tier[data-t="3"]'); await ui.settle();
  assert.equal(ui.posted.filter((p) => p.path === '/api/landsraad/apply').length, 1, 'the raise was sent');
  if (!ui.doc.querySelector('.lcc-bd-insp-title')) click(ui, tileOf(ui, 'Kenola').querySelector('.lcc-hc-select'));
  assert.equal(levelText(ui), 'Level 15', 'the new level of the re-read answer, never the old 12');
  assert.deepEqual(JSON.parse(JSON.stringify(ui.w.eval('window.LCC_HOST.specLevels()'))), { gathering: 15, sabotage: 40 });
});

test('#1108: after a write, an answer without levels leaves the dash, and a later visit to the Specializations tab is the newest read', async () => {
  const a = synthetic();
  a.specialization_levels = { ...LEVELS };
  const ui = await open(a);
  delete a.specialization_levels;
  click(ui, tileOf(ui, 'Kenola').querySelector('.lcc-hc-select'));
  click(ui, '.lcc-bd-insp .lcc-bd-tier[data-t="3"]'); await ui.settle();
  assert.equal(ui.w.eval('window.LCC_HOST.specLevels()'), null, 'the 12 from before the write is not shown');
  click(ui, 'button[data-tab="player"]'); await ui.settle();
  click(ui, '[data-sub="player:specialization"]'); await ui.settle();
  const tab = ui.w.eval('window.LCC_HOST.specLevels()');
  assert.ok(tab && Number.isInteger(tab.gathering), 'the Specializations tab read is the newest and is used');
});
