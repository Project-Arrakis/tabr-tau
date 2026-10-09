// Renders every tab and sub-view of the real UI code in a browser engine (jsdom), fed the REAL API responses produced
// over a save whose every TEXT column holds attack strings (internal/web/fixtures_test.go). Fails if any of it becomes
// markup, an attribute, an event handler, or a new element. Run: UI_FIXTURES_DIR=... go test ./internal/web -run
// TestDumpUIFixtures, then `npm test` here.
import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { JSDOM } from 'jsdom';

const here = path.dirname(fileURLToPath(import.meta.url));
const static_ = path.join(here, '..', 'internal', 'web', 'static');
const FIX = process.env.UI_FIXTURES_FILE || path.join(process.env.UI_FIXTURES_DIR || path.join(here, 'fixtures'), 'ui-fixtures.json');
const MARK = 'onerror=window.__pwned=1'; // the full attack string (used for values the server passes through untouched)
const SEEN = '__pwned=1'; // survives server-side display-name truncation (class names are cut after the last '.')

if (!fs.existsSync(FIX)) {
  throw new Error(`UI fixtures not found at ${FIX}. Generate them first:\n  UI_FIXTURES_DIR=${path.dirname(FIX)} go test ./internal/web -run TestDumpUIFixtures`);
}
const baseFixtures = JSON.parse(fs.readFileSync(FIX, 'utf8'));
const ATTACK = `"><img src=x ${'onerror=window.__pwned=1'}>`;
// Type confusion: the real schema is STRICT, so a numeric column cannot hold text today. A game patch, a non-strict
// table or a hand-edited database could change that, so the UI must be safe even when every number is an attack string.
const confuse = (v) => (typeof v === 'number' ? ATTACK : Array.isArray(v) ? v.map(confuse) : v && typeof v === 'object' ? Object.fromEntries(Object.entries(v).map(([k, x]) => [k, confuse(x)])) : v);
const confusedFixtures = Object.fromEntries(Object.entries(baseFixtures).map(([k, f]) => [k, { status: f.status, body: confuse(f.body) }]));
let fixtures = baseFixtures;
const read = (n) => fs.readFileSync(path.join(static_, n), 'utf8');

// Allowlists: anything else appearing in the rendered DOM is an injection.
const TAGS = new Set('html head body meta title link script header nav main div span b a p h1 h3 ul li code table thead tbody tr th td input select option datalist button details summary textarea br label canvas'.split(' '));
const SCRIPTS = new Set(['/html.js', '/livemap.js', '/app.js']); // the only scripts the page may contain, both same-origin files
const ATTRS = new Set('lang charset name content rel href class id type value min max step placeholder size list selected disabled rows spellcheck colspan title src download role width height aria-modal tabindex label checked'.split(' '));

export function assertClean(doc, label) {
  const bad = [];
  for (const el of doc.querySelectorAll('*')) {
    const tag = el.tagName.toLowerCase();
    if (!TAGS.has(tag)) bad.push(`unexpected <${tag}>`);
    if (tag === 'script' && !SCRIPTS.has(el.getAttribute('src'))) bad.push(`<script> that is not one of the two bundled files: ${el.getAttribute('src') ?? 'inline'}`);
    for (const a of el.attributes) {
      const n = a.name.toLowerCase();
      if (n.startsWith('on')) bad.push(`event handler attribute ${n}`);
      if (!(ATTRS.has(n) || n.startsWith('data-'))) bad.push(`unexpected attribute ${n} on <${tag}>`);
      if (/[<>"'=\s]/.test(n)) bad.push(`malformed attribute name ${JSON.stringify(n)}`);
    }
  }
  if (doc.defaultView.__pwned !== undefined) bad.push('window.__pwned was set');
  if (bad.length) assert.fail(`${label}: ${[...new Set(bad)].slice(0, 8).join('; ')}`);
}

function haystack(doc) {
  let h = doc.body.textContent;
  for (const el of doc.querySelectorAll('*')) for (const a of el.attributes) h += ' ' + a.value;
  for (const el of doc.querySelectorAll('input,textarea,option')) h += ' ' + (el.value ?? '');
  return h;
}

async function boot({ post = {}, get = {}, confused = false } = {}) {
  const dom = new JSDOM(read('index.html'), { url: 'http://127.0.0.1:8090/', runScripts: 'dangerously', pretendToBeVisual: true });
  const w = dom.window;
  let inflight = 0;
  const posted = [];
  w.fetch = async (p, opt = {}) => {
    inflight++;
    try {
      await Promise.resolve();
      const u = new URL(p, 'http://127.0.0.1:8090');
      if ((opt.method || 'GET') === 'POST') {
        const b = post[u.pathname] ?? { ok: true };
        posted.push({ path: u.pathname, body: opt.body ? JSON.parse(opt.body) : null });
        if (b.__fail) return { ok: false, status: 400, statusText: 'Bad Request', json: async () => ({ error: b.__fail, code: b.__code }) };
        return { ok: true, status: 200, statusText: 'OK', json: async () => b };
      }
      if (get[u.pathname]) return { ok: true, status: 200, statusText: 'OK', json: async () => get[u.pathname] };
      const f = (confused ? confusedFixtures : baseFixtures)[u.pathname];
      const status = f ? f.status : 404;
      return { ok: status < 400, status, statusText: f ? 'OK' : 'Not Found', json: async () => (f ? f.body : { error: 'not in fixtures: ' + u.pathname }), blob: async () => new w.Blob(['x']) };
    } finally { inflight--; }
  };
  w.confirm = () => true; w.prompt = () => '1';
  w.eval(read('html.js'));
  w.eval(read('livemap.js'));
  w.eval(read('app.js'));
  const settle = async () => { for (let i = 0; i < 40; i++) { await new Promise((r) => w.setTimeout(r, 0)); if (inflight === 0 && i > 4) return; } };
  await settle();
  return { dom, w, doc: w.document, settle, posted, click: (sel) => { const el = w.document.querySelector(sel); assert.ok(el, `no element for ${sel}`); el.dispatchEvent(new w.MouseEvent('click', { bubbles: true, cancelable: true })); } };
}

const BASE7 = [{ base_id: 7, name: 'Base', base_type: 'Advanced Sub-Fief', owner: 'Owner', map: 'HaggaBasin', generators: 0, pieces: 0, placeables: 0, x: 0, y: 0, z: 0 }];
const VIEWS = [
  ['player', 'char:overview'], ['player', 'char:inventory'], ['player', 'char:reputation'], ['player', 'player:specialization'], ['player', 'player:journey'],
  ['player', 'player:buildingsets'], ['player', 'player:admin'],
  ['player', 'player:crafting'], ['player', 'player:research'], ['player', 'player:customizations'], ['player', 'player:skills'], ['player', 'player:blueprints'],
  ['player', 'player:bases'], ['player', 'player:bases', '[data-bstab="water"]'], ['player', 'player:bases', '[data-bstab="inventory"]'], ['player', 'player:bases', '[data-bstab="claim"]'],
  ['player', 'player:vehicles'], ['livemap'], ['landsraad'], ['config'], ['extras'],
  ['db', 'db:browse'], ['db', 'db:sql'], ['db', 'db:backups'],
];
// tabs that only say "not in tabr-tau yet", or whose rows are all numbers (specialization tracks), have no text to show
const NO_DATA = new Set(['player:admin', 'player:specialization', 'player:crafting', 'player:research', 'player:customizations', 'player:skills', 'player:blueprints', 'player:vehicles', 'db:sql', 'db:backups']);

for (const confused of [false, true]) for (const [tabName, ...subs] of VIEWS) {
  const label = [tabName, ...subs].join(' > ');
  test(`${confused ? 'type-confused numbers' : 'hostile data'} stay data: ${label}`, async () => {
    const ui = await boot({ confused });
    ui.click(`[data-tab="${tabName}"]`);
    await ui.settle();
    for (const x of subs) { ui.click(x.startsWith('[') ? x : `[data-sub="${x}"]`); await ui.settle(); }
    assertClean(ui.doc, label);
    const main = ui.doc.querySelector('#main').textContent;
    if (!confused) assert.ok(!main.includes('TypeError') && !main.includes('is not a function') && !main.includes('Cannot read'), `UI threw while rendering: ${main.slice(0, 200)}`);
    if (!confused && !NO_DATA.has(subs.at(-1))) {
      assert.ok(haystack(ui.doc).includes(SEEN), `${label}: the hostile text should be visible as plain data, but it is not in the rendered page at all (fixture or view not exercised)`);
    }
    ui.dom.window.close();
  });
}

test('the Player Summary shows what the save holds, above the Player tabs', async () => {
  const summary = {
    name: 'Tester', status: 'Offline', map: 'HaggaBasin', faction: 'Harkonnen', guild: null, level: 71, xp: 41221,
    skillPoints: { unspent: 9, total: 70 }, intel: { points: 333, max: 2779 },
    vitals: { health: 112.5, healthMinMax: 150, hydration: 52.5, maxHydration: 100, spiceAddiction: 0, maxSpiceAddiction: 10 },
    identity: { platform: 'Steam', platformId: 'P1', funcomId: 'F1', flsId: 'L1' }, ids: { actor: 2, account: 1, controller: 1, playerState: 3 },
    currency: [{ label: 'Solari Credit', balance: 591227 }, { label: 'Solari Coin', balance: 95470 }, { label: 'House Credit', balance: 0 }],
  };
  const ui = await boot({ get: { '/api/player/summary': summary } });
  ui.click('[data-tab="player"]'); await ui.settle();
  const text = ui.doc.querySelector('#main').textContent;
  for (const want of ['Player Summary', 'Harkonnen', '71', '41,221', '9 / 70', '333 / 2,779', '112.5 (max at least 150)', '52.5 / 100', 'Steam ID', 'Solari Credit', '591,227', 'Solari Coin', '95,470', 'House Credit']) {
    assert.ok(text.includes(want), `the summary should show ${want}`);
  }
  assert.ok(ui.doc.querySelector('[data-sub="player:bases"]'), 'the Player tabs are still there');
  ui.dom.window.close();
});

test('Player > Vehicles shows type, condition, fuel, components and the cargo hold of the open vehicle', async () => {
  const vehicle = (id, name, extra) => ({ id, name, type: name.replace('BP_', '').split('_')[0], map: 'HaggaBasin', x: 5228, y: -408425, z: 0, condition: 40, fuel: 90.5, modules: [], inventories: [], components: [], cargo: null, ...extra });
  const v1 = vehicle(152, 'BP_Sandbike_CHOAM', {
    components: [{ template_id: 'SandbikeEngine_1', current: 900, max: 1000, percent: 90 }, { template_id: 'SandbikeChassis_1', current: 2000, max: null, percent: null }],
    cargo: { inventory_id: 59, slots: 15, items: [{ id: 1, template_id: 'CopperOre', stack_size: 40, quality_level: 0, position_index: 2 }] } });
  const v2 = vehicle(153, 'BP_Buggy_CHOAM', { condition: 85, fuel: null, cargo: { inventory_id: 60, slots: 20, items: [] } });
  const ui = await boot({ get: { '/api/vehicles': { vehicles: [v1, v2], recovered: [], backups: [], hidden: 12 } } });
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="player:vehicles"]'); await ui.settle();
  const main = () => ui.doc.querySelector('#main').textContent;
  for (const want of ['Your vehicles (2)', 'Sandbike', 'Buggy', '40%', '85%', '90.5', '5,228, -408,425', 'Sandbike: 2 components', 'Sandbike Engine', '90%', '900 / 1000', 'Sandbike Chassis', '2000', 'Cargo hold', '1 / 15 slots used', 'CopperOre', '12 other vehicles']) {
    assert.ok(main().includes(want), `the vehicles tab should show ${want}`);
  }
  ui.click('[data-vhopen="153"]'); await ui.settle();
  assert.ok(main().includes('Buggy: 0 components') && main().includes('0 / 20 slots used') && main().includes('The cargo hold is empty.') && !main().includes('CopperOre'), 'the second vehicle has its own details');
  assert.ok(ui.doc.querySelector('[data-act="bring"]'), 'Bring to me is still offered');
  ui.dom.window.close();
});

test('Player > Bases lists the bases and shows Power, Water and Inventory for the open one', async () => {
  const ui = await boot({ get: {
    '/api/bases/list': [{ base_id: 164, name: 'Advanced Sub-Fief Console', base_type: 'Advanced Sub-Fief', owner: 'Tester', map: 'HaggaBasin', generators: 11, pieces: 486, placeables: 90, x: -67159, y: -211307, z: 12 }],
    '/api/bases/power': { types: [{ kind: 'generator_placeable', name: 'Generator', fuel: 'Oil', devices: 5, queued: 2484, capacity: 2500, empty: 0, refillable: true }, { kind: 'windtrap_placeable', name: 'Windtrap', fuel: '', devices: 6, queued: 0, capacity: 0, empty: 6, refillable: false }] },
    '/api/bases/water': { types: [{ kind: 'watercistern_placeable', name: 'Water Cistern', devices: 16, stored: 80000, capacity: 80000, fillPercent: 100, known: true }] },
    '/api/bases/inventory': { groups: [{ id: 'storage', label: 'Storage', count: 1 }, { id: 'refining', label: 'Refining', count: 1 }, { id: 'crafting', label: 'Crafting', count: 0 }, { id: 'other', label: 'Other', count: 0 }],
      containers: [{ inventory_id: 1, actor_id: 9, base_id: 164, group: 'storage', type: 'Chest', name: 'Ore Out', items: 3, max_item_count: 20 }, { inventory_id: 2, actor_id: 10, base_id: 164, group: 'refining', type: 'Small Ore Refinery', name: 'Small Ore Refinery', items: 1, max_item_count: 5 }] } } });
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="player:bases"]'); await ui.settle();
  const main = () => ui.doc.querySelector('#main').textContent;
  for (const want of ['Bases (1)', 'Advanced Sub-Fief Console', 'Advanced Sub-Fief', 'Tester', 'Building Pieces', '486', '-67,159, -211,307, 12', 'Generator', '2,484', '2,500', 'Windtrap', 'Refill generators']) assert.ok(main().includes(want), `the Power tab should show ${want}`);
  assert.deepEqual([...ui.doc.querySelectorAll('[data-bstab]')].map((b) => b.textContent.trim()), ['Power', 'Water', 'Inventory', 'Land Claim Editor']);
  assert.ok(!main().includes('Permissions'), 'the permission tabs are not mirrored');
  ui.click('[data-bstab="water"]'); await ui.settle();
  for (const want of ['Water Cistern', '16', '80,000', '100%', 'Refill base water']) assert.ok(main().includes(want), `the Water tab should show ${want}`);
  ui.click('[data-bstab="inventory"]'); await ui.settle();
  assert.deepEqual([...ui.doc.querySelectorAll('[data-bsgrp]')].map((b) => b.textContent.trim()), ['Storage (1)', 'Refining (1)', 'Crafting (0)', 'Other (0)']);
  assert.ok(main().includes('Ore Out') && main().includes('3 / 20') && !main().includes('Small Ore Refinery'), 'the first group with containers is shown');
  ui.click('[data-bsgrp="refining"]'); await ui.settle();
  assert.ok(main().includes('Small Ore Refinery') && main().includes('1 / 5') && !main().includes('Ore Out'), 'another group');
  ui.dom.window.close();
});

