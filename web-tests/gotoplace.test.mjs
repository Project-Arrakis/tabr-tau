// Go to a place keeps its choice (#506): a place chosen in the list fills Teleport X, Y and Z, and what was chosen or typed in the Movement card stays there while the page is
// drawn again (the game saves and the character's saved position moves; an action elsewhere on the page; a refused change), until a Teleport succeeds.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import { boot, closeAll } from './boot.mjs';

afterEach(closeAll);

const dest = () => ({ map: 'HaggaBasin', sameMap: true, kind: 'marker', total: 2, destinations: [
  { kind: 'marker', label: 'NearPost', x: 1111.4, y: 2222.6, z: 33, distance: 448800 },
  { kind: 'marker', label: 'FarPost', x: 9000, y: 8000, z: 70, distance: 5200000 }] });

// The Movement card with a character saved at 100, 200, 300 and a Find that has answered. save is the fake answer for the character: a test moves its position the way the game's next save does.
async function open(extra = {}) {
  const get = { '/api/player/destinations': dest() };
  const ui = await boot({ get, post: { '/api/player/teleport': { ok: true }, '/api/save/discard': { ok: true } }, ...extra });
  const save = await (await ui.w.fetch('/api/player')).json();
  save.actor = { ...(save.actor || {}), x: 100, y: 200, z: 300, map: 'HaggaBasin' };
  get['/api/player'] = save;
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="player:admin"]'); await ui.settle();
  ui.click('[data-act="tdFind"]'); await ui.settle();
  const moveSave = (x, y, z, map) => { save.actor = { ...save.actor, x, y, z, ...(map ? { map } : {}) }; };
  return { ui, get, moveSave };
}
const fields = (ui) => ['tx', 'ty', 'tz'].map((id) => ui.doc.querySelector('#' + id).value);
const choose = (ui, v) => { const s = ui.doc.querySelector('#tdsel'); s.value = v; s.dispatchEvent(new ui.w.Event('change', { bubbles: true })); };
const type = (ui, id, v) => { const el = ui.doc.querySelector('#' + id); el.value = v; el.dispatchEvent(new ui.w.Event('input', { bubbles: true })); };
const note = (ui) => ui.doc.querySelector('#tdpick').textContent;

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
  assert.deepEqual(fields(ui), ['9000', '8000', '70'], 'the heading of the list changes nothing');
  assert.equal(note(ui), '');
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

test('typed X, Y, Z, Facing, Map and Find boxes survive a redraw; typing a coordinate over a chosen place drops its name', async () => {
  const { ui, moveSave } = await open();
  choose(ui, '0');
  assert.equal(note(ui), 'Selected: NearPost, 4,488 m away');
  type(ui, 'tx', '555');
  assert.equal(note(ui), '', 'the place is no longer what Teleport will do');
  assert.equal(ui.doc.querySelector('#tdsel').value, '', 'nor is it shown chosen in the list');
  type(ui, 'tdyaw', '90'); type(ui, 'tdq', 'post');
  ui.doc.querySelector('#tmap').value = 'DeepDesert'; ui.doc.querySelector('#tmap').dispatchEvent(new ui.w.Event('change', { bubbles: true }));
  moveSave(1, 2, 3);
  await ui.w.render(true); await ui.settle();
  assert.deepEqual(fields(ui), ['555', '2223', '33'], 'what was typed and what was chosen before it');
  assert.equal(ui.doc.querySelector('#tdyaw').value, '90');
  assert.equal(ui.doc.querySelector('#tdq').value, 'post');
  assert.equal(ui.doc.querySelector('#tmap').value, 'DeepDesert');
  ui.click('[data-act="teleport"]'); await ui.settle();
  assert.deepEqual(ui.posted.filter((p) => p.path === '/api/player/teleport').map((p) => p.body), [{ x: 555, y: 2223, z: 33, yaw: 90, map: 'DeepDesert' }]);
  ui.dom.window.close();
});

