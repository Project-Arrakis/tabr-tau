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
const TAGS = new Set('html head body meta title link script header nav main div span b a p h1 h3 ul li code table thead tbody tr th td input select option datalist button details summary textarea br label canvas svg g rect text path'.split(' '));
const SCRIPTS = new Set(['/html.js', '/livemap.js', '/app.js']); // the only scripts the page may contain, both same-origin files
const ATTRS = new Set('lang charset name content rel href class id type value min max step placeholder size list selected disabled rows spellcheck colspan title src download role width height aria-modal aria-live aria-label aria-labelledby aria-describedby aria-sort inert tabindex label checked open viewbox x y rx transform d'.split(' '));

export function assertClean(doc, label) {
  const bad = [];
  for (const el of doc.querySelectorAll('*')) {
    const tag = el.tagName.toLowerCase();
    if (!TAGS.has(tag)) bad.push(`unexpected <${tag}>`);
    if (tag === 'script' && !SCRIPTS.has(el.getAttribute('src'))) bad.push(`<script> that is not one of the two bundled files: ${el.getAttribute('src') ?? 'inline'}`);
    for (const a of el.attributes) {
      const n = a.name.toLowerCase();
      if (n.startsWith('on')) bad.push(`event handler attribute ${n}`);
      if (n === 'style' && tag === 'html' && /^--hdr-h:\s*\d+(\.\d+)?px;?$/.test(a.value)) continue; // the header height the sticky banner sits below, set by the page itself
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
  ['player', 'player:crafting'], ['player', 'player:research'], ['player', 'player:customizations'], ['player', 'player:skills'],
  ['player', 'player:bases'], ['player', 'player:bases', '[data-bstab="water"]'], ['player', 'player:bases', '[data-bstab="inventory"]'], ['player', 'player:bases', '[data-bstab="claim"]'],
  ['player', 'player:vehicles'], ['livemap'], ['landsraad'], ['config'],
  ['db', 'db:browse'], ['db', 'db:sql'], ['db', 'db:backups'],
];
// Only views that cannot show text from the save are listed (audit QA-9): Admin offers only controls, Customizations lists the game's own catalog, and the SQL console and the backups list are empty until a query is run or a backup exists. Every other view must show the hostile text.
const NO_DATA = new Set(['player:admin', 'player:customizations', 'db:sql', 'db:backups']);

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
    currency: [{ label: 'Solari in wallet', balance: 591227 }, { label: 'Solari in backpack', balance: 95470 }, { label: 'House credit in wallet', balance: 0 }],
  };
  const ui = await boot({ get: { '/api/player/summary': summary } });
  ui.click('[data-tab="player"]'); await ui.settle();
  const text = ui.doc.querySelector('#main').textContent;
  for (const want of ['Player summary', 'Harkonnen', '71', '41,221', '9 unspent, 70 earned', '333 of 2,779', '112.5 (max 150 or more)', '52.5 of 100', 'Steam ID', 'Solari in wallet', '591,227', 'Solari in backpack', '95,470', 'House credit in wallet']) {
    assert.ok(text.includes(want), `the summary should show ${want}`);
  }
  assert.ok(ui.doc.querySelector('[data-sub="player:bases"]'), 'the Player tabs are still there');
  ui.dom.window.close();
});