test('Character > Inventory shows the console groups, a filter, slot usage and augments', async () => {
  const it = (id, tmpl, type, slot, extra = {}) => ({ id, template_id: tmpl, stack_size: 1, quality_level: 0, position_index: slot, inventory_id: type, inventory_type: type, inventory_name: 'x', durability: null, max_durability: null, augments: [], ...extra });
  const inventory = {
    items: [it(1, 'CopperOre', 0, 0), it(2, 'Literjon_T1', 0, 1), it(3, 'Combat_Heavy_Helmet_06', 1, 0, { augments: [{ name: 'T6_Augment_Armor16', quality: 5 }] }), it(4, 'Knife', 15, 0)],
    inventories: [{ id: 1, inventory_type: 0, max_item_count: 35 }, { id: 2, inventory_type: 1, max_item_count: 10 }, { id: 3, inventory_type: 15, max_item_count: 8 }],
    templates: ['CopperOre', 'Literjon_T1', 'Combat_Heavy_Helmet_06', 'Knife'],
  };
  const ui = await boot({ get: { '/api/player/inventory': inventory } });
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="char:inventory"]'); await ui.settle();
  const names = () => [...ui.doc.querySelectorAll('[data-invg]')].map((b) => b.textContent.trim());
  assert.deepEqual(names(), ['Backpack (2)', 'Character (1)', 'Loadout (1)', 'Unique schematics (0)']);
  const main = () => ui.doc.querySelector('#main').textContent;
  assert.ok(main().includes('2 of 2 - 2 / 35 slots used') && main().includes('CopperOre') && !main().includes('Knife'), 'the backpack is the first group with items');
  ui.click('[data-invg="character"]'); await ui.settle();
  assert.ok(ui.doc.querySelector('[title^="T6_Augment_Armor16"]') && main().includes('Combat_Heavy_Helmet_06') && !main().includes('CopperOre'), 'the character group lists the helmet with its augment');
  ui.click('[data-invg="backpack"]'); await ui.settle();
  ui.doc.querySelector('#invq').value = 'liter';
  ui.click('[data-act="invfilter"]'); await ui.settle();
  assert.ok(main().includes('1 of 2') && main().includes('Literjon_T1') && !main().includes('CopperOre'), 'the filter narrows the group');
  ui.click('[data-act="invclear"]'); await ui.settle();
  assert.ok(main().includes('2 of 2'), 'Clear shows the group again');
  ui.dom.window.close();
});

test('Give XP and Give currency post their amounts and report the result', async () => {
  const ui = await boot({ post: {
    '/api/player/xp': { ok: true, before: 41221, after: 42221, applied: 1000, levelBefore: 71, levelAfter: 72, skillPointsGained: 1, capped: false },
    '/api/player/currency': { ok: true, currency: 1, before: 0, after: 250 } } });
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.doc.querySelector('#xpAmt').value = '1000';
  ui.click('[data-act="xp"]'); await ui.settle();
  let toast = ui.doc.querySelector('#toast').textContent;
  assert.ok(toast.includes('41,221') && toast.includes('42,221') && toast.includes('level 71 to 72') && toast.includes('+1 skill points'), toast);
  ui.doc.querySelector('#curSel').value = '1'; ui.doc.querySelector('#curAmt').value = '250';
  ui.click('[data-act="currency"]'); await ui.settle();
  toast = ui.doc.querySelector('#toast').textContent;
  assert.ok(toast.includes('House Credit') && toast.includes('250'), toast);
  assert.deepEqual(ui.posted.filter((x) => x.path === '/api/player/xp').map((x) => x.body), [{ amount: 1000 }]);
  assert.deepEqual(ui.posted.filter((x) => x.path === '/api/player/currency').map((x) => x.body), [{ currency: 1, amount: 250 }]);
  ui.dom.window.close();
});

test('Give Intel posts the amount and reports what was applied, including the cap', async () => {
  const ui = await boot({ post: { '/api/player/intel': { ok: true, before: 2750, after: 2779, applied: 29, capped: true } } });
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.doc.querySelector('#intelAmt').value = '100';
  ui.click('[data-act="intel"]'); await ui.settle();
  assert.deepEqual(ui.posted.filter((x) => x.path === '/api/player/intel').map((x) => x.body), [{ amount: 100 }]);
  const toast = ui.doc.querySelector('#toast').textContent;
  assert.ok(toast.includes('2,750') && toast.includes('2,779') && toast.includes('capped'), toast);
  ui.dom.window.close();
});

test('Admin mirrors the console: the actions tabr-tau has post, the ones it lacks are disabled, server-only ones are absent', async () => {
  const ui = await boot({});
  ui.click('[data-tab="player"]'); await ui.settle();
  assert.equal(ui.doc.querySelector('[data-act="teleport"]'), null, 'Teleport is on Admin, not on Character > Overview');
  assert.equal(ui.doc.querySelector('[data-act="repair"]'), null, 'Repair gear is on Admin, not on the inventory');
  ui.click('[data-sub="player:admin"]'); await ui.settle();
  for (const act of ['teleport', 'repair', 'repairV', 'faction']) assert.ok(ui.doc.querySelector(`[data-act="${act}"]`), `Admin has ${act}`);
  const text = ui.doc.querySelector('#main').textContent;
  for (const lacks of ['Repair Faction', 'Repair Quests', 'Wipe Inventory', 'Reset Progression', 'Spawn']) {
    const b = [...ui.doc.querySelectorAll('button')].find((x) => x.textContent.trim() == lacks);
    assert.ok(b && b.disabled, `${lacks} is shown disabled`);
  }
  for (const serverOnly of ['Kick Player', 'Ban Player', 'Repair Login Queue', 'Recover Character']) assert.ok(!text.includes(serverOnly), `${serverOnly} does not apply to single-player`);
  ui.click('[data-act="repair"]'); await ui.settle();
  ui.click('[data-act="teleport"]'); await ui.settle();
  for (const path of ['/api/player/repair', '/api/player/teleport']) assert.equal(ui.posted.filter((x) => x.path === path).length, 1, path);
  ui.dom.window.close();
});