test('after a successful Teleport the boxes follow the saved position again, and a refused one keeps them', async () => {
  const { ui, get, moveSave } = await open({ post: { '/api/player/teleport': { __fail: 'no' } } });
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
  assert.equal(ok.ui.doc.querySelector('#tdsel'), null, 'the list of distances from the old position is gone');
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
  assert.equal(ui.doc.querySelector('#tdsel'), null);
  ui.dom.window.close();
});

// ---- the Named places list (#500): the places are under four headings, each place by the name a player sees

const named = () => ({ map: 'HaggaBasin', sameMap: true, kind: 'marker', total: 5,
  groups: [{ key: 'tradepost', label: 'Trading posts' }, { key: 'base', label: 'Personal bases' }, { key: 'vehicle', label: 'Vehicles' }, { key: 'taxi', label: 'Taxis' }],
  destinations: [
    { kind: 'marker', group: 'tradepost', label: 'Pinnacle Station Tradepost', x: 1111.4, y: 2222.6, z: 33, distance: 448800 },
    { kind: 'marker', group: 'tradepost', label: 'Griffin\'s Reach Tradepost', x: 9000, y: 8000, z: 70, distance: 5200000 },
    { kind: 'marker', group: 'base', label: 'Spice Camp #602', x: 21, y: 31, z: 41, distance: 3000 },
    { kind: 'marker', group: 'vehicle', label: 'Red Runner #802', x: 300, y: 400, z: 50, distance: 9000 },
    { kind: 'marker', group: 'taxi', label: 'Arrakeen Taxi Service', x: 11695, y: -258, z: 5, distance: 1000 }] });

const headings = (ui) => [...ui.doc.querySelectorAll('#tdsel optgroup')].map((g) => ({ label: g.label, places: [...g.querySelectorAll('option')].map((o) => o.textContent) }));

test('the Named places list is drawn under its headings, in the order the answer gives, with the name and distance of each place', async () => {
  const { ui } = await open({ get: { '/api/player/destinations': named() } });
  assert.deepEqual(headings(ui), [
    { label: 'Trading posts', places: ['Pinnacle Station Tradepost - 4,488 m away', 'Griffin\'s Reach Tradepost - 52,000 m away'] },
    { label: 'Personal bases', places: ['Spice Camp #602 - 30 m away'] },
    { label: 'Vehicles', places: ['Red Runner #802 - 90 m away'] },
    { label: 'Taxis', places: ['Arrakeen Taxi Service - 10 m away'] }]);
  assert.equal(ui.doc.querySelector('#tdsel').options[0].textContent, '5 places on HaggaBasin, nearest first', 'the heading option of the list counts the places');
  assert.equal([...ui.doc.querySelector('#tdkind').options].map((o) => o.textContent).includes('Named places'), true, 'the kind of place is called Named places');
  ui.dom.window.close();
});

test('choosing a place under a heading fills X, Y and Z and says which place is selected, whichever group it is in', async () => {
  const { ui } = await open({ get: { '/api/player/destinations': named() } });
  choose(ui, '3'); // the vehicle: the value of an option is its place in the answer, not in its group
  assert.deepEqual(fields(ui), ['300', '400', '50']);
  assert.equal(note(ui), 'Selected: Red Runner #802, 90 m away');
  choose(ui, '0');
  assert.deepEqual(fields(ui), ['1111', '2223', '33']);
  assert.equal(note(ui), 'Selected: Pinnacle Station Tradepost, 4,488 m away');
  await ui.w.render(true); await ui.settle();
  assert.equal(ui.doc.querySelector('#tdsel').value, '0', 'the choice survives a redraw');
  assert.equal(ui.doc.querySelector('#tdsel').selectedOptions[0].parentElement.label, 'Trading posts');
  ui.dom.window.close();
});

