// Go to a place (#519) is one drop-down of places, filled at once: trading posts, the character's own deployed bases, vehicles and respawn points, and taxi hubs, each under its heading.
// A place chosen in it fills Teleport X, Y and Z, and what was chosen or typed in the Movement card stays there while the page is drawn again (#506: the game saves and the character's
// saved position moves; an action elsewhere on the page; a refused change), until a Teleport succeeds.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import { boot, closeAll } from './boot.mjs';

afterEach(closeAll);

const TP = { key: 'tradepost', label: 'Trading posts' }, BASE = { key: 'base', label: 'Personal bases' }, VEH = { key: 'vehicle', label: 'Vehicles' }, RESP = { key: 'respawn', label: 'Respawn points' }, TAXI = { key: 'taxi', label: 'Taxis' };

// Two trading posts: the answer of a save with nothing else to go to.
const dest = () => ({ map: 'HaggaBasin', sameMap: true, total: 2, groups: [TP], destinations: [
  { group: 'tradepost', label: 'NearPost', x: 1111.4, y: 2222.6, z: 33, distance: 448800 },
  { group: 'tradepost', label: 'FarPost', x: 9000, y: 8000, z: 70, distance: 5200000 }] });

// Every heading has a place.
const named = () => ({ map: 'HaggaBasin', sameMap: true, total: 6, groups: [TP, BASE, VEH, RESP, TAXI],
  destinations: [
    { group: 'tradepost', label: 'Pinnacle Station Tradepost', x: 1111.4, y: 2222.6, z: 33, distance: 448800 },
    { group: 'tradepost', label: 'Griffin\'s Reach Tradepost', x: 9000, y: 8000, z: 70, distance: 5200000 },
    { group: 'base', label: 'Spice Camp #602', x: 21, y: 31, z: 41, distance: 3000 },
    { group: 'vehicle', label: 'Red Runner #802', x: 300, y: 400, z: 50, distance: 9000 },
    { group: 'respawn', label: 'Respawn point #803', x: 1000, y: 1000, z: 10, distance: 120000 },
    { group: 'taxi', label: 'Arrakeen Taxi Service', x: 11695, y: -258, z: 5, distance: 1000 }] });

const deepDesert = () => ({ map: 'DeepDesert', sameMap: false, total: 1, groups: [TAXI],
  destinations: [{ group: 'taxi', label: 'Deep Desert Taxi Service', x: -300000, y: -590000, z: 1000, distance: 0 }] });

// The Movement card with a character saved at 100, 200, 300. answers holds the fake answers (a test may swap them); asked holds the address of every request for places. The drop-down is
// filled by the page itself: nothing is pressed.
async function open(first = dest(), extra = {}) {
  const get = { '/api/player/destinations': first };
  const ui = await boot({ get, post: { '/api/player/teleport': { ok: true }, '/api/save/discard': { ok: true } }, ...extra });
  const save = await (await ui.w.fetch('/api/player')).json();
  save.actor = { ...(save.actor || {}), x: 100, y: 200, z: 300, map: 'HaggaBasin' };
  get['/api/player'] = save;
  const asked = [];
  const realFetch = ui.w.fetch;
  ui.w.fetch = (u, ...rest) => { if (String(u).startsWith('/api/player/destinations')) asked.push(String(u)); return realFetch(u, ...rest); };
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="player:admin"]'); await ui.settle();
  const moveSave = (x, y, z, map) => { save.actor = { ...save.actor, x, y, z, ...(map ? { map } : {}) }; };
  const pick = async (id, v) => { const el = ui.doc.querySelector('#' + id); el.value = v; el.dispatchEvent(new ui.w.Event('change', { bubbles: true })); await ui.settle(); };
  return { ui, get, answers: get, asked, moveSave, pick };
}
const fields = (ui) => ['tx', 'ty', 'tz'].map((id) => ui.doc.querySelector('#' + id).value);
const choose = (ui, v) => { const s = ui.doc.querySelector('#tdsel'); s.value = v; s.dispatchEvent(new ui.w.Event('change', { bubbles: true })); };
const type = (ui, id, v) => { const el = ui.doc.querySelector('#' + id); el.value = v; el.dispatchEvent(new ui.w.Event('input', { bubbles: true })); };
const note = (ui) => ui.doc.querySelector('#tdpick').textContent;
const headings = (ui) => [...ui.doc.querySelectorAll('#tdsel optgroup')].map((g) => ({ label: g.label, places: [...g.querySelectorAll('option')].map((o) => o.textContent) }));

