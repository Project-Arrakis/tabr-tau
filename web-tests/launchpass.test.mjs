// The countdown before the automatic pass at launch (#460): a bar with the seconds, Skip and a way to turn the switch off, both single clicks that act (no question);
// the note of what the pass did when it ends; and the "Run the automatic pass now" button on the Automatic refill card.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import { boot, closeAll } from './boot.mjs';

afterEach(closeAll);

const JOBS = [{ id: 'water', label: 'Base water', desc: 'Fills Water Cisterns to capacity.', key: 'autoRefillWater', ready: true, reason: '', on: true }];
const prefs = (extra = {}) => ({ autoRefillOnOpen: true, jobs: JOBS, last: '', lastProblem: '', problem: '', ...extra });
const bar = (ui) => ui.doc.querySelector('#launchpass');
const shown = (ui) => !bar(ui).classList.contains('hide');
const gets = (ui, p) => ui.fetched.filter((x) => x === p).length;

async function openBases(ui) {
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="player:bases"]'); await ui.settle();
}

test('A pass that is counting down shows the seconds, Skip and Turn it off at the top of the page', async () => {
  const ui = await boot({ get: { '/api/settings': prefs(), '/api/autopass/launch': { pending: true, running: false, secondsLeft: 15, seconds: 15 } } });
  assert.ok(shown(ui), 'the bar is on screen');
  assert.equal(ui.doc.querySelector('#launchsecs').textContent, '15');
  assert.ok(bar(ui).textContent.includes('Automatic refill and repair starts in 15 s.'), bar(ui).textContent);
  assert.equal(bar(ui).querySelector('[data-act="launchSkip"]').textContent, 'Skip');
  assert.equal(bar(ui).querySelector('[data-act="launchOff"]').textContent, 'Turn it off');
  assert.equal(ui.posted.length, 0, 'nothing was sent just by showing the bar');
});

test('The seconds count down on the page', async () => {
  const ui = await boot({ get: { '/api/settings': prefs(), '/api/autopass/launch': { pending: true, running: false, secondsLeft: 15, seconds: 15 } } });
  await ui.wait(2300);
  const n = Number(ui.doc.querySelector('#launchsecs').textContent);
  assert.ok(n <= 13 && n >= 12, 'after about two seconds the page shows ' + n);
});

test('Without a pass counting down there is no bar', async () => {
  for (const launch of [{ pending: false, running: false, secondsLeft: 0, seconds: 15 }, { __fail: 'not found' }]) {
    const ui = await boot({ get: { '/api/settings': prefs(), '/api/autopass/launch': launch } });
    assert.ok(!shown(ui), JSON.stringify(launch));
    assert.equal(bar(ui).textContent, '');
  }
});

test('Skip is one click: it sends the skip, asks nothing, and the bar goes', async () => {
  const get = { '/api/settings': prefs(), '/api/autopass/launch': { pending: true, running: false, secondsLeft: 12, seconds: 15 } };
  const ui = await boot({ get, post: { '/api/autopass/skip': { skipped: true, started: false, pending: false } } });
  assert.ok(shown(ui));
  get['/api/autopass/launch'] = { pending: false, running: false, secondsLeft: 0, seconds: 15 }; // what the editor answers after Skip
  ui.click('[data-act="launchSkip"]'); await ui.settle();
  assert.deepEqual(ui.posted.map((p) => p.path), ['/api/autopass/skip']);
  assert.ok(ui.doc.querySelector('#ask').classList.contains('hide'), 'no question was asked');
  assert.ok(!shown(ui), 'the bar is gone');
  assert.ok(ui.doc.querySelector('#toast').textContent.includes('skipped for this run'));
});

