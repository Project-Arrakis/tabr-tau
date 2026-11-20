// #1083 and #1081, and the leftover of #1069, in the page's own code (jsdom) with the synthetic answer of M1 and no game save.
//   #1083: the owner line of a House card and the pledge cell of the list have no bullet or circle before the words; the card hover and the Inspector say "House Harkonnen"; the line keeps off the card
//          edge (a CSS rule test here; the real fit, a text range against its card in a real browser engine, is measured by scripts/lcc-m7-sweep.mjs).
//   #1081: the empty Inspector hint follows the view.
//   #1069: a place with no pin says "no picture of X yet" only when there is no picture, and "X is outside the picture of Y" when the picture exists.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import { boot, closeAll, } from './boot.mjs';
import { synthetic } from './lcc-fixture.mjs';
import { load, read } from './lcc-load.mjs';

afterEach(closeAll);
const text = (el) => (el ? el.textContent.replace(/\s+/g, ' ').trim() : '');
const idOf = (a, name) => a.tasks.find((t) => t.house_name === 'DA_House' + name).id;
async function open(layout, mod, view = 'board') {
  const a = synthetic();
  if (mod) mod(a);
  const ui = await boot({ get: { '/api/landsraad': a }, local: { tab: 'landsraad', 'lsb.view': view, 'lsb.layout': layout } });
  await ui.settle();
  return { ui, a };
}
const cardOf = (ui, name) => [...ui.doc.querySelectorAll('#lsb-lcc .lcc-hc')].find((c) => text(c.querySelector('.lcc-hc-name')).endsWith(name));
const ownerOf = (ui, name) => cardOf(ui, name).querySelector('.lcc-hc-owner');

test('#1083: no bullet or circle before the owner line of a card: the words alone, the faction colour on the pledged ones', async () => {
  const { ui } = await open('grid');
  assert.equal(text(ownerOf(ui, 'Vernius')), 'Pledged to Harkonnen');
  assert.equal(text(ownerOf(ui, 'Richese')), 'Pledged to Atreides');
  assert.equal(text(ownerOf(ui, 'Kenola')), 'Wavering');
  for (const o of ui.doc.querySelectorAll('#lsb-lcc .lcc-hc-owner')) assert.doesNotMatch(o.textContent, /[●○]/, 'no dot in "' + o.textContent + '"');
  assert.ok(ownerOf(ui, 'Vernius').closest('.lcc-hc').classList.contains('lcc-hc-h'), 'the colour comes from the card class: red text for Harkonnen');
  assert.ok(ownerOf(ui, 'Richese').closest('.lcc-hc').classList.contains('lcc-hc-a'));
});

test('#1083: no circle before Undecided or Wavering in the list either, and the hover keeps the words', async () => {
  const { ui } = await open('list');
  const cells = [...ui.doc.querySelectorAll('#lsb-lcc .lcc-bd-c-owner .lcc-owner')];
  assert.ok(cells.length >= 20, 'the list has its pledge cells');
  for (const c of cells) assert.doesNotMatch(c.textContent, /[●○]/, 'no dot in "' + c.textContent + '"');
  assert.ok(cells.some((c) => text(c) === 'Wavering'));
  assert.ok(cells.some((c) => text(c) === 'Harkonnen' && c.getAttribute('title') === 'Pledged to Harkonnen'));
});