// ---- one drop-down, filled at once, no kind, no filter, no Find

test('Go to a place is one drop-down that is filled at once, under its headings, with no kind of place, no filter box and no Find', async () => {
  const { ui, asked } = await open(named());
  assert.equal(asked.length, 1, 'the page asked for the places once, by itself');
  assert.equal(asked[0], '/api/player/destinations?map=', 'the places of the character\'s own map, with no kind and no filter');
  assert.deepEqual(headings(ui), [
    { label: 'Trading posts', places: ['Pinnacle Station Tradepost - 4,488 m away', 'Griffin\'s Reach Tradepost - 52,000 m away'] },
    { label: 'Personal bases', places: ['Spice Camp #602 - 30 m away'] },
    { label: 'Vehicles', places: ['Red Runner #802 - 90 m away'] },
    { label: 'Respawn points', places: ['Respawn point #803 - 1,200 m away'] },
    { label: 'Taxis', places: ['Arrakeen Taxi Service - 10 m away'] }]);
  const sel = ui.doc.querySelector('#tdsel');
  assert.equal(sel.getAttribute('aria-label'), 'Place to go to', 'the select is labelled');
  assert.equal(sel.options[0].textContent, '6 places on HaggaBasin, nearest first', 'the first option counts the places');
  assert.equal(sel.value, '', 'nothing is chosen until the person chooses');
  assert.equal(ui.doc.querySelector('#tdkind'), null, 'no kind of place');
  assert.equal(ui.doc.querySelector('#tdq'), null, 'no filter box');
  assert.equal(ui.doc.querySelector('[data-act="tdFind"]'), null, 'no Find button');
  assert.equal([...ui.doc.querySelectorAll('#main button')].some((b) => b.textContent.trim() === 'Find'), false);
  assert.equal(ui.doc.querySelectorAll('#main select[aria-label="Kind of place"]').length, 0);
  assert.equal(ui.doc.querySelectorAll('select[aria-label="Place to go to"]').length, 1, 'exactly one drop-down of places');
  ui.dom.window.close();
});

test('every group lists at once: bases, vehicles and respawn points are in the one drop-down with the trading posts and the taxis', async () => {
  const { ui } = await open(named());
  const all = headings(ui);
  assert.deepEqual(all.map((g) => g.label), ['Trading posts', 'Personal bases', 'Vehicles', 'Respawn points', 'Taxis'], 'the headings, in this order');
  assert.equal(ui.doc.querySelectorAll('#tdsel option').length, 7, 'six places and the first option');
  ui.dom.window.close();
});

test('a heading the answer does not name is not drawn', async () => {
  const two = named();
  two.groups = [TP, BASE];
  two.destinations = two.destinations.slice(0, 3);
  two.total = 3;
  const { ui } = await open(two);
  assert.deepEqual(headings(ui).map((g) => g.label), ['Trading posts', 'Personal bases']);
  const some = named();
  some.groups = [VEH, TAXI];
  some.destinations = [some.destinations[3], some.destinations[5]];
  some.total = 2;
  const other = await open(some);
  assert.deepEqual(headings(other.ui).map((g) => g.label), ['Vehicles', 'Taxis']);
  ui.dom.window.close(); other.ui.dom.window.close();
});

test('a map with no place says so and draws no drop-down', async () => {
  const { ui } = await open({ map: 'HaggaBasin', sameMap: true, total: 0, groups: [], destinations: [] });
  assert.equal(ui.doc.querySelector('#tdsel'), null);
  assert.equal(ui.doc.querySelector('#tdout').textContent, 'No places on HaggaBasin.');
  ui.dom.window.close();
});