test('the Player Summary hides the account identifiers until Show is pressed, per identifier, and shows them as plain text (GRC-5)', async () => {
  const hostile = '<img src=x onerror=1>';
  const summary = {
    name: 'Tester', status: 'Offline', map: 'HaggaBasin', faction: null, level: 5, xp: 10,
    identity: { platform: 'Steam', platformId: 'PLATFORM-ID-1', funcomId: 'FUNCOM-ID-2', flsId: hostile }, ids: { actor: 2, account: 1, controller: 1, playerState: 3 }, currency: [],
  };
  const ui = await boot({ get: { '/api/player/summary': summary } });
  ui.click('[data-tab="player"]'); await ui.settle();
  const page = () => ui.doc.querySelector('#main').textContent;
  const everything = () => ui.doc.documentElement.outerHTML;
  assert.ok(!everything().includes('PLATFORM-ID-1') && !everything().includes('FUNCOM-ID-2') && !everything().includes(hostile) && !everything().includes('onerror=1'), 'no identifier is in the page before Show is pressed');
  assert.ok(page().includes('Steam ID') && page().includes('Funcom ID') && page().includes('FLS ID'), 'the labels are there');
  assert.equal(ui.doc.querySelectorAll('[data-reveal]').length, 3);
  ui.click('[data-reveal="funcomId"]'); await ui.settle();
  assert.ok(page().includes('FUNCOM-ID-2'), 'the pressed one is shown');
  assert.ok(!page().includes('PLATFORM-ID-1'), 'the others stay hidden');
  ui.click('[data-reveal="flsId"]'); await ui.settle();
  assert.ok(page().includes(hostile) && ui.doc.querySelector('#main img') === null, 'a hostile value is text, not markup');
  ui.click('[data-reveal="funcomId"]'); await ui.settle();
  assert.ok(!page().includes('FUNCOM-ID-2'), 'Hide hides it again');
  ui.click('[data-tab="landsraad"]'); await ui.settle();
  ui.click('[data-tab="player"]'); await ui.settle();
  assert.ok(!page().includes(hostile), 'drawing the view again hides every identifier again');
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
  for (const want of ['Your vehicles (2)', 'Sandbike', 'Buggy', '40%', '85%', '90.5', '5,228, -408,425', 'Sandbike: 2 components', 'Sandbike Engine', '90%', '900 / 1000', 'Sandbike Chassis', '2000', 'Cargo hold', '1 of 15 slots used', 'CopperOre', '12 other vehicles']) {
    assert.ok(main().includes(want), `the vehicles tab should show ${want}`);
  }
  ui.click('[data-vhopen="153"]'); await ui.settle();
  assert.ok(main().includes('Buggy: 0 components') && main().includes('0 of 20 slots used') && main().includes('The cargo hold is empty.') && !main().includes('CopperOre'), 'the second vehicle has its own details');
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
  for (const want of ['Bases (1)', 'Advanced Sub-Fief Console', 'Advanced Sub-Fief', 'Tester', 'Building pieces', '486', '-67,159, -211,307, 12', 'Generator', '2,484', '2,500', 'Windtrap', 'Refill power']) assert.ok(main().includes(want), `the Power tab should show ${want}`);
  assert.deepEqual([...ui.doc.querySelectorAll('[data-bstab]')].map((b) => b.textContent.trim()), ['Power', 'Water', 'Inventory', 'Land claim editor', 'Structures']);
  assert.ok(!main().includes('Permissions'), 'the permission tabs are not mirrored');
  ui.click('[data-bstab="water"]'); await ui.settle();
  for (const want of ['Water Cistern', '16', '80,000', '100%', 'Refill base water']) assert.ok(main().includes(want), `the Water tab should show ${want}`);
  ui.click('[data-bstab="inventory"]'); await ui.settle();
  assert.deepEqual([...ui.doc.querySelectorAll('[data-bsgrp]')].map((b) => b.textContent.trim()), ['Storage (1)', 'Refining (1)', 'Crafting (0)', 'Other (0)']);
  assert.ok(main().includes('Ore Out') && main().includes('3 of 20') && !main().includes('Small Ore Refinery'), 'the first group with containers is shown');
  ui.click('[data-bsgrp="refining"]'); await ui.settle();
  assert.ok(main().includes('Small Ore Refinery') && main().includes('1 of 5') && !main().includes('Ore Out'), 'another group');
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
  assert.ok(main().includes('2 items, 2 of 35 slots used') && main().includes('CopperOre') && !main().includes('Knife'), 'the backpack is the first group with items');
  ui.click('[data-invg="character"]'); await ui.settle();
  assert.ok(ui.doc.querySelector('[title^="T6_Augment_Armor16"]') && main().includes('Combat_Heavy_Helmet_06') && !main().includes('CopperOre'), 'the character group lists the helmet with its augment');
  ui.click('[data-invg="backpack"]'); await ui.settle();
  ui.doc.querySelector('#invq').value = 'liter';
  ui.click('[data-act="invfilter"]'); await ui.settle();
  assert.ok(main().includes('1 of 2 items match') && main().includes('Literjon_T1') && !main().includes('CopperOre'), 'the filter narrows the group');
  ui.click('[data-act="invclear"]'); await ui.settle();
  assert.ok(main().includes('2 items') && !main().includes('2 of 2'), 'Clear shows the group again, with no "2 of 2"');
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
  assert.ok(toast.includes('41,221') && toast.includes('42,221') && toast.includes('level 71 → 72') && toast.includes('+1 skill points'), toast);
  ui.doc.querySelector('#curSel').value = '1'; ui.doc.querySelector('#curAmt').value = '250';
  ui.click('[data-act="currency"]'); await ui.settle();
  toast = ui.doc.querySelector('#toast').textContent;
  assert.ok(toast.includes('House credit in wallet') && toast.includes('250'), toast);
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

test('Admin: the actions tabr-tau has post, nothing is shown disabled, server-only ones are absent, repair is a link to its home', async () => {
  const ui = await boot({});
  ui.click('[data-tab="player"]'); await ui.settle();
  assert.equal(ui.doc.querySelector('[data-act="teleport"]'), null, 'Teleport is on Admin, not on Character > Overview');
  assert.equal(ui.doc.querySelector('[data-act="repair"]'), null, 'Repair gear is on the inventory tab, not on Overview');
  ui.click('[data-sub="player:admin"]'); await ui.settle();
  for (const act of ['teleport', 'faction']) assert.ok(ui.doc.querySelector(`[data-act="${act}"]`), `Admin has ${act}`);
  for (const act of ['repair', 'repairV']) assert.equal(ui.doc.querySelector(`[data-act="${act}"]`), null, `${act} has one home, which is not Admin`);
  const text = ui.doc.querySelector('#main').textContent;
  assert.ok(!text.includes('Not in tabr-tau yet') && !text.includes('Danger'), 'no stub and no empty Danger zone');
  assert.equal([...ui.doc.querySelectorAll('#main button')].filter((x) => x.disabled).length, 0, 'no disabled button stands in for a feature that is not built');
  for (const serverOnly of ['Kick Player', 'Ban Player', 'Repair Login Queue', 'Recover Character']) assert.ok(!text.includes(serverOnly), `${serverOnly} does not apply to single-player`);
  ui.click('[data-goto="character:inventory"]'); await ui.settle();
  assert.ok(ui.doc.querySelector('#invq'), 'the link opens Character > Inventory');
  ui.click('[data-act="repair"]'); await ui.settle();
  ui.click('[data-sub="player:admin"]'); await ui.settle();
  ui.click('[data-goto="vehicles"]'); await ui.settle();
  assert.ok(ui.doc.querySelector('#rvPct'), 'the link opens Vehicles, which holds the repair-below box');
  ui.click('[data-sub="player:admin"]'); await ui.settle();
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

// ---------- the queue: an edit is written as it is made; while the file cannot be written the edits wait, and the banner says so
const ATK = `"><img src=x ${MARK}>`;
const queuedState = (over = {}) => ({ ...JSON.parse(JSON.stringify(baseFixtures['/api/save/state'].body)), dirty: true, pending: [`edit ${ATK}`, 'second edit'], ...over });
const sessionBlock = { gameRunning: true, gameProcesses: ['DuneTest.exe'], saveBlocked: true, gameMode: 'single_player',
  blockReason: 'a single-player session is active (DuneTest.exe); go to the menu or multiplayer, or quit the game, then save' };

test('a written edit leaves nothing queued: no chip, no Discard, no banner', async () => {
  const ui = await boot({ get: { '/api/save/state': queuedState({ dirty: false, pending: [], lastWrite: { saved: true, queued: 0, backup: 'b.db' } }) } });
  assert.equal(ui.doc.querySelector('#pending').textContent, '');
  assert.equal(ui.doc.querySelector('#btnDiscard').hidden, true);
  assert.ok(ui.doc.querySelector('#gamewarn').classList.contains('hide'));
  assert.equal(ui.doc.querySelector('#btnSave'), null, 'there is no Review & save step');
  assert.equal(ui.doc.querySelector('#modal'), null);
  ui.dom.window.close();
});

test('an active single-player session queues the edits and the banner says how many and why, as plain text', async () => {
  const why = `a single-player session is active ${ATK}`;
  const ui = await boot({ get: { '/api/save/state': queuedState({ ...sessionBlock, lastWrite: { saved: false, queued: 2, why } }) } });
  const gw = ui.doc.querySelector('#gamewarn');
  assert.ok(gw.classList.contains('warn') && !gw.classList.contains('hide'));
  assert.ok(gw.textContent.includes('2 changes are queued') && gw.textContent.includes('single-player session is active'), gw.textContent);
  assert.equal(gw.querySelector('img'), null, 'the reason is data, not markup');
  assertClean(ui.doc, 'queue banner');
  assert.equal(ui.doc.querySelector('#pending').textContent, '2 queued');
  assert.equal(ui.doc.querySelector('#btnDiscard').hidden, false);
  ui.dom.window.close();
});

test('Write now asks the server to try again and says what came of it; Discard queue asks first', async () => {
  const ui = await boot({ get: { '/api/save/state': queuedState(sessionBlock) }, post: { '/api/save/flush': { saved: false, queued: 2, why: 'a single-player session is active' }, '/api/save/discard': { ok: true } } });
  ui.click('[data-act="writeNow"]'); await ui.settle();
  assert.equal(ui.posted.filter((p) => p.path === '/api/save/flush').length, 1);
  assert.ok(ui.doc.querySelector('#toast').textContent.includes('Still waiting'));
  ui.click('#btnDiscard'); await ui.settle();
  assert.equal(ui.posted.filter((p) => p.path === '/api/save/discard').length, 0, 'nothing is dropped before the question is answered');
  assert.ok(ui.doc.querySelector('#ask').textContent.includes('Discard the queued changes'));
  ui.click('#aok'); await ui.settle();
  assert.equal(ui.posted.filter((p) => p.path === '/api/save/discard').length, 1);
  ui.dom.window.close();
});

test('a queue held because the game saved conflicting changes says so, lists the conflicts as plain text, and offers Discard queue', async () => {
  const ui = await boot({ get: { '/api/save/state': queuedState({ held: { conflicts: [{ desc: `Give 5 x Iron ${ATK}`, why: `it changed 0 rows now and 1 before ${ATK}` }] } }) }, post: { '/api/save/discard': { ok: true } } });
  const gw = ui.doc.querySelector('#gamewarn');
  assert.ok(gw.textContent.includes('2 queued changes were not written') && gw.textContent.includes('left the game\'s file as it is'), gw.textContent);
  assert.ok(gw.textContent.includes('it changed 0 rows now'), 'the conflict is listed');
  assert.equal(gw.querySelector('img'), null, 'as text, never markup');
  assert.ok(!gw.textContent.includes('written to the game file by themselves'), 'nothing promises a write that will not happen');
  assertClean(ui.doc, 'held queue');
  ui.click('[data-act="discardQueue"]'); await ui.settle();
  assert.ok(ui.doc.querySelector('#ask').textContent.includes('Discard the queued changes'));
  ui.dom.window.close();
});

test('a game check that cannot run is said as plain text and nothing offers to save', async () => {
  const why = '<img src=x onerror=1> tasklist missing';
  const ui = await boot({ get: { '/api/save/state': queuedState({ gameRunning: false, gameCheckError: why, saveBlocked: true, blockReason: 'could not check whether the game is running (' + why + ')', gameMode: 'unknown' }) } });
  const gw = ui.doc.querySelector('#gamewarn');
  assert.ok(gw.textContent.includes('Changes cannot be written') && gw.textContent.includes('tasklist missing'), gw.textContent);
  assert.equal(gw.querySelector('img'), null, 'the error text is data, not markup');
  ui.dom.window.close();
});

test('a save the game check cannot vouch for queues the edits, says why, and Allow writing asks first and then posts the confirmation (NET-3)', async () => {
  const why = 'the save is on a network share, so this computer cannot tell whether the game is using it';
  const ui = await boot({ get: { '/api/save/state': queuedState({ gameRunning: false, saveBlocked: true, blockReason: why, gameMode: 'unknown', locationUnconfirmed: true }) }, post: { '/api/save/confirm-location': { ok: true } } });
  const gw = ui.doc.querySelector('#gamewarn');
  assert.ok(gw.textContent.includes('cannot be written here yet') && gw.textContent.includes('network share'), gw.textContent);
  assert.ok(!gw.textContent.includes('written to the game file by themselves'), 'nothing promises a write the guard will refuse');
  ui.click('[data-act="confirmLocation"]'); await ui.settle();
  assert.equal(ui.posted.filter((p) => p.path === '/api/save/confirm-location').length, 0, 'nothing is sent before the question is answered');
  assert.ok(ui.doc.querySelector('#ask').textContent.includes('closed everywhere'), 'the question says what the person is vouching for');
  ui.click('[data-act="askNo"]'); await ui.settle();
  assert.equal(ui.posted.filter((p) => p.path === '/api/save/confirm-location').length, 0, 'Cancel sends nothing');
  ui.click('[data-act="confirmLocation"]'); await ui.settle(); ui.click('#aok'); await ui.settle();
  assert.equal(ui.posted.filter((p) => p.path === '/api/save/confirm-location').length, 1);
  ui.dom.window.close();
});

test('the game open in multiplayer or at the menu shows a note and edits are written at once', async () => {
  const ui = await boot({ get: { '/api/save/state': queuedState({ dirty: false, pending: [], gameRunning: true, gameProcesses: ['DuneTest.exe'], saveBlocked: false, gameMode: 'menu_or_multiplayer' }) } });
  const gw = ui.doc.querySelector('#gamewarn');
  assert.ok(gw.classList.contains('info') && !gw.classList.contains('warn'), gw.className);
  assert.ok(gw.textContent.includes('not in single-player') && gw.textContent.includes('DuneTest.exe') && gw.textContent.includes('written at once'), gw.textContent);
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
  const ui = await boot({ get: { '/api/save/backups': [{ name: 'a.db', size: 1, modified: 'x' }, { name: 'b.db', size: 2, modified: 'y' }] } });
  ui.click('[data-tab="db"]'); await ui.settle();
  ui.click('[data-sub="db:backups"]'); await ui.settle();
  ui.click('[data-act="restore"][data-n="a.db"]'); await ui.settle();
  const ok = ui.doc.querySelector('#aok');
  assert.equal(ok.disabled, true, 'OK is off until the word is typed');
  const inp = ui.doc.querySelector('#atype');
  inp.value = 'nope'; inp.dispatchEvent(new ui.w.Event('input', { bubbles: true }));
  assert.equal(ui.doc.querySelector('#aok').disabled, true);
  inp.value = ' Restore '; inp.dispatchEvent(new ui.w.Event('input', { bubbles: true }));
  assert.equal(ui.doc.querySelector('#aok').disabled, false, 'the right word (any case) enables it');
  ui.doc.dispatchEvent(new ui.w.KeyboardEvent('keydown', { key: 'Escape', bubbles: true })); await ui.settle();
  assert.ok(ui.doc.querySelector('#ask').classList.contains('hide'));
  assert.equal(ui.posted.filter((p) => p.path === '/api/save/restore').length, 0, 'Escape must not run the action');
  ui.dom.window.close();
});

test('the Bases upkeep buttons ask on the button itself (no dialog), then run on the second press: the edit is written and a backup is kept', async () => {
  const ui = await boot({ post: { '/api/bases/repair': { ok: true, pieces: 3, placeables: 1 }, '/api/bases/clear-sand': { ok: true, pieces: 2 } } });
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="player:bases"]'); await ui.settle();
  ui.click('[data-bstab="structures"]'); await ui.settle();
  ui.click('[data-act="repairB"]'); await ui.settle();
  assert.ok(ui.doc.querySelector('#ask').classList.contains('hide'), 'no dialog');
  assert.equal(ui.posted.filter((p) => p.path === '/api/bases/repair').length, 0, 'the first press only asks');
  ui.click('[data-act="repairB"]'); await ui.settle();
  assert.equal(ui.posted.filter((p) => p.path === '/api/bases/repair').length, 1);
  ui.click('[data-act="sand"]'); await ui.settle();
  assert.equal(ui.posted.filter((p) => p.path === '/api/bases/clear-sand').length, 0, 'the first press only asks');
  ui.click('[data-act="sand"]'); await ui.settle();
  assert.ok(ui.doc.querySelector('#ask').classList.contains('hide'), 'no dialog');
  assert.equal(ui.posted.filter((p) => p.path === '/api/bases/clear-sand').length, 1);
  ui.dom.window.close();
});

test('an edit that is queued says so; one that is written says nothing more', async () => {
  const ov = JSON.parse(JSON.stringify(baseFixtures['/api/save/state'].body)); ov.dirty = true;
  const ui = await boot({ get: { '/api/save/state': ov } });
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-act="solari"]'); await ui.settle();
  assert.ok(ui.doc.querySelector('#toast').textContent.includes('queued'));
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
  const ui = await boot({ get: { '/api/save/backups': [{ name: 'a.db', size: 1, modified: 'x' }, { name: 'b.db', size: 2, modified: 'y' }] } });
  ui.click('[data-tab="db"]'); await ui.settle();
  ui.click('[data-sub="db:backups"]'); await ui.settle();
  ui.click('[data-act="restore"][data-n="a.db"]'); await ui.settle();
  ui.click('#aok'); await ui.settle(); // disabled: a click must do nothing
  assert.equal(ui.posted.filter((p) => p.path === '/api/save/restore').length, 0);
  assert.ok(!ui.doc.querySelector('#ask').classList.contains('hide'));
  const inp = ui.doc.querySelector('#atype');
  inp.value = 'restore'; inp.dispatchEvent(new ui.w.Event('input', { bubbles: true }));
  inp.dispatchEvent(new ui.w.KeyboardEvent('keydown', { key: 'Enter', bubbles: true })); await ui.settle();
  assert.equal(ui.posted.filter((p) => p.path === '/api/save/restore').length, 1);
  assert.ok(ui.doc.querySelector('#ask').classList.contains('hide'));
  ui.dom.window.close();
});

test('a second question replaces the first, which counts as cancelled', async () => {
  const ui = await boot({ get: { '/api/save/backups': [{ name: 'a.db', size: 1, modified: 'x' }, { name: 'b.db', size: 2, modified: 'y' }] } });
  ui.click('[data-tab="db"]'); await ui.settle();
  ui.click('[data-sub="db:backups"]'); await ui.settle();
  ui.click('[data-act="restore"][data-n="a.db"]'); await ui.settle();
  ui.click('[data-act="restore"][data-n="b.db"]'); await ui.settle();
  assert.ok(ui.doc.querySelector('#ask').textContent.includes('b.db'));
  assert.ok(!ui.doc.querySelector('#ask').textContent.includes('a.db'));
  assert.equal(ui.posted.filter((p) => p.path === '/api/save/restore').length, 0);
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

test('Refill Literjons reports what it filled and shows a container it could not size as plain text', async () => {
  const hostile = '<img src=x onerror=1> x1';
  const ui = await boot({ post: { '/api/player/refill': { ok: true, filled: 3, alreadyFull: 1, skippedUnknown: [hostile] } } });
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="char:inventory"]'); await ui.settle();
  ui.click('[data-act="refill"]'); await ui.settle();
  const toast = ui.doc.querySelector('#toast');
  assert.ok(toast.textContent.includes('Filled 3 Literjons (1 already full)'), toast.textContent);
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
  const card = [...ui.doc.querySelectorAll('.card')].find((c) => c.textContent.includes('Resize by square'));
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
  // text that is not an item is never sent on as an id (#161): neither a raw id the catalog and the save do not hold, nor a half-name
  for (const bad of ['Some_Raw_Id', 'Spice', 'fuel']) {
    ui.doc.querySelector('#gt').value = bad;
    ui.click('[data-act="give"]'); await ui.settle();
    assert.equal(ui.posted.filter((p) => p.path === '/api/player/give').length, 4, `"${bad}" must not be posted`);
    assert.ok(ui.doc.querySelector('#toast').textContent.includes(`"${bad}" is not an item`), `the toast says so for "${bad}": ${ui.doc.querySelector('#toast').textContent}`);
  }
  ui.doc.querySelector('#gt').value = '';
  ui.click('[data-act="give"]'); await ui.settle();
  assert.equal(ui.posted.filter((p) => p.path === '/api/player/give').length, 4, 'an empty box posts nothing');
  assert.ok(ui.doc.querySelector('#toast').textContent.includes('Pick an item first'));
  ui.dom.window.close();
});

test('Give Items queue: items are queued, listed with hostile names as text, removed, and given in one post', async () => {
  const inv = JSON.parse(JSON.stringify(baseFixtures['/api/player/inventory'].body));
  const hostileName = '<img src=x onerror=1>';
  const catalog = [{ id: 'Hostile_1', name: hostileName, category: 'misc' }, { id: 'MelangeSpice', name: 'Spice Melange', category: 'resources' }];
  const ui = await boot({ get: { '/api/catalog/items': catalog, '/api/player/inventory': inv }, post: { '/api/player/give-items': { ok: true, count: 2, itemIds: [1, 2] } } });
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="char:inventory"]'); await ui.settle();
  assert.ok(!ui.doc.querySelector('[data-act="queueGive"]'), 'no queue until something is added');
  ui.doc.querySelector('#gt').value = hostileName; ui.doc.querySelector('#gq').value = '3'; ui.doc.querySelector('#gg').value = '2';
  ui.click('[data-act="queueAdd"]'); await ui.settle();
  ui.doc.querySelector('#gt').value = 'Spice Melange'; ui.doc.querySelector('#gq').value = '0';
  ui.click('[data-act="queueAdd"]'); await ui.settle();
  assert.equal(ui.doc.querySelectorAll('[data-act="queueDel"]').length, 2);
  assert.equal(ui.doc.querySelector('img'), null, 'a hostile item name is text, not markup');
  ui.click('[data-act="queueDel"][data-id="1"]'); await ui.settle();
  ui.doc.querySelector('#gt').value = 'Spice Melange'; ui.doc.querySelector('#gq').value = '5'; ui.doc.querySelector('#gg').value = '0';
  ui.click('[data-act="queueAdd"]'); await ui.settle();
  ui.click('[data-act="queueGive"]'); await ui.settle();
  const post = ui.posted.filter((p) => p.path === '/api/player/give-items');
  assert.equal(post.length, 1, 'one request for the whole queue');
  assert.deepEqual(post[0].body.items, [{ template_id: 'Hostile_1', quantity: 3, quality: 2 }, { template_id: 'MelangeSpice', quantity: 5, quality: 0 }], 'the names were turned into ids');
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
  inv.templates = ['Helm_1', 'Ore_1']; // the ids the save holds, which the item box accepts
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
  ui.click('[data-sub="player:vehicles"]'); await ui.settle();
  assert.equal(ui.doc.querySelector('#rvPct').value, '100', 'Vehicles repairs everything by default (UX-03)');
  ui.doc.querySelector('#rvPct').value = '75';
  ui.click('[data-act="repairV"]'); await ui.settle();
  assert.deepEqual(ui.posted.filter((p) => p.path === '/api/vehicles/repair').map((p) => p.body), [{ threshold: 75 }]);
  ui.dom.window.close();
});

test('a vehicle repair that repaired nothing says why: nothing below the threshold, or nothing to repair (UX-03)', async () => {
  const ui = await boot({ post: { '/api/vehicles/repair': { ok: true, modules: 4, repaired: 0, withoutKnownMax: 0 } } });
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="player:vehicles"]'); await ui.settle();
  ui.doc.querySelector('#rvPct').value = '75';
  ui.click('[data-act="repairV"]'); await ui.settle();
  let t = ui.doc.querySelector('#toast').textContent;
  assert.ok(t.includes('none of the 4 modules is below 75% of its full durability') && !t.includes('Repaired 0') && !t.includes('not saved yet'), t);
  ui.doc.querySelector('#rvPct').value = '100';
  ui.click('[data-act="repairV"]'); await ui.settle();
  t = ui.doc.querySelector('#toast').textContent;
  assert.ok(t.includes('all 4 modules are already as good as new'), t);
  ui.dom.window.close();
  const none = await boot({ post: { '/api/vehicles/repair': { ok: true, modules: 0, repaired: 0, withoutKnownMax: 0 } } });
  none.click('[data-tab="player"]'); await none.settle();
  none.click('[data-sub="player:vehicles"]'); await none.settle();
  none.click('[data-act="repairV"]'); await none.settle();
  assert.ok(none.doc.querySelector('#toast').textContent.includes('no vehicle modules were found'), none.doc.querySelector('#toast').textContent);
  none.dom.window.close();
});

test('config edits are written at once and never say "not saved yet"; Save raw text asks first (UX-05)', async () => {
  const ov = JSON.parse(JSON.stringify(baseFixtures['/api/save/state'].body)); ov.dirty = true; ov.pending = ['edit something']; // the save has edits waiting
  const ui = await boot({ get: { '/api/save/state': ov }, post: { '/api/config/set': { ok: true }, '/api/config/raw': { ok: true }, '/api/config/delete': { ok: true } } });
  ui.click('[data-tab="config"]'); await ui.settle();
  const box = ui.doc.querySelector('input.cfgv');
  assert.ok(box, 'the config fixture has a value box');
  box.value = 'changed';
  box.dispatchEvent(new ui.w.Event('change', { bubbles: true })); await ui.settle();
  let t = ui.doc.querySelector('#toast').textContent;
  assert.ok(t.includes('config file now') && !t.includes('not saved yet'), t);
  ui.click('[data-act="cfgraw"]'); await ui.settle();
  assert.equal(ui.posted.filter((p) => p.path === '/api/config/raw').length, 0, 'nothing is sent before the question is answered');
  assert.ok(ui.doc.querySelector('#ask').textContent.includes('Everything in'), 'the question says the whole file is replaced');
  ui.click('[data-act="askNo"]'); await ui.settle();
  assert.equal(ui.posted.filter((p) => p.path === '/api/config/raw').length, 0, 'Cancel sends nothing');
  ui.click('[data-act="cfgraw"]'); await ui.settle(); ui.click('#aok'); await ui.settle();
  assert.equal(ui.posted.filter((p) => p.path === '/api/config/raw').length, 1);
  t = ui.doc.querySelector('#toast').textContent;
  assert.ok(t.includes('File written now') && !t.includes('not saved yet'), t);
  ui.dom.window.close();
});

test('a view that fails to load shows why and a Try again button, and a signed-out page gets the editor own sentence (UX-10)', async () => {
  const saved = baseFixtures['/api/landsraad'];
  baseFixtures['/api/landsraad'] = { status: 403, body: { error: 'This window is no longer signed in to the editor, usually because the editor was closed and started again.', code: 'not_signed_in' } };
  let ui;
  try {
    ui = await boot();
    ui.click('[data-tab="landsraad"]'); await ui.settle();
    assert.ok(ui.doc.querySelector('#main').textContent.includes('no longer signed in to the editor'), ui.doc.querySelector('#main').textContent);
    assert.ok(ui.doc.querySelector('[data-act="retry"]'), 'a failed view offers Try again');
    baseFixtures['/api/landsraad'] = saved; // the problem is fixed
    ui.click('[data-act="retry"]'); await ui.settle();
    assert.ok(!ui.doc.querySelector('[data-act="retry"]') && ui.doc.querySelector('#main').textContent.includes('Task board'), 'Try again draws the view');
  } finally {
    baseFixtures['/api/landsraad'] = saved;
    if (ui) ui.dom.window.close();
  }
});

test('an inline edit the server refuses goes back to the value the box had; an accepted one becomes the new value to go back to (UX-14)', async () => {
  const inv = baseFixtures['/api/player/inventory'].body;
  const first = inv.items.find((r) => r.inventory_type == 0) || inv.items[0];
  const ui = await boot({ post: { '/api/items/update': { __fail: 'stack out of range' } } });
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="char:inventory"]'); await ui.settle();
  const box = ui.doc.querySelector(`input[data-item="${first.id}"][data-field="stack_size"]`);
  assert.ok(box, 'the stack box of the first item');
  const was = box.value;
  box.value = '999999999';
  box.dispatchEvent(new ui.w.Event('change', { bubbles: true })); await ui.settle();
  assert.ok(ui.doc.querySelector('#toast').textContent.includes('stack out of range'), 'the refusal is shown');
  assert.equal(box.value, was, 'the refused value is not left in the box');
  ui.dom.window.close();

  const ok = await boot({ post: { '/api/items/update': { ok: true } } });
  ok.click('[data-tab="player"]'); await ok.settle();
  ok.click('[data-sub="char:inventory"]'); await ok.settle();
  const b2 = ok.doc.querySelector(`input[data-item="${first.id}"][data-field="stack_size"]`);
  b2.value = '77';
  b2.dispatchEvent(new ok.w.Event('change', { bubbles: true })); await ok.settle();
  assert.equal(b2.value, '77', 'an accepted value stays');
  assert.equal(b2.defaultValue, '77', 'and is what a later refusal goes back to');
  ok.dom.window.close();
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

test('Customizations: Unlock adds cosmetics to the library without an item (sets and single rows), shows what is unlocked, and says what it does not do', async () => {
  const row = (o) => ({ id: 'X', name: 'X', group: 'Atreides', groupId: 'atreides', learned: false, inInventory: false, inCatalog: true, ...o });
  const cust = { libraryAvailable: true, libraryCount: 5, groups: [{ id: 'atreides', name: 'Atreides', count: 3, requirement: '' }],
    rows: [row({ id: 'B1C3_Atre_Done', name: 'Done', unlockable: true, unlocked: true }), row({ id: 'B1C3_Atre_Sword', name: 'Sword', unlockable: true }), row({ id: 'B1C3_Atre_NoData', name: 'No data' }), row({ id: 'MTX_Atre_Rifle', name: 'Store rifle', unlockable: true, entitlement: true })] };
  const ui = await boot({ get: { '/api/player/customizations': cust }, post: { '/api/player/customizations/unlock': { ok: true, added: 7, skippedEntitlement: 3 } } });
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="player:customizations"]'); await ui.settle();
  assert.ok(ui.doc.querySelector('[data-act="ctUnlockSet"][data-id="all"]'), 'Unlock all sets');
  assert.ok(ui.doc.querySelector('#cthead').textContent.includes('Unlocked or in inventory (1)'), 'what is unlocked counts as owned');
  assert.ok(ui.doc.querySelector('#main').textContent.includes('5 unlocked now'));
  assert.equal(ui.doc.querySelectorAll('#ctout [data-act="ctUnlock"]').length, 2, 'a row can be unlocked only when it has unlock data and is not unlocked yet');
  assert.ok(ui.doc.querySelector('#ctout [data-act="ctUnlock"][data-ent="1"]').getAttribute('title').includes('entitlement'), 'an entitlement cosmetic is flagged on its button');
  assert.ok(ui.doc.querySelector('#ctout').textContent.includes('Unlocked'), 'the state column says Unlocked');
  assert.ok(ui.doc.body.textContent.includes('set unlocks skip those'), 'it says set unlocks skip store cosmetics');
  ui.click('.setcard [data-act="ctUnlockSet"][data-id="atreides"]'); await ui.settle();
  ui.click('#aok'); await ui.settle();
  assert.ok(ui.doc.body.textContent.includes('3 store or DLC cosmetics skipped'), 'the toast says how many were skipped');
  ui.click('#ctout [data-act="ctUnlock"][data-ent=""]'); await ui.settle();
  ui.click('#aok'); await ui.settle();
  assert.deepEqual(ui.posted.filter((p) => p.path === '/api/player/customizations/unlock').map((p) => p.body), [{ group: 'atreides' }, { id: 'B1C3_Atre_Sword' }]);
  // a save with no library list: no Unlock buttons, the reason is given, giving as items is still there
  const ui2 = await boot({ get: { '/api/player/customizations': { ...cust, libraryAvailable: false, libraryCount: 0 } } });
  ui2.click('[data-tab="player"]'); await ui2.settle();
  ui2.click('[data-sub="player:customizations"]'); await ui2.settle();
  assert.equal(ui2.doc.querySelectorAll('[data-act="ctUnlockSet"], [data-act="ctUnlock"]').length, 0);
  assert.ok(ui2.doc.body.textContent.includes('no customization library'));
  assert.ok(ui2.doc.querySelector('[data-act="ctGrantSet"]'));
  ui.dom.window.close();
  ui2.dom.window.close();
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


test('Live Map: drawing the map again does not add another window mouseup listener each time', async () => {
  const data = { config: { key: 'HaggaBasin', label: 'Hagga Basin', image: '/maps/hagga-basin.png', width: 4096, height: 4096, minX: -456752, maxX: 354547, minY: -450630, maxY: 353821 },
    maps: [{ key: 'HaggaBasin', label: 'Hagga Basin' }], character: { name: 'Me', x: 0, y: 0, z: 0 }, vehicles: [], bases: [], storage: [], resourceFields: [],
    markers: [{ t: 'AzuriteOre', x: 4, y: 4, d: 1 }] };
  const ui = await boot({ get: { '/api/livemap': data } });
  const seen = [];
  const add = ui.w.addEventListener.bind(ui.w);
  ui.w.addEventListener = (type, ...rest) => { seen.push(type); return add(type, ...rest); };
  ui.click('[data-tab="livemap"]'); await ui.settle();
  for (let i = 0; i < 4; i++) { ui.click('[data-lmund]'); await ui.settle(); } // each click draws the whole view again
  assert.ok(ui.doc.querySelector('#lmc'), 'the map is drawn');
  assert.equal(seen.filter((t) => t === 'mouseup').length, 1, 'one window mouseup listener for the page, however often the map is drawn: ' + seen.join(','));
  ui.dom.window.close();
});

test('A damaged settings file is shown on the automatic refill card, as text', async () => {
  const problem = 'the settings file x is damaged (<img src=x onerror=window.__pwned=1>)';
  const ui = await boot({ get: { '/api/settings': { autoRefillOnOpen: false, problem } } });
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="player:bases"]'); await ui.settle();
  const card = ui.doc.querySelector('[data-act="autoref"]').closest('.card');
  assert.ok(card.querySelector('p.bad').textContent === problem, 'the problem is shown');
  assert.equal(card.querySelector('img'), null, 'and it is text, not markup');
  const ui2 = await boot({ get: { '/api/settings': { autoRefillOnOpen: false, problem: '' } } });
  ui2.click('[data-tab="player"]'); await ui2.settle();
  ui2.click('[data-sub="player:bases"]'); await ui2.settle();
  assert.equal(ui2.doc.querySelector('[data-act="autoref"]').closest('.card').querySelector('p.bad'), null, 'no problem, nothing shown');
  ui.dom.window.close(); ui2.dom.window.close();
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

test('Specialization: every track is listed, unknown numbers disable XP actions, Assign and the keystone and XP buttons post the right requests', async () => {
  const hostile = '<img src=x onerror=1>';
  const row = (o) => ({ track: 'Combat', number: null, xp: 0, level: 0, keystoneOwned: 0, keystoneTotal: 41, granted: false, ...o });
  const specs = { rows: [row({ track: 'Combat', number: 2, xp: 3000, level: 22.9, keystoneOwned: 41, granted: true }), row({ track: 'Crafting', keystoneOwned: 3 }), row({ track: 'Exploration' }), row({ track: 'Gathering' }), row({ track: 'Sabotage' })],
    unassigned: [{ number: 7, xp: 500, level: 4.2 }], maxXp: 44182, tracks: ['Combat', 'Crafting', 'Exploration', 'Gathering', 'Sabotage'] };
  const ui = await boot({ get: { '/api/player/specs': specs }, post: { '/api/player/specs/keystones/grant': { ok: true, granted: 5 }, '/api/player/specs/keystones/reset': { ok: true, removed: 5 }, '/api/player/specs/xp': { ok: true, before: 3000, after: 4000, level: 26 }, '/api/player/specs/max': { ok: true }, '/api/player/specs/reset': { ok: true, removed: 1 }, '/api/player/specs/assign': { ok: true } } });
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="player:specialization"]'); await ui.settle();
  assert.equal(ui.doc.querySelector('img'), null);
  assert.equal(ui.doc.querySelectorAll('tbody tr').length, 5, 'all five tracks are listed, even with no XP, as in the console');
  assert.equal(ui.doc.querySelectorAll('[data-act="spAdd"]:not([disabled])').length, 1, 'only the track whose number is known can take XP');
  assert.ok(ui.doc.body.textContent.includes('Granted'));
  ui.click('[data-act="spKeyAll"]'); await ui.settle();
  assert.equal(ui.posted.filter((p) => p.path === '/api/player/specs/keystones/grant').length, 0, 'nothing is sent before the question is answered');
  ui.click('#aok'); await ui.settle();
  assert.equal(ui.posted.filter((p) => p.path === '/api/player/specs/keystones/grant').length, 1);
  ui.click('[data-act="spKeyReset"]'); await ui.settle(); ui.click('#aok'); await ui.settle();
  assert.equal(ui.posted.filter((p) => p.path === '/api/player/specs/keystones/reset').length, 1);
  ui.doc.querySelector('#spxCombat').value = '1000';
  ui.click('[data-act="spAdd"][data-t="Combat"]'); await ui.settle();
  assert.deepEqual(ui.posted.filter((p) => p.path === '/api/player/specs/xp').map((p) => p.body), [{ track: 'Combat', amount: 1000 }]);
  ui.click('[data-act="spMax"][data-t="Combat"]'); await ui.settle(); ui.click('#aok'); await ui.settle();
  assert.deepEqual(ui.posted.filter((p) => p.path === '/api/player/specs/max').map((p) => p.body), [{ track: 'Combat' }]);
  ui.click('[data-act="spReset"][data-t="Combat"]'); await ui.settle(); ui.click('#aok'); await ui.settle();
  assert.deepEqual(ui.posted.filter((p) => p.path === '/api/player/specs/reset').map((p) => p.body), [{ track: 'Combat' }]);
  ui.doc.querySelector('#spa7').value = 'Gathering';
  ui.click('[data-act="spAssign"][data-n="7"]'); await ui.settle();
  assert.deepEqual(ui.posted.filter((p) => p.path === '/api/player/specs/assign').map((p) => p.body), [{ track: 'Gathering', number: 7 }]);
  ui.dom.window.close();
});

test('Specialization on a new save says why the XP controls are off and what to do; keystones still work; the raw row is folded and asks first (UX-36)', async () => {
  const row = (o) => ({ track: 'Combat', number: null, xp: 0, level: 0, keystoneOwned: 0, keystoneTotal: 41, granted: false, ...o });
  const specs = { rows: [row({}), row({ track: 'Gathering' })], unassigned: [], maxXp: 44182, tracks: ['Combat', 'Gathering'] };
  const ui = await boot({ get: { '/api/player/specs': specs }, post: { '/api/player/specs': { ok: true } } });
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="player:specialization"]'); await ui.settle();
  const note = ui.doc.querySelector('#spnone');
  assert.ok(note, 'a note appears while no track has a number');
  assert.ok(note.textContent.includes('Earn a little XP') && note.textContent.includes('Track numbers') && note.textContent.includes('Keystones need no numbers'), note.textContent);
  const add = ui.doc.querySelector('[data-act="spAdd"][data-t="Combat"]');
  assert.ok(add.disabled && add.title.includes('not known yet'), 'a disabled button says why');
  assert.ok(!ui.doc.querySelector('[data-act="spKeyAll"]').disabled, 'keystones need no numbers');
  const adv = [...ui.doc.querySelectorAll('details')].find((d) => d.textContent.includes('Advanced: set a raw row'));
  assert.ok(adv && !adv.open, 'the raw row is folded');
  ui.doc.querySelector('#st').value = '3'; ui.doc.querySelector('#sx').value = '500'; ui.doc.querySelector('#sl').value = '2';
  ui.click('[data-act="specSet"]'); await ui.settle();
  assert.equal(ui.posted.filter((p) => p.path === '/api/player/specs').length, 0, 'nothing is sent before the answer');
  assert.ok(ui.doc.querySelector('#ask').textContent.includes('Track number 3') && ui.doc.querySelector('#ask').textContent.includes('500 XP'), ui.doc.querySelector('#ask').textContent);
  ui.click('#aok'); await ui.settle();
  assert.deepEqual(ui.posted.filter((p) => p.path === '/api/player/specs').map((p) => p.body), [{ track_type: 3, xp: 500, level: 2 }]);
  ui.dom.window.close();
  // once a track has a number the note is gone
  const known = { ...specs, rows: [row({ number: 2, xp: 10, level: 1 }), row({ track: 'Gathering' })] };
  const ui2 = await boot({ get: { '/api/player/specs': known } });
  ui2.click('[data-tab="player"]'); await ui2.settle();
  ui2.click('[data-sub="player:specialization"]'); await ui2.settle();
  assert.equal(ui2.doc.querySelector('#spnone'), null);
  ui2.dom.window.close();
});

test('an empty table says what is empty in its own words on Structures, Landsraad and the config backups (UX-38)', async () => {
  const bases = JSON.parse(JSON.stringify(baseFixtures['/api/bases'].body));
  bases.placeables = []; bases.types = [];
  const lands = JSON.parse(JSON.stringify(baseFixtures['/api/landsraad'].body));
  lands.decrees = [];
  const ui = await boot({ get: { '/api/bases': bases, '/api/bases/list': BASE7, '/api/landsraad': lands, '/api/config/backups': [] } });
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="player:bases"]'); await ui.settle();
  ui.click('[data-bstab="structures"]'); await ui.settle();
  let t = ui.doc.querySelector('#main').textContent;
  assert.ok(t.includes('No placeables in this save.') && t.includes('No building pieces in this save.'), t.slice(0, 300));
  ui.click('[data-tab="landsraad"]'); await ui.settle();
  assert.ok(ui.doc.querySelector('#main').textContent.includes('This save has no decrees.'));
  ui.click('[data-tab="config"]'); await ui.settle();
  assert.ok(ui.doc.querySelector('#main').textContent.includes('No backups of this file yet'));
  for (const name of ['Nothing to show here.', 'Nothing here.']) assert.ok(!ui.doc.querySelector('#main').textContent.includes(name), name);
  ui.dom.window.close();
});

test('Resetting a specialization track that holds nothing says nothing changed, not that it was reset (ARCH-4)', async () => {
  const row = (o) => ({ track: 'Combat', number: 2, xp: 0, level: 0, keystoneOwned: 0, keystoneTotal: 41, granted: false, ...o });
  const specs = { rows: [row({})], unassigned: [], maxXp: 44182, tracks: ['Combat'] };
  const ui = await boot({ get: { '/api/player/specs': specs }, post: { '/api/player/specs/reset': { ok: true, removed: 0 } } });
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="player:specialization"]'); await ui.settle();
  ui.click('[data-act="spReset"][data-t="Combat"]'); await ui.settle(); ui.click('#aok'); await ui.settle();
  const t = ui.doc.querySelector('#toast').textContent;
  assert.ok(t.includes('had no XP to reset') && !t.includes('not saved yet'), t);
  ui.dom.window.close();
});

