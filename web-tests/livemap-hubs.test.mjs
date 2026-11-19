// The Saved map opens the two social hubs, Arrakeen and Harko Village (#758): the map selector offers them, each has its picture (a map with none is drawn as its outline and says so), a world position turns into
// the place on the hub's map the same way as on the other maps, and the Landsraad board's link from a House at a hub lands on that hub, centred on the House.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import { boot, closeAll } from './boot.mjs';

afterEach(closeAll);

const HAGGA = { key: 'HaggaBasin', label: 'Hagga Basin', image: '/maps/hagga-basin.png', width: 4096, height: 4096, minX: -456752.21, maxX: 354547.46, minY: -450630.14, maxY: 353821.95 };
// the hubs' configurations as the server gives them (internal/ops/livemap.go, hubMaps): each with its picture
const ARRAKEEN = { key: 'Arrakeen', label: 'Arrakeen', image: '/maps/arrakeen.png', width: 4096, height: 2342, minX: -27190, maxX: 17043.26, minY: -12790, maxY: 12501.58 };
const HARKO = { key: 'HarkoVillage', label: 'Harko Village', image: '/maps/harko-village.png', width: 4096, height: 4096, minX: -24900, maxX: 34800, minY: -22360, maxY: 37340 };
const MAPS = [{ key: 'HaggaBasin', label: 'Hagga Basin' }, { key: 'DeepDesert', label: 'The Deep Desert' }, { key: 'Arrakeen', label: 'Arrakeen' }, { key: 'HarkoVillage', label: 'Harko Village' }];
const MAP = (config, extra) => ({ config, maps: MAPS, character: null, vehicles: [], bases: [], storage: [], resourceFields: [], markers: [], ...extra });
const text = (el) => el.textContent.replace(/\s+/g, ' ').trim();

async function openMap(payload) {
  const ui = await boot({ get: { '/api/livemap': payload } });
  ui.click('[data-tab="livemap"]'); await ui.settle();
  return ui;
}

test('the map selector offers the two hubs after the two maps, and the open one is marked', async () => {
  const ui = await openMap(MAP(ARRAKEEN));
  const buttons = [...ui.doc.querySelectorAll('#main [data-lmmap]')];
  assert.deepEqual(buttons.map((b) => [b.dataset.lmmap, text(b), b.classList.contains('primary')]),
    [['HaggaBasin', 'Hagga Basin', false], ['DeepDesert', 'The Deep Desert', false], ['Arrakeen', 'Arrakeen', true], ['HarkoVillage', 'Harko Village', false]]);
  assert.ok(ui.doc.querySelector('#lmc').getAttribute('aria-label').startsWith('Saved map of Arrakeen.'));
});

test('a click on a hub in the selector asks for that hub', async () => {
  const ui = await openMap(MAP(HAGGA));
  ui.fetchedUrls.length = 0;
  ui.click('[data-lmmap="HarkoVillage"]'); await ui.settle();
  assert.ok(ui.fetchedUrls.includes('/api/livemap?map=HarkoVillage'), ui.fetchedUrls.join(' '));
});

test('a hub has its picture and says nothing; a map with no picture says so and draws its outline', async () => {
  for (const cfg of [ARRAKEEN, HARKO]) {
    const hub = await openMap(MAP(cfg));
    assert.equal(hub.doc.querySelector('#lmnopic'), null, cfg.key);
  }
  // the page code still copes with a map the editor holds no picture of (a map added later without one)
  const bare = await openMap(MAP({ ...HARKO, image: '' }));
  assert.equal(text(bare.doc.querySelector('#lmnopic')), 'The editor has no picture of Harko Village yet, so its markers are drawn on the outline of the map.');
});

// The five House representatives of the hubs, the table's positions (internal/ops/landsraad_fixed_places.go) and the pixel of the map space each is at (printed by internal/ops
// TestHubFramesAreNotStretchedAndHoldTheirHouses): the page uses the same formula as the other maps, x across and y down, no flip.
test('a world position is the same place on a hub as the server worked out for it', async () => {
  const ui = await openMap(MAP(ARRAKEEN));
  const at = (cfg, x, y) => { const p = ui.w.liveMapPixel(cfg, x, y); return [Math.round(p.px * 10) / 10, Math.round(p.py * 10) / 10]; };
  assert.deepEqual(at(ARRAKEEN, 3622, -3222), [2853.2, 886.0], 'Mutelli');
  assert.deepEqual(at(ARRAKEEN, -8708, -7988), [1711.4, 444.7], 'Varota');
  assert.deepEqual(at(ARRAKEEN, -27134, 2864), [5.2, 1449.6], 'Wallach, 56 cm inside the west edge');
  assert.deepEqual(at(HARKO, 9978, 11503), [2393.0, 2323.3], 'Alexin');
  assert.deepEqual(at(HARKO, 3216, 9403), [1929.0, 2179.3], 'Spinette');
  // the corners of the frame are the corners of the map space; north is not flipped (a larger y is lower down)
  assert.deepEqual(at(ARRAKEEN, ARRAKEEN.minX, ARRAKEEN.minY), [0, 0]);
  const far = ui.w.liveMapPixel(ARRAKEEN, ARRAKEEN.maxX, ARRAKEEN.maxY);
  assert.ok(Math.abs(far.px - 4096) < 1e-6 && Math.abs(far.py - 2342) < 1e-6, JSON.stringify(far));
});

