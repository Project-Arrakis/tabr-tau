// The Care package area on Player > Character (#417): the kits kept in the editor's settings, made, changed, duplicated, deleted, exported and imported. Every
// change is an immediate settings change (nothing is queued, nothing touches the save), and a refusal leaves the page as it was.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { boot, closeAll } from './boot.mjs';

afterEach(closeAll);

const LIMITS = { kits: 20, items: 50, name: 60, quantity: 1000000, xp: 100000000, skillPoints: 100000, fileBytes: 65536 };
const STARTER = { name: 'Starter', items: [{ item: 'SpicedFuelCell', quantity: 100 }, { item: 'Combat_Heavy_Unique_Reinforced_Helmet_06', quantity: 1, grade: 3, augments: ['T6_Augment_Armor10'], augmentGrade: 4 }], xp: 5000, skillPoints: 3, problem: '' };
const LEVELS = { name: 'Levels', items: [], xp: 900, skillPoints: 0, problem: '' };
const KITS = (kits, problem = '') => ({ kits, limits: LIMITS, problem });
const open = async (ui) => { ui.click('[data-tab="player"]'); await ui.settle(); ui.click('[data-sub="char:carepackage"]'); await ui.settle(); };
const posts = (ui, p) => ui.posted.filter((x) => x.path === p);
const set = (ui, sel, v) => { const el = ui.doc.querySelector(sel); assert.ok(el, sel); el.value = v; return el; };
const change = (ui, sel, v) => { const el = set(ui, sel, v); el.dispatchEvent(new ui.w.Event('change', { bubbles: true })); };

test('Character has a Care package tab; it lists the kits with what is in them and the actions on each', async () => {
  const ui = await boot({ get: { '/api/kits': KITS([STARTER, LEVELS]) } });
  await open(ui);
  assert.ok(ui.fetched.includes('/api/kits'));
  assert.equal(ui.doc.querySelector('[data-sub="char:carepackage"]').getAttribute('aria-current'), 'true');
  const rows = [...ui.doc.querySelectorAll('table tbody tr')].map((r) => r.textContent.replace(/\s+/g, ' ').trim());
  assert.equal(rows.length, 2);
  assert.ok(rows[0].includes('Starter') && rows[0].includes('100 x Spice-infused Fuel Cell') && rows[0].includes('(grade 3)') && rows[0].includes('5,000') && rows[0].includes('Delete'), rows[0]);
  assert.ok(rows[1].includes('Levels') && rows[1].includes('No items') && rows[1].includes('900'), rows[1]);
  for (const a of ['kitEdit', 'kitDup', 'kitExport', 'kitDel']) assert.equal(ui.doc.querySelectorAll(`table [data-act="${a}"]`).length, 2, a);
  assert.ok(ui.doc.querySelector('[data-act="kitNew"]') && ui.doc.querySelector('#kitFile') && ui.doc.querySelector('[data-act="kitExport"][data-name=""]'));
  assert.equal(ui.doc.querySelector('#kitName'), null, 'no editor until a kit is made or changed');
  ui.dom.window.close();
});

test('with no kits the tab says so, and Export all is off', async () => {
  const ui = await boot({ get: { '/api/kits': KITS([]) } });
  await open(ui);
  assert.ok(ui.doc.querySelector('#main').textContent.includes('No kits yet.'));
  assert.equal(ui.doc.querySelector('[data-act="kitExport"][data-name=""]').disabled, true);
  ui.dom.window.close();
});

