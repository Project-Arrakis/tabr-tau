// The Command Center's styles (M2): they are scoped to their own classes so loading them changes nothing outside the parts they draw, they take their colours from the shared design
// tokens, they carry a forced-colours variant and a reduced-motion variant, and they reach nothing outside this origin. A small parser reads the file; it is not a CSS engine, but it
// is exact for the plain rules this file is written with (no nesting, no escapes).
import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { lccDir, read, root } from './lcc-load.mjs';

// the foundation's styles and the board's (M3) are one body of rules for these checks: the same scoping, tokens and variants apply to both
const css = ['lcc.css', 'lcc-board.css'].map((f) => fs.readFileSync(path.join(lccDir, f), 'utf8').replace(/\r/g, '')).join('\n');
const bare = css.replace(/\/\*[\s\S]*?\*\//g, '');

// rules: { at: '@container ...' | '', sel: 'a, b', body }
function parse(src) {
  const out = [];
  let i = 0;
  const walk = (end, at) => {
    while (i < end) {
      const open = src.indexOf('{', i);
      if (open < 0 || open >= end) break;
      const head = src.slice(i, open).trim();
      let depth = 1, j = open + 1;
      while (depth && j < end) { if (src[j] === '{') depth++; else if (src[j] === '}') depth--; j++; }
      const body = src.slice(open + 1, j - 1);
      if (head.startsWith('@media') || head.startsWith('@container') || head.startsWith('@supports')) { const save = i; i = open + 1; walk(j - 1, head); i = j; void save; }
      else out.push({ at, sel: head, body });
      i = j;
    }
  };
  walk(src.length, '');
  return out;
}
const rules = parse(bare);

test('the stylesheet is not empty and parses into rules', () => {
  assert.ok(rules.length > 120, 'rules found: ' + rules.length);
});

test('every selector is scoped to a lcc class: loading the file changes nothing else on the page', () => {
  for (const r of rules) {
    if (r.sel.startsWith('@keyframes')) { assert.match(r.sel, /^@keyframes lcc-[a-z-]+$/, r.sel); continue; }
    if (r.sel.startsWith('@font-face') || r.sel.startsWith('@import') || r.sel.startsWith('@charset')) assert.fail('at-rule ' + r.sel);
    for (const s of r.sel.split(',').map((x) => x.trim())) {
      assert.match(s, /\.lcc(-[a-z0-9-]+)?\b/, 'selector is not scoped: ' + s);
      // the first thing in a selector is a lcc class, or the root / html element that carries a theme or motion attribute in front of one
      assert.match(s, /^(\.lcc|:root(\[[^\]]+\]|:not\(\[[^\]]+\]\))*\s+\.lcc|html(\[[^\]]+\])+\s+\.lcc)/, 'selector starts outside the feature: ' + s);
      assert.doesNotMatch(s, /^(body|html|\*|:root)\s*(\{|$)/);
    }
  }
});

test('the keyframes are the feature\'s own, and one-shot: nothing loops', () => {
  const kf = [...bare.matchAll(/@keyframes\s+([a-z0-9-]+)/g)].map((m) => m[1]);
  assert.deepEqual([...kf].sort(), ['lcc-reveal', 'lcc-win-pulse', 'lcc-win-sweep']);
  // the exceptions to one-shot are the optional animated border of a winning House (lcc-win-sweep: on the House, and on the centre seat of a win by 13 Houses) and the slow glow of a winning House
  // (lcc-win-pulse): each loops and sits in the no-preference block under Motion Full
  assert.equal((bare.match(/infinite/g) || []).length, 3);
  assert.match(bare, /@media \(prefers-reduced-motion: no-preference\)\s*\{[^]*html\[data-motion="full"\][^]*animation: lcc-win-sweep 9s linear infinite/);
  for (const m of bare.matchAll(/animation:\s*([^;}]+)/g)) if (!/^none/.test(m[1].trim()) && !/^lcc-win-sweep /.test(m[1].trim())) assert.match(m[1], /\b1\b/, 'an animation that is not one-shot: ' + m[1]);
});

test('custom properties defined here are all --lcc-*, and every design token used exists in the three looks', () => {
  for (const m of bare.matchAll(/(--[a-z0-9-]+)\s*:/g)) assert.match(m[1], /^--lcc-/, 'a custom property outside the feature: ' + m[1]);
  const looks = ['fremen', 'awakening', 'sietch'].map((l) => fs.readFileSync(path.join(root, 'design', 'assets', 'tokens-' + l + '.css'), 'utf8'));
  const defined = (name) => looks.every((t) => t.includes(name + ':') || t.includes(name + ' :'));
  // a token a look does not define may be used only with a fallback (var(--x, fallback)), which the pattern below leaves out
  const used = new Set([...bare.matchAll(/var\((--(?:c|t|mo)-[a-z0-9-]+)(?=\))/g)].map((m) => m[1]));
  assert.ok(used.size >= 10, 'design tokens used: ' + [...used].join(' '));
  assert.match(bare, /var\(--c-bronze, var\(--c-rule\)\)/);
  for (const u of used) assert.ok(defined(u), u + ' is not defined in every look of the design module');
  // the aliases are where a token is named: --lcc-* defined from --c-*
  assert.match(bare, /--lcc-ground:\s*var\(--c-ground\)/);
  assert.match(bare, /--lcc-text:\s*var\(--c-text\)/);
  assert.match(bare, /--lcc-focus:\s*var\(--c-focus\)/);
});