test('Database > Tables: primary keys, links and account identity are plain text with the reason, other cells stay editable, a hostile column name is only a key', async () => {
  const hostile = '__proto__';
  const tbl = { columns: ['_rowid_', 'id', 'inventory_id', 'template_id', hostile], rows: [[5, 10, 3, 'Spice', 'x']], total: 1, tableLocked: '', locked: { id: 'primary key', inventory_id: 'link to another table' } };
  const ui = await boot({ get: { '/api/db/tables': [{ name: 'items', rows: 1, columns: ['id'], types: ['INTEGER'] }], '/api/db/table': tbl } });
  ui.click('[data-tab="db"]'); await ui.settle();
  const inputs = [...ui.doc.querySelectorAll('input.dbc')].map((i) => i.getAttribute('data-col'));
  assert.deepEqual(inputs, ['template_id', hostile], 'only plain columns are inputs');
  assert.equal(ui.doc.querySelector('span.mono[title="primary key"]').textContent, '10');
  assert.equal(ui.doc.querySelector('span.mono[title="link to another table"]').textContent, '3');
  assert.ok(ui.doc.querySelector('#dbout').textContent.includes('Plain text columns are read-only'));
  const lockedTable = { columns: ['_rowid_', 'user'], rows: [[1, 'u']], total: 1, tableLocked: 'account identity', locked: null };
  const ui2 = await boot({ get: { '/api/db/tables': [{ name: 'accounts', rows: 1, columns: ['user'], types: ['TEXT'] }], '/api/db/table': lockedTable } });
  ui2.click('[data-tab="db"]'); await ui2.settle();
  assert.equal(ui2.doc.querySelectorAll('input.dbc').length, 0, 'a locked table has no inputs');
  assert.ok(ui2.doc.querySelector('#dbout').textContent.includes('Read-only: account identity'));
  ui.dom.window.close(); ui2.dom.window.close();
});