test('a new kit: add items with a grade and augments, change a quantity, save: one post of the whole kit, no queue hint', async () => {
  const ui = await boot({
    get: { '/api/kits': KITS([]) },
    post: { '/api/kits/save': { ok: true, saved: 'Armor set', ...KITS([]) }, '/api/items/augment-options': { limit: 2, options: [{ id: 'T6_Augment_Armor10', name: 'Armor 10', effects: [] }, { id: 'T6_Augment_Armor16', name: 'Armor 16', effects: [] }] } },
  });
  await open(ui);
  ui.click('[data-act="kitNew"]'); await ui.settle();
  assert.ok(ui.doc.querySelector('#kitName') && ui.doc.querySelector('#kitItem') && ui.doc.querySelector('#kitXp'));
  assert.equal(ui.doc.querySelector('[data-act="kitNew"]').disabled, true, 'one kit is edited at a time');
  assert.equal(ui.doc.querySelector('#kitFile').disabled, true);
  set(ui, '#kitName', '  Armor set ');
  // an item that is not one is refused with the picker's own words, and nothing is added
  set(ui, '#kitItem', 'Spice');
  ui.click('[data-act="kitAddLine"]'); await ui.settle();
  assert.ok(ui.doc.querySelector('#toast').textContent.includes('is not an item'), ui.doc.querySelector('#toast').textContent);
  assert.equal(ui.doc.querySelector('#kitName').value, '  Armor set ', 'what was typed stays');
  // a helmet with an augment
  change(ui, '#kitItem', 'Combat_Heavy_Unique_Reinforced_Helmet_06'); await ui.settle();
  assert.ok(ui.doc.querySelector('#ka0') && ui.doc.querySelector('#kag'), 'the augment slots of the item appear');
  set(ui, '#ka0', 'T6_Augment_Armor10'); set(ui, '#kag', '4'); set(ui, '#kitGrade', '3');
  ui.click('[data-act="kitAddLine"]'); await ui.settle();
  // fuel cells, 100
  set(ui, '#kitItem', 'SpicedFuelCell'); set(ui, '#kitQty', '100');
  ui.click('[data-act="kitAddLine"]'); await ui.settle();
  assert.equal(ui.doc.querySelector('#kitName').value, '  Armor set ', 'the name survives the page being drawn again');
  assert.equal(ui.doc.querySelectorAll('[data-kitqty]').length, 2);
  set(ui, '[data-kitqty="1"]', '250');
  set(ui, '#kitXp', '5000'); set(ui, '#kitSp', '3');
  ui.click('[data-act="kitSave"]'); await ui.settle();
  const p = posts(ui, '/api/kits/save');
  assert.equal(p.length, 1);
  assert.deepEqual(p[0].body, { was: '', kit: { name: '  Armor set ', xp: 5000, skillPoints: 3, items: [
    { item: 'Combat_Heavy_Unique_Reinforced_Helmet_06', quantity: 1, grade: 3, augments: ['T6_Augment_Armor10'], augmentGrade: 4 },
    { item: 'SpicedFuelCell', quantity: 250, grade: 0 }] } });
  const toast = ui.doc.querySelector('#toast').textContent;
  assert.ok(toast.includes('Saved kit "Armor set"') && !toast.includes('queued'), toast);
  assert.equal(ui.doc.querySelector('#kitName'), null, 'the editor closes once the kit is saved');
  assert.deepEqual(ui.posted.map((x) => x.path).filter((x) => x !== '/api/items/augment-options'), ['/api/kits/save'], 'nothing else was posted');
  ui.dom.window.close();
});

test('a kit the editor refuses stays in the editor with the reason', async () => {
  const ui = await boot({ get: { '/api/kits': KITS([]) }, post: { '/api/kits/save': { __fail: 'Spice-infused Fuel Cell: the quantity must be a whole number from 1 to 1000000', __code: 'bad_argument', __status: 422 } } });
  await open(ui);
  ui.click('[data-act="kitNew"]'); await ui.settle();
  set(ui, '#kitName', 'Bad'); set(ui, '#kitItem', 'SpicedFuelCell'); set(ui, '#kitQty', '5');
  ui.click('[data-act="kitAddLine"]'); await ui.settle();
  ui.click('[data-act="kitSave"]'); await ui.settle();
  assert.ok(ui.doc.querySelector('#toast .err').textContent.includes('the quantity must be'));
  assert.equal(ui.doc.querySelector('#kitName').value, 'Bad');
  assert.equal(ui.doc.querySelectorAll('[data-kitqty]').length, 1, 'the draft is still there');
  ui.click('[data-act="kitCancel"]'); await ui.settle();
  assert.equal(ui.doc.querySelector('#kitName'), null);
  assert.equal(ui.doc.querySelector('[data-act="kitNew"]').disabled, false);
  ui.dom.window.close();
});

