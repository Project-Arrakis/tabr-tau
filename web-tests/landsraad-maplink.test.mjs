// #1025: every place link of a House on the Landsraad pages opens the Saved map on the right map, with the map scrolled into view and that House's pin marked, and says which House it shows.
// The bug: the page kept the scroll offset of the board it had just left, and the Saved map (a shorter page) ended up scrolled to the table of marker types at its bottom, with no map in view.
// The test follows each link variant (Classic list row, Classic hover card, Command Center list link, list and grid hover cards, the inspector's place line and its Open in the Saved map action, and
// the Jump to House then the inspector) for a House on every map the Saved map has (Hagga Basin, the Deep Desert, the two hubs). jsdom has no layout, so what it can check is where the page asks
// to scroll: the Saved map card is the scroll target (scrollIntoView, which the style sheet's scroll-padding keeps clear of the sticky header) and the page is not left at the old offset.
// The real offsets in a browser engine are measured by the headless sweep (scripts/lcc-m7-sweep.mjs, case maplink-*). Display and navigation only: nothing is written.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import { boot, closeAll } from './boot.mjs';
import { synthetic } from './lcc-fixture.mjs';

afterEach(closeAll);

const CFG = {
  HaggaBasin: { key: 'HaggaBasin', label: 'Hagga Basin', image: '/maps/hagga-basin.png', width: 4096, height: 4096, minX: -456752.21, maxX: 354547.46, minY: -450630.14, maxY: 353821.95 },
  DeepDesert: { key: 'DeepDesert', label: 'The Deep Desert', image: '/maps/deep-desert.png', width: 4096, height: 4096, minX: -1, maxX: 1, minY: -1, maxY: 1 },
  Arrakeen: { key: 'Arrakeen', label: 'Arrakeen', image: '', width: 4096, height: 2342, minX: -27190, maxX: 17043.26, minY: -12790, maxY: 12501.58 },
  HarkoVillage: { key: 'HarkoVillage', label: 'Harko Village', image: '', width: 4096, height: 4096, minX: -24900, maxX: 34800, minY: -22360, maxY: 37340 },
};
const MAPS = Object.values(CFG).map((c) => ({ key: c.key, label: c.label }));
const livemap = (key) => ({ config: CFG[key], maps: MAPS, character: null, vehicles: [], bases: [], storage: [], resourceFields: [], markers: [] });
// a House on each map: [house, map key, x, y (fractions of the map picture), area]
const PLACES = [['Kenola', 'HaggaBasin', 0.25, 0.5, 'The O’dham'], ['Vernius', 'DeepDesert', 0.6, 0.4, 'Deep Desert'], ['Alexin', 'Arrakeen', 0.6965799039003683, 0.37830772138395463, 'Arrakeen'], ['Richese', 'HarkoVillage', 0.5842211055276382, 0.5672194304857622, 'Harko Village']];
function board() {
  const a = synthetic();
  for (const t of a.tasks) t.location = { known: false };
  for (const [name, key, x, y, area] of PLACES) a.tasks.find((t) => t.house_name === 'DA_House' + name).location = { known: true, map: CFG[key].label, map_key: key, area, place: '', cell: '', unconfirmed: false, x, y, from_you_cm: 150000 };
  return a;
}
const idOf = (a, name) => a.tasks.find((t) => t.house_name === 'DA_House' + name).id;
const click = (ui, el) => { if (typeof el === 'string') el = ui.doc.querySelector(el); assert.ok(el, 'no element'); el.focus(); el.dispatchEvent(new ui.w.MouseEvent('click', { bubbles: true, cancelable: true })); };

