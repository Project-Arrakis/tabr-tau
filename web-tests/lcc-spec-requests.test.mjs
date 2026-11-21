// Every control of the Specialization tab that writes sends the request, asks the question and says the thing recorded in web-tests/lcc-spec-requests.golden.json (M5; #1105). The expected values were
// recorded from the Classic list, the page as it was before the Command Center view existed, with the same inputs; the Classic list is retired (#1105) and this file now plays each scenario on the one view
// (the Command Center, with the Advanced cards the page keeps under it) and compares it with the file one by one, so the Classic == Command Center equivalence that an earlier version of this file proved by
// playing both views now stands on the file alone (it is unchanged by #1105: git diff of the golden file is empty). Real page code, the synthetic answer of M1, no game save. Set LCC_RECORD_REQUESTS=1 to
// write the file again (only ever done on a tree whose request builders are known to be right: the Classic list it was recorded from no longer exists).
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
const ON = { 'Add XP on a track that is not identified: nothing is sent': 'unresolved', 'Grant all keystones on a save with a track not identified, asked then Yes': 'unresolved', 'Assign number 1 to Combat': 'unresolved', 'Assign by hand, Combat 9': 'unresolved', 'Assign by hand with no number': 'unresolved', 'Raw row: Sabotage 500 XP level 2, asked then Yes': 'unresolved', 'Raw row: XP 44182 and level 10, mismatch kept as typed': 'unresolved' };
const text = (e) => (e ? e.textContent.replace(/\s+/g, ' ').trim() : null);
const POSTS = { '/api/player/specs/xp': { ok: true, before: 3000, after: 4000, level: 26 }, '/api/player/specs/max': { ok: true }, '/api/player/specs/reset': { ok: true, removed: 1 },
  '/api/player/specs/reset-all': { ok: true, changed: 5 }, '/api/player/specs/keystones/grant': { ok: true, granted: 5 }, '/api/player/specs/keystones/reset': { ok: true, removed: 5 },
  '/api/player/specs/assign': { ok: true }, '/api/player/specs': { ok: true } };
const ROUTE = /^\/api\/player\/specs/;

// the scenario where the controls are off and nothing is pressed
const SENDS_NOTHING = new Set(['Add XP on a track that is not identified: nothing is sent', 'Remove XP -1000: refused, nothing is sent']);

