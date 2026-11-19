// Two display-only rules of the Command Center Term and Decrees section (the page, no save write):
//  1. the plain SpecialVendorActive decree is shown nowhere (operator order); a term that holds it says "Another decree" and Apply writes the same id back;
//  2. on a phone the section comes after the board and is one closed disclosure (#933); on a wide window it is open and its summary is not drawn.
// Real app.js and lcc scripts over the synthetic answer of M1; no game save is read.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { boot, closeAll } from './boot.mjs';
import { synthetic } from './lcc-fixture.mjs';
import { lccDir } from './lcc-load.mjs';

afterEach(closeAll);
const text = (el) => (el ? el.textContent.replace(/\s+/g, ' ').trim() : '');
const HIDDEN = 'SpecialVendorActive';
async function open(answer = synthetic(), pre) {
  const ui = await boot({ get: { '/api/landsraad': answer }, post: { '/api/landsraad/term': { ok: true } }, local: { tab: 'landsraad', 'lsb.view': 'board', 'lsb.layout': 'grid' }, pre });
  await ui.settle();
  return ui;
}
const click = (ui, el) => { if (typeof el === 'string') el = ui.doc.querySelector(el); assert.ok(el, 'no element'); el.focus(); el.dispatchEvent(new ui.w.MouseEvent('click', { bubbles: true, cancelable: true })); };
const noPlain = (s) => !/Special Vendor/.test(s) && !/SpecialVendorActive(?!_)/.test(s);

test('the plain SpecialVendorActive row is in the data and nowhere on the board view: no row, no closed list, no drop-down entry, no wording', async () => {
  const a = synthetic();
  const plain = a.decrees.find((d) => d.decree_name === HIDDEN);
  assert.ok(plain, 'the synthetic save holds the plain row (the data keeps it)');
  const ui = await open(a);
  const sec = ui.doc.querySelector('#lcc-term');
  assert.equal(sec.querySelector('#lsb-dphid'), null, 'the closed list under the pool is gone');
  assert.equal(sec.querySelectorAll('.lcc-tm-row').length, a.decrees.length - 1, 'every decree but the plain one is a row');
  assert.equal(sec.querySelector(`[data-decree="${plain.id}"]`), null);
  assert.equal(sec.querySelector(`#lsb-tad option[value="${plain.id}"]`), null, 'not in the Active decree drop-down');
  assert.ok(noPlain(ui.doc.body.textContent), 'no Special Vendor wording and no plain id anywhere on the page');
  assert.match(text(sec.querySelector('#lsb-tdp > summary')), new RegExp('Decree pool · ' + (a.decrees.length - 1) + ' decrees'), 'the count leaves it out');
  // the four category vendors stay
  for (const n of ['Vehicle', 'Weapon', 'Armor', 'Tool']) assert.ok(text(sec).includes(`Access to a Landsraad ${n} Vendor`), n + ': the effect stays');
  assert.equal(sec.querySelectorAll('.lcc-tm-did, .did').length, 0, 'no line for a game id (#1040)');
  assert.ok(!/[A-Za-z]+_[A-Z][A-Za-z]+/.test(text(sec)) && !sec.outerHTML.includes(`${HIDDEN}_`), 'no game id in the text, a title or a label of the section (#1040)');
});

test('a term that holds the plain row says "Another decree" in the facts, the drop-down and the plaque; Apply writes the same id back', async () => {
  const a = synthetic();
  const plain = a.decrees.find((d) => d.decree_name === HIDDEN);
  a.term.active_decree_id = plain.id; a.term.elected_decree_id = plain.id;
  const ui = await open(a);
  const sec = ui.doc.querySelector('#lcc-term');
  const facts = [...sec.querySelectorAll('.lcc-tm-fact')].map(text).join(' | ');
  assert.match(facts, /ACTIVE DECREEAnother decree/);
  assert.match(facts, /Another decree/);
  assert.ok(noPlain(ui.doc.body.textContent), 'no plain name or id anywhere');
  const sel = sec.querySelector('#lsb-tad');
  assert.equal(sel.value, String(plain.id), 'the choice stays on the held decree, so the drop-down never falls back to none');
  assert.equal(text(sel.querySelector(`option[value="${plain.id}"]`)), 'Another decree');
  assert.match(text(ui.doc.querySelector('.lcc-pl-title')), /Another decree/, 'the plaque never reads as having no decree');
  // a draft that moves away from it names it the same way
  const other = a.decrees.find((d) => d.decree_name !== HIDDEN && d.id !== plain.id);
  sel.value = String(other.id); sel.dispatchEvent(new ui.w.Event('change', { bubbles: true }));
  assert.match(text(ui.doc.querySelector('#lcc-tm-pending')), /the term still has Another decree as its active decree/);
  assert.ok(noPlain(ui.doc.body.textContent), 'the draft line names no plain row either');
  sel.value = String(plain.id); sel.dispatchEvent(new ui.w.Event('change', { bubbles: true }));
  click(ui, '[data-lb="tapply"]');
  await ui.settle();
  const sent = ui.posted.filter((p) => p.path === '/api/landsraad/term').map((p) => p.body);
  assert.equal(sent.length, 1);
  assert.equal(String(sent[0].active_decree_id), String(plain.id), 'the decree is written back as the save holds it, never as none');
});

