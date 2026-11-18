// Every control of the Command Center's Term and Decrees section that writes still sends exactly what the classic panel's control sends. The expected requests, the result lines and the focus after
// the write were recorded from the CLASSIC panel (web-tests/lcc-term-requests.golden.json); this file plays the same scenarios against the classic panel and against the Command Center section and
// compares them one by one. Real page code, the synthetic answer of M1, no game save. Set LCC_RECORD_REQUESTS=1 to write the file again (only ever done on a tree that is known to be right, and only
// from the classic view).
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { boot, closeAll } from './boot.mjs';
import { synthetic } from './lcc-fixture.mjs';

afterEach(closeAll);

const here = path.dirname(fileURLToPath(import.meta.url));
const GOLDEN = path.join(here, 'lcc-term-requests.golden.json');
const RECORD = process.env.LCC_RECORD_REQUESTS === '1';
const text = (el) => (el ? el.textContent.replace(/\s+/g, ' ').trim() : null);
const click = (ui, sel) => { const el = typeof sel === 'string' ? ui.doc.querySelector(sel) : sel; assert.ok(el, 'no element ' + sel); el.focus(); el.dispatchEvent(new ui.w.MouseEvent('click', { bubbles: true, cancelable: true })); };
const select = (ui, sel, v) => { const el = ui.doc.querySelector(sel); assert.ok(el, 'no element ' + sel); el.value = String(v); el.dispatchEvent(new ui.w.Event('change', { bubbles: true })); };
const WRITES = ['/api/landsraad/term', '/api/landsraad/decree'];

const SCENARIOS = {
  'Apply as it stands': async (ui) => { click(ui, '[data-lb="tapply"]'); },
  'Apply with another active decree': async (ui) => { select(ui, '#lsb-tad', 3); click(ui, '[data-lb="tapply"]'); },
  'Apply with the other faction': async (ui) => { select(ui, '#lsb-trf', 2); await ui.settle(); click(ui, '[data-lb="tapply"]'); },
  'Apply with another decree and the other faction': async (ui) => { select(ui, '#lsb-tad', 4); select(ui, '#lsb-trf', 2); await ui.settle(); click(ui, '[data-lb="tapply"]'); },
  'Apply with no decree and no faction': async (ui) => { select(ui, '#lsb-tad', ''); select(ui, '#lsb-trf', ''); await ui.settle(); click(ui, '[data-lb="tapply"]'); },
  'Make active on a row': async (ui) => { click(ui, '[data-lb="tmk"][data-id="4"]'); },
  'Make active on a row for the other faction': async (ui) => { select(ui, '#lsb-trf', 2); await ui.settle(); click(ui, '[data-lb="tmk"][data-id="6"]'); },
  'Make active on the hidden vendor row': async (ui) => { click(ui, '[data-lb="tmk"][data-id="11"]'); },
  'Make active with no faction chosen': async (ui) => { select(ui, '#lsb-trf', ''); await ui.settle(); click(ui, '[data-lb="tmk"][data-id="4"]'); },
  'Set the pool flag to 1': async (ui) => { click(ui, '[data-lb="tflag"][data-id="4"]'); },
  'Set the pool flag to 0': async (ui) => { click(ui, '[data-lb="tflag"][data-id="7"]'); },
  'Set the pool flag of the hidden row': async (ui) => { click(ui, '[data-lb="tflag"][data-id="11"]'); },
  'Apply, refused by the save': async (ui) => { select(ui, '#lsb-tad', 5); click(ui, '[data-lb="tapply"]'); },
  'Set the pool flag, refused by the save': async (ui) => { click(ui, '[data-lb="tflag"][data-id="2"]'); },
};
const REFUSED = { 'Apply, refused by the save': '/api/landsraad/term', 'Set the pool flag, refused by the save': '/api/landsraad/decree' };