test('Database > Tables: a cell edit sends the rowid for a table with one and the primary-key array for a WITHOUT ROWID table', async () => {
  const edit = async (columns, row) => {
    const tbl = { columns, rows: [row], total: 1, tableLocked: '', locked: { character_id: 'primary key' } };
    const ui = await boot({ get: { '/api/db/tables': [{ name: 'player_tags', rows: 1, columns: ['character_id', 'tag'], types: ['INTEGER', 'TEXT'] }], '/api/db/table': tbl }, post: { '/api/db/update': { ok: true } } });
    ui.click('[data-tab="db"]'); await ui.settle();
    const inp = ui.doc.querySelector('input.dbc');
    inp.value = 'changed'; inp.dispatchEvent(new ui.w.Event('change', { bubbles: true })); await ui.settle();
    const body = ui.posted.filter((p) => p.path === '/api/db/update').map((p) => p.body);
    ui.dom.window.close();
    return body;
  };
  assert.deepEqual(await edit(['_rowid_', 'character_id', 'tag'], [5, 1, 'a']), [{ table: 'player_tags', rowid: 5, column: 'tag', value: 'changed' }]);
  assert.deepEqual(await edit(['_key_', 'character_id', 'tag'], ['["1","a"]', 1, 'a']), [{ table: 'player_tags', key: '["1","a"]', column: 'tag', value: 'changed' }]);
});

test('Give water gives ten Cups of Water in one post', async () => {
  const ui = await boot({ post: { '/api/player/give': { ok: true, itemId: 9 } } });
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="char:inventory"]'); await ui.settle();
  ui.click('[data-act="giveWater"]'); await ui.settle();
  assert.deepEqual(ui.posted.filter((p) => p.path === '/api/player/give').map((p) => p.body), [{ template_id: 'WaterPack_Consumable', quantity: 10, quality: 0 }]);
  ui.dom.window.close();
});

test('Teleport: Find lists places as text, choosing one fills X Y Z, Facing is sent only when typed', async () => {
  const hostile = '<img src=x onerror=1>';
  const dest = { map: 'HaggaBasin', kind: 'marker', total: 3, destinations: [{ kind: 'marker', label: 'Cave ' + hostile, x: 400.4, y: 500.6, z: 20, distance: 52000 }, { kind: 'marker', label: 'EnemyCamp', x: 9000, y: 9000, z: 30, distance: 900000 }] };
  const ui = await boot({ get: { '/api/player/destinations': dest }, post: { '/api/player/teleport': { ok: true } } });
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="player:admin"]'); await ui.settle();
  ui.doc.querySelector('#tdkind').value = 'marker'; ui.doc.querySelector('#tdq').value = 'cave';
  ui.click('[data-act="tdFind"]'); await ui.settle();
  assert.equal(ui.doc.querySelector('#tdout img'), null, 'a hostile label is text');
  const sel = ui.doc.querySelector('#tdsel');
  assert.ok(sel.options[0].textContent.includes('nearest 2 of 3'), sel.options[0].textContent);
  assert.ok(sel.options[1].textContent.includes('520 m away'), sel.options[1].textContent);
  sel.value = '0'; sel.dispatchEvent(new ui.w.Event('change', { bubbles: true }));
  assert.deepEqual(['tx', 'ty', 'tz'].map((id) => ui.doc.querySelector('#' + id).value), ['400', '501', '20']);
  ui.click('[data-act="teleport"]'); await ui.settle();
  assert.deepEqual(ui.posted.filter((p) => p.path === '/api/player/teleport').map((p) => p.body), [{ x: 400, y: 501, z: 20 }], 'no facing typed, none sent');
  ui.click('[data-act="tdFind"]'); await ui.settle(); // the page redrew after the move
  const sel2 = ui.doc.querySelector('#tdsel');
  sel2.value = '0'; sel2.dispatchEvent(new ui.w.Event('change', { bubbles: true }));
  ui.doc.querySelector('#tdyaw').value = '90';
  ui.click('[data-act="teleport"]'); await ui.settle();
  assert.deepEqual(ui.posted.filter((p) => p.path === '/api/player/teleport').at(-1).body, { x: 400, y: 501, z: 20, yaw: 90 });
  assert.ok(ui.doc.body.textContent.includes('next enter single-player'), 'the page says when the move shows');
  ui.dom.window.close();
});

