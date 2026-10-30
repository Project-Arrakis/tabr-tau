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
