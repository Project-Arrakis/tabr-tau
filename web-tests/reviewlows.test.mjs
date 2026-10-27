// The low findings CR-4, CR-5 and CR-8 of the code review of 2026-10-27 (#443): the blood-sack caveat is in the result of the fill, the reason a control is off is text that the
// control is described by (a disabled button cannot take the focus, so a reason only in its title is never reached from the keyboard), and a Landsraad board with no term row
// is answered in words.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import { boot, closeAll } from './boot.mjs';

afterEach(closeAll);

const CAVEAT = 'The fill of blood sacks has not been checked in the game yet.';
const posts = (ui, p) => ui.posted.filter((x) => x.path === p);

// The reason a control is off, as a keyboard or screen-reader user reaches it: the control is disabled, names the text that says why, and that text is on the page and not empty.
function reason(ui, el) {
  assert.ok(el, 'no such control');
  assert.equal(el.disabled, true, 'the control is off');
  const ids = (el.getAttribute('aria-describedby') || '').split(/\s+/).filter(Boolean);
  assert.ok(ids.length, 'a disabled control names the text that says why: ' + el.outerHTML);
  const text = ids.map((id) => { const n = ui.doc.getElementById(id); assert.ok(n, `aria-describedby names ${id}, which is not on the page`); return n.textContent.trim(); }).join(' ');
  assert.ok(text.length > 5, 'the reason is words: ' + text);
  return text;
}

// ---------- CR-4

test('CR-4: the result of a blood-sack fill says the fill has not been checked in the game; a fill that changed nothing does not', async () => {
  const ui = await boot({ get: { '/api/player/blood-sacks': { total: 2, notFull: 1, unknown: 0, holdingOther: 0 } },
    post: { '/api/player/fill-blood-sacks': { ok: true, filled: 1, byTemplate: { Bloodsack_01: 1 }, alreadyFull: 1, holdingOther: 0, unknown: 0, skippedUnknown: [] } } });
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-act="fillBlood"]'); await ui.settle();
  assert.equal(posts(ui, '/api/player/fill-blood-sacks').length, 1, 'the action does not ask first');
  const t = ui.doc.querySelector('#toast').textContent;
  assert.ok(t.includes('Filled 1 blood sack') && t.includes(CAVEAT), t);
  ui.dom.window.close();

  const none = await boot({ get: { '/api/player/blood-sacks': { total: 2, notFull: 1, unknown: 0, holdingOther: 0 } },
    post: { '/api/player/fill-blood-sacks': { ok: true, filled: 0, byTemplate: {}, alreadyFull: 2, holdingOther: 0, unknown: 0, skippedUnknown: [] } } });
  none.click('[data-tab="player"]'); await none.settle();
  none.click('[data-act="fillBlood"]'); await none.settle();
  assert.ok(!none.doc.querySelector('#toast').textContent.includes('has not been checked'), 'nothing was written, so there is nothing unproven to say');
  none.dom.window.close();
});

test('CR-4: Fill all liquids says it too when it filled a blood sack, and not when it filled only water', async () => {
  const get = { '/api/player/liquids': { water: { notFull: 1 }, bloodSacks: { notFull: 1 }, vehicles: { notFull: 0 } } };
  const ui = await boot({ get, post: { '/api/player/fill-all-liquids': { ok: true, vehiclesIncluded: true, water: { filled: 1 }, bloodSacks: { filled: 1 }, vehicles: { filled: 0 } } } });
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-act="fillLiquids"]'); await ui.settle();
  assert.ok(ui.doc.querySelector('#toast').textContent.includes(CAVEAT), ui.doc.querySelector('#toast').textContent);
  ui.dom.window.close();

  const water = await boot({ get, post: { '/api/player/fill-all-liquids': { ok: true, vehiclesIncluded: true, water: { filled: 2 }, bloodSacks: { filled: 0 }, vehicles: { filled: 0 } } } });
  water.click('[data-tab="player"]'); await water.settle();
  water.click('[data-act="fillLiquids"]'); await water.settle();
  assert.ok(water.doc.querySelector('#toast').textContent.includes('Filled 2 water containers') && !water.doc.querySelector('#toast').textContent.includes(CAVEAT), water.doc.querySelector('#toast').textContent);
  water.dom.window.close();
});

// ---------- CR-5

