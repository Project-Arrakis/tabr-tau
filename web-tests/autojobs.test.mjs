// The automatic refill and repair card (#352): a master switch and one checkbox per job, a job that is not available greyed with its reason,
// what the last pass did (or why it could not finish) as text, and a refused change put back as the server holds it.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import { boot, closeAll } from './boot.mjs';

afterEach(closeAll);

const JOBS = [
  { id: 'water', label: 'Base water', desc: 'Fills Water Cisterns, Windtraps and Blood Purifiers to capacity.', key: 'autoRefillWater', ready: true, reason: '', on: true },
  { id: 'power', label: 'Base power', desc: 'Tops up generators, wind turbines and windtraps (fuel, lubricant, filters).', key: 'autoRefillPower', ready: true, reason: '', on: true },
  { id: 'repair-vehicles', label: 'Repair vehicles', desc: 'Repairs every part of your own vehicles to full durability.', key: 'autoRepairVehicles', ready: true, reason: '', on: false },
  { id: 'fill-blood', label: 'Fill blood sacks', desc: 'Fills the blood sacks you carry.', key: 'autoFillBloodSacks', ready: false, reason: 'arrives with the blood sack feature', on: true },
  { id: 'refuel', label: 'Refuel vehicles', desc: 'Fills the fuel tank of your own vehicles.', key: 'autoRefuelVehicles', ready: false, reason: 'arrives with the refuel feature', on: true },
];
const prefs = (extra = {}) => ({ autoRefillOnOpen: true, jobs: JOBS, last: '', lastProblem: '', problem: '', ...extra });

async function openBases(ui, tab) {
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="player:bases"]'); await ui.settle();
  if (tab) { ui.click(`[data-bstab="${tab}"]`); await ui.settle(); }
}
const card = (ui) => ui.doc.querySelector('[data-act="autoref"]').closest('.card');
const posts = (ui) => ui.posted.filter((x) => x.path === '/api/settings');

test('The card has the new title, the master switch and one checkbox per job with its label and description', async () => {
  const ui = await boot({ get: { '/api/settings': prefs() } });
  await openBases(ui);
  const c = card(ui);
  assert.equal(c.querySelector('h3').textContent, 'Automatic refill and repair when the editor opens');
  const boxes = [...c.querySelectorAll('input[type="checkbox"][data-autojob]')];
  assert.deepEqual(boxes.map((b) => b.dataset.autojob), JOBS.map((j) => j.key), 'one per job, in the order the server gives');
  assert.deepEqual(boxes.map((b) => b.checked), JOBS.map((j) => j.on), 'each shows the switch the server holds');
  for (const [i, j] of JOBS.entries()) {
    const b = boxes[i];
    assert.ok(b.closest('label').textContent.includes(j.label), `${j.id}: the label is next to the box`);
    const note = ui.doc.getElementById('ajt-' + j.key);
    assert.ok(note && note.classList.contains('tipbox') && note.textContent.includes(j.desc), `${j.id}: the description is the tooltip of the box (not a permanent line, #426)`);
    assert.ok(b.getAttribute('aria-describedby').split(' ').includes('ajt-' + j.key), `${j.id}: and the box is described by it`);
  }
  assert.equal(c.querySelector('button[data-act="autoref"]').textContent, 'Turn off', 'the master switch is on in this fixture');
});

test('A job that is not available is greyed with the reason; the ones that are available can be changed', async () => {
  const ui = await boot({ get: { '/api/settings': prefs() } });
  await openBases(ui);
  const c = card(ui);
  const box = (k) => c.querySelector(`[data-autojob="${k}"]`);
  assert.equal(box('autoFillBloodSacks').disabled, true);
  assert.equal(box('autoRefuelVehicles').disabled, true);
  assert.ok(ui.doc.getElementById('aj-autoFillBloodSacks').textContent.includes('arrives with the blood sack feature'), 'the reason is on the page');
  assert.ok(ui.doc.getElementById('aj-autoRefuelVehicles').textContent.includes('arrives with the refuel feature'));
  assert.equal(box('autoRefillWater').disabled, false);
  assert.equal(ui.doc.getElementById('aj-autoRefillWater'), null, 'an available job has no reason line');
});

test('Ticking a job sends just that key, without a question, and says what changed', async () => {
  const after = prefs({ jobs: JOBS.map((j) => (j.key === 'autoRepairVehicles' ? { ...j, on: true } : j)) });
  const ui = await boot({ get: { '/api/settings': prefs() }, post: { '/api/settings': after } });
  await openBases(ui);
  const b = card(ui).querySelector('[data-autojob="autoRepairVehicles"]');
  b.checked = true;
  b.dispatchEvent(new ui.w.Event('change', { bubbles: true }));
  await ui.settle();
  assert.deepEqual(posts(ui).map((p) => p.body), [{ autoRepairVehicles: true }], 'only the changed key goes, as a boolean');
  assert.equal(ui.doc.querySelector('#ask [role="dialog"]'), null, 'a job switch needs no confirmation (the master switch asks)');
  assert.ok(ui.doc.querySelector('#toast').textContent.includes('Repair vehicles is now on'), ui.doc.querySelector('#toast').textContent);
});