test('Turn it off is one click: it sends the switch off, asks nothing, and the bar goes', async () => {
  const get = { '/api/settings': prefs(), '/api/autopass/launch': { pending: true, running: false, secondsLeft: 12, seconds: 15 } };
  const ui = await boot({ get, post: { '/api/settings': prefs({ autoRefillOnOpen: false }) } });
  get['/api/autopass/launch'] = { pending: false, running: false, secondsLeft: 0, seconds: 15 };
  ui.click('[data-act="launchOff"]'); await ui.settle();
  assert.deepEqual(ui.posted.map((p) => [p.path, p.body]), [['/api/settings', { autoRefillOnOpen: false }]]);
  assert.ok(ui.doc.querySelector('#ask').classList.contains('hide'), 'no question was asked');
  assert.ok(!shown(ui));
});

test('When the seconds run out the page asks the editor, then shows what the pass did', async () => {
  const get = { '/api/settings': prefs(), '/api/startup': { note: '' }, '/api/autopass/launch': { pending: true, running: false, secondsLeft: 1, seconds: 15 } };
  const ui = await boot({ get });
  assert.ok(shown(ui));
  get['/api/autopass/launch'] = { pending: false, running: false, secondsLeft: 0, seconds: 15 };
  get['/api/startup'] = { note: 'Automatic refill saved 1 Water Cistern to the game (the previous file is backed up as x.db).', warn: false };
  await ui.wait(1500); await ui.settle();
  assert.ok(!shown(ui), 'the bar is gone');
  assert.ok(ui.doc.querySelector('#toast').textContent.includes('Automatic refill saved 1 Water Cistern'));
  assert.deepEqual(ui.posted, [], 'the page started nothing: the editor runs the pass');
});

test('While the editor is working the bar says so and has no buttons', async () => {
  const ui = await boot({ get: { '/api/settings': prefs(), '/api/autopass/launch': { pending: false, running: true, secondsLeft: 0, seconds: 15 } } });
  assert.ok(shown(ui));
  assert.ok(bar(ui).textContent.includes('is running'), bar(ui).textContent);
  assert.equal(bar(ui).querySelectorAll('button').length, 0);
});

test('The Automatic refill card has Run the automatic pass now: one click, no question, the answer is shown', async () => {
  const note = 'Automatic refill saved 2 vehicles refuelled to the game (the previous file is backed up as x.db).';
  const ui = await boot({ get: { '/api/settings': prefs({ autoRefillOnOpen: false }) }, post: { '/api/autopass/run': { note, warn: false } } });
  await openBases(ui);
  const b = ui.doc.querySelector('[data-act="autoNow"]');
  assert.ok(b, 'the button is on the card');
  assert.equal(b.textContent, 'Run the automatic pass now');
  assert.ok(b.closest('.card').querySelector('h3').textContent.startsWith('Automatic refill and repair'));
  ui.click('[data-act="autoNow"]'); await ui.settle();
  assert.deepEqual(ui.posted.map((p) => [p.path, p.body]), [['/api/autopass/run', {}]]);
  assert.ok(ui.doc.querySelector('#ask').classList.contains('hide'), 'no question was asked');
  assert.ok(ui.doc.querySelector('#toast').textContent.includes('Automatic refill saved 2 vehicles'));
});

test('A refused run (the game may still hold the save) is shown as a warning that stays', async () => {
  const note = 'The automatic pass was not run: a single-player session is active, and the game rewrites the save about every 20 seconds while it runs. Run it again once that clears.';
  const ui = await boot({ get: { '/api/settings': prefs() }, post: { '/api/autopass/run': { note, warn: true } } });
  await openBases(ui);
  ui.click('[data-act="autoNow"]'); await ui.settle();
  const t = ui.doc.querySelector('#toast div.err');
  assert.ok(t && t.textContent.includes('a single-player session is active'), 'the warning is an error-style toast, dismissed by hand');
});

test('The page asks about the countdown once at open and not again when nothing counts', async () => {
  const ui = await boot({ get: { '/api/settings': prefs(), '/api/autopass/launch': { pending: false, running: false, secondsLeft: 0, seconds: 15 } } });
  await ui.wait(1300);
  assert.equal(gets(ui, '/api/autopass/launch'), 1);
});