async function play(name, view) {
  const a = synthetic();
  const post = { '/api/landsraad/term': { ok: true }, '/api/landsraad/decree': { ok: true } };
  if (REFUSED[name]) post[REFUSED[name]] = { __fail: 'the save changed while the page was open', __code: 'conflict' };
  const ui = await boot({ get: { '/api/landsraad': a }, post, local: { tab: 'landsraad', 'lsb.view': view, 'lsb.layout': 'grid' } });
  await ui.settle();
  await SCENARIOS[name](ui);
  await ui.settle();
  const act = ui.doc.activeElement;
  const out = {
    sent: ui.posted.filter((p) => WRITES.includes(p.path)).map((p) => ({ path: p.path, body: p.body })),
    others: ui.posted.filter((p) => !WRITES.includes(p.path)).map((p) => p.path),
    result: text(ui.doc.querySelector('#lsb-tresult')),
    focus: act && act.dataset ? act.dataset.fk || null : null,
    chose: { decree: ui.doc.querySelector('#lsb-tad') ? ui.doc.querySelector('#lsb-tad').value : null, faction: ui.doc.querySelector('#lsb-trf') ? ui.doc.querySelector('#lsb-trf').value : null },
  };
  closeAll();
  return out;
}

const golden = RECORD || !fs.existsSync(GOLDEN) ? {} : JSON.parse(fs.readFileSync(GOLDEN, 'utf8'));
const recorded = {};
for (const name of Object.keys(SCENARIOS)) {
  if (RECORD) {
    test('record, from the classic panel: ' + name, async () => {
      recorded[name] = await play(name, 'classic');
      fs.writeFileSync(GOLDEN, JSON.stringify(Object.fromEntries(Object.keys(recorded).sort().map((k) => [k, recorded[k]])), null, 1) + '\n');
    });
    continue;
  }
  for (const view of ['classic', 'board']) {
    test('the same request, result line and focus as the classic panel (' + view + ' view): ' + name, async () => {
      assert.ok(golden[name], 'a recorded scenario for ' + name);
      const got = await play(name, view);
      assert.deepEqual(got, golden[name]);
      if (!/no faction chosen/.test(name)) assert.ok(got.sent.length === 1, 'one write was made');
      else assert.equal(got.sent.length, 0, 'nothing is sent when Make active is not allowed');
    });
  }
}

test('the board view draws the Command Center section and not the classic panel; the classic view draws the classic panel', async () => {
  const a = synthetic();
  const board = await boot({ get: { '/api/landsraad': a }, local: { tab: 'landsraad', 'lsb.view': 'board' } });
  await board.settle();
  assert.ok(board.doc.querySelector('#lcc-term.lcc-tm'), 'the section is drawn');
  assert.equal(board.doc.querySelectorAll('#lsb-tad').length, 1, 'one set of controls, never two');
  assert.equal(board.doc.querySelector('.lsb .tctl'), null, 'no classic panel beside it');
  closeAll();
  const classic = await boot({ get: { '/api/landsraad': a }, local: { tab: 'landsraad', 'lsb.view': 'classic' } });
  await classic.settle();
  assert.equal(classic.doc.querySelector('#lcc-term'), null);
  assert.equal(classic.doc.querySelectorAll('#lsb-tad').length, 1);
  assert.ok(classic.doc.querySelector('.lsb .tctl'));
});

test('a decree is only written by Apply, Make active and Set flag: choosing in a drop-down, opening the pool or the earlier terms sends nothing', async () => {
  const a = synthetic();
  const ui = await boot({ get: { '/api/landsraad': a }, local: { tab: 'landsraad', 'lsb.view': 'board' } });
  await ui.settle();
  select(ui, '#lsb-tad', 3);
  select(ui, '#lsb-trf', 2);
  await ui.settle();
  for (const d of ui.doc.querySelectorAll('#lcc-term details')) d.open = true;
  assert.equal(ui.posted.length, 0, 'nothing was sent: ' + JSON.stringify(ui.posted));
});
