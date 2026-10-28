// The Landsraad decree pool tells the decree in effect apart from the pool flag, and the five special vendor decrees apart from each other (#448).
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import { boot, closeAll } from './boot.mjs';

afterEach(closeAll);

// the decree rows and the term of a save read in the game: term 10 has decree 8 (SpecialVendorActive_Vehicles) active, and the rotation held 1, 10 and 4
const NAMES = ['ExperienceRateIncrease', 'RangedDamageIncreased', 'MeleeDamageIncreased', 'CraftingCostReduced', 'DropInventoryOnDefeatActive', 'RepairAndRefiningTimes',
  'SpecialVendorActive', 'SpecialVendorActive_Vehicles', 'SpecialVendorActive_Weapons', 'SpecialVendorActive_Armor', 'SpecialVendorActive_Utilities'];
const FLAG = [1, 1, 1, 1, 0, 1, 0, 1, 1, 1, 1];
const decrees = () => NAMES.map((n, i) => ({ id: i + 1, decree_name: n, disabled: FLAG[i], weight: 1, active: i + 1 === 8, elected: false, in_rotation: [1, 10, 4].includes(i + 1) }));

const landsraad = () => ({
  term: { term_id: 10, start_time: 1, end_time: 2, reigning_faction_id: 2, active_decree_id: 8, elected_decree_id: null },
  factions: [{ id: 1, name: 'Atreides' }, { id: 2, name: 'Harkonnen' }],
  character: { player_id: 1, name: 'Tester', faction_id: 2 },
  decrees: decrees(), rotation: [{ decree_id: 1 }, { decree_id: 10 }, { decree_id: 4 }], tasks: [],
});

async function open() {
  const ui = await boot({ get: { '/api/landsraad': landsraad() }, post: { '/api/landsraad/decree': { ok: true, outcome: 'written' } } });
  ui.click('[data-tab="landsraad"]'); await ui.settle();
  return ui;
}

const poolRow = (ui, id) => ui.doc.querySelector(`[data-act="decree"][data-id="${id}"]`).closest('tr');

test('the Active decree line names SpecialVendorActive_Vehicles in full', async () => {
  const ui = await open();
  const text = ui.doc.querySelector('#main').textContent;
  assert.ok(/Active decree\s*SpecialVendorActive_Vehicles/.test(text), text.slice(0, 400));
});

test('only the decree the term has active is marked Active in the pool, and it is the Vehicles variant, not SpecialVendorActive', async () => {
  const ui = await open();
  const tagged = [...ui.doc.querySelectorAll('[data-act="decree"]')].map((b) => b.closest('tr')).filter((tr) => tr.textContent.includes('Active') && tr.querySelector('.tag'));
  const names = tagged.filter((tr) => [...tr.querySelectorAll('.tag')].some((t) => t.textContent.trim() === 'Active')).map((tr) => tr.children[1].textContent.trim());
  assert.deepEqual(names, ['SpecialVendorActive_Vehicles']);
  assert.ok(!poolRow(ui, 7).querySelector('.tag'), 'the base row SpecialVendorActive is not in this term: ' + poolRow(ui, 7).textContent);
});

test('all five special vendor rows are listed under their own names with their own ids', async () => {
  const ui = await open();
  const got = [7, 8, 9, 10, 11].map((id) => poolRow(ui, id).children[1].textContent.trim());
  assert.deepEqual(got, ['SpecialVendorActive', 'SpecialVendorActive_Vehicles', 'SpecialVendorActive_Weapons', 'SpecialVendorActive_Armor', 'SpecialVendorActive_Utilities']);
});

test('the rotation shows as In rotation, and the pool flag is shown as the stored number, never as enabled or disabled', async () => {
  const ui = await open();
  const heads = [...ui.doc.querySelectorAll('#main th')].map((t) => t.textContent.trim());
  assert.ok(heads.includes('In this term') && heads.includes('Pool flag (disabled)'), heads.join(' | '));
  assert.ok(poolRow(ui, 10).textContent.includes('In rotation'));
  assert.ok(!poolRow(ui, 9).textContent.includes('In rotation'));
  const pool = poolRow(ui, 7).closest('table').textContent;
  assert.ok(!/\benabled\b/i.test(pool) && !/\bdisabled\b(?! *\))/.test(pool.replace('Pool flag (disabled)', '')), 'no row claims enabled or disabled: ' + pool);
  assert.equal(poolRow(ui, 7).children[3].textContent.trim(), '0');
  assert.equal(poolRow(ui, 8).children[3].textContent.trim(), '1');
  assert.ok(/does not say whether a decree is in effect/.test(ui.doc.querySelector('#lsdecnote').textContent));
});

test('the flag button of one row sends that row only', async () => {
  const ui = await open();
  assert.match(poolRow(ui, 7).querySelector('[data-act="decree"]').textContent, /Set flag to 1/);
  ui.click('[data-act="decree"][data-id="7"]'); await ui.settle();
  const sent = ui.posted.filter((p) => p.path === '/api/landsraad/decree');
  assert.equal(sent.length, 1);
  assert.deepEqual(sent[0].body, { id: 7, disabled: true });
});