test('A refused change is shown as an error and the box goes back to what the server holds', async () => {
  const ui = await boot({ get: { '/api/settings': prefs() }, post: { '/api/settings': { __fail: 'the settings file x is damaged' } } });
  await openBases(ui);
  const b = card(ui).querySelector('[data-autojob="autoRefillWater"]');
  assert.equal(b.checked, true);
  b.checked = false;
  b.dispatchEvent(new ui.w.Event('change', { bubbles: true }));
  await ui.settle();
  assert.deepEqual(posts(ui).map((p) => p.body), [{ autoRefillWater: false }], 'unticking sends false');
  assert.ok(ui.doc.querySelector('#toast .err').textContent.includes('damaged'), 'the reason is shown');
  assert.equal(card(ui).querySelector('[data-autojob="autoRefillWater"]').checked, true, 'redrawn from the server, not left unticked');
});

test('What the last pass did and why a pass could not finish are shown as text on the card', async () => {
  const hostile = '<img src=x onerror=window.__pwned=1> job';
  const ui = await boot({ get: { '/api/settings': prefs({ last: '2026-10-23 09:00: Automatic refill saved 1 Water Cistern; 11 vehicle parts repaired; 4 vehicles refuelled to the game (the previous file is backed up as x.db)', lastProblem: 'Automatic refill could not finish: the job "Repair vehicles" failed (' + hostile + '). Nothing was changed.' }) } });
  await openBases(ui);
  const c = card(ui);
  assert.ok(c.textContent.includes('Last automatic save: 2026-10-23 09:00: Automatic refill saved 1 Water Cistern; 11 vehicle parts repaired; 4 vehicles refuelled'), 'per job, in words');
  const bad = c.querySelector('p.bad');
  assert.ok(bad && bad.textContent.includes('the job "Repair vehicles" failed') && bad.textContent.includes(hostile), 'the failing job and the error');
  assert.equal(c.querySelector('img'), null, 'text, never markup');
  const quiet = await boot({ get: { '/api/settings': prefs() } });
  await openBases(quiet);
  assert.equal(card(quiet).querySelector('p.bad'), null, 'no problem, nothing shown');
});

test('The card says it is one edit, skipped during a session, writes nothing when nothing is needed, and that the jobs default to on', async () => {
  const ui = await boot({ get: { '/api/settings': prefs() } });
  await openBases(ui);
  const t = card(ui).textContent;
  for (const want of ['one edit', 'saves straight away', 'It waits while the game is open', 'nothing is written or queued', 'When nothing needs doing nothing is written', 'If one job fails, nothing is changed', 'Every job is ticked by default, the repairs too', 'your character owns or carries']) {
    assert.ok(t.includes(want), `the card lacks: ${want}`);
  }
});

test('The same card is on the Power and the Water tab', async () => {
  const ui = await boot({ get: { '/api/settings': prefs() } });
  await openBases(ui);
  assert.equal(ui.doc.querySelectorAll('[data-autojob]').length, JOBS.length, 'Power tab');
  ui.click('[data-bstab="water"]'); await ui.settle();
  assert.equal(ui.doc.querySelectorAll('[data-autojob]').length, JOBS.length, 'Water tab');
  assert.equal(ui.doc.querySelector('[data-act="autoref"]').closest('.card').querySelector('h3').textContent, 'Automatic refill and repair when the editor opens');
});

test('Turning the master switch on still asks first, and the question names the new jobs', async () => {
  const ui = await boot({ get: { '/api/settings': prefs({ autoRefillOnOpen: false }) }, post: { '/api/settings': prefs() } });
  await openBases(ui);
  ui.click('[data-act="autoref"]'); await ui.settle();
  const ask = ui.doc.querySelector('#ask').textContent;
  assert.ok(ask.includes('repairs your vehicles') && ask.includes('like every other change'), ask);
  assert.equal(posts(ui).length, 0, 'nothing is sent before confirming');
  ui.click('[data-act="askOk"]'); await ui.settle();
  assert.deepEqual(posts(ui)[0].body, { autoRefillOnOpen: true });
});

test('A server that sends no job list still shows the master switch (an older answer)', async () => {
  const ui = await boot({ get: { '/api/settings': { autoRefillOnOpen: false } } });
  await openBases(ui);
  assert.ok(card(ui).textContent.includes('off'));
  assert.equal(ui.doc.querySelectorAll('[data-autojob]').length, 0);
});