test('wide: the whole section is one open disclosure whose summary is not drawn; the sections under it keep their own closed state', async () => {
  const ui = await open();
  const all = ui.doc.querySelector('#lcc-tm-all');
  assert.ok(all && all.tagName === 'DETAILS' && all.open, 'open without a phone');
  assert.match(text(all.querySelector(':scope > summary')), /^Term \d+ · .+(Show|Hide)$/);
  assert.ok(all.querySelector('.lcc-tm-facts') && all.querySelector('#lsb-tad') && all.querySelector('#lsb-tdp'), 'facts, controls and pool are inside');
  assert.ok(!ui.doc.querySelector('#lsb-tdp').open, 'the pool stays closed');
  assert.equal(ui.doc.querySelector('.lcc-tm-surface > .lcc-tm-head').nextElementSibling, all, 'the heading stays outside, above the disclosure');
});

test('phone: the section is closed until Show; the choice survives a redraw; a wider window opens it again', async () => {
  const listeners = [];
  const ui = await open(synthetic(), (w) => {
    const mq = { matches: true, media: '(max-width: 640px)', addEventListener: (t, f) => listeners.push(f), removeEventListener() {} };
    w.matchMedia = () => mq;
    w.__mq = mq;
  });
  const all = () => ui.doc.querySelector('#lcc-tm-all');
  assert.ok(all() && !all().open, 'closed by default on a phone');
  assert.match(text(all().querySelector(':scope > summary')), /Show$/);
  assert.match(text(all().querySelector(':scope > summary')), /^Term \d+ · \w+ · /, 'the line says the term, the faction and the decree');
  all().open = true;
  all().dispatchEvent(new ui.w.Event('toggle'));
  ui.doc.querySelector('#lsb-trf'); // a redraw: choose the other faction
  const sf = ui.doc.querySelector('#lsb-trf'); sf.value = '2'; sf.dispatchEvent(new ui.w.Event('change', { bubbles: true }));
  await ui.settle();
  assert.ok(all().open, 'the person opened it: it stays open over a redraw');
  all().open = false;
  all().dispatchEvent(new ui.w.Event('toggle'));
  ui.w.__mq.matches = false;
  listeners.forEach((f) => f());
  assert.ok(all().open, 'a window wider than a phone shows it open again');
});

test('the phone order and the disclosure are in the style files: the section after the board, the summary only on a phone, content kept visible when wide', () => {
  const board = fs.readFileSync(path.join(lccDir, 'lcc-board.css'), 'utf8').replace(/\r/g, '').replace(/\/\*[\s\S]*?\*\//g, '');
  const term = fs.readFileSync(path.join(lccDir, 'lcc-term.css'), 'utf8').replace(/\r/g, '').replace(/\/\*[\s\S]*?\*\//g, '');
  assert.match(board, /@media \(max-width: 640px\) \{\s*\.lcc-bd \{ display: flex; flex-direction: column; \}[^@]*\.lcc-bd-termslot \{ order: 9;/);
  assert.match(term, /\.lcc-tm \.lcc-tm-all > summary \{ display: none; \}/, 'wide: no summary');
  assert.match(term, /\.lcc-tm \.lcc-tm-all:not\(\[open\]\)::details-content \{ content-visibility: visible; \}/, 'wide: nothing is hidden even if a script did not open it');
  assert.match(term, /@media \(max-width: 640px\) \{\s*\.lcc-tm \.lcc-tm-all > summary \{ display: flex;[^}]*min-height: 44px;/, 'phone: a 44 px summary');
  assert.match(term, /\.lcc-tm \.lcc-tm-all > summary:focus-visible \{ outline: 3px solid var\(--lcc-focus\)/);
});