test('interactions that render results also stay clean (storage inventory, journey filter, SQL result)', async () => {
  const hostile = `"><img src=x ${MARK}>`;
  const ui = await boot({ post: { '/api/db/sql': { columns: [hostile], rows: [[hostile], [hostile + "'"]] }, '/api/db/exec': { changes: 3 } } });
  ui.click('[data-tab=\"player\"]'); await ui.settle();
  ui.click('[data-sub=\"player:bases\"]'); await ui.settle();
  ui.click('[data-bstab="inventory"]'); await ui.settle();
  ui.click('[data-act="openInv"]'); await ui.settle();
  assertClean(ui.doc, 'open inventory');
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="player:journey"]'); await ui.settle();
  ui.click('[data-act="jfilter"]'); await ui.settle();
  assertClean(ui.doc, 'journey filter');
  ui.click('[data-tab="db"]'); await ui.settle();
  ui.click('[data-sub="db:sql"]'); await ui.settle();
  ui.click('[data-act="runsql"]'); await ui.settle();
  assertClean(ui.doc, 'sql result');
  assert.ok(ui.doc.querySelector('#sqlout').textContent.includes(MARK));
  ui.dom.window.close();
});

test('prototype-key and forged dataset values cannot reach handlers or navigation', async () => {
  const ui = await boot();
  const before = ui.doc.querySelector('#main').innerHTML;
  const b = ui.doc.createElement('button');
  b.dataset.act = '__proto__'; ui.doc.body.append(b); b.click(); await ui.settle();
  const c = ui.doc.createElement('button');
  c.dataset.act = 'constructor'; ui.doc.body.append(c); c.click(); await ui.settle();
  const d = ui.doc.createElement('button');
  d.dataset.tab = '__proto__'; ui.doc.body.append(d); d.click(); await ui.settle();
  const e = ui.doc.createElement('button');
  e.dataset.sub = '__proto__:x'; ui.doc.body.append(e); e.click(); await ui.settle();
  assert.equal(ui.doc.querySelector('#main').innerHTML, before, 'forged data-* values must not change the UI');
  ui.dom.window.close();
});

// NEGATIVE CONTROL: the detector must fail on a renderer that concatenates hostile data into innerHTML.
test('the detector catches a vulnerable renderer (proves this suite can fail)', () => {
  const dom = new JSDOM('<main id="m"></main>', { runScripts: 'dangerously', url: 'http://127.0.0.1/' });
  const hostile = `"><img src=x ${MARK}>`;
  dom.window.document.querySelector('#m').innerHTML = `<input value="${hostile}"><button data-act="${hostile}">x</button>`;
  assert.throws(() => assertClean(dom.window.document, 'vulnerable'), /unexpected <img>|unexpected attribute|malformed attribute/);
});

test('the detector also catches handler-attribute injection', () => {
  const dom = new JSDOM('<main id="m"></main>', { url: 'http://127.0.0.1/' });
  dom.window.document.querySelector('#m').innerHTML = `<div class="x" onclick="alert(1)">x</div>`;
  assert.throws(() => assertClean(dom.window.document, 'handler'), /event handler attribute onclick/);
});

// ---------- review pane (the only way to save)
const ATK = `"><img src=x ${MARK}>`;
const hostileReview = () => ({
  dirty: true, token: 'tok-1',
  ops: [{ desc: `edit ${ATK}`, rows: 1, at: '2026-10-07T00:00:00Z' }, { desc: 'second edit', rows: 2, at: '2026-10-07T00:00:01Z' }],
  diff: { redacted: true, tables: [{ name: `items${ATK}`, key: ['id'], numAdded: 1, numRemoved: 0, numModified: 1,
    modified: [{ key: [7, ATK], changes: [{ kind: 'value', path: `stack${ATK}`, before: ATK, after: { deep: ATK } }] }] }] },
});
async function openReviewUI(opts = {}) {
  const ui = await boot({ get: { '/api/save/review': opts.review || hostileReview() }, post: opts.post });
  ui.doc.querySelector('#btnSave').disabled = false; // the fixture save is clean; the button is enabled once there are edits
  ui.click('#btnSave'); await ui.settle();
  return ui;
}

test('the review pane shows hostile edits and values as plain data', async () => {
  const ui = await openReviewUI();
  assertClean(ui.doc, 'review pane');
  const m = ui.doc.querySelector('#modal');
  assert.ok(!m.classList.contains('hide'), 'the review must be visible');
  assert.ok(m.textContent.includes(MARK), 'the attack text should be visible as data');
  assert.ok(m.textContent.includes('second edit'));
  ui.dom.window.close();
});

test('Save is inside the review, sends the reviewed token, and closes the review', async () => {
  const ui = await openReviewUI({ post: { '/api/save/commit': { saved: true, backup: 'b.db' } } });
  ui.click('[data-act="doSave"]'); await ui.settle();
  const commit = ui.posted.filter((p) => p.path === '/api/save/commit');
  assert.equal(commit.length, 1);
  assert.deepEqual(commit[0].body, { reviewed: 'tok-1' });
  assert.ok(ui.doc.querySelector('#modal').classList.contains('hide'));
  assert.ok(ui.doc.querySelector('#toast').textContent.includes('Saved'));
  ui.dom.window.close();
});

test('a save error stays in the review with a way forward; a changed-on-disk save offers reload', async () => {
  const stale = 'the save changed on disk since it was loaded (game autosave?); discard and redo the edits';
  const ui = await openReviewUI({ post: { '/api/save/commit': { __fail: stale, __code: 'changed_on_disk' } } });
  ui.click('[data-act="doSave"]'); await ui.settle();
  const m = ui.doc.querySelector('#modal');
  assert.ok(!m.classList.contains('hide'), 'the review must stay open on an error');
  assert.ok(m.textContent.includes('The save changed on disk'));
  assert.ok(m.textContent.includes('second edit'), 'the edits to redo must still be listed');
  assert.equal(ui.doc.querySelector('#btnDoSave').disabled, true, 'saving is blocked until the file is reloaded');
  ui.click('[data-act="reload"]'); await ui.settle();
  assert.ok(!ui.doc.querySelector('#ask').classList.contains('hide'), 'reloading asks first');
  assert.ok(!ui.posted.some((p) => p.path === '/api/save/discard'), 'nothing is discarded before the answer');
  ui.click('[data-act="askOk"]'); await ui.settle();
  assert.ok(ui.posted.some((p) => p.path === '/api/save/discard'));
  assert.ok(ui.doc.querySelector('#modal').classList.contains('hide'));
  ui.dom.window.close();
});

test('a game check that cannot run blocks Save and shows why, as plain text', async () => {
  const why = '<img src=x onerror=1> tasklist missing';
  const ov = { ...JSON.parse(JSON.stringify(baseFixtures['/api/save/state'].body)), gameRunning: false, dirty: true, gameCheckError: why, saveBlocked: true, blockReason: 'could not check whether the game is running (' + why + ')', gameMode: 'unknown' };
  const ui = await boot({ get: { '/api/save/state': ov, '/api/save/review': hostileReview() } });
  const gw = ui.doc.querySelector('#gamewarn');
  assert.ok(gw.textContent.includes('Saving is blocked'));
  assert.ok(gw.textContent.includes('tasklist missing'));
  assert.equal(gw.querySelector('img'), null, 'the error text is data, not markup');
  ui.click('#btnSave'); await ui.settle();
  assert.equal(ui.doc.querySelector('#btnDoSave').disabled, true);
  ui.dom.window.close();
});

test('an active single-player session blocks Save inside the review and the banner says why', async () => {
  const ov = { ...JSON.parse(JSON.stringify(baseFixtures['/api/save/state'].body)), gameRunning: true, dirty: true, gameProcesses: ['DuneTest.exe'], saveBlocked: true, gameMode: 'single_player',
    blockReason: 'a single-player session is active (DuneTest.exe); go to the menu or multiplayer, or quit the game, then save' };
  const ui = await boot({ get: { '/api/save/state': ov, '/api/save/review': hostileReview() } });
  const gw = ui.doc.querySelector('#gamewarn');
  assert.ok(gw.classList.contains('warn') && !gw.classList.contains('hide'));
  assert.ok(gw.textContent.includes('single-player session is active') && gw.textContent.includes('DuneTest.exe'), 'the banner names the process it found');
  ui.click('#btnSave'); await ui.settle();
  assert.equal(ui.doc.querySelector('#btnDoSave').disabled, true);
  assert.ok(ui.doc.querySelector('#savewhy').textContent.includes('single-player session is active'));
  ui.dom.window.close();
});

test('the game open in multiplayer or at the menu shows a note and does not block Save', async () => {
  const ov = { ...JSON.parse(JSON.stringify(baseFixtures['/api/save/state'].body)), gameRunning: true, dirty: true, gameProcesses: ['DuneTest.exe'], saveBlocked: false, gameMode: 'menu_or_multiplayer', blockReason: 'DuneTest.exe is running but not in single-player; saving is allowed' };
  const ui = await boot({ get: { '/api/save/state': ov, '/api/save/review': hostileReview() } });
  const gw = ui.doc.querySelector('#gamewarn');
  assert.ok(gw.classList.contains('info') && !gw.classList.contains('warn'), gw.className);
  assert.ok(gw.textContent.includes('not in single-player') && gw.textContent.includes('DuneTest.exe'));
  ui.click('#btnSave'); await ui.settle();
  assert.equal(ui.doc.querySelector('#btnDoSave').disabled, false, 'Save must be possible when the game is not in single-player');
  ui.dom.window.close();
});