test('Edit fills the editor from the kit and saves it under the name it had; Remove takes a line out', async () => {
  const ui = await boot({ get: { '/api/kits': KITS([STARTER, LEVELS]) }, post: { '/api/kits/save': { ok: true, saved: 'Starter 2', ...KITS([STARTER]) } } });
  await open(ui);
  ui.click('table [data-act="kitEdit"][data-name="Starter"]'); await ui.settle();
  assert.equal(ui.doc.querySelector('#kitName').value, 'Starter');
  assert.equal(ui.doc.querySelector('#kitXp').value, '5000');
  assert.equal(ui.doc.querySelector('#kitSp').value, '3');
  assert.equal(ui.doc.querySelectorAll('[data-kitqty]').length, 2);
  assert.ok(ui.doc.querySelector('h3') && [...ui.doc.querySelectorAll('h3')].some((h) => h.textContent === 'Edit kit: Starter'));
  assert.ok([...ui.doc.querySelectorAll('table [data-act="kitEdit"]')].every((b) => b.disabled), 'the other kits cannot be edited meanwhile');
  ui.click('[data-act="kitDelLine"][data-id="0"]'); await ui.settle();
  set(ui, '#kitName', 'Starter 2');
  ui.click('[data-act="kitSave"]'); await ui.settle();
  const b = posts(ui, '/api/kits/save')[0].body;
  assert.equal(b.was, 'Starter');
  assert.equal(b.kit.name, 'Starter 2');
  assert.deepEqual(b.kit.items.map((l) => l.item), ['Combat_Heavy_Unique_Reinforced_Helmet_06']);
  assert.deepEqual(b.kit.items[0].augments, ['T6_Augment_Armor10']);
  ui.dom.window.close();
});

test('Duplicate posts the name; Delete asks first and posts only after OK', async () => {
  const ui = await boot({ get: { '/api/kits': KITS([STARTER, LEVELS]) }, post: { '/api/kits/duplicate': { ok: true, saved: 'Levels (copy)', ...KITS([STARTER, LEVELS]) }, '/api/kits/delete': { ok: true, deleted: 'Levels', ...KITS([STARTER]) } } });
  await open(ui);
  ui.click('table [data-act="kitDup"][data-name="Levels"]'); await ui.settle();
  assert.deepEqual(posts(ui, '/api/kits/duplicate').map((x) => x.body), [{ name: 'Levels' }]);
  assert.ok(ui.doc.querySelector('#toast').textContent.includes('Made "Levels (copy)"'));
  ui.click('table [data-act="kitDel"][data-name="Levels"]'); await ui.settle();
  assert.equal(posts(ui, '/api/kits/delete').length, 0, 'asked first');
  assert.ok(ui.doc.querySelector('#ask').textContent.includes('Delete the kit "Levels"'));
  ui.click('[data-act="askNo"]'); await ui.settle();
  assert.equal(posts(ui, '/api/kits/delete').length, 0, 'Cancel deletes nothing');
  ui.click('table [data-act="kitDel"][data-name="Levels"]'); await ui.settle();
  ui.click('#aok'); await ui.settle();
  assert.deepEqual(posts(ui, '/api/kits/delete').map((x) => x.body), [{ name: 'Levels' }]);
  assert.ok(ui.doc.querySelector('#toast').textContent.includes('Deleted kit "Levels"'));
  ui.dom.window.close();
});

test('Export downloads the kit file the editor answered, named after the kit with nothing unsafe in the name; Export all asks for every kit', async () => {
  const FILE = { format: 'tabr-tau-kits', version: 1, kits: [{ name: 'Starter', items: [], xp: 1 }] };
  const ui = await boot({ get: { '/api/kits': KITS([{ ...STARTER, name: '../Evil kit' }, LEVELS]), '/api/kits/export': FILE } });
  const blobs = [], names = [];
  ui.w.URL.createObjectURL = (b) => { blobs.push(b); return 'blob:x'; };
  ui.w.URL.revokeObjectURL = () => {};
  ui.w.HTMLAnchorElement.prototype.click = function () { names.push(this.download); };
  await open(ui);
  ui.click('table [data-act="kitExport"][data-name="../Evil kit"]'); await ui.settle();
  ui.click('[data-act="kitExport"][data-name=""]'); await ui.settle();
  assert.ok(ui.fetched.includes('/api/kits/export'));
  assert.deepEqual(names, ['tabr-tau-kit-.._Evil_kit.json', 'tabr-tau-kits.json']);
  assert.deepEqual(JSON.parse(await blobs[0].text()), FILE);
  assert.ok(!names[0].includes('/') && !names[0].includes('\\'));
  ui.dom.window.close();
});

