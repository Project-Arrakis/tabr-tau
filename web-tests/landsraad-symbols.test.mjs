// The house symbols on the Landsraad board (#645): every house the save holds shows its symbol picture, a house with none keeps the two letters,
// the name stays beside the symbol, and a strange house name can only miss the list (it never becomes part of a file path).
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import { readdirSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { boot, closeAll } from './boot.mjs';

afterEach(closeAll);

// the 25 houses of the board, as the save writes them (landsraad_tasks.house_name without the DA_House prefix)
const HOUSES = ['Alexin', 'Argosaz', 'Dyvetz', 'Ecaz', 'Hagal', 'Hurata', 'Imota', 'Kenola', 'Lindaren', 'Maros', 'Mikkarol', 'Moritani', 'Mutelli', 'Novebruns', 'Richese', 'Sor', 'Spinette', 'Taligari', 'Thorvald', 'Tseida', 'Varota', 'Vernius', 'Wallach', 'Wayku', 'Wydras'];
const FILES = readdirSync(fileURLToPath(new URL('../internal/web/static/houses/', import.meta.url)));

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
const crestOf = (r) => r.querySelector('.hn .crest');

test('there is one symbol file for each of the 25 houses, and nothing else is in the folder', () => {
  assert.deepEqual([...FILES].sort(), HOUSES.map((n) => n.toLowerCase() + '.webp').sort());
});

test('every house of the board shows its own symbol picture at 32 px with the name beside it', async () => {
  const ui = await open(HOUSES);
  const rows = [...ui.doc.querySelectorAll('#lsb-rows > li')];
  assert.equal(rows.length, 25);
  const seen = new Set();
  for (const r of rows) {
    const name = r.querySelector('.hn b').textContent;
    const img = crestOf(r).querySelector('img');
    assert.ok(img, name + ' has no picture');
    const file = img.getAttribute('src');
    assert.match(file, /^\/houses\/[a-z]+\.webp$/, name);
    assert.ok(FILES.includes(file.slice('/houses/'.length)), name + ' points at a file that is not embedded: ' + file);
    assert.equal(file, '/houses/' + name.toLowerCase() + '.webp');
    seen.add(file);
    assert.equal(img.getAttribute('alt'), '', 'the picture is decoration, the name is the text');
    assert.equal(crestOf(r).getAttribute('aria-hidden'), 'true');
    // the symbol and the name are the one button, which still opens the house card
    const btn = r.querySelector('button.hn');
    assert.ok(btn.contains(crestOf(r)) && btn.contains(r.querySelector('.hn b')));
    assert.ok(r.querySelector('.hcard'), name + ' has no card');
  }
  assert.equal(seen.size, 25, 'every house has a picture of its own');
});

test('the game spells one house Mikarrol: it finds the same picture as the save\'s Mikkarol', async () => {
  const ui = await open(['Mikarrol', 'Mikkarol']);
  const srcs = [...ui.doc.querySelectorAll('#lsb-rows .hn .crest img')].map((i) => i.getAttribute('src'));
  assert.deepEqual(srcs, ['/houses/mikkarol.webp', '/houses/mikkarol.webp']);
});

test('a house with no picture keeps the two letters, and a strange name never becomes a path', async () => {
  const odd = ['Nonesuch', '../../app.js', '__proto__', 'constructor', 'toString', '<img src=x onerror=alert(1)>', 'Alexin/../Sor', 'ALEXIN', ''];
  const ui = await open(odd);
  const rows = [...ui.doc.querySelectorAll('#lsb-rows > li')];
  assert.equal(rows.length, odd.length);
  for (const r of rows) {
    const name = r.querySelector('.hn b').textContent;
    const crest = crestOf(r), img = crest.querySelector('img');
    if (name === 'ALEXIN') { assert.equal(img.getAttribute('src'), '/houses/alexin.webp'); continue; } // the lookup ignores case
    assert.equal(img, null, JSON.stringify(name) + ' must not get a picture');
    assert.equal(crest.textContent, name.slice(0, 2));
  }
  assert.equal(ui.doc.querySelectorAll('#lsb-rows img[onerror], #lsb-rows .hn b img').length, 0);
  assert.equal([...ui.doc.querySelectorAll('#lsb-rows img')].filter((i) => !/^\/houses\/[a-z]+\.webp$/.test(i.getAttribute('src'))).length, 0);
});