// every place link of the pages, by the way a person reaches it: where the page is shown (view, layout), what is done first, and the link
const VARIANTS = [
  { name: 'Classic list: the place link of the row', view: 'classic', layout: 'grid', link: (id) => `#lsrow-${id} .loc-t[data-lb="map"]` },
  { name: 'Classic list: the Place line of the hover card', view: 'classic', layout: 'grid', link: (id) => `#lsrow-${id} .hcard .hcard-place` },
  { name: 'Command Center list: the place link of the row', view: 'board', layout: 'list', link: (id) => `td.lcc-bd-loc .lcc-bd-linkb[data-id="${id}"]` },
  { name: 'Command Center list: the Place line of the hover card', view: 'board', layout: 'list', link: (id) => `.lcc-bd-row .hcard-place[data-id="${id}"]` },
  { name: 'Command Center grid: the Place line of the hover card', view: 'board', layout: 'grid', link: (id) => `li.lcc-bd-cell[data-id="${id}"] .hcard-place` },
  { name: 'Command Center inspector: the place line under the House name', view: 'board', layout: 'grid', pre: (ui, id) => click(ui, `[data-fk="ht:${id}"]`), link: () => '.lcc-bd-insp .lcc-bd-loc[data-lb="map"]' },
  { name: 'Command Center inspector: the Open in the Saved map action', view: 'board', layout: 'grid', pre: (ui, id) => click(ui, `[data-fk="ht:${id}"]`), link: () => '.lcc-bd-insp .lcc-bd-act[data-lb="map"]' },
  { name: 'Command Center: Jump to House, then the inspector', view: 'board', layout: 'grid', pre: (ui, id) => { const j = ui.doc.querySelector('[data-lcb="jump"]'); assert.ok(j, 'the jump'); j.value = String(id); j.dispatchEvent(new ui.w.Event('change', { bubbles: true })); }, link: () => '.lcc-bd-insp .lcc-bd-act[data-lb="map"]' },
];

// what the page was asked to scroll, in order: the window to an offset, an element into view (by its id or its class)
function watchScroll(ui) {
  const log = [];
  ui.w.scrollTo = (x, y) => { log.push(['window', x, y]); };
  ui.w.Element.prototype.scrollIntoView = function (o) { log.push(['into', this.id || this.className, o && o.block]); };
  return log;
}

for (const v of VARIANTS) {
  for (const [name, key, fx, fy, area] of PLACES) {
    test(`#1025: ${v.name}, a House on ${CFG[key].label}: the Saved map opens on that map, scrolled to the map, with the pin marked and the House named`, async () => {
      const a = board(), id = idOf(a, name);
      const ui = await boot({ get: { '/api/landsraad': a, '/api/livemap': livemap(key) }, local: { tab: 'landsraad', 'lsb.view': v.view, 'lsb.layout': v.layout } });
      await ui.settle();
      if (v.pre) { v.pre(ui, id); await ui.settle(); }
      const link = ui.doc.querySelector(v.link(id));
      assert.ok(link, 'the link is on the page: ' + v.link(id));
      assert.notEqual(link.getAttribute('aria-disabled'), 'true', 'a place on a map the Saved map has is a link');
      const log = watchScroll(ui);
      ui.fetchedUrls.length = 0;
      click(ui, link); await ui.settle();
      // the Saved map is asked for the right map
      assert.equal(ui.doc.querySelector('[data-tab=livemap]').getAttribute('aria-current'), 'page');
      assert.ok(ui.fetchedUrls.includes('/api/livemap?map=' + key), ui.fetchedUrls.join(' '));
      const s = ui.w.liveMapState();
      assert.equal(s.map, key);
      // the pin: the House's place, selected, and the view centred on it
      assert.deepEqual(JSON.parse(JSON.stringify(s.focus)), { key, fx, fy, label: name });
      assert.ok(s.view && s.view.z === 4 && Math.abs(s.view.cx - fx * CFG[key].width) < 1e-6 && Math.abs(s.view.cy - fy * CFG[key].height) < 1e-6, JSON.stringify(s.view));
      // the page does not keep the offset of the board: it goes to the top while the other page is drawn, and then the Saved map card is the scroll target
      const top = log.findIndex((e) => e[0] === 'window' && e[1] === 0 && e[2] === 0), card = log.findIndex((e) => e[0] === 'into' && e[1] === 'lmcard');
      assert.ok(top >= 0, 'the page went to the top: ' + JSON.stringify(log));
      assert.ok(card > top, 'then the Saved map card was scrolled to: ' + JSON.stringify(log));
      assert.equal(log[card][2], 'start', 'its top edge (the sticky header is kept clear by the style sheet)');
      assert.ok(ui.doc.querySelector('#main #lmcard #lmc'), 'the map is in that card');
      // the line that says which House is shown, next to the map
      const line = ui.doc.querySelector('#lmhouse');
      assert.ok(line, 'a visible line names the House');
      assert.equal(line.nextElementSibling.className, 'lm-wrap', 'right above the map');
      const t = line.textContent.replace(/\s+/g, ' ').trim();
      assert.ok(t.startsWith(`Showing House ${name} on ${CFG[key].label}.`), t);
      assert.match(ui.doc.querySelector('#lmc').getAttribute('aria-label'), new RegExp(`^Saved map of ${CFG[key].label}, centred on House ${name}\\.`));
      // the keyboard lands on the map, not on the page body or the table
      assert.equal(ui.doc.activeElement.id, 'lmc');
      void area;
    });
  }
}

