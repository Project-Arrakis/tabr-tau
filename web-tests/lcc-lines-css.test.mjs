// The style of the twelve lines (M4): a line of five uses the full strength of the win token (not the 0.5 dial of the plaque) on the Houses of the board, the moving border is drawn only with Motion Full and no
// reduced-motion request, every other case has the still outline, and forced colours have an outline and a border that do not depend on colour. These are checks of the style text; the computed styles in a real
// browser engine are read by the capture script and listed in docs/design/landsraad-command-center/07-m4-sysselraad.md.
import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { lccDir } from './lcc-load.mjs';

const strip = (f) => fs.readFileSync(path.join(lccDir, f), 'utf8').replace(/\r/g, '').replace(/\/\*[\s\S]*?\*\//g, '');
const board = strip('lcc-board.css');
const base = strip('lcc.css');
const rule = (css, sel) => { const i = css.indexOf(sel + ' {'); assert.ok(i >= 0, 'rule not found: ' + sel); return css.slice(i, css.indexOf('}', i) + 1); };
// the content of the nearest forced-colors block that follows the M4 marker's rules
const forced = board.slice(board.indexOf('@media (forced-colors: active) {', board.indexOf('.lcc-bd-line[data-status="unknown"]')));
const forcedBlock = forced.slice(0, forced.indexOf(String.fromCharCode(10) + '}' + String.fromCharCode(10)) + 3);

test('the circuit outline of a House of the board uses the win outline and the full win glow, never the dial of the plaque', () => {
  const r = rule(board, '.lcc-hc[data-victory="circuit"]');
  assert.match(r, /outline:\s*var\(--lcc-outline-win\)/);
  assert.match(r, /calc\(75% \* var\(--lcc-glow-win\)\)/);
  assert.doesNotMatch(r, /var\(--lcc-glow\)/, 'the 0.5 dial scales the plaque and the rest of the page only');
  const d = rule(board, '.lcc-hc[data-victory="dominion"]');
  assert.match(d, /var\(--lcc-glow-win\)/);
  assert.doesNotMatch(d, /var\(--lcc-glow\)/);
  // the token is a plain number, 1, and is not made from the dial
  assert.match(base, /--lcc-glow-win:\s*1;/);
  assert.doesNotMatch(base.match(/--lcc-glow-win:[^;]*;/)[0], /--lcc-glow\b(?!-win)/);
  assert.match(base, /--lcc-glow:\s*0?\.5/, 'the dial for the rest of the page is still 0.5');
  // the circuit is the stronger outline of the two
  assert.match(rule(board, '.lcc-hc[data-victory="circuit"]'), /0 0 20px 5px/);
  assert.match(d, /0 0 14px 4px/);
});

test('the moving border exists once, moves by transform only, and is drawn only with Motion Full and no reduced-motion request', () => {
  const kf = base.match(/@keyframes lcc-win-sweep\s*\{([^]*?\})\s*\}/);
  assert.ok(kf);
  assert.match(kf[1], /rotate:\s*360deg/);
  assert.doesNotMatch(kf[1], /\b(width|height|top|left|margin|padding|border)\b/);
  assert.equal((base.match(/animation:\s*lcc-win-sweep/g) || []).length, 1);
  const block = base.match(/@media \(prefers-reduced-motion: no-preference\)\s*\{([^]*?)\n\}\n/);
  assert.ok(block, 'the no-preference block');
  assert.match(block[1], /html\[data-motion="full"\] \.lcc-win-anim > \.lcc-win-sweep \{/);
  assert.match(block[1], /opacity:\s*calc\(\.85 \* var\(--lcc-glow-win\)\)/, 'the border is also at the win strength');
  // outside that block the sweep is not shown
  assert.match(base, /\.lcc-win-sweep\s*\{\s*display:\s*none;\s*\}/);
  // the one-time reveal of a winning House stops under Calm, Off and reduced motion
  assert.match(base, /@media \(prefers-reduced-motion: reduce\)\s*\{\s*\.lcc \[data-victory\], \.lcc-scope \[data-victory\] \{ animation: none; \}/);
  assert.match(base, /html\[data-motion="calm"\] \.lcc \[data-victory\], html\[data-motion="off"\] \.lcc \[data-victory\]/);
  // the board's own rules for a winning House start no animation
  for (const sel of ['.lcc-hc[data-victory="circuit"]', '.lcc-hc[data-victory="dominion"]']) assert.doesNotMatch(rule(board, sel), /animation/, sel);
});

test('forced colours: a House of a line has a thick solid border and outline, a House of the 13 a double one, the panel a double border, and no moving border', () => {
  assert.match(forcedBlock, /\.lcc-hc\[data-victory="circuit"\] \{ border: 4px solid Highlight; outline: 3px solid Highlight; box-shadow: none; \}/);
  assert.match(forcedBlock, /\.lcc-hc\[data-victory="dominion"\] \{ border: 5px double CanvasText; outline: 3px solid CanvasText; box-shadow: none; \}/);
  assert.match(forcedBlock, /\.lcc-bd-victory\[data-state="line"\][^{]*\{ border: 3px double CanvasText; box-shadow: none; \}/);
  assert.match(base, /@media \(forced-colors: active\)[^]*html\[data-motion\] \.lcc-win-anim > \.lcc-win-sweep \{ display: none; \}/, 'the sweep is not drawn in forced colours');
  // the two outlines differ by their shape as well as their colour, so they are told apart without colour
  assert.notEqual(forcedBlock.match(/data-victory="circuit"\] \{ border: ([^;]*);/)[1], forcedBlock.match(/data-victory="dominion"\] \{ border: ([^;]*);/)[1]);
});

test('the stage never hides a control: the moving border and the outline do not take a click, and a House of a line keeps its tier boxes above them', () => {
  assert.match(rule(base, 'html[data-motion="full"] .lcc-win-anim > .lcc-win-sweep'), /pointer-events:\s*none/);
  assert.match(rule(board, '.lcc-hc-tiers'), /z-index:\s*4/);
});

test('the new rules keep the text floor: nothing under 11 px, names and key labels 12 px or more', () => {
  const a = board.indexOf('.lcc-hc[data-victory="circuit"] {'), m4 = board.slice(a, board.indexOf('.lcc-bd-line[data-status="unknown"]', a));
  const sizes = [...m4.matchAll(/font:\s*(?:\d{3}\s+)?(\d+(?:\.\d+)?)px/g)].map((x) => Number(x[1]));
  assert.ok(sizes.length >= 3);
  for (const s of sizes) assert.ok(s >= 11, 'a font of ' + s + ' px');
  assert.match(rule(board, '.lcc-bd-line b'), /font:\s*700 12px/);
});