// a scenario is a step run against the page: the controls it presses are the Command Center's, or the Advanced cards under it (assign, assign by hand, raw row)
const ctl = (ui, sel) => { const e = ui.doc.querySelector(sel); assert.ok(e, 'no element for ' + sel); return e; };
const set = (ui, sel, v) => { const e = ctl(ui, sel); e.value = String(v); e.dispatchEvent(new ui.w.Event('input', { bubbles: true })); };
const seen = (ui) => { ui.asked = text(ui.doc.querySelector('#ask [role="dialog"]')); }; // the question as it was put, before it is answered
const yes = async (ui) => { await ui.settle(); seen(ui); ui.click('#aok'); await ui.settle(); };
const no = async (ui) => { await ui.settle(); seen(ui); ui.click('[data-act="askNo"]'); };
const lane = (ui, key) => { ui.click('[data-lcs-lane="' + key + '"]'); };
const SCENARIOS = {
  'Add XP 1000': async (ui) => { lane(ui, 'combat'); set(ui, '#spxCombat', 1000); ui.click('[data-act="spAddCC"][data-t="Combat"]'); },
  'Remove XP -1000: refused, nothing is sent': async (ui) => { lane(ui, 'combat'); set(ui, '#spxCombat', -1000); ui.click('[data-act="spAddCC"][data-t="Combat"]'); },
  'Add XP on another track': async (ui) => { lane(ui, 'gathering'); set(ui, '#spxGathering', 250); ui.click('[data-act="spAddCC"][data-t="Gathering"]'); },
  'Grant max, asked then Yes': async (ui) => { lane(ui, 'combat'); ui.click('[data-act="spMax"][data-t="Combat"]'); await yes(ui); },
  'Grant max, asked then No': async (ui) => { lane(ui, 'combat'); ui.click('[data-act="spMax"][data-t="Combat"]'); await no(ui); },
  'Reset one track, asked then Yes': async (ui) => { lane(ui, 'sabotage'); ui.click('[data-act="spReset"][data-t="Sabotage"]'); await yes(ui); },
  // a track with no number (#899): the XP controls are off in both views, so no request, and the keystone buttons still work and send the same request
  // (a disabled button sends no click in a browser, so the scenario checks that the controls are off and presses none)
  'Add XP on a track that is not identified: nothing is sent': async (ui) => { lane(ui, 'combat'); for (const sel of ['#spxCombat', '[data-act="spMax"][data-t="Combat"]', '[data-act="spReset"][data-t="Combat"]', '[data-act="spAddCC"][data-t="Combat"]']) assert.equal(ctl(ui, sel).disabled, true, sel + ' is off'); },
  'Grant all keystones on a save with a track not identified, asked then Yes': async (ui) => { ui.click('[data-act="spKeyAll"]'); await yes(ui); },
  'Grant all keystones, asked then Yes': async (ui) => { ui.click('[data-act="spKeyAll"]'); await yes(ui); },
  'Reset all keystones, asked then Yes': async (ui) => { ui.click('[data-act="spKeyReset"]'); await yes(ui); },
  'Reset all keystones, asked then No': async (ui) => { ui.click('[data-act="spKeyReset"]'); await no(ui); },
  'Reset XP of all known tracks, typed sentence': async (ui) => { ui.freeze(); ui.click('[data-act="spResetAll"]'); await ui.settle(); ui.type('reset 2 specializations'); seen(ui); ui.tick(GAP); ui.click('#aok'); },
  'Reset XP of all known tracks, wrong sentence': async (ui) => { ui.freeze(); ui.click('[data-act="spResetAll"]'); await ui.settle(); ui.type('reset 5 specializations'); seen(ui); ui.tick(GAP); ui.click('#aok'); },
  'Assign number 1 to Combat': async (ui) => { set(ui, '#spa1', 'Combat'); ui.click('[data-act="spAssign"][data-n="1"]'); },
  'Assign by hand, Combat 9': async (ui) => { set(ui, '#spmt', 'Combat'); set(ui, '#spmn', 9); ui.click('[data-act="spAssignHand"]'); },
  'Assign by hand with no number': async (ui) => { set(ui, '#spmt', 'Combat'); ui.click('[data-act="spAssignHand"]'); },
  'Raw row: Sabotage 500 XP level 2, asked then Yes': async (ui) => { set(ui, '#st', 5); set(ui, '#sx', 500); set(ui, '#sl', 2); ui.click('[data-act="specSet"]'); await yes(ui); },
  'Raw row: XP 44182 and level 10, mismatch kept as typed': async (ui) => { set(ui, '#st', 3); set(ui, '#sx', 44182); set(ui, '#sl', 10); ui.click('[data-act="specSet"]'); await yes(ui); },
};

async function play(name) {
  const ui = await boot({ get: { '/api/player/specs': SPECS(ON[name]), '/api/landsraad': synthetic() }, post: POSTS, local: { tab: 'player' } });
  ui.click('[data-sub="player:specialization"]'); await ui.settle();
  assert.ok(ui.doc.querySelector('#lcs-root'), 'the Command Center view is the one drawn');
  await SCENARIOS[name](ui);
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
  test('the page sends what was recorded, asks the same question and says the same: ' + name, async () => {
    const got = await play(name);
    if (RECORD) { recorded[name] = got; fs.writeFileSync(GOLDEN, JSON.stringify(Object.fromEntries(Object.keys(recorded).sort().map((k) => [k, recorded[k]])), null, 1) + '\n'); return; }
    assert.ok(golden[name], 'a recorded scenario for ' + name);
    assert.deepEqual(got, golden[name]);
    if (!SENDS_NOTHING.has(name)) assert.ok(got.sent.length === 1 || got.asked !== null || got.said !== null, 'a request was sent, a question asked or a reason given');
    else { assert.deepEqual(got.sent, []); assert.equal(got.asked, null); }
  });
}

