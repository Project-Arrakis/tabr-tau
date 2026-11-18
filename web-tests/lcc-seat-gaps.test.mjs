// The 25 seats of the plaque keep one gap between every two neighbouring circles, measured from outer edge to outer edge (#965), and nothing a seat draws goes beyond its own outer edge (#964).
// The edge of a seat is read from what the drawing really contains (every circle's radius plus half its stroke, every picture's half box), not from a figure the code states, so a seat that is
// drawn wider than the space it was given fails here. The gold ring of the 13th seat is wider than a seat: its gap to the two neighbours is the same as every other gap. Pure DOM tests; the pixel
// side (the same measurement in a real browser engine at 1600, 1280, 1024, 768, 390 and 320 wide) is in the pull request that made them.
import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { load, lccDir } from './lcc-load.mjs';

const { L } = load();
const css = fs.readFileSync(path.join(lccDir, 'lcc.css'), 'utf8').replace(/\r/g, '').replace(/\/\*[\s\S]*?\*\//g, '');
const plaque = (a, h) => L.renderPlaque({ a, h, decree: { name: 'N', effect: 'E' }, holder: 'a' });
const seats = (p) => [...p.querySelectorAll('.lcc-seat')];
const cx = (g) => Number(/translate\(([-\d.]+),/.exec(g.getAttribute('transform'))[1]);
// the CSS stroke of a won 13th seat is the one the drawing carries too: read it from the stylesheet so a rule that overrides the drawing is counted
const cssWinStroke = Number(/\.lcc-seat-win13 \.lcc-seat-halo\s*\{[^}]*stroke-width:\s*([\d.]+)/.exec(css)[1]);
const strokeOf = (c, g) => {
  const sw = Number(c.getAttribute('stroke-width') || 0);
  return c.classList.contains('lcc-seat-halo') && g.classList.contains('lcc-seat-win13') ? Math.max(sw, cssWinStroke) : sw;
};
function outerR(g) {
  let m = 0;
  for (const c of g.querySelectorAll('circle')) m = Math.max(m, Number(c.getAttribute('r')) + strokeOf(c, g) / 2);
  for (const im of g.querySelectorAll('image')) m = Math.max(m, Number(im.getAttribute('width')) / 2, Number(im.getAttribute('height')) / 2);
  return m;
}
const gapsOf = (p) => { const g = seats(p); const out = []; for (let i = 0; i < 24; i++) out.push(cx(g[i + 1]) - outerR(g[i + 1]) - (cx(g[i]) + outerR(g[i]))); return out; };
const spread = (a) => Math.max(...a) - Math.min(...a);

// every (Atreides, Harkonnen) pair of 0 to 12 won seats a side, and the 13-House wins (the 13th seat then carries the ring and the arc of the win)
const cases = [];
for (let a = 0; a <= 12; a++) for (let h = 0; h <= 12; h++) cases.push([a, h]);
cases.push([13, 0], [0, 13], [13, 12], [12, 13], [25, 0], [0, 25]);
// the plaque's widths (the drawing is one coordinate system of 2172 units; the narrow layouts show it at most at about 1.42 times the width of the column)
const WIDTHS = [1600, 1280, 1024, 768, 390, 320];

test('all 24 gaps between neighbouring seat edges are equal, for every count of won seats and for a win of 13 Houses', () => {
  for (const [a, h] of cases) {
    const gaps = gapsOf(plaque(a, h));
    assert.equal(gaps.length, 24);
    assert.ok(gaps.every((x) => x > 0), a + '/' + h + ': no two circles touch or overlap (' + gaps.join(', ') + ')');
    assert.ok(spread(gaps) < 0.001, a + '/' + h + ': gaps ' + gaps.join(', '));
    // the same in pixels: under 1 px at the plaque widths a person sees, however the drawing is scaled
    for (const w of WIDTHS) assert.ok(spread(gaps) * ((w * 1.4215) / L.GEOMETRY.W) < 1, a + '/' + h + ' at ' + w);
  }
});

test('the gap beside the gold 13th seat is the same on its left and on its right and the same as every other gap', () => {
  for (const [a, h] of [[0, 0], [1, 1], [2, 2], [3, 3], [12, 4], [13, 12], [12, 13]]) {
    const gaps = gapsOf(plaque(a, h));
    assert.ok(Math.abs(gaps[11] - gaps[12]) < 0.001, a + '/' + h + ': left ' + gaps[11] + ' right ' + gaps[12]);
    assert.ok(Math.abs(gaps[11] - gaps[0]) < 0.001, a + '/' + h + ': beside the gold seat ' + gaps[11] + ' against ' + gaps[0]);
  }
});

test('a claimed seat takes no more room than an open one: the same gaps with 0, 1, 2 and 3 won seats on each side', () => {
  const base = gapsOf(plaque(0, 0));
  for (const [a, h] of [[1, 0], [2, 0], [3, 0], [0, 1], [0, 2], [0, 3], [2, 3], [3, 2]]) {
    const gaps = gapsOf(plaque(a, h));
    gaps.forEach((x, i) => assert.ok(Math.abs(x - base[i]) < 0.001, a + '/' + h + ' gap ' + (i + 1)));
  }
  // and every seat box has the one outer size, apart from the gold one
  for (const [a, h] of [[0, 0], [2, 3], [13, 12]]) {
    const g = seats(plaque(a, h));
    g.forEach((s, i) => assert.equal(outerR(s), L.seatOuter(i), a + '/' + h + ' seat ' + (i + 1)));
    assert.equal(new Set(g.filter((_, i) => i !== L.CENTER).map(outerR)).size, 1);
  }
});

test('nothing a seat draws goes beyond its own outer edge: the ring around an emblem is inside its seat box', () => {
  for (const [a, h] of [[0, 0], [2, 2], [12, 12], [13, 12], [12, 13]]) {
    const p = plaque(a, h);
    seats(p).forEach((s, i) => {
      for (const c of s.querySelectorAll('circle')) {
        assert.ok(Number(c.getAttribute('r')) + strokeOf(c, s) / 2 <= L.seatOuter(i) + 1e-9, a + '/' + h + ' seat ' + (i + 1) + ' ' + c.getAttribute('class'));
      }
      for (const im of s.querySelectorAll('image')) {
        assert.ok(Number(im.getAttribute('width')) / 2 <= L.seatOuter(i) + 1e-9, 'picture of seat ' + (i + 1));
        // a picture is never cut by its own box (a fractional pixel scale could shave the ring off its right edge)
        assert.equal(im.getAttribute('overflow'), 'visible', 'picture of seat ' + (i + 1));
      }
    });
  }
});

test('the won ring of the 13th seat has the same outer edge as the still ring, and the drawing agrees with the stylesheet', () => {
  const still = plaque(12, 4).querySelector('.lcc-seat-halo');
  const won = plaque(13, 12).querySelector('.lcc-seat-halo');
  const edge = (c) => Number(c.getAttribute('r')) + Number(c.getAttribute('stroke-width')) / 2;
  assert.equal(edge(still), L.TRACK.halo);
  assert.equal(edge(won), L.TRACK.halo);
  assert.equal(Number(won.getAttribute('stroke-width')), cssWinStroke, 'the stroke in the drawing is the one the stylesheet sets');
  const sweep = plaque(13, 12).querySelector('.lcc-seat-win-sweep');
  assert.equal(Number(sweep.getAttribute('r')) + Number(sweep.getAttribute('stroke-width')) / 2, L.TRACK.halo, 'the travelling arc stays inside the ring edge');
});

test('the seats stay inside the track and the two ends are inset by the same amount', () => {
  const T = L.TRACK;
  const g = seats(plaque(2, 2));
  const first = cx(g[0]) - outerR(g[0]), last = cx(g[24]) + outerR(g[24]);
  assert.ok(first > T.wallL && last < T.wallR);
  assert.equal(first - T.wallL, T.wallR - last);
});
