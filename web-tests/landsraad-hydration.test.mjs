// The Landsraad tab keeps the house goal and the reward tiers apart (#363), and Fill hydration says plainly when it is only queued (#371).
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import { boot, closeAll } from './boot.mjs';

afterEach(closeAll);

const landsraad = () => ({
  term: { term_id: 7, start_time: 1, end_time: 2, reigning_faction_id: 1, active_decree_id: 1, elected_decree_id: 1 },
  factions: [{ id: 1, name: 'Atreides' }, { id: 2, name: 'Harkonnen' }],
  character: { player_id: 1, name: 'Tester', faction_id: 1 },
  decrees: [{ id: 1, decree_name: 'Decree A', disabled: 0 }],
  rotation: [],
  tasks: [{
    id: 900, board_index: 20, house_name: 'DA_HouseVarota', goal_amount: 4000, completed: 0,
    contributions: [{ faction_id: 1, amount: 1000 }],
    player_contributions: [{ player_id: 5, faction_id: 1, amount: 4000, mine: false }, { player_id: 1, faction_id: 1, amount: 6500, mine: true }],
    my_contribution: 6500, my_recorded: true,
    rewards: [1200, 2400, 3600, 4800, 6000].map((threshold) => ({ threshold, amount: 1, template_id: 'Item.A', reached: true, delivered: threshold <= 2400 })),
  }, {
    id: 901, board_index: 21, house_name: 'DA_HouseKenola', goal_amount: 4000, completed: 0, contributions: [], player_contributions: [],
    my_contribution: 0, my_recorded: false,
    rewards: [{ threshold: 1200, amount: 1, template_id: 'Item.A', reached: false, delivered: false }],
  }],
});

async function openLandsraad() {
  const ui = await boot({ get: { '/api/landsraad': landsraad() }, post: { '/api/landsraad/progress': { ok: true, outcome: 'written' }, '/api/landsraad/mine': { ok: true, before: 0, after: 1200, changed: true, faction_changed: false, outcome: 'written' } } });
  ui.click('[data-tab="landsraad"]'); await ui.settle();
  return ui;
}

