// #1072: a link that opens the Landsraad tab at one House ("Back to House X" on the Saved map, "Open House" of a path on the Specializations page) opens the page with that House chosen AND its card or row
// scrolled into view, not at the top of the page. A move to another page scrolls to the top before the new page draws (#1025), so the scroll to the House has to come after the draw: render() runs a request made
// with askReveal once, after the draw (the same mechanism as the Specialization links, #1041). Real app.js and lcc scripts over the synthetic answer of M1; jsdom has no layout, so the test records the scrolls
// (window.scrollTo and scrollIntoView) in the order they happen. The real scroll position is checked in a browser engine by the sweep and by the headless check of the fix.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import { boot, closeAll } from './boot.mjs';
import { synthetic } from './lcc-fixture.mjs';
import { read } from './lcc-load.mjs';

afterEach(closeAll);
const CASES = JSON.parse(read('docs', 'design', 'landsraad-command-center', 'fixtures', 'api-player-specs.cases.json')).cases;
const SPECS = () => JSON.parse(JSON.stringify(CASES.find((c) => c.id === 'mixed-five-tracks').response));
const click = (ui, el) => { if (typeof el === 'string') el = ui.doc.querySelector(el); assert.ok(el, 'no element'); el.focus(); el.dispatchEvent(new ui.w.MouseEvent('click', { bubbles: true, cancelable: true })); };
const CFG = { key: 'HaggaBasin', label: 'Hagga Basin', image: '/maps/hagga-basin.png', width: 4096, height: 4096, minX: -456752.21, maxX: 354547.46, minY: -450630.14, maxY: 353821.95 };
const livemap = () => ({ config: CFG, maps: [{ key: CFG.key, label: CFG.label }], character: null, vehicles: [], bases: [], storage: [], resourceFields: [], markers: [] });
function landsraad() {
  const a = synthetic();
  for (const t of a.tasks) t.location = { known: false };
  a.tasks.find((t) => t.house_name === 'DA_HouseKenola').location = { known: true, map: CFG.label, map_key: CFG.key, area: 'The O’dham', place: '', cell: '', unconfirmed: false, x: 0.25, y: 0.5, from_you_cm: 150000 };
  return a;
}
const kenola = (a) => a.tasks.find((t) => t.house_name === 'DA_HouseKenola');
const where = (n) => (n.closest('.lcc-bd-cell') ? 'board-card' : n.closest('.lcc-bd-row') ? 'board-row' : 'elsewhere:' + n.className);

async function open(local, extra = {}) {
  const log = [], a = landsraad();
  const ui = await boot({
    get: { '/api/landsraad': a, '/api/livemap': livemap(), ...extra }, local,
    pre: (w) => {
      w.scrollTo = (x, y) => log.push(['top', y]);
      w.Element.prototype.scrollIntoView = function (o) { log.push(['into', this, o && o.block]); };
    },
  });
  await ui.settle();
  return { ui, log, a, id: kenola(a).id };
}
// The Saved map line "Showing House X": the place button of the House opens it, from the Inspector (grid or list) or, with `cell`, from the place cell of the list row
async function viaMap(ui, id, cell) {
  if (cell) click(ui, `tr.lcc-bd-row[data-id="${id}"] [data-lb="map"]`);
  else { click(ui, `[data-fk="ht:${id}"]`); await ui.settle(); click(ui, '.lcc-bd-insp .lcc-bd-loc[data-lb="map"]'); }
  await ui.settle();
  assert.ok(ui.doc.querySelector('#lmhouse [data-lmback]'), 'the Saved map has the way back');
}
function checkOrder(log, id, kind, block) {
  const into = log.filter((e) => e[0] === 'into');
  assert.equal(into.length, 1, 'one scroll into view: ' + JSON.stringify(log.map((e) => e[0])));
  assert.equal(where(into[0][1]), kind, 'what is scrolled to');
  assert.equal(String(into[0][1].closest('[data-id]').getAttribute('data-id')), String(id), 'the House that was asked for');
  assert.equal(into[0][2], block);
  const top = log.findIndex((e) => e[0] === 'top'), at = log.findIndex((e) => e[0] === 'into');
  assert.ok(top >= 0 && at > top, 'the move to the top comes first and the scroll to the House after it, so it is not undone: ' + JSON.stringify(log.map((e) => e[0])));
}

