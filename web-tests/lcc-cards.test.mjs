// The House card and the specialization lane card of the Command Center (M2): state is words as well as colour, the XP cap and the keystones are two separate facts, a missing picture
// loses nothing, and text from a save is only ever text. Pure DOM tests of the helpers, with made-up values.
import test from 'node:test';
import assert from 'node:assert/strict';
import { load } from './lcc-load.mjs';

const { L, doc } = load();
const lane = (o) => L.renderLaneCard(Object.assign({ track: 'combat', level: 10, maxLevel: 100, xp: 1000, maxXp: 44182, keystonesOwned: 3, keystonesTotal: 41 }, o));
const tags = (c) => [...c.querySelectorAll('.lcc-lane-tags .lcc-flag')].map((t) => t.textContent);

test('a lane at level 100 shows the XP cap and says nothing about keystones; all keystones at level 0 shows the keystones and no cap', () => {
  const cap = lane({ level: 100, xp: 44182, keystonesOwned: 0 });
  assert.deepEqual(tags(cap), ['✦ XP CAP REACHED']);
  assert.equal(cap.getAttribute('data-xpcap'), 'true');
  assert.equal(cap.getAttribute('data-ks-complete'), null);
  assert.match(cap.getAttribute('aria-label'), /XP cap reached/);
  assert.match(cap.getAttribute('aria-label'), /0 of 41 keystones/);
  assert.doesNotMatch(cap.getAttribute('aria-label'), /all keystones/);
  const ks = lane({ level: 0, xp: 0, keystonesOwned: 41 });
  assert.deepEqual(tags(ks), ['KEYSTONES COMPLETE']);
  assert.equal(ks.getAttribute('data-xpcap'), null);
  assert.equal(ks.getAttribute('data-ks-complete'), 'true');
  const both = lane({ level: 100, xp: 44182, keystonesOwned: 41 });
  assert.deepEqual(tags(both), ['✦ FULL PATH COMPLETE']);
  assert.equal(both.getAttribute('data-full'), 'true');
  const mid = lane({ level: 60, xp: 21000, keystonesOwned: 0 });
  assert.deepEqual(tags(mid), ['PATH IN PROGRESS']);
});

test('level 0, 60 and 100 with keystones 0 of 41 and 41 of 41: six cards, each with the right numbers and tags', () => {
  const rows = [[0, 0, 0], [0, 0, 41], [60, 21000, 0], [60, 21000, 41], [100, 44182, 0], [100, 44182, 41]];
  const seen = rows.map(([level, xp, ks]) => lane({ level, xp, keystonesOwned: ks }));
  assert.deepEqual(seen.map((c) => c.querySelector('.lcc-lane-level b').textContent), ['0', '0', '60', '60', '100', '100']);
  assert.deepEqual(seen.map((c) => c.querySelector('.lcc-lane-nums').textContent.split(' · ')[1]), ['0/41 keystones', '41/41 keystones', '0/41 keystones', '41/41 keystones', '0/41 keystones', '41/41 keystones']);
  assert.deepEqual(seen.map((c) => c.getAttribute('data-xpcap')), [null, null, null, null, 'true', 'true']);
});

test('an unknown keystone total is never complete, and an unidentified track says so', () => {
  const u = lane({ keystonesOwned: 0, keystonesTotal: 0, level: 100, xp: 44182 });
  assert.match(u.querySelector('.lcc-lane-nums').textContent, /keystones unknown/);
  assert.equal(u.getAttribute('data-ks-complete'), null);
  assert.equal(u.getAttribute('data-full'), null);
  assert.deepEqual(tags(lane({ unresolved: true })), ['TRACK NOT IDENTIFIED']);
  assert.equal(L.laneState({ xp: 5, maxXp: 0, keystonesOwned: 1, keystonesTotal: 0 }).capped, false, 'a cap of 0 is unknown, not reached');
});

test('the XP bar is the share of the overall cap, floored, so one XP short of the cap never reads as full', () => {
  assert.equal(L.laneState({ xp: 44181, maxXp: 44182, keystonesTotal: 41 }).pct, 99);
  assert.equal(L.laneState({ xp: 44182, maxXp: 44182, keystonesTotal: 41 }).pct, 100);
  assert.equal(L.laneState({ xp: -5, maxXp: 44182 }).pct, 0);
  assert.equal(L.laneState({ xp: 99999, maxXp: 44182 }).pct, 100);
  const bar = lane({ xp: 22091 }).querySelector('[role="progressbar"]');
  assert.equal(bar.getAttribute('aria-valuenow'), '50');
  assert.equal(bar.querySelector('.lcc-lane-fill').style.getPropertyValue('--lcc-v'), '50%');
  assert.equal(bar.querySelector('.lcc-lane-fill').getAttribute('style') && bar.querySelector('.lcc-lane-fill').getAttribute('style').includes('--lcc-v'), true);
});