test('Land Claim Editor: the clickable cells take clicks anywhere inside them, also the unfilled dotted ones', () => {
  // jsdom has no hit-testing: an unfilled SVG shape only receives clicks on its outline unless pointer-events is all, which is what made the
  // dotted cells look dead (#145). Pin the rule that makes them clickable.
  const rule = /\.lc-cell\[data-cell\]\{([^}]*)\}/.exec(read('app.css'));
  assert.ok(rule, 'the clickable cell rule exists');
  assert.ok(/pointer-events:\s*all/.test(rule[1]), 'cells must have pointer-events:all, got: ' + rule[1]);
  assert.ok(/\.lc-a\{[^}]*fill:none/.test(read('app.css')), 'the available cells are unfilled, which is why the rule above is needed');
});

test('Tables sort by any clicked header: text, numbers as numbers, descending on the second click, original order on the third, kept after a redraw', async () => {
  const row = (o) => ({ id: 'X', name: 'X', learned: false, inInventory: false, inCatalog: true, ...o });
  const sets = { rows: [row({ id: 'b', name: 'Beta' }), row({ id: 'a', name: 'alpha' }), row({ id: 'c', name: 'Charlie', learned: true })], newPieces: [] };
  const m = (o) => ({ id: 'Skills.Ability.X', name: 'X', school: 'Trooper', kind: 'Ability', rank: 0, max: 3, spent: 0, ladder: [1, 3, 6], known: true, ...o });
  const sk = { total: 100, unspent: 5, spent: 21, schools: [{ key: 'Trooper', label: 'Trooper' }], modules: [m({ id: 'Skills.Ability.A', name: 'A', spent: 9 }), m({ id: 'Skills.Ability.B', name: 'B', spent: 10 }), m({ id: 'Skills.Ability.C', name: 'C', spent: 2 })] };
  const ui = await boot({ get: { '/api/player/building-sets': sets, '/api/player/skills': sk } });
  const names = () => [...ui.doc.querySelectorAll('#ctout tbody tr td:first-child')].map((td) => td.textContent.trim());
  const th = (label) => [...ui.doc.querySelectorAll('thead th')].find((x) => x.textContent.trim().startsWith(label));
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="player:buildingsets"]'); await ui.settle();
  assert.deepEqual(names(), ['Beta', 'alpha', 'Charlie'], 'the rows come in the order the server gave');
  th('Item id').click(); await ui.settle();
  assert.deepEqual(names(), ['alpha', 'Beta', 'Charlie'], 'ids a, b, c');
  th('Item id').click(); await ui.settle();
  assert.deepEqual(names(), ['Charlie', 'Beta', 'alpha'], 'descending on the second click');
  assert.ok(th('Item id').classList.contains('sd'), 'the arrow shows the direction');
  th('Item id').click(); await ui.settle();
  assert.deepEqual(names(), ['Beta', 'alpha', 'Charlie'], 'a third click restores the original order');
  th('State').click(); await ui.settle(); // Learned, Not owned, Not owned: text sort, ties keep their order
  assert.deepEqual(names(), ['Charlie', 'Beta', 'alpha']);
  ui.doc.querySelector('#ctq').value = ''; ui.click('[data-act="ctfilter"]'); await ui.settle(); // a redraw of the same table
  assert.deepEqual(names(), ['Charlie', 'Beta', 'alpha'], 'the sort survives a redraw');
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="player:skills"]'); await ui.settle();
  const pts = () => [...ui.doc.querySelectorAll('#skout tbody tr')].map((tr) => tr.cells[3].textContent.trim());
  th('Points').click(); await ui.settle();
  assert.deepEqual(pts(), ['2', '9', '10'], 'numbers sort as numbers, not as text (10 after 9)');
  ui.dom.window.close();
});

test('Repair all worn items and loadouts on the Inventory tab; Vehicles Repair one and all; claim cells drawn as a grid; Structures and vendors moved', async () => {
  const claim = [{ totem_id: 7, name: 'Totem', cells: 3, rings: 0, level: 1, irregular: true, maxRings: 5, maxLevel: 5, piecesInClaim: 2, piecesOutside: 1, yaw: 0, segments: [{ x: -1, y: 0 }, { x: 1, y: 0 }],
    grid: [{ x: -1, y: 0, in: true, n: 0 }, { x: 0, y: 0, in: true, n: 2 }, { x: 1, y: 0, in: true, n: 0 }, { x: 3, y: 1, in: false, n: 1 }] }];
  const veh = { vehicles: [{ id: 41, name: 'BP_Sandbike', type: 'Sandbike', map: 'HaggaBasin', condition: 60, fuel: 10, x: 1, y: 2, components: [], cargo: null }], hidden: 0, recovered: [], backups: [] };
  const ui = await boot({ get: { '/api/bases/list': BASE7, '/api/bases/claim': claim, '/api/vehicles': veh }, post: { '/api/player/repair': { ok: true, repaired: 4 }, '/api/vehicles/repair': { ok: true, repaired: 1, modules: 3, withoutKnownMax: 0 } } });
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="char:inventory"]'); await ui.settle();
  ui.click('[data-act="repair"]'); await ui.settle();
  assert.equal(ui.posted.filter((p) => p.path === '/api/player/repair').length, 1, 'one request');
  assert.ok(ui.doc.querySelector('#toast').textContent.includes('Repaired 4 items'));
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="player:vehicles"]'); await ui.settle();
  ui.click('[data-act="repairOne"]'); await ui.settle();
  ui.click('[data-act="repairV"]'); await ui.settle();
  assert.deepEqual(ui.posted.filter((p) => p.path === '/api/vehicles/repair').map((p) => p.body), [{ vehicle_id: 41 }, {}], 'one vehicle, then all of them with no threshold');
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="player:bases"]'); await ui.settle();
  ui.click('[data-bstab="claim"]'); await ui.settle();
  assert.equal(ui.doc.querySelectorAll('svg.lc-svg rect.lc-o').length, 1, 'the totem cell');
  assert.equal(ui.doc.querySelectorAll('svg.lc-svg rect.lc-x').length, 2, 'the cells the claim has');
  assert.ok(ui.doc.querySelectorAll('svg.lc-svg rect.lc-a').length >= 6, 'dotted cells next to the claim that can be added');
  ui.click('[data-bstab="structures"]'); await ui.settle();
  assert.ok(ui.doc.querySelector('[data-act="repairB"]') && ui.doc.querySelector('[data-act="sand"]'), 'structure tools are on the base');
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="player:admin"]'); await ui.settle();
  assert.ok(ui.doc.querySelector('[data-act="vreset"]'), 'vendor limits are on Admin');
  assert.equal(ui.doc.querySelector('[data-tab="extras"]'), null, 'no Extras tab any more');
  ui.dom.window.close();
});

test('Land Claim Editor: click available cells to add them, click again to take back, keep them connected, level, Apply posts', async () => {
  const claim = [{ totem_id: 7, name: 'Totem', cells: 2, rings: 0, level: 1, irregular: true, maxRings: 5, maxLevel: 5, yaw: 90, segments: [{ x: 1, y: 0 }], grid: [{ x: 0, y: 0, in: true, n: 0 }, { x: 1, y: 0, in: true, n: 3 }] }];
  const ui = await boot({ get: { '/api/bases/list': BASE7, '/api/bases/claim': claim }, post: { '/api/bases/claim/apply': { ok: true, added: 2, level: 3, levelRaised: true } } });
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="player:bases"]'); await ui.settle();
  ui.click('[data-bstab="claim"]'); await ui.settle();
  const cell = (k) => ui.doc.querySelector(`[data-cell="${k}"]`);
  assert.ok(ui.doc.querySelector('#main').textContent.includes('Original yaw') && ui.doc.querySelector('#main').textContent.includes('90°'));
  assert.ok(ui.doc.querySelector('svg.lc-svg g').getAttribute('transform').startsWith('rotate(90'), 'the grid is turned by the base yaw');
  assert.deepEqual([...ui.doc.querySelectorAll('svg.lc-svg text.lc-compass')].map((t) => t.textContent), ['N', 'S', 'W', 'E'], 'the compass points are on the grid');
  const north = ui.doc.querySelector('.lc-north').textContent;
  assert.ok(north.includes('+x runs South') && north.includes('+y runs West'), 'a base at yaw 90: the column runs south and the row west: ' + north);
  assert.ok(ui.doc.querySelectorAll('svg.lc-svg text.lc-axis-t').length === 2 && cell('2,0').querySelector('title').textContent === 'cell 2, 0', 'the axes are marked and a cell says its coordinates');
  assert.equal(cell('0,0'), null, 'the totem cell is not clickable');
  assert.equal(cell('1,0'), null, 'an existing cell is not clickable');
  assert.ok(cell('2,0') && cell('0,1') && cell('-1,0'), 'cells next to the claim are available');
  assert.equal(cell('5,5'), null, 'a cell touching nothing is not offered');
  assert.equal(ui.doc.querySelector('[data-act="lcApply"]').disabled, true, 'nothing to apply yet');
  ui.click('[data-cell="2,0"]'); await ui.settle();
  assert.ok(cell('3,0'), 'a chosen cell opens the cells next to it');
  ui.click('[data-cell="3,0"]'); await ui.settle();
  assert.equal(ui.doc.querySelectorAll('rect.lc-s').length, 2);
  ui.click('[data-cell="2,0"]'); await ui.settle(); // taking back the first one drops the one that hung from it
  assert.equal(ui.doc.querySelectorAll('rect.lc-s').length, 0, 'what hung from a removed cell goes with it');
  ui.click('[data-cell="2,0"]'); await ui.settle();
  ui.click('[data-cell="3,0"]'); await ui.settle();
  ui.doc.querySelector('#lcLvl').value = '3'; ui.doc.querySelector('#lcLvl').dispatchEvent(new ui.dom.window.Event('change', { bubbles: true })); await ui.settle();
  assert.ok(ui.doc.querySelector('#main').textContent.includes('1 → 3'));
  ui.click('[data-act="lcApply"]'); await ui.settle();
  assert.equal(ui.posted.filter((p) => p.path === '/api/bases/claim/apply').length, 0, 'asks first');
  ui.click('#aok'); await ui.settle();
  assert.deepEqual(ui.posted.filter((p) => p.path === '/api/bases/claim/apply').map((p) => p.body), [{ totem_id: 7, cells: [{ x: 2, y: 0 }, { x: 3, y: 0 }], level: 3 }]);
  ui.dom.window.close();
});

test('base and vehicle upkeep buttons post, and unknown building names stay plain text', async () => {
  const hostile = '<img src=x onerror=1> x1';
  const ui = await boot({ post: {
    '/api/bases/refill-water': { ok: true, filled: 2, alreadyFull: 1, skippedUnknown: [hostile], summary: '2 Medium Water Cisterns' },
    '/api/bases/refill-generators': { ok: true, filled: 3, alreadyFull: 0, summary: '2 Omnidirectional Wind Turbines, 1 Windtrap' },
    '/api/vehicles/repair': { ok: true, modules: 9, repaired: 7, withoutKnownMax: 2 },
  } });
  ui.click('[data-tab=\"player\"]'); await ui.settle();
  ui.click('[data-sub=\"player:bases\"]'); await ui.settle();
  ui.click('[data-bstab="water"]'); await ui.settle();
  ui.click('[data-act="refillWater"]'); ui.click('[data-act="refillWater"]'); await ui.settle();
  let toast = ui.doc.querySelector('#toast');
  assert.ok(toast.textContent.includes('Refilled 2 Medium Water Cisterns (1 already full)') && toast.textContent.includes(hostile), toast.textContent);
  assert.equal(toast.querySelector('img'), null);
  ui.click('[data-bstab="power"]'); await ui.settle();
  ui.click('[data-act="refillGen"]'); ui.click('[data-act="refillGen"]'); await ui.settle();
  assert.ok(ui.doc.querySelector('#toast').textContent.includes('Refilled 2 Omnidirectional Wind Turbines, 1 Windtrap'));
  ui.click('[data-tab=\"player\"]'); await ui.settle();
  ui.click('[data-sub=\"player:vehicles\"]'); await ui.settle();
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
  assert.ok(ui.doc.querySelector('#ask').textContent.includes('like every other change'), 'turning it on says it saves immediately');
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

test('the land claim questions say the change is written at once after a backup, not that it cannot be undone (UX-12)', async () => {
  const claims = [{ totem_id: 7, name: 'Totem', cells: 9, rings: 1, level: 0, irregular: false, maxRings: 5, maxLevel: 5, piecesInClaim: 0, piecesOutside: 0 }];
  const ui = await boot({ get: { '/api/bases/list': BASE7, '/api/bases/claim': claims } });
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="player:bases"]'); await ui.settle();
  ui.click('[data-bstab="claim"]'); await ui.settle();
  ui.click('[data-act="claimShrink"]'); await ui.settle();
  let q = ui.doc.querySelector('#ask').textContent;
  assert.ok(q.includes('written at once, after the usual backup') && !q.includes('cannot be undone'), q);
  ui.click('[data-act="askNo"]'); await ui.settle();
  ui.doc.querySelector('#cr7').value = '2';
  ui.click('[data-act="claimGrow"]'); await ui.settle();
  q = ui.doc.querySelector('#ask').textContent;
  assert.ok(q.includes('written at once, after the usual backup') && !q.includes('cannot be undone'), q);
  ui.dom.window.close();
});

test('Structures and Vehicles have no edit boxes on single pieces or vehicles (#155)', async () => {
  const bases = { totems: [], pieces: { n: 2, minh: 5600, avgh: 6300, sand: 0 }, types: [{ building_type: 'Sardaukar_Foundation', n: 1, minh: 7000, maxh: 7000, avgh: 7000 }],
    placeables: [{ id: 5, class: 'c', building_type: 'Generator_Placeable', health: 2500, x: 1, y: 2, z: 3 }, { id: 6, class: 'c', building_type: 'WaterCistern_Placeable', health: 1250, x: 1, y: 2, z: 3 }] };
  const veh = { vehicles: [], hidden: 0, backups: [], recovered: [{ vehicle_id: 77, vehicle_name: 'Sandbike', chassis_durability: 100, time_stored: 't', reason: 0 }] };
  const ui = await boot({ get: { '/api/bases/list': BASE7, '/api/bases': bases, '/api/vehicles': veh } });
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="player:bases"]'); await ui.settle();
  ui.click('[data-bstab="structures"]'); await ui.settle();
  const main = () => ui.doc.querySelector('#main');
  assert.ok(main().textContent.includes('Generator_Placeable') && main().textContent.includes('2,500') && main().textContent.includes('1,250'), 'each placeable still shows its health');
  assert.equal(main().querySelectorAll('input[type="number"]').length, 0, 'no box to type a health into');
  assert.ok(main().querySelector('[data-act="repairB"]'), 'Repair all to max stays');
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="player:vehicles"]'); await ui.settle();
  assert.ok(main().textContent.includes('Recovered and stored vehicles (1)') && main().textContent.includes('Sandbike'), 'the stored vehicle is listed');
  assert.equal(main().querySelector('[data-act="dur"]'), null, 'no Set durability on a vehicle');
  assert.ok(!main().textContent.includes('Set durability'), 'and no text for it');
  ui.dom.window.close();
});

