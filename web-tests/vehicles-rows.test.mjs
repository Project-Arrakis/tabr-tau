// Player > Vehicles, the rows (#1230): there is no Details button; a row opens its details under it from its name (a button: keyboard and pointer) or from a tap on the text of its
// other cells, one row at a time, and says whether it is open. The action buttons and empty row space do not open it. A sort keeps the details under their row. jsdom has no layout:
// the narrow window (900 px and below, each vehicle a block) is checked by the rules of app.css, not by a drawn page.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import { boot, closeAll } from './boot.mjs';

afterEach(closeAll);

const veh = (id, o = {}) => ({ id, name: 'BP_Buggy_CHOAM', type: 'Buggy', map: 'HaggaBasin', condition: 60, fuel: 10, x: 1, y: 2, components: [{ template_id: `Engine_${id}`, percent: 80, current: 80, max: 100 }], cargo: null, ...o });
const THREE = [veh(41, { type: 'Sandbike', condition: 90 }), veh(42, { type: 'Buggy', condition: 20 }), veh(43, { type: 'Sandcrawler', condition: 50, stored: 'backup', storedLabel: 'In the vehicle backup' })];

async function open(vehicles = THREE) {
  const ui = await boot({ get: { '/api/vehicles': { vehicles, hidden: 0, recovered: [], backups: [], stored: [] }, '/api/vehicles/fuel': { total: 3, notFull: 0, unknown: 0, noFuelRecord: 0 } },
    post: { '/api/vehicles/repair': { ok: true }, '/api/vehicles/bring': { ok: true } } });
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="player:vehicles"]'); await ui.settle();
  return ui;
}
const btn = (ui, id) => ui.doc.querySelector(`#main [data-vhopen="${id}"]`);
const detail = (ui, id) => ui.doc.getElementById('vdetail' + id);
// which rows say they are open, and which details are drawn and shown: the two must be the same
const opened = (ui) => {
  const said = [...ui.doc.querySelectorAll('#main [data-vhopen][aria-expanded="true"]')].map((b) => +b.dataset.vhopen);
  const shown = [...ui.doc.querySelectorAll('#main tr.vdetail:not([hidden])')].map((r) => +r.id.slice('vdetail'.length));
  assert.deepEqual(shown, said);
  assert.equal(ui.doc.querySelectorAll('#main tr.vdetail .vpane').length, said.length * 2, 'only the open row has panes');
  assert.deepEqual([...ui.doc.querySelectorAll('#main tr.vrow.on [data-vhopen]')].map((b) => +b.dataset.vhopen), said);
  return said;
};

test('no row is open on a fresh page, there is no Details button, and each row has a button that names what it opens', async () => {
  const ui = await open();
  assert.deepEqual(opened(ui), []);
  assert.ok(![...ui.doc.querySelectorAll('#main button')].some((b) => b.textContent.trim() === 'Details'));
  assert.equal(ui.doc.querySelector('#cargoroom'), null);
  for (const id of [41, 42, 43]) {
    const b = btn(ui, id);
    assert.equal(b.tagName, 'BUTTON');
    assert.equal(b.type, 'button');
    assert.equal(b.getAttribute('aria-controls'), 'vdetail' + id);
    assert.equal(b.getAttribute('aria-expanded'), 'false');
    assert.equal(b.closest('tr').nextElementSibling, detail(ui, id), 'its details row is directly under it');
  }
});

test('the row button opens and closes the details, one row at a time, and keeps the focus', async () => {
  const ui = await open();
  btn(ui, 42).focus(); ui.click('[data-vhopen="42"]'); await ui.settle();
  assert.deepEqual(opened(ui), [42]);
  assert.ok(detail(ui, 42).textContent.includes('Buggy: 1 components') && detail(ui, 42).textContent.includes('This vehicle has no cargo hold.'));
  assert.equal(ui.doc.activeElement, btn(ui, 42));
  ui.click('[data-vhopen="41"]'); await ui.settle();
  assert.deepEqual(opened(ui), [41]);
  assert.equal(ui.doc.activeElement, btn(ui, 41));
  ui.click('[data-vhopen="41"]'); await ui.settle();
  assert.deepEqual(opened(ui), []);
  assert.equal(ui.posted.length, 0, 'opening a row writes nothing');
});

test('a tap on the text of any cell opens the row, the Stored line too', async () => {
  const ui = await open();
  assert.equal(ui.doc.querySelectorAll('#main [data-vhtap="41"]').length, 4, 'type, condition, fuel, location');
  for (let i = 0; i < 4; i++) {
    ui.doc.querySelectorAll('#main [data-vhtap="41"]')[i].click(); await ui.settle();
    assert.deepEqual(opened(ui), i % 2 ? [] : [41], 'cell ' + i);
  }
  ui.click('#vstored43'); await ui.settle();
  assert.deepEqual(opened(ui), [43]);
  assert.equal(ui.doc.activeElement, btn(ui, 43), 'the keyboard goes on from the tapped row');
});