test('#1083: the card hover and the Inspector say "House Harkonnen" / "House Atreides"; the card itself stays short', async () => {
  const { ui, a } = await open('grid');
  const ttl = (n) => { const e = cardOf(ui, n).querySelector('.lcc-hc-select'); return e.getAttribute('data-says') || e.getAttribute('title'); };
  assert.match(ttl('Vernius'), /^House Vernius, pledged to House Harkonnen\. /);
  assert.match(ttl('Kenola'), /^House Kenola, wavering\. /);
  assert.doesNotMatch(text(ownerOf(ui, 'Vernius')), /House/, 'a card reads "House" as its own House: the line has no room for it');
  const pick = async (n) => { ui.doc.querySelector('[data-fk="ht:' + idOf(a, n) + '"]').click(); await ui.settle(); return ui.doc.querySelector('.lcc-bd-insp .lcc-bd-banner'); };
  const b = await pick('Vernius');
  assert.equal(text(b.querySelector('.lcc-bd-control')), 'PLEDGED TO HOUSE HARKONNEN');
  assert.match(b.querySelector('.lcc-bd-control').getAttribute('title'), /^Pledged to House Harkonnen \(completed/);
  assert.match(text(b.querySelector('.lcc-bd-sr')), /^Pledged to House Harkonnen/);
  assert.equal(text((await pick('Kenola')).querySelector('.lcc-bd-control')), 'WAVERING');
});

test('#1083: the owner line keeps off the card edge and never clips (CSS rules; the measured fit is in the sweep)', () => {
  const css = read('internal', 'web', 'static', 'lcc', 'lcc-board.css').replace(/\/\*[\s\S]*?\*\//g, '');
  const rules = [...css.matchAll(/(?:^|\})\s*\.lcc-hc-owner \{([^}]*)\}/g)].map((m) => m[1]);
  const win = rules[rules.length - 1]; // the last rule of the selector is the one that wins
  assert.match(win, /padding:\s*0 (\d+)px/, 'side padding');
  assert.ok(Number(win.match(/padding:\s*0 (\d+)px/)[1]) >= 4, 'at least 4 px each side');
  assert.match(win, /box-sizing:\s*border-box/);
  assert.match(win, /max-width:\s*100%/);
  assert.match(win, /white-space:\s*normal/, 'a card too narrow for the line breaks it between the words');
  assert.doesNotMatch(win, /text-overflow:\s*(clip|ellipsis)|overflow:\s*(hidden|clip)|white-space:\s*nowrap/, 'no clip, no ellipsis, no forced single line');
});

// ---- #1081 -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------
const hintOf = (ui) => text(ui.doc.querySelector('#lsb-lcc .lcc-bd-insp > .lcc-bd-mut'));
test('#1081: the empty Inspector hint follows the view: on the board in the grid, in the list in the list, and no "what each action would do"', async () => {
  const g = await open('grid');
  assert.equal(hintOf(g.ui), 'Choose a House on the board to see its tiers and progress.');
  const l = await open('list');
  assert.equal(hintOf(l.ui), 'Choose a House in the list to see its tiers and progress.');
  for (const s of [hintOf(g.ui), hintOf(l.ui)]) assert.doesNotMatch(s, /action|or in the list|on the board or/);
});
test('#1081: switching the view changes the hint, and Classic has no such sentence', async () => {
  const { ui } = await open('grid');
  ui.doc.querySelector('#lsb-lcc [data-lcb="layout"][data-val="list"]').click();
  await ui.settle();
  assert.equal(hintOf(ui), 'Choose a House in the list to see its tiers and progress.');
  ui.doc.querySelector('#lsb-lcc [data-lcb="layout"][data-val="grid"]').click();
  await ui.settle();
  assert.equal(hintOf(ui), 'Choose a House on the board to see its tiers and progress.');
  const classic = await open('grid', null, 'classic');
  assert.doesNotMatch(text(classic.ui.doc.body), /Choose a House on the board|its tiers, its progress|what each action would do/);
});

