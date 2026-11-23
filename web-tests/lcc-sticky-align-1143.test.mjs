// #1143: while the page is scrolled, the House Intelligence panel is held at the same offset as the board's control strip, so the two columns start on one line (it was held 12 px lower), and it keeps
// 12 px to the bottom of the window (it kept 12 px before, too: the height and the offset moved together). jsdom has no layout: the two rules are read from the style files; the measured numbers
// (the boxes of both at several scroll positions, in headless Edge) are in the pull request.
import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const read = (...p) => fs.readFileSync(path.join(here, '..', 'internal', 'web', 'static', ...p), 'utf8').replace(/\/\*[\s\S]*?\*\//g, '');
const APP = read('app.css'), BOARD = read('lcc', 'lcc-board.css');

// the offset as a comparable text: the fallback of the header height does not change the value
const norm = (s) => s.replace(/var\(--hdr-h, 56px\)/g, 'var(--hdr-h)').replace(/\s+/g, ' ').trim();

test('#1143: the sticky offset of the Inspector is the offset of the control strip', () => {
  const strip = BOARD.match(/\.lcc-k-e6 \.lcc-k-strip \{ top: (calc\([^;]*\)); \}/);
  assert.ok(strip, 'the strip is held under the header and the warning bars');
  const insp = APP.match(/\.lsb #lsb-board \.lcc-bd-insp \{ position: sticky; top: (calc\([^;]*\));/);
  assert.ok(insp, 'the Inspector is sticky');
  assert.equal(norm(insp[1]), norm(strip[1]), 'one offset for both');
  assert.equal(norm(insp[1]), 'calc(var(--hdr-h) + var(--lsb-warn-room, 0px))');
});

test('#1143: the Inspector is as tall as the window allows and keeps 12 px at the bottom: the top moved up by 12 px, so the height grew by 12 px', () => {
  const m = APP.match(/\.lsb #lsb-board \.lcc-bd-insp \{ position: sticky;[^}]*max-height: calc\(100vh - var\(--hdr-h\) - var\(--lsb-warn-room, 0px\) - (\d+)px\); max-height: calc\(100dvh - var\(--hdr-h\) - var\(--lsb-warn-room, 0px\) - (\d+)px\);/);
  assert.ok(m, 'both height rules (vh for the old browsers, dvh)');
  assert.deepEqual([m[1], m[2]], ['12', '12'], 'the gap to the bottom of the window is 12 px, as before (it was 12 px of top offset and 24 px taken off the height)');
});

test('#1143: the side-by-side layout is as it was: the same grid areas, the column stretches to the board panel, the Inspector sits under the victory analysis and scrolls inside itself', () => {
  assert.match(APP, /\.lsb #lsb-board \.lcc-bd-grid > \.lcc-bd-side \{ grid-area: sd; margin-top: 0; align-self: stretch; \}/);
  assert.match(APP, /align-self: flex-start; width: 100%; box-sizing: border-box;\s*max-height:/);
  assert.match(APP, /overflow-y: auto; overflow-x: hidden; overscroll-behavior: contain; \}/);
});
