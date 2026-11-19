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
// What the server answers to an Unfill: a lowering (#1013). The canned raise-shaped answer above made the recorded sentence of the several-Houses Unfill say "raised", which no Unfill says.
const LOWERED = { ...OK, contribution_raised: 0, contribution_lowered: 1 };
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
// #879: several houses chosen but only one would change still asks (the others are left alone); one house chosen writes at once, as before
const ONE_OF_TWO = { fill: ['Sor', 'Hagal'], complete: ['Sor', 'Wallach'], reopen: ['Hurata', 'Sor'] }; // Hagal has all five tiers, Wallach is already Atreides', Sor is not taken
for (const kind of ['fill', 'complete', 'reopen']) {
  for (const via of ['bar', 'phone']) {
    SCENARIOS['two houses chosen, only one would change, ' + kind + ' chosen, asked, then Yes' + (via === 'phone' ? ' (the phone bar)' : '')] = async (ui, a) => {
      pick(ui, a, ONE_OF_TWO[kind]);
      click(ui, bulkBtn(kind, via));
      await ui.settle();
      click(ui, '[data-fk="c-yes"]');
    };
  }
  SCENARIOS['one house chosen, ' + kind + ' chosen, no question'] = async (ui, a) => { pick(ui, a, [ONE_OF_TWO[kind][0]]); click(ui, bulkBtn(kind, 'bar')); };
}
SCENARIOS['several houses, nothing chosen'] = async (ui) => { click(ui, '[data-fk="b-fill"]'); };
// #932: Unfill chosen and Complete and fill chosen. New scenarios only: the request items of the routes are the old ones (Unfill is a removal to 0 that also carries was_taken; Complete and fill is Fill faction goal on each House).
const NEW_PICK = { unfill: ['Alexin', 'Wallach', 'Moritani', 'Sor'], cfill: ['Sor', 'Lindaren', 'Kenola', 'Vernius'] }; // Sor has no contribution of yours: Unfill leaves it out
for (const kind of ['unfill', 'cfill']) {
  for (const via of ['bar', 'phone']) {
    SCENARIOS['several houses, ' + kind + ' chosen, asked, then Yes' + (via === 'phone' ? ' (the phone bar)' : '')] = async (ui, a) => {
      pick(ui, a, NEW_PICK[kind]);
      click(ui, bulkBtn(kind, via));
      await ui.settle();
      click(ui, '[data-fk="c-yes"]');
    };
  }
  SCENARIOS['another faction chosen, ' + kind + ' chosen is off'] = async (ui, a) => { select(ui, '#lsb-fac', 2); await ui.settle(); pick(ui, a, ['Alexin', 'Sor']); click(ui, bulkBtn(kind, 'bar')); };
}
SCENARIOS['one house chosen, unfill chosen, asked, then Yes'] = async (ui, a) => { pick(ui, a, ['Alexin']); click(ui, bulkBtn('unfill', 'bar')); await ui.settle(); click(ui, '[data-fk="c-yes"]'); };
SCENARIOS['one house chosen, cfill chosen, no question'] = async (ui, a) => { pick(ui, a, ['Lindaren']); click(ui, bulkBtn('cfill', 'bar')); };

// The board says its results as a toast, one result per line, not as a line kept on the page (#943). What is said is compared with the recorded sentence without the punctuation that joins the parts.
const toastSaid = (ui) => { const t = [...ui.doc.querySelectorAll('#toast .d-toast')].pop(); return t ? text(t.querySelector('span')) : ''; };
const same = (s) => String(s).replace(/[.;,:]/g, '').replace(/\s+/g, ' ').trim();
async function play(name) {
  const a = synthetic();
  const ui = await boot({ get: { '/api/landsraad': a }, post: { '/api/landsraad/apply': / unfill chosen, asked/.test(name) ? LOWERED : OK }, local: { tab: 'landsraad', 'lsb.view': 'board', 'lsb.layout': 'grid' } });
  await ui.settle();
  await SCENARIOS[name](ui, a);
  await ui.settle();
  const out = { sent: ui.posted.filter((p) => p.path === '/api/landsraad/apply').map((p) => p.body), others: ui.posted.filter((p) => p.path !== '/api/landsraad/apply').map((p) => p.path), asked: text(ui.doc.querySelector('#lsb-confirm b')), said: toastSaid(ui) };
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
    const { said, ...rest } = got, { said: was, ...wasRest } = golden[key];
    assert.deepEqual(rest, wasRest, 'the request, the others and the question are as recorded');
    assert.equal(same(said), same(was), 'the board says the same things as recorded, in lines');
    if (!/nothing chosen| is off$/.test(name)) assert.ok(got.sent.length === 1 || got.asked !== null, 'a write was made or a question was asked');
  });
}

// tau #879: with several houses chosen a bulk button always asks, even when only one of them would change, and the question says the others are left alone. Nothing is written until Yes, and
// then the request is the same single one that the shortcut used to send at once.
const LEFT_ALONE = /1 other chosen house is left alone: nothing to change for it\./;
for (const kind of ['fill', 'complete', 'reopen']) {
  for (const via of ['bar', 'phone']) {
    test('two houses chosen, only one would change: ' + kind + ' chosen asks first, writes nothing until Yes' + (via === 'phone' ? ' (the phone bar)' : ''), async () => {
      const a = synthetic();
      const ui = await boot({ get: { '/api/landsraad': a }, post: { '/api/landsraad/apply': OK }, local: { tab: 'landsraad', 'lsb.view': 'board', 'lsb.layout': 'grid' } });
      await ui.settle();
      pick(ui, a, ONE_OF_TWO[kind]);
      click(ui, bulkBtn(kind, via));
      await ui.settle();
      const q = text(ui.doc.querySelector('#lsb-confirm'));
      assert.ok(q, 'a question is asked');
      assert.match(q, LEFT_ALONE);
      assert.equal(ui.posted.filter((p) => p.path === '/api/landsraad/apply').length, 0, 'nothing is written before the answer');
      click(ui, '[data-fk="c-no"]');
      await ui.settle();
      assert.equal(ui.posted.filter((p) => p.path === '/api/landsraad/apply').length, 0, 'No writes nothing');
      click(ui, bulkBtn(kind, via));
      await ui.settle();
      click(ui, '[data-fk="c-yes"]');
      await ui.settle();
      const sent = ui.posted.filter((p) => p.path === '/api/landsraad/apply').map((p) => p.body);
      assert.equal(sent.length, 1, 'one request after Yes');
      const key = 'two houses chosen, only one would change, ' + kind + ' chosen, asked, then Yes';
      assert.deepEqual(sent, golden[key].sent);
      closeAll();
    });
  }
  test('one house chosen: ' + kind + ' chosen is written at once, with no question (unchanged)', async () => {
    const a = synthetic();
    const ui = await boot({ get: { '/api/landsraad': a }, post: { '/api/landsraad/apply': OK }, local: { tab: 'landsraad', 'lsb.view': 'board', 'lsb.layout': 'grid' } });
    await ui.settle();
    pick(ui, a, [ONE_OF_TWO[kind][0]]);
    click(ui, bulkBtn(kind, 'bar'));
    await ui.settle();
    assert.equal(ui.doc.querySelector('#lsb-confirm'), null, 'no question');
    assert.equal(ui.posted.filter((p) => p.path === '/api/landsraad/apply').length, 1, 'written at once');
    closeAll();
  });
}