test('a heading the answer does not name is not drawn, and the other lists have none', async () => {
  const two = named();
  two.groups = two.groups.slice(0, 2);
  two.destinations = two.destinations.slice(0, 3);
  two.total = 3;
  const { ui } = await open({ get: { '/api/player/destinations': two } });
  assert.deepEqual(headings(ui).map((g) => g.label), ['Trading posts', 'Personal bases']);
  const flat = await open(); // the default answer of these tests is a list of one kind with no groups
  assert.equal(flat.ui.doc.querySelectorAll('#tdsel optgroup').length, 0);
  assert.equal(flat.ui.doc.querySelectorAll('#tdsel option').length, 3);
  ui.dom.window.close(); flat.ui.dom.window.close();
});

test('the filter box is sent with Find and the list shows what the filter left', async () => {
  const answers = { '/api/player/destinations': named() };
  const { ui } = await open({ get: answers });
  const asked = [];
  const realFetch = ui.w.fetch;
  ui.w.fetch = (u, ...rest) => { asked.push(String(u)); return realFetch(u, ...rest); };
  const one = named();
  one.groups = [one.groups[0]];
  one.destinations = [one.destinations[0]];
  one.total = 1;
  answers['/api/player/destinations'] = one;
  ui.doc.querySelector('#tdkind').value = 'marker';
  type(ui, 'tdq', 'pinn');
  ui.click('[data-act="tdFind"]'); await ui.settle();
  const url = asked.find((u) => u.startsWith('/api/player/destinations?'));
  assert.ok(url && url.includes('kind=marker') && url.includes('q=pinn'), url);
  assert.deepEqual(headings(ui), [{ label: 'Trading posts', places: ['Pinnacle Station Tradepost - 4,488 m away'] }]);
  ui.dom.window.close();
});

test('a heading or a name that holds markup is shown as text', async () => {
  const hostile = '"><img src=x onerror=window.__pwned=1>';
  const d = named();
  d.groups[0].label = hostile;
  d.destinations[0].label = hostile + 'Post';
  const { ui } = await open({ get: { '/api/player/destinations': d } });
  assert.equal(ui.doc.querySelector('#tdsel img'), null);
  assert.equal(ui.doc.querySelector('#tdsel optgroup').label, hostile);
  assert.equal(ui.w.__pwned, undefined);
  ui.dom.window.close();
});

// ---- the arrival points (#515): a trading post or hub is listed at the game's arrival point, and the boxes get that point

const arrival = () => ({ map: 'HaggaBasin', sameMap: true, kind: 'marker', total: 2,
  groups: [{ key: 'tradepost', label: 'Trading posts' }, { key: 'base', label: 'Personal bases' }],
  destinations: [
    // the answer already holds the arrival point for the post (x, y, z) and the distance to it
    { kind: 'marker', group: 'tradepost', label: 'Pinnacle Station Tradepost', x: -32905.3, y: -313888.6, z: 12053, distance: 315818, arrival: true },
    { kind: 'marker', group: 'base', label: 'Spice Camp #602', x: 21, y: 31, z: 41, distance: 3000 }] });

test('choosing a named trading post fills the boxes with its arrival point and shows the distance to that point', async () => {
  const { ui } = await open({ get: { '/api/player/destinations': arrival() } });
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
  const { ui } = await open({ get: { '/api/player/destinations': arrival() } });
  choose(ui, '1');
  assert.deepEqual(fields(ui), ['21', '31', '41']);
  ui.dom.window.close();
});

test('the Movement card\'s About text says which position is used, once, and not in every row', async () => {
  const { ui } = await open({ get: { '/api/player/destinations': arrival() } });
  const about = ui.doc.querySelector('details[data-about="admin.movement"]').textContent;
  assert.match(about, /trading posts and hubs the boxes are filled with the point where the game's own ornithopter trip would put you; for other places, with the place's own position/);
  assert.match(about, /Griffin's Reach and Crossroads\) are computed from the game's level data and have not been seen in use/);
  assert.equal([...ui.doc.querySelectorAll('#tdsel option')].some((o) => /arrival|marker position/i.test(o.textContent)), false, 'no row repeats it');
  ui.dom.window.close();
});

// ---- Named places is a drop-down that is filled at once (#519): no filter box and no Find; the other kinds are as they were

