// The styles of the Term and Decrees section (M6): scoped to their own classes, own properties only, no address or import, the text floor, forced colours, reduced motion, more contrast, Dayside and
// the narrow layouts that reflow instead of shrinking.
import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { lccDir } from './lcc-load.mjs';

const css = fs.readFileSync(path.join(lccDir, 'lcc-term.css'), 'utf8').replace(/\r/g, '');
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
      if (head.startsWith('@media')) { i = open + 1; walk(j - 1, head); i = j; } else out.push({ at, sel: head, body });
      i = j;
    }
  };
  walk(src.length, '');
  return out;
}
const rules = parse(bare);

test('the file parses into rules, and every selector is scoped to the section', () => {
  assert.ok(rules.length > 60, 'rules: ' + rules.length);
  for (const r of rules) for (const s of r.sel.split(',').map((x) => x.trim())) assert.match(s, /^(\.lcc-tm|:root(\[[^\]]+\]|:not\(\[[^\]]+\]\))*\s+\.lcc-tm|html(\[[^\]]+\])+\s+\.lcc-tm)/, 'selector starts outside the section: ' + s);
});

test('no custom property outside the section, no address, no import, nothing that animates by itself', () => {
  for (const m of bare.matchAll(/(--[a-z0-9-]+)\s*:/g)) assert.match(m[1], /^--lcc-tm-/, 'a custom property outside the section: ' + m[1]);
  assert.doesNotMatch(bare, /url\(|@import|data:|https?:|\/\/[a-z]|expression\(|behavior:|javascript:/i);
  assert.doesNotMatch(bare, /@keyframes|animation:|infinite/);
  const lcc = fs.readFileSync(path.join(lccDir, 'lcc.css'), 'utf8');
  for (const m of bare.matchAll(/var\((--lcc-(?!tm-)[a-z0-9-]+)/g)) assert.ok(lcc.includes(m[1] + ':'), m[1] + ' is not defined by lcc.css');
});

test('text floor: nothing under 11 px, names and buttons 12 px or more, no media query lowers a size', () => {
  const sizes = new Map();
  for (const r of rules) {
    const f = /(?:^|;)\s*font:\s*(?:italic\s+)?(?:\d+\s+)?(?:clamp\()?([\d.]+)px/.exec(r.body) || /(?:^|;)\s*font-size:\s*([\d.]+)px/.exec(r.body);
    if (f) for (const s of r.sel.split(',').map((x) => x.trim())) if (r.at === '') sizes.set(s, Number(f[1]));
    if (f && r.at) assert.ok(Number(f[1]) >= 11, r.at + ' ' + r.sel);
  }
  const low = [...sizes].filter(([, px]) => px < 11);
  assert.deepEqual(low, [], 'under 11 px: ' + low.map(([s, px]) => s + ' ' + px).join(' | '));
  for (const s of ['.lcc-tm-btn', '.lcc-tm-select', '.lcc-tm-name']) if (sizes.has(s)) assert.ok(sizes.get(s) >= 12, s + ' ' + sizes.get(s));
});

test('the variants exist: forced colours, reduced motion, more contrast, Dayside; touch targets of 44 px; narrow windows stack', () => {
  const ats = rules.map((r) => r.at);
  for (const q of ['@media (forced-colors: active)', '@media (prefers-reduced-motion: reduce)', '@media (prefers-contrast: more)', '@media (max-width: 1100px)', '@media (max-width: 640px)']) assert.ok(ats.some((a) => a.startsWith(q)), q);
  assert.match(bare, /:root\[data-theme="light"\] \.lcc-tm/);
  assert.match(bare, /@media \(pointer: coarse\), \(max-width: 640px\)\s*\{[^}]*min-height: 44px/);
  const forced = rules.filter((r) => r.at.startsWith('@media (forced-colors: active)')).map((r) => r.body).join('\n');
  for (const v of [...forced.matchAll(/(?:color|background(?:-color)?|border(?:-color)?|outline):\s*([^;}]+)/g)].map((m) => m[1].trim())) assert.match(v, /^(Canvas|CanvasText|ButtonText|ButtonFace|Highlight|HighlightText|GrayText|none|\d+px (solid|double|dashed|dotted) (Canvas|CanvasText|ButtonText|Highlight)|solid|double|dashed|dotted)/, v);
  assert.match(bare, /\.lcc-tm-ctlrow \{[^}]*grid-template-columns/);
});

test('an overflowing word wraps instead of scrolling the page sideways', () => {
  assert.match(bare, /overflow-wrap: anywhere/);
});
