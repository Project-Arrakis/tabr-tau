// The quick actions of a row of the board's List layout (#1028, ported from the Classic list's rows when that list was retired): Fill, and Complete or Reopen. They are the Inspector's own controls
// (data-lb="qa"), so a click sends the request the Inspector's button sends, which is the request recorded in lcc-board-requests.golden.json (recorded from the Classic list). This file presses the row
// button on the List layout and compares the request, the question and the result line with those recorded ones, and the on and off of every row button with the Inspector's button for the same House.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { boot, closeAll } from './boot.mjs';
import { synthetic, questionText } from './lcc-fixture.mjs';

afterEach(closeAll);

const here = path.dirname(fileURLToPath(import.meta.url));
const golden = JSON.parse(fs.readFileSync(path.join(here, 'lcc-board-requests.golden.json'), 'utf8'));
const OK = { ok: true, changed: 1, houses: 1, skipped: 0, contribution_raised: 1, contribution_lowered: 0, completed: 0, reopened: 0, reward_rows: 0, outcome: 'written' };
const select = (ui, sel, v) => { const el = ui.doc.querySelector(sel); el.value = String(v); el.dispatchEvent(new ui.w.Event('change', { bubbles: true })); };
const text = (el) => (el ? el.textContent.replace(/\s+/g, ' ').trim() : null);
const click = (ui, el) => { if (typeof el === 'string') el = ui.doc.querySelector(el); assert.ok(el, 'no element'); el.focus(); el.dispatchEvent(new ui.w.MouseEvent('click', { bubbles: true, cancelable: true })); };
const idOf = (a, name) => a.tasks.find((t) => t.house_name === 'DA_House' + name).id;
const toastSaid = (ui) => { const t = [...ui.doc.querySelectorAll('#toast .d-toast')].pop(); return t ? text(t.querySelector('span')) : ''; };
const same = (s) => String(s).replace(/[.;,:]/g, '').replace(/\s+/g, ' ').trim();
const open = async (a, post) => { const ui = await boot({ get: { '/api/landsraad': a }, post: { '/api/landsraad/apply': post || OK }, local: { tab: 'landsraad', 'lsb.view': 'board', 'lsb.layout': 'list' } }); await ui.settle(); return ui; };

for (const [name, house, fk, was] of [
  ['Fill', 'Lindaren', 'rf:', 'Fill 5 tiers'],
  ['Complete', 'Sor', 'rc:', 'Complete House'],
  ['Reopen', 'Hurata', 'rc:', 'Reopen'],
]) {
  test('a row button of the List (' + name + ' on House ' + house + ') sends the recorded request of the ' + was + ' button', async () => {
    const a = synthetic(), ui = await open(a);
    const b = ui.doc.querySelector('.lcc-bd-list tr[data-id="' + idOf(a, house) + '"] [data-fk="' + fk + idOf(a, house) + '"]');
    assert.ok(b, 'the row has the button');
    assert.equal(b.getAttribute('data-lb'), 'qa');
    assert.match(b.getAttribute('aria-label'), new RegExp('^' + name + ' House '));
    click(ui, b);
    await ui.settle();
    const sent = ui.posted.filter((p) => p.path === '/api/landsraad/apply').map((p) => p.body);
    assert.deepEqual(sent, golden[was].sent, 'the same request, byte for byte');
    assert.deepEqual(ui.posted.filter((p) => p.path !== '/api/landsraad/apply').map((p) => p.path), golden[was].others);
    assert.equal(questionText(ui.doc), golden[was].asked, 'the same question (or none)');
    assert.equal(same(toastSaid(ui)), same(golden[was].said), 'the same result line');
  });
}

test('every row button is on or off as the Inspector\'s button for the same House, and a row says Reopen exactly where the House is pledged', async () => {
  const a = synthetic(), ui = await open(a);
  let n = 0;
  for (const t of a.tasks) {
    const row = ui.doc.querySelector('.lcc-bd-list tr[data-id="' + t.id + '"]');
    assert.ok(row, 'a row for every House');
    click(ui, '[data-fk="ht:' + t.id + '"]');
    await ui.settle();
    const insp = ui.doc.querySelector('.lcc-bd-insp');
    assert.equal(insp.dataset.id, String(t.id));
    const fill = row.querySelector('[data-fk="rf:' + t.id + '"]'), second = row.querySelector('[data-fk="rc:' + t.id + '"]');
    assert.equal(fill.getAttribute('data-do'), 'fill');
    assert.equal(fill.getAttribute('aria-disabled'), insp.querySelector('[data-do="fill"]').getAttribute('aria-disabled'), 'Fill on and off for ' + t.house_name);
    const reopen = insp.querySelector('[data-do="reopen"]'), reopens = reopen.getAttribute('aria-disabled') === 'false';
    assert.equal(second.getAttribute('data-do'), reopens ? 'reopen' : 'complete', 'the toggle for ' + t.house_name);
    assert.equal(second.getAttribute('aria-disabled'), insp.querySelector('[data-do="' + (reopens ? 'reopen' : 'complete') + '"]').getAttribute('aria-disabled'));
    assert.doesNotMatch(row.querySelector('.lcc-bd-c-qa').textContent + [...row.querySelectorAll('.lcc-bd-c-qa [title]')].map((e) => e.getAttribute('title')).join(' '), /\btaken\b/i, 'the board says pledged');
    n++;
  }
  assert.equal(n, 25);
});