test('the representatives of a hub are listed among the markers of its map', async () => {
  const ui = await openMap(MAP(ARRAKEEN, { markers: [{ t: 'HouseRepresentativeMutelli', x: 3622, y: -3222, d: 0 }, { t: 'HouseRepresentativeVarota', x: -8708, y: -7988, d: 3 }] }));
  const rows = [...ui.doc.querySelectorAll('#main table tbody tr')].map((r) => text(r));
  assert.ok(rows.some((r) => r.includes('House Representative Mutelli')), rows.join(' | '));
  assert.ok(rows.some((r) => r.includes('House Representative Varota')), rows.join(' | '));
});

// The Landsraad board: a House at a hub has the map link now; the click lands on that hub's map centred on the House.
const TH = [1200, 2400, 3600, 4800, 6000];
const OWN = ['SolarisCoin', 'T6ArmorPlating', 'T6RangeFinder', 'T6HoltzmanActuator', 'T6DiamodineBladeParts'];
const house = (id, name, location) => ({
  id, board_index: id - 900, house_name: 'DA_House' + name, goal_amount: 4000, completed: 0, winning_faction_id: null, completion_time: null, contributions: [], player_contributions: [],
  my_contribution: 0, my_recorded: false, swatch_held: false, location,
  rewards: TH.map((threshold, i) => ({ threshold, amount: 1, template_id: OWN[i], reached: false, delivered: false, shown_template_id: OWN[i], shown_amount: 1, swatch: false, reward_state: 'not_reached' })),
});
const board = (tasks) => ({
  term: { term_id: 7, start_time: 100, end_time: 200, reigning_faction_id: 1, active_decree_id: 1, elected_decree_id: 1 }, terms: [{ term_id: 7, start_time: 100, end_time: 200 }], earlier_terms: [],
  factions: [{ id: 1, name: 'Atreides' }, { id: 2, name: 'Harkonnen' }], character: { player_id: 1, name: 'Tester', faction_id: 1 }, decrees: [{ id: 1, decree_name: 'Decree A', disabled: 0 }], rotation: [], tasks,
});

test('the link of a House at a hub opens the Saved map on that hub, centred on the House', async () => {
  // the fractions are the ones the server gives for Mutelli and Alexin (landsraad_locations.go)
  const mutelli = { known: true, map: 'Arrakeen', map_key: 'Arrakeen', area: 'Arrakeen', place: '', cell: '', unconfirmed: false, x: 0.6965799039003683, y: 0.37830772138395463 };
  const alexin = { known: true, map: 'Harko Village', map_key: 'HarkoVillage', area: 'Harko Village', place: '', cell: '', unconfirmed: false, x: 0.5842211055276382, y: 0.5672194304857622 };
  const ui = await boot({ get: { '/api/landsraad': board([house(900, 'Alexin', alexin), house(901, 'Mutelli', mutelli)]), '/api/livemap': MAP(ARRAKEEN) } });
  ui.click('[data-tab="landsraad"]'); await ui.settle();
  const btn = ui.doc.querySelector('#lsrow-901 .loc-t');
  assert.ok(!btn.classList.contains('off') && btn.getAttribute('aria-disabled') !== 'true', 'the place is a link, not muted');
  assert.equal(ui.doc.querySelector('#lsrow-901 .loc-t .cue'), null, 'it no longer says "no map picture"');
  const calls = []; const real = ui.w.liveMapShow; ui.w.liveMapShow = (...a) => { calls.push(a); real(...a); };
  ui.fetchedUrls.length = 0;
  ui.doc.querySelector('#lsrow-901 [data-lb=map]').dispatchEvent(new ui.w.MouseEvent('click', { bubbles: true, cancelable: true }));
  await ui.settle();
  assert.deepEqual(calls, [['Arrakeen', mutelli.x, mutelli.y, 'Mutelli', 901]]);
  assert.ok(ui.fetchedUrls.includes('/api/livemap?map=Arrakeen'), ui.fetchedUrls.join(' '));
  assert.equal(ui.doc.querySelector('[data-tab=livemap]').getAttribute('aria-current'), 'page');
  assert.match(ui.doc.querySelector('#lmc').getAttribute('aria-label'), /^Saved map of Arrakeen, centred on House Mutelli\./);
  const v = ui.w.liveMapState().view;
  assert.ok(v && Math.abs(v.cx - 2853.2) < 0.1 && Math.abs(v.cy - 886.0) < 0.1 && v.z === 4, JSON.stringify(v));
});