test('Import reads the chosen file, sends its text for the editor to check, and says which names were taken', async () => {
  const ui = await boot({ get: { '/api/kits': KITS([STARTER]) }, post: { '/api/kits/import': { ok: true, imported: 2, renamed: 1, ...KITS([STARTER]) } } });
  await open(ui);
  const text = '{"format":"tabr-tau-kits","version":1,"kits":[]}';
  const input = ui.doc.querySelector('#kitFile');
  Object.defineProperty(input, 'files', { value: [{ size: text.length, text: async () => text }], configurable: true });
  input.dispatchEvent(new ui.w.Event('change', { bubbles: true })); await ui.settle();
  assert.deepEqual(posts(ui, '/api/kits/import').map((x) => x.body), [{ text }]);
  const t = ui.doc.querySelector('#toast').textContent;
  assert.ok(t.includes('Imported 2 kits') && t.includes('1 name was taken') && t.includes('(imported)'), t);
  ui.dom.window.close();
});

test('Import does not read a file past the size limit and shows the editor\'s refusal of a bad file', async () => {
  let ui = await boot({ get: { '/api/kits': KITS([]) } });
  await open(ui);
  let input = ui.doc.querySelector('#kitFile');
  let read = false;
  Object.defineProperty(input, 'files', { value: [{ size: 70000, text: async () => { read = true; return ''; } }], configurable: true });
  input.dispatchEvent(new ui.w.Event('change', { bubbles: true })); await ui.settle();
  assert.equal(read, false);
  assert.equal(posts(ui, '/api/kits/import').length, 0);
  assert.ok(ui.doc.querySelector('#toast .err').textContent.includes('larger than 64 KiB'));
  ui.dom.window.close();

  ui = await boot({ get: { '/api/kits': KITS([]) }, post: { '/api/kits/import': { __fail: 'kit 1 (A): item 1: "Nope" is not an item in the editor\'s catalog: pick an item from the list', __code: 'bad_argument', __status: 422 } } });
  await open(ui);
  input = ui.doc.querySelector('#kitFile');
  Object.defineProperty(input, 'files', { value: [{ size: 10, text: async () => 'nonsense' }], configurable: true });
  input.dispatchEvent(new ui.w.Event('change', { bubbles: true })); await ui.settle();
  assert.ok(ui.doc.querySelector('#toast .err').textContent.includes('is not an item in the editor'));
  ui.dom.window.close();
});

test('a kit the editor says cannot be used shows why, and a damaged settings file is shown above the list', async () => {
  const ui = await boot({ get: { '/api/kits': KITS([{ ...LEVELS, problem: '"Nope" is not an item in the editor\'s catalog' }], 'The settings file settings.json is damaged') } });
  await open(ui);
  const t = ui.doc.querySelector('#main').textContent;
  assert.ok(t.includes('Cannot be used: "Nope" is not an item'), t);
  assert.ok(t.includes('settings.json is damaged'));
  ui.dom.window.close();
});

test('kit names and items from a hand-edited settings file are shown as text, never as markup', async () => {
  const bad = '"><img src=x onerror=window.__pwned=1>';
  const ui = await boot({ get: { '/api/kits': KITS([{ name: bad, items: [{ item: bad, quantity: 1, augments: [bad] }], xp: 1, skillPoints: 0, problem: bad }]) } });
  await open(ui);
  assert.equal(ui.doc.querySelector('#main img'), null);
  assert.ok(ui.doc.querySelector('#main').textContent.includes(bad));
  assert.equal(ui.w.__pwned, undefined);
  ui.click('table [data-act="kitEdit"]'); await ui.settle();
  assert.equal(ui.doc.querySelector('#main img'), null);
  assert.equal(ui.doc.querySelector('#kitName').value, bad);
  ui.dom.window.close();
});

test('every field of the Care package tab has an accessible name, with the editor and the augment slots open', async () => {
  const ui = await boot({ get: { '/api/kits': KITS([STARTER]) }, post: { '/api/items/augment-options': { limit: 2, options: [{ id: 'T6_Augment_Armor10', name: 'Armor 10', effects: [] }] } } });
  await open(ui);
  ui.click('table [data-act="kitEdit"]'); await ui.settle();
  change(ui, '#kitItem', 'Combat_Heavy_Unique_Reinforced_Helmet_06'); await ui.settle();
  assert.ok(ui.doc.querySelector('#ka0'));
  const named = (el) => !!(el.getAttribute('aria-label') || el.getAttribute('aria-labelledby') || el.closest('label'));
  const bad = [...ui.doc.querySelectorAll('#main input, #main select, #main textarea')].filter((el) => !named(el)).map((el) => el.outerHTML.slice(0, 100));
  assert.deepEqual(bad, []);
  const unnamedButtons = [...ui.doc.querySelectorAll('#main button')].filter((b) => !b.textContent.trim() && !b.getAttribute('aria-label'));
  assert.equal(unnamedButtons.length, 0);
  ui.dom.window.close();
});

