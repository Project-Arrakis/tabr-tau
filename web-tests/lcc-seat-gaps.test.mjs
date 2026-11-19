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
// the 13th seat's outline is one path: the ring (two arcs) and the two wedges (#983). Its sideways reach is that of its arcs, the radius of the arc and half the stroke; the wedges run up and down
const arcR = (path) => Math.max(...[...path.getAttribute('d').matchAll(/A([\d.]+),([\d.]+)/g)].map((m) => Number(m[1])));
function outerR(g) {
  let m = 0;
  for (const c of g.querySelectorAll('circle')) m = Math.max(m, Number(c.getAttribute('r')) + strokeOf(c, g) / 2);
  for (const c of g.querySelectorAll('path.lcc-seat-halo')) m = Math.max(m, arcR(c) + strokeOf(c, g) / 2);
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
      for (const c of s.querySelectorAll('path.lcc-seat-halo')) assert.ok(arcR(c) + strokeOf(c, s) / 2 <= L.seatOuter(i) + 1e-9, a + '/' + h + ' outline of seat ' + (i + 1));
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
  const edge = (c) => arcR(c) + Number(c.getAttribute('stroke-width')) / 2;
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

// ---- the 13th seat: a ring about twice a seat with an upward wedge on its top and a downward one on its bottom, one outline, and a separate faceted gem above (#983, #982) ----
// the points of a path of M, L, A and Z commands (the end point of an arc), in order
const pts = (d) => [...d.matchAll(/([MLA])([^MLAZ]*)/g)].map((m) => { const n = m[2].trim().split(/[\s,]+/).map(Number); return m[1] === 'A' ? [n[5], n[6]] : [n[0], n[1]]; });
const centre = (a, h) => seats(plaque(a, h))[12];

test('the 13th seat is a ring between 1.8 and 2 times the width of an ordinary seat, drawn in every state with the same outer edge', () => {
  for (const [a, h] of [[0, 0], [5, 5], [13, 0], [0, 13]]) {
    const g = centre(a, h);
    const ratio = outerR(g) / L.TRACK.seat;
    assert.ok(ratio >= 1.8 && ratio <= 2, a + '/' + h + ': ratio ' + ratio);
    assert.equal(outerR(g), L.TRACK.halo);
  }
});

test('the ring and its two wedges are ONE closed outline: a single path, one subpath, whose wedges are mirror images of each other', () => {
  for (const [a, h] of [[0, 0], [13, 0]]) {
    const g = centre(a, h);
    const halos = g.querySelectorAll('.lcc-seat-halo');
    assert.equal(halos.length, 1, 'one outline, not a ring with a triangle placed on it');
    const d = halos[0].getAttribute('d');
    assert.equal((d.match(/M/g) || []).length, 1, 'one subpath');
    assert.equal((d.match(/Z/g) || []).length, 1, 'closed');
    assert.equal(halos[0].getAttribute('fill'), 'none');
    const p = pts(d);
    const has = (x, y) => p.some((q) => Math.abs(q[0] - x) < 0.002 && Math.abs(q[1] - y) < 0.002);
    // symmetric left to right and top to bottom: every corner has its mirror images
    for (const [x, y] of p) { assert.ok(has(-x, y), 'left-right mirror of ' + x + ',' + y); assert.ok(has(x, -y), 'top-bottom mirror of ' + x + ',' + y); }
    const tips = p.filter(([x]) => x === 0);
    assert.equal(tips.length, 2, 'two tips');
    assert.deepEqual(tips.map((t) => t[1]).sort((u, v) => u - v), [-L.TRACK.wedge, L.TRACK.wedge], 'one pointing up, one pointing down, the same distance from the centre');
    // the wedges are filled and are mirror images too: the same base corners and the same tip distance
    const up = g.querySelector('.lcc-seat-wedge-top'), down = g.querySelector('.lcc-seat-wedge-bottom');
    const pu = pts(up.getAttribute('d')), pd = pts(down.getAttribute('d'));
    pu.forEach(([x, y], i) => { assert.ok(pd.some((q) => Math.abs(q[0] - x) < 0.002 && Math.abs(q[1] + y) < 0.002), 'wedge corner ' + i + ' is mirrored'); });
    // the corners of a wedge's base are on the ring, so the rim runs on into the wedge sides without a seam
    const rr = arcR(halos[0]);
    for (const [x, y] of pu.filter(([x]) => x !== 0)) assert.ok(Math.abs(Math.hypot(x, y) - rr) < 0.01, 'base corner is on the ring: ' + Math.hypot(x, y) + ' against ' + rr);
  }
});

test('the wedges reach the rails of the track and hang past them, the rails stay where they were, and the tips stay clear of the plaque below (#982)', () => {
  const T = L.TRACK;
  const half = T.bottom - T.cy; // the distance of a rail from the centre line of the seats
  assert.equal(T.cy - T.top, half);
  assert.ok(T.wedge > half, 'the tip is past the rail');
  assert.ok(T.wedge - half < 12, 'no more than a short point beyond the rail');
  assert.ok(Math.hypot(T.wedgeHalf, 0) < T.halo, 'the base of a wedge is inside the ring');
  const d = centre(0, 0).querySelector('.lcc-seat-halo').getAttribute('d');
  const baseY = Math.max(...pts(d).map((q) => Math.abs(q[1])).filter((v) => v < T.wedge));
  assert.ok(baseY < half, 'the corners of the base are inside the rails: the rail runs behind the wedge');
});

test('a faceted diamond gem floats above the upward wedge, a 1 to 2 unit gap between the outer edge of the wedge tip and the gem, and it is separate from the outline', () => {
  for (const [a, h] of [[0, 0], [13, 0]]) {
    const g = centre(a, h);
    const gem = g.querySelector('.lcc-seat-gem');
    assert.ok(gem, 'the gem is drawn');
    assert.equal(gem.querySelectorAll('.lcc-seat-facet').length, 4, 'four facets');
    const outline = [...gem.querySelectorAll('path')].find((x) => x.getAttribute('fill') === 'none');
    const ys = pts(outline.getAttribute('d')).map((q) => q[1]);
    const gemBottomEdge = Math.max(...ys) + Number(outline.getAttribute('stroke-width')) / 2; // outer edge of its lower point (round join)
    const halo = g.querySelector('.lcc-seat-halo');
    const tipTopEdge = -(L.TRACK.wedge + strokeOf(halo, g) / 2); // outer edge of the upward wedge's tip
    const gap = tipTopEdge - gemBottomEdge;
    assert.ok(gap >= 1 - 1e-9 && gap <= 2 + 1e-9, a + '/' + h + ': gap ' + gap);
    // about 40 percent of the ring's diameter in width; centred on the seat; not joined to the outline
    const xs = pts(outline.getAttribute('d')).map((q) => q[0]);
    const w = Math.max(...xs) - Math.min(...xs);
    assert.ok(w / (2 * L.TRACK.halo) > 0.35 && w / (2 * L.TRACK.halo) < 0.45, 'gem width ' + w);
    assert.equal(Math.max(...xs) + Math.min(...xs), 0);
    assert.ok(!halo.contains(gem) && !gem.contains(halo));
    assert.equal(gem.getAttribute('aria-hidden'), 'true');
  }
});

test('the gem and the wedges are inside the seat group: they move and scale with the seat', () => {
  const g = centre(13, 0);
  for (const sel of ['.lcc-seat-gem', '.lcc-seat-wedge-top', '.lcc-seat-wedge-bottom', '.lcc-seat-halo', '.lcc-glow-seat']) assert.ok(g.querySelector(sel), sel);
  const p = plaque(13, 0);
  assert.equal(p.querySelectorAll('.lcc-seat-gem').length, 1);
  assert.equal(p.querySelectorAll('.lcc-seat-wedge').length, 2);
});

test('the travelling arc of a win runs round the ring only: the wedges are cut out of its clip', () => {
  const g = centre(13, 0);
  const sweep = g.querySelector('.lcc-seat-win-sweep');
  assert.ok(sweep.parentNode.getAttribute('clip-path'), 'clipped');
  const id = /url\(#([^)]+)\)/.exec(sweep.parentNode.getAttribute('clip-path'))[1];
  const clip = g.querySelector('clipPath[id="' + id + '"] path');
  assert.equal(clip.getAttribute('clip-rule'), 'evenodd');
  assert.ok(clip.getAttribute('d').includes(g.querySelector('.lcc-seat-wedge-top').getAttribute('d')) && clip.getAttribute('d').includes(g.querySelector('.lcc-seat-wedge-bottom').getAttribute('d')));
});
