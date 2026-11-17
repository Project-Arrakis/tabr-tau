// The voting plaque of the Command Center (M2): the 25 seats and the gold 13th, the equal spacing and the equal inset of the two ends, the strict reading of the counts, the single
// continuous HELD BY rule, the gold line between the nameplate and the count, and the guarantee that text from a save is only ever text. Pure DOM tests: no layout engine, so the pixel
// side (the crops against the approved picture) is in docs/design/landsraad-command-center/05-m2-visual-foundation.md.
import test from 'node:test';
import assert from 'node:assert/strict';
import { load } from './lcc-load.mjs';

const { L, doc } = load();
const plaque = (vm) => L.renderPlaque(Object.assign({ decree: { name: 'N', effect: 'E' }, holder: 'h' }, vm));
const seats = (p) => [...p.querySelectorAll('.lcc-seat')];
const cx = (g) => Number(/translate\(([-\d.]+),/.exec(g.getAttribute('transform'))[1]);
const cy = (g) => Number(/translate\([-\d.]+,([-\d.]+)\)/.exec(g.getAttribute('transform'))[1]);
const stateOf = (p) => seats(p).map((g) => g.getAttribute('data-state')[0]).join('');

test('seatStates: Atreides fill from the left, Harkonnen from the right, the 13th seat is the middle one', () => {
  assert.equal(L.SEATS, 25);
  assert.equal(L.CENTER, 12);
  const s = (a, h) => L.seatStates(a, h).map((x) => (x.state === 'open' ? '.' : x.state)).join('');
  assert.equal(s(12, 4), 'aaaaaaaaaaaa.........hhhh');
  assert.equal(s(13, 12), 'aaaaaaaaaaaaahhhhhhhhhhhh');
  assert.equal(s(12, 13), 'aaaaaaaaaaaahhhhhhhhhhhhh');
  assert.equal(s(0, 0), '.'.repeat(25));
  assert.equal(s(6, 10), 'aaaaaa' + '.'.repeat(9) + 'h'.repeat(10));
  assert.equal(s(25, 0), 'a'.repeat(25));
  assert.equal(s(0, 25), 'h'.repeat(25));
  for (const [a, h] of [[12, 4], [13, 12], [12, 13], [0, 0], [25, 0], [0, 25], [1, 24]]) {
    const st = L.seatStates(a, h);
    assert.equal(st.length, 25);
    assert.deepEqual(Array.from(st.map((x) => x.slot)), Array.from({ length: 25 }, (_, i) => i + 1));
    assert.deepEqual(Array.from(st.filter((x) => x.center).map((x) => x.slot)), [13]);
    assert.equal(st.filter((x) => x.state === 'a').length, a);
    assert.equal(st.filter((x) => x.state === 'h').length, h);
  }
  // 13th seat: it takes the faction of the cut when claimed, and stays open and gold otherwise
  assert.equal(L.seatStates(13, 12)[12].state, 'a');
  assert.equal(L.seatStates(12, 13)[12].state, 'h');
  assert.equal(L.seatStates(12, 4)[12].state, 'open');
});

test('seatStates reads the counts strictly: anything that is not two whole numbers that fit in 25 together is null', () => {
  for (const [a, h] of [[13, 13], [26, 0], [0, 26], [-1, 0], [0, -1], [1.5, 0], ['12', 4], [null, 4], [undefined, 0], [NaN, 0], [Infinity, 0], [12, '4'], [{}, 1], [[12], 4]]) {
    assert.equal(L.seatStates(a, h), null, JSON.stringify([a, h]));
  }
});

test('the plaque draws exactly 25 seats, one of them the gold 13th, for every count that fits', () => {
  for (const [a, h, want] of [[12, 4, 'aaaaaaaaaaaa.........hhhh'], [13, 12, 'a'.repeat(13) + 'h'.repeat(12)], [12, 13, 'a'.repeat(12) + 'h'.repeat(13)], [0, 0, '.'.repeat(25)]]) {
    const p = plaque({ a, h });
    const g = seats(p);
    assert.equal(g.length, 25, 'never a 26th seat');
    assert.equal(g.map((x) => (x.getAttribute('data-state') === 'open' ? '.' : x.getAttribute('data-state'))).join(''), want);
    assert.deepEqual(Array.from(g.map((x) => x.getAttribute('data-slot'))), Array.from({ length: 25 }, (_, i) => String(i + 1)));
    const centre = p.querySelectorAll('.lcc-seat[data-center="true"]');
    assert.equal(centre.length, 1);
    assert.equal(centre[0].getAttribute('data-slot'), '13');
    assert.ok(centre[0].classList.contains('lcc-seat-center'));
    assert.equal(centre[0].querySelectorAll('.lcc-seat-halo').length, 1, 'the gold halo is outside the base circle of the 13th seat');
    assert.equal(p.querySelectorAll('.lcc-seat-halo').length, 1, 'only the 13th seat has a halo');
    // the 13th has a gold border whatever it holds; the other seats do not
    const gold = '#fff0b8';
    assert.equal(centre[0].querySelector('.lcc-seat-base').getAttribute('stroke'), gold);
    for (const o of g.filter((x) => x !== centre[0])) assert.notEqual(o.querySelector('.lcc-seat-base').getAttribute('stroke'), gold);
    // a claimed 13th seat carries the owner's mark inside the gold circle
    const mark = centre[0].querySelectorAll('.lcc-seat-mark').length;
    assert.equal(mark, centre[0].getAttribute('data-state') === 'open' ? 0 : 1);
  }
});

test('the seats are equally spaced, share one size, sit on one line, and the two ends are inset by the same amount from the tips', () => {
  const T = L.TRACK;
  const p = plaque({ a: 12, h: 4 });
  const g = seats(p);
  const xs = g.map(cx);
  for (let i = 1; i < 25; i++) assert.equal(xs[i] - xs[i - 1], T.pitch, 'pitch at seat ' + (i + 1));
  assert.equal(xs[12], L.GEOMETRY.W / 2, 'the 13th seat is on the middle of the plaque and of the track');
  assert.ok(g.every((x) => cy(x) === T.cy));
  const radii = new Set(g.map((x) => x.querySelector('.lcc-seat-base').getAttribute('r')));
  assert.deepEqual([...radii], [String(T.r)], 'every seat has the same diameter');
  // the field of the seats is measured between the inner walls, not from the picture
  const first = xs[0] - T.r, last = xs[24] + T.r;
  assert.equal(first - T.wallL, T.wallR - last, 'Atreides and Harkonnen start the same distance from their own wall');
  assert.equal(T.cx - T.tipL, T.tipR - T.cx, 'the track is symmetric about the middle seat');
  assert.ok(T.haloR + T.r < T.pitch + T.r, 'the gold halo does not touch a neighbouring seat');
  assert.ok(T.pitch - T.haloR - T.r > 0, 'there is a gap between the halo and the next seat');
  for (let i = 0; i < 25; i++) assert.equal(L.seatX(i), xs[i]);
});

test('every seat is named in words: the slot, the owner and, for the 13th, that it is the gold middle seat; the track carries a number summary', () => {
  const p = plaque({ a: 13, h: 12 });
  const titles = seats(p).map((g) => g.querySelector('title').textContent);
  assert.equal(titles[0], 'Slot 1 of 25 — Atreides');
  assert.equal(titles[12], 'Slot 13 of 25 — Atreides (the gold middle seat)');
  assert.equal(titles[24], 'Slot 25 of 25 — Harkonnen');
  assert.equal(new Set(titles).size, 25);
  const sum = p.querySelector('.lcc-track-svg').getAttribute('aria-label');
  assert.match(sum, /Atreides 13, Harkonnen 12, open 0/);
  assert.match(sum, /Seat 13 is the gold middle seat/);
  assert.match(L.summaryText(12, 4), /Atreides 12, Harkonnen 4, open 9/);
  assert.equal(L.summaryText(30, 1), 'The tally of the 25 seats is not available.');
});

test('counts that cannot be read draw 25 open seats, say so, and light nothing', () => {
  for (const [a, h] of [[30, 2], [13, 13], [-1, 4], [null, null], [1.5, 2]]) {
    const p = plaque({ a, h });
    assert.equal(p.getAttribute('data-seats'), 'unavailable');
    assert.equal(seats(p).length, 25);
    assert.ok(seats(p).every((g) => g.getAttribute('data-state') === 'open'));
    assert.equal(p.querySelector('.lcc-pl-standing').textContent, 'Tally not available');
    assert.equal(p.querySelector('.lcc-track-svg').getAttribute('aria-label'), 'The tally of the 25 seats is not available.');
    assert.deepEqual([...p.querySelectorAll('.lcc-pl-count b')].map((b) => b.textContent), ['—', '—']);
  }
});

test('the counts under the two nameplates are the counts of the seats, never anything else', () => {
  const p = plaque({ a: 12, h: 4 });
  assert.deepEqual([...p.querySelectorAll('.lcc-pl-count')].map((n) => n.textContent), ['12 HOUSES', '4 HOUSES']);
  assert.equal(p.querySelectorAll('.lcc-seat[data-state="a"]').length, 12);
  assert.equal(p.querySelectorAll('.lcc-seat[data-state="h"]').length, 4);
  // the caption never claims a result: a lead is a count, and the vote is not confirmed
  for (const [a, h] of [[12, 4], [13, 12], [12, 13], [25, 0], [0, 25], [3, 3]]) {
    const t = L.standingLine(a, h);
    assert.doesNotMatch(t, /WIN|WON|SECURED|VICTORY|MAJORITY/i, t);
    if (a !== h) assert.match(t, /VOTE NOT CONFIRMED/);
  }
  assert.equal(L.standingLine(12, 4), 'ATREIDES LEAD 12 TO 4 · 9 HOUSES OPEN · VOTE NOT CONFIRMED');
  assert.equal(L.standingLine(4, 12), 'HARKONNEN LEAD 12 TO 4 · 9 HOUSES OPEN · VOTE NOT CONFIRMED');
  assert.equal(L.standingLine(12, 12), 'HOUSE COUNT EVEN AT 12 · 1 HOUSE OPEN');
  assert.equal(L.standingLine(40, 1), 'HOUSE COUNT UNAVAILABLE');
});

test('the artwork layer has one HELD BY rule, drawn last, so nothing in it is ever painted over the rule', () => {
  const p = plaque({ a: 12, h: 4 });
  const art = p.querySelector('.lcc-art');
  const held = art.querySelectorAll('.lcc-art-rule-held');
  assert.equal(held.length, 1, 'one continuous HELD BY rule');
  assert.equal(art.querySelectorAll('.lcc-art-rule').length, 2, 'the rule after the decree and the HELD BY rule, and no other rule');
  const kids = [...art.children];
  assert.equal(kids[kids.length - 1], held[0], 'the HELD BY rule is the last thing drawn');
  assert.equal(kids[kids.length - 2].classList.contains('lcc-art-rule'), true);
  const r = held[0];
  assert.equal(r.tagName.toLowerCase(), 'rect');
  assert.ok(Number(r.getAttribute('height')) >= 90 && Number(r.getAttribute('width')) <= 4, 'a tall thin rectangle, not a dashed or split line');
  // the rule stands clear of the HELD BY text box and of the shield (x 1966): it is left of the text
  const x = Number(r.getAttribute('x'));
  assert.ok(x < 1600 && x > 1500);
  // one element only: no second element in the layer sits on its column
  for (const k of kids) {
    if (k === r) continue;
    if (k.tagName.toLowerCase() === 'rect' && k.getAttribute('class') !== 'lcc-art-rule') {
      const kx = Number(k.getAttribute('x')), kw = Number(k.getAttribute('width')), ky = Number(k.getAttribute('y')), kh = Number(k.getAttribute('height'));
      const overlaps = kx < x + 4 && kx + kw > x && ky < 134 && ky + kh > 36;
      if (k !== [...art.children].find((c) => c.tagName.toLowerCase() === 'rect')) assert.equal(overlaps, false, 'a rect of the layer lies on the HELD BY rule');
    }
  }
  // in the narrow layout the same rule is the one DOM element; the wide layout hides it, so exactly one is ever shown
  assert.equal(p.querySelectorAll('.lcc-rule-v').length, 1);
});

test('no opaque patch is drawn over the artwork: the only solid dark fill is the field itself', () => {
  const art = plaque({ a: 12, h: 4 }).querySelector('.lcc-art');
  const solidDark = [...art.querySelectorAll('rect, path')].filter((e) => /^#(0|1)[0-9a-f]{5}$/i.test(e.getAttribute('fill') || ''));
  // the field, the hollow diamonds (their dark centre is the field's colour) and nothing that could cover text or a rule
  for (const e of solidDark) assert.ok(e.tagName.toLowerCase() === 'rect' ? e === [...art.children].find((c) => c.tagName.toLowerCase() === 'rect') : /^M\d+,6\d\d L/.test(e.getAttribute('d')), 'unexpected solid dark shape: ' + e.getAttribute('d'));
  assert.equal(art.querySelectorAll('text, foreignObject').length, 0, 'the artwork has no text in it');
  assert.deepEqual([...art.querySelectorAll('image')].map((i) => i.getAttribute('class')), ['lcc-art-crest', 'lcc-art-crest', 'lcc-art-crest', 'lcc-art-tip', 'lcc-art-crest', 'lcc-art-tip'], 'the only pictures in it are the four faction crests and the two cloth feet');
});

test('the gold line is BETWEEN the nameplate and the count and ends at the hollow diamond, centred on it', () => {
  const art = plaque({ a: 12, h: 4 }).querySelector('.lcc-art');
  const lines = [...art.querySelectorAll('.lcc-art-line')];
  assert.equal(lines.length, 2);
  const num = (e, k) => Number(e.getAttribute(k));
  // the nameplate shapes: gold paths from y 578 to 626
  const plates = [...art.querySelectorAll('path')].filter((e) => /^M\d+,602 L/.test(e.getAttribute('d')) && e.getAttribute('fill').startsWith('url'));
  assert.equal(plates.length, 2);
  const plateBottom = 626, countCentre = 660;
  const diamonds = [...art.querySelectorAll('path')].filter((e) => /^M\d+,616 L\d+,635 L\d+,654 L/.test(e.getAttribute('d')));
  assert.equal(diamonds.length, 2);
  for (const l of lines) {
    const y = num(l, 'y') + num(l, 'height') / 2;
    assert.ok(y > plateBottom + 4, 'below the plate: ' + y);
    assert.ok(y < countCentre - 12, 'above the count: ' + y);
    // the diamond whose middle is on the same line, and the line stops before its point
    const d = diamonds.find((e) => Math.abs(635 - y) < 1);
    assert.ok(d, 'a diamond is centred on the line');
    const m = /^M(\d+),616 L(\d+),635 L\d+,654 L(\d+),635 Z$/.exec(d.getAttribute('d'));
    const left = Number(m[3]), right = Number(m[2]), mid = Number(m[1]);
    assert.equal((left + right) / 2, mid);
    const lx0 = num(l, 'x'), lx1 = lx0 + num(l, 'width');
    assert.ok(lx1 <= left - 8 || lx0 >= right + 8, 'the line stops short of the diamond and never crosses it');
    assert.equal(d.getAttribute('fill'), '#090914', 'a hollow diamond');
  }
  // the DOM order of a faction is: the nameplate, then the count
  for (const f of ['a', 'h']) {
    const box = plaque({ a: 1, h: 1 }).querySelector('.lcc-pl-faction-' + f);
    assert.deepEqual([...box.children].map((c) => c.className.split(' ')[0]), ['lcc-pill', 'lcc-pl-count']);
  }
});

test('text from a save is only ever text: a decree, an effect, a contributor and a faction name cannot become markup', () => {
  const hostile = '<img src=x onerror=alert(1)><b>bold</b>"\'&amp;';
  const p = L.renderPlaque({ a: 3, h: 3, decree: { name: hostile, effect: hostile }, holder: 'a', contributors: { a: [hostile, hostile], h: [hostile] }, sessionLabel: hostile, standing: hostile });
  doc.body.appendChild(p);
  assert.equal(p.querySelectorAll('img:not(.lcc-standard-img):not(.lcc-standard-tip), script, iframe, object, embed, a, link, style').length, 0);
  assert.equal(p.querySelectorAll('img').length, 4, 'only the two faction crests and the two feet of the standards are pictures');
  assert.equal(p.querySelector('.lcc-pl-title').textContent, hostile);
  assert.equal(p.querySelector('.lcc-pl-effect').textContent, hostile);
  assert.equal(p.querySelector('.lcc-pl-session').textContent, hostile);
  assert.equal(p.querySelector('.lcc-pl-standing').textContent, hostile);
  assert.equal(p.querySelectorAll('.lcc-pl-std-a li').length, 2);
  assert.ok([...p.querySelectorAll('*')].every((e) => ![...e.attributes].some((a) => /^on/i.test(a.name) || a.name === 'style')));
  p.remove();
});

test('the placeholders are honest: no contributor names unless given, a missing decree and holder say so', () => {
  const p = L.renderPlaque({ a: 1, h: 1 });
  assert.equal(p.querySelectorAll('.lcc-standard-list').length, 0);
  assert.deepEqual([...p.querySelectorAll('.lcc-standard-none')].map((n) => n.textContent), ['Not in the save', 'Not in the save']);
  assert.equal(p.querySelector('.lcc-pl-title').textContent, 'No active decree');
  assert.equal(p.querySelector('.lcc-pl-effect'), null);
  assert.equal(p.querySelector('.lcc-heldby-n').textContent, 'Not recorded');
  assert.equal(L.renderPlaque({ a: 1, h: 1, demo: true }).querySelectorAll('.lcc-badge-demo').length, 1);
  assert.equal(p.querySelectorAll('.lcc-badge').length, 0, 'a live plaque carries no demo badge');
});

test('seat states are the same for the two layouts: one set of seats, no second track', () => {
  const p = plaque({ a: 12, h: 13 });
  assert.equal(p.querySelectorAll('.lcc-track').length, 1);
  assert.equal(p.querySelectorAll('.lcc-track-svg').length, 1);
  assert.equal(stateOf(p), 'a'.repeat(12) + 'h'.repeat(13));
  assert.equal(p.querySelector('.lcc-seat[data-center="true"]').getAttribute('data-state'), 'h');
  // each plaque draws its own gradients: two plaques on a page never share an id
  const q = plaque({ a: 1, h: 1 });
  const ids = (e) => [...e.querySelectorAll('[id]')].map((x) => x.id);
  assert.equal(ids(p).filter((i) => ids(q).includes(i)).length, 0);
  for (const e of p.querySelectorAll('[fill^="url(#"], [stroke^="url(#"]')) {
    const v = [e.getAttribute('fill'), e.getAttribute('stroke')].find((x) => x && x.startsWith('url(#'));
    const ref = /url\(#([^)]+)\)/.exec(v)[1];
    assert.ok(ids(p).includes(ref), 'a gradient that is not defined: ' + ref);
  }
});

test('the faction crests are pictures: header shields, cloth standards, claimed seats (medallion) and a claimed 13th seat (white crest); an open seat has none', () => {
  const p = plaque({ a: 13, h: 12 });
  const files = (sel) => [...p.querySelectorAll(sel)].map((i) => i.getAttribute('href'));
  assert.deepEqual(files('svg.lcc-art image.lcc-art-crest'), ['/lcc/art/factions/atreides.webp', '/lcc/art/factions/harkonnen.webp', '/lcc/art/factions/atreides.webp', '/lcc/art/factions/harkonnen.webp'], 'two shields, then two standards');
  assert.equal(p.querySelector('svg.lcc-art').getAttribute('aria-hidden'), 'true');
  const seatPics = [...p.querySelectorAll('.lcc-seat')].map((g) => [...g.querySelectorAll('image.lcc-seat-art')].map((i) => i.getAttribute('href')));
  assert.equal(seatPics.filter((a) => a.length === 1).length, 25);
  assert.equal(seatPics[0][0], '/lcc/art/factions/atreides-medallion.webp');
  assert.equal(seatPics[24][0], '/lcc/art/factions/harkonnen-medallion.webp');
  assert.equal(seatPics[12][0], '/lcc/art/factions/atreides.webp', 'the claimed 13th seat takes the white crest of its owner');
  const open = plaque({ a: 3, h: 2 });
  const op = [...open.querySelectorAll('.lcc-seat[data-state="open"]')];
  assert.equal(op.length, 20);
  assert.ok(op.every((g) => g.querySelectorAll('image').length === 0), 'an open seat carries no faction picture');
  // the seat's drawn mark stays as the fallback and the word of the state stays in its title
  assert.equal(open.querySelectorAll('.lcc-seat[data-state="a"] .lcc-seat-mark').length, 3);
  assert.match(open.querySelector('.lcc-seat[data-state="h"] title').textContent, /Harkonnen/);
  // a picture that has loaded covers the drawn mark (the stylesheet hides it); one that has not leaves it
  const g = open.querySelector('.lcc-seat[data-state="a"]');
  assert.ok(!g.classList.contains('lcc-seat-has-art'));
  g.querySelector('image.lcc-seat-art').dispatchEvent(new doc.defaultView.Event('load'));
  assert.ok(g.classList.contains('lcc-seat-has-art'));
  assert.ok(!open.querySelector('.lcc-seat[data-state="h"]').classList.contains('lcc-seat-has-art'));
  // a picture that fails to load draws nothing
  const im = open.querySelector('image.lcc-art-crest');
  im.dispatchEvent(new doc.defaultView.Event('error'));
  assert.equal(im.getAttribute('visibility'), 'hidden');
  // the drawing no longer carries vector faction marks: every shape of the shields and the standards is a frame, a cloth or a rod
  assert.equal(open.querySelector('svg.lcc-art path[d*="L"][fill="#efe9d6"]'), null);
});

test('a standard shows its faction crest as a picture with no alt, and the words stay when the picture is gone', () => {
  for (const [f, file] of [['a', 'atreides'], ['h', 'harkonnen']]) {
    const s = L.standard(f, ['x'], 'TOP CONTRIBUTORS', 'none');
    const img = s.querySelector('.lcc-standard-mark > img.lcc-standard-img');
    assert.equal(img.getAttribute('src'), '/lcc/art/factions/' + file + '.webp');
    assert.equal(img.getAttribute('alt'), '');
    assert.equal(s.querySelector('.lcc-standard-mark').getAttribute('aria-hidden'), 'true');
    img.dispatchEvent(new doc.defaultView.Event('error'));
    assert.equal(img.hidden, true);
    assert.equal(s.querySelector('.lcc-standard-h').textContent, 'TOP CONTRIBUTORS');
    assert.equal(s.querySelector('li').textContent, 'x');
  }
});