test('no card says MASTERED, and a lane card is one native button with a full name', () => {
  for (const c of [lane({ level: 100, xp: 44182, keystonesOwned: 41 }), lane({}), lane({ selected: true })]) {
    assert.equal(c.tagName, 'BUTTON');
    assert.equal(c.getAttribute('type'), 'button');
    assert.doesNotMatch(c.textContent + c.getAttribute('aria-label'), /master/i);
    assert.ok(['true', 'false'].includes(c.getAttribute('aria-pressed')));
  }
  assert.match(lane({ selected: true }).getAttribute('aria-label'), /selected$/);
  assert.equal(lane({ track: 'gathering' }).querySelector('.lcc-lane-img').getAttribute('src'), '/lcc/art/lanes/gathering.webp');
  assert.equal(lane({ track: 'gathering' }).querySelector('.lcc-lane-img').getAttribute('alt'), '');
});

const house = (o) => L.renderHouseCard(Object.assign({ name: 'DA_HouseEcaz', owner: 'open', track: 'combat' }, o));

test('a House card says its owner in words and in a mark, not only in colour', () => {
  const t = (o) => house(o).querySelector('.lcc-owner').textContent;
  assert.equal(t({ owner: 'a' }), 'Atreides', 'one mark: the name, never a letter and the name (#950)');
  assert.equal(t({ owner: 'h' }), 'Harkonnen');
  assert.equal(t({ owner: 'open' }), '○ Not won');
  assert.equal(t({ owner: 'unknown' }), '? Unreadable');
  assert.equal(t({ owner: 'nonsense' }), '? Unreadable', 'an owner that is not one of the four is unreadable, never open');
  assert.equal(house({ owner: 'nonsense' }).getAttribute('data-owner'), 'unknown');
  assert.match(house({ owner: 'a' }).getAttribute('aria-label'), /^Ecaz, favor won by Atreides, specialization Combat$/);
  assert.match(house({ owner: 'open' }).getAttribute('aria-label'), /^Ecaz, favor not won,/);
  assert.match(house({ owner: 'unknown' }).getAttribute('aria-label'), /favor unreadable/);
});

test('one away, selected, rewards waiting and the two victory outlines each have text and an attribute, and the outlines never combine', () => {
  const c = house({ owner: 'open', oneAway: true, selected: true, waiting: 2 });
  assert.equal(c.getAttribute('data-one-away'), 'true');
  assert.equal(c.getAttribute('aria-pressed'), 'true');
  assert.deepEqual([...c.querySelectorAll('.lcc-flag')].map((f) => f.textContent), ['ONE AWAY', '2 WAITING']);
  assert.match(c.getAttribute('aria-label'), /one House away from a line, 2 rewards waiting, selected/);
  assert.match(house({ waiting: 1 }).getAttribute('aria-label'), /1 reward waiting/);
  const line = house({ owner: 'a', inLine: true, win: true });
  assert.equal(line.getAttribute('data-victory'), 'win');
  assert.deepEqual([...line.querySelectorAll('.lcc-flag')].map((f) => f.textContent), ['SYSSELRAAD WON']);
  const dom = house({ owner: 'h', win: true });
  assert.equal(dom.getAttribute('data-victory'), 'win');
  assert.deepEqual([...dom.querySelectorAll('.lcc-flag')].map((f) => f.textContent), ['SYSSELRAAD WON'], 'a House of a 13-House win has the same outline and the same flag');
  const both = house({ owner: 'a', inLine: true, win: true });
  assert.equal(both.getAttribute('data-victory'), 'win', 'a House is shown with one outline');
  assert.equal(house({ owner: 'a' }).getAttribute('data-victory'), null);
  const el = doc.createElement('div');
  L.victory(el, true); assert.equal(el.getAttribute('data-victory'), 'win');
  L.victory(el, null); assert.equal(el.getAttribute('data-victory'), null);
});