const LIMITS = { kits: 2, items: 50, name: 60, quantity: 1000000, xp: 100000000, skillPoints: 100000, fileBytes: 65536 };
const kit = (name, extra = {}) => ({ name, items: [{ item: 'SpicedFuelCell', quantity: 10 }], xp: 0, skillPoints: 0, problem: '', ...extra });
const openKits = async (ui) => { ui.click('[data-tab="player"]'); await ui.settle(); ui.click('[data-sub="char:carepackage"]'); await ui.settle(); };

test('CR-5: on the Care package tab, New kit, Import, Edit and Apply say why they are off in text they are described by', async () => {
  // the most kits are kept; one kit cannot be used
  let ui = await boot({ get: { '/api/kits': { kits: [kit('Alpha'), kit('Beta', { problem: 'x is not an item' })], limits: LIMITS, problem: '' } } });
  await openKits(ui);
  assert.ok(reason(ui, ui.doc.querySelector('[data-act="kitNew"]')).includes('The most kits are kept'));
  assert.ok(reason(ui, ui.doc.querySelector('[data-act="kitApply"][data-name="Beta"]')).includes('x is not an item'));
  assert.equal(ui.doc.querySelector('[data-act="kitApply"][data-name="Alpha"]').getAttribute('aria-describedby'), null, 'a control that is on is not described by a reason');
  ui.dom.window.close();

  // a kit is being edited
  ui = await boot({ get: { '/api/kits': { kits: [kit('Alpha')], limits: LIMITS, problem: '' } } });
  await openKits(ui);
  assert.equal(ui.doc.querySelector('#kitediting'), null);
  assert.equal(ui.doc.querySelector('[data-act="kitNew"]').getAttribute('aria-describedby'), null);
  ui.click('table [data-act="kitEdit"][data-name="Alpha"]'); await ui.settle();
  assert.ok(reason(ui, ui.doc.querySelector('[data-act="kitNew"]')).includes('Finish or cancel the kit you are editing'));
  assert.ok(reason(ui, ui.doc.querySelector('table [data-act="kitEdit"]')).includes('Finish or cancel'));
  assert.ok(reason(ui, ui.doc.querySelector('#kitFile')).includes('Finish or cancel'));
  ui.dom.window.close();
});

test('CR-5: a specialization track whose number is not known says so beside its controls, and all three are described by it', async () => {
  const row = (o) => ({ track: 'Combat', number: null, xp: 0, level: 0, keystoneOwned: 0, keystoneTotal: 41, granted: false, ...o });
  const specs = { rows: [row({}), row({ track: 'Gathering', number: 2, xp: 500, level: 3 })], unassigned: [], maxXp: 44182, tracks: ['Combat', 'Gathering'] };
  const ui = await boot({ get: { '/api/player/specs': specs } });
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="player:specialization"]'); await ui.settle();
  for (const a of ['spAdd', 'spMax', 'spReset']) assert.ok(reason(ui, ui.doc.querySelector(`[data-act="${a}"][data-t="Combat"]`)).includes('Number not known yet'), a);
  assert.ok(reason(ui, ui.doc.querySelector('#spxCombat')).includes('Number not known yet'));
  for (const a of ['spAdd', 'spMax', 'spReset']) assert.equal(ui.doc.querySelector(`[data-act="${a}"][data-t="Gathering"]`).getAttribute('aria-describedby'), null, 'a known track: nothing is off');
  assert.equal(ui.doc.querySelector('#spnumGathering'), null);
  ui.dom.window.close();
});

test('CR-5: a stored vehicle says in text why Bring to me and Duplicate are off', async () => {
  const veh = (id, extra) => ({ id, name: 'BP_Sandbike_CHOAM', type: 'Sandbike', map: 'HaggaBasin', condition: 60, fuel: 10, x: 1, y: 2, components: [], cargo: null, ...extra });
  const ui = await boot({ get: { '/api/vehicles': { vehicles: [veh(1), veh(2, { stored: 'backup', storedLabel: 'In the vehicle backup' })], hidden: 0, recovered: [], backups: [] }, '/api/vehicles/fuel': { total: 2, notFull: 0, unknown: 0, noFuelRecord: 0 } } });
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="player:vehicles"]'); await ui.settle();
  assert.ok(reason(ui, ui.doc.querySelector('[data-act="bring"][data-id="2"]')).includes('take it out of storage in the game first'));
  assert.ok(reason(ui, ui.doc.querySelector('[data-act="dupV"][data-id="2"]')).includes('take it out of storage'));
  for (const a of ['bring', 'dupV']) assert.equal(ui.doc.querySelector(`[data-act="${a}"][data-id="1"]`).getAttribute('aria-describedby'), null);
  ui.dom.window.close();
});

