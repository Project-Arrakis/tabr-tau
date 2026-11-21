// The house symbols on the Landsraad board (#645): every house the save holds shows its symbol picture on its card of the Command Center board (the Command Center art folder), a house with none keeps
// the two letters, the name stays beside the symbol, and a strange house name can only miss the list (it never becomes part of a file path).
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import { readdirSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { boot, closeAll } from './boot.mjs';

afterEach(closeAll);

// the 25 houses of the board, as the save writes them (landsraad_tasks.house_name without the DA_House prefix)
const HOUSES = ['Alexin', 'Argosaz', 'Dyvetz', 'Ecaz', 'Hagal', 'Hurata', 'Imota', 'Kenola', 'Lindaren', 'Maros', 'Mikkarol', 'Moritani', 'Mutelli', 'Novebruns', 'Richese', 'Sor', 'Spinette', 'Taligari', 'Thorvald', 'Tseida', 'Varota', 'Vernius', 'Wallach', 'Wayku', 'Wydras'];
const FILES = readdirSync(fileURLToPath(new URL('../internal/web/static/houses/', import.meta.url)));
const ART = readdirSync(fileURLToPath(new URL('../internal/web/static/lcc/art/houses/', import.meta.url)));

const TH = [1200, 2400, 3600, 4800, 6000];
const task = (id, name) => ({
  id, board_index: id - 900, house_name: 'DA_House' + name, goal_amount: 4000, completed: 0, winning_faction_id: null, completion_time: null,
  contributions: [], player_contributions: [], my_contribution: 0, my_recorded: false, swatch_held: false, location: { known: false },
  rewards: TH.map((threshold) => ({ threshold, amount: 1, template_id: 'SolarisCoin', reached: false, delivered: false, shown_template_id: 'SolarisCoin', shown_amount: 1, swatch: false, reward_state: 'not_reached' })),
});
const board = (names) => ({
  term: { term_id: 7, start_time: 100, end_time: 200, reigning_faction_id: 1, active_decree_id: 1, elected_decree_id: 1 },
  terms: [{ term_id: 7, start_time: 100, end_time: 200 }], earlier_terms: [],
  factions: [{ id: 1, name: 'Atreides' }, { id: 2, name: 'Harkonnen' }],
  character: { player_id: 1, name: 'Tester', faction_id: 1 },
  decrees: [{ id: 1, decree_name: 'Decree A', disabled: 0 }], rotation: [],
  tasks: names.map((n, i) => task(900 + i, n)),
});
async function open(names) {
  const ui = await boot({ get: { '/api/landsraad': board(names) } });
  ui.click('[data-tab="landsraad"]'); await ui.settle();
  return ui;
}
const cells = (ui) => [...ui.doc.querySelectorAll('#lsb-lcc .lcc-bd-cell')]; // one card of the grid for each house
const crestOf = (c) => c.querySelector('.lcc-hc-select .lcc-hc-crest');

test('there is one symbol file for each of the 25 houses, and nothing else is in the folder', () => {
  assert.deepEqual([...FILES].sort(), HOUSES.map((n) => n.toLowerCase() + '.webp').sort());
});

test('the Command Center art folder has the same 25 symbols', () => {
  assert.deepEqual([...ART].sort(), [...FILES].sort());
});

test('every house of the board shows its own symbol picture on its card with the name beside it', async () => {
  const ui = await open(HOUSES);
  const rows = cells(ui);
  assert.equal(rows.length, 25);
  const seen = new Set();
  for (const r of rows) {
    const name = r.querySelector('.lcc-hc-name').textContent.replace(/^House /, '');
    const img = crestOf(r).querySelector('img');
    assert.ok(img, name + ' has no picture');
    const file = img.getAttribute('src');
    assert.match(file, /^\/lcc\/art\/houses\/[a-z]+\.webp$/, name);
    assert.ok(ART.includes(file.slice('/lcc/art/houses/'.length)), name + ' points at a file that is not embedded: ' + file);
    assert.equal(file, '/lcc/art/houses/' + name.toLowerCase() + '.webp');
    seen.add(file);
    assert.equal(img.getAttribute('alt'), '', 'the picture is decoration, the name is the text');
    // the symbol and the name are the one button, which looks at the house
    const btn = r.querySelector('button.lcc-hc-select');
    assert.equal(btn.dataset.fk, 'ht:' + r.dataset.id);
    assert.ok(btn.contains(crestOf(r)) && btn.contains(r.querySelector('.lcc-hc-name')));
    // the hover card of the card carries the large symbol (#1069): the same file
    const big = r.querySelector('.hcard img.hsym');
    assert.ok(big, name + ' has no card');
    assert.equal(big.getAttribute('src'), file);
  }
  assert.equal(seen.size, 25, 'every house has a picture of its own');
});

// A board that is not complete (here 2 of 25 Houses) is shown as the read-only list of the Houses by name: the same hover card, the same symbol file.
test("the game spells one house Mikarrol: it finds the same picture as the save's Mikkarol", async () => {
  const ui = await open(['Mikarrol', 'Mikkarol']);
  const rows = [...ui.doc.querySelectorAll('#lsb-lcc tr.lcc-bd-row')];
  assert.equal(rows.length, 2);
  assert.deepEqual(rows.map((r) => r.querySelector('.hcard img.hsym').getAttribute('src')), ['/lcc/art/houses/mikkarol.webp', '/lcc/art/houses/mikkarol.webp']);
});

test('a house with no picture keeps the two letters, and a strange name never becomes a path', async () => {
  // on the card of a full board: the two letters in the place of the picture
  const names = HOUSES.slice(); names[names.indexOf('Sor')] = 'Nonesuch';
  const full = await open(names);
  const card = cells(full).find((c) => /Nonesuch/.test(c.querySelector('.lcc-hc-name').textContent));
  assert.ok(card, 'the House is drawn');
  assert.equal(crestOf(card).querySelector('img'), null, 'no picture');
  assert.equal(crestOf(card).textContent, 'NO');
  assert.equal(card.querySelector('.hcard img.hsym'), null, 'and none in its hover card');
  closeAll();
  // strange names in the read-only list: they can only miss the list of symbols
  const odd = ['Nonesuch', '../../app.js', '__proto__', 'constructor', 'toString', '<img src=x onerror=alert(1)>', 'Alexin/../Sor', 'ALEXIN', ''];
  const ui = await open(odd);
  const rows = [...ui.doc.querySelectorAll('#lsb-lcc tr.lcc-bd-row')];
  assert.equal(rows.length, odd.length);
  let withPicture = 0;
  for (const r of rows) {
    const name = r.querySelector('.lcc-bd-namebtn').textContent, img = r.querySelector('.hcard img.hsym');
    if (name.toLowerCase() === 'alexin') { withPicture++; assert.equal(img.getAttribute('src'), '/lcc/art/houses/alexin.webp'); continue; } // the lookup ignores case
    assert.equal(img, null, JSON.stringify(name) + ' must not get a picture');
  }
  assert.equal(withPicture, 1);
  assert.equal(ui.doc.querySelectorAll('#lsb-lcc img[onerror], #lsb-lcc .lcc-bd-namebtn img').length, 0);
  // every picture of the board is a file of the Command Center art folder, found in the same fixed list
  const bad = [...ui.doc.querySelectorAll('#lsb-lcc img')].map((i) => i.getAttribute('src')).filter((s) => !/^\/lcc\/art\/(houses|emblems|banners|lanes|items|decrees|factions|standards)\/[a-z0-9_-]+\.(webp|png|svg)$/i.test(s));
  assert.deepEqual(bad, []);
});
