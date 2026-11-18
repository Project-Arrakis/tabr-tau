// Every seat of the plaque is a symmetric ring (#971): the shading does not favour one side, so the right edge of a circle is drawn as the left one is, and nothing a seat draws is off its own centre
// or beyond its outer edge. Before this the seat gradients were lit from the upper left (the edge of a circle was 15 to 20 percent darker on the right than on the left, which on the dark red and
// dark green of the track looked like a cut-off circle) and the two medallion pictures carry lighting of their own (bright rim on one side). Pure DOM tests; the pixel side (the mean light of the left
// and right half of the rim of an open seat, a won Atreides seat, a won Harkonnen seat and the gold 13th seat, in a real browser engine at device scales 1, 1.25, 1.5 and 2) is in the pull request.
import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { load, lccDir } from './lcc-load.mjs';

const { L } = load();
const css = fs.readFileSync(path.join(lccDir, 'lcc.css'), 'utf8').replace(/\r/g, '').replace(/\/\*[\s\S]*?\*\//g, '');
const plaque = (a, h) => L.renderPlaque({ a, h, decree: { name: 'N', effect: 'E' }, holder: 'a' });
const num = (v) => Number(v == null || v === '' ? 0 : v);
const OUT = L.TRACK.seat;

test('the four seat gradients are centred on the seat: the same colour at the left and right edge', () => {
  const p = plaque(3, 3);
  const grads = [...p.querySelectorAll('radialGradient')];
  assert.equal(grads.length, 4, 'open, Atreides, Harkonnen and gold seat');
  for (const g of grads) {
    assert.equal(g.getAttribute('cx'), '50%', g.id);
    assert.equal(g.getAttribute('cy'), '50%', g.id);
    assert.equal(g.getAttribute('r'), '50%', g.id);
    assert.equal(g.getAttribute('fx'), null, g.id + ': no focal point off the centre');
    assert.equal(g.getAttribute('fy'), null, g.id + ': no focal point off the centre');
    assert.equal(g.getAttribute('gradientTransform'), null, g.id);
    assert.ok([null, 'objectBoundingBox'].includes(g.getAttribute('gradientUnits')), g.id);
    assert.ok(g.querySelectorAll('stop').length >= 3);
  }
});

test('every circle and picture of every seat is centred on the seat and the right half is drawn inside the seat box', () => {
  for (const [a, h] of [[0, 0], [5, 5], [12, 12], [13, 0], [0, 13]]) {
    const p = plaque(a, h);
    for (const g of p.querySelectorAll('.lcc-seat')) {
      const centre = g.classList.contains('lcc-seat-center');
      const edge = centre ? L.TRACK.halo : OUT;
      for (const c of g.querySelectorAll('circle')) {
        assert.equal(num(c.getAttribute('cx')), 0, 'slot ' + g.dataset.slot + ' circle is centred (x)');
        assert.equal(num(c.getAttribute('cy')), 0, 'slot ' + g.dataset.slot + ' circle is centred (y)');
        assert.ok(num(c.getAttribute('r')) + num(c.getAttribute('stroke-width')) / 2 <= edge + 1e-9, 'slot ' + g.dataset.slot + ': the right edge of the ring is inside the seat box');
      }
      for (const im of g.querySelectorAll('image')) {
        const w = num(im.getAttribute('width')), hh = num(im.getAttribute('height'));
        assert.equal(num(im.getAttribute('x')), -w / 2, 'slot ' + g.dataset.slot + ' picture is centred (x)');
        assert.equal(num(im.getAttribute('y')), -hh / 2, 'slot ' + g.dataset.slot + ' picture is centred (y)');
      }
    }
  }
});

test('a claimed seat has a flat ring of its faction colour over the picture, centred and inside the seat, so the lighting of the medallion art does not make one side dull', () => {
  const p = plaque(4, 4);
  for (const g of p.querySelectorAll('.lcc-seat-a, .lcc-seat-h')) {
    if (g.classList.contains('lcc-seat-center')) continue;
    const rim = g.querySelector('circle.lcc-seat-rim');
    assert.ok(rim, 'slot ' + g.dataset.slot + ' has the ring');
    assert.equal(num(rim.getAttribute('cx')), 0);
    assert.equal(num(rim.getAttribute('cy')), 0);
    assert.match(rim.getAttribute('stroke'), /^#[0-9a-f]{6}$/i, 'a flat colour, not a gradient');
    assert.equal(rim.getAttribute('fill'), 'none');
    assert.ok(num(rim.getAttribute('r')) + num(rim.getAttribute('stroke-width')) / 2 < OUT, 'inside the seat outline');
    const kids = [...g.children];
    assert.ok(kids.indexOf(rim) > kids.indexOf(g.querySelector('image')), 'drawn over the picture');
  }
  // open seats and the gold seat have no such ring (their gradient is already symmetric); the two factions differ in colour
  assert.equal(p.querySelectorAll('.lcc-seat-o circle.lcc-seat-rim, .lcc-seat-center circle.lcc-seat-rim').length, 0);
  const strokes = new Set([...p.querySelectorAll('circle.lcc-seat-rim')].map((c) => c.getAttribute('stroke')));
  assert.equal(strokes.size, 2);
});

test('forced colours hide the ring with the pictures, so a seat is told apart by the drawn mark and the fill alone', () => {
  const block = css.slice(css.indexOf('@media (forced-colors: active)'));
  assert.match(block, /\.lcc-seat-rim\s*\{\s*display:\s*none/);
});