test('CR-5: a research group marker and a config Reset that is off say why in text', async () => {
  const rr = (o) => ({ itemKey: 'RCP_X', name: 'X', category: 'Combat', productGroup: 'Iron Products', type: 'Recipe', state: 'NotPurchased', isNew: false, unlockKind: 'recipe', unlockId: 'X', purchased: false, materialized: false, unlocked: false, needsRepair: false, actionable: true, ...o });
  const research = { rows: [rr({ itemKey: 'RCP_Rifle', name: 'Rifle' }), rr({ itemKey: 'DA_GRP W.1', name: 'Water', category: 'Water Discipline', type: 'Group', actionable: false, unlockKind: 'group', unlockId: '' })] };
  let ui = await boot({ get: { '/api/player/research': research } });
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="player:research"]'); await ui.settle();
  assert.ok(reason(ui, ui.doc.querySelector('#rsout button[disabled]')).includes('group marker'));
  ui.dom.window.close();

  const S = '/Script/DuneSandbox.UserServerCustomSettings';
  const file = { name: 'ServerCustomSettings.ini', text: '', gameRunning: false,
    entries: [{ line: 1, section: S, key: 'DifficultyLevel', value: 'Custom' }, { line: 4, section: S, key: 'LootRespawnSpeed', value: '1.000000' }],
    defaults: [{ line: 1, key: 'DifficultyLevel', known: false, atDefault: false }, { line: 4, key: 'LootRespawnSpeed', known: true, default: '1.000000', atDefault: true, min: 0.1, max: 10 }],
    resetCounts: { change: 0, atDefault: 1, noDefault: 1, otherParts: 0 } };
  ui = await boot({ get: { '/api/config/files': [{ name: 'ServerCustomSettings.ini', empty: false }], '/api/config/file': file, '/api/config/backups': [] } });
  ui.click('[data-tab="config"]'); await ui.settle();
  const rowOf = (key) => [...ui.doc.querySelectorAll('table tbody tr')].find((tr) => tr.children[1] && tr.children[1].textContent.trim() === key);
  assert.ok(reason(ui, rowOf('LootRespawnSpeed').querySelector('button')).includes('Already the default'));
  assert.ok(reason(ui, rowOf('DifficultyLevel').querySelector('button')).includes('default unknown'));
  ui.dom.window.close();
});

// ---------- CR-8

test('CR-8: a Landsraad board with tasks and no term row is answered in words, before any question, with no raw script error', async () => {
  const TIERS = [1200, 2400, 3600, 4800, 6000];
  const task = { id: 900, board_index: 0, house_name: 'DA_HouseAlexin', goal_amount: 4000, completed: 0, contributions: [], player_contributions: [], my_contribution: 0, my_recorded: false,
    rewards: TIERS.map((threshold) => ({ threshold, amount: 1, template_id: 'Item.A', reached: false, delivered: false })) };
  const board = { term: null, factions: [{ id: 1, name: 'Atreides' }], character: { player_id: 1, name: 'Tester', faction_id: 1 }, decrees: [], rotation: [], tasks: [task] };
  const ui = await boot({ get: { '/api/landsraad': board }, post: { '/api/landsraad/mine-many': { ok: true, changed: 1 } } });
  ui.freeze();
  ui.click('[data-tab="landsraad"]'); await ui.settle();
  const box = ui.doc.querySelector('[data-lshouse="900"]');
  assert.ok(box, 'the board is drawn without a term');
  box.checked = true; box.dispatchEvent(new ui.w.Event('change', { bubbles: true }));
  ui.click('[data-act="lsFill"]'); await ui.settle();
  const t = ui.doc.querySelector('#toast');
  assert.ok(t.textContent.includes('no Landsraad term') && t.querySelector('.err') && !/null|undefined|Cannot read/.test(t.textContent), t.textContent);
  assert.equal(ui.doc.querySelector('#ask [role="dialog"]'), null, 'no question is asked');
  assert.equal(posts(ui, '/api/landsraad/mine-many').length, 0);
  ui.dom.window.close();
});