// Apply kit (slice 3). The editor's answers are the shape of internal/ops/kit.go.
const WHOLE = { ok: true, kit: 'Starter', kitHash: '0123456789abcdef', dryRun: false, complete: true, changed: true, requested: 1001, given: 1001, notGiven: [], volumeNote: '',
  lines: [{ template_id: 'SpicedFuelCell', name: 'Spice-infused Fuel Cell', requested: 1000, given: 1000, reason: '', limit: '' }, { template_id: 'Combat_Heavy_Unique_Reinforced_Helmet_06', name: 'Bulwark Helmet', requested: 1, given: 1, reason: '', limit: '' }],
  xp: { requested: 215, applied: 215, levelBefore: 0, levelAfter: 2, capped: false }, skillPoints: { requested: 3, applied: 3, capped: false } };
const PARTIAL = { ...WHOLE, complete: false, requested: 2001, given: 1500, notGiven: [{ name: 'Spice-infused Fuel Cell', requested: 2000, short: 500, reason: 'the backpack is full' }],
  lines: [{ template_id: 'SpicedFuelCell', name: 'Spice-infused Fuel Cell', requested: 2000, given: 1500, reason: 'the backpack is full', limit: 'slots' }, { template_id: 'Combat_Heavy_Unique_Reinforced_Helmet_06', name: 'Bulwark Helmet', requested: 1, given: 0, reason: 'the backpack is full', limit: 'slots' }] };
const fixtureState = () => JSON.parse(fs.readFileSync(path.join(path.dirname(fileURLToPath(import.meta.url)), 'fixtures', 'ui-fixtures.json'), 'utf8'))['/api/save/state'].body;
const applyUI = (preview, apply, kits = [STARTER]) => boot({ get: { '/api/kits': KITS(kits) }, post: { '/api/kits/preview': { ...preview, dryRun: true }, '/api/kits/apply': apply } });

test('Apply shows the dry run first, applies nothing on Cancel, and applies once on OK', async () => {
  const ui = await applyUI(WHOLE, { ...WHOLE, outcome: 'written' });
  await open(ui);
  ui.click('table [data-act="kitApply"][data-name="Starter"]'); await ui.settle();
  assert.deepEqual(posts(ui, '/api/kits/preview').map((x) => x.body), [{ name: 'Starter' }]);
  assert.equal(posts(ui, '/api/kits/apply').length, 0, 'nothing is applied before the person says so');
  const q = ui.doc.querySelector('#ask');
  const items = [...q.querySelectorAll('li')].map((l) => l.textContent);
  assert.deepEqual(items, ['1,000 x Spice-infused Fuel Cell', '1 x Bulwark Helmet', '+215 XP, level 0 to 2', '+3 skill points']);
  assert.ok(q.textContent.includes('Apply kit "Starter" to the character?') && q.textContent.includes('written at once, after a backup'), q.textContent);
  ui.click('[data-act="askNo"]'); await ui.settle();
  assert.equal(posts(ui, '/api/kits/apply').length, 0, 'Cancel applies nothing');
  ui.click('table [data-act="kitApply"]'); await ui.settle();
  await ui.wait(450); // the gap the question keeps against a double click
  ui.click('#aok'); await ui.settle();
  assert.deepEqual(posts(ui, '/api/kits/apply').map((x) => x.body), [{ name: 'Starter', kitHash: '0123456789abcdef' }], 'the apply carries the hash of the kit the question was about');
  const t = ui.doc.querySelector('#toast');
  assert.ok(t.textContent.includes('Applied kit "Starter": 1,000 x Spice-infused Fuel Cell') && !t.textContent.includes('queued') && !t.querySelector('.part'), t.textContent);
  // a second time says that it was applied before
  ui.click('table [data-act="kitApply"]'); await ui.settle();
  assert.ok(ui.doc.querySelector('#ask').textContent.includes('You applied this kit earlier in this session'));
  ui.dom.window.close();
});