test('a stale review token refetches the review and says why, instead of looping', async () => {
  let n = 0;
  const reviews = [{ ...hostileReview(), token: 'old' }, { ...hostileReview(), token: 'new' }];
  const ui = await boot({ get: {}, post: { '/api/save/commit': { __fail: 'the pending changes are different from the ones you reviewed; review them again', __code: 'review_changed' } } });
  ui.w.fetch = ((orig) => async (p, o) => (new URL(p, 'http://x').pathname === '/api/save/review' ? { ok: true, status: 200, json: async () => reviews[Math.min(n++, 1)] } : orig(p, o)))(ui.w.fetch);
  ui.doc.querySelector('#btnSave').disabled = false;
  ui.click('#btnSave'); await ui.settle();
  ui.click('[data-act="doSave"]'); await ui.settle();
  const m = ui.doc.querySelector('#modal');
  assert.ok(m.textContent.includes('different from the ones you reviewed'));
  assert.equal(n, 2, 'the review must be fetched again');
  ui.dom.window.close();
});

test('the review pane is a labelled dialog, takes focus, and closes on Escape', async () => {
  const ui = await openReviewUI();
  assert.equal(ui.doc.querySelector('.mbox').getAttribute('role'), 'dialog');
  assert.equal(ui.doc.activeElement.id, 'mtitle');
  ui.doc.dispatchEvent(new ui.w.KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
  assert.ok(ui.doc.querySelector('#modal').classList.contains('hide'));
  assert.ok(!ui.doc.body.classList.contains('noscroll'));
  ui.dom.window.close();
});

test('a table with more changed rows than listed says the list is partial', async () => {
  const r = hostileReview(); r.diff.tables[0].numModified = 500;
  const ui = await openReviewUI({ review: r });
  assert.ok(ui.doc.querySelector('#modal').textContent.includes('counts above are complete'));
  ui.dom.window.close();
});

// ---------- confirmations
async function inventoryUI(post) {
  const ui = await boot({ post });
  ui.click('[data-tab=\"player\"]'); await ui.settle();
  ui.click('[data-sub=\"player:bases\"]'); await ui.settle();
  ui.click('[data-bstab="inventory"]'); await ui.settle();
  ui.click('[data-act="openInv"]'); await ui.settle();
  return ui;
}

test('deleting an item asks first, shows the item as data, and Cancel sends nothing', async () => {
  const ui = await inventoryUI();
  ui.click('[data-act="delItem"]'); await ui.settle();
  const a = ui.doc.querySelector('#ask');
  assert.ok(!a.classList.contains('hide'), 'a question must be shown');
  assertClean(ui.doc, 'delete confirmation');
  assert.equal(ui.posted.filter((p) => p.path === '/api/items/delete').length, 0, 'nothing is sent before the answer');
  ui.click('[data-act="askNo"]'); await ui.settle();
  assert.ok(a.classList.contains('hide'));
  assert.equal(ui.posted.filter((p) => p.path === '/api/items/delete').length, 0);
  ui.click('[data-act="delItem"]'); await ui.settle();
  ui.click('[data-act="askOk"]'); await ui.settle();
  assert.equal(ui.posted.filter((p) => p.path === '/api/items/delete').length, 1);
  ui.dom.window.close();
});

test('a far-reaching action needs the word typed, and Escape cancels', async () => {
  const ui = await boot();
  ui.click('[data-tab=\"player\"]'); await ui.settle();
  ui.click('[data-tab="extras"]'); await ui.settle();
  const btn = ui.doc.querySelector('[data-act="sand"]');
  assert.ok(btn, 'the clear-sand button must exist in the bases view');
  ui.click('[data-act="sand"]'); await ui.settle();
  const ok = ui.doc.querySelector('#aok');
  assert.equal(ok.disabled, true, 'OK is off until the word is typed');
  const inp = ui.doc.querySelector('#atype');
  inp.value = 'nope'; inp.dispatchEvent(new ui.w.Event('input', { bubbles: true }));
  assert.equal(ui.doc.querySelector('#aok').disabled, true);
  inp.value = ' Clear '; inp.dispatchEvent(new ui.w.Event('input', { bubbles: true }));
  assert.equal(ui.doc.querySelector('#aok').disabled, false, 'the right word (any case) enables it');
  ui.doc.dispatchEvent(new ui.w.KeyboardEvent('keydown', { key: 'Escape', bubbles: true })); await ui.settle();
  assert.ok(ui.doc.querySelector('#ask').classList.contains('hide'));
  assert.equal(ui.posted.filter((p) => p.path === '/api/bases/clear-sand').length, 0, 'Escape must not run the action');
  ui.dom.window.close();
});

test('a successful edit says it is not in the game yet', async () => {
  const ov = JSON.parse(JSON.stringify(baseFixtures['/api/save/state'].body)); ov.dirty = true;
  const ui = await boot({ get: { '/api/save/state': ov } });
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-act="solari"]'); await ui.settle();
  assert.ok(ui.doc.querySelector('#toast').textContent.includes('not saved yet'));
  ui.dom.window.close();
});

test('the question text is data even when the item name is an attack string', async () => {
  const ui = await boot({ get: { '/api/bases/storage/items?inventory=1': [{ id: 5, position_index: 0, template_id: `"><img src=x ${MARK}>`, stack_size: 1 }] } });
  ui.click('[data-tab=\"player\"]'); await ui.settle();
  ui.click('[data-sub=\"player:bases\"]'); await ui.settle();
  ui.click('[data-bstab="inventory"]'); await ui.settle();
  ui.click('[data-act="openInv"]'); await ui.settle();
  ui.click('[data-act="delItem"]'); await ui.settle();
  assertClean(ui.doc, 'hostile ask body');
  assert.ok(ui.doc.querySelector('#ask').textContent.includes(MARK));
  ui.dom.window.close();
});

test('typing the word completes the action; Enter in the box does too; the disabled button does nothing', async () => {
  const ui = await boot();
  ui.click('[data-tab=\"player\"]'); await ui.settle();
  ui.click('[data-tab="extras"]'); await ui.settle();
  ui.click('[data-act="sand"]'); await ui.settle();
  ui.click('#aok'); await ui.settle(); // disabled: a click must do nothing
  assert.equal(ui.posted.filter((p) => p.path === '/api/bases/clear-sand').length, 0);
  assert.ok(!ui.doc.querySelector('#ask').classList.contains('hide'));
  const inp = ui.doc.querySelector('#atype');
  inp.value = 'clear'; inp.dispatchEvent(new ui.w.Event('input', { bubbles: true }));
  inp.dispatchEvent(new ui.w.KeyboardEvent('keydown', { key: 'Enter', bubbles: true })); await ui.settle();
  assert.equal(ui.posted.filter((p) => p.path === '/api/bases/clear-sand').length, 1);
  assert.ok(ui.doc.querySelector('#ask').classList.contains('hide'));
  ui.dom.window.close();
});

test('Escape closes only the question when the review is open underneath, and focus returns', async () => {
  const ui = await openReviewUI();
  const trigger = ui.doc.querySelector('[data-act="reload"]') || ui.doc.querySelector('#btnDoSave');
  trigger.focus();
  ui.click('[data-act="closeReview"]'); await ui.settle();
  const ui2 = await openReviewUI({ post: { '/api/save/commit': { __fail: 'the save changed on disk since it was loaded (game autosave?); discard and redo the edits', __code: 'changed_on_disk' } } });
  ui2.click('[data-act="doSave"]'); await ui2.settle();
  const reload = ui2.doc.querySelector('[data-act="reload"]');
  reload.focus();
  ui2.click('[data-act="reload"]'); await ui2.settle();
  assert.ok(!ui2.doc.querySelector('#ask').classList.contains('hide'));
  ui2.doc.dispatchEvent(new ui2.w.KeyboardEvent('keydown', { key: 'Escape', bubbles: true })); await ui2.settle();
  assert.ok(ui2.doc.querySelector('#ask').classList.contains('hide'));
  assert.ok(!ui2.doc.querySelector('#modal').classList.contains('hide'), 'the review must stay open');
  assert.equal(ui2.doc.activeElement, reload, 'focus returns to the button that asked');
  ui.dom.window.close(); ui2.dom.window.close();
});

test('a second question replaces the first, which counts as cancelled', async () => {
  const ui = await boot();
  ui.click('[data-tab=\"player\"]'); await ui.settle();
  ui.click('[data-tab="extras"]'); await ui.settle();
  ui.click('[data-act="sand"]'); await ui.settle();
  ui.click('[data-act="repairB"]'); await ui.settle();
  assert.ok(ui.doc.querySelector('#ask').textContent.includes('Repair every base piece'));
  assert.equal(ui.posted.filter((p) => p.path === '/api/bases/clear-sand').length, 0);
  ui.dom.window.close();
});

test('no "not saved yet" hint when nothing is pending', async () => {
  const ov = JSON.parse(JSON.stringify(baseFixtures['/api/save/state'].body)); ov.dirty = false;
  const ui = await boot({ get: { '/api/save/state': ov } });
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-act="solari"]'); await ui.settle();
  const t = ui.doc.querySelector('#toast').textContent;
  assert.ok(t.includes('Solari updated') && !t.includes('not saved yet'), t);
  ui.dom.window.close();
});

test('a failed status refresh after a successful edit still shows the success message', async () => {
  const ui = await boot();
  ui.click('[data-tab="player"]'); await ui.settle();
  const f = ui.w.fetch;
  ui.w.fetch = async (p, o) => { if (new URL(p, 'http://x').pathname === '/api/save/state') throw new Error('network down'); return f(p, o); };
  ui.click('[data-act="solari"]'); await ui.settle();
  assert.ok(ui.doc.querySelector('#toast').textContent.includes('Solari updated'));
  ui.dom.window.close();
});

test('Refill containers reports what it filled and shows a container it could not size as plain text', async () => {
  const hostile = '<img src=x onerror=1> x1';
  const ui = await boot({ post: { '/api/player/refill': { ok: true, filled: 3, alreadyFull: 1, skippedUnknown: [hostile] } } });
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="char:inventory"]'); await ui.settle();
  ui.click('[data-act="refill"]'); await ui.settle();
  const toast = ui.doc.querySelector('#toast');
  assert.ok(toast.textContent.includes('Filled 3 containers (1 already full)'), toast.textContent);
  assert.ok(toast.textContent.includes(hostile), 'the name is shown as text');
  assert.equal(toast.querySelector('img'), null, 'and never becomes markup');
  assert.equal(ui.posted.filter((p) => p.path === '/api/player/refill').length, 1);
  ui.dom.window.close();
});