test('it reaches nothing outside this origin and needs no looser content security policy', () => {
  assert.doesNotMatch(bare, /url\(/);
  assert.doesNotMatch(bare, /@import|data:|https?:|\/\/[a-z]/i);
  assert.doesNotMatch(bare, /expression\(|behavior:|-moz-binding|javascript:/);
  for (const f of ['lcc.js', 'lcc-model.js', 'lcc-board.js']) {
    const js = fs.readFileSync(path.join(lccDir, f), 'utf8');
    assert.doesNotMatch(js, /setAttribute\(\s*['"]style['"]|\.cssText|insertRule/, f);
    assert.doesNotMatch(js, /\.src\s*=\s*['"`]https?:/, f);
  }
});

test('the variants the design asks for exist: forced colours, reduced motion, Off and Calm, more contrast, Dayside', () => {
  const ats = new Set(rules.map((r) => r.at));
  assert.ok([...ats].some((a) => a.startsWith('@media (forced-colors: active)')), 'forced colours');
  assert.ok([...ats].some((a) => a.startsWith('@media (prefers-reduced-motion: reduce)')), 'reduced motion');
  assert.ok([...ats].some((a) => a.startsWith('@media (prefers-contrast: more)')), 'more contrast');
  assert.ok([...ats].some((a) => a.startsWith('@media (prefers-color-scheme: light)')), 'system Dayside');
  assert.match(bare, /:root\[data-theme="light"\] \.lcc/);
  assert.match(bare, /html\[data-motion="off"\] \.lcc \[data-victory\]/);
  assert.match(bare, /html\[data-motion="calm"\] \.lcc \[data-victory\]/);
  // the plaque has a wide and a narrow layout, driven by the box it is in
  assert.ok([...ats].some((a) => a.startsWith('@container (min-width: 940px)')));
  assert.ok([...ats].some((a) => a.startsWith('@container not (min-width: 940px)')));
  assert.ok([...ats].some((a) => a.startsWith('@container (max-width: 520px)')));
  assert.match(bare, /\.lcc-plaque-host\s*\{[^}]*container-type:\s*inline-size/);
});

test('in forced colours the decoration goes and the factions still differ, by fill and by shape', () => {
  const f = rules.filter((r) => r.at.startsWith('@media (forced-colors: active)'));
  const body = f.map((r) => r.sel + '{' + r.body + '}').join('\n');
  assert.match(body, /\.lcc-art\s*\{\s*display:\s*none/);
  assert.match(body, /\.lcc-seat-a \.lcc-seat-base\s*\{[^}]*fill:\s*Highlight/);
  assert.match(body, /\.lcc-seat-h \.lcc-seat-base\s*\{[^}]*fill:\s*CanvasText/);
  assert.match(body, /\.lcc-house\[data-owner="a"\][^}]*border-left-style:\s*solid/);
  assert.match(body, /\.lcc-house\[data-owner="h"\][^}]*border-left-style:\s*double/);
  assert.match(body, /\.lcc-house\[data-victory="win"\][^}]*border-style:\s*double/);
  assert.match(body, /\.lcc-house\[aria-pressed="true"\][^}]*outline:\s*3px solid Highlight/);
  // only system colours are used inside the block
  const named = [...body.matchAll(/(?:color|fill|stroke|background(?:-color)?|border(?:-left)?(?:-color)?):\s*([^;}]+)/g)].map((m) => m[1].trim());
  for (const v of named) assert.match(v, /^(Canvas|CanvasText|ButtonText|ButtonFace|Highlight|HighlightText|none|\d+px (solid|double|dashed|dotted) (Canvas|CanvasText|ButtonText|Highlight)|solid|double|dashed|dotted)\b/, 'a colour that is not a system colour in forced colours: ' + v);
});

test('the colours of the words on the plaque pass 4.5:1 on its own field, and the Dayside gold passes on the Dayside panel', () => {
  const lum = (hex) => { const c = [1, 3, 5].map((i) => parseInt(hex.slice(i, i + 2), 16) / 255).map((v) => (v <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4)); return 0.2126 * c[0] + 0.7152 * c[1] + 0.0722 * c[2]; };
  const ratio = (a, b) => { const [x, y] = [lum(a), lum(b)].sort((p, q) => q - p); return (x + 0.05) / (y + 0.05); };
  const field = '#0a0b1e';
  for (const fg of ['#f1d9a4', '#e7c69a', '#f4d8a8', '#f6e2b3', '#f7dca4', '#ffe9c7', '#ffd98a', '#f2857f', '#f4e6c3']) assert.ok(ratio(fg, field) >= 4.5, fg + ' on the field: ' + ratio(fg, field).toFixed(2));
  // words on the two cloth standards
  assert.ok(ratio('#fdf6e6', '#0f6a54') >= 4.5 && ratio('#fdf6e6', '#063a2e') >= 4.5, 'on the Atreides cloth');
  assert.ok(ratio('#fdf6e6', '#8c1219') >= 4.5 && ratio('#fdf6e6', '#4a070c') >= 4.5, 'on the Harkonnen cloth');
  assert.ok(ratio('#ffd98a', '#0f6a54') >= 4.5, 'the cloth heading');
  // letters on a gold plate, and in Dayside on the dark gold
  assert.ok(ratio('#2a1a05', '#e3b24d') >= 4.5, 'ink on gold');
  assert.ok(ratio('#fff6df', '#8a5a10') >= 4.5, 'on-gold on Dayside gold');
  // Dayside: the gold used as text is the look's own heading sand; its panel is the Dayside panel (the look's tokens, checked by the design module's tests)
  assert.ok(ratio('#8a5a10', '#f5f2ea') >= 4.5, 'dark gold ring colour is not text but must still show on the Dayside panel');
  // owner chips
  assert.ok(ratio('#fdf6e6', '#0b5a47') >= 4.5 && ratio('#fdf6e6', '#7a1118') >= 4.5, 'owner chips');
});

test('the page has the same rules in the code: the plaque is one section with a name, the cards are buttons', () => {
  const js = fs.readFileSync(path.join(lccDir, 'lcc.js'), 'utf8');
  assert.match(js, /'aria-label': 'Landsraad decree and voting plaque'/);
  assert.match(js, /'lcc lcc-house', '', \{ type: 'button'/);
  assert.match(js, /'lcc lcc-lane', '', \{ type: 'button'/);
  void read;
});

// The glow of the plaque is one dial (--lcc-glow): the pieces that glow read it, and nothing else in the file names a glow strength of its own.
test('the plaque glow is one custom property that the glowing parts read', () => {
  const dial = bare.match(/--lcc-glow:\s*([0-9.]+)/g) || [];
  assert.equal(dial.length, 1, 'one place sets the dial');
  const v = Number(dial[0].replace(/[^0-9.]/g, ''));
  assert.ok(v >= 0 && v <= 1, 'the dial is between 0 and 1: ' + v);
  const users = rules.filter((r) => /var\(--lcc-glow\)/.test(r.body)).map((r) => r.sel).join(' | ');
  for (const part of ['.lcc-glow-frame', '.lcc-glow-rail', '.lcc-glow-line', '.lcc-glow-edge', '.lcc-seat-halo']) assert.ok(users.includes(part), part + ' reads the dial');
});

test('the Choose box of the list sits in its table cell (it is only absolute over the corner of a tile)', () => {
  assert.ok(bare.includes('.lcc-bd-c-pick .lcc-bd-pickl { position: static;'));
});

// ---- Display remediation A (display only): the text floor, the tier boxes, the side-by-side panel, the guards and the light-theme contrast ----
const appCss = read('internal', 'web', 'static', 'app.css').replace(/\r/g, '');
const appBare = appCss.replace(/\/\*[\s\S]*?\*\//g, '');
const lccCss = fs.readFileSync(path.join(lccDir, 'lcc.css'), 'utf8').replace(/\r/g, '');
const boardCss = fs.readFileSync(path.join(lccDir, 'lcc-board.css'), 'utf8').replace(/\r/g, '');

// the last plain font size of each selector: rules are read in file order and the remediation block repeats a selector to raise it
function finalSizes() {
  const sizes = new Map();
  for (const r of rules) {
    if (r.at !== '' && !r.at.startsWith('@supports')) continue;
    const m = /(?:^|;)\s*font-size:\s*([\d.]+)px/.exec(r.body) || /(?:^|;)\s*font:\s*(?:italic\s+)?(?:\d+\s+)?([\d.]+)px/.exec(r.body);
    if (!m) continue;
    for (const s of r.sel.split(',').map((x) => x.trim())) sizes.set(s, Number(m[1]));
  }
  return sizes;
}

test('text floor: no rule of the board leaves text under 11 px, and names and key labels are 12 px or more', () => {
  const sizes = finalSizes();
  // .lcc-flag is the foundation's own flag (the gallery); the board's use of it, the badge of a card, is raised by .lcc-hc-badge
  const low = [...sizes].filter(([sel, px]) => px < 11 && sel !== '.lcc-flag');
  assert.deepEqual(low, [], 'selectors under 11 px: ' + low.map(([s, px]) => s + ' ' + px).join(' | '));
  for (const s of ['.lcc-hc-owner', '.lcc-hc-count', '.lcc-bd-chip', '.lcc-bd-lbl', '.lcc-bd-sec-h', '.lcc-bd-control', '.lcc-bd-vtag', '.lcc-bd-tier', '.lcc-bd-tier-n', '.lcc-bd-tier-s', '.lcc-bd-elab', '.lcc-bd-metric-k', '.lcc-bd-fc-head']) assert.ok(sizes.get(s) >= 12, s + ' is a key label: ' + sizes.get(s));
  assert.match(bare, /\.lcc-hc-name \{ font: 700 12px\//, 'a card name is 12 px at its smallest');
  // the plaque keeps its floor in the wide layout: every clamp() starts at 11 px or more
  const wide = lccCss.replace(/\/\*[\s\S]*?\*\//g, '');
  const lows = [...wide.slice(wide.indexOf('@container (min-width: 940px) {')).matchAll(/clamp\((\d+(?:\.\d+)?)px/g)].map((m) => Number(m[1])).filter((n) => n < 11);
  assert.deepEqual(lows, [], 'a plaque size below 11 px');
});

test('tier boxes: 24 by 24 px with 2 px between them where touch or a narrow window is in play, the 18 by 20 of the design only in a wide panel, and a 36 px target', () => {
  assert.match(bare, /\.lcc-hc-t \{ position: relative; flex: 0 0 24px; width: 24px; max-width: 24px; height: 24px;/);
  assert.match(bare, /\.lcc-hc-tiers \{ position: relative; gap: 2px;/);
  assert.match(bare, /\.lcc-hc-t::after \{ content: ''; position: absolute; left: -1px; right: -1px; top: -6px; bottom: -6px; \}/);
  const wide = rules.find((r) => r.at === '@container (min-width: 760px)' && r.sel === '.lcc-hc-t');
  assert.ok(wide && /width: 18px/.test(wide.body) && /height: 20px/.test(wide.body), 'the wide panel keeps the 18 by 20 boxes');
  const coarse = rules.filter((r) => r.at === '@media (pointer: coarse), (max-width: 900px)' && r.sel === '.lcc-hc-t');
  assert.equal(coarse.length, 1, 'a touch screen and a window of 900 px or less take the 24 px boxes after the wide rule');
  assert.ok(bare.indexOf('@media (pointer: coarse), (max-width: 900px)') > bare.indexOf('@container (min-width: 760px)'), 'the touch rule comes after the wide rule, so it wins');
  // the arithmetic of the five columns: five boxes, four gaps, the card's padding and border make the card; five cards and four 4 px gaps make the container step
  const card = 5 * 24 + 4 * 2 + 2 * 3 + 2, five = 5 * card + 4 * 4;
  assert.equal(card, 136);
  assert.ok(bare.includes('@container (min-width: ' + five + 'px)'), 'five columns start at ' + five + ' px of panel');
  assert.match(bare, /\.lcc-hc \{ min-height: 170px; gap: 3px; padding: 8px 3px 9px; \}/);
});

test('the cards of a row line up: the footer is two lines tall, the counts stand together on one, and the empty case is styled like the other owner lines', () => {
  assert.match(bare, /\.lcc-hc-count \{ font-size: 12px; gap: 2px 6px; min-height: 33px;/);
  assert.match(bare, /\.lcc-hc-count b \{ flex: 0 0 100%; text-align: center;/);
  assert.match(bare, /\.lcc-hc-count \.lcc-hc-q \{ white-space: nowrap; \}/);
  assert.match(bare, /\.lcc-hc-count > b:only-child \{[^}]*text-transform: uppercase/);
  assert.match(bare, /\.lcc-hc-owner \{[^}]*white-space: nowrap/, 'the bullet and the faction name do not part');
});

// at-rules that need container queries or :has() sit inside a guard, so a browser without them shows the plain page and no rule applies halfway
function unguarded(src) {
  const stack = [];
  const bad = [];
  let head = '';
  for (const c of src) {
    if (c === '{') {
      stack.push(head.trim());
      const h = stack[stack.length - 1];
      if (h.startsWith('@container') && !stack.some((s) => s.startsWith('@supports'))) bad.push(h);
      head = '';
    } else if (c === '}') { stack.pop(); head = ''; }
    else if (c === ';') head = '';
    else head += c;
  }
  return bad;
}

test('guards: every container query of the remediation is inside @supports, and the board view layout needs :has() and container queries together', () => {
  const strip = (s) => s.replace(/\/\*[\s\S]*?\*\//g, '');
  const b = strip(boardCss);
  assert.deepEqual(unguarded(b.slice(b.indexOf('.lcc-bd-side { display: flex;'))), []);
  const l = lccCss.slice(lccCss.indexOf('/* ==== The narrow plaque, compacted'));
  assert.ok(l.length > 100);
  assert.deepEqual(unguarded(strip(l)), []);
  const at = appBare.indexOf('@supports selector(:has(*)) and (container-type: inline-size) {');
  assert.ok(at > 0, 'the board view layout is behind a guard');
  assert.deepEqual(unguarded(appBare.slice(at)), [], 'no @container of the board view layout stands outside the guard');
  const outside = appBare.slice(0, at);
  assert.doesNotMatch(outside, /#lsb-lcc/);
  assert.doesNotMatch(outside, /@container lsbv/);
});

test('layout: the page cap, the side-by-side switch and the sticky House Intelligence', () => {
  assert.match(appBare, /main:has\(#lsb-lcc\) \{ max-width: 1800px; \}/);
  assert.match(appBare, /@container lsbv \(min-width: 1080px\) \{/);
  assert.doesNotMatch(appBare, /@container lsbv \(min-width: 700px\) and \(max-width: 1079px\)/, 'the victory analysis no longer stands in a row beside the taller panel');
  // at the switch the board must keep five columns beside the side column: 696 px of panel, its padding (10) and border, the gap and the side column, inside the tab's card (8 px of padding each side)
  const panel = 696 + 2 * 10 + 2, side = 284, gap = 18, shell = 2 * 8;
  assert.ok(1080 - shell >= panel + gap + side, 'the board keeps five columns beside the House Intelligence at the switch width');
  assert.match(appBare, /position: sticky; top: calc\(var\(--hdr-h\) \+ 12px \+ var\(--lsb-warn-room, 0px\)\)/);
  assert.match(appBare, /max-height: calc\(100vh - var\(--hdr-h\) - var\(--lsb-warn-room, 0px\) - 24px\); max-height: calc\(100dvh - var\(--hdr-h\) - var\(--lsb-warn-room, 0px\) - 24px\);/);
  assert.match(appBare, /overscroll-behavior: contain/);
  assert.match(appBare, /body:has\(#gamewarn\.warn, #gamewarn\.info, #launchpass\.info\) \{ --lsb-warn-room: 76px; \}/);
  const ring = rules.filter((r) => /\.lcc-bd-insp \.lcc-bd-tier:focus-visible/.test(r.sel));
  assert.ok(ring.length && /outline-offset: -3px/.test(ring[0].body), 'a focus ring on a full-width row is drawn inside the row');
});

test('stacked layout (under 1080 px of the tab): no empty band beside the House Intelligence panel; from 860 px the ledger stands beside the scales, the note and the place', () => {
  // the row rule that put the short victory analysis beside the tall panel (a band of more than 1000 px) is gone: the side box stays one column, so both panels are full width
  assert.doesNotMatch(appBare, /lcc-bd-side \{ flex-direction: row/);
  assert.doesNotMatch(appBare, /lcc-bd-side > \* \{ flex: 1 1 0/);
  const appRules = parse(appBare);
  const at = '@container lsbv (min-width: 860px) and (max-width: 1079px)';
  const two = appRules.find((r) => r.at === at && r.sel === '.lsb #lsb-board .lcc-bd-insp');
  assert.ok(two && /display: grid/.test(two.body) && /grid-template-columns: minmax\(0, 1fr\) minmax\(0, 1fr\)/.test(two.body) && /grid-auto-flow: row dense/.test(two.body), 'the panel is a grid of two columns from 860 px');
  const all = appRules.find((r) => r.at === at && r.sel === '.lsb #lsb-board .lcc-bd-insp > *');
  assert.ok(all && /grid-column: 1 \/ -1/.test(all.body), 'the head, the way back, the identity, the actions and the banner run across both columns');
  const led = appRules.find((r) => r.at === at && r.sel === '.lsb #lsb-board .lcc-bd-insp > .lcc-bd-ledger');
  assert.ok(led && /grid-column: 1/.test(led.body) && /grid-row: span 4/.test(led.body), 'the ledger takes the left column for the four blocks beside it');
  const right = appRules.find((r) => r.at === at && /grid-column: 2/.test(r.body));
  assert.ok(right && /aria-labelledby="lcc-bd-s1"/.test(right.sel) && /aria-labelledby="lcc-bd-s2"/.test(right.sel) && /lcc-bd-disc/.test(right.sel) && /aria-labelledby="lcc-bd-s5"/.test(right.sel), 'the two scales, the note and the place stand in the right column, so neither column is left empty');
  // two columns of about 345 px or more each (the Inspector's own container step) from 860 px of tab: (860 - 2 * 20 padding - 28 gap) / 2
  assert.ok((860 - 2 * 20 - 28) / 2 >= 345 - 1);
  // all of it stays inside the guard
  assert.deepEqual(unguarded(appBare.slice(appBare.indexOf('@supports selector(:has(*)) and (container-type: inline-size) {'))), []);
});

test('markup order is the visual order: the action bar is part of the board panel\'s control bar (not a place of the board grid), and no landmark wrapper is flattened with display: contents', () => {
  assert.doesNotMatch(appBare.slice(appBare.indexOf('@supports selector(:has(*)) and (container-type: inline-size) {')), /#lsb-lcc[^{}]*\{[^}]*display: contents/);
  assert.doesNotMatch(appBare.slice(appBare.indexOf('Command Center board view: width')), /\.lcc-bd(-grid)?\s*\{[^}]*display: contents/);
  assert.doesNotMatch(appBare, /\.actbar[^{}]*\{[^}]*grid-area/, 'the bar is not placed by a grid area of the page card any more');
  const appRules = parse(appBare);
  assert.ok(appRules.some((r) => r.at === '@container lsbv (min-width: 1080px)' && r.sel === '.lsb #lsb-board .lcc-bd-grid' && /grid-template-areas: "pn sd" "fl sd"/.test(r.body)), 'beside the board: the board panel in the left column, the side column on the right');
  assert.ok(!appRules.some((r) => /\.lcc-bd-grid > \.lcc-bd-slot/.test(r.sel)), 'the question and the result line are in the panel, no longer a place of the grid');
});

test('phones: the bar of the chosen houses is sticky at the bottom, clear of the home bar and of a focused control; the cards have no crest; the way back is shown where the panel is not beside the board', () => {
  assert.match(appBare, /\.lsb \.phonebar \{ display: none; \}/);
  const phone = parse(appBare).find((r) => /@media \(max-width: 640px\), \(pointer: coarse\) and \(max-width: 900px\)/.test(r.at) && r.sel === '.lsb .phonebar:not([hidden])');
  assert.ok(phone && /position: sticky; bottom: 0/.test(phone.body) && /env\(safe-area-inset-bottom/.test(phone.body), 'sticky at the bottom with the safe-area inset');
  assert.match(appBare, /html:has\(\.phonebar:not\(\[hidden\]\)\) \{ scroll-padding-bottom: 110px; \}/);
  assert.match(appBare, /\.lsb \.phonebar \.qa \{[^}]*min-height: 44px/, 'a touch target');
  assert.match(bare, /@media \(max-width: 640px\), \(pointer: coarse\) and \(max-width: 900px\) \{\s*\.lcc-hc \{ min-height: 0; \}\s*\.lcc-hc-crest \{ display: none; \}/);
  assert.match(appBare, /@container lsbv \(max-width: 1079px\) \{\s*\.lsb #lsb-board \.lcc-bd-back \{ display: inline-flex; \}/);
  assert.match(bare, /\.lcc-bd-back \{ all: unset; display: none;/);
});

test('disclosures: the long notes of the panel are native details with a plain summary, and the question place exists; no copy of a result is styled (#943)', () => {
  assert.match(bare, /\.lcc-bd-disc > summary \{[^}]*font: 600 12px/);
  assert.match(bare, /\.lcc-bd-eres:empty \{ display: none; \}/);
  assert.match(bare, /\.lcc-bd-ask:empty \{ display: none; \}/);
});

test('the plaque: the narrow layout is compact and, from 600 px, puts the two standards at the two sides of the track', () => {
  const g = rules.find((r) => r.at === '@container (min-width: 600px) and (max-width: 939px)' && r.sel === '.lcc-plaque');
  assert.ok(g && /display: grid/.test(g.body) && /grid-template-areas: "hd hd hd" "sa tk sh"/.test(g.body));
  assert.ok(rules.some((r) => r.at === '@container (min-width: 600px) and (max-width: 939px)' && r.sel === '.lcc-pl-foot' && /display: contents/.test(r.body)));
  assert.match(bare, /\.lcc-pl-std-a \.lcc-standard-h, \.lcc-pl-std-h \.lcc-standard-h \{ margin: 0 0 4px; letter-spacing: \.02em/);
  assert.match(bare, /\.lcc-pl-nameplate \{ grid-row: 1; padding: 4px 14px; min-height: 26px; font-size: 12px; max-width: 100%; white-space: normal;/);
});

test('light theme: the borders of the tier boxes pass 3:1 against both ends of the card, the reached, waiting and unknown boxes too', () => {
  const lum = (hex) => { const c = [1, 3, 5].map((i) => parseInt(hex.slice(i, i + 2), 16) / 255).map((v) => (v <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4)); return 0.2126 * c[0] + 0.7152 * c[1] + 0.0722 * c[2]; };
  const ratio = (a, b) => { const [x, y] = [lum(a), lum(b)].sort((p, q) => q - p); return (x + 0.05) / (y + 0.05); };
  const blocks = rules.filter((r) => /--lcc-card-a:/.test(r.body) && /data-theme="(light|dark)"/.test(r.sel));
  assert.ok(blocks.length >= 2);
  for (const b of blocks) {
    const tokens = Object.fromEntries([...b.body.matchAll(/--lcc-([a-z-]+):\s*(#[0-9a-f]{6})/g)].map((m) => [m[1], m[2]]));
    const v = (n) => { assert.ok(tokens[n], 'token --lcc-' + n + ' is set'); return tokens[n]; };
    for (const k of ['tier-edge', 'ok-edge', 'wait-edge', 'unk-edge']) for (const end of ['card-a', 'card-b']) assert.ok(ratio(v(k), v(end)) >= 3, k + ' on ' + end + ': ' + ratio(v(k), v(end)).toFixed(2));
  }
});

// ---- Light-theme text contrast, the phone victory panel and the folded notes (follow-up to the board's final check; display only) ----
const lumHex = (hex) => { const c = [1, 3, 5].map((i) => parseInt(hex.slice(i, i + 2), 16) / 255).map((v) => (v <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4)); return 0.2126 * c[0] + 0.7152 * c[1] + 0.0722 * c[2]; };
const ratioHex = (a, b) => { const [x, y] = [lumHex(a), lumHex(b)].sort((p, q) => q - p); return (x + 0.05) / (y + 0.05); };
const tokensOf = (body) => Object.fromEntries([...body.matchAll(/--lcc-([a-z0-9-]+):\s*(#[0-9a-f]{6})/g)].map((m) => [m[1], m[2]]));

test('contrast: 12 px text on the cards and in the Actions panel passes 4.5:1 against the worst end of its surface, dark and light, computed from the tokens', () => {
  const dark = rules.find((r) => r.at === '' && r.sel === '.lcc, .lcc-scope');
  const lights = rules.filter((r) => /--lcc-card-a:/.test(r.body) && /data-theme="(light|dark)"/.test(r.sel) && /:root\[data-theme="light"\]|:root:not\(\[data-theme="dark"\]\)/.test(r.sel));
  assert.ok(dark && lights.length === 2, 'the dark base block and the two light blocks exist');
  // the default look's own raised surface (the Reopen button stands on it): read from the shared tokens, so a change of those fails here
  const tk = read('design', 'assets', 'tokens.css').replace(/\r/g, '');
  const raised = [...tk.matchAll(/--c-raised:\s*(#[0-9a-f]{6})/g)].map((m) => m[1]);
  assert.ok(raised.includes('#241c15') && raised.includes('#f0e8da'), 'the default dark and light raised surfaces are the ones this test uses: ' + raised.join(' '));
  const sets = [['dark', tokensOf(dark.body)], ...lights.map((b, i) => ['light ' + (i + 1), { ...tokensOf(dark.body), ...tokensOf(b.body) }])];
  // [label, text token, surfaces it stands on]: card = both ends of the card gradient, panel = both ends of the panel gradient
  const pairs = [
    ['card owner Atreides', 'teal-t', ['card-a', 'card-b']],
    ['card owner Harkonnen', 'red-t', ['card-a', 'card-b']],
    ['card counts: collected', 'teal-t', ['card-a', 'card-b']],
    ['card counts: waiting', 'gold-t', ['card-a', 'card-b']],
    ['card counts: no record', 'red-t', ['card-a', 'card-b']],
    ['ACTIONS heading', 'gold-t', ['tier-bg']],
    ['House Harkonnen faction label', 'red-t', ['s1', 's2']],
    ['House Atreides faction label', 'teal-t', ['s1', 's2']],
    ['collected mark', 'teal-t', ['tier-bg']],
    ['Reopen button', 'red-t', ['RAISED']],
  ];
  for (const [label, tok] of sets) for (const [what, t, ends] of pairs) for (const e of ends) {
    assert.ok(tok[t], label + ': token --lcc-' + t + ' is set');
    const bg = e === 'RAISED' ? ((label === 'dark') ? '#241c15' : '#f0e8da') : tok[e];
    assert.ok(bg, label + ': surface ' + e);
    assert.ok(ratioHex(tok[t], bg) >= 4.5, label + ' ' + what + ' (' + tok[t] + ' on ' + bg + '): ' + ratioHex(tok[t], bg).toFixed(2));
  }

});


test('contrast: the rules that paint that text use the text tokens, and the strokes keep the accents', () => {
  const use = (sel, tok) => assert.ok(rules.some((r) => r.at === '' && r.sel.split(',').map((s) => s.trim()).includes(sel) && r.body.includes('color: var(--lcc-' + tok + ')')), sel + ' takes color from --lcc-' + tok);
  use('.lcc-hc-a .lcc-hc-owner', 'teal-t'); use('.lcc-hc-h .lcc-hc-owner', 'red-t');
  use('.lcc-hc-q-ok', 'teal-t'); use('.lcc-hc-q-wait', 'gold-t'); use('.lcc-hc-q-unk', 'red-t');
  use('.lcc-bd-editor .lcc-bd-sec-h', 'gold-t'); use('.lcc-bd-fc-h', 'red-t'); use('.lcc-bd-fc-a', 'teal-t');
  use('.lcc-bd-tier-s.delivered', 'teal-t'); use('.lcc-bd-act.lcc-bd-act-danger', 'red-t');
  assert.match(bare, /\.lcc-bd-act\.lcc-bd-act-danger \{ border-color: var\(--lcc-red\);/, 'the danger button keeps the accent for its stroke');
  assert.match(bare, /\.lcc-hc-a \{ border-color: var\(--lcc-teal\);/, 'the card stroke keeps the accent');
});

test('the phone victory panel keeps a smaller emblem, so the status tag has a clear gap above the heading', () => {
  const phone = rules.filter((r) => /max-width: 640px/.test(r.at) && /pointer: coarse/.test(r.at) && r.sel === '.lcc-bd-emblem');
  assert.equal(phone.length, 1, 'the phone rule for the emblem exists');
  assert.ok(!/display:\s*none/.test(phone[0].body), 'the emblem is not hidden on a phone');
  const size = +(/font-size:\s*(\d+)px/.exec(phone[0].body) || [])[1];
  const margin = /margin:\s*(\d+)px 0 (\d+)px/.exec(phone[0].body);
  assert.ok(size >= 24 && size <= 40, 'a smaller emblem: ' + size + 'px');
  assert.ok(margin && +margin[1] + +margin[2] >= 8, 'a gap above and below the emblem');
  assert.ok(!rules.some((r) => r.sel === '.lcc-bd-emblem' && /display:\s*none/.test(r.body)), 'no rule hides the emblem at any width');
});

test('the folded notes use the board palette, not the page link blue: a visible marker, a focus ring, and forced colours', () => {
  const sum = rules.find((r) => r.at === '' && r.sel === '.lcc-bd .lcc-bd-disc > summary');
  assert.ok(sum, 'a summary rule that outranks the page rule (.lsb details > summary)');
  assert.match(sum.body, /color: var\(--lcc-gold-text\)/);
  assert.match(sum.body, /list-style: none/);
  assert.ok(!/#[0-9a-f]{3,8}\b|--acc|--c-spice|--c-link/i.test(sum.body), 'no literal colour and no app accent in it');
  const marker = rules.find((r) => r.at === '' && r.sel === '.lcc-bd .lcc-bd-disc > summary::before');
  assert.ok(marker && /border-left: 7px solid currentColor/.test(marker.body), 'a drawn triangle in the text colour');
  assert.ok(rules.some((r) => r.sel === '.lcc-bd .lcc-bd-disc[open] > summary::before'), 'a turned triangle when open');
  assert.ok(rules.some((r) => r.sel === '.lcc-bd .lcc-bd-disc > summary:focus-visible' && /outline: 2px solid var\(--lcc-focus\)/.test(r.body)), 'a focus ring');
  assert.ok(rules.some((r) => r.sel === '.lcc-bd .lcc-bd-disc > summary:hover' && /underline/.test(r.body)), 'underlined on hover');
  const fc = rules.find((r) => r.at === '@media (forced-colors: active)' && /\.lcc-bd \.lcc-bd-disc > summary/.test(r.sel));
  assert.ok(fc && /color: CanvasText/.test(fc.body), 'forced colours: the system text colour');
});

test('the action bar in the control bar (#914): one group with Jump to House that wraps, controls of 36 px (44 px on a phone or a touch screen), type of 12 px or more, forced colours', () => {
  // the board's own side: the line of Jump to House holds the choose slot and wraps; the slots are not drawn while empty; the actions are a line of their own
  const jl = rules.find((r) => r.sel === '.lcc-bd-jumpline');
  assert.ok(jl && /display: inline-flex/.test(jl.body) && /flex-wrap: wrap/.test(jl.body), 'Jump to House and the choose buttons are one group that wraps');
  assert.ok(rules.some((r) => r.sel === '.lcc-bd-barslot:empty' && /display: none/.test(r.body)), 'an empty slot is not drawn');
  assert.ok(rules.some((r) => r.sel === '.lcc-bd-actslot' && /flex: 1 1 100%/.test(r.body)), 'the actions are a line of the bar');
  assert.ok(rules.some((r) => /^@media \(max-width: 640px\), \(pointer: coarse\)$/.test(r.at) && r.sel === '.lcc-bd-filters .lcc-bd-chip' && /min-height: 44px/.test(r.body)), 'the chips of the bar are 44 px on a phone');
  // the page's side: the parts the page seats there
  const own = rules.filter((r) => /\.barpart/.test(r.sel));
  assert.ok(own.length > 10);
  for (const r of own) for (const m of r.body.matchAll(/font:\s*(?:\d+\s+)?(\d+)px/g)) assert.ok(+m[1] >= 12, 'text under 12 px in ' + r.sel + ': ' + m[1]);
  assert.ok(own.some((r) => r.sel === '.lcc-bd .barpart .qa' && /min-height: 36px/.test(r.body) && /white-space: normal/.test(r.body)), 'a button is 36 px tall and wraps its words');
  assert.ok(own.some((r) => r.sel === '.lcc-bd .barpart select' && /min-height: 36px/.test(r.body)));
  assert.ok(own.some((r) => /^@media \(max-width: 640px\), \(pointer: coarse\)$/.test(r.at) && /\.barpart \.qa/.test(r.sel) && /\.barpart select/.test(r.sel) && /min-height: 44px/.test(r.body)), 'a touch target is 44 px');
  assert.ok(own.some((r) => r.at === '@media (forced-colors: active)' && /ButtonFace/.test(r.body) && /ButtonText/.test(r.body)), 'forced colours: system colours');
  assert.ok(own.some((r) => r.sel === '.lcc-bd .barpart #lsb-sum' && /flex: 1 1 100%/.test(r.body)), 'the count of the chosen houses has a line of its own');
});