test('#1072: Back to House on the Saved map opens the board with the House chosen and its card scrolled into view', async () => {
  const { ui, log, id } = await open({ tab: 'landsraad', 'lsb.layout': 'grid' });
  await viaMap(ui, id, false);
  ui.w.LCCB.state.sel = null;
  log.length = 0;
  click(ui, '#lmhouse [data-lmback]'); await ui.settle();
  assert.ok(ui.doc.querySelector('.lcc-bd-insp[data-id="' + id + '"]'), 'the House is chosen in the Inspector');
  checkOrder(log, id, 'board-card', 'center');
  assert.equal(ui.doc.activeElement, ui.doc.querySelector(`[data-fk="ht:${id}"]`), 'the focus is on the House, as after Jump to House');
  assert.deepEqual(ui.posted, [], 'nothing is written');
});

test('#1072: and in the list layout of the board', async () => {
  const { ui, log, id } = await open({ tab: 'landsraad', 'lsb.layout': 'list' });
  await viaMap(ui, id, false);
  ui.w.LCCB.state.sel = null;
  log.length = 0;
  click(ui, '#lmhouse [data-lmback]'); await ui.settle();
  assert.ok(ui.doc.querySelector('.lcc-bd-row'), 'the list layout is drawn');
  checkOrder(log, id, 'board-row', 'center');
});

test('#1072: and from the place cell of a row of the list layout', async () => {
  const { ui, log, id } = await open({ tab: 'landsraad', 'lsb.layout': 'list' });
  await viaMap(ui, id, true);
  ui.w.LCCB.state.sel = null;
  log.length = 0;
  click(ui, '#lmhouse [data-lmback]'); await ui.settle();
  assert.ok(ui.doc.querySelector('.lcc-bd-insp[data-id="' + id + '"]'), 'the House is chosen in the Inspector');
  checkOrder(log, id, 'board-row', 'center');
  assert.deepEqual(ui.posted, [], 'nothing is written');
});

test('#1072: Open House of a path on the Specializations page scrolls to the House on the board too (the same mechanism)', async () => {
  const { ui, log, a } = await open({ tab: 'player', 'lsb.view': 'classic' }, { '/api/player/specs': SPECS() });
  ui.click('[data-sub="player:specialization"]'); await ui.settle();
  const house = a.tasks.find((t) => t.line === 'Crafting');
  log.length = 0;
  click(ui, '[data-lcs-go="' + house.id + '"]'); await ui.settle();
  assert.ok(ui.doc.querySelector('.lcc-bd-insp[data-id="' + house.id + '"]'));
  checkOrder(log, house.id, 'board-card', 'center');
  assert.equal(ui.w.localStorage.getItem('lsb.view'), null, 'the old saved Classic choice is gone (#1060, #1028): the board is the only view');
});

test('#1072: a visit by the tabs is not scrolled to the House again, and starts at the top', async () => {
  const { ui, log, id } = await open({ tab: 'landsraad', 'lsb.layout': 'grid' });
  await viaMap(ui, id, false);
  click(ui, '#lmhouse [data-lmback]'); await ui.settle();
  assert.ok(log.some((e) => e[0] === 'into'), 'the link scrolled');
  click(ui, ui.doc.querySelector('[data-tab="player"]')); await ui.settle();
  log.length = 0;
  click(ui, ui.doc.querySelector('[data-tab="landsraad"]')); await ui.settle();
  assert.equal(log.filter((e) => e[0] === 'into').length, 0, 'a plain visit is not scrolled to a House: ' + JSON.stringify(log.map((e) => e[0])));
  assert.ok(log.some((e) => e[0] === 'top'), 'and is moved to the top');
});

test('#1072: Find House (the jump of the control area, tau #974) still scrolls to the card once, and the place link of a House still opens the Saved map without scrolling to a card', async () => {
  const { ui, log, id } = await open({ tab: 'landsraad', 'lsb.layout': 'grid' });
  assert.ok(ui.doc.querySelector('[data-lk="findbtn"]'), 'the Find House key');
  log.length = 0;
  click(ui, '[data-lk="findbtn"]');
  click(ui, '[data-lk="fpick"][data-id="' + id + '"]');
  assert.equal(log.filter((e) => e[0] === 'into').length, 1, 'Find House scrolls to the card once');
  assert.equal(where(log.find((e) => e[0] === 'into')[1]), 'board-card', 'the card is what is scrolled to');
  click(ui, '[data-fk="ht:' + id + '"]'); await ui.settle(); // Find House opens no panel and ticks nothing: the House is chosen as before, by its card
  click(ui, '.lcc-bd-insp .lcc-bd-loc[data-lb="map"]'); await ui.settle();
  assert.ok(ui.doc.querySelector('#lmhouse'), 'the Saved map shows the House');
  assert.equal(log.filter((e) => e[0] === 'into' && e[1].closest && e[1].closest('.lcc-bd-cell')).length, 1, 'only Find House scrolled to a card; the link to the map did not');
});
