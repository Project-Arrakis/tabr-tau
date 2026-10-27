// Several houses and single tiers chosen for a fill on the Landsraad tab (#364): a house chosen is all five tiers, the highest counts, the question types the number, and
// the confirmed count and the term go with the request.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import { boot, closeAll, GAP } from './boot.mjs';

afterEach(closeAll);

const TIERS = [1200, 2400, 3600, 4800, 6000];
const house = (id, name, mine, extra = {}) => ({
  id, board_index: id - 900, house_name: 'DA_House' + name, goal_amount: 4000, completed: 0, contributions: [], player_contributions: [],
  my_contribution: mine, my_recorded: mine > 0,
  rewards: TIERS.map((threshold) => ({ threshold, amount: 1, template_id: 'Item.A', reached: mine >= threshold, delivered: false })),
  ...extra,
});
const board = (faction = 1) => ({
  term: { term_id: 7, start_time: 1, end_time: 2, reigning_faction_id: 1, active_decree_id: 1, elected_decree_id: 1 },
  factions: [{ id: 1, name: 'Atreides' }, { id: 2, name: 'Harkonnen' }],
  character: { player_id: 1, name: 'Tester', faction_id: faction },
  decrees: [{ id: 1, decree_name: 'Decree A', disabled: 0 }], rotation: [],
  tasks: [house(900, 'Alexin', 0), house(901, 'Thorvald', 1300), house(902, 'Hurata', 7000)],
});

async function open(extra = {}, data = board()) {
  const ui = await boot({ get: { '/api/landsraad': data }, post: { '/api/landsraad/mine-many': { ok: true, changed: 2, noop: false, houses: 2, faction_changed: 0, outcome: 'written' }, ...extra } });
  ui.freeze();
  ui.click('[data-tab="landsraad"]'); await ui.settle();
  return ui;
}
const tick = (ui, sel, on = true) => { const el = ui.doc.querySelector(sel); assert.ok(el, 'no ' + sel); el.checked = on; el.dispatchEvent(new ui.w.Event('change', { bubbles: true })); };
const mine = (ui) => ui.posted.filter((p) => p.path === '/api/landsraad/mine-many');
const sum = (ui) => ui.doc.querySelector('#lsum').textContent;

test('every house has a box with a name, and choosing a house chooses and locks all five of its tiers', async () => {
  const ui = await open();
  const boxes = [...ui.doc.querySelectorAll('[data-lshouse]')];
  assert.equal(boxes.length, 3);
  assert.ok(boxes.every((b) => /^Choose House \w+ with all its tiers$/.test(b.getAttribute('aria-label'))), boxes.map((b) => b.getAttribute('aria-label')).join(' | '));
  assert.equal(ui.doc.querySelectorAll('[data-lstier="1200"]').length, 3);
  assert.ok(sum(ui).startsWith('Nothing is chosen'), sum(ui));
  tick(ui, '[data-lshouse="900"]');
  const tiers = [...ui.doc.querySelectorAll('[data-lsid="900"]')];
  assert.equal(tiers.length, 5);
  assert.ok(tiers.every((t) => t.checked && t.disabled), 'all five tiers are chosen and cannot be unchosen one by one');
  assert.ok([...ui.doc.querySelectorAll('[data-lsid="901"]')].every((t) => !t.checked && !t.disabled), 'another house is untouched');
  assert.ok(sum(ui).includes('1 house with all their tiers') && sum(ui).includes('raise your contribution on 1 house'), sum(ui));
  tick(ui, '[data-lshouse="900"]', false);
  assert.ok([...ui.doc.querySelectorAll('[data-lsid="900"]')].every((t) => !t.checked && !t.disabled));
});

test('Choose all houses and Clear the choice', async () => {
  const ui = await open();
  ui.click('[data-act="lsAll"]'); await ui.settle();
  assert.ok([...ui.doc.querySelectorAll('[data-lshouse]')].every((b) => b.checked));
  assert.ok(sum(ui).includes('3 houses'), sum(ui));
  assert.ok(sum(ui).includes('raise your contribution on 2 houses'), 'Hurata is at 7,000 and does not count: ' + sum(ui));
  ui.click('[data-act="lsNone"]'); await ui.settle();
  assert.ok([...ui.doc.querySelectorAll('[data-lshouse], [data-lstier]')].every((b) => !b.checked && !b.disabled));
});