test('a House card draws the symbol and the path banner from the lists; a House with no symbol keeps its initials and loses nothing', () => {
  const c = house({ name: 'Mikarrol' });
  assert.equal(c.querySelector('.lcc-house-img').getAttribute('src'), '/lcc/art/houses/mikkarol.webp');
  assert.equal(c.querySelector('.lcc-house-path-img').getAttribute('src'), '/lcc/art/banners/combat.webp');
  assert.equal(c.querySelector('.lcc-house-path span').textContent, 'Combat');
  const none = house({ name: 'DA_HouseNobody', track: 'unknown path' });
  assert.equal(none.querySelector('.lcc-house-img'), null);
  assert.equal(none.querySelector('.lcc-house-initials').textContent, 'NO');
  assert.equal(none.querySelector('.lcc-house-path'), null);
  assert.equal(none.querySelector('.lcc-house-name').textContent, 'Nobody');
  // a picture that fails to load is hidden; the name and the state stay
  const img = c.querySelector('.lcc-house-img');
  img.dispatchEvent(new (doc.defaultView.Event)('error'));
  assert.equal(img.hidden, true);
  assert.equal(c.querySelector('.lcc-house-name').textContent, 'Mikarrol', 'the name shown is the name given');
  for (const a of c.querySelectorAll('img')) assert.equal(a.getAttribute('alt'), '', 'decorative pictures have no alt: the text beside them names them');
});

test('names from a save are only text on both cards', () => {
  const hostile = '<img src=x onerror=alert(1)>';
  const c = L.renderHouseCard({ name: hostile, owner: 'a', track: hostile });
  assert.equal(c.querySelectorAll('img, script, a').length, 0);
  assert.equal(c.querySelector('.lcc-house-name').textContent.toLowerCase(), hostile.toLowerCase().replace(/^./, (x) => x.toUpperCase()).toLowerCase());
  const l = L.renderLaneCard({ track: hostile, level: hostile, xp: hostile, keystonesOwned: hostile, keystonesTotal: hostile });
  assert.equal(l.querySelectorAll('img, script, a').length, 0);
  assert.match(l.querySelector('.lcc-lane-name').textContent, /Unknown path/);
  assert.doesNotMatch(l.textContent, /onerror/);
  assert.ok([...c.querySelectorAll('*'), ...l.querySelectorAll('*')].every((e) => ![...e.attributes].some((a) => /^on/i.test(a.name))));
});

test('the small parts: the frame, the nameplate, HELD BY, the standard, the badges and the tooltip', () => {
  const f = L.frame([doc.createElement('p')]);
  assert.ok(f.classList.contains('lcc-frame') && f.querySelector('.lcc-frame-in p'));
  assert.equal(L.pill('House Atreides').textContent, 'House Atreides');
  const held = L.heldBy('HOUSE HARKONNEN', 'h');
  assert.equal(held.querySelector('.lcc-heldby-l').textContent, 'HELD BY');
  assert.equal(held.querySelector('.lcc-heldby-n').textContent, 'HOUSE HARKONNEN');
  assert.equal(L.heldBy('', '').querySelector('.lcc-heldby-n').textContent, 'Not recorded');
  assert.equal(L.ruleV().getAttribute('aria-hidden'), 'true');
  assert.deepEqual([...L.standard('a', ['x', 'y']).querySelectorAll('li')].map((i) => i.textContent), ['x', 'y']);
  assert.equal(L.standard('h', Array.from({ length: 9 }, (_, i) => 'n' + i)).querySelectorAll('li').length, 5, 'a standard lists five at most');
  assert.equal(L.standard('a', null).querySelector('.lcc-standard-none').textContent, 'Not in the save');
  assert.equal(L.badge('demo').textContent, 'FIXTURE · DEMO DATA');
  assert.equal(L.badge('readonly').textContent, 'READ-ONLY');
  assert.equal(L.badge('nonsense').className, 'lcc-badge lcc-badge-readonly');
  const tip = L.richTip('Why', 'Because.');
  const id = tip.querySelector('.lcc-tip').id;
  assert.equal(tip.querySelector('button').getAttribute('aria-describedby'), id);
  assert.equal(tip.querySelector('.lcc-tip').getAttribute('role'), 'tooltip');
  assert.notEqual(L.richTip('a', 'b').querySelector('.lcc-tip').id, id, 'two tips never share an id');
});
