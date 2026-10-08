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
const TAGS = new Set('html head body meta title link script header nav main div span b a p h1 h3 ul li code table thead tbody tr th td input select option datalist button details summary textarea br'.split(' '));
const SCRIPTS = new Set(['/html.js', '/app.js']); // the only scripts the page may contain, both same-origin files
const ATTRS = new Set('lang charset name content rel href class id type value min max step placeholder size list selected disabled rows spellcheck colspan title src download role aria-modal tabindex label'.split(' '));

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
  w.eval(read('app.js'));
  const settle = async () => { for (let i = 0; i < 40; i++) { await new Promise((r) => w.setTimeout(r, 0)); if (inflight === 0 && i > 4) return; } };
  await settle();
  return { dom, w, doc: w.document, settle, posted, click: (sel) => { const el = w.document.querySelector(sel); assert.ok(el, `no element for ${sel}`); el.dispatchEvent(new w.MouseEvent('click', { bubbles: true, cancelable: true })); } };
}

const VIEWS = [
  ['player', 'player:overview'], ['player', 'player:inventory'], ['player', 'player:progress'], ['player', 'player:journey'], ['player', 'player:recipes'],
  ['bases', 'bases:overview'], ['bases', 'bases:storage'], ['bases', 'bases:parts'],
  ['vehicles'], ['exchange'], ['landsraad'], ['config'],
  ['db', 'db:browse'], ['db', 'db:sql'], ['db', 'db:backups'],
];

for (const confused of [false, true]) for (const [tabName, sub] of VIEWS) {
  test(`${confused ? 'type-confused numbers' : 'hostile data'} stay data: ${tabName}${sub ? ' / ' + sub.split(':')[1] : ''}`, async () => {
    const ui = await boot({ confused });
    ui.click(`[data-tab="${tabName}"]`);
    await ui.settle();
    if (sub) { ui.click(`[data-sub="${sub}"]`); await ui.settle(); }
    assertClean(ui.doc, `${tabName} ${sub || ''}`);
    const main = ui.doc.querySelector('#main').textContent;
    if (!confused) assert.ok(!main.includes('TypeError') && !main.includes('is not a function') && !main.includes('Cannot read'), `UI threw while rendering: ${main.slice(0, 200)}`);
    if (!confused && !['db:sql', 'db:backups'].includes(sub) && tabName !== 'vehicles') {
      assert.ok(haystack(ui.doc).includes(SEEN), `${tabName} ${sub || ''}: the hostile text should be visible as plain data, but it is not in the rendered page at all (fixture or view not exercised)`);
    }
    ui.dom.window.close();
  });
}

test('interactions that render results also stay clean (storage inventory, journey filter, SQL result)', async () => {
  const hostile = `"><img src=x ${MARK}>`;
  const ui = await boot({ post: { '/api/db/sql': { columns: [hostile], rows: [[hostile], [hostile + "'"]] }, '/api/db/exec': { changes: 3 } } });
  ui.click('[data-tab="bases"]'); await ui.settle();
  ui.click('[data-sub="bases:storage"]'); await ui.settle();
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
  ui.click('[data-tab="bases"]'); await ui.settle();
  ui.click('[data-sub="bases:storage"]'); await ui.settle();
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
  ui.click('[data-tab="bases"]'); await ui.settle();
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
  ui.click('[data-tab="bases"]'); await ui.settle();
  ui.click('[data-sub="bases:storage"]'); await ui.settle();
  ui.click('[data-act="openInv"]'); await ui.settle();
  ui.click('[data-act="delItem"]'); await ui.settle();
  assertClean(ui.doc, 'hostile ask body');
  assert.ok(ui.doc.querySelector('#ask').textContent.includes(MARK));
  ui.dom.window.close();
});

test('typing the word completes the action; Enter in the box does too; the disabled button does nothing', async () => {
  const ui = await boot();
  ui.click('[data-tab="bases"]'); await ui.settle();
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
  ui.click('[data-tab="bases"]'); await ui.settle();
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
  ui.click('[data-sub="player:inventory"]'); await ui.settle();
  ui.click('[data-act="refill"]'); await ui.settle();
  const toast = ui.doc.querySelector('#toast');
  assert.ok(toast.textContent.includes('Filled 3 containers (1 already full)'), toast.textContent);
  assert.ok(toast.textContent.includes(hostile), 'the name is shown as text');
  assert.equal(toast.querySelector('img'), null, 'and never becomes markup');
  assert.equal(ui.posted.filter((p) => p.path === '/api/player/refill').length, 1);
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
  ui.click('[data-sub="player:inventory"]'); await ui.settle();
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
