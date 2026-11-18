// The styles of the Specializations view (M5): scoped to their own classes, built on the shared design tokens, no address or import, the text floor (names and key labels 12 px or more, other
// text 11 px or more), the 0.5 glow dial, forced colours, reduced motion, more contrast, Dayside, and the narrow layouts that reflow instead of shrinking.
import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { lccDir, root } from './lcc-load.mjs';

const css = fs.readFileSync(path.join(lccDir, 'lcc-spec.css'), 'utf8').replace(/\r/g, '');
const bare = css.replace(/\/\*[\s\S]*?\*\//g, '');
function parse(src) {
  const out = []; let i = 0;
  const walk = (end, at) => {
    while (i < end) {
      const open = src.indexOf('{', i);
      if (open < 0 || open >= end) break;
      const head = src.slice(i, open).trim();
      let depth = 1, j = open + 1;
      while (depth && j < end) { if (src[j] === '{') depth++; else if (src[j] === '}') depth--; j++; }
      const body = src.slice(open + 1, j - 1);
      if (head.startsWith('@media') || head.startsWith('@container') || head.startsWith('@supports')) { i = open + 1; walk(j - 1, head); i = j; } else out.push({ at, sel: head, body });
      i = j;
    }
  };
  walk(src.length, '');
  return out;
}
const rules = parse(bare);

test('the file parses into rules', () => { assert.ok(rules.length > 120, 'rules: ' + rules.length); });

test('every selector is scoped to the view: loading the file changes nothing else on the page', () => {
  for (const r of rules) {
    assert.doesNotMatch(r.sel, /^@keyframes|@font-face|@import|@charset/, r.sel);
    for (const s of r.sel.split(',').map((x) => x.trim())) {
      assert.match(s, /^(\.lcc-sp|:root(\[[^\]]+\]|:not\(\[[^\]]+\]\))*\s+\.lcc-sp|html(\[[^\]]+\])+\s+\.lcc-sp)/, 'selector starts outside the view: ' + s);
    }
  }
});