// ---- #1069 leftover: the tooltip of a place with no pin ---------------------------------------------------------------------------------------------------------------------------------------
const NO_PIC = (n) => 'The Saved map has no picture of ' + n + ' yet';
test('a place with no pin says "no picture of X yet" only when there is no picture, and "X is outside the picture of Y" when the picture exists', async () => {
  const { w } = load();
  const says = w.LCCM.noPictureSays;
  assert.equal(says({ map: 'Caladan', area: 'Caladan' }), NO_PIC('Caladan') + '.');
  assert.equal(says({ map: 'Hagga Basin', area: 'Imperial Basin' }), 'Imperial Basin is outside the picture of Hagga Basin.');
  assert.equal(says({ map: 'Hagga Basin', area: 'Hagga Basin' }), 'This place is outside the picture of Hagga Basin.');
  assert.equal(says({ map: 'Deep Desert', area: 'Deep Desert' }), 'This place is outside the picture of Deep Desert.');
  assert.equal(says({ map: 'Harko Village', area: 'Harko Village' }), 'This place is outside the picture of Harko Village.');
  assert.equal(says({}), 'The Saved map has no picture of this place yet.');
});
test('the list link tooltip, the Inspector note and the Classic hover say it the same, in both cases', async () => {
  const { ui, a } = await open('list', (x) => {
    const set = (n, l) => { x.tasks.find((t) => t.house_name === 'DA_House' + n).location = { known: true, cell: '', place: '', unconfirmed: false, map_key: '', ...l }; };
    set('Sor', { map: 'Caladan', area: 'Caladan' }); // no picture at all
    set('Thorvald', { map: 'Hagga Basin', area: 'Imperial Basin' }); // the picture exists, the place lies off it
  });
  const row = (n) => [...ui.doc.querySelectorAll('#lsb-lcc .lcc-bd-row')].find((r) => text(r.querySelector('.lcc-bd-namebtn')) === n);
  assert.equal(row('Sor').querySelector('.lcc-bd-nopic').getAttribute('title'), NO_PIC('Caladan'));
  assert.equal(row('Thorvald').querySelector('.lcc-bd-nopic').getAttribute('title'), 'Imperial Basin is outside the picture of Hagga Basin');
  for (const [n, say] of [['Sor', NO_PIC('Caladan') + '.'], ['Thorvald', 'Imperial Basin is outside the picture of Hagga Basin.']]) {
    ui.doc.querySelector('[data-fk="ht:' + idOf(a, n) + '"]').click();
    await ui.settle();
    assert.ok([...ui.doc.querySelectorAll('#lsb-lcc .lcc-bd-insp .lcc-bd-cap')].some((p) => text(p) === say), n + ': the Place note says ' + say);
  }
  // the Classic hover (app.js has its own copy of the words)
  const classic = await open('grid', (x) => {
    x.tasks.find((t) => t.house_name === 'DA_HouseSor').location = { known: true, cell: '', place: '', unconfirmed: false, map_key: '', map: 'Caladan', area: 'Caladan' };
    x.tasks.find((t) => t.house_name === 'DA_HouseThorvald').location = { known: true, cell: '', place: '', unconfirmed: false, map_key: '', map: 'Hagga Basin', area: 'Imperial Basin' };
  }, 'classic');
  const hov = (n) => classic.ui.doc.querySelector('#lsrow-' + idOf(classic.a, n) + ' .loc-t').title;
  assert.equal(hov('Sor'), NO_PIC('Caladan') + '.');
  assert.equal(hov('Thorvald'), 'Imperial Basin is outside the picture of Hagga Basin.');
});
test('the maps the two copies of the words know are the maps of the Go table and of the page', () => {
  const go = read('internal', 'ops', 'landsraad_locations.go').match(/var houseMapKey = map\[string\]string\{([^}]*)\}/)[1];
  const goLabels = [...go.matchAll(/"([^"]+)":/g)].map((m) => m[1]).sort();
  const { w } = load();
  assert.deepEqual([...w.LCCM.PIC_MAPS].sort(), goLabels);
  const app = read('internal', 'web', 'static', 'app.js').match(/const LB_PIC_LABELS = \[([^\]]*)\]/)[1];
  assert.deepEqual([...app.matchAll(/'([^']+)'/g)].map((m) => m[1]).sort(), goLabels);
});