test('Expand claim asks first, posts the chosen size and level, and the totem name stays plain text', async () => {
  const claim = [{ totem_id: 7, name: '<img src=x onerror=1> Totem', cells: 1, rings: 0, level: 0, maxRings: 5, maxLevel: 5 }];
  const ui = await boot({ get: { '/api/bases/list': BASE7, '/api/bases/claim': claim }, post: { '/api/bases/claim/expand': { ok: true, added: 24, totalCells: 25, level: 2 } } });
  ui.click('[data-tab=\"player\"]'); await ui.settle();
  ui.click('[data-sub=\"player:bases\"]'); await ui.settle();
  ui.click('[data-bstab="claim"]'); await ui.settle();
  const card = [...ui.doc.querySelectorAll('.card')].find((c) => c.textContent.includes('Land claim size'));
  assert.ok(card, 'the land claim card is shown');
  assert.equal(card.querySelector('img'), null, 'the totem name is data, not markup');
  ui.doc.querySelector('#cr7').value = '2'; ui.doc.querySelector('#cv7').value = '2';
  ui.click('[data-act="claimGrow"]'); await ui.settle();
  assert.ok(ui.doc.querySelector('#ask').textContent.includes('5 x 5'), 'the question states the resulting size');
  assert.equal(ui.posted.filter((p) => p.path === '/api/bases/claim/expand').length, 0, 'nothing is sent before confirming');
  ui.click('[data-act="askOk"]'); await ui.settle();
  const sent = ui.posted.filter((p) => p.path === '/api/bases/claim/expand');
  assert.equal(sent.length, 1);
  assert.deepEqual([sent[0].body.totem_id, sent[0].body.rings, sent[0].body.level], [7, 2, 2]);
  assert.ok(ui.doc.querySelector('#toast').textContent.includes('Claim now 25 cells'));
  ui.dom.window.close();
});

test('item pickers list in-game names, send the template id, and item tables show names', async () => {
  const catalog = [
    { id: 'Literjon_T6', name: 'Literjon Mk6', category: 'consumables' },
    { id: 'Oil', name: 'Fuel Cell', category: 'consumables' },
    { id: 'DupA', name: 'Twin Name', category: 'x' }, { id: 'DupB', name: 'Twin Name', category: 'x' },
    { id: 'Crysknife', name: 'Unfixed Crysknife', category: 'weapons' }, { id: 'Crysknife_CR', name: 'Crysknife', category: 'weapons' },
  ];
  const inv = JSON.parse(JSON.stringify(baseFixtures['/api/player/inventory'].body));
  inv.templates = ['Literjon_T6', 'Emote_Unlisted'];
  inv.items = [{ id: 1, inventory_id: 1, inventory_name: 'Backpack', position_index: 0, template_id: 'Literjon_T6', stack_size: 1, quality_level: 0, durability: null, max_durability: null, stats: '{}' }];
  const ui = await boot({ get: { '/api/catalog/items': catalog, '/api/player/inventory': inv }, post: { '/api/player/give': { ok: true, itemId: 9 } } });
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="char:inventory"]'); await ui.settle();
  const opts = [...ui.doc.querySelectorAll('#gt-dl option')].map((o) => o.value);
  assert.ok(opts.includes('Literjon Mk6') && opts.includes('Fuel Cell'), 'in-game names are offered');
  assert.ok(!opts.includes('Literjon_T6') && !opts.includes('Oil'), 'template ids are not the labels');
  assert.ok(opts.includes('Twin Name (DupA)') && opts.includes('Twin Name (DupB)'), 'a shared name is told apart by its id');
  assert.ok(opts.includes('Emote_Unlisted'), 'an id the catalog does not know stays selectable');
  assert.equal(ui.doc.querySelector('#gt-dl option[value="Fuel Cell"]').getAttribute('label'), 'Oil', 'the template id is the option label, so typing the id finds the item');
  assert.ok(ui.doc.querySelector('table').textContent.includes('Literjon Mk6'), 'the item table shows the name');
  assert.equal(ui.doc.querySelector('span[title="Literjon_T6"]').textContent, 'Literjon Mk6', 'and keeps the id on hover');
  ui.doc.querySelector('#gt').value = 'fuel cell';
  ui.click('[data-act="give"]'); await ui.settle();
  const give = ui.posted.filter((p) => p.path === '/api/player/give');
  assert.equal(give.length, 1);
  assert.equal(give[0].body.template_id, 'Oil', 'the name is turned back into the template id');
  assert.ok(opts.includes('Crysknife (Crysknife_CR)') && !opts.includes('Crysknife'), 'a name that spells another item\'s id is shown with its id');
  ui.doc.querySelector('#gt').value = 'Crysknife';
  ui.click('[data-act="give"]'); await ui.settle();
  assert.equal(ui.posted.filter((p) => p.path === '/api/player/give')[1].body.template_id, 'Crysknife', 'an exact template id always wins over a name');
  ui.doc.querySelector('#gt').value = 'Crysknife (Crysknife_CR)';
  ui.click('[data-act="give"]'); await ui.settle();
  assert.equal(ui.posted.filter((p) => p.path === '/api/player/give')[2].body.template_id, 'Crysknife_CR', 'the shown label maps to its own id');
  ui.doc.querySelector('#gt').value = 'Emote_Unlisted';
  ui.click('[data-act="give"]'); await ui.settle();
  assert.equal(ui.posted.filter((p) => p.path === '/api/player/give')[3].body.template_id, 'Emote_Unlisted');
  ui.doc.querySelector('#gt').value = 'Some_Raw_Id';
  ui.click('[data-act="give"]'); await ui.settle();
  assert.equal(ui.posted.filter((p) => p.path === '/api/player/give')[4].body.template_id, 'Some_Raw_Id', 'a typed id is sent as is');
  ui.dom.window.close();
});

test('Give Items queue: items are queued, listed with hostile names as text, removed, and given in one post', async () => {
  const inv = JSON.parse(JSON.stringify(baseFixtures['/api/player/inventory'].body));
  const ui = await boot({ get: { '/api/player/inventory': inv }, post: { '/api/player/give-items': { ok: true, count: 2, itemIds: [1, 2] } } });
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="char:inventory"]'); await ui.settle();
  assert.ok(!ui.doc.querySelector('[data-act="queueGive"]'), 'no queue until something is added');
  ui.doc.querySelector('#gt').value = '<img src=x onerror=1>'; ui.doc.querySelector('#gq').value = '3'; ui.doc.querySelector('#gg').value = '2';
  ui.click('[data-act="queueAdd"]'); await ui.settle();
  ui.doc.querySelector('#gt').value = 'Spice'; ui.doc.querySelector('#gq').value = '0';
  ui.click('[data-act="queueAdd"]'); await ui.settle();
  assert.equal(ui.doc.querySelectorAll('[data-act="queueDel"]').length, 2);
  assert.equal(ui.doc.querySelector('img'), null, 'a hostile item name is text, not markup');
  ui.click('[data-act="queueDel"][data-id="1"]'); await ui.settle();
  ui.doc.querySelector('#gt').value = 'Spice'; ui.doc.querySelector('#gq').value = '5'; ui.doc.querySelector('#gg').value = '0';
  ui.click('[data-act="queueAdd"]'); await ui.settle();
  ui.click('[data-act="queueGive"]'); await ui.settle();
  const post = ui.posted.filter((p) => p.path === '/api/player/give-items');
  assert.equal(post.length, 1, 'one request for the whole queue');
  assert.deepEqual(post[0].body.items, [{ template_id: '<img src=x onerror=1>', quantity: 3, quality: 2 }, { template_id: 'Spice', quantity: 5, quality: 0 }]);
  assert.ok(!ui.doc.querySelector('[data-act="queueGive"]'), 'the queue is cleared after giving');
  ui.dom.window.close();
});

test('Augments: the editor lists the fitting augments as text, applies the picked ones, and Give carries augments', async () => {
  const hostile = '<img src=x onerror=1>';
  const fit = { template_id: 'Helm_1', kind: 'clothing', limit: 2, options: [
    { id: 'Aug_A', name: 'Plate ' + hostile, effects: ['Armor +5%', 'Weight -1%'] }, { id: 'Aug_B', name: 'Weave', effects: [] }], applied: [{ id: 'Aug_B', name: 'Weave', quality: '3' }] };
  const inv = JSON.parse(JSON.stringify(baseFixtures['/api/player/inventory'].body));
  inv.items = [{ id: 7, inventory_id: 1, inventory_name: 'Backpack', position_index: 0, template_id: 'Helm_1', stack_size: 1, quality_level: 5, durability: null, max_durability: null, augments: [{ name: 'Aug_B', quality: 3 }], aug_limit: 2 },
    { id: 8, inventory_id: 1, inventory_name: 'Backpack', position_index: 1, template_id: 'Ore_1', stack_size: 5, quality_level: 0, durability: null, max_durability: null, augments: [], aug_limit: 0 }];
  const ui = await boot({ get: { '/api/player/inventory': inv }, post: { '/api/items/augment-options': fit, '/api/items/augment': { ok: true, augments: 1, slotsUnlocked: 2 }, '/api/player/give-items': { ok: true, count: 1, itemIds: [9] } } });
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="char:inventory"]'); await ui.settle();
  assert.equal(ui.doc.querySelectorAll('[data-act="augEdit"]').length, 1, 'only an item that takes augments gets the button');
  ui.click('[data-act="augEdit"]'); await ui.settle();
  assert.equal(ui.doc.querySelector('img'), null, 'a hostile augment name is text, not markup');
  assert.equal(ui.doc.querySelectorAll('#au0 option').length, 3, 'none plus the two fitting augments');
  assert.equal(ui.doc.querySelector('#au0').value, 'Aug_B', 'the applied augment is preselected');
  assert.ok(ui.doc.querySelector('#au0 option[value="Aug_A"]').textContent.includes('Armor +5%; Weight -1%'), 'effects are shown');
  assert.ok(!ui.doc.querySelector('#au2'), 'only as many slots as the item holds');
  ui.doc.querySelector('#au1').value = 'Aug_A'; ui.doc.querySelector('#augG').value = '4';
  ui.click('[data-act="augApply"]'); await ui.settle();
  const ap = ui.posted.filter((p) => p.path === '/api/items/augment');
  assert.equal(ap.length, 1);
  assert.deepEqual(ap[0].body, { item_id: 7, augments: ['Aug_B', 'Aug_A'], grade: 4, unlock_slots: true });
  assert.ok(!ui.doc.querySelector('#au0'), 'the editor closes after applying');
  // Give: picking an item that takes augments shows the slots; the queue line carries them
  ui.doc.querySelector('#gt').value = 'Helm_1';
  ui.doc.querySelector('#gt').dispatchEvent(new ui.dom.window.Event('change', { bubbles: true })); await ui.settle();
  assert.ok(ui.doc.querySelector('#ga1'), 'augment slots appear for the picked item');
  ui.doc.querySelector('#ga0').value = 'Aug_A'; ui.doc.querySelector('#gag').value = '2';
  ui.click('[data-act="give"]'); await ui.settle();
  const gi = ui.posted.filter((p) => p.path === '/api/player/give-items');
  assert.equal(gi.length, 1);
  assert.deepEqual(gi[0].body.items, [{ template_id: 'Helm_1', quantity: 1, quality: 0, augments: ['Aug_A'], grade: 2 }]);
  ui.dom.window.close();
});