test('#1025: the page starts at the top after a move to another page by the tab bar, and keeps its place after its own redraw', async () => {
  const a = board();
  const ui = await boot({ get: { '/api/landsraad': a, '/api/livemap': livemap('HaggaBasin') }, local: { tab: 'landsraad', 'lsb.view': 'board', 'lsb.layout': 'grid' } });
  await ui.settle();
  const log = watchScroll(ui);
  click(ui, '[data-tab="livemap"]'); await ui.settle();
  assert.ok(log.some((e) => e[0] === 'window' && e[1] === 0 && e[2] === 0), 'a move to another page starts at the top: ' + JSON.stringify(log));
  assert.equal(ui.doc.querySelector('#lmhouse'), null, 'no House named when the Saved map was opened from the tab bar');
  assert.ok(!log.some((e) => e[0] === 'into' && e[1] === 'lmcard'), 'and the map card is not scrolled to');
  // the choice of another map is a redraw of the same page: the offset is not sent to the top
  log.length = 0;
  click(ui, '[data-lmmap="DeepDesert"]'); await ui.settle();
  assert.deepEqual(log.filter((e) => e[0] === 'window' && e[2] === 0), [], 'no jump to the top for another map of the same page: ' + JSON.stringify(log));
});

test('#1025: "Stop showing it" takes the House off the Saved map (the ring and the line) and writes nothing', async () => {
  const a = board(), id = idOf(a, 'Kenola');
  const ui = await boot({ get: { '/api/landsraad': a, '/api/livemap': livemap('HaggaBasin') }, local: { tab: 'landsraad', 'lsb.view': 'classic' } });
  await ui.settle();
  click(ui, `#lsrow-${id} .loc-t[data-lb="map"]`); await ui.settle();
  assert.ok(ui.doc.querySelector('#lmhouse'));
  click(ui, '[data-lmclear]'); await ui.settle();
  assert.equal(ui.doc.querySelector('#lmhouse'), null);
  assert.equal(ui.w.liveMapState().focus, null);
  assert.doesNotMatch(ui.doc.querySelector('#lmc').getAttribute('aria-label'), /centred on/);
  assert.deepEqual(ui.posted, [], 'nothing is written');
});

test('#1025: a place with no map picture of its own (a hub) is still a link, and the Saved map draws its outline with the House marked', async () => {
  const a = board(), id = idOf(a, 'Alexin');
  const ui = await boot({ get: { '/api/landsraad': a, '/api/livemap': livemap('Arrakeen') }, local: { tab: 'landsraad', 'lsb.view': 'classic' } });
  await ui.settle();
  click(ui, `#lsrow-${id} .loc-t[data-lb="map"]`); await ui.settle();
  assert.ok(ui.doc.querySelector('#lmnopic'), 'says the map has no picture yet');
  assert.match(ui.doc.querySelector('#lmhouse').textContent, /Showing House Alexin on Arrakeen/);
});

test('#1025: a click on the quick peek (the small map under a place, a sheet over the link at a narrow window) is a click on the place', async () => {
  for (const [view, layout, pop] of [['classic', 'grid', (i) => `#lsrow-${i} .loc > .pop`], ['board', 'list', (i) => `.lcc-bd-row .hcard .loc > .pop`]]) {
    const a = board(), hid = idOf(a, 'Kenola');
    const ui = await boot({ get: { '/api/landsraad': a, '/api/livemap': livemap('HaggaBasin') }, local: { tab: 'landsraad', 'lsb.view': view, 'lsb.layout': layout } });
    await ui.settle();
    const p = [...ui.doc.querySelectorAll(pop(hid))].find((n) => n.querySelector('.minimap') && /Kenola/.test(n.textContent));
    assert.ok(p, view + ': the peek of Kenola');
    click(ui, p.querySelector('.minimap')); await ui.settle();
    assert.equal(ui.w.liveMapState().focus.label, 'Kenola', view);
    assert.ok(ui.doc.querySelector('#lmhouse'), view);
  }
});
