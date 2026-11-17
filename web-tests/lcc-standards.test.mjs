// The two cloth standards of the plaque (M2, standards review): the five-line list with a very long name, the empty state, the mirror between the two standards, the foot of the cloth
// against the nameplates, and the contrast of the words on the cloth. Pure DOM and file tests (no layout engine); the pixels are in the captures named in 05-m2-visual-foundation.md.
import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { load, lccDir } from './lcc-load.mjs';

const { L, doc } = load();
const css = fs.readFileSync(path.join(lccDir, 'lcc.css'), 'utf8').replace(/\r/g, '').replace(/\/\*[\s\S]*?\*\//g, '');
const js = fs.readFileSync(path.join(lccDir, 'lcc.js'), 'utf8');
const W = 2172;
const LONG = 'A guild name that is far too long to fit on a cloth standard in any layout, with an unbroken_part_that_has_no_space_at_all_to_wrap_on';
const plaque = (c) => L.renderPlaque({ a: 13, h: 12, holder: 'a', decree: { name: 'N', effect: 'E' }, contributors: c });

test('five contributor lines with a very long name on both standards: five items, the text exact, markup never, the full name kept in the title', () => {
  const hostile = '<b>x</b><img src=x onerror=alert(1)>' + LONG;
  const lines = ['One', LONG, 'Three', hostile, 'Five'];
  const p = plaque({ a: lines, h: lines.slice().reverse() });
  doc.body.appendChild(p);
  for (const [cls, want] of [['.lcc-pl-std-a', lines], ['.lcc-pl-std-h', lines.slice().reverse()]]) {
    const items = [...p.querySelectorAll(cls + ' ol > li')];
    assert.equal(items.length, 5);
    assert.deepEqual(items.map((i) => i.textContent), want);
    for (const li of items) {
      assert.equal(li.children.length, 1, 'one span per item, nothing else');
      assert.equal(li.firstElementChild.className, 'lcc-standard-name');
      assert.equal(li.firstElementChild.getAttribute('title'), li.textContent, 'the title is the whole name');
    }
    assert.equal(p.querySelector(cls + ' ol').getAttribute('role'), 'list');
  }
  assert.equal(p.querySelectorAll('.lcc-standard-list b, .lcc-standard-list script').length, 0);
  assert.equal(p.querySelectorAll('img:not(.lcc-standard-img):not(.lcc-standard-tip)').length, 0);
  assert.ok([...p.querySelectorAll('*')].every((e) => ![...e.attributes].some((a) => /^on/i.test(a.name))));
  p.remove();
});

test('more than five lines show five, and a line that is not a string is shown as text, not dropped or run', () => {
  const s = L.standard('h', ['a', 'b', 'c', 'd', 'e', 'f', 'g']);
  assert.deepEqual([...s.querySelectorAll('li')].map((i) => i.textContent), ['a', 'b', 'c', 'd', 'e']);
  assert.deepEqual([...L.standard('a', [42]).querySelectorAll('li')].map((i) => i.textContent), ['42']);
});

test('the empty state is honest and the same on both standards: no list, the words, the crest and the heading still there', () => {
  for (const f of ['a', 'h']) {
    for (const lines of [null, [], undefined]) {
      const s = L.standard(f, lines, 'TOP CONTRIBUTORS', 'Not in the save');
      assert.equal(s.querySelector('.lcc-standard-list'), null);
      assert.equal(s.querySelector('.lcc-standard-none').textContent, 'Not in the save');
      assert.equal(s.querySelector('.lcc-standard-h').textContent, 'TOP CONTRIBUTORS');
      assert.ok(s.querySelector('.lcc-standard-mark'));
    }
  }
});

// the points of an absolute path (M, L, H, V)
function points(d) {
  const out = []; let x = 0, y = 0;
  for (const m of d.matchAll(/([MLHV])\s*(-?[\d.]+)(?:[ ,]+(-?[\d.]+))?/g)) {
    if (m[1] === 'H') x = Number(m[2]); else if (m[1] === 'V') y = Number(m[2]); else { x = Number(m[2]); y = Number(m[3]); }
    out.push([x, y]);
  }
  return out;
}
const key = (pts) => pts.map(([x, y]) => x.toFixed(2) + ',' + y.toFixed(2)).sort().join(' ');

test('mirror: the Harkonnen cloth, rod, crest and foot picture are the Atreides ones mirrored across the middle of the drawing', () => {
  const p = plaque({ a: ['x'], h: ['y'] });
  const art = p.querySelector('svg.lcc-art');
  const cloth = (end) => [...art.querySelectorAll('path')].filter((e) => (e.getAttribute('fill') || '').endsWith(end + ')') && /V494/.test(e.getAttribute('d')));
  const ca = cloth('em'), ch = cloth('cr');
  assert.equal(ca.length, 1); assert.equal(ch.length, 1);
  assert.equal(key(points(ch[0].getAttribute('d'))), key(points(ca[0].getAttribute('d')).map(([x, y]) => [W - x, y])), 'the cloth outline mirrors');
  const ga = ca[0].parentNode, gh = ch[0].parentNode;
  assert.equal(ga.children.length, gh.children.length);
  const mid = (e) => {
    const t = e.tagName.toLowerCase();
    if (t === 'circle') return Number(e.getAttribute('cx'));
    if (t === 'rect' || t === 'image') return Number(e.getAttribute('x')) + Number(e.getAttribute('width')) / 2;
    return null;
  };
  // the two rod finials swap sides: the set of their centres mirrors
  const fin = (g) => [...g.querySelectorAll('circle')].map((c) => Number(c.getAttribute('cx')));
  assert.deepEqual(fin(gh).sort((x, y) => x - y), fin(ga).map((x) => W - x).sort((x, y) => x - y));
  let ornaments = 0;
  [...ga.children].forEach((e, i) => {
    const o = gh.children[i];
    assert.equal(e.tagName, o.tagName);
    const isTip = e.getAttribute('class') === 'lcc-art-tip';
    const ma = mid(e), mh = mid(o);
    if (ma != null && e.tagName !== 'circle' && !isTip) assert.ok(Math.abs(ma + mh - W) < 0.01, e.tagName + ' ' + i + ' is mirrored (' + ma + ' / ' + mh + ')');
    if (isTip) {
      ornaments++;
      // the foot pictures are two different files (the two cloths were painted apart), each put at its own place; their gold outlines are mirrors of each other within a pixel
      const gold = (el, off) => Number(el.getAttribute('x')) + off; // the middle of the gold outline: 154 in the Atreides picture, 163 in the Harkonnen one
      assert.equal(e.getAttribute('y'), o.getAttribute('y'), 'same height');
      assert.ok(Math.abs(gold(e, 154) + gold(o, 163) - W) <= 1, 'the two points are mirrors within a pixel');
    }
  });
  assert.equal(ornaments, 1, 'one foot picture per cloth, seen once while walking the first');
});

test('the foot of each cloth is the approved picture, one file per faction, at its own place; the narrow cloth carries the same picture as a text-free piece', () => {
  const art = plaque({ a: ['x'], h: ['y'] }).querySelector('svg.lcc-art');
  const tips = [...art.querySelectorAll('image.lcc-art-tip')];
  assert.deepEqual(tips.map((t) => t.getAttribute('href')), ['/lcc/art/standards/atreides-tip.png', '/lcc/art/standards/harkonnen-tip.png']);
  assert.deepEqual(tips.map((t) => [t.getAttribute('x'), t.getAttribute('y'), t.getAttribute('width'), t.getAttribute('height')]), [['36', '464', '320', '111'], ['1819', '464', '319', '111']]);
  assert.ok(tips.every((t) => t.getAttribute('mask').startsWith('url(#') && t.getAttribute('mask').endsWith('tip)')), 'each picture fades into the drawing');
  assert.ok(!art.querySelector('.lcc-art-sprig'), 'no drawn ornament: the wheat is in the picture');
  for (const f of ['a', 'h']) {
    const s = L.standard(f, ['x'], 'TOP CONTRIBUTORS', 'none');
    const foot = s.querySelector('.lcc-standard-foot');
    assert.ok(foot);
    assert.equal(foot.getAttribute('aria-hidden'), 'true');
    assert.equal(foot.textContent, '');
    assert.equal(foot.querySelector('img.lcc-standard-tip').getAttribute('src'), '/lcc/art/standards/' + (f === 'a' ? 'atreides' : 'harkonnen') + '-tip.png');
    assert.equal(foot.querySelector('img').getAttribute('alt'), '');
  }
  assert.ok(css.includes('.lcc-pl-std-a .lcc-standard-foot, .lcc-pl-std-h .lcc-standard-foot { display: none; }'), 'wide: the foot is in the artwork');
  assert.ok(css.includes('.lcc-standard-foot { display: none; }'), 'forced colours: the picture is dropped');
});

test('mirror in the stylesheet: one rule sizes both word boxes, they sit at mirrored places, and every selector for one has its twin', () => {
  const left = (c) => Number((c === 'a' ? /\.lcc-pl-std-a \{ left: ([\d.]+)%/ : /\.lcc-pl-std-h \{ left: ([\d.]+)%/).exec(css)[1]);
  const box = /\.lcc-plaque \.lcc-pl-std-a, \.lcc-plaque \.lcc-pl-std-h \{ top: ([\d.]+)%; width: ([\d.]+)%; height: ([\d.]+)%/.exec(css);
  assert.ok(box);
  assert.ok(Math.abs(left('a') + Number(box[2]) + left('h') - 100) < 0.011, 'left ' + left('a') + ' + width ' + box[2] + ' + right gap ' + (100 - left('h') - Number(box[2])));
  assert.equal((css.match(/lcc-pl-std-a/g) || []).length, (css.match(/lcc-pl-std-h/g) || []).length, 'each selector for the Atreides standard has its Harkonnen twin');
});

test('wide: heading on one line, long names cut with an ellipsis, the words between the crest and the ornament; narrow: names wrap and the padding is deeper than the foot', () => {
  assert.match(css, /\.lcc-pl-std-a \.lcc-standard-h, \.lcc-pl-std-h \.lcc-standard-h \{[^}]*white-space: nowrap/);
  assert.match(css, /\.lcc-pl-std-a \.lcc-standard-name, \.lcc-pl-std-h \.lcc-standard-name \{[^}]*white-space: nowrap; overflow: hidden; text-overflow: ellipsis/);
  assert.match(css, /\.lcc-standard-name \{[^}]*overflow-wrap: anywhere/);
  const box = /\.lcc-plaque \.lcc-pl-std-a, \.lcc-plaque \.lcc-pl-std-h \{ top: ([\d.]+)%; width: [\d.]+%; height: ([\d.]+)%/.exec(css);
  const top = (Number(box[1]) / 100) * 724, bottom = ((Number(box[1]) + Number(box[2])) / 100) * 724;
  // the crest is a picture 88 units square centred at y 246, of which about three quarters is the crest; the ornament starts at y 497
  assert.match(js, /STANDARD_MARK = 88/);
  assert.match(js, /cx, 246, STANDARD_MARK/);
  assert.ok(top >= 246 + (88 * 0.75) / 2 + 8, 'the words start at least 8 units under the crest: ' + top);
  assert.ok(bottom <= 503, 'the words end above the point of the foot picture: ' + bottom);
  // the foot picture starts at y 464 and ends at 575, above the nameplates (their top is at y 578)
  assert.ok(js.includes('y: 464, width: tip.width, height: tip.height'));
  assert.ok(js.includes('TIP_H = 111'));
  // the narrow cloth: the picture follows the words in the flow, so a line of names never runs into it; its gold outline is lined up with the edge of the cloth
  assert.match(css, /[.]lcc-standard-cloth [{][^}]*padding: [0-9]+px [0-9]+px [0-9]+px;/);
  assert.ok(css.includes('.lcc-f-a > .lcc-standard-foot { width: 127%; margin-left: -11.11%;'));
  assert.ok(css.includes('.lcc-f-h > .lcc-standard-foot { width: 125.59%; margin-left: -14.17%;'));
});

test('forced colours, wide: the standard is an outlined box with the drawn mark, not bare words', () => {
  const m = /@container \(min-width: 900px\) \{\s*@media \(forced-colors: active\) \{([\s\S]*?)\n  \}\n\}/.exec(css);
  assert.ok(m, 'the wide forced-colours block exists');
  assert.match(m[1], /border: 2px solid CanvasText/);
  assert.match(m[1], /\.lcc-standard-mark \{ display: block/);
});

test('the words on the cloth pass 4.5:1 at the top and the foot of both cloths: names, heading and numbers, and the empty-state text', () => {
  const lum = (hex) => { const c = [1, 3, 5].map((i) => parseInt(hex.slice(i, i + 2), 16) / 255).map((v) => (v <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4)); return 0.2126 * c[0] + 0.7152 * c[1] + 0.0722 * c[2]; };
  const ratio = (a, b) => { const [x, y] = [lum(a), lum(b)].sort((p, q) => q - p); return (x + 0.05) / (y + 0.05); };
  for (const cloth of ['#0f6a54', '#063a2e', '#8c1219', '#4a070c']) for (const fg of ['#fdf6e6', '#ffd98a', '#f4e6c3']) assert.ok(ratio(fg, cloth) >= 4.5, fg + ' on ' + cloth + ': ' + ratio(fg, cloth).toFixed(2));
});
