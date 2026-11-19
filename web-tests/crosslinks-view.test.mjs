// #1060: the links between the Landsraad and the Specializations tabs, and the way back from the Saved map, do not rewrite the view the person chose.
// "Open in Specializations" (House Inspector) and "Open House" (a House of a path) used to save the Command Center view for good (browser storage keys spec.view and lsb.view), so a Classic user was
// switched for good. They show the other view for this visit only now; the saved choice is untouched and the View switch is still there. The Saved map's "Showing House X" line has a way back to the
// House, in the view the person is in. Display and navigation only: nothing is written.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import { boot, closeAll } from './boot.mjs';
import { synthetic } from './lcc-fixture.mjs';
import { read } from './lcc-load.mjs';

afterEach(closeAll);
const CASES = JSON.parse(read('docs', 'design', 'landsraad-command-center', 'fixtures', 'api-player-specs.cases.json')).cases;
const SPECS = () => JSON.parse(JSON.stringify(CASES.find((c) => c.id === 'mixed-five-tracks').response));
const click = (ui, el) => { if (typeof el === 'string') el = ui.doc.querySelector(el); assert.ok(el, 'no element'); el.focus(); el.dispatchEvent(new ui.w.MouseEvent('click', { bubbles: true, cancelable: true })); };
const text = (e) => (e ? e.textContent.replace(/\s+/g, ' ').trim() : null);

const CFG = { key: 'HaggaBasin', label: 'Hagga Basin', image: '/maps/hagga-basin.png', width: 4096, height: 4096, minX: -456752.21, maxX: 354547.46, minY: -450630.14, maxY: 353821.95 };
const livemap = () => ({ config: CFG, maps: [{ key: CFG.key, label: CFG.label }], character: null, vehicles: [], bases: [], storage: [], resourceFields: [], markers: [] });
function board() {
  const a = synthetic();
  for (const t of a.tasks) t.location = { known: false };
  a.tasks.find((t) => t.house_name === 'DA_HouseKenola').location = { known: true, map: CFG.label, map_key: CFG.key, area: 'The O’dham', place: '', cell: '', unconfirmed: false, x: 0.25, y: 0.5, from_you_cm: 150000 };
  return a;
}
const kenola = (a) => a.tasks.find((t) => t.house_name === 'DA_HouseKenola');

test('#1060: Open House from the Specializations shows the board for this visit and keeps the saved Classic choice', async () => {
  const a = synthetic();
  const ui = await boot({ get: { '/api/player/specs': SPECS(), '/api/landsraad': a }, local: { tab: 'player', 'spec.view': 'cc', 'lsb.view': 'classic' } });
  ui.click('[data-sub="player:specialization"]'); await ui.settle();
  const house = a.tasks.find((t) => t.line === 'Crafting');
  click(ui, '[data-lcs-go="' + house.id + '"]'); await ui.settle(); await ui.wait(20); await ui.settle();
  assert.equal(ui.w.localStorage.getItem('tab'), 'landsraad');
  assert.ok(ui.doc.querySelector('.lcc-bd-insp[data-id="' + house.id + '"]'), 'the House is open on the board');
  assert.equal(ui.w.localStorage.getItem('lsb.view'), 'classic', 'the saved choice is not rewritten');
  assert.equal(ui.doc.querySelector('[data-lb="view"][data-view="board"]').getAttribute('aria-pressed'), 'true', 'the View switch says which view is shown');
  click(ui, '[data-lb="view"][data-view="classic"]'); await ui.settle();
  assert.equal(ui.w.localStorage.getItem('lsb.view'), 'classic');
  assert.ok(ui.doc.querySelector('#lsb-rows'), 'the switch is the way back to the Classic list');
  assert.deepEqual(ui.posted, [], 'nothing is written');
});