test('Admin: Faction Assignment posts the chosen faction, Repair Below posts its threshold, and a hostile faction name is text', async () => {
  const hostile = '<img src=x onerror=1>';
  const fac = { current: { id: 2, name: 'Harkonnen' }, options: [{ id: 3, name: 'Neutral' }, { id: 1, name: 'Atreides' }, { id: 2, name: hostile }] };
  const ui = await boot({ get: { '/api/player/faction': fac }, post: { '/api/player/faction': { ok: true, faction: 'Atreides' }, '/api/vehicles/repair': { ok: true, modules: 3, repaired: 1, withoutKnownMax: 0 } } });
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="player:admin"]'); await ui.settle();
  assert.equal(ui.doc.querySelector('img'), null, 'a hostile faction name is text, not markup');
  assert.equal(ui.doc.querySelector('#facSel').value, '2', 'the current faction is preselected');
  ui.doc.querySelector('#facSel').value = '1';
  ui.click('[data-act="faction"]'); await ui.settle();
  assert.deepEqual(ui.posted.filter((p) => p.path === '/api/player/faction').map((p) => p.body), [{ faction_id: 1 }]);
  assert.ok(ui.doc.querySelector('#toast').textContent.includes('Faction set to Atreides'));
  assert.equal(ui.doc.querySelector('#rvPct').value, '50', 'the console defaults to 50 %');
  ui.doc.querySelector('#rvPct').value = '75';
  ui.click('[data-act="repairV"]'); await ui.settle();
  assert.deepEqual(ui.posted.filter((p) => p.path === '/api/vehicles/repair').map((p) => p.body), [{ threshold: 75 }]);
  ui.dom.window.close();
});

test('Journey browser: groups, filter, indented names as text, and Complete / Reset post the right requests', async () => {
  const hostile = '<img src=x onerror=1>';
  const row = (o) => ({ id: 'X', name: 'X', category: 'Story', depth: 0, parent: '', status: 'Incomplete', complete: false, revealed: false, pendingReward: false, tags: 0, actionable: true, ...o });
  const browse = {
    story: [row({ id: 'DA_MQ_A', name: 'A New Beginning', status: 'Complete', complete: true }), row({ id: 'DA_MQ_A.Wake', name: 'Wake ' + hostile, depth: 1, parent: 'DA_MQ_A', status: 'Revealed', revealed: true, pendingReward: true, tags: 2 }), row({ id: 'DA_MQ_A.Run', name: 'Run', depth: 1, parent: 'DA_MQ_A' })],
    contract: [row({ id: 'DA_CT_X', name: 'A Contract', category: 'Contract', actionable: false, tags: 1 })],
    codex: [row({ id: 'DA_Dunipedia_K', name: 'Known Universe', category: 'Codex', status: 'Complete', complete: true })],
    tutorial: [row({ id: '4', name: 'Move', category: 'Tutorial', status: 'Started' })],
  };
  const ui = await boot({ get: { '/api/player/journey/browse': browse }, post: { '/api/player/journey': { ok: true, rows: 3 }, '/api/player/tutorials': { ok: true } } });
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="player:journey"]'); await ui.settle();
  assert.equal(ui.doc.querySelector('img'), null, 'a hostile node name is text, not markup');
  assert.equal(ui.doc.querySelectorAll('[data-jgrp]').length, 4);
  assert.equal(ui.doc.querySelector('[data-jgrp="story"]').textContent, 'Story (3)');
  assert.ok(ui.doc.querySelector('span.ind1[title="DA_MQ_A.Wake"]'), 'a child is indented by its depth and keeps its id on hover');
  assert.ok(ui.doc.querySelector('#jout').textContent.includes('reward pending'));
  assert.equal(ui.doc.querySelectorAll('[data-act="jset"]').length, 3);
  ui.doc.querySelector('#jq').value = 'run'; ui.click('[data-act="jfilter"]'); await ui.settle();
  assert.equal(ui.doc.querySelectorAll('#jout tbody tr').length, 1, 'the filter matches names');
  ui.click('[data-act="jset"]'); await ui.settle();
  assert.deepEqual(ui.posted.filter((p) => p.path === '/api/player/journey').map((p) => p.body), [{ node_id: 'DA_MQ_A.Run', complete: true }]);
  ui.click('[data-act="jclear"]'); await ui.settle();
  ui.click('[data-jgrp="contract"]'); await ui.settle();
  assert.equal(ui.doc.querySelectorAll('[data-act="jset"]').length, 0, 'contracts are read-only');
  ui.click('[data-jgrp="tutorial"]'); await ui.settle();
  ui.click('[data-act="tut"]'); await ui.settle();
  assert.deepEqual(ui.posted.filter((p) => p.path === '/api/player/tutorials').map((p) => p.body), [{ id: 4, complete: true }]);
  ui.dom.window.close();
});

test('Skills: schools, rank bars, set rank (with and without paying), starter skills and unspent points', async () => {
  const hostile = '<img src=x onerror=1>';
  const m = (o) => ({ id: 'Skills.Ability.X', name: 'X', school: 'Trooper', kind: 'Ability', rank: 0, max: 3, spent: 0, ladder: [1, 3, 6], known: true, ...o });
  const sk = { total: 106, unspent: 45, spent: 14, schools: [{ key: 'Trooper', label: 'Trooper' }, { key: 'Mentat', label: 'Mentat' }], modules: [
    m({ id: 'Skills.Ability.Sprint', name: 'Sprint ' + hostile, rank: 2, spent: 3 }), m({ id: 'Skills.Perk.Aim', name: 'Aim', kind: 'Perk', max: 1, ladder: [2] }),
    m({ id: 'Skills.Ability.Mind', name: 'Mind', school: 'Mentat' }), m({ id: 'Skills.Attribute.Odd', name: 'Odd', school: 'Other', known: false, rank: 1, spent: 9, max: 0, ladder: [] })] };
  const ui = await boot({ get: { '/api/player/skills': sk }, post: { '/api/player/skills/module': { ok: true, pointsBefore: 3, pointsAfter: 6 }, '/api/player/skills/points': { ok: true, before: 45, after: 90 }, '/api/player/skills/starter': { ok: true, changed: 2 } } });
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="player:skills"]'); await ui.settle();
  assert.equal(ui.doc.querySelector('img'), null, 'a hostile skill name is text, not markup');
  assert.ok(ui.doc.querySelector('#main').textContent.includes('Earned in total') && ui.doc.querySelector('#main').textContent.includes('106'));
  assert.equal(ui.doc.querySelector('[data-skgrp="Trooper"]').textContent, 'Trooper (1)', 'the school chip counts learned skills');
  assert.equal(ui.doc.querySelectorAll('select[data-skill]').length, 2, 'only the chosen school skills are listed');
  assert.equal(ui.doc.querySelector('.rankbar').textContent, '●●○', 'rank 2 of 3');
  ui.doc.querySelector('select[data-skill="Skills.Ability.Sprint"]').value = '3';
  ui.doc.querySelector('select[data-skill="Skills.Ability.Sprint"]').dispatchEvent(new ui.dom.window.Event('change', { bubbles: true })); await ui.settle();
  ui.doc.querySelector('#skCharge').checked = true;
  ui.doc.querySelector('#skCharge').dispatchEvent(new ui.dom.window.Event('change', { bubbles: true })); await ui.settle();
  const sel = ui.doc.querySelector('select[data-skill="Skills.Perk.Aim"]'); sel.value = '1';
  sel.dispatchEvent(new ui.dom.window.Event('change', { bubbles: true })); await ui.settle();
  assert.deepEqual(ui.posted.filter((p) => p.path === '/api/player/skills/module').map((p) => p.body), [
    { module: 'Skills.Ability.Sprint', level: 3, charge: false }, { module: 'Skills.Perk.Aim', level: 1, charge: true }]);
  ui.click('[data-act="skStarter"]'); await ui.settle();
  assert.deepEqual(ui.posted.filter((p) => p.path === '/api/player/skills/starter').map((p) => p.body), [{ school: 'Trooper' }]);
  ui.doc.querySelector('#skPts').value = '90'; ui.click('[data-act="skPoints"]'); await ui.settle();
  assert.deepEqual(ui.posted.filter((p) => p.path === '/api/player/skills/points').map((p) => p.body), [{ points: 90 }]);
  ui.click('[data-skgrp="Other"]'); await ui.settle();
  assert.equal(ui.doc.querySelectorAll('select[data-skill]').length, 0, 'a skill the catalog does not know cannot be given a rank');
  assert.ok(ui.doc.querySelector('#skout').textContent.includes('catalog does not know its ranks'));
  ui.dom.window.close();
});

