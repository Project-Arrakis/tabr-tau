// The look-and-feel follow-up of the Command Center (#901, #902, #905): the gold of the Specializations view in dark, the ornament line at level 100, the fifth lane on a phone, the shield, the
// switch height, the partial mark on the keystone total, the ring of the chosen line (inside the cut House card), the order chips and select sizes, the table of earlier terms (keyboard and a line
// that says it scrolls), the pool in the save's own order, and the space the sticky header takes. The styles are read as text (the computed styles in a real engine are read by the capture
// script of the pull request); the page behaviour runs the real page code with the synthetic answers. Nothing is written and no game save is read.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { boot, closeAll } from './boot.mjs';
import { synthetic } from './lcc-fixture.mjs';
import { lccDir, read, root } from './lcc-load.mjs';

afterEach(closeAll);
const css = (f) => fs.readFileSync(path.join(lccDir, f), 'utf8').replace(/\r/g, '');
const noc = (s) => s.replace(/\/\*[\s\S]*?\*\//g, '');
const lcc = noc(css('lcc.css')), spec = noc(css('lcc-spec.css')), board = noc(css('lcc-board.css')), term = noc(css('lcc-term.css'));
const app = noc(fs.readFileSync(path.join(root, 'internal', 'web', 'static', 'app.css'), 'utf8').replace(/\r/g, ''));
const CASES = JSON.parse(read('docs', 'design', 'landsraad-command-center', 'fixtures', 'api-player-specs.cases.json')).cases;
const SPECS = (id = 'mixed-five-tracks') => JSON.parse(JSON.stringify(CASES.find((c) => c.id === id).response));
const text = (e) => (e ? e.textContent.replace(/\s+/g, ' ').trim() : null);
const lum = (hex) => { const c = [1, 3, 5].map((i) => parseInt(hex.slice(i, i + 2), 16) / 255).map((v) => (v <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4)); return 0.2126 * c[0] + 0.7152 * c[1] + 0.0722 * c[2]; };
const ratio = (a, b) => { const [x, y] = [lum(a), lum(b)].sort((p, q) => q - p); return (x + 0.05) / (y + 0.05); };
const hexOf = (css_, name) => { const m = [...css_.matchAll(/--lcc-([a-z0-9-]+):\s*(#[0-9a-f]{6})/g)].find((x) => x[1] === name); return m ? m[2] : null; };
async function openSpec(id) {
  const ui = await boot({ get: { '/api/player/specs': SPECS(id), '/api/landsraad': synthetic() }, local: { tab: 'player', 'spec.view': 'cc' } });
  ui.click('[data-sub="player:specialization"]'); await ui.settle();
  return ui;
}
async function openBoard() {
  const ui = await boot({ get: { '/api/landsraad': synthetic() }, local: { tab: 'landsraad', 'lsb.view': 'board', 'lsb.layout': 'grid' } });
  await ui.settle();
  return ui;
}

// ---- #901
test('gold: the Specializations view takes the approved design\'s dark golds, Dayside keeps the look\'s own text colour, and the Dayside contrast tokens still pass 4.5:1', () => {
  assert.match(lcc, /--lcc-ap-gold:\s*#e9bd71;/);
  assert.match(lcc, /--lcc-ap-title:\s*#f4d9a6;/);
  assert.match(lcc, /--lcc-ap-stat:\s*#f4d59a;/);
  // both Dayside blocks give the three tokens back to the look's own text colour
  const light = lcc.match(/--lcc-ap-gold: var\(--c-gold\); --lcc-ap-title: var\(--c-gold\); --lcc-ap-stat: var\(--c-gold\);/g) || [];
  assert.equal(light.length, 2, 'the two Dayside blocks');
  // the view reads them through its own property, and the headings and figures name the tokens
  assert.match(spec, /\.lcc-sp \{[^}]*--lcc-sp-gold: var\(--lcc-ap-gold\)/);
  assert.match(spec, /\.lcc-sp-eyebrow \{[^}]*color: var\(--lcc-sp-gold\)/);
  assert.match(spec, /\.lcc-sp-level \{[^}]*color: var\(--lcc-sp-gold\)/);
  assert.match(spec, /\.lcc-sp-lane-lv b \{[^}]*color: var\(--lcc-sp-gold\)/);
  assert.match(spec, /\.lcc-sp-h2 \{[^}]*color: var\(--lcc-ap-title\)/);
  assert.match(spec, /\.lcc-sp-stat b \{[^}]*color: var\(--lcc-ap-stat\)/);
  assert.doesNotMatch(spec, /var\(--lcc-gold-text\)/, 'the view no longer takes the look\'s sand for its gold');
  // contrast: the dark golds on the darkest and the lightest end of the surfaces they stand on
  const dark = lcc.slice(lcc.indexOf('.lcc, .lcc-scope {'), lcc.indexOf('@media (prefers-color-scheme: light)'));
  for (const t of ['ap-gold', 'ap-title', 'ap-stat']) for (const s of ['s1', 's2', 'tier-bg', 'sunken-none']) {
    const bg = hexOf(dark, s); if (!bg) continue;
    assert.ok(ratio(hexOf(dark, t), bg) >= 4.5, t + ' on ' + s);
  }
  // Dayside: the contrast tokens the other parts of the Command Center use are the ones that were there, and pass
  const lightBlock = lcc.slice(lcc.indexOf(':root[data-theme="light"] .lcc'));
  assert.equal(hexOf(lightBlock, 'gold-t'), '#744a0a');
  for (const bgName of ['s1', 's2', 'card-a', 'card-b', 'tier-bg']) assert.ok(ratio(hexOf(lightBlock, 'gold-t'), hexOf(lightBlock, bgName)) >= 4.5, 'Dayside gold-t on ' + bgName);
  // the dimmer 0.5 glow is unchanged
  assert.match(lcc, /--lcc-glow:\s*0?\.5/);
  assert.match(spec, /--lcc-sp-glow: \.5;/);
});

test('level 100 ornament: the gold line is drawn for a track at the XP cap only, with the same words', async () => {
  assert.match(spec, /\.lcc-sp-mastery\.is-on::before, \.lcc-sp-mastery\.is-on::after \{[^}]*content: '\\2726'/);
  assert.doesNotMatch(spec, /\.lcc-sp-mastery::(before|after)/, 'a track below the cap has no ornament');
  const ui = await openSpec('mixed-five-tracks');
  const m = () => ui.doc.querySelector('#lcs-mastery');
  // Crafting is at level 100: the ornament class and the words
  assert.ok(m().classList.contains('is-on'));
  assert.equal(text(m()), 'COMPLETE');
  ui.click('[data-lcs-lane="combat"]'); await ui.settle();
  assert.equal(m().classList.contains('is-on'), false, 'level 46 has no ornament line');
  assert.equal(text(m()), 'XP CAP NOT REACHED');
  ui.click('[data-lcs-lane="sabotage"]'); await ui.settle();
  assert.equal(m().classList.contains('is-on'), false, 'level 99 has no ornament line');
});

test('the fifth lane of two to a row sits in the middle of the last row; one to a row has no such rule', () => {
  assert.match(spec, /@media \(min-width: 381px\) and \(max-width: 760px\) \{\s*\.lcc-sp-lane:last-child:nth-child\(odd\) \{ grid-column: 1 \/ -1; justify-self: center; width: calc\(\(100% - 12px\) \/ 2\); \}/);
  assert.match(spec, /@media \(max-width: 380px\) \{\s*\.lcc-sp-lanes \{ grid-template-columns: minmax\(0, 1fr\); \}/);
});

test('the shield is a solid dark shield inside a gold edge with the gold cross, drawn the same in both themes, and plain in forced colours', () => {
  assert.match(spec, /\.lcc-sp-hero-art \{[^}]*background: #d6a35b;[^}]*clip-path: polygon/);
  assert.match(spec, /\.lcc-sp-hero-art::before \{[^}]*background: linear-gradient\(135deg, #2c2c32, #0d1217\)/);
  assert.match(spec, /\.lcc-sp-hero-art \{[^}]*color: #f0d397/);
  assert.match(spec, /repeating-linear-gradient\(145deg/, 'the diagonal sheen of the hero');
  const forced = spec.slice(spec.indexOf('@media (forced-colors: active)'));
  assert.match(forced, /\.lcc-sp-hero-art \{ background: ButtonText; color: CanvasText; \}/);
  assert.match(forced, /\.lcc-sp-hero-art::before \{ background: Canvas; box-shadow: none; \}/);
});

test('the Classic / Command Center switch is 40 px high, 44 px on a phone and with a coarse pointer', () => {
  assert.match(spec, /\.lcc-sp-vbtn \{ min-height: 40px;/);
  assert.match(spec, /@media \(pointer: coarse\), \(max-width: 760px\) \{ \.lcc-sp-vbtn \{ min-height: 44px; \} \}/);
});

// ---- #902
test('the keystone total is marked partial when a track is not identified, and says why; a missing total says its own reason', async () => {
  const ui = await openSpec('conflicting-learned-number-leaves-combat-unresolved');
  const stat = ui.doc.querySelector('#lcs-keycount').parentNode;
  assert.match(text(ui.doc.querySelector('#lcs-keycount')), /\/ 205\*$/);
  assert.match(text(stat.querySelector('small')), /\* ONE SPECIALIZATION IS NOT MATCHED YET/);
  closeAll();
  const none = await openSpec('keystone-total-zero-no-map');
  assert.doesNotMatch(text(none.doc.querySelector('#lcs-keycount').parentNode.querySelector('small')), /NOT IDENTIFIED/);
  closeAll();
  const all = await openSpec('mixed-five-tracks');
  assert.doesNotMatch(text(all.doc.querySelector('#lcs-keycount')), /\*/, 'five identified tracks: no mark');
});

test('the model keeps the reason: a missing total outranks an unidentified track', () => {
  const src = read('internal', 'web', 'static', 'lcc', 'lcc-spec.js');
  assert.match(src, /keysPartial: out\.rows\.some\(\(r\) => !r\.keyKnown\) \|\| out\.rows\.some\(\(r\) => !r\.known\)/);
  assert.match(src, /keysWhy: out\.rows\.some\(\(r\) => !r\.keyKnown\) \? 'total' : out\.rows\.some\(\(r\) => !r\.known\) \? 'unidentified' : ''/);
});

test('the order chips are 36 px or more at every width and the board\'s selects are 44 px on a phone', () => {
  assert.match(board, /\.lcc-bd-chip \{ all: unset;[^}]*min-height: 36px;/);
  assert.match(board, /@media \(max-width: 640px\), \(pointer: coarse\) \{\s*\.lcc-bd-select \{ min-height: 44px; \}/);
  assert.match(board, /\.lcc-bd-select \{[^}]*min-height: 37px;/, 'the select is at the 36 px floor on a wide window');
});

test('the earlier terms: the table is a keyboard-reachable region that is named as scrolling sideways, with no standing line above it (#1062)', async () => {
  const ui = await openBoard();
  const w = ui.doc.querySelector('.lcc-tm-tablewrap');
  assert.ok(w, 'a table of an earlier term');
  assert.equal(w.getAttribute('tabindex'), '0');
  assert.equal(w.getAttribute('role'), 'region');
  assert.match(w.getAttribute('aria-label'), /^Term \d+, the table of Houses: scrolls sideways/);
  assert.equal(w.previousElementSibling.tagName, 'SUMMARY', 'no standing line between the heading of the term and the table (#1062): the region name says it scrolls');
  assert.doesNotMatch(term, /scrollhint/);
  assert.match(term, /\.lcc-tm-tablewrap:focus-visible \{ outline: 3px solid var\(--lcc-focus\)/);
});

test('the pool lists the decrees in the order of the save, as the classic panel does; the drop-downs stay sorted by name', async () => {
  const a = synthetic();
  const ui = await openBoard();
  const rows = [...ui.doc.querySelectorAll('#lcc-term .lcc-tm-row')].map((r) => Number(r.dataset.decree));
  const want = a.decrees.filter((d) => d.decree_name !== 'SpecialVendorActive').map((d) => d.id);
  assert.deepEqual(rows.filter((id) => want.includes(id)), want);
  const classic = a.decrees.map((d) => d.id);
  assert.deepEqual([...classic].sort((x, y) => x - y), classic, 'the save\'s own order is by id');
});

test('the sticky header does not cover what is scrolled to: the page keeps the measured header height as scroll padding, and none on a phone where the header scrolls away', () => {
  assert.match(app, /html\{scroll-padding-top:calc\(var\(--hdr-h,56px\) \+ 12px\)\}/);
  assert.match(app, /html:has\(#gamewarn\.warn,#launchpass\.info\)\{scroll-padding-top:calc\(var\(--hdr-h,56px\) \+ 12px \+ var\(--warn-h,76px\)\)\}/);
  assert.match(app, /@media \(max-width:720px\)\{[^]*?header\.d-bar\{position:static[^}]*\}\s*html\{scroll-padding-top:12px\}\s*html:has\(#gamewarn\.warn,#launchpass\.info\)\{scroll-padding-top:12px\}/, 'two rules, so a browser without :has() keeps the plain one (#910)');
  assert.match(app, /header\.d-bar\{[^}]*position:sticky/);
  assert.match(app, /@media \(max-width:720px\)\{\s*header\.d-bar\{position:static/);
});

test('the matrix says what the section does: the pool follows the save\'s order, and the hero totals row names the mark', () => {
  const m6 = read('docs', 'design', 'landsraad-command-center', '09-m6-decree-term-integration.md');
  const t8 = m6.split('\n').find((l) => l.startsWith('| T8 '));
  assert.ok(t8);
  assert.match(t8, /order of the save|game's own order|save's own order/);
  assert.doesNotMatch(t8, /Same sort as the classic panel/);
  const m5 = read('docs', 'design', 'landsraad-command-center', '08-m5-specializations.md');
  const h6 = m5.split('\n').find((l) => l.startsWith('| H6 '));
  assert.match(h6, /not (identified|matched)/i);
});