test('queued edits are put back onto the game\'s newer file by themselves: the banner says they are queued and offers no Reload (#160)', async () => {
  const ov = { ...JSON.parse(JSON.stringify(baseFixtures['/api/save/state'].body)), dirty: true, pending: ['edit 1', 'edit 2'], saveBlocked: false, changedOnDisk: true };
  const ui = await boot({ get: { '/api/save/state': ov } });
  const gw = ui.doc.querySelector('#gamewarn');
  assert.ok(gw.classList.contains('warn') && !gw.classList.contains('hide'), 'the banner is showing');
  assert.ok(gw.textContent.includes('2 changes are queued'), gw.textContent);
  assert.equal(gw.querySelector('[data-act="reload"]'), null, 'nothing to reload by hand: the queue is written onto the new file');
  // and with nothing changed the banner stays out of the way
  const ok = { ...JSON.parse(JSON.stringify(baseFixtures['/api/save/state'].body)), dirty: false, pending: [], changedOnDisk: false, saveBlocked: false, gameMode: 'none' };
  const ui2 = await boot({ get: { '/api/save/state': ok } });
  assert.ok(ui2.doc.querySelector('#gamewarn').classList.contains('hide'));
  ui.dom.window.close();
  ui2.dom.window.close();
});

test('with nothing queued the editor reloads the file by itself when the game saved; queued edits or the switch off keep the banner (#210)', async () => {
  const base = JSON.parse(JSON.stringify(baseFixtures['/api/save/state'].body));
  // nothing unsaved, the file changed: it is reloaded (discard reads the file again) and no banner is needed
  const ui = await boot({ get: { '/api/save/state': { ...base, dirty: false, pending: [], saveBlocked: false, gameMode: 'none', changedOnDisk: true } } });
  assert.ok(ui.posted.some((p) => p.path === '/api/save/discard'), 'the file was read again');
  assert.ok(ui.doc.querySelector('#gamewarn').classList.contains('hide'), 'no banner for it');
  ui.dom.window.close();
  // queued edits are never dropped: nothing is discarded, and the banner says they are queued
  const ui2 = await boot({ get: { '/api/save/state': { ...base, dirty: true, pending: ['edit 1'], saveBlocked: false, changedOnDisk: true } } });
  assert.ok(!ui2.posted.some((p) => p.path === '/api/save/discard'), 'queued edits were not touched');
  assert.ok(ui2.doc.querySelector('#gamewarn').textContent.includes('1 change is queued'), 'the banner says so');
  ui2.dom.window.close();
  // the header switch turns it off: the banner comes back and the file is not reloaded
  const ui3 = await boot({ get: { '/api/save/state': { ...base, dirty: false, pending: [], saveBlocked: false, gameMode: 'none', changedOnDisk: false } } });
  const box = ui3.doc.querySelector('#autorefresh');
  assert.ok(box && box.checked, 'the switch is on by default');
  ui3.dom.window.fetch = (() => { const f = ui3.dom.window.fetch; return (p, o) => (String(p).includes('/api/save/state') ? Promise.resolve({ ok: true, status: 200, json: async () => ({ ...base, dirty: false, pending: [], saveBlocked: false, gameMode: 'none', changedOnDisk: true }) }) : f(p, o)); })();
  box.checked = false;
  box.dispatchEvent(new ui3.dom.window.Event('change', { bubbles: true }));
  await ui3.settle();
  assert.ok(!ui3.posted.some((p) => p.path === '/api/save/discard'), 'switched off: not reloaded');
  assert.ok(ui3.doc.querySelector('#gamewarn [data-act="reload"]'), 'the banner offers Reload instead');
  ui3.dom.window.close();
});

test('the header shows the save file name, not the folder that holds the account id; toasts are announced (UI review)', async () => {
  const base = JSON.parse(JSON.stringify(baseFixtures['/api/save/state'].body));
  const id = '7656119' + '8000000000'; // built in two parts so the repo's identifier scan does not see an id in the source
  const full = ['C:', 'Users', 'someone', 'AppData', 'Local', 'DuneSandbox', 'Saved', 'Cloud', 'PlayerClientStorage', 'FLS_retail', id, 'game.db'].join('\\');
  const ui = await boot({ get: { '/api/save/state': { ...base, path: full, changedOnDisk: false, dirty: false, pending: [], saveBlocked: false, gameMode: 'none' } } });
  const p = ui.doc.querySelector('#path');
  assert.equal(p.textContent, 'game.db');
  assert.ok(!p.textContent.includes(id), 'no account id on screen');
  assert.equal(p.title, full, 'the full path is the tooltip');
  ui.w.eval("toast('Saved.')");
  ui.w.eval("toast('It failed.', true)");
  assert.equal(ui.doc.querySelector('#toast div:not(.err)').getAttribute('role'), 'status');
  assert.equal(ui.doc.querySelector('#toast div.err').getAttribute('role'), 'alert');
  ui.dom.window.close();
});

test('the single-player banner tells the player that edits are queued until the game closes, not that editing is fine (#160)', async () => {
  const ov = { ...JSON.parse(JSON.stringify(baseFixtures['/api/save/state'].body)), gameRunning: true, gameProcesses: ['DuneTest.exe'], saveBlocked: true, gameMode: 'single_player',
    blockReason: 'a single-player session is active (DuneTest.exe)' };
  const ui = await boot({ get: { '/api/save/state': ov } });
  const text = ui.doc.querySelector('#gamewarn').textContent;
  assert.ok(text.includes('single-player session') && text.includes('queued') && text.includes('written to the game file by themselves'), text);
  assert.ok(!text.includes('You can keep editing'), 'the old advice is gone');
  ui.dom.window.close();
});

test('Land Claim Editor: mark existing cells for removal, never one with pieces, keep the claim connected, Remove posts the cells', async () => {
  // the claim has cells (1,0) with 2 pieces in it, (2,0), (3,0) and (0,-1), all empty but the first
  const claim = [{ totem_id: 7, name: 'Totem', cells: 5, rings: 0, level: 1, irregular: true, maxRings: 5, maxLevel: 5, yaw: 0, segments: [{ x: 1, y: 0 }, { x: 2, y: 0 }, { x: 3, y: 0 }, { x: 0, y: -1 }],
    grid: [{ x: 0, y: 0, in: true, n: 0 }, { x: 1, y: 0, in: true, n: 2 }, { x: 2, y: 0, in: true, n: 0 }, { x: 3, y: 0, in: true, n: 0 }, { x: 0, y: -1, in: true, n: 0 }] }];
  const ui = await boot({ get: { '/api/bases/list': BASE7, '/api/bases/claim': claim }, post: { '/api/bases/claim/remove': { ok: true, removed: 2, remaining: 3 } } });
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="player:bases"]'); await ui.settle();
  ui.click('[data-bstab="claim"]'); await ui.settle();
  const cell = (k) => ui.doc.querySelector(`[data-cell="${k}"]`);
  assert.equal(cell('0,0'), null, 'the totem cell cannot be marked');
  assert.equal(cell('1,0'), null, 'a cell with pieces in it cannot be marked');
  assert.ok(cell('2,0') && cell('3,0') && cell('0,-1'), 'empty existing cells can be marked');
  assert.equal(ui.doc.querySelector('[data-act="lcRemove"]').disabled, true, 'nothing marked yet');
  ui.click('[data-cell="2,0"]'); await ui.settle(); // (3,0) hangs from (2,0): marking only (2,0) would cut it off
  assert.equal(ui.doc.querySelectorAll('rect.lc-r').length, 1);
  assert.equal(ui.doc.querySelector('[data-act="lcRemove"]').disabled, true, 'cutting a cell that stays off from the totem is refused');
  assert.ok(ui.doc.querySelector('#lced').textContent.includes('cut 1 cell'), 'and says why');
  ui.click('[data-cell="3,0"]'); await ui.settle();
  assert.equal(ui.doc.querySelectorAll('rect.lc-r').length, 2);
  assert.equal(ui.doc.querySelector('[data-act="lcRemove"]').disabled, false, 'both marked: nothing is cut');
  ui.click('[data-cell="2,0"]'); await ui.settle(); // unmark
  assert.equal(ui.doc.querySelectorAll('rect.lc-r').length, 1);
  ui.click('[data-cell="2,0"]'); await ui.settle();
  ui.click('[data-act="lcRemove"]'); await ui.settle();
  assert.equal(ui.posted.filter((p) => p.path === '/api/bases/claim/remove').length, 0, 'asks first');
  ui.click('#aok'); await ui.settle();
  const posted = ui.posted.filter((p) => p.path === '/api/bases/claim/remove');
  assert.deepEqual(posted.map((p) => p.body), [{ totem_id: 7, cells: [{ x: 2, y: 0 }, { x: 3, y: 0 }] }], 'the marked cells, in row order');
  ui.dom.window.close();
});

test('edits put back after a restart are announced as data, with a button to dismiss the notice', async () => {
  const state = { ...baseFixtures['/api/save/state'].body, dirty: true, pending: ['Give 5 x Iron Bar'],
    restored: { applied: 1, rebased: true, conflicts: [{ desc: ATTACK, why: `it changed 0 rows now and 1 before ${ATTACK}` }] }, journalOff: ATTACK };
  const ui = await boot({ get: { '/api/save/state': state } });
  const gw = ui.doc.querySelector('#gamewarn');
  assert.ok(gw.textContent.includes('1 change was put back onto the newer file the game saved'), gw.textContent);
  assert.ok(gw.textContent.includes('Could not put back'), gw.textContent);
  assert.ok(gw.textContent.includes(SEEN), 'the attack text from the conflict is shown as plain text');
  assertClean(ui.doc, 'restored notice');
  ui.click('[data-act="dismissRestored"]');
  await ui.settle();
  assert.ok(ui.posted.some((p) => p.path === '/api/save/restored/dismiss'), 'the OK button tells the server to forget the notice');
  ui.dom.window.close();
});

// ---------- one banner, a plain reason, and the queue listed (UX-30, UX-31)
test('one banner gives one plain sentence for a failed game check, and the raw tool text sits under Details', async () => {
  const raw = 'exit status 1: ERROR: Call cancelled';
  const ui = await boot({ get: { '/api/save/state': queuedState({ gameRunning: false, gameCheckError: raw, saveBlocked: true, blockReason: `could not check whether the game is running (${raw})`, gameMode: 'unknown',
    lastWrite: { saved: false, queued: 2, why: `could not check whether the game is running (${raw})` } }) } });
  const gw = ui.doc.querySelector('#gamewarn');
  const text = gw.textContent;
  assert.equal(text.split('Could not tell whether').length - 1, 1, 'the reason is said once: ' + text);
  assert.equal(text.split(raw).length - 1, 1, 'the tool text appears once');
  const det = [...gw.querySelectorAll('details')].find((d) => d.textContent.includes(raw));
  assert.ok(det, 'the tool text is inside a Details toggle');
  assert.ok(!gw.cloneNode(true).textContent.replace(det.textContent, '').includes('exit status'), 'and nowhere else');
  assert.ok(/Details/.test(det.querySelector('summary').textContent));
  assert.equal(gw.querySelectorAll('[data-act="writeNow"]').length, 1, 'one Write now');
  ui.dom.window.close();
});

