// The term and decrees panel of the Landsraad tab (part 4 of the tab rework; refs #448, #608, #634): the decree pool tells the decree in effect apart from the pool flag and the five
// special vendor decrees apart from each other, decrees are shown by a readable name with the game's internal id as a small second line, and the controls write what they say.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import { boot, closeAll } from './boot.mjs';

afterEach(closeAll);

// the decree rows and the term of a save read in the game: term 10 has decree 8 (SpecialVendorActive_Vehicles) active, and the rotation held 1, 10 and 4
const NAMES = ['ExperienceRateIncrease', 'RangedDamageIncreased', 'MeleeDamageIncreased', 'CraftingCostReduced', 'DropInventoryOnDefeatActive', 'RepairAndRefiningTimes',
  'SpecialVendorActive', 'SpecialVendorActive_Vehicles', 'SpecialVendorActive_Weapons', 'SpecialVendorActive_Armor', 'SpecialVendorActive_Utilities'];
const FLAG = [1, 1, 1, 1, 0, 1, 0, 1, 1, 1, 1];
const decrees = () => NAMES.map((n, i) => ({ id: i + 1, decree_name: n, disabled: FLAG[i], weight: 1, active: i + 1 === 8, elected: i + 1 === 5, in_rotation: [1, 10, 4].includes(i + 1) }));

const landsraad = (over = {}) => ({
  term: { term_id: 10, start_time: 1, end_time: 2, reigning_faction_id: 2, active_decree_id: 8, elected_decree_id: 5 },
  factions: [{ id: 1, name: 'Atreides' }, { id: 2, name: 'Harkonnen' }],
  character: { player_id: 1, name: 'Tester', faction_id: 1 },
  decrees: decrees(), rotation: [{ decree_id: 1 }, { decree_id: 10 }, { decree_id: 4 }], tasks: [],
  ...over,
});

async function open(data = landsraad(), post = {}) {
  const ui = await boot({ get: { '/api/landsraad': data }, post: { '/api/landsraad/decree': { ok: true, outcome: 'written' }, '/api/landsraad/term': { ok: true, outcome: 'written' }, ...post } });
  ui.click('[data-tab="landsraad"]'); await ui.settle();
  return ui;
}

const poolRow = (ui, id) => ui.doc.querySelector(`#lsb-tdp .dp [data-lb="tflag"][data-id="${id}"]`).closest('li');
const text = (el) => el.textContent.replace(/\s+/g, ' ').trim();
const parts = (el) => [...el.children].map(text).join(' ');
const posted = (ui, path) => ui.posted.filter((p) => p.path === path);
const select = (ui, sel, v) => { const el = ui.doc.querySelector(sel); el.value = String(v); el.dispatchEvent(new ui.w.Event('change', { bubbles: true })); };

test('the panel is closed, its heading line names the term, the reigning faction and the active decree, and the facts are in it', async () => {
  const ui = await open();
  const d = ui.doc.querySelector('#lsb-tdp');
  assert.ok(d && !d.open, 'closed by default');
  assert.equal(text(d.querySelector('summary')), 'Term and decrees Term 10 · Harkonnen · Special Vendor: Vehicles');
  const kv = text(d.querySelector('.tkv'));
  assert.ok(kv.includes('Term10') && kv.includes('Reigning factionHarkonnen') && kv.includes('Active decreeSpecial Vendor: VehiclesSpecialVendorActive_Vehicles') && kv.includes('Elected decreeDrop Inventory on DefeatDropInventoryOnDefeatActive'), kv);
  assert.ok(!ui.doc.querySelector('.lsb-prev'), 'the old section of previous controls is gone');
  // the window is on the play clock, drawn in seconds with a thousands separator as the mock-up does
  const ui2 = await open(landsraad({ term: { term_id: 10, start_time: 360000, end_time: 396000, reigning_faction_id: 2, active_decree_id: 8, elected_decree_id: 5 } }));
  const kv2 = text(ui2.doc.querySelector('.tkv'));
  assert.ok(kv2.includes('Start360,000 s') && kv2.includes('End396,000 s'), kv2);
});