// The Movement card, with Named places chosen the way a person chooses it (a change of the kind) and no Find pressed. answers holds the fake answer for the places, which a test may swap;
// asked holds the address of every request for places.
async function openPlaces(first = named(), extra = {}) {
  const answers = { '/api/player/destinations': first };
  const ui = await boot({ get: answers, post: { '/api/player/teleport': { ok: true }, '/api/save/discard': { ok: true } }, ...extra });
  const save = await (await ui.w.fetch('/api/player')).json();
  save.actor = { ...(save.actor || {}), x: 100, y: 200, z: 300, map: 'HaggaBasin' };
  answers['/api/player'] = save;
  const asked = [];
  const realFetch = ui.w.fetch;
  ui.w.fetch = (u, ...rest) => { if (String(u).startsWith('/api/player/destinations')) asked.push(String(u)); return realFetch(u, ...rest); };
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="player:admin"]'); await ui.settle();
  const pick = async (id, v) => { const el = ui.doc.querySelector('#' + id); el.value = v; el.dispatchEvent(new ui.w.Event('change', { bubbles: true })); await ui.settle(); };
  return { ui, answers, asked, pick };
}

test('choosing Named places lists the places at once, under their headings, with no Find pressed and no filter box', async () => {
  const { ui, asked, pick } = await openPlaces();
  assert.equal(ui.doc.querySelector('#tdsel'), null, 'nothing is listed before a kind is chosen');
  assert.deepEqual(asked, []);
  await pick('tdkind', 'marker');
  assert.equal(asked.length, 1);
  assert.match(asked[0], /^\/api\/player\/destinations\?kind=marker&q=&map=$/, 'the places of the character\'s own map, with no filter');
  assert.deepEqual(headings(ui), [
    { label: 'Trading posts', places: ['Pinnacle Station Tradepost - 4,488 m away', 'Griffin\'s Reach Tradepost - 52,000 m away'] },
    { label: 'Personal bases', places: ['Spice Camp #602 - 30 m away'] },
    { label: 'Vehicles', places: ['Red Runner #802 - 90 m away'] },
    { label: 'Taxis', places: ['Arrakeen Taxi Service - 10 m away'] }]);
  assert.equal(ui.doc.querySelector('#tdq'), null, 'no filter box');
  assert.equal(ui.doc.querySelector('[data-act="tdFind"]'), null, 'no Find button');
  const sel = ui.doc.querySelector('#tdsel');
  assert.equal(sel.getAttribute('aria-label'), 'Place to go to', 'the select is labelled');
  assert.equal(sel.options[0].textContent, '5 places on HaggaBasin, nearest first');
  ui.dom.window.close();
});

test('a place chosen in the Named places drop-down fills X, Y and Z (the arrival point where there is one) and stays through a redraw', async () => {
  const { ui, pick, asked } = await openPlaces(arrival());
  await pick('tdkind', 'marker');
  await pick('tdsel', '0');
  assert.deepEqual(fields(ui), ['-32905', '-313889', '12053']);
  assert.equal(note(ui), 'Selected: Pinnacle Station Tradepost, 3,158 m away');
  await pick('tdsel', '1');
  assert.deepEqual(fields(ui), ['21', '31', '41'], 'a place with no arrival point: its own position');
  assert.equal(note(ui), 'Selected: Spice Camp #602, 30 m away');
  const before = asked.length;
  await ui.w.render(true); await ui.settle();
  assert.deepEqual(fields(ui), ['21', '31', '41']);
  assert.equal(ui.doc.querySelector('#tdsel').value, '1');
  assert.equal(ui.doc.querySelector('#tdsel').selectedOptions[0].parentElement.label, 'Personal bases');
  assert.equal(ui.doc.querySelector('#tdkind').value, 'marker', 'Named places stays chosen through the redraw');
  assert.equal(ui.doc.querySelector('[data-act="tdFind"]'), null);
  assert.equal(asked.length, before + 1, 'the redraw asked for the places again');
  ui.click('[data-act="teleport"]'); await ui.settle();
  assert.deepEqual(ui.posted.filter((p) => p.path === '/api/player/teleport').map((p) => p.body), [{ x: 21, y: 31, z: 41 }]);
  ui.dom.window.close();
});