test('the Landsraad tab shows the house goal and the reward tiers as two separate things', async () => {
  const ui = await openLandsraad();
  const heads = [...ui.doc.querySelectorAll('#main th')].map((t) => t.textContent.trim());
  assert.ok(heads.includes('House goal (faction)'), heads.join(' | '));
  assert.ok(heads.includes('Reward tiers (personal contribution)'), heads.join(' | '));
  assert.ok(!heads.includes('Goal') && !heads.includes('Rewards'), 'the old labels that put both on one scale are gone: ' + heads.join(' | '));
  const row = ui.doc.querySelector('[data-act="tfill"][data-id="900"]').closest('tr').textContent;
  assert.ok(row.includes('4,000'), 'the goal is 4,000: ' + row);
  assert.ok(row.includes('1,000 of 4,000'), 'faction progress is read against the goal: ' + row);
  assert.ok(row.includes('5 tiers'), 'five tiers: ' + row);
  const tiers = ui.doc.querySelector('[data-act="tfill"][data-id="900"]').closest('tr').querySelector('details').textContent;
  for (const n of ['1,200', '2,400', '3,600', '4,800', '6,000']) assert.ok(tiers.includes('At ' + n + ' contributed'), tiers);
  const note = ui.doc.querySelector('#lscales').textContent;
  assert.ok(/can go above the house goal/.test(note), note);
  assert.ok(/Whether the game hands out a tier when it loads the save after a fill is not known/.test(note), 'what is unknown is said: ' + note);
  assert.ok(/row the save keeps under your character's own id/.test(note), note);
});

test('your contribution is the character row, shown whole above the goal; the rows of the simulated players are told apart; none recorded says so', async () => {
  const ui = await openLandsraad();
  const heads = [...ui.doc.querySelectorAll('#main th')].map((t) => t.textContent.trim());
  assert.ok(heads.includes('Your contribution'), heads.join(' | '));
  const a = ui.doc.querySelector('[data-act="tfill"][data-id="900"]').closest('tr').textContent;
  assert.ok(a.includes('6,500') && a.includes('5 of 5 tiers reached'), a);
  assert.ok(a.includes('other players: 1 row, largest 4,000'), 'the other rows are not mixed into the character row: ' + a);
  const b = ui.doc.querySelector('[data-act="tfill"][data-id="901"]').closest('tr').textContent;
  assert.ok(b.includes('none recorded') && b.includes('0 of 1 tier reached'), b);
});

test('each tier says reached or not and delivered or not, and only a tier not yet reached has a Fill button', async () => {
  const ui = await openLandsraad();
  const lines = [...ui.doc.querySelector('[data-act="tfill"][data-id="900"]').closest('tr').querySelectorAll('details > div')].map((d) => d.textContent);
  assert.ok(lines[0].includes('reached, recorded as delivered'), lines[0]);
  assert.ok(lines[4].includes('reached, no delivery recorded in this term'), lines[4]);
  assert.equal(ui.doc.querySelectorAll('[data-act="mfill"][data-id="900"]').length, 0, 'every tier of 900 is reached');
  const one = ui.doc.querySelectorAll('[data-act="mfill"][data-id="901"]');
  assert.equal(one.length, 1);
  assert.ok(ui.doc.querySelector('[data-act="tfill"][data-id="901"]').closest('tr').textContent.includes('not reached'));
  assert.ok(/reward itself is not written/.test(one[0].title), one[0].title);
});

test('Fill to this tier sends the task and the tier, and the faction goal only when the box is ticked', async () => {
  const ui = await openLandsraad();
  ui.click('[data-act="mfill"][data-id="901"]'); await ui.settle();
  let sent = ui.posted.filter((p) => p.path === '/api/landsraad/mine');
  assert.equal(sent.length, 1);
  assert.deepEqual(sent[0].body, { task_id: 901, tier: 1200, with_faction: false });
  assert.ok(ui.doc.querySelector('#toast').textContent.includes('Your contribution 0 → 1,200'), ui.doc.querySelector('#toast').textContent);
  ui.doc.querySelector('#lwf').checked = true;
  ui.click('[data-act="mfill"][data-id="901"]'); await ui.settle();
  sent = ui.posted.filter((p) => p.path === '/api/landsraad/mine');
  assert.equal(sent[1].body.with_faction, true);
});

test('the fill button says it fills the faction goal and sends the goal, nothing about a personal contribution', async () => {
  const ui = await openLandsraad();
  const btn = ui.doc.querySelector('[data-act="tfill"][data-id="900"]');
  assert.equal(btn.textContent.trim(), 'Fill faction goal');
  assert.ok(/personal contribution and the reward tiers are not changed/.test(btn.title), btn.title);
  ui.click('[data-act="tfill"][data-id="900"]'); await ui.settle();
  const sent = ui.posted.filter((p) => p.path === '/api/landsraad/progress');
  assert.equal(sent.length, 1);
  assert.deepEqual(Object.keys(sent[0].body).sort(), ['amount', 'faction_id', 'task_id']);
  assert.equal(sent[0].body.amount, 4000);
});

async function fillHydration(post) {
  const ui = await boot({ post: { '/api/player/hydration': post } });
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="char:inventory"]'); await ui.settle();
  ui.click('[data-act="fillWater"]'); await ui.settle();
  return ui;
}

test('Fill hydration queued while the game runs says it will not help right now, and is not a success message', async () => {
  const ui = await fillHydration({ ok: true, before: 0, after: 100, outcome: 'queued' });
  const warn = ui.doc.querySelector('#toast .part');
  assert.ok(warn, 'a warning that stays until it is dismissed: ' + ui.doc.querySelector('#toast').innerHTML);
  const t = warn.textContent;
  assert.ok(t.includes('queued, not applied'), t);
  assert.ok(t.includes('will not help right now'), t);
  assert.ok(t.includes('needs another source of water now'), t);
  assert.ok(t.includes('after the game closes and loads this save again'), t);
  assert.ok(!/^Hydration/.test(t), 'it does not read as the plain success line');
  assert.ok(ui.doc.querySelector('[data-act="fillWater"]').title.includes('will not help a thirsty character right now'), 'the button says it too');
});

test('Fill hydration written at once keeps the short success line', async () => {
  const ui = await fillHydration({ ok: true, before: 12.34, after: 100, outcome: 'written' });
  assert.equal(ui.doc.querySelector('#toast .part'), null);
  assert.equal(ui.doc.querySelector('#toast').textContent.trim(), 'Hydration 12.3 → 100');
});