test('decrees are shown by a readable name with the internal id as a small second line; the Refining Contract carries the game\'s own name', async () => {
  const ui = await open();
  const got = (id) => parts(poolRow(ui, id).querySelector('.dc'));
  assert.equal(got(1), 'Experience Rate Increase ExperienceRateIncrease');
  assert.equal(got(5), 'Drop Inventory on Defeat DropInventoryOnDefeatActive');
  assert.equal(got(6), 'CHOAM Refining Contract RepairAndRefiningTimes');
  assert.equal(got(7), 'Special Vendor SpecialVendorActive');
  assert.equal(got(8), 'Special Vendor: Vehicles SpecialVendorActive_Vehicles');
  assert.ok(poolRow(ui, 6).querySelector('.dn').title.includes('75%'));
  assert.equal(poolRow(ui, 8).querySelector('.did').title, "The game's internal id, kept for support");
});

test('only the decree the term has active is marked Active in the pool, and it is the Vehicles variant, not SpecialVendorActive', async () => {
  const ui = await open();
  const tags = (id) => [...poolRow(ui, id).querySelectorAll('.tag')].map((t) => t.textContent.trim());
  assert.deepEqual(tags(8), ['Active']);
  assert.deepEqual(tags(5), ['Elected']);
  assert.deepEqual(tags(10), ['In rotation']);
  assert.deepEqual(tags(7), [], 'the base row SpecialVendorActive is not in this term: ' + text(poolRow(ui, 7)));
  assert.ok(text(poolRow(ui, 7)).includes('not in this term'));
});

test('the pool flag is shown as the stored number, never as enabled or disabled, and the About text says it does not mean in effect', async () => {
  const ui = await open();
  const pool = text(ui.doc.querySelector('#lsb-tdp .dp'));
  assert.ok(!/\benabled\b/i.test(pool) && !/\bdisabled\b/i.test(pool), pool);
  assert.equal(text(poolRow(ui, 7).querySelector('.fl')), '0');
  assert.equal(text(poolRow(ui, 8).querySelector('.fl')), '1');
  assert.ok(/does not say whether a decree is in effect/.test(ui.doc.querySelector('#lsdecnote').textContent));
  assert.ok(ui.doc.querySelector('#lsdecnote').closest('details.about'), 'the long text is in an About disclosure');
});

test('all five special vendor rows are listed under their own names with their own ids', async () => {
  const ui = await open();
  const got = [7, 8, 9, 10, 11].map((id) => text(poolRow(ui, id).querySelector('.did')));
  assert.deepEqual(got, ['SpecialVendorActive', 'SpecialVendorActive_Vehicles', 'SpecialVendorActive_Weapons', 'SpecialVendorActive_Armor', 'SpecialVendorActive_Utilities']);
});

test('the flag button of one row sends that row only', async () => {
  const ui = await open();
  assert.match(text(poolRow(ui, 7).querySelector('[data-lb="tflag"]')), /Set flag to 1/);
  assert.match(text(poolRow(ui, 8).querySelector('[data-lb="tflag"]')), /Set flag to 0/);
  ui.click('#lsb-tdp [data-lb="tflag"][data-id="7"]'); await ui.settle();
  const sent = posted(ui, '/api/landsraad/decree');
  assert.equal(sent.length, 1);
  assert.deepEqual(sent[0].body, { id: 7, disabled: true });
  assert.deepEqual(posted(ui, '/api/landsraad/term'), [], 'the flag does not touch the term');
});

