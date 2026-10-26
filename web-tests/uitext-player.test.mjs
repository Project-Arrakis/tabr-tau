// The text sweep (#426) on the Player pages Crafting, Research, Building sets, Customizations, Skills, Specialization and Journey. See uitext.test.mjs for the checks that hold on every page.
import test from 'node:test';
import assert from 'node:assert/strict';
import { boot, closeAll } from './boot.mjs';
import { visibleText, movedText } from './uitext.mjs';

test.afterEach(() => closeAll());

const page = async (ui, s) => { ui.click('[data-tab="player"]'); await ui.settle(); ui.click(`[data-sub="player:${s}"]`); await ui.settle(); return ui.doc.querySelector('#main'); };
const moves = (main, list) => {
  const shown = visibleText(main), moved = movedText(main);
  for (const s of list) { assert.ok(!shown.includes(s), 'still shown: ' + s); assert.ok(moved.includes(s), 'lost, not moved: ' + s); }
};
const kept = (main, list) => { const shown = visibleText(main); for (const s of list) assert.ok(shown.includes(s), 'missing from the default view: ' + s); };

const SKILL = { id: 'Skills.Ability.X', name: 'X', school: 'Trooper', kind: 'Ability', rank: 1, max: 3, spent: 1, ladder: [1, 3, 6], known: true };
const skills = (resetCount) => ({ total: 10, unspent: 5, spent: 4, schools: [{ key: 'Trooper', label: 'Trooper' }], modules: [SKILL], resetCount });
const specs = (resetCount, unassigned = [], known = 2) => ({ rows: ['Combat', 'Crafting', 'Exploration', 'Gathering', 'Sabotage'].map((track, i) => ({ track, number: i < known ? i + 2 : null, xp: i < known ? 500 : 0, level: 3, keystoneOwned: 0, keystoneTotal: 2, granted: false })),
  unassigned, maxXp: 44182, tracks: ['Combat', 'Crafting', 'Exploration', 'Gathering', 'Sabotage'], resetCount });
const journey = (nodes, tuts) => ({ story: [], contract: [], codex: [], tutorial: [], resetCount: nodes + tuts, resetNodes: nodes, resetTutorials: tuts });

test('Crafting and Research: the button hints are tooltips, the card background is in About this, the counts and the reason a reset is off stay', async () => {
  let ui = await boot({ get: { '/api/player/crafting': { rows: [] }, '/api/player/research': { rows: [], resetCount: 4, resetRecipes: 3, resetSets: 2 } } });
  let main = await page(ui, 'crafting');
  moves(main, ['Adds every recipe your research tree offers that you do not know yet', 'The recipes listed are the ones you know plus the ones your research tree can still give']);
  kept(main, ['Unlock all recipes', '0 recipes; 0 of 0 known.']);
  main = await page(ui, 'research');
  moves(main, ['Buys every entry that unlocks a recipe or a building set and adds what it unlocks', 'Puts the research back to what a new character has', 'Intel is not given back']);
  assert.ok(!visibleText(main).includes('group markers cannot be unlocked as one entry'));
  assert.ok(!movedText(main).includes('group markers cannot be unlocked as one entry'), 'deleted for good: the Group button says it');
  kept(main, ['Unlock all research', 'Reset research (4 entries)', 'entries unlocked in all.']);
  assert.equal(ui.doc.querySelector('[data-act="rsReset"]').getAttribute('aria-describedby'), 'rsresetnote');
  ui.dom.window.close();

  ui = await boot({ get: { '/api/player/research': { rows: [], resetCount: 0, resetRecipes: 0, resetSets: 0 } } });
  main = await page(ui, 'research');
  kept(main, ['Nothing to reset: no entry beyond the four a new character starts with is bought.']);
  assert.equal(ui.doc.querySelector('[data-act="rsReset"]').closest('.tip'), null);
  ui.dom.window.close();
});

test('Building sets and Customizations: hints are tooltips; "N unlocked now" and the missing-library line stay; the long note is in About this', async () => {
  let ui = await boot({ get: { '/api/player/building-sets': { newPieces: [], rows: [] } } });
  let main = await page(ui, 'buildingsets');
  moves(main, ['Teaches every set below except the experimental ones', 'Give adds the item to your backpack']);
  kept(main, ['Unlock all building sets']);
  ui.dom.window.close();

  const cu = (libraryAvailable) => ({ groups: [], libraryAvailable, libraryCount: 12, rows: [] });
  ui = await boot({ get: { '/api/player/customizations': cu(true) } });
  main = await page(ui, 'customizations');
  moves(main, ['Adds the cosmetics of the five sets to your character\'s customization library', 'Store and DLC cosmetics are skipped', 'Puts one cosmetic item of each in your backpack instead',
    'Neither gives a DLC or an account entitlement']);
  kept(main, ['Unlock all sets', 'Give all sets as items', '12 unlocked now.']);
  assert.ok(!visibleText(main).includes('Give adds the item to your backpack'));
  ui.dom.window.close();

  ui = await boot({ get: { '/api/player/customizations': cu(false) } });
  main = await page(ui, 'customizations');
  kept(main, ['This save has no customization library for the character, so cosmetics can only be given as items.']);
  ui.dom.window.close();
});