test('a failed request for the places says nothing on a redraw, says why when another map is chosen, and leaves no drop-down', async () => {
  const { ui, answers, pick } = await open({ __fail: 'the save could not be read' });
  assert.equal(ui.doc.querySelector('#tdsel'), null);
  assert.equal(ui.doc.body.textContent.includes('the save could not be read'), false, 'the card was drawn and the person asked for nothing: no message');
  answers['/api/player/destinations'] = { __fail: 'the save could not be read' };
  await pick('tmap', 'DeepDesert');
  assert.equal(ui.doc.querySelector('#tdsel'), null);
  assert.ok(ui.doc.body.textContent.includes('the save could not be read'), 'the person asked for another map: the reason is shown');
  ui.dom.window.close();
});

// ---- choosing a place (#506)

test('choosing the nearest and then the farthest place changes the fields each time, and the page says which place is selected', async () => {
  const { ui } = await open();
  assert.deepEqual(fields(ui), ['100', '200', '300'], 'the boxes start at the saved position');
  assert.equal(note(ui), '');
  choose(ui, '0');
  assert.deepEqual(fields(ui), ['1111', '2223', '33']);
  assert.equal(note(ui), 'Selected: NearPost, 4,488 m away');
  choose(ui, '1');
  assert.deepEqual(fields(ui), ['9000', '8000', '70']);
  assert.equal(note(ui), 'Selected: FarPost, 52,000 m away');
  choose(ui, '');
  assert.deepEqual(fields(ui), ['9000', '8000', '70'], 'the first option of the list changes nothing');
  assert.equal(note(ui), '');
  ui.dom.window.close();
});

test('choosing a place under any heading fills X, Y and Z and says which place is selected; the choice survives a redraw', async () => {
  const { ui, asked } = await open(named());
  choose(ui, '3'); // the vehicle: the value of an option is its place in the answer, not in its group
  assert.deepEqual(fields(ui), ['300', '400', '50']);
  assert.equal(note(ui), 'Selected: Red Runner #802, 90 m away');
  choose(ui, '4'); // the respawn point
  assert.deepEqual(fields(ui), ['1000', '1000', '10']);
  assert.equal(note(ui), 'Selected: Respawn point #803, 1,200 m away');
  choose(ui, '2'); // the base
  assert.deepEqual(fields(ui), ['21', '31', '41']);
  const before = asked.length;
  await ui.w.render(true); await ui.settle();
  assert.equal(ui.doc.querySelector('#tdsel').value, '2', 'the choice survives a redraw');
  assert.equal(ui.doc.querySelector('#tdsel').selectedOptions[0].parentElement.label, 'Personal bases');
  assert.deepEqual(fields(ui), ['21', '31', '41']);
  assert.equal(note(ui), 'Selected: Spice Camp #602, 30 m away');
  assert.equal(asked.length, before + 1, 'the redraw asked for the places again');
  ui.click('[data-act="teleport"]'); await ui.settle();
  assert.deepEqual(ui.posted.filter((p) => p.path === '/api/player/teleport').map((p) => p.body), [{ x: 21, y: 31, z: 41 }]);
  ui.dom.window.close();
});

test('a chosen place survives the redraws the page makes, though the saved position has moved meanwhile', async () => {
  const { ui, get, moveSave } = await open();
  choose(ui, '1');
  moveSave(150, 250, 310); // the game saved and the character stands elsewhere
  await ui.w.render(true); await ui.settle();
  assert.deepEqual(fields(ui), ['9000', '8000', '70'], 'a redraw after an action');
  assert.equal(ui.doc.querySelector('#tdsel').value, '1', 'the list keeps the chosen entry');
  assert.equal(note(ui), 'Selected: FarPost, 52,000 m away');
  moveSave(160, 260, 320);
  await ui.w.render('scroll'); await ui.settle();
  assert.deepEqual(fields(ui), ['9000', '8000', '70'], 'a redraw after a refused change');
  // the page's own reload of the file after the game saved
  const state = await (await ui.w.fetch('/api/save/state')).json();
  get['/api/save/state'] = { ...state, changedOnDisk: true, pending: [], dirty: false };
  ui.doc.activeElement.blur();
  moveSave(170, 270, 330);
  const before = ui.doc.querySelector('#tx');
  await ui.w.status(); await ui.settle();
  assert.notEqual(ui.doc.querySelector('#tx'), before, 'the page was drawn again from the file');
  assert.deepEqual(fields(ui), ['9000', '8000', '70'], 'the automatic reload of the file');
  assert.equal(ui.doc.querySelector('#tdsel').value, '1');
  ui.dom.window.close();
});