test('a dry run that does not fit says what would not, and the result after OK is a warning that stays', async () => {
  const ui = await applyUI(PARTIAL, { ...PARTIAL, outcome: 'written' });
  await open(ui);
  ui.click('table [data-act="kitApply"]'); await ui.settle();
  const q = ui.doc.querySelector('#ask');
  assert.ok(q.textContent.includes('and what would not fit'));
  assert.ok([...q.querySelectorAll('li')].map((l) => l.textContent).includes('1,500 of 2,000 x Spice-infused Fuel Cell: 500 did not fit (the backpack is full)'), q.textContent);
  assert.ok([...q.querySelectorAll('li')].map((l) => l.textContent).includes('0 of 1 x Bulwark Helmet: 1 did not fit (the backpack is full)'));
  await ui.wait(450);
  ui.click('#aok'); await ui.settle();
  const warn = ui.doc.querySelector('#toast .part');
  assert.ok(warn && warn.textContent.includes('not all of it fitted') && warn.textContent.includes('500 did not fit'), ui.doc.querySelector('#toast').textContent);
  ui.dom.window.close();
});

test('while the game runs the question says the edit is queued, and the result carries the queued hint', async () => {
  const ui = await boot({ get: { '/api/kits': KITS([STARTER]), '/api/save/state': { ...fixtureState(), saveBlocked: true, blockReason: 'The game is running.' } }, post: { '/api/kits/preview': { ...WHOLE, dryRun: true }, '/api/kits/apply': { ...WHOLE, outcome: 'queued' } } });
  await open(ui);
  ui.click('table [data-act="kitApply"]'); await ui.settle();
  assert.ok(ui.doc.querySelector('#ask').textContent.includes('the edit is queued and written as soon as it can be'), ui.doc.querySelector('#ask').textContent);
  await ui.wait(450);
  ui.click('#aok'); await ui.settle();
  const t = ui.doc.querySelector('#toast').textContent;
  assert.ok(t.includes('Applied kit "Starter"') && t.includes('queued: written as soon as the save can be written'), t);
  ui.dom.window.close();
});

test('Apply is off for a kit that cannot be used; a refused preview asks nothing; a kit that would change nothing says so', async () => {
  let ui = await applyUI(WHOLE, WHOLE, [{ ...STARTER, problem: 'x is not an item' }]);
  await open(ui);
  assert.equal(ui.doc.querySelector('[data-act="kitApply"]').disabled, true);
  ui.dom.window.close();

  ui = await boot({ get: { '/api/kits': KITS([STARTER]) }, post: { '/api/kits/preview': { __fail: 'Nothing was given: the backpack is full.', __code: 'no_room', __status: 409 } } });
  await open(ui);
  ui.click('table [data-act="kitApply"]'); await ui.settle();
  assert.equal(ui.doc.querySelector('#ask').className, 'hide', 'no question');
  assert.ok(ui.doc.querySelector('#toast .err').textContent.includes('the backpack is full'));
  assert.equal(posts(ui, '/api/kits/apply').length, 0);
  ui.dom.window.close();

  ui = await applyUI({ ...PARTIAL, changed: false, given: 0, xp: undefined, skillPoints: undefined, lines: [PARTIAL.lines[0]].map((l) => ({ ...l, given: 0 })) }, WHOLE);
  await open(ui);
  ui.click('table [data-act="kitApply"]'); await ui.settle();
  assert.ok(ui.doc.querySelector('#toast').textContent.includes('Nothing would change'));
  assert.equal(posts(ui, '/api/kits/apply').length, 0);
  ui.dom.window.close();
});

// CR-3: the kit changed between the question and the answer: the editor refuses (409) and the page says so, with no "Applied" and no further request.
test('a kit that changed after the question is not applied and the refusal is shown', async () => {
  const ui = await boot({ get: { '/api/kits': KITS([STARTER]) }, post: { '/api/kits/preview': { ...WHOLE, dryRun: true },
    '/api/kits/apply': { __fail: 'The kit "Starter" has changed since you were asked. Nothing was applied. Look at the kit and apply it again.', __code: 'conflict', __status: 409 } } });
  await open(ui);
  ui.click('table [data-act="kitApply"]'); await ui.settle();
  await ui.wait(450);
  ui.click('#aok'); await ui.settle();
  const t = ui.doc.querySelector('#toast');
  assert.ok(t.querySelector('.err') && t.textContent.includes('has changed since you were asked') && !t.textContent.includes('Applied kit'), t.textContent);
  assert.equal(posts(ui, '/api/kits/apply').length, 1);
  ui.dom.window.close();
});