test('#1060: Open in Specializations from the House Inspector shows the Command Center view for this visit and keeps the saved Classic choice', async () => {
  const ui = await boot({ get: { '/api/player/specs': SPECS(), '/api/landsraad': synthetic() }, local: { tab: 'landsraad', 'lsb.view': 'board', 'lsb.layout': 'grid', 'spec.view': 'classic' } });
  await ui.settle();
  let went = null;
  for (const c of ui.doc.querySelectorAll('.lcc-bd-cell .lcc-hc-select')) { click(ui, c); const g = ui.doc.querySelector('[data-lcb="gopath"]'); if (g) { went = g; break; } }
  assert.ok(went, 'a House with a path');
  click(ui, went); await ui.settle();
  assert.ok(ui.doc.querySelector('#lcs-mount .lcc-sp'), 'the Specializations view is drawn');
  assert.equal(ui.w.localStorage.getItem('spec.view'), 'classic', 'the saved choice is not rewritten');
  assert.equal(ui.doc.querySelector('[data-spview="cc"]').getAttribute('aria-pressed'), 'true');
  click(ui, '[data-spview="classic"]'); await ui.settle();
  assert.ok(ui.doc.querySelector('table'), 'the switch is the way back to the Classic list');
  assert.equal(ui.w.localStorage.getItem('spec.view'), 'classic');
  assert.deepEqual(ui.posted, [], 'nothing is written');
});

test('#1060: the Saved map line "Showing House X" has a way back to that House, in the Classic list', async () => {
  const a = board(), id = kenola(a).id;
  const ui = await boot({ get: { '/api/landsraad': a, '/api/livemap': livemap() }, local: { tab: 'landsraad', 'lsb.view': 'classic' } });
  await ui.settle();
  click(ui, `#lsrow-${id} .loc-t[data-lb="map"]`); await ui.settle();
  const back = ui.doc.querySelector('#lmhouse [data-lmback]');
  assert.ok(back, 'the way back');
  assert.equal(text(back), 'Back to House Kenola');
  assert.equal(back.dataset.lmback, String(id));
  assert.match(text(ui.doc.querySelector('#lmhouse')), /^Showing House Kenola on Hagga Basin\./);
  assert.doesNotMatch(text(ui.doc.querySelector('#lmhouse')), /yellow ring/, 'the standing sentence is a hover text now (#1062)');
  assert.match(ui.doc.querySelector('#lmhouse').getAttribute('title'), /yellow ring/);
  click(ui, back); await ui.settle();
  assert.equal(ui.w.localStorage.getItem('tab'), 'landsraad');
  assert.ok(ui.doc.querySelector('#lsb-rows'), 'the Classic list, as it was');
  assert.equal(ui.doc.querySelector('#lsb-jump').value, String(id), 'only that House is shown');
  assert.equal(ui.doc.querySelector(`#lsrow-${id}`).hidden, false);
  assert.equal(ui.doc.querySelector(`#lsx-${id}`).checked, true, 'and its row is open');
  assert.equal(ui.w.localStorage.getItem('lsb.view'), 'classic');
  assert.deepEqual(ui.posted, [], 'nothing is written');
});

test('#1060: and in the Command Center board, with that House chosen', async () => {
  const a = board(), id = kenola(a).id;
  const ui = await boot({ get: { '/api/landsraad': a, '/api/livemap': livemap() }, local: { tab: 'landsraad', 'lsb.view': 'board', 'lsb.layout': 'grid' } });
  await ui.settle();
  click(ui, `[data-fk="ht:${id}"]`); await ui.settle();
  click(ui, '.lcc-bd-insp .lcc-bd-loc[data-lb="map"]'); await ui.settle();
  assert.ok(ui.doc.querySelector('#lmhouse [data-lmback]'));
  ui.w.LCCB.state.sel = null; // the choice on the board is gone while the Saved map is open
  click(ui, '#lmhouse [data-lmback]'); await ui.settle();
  assert.equal(ui.w.localStorage.getItem('tab'), 'landsraad');
  assert.equal(ui.w.LCCB.state.sel, id);
  assert.ok(ui.doc.querySelector('.lcc-bd-insp[data-id="' + id + '"]'), 'the House is open on the board');
  assert.deepEqual(ui.posted, [], 'nothing is written');
});