// ---- what the Command Center view adds: a number the editor would refuse is stopped before it is sent ----
const BAD = [['blank', ''], ['zero', '0'], ['a fraction', '1.5'], ['not a number', 'abc'], ['over the limit', '44183'], ['under the limit', '-44183'], ['a minus sign', '-1000'], ['minus one', '-1'], ['the negative limit', '-44182'], ['minus zero', '-0'], ['a negative exponent form', '-1e3'], ['an exponent past the limit', '1e5'], ['a fraction as an exponent', '1e-1']];
for (const [what, v] of BAD) {
  test('Command Center, Add XP with ' + what + ': nothing is sent and the reason is shown', async () => {
    const ui = await boot({ get: { '/api/player/specs': SPECS(), '/api/landsraad': synthetic() }, post: POSTS, local: { tab: 'player' } });
    ui.click('[data-sub="player:specialization"]'); await ui.settle();
    ui.click('[data-lcs-lane="combat"]');
    set(ui, '#spxCombat', v);
    ui.click('[data-act="spAddCC"][data-t="Combat"]'); await ui.settle();
    assert.equal(ui.posted.filter((p) => ROUTE.test(p.path)).length, 0, 'no request');
    assert.match(text(ui.doc.querySelector('#toast')), /whole number of XP from 1 to 44,182\. XP is added here, never removed \(use Reset to take it back\)\. Nothing was sent\./);
    closeAll();
  });
}
test('Command Center, Add XP at the two limits is sent as typed', async () => {
  for (const v of ['44182', '1']) {
    const ui = await boot({ get: { '/api/player/specs': SPECS(), '/api/landsraad': synthetic() }, post: POSTS, local: { tab: 'player' } });
    ui.click('[data-sub="player:specialization"]'); await ui.settle();
    ui.click('[data-lcs-lane="combat"]');
    set(ui, '#spxCombat', v);
    ui.click('[data-act="spAddCC"][data-t="Combat"]'); await ui.settle();
    assert.deepEqual(ui.posted.filter((p) => ROUTE.test(p.path)), [{ path: '/api/player/specs/xp', body: { track: 'Combat', amount: Number(v) } }]);
    closeAll();
  }
});

// 1e3 and 1.0 are whole numbers: the page sends them (as 1000 and 1), the same request for the same typing (#899)
for (const [typed, sent] of [['1e3', 1000], ['1.0', 1], ['2E2', 200]]) {
  test('Add XP typed as ' + typed + ' is sent as ' + sent, async () => {
    const ui = await boot({ get: { '/api/player/specs': SPECS(), '/api/landsraad': synthetic() }, post: POSTS, local: { tab: 'player' } });
    ui.click('[data-sub="player:specialization"]'); await ui.settle();
    ui.click('[data-lcs-lane="combat"]');
    set(ui, '#spxCombat', typed);
    ui.click('[data-act="spAddCC"][data-t="Combat"]'); await ui.settle();
    assert.deepEqual(ui.posted.filter((p) => ROUTE.test(p.path)), [{ path: '/api/player/specs/xp', body: { track: 'Combat', amount: sent } }]);
    closeAll();
  });
}

// ---- the golden file covers every operation of the editor ----
test('the golden file covers all nine operations of the editor, and the page reaches all nine', async () => {
  const routes = new Set();
  for (const name of Object.keys(SCENARIOS)) { const g = golden[name]; if (g) for (const s of g.sent) routes.add(s.path); }
  if (RECORD) return;
  assert.deepEqual([...routes].sort(), Object.keys(POSTS).sort(), 'the golden file covers every route');
});
