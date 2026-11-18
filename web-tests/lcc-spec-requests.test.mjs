// Every control of the Specialization tab that writes sends the same request, asks the same question and says the same thing in the Command Center view as in the classic list (M5). The expected
// values (web-tests/lcc-spec-requests.golden.json) were recorded from the classic list, the page as it was before the Command Center view existed, with the same inputs; this file plays each
// scenario in both views and compares them with the file one by one. Real page code, the synthetic answer of M1, no game save. Set LCC_RECORD_REQUESTS=1 to write the file again (only ever done on
// a tree whose classic list is known to be right).
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { boot, closeAll, GAP } from './boot.mjs';
import { synthetic } from './lcc-fixture.mjs';
import { read } from './lcc-load.mjs';

afterEach(closeAll);

const here = path.dirname(fileURLToPath(import.meta.url));
const GOLDEN = path.join(here, 'lcc-spec-requests.golden.json');
const RECORD = process.env.LCC_RECORD_REQUESTS === '1';
const CASES = JSON.parse(read('docs', 'design', 'landsraad-command-center', 'fixtures', 'api-player-specs.cases.json')).cases;
// two saves: every track known with a row of the save that no track has (the ordinary one), and one where Combat has lost its number (the Track numbers card and the raw row's list of tracks differ)
const SAVE = { ordinary: 'unassigned-row-number-9', unresolved: 'conflicting-learned-number-leaves-combat-unresolved' };
const SPECS = (which = 'ordinary') => JSON.parse(JSON.stringify(CASES.find((c) => c.id === SAVE[which]).response));
const ON = { 'Assign number 1 to Combat': 'unresolved', 'Assign by hand, Combat 9': 'unresolved', 'Assign by hand with no number': 'unresolved', 'Raw row: Sabotage 500 XP level 2, asked then Yes': 'unresolved', 'Raw row: XP 44182 and level 10, mismatch kept as typed': 'unresolved' };
const text = (e) => (e ? e.textContent.replace(/\s+/g, ' ').trim() : null);
const POSTS = { '/api/player/specs/xp': { ok: true, before: 3000, after: 4000, level: 26 }, '/api/player/specs/max': { ok: true }, '/api/player/specs/reset': { ok: true, removed: 1 },
  '/api/player/specs/reset-all': { ok: true, changed: 5 }, '/api/player/specs/keystones/grant': { ok: true, granted: 5 }, '/api/player/specs/keystones/reset': { ok: true, removed: 5 },
  '/api/player/specs/assign': { ok: true }, '/api/player/specs': { ok: true } };
const ROUTE = /^\/api\/player\/specs/;

// a scenario is [what is pressed in the classic list, what is pressed in the Command Center view]; both are steps run against the page
const ctl = (ui, sel) => { const e = ui.doc.querySelector(sel); assert.ok(e, 'no element for ' + sel); return e; };
const set = (ui, sel, v) => { const e = ctl(ui, sel); e.value = String(v); e.dispatchEvent(new ui.w.Event('input', { bubbles: true })); };
const seen = (ui) => { ui.asked = text(ui.doc.querySelector('#ask [role="dialog"]')); }; // the question as it was put, before it is answered
const yes = async (ui) => { await ui.settle(); seen(ui); ui.click('#aok'); await ui.settle(); };
const no = async (ui) => { await ui.settle(); seen(ui); ui.click('[data-act="askNo"]'); };
const lane = (ui, v, key) => { if (v === 'cc') { ui.click('[data-lcs-lane="' + key + '"]'); } };
const SCENARIOS = {
  'Add XP 1000': async (ui, v) => { lane(ui, v, 'combat'); set(ui, '#spxCombat', 1000); ui.click(v === 'cc' ? '[data-act="spAddCC"][data-t="Combat"]' : '[data-act="spAdd"][data-t="Combat"]'); },
  'Remove XP -1000': async (ui, v) => { lane(ui, v, 'combat'); set(ui, '#spxCombat', -1000); ui.click(v === 'cc' ? '[data-act="spAddCC"][data-t="Combat"]' : '[data-act="spAdd"][data-t="Combat"]'); },
  'Add XP on another track': async (ui, v) => { lane(ui, v, 'gathering'); set(ui, '#spxGathering', 250); ui.click(v === 'cc' ? '[data-act="spAddCC"][data-t="Gathering"]' : '[data-act="spAdd"][data-t="Gathering"]'); },
  'Grant max, asked then Yes': async (ui, v) => { lane(ui, v, 'combat'); ui.click('[data-act="spMax"][data-t="Combat"]'); await yes(ui); },
  'Grant max, asked then No': async (ui, v) => { lane(ui, v, 'combat'); ui.click('[data-act="spMax"][data-t="Combat"]'); await no(ui); },
  'Reset one track, asked then Yes': async (ui, v) => { lane(ui, v, 'sabotage'); ui.click('[data-act="spReset"][data-t="Sabotage"]'); await yes(ui); },
  'Grant all keystones, asked then Yes': async (ui) => { ui.click('[data-act="spKeyAll"]'); await yes(ui); },
  'Reset all keystones, asked then Yes': async (ui) => { ui.click('[data-act="spKeyReset"]'); await yes(ui); },
  'Reset all keystones, asked then No': async (ui) => { ui.click('[data-act="spKeyReset"]'); await no(ui); },
  'Reset XP of all known tracks, typed sentence': async (ui) => { ui.freeze(); ui.click('[data-act="spResetAll"]'); await ui.settle(); ui.type('reset 2 tracks'); seen(ui); ui.tick(GAP); ui.click('#aok'); },
  'Reset XP of all known tracks, wrong sentence': async (ui) => { ui.freeze(); ui.click('[data-act="spResetAll"]'); await ui.settle(); ui.type('reset 5 tracks'); seen(ui); ui.tick(GAP); ui.click('#aok'); },
  'Assign number 1 to Combat': async (ui) => { set(ui, '#spa1', 'Combat'); ui.click('[data-act="spAssign"][data-n="1"]'); },
  'Assign by hand, Combat 9': async (ui) => { set(ui, '#spmt', 'Combat'); set(ui, '#spmn', 9); ui.click('[data-act="spAssignHand"]'); },
  'Assign by hand with no number': async (ui) => { set(ui, '#spmt', 'Combat'); ui.click('[data-act="spAssignHand"]'); },
  'Raw row: Sabotage 500 XP level 2, asked then Yes': async (ui) => { set(ui, '#st', 5); set(ui, '#sx', 500); set(ui, '#sl', 2); ui.click('[data-act="specSet"]'); await yes(ui); },
  'Raw row: XP 44182 and level 10, mismatch kept as typed': async (ui) => { set(ui, '#st', 3); set(ui, '#sx', 44182); set(ui, '#sl', 10); ui.click('[data-act="specSet"]'); await yes(ui); },
};

