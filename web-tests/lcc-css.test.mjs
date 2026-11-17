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
  assert.deepEqual(kf, ['lcc-reveal']);
  assert.doesNotMatch(bare, /infinite/);
  for (const m of bare.matchAll(/animation:\s*([^;}]+)/g)) if (!/^none/.test(m[1].trim())) assert.match(m[1], /\b1\b/, 'an animation that is not one-shot: ' + m[1]);
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
  assert.ok([...ats].some((a) => a.startsWith('@container (min-width: 900px)')));
  assert.ok([...ats].some((a) => a.startsWith('@container not (min-width: 900px)')));
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
  assert.match(body, /\.lcc-house\[data-victory="dominion"\][^}]*border-style:\s*double/);
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