test('Research and Crafting: categories, filter, hostile names as text, and Unlock posts the right key', async () => {
  const hostile = '<img src=x onerror=1>';
  const rr = (o) => ({ itemKey: 'RCP_X', name: 'X', category: 'Combat', productGroup: 'Iron Products', type: 'Recipe', state: 'NotPurchased', isNew: false, unlockKind: 'recipe', unlockId: 'X', purchased: false, materialized: false, unlocked: false, needsRepair: false, actionable: true, ...o });
  const research = { rows: [rr({ itemKey: 'RCP_Rifle', name: 'Rifle ' + hostile }), rr({ itemKey: 'RCP_Done', name: 'Done', purchased: true, materialized: true, unlocked: true, state: 'Purchased' }),
    rr({ itemKey: 'RCP_Sand', name: 'Sandbike', category: 'Vehicles', productGroup: 'Copper Products', purchased: true, needsRepair: true, state: 'Purchased' }), rr({ itemKey: 'DA_GRP_W', name: 'Water', category: 'Water Discipline', type: 'Group', actionable: false, unlockKind: 'group', unlockId: '' })] };
  const crafting = { rows: [{ recipeId: 'HealthPackRecipe', name: 'Health Pack', category: 'Essentials', source: 'SchematicPickup', known: true, limited: true, uses: 1 }, { recipeId: 'T4_Rifle_Recipe', name: 'Rifle ' + hostile, category: 'Combat', source: 'Research', known: false, limited: false, uses: 0 }] };
  const ui = await boot({ get: { '/api/player/research': research, '/api/player/crafting': crafting }, post: { '/api/player/research/unlock': { ok: true, alreadyPurchased: false }, '/api/player/crafting/unlock': { ok: true, alreadyKnown: false } } });
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="player:research"]'); await ui.settle();
  assert.equal(ui.doc.querySelector('img'), null, 'a hostile research name is text, not markup');
  assert.equal(ui.doc.querySelector('[data-rscat=""]').textContent, 'All (4)');
  assert.equal(ui.doc.querySelectorAll('[data-act="rsUnlock"]').length, 2, 'Unlock only where something is missing');
  assert.ok([...ui.doc.querySelectorAll('[data-act="rsUnlock"]')].some((b) => b.textContent == 'Repair unlock'), 'a purchased entry with its recipe missing offers a repair');
  assert.ok(ui.doc.querySelector('button[disabled]').textContent == 'Group', 'a group marker has a disabled button');
  ui.click('[data-rscat="Vehicles"]'); await ui.settle();
  assert.equal(ui.doc.querySelectorAll('#rsout tbody tr').length, 1);
  assert.ok(ui.doc.querySelector('#rsGrp'), 'product groups appear once a category is chosen');
  ui.click('[data-rscat=""]'); await ui.settle();
  ui.doc.querySelector('#rsq').value = 'rifle'; ui.click('[data-act="rsfilter"]'); await ui.settle();
  assert.equal(ui.doc.querySelectorAll('#rsout tbody tr').length, 1);
  ui.click('[data-act="rsUnlock"]'); await ui.settle();
  assert.deepEqual(ui.posted.filter((p) => p.path === '/api/player/research/unlock').map((p) => p.body), [{ item_key: 'RCP_Rifle' }]);
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="player:crafting"]'); await ui.settle();
  assert.equal(ui.doc.querySelector('img'), null);
  assert.ok(ui.doc.querySelector('#crout').textContent.includes('limited use (1)'));
  assert.equal(ui.doc.querySelectorAll('[data-act="crUnlock"]').length, 1, 'Unlock only for recipes not known');
  ui.click('[data-act="crUnlock"]'); await ui.settle();
  assert.deepEqual(ui.posted.filter((p) => p.path === '/api/player/crafting/unlock').map((p) => p.body), [{ recipe_id: 'T4_Rifle_Recipe' }]);
  ui.dom.window.close();
});

test('Building Sets (groups, experimental toggle) and Customizations (sets, grant set) and the unlock-all buttons', async () => {
  const hostile = '<img src=x onerror=1>';
  const row = (o) => ({ id: 'X', name: 'X', group: 'Structures & Building Sets', learned: false, inInventory: false, inCatalog: true, ...o });
  const sets = { rows: [row({ id: 'AtreidesSet', name: 'Atreides ' + hostile, group: 'Faction & House Sets', learned: true }), row({ id: 'HarkSet', name: 'Harkonnen', group: 'Faction & House Sets', inInventory: true }),
    row({ id: 'Free_Patent', name: 'Free' }), row({ id: 'Dev_Patent', name: 'Dev', group: 'Experimental', experimental: true }), row({ id: 'MTX_Pack', name: 'Pack', group: 'Learned, not in the catalog', learned: true, inCatalog: false })], newPieces: [{ name: 'Piece_' + hostile }] };
  const cust = { rows: [row({ id: 'B1C3_Atre_A', name: 'Skin A', group: 'Atreides', groupId: 'atreides', inInventory: true }), row({ id: 'MTX_B1C2_DuneMan_B', name: 'Skin B', group: 'Dune Man', groupId: 'dune-man', requiredDlc: 'Lost Harvest', entitlement: true })],
    groups: [{ id: 'atreides', name: 'Atreides', count: 1, requirement: '' }, { id: 'dune-man', name: 'Dune Man', count: 1, requirement: 'Requires Lost Harvest' }] };
  const ui = await boot({ get: { '/api/player/building-sets': sets, '/api/player/customizations': cust }, post: { '/api/player/give-items': { ok: true, count: 1, itemIds: [5] }, '/api/player/building-sets/unlock-all': { ok: true, added: 4 }, '/api/player/customizations/grant': { ok: true, granted: 1 } } });
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="player:buildingsets"]'); await ui.settle();
  assert.equal(ui.doc.querySelector('img'), null, 'hostile names are text, not markup');
  assert.equal(ui.doc.querySelector('[data-ctf="have"]').textContent, 'Learned or in inventory (3)');
  assert.ok([...ui.doc.querySelectorAll('#ctgrp option')].some((o) => o.textContent == 'Experimental'), 'the Experimental group is listed');
  ui.doc.querySelector('#ctexp').checked = false; ui.doc.querySelector('#ctexp').dispatchEvent(new ui.dom.window.Event('change', { bubbles: true })); await ui.settle();
  assert.ok(![...ui.doc.querySelectorAll('#ctgrp option')].some((o) => o.textContent == 'Experimental'), 'hidden with the toggle off');
  assert.equal(ui.doc.querySelectorAll('#ctout tbody tr').length, 4, 'the experimental set is not listed');
  ui.doc.querySelector('#ctgrp').value = 'Faction & House Sets'; ui.doc.querySelector('#ctgrp').dispatchEvent(new ui.dom.window.Event('change', { bubbles: true })); await ui.settle();
  assert.equal(ui.doc.querySelectorAll('#ctout tbody tr').length, 2);
  assert.equal(ui.doc.querySelectorAll('[data-act="ctGive"]').length, 2);
  ui.click('[data-act="bsAll"]'); await ui.settle();
  ui.click('#aok'); await ui.settle();
  assert.equal(ui.posted.filter((p) => p.path === '/api/player/building-sets/unlock-all').length, 1, 'one request after the question is answered');
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="player:customizations"]'); await ui.settle();
  assert.equal(ui.doc.querySelectorAll('.setcard').length, 2, 'a card for each set');
  assert.ok(ui.doc.querySelector('#cthead').textContent.includes('Requires Lost Harvest'), 'the DLC a set needs is shown');
  ui.click('.setcard [data-act="ctGrantSet"][data-id="dune-man"]'); await ui.settle();
  ui.click('#aok'); await ui.settle();
  assert.deepEqual(ui.posted.filter((p) => p.path === '/api/player/customizations/grant').map((p) => p.body), [{ group: 'dune-man' }]);
  ui.click('[data-ctg="atreides"]'); await ui.settle();
  assert.equal(ui.doc.querySelectorAll('#ctout tbody tr').length, 1, 'clicking a set shows only its cosmetics');
  ui.dom.window.close();
});

test('Live Map: marker types that name inherited properties find no picture, known types get their icon class, nothing breaks', async () => {
  const hostile = '<img src=x onerror=1>';
  const data = { config: { key: 'HaggaBasin', label: 'Hagga Basin', image: '/maps/hagga-basin.png', width: 4096, height: 4096, minX: -456752, maxX: 354547, minY: -450630, maxY: 353821 },
    maps: [{ key: 'HaggaBasin', label: 'Hagga Basin' }], character: { name: 'Me', x: 0, y: 0, z: 0 },
    vehicles: [{ id: 1, name: 'Sandbike', class: '/Game/BP_Sandbike_CHOAM.BP_Sandbike_CHOAM_C', x: 10, y: 10 }], bases: [], storage: [],
    resourceFields: [{ kind: 'spice', size: 'Small', x: 5, y: 5, left: 5000 }, { kind: 'flour', size: '', x: 6, y: 6, left: 60000 }],
    markers: [{ t: '__proto__', x: 1, y: 1, d: 1 }, { t: 'constructor', x: 2, y: 2, d: 0 }, { t: 'toString', x: 3, y: 3, d: 1 }, { t: 'AzuriteOre', x: 4, y: 4, d: 1 }, { t: hostile, x: 5, y: 5, d: 1 }] };
  const ui = await boot({ get: { '/api/livemap': data } });
  ui.click('[data-tab="livemap"]'); await ui.settle();
  const main = ui.doc.querySelector('#main');
  assert.ok(main.textContent.includes('__proto__') && main.textContent.includes('constructor'), 'every type is listed: ' + main.textContent.slice(0, 300));
  assert.ok(!main.textContent.includes('TypeError') && !main.textContent.includes('is not a function'), main.textContent.slice(0, 200));
  assert.equal(ui.doc.querySelector('img'), null, 'a hostile type is text, not markup');
  assert.ok([...main.querySelectorAll('.lm-layer')].some((b) => b.textContent.includes('Spice fields (1)')) && [...main.querySelectorAll('.lm-layer')].some((b) => b.textContent.includes('Flour sand (1)')), 'the spice and flour sand layers are listed with their counts');
  assert.ok(main.textContent.includes('placed by decoding their ids'), 'the decoded-position note is shown');
  assert.equal(main.querySelectorAll('.lm-i-azuriteore').length, 1, 'a known type gets its icon class, the inherited names do not');
  assert.equal(main.querySelectorAll('#lmc ~ * .lm-ic, .lm-ic').length >= 2, true, 'the character legend entry has its picture');
  ui.dom.window.close();
});


test('No Guild or Respawn points in the Player view; Bases: the open base is labelled, others get Open, automatic refill is on Power and Water', async () => {
  const two = [{ base_id: 7, name: 'Base', base_type: 'Advanced Sub-Fief', owner: 'Owner', map: 'HaggaBasin', generators: 0, pieces: 0, placeables: 0, x: 0, y: 0, z: 0 },
    { base_id: 8, name: 'Outpost', base_type: 'Sub-Fief', owner: 'Owner', map: 'HaggaBasin', generators: 0, pieces: 0, placeables: 0, x: 1, y: 1, z: 1 }];
  const ui = await boot({ get: { '/api/bases/list': two, '/api/settings': { autoRefillOnOpen: false } } });
  ui.click('[data-tab="player"]'); await ui.settle();
  assert.ok(!ui.doc.querySelector('#main').textContent.includes('Guild'), 'a single-player game has no guild');
  assert.ok(!ui.doc.querySelector('#main').textContent.includes('Respawn points'), 'the respawn points card is gone');
  ui.click('[data-sub="player:bases"]'); await ui.settle();
  assert.equal(ui.doc.querySelectorAll('[data-bsopen]').length, 1, 'only the base that is not open gets an Open button');
  assert.ok(ui.doc.querySelector('#main').textContent.includes('Open below'), 'the open base says so');
  assert.ok(ui.doc.querySelector('[data-act="autoref"]'), 'automatic refill is on the Power tab');
  ui.click('[data-bstab="water"]'); await ui.settle();
  assert.ok(ui.doc.querySelector('[data-act="autoref"]'), 'and on the Water tab');
  ui.click('[data-bsopen="8"]'); await ui.settle();
  assert.ok(ui.doc.querySelector('#main').textContent.includes('Outpost (#8)'), 'Open switches to that base');
  ui.dom.window.close();
});