test('typed X, Y, Z, Facing and Map boxes survive a redraw; typing a coordinate over a chosen place drops its name', async () => {
  const { ui, moveSave } = await open();
  choose(ui, '0');
  assert.equal(note(ui), 'Selected: NearPost, 4,488 m away');
  type(ui, 'tx', '555');
  assert.equal(note(ui), '', 'the place is no longer what Teleport will do');
  assert.equal(ui.doc.querySelector('#tdsel').value, '', 'nor is it shown chosen in the list');
  type(ui, 'tdyaw', '90');
  ui.doc.querySelector('#tmap').value = 'DeepDesert'; ui.doc.querySelector('#tmap').dispatchEvent(new ui.w.Event('change', { bubbles: true }));
  moveSave(1, 2, 3);
  await ui.w.render(true); await ui.settle();
  assert.deepEqual(fields(ui), ['555', '2223', '33'], 'what was typed and what was chosen before it');
  assert.equal(ui.doc.querySelector('#tdyaw').value, '90');
  assert.equal(ui.doc.querySelector('#tmap').value, 'DeepDesert');
  ui.click('[data-act="teleport"]'); await ui.settle();
  assert.deepEqual(ui.posted.filter((p) => p.path === '/api/player/teleport').map((p) => p.body), [{ x: 555, y: 2223, z: 33, yaw: 90, map: 'DeepDesert' }]);
  ui.dom.window.close();
});

test('after a successful Teleport the boxes follow the saved position again, and a refused one keeps them', async () => {
  const { ui } = await open(dest(), { post: { '/api/player/teleport': { __fail: 'no' } } });
  choose(ui, '1');
  ui.click('[data-act="teleport"]'); await ui.settle();
  assert.deepEqual(fields(ui), ['9000', '8000', '70'], 'a refused Teleport leaves the choice for another try');
  ui.dom.window.close();
  const ok = await open();
  choose(ok.ui, '1');
  ok.moveSave(9000, 8000, 70); // the save now holds the place the character was sent to
  ok.ui.click('[data-act="teleport"]'); await ok.ui.settle();
  assert.deepEqual(ok.ui.posted.filter((p) => p.path === '/api/player/teleport').map((p) => p.body), [{ x: 9000, y: 8000, z: 70 }]);
  assert.deepEqual(fields(ok.ui), ['9000', '8000', '70']);
  assert.equal(note(ok.ui), '');
  assert.equal(ok.ui.doc.querySelector('#tdsel').value, '', 'the drop-down is filled again from the new position and nothing is chosen in it');
  ok.moveSave(40, 50, 60);
  await ok.ui.w.render(true); await ok.ui.settle();
  assert.deepEqual(fields(ok.ui), ['40', '50', '60'], 'the boxes follow the saved position again');
  ok.ui.dom.window.close();
});

test('when the character is on another map in the save, boxes meant for this map go back to the saved position', async () => {
  const { ui, moveSave } = await open();
  choose(ui, '1');
  moveSave(1, 2, 3, 'DeepDesert');
  await ui.w.render(true); await ui.settle();
  assert.deepEqual(fields(ui), ['1', '2', '3']);
  assert.equal(ui.doc.querySelector('#tdsel').value, '', 'the chosen place was for the other map');
  assert.equal(note(ui), '');
  ui.dom.window.close();
});

// ---- the refresh rules

test('the drop-down is filled again when the page is drawn, and the chosen place stays chosen by its name', async () => {
  const { ui, answers } = await open(named());
  choose(ui, '1'); // Griffin's Reach Tradepost
  assert.equal(note(ui), 'Selected: Griffin\'s Reach Tradepost, 52,000 m away');
  const moved = named(); // the character moved: the order and the distances differ
  moved.destinations = [moved.destinations[1], moved.destinations[0], ...moved.destinations.slice(2)];
  moved.destinations[0].distance = 100000;
  moved.destinations[1].distance = 600000;
  answers['/api/player/destinations'] = moved;
  await ui.w.render(true); await ui.settle();
  assert.deepEqual(headings(ui)[0].places, ['Griffin\'s Reach Tradepost - 1,000 m away', 'Pinnacle Station Tradepost - 6,000 m away']);
  assert.equal(ui.doc.querySelector('#tdsel').value, '0', 'the same place, in its new position in the list');
  assert.equal(note(ui), 'Selected: Griffin\'s Reach Tradepost, 1,000 m away');
  const gone = named(); // the place is no longer in the answer
  gone.destinations = gone.destinations.filter((x) => x.label !== 'Griffin\'s Reach Tradepost');
  gone.total = 5;
  answers['/api/player/destinations'] = gone;
  await ui.w.render(true); await ui.settle();
  assert.equal(ui.doc.querySelector('#tdsel').value, '', 'a place that is gone is not shown as chosen');
  assert.equal(note(ui), '');
  assert.deepEqual(fields(ui), ['9000', '8000', '70'], 'the boxes keep what they held (#506)');
  ui.dom.window.close();
});

