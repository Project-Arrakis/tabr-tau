// #1051, first slice: the headings, eyebrows, labels, disclosure titles and Show buttons of the Command Center (Term and Decrees, the Landsraad board and its Inspector, the Specialization page)
// are drawn in the look's own text colours (--lcc-heading, --lcc-heading-quiet), not in gold. The gold that stays (the plaque and banners, the win, icons and emblems, the active decree, the
// figures and the flags) still names a gold token.
import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const lccDir = path.join(path.dirname(fileURLToPath(import.meta.url)), '..', 'internal', 'web', 'static', 'lcc');

const FILES = ['lcc.css', 'lcc-term.css', 'lcc-board.css', 'lcc-spec.css'];
const strip = (s) => s.replace(/\r/g, '').replace(/\/\*[\s\S]*?\*\//g, '');
const src = Object.fromEntries(FILES.map((f) => [f, strip(fs.readFileSync(path.join(lccDir, f), 'utf8'))]));

// every innermost rule { selectors[], decls{} } of one file (the rules inside an @media block too)
function rulesOf(text) {
  const out = []; const re = /([^{}]+)\{([^{}]*)\}/g; let m;
  while ((m = re.exec(text))) {
    const decls = {};
    for (const d of m[2].split(';')) { const i = d.indexOf(':'); if (i > 0) decls[d.slice(0, i).trim()] = d.slice(i + 1).trim(); }
    out.push({ sels: m[1].split(',').map((s) => s.trim().replace(/\s+/g, ' ')), decls });
  }
  return out;
}
const all = Object.fromEntries(FILES.map((f) => [f, rulesOf(src[f])]));
// the forced-colours rules (system colours: CanvasText and the like) stay as they are, so they are left out
const SYSTEM = /^(CanvasText|ButtonText|LinkText|Highlight|HighlightText|GrayText|ActiveText|inherit|currentColor)$/i;
const colorOf = (file, sel) => all[file].filter((r) => r.sels.includes(sel) && r.decls.color !== undefined && !SYSTEM.test(r.decls.color)).map((r) => r.decls.color);
const GOLD = /--lcc-gold|--lcc-ap-|--lcc-sp-gold|--c-gold/;
const CALM = /^var\(--lcc-heading(-quiet)?\)$/;

const MOVED = [
  ['lcc-term.css', '.lcc-tm .lcc-tm-eyebrow', '--lcc-heading-quiet'],
  ['lcc-term.css', '.lcc-tm .lcc-tm-h4', '--lcc-heading'],
  ['lcc-term.css', '.lcc-tm-lbl', '--lcc-heading-quiet'],
  ['lcc-term.css', '.lcc-tm .lcc-tm-disc > summary', '--lcc-heading'],
  ['lcc-term.css', '.lcc-tm-showhide', '--lcc-heading'],
  ['lcc-term.css', '.lcc-tm .lcc-tm-all > summary', '--lcc-heading'],
  ['lcc-board.css', '.lcc-bd .lcc-bd-eyebrow', '--lcc-heading-quiet'],
  ['lcc-board.css', '.lcc-bd-editor .lcc-bd-sec-h', '--lcc-heading'],
  ['lcc-board.css', '.lcc-bd .lcc-bd-disc > summary', '--lcc-heading'],
  ['lcc-board.css', '.lcc-bd-grouprow th', '--lcc-heading'],
  ['lcc-spec.css', '.lcc-sp-eyebrow', '--lcc-heading-quiet'],
  ['lcc-spec.css', '.lcc-sp-h2', '--lcc-heading'],
  ['lcc-spec.css', '.lcc-sp-glabel span', '--lcc-heading-quiet'],
];

const KEPT = [
  ['lcc-term.css', '.lcc-tm-mkon'], ['lcc-term.css', '.lcc-tm-badge.is-active'],
  ['lcc-board.css', '.lcc-bd-emblem'], ['lcc-board.css', '.lcc-bd-sigil'],
  ['lcc-board.css', '.lcc-bd-victory[data-state="line"] .lcc-bd-vtag'],
  ['lcc.css', '.lcc-flag-one'], ['lcc.css', '.lcc-lane-level'],
  ['lcc-spec.css', '.lcc-sp-level'], ['lcc-spec.css', '.lcc-sp-stat b'], ['lcc-spec.css', '.lcc-sp-flag.is-cap'], ['lcc-spec.css', '.lcc-sp-lane-lv b'],
];

test('the two heading tokens are defined from the look\'s own text tokens, not from gold', () => {
  const root = all['lcc.css'].find((r) => r.sels.includes('.lcc') && r.decls['--lcc-heading'] !== undefined);
  assert.ok(root, '--lcc-heading is defined on .lcc, .lcc-scope');
  assert.equal(root.decls['--lcc-heading'], 'var(--lcc-text)');
  assert.equal(root.decls['--lcc-heading-quiet'], 'color-mix(in srgb, var(--lcc-dim) 50%, var(--lcc-text))');
  assert.match(src['lcc.css'], /--lcc-text:\s*var\(--c-text\)/);
  assert.match(src['lcc.css'], /--lcc-dim:\s*var\(--c-dim\)/);
  // no Dayside or palette block redefines them as a gold
  for (const r of all['lcc.css']) for (const k of ['--lcc-heading', '--lcc-heading-quiet']) if (r.decls[k] !== undefined) assert.doesNotMatch(r.decls[k], GOLD, k);
});

for (const [file, sel, token] of MOVED) {
  test('heading colour: ' + sel + ' uses ' + token + ', not gold', () => {
    const cs = colorOf(file, sel);
    assert.ok(cs.length > 0, sel + ' has a colour rule in ' + file);
    for (const c of cs) { assert.doesNotMatch(c, GOLD, sel + ' still names a gold: ' + c); assert.match(c, CALM, sel); }
    assert.ok(cs.every((c) => c === 'var(' + token + ')'), sel + ' should use ' + token + ': ' + cs.join(' | '));
  });
}

for (const [file, sel] of KEPT) {
  test('the gold that stays: ' + sel, () => {
    const cs = colorOf(file, sel);
    assert.ok(cs.length > 0, sel + ' has a colour rule in ' + file);
    assert.ok(cs.some((c) => GOLD.test(c)), sel + ' lost its gold: ' + cs.join(' | '));
  });
}

test('no heading, eyebrow, label, disclosure title or Show button of the four files is gold text any more', () => {
  const words = /(eyebrow|-h4?\b|-h2\b|sec-h|lbl|disc > summary|all > summary|showhide|grouprow|glabel)/;
  for (const f of FILES) for (const r of all[f]) for (const s of r.sels) {
    if (!words.test(s) || r.decls.color === undefined || SYSTEM.test(r.decls.color)) continue;
    assert.doesNotMatch(r.decls.color, GOLD, f + ': ' + s + ' { color: ' + r.decls.color + ' }');
  }
});