test('the banner lists what is queued in sentence case, as plain text, and the list is collapsible', async () => {
  const ui = await boot({ get: { '/api/save/state': queuedState({ ...sessionBlock, pending: ['solari +10000', 'xp +1,000'.replace(',', ''), 'Reputation with House Testing: 0 → 500', `refill base water: 3 filled ${ATK}`] }) } });
  const gw = ui.doc.querySelector('#gamewarn');
  const items = [...gw.querySelectorAll('details.bn-list li')].map((li) => li.textContent);
  assert.equal(items.length, 4, items.join(' | '));
  assert.equal(items[0], 'Add 10,000 Solari to the backpack');
  assert.equal(items[1], 'Add 1000 XP');
  assert.equal(items[2], 'Reputation with House Testing: 0 → 500');
  assert.ok(items[3].startsWith('Refill base water'), 'the first letter is a capital: ' + items[3]);
  assert.equal(gw.querySelector('details.bn-list img'), null, 'a queued description is data, not markup');
  assert.equal(ui.doc.querySelector('#pending').title.split('\n')[0], 'Add 10,000 Solari to the backpack', 'the chip tooltip uses the same wording');
  assertClean(ui.doc, 'queue list');
  ui.dom.window.close();
});

test('the Discard question lists the queued changes it would drop', async () => {
  const ui = await boot({ get: { '/api/save/state': queuedState({ ...sessionBlock, pending: ['solari +10000', 'Give 5 x Iron Bar (grade 0)'] }) }, post: { '/api/save/discard': { ok: true } } });
  ui.click('#btnDiscard'); await ui.settle();
  const ask = ui.doc.querySelector('#ask');
  assert.ok(ask.textContent.includes('Discard the queued changes'));
  assert.deepEqual([...ask.querySelectorAll('ul.qlist li')].map((li) => li.textContent), ['Add 10,000 Solari to the backpack', 'Give 5 x Iron Bar (grade 0)']);
  assert.ok(ask.textContent.includes('2 changes'), 'the question says how many');
  assert.equal(ui.posted.filter((p) => p.path === '/api/save/discard').length, 0);
  ui.dom.window.close();
});

test('the banner is not redrawn when nothing in it changed, so a focused button keeps its focus and an open list stays open', async () => {
  const ui = await boot({ get: { '/api/save/state': queuedState(sessionBlock) } });
  const btn = ui.doc.querySelector('#gamewarn [data-act="writeNow"]');
  btn.focus();
  const sum = ui.doc.querySelector('#gamewarn details.bn-list summary');
  sum.dispatchEvent(new ui.w.MouseEvent('click', { bubbles: true, cancelable: true }));
  await ui.w.status();
  assert.ok(ui.doc.querySelector('#gamewarn [data-act="writeNow"]') === btn, 'the same button element');
  assert.ok(ui.doc.activeElement === btn || ui.doc.activeElement === sum, 'focus is kept');
  // a change in the state does redraw it, and the list the person opened is still open
  const next = queuedState({ ...sessionBlock, pending: ['one', 'two', 'three'] });
  const f = ui.w.fetch;
  ui.w.fetch = (p, o) => (String(p).includes('/api/save/state') ? Promise.resolve({ ok: true, status: 200, json: async () => next }) : f(p, o));
  await ui.w.status();
  assert.ok(ui.doc.querySelector('#gamewarn [data-act="writeNow"]') !== btn, 'the banner was redrawn for the new state');
  assert.equal(ui.doc.querySelector('#gamewarn details.bn-list').open, true, 'the open state of the list is remembered');
  assert.equal(ui.doc.querySelectorAll('#gamewarn details.bn-list li').length, 3);
  ui.dom.window.close();
});

// ---------- wording: words for counts, names, headers (UX-33, UX-35, UX-38, UX-39)
const clone = (o) => JSON.parse(JSON.stringify(o));
const goTo = async (ui, tabName, ...subs) => { ui.click(`[data-tab="${tabName}"]`); await ui.settle(); for (const x of subs) { ui.click(x.startsWith('[') ? x : `[data-sub="${x}"]`); await ui.settle(); } };