test('the reigning faction drop-down starts on the character\'s own faction; Apply sends the two drop-downs in one request', async () => {
  const ui = await open();
  assert.equal(ui.doc.querySelector('#lsb-trf').value, '1', 'the character is in Atreides');
  assert.ok(text(ui.doc.querySelector('#lsb-trf')).includes('Atreides (you)'));
  assert.equal(ui.doc.querySelector('#lsb-tad').value, '8');
  const panel = ui.doc.querySelector('#lsb-tdp'); panel.open = true; panel.dispatchEvent(new ui.w.Event('toggle'));
  select(ui, '#lsb-tad', 6);
  assert.equal(text(ui.doc.querySelector('#lsb-tad').closest('label').querySelector('.did')), 'RepairAndRefiningTimes', 'the id line follows the choice');
  select(ui, '#lsb-trf', 2);
  ui.click('#lsb-tdp [data-lb="tapply"]'); await ui.settle();
  const sent = posted(ui, '/api/landsraad/term');
  assert.equal(sent.length, 1);
  assert.deepEqual(sent[0].body, { active_decree_id: '6', reigning_faction_id: '2' });
  assert.ok(ui.doc.querySelector('#lsb-tdp').open, 'the panel stays open after the write');
  assert.ok(text(ui.doc.querySelector('#lsb-tresult')).includes('Applied: active decree CHOAM Refining Contract, reigning faction Harkonnen'), text(ui.doc.querySelector('#lsb-tresult')));
});

test('none clears a field: the empty choices are sent as empty', async () => {
  const ui = await open();
  select(ui, '#lsb-tad', ''); select(ui, '#lsb-trf', '');
  ui.click('#lsb-tdp [data-lb="tapply"]'); await ui.settle();
  assert.deepEqual(posted(ui, '/api/landsraad/term')[0].body, { active_decree_id: '', reigning_faction_id: '' });
});

test('Make active on a pool row sends the same request as Apply for that decree and the chosen faction; the row that is already active shows Active', async () => {
  const ui = await open(landsraad({ term: { term_id: 10, start_time: 1, end_time: 2, reigning_faction_id: 1, active_decree_id: 8, elected_decree_id: 5 } }));
  assert.ok(text(poolRow(ui, 8)).includes('✓ Active'), 'decree 8 is active for Atreides, the faction the drop-down starts on');
  assert.ok(!poolRow(ui, 8).querySelector('[data-lb="tmk"]'));
  const b = poolRow(ui, 9).querySelector('[data-lb="tmk"]');
  assert.equal(text(b), 'Make active for Atreides');
  ui.click('#lsb-tdp [data-lb="tmk"][data-id="9"]'); await ui.settle();
  assert.deepEqual(posted(ui, '/api/landsraad/term').map((p) => p.body), [{ active_decree_id: '9', reigning_faction_id: '1' }]);
});

test('Make active follows the faction drop-down, and with none chosen it is disabled and sends nothing', async () => {
  const ui = await open();
  select(ui, '#lsb-trf', 2);
  assert.equal(text(poolRow(ui, 9).querySelector('[data-lb="tmk"]')), 'Make active for Harkonnen');
  assert.ok(text(poolRow(ui, 8)).includes('✓ Active'), 'decree 8 is active for Harkonnen, the faction the term has');
  select(ui, '#lsb-trf', '');
  const b = poolRow(ui, 9).querySelector('[data-lb="tmk"]');
  assert.equal(b.getAttribute('aria-disabled'), 'true');
  ui.click('#lsb-tdp [data-lb="tmk"][data-id="9"]'); await ui.settle();
  assert.deepEqual(posted(ui, '/api/landsraad/term'), []);
  assert.ok(text(ui.doc.querySelector('#lsb-tresult')).includes('Choose a reigning faction'));
});

test('a save without a term says so, and a hostile decree name is shown as text', async () => {
  let ui = await open(landsraad({ term: null }));
  assert.ok(text(ui.doc.querySelector('#lsb-term-h')).includes('no Landsraad term'));
  const evil = decrees(); evil[0].decree_name = '<img src=x onerror=alert(1)>';
  ui = await open(landsraad({ decrees: evil }));
  assert.ok(!ui.doc.querySelector('#lsb-tdp img'), 'no element was made from the name');
  assert.ok(text(poolRow(ui, 1)).includes('<img src=x onerror=alert(1)>'));
});