test('Fill chosen asks nothing: one press sends the highest tier per house, the term and the count, and the result says the rewards are not written (#450)', async () => {
  const ui = await open();
  tick(ui, '[data-lshouse="900"]');                   // all five tiers of Alexin
  tick(ui, '[data-lstier="2400"][data-lsid="901"]');  // single tiers of Thorvald: 2,400 and 3,600
  tick(ui, '[data-lstier="3600"][data-lsid="901"]');
  tick(ui, '[data-lstier="1200"][data-lsid="902"]');  // already above it
  ui.click('[data-act="lsFill"]'); await ui.settle();
  assert.equal(ui.doc.querySelector('#ask [role="dialog"]'), null, 'no question is asked');
  assert.deepEqual(mine(ui).map((p) => p.body), [{ term_id: 7, items: [{ task_id: 900, tier: 6000 }, { task_id: 901, tier: 3600 }, { task_id: 902, tier: 1200 }], with_faction: false, expected: 2 }]);
  assert.ok(ui.doc.querySelector('#toast').textContent.includes('Your contribution raised on 2 houses'), ui.doc.querySelector('#toast').textContent);
  assert.ok(ui.doc.querySelector('#toast').textContent.includes('The rewards themselves are not written'), 'the result stays honest about the rewards');
  assert.ok([...ui.doc.querySelectorAll('[data-lshouse], [data-lstier]')].every((b) => !b.checked), 'the choice is cleared after the fill');
});

test('with the faction box ticked the faction goal is part of the count and of the request', async () => {
  const ui = await open();
  tick(ui, '[data-lshouse="902"]'); // 7,000 contributed, faction progress 0 of 4,000
  ui.click('[data-act="lsFill"]'); await ui.settle();
  assert.equal(ui.doc.querySelector('#ask [role="dialog"]'), null, 'nothing changes without the box');
  assert.ok(ui.doc.querySelector('#toast').textContent.includes('Nothing to change'), ui.doc.querySelector('#toast').textContent);
  tick(ui, '#lwf');
  assert.ok(sum(ui).includes('or the faction goal'), sum(ui));
  ui.click('[data-act="lsFill"]'); await ui.settle();
  assert.equal(ui.doc.querySelector('#ask [role="dialog"]'), null, 'no question is asked');
  assert.deepEqual(mine(ui).map((p) => p.body), [{ term_id: 7, items: [{ task_id: 902, tier: 6000 }], with_faction: true, expected: 1 }]);
});

test('nothing chosen sends nothing and asks nothing', async () => {
  const ui = await open();
  ui.click('[data-act="lsFill"]'); await ui.settle();
  assert.equal(ui.doc.querySelector('#ask [role="dialog"]'), null);
  assert.ok(ui.doc.querySelector('#toast').textContent.includes('Nothing is chosen'));
  assert.equal(mine(ui).length, 0);
});

test('a character in no faction cannot fill: the reason is said and no question is asked', async () => {
  const ui = await open({}, board(null));
  tick(ui, '[data-lshouse="900"]');
  ui.click('[data-act="lsFill"]'); await ui.settle();
  assert.equal(ui.doc.querySelector('#ask [role="dialog"]'), null);
  assert.ok(ui.doc.querySelector('#toast').textContent.includes('no faction'), ui.doc.querySelector('#toast').textContent);
  assert.equal(mine(ui).length, 0);
});

test('a refused fill (the board changed) shows the reason and keeps the choice', async () => {
  const ui = await open({ '/api/landsraad/mine-many': { __fail: 'the number of houses to fill is now 1, not the 2 the page counted, so nothing was changed: reload the page and look again', __code: 'conflict', __status: 409 } });
  tick(ui, '[data-lshouse="900"]'); tick(ui, '[data-lshouse="901"]');
  ui.click('[data-act="lsFill"]'); await ui.settle();
  assert.ok(ui.doc.querySelector('#toast').textContent.includes('nothing was changed'), ui.doc.querySelector('#toast').textContent);
  assert.equal(ui.doc.querySelector('[data-lshouse="900"]').checked, true);
});