test('a double click on text, and a click that ends marking text, do not open a row', async () => {
  const ui = await open();
  const cell = () => ui.doc.querySelector('#main [data-vhtap="41"]');
  cell().dispatchEvent(new ui.w.MouseEvent('click', { bubbles: true, cancelable: true, detail: 2 })); await ui.settle();
  assert.deepEqual(opened(ui), []);
  ui.w.getSelection().selectAllChildren(cell());
  cell().click(); await ui.settle();
  assert.deepEqual(opened(ui), []);
});

test('the action buttons, a button that is off and the empty space of a row do not open or close it', async () => {
  const ui = await open();
  const row = (id) => btn(ui, id).closest('tr');
  for (const td of row(41).querySelectorAll('td')) { td.click(); await ui.settle(); }
  row(41).click(); await ui.settle();
  row(41).querySelector('.vacts').click(); await ui.settle();
  assert.deepEqual(opened(ui), [], 'empty space');
  for (const act of ['repairOne', 'bring']) { ui.click(`[data-act="${act}"][data-id="41"]`); await ui.settle(); assert.deepEqual(opened(ui), [], act); }
  ui.click('[data-act="dupV"][data-id="41"]'); await ui.settle();
  assert.deepEqual(opened(ui), [], 'Duplicate asks, the row stays shut');
  ui.click('#ask [data-act="askNo"]'); await ui.settle();
  for (const act of ['bring', 'dupV']) { row(43).querySelector(`[data-act="${act}"]`).click(); await ui.settle(); assert.deepEqual(opened(ui), [], 'stored ' + act); }
  assert.deepEqual(ui.posted.map((p) => [p.path, p.body]), [['/api/vehicles/repair', { vehicle_id: 41 }], ['/api/vehicles/bring', { id: 41 }]], 'the two writes, as before, and no other');
  // an open row stays open when one of its buttons is pressed
  ui.click('[data-vhopen="41"]'); await ui.settle();
  ui.click('[data-act="repairOne"][data-id="41"]'); await ui.settle();
  assert.deepEqual(opened(ui), [41]);
});

test('sorting moves each details row with its vehicle, and the open row survives a redraw', async () => {
  const ui = await open();
  ui.click('[data-vhopen="42"]'); await ui.settle();
  const pairs = () => [...ui.doc.querySelector('#main .vlist > table').tBodies[0].rows].map((r) => (r.classList.contains('vdetail') ? 'd' + r.id.slice(7) : 'r' + r.querySelector('[data-vhopen]').dataset.vhopen));
  const th = (name) => [...ui.doc.querySelectorAll('#main .vlist > table > thead th')].find((h) => h.textContent === name);
  assert.deepEqual(pairs(), ['r41', 'd41', 'r42', 'd42', 'r43', 'd43']);
  th('Lowest condition').click();
  assert.deepEqual(pairs(), ['r42', 'd42', 'r43', 'd43', 'r41', 'd41']);
  th('Lowest condition').click();
  assert.deepEqual(pairs(), ['r41', 'd41', 'r43', 'd43', 'r42', 'd42']);
  ui.click('[data-vhopen="43"]'); await ui.settle(); // drawn again with the sort kept
  assert.deepEqual(pairs(), ['r41', 'd41', 'r43', 'd43', 'r42', 'd42']);
  assert.deepEqual(opened(ui), [43]);
  th('Lowest condition').click();
  assert.deepEqual(pairs(), ['r41', 'd41', 'r42', 'd42', 'r43', 'd43'], 'back to the original order');
});

test('narrow window: the rows become blocks at 900 px and below, with no column headers, and the actions stay on one line', () => {
  const css = fs.readFileSync(new URL('../internal/web/static/app.css', import.meta.url), 'utf8').replace(/\r/g, '');
  const at = css.indexOf('/* ---- Player > Vehicles'), block = css.slice(css.indexOf('@media (max-width:900px){', at), css.indexOf('\n}', css.indexOf('@media (max-width:900px){', at)));
  assert.ok(at > 0 && block.includes('.vlist>table>thead{display:none}'), block.slice(0, 200));
  assert.ok(block.includes('.vlist>table,.vlist>table>tbody,.vlist>table>tbody>tr,.vlist>table>tbody>tr>td{display:block;width:100%}'));
  assert.ok(css.slice(at).includes('.vacts{display:flex;gap:6px;flex-wrap:nowrap;'));
});