test('it defines no custom property outside the feature, uses design tokens that every look defines, and reaches nothing outside this origin', () => {
  for (const m of bare.matchAll(/(--[a-z0-9-]+)\s*:/g)) assert.match(m[1], /^--lcc-sp-/, 'a custom property outside the view: ' + m[1]);
  const looks = ['fremen', 'awakening', 'sietch'].map((l) => fs.readFileSync(path.join(root, 'design', 'assets', 'tokens-' + l + '.css'), 'utf8'));
  for (const m of bare.matchAll(/var\((--(?:c|t|mo)-[a-z0-9-]+)(?=\))/g)) assert.ok(looks.every((t) => t.includes(m[1] + ':')), m[1] + ' is not defined in every look');
  assert.doesNotMatch(bare, /url\(|@import|data:|https?:|\/\/[a-z]|expression\(|behavior:|javascript:/i);
  // every --lcc-* it reads is one lcc.css defines
  const lcc = fs.readFileSync(path.join(lccDir, 'lcc.css'), 'utf8');
  for (const m of bare.matchAll(/var\((--lcc-(?!sp-|v[,)])[a-z0-9-]+)/g)) assert.ok(lcc.includes(m[1] + ':'), m[1] + ' is not defined by lcc.css');
});

test('nothing animates by itself: no keyframes and no loop; the only motion is a short hover lift that Off, Calm and reduced motion remove', () => {
  assert.doesNotMatch(bare, /@keyframes|animation:|infinite/);
  assert.ok(rules.some((r) => r.at.startsWith('@media (prefers-reduced-motion: reduce)') && /transition: none/.test(r.body)));
  assert.match(bare, /html\[data-motion="off"\] \.lcc-sp-lane/);
  assert.match(bare, /html\[data-motion="calm"\] \.lcc-sp-lane/);
});

test('the glow is one dial at 0.5, and the pieces that glow read it', () => {
  const dial = bare.match(/--lcc-sp-glow:\s*([0-9.]+)/g) || [];
  assert.equal(dial.length, 1);
  assert.equal(Number(dial[0].replace(/[^0-9.]/g, '')), 0.5);
  const users = rules.filter((r) => /var\(--lcc-sp-glow\)/.test(r.body)).map((r) => r.sel).join(' | ');
  for (const p of ['.lcc-sp-hero::before', '.lcc-sp-lane[data-kind="cap"]', '.lcc-sp-dossier[data-kind="cap"] .lcc-sp-level']) assert.ok(users.includes(p), p + ' reads the dial');
  // no glow strength written anywhere else: every blur radius of a shadow of gold goes through the dial
  for (const m of bare.matchAll(/(?:box-shadow|text-shadow):([^;}]+)/g)) for (const part of m[1].split(/,(?![^(]*\))/)) if (/color-mix\(in srgb, var\(--lcc-gold\)/.test(part) && /\s(\d+)px\s/.test(part) && !/inset 0 0 22px/.test(part)) assert.match(part, /--lcc-sp-glow/, part);
});

// the last plain font size of each selector, in file order, outside media queries
function sizes() {
  const m = new Map();
  for (const r of rules) {
    if (r.at !== '') continue;
    const f = /(?:^|;)\s*font:\s*(?:italic\s+)?(?:\d+\s+)?(?:clamp\()?([\d.]+)px/.exec(r.body) || /(?:^|;)\s*font-size:\s*([\d.]+)px/.exec(r.body);
    if (f) for (const s of r.sel.split(',').map((x) => x.trim())) m.set(s, Number(f[1]));
  }
  return m;
}
test('text floor: nothing under 11 px; names, key labels, buttons and inputs 12 px or more; a clamp() starts at 11 px or more', () => {
  const z = sizes();
  const low = [...z].filter(([, px]) => px < 11);
  assert.deepEqual(low, [], 'selectors under 11 px: ' + low.map(([s, px]) => s + ' ' + px).join(' | '));
  for (const s of ['.lcc-sp-lane-name', '.lcc-sp-lane-nums', '.lcc-sp-lane-badge', '.lcc-sp-btn', '.lcc-sp-input', '.lcc-sp-hn strong', '.lcc-sp-mark', '.lcc-sp-line', '.lcc-sp-flag', '.lcc-sp-mastery', '.lcc-sp-sub', '.lcc-sp-mapentry', '.lcc-sp-lbl', '.lcc-sp-small', '.lcc-sp-hint', '.lcc-sp-note', '.lcc-sp-vbtn', '.lcc-sp-vlbl']) assert.ok(z.get(s) >= 12 || (s === '.lcc-sp-lane-name' && z.get(s) >= 12), s + ' ' + z.get(s));
  for (const m of bare.matchAll(/clamp\((\d+(?:\.\d+)?)px/g)) assert.ok(Number(m[1]) >= 11, 'a clamp below 11 px: ' + m[0]);
  // a media query never lowers a font size: narrow windows reflow
  for (const r of rules.filter((x) => x.at.startsWith('@media (max-width'))) {
    const f = /font(?:-size)?:\s*(?:[^;]*?\s)?([\d.]+)px/.exec(r.body);
    if (f && /font/.test(r.body)) assert.ok(Number(f[1]) >= 11, r.at + ' ' + r.sel + ' ' + f[1]);
  }
});

test('the variants the design asks for exist: forced colours with system colours only, reduced motion, more contrast, system Dayside, and the Dayside switch', () => {
  const ats = new Set(rules.map((r) => r.at));
  assert.ok([...ats].some((a) => a.startsWith('@media (forced-colors: active)')));
  assert.ok([...ats].some((a) => a.startsWith('@media (prefers-reduced-motion: reduce)')));
  assert.ok([...ats].some((a) => a.startsWith('@media (prefers-contrast: more)')));
  assert.ok([...ats].some((a) => a.startsWith('@media (prefers-color-scheme: light)')));
  assert.match(bare, /:root\[data-theme="light"\] \.lcc-sp/);
  const body = rules.filter((r) => r.at.startsWith('@media (forced-colors: active)')).map((r) => r.sel + '{' + r.body + '}').join('\n');
  assert.match(body, /\.lcc-sp-lane\[aria-pressed="true"\]\s*\{\s*outline: 3px solid Highlight/);
  assert.match(body, /\.lcc-sp-lane\[data-kind="unresolved"\]\s*\{\s*border-style: dashed/);
  assert.match(body, /\.lcc-sp-btn\.is-danger\s*\{[^}]*double/);
  const named = [...body.matchAll(/(?:color|fill|stroke|background(?:-color)?|border(?:-color)?|outline):\s*([^;}]+)/g)].map((m) => m[1].trim());
  for (const v of named) assert.match(v, /^(Canvas|CanvasText|ButtonText|ButtonFace|Highlight|HighlightText|GrayText|none|\d+px (solid|double|dashed|dotted) (Canvas|CanvasText|ButtonText|Highlight)|solid|double|dashed|dotted|Highlight)\b/, 'not a system colour in forced colours: ' + v);
});

test('the states do not rely on colour alone: the cap, the unresolved track and the pressed lane differ by shape, border or words', () => {
  assert.match(bare, /\.lcc-sp-lane\[data-kind="unresolved"\] \{ border-style: dashed; \}/);
  assert.match(bare, /\.lcc-sp-lane\[aria-pressed="true"\] \{[^}]*box-shadow/);
  assert.match(bare, /\.lcc-sp-lane\[data-kind="cap"\], \.lcc-sp-lane\[data-kind="full"\] \{ box-shadow: inset 0 0 0 1px/);
});

test('the layout reflows at the widths asked for: five lanes, three, two, one; stacked dossier; touch targets of 44 px', () => {
  const at = (q, sel) => rules.find((r) => r.at === q && r.sel === sel);
  assert.match(bare, /\.lcc-sp-lanes \{ display: grid; grid-template-columns: repeat\(5, minmax\(0, 1fr\)\)/);
  assert.match(at('@media (max-width: 1000px)', '.lcc-sp-lanes').body, /repeat\(3,/);
  assert.match(at('@media (max-width: 760px)', '.lcc-sp-lanes').body, /repeat\(2,/);
  assert.match(at('@media (max-width: 380px)', '.lcc-sp-lanes').body, /minmax\(0, 1fr\)\)?;?\s*$/);
  assert.match(at('@media (max-width: 1180px)', '.lcc-sp-two').body, /minmax\(0, 1fr\)/);
  assert.match(at('@media (max-width: 520px)', '.lcc-sp-grid2').body, /minmax\(0, 1fr\)/);
  assert.match(at('@media (pointer: coarse)', '.lcc-sp-btn, .lcc-sp-input').body, /min-height: 44px/);
  assert.match(bare, /\.lcc-sp-btn \{ min-height: 40px;/);
  // no sideways page scroll: the long words wrap
  assert.match(bare, /\.lcc-sp-btn \{[^}]*overflow-wrap: anywhere/);
  assert.match(bare, /\.lcc-sp-lane-name \{[^}]*overflow-wrap: anywhere/);
});

test('the gold of the primary button and the letters on it pass 4.5:1 in dark and in Dayside', () => {
  const lum = (hex) => { const c = [1, 3, 5].map((i) => parseInt(hex.slice(i, i + 2), 16) / 255).map((v) => (v <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4)); return 0.2126 * c[0] + 0.7152 * c[1] + 0.0722 * c[2]; };
  const ratio = (a, b) => { const [x, y] = [lum(a), lum(b)].sort((p, q) => q - p); return (x + 0.05) / (y + 0.05); };
  const lcc = fs.readFileSync(path.join(lccDir, 'lcc.css'), 'utf8');
  assert.ok(ratio('#2a1a05', '#e3b24d') >= 4.5, 'dark: on-gold on gold');
  assert.ok(ratio('#fff6df', '#8a5a10') >= 4.5, 'Dayside: on-gold on gold');
  assert.match(bare, /\.lcc-sp-btn\.is-primary \{ background: var\(--lcc-gold\); border-color: var\(--lcc-gold-hi\); color: var\(--lcc-on-gold\); \}/);
  assert.ok(lcc.includes('--lcc-on-gold: #2a1a05') && lcc.includes('--lcc-on-gold: #fff6df'));
});
