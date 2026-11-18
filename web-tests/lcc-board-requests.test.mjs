// Every control of the board that writes still sends exactly the request it sent before the display remediation B (the bulk-edit bar moved into the board's grid, the Actions block moved up, the notes
// became disclosures, the bar of the chosen houses for a phone was added). The expected requests, the questions asked and the result lines are the ones the board produced before that change, recorded
// from the same inputs (web-tests/lcc-board-requests.golden.json); this file plays the same scenarios against the page now and compares them one by one. Real page code, the synthetic answer of M1,
// no game save. Set LCC_RECORD_REQUESTS=1 to write the file again (only ever done on a tree that is known to be right).
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { boot, closeAll } from './boot.mjs';
import { synthetic } from './lcc-fixture.mjs';

afterEach(closeAll);

const here = path.dirname(fileURLToPath(import.meta.url));
const GOLDEN = path.join(here, 'lcc-board-requests.golden.json');
const RECORD = process.env.LCC_RECORD_REQUESTS === '1';
const OK = { ok: true, changed: 1, houses: 1, skipped: 0, contribution_raised: 1, contribution_lowered: 0, completed: 0, reopened: 0, reward_rows: 0, outcome: 'written' };
const text = (el) => (el ? el.textContent.replace(/\s+/g, ' ').trim() : null);
const click = (ui, el) => { if (typeof el === 'string') el = ui.doc.querySelector(el); assert.ok(el, 'no element'); el.focus(); el.dispatchEvent(new ui.w.MouseEvent('click', { bubbles: true, cancelable: true })); };
const select = (ui, sel, v) => { const el = ui.doc.querySelector(sel); el.value = String(v); el.dispatchEvent(new ui.w.Event('change', { bubbles: true })); };
const idOf = (a, name) => a.tasks.find((t) => t.house_name === 'DA_House' + name).id;
const look = (ui, a, name) => { const insp = ui.doc.querySelector('.lcc-bd-insp'); if (!insp || insp.dataset.id !== String(idOf(a, name))) click(ui, '[data-fk="ht:' + idOf(a, name) + '"]'); };
const pick = (ui, a, names) => { for (const n of names) { const c = ui.doc.querySelector('.lcc-bd-pick[data-id="' + idOf(a, n) + '"]'); c.checked = true; c.dispatchEvent(new ui.w.Event('change', { bubbles: true })); } };
const act = (ui, kind) => '.lcc-bd-insp .lcc-bd-act[data-do="' + kind + '"]';

// where a bulk press comes from: the action bar, or (on a phone) the bar of the chosen houses, which presses the action bar's own button
const bulkBtn = (kind, via) => (via === 'phone' ? '[data-pb="' + kind + '"]' : '[data-fk="b-' + kind + '"]');

const SCENARIOS = {
  'a tier box of a card, raise': async (ui, a) => { click(ui, '[data-fk="ct:' + idOf(a, 'Kenola') + ':3"]'); },
  'a tier box of a card, remove the highest': async (ui, a) => { click(ui, '[data-fk="ct:' + idOf(a, 'Kenola') + ':1"]'); },
  'a tier row of the panel, raise': async (ui, a) => { look(ui, a, 'Kenola'); click(ui, '.lcc-bd-insp .lcc-bd-tier[data-t="3"]'); },
  'Fill 5 tiers': async (ui, a) => { look(ui, a, 'Lindaren'); click(ui, act(ui, 'fill')); },
  'Fill faction goal': async (ui, a) => { look(ui, a, 'Lindaren'); click(ui, act(ui, 'fillgoal')); },
  'Complete House': async (ui, a) => { look(ui, a, 'Sor'); click(ui, act(ui, 'complete')); },
  'Reopen': async (ui, a) => { look(ui, a, 'Hurata'); click(ui, act(ui, 'reopen')); },
  'Complete House on a House another faction took: asked, then Yes': async (ui, a) => { look(ui, a, 'Vernius'); click(ui, act(ui, 'complete')); await ui.settle(); click(ui, '[data-fk="c-yes"]'); },
  'Apply tier, raise': async (ui, a) => { look(ui, a, 'Kenola'); select(ui, '#lcc-bd-tierset', 3); click(ui, '.lcc-bd-insp [data-lcb="settier"]'); },
  'Apply tier, remove the highest': async (ui, a) => { look(ui, a, 'Kenola'); select(ui, '#lcc-bd-tierset', 0); click(ui, '.lcc-bd-insp [data-lcb="settier"]'); },
  'another faction chosen, Complete House': async (ui, a) => { select(ui, '#lsb-fac', 2); await ui.settle(); look(ui, a, 'Lindaren'); click(ui, act(ui, 'complete')); },
};
for (const kind of ['fill', 'complete', 'reopen']) {
  for (const via of ['bar', 'phone']) {
    SCENARIOS['several houses, ' + kind + ' chosen, asked, then Yes' + (via === 'phone' ? ' (the phone bar)' : '')] = async (ui, a) => {
      pick(ui, a, kind === 'reopen' ? ['Vernius', 'Hurata', 'Tseida'] : ['Sor', 'Lindaren', 'Kenola', 'Vernius']); // Reopen changes only the houses that are taken
      click(ui, bulkBtn(kind, via));
      await ui.settle();
      click(ui, '[data-fk="c-yes"]');
    };
  }
}
SCENARIOS['several houses, nothing chosen'] = async (ui) => { click(ui, '[data-fk="b-fill"]'); };

async function play(name) {
  const a = synthetic();
  const ui = await boot({ get: { '/api/landsraad': a }, post: { '/api/landsraad/apply': OK }, local: { tab: 'landsraad', 'lsb.view': 'board', 'lsb.layout': 'grid' } });
  await ui.settle();
  await SCENARIOS[name](ui, a);
  await ui.settle();
  const out = { sent: ui.posted.filter((p) => p.path === '/api/landsraad/apply').map((p) => p.body), others: ui.posted.filter((p) => p.path !== '/api/landsraad/apply').map((p) => p.path), asked: text(ui.doc.querySelector('#lsb-confirm b')), said: text(ui.doc.querySelector('#lsb-result')) };
  closeAll();
  return out;
}

const golden = RECORD || !fs.existsSync(GOLDEN) ? {} : JSON.parse(fs.readFileSync(GOLDEN, 'utf8'));
const recorded = {};
for (const name of Object.keys(SCENARIOS)) {
  const phone = name.endsWith('(the phone bar)');
  test('the same request, question and result line as before: ' + name, async (t) => {
    const key = name.replace(' (the phone bar)', '');
    if (phone && RECORD) { t.skip('the phone bar did not exist when this was recorded'); return; }
    const got = await play(name);
    if (RECORD) { recorded[key] = got; fs.writeFileSync(GOLDEN, JSON.stringify(Object.fromEntries(Object.keys(recorded).sort().map((k) => [k, recorded[k]])), null, 1) + '\n'); return; }
    assert.ok(golden[key], 'a recorded scenario for ' + key);
    assert.deepEqual(got, golden[key]);
    if (!/nothing chosen/.test(name)) assert.ok(got.sent.length === 1 || got.asked !== null, 'a write was made or a question was asked');
  });
}