test('Skills: the three-numbers note is in About this; the reset explanation is a tooltip and the reason it is off stays', async () => {
  let ui = await boot({ get: { '/api/player/skills': skills(2) } });
  let main = await page(ui, 'skills');
  moves(main, ['The save does not keep spent plus unspent equal to the total', 'Every skill of the five schools that has points goes back to rank 0']);
  kept(main, ['Earned in total 10', 'Unspent 5', 'Reset all skills to rank 0 (2)', '1 skill, 1 learned.']);
  ui.dom.window.close();

  ui = await boot({ get: { '/api/player/skills': skills(0) } });
  main = await page(ui, 'skills');
  kept(main, ['Nothing to reset: every skill of the five schools is already at rank 0.']);
  assert.equal(ui.doc.querySelector('[data-act="skReset"]').closest('.tip'), null);
  ui.dom.window.close();
});

test('Specialization: the 90-word paragraph is in About this, the dead-end note is shorter and drawn only while no track has a number, the unknown-rows line stays', async () => {
  let ui = await boot({ get: { '/api/player/specs': specs(2, [{ number: 7, xp: 900, level: 8 }]) } });
  let main = await page(ui, 'specialization');
  moves(main, ['The game\'s own database names its specialization tracks', 'Removes the XP of every track whose number is known']);
  assert.ok(!visibleText(main).includes('Changes are written to the game file as you make them'));
  assert.ok(!movedText(main).includes('Changes are written to the game file as you make them'), 'deleted for good: the banner and the results say it');
  assert.equal(ui.doc.querySelector('#spnone'), null, 'some track has a number: no dead-end note');
  kept(main, ['Reset XP of all known tracks (2)', '1 row of the save whose track is not known yet is left alone.', 'Assigned: Combat = 2, Crafting = 3.', 'Track number 7: 900 XP, level 8']);
  assert.deepEqual(ui.doc.querySelector('[data-act="spResetAll"]').getAttribute('aria-describedby').split(' '), ['spresettip', 'spresetleft']);
  assert.ok(ui.doc.querySelector('[data-act="spMax"]').title.includes('44,182'), 'Max XP is on Grant max');
  // the raw-row warning is still inside its own closed disclosure
  const adv = [...ui.doc.querySelectorAll('details')].find((d) => d.textContent.includes('A wrong number changes another track'));
  assert.ok(adv && !adv.open);
  ui.dom.window.close();

  ui = await boot({ get: { '/api/player/specs': specs(0, [], 0) } });
  main = await page(ui, 'specialization');
  const none = ui.doc.querySelector('#spnone');
  assert.ok(none, 'no track has a number: the note is drawn');
  assert.ok(none.textContent.includes('No track has a number in this save yet, so the XP controls below are off.') && none.textContent.includes('Keystones work now.'));
  assert.ok(none.textContent.split(/\s+/).length < 50, 'shorter than before: ' + none.textContent.split(/\s+/).length);
  kept(main, ['Nothing to reset: no track whose number is known has XP.']);
  ui.dom.window.close();
});

test('Journey: the reset explanation is a tooltip, the children sentence is in About this, the contract line is drawn only on the Contract list', async () => {
  let ui = await boot({ get: { '/api/player/journey/browse': journey(3, 1) } });
  let main = await page(ui, 'journey');
  moves(main, ['Every finished Story and Codex step goes back to not finished', 'Completing a node completes its children; resetting clears it and its children']);
  kept(main, ['Reset journey (3 steps, 1 tutorial)', '0 nodes.']);
  assert.ok(!visibleText(main).includes('Contracts are shown read-only'));
  ui.click('[data-jgrp="contract"]'); await ui.settle();
  assert.ok(visibleText(ui.doc.querySelector('#main')).includes('Contracts are shown read-only; a contract is complete when all its tags are on the character.'));
  ui.dom.window.close();

  ui = await boot({ get: { '/api/player/journey/browse': journey(0, 0) } });
  main = await page(ui, 'journey');
  kept(main, ['Nothing to reset: no Story or Codex step is finished and no tutorial is started.']);
  ui.dom.window.close();
});

test('Journey: the lists can be switched before the filter was ever used (#429), and the filter text is kept across the switch', async () => {
  const ui = await boot({ get: { '/api/player/journey/browse': { ...journey(0, 0), story: [{ id: 'a', name: 'Alpha', depth: 0, status: 'Complete', complete: true, actionable: true }], codex: [{ id: 'b', name: 'Beta', depth: 0, status: 'Started', complete: false, actionable: true }] } } });
  await page(ui, 'journey');
  ui.click('[data-jgrp="codex"]'); await ui.settle();
  assert.ok(visibleText(ui.doc.querySelector('#jout')).includes('Beta'), 'the Codex list drew');
  const q = ui.doc.querySelector('#jq'); q.value = 'bet';
  ui.click('[data-act="jfilter"]'); await ui.settle();
  ui.click('[data-jgrp="story"]'); await ui.settle();
  assert.equal(ui.doc.querySelector('#jq').value, 'bet');
  assert.ok(visibleText(ui.doc.querySelector('#jout')).includes('No node matches the filter.'));
  ui.dom.window.close();
});
