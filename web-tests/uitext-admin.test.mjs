// The text sweep (#426) on Player > Admin. See uitext.test.mjs for the checks that hold on every page.
import test from 'node:test';
import assert from 'node:assert/strict';
import { boot, closeAll } from './boot.mjs';
import { visibleText, movedText } from './uitext.mjs';

test.afterEach(() => closeAll());

const open = async (ui) => { ui.click('[data-tab="player"]'); await ui.settle(); ui.click('[data-sub="player:admin"]'); await ui.settle(); return ui.doc.querySelector('#main'); };

test('Admin: the card that only pointed to other pages is gone; the explanations are tooltips or in the Movement card\'s About this', async () => {
  const ui = await boot({});
  const main = await open(ui);
  const shown = visibleText(main), moved = movedText(main);
  assert.ok(!shown.includes('Faction, teleport and the vendor purchase limits') && !moved.includes('Faction, teleport and the vendor purchase limits'), 'deleted for good');
  assert.ok([...ui.doc.querySelectorAll('h3')].every((h) => h.textContent !== 'Admin actions'));
  for (const s of ['Changes which faction the character belongs to', 'Choosing a place fills X, Y and Z', 'Facing is optional: 0 looks along +X', 'stay above the terrain or you may fall through',
    'Places are on the map chosen in Map', 'Vendors limit how much you can buy per restock cycle', 'The game keeps its own copy of the save in memory while single-player runs']) {
    assert.ok(!shown.includes(s), 'still shown: ' + s);
    assert.ok(moved.includes(s), 'lost, not moved: ' + s);
  }
  for (const s of ['Faction assignment', 'Change faction', 'Teleport to', 'Teleport', 'Find', 'Reset all vendors', 'Open Character > Inventory', 'Open Vehicles']) assert.ok(shown.includes(s), 'missing from the default view: ' + s);
  for (const id of ['tztip', 'tyawtip', 'tmaptip', 'findtip', 'factip', 'vendtip']) assert.ok(ui.doc.getElementById(id).classList.contains('tipbox'), id);
  assert.equal(ui.doc.querySelector('#tz').getAttribute('aria-describedby'), 'tztip');
  ui.dom.window.close();
});

test('Admin: the warning about a move between maps is drawn only while another map is chosen, and goes when "This map" is chosen again', async () => {
  const ui = await boot({});
  await open(ui);
  assert.equal(ui.doc.querySelector('#tmapwarn'), null, 'this map: no warning');
  const sel = ui.doc.querySelector('#tmap');
  sel.value = 'DeepDesert'; sel.dispatchEvent(new ui.w.Event('change', { bubbles: true }));
  const warn = ui.doc.querySelector('#tmapwarn');
  assert.ok(warn, 'another map: the warning');
  assert.ok(visibleText(ui.doc.querySelector('#main')).includes('No other move was checked, so try it on a copy first.'));
  assert.ok(warn.textContent.includes('A position typed for the Deep Desert is not accepted for Hagga Basin.'));
  sel.value = ''; sel.dispatchEvent(new ui.w.Event('change', { bubbles: true }));
  assert.equal(ui.doc.querySelector('#tmapwarn'), null);
  // the same warning is always in About this
  assert.ok(ui.doc.querySelector('details[data-about="admin.movement"]').textContent.includes('No other move was checked, so try it on a copy first.'));
  ui.dom.window.close();
});

test('Admin: the warning comes back after the page is drawn again with another map chosen', async () => {
  const ui = await boot({ post: { '/api/player/teleport': { ok: true } } });
  await open(ui);
  const sel = ui.doc.querySelector('#tmap');
  sel.value = 'DeepDesert'; sel.dispatchEvent(new ui.w.Event('change', { bubbles: true }));
  ui.click('[data-act="teleport"]'); await ui.settle();
  assert.equal(ui.doc.querySelector('#tmap').value, 'DeepDesert', 'the choice is kept across the redraw');
  assert.ok(ui.doc.querySelector('#tmapwarn'), 'and so is the warning');
  ui.dom.window.close();
});