test('the row buttons sit in their own column, the heading says Actions, and the group rows and the empty row span it', async () => {
  const a = synthetic(), ui = await open(a);
  const heads = [...ui.doc.querySelectorAll('.lcc-bd-list thead th')];
  assert.equal(text(heads[heads.length - 1]), 'Actions');
  const cells = ui.doc.querySelector('.lcc-bd-list tbody tr.lcc-bd-row').querySelectorAll('td').length;
  assert.equal(cells, heads.length, 'a row has a cell under every heading');
});

// The row buttons act for the faction chosen on the page, which may not be the person's own and whose crests are hidden in the List (#1028 review). The name of Complete says for whom, and a line above the table says it too.
const rowOf = (ui, a, house) => ui.doc.querySelector('.lcc-bd-list tr[data-id="' + idOf(a, house) + '"]');
test('the List says which faction the row buttons act for, and Complete names it when it is not the person\'s own', async () => {
  const a = synthetic(), ui = await open(a);
  const line = () => text(ui.doc.querySelector('.lcc-bd-rowfac'));
  assert.match(line(), /^Row actions act for \S+\.$/, 'the line is there with the own faction');
  const own = rowOf(ui, a, 'Sor').querySelector('[data-fk="rc:' + idOf(a, 'Sor') + '"]');
  assert.doesNotMatch(own.getAttribute('aria-label'), / for /, 'the own faction needs no words');
  select(ui, '#lsb-fac', 2);
  await ui.settle();
  assert.equal(line(), 'Row actions act for Harkonnen.');
  const b = rowOf(ui, a, 'Sor').querySelector('[data-fk="rc:' + idOf(a, 'Sor') + '"]');
  assert.match(b.getAttribute('aria-label'), /^Complete House .+ for Harkonnen$/);
  assert.match(b.getAttribute('title'), /^Complete for Harkonnen/, 'the visible mouse-over says it too');
  const r = rowOf(ui, a, 'Hurata').querySelector('[data-fk="rc:' + idOf(a, 'Hurata') + '"]');
  assert.doesNotMatch(r.getAttribute('aria-label'), / for /, 'Reopen is the same for every faction and says nothing of one');
});

test('Complete on a row for another faction sends the request the Inspector\'s Complete sends for that faction, unchanged', async () => {
  const a = synthetic(), ui = await open(a);
  select(ui, '#lsb-fac', 2);
  await ui.settle();
  click(ui, rowOf(ui, a, 'Sor').querySelector('[data-fk="rc:' + idOf(a, 'Sor') + '"]'));
  await ui.settle();
  const viaRow = ui.posted.filter((p) => p.path === '/api/landsraad/apply').map((p) => p.body);
  assert.equal(viaRow.length, 1);
  assert.equal(JSON.stringify(viaRow[0]).includes('"complete_for":2'), true);
  const b = synthetic(), ui2 = await open(b);
  select(ui2, '#lsb-fac', 2);
  await ui2.settle();
  click(ui2, '[data-fk="ht:' + idOf(b, 'Sor') + '"]');
  click(ui2, '.lcc-bd-insp .lcc-bd-act[data-do="complete"]');
  await ui2.settle();
  const viaInspector = ui2.posted.filter((p) => p.path === '/api/landsraad/apply').map((p) => p.body);
  assert.deepEqual(viaRow, viaInspector, 'the same request, byte for byte');
});

test('the Reopen mouse-over says that Reopen writes or removes no reward, and Help says it too', async () => {
  const a = synthetic(), ui = await open(a);
  const r = rowOf(ui, a, 'Hurata').querySelector('[data-fk="rc:' + idOf(a, 'Hurata') + '"]');
  assert.match(r.getAttribute('title'), /Reopen writes or removes no reward\./);
  const help = fs.readFileSync(path.join(here, '..', 'docs', 'help', 'fill-and-tiers.md'), 'utf8');
  assert.match(help, /Reopen writes or removes no reward\./);
});