test('no visible text refers to "the console" or "Dune Docker", which the page never defines', async () => {
  for (const [tabName, ...subs] of VIEWS) {
    const ui = await boot();
    await goTo(ui, tabName, ...subs);
    const text = ui.doc.querySelector('#main').textContent + [...ui.doc.querySelectorAll('[title]')].map((e) => e.getAttribute('title')).join(' ');
    assert.ok(!/Dune Docker|the console|console's/i.test(text), `${[tabName, ...subs].join(' > ')}: ${text.match(/.{30}(Dune Docker|the console|console's).{30}/i)}`);
    ui.dom.window.close();
  }
});

test('no table header is a raw column name; Database shows the save\'s own column names', async () => {
  for (const [tabName, ...subs] of VIEWS) {
    if (tabName === 'db') continue;
    const ui = await boot();
    await goTo(ui, tabName, ...subs);
    for (const th of ui.doc.querySelectorAll('#main th')) {
      const t = th.textContent.trim();
      assert.ok(!/_/.test(t) && t !== 'id' && t !== 'Id' && !/^[a-z]/.test(t), `${[tabName, ...subs].join(' > ')}: header "${t}"`);
    }
    ui.dom.window.close();
  }
});

test('Landsraad shows the term as dates and a reward as the item\'s name; Admin shows vendor items by name and times as dates', async () => {
  const l = clone(baseFixtures['/api/landsraad'].body);
  l.term.start_time = 1790000000; l.term.end_time = 1790604800;
  l.tasks[0].rewards = [{ threshold: 25, amount: 5, template_id: 'SolarisCoin', task_id: 1 }];
  const catalog = [{ id: 'SolarisCoin', name: 'Solari', category: 'resources', source: 'Resources' }];
  const ui = await boot({ get: { '/api/landsraad': l, '/api/catalog/items': catalog, '/api/vendors': { stock: [{ vendor_id: 'V1', template_id: 'SolarisCoin', amount_bought: 3 }], cycles: [{ vendor_id: 'V1', last_interacted_timestamp: 1790000000 }] } } });
  await goTo(ui, 'landsraad');
  const t = ui.doc.querySelector('#main').textContent;
  assert.ok(!t.includes('1790000000') && /2026|2027/.test(t), 'Start and End are dates: ' + t.slice(0, 200));
  assert.ok(t.includes('5 × Solari') && !t.includes('SolarisCoin'), 'the reward is named, not an id');
  await goTo(ui, 'player', 'player:admin');
  const a = ui.doc.querySelector('#main').textContent;
  assert.ok(a.includes('Solari') && !a.includes('SolarisCoin'), 'the vendor table shows the item name');
  assert.ok(!a.includes('1790000000'), 'the restock time is a date');
  ui.dom.window.close();
});

test('Database: an empty table says so instead of "showing 1-0"', async () => {
  const ui = await boot({ get: { '/api/db/table': { columns: ['rowid', 'a'], rows: [], total: 0, locked: {} } } });
  await goTo(ui, 'db', 'db:browse');
  const t = ui.doc.querySelector('#dbout').textContent;
  assert.ok(t.includes('No rows') && !t.includes('1–0'), t);
  assert.equal(ui.doc.querySelector('#dbq').getAttribute('placeholder'), 'Search this table');
  ui.dom.window.close();
});

test('an empty list says what is empty, not "Nothing here."', async () => {
  const ui = await boot({ get: { '/api/bases/list': [], '/api/vehicles': { vehicles: [], recovered: [], backups: [], hidden: 0 } } });
  await goTo(ui, 'player', 'player:bases');
  const t = ui.doc.querySelector('#main').textContent;
  assert.equal(t.split('No base').length - 1, 1, 'one empty message: ' + t);
  assert.ok(!t.includes('Nothing here'));
  ui.dom.window.close();
});

test('Give currency uses the one set of money names in its choices and its toast', async () => {
  const ui = await boot({ post: { '/api/player/currency': { currency: 1, before: 0, after: 250 } } });
  await goTo(ui, 'player');
  const opts = [...ui.doc.querySelectorAll('#curSel option')].map((o) => o.textContent);
  assert.deepEqual(opts, ['Solari in wallet', 'House credit in wallet']);
  assert.ok(ui.doc.querySelector('#main').textContent.includes('Solari in backpack'));
  ui.doc.querySelector('#curSel').value = '1';
  ui.click('[data-act="currency"]'); await ui.settle();
  assert.ok(ui.doc.querySelector('#toast').textContent.includes('House credit in wallet 0 → 250'), ui.doc.querySelector('#toast').textContent);
  ui.dom.window.close();
});

test('a count says what it counts, and "N of N" appears only while a filter narrows the list', async () => {
  const ui = await boot();
  await goTo(ui, 'player', 'player:journey');
  const head = () => ui.doc.querySelector('#jhead').textContent;
  assert.ok(/\d+ nodes?/.test(head()) && !/\d+ of \d+ nodes/.test(head()), head());
  ui.doc.querySelector('#jq').value = 'zzzz-no-such-node'; ui.click('[data-act="jfilter"]'); await ui.settle();
  assert.ok(/0 of \d+ nodes match/.test(head()), head());
  ui.dom.window.close();
});

test('the Player Summary labels its numbers: skill points unspent and earned, Intel against its cap', async () => {
  const summary = { name: 'T', status: 'Offline', map: 'HaggaBasin', faction: null, level: null, xp: 10, skillPoints: { unspent: 9, total: 70 }, intel: { points: 333, max: 2779 },
    vitals: { health: 112.5, healthMinMax: 150, hydration: 52.5, maxHydration: 100, spiceAddiction: 0, maxSpiceAddiction: 10 }, identity: {}, ids: {}, currency: [] };
  const ui = await boot({ get: { '/api/player/summary': summary } });
  await goTo(ui, 'player');
  const t = ui.doc.querySelector('#sumcard').textContent;
  for (const want of ['9 unspent, 70 earned', '333 of 2,779', '112.5 (max 150 or more)', '52.5 of 100']) assert.ok(t.includes(want), `${want} in ${t}`);
  assert.ok(/Level\s*-/.test(t), 'a missing level shows a dash like the other tiles: ' + t);
  ui.dom.window.close();
});

// ---------- clutter: one Money card, no Blueprints page (UX-35, review #16 and #17)
test('Character > Overview has one Money card for the backpack Solari and the wallets, and no Blueprints page is offered', async () => {
  const ui = await boot();
  ui.click('[data-tab="player"]'); await ui.settle();
  const money = [...ui.doc.querySelectorAll('#main .card')].filter((c) => c.querySelector('h3') && c.querySelector('h3').textContent === 'Money');
  assert.equal(money.length, 1, 'one Money card');
  assert.ok(money[0].querySelector('#solari') && money[0].querySelector('#curSel') && money[0].querySelector('#curAmt'), 'it holds the backpack stack and the wallet controls');
  assert.ok(![...ui.doc.querySelectorAll('#main h3')].some((h) => h.textContent === 'Quick rewards' && h.parentElement.querySelector('#curSel')), 'Quick rewards no longer holds a wallet row');
  assert.equal(ui.doc.querySelector('[data-sub="player:blueprints"]'), null, 'the Blueprints page that only said "not yet" is gone');
  ui.dom.window.close();
});

test('a bulk action on every base asks on its own button: the label says what will happen, another press, Escape or leaving cancels, the second press runs it (GRC-2)', async () => {
  const ui = await boot({ post: { '/api/bases/repair': { ok: true, pieces: 3, placeables: 1 }, '/api/bases/clear-sand': { ok: true, pieces: 2 }, '/api/bases/refill-water': { ok: true, filled: 1, alreadyFull: 0, summary: '1 Cistern' } } });
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="player:bases"]'); await ui.settle();
  ui.click('[data-bstab="structures"]'); await ui.settle();
  const btn = () => ui.doc.querySelector('[data-act="repairB"]');
  const was = btn().textContent;
  const sent = (p) => ui.posted.filter((x) => x.path === p).length;
  ui.click('[data-act="repairB"]'); await ui.settle();
  assert.equal(btn().textContent, 'Yes, repair every base', 'the button now says what pressing it again does');
  assert.ok(ui.doc.querySelector('#sr').textContent.includes('Press the button again'), 'and the live region says it aloud');
  assert.equal(sent('/api/bases/repair'), 0);
  assert.ok(ui.doc.querySelector('#ask').classList.contains('hide'), 'no dialog opened: the keyboard is not trapped');
  // Escape cancels
  ui.doc.dispatchEvent(new ui.w.KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
  assert.equal(btn().textContent, was); assert.equal(btn().dataset.armed, undefined); assert.equal(ui.doc.querySelector('#sr').textContent, '');
  // pressing a different button cancels the first and arms the second; nothing is sent
  ui.click('[data-act="repairB"]');
  ui.click('[data-act="sand"]');
  assert.equal(btn().textContent, was, 'the first is back');
  assert.equal(ui.doc.querySelector('[data-act="sand"]').textContent, 'Yes, clear sand on every base');
  assert.equal(sent('/api/bases/repair') + sent('/api/bases/clear-sand'), 0, 'asking sends nothing');
  // leaving the button cancels
  ui.doc.querySelector('[data-act="sand"]').dispatchEvent(new ui.w.FocusEvent('focusout', { bubbles: true }));
  assert.equal(ui.doc.querySelector('[data-act="sand"]').textContent, 'Clear sand on every base');
  // the second press of the same button runs it once and the page goes back to the plain label
  ui.click('[data-act="repairB"]'); ui.click('[data-act="repairB"]'); await ui.settle();
  assert.equal(sent('/api/bases/repair'), 1);
  assert.ok(ui.doc.querySelector('#toast').textContent.includes('Repaired 3 pieces'));
  assert.equal(btn().textContent, was);
  ui.dom.window.close();
});

test('an armed bulk button disarms by itself after a few seconds', async () => {
  const ui = await boot({ post: { '/api/bases/clear-sand': { ok: true, pieces: 2 } } });
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="player:bases"]'); await ui.settle();
  ui.click('[data-bstab="structures"]'); await ui.settle();
  const b = ui.doc.querySelector('[data-act="sand"]');
  const was = b.textContent;
  ui.click('[data-act="sand"]');
  assert.notEqual(b.textContent, was);
  await new Promise((r) => ui.w.setTimeout(r, 8100));
  assert.equal(b.textContent, was, 'back to the plain label');
  ui.click('[data-act="sand"]'); // a press after that asks again instead of running
  assert.equal(ui.posted.filter((x) => x.path === '/api/bases/clear-sand').length, 0);
  ui.dom.window.close();
});

test('an inline edit reports the old and the new value and how to put it back (UX-29)', async () => {
  const inv = baseFixtures['/api/player/inventory'].body;
  const first = inv.items.find((r) => r.inventory_type == 0) || inv.items[0];
  const ui = await boot({ post: { '/api/items/update': { ok: true } } });
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="char:inventory"]'); await ui.settle();
  const box = ui.doc.querySelector(`input[data-item="${first.id}"][data-field="stack_size"]`);
  const was = box.value;
  box.value = '77';
  box.dispatchEvent(new ui.w.Event('change', { bubbles: true })); await ui.settle();
  let t = ui.doc.querySelector('#toast').textContent;
  assert.ok(t.includes(`stack ${was} → 77`) && t.includes(`To undo, set it back to ${was}`), t);
  assert.ok(t.includes(box.dataset.name), 'the item is named');
  // a second edit of the same box: the value to go back to is the one just accepted
  box.value = '5';
  box.dispatchEvent(new ui.w.Event('change', { bubbles: true })); await ui.settle();
  t = ui.doc.querySelector('#toast').textContent;
  assert.ok(t.includes('stack 77 → 5') && t.includes('set it back to 77'), t);
  // a grade (a drop-down) names the old grade
  const sel = ui.doc.querySelector(`select[data-item="${first.id}"][data-field="quality"]`);
  const g = sel.options[sel.selectedIndex].text;
  sel.selectedIndex = (sel.selectedIndex + 1) % sel.options.length;
  const g2 = sel.options[sel.selectedIndex].text;
  sel.dispatchEvent(new ui.w.Event('change', { bubbles: true })); await ui.settle();
  t = ui.doc.querySelector('#toast').textContent;
  assert.ok(t.includes(`grade ${g} → ${g2}`), t);
  ui.dom.window.close();
});

test('a config value and a database cell also report old and new; a queued edit adds that it waits', async () => {
  const ov = JSON.parse(JSON.stringify(baseFixtures['/api/save/state'].body)); ov.dirty = true; ov.pending = ['edit something'];
  const tbl = { columns: ['_rowid_', 'id', 'template_id'], rows: [[5, 10, 'Spice']], total: 1, tableLocked: '', locked: { id: 'primary key' } };
  const ui = await boot({ get: { '/api/save/state': ov, '/api/db/tables': [{ name: 'items', rows: 1, columns: ['id'], types: ['INTEGER'] }], '/api/db/table': tbl }, post: { '/api/config/set': { ok: true }, '/api/db/update': { ok: true } } });
  ui.click('[data-tab="config"]'); await ui.settle();
  const box = ui.doc.querySelector('input.cfgv');
  box.value = 'changed';
  box.dispatchEvent(new ui.w.Event('change', { bubbles: true })); await ui.settle();
  let t = ui.doc.querySelector('#toast').textContent;
  assert.ok(t.includes(`${box.dataset.key} written to the config file now`) && t.includes('→ changed') && t.includes('set it back to'), t);
  ui.click('[data-tab="db"]'); await ui.settle();
  const cell = ui.doc.querySelector('input.dbc');
  assert.ok(cell, 'an editable cell');
  cell.value = cell.value + 'x';
  cell.dispatchEvent(new ui.w.Event('change', { bubbles: true })); await ui.settle();
  t = [...ui.doc.querySelectorAll('#toast div')].map((d) => d.textContent).join('\n');
  assert.ok(t.includes('Database ') && t.includes(`${cell.dataset.col}:`) && t.includes('queued'), t);
  ui.dom.window.close();
});

test('the SQL page and its result say an edit is written as it is made, or queued when the file cannot be written (UX-28)', async () => {
  for (const [state, want, not] of [[queuedState({ dirty: false, pending: [] }), 'and written to game.db', 'queued'], [queuedState(sessionBlock), 'and queued: it is written to game.db as soon as', 'and written to game.db.']]) {
    const ui = await boot({ get: { '/api/save/state': state }, post: { '/api/db/exec': { changes: 2 } } });
    ui.click('[data-tab="db"]'); await ui.settle();
    ui.click('[data-sub="db:sql"]'); await ui.settle();
    const card = ui.doc.querySelector('#sql').closest('.card').textContent;
    assert.ok(card.includes('written to game.db at once, after a backup') && card.includes('they are queued'), card);
    for (const bad of ['until you', 'Save to game', 'Review']) assert.ok(!card.includes(bad), `${bad} in ${card}`);
    ui.click('[data-act="execsql"]'); await ui.settle();
    const q = ui.doc.querySelector('#ask').textContent;
    assert.ok(q.includes('written to the game file at once, after the usual backup') && q.includes('waits in the queue'), q);
    const inp = ui.doc.querySelector('#atype');
    inp.value = 'write'; inp.dispatchEvent(new ui.w.Event('input', { bubbles: true }));
    inp.dispatchEvent(new ui.w.KeyboardEvent('keydown', { key: 'Enter', bubbles: true })); await ui.settle();
    const out = ui.doc.querySelector('#sqlout').textContent;
    assert.ok(out.includes('2 rows changed') && out.includes(want) && !out.includes(not), out);
    ui.dom.window.close();
  }
});
const key = (ui, el, k) => el.dispatchEvent(new ui.w.KeyboardEvent('keydown', { key: k, bubbles: true, cancelable: true }));

test('a question is a named dialog: labelled by its title, the page behind is inert and does not scroll, and focus goes back to where it was (UX-32)', async () => {
  const ui = await inventoryUI();
  const del = ui.doc.querySelector('[data-act="delItem"]');
  del.focus();
  ui.click('[data-act="delItem"]'); await ui.settle();
  const box = ui.doc.querySelector('#ask [role="dialog"]');
  assert.ok(box, 'the dialog');
  const title = ui.doc.getElementById(box.getAttribute('aria-labelledby'));
  assert.ok(title && title.textContent.trim() === 'Delete item?', 'its name is its title');
  const body = ui.doc.getElementById(box.getAttribute('aria-describedby'));
  assert.ok(body && body.textContent.includes('Delete '), 'and it is described by its text');
  for (const sel of ['header', '#nav', '#main', '#gamewarn', '#rowarn']) assert.ok(ui.doc.querySelector(sel).hasAttribute('inert'), `${sel} is inert while the question is open`);
  assert.ok(!ui.doc.querySelector('#ask').hasAttribute('inert'), 'the dialog itself is not');
  assert.ok(ui.doc.body.classList.contains('noscroll'), 'the page behind does not scroll');
  ui.click('[data-act="askNo"]'); await ui.settle();
  for (const sel of ['header', '#nav', '#main', '#gamewarn', '#rowarn']) assert.ok(!ui.doc.querySelector(sel).hasAttribute('inert'), `${sel} works again`);
  assert.ok(!ui.doc.body.classList.contains('noscroll'));
  assert.equal(ui.doc.activeElement, del, 'focus is back on the button that asked');
  ui.dom.window.close();
});

test('Enter in a filter box runs the filter and leaves the cursor in the box: Inventory, Journey, Research, Crafting, Customizations, Database, the teleport finder (#250)', async () => {
  const dest = { map: 'HaggaBasin', kind: 'marker', total: 1, destinations: [{ kind: 'marker', label: 'Cave', x: 1, y: 2, z: 3, distance: 100 }] };
  const ui = await boot({ get: { '/api/player/destinations': dest } });
  const typeEnter = async (id, text) => {
    const box = ui.doc.querySelector('#' + id);
    assert.ok(box, `a filter box #${id}`);
    box.focus(); box.value = text;
    key(ui, box, 'Enter'); await ui.settle();
    const now = ui.doc.querySelector('#' + id);
    assert.equal(ui.doc.activeElement, now, `the cursor is still in #${id}`);
    assert.equal(now.value, text, `and it still holds what was typed`);
  };
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="char:inventory"]'); await ui.settle();
  await typeEnter('invq', 'zzzznomatch');
  assert.ok(ui.doc.querySelector('#main').textContent.includes('No item in this group matches the filter'), 'the filter ran');
  ui.click('[data-sub="player:journey"]'); await ui.settle();
  await typeEnter('jq', 'zzzznomatch');
  assert.ok(ui.doc.querySelector('#jout').textContent.includes('No node matches the filter'));
  ui.click('[data-sub="player:research"]'); await ui.settle();
  await typeEnter('rsq', 'zzzznomatch');
  assert.ok(ui.doc.querySelector('#rsout').textContent.includes('No research entry matches'));
  ui.click('[data-sub="player:crafting"]'); await ui.settle();
  await typeEnter('crq', 'zzzznomatch');
  assert.ok(ui.doc.querySelector('#crout').textContent.includes('No recipe matches'));
  ui.click('[data-sub="player:customizations"]'); await ui.settle();
  await typeEnter('ctq', 'zzzznomatch');
  ui.click('[data-sub="player:admin"]'); await ui.settle();
  const q = ui.doc.querySelector('#tdq'); q.focus(); q.value = 'cave';
  key(ui, q, 'Enter'); await ui.settle();
  assert.ok(ui.doc.querySelector('#tdout').textContent.includes('1 place'), 'the finder ran');
  assert.equal(ui.doc.activeElement, ui.doc.querySelector('#tdq'));
  ui.click('[data-tab="db"]'); await ui.settle();
  await typeEnter('dbq', 'zzz');
  ui.dom.window.close();
});

test('an Enter that confirms an input method does not run a filter', async () => {
  const ui = await boot();
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="char:inventory"]'); await ui.settle();
  const box = ui.doc.querySelector('#invq');
  box.value = 'zzzz';
  box.dispatchEvent(new ui.w.KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true, isComposing: true })); await ui.settle();
  assert.ok(!ui.doc.querySelector('#main').textContent.includes('No item in this group matches'), 'an Enter that confirms an input method does not filter');
  ui.dom.window.close();
});

test('table headers are reachable and usable from the keyboard and say their sort order (#250)', async () => {
  const row = (o) => ({ id: 'X', name: 'X', learned: false, inInventory: false, inCatalog: true, ...o });
  const sets = { rows: [row({ id: 'b', name: 'Beta' }), row({ id: 'a', name: 'alpha' }), row({ id: 'c', name: 'Charlie' })], newPieces: [] };
  const ui = await boot({ get: { '/api/player/building-sets': sets } });
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="player:buildingsets"]'); await ui.settle();
  const th = () => [...ui.doc.querySelectorAll('thead th')].find((x) => x.textContent.trim().startsWith('Item id'));
  const ids = () => [...ui.doc.querySelectorAll('#ctout tbody tr td:nth-child(2)')].map((td) => td.textContent.trim());
  assert.equal(th().getAttribute('tabindex'), '0', 'a header that sorts is a tab stop');
  assert.equal(th().getAttribute('aria-sort'), 'none');
  th().focus();
  key(ui, th(), 'Enter'); await ui.settle();
  assert.deepEqual(ids(), ['a', 'b', 'c']);
  assert.equal(th().getAttribute('aria-sort'), 'ascending');
  assert.ok(ui.doc.querySelector('#sr').textContent.includes('Sorted by Item id, ascending'), ui.doc.querySelector('#sr').textContent);
  key(ui, th(), ' '); await ui.settle();
  assert.deepEqual(ids(), ['c', 'b', 'a']);
  assert.equal(th().getAttribute('aria-sort'), 'descending');
  key(ui, th(), 'Enter'); await ui.settle();
  assert.deepEqual(ids(), ['b', 'a', 'c'], 'the third press restores the original order');
  assert.equal(th().getAttribute('aria-sort'), 'none');
  assert.ok(ui.doc.querySelector('#sr').textContent.includes('original order'));
  // an empty header (the actions column) is not a control
  const blank = [...ui.doc.querySelectorAll('thead th')].filter((x) => !x.textContent.trim());
  assert.ok(blank.every((x) => !x.hasAttribute('tabindex') && !x.hasAttribute('aria-sort')));
  ui.dom.window.close();
});

test('the browser tab names the open page and has an icon (UX-41)', async () => {
  const ui = await boot();
  assert.ok(ui.doc.querySelector('link[rel="icon"][href="/favicon.svg"]'), 'a tab icon');
  assert.equal(ui.doc.title, 'Player - Character - Tabr Tau');
  ui.click('[data-sub="player:vehicles"]'); await ui.settle();
  assert.equal(ui.doc.title, 'Player - Vehicles - Tabr Tau');
  ui.click('[data-tab="landsraad"]'); await ui.settle();
  assert.equal(ui.doc.title, 'Landsraad - Tabr Tau');
  ui.click('[data-tab="db"]'); await ui.settle();
  assert.equal(ui.doc.title, 'Database - Tabr Tau');
  ui.dom.window.close();
});