test('a place of one heading is not mistaken for a place of another with the same name', async () => {
  const same = named();
  same.destinations.push({ group: 'vehicle', label: 'Spice Camp #602', x: 7, y: 8, z: 9, distance: 10 });
  same.destinations.sort((a, b) => ['tradepost', 'base', 'vehicle', 'respawn', 'taxi'].indexOf(a.group) - ['tradepost', 'base', 'vehicle', 'respawn', 'taxi'].indexOf(b.group));
  same.total = 7;
  const { ui, answers } = await open(same);
  const moved = () => { answers['/api/player/destinations'] = { ...same, destinations: same.destinations.map((d) => ({ ...d, distance: d.distance + 1000 })) }; }; // the character moved, so the page does refresh
  const heading = (label) => [...ui.doc.querySelectorAll('#tdsel option')].find((o) => o.textContent.startsWith('Spice Camp #602') && o.parentElement.label === label);
  choose(ui, heading('Vehicles').value); // the second of the two, so that a match by name alone would pick the base
  assert.deepEqual(fields(ui), ['7', '8', '9']);
  moved();
  await ui.w.render(true); await ui.settle();
  assert.equal(ui.doc.querySelector('#tdsel').selectedOptions[0].parentElement.label, 'Vehicles', 'kept by name and heading');
  assert.equal(note(ui), 'Selected: Spice Camp #602, 10 m away', 'the answer was taken again, with the new distance');
  choose(ui, heading('Personal bases').value);
  answers['/api/player/destinations'] = { ...same, destinations: same.destinations.map((d) => ({ ...d, distance: d.distance + 2000 })) };
  await ui.w.render(true); await ui.settle();
  assert.equal(ui.doc.querySelector('#tdsel').selectedOptions[0].parentElement.label, 'Personal bases');
  ui.dom.window.close();
});

test('a refresh that finds nothing new does not draw the drop-down again, so one that is open does not close', async () => {
  const { ui, answers } = await open(named());
  const seen = [];
  new ui.w.MutationObserver((m) => seen.push(...m)).observe(ui.doc.querySelector('#tdout'), { childList: true, subtree: true });
  await ui.w.goPlaces(false); await ui.settle();
  assert.equal(seen.length, 0, 'the same answer: nothing is drawn');
  const moved = named();
  moved.destinations[0].distance = 700000;
  answers['/api/player/destinations'] = moved;
  await ui.w.goPlaces(false); await ui.settle();
  assert.ok(seen.length > 0, 'another answer is drawn');
  ui.dom.window.close();
});

test('choosing another map reloads the drop-down for that map, with the cross-map warning and no distances', async () => {
  const { ui, answers, asked, pick } = await open(named());
  answers['/api/player/destinations'] = deepDesert();
  await pick('tmap', 'DeepDesert');
  assert.equal(asked.at(-1), '/api/player/destinations?map=DeepDesert');
  assert.deepEqual(headings(ui), [{ label: 'Taxis', places: ['Deep Desert Taxi Service'] }], 'no distance across maps');
  assert.equal(ui.doc.querySelector('#tdsel').options[0].textContent, '1 place on DeepDesert');
  assert.ok(ui.doc.querySelector('#tmapwarn'), 'the warning about a move to another map stays');
  await pick('tdsel', '0');
  ui.click('[data-act="teleport"]'); await ui.settle();
  assert.deepEqual(ui.posted.filter((p) => p.path === '/api/player/teleport').map((p) => p.body), [{ x: -300000, y: -590000, z: 1000, map: 'DeepDesert' }]);
  ui.dom.window.close();
});

