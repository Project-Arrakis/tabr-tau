// The axis arrows of the Land claim editor (#457): the arrowheads and the +x / +y labels are drawn at the size they are meant to be.
// An SVG stroke with no stroke-width is one unit, and the grid is drawn in cell units, so an arrowhead (0.28 wide) was stroked one whole cell wide
// with mitered spikes. The page test cannot load the stylesheet, so the rules are read from app.css; the geometry is read from the drawn page.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { boot, closeAll } from './boot.mjs';

afterEach(closeAll);

const css = fs.readFileSync(path.join(path.dirname(fileURLToPath(import.meta.url)), '..', 'internal', 'web', 'static', 'app.css'), 'utf8');
// the declarations of the rule whose selector list is exactly this selector
const rule = (sel) => { const at = css.indexOf(sel + '{'); return at < 0 ? null : css.slice(at + sel.length + 1, css.indexOf('}', at)); };

const BASE7 = [{ base_id: 7, name: 'Base', base_type: 'Advanced Sub-Fief', owner: 'Owner', map: 'HaggaBasin', generators: 0, pieces: 0, placeables: 0, x: 0, y: 0, z: 0 }];
const claim = (yaw) => [{ totem_id: 7, name: 'Totem', cells: 2, rings: 0, level: 1, irregular: true, maxRings: 5, maxLevel: 5, yaw, segments: [{ x: 1, y: 0 }], grid: [{ x: 0, y: 0, in: true, n: 0 }, { x: 1, y: 0, in: true, n: 3 }] }];

async function open(yaw) {
  const ui = await boot({ get: { '/api/bases/list': BASE7, '/api/bases/claim': claim(yaw) } });
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="player:bases"]'); await ui.settle();
  ui.click('[data-bstab="claim"]'); await ui.settle();
  return ui;
}

test('the arrowheads have a thin stroke width and round joins, and the labels have no stroke', () => {
  const head = rule('.lc-axis-head');
  assert.ok(head, 'a rule for .lc-axis-head');
  const w = /stroke-width:\s*([0-9.]+)/.exec(head);
  assert.ok(w && Number(w[1]) > 0 && Number(w[1]) <= 0.1, 'the head has a stroke width of at most a tenth of a cell: ' + head);
  assert.ok(/stroke-linejoin:\s*round/.test(head), 'round joins, no mitered spikes: ' + head);
  const label = rule('.lc-axis-t.lc-axis-x,.lc-axis-t.lc-axis-y');
  assert.ok(label, 'a rule for the labels that comes after the colour rules');
  assert.ok(/stroke:\s*none/.test(label), 'the labels are filled text with no stroke: ' + label);
  assert.ok(css.indexOf('.lc-axis-t.lc-axis-x,.lc-axis-t.lc-axis-y') > css.indexOf('.lc-axis-x{'), 'the label rule is after the .lc-axis-x colour rule, so it wins');
  assert.ok(/\.lc-axis\{[^}]*stroke-width:/.test(css), 'the axis line keeps its own width');
});

for (const yaw of [0, 90, 165, 270]) {
  test(`at yaw ${yaw} the heads are small and the +x and +y labels are inside the drawing, turned upright`, async () => {
    const ui = await open(yaw);
    const svg = ui.doc.querySelector('svg.lc-svg');
    const [vx, vy, vw, vh] = svg.getAttribute('viewBox').split(' ').map(Number);
    const heads = [...svg.querySelectorAll('path.lc-axis-head')];
    assert.equal(heads.length, 2);
    assert.ok(svg.querySelector('path.lc-axis-head.lc-axis-x') && svg.querySelector('path.lc-axis-head.lc-axis-y'));
    for (const h of heads) assert.ok(h.getAttribute('d').length < 40 && /z$/.test(h.getAttribute('d')), 'a small closed triangle: ' + h.getAttribute('d'));
    const rad = (yaw * Math.PI) / 180;
    for (const t of svg.querySelectorAll('text.lc-axis-t')) {
      const tr = /^rotate\((-?[0-9.]+) (-?[0-9.]+) (-?[0-9.]+)\)$/.exec(t.getAttribute('transform'));
      assert.ok(tr && Number(tr[1]) === -yaw, 'turned back upright: ' + t.getAttribute('transform'));
      const px = Number(tr[2]), py = Number(tr[3]); // the label turns about its own anchor, so it stays where the grid's turn put it
      const wx = px * Math.cos(rad) - py * Math.sin(rad), wy = px * Math.sin(rad) + py * Math.cos(rad); // where the grid's turn puts it
      assert.ok(wx > vx + 0.3 && wx < vx + vw - 0.3 && wy > vy + 0.5 && wy < vy + vh - 0.3, `${t.textContent} at ${wx.toFixed(2)},${wy.toFixed(2)} is inside the viewBox ${vx},${vy},${vw},${vh}`);
    }
    assert.deepEqual([...svg.querySelectorAll('text.lc-axis-t')].map((t) => t.textContent), ['+x', '+y']);
  });
}