test('Unlock all: Crafting, Research and Skills each ask first and post one request', async () => {
  const m = (o) => ({ id: 'Skills.Ability.X', name: 'X', school: 'Trooper', kind: 'Ability', rank: 0, max: 3, spent: 0, ladder: [1, 3, 6], known: true, ...o });
  const sk = { total: 10, unspent: 5, spent: 0, schools: [{ key: 'Trooper', label: 'Trooper' }], modules: [m({})] };
  const ui = await boot({ get: { '/api/player/skills': sk }, post: { '/api/player/crafting/unlock-all': { ok: true, added: 3 }, '/api/player/research/unlock-all': { ok: true, bought: 2, repaired: 0, unlocksAdded: 2 }, '/api/player/skills/max': { ok: true, changed: 1 } } });
  ui.click('[data-tab="player"]'); await ui.settle();
  for (const [sub, act, path, body] of [['player:crafting', 'crAll', '/api/player/crafting/unlock-all', {}], ['player:research', 'rsAll', '/api/player/research/unlock-all', {}], ['player:skills', 'skMax', '/api/player/skills/max', { school: 'Trooper' }]]) {
    ui.click(`[data-sub="${sub}"]`); await ui.settle();
    ui.click(`[data-act="${act}"]`); await ui.settle();
    assert.equal(ui.posted.filter((p) => p.path === path).length, 0, 'nothing is sent before the question is answered');
    ui.click('#aok'); await ui.settle();
    assert.deepEqual(ui.posted.filter((p) => p.path === path).map((p) => p.body), [body], path);
  }
  ui.click('[data-act="skMax"][data-school="all"]'); await ui.settle();
  ui.click('#aok'); await ui.settle();
  assert.deepEqual(ui.posted.filter((p) => p.path === '/api/player/skills/max').map((p) => p.body).at(-1), { school: 'all' });
  ui.dom.window.close();
});

test('base and vehicle upkeep buttons post, and unknown device names stay plain text', async () => {
  const hostile = '<img src=x onerror=1> x1';
  const ui = await boot({ post: {
    '/api/bases/refill-water': { ok: true, filled: 2, alreadyFull: 1, skippedUnknown: [hostile] },
    '/api/bases/refill-generators': { ok: true, filled: 3, alreadyFull: 0 },
    '/api/vehicles/repair': { ok: true, modules: 9, repaired: 7, withoutKnownMax: 2 },
  } });
  ui.click('[data-tab=\"player\"]'); await ui.settle();
  ui.click('[data-sub=\"player:bases\"]'); await ui.settle();
  ui.click('[data-bstab="water"]'); await ui.settle();
  ui.click('[data-act="refillWater"]'); await ui.settle();
  let toast = ui.doc.querySelector('#toast');
  assert.ok(toast.textContent.includes('Filled 2 water devices (1 already full)') && toast.textContent.includes(hostile), toast.textContent);
  assert.equal(toast.querySelector('img'), null);
  ui.click('[data-bstab="power"]'); await ui.settle();
  ui.click('[data-act="refillGen"]'); await ui.settle();
  assert.ok(ui.doc.querySelector('#toast').textContent.includes('Filled 3 generators'));
  ui.click('[data-tab=\"player\"]'); await ui.settle();
  ui.click('[data-sub=\"player:admin\"]'); await ui.settle();
  ui.click('[data-act="repairV"]'); await ui.settle();
  assert.ok(ui.doc.querySelector('#toast').textContent.includes('Repaired 7 vehicle modules'));
  for (const p of ['/api/bases/refill-water', '/api/bases/refill-generators', '/api/vehicles/repair']) assert.equal(ui.posted.filter((x) => x.path === p).length, 1, p);
  ui.dom.window.close();
});

test('Expand claim with "no change" size sends only the level, and nothing at all when nothing is chosen', async () => {
  const claim = [{ totem_id: 7, name: 'Totem', cells: 1, rings: 0, level: 1, irregular: false, maxRings: 5, maxLevel: 5 }];
  const ui = await boot({ get: { '/api/bases/list': BASE7, '/api/bases/claim': claim }, post: { '/api/bases/claim/expand': { ok: true, added: 0, totalCells: 1, level: 3 } } });
  ui.click('[data-tab=\"player\"]'); await ui.settle();
  ui.click('[data-sub=\"player:bases\"]'); await ui.settle();
  ui.click('[data-bstab="claim"]'); await ui.settle();
  assert.deepEqual([...ui.doc.querySelectorAll('#cv7 option')].map((o) => o.value), ['1', '2', '3', '4', '5'], 'a lower level is not offered');
  ui.click('[data-act="claimGrow"]'); await ui.settle();
  assert.ok(ui.doc.querySelector('#toast').textContent.includes('Nothing to change'), 'defaults change nothing');
  assert.ok(ui.doc.querySelector('#ask').classList.contains('hide'), 'and no question is asked');
  ui.doc.querySelector('#cv7').value = '3';
  ui.click('[data-act="claimGrow"]'); await ui.settle();
  assert.ok(!ui.doc.querySelector('#ask').textContent.includes('square'), 'a level-only change does not talk about size');
  ui.click('[data-act="askOk"]'); await ui.settle();
  const sent = ui.posted.filter((p) => p.path === '/api/bases/claim/expand');
  assert.deepEqual(sent[0].body, { totem_id: 7, level: 3 });
  ui.dom.window.close();
});

test('Automatic refill setting shows its state, toggles through the API, and the start-up note is shown once as text', async () => {
  const hostile = '<img src=x onerror=1> note';
  const ui = await boot({ get: { '/api/settings': { autoRefillOnOpen: false }, '/api/startup': { note: hostile } }, post: { '/api/settings': { autoRefillOnOpen: true } } });
  assert.ok(ui.doc.querySelector('#toast').textContent.includes(hostile), 'the note is shown');
  assert.equal(ui.doc.querySelector('#toast img'), null, 'as text, never markup');
  ui.click('[data-tab=\"player\"]'); await ui.settle();
  ui.click('[data-sub=\"player:bases\"]'); await ui.settle();
  const card = [...ui.doc.querySelectorAll('.card')].find((c) => c.textContent.includes('Automatic refill'));
  assert.ok(card && card.textContent.includes('off'), 'the card shows the setting is off');
  ui.click('[data-act="autoref"]'); await ui.settle();
  assert.ok(ui.doc.querySelector('#ask').textContent.includes('without the Review & save step'), 'turning it on says it saves immediately');
  assert.equal(ui.posted.filter((p) => p.path === '/api/settings').length, 0, 'nothing is sent before confirming');
  ui.click('[data-act="askOk"]'); await ui.settle();
  assert.deepEqual(ui.posted.filter((p) => p.path === '/api/settings')[0].body, { autoRefillOnOpen: true }, 'turning on sends true');
  ui.dom.window.close();
});

test('A start-up note that reports a problem stays on screen as an error, and the last automatic save is shown on the card', async () => {
  const ui = await boot({ get: { '/api/startup': { note: 'Automatic refill skipped: a single-player session is active.', warn: true }, '/api/settings': { autoRefillOnOpen: true, last: '2026-10-08 13:00: Automatic refill saved 2 water devices (backed up as x.db)' } } });
  assert.ok(ui.doc.querySelector('#toast .err'), 'a problem note uses the error toast, which stays until dismissed');
  assert.ok(ui.doc.querySelector('#toast .err').textContent.includes('skipped'));
  ui.click('[data-tab=\"player\"]'); await ui.settle();
  ui.click('[data-sub=\"player:bases\"]'); await ui.settle();
  assert.ok(ui.doc.body.textContent.includes('Last automatic save: 2026-10-08 13:00'), 'the card keeps the last automatic save');
  ui.dom.window.close();
});

test('Shrink claim asks first, posts the chosen size, and is offered only when extra cells exist', async () => {
  const claims = [
    { totem_id: 7, name: 'Totem', cells: 9, rings: 1, level: 0, irregular: false, maxRings: 5, maxLevel: 5, piecesInClaim: 486, piecesOutside: 0 },
    { totem_id: 8, name: 'Other', cells: 1, rings: 0, level: 0, irregular: false, maxRings: 5, maxLevel: 5 },
  ];
  const ui = await boot({ get: { '/api/bases/list': BASE7, '/api/bases/claim': claims }, post: { '/api/bases/claim/shrink': { ok: true, removed: 8, remaining: 1 } } });
  ui.click('[data-tab=\"player\"]'); await ui.settle();
  ui.click('[data-sub=\"player:bases\"]'); await ui.settle();
  ui.click('[data-bstab="claim"]'); await ui.settle();
  assert.equal(ui.doc.querySelectorAll('[data-act="claimShrink"]').length, 1, 'only a claim with extra cells can shrink');
  assert.deepEqual([...ui.doc.querySelectorAll('#cs7 option')].map((o) => o.value), ['0'], 'a 3x3 claim can only shrink to its own cell, not to sizes it does not exceed');
  assert.ok(ui.doc.body.textContent.includes('486 pieces inside the claim'));
  ui.click('[data-act="claimShrink"]'); await ui.settle();
  assert.ok(ui.doc.querySelector('#ask').textContent.includes("the totem's own cell"), 'the question names the result');
  assert.equal(ui.posted.filter((p) => p.path === '/api/bases/claim/shrink').length, 0, 'nothing is sent before confirming');
  ui.click('[data-act="askOk"]'); await ui.settle();
  assert.deepEqual(ui.posted.filter((p) => p.path === '/api/bases/claim/shrink')[0].body, { totem_id: 7, rings: 0 });
  assert.ok(ui.doc.querySelector('#toast').textContent.includes('Removed 8 cells'));
  ui.dom.window.close();
});