test('the Named places drop-down is filled again when the page is drawn, and the chosen place stays chosen by its name', async () => {
  const { ui, answers, pick } = await openPlaces();
  await pick('tdkind', 'marker');
  await pick('tdsel', '1'); // Griffin's Reach Tradepost
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
  gone.total = 4;
  answers['/api/player/destinations'] = gone;
  await ui.w.render(true); await ui.settle();
  assert.equal(ui.doc.querySelector('#tdsel').value, '', 'a place that is gone is not shown as chosen');
  assert.equal(note(ui), '');
  assert.deepEqual(fields(ui), ['9000', '8000', '70'], 'the boxes keep what they held (#506)');
  ui.dom.window.close();
});

test('a refresh that finds nothing new does not draw the drop-down again, so one that is open does not close', async () => {
  const { ui, answers, pick } = await openPlaces();
  await pick('tdkind', 'marker');
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

test('a heading with no place is not drawn in the Named places drop-down', async () => {
  const some = named();
  some.groups = [some.groups[0], some.groups[3]];
  some.destinations = [some.destinations[0], some.destinations[4]];
  some.total = 2;
  const { ui, pick } = await openPlaces(some);
  await pick('tdkind', 'marker');
  assert.deepEqual(headings(ui).map((g) => g.label), ['Trading posts', 'Taxis']);
  assert.equal(ui.doc.querySelectorAll('#tdsel optgroup').length, 2);
  const none = { map: 'HaggaBasin', sameMap: true, kind: 'marker', total: 0, groups: [], destinations: [] };
  const empty = await openPlaces(none);
  await empty.pick('tdkind', 'marker');
  assert.equal(empty.ui.doc.querySelector('#tdsel'), null);
  assert.equal(empty.ui.doc.querySelector('#tdout').textContent, 'None on HaggaBasin.');
  ui.dom.window.close(); empty.ui.dom.window.close();
});

test('choosing another map reloads the Named places drop-down for that map, with the cross-map warning and no distances', async () => {
  const { ui, answers, asked, pick } = await openPlaces();
  await pick('tdkind', 'marker');
  const dd = { map: 'DeepDesert', sameMap: false, kind: 'marker', total: 1, groups: [{ key: 'taxi', label: 'Taxis' }],
    destinations: [{ kind: 'marker', group: 'taxi', label: 'Deep Desert Taxi Service', x: -300000, y: -590000, z: 1000, distance: 0 }] };
  answers['/api/player/destinations'] = dd;
  await pick('tmap', 'DeepDesert');
  assert.match(asked.at(-1), /^\/api\/player\/destinations\?kind=marker&q=&map=DeepDesert$/);
  assert.deepEqual(headings(ui), [{ label: 'Taxis', places: ['Deep Desert Taxi Service'] }], 'no distance across maps');
  assert.equal(ui.doc.querySelector('#tdsel').options[0].textContent, '1 place on DeepDesert');
  assert.ok(ui.doc.querySelector('#tmapwarn'), 'the warning about a move to another map stays');
  await pick('tdsel', '0');
  ui.click('[data-act="teleport"]'); await ui.settle();
  assert.deepEqual(ui.posted.filter((p) => p.path === '/api/player/teleport').map((p) => p.body), [{ x: -300000, y: -590000, z: 1000, map: 'DeepDesert' }]);
  ui.dom.window.close();
});

test('a map change with another kind chosen asks for no places, and a Named places list does not stay under another kind', async () => {
  const { ui, asked, pick } = await openPlaces();
  await pick('tmap', 'DeepDesert');
  assert.deepEqual(asked, [], 'Base, Respawn point and My vehicle still wait for Find');
  await pick('tdkind', 'marker');
  assert.equal(asked.length, 1);
  await pick('tdkind', 'base');
  assert.equal(ui.doc.querySelector('#tdsel'), null);
  ui.dom.window.close();
});

test('Base, Respawn point and My vehicle keep the filter box and Find; Named places takes them away and another kind brings them back', async () => {
  const { ui, asked, pick } = await openPlaces(dest());
  for (const kind of ['base', 'respawn', 'vehicle']) {
    await pick('tdkind', kind);
    assert.ok(ui.doc.querySelector('#tdq'), `${kind}: the filter box`);
    assert.ok(ui.doc.querySelector('[data-act="tdFind"]'), `${kind}: Find`);
    assert.equal(ui.doc.querySelector('#tdsel'), null, `${kind}: nothing is listed until Find`);
  }
  assert.deepEqual(asked, []);
  type(ui, 'tdq', 'pinn');
  ui.click('[data-act="tdFind"]'); await ui.settle();
  assert.match(asked[0], /kind=vehicle&q=pinn/);
  assert.equal(ui.doc.querySelectorAll('#tdsel optgroup').length, 0, 'a flat list with no headings, as before');
  assert.equal(ui.doc.querySelectorAll('#tdsel option').length, 3);
  await pick('tdkind', 'marker');
  assert.equal(ui.doc.querySelector('#tdq'), null);
  assert.equal(ui.doc.querySelector('[data-act="tdFind"]'), null);
  await pick('tdkind', 'base');
  assert.equal(ui.doc.querySelector('#tdq').value, 'pinn', 'what was typed in the filter box was kept');
  assert.ok(ui.doc.querySelector('[data-act="tdFind"]'));
  ui.dom.window.close();
});

test('the Movement card\'s About text still says what is true, and nothing in it speaks of searching for Named places', async () => {
  const { ui, pick } = await openPlaces(arrival());
  await pick('tdkind', 'marker');
  const about = ui.doc.querySelector('details[data-about="admin.movement"]').textContent;
  assert.match(about, /Named places: for trading posts and hubs the boxes are filled with the point where the game's own ornithopter trip would put you/);
  assert.doesNotMatch(about, /filter|search|Find/i);
  ui.dom.window.close();
});

test('a failed request for the Named places says why and leaves no drop-down', async () => {
  const { ui, pick } = await openPlaces({ __fail: 'the save could not be read' });
  await pick('tdkind', 'marker');
  assert.equal(ui.doc.querySelector('#tdsel'), null);
  assert.ok(ui.doc.body.textContent.includes('the save could not be read'), 'the reason is shown');
  ui.dom.window.close();
});

test('an older answer for the Named places that comes late does not replace the newer one', async () => {
  const { ui, answers, pick } = await openPlaces();
  const slow = named();
  const dd = { map: 'DeepDesert', sameMap: false, kind: 'marker', total: 1, groups: [{ key: 'taxi', label: 'Taxis' }],
    destinations: [{ kind: 'marker', group: 'taxi', label: 'Deep Desert Taxi Service', x: -300000, y: -590000, z: 1000, distance: 0 }] };
  let release;
  const gate = new Promise((r) => { release = r; });
  const realFetch = ui.w.fetch;
  ui.w.fetch = (u, ...rest) => (String(u).includes('kind=marker&q=&map=&') || String(u).endsWith('kind=marker&q=&map=') ? gate.then(() => realFetch(u, ...rest)) : realFetch(u, ...rest));
  answers['/api/player/destinations'] = slow;
  const first = pick('tdkind', 'marker'); // asks for this map's places, and waits
  await new Promise((r) => ui.w.setTimeout(r, 20));
  answers['/api/player/destinations'] = dd;
  await pick('tmap', 'DeepDesert'); // asks again, and is answered at once
  answers['/api/player/destinations'] = slow;
  release(); await first; await ui.settle();
  assert.deepEqual(headings(ui), [{ label: 'Taxis', places: ['Deep Desert Taxi Service'] }], 'the Hagga Basin answer came last and is dropped');
  ui.dom.window.close();
});