async function play(name, view) {
  const ui = await boot({ get: { '/api/player/specs': SPECS(ON[name]), '/api/landsraad': synthetic() }, post: POSTS, local: { tab: 'player', 'spec.view': view } });
  ui.click('[data-sub="player:specialization"]'); await ui.settle();
  assert.equal(!!ui.doc.querySelector('#lcs-root'), view === 'cc', 'the view that was asked for is the one drawn');
  await SCENARIOS[name](ui, view);
  await ui.settle(); await ui.wait(20); await ui.settle();
  const ask = ui.doc.querySelector('#ask');
  const out = {
    sent: ui.posted.filter((p) => ROUTE.test(p.path)).map((p) => ({ path: p.path, body: p.body })),
    asked: ui.asked || (ask && !ask.classList.contains('hide') ? text(ask.querySelector('[role="dialog"]')) : null),
    said: text(ui.doc.querySelector('#toast')),
  };
  closeAll();
  return out;
}

const golden = RECORD || !fs.existsSync(GOLDEN) ? {} : JSON.parse(fs.readFileSync(GOLDEN, 'utf8'));
const recorded = {};
for (const name of Object.keys(SCENARIOS)) {
  test('the classic list sends what was recorded: ' + name, async () => {
    const got = await play(name, 'classic');
    if (RECORD) { recorded[name] = got; fs.writeFileSync(GOLDEN, JSON.stringify(Object.fromEntries(Object.keys(recorded).sort().map((k) => [k, recorded[k]])), null, 1) + '\n'); return; }
    assert.ok(golden[name], 'a recorded scenario for ' + name);
    assert.deepEqual(got, golden[name]);
  });
  test('the Command Center view sends the same request, asks the same question and says the same: ' + name, async () => {
    if (RECORD) return;
    const got = await play(name, 'cc');
    assert.ok(golden[name], 'a recorded scenario for ' + name);
    assert.deepEqual(got, golden[name]);
    assert.ok(got.sent.length === 1 || got.asked !== null || got.said !== null, 'a request was sent, a question asked or a reason given');
  });
}

// ---- what the Command Center view adds: a number the editor would refuse is stopped before it is sent ----
const BAD = [['blank', ''], ['zero', '0'], ['a fraction', '1.5'], ['not a number', 'abc'], ['over the limit', '44183'], ['under the limit', '-44183'], ['an exponent', '1e3']];
for (const [what, v] of BAD) {
  test('Command Center, Apply XP with ' + what + ': nothing is sent and the reason is shown', async () => {
    const ui = await boot({ get: { '/api/player/specs': SPECS(), '/api/landsraad': synthetic() }, post: POSTS, local: { tab: 'player', 'spec.view': 'cc' } });
    ui.click('[data-sub="player:specialization"]'); await ui.settle();
    ui.click('[data-lcs-lane="combat"]');
    set(ui, '#spxCombat', v);
    ui.click('[data-act="spAddCC"][data-t="Combat"]'); await ui.settle();
    assert.equal(ui.posted.filter((p) => ROUTE.test(p.path)).length, 0, 'no request');
    assert.match(text(ui.doc.querySelector('#toast')), /whole number of XP, not 0, from -44,182 to 44,182\. Nothing was sent\./);
    closeAll();
  });
}
test('Command Center, Apply XP at the two limits is sent as typed', async () => {
  for (const v of ['44182', '-44182']) {
    const ui = await boot({ get: { '/api/player/specs': SPECS(), '/api/landsraad': synthetic() }, post: POSTS, local: { tab: 'player', 'spec.view': 'cc' } });
    ui.click('[data-sub="player:specialization"]'); await ui.settle();
    ui.click('[data-lcs-lane="combat"]');
    set(ui, '#spxCombat', v);
    ui.click('[data-act="spAddCC"][data-t="Combat"]'); await ui.settle();
    assert.deepEqual(ui.posted.filter((p) => ROUTE.test(p.path)), [{ path: '/api/player/specs/xp', body: { track: 'Combat', amount: Number(v) } }]);
    closeAll();
  }
});

// ---- the control lists of the two views are the same set of operations ----
test('both views reach all nine operations of the editor, by the same routes', async () => {
  const routes = new Set();
  for (const name of Object.keys(SCENARIOS)) for (const view of ['classic', 'cc']) { const g = golden[name]; if (g) for (const s of g.sent) routes.add(s.path); void view; }
  if (RECORD) return;
  assert.deepEqual([...routes].sort(), Object.keys(POSTS).sort(), 'the golden file covers every route');
});