test('an older answer that comes late does not replace the newer one', async () => {
  const { ui, answers, pick } = await open(named());
  let release;
  const gate = new Promise((r) => { release = r; });
  const realFetch = ui.w.fetch;
  ui.w.fetch = (u, ...rest) => (String(u).endsWith('destinations?map=') ? gate.then(() => realFetch(u, ...rest)) : realFetch(u, ...rest));
  const first = ui.w.goPlaces(false); // asks for this map's places, and waits
  await new Promise((r) => ui.w.setTimeout(r, 20));
  answers['/api/player/destinations'] = deepDesert();
  await pick('tmap', 'DeepDesert'); // asks again, and is answered at once
  answers['/api/player/destinations'] = named();
  release(); await first; await ui.settle();
  assert.deepEqual(headings(ui), [{ label: 'Taxis', places: ['Deep Desert Taxi Service'] }], 'the Hagga Basin answer came last and is dropped');
  ui.dom.window.close();
});

test('a heading or a name that holds markup is shown as text', async () => {
  const hostile = '"><img src=x onerror=window.__pwned=1>';
  const d = named();
  d.groups[0] = { ...d.groups[0], label: hostile };
  d.destinations[0].label = hostile + 'Post';
  const { ui } = await open(d);
  assert.equal(ui.doc.querySelector('#tdsel img'), null);
  assert.equal(ui.doc.querySelector('#tdsel optgroup').label, hostile);
  assert.equal(ui.w.__pwned, undefined);
  ui.dom.window.close();
});

// ---- the arrival points (#515): a trading post or hub is listed at the game's arrival point, and the boxes get that point

const arrival = () => ({ map: 'HaggaBasin', sameMap: true, total: 2, groups: [TP, BASE],
  destinations: [
    // the answer already holds the arrival point for the post (x, y, z) and the distance to it
    { group: 'tradepost', label: 'Pinnacle Station Tradepost', x: -32905.3, y: -313888.6, z: 12053, distance: 315818, arrival: true },
    { group: 'base', label: 'Spice Camp #602', x: 21, y: 31, z: 41, distance: 3000 }] });

test('choosing a named trading post fills the boxes with its arrival point and shows the distance to that point', async () => {
  const { ui } = await open(arrival());
  choose(ui, '0');
  assert.deepEqual(fields(ui), ['-32905', '-313889', '12053'], 'the boxes hold the arrival point, rounded as every place is');
  assert.equal(note(ui), 'Selected: Pinnacle Station Tradepost, 3,158 m away', 'the distance is to the point that will be used');
  await ui.w.render(true); await ui.settle();
  assert.deepEqual(fields(ui), ['-32905', '-313889', '12053'], 'the choice stays through a redraw (#506)');
  ui.click('[data-act="teleport"]'); await ui.settle();
  assert.deepEqual(ui.posted.filter((p) => p.path === '/api/player/teleport').map((p) => p.body), [{ x: -32905, y: -313889, z: 12053 }]);
  ui.dom.window.close();
});

test('a place without an arrival point still fills the boxes with its own position', async () => {
  const { ui } = await open(arrival());
  choose(ui, '1');
  assert.deepEqual(fields(ui), ['21', '31', '41']);
  ui.dom.window.close();
});

test('the Movement card\'s About text says which position is used, once, and not in every row, and nothing in it speaks of a filter or Find', async () => {
  const { ui } = await open(arrival());
  const about = ui.doc.querySelector('details[data-about="admin.movement"]').textContent;
  assert.match(about, /trading posts and hubs the boxes are filled with the point where the game's own ornithopter trip would put you; for other places, with the place's own position/);
  assert.match(about, /Griffin's Reach and Crossroads\) are computed from the game's level data and have not been seen in use/);
  assert.match(about, /your own bases that stand in the world, your own vehicles that stand in the world, your respawn points/);
  assert.doesNotMatch(about, /filter|search|Find|Named places/i);
  assert.equal([...ui.doc.querySelectorAll('#tdsel option')].some((o) => /arrival|marker position/i.test(o.textContent)), false, 'no row repeats it');
  ui.dom.window.close();
});
