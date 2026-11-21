// The Landsraad page on the Command Center board (#599, ported from the Classic list in #1028, the board is the only view): what a House says about itself in the list, its hover card, its Inspector, its place
// (the link to the Saved map, the quick peek, the reasons a place cannot be opened), the earlier terms and the words of a save with no Houses. The data here is a few Houses, which is not a complete board,
// so the page draws the read-only list of the Houses (nothing can be written from it); the tier and the faction box tests pad the read to a full 25 Houses so that the board is complete. Every write has its
// own file (lcc-board-write, lcc-board-requests, lcc-inspector-pass, lcc-list-row-actions). Real app.js and lcc scripts, no game save.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import { boot, closeAll } from './boot.mjs';

afterEach(closeAll);

const TH = [1200, 2400, 3600, 4800, 6000];
const OWN = ['SolarisCoin', 'T6ArmorPlating', 'T6RangeFinder', 'T6HoltzmanActuator', 'T6DiamodineBladeParts'];
// a house with the character at `mine`; states: the reward_state of each tier by threshold (default: reached ones are no_row, others not_reached)
const house = (id, name, mine, extra = {}, states = {}) => ({
  id, board_index: id - 900, house_name: 'DA_House' + name, goal_amount: 4000, completed: 0, winning_faction_id: null, completion_time: null,
  contributions: [], player_contributions: [], my_contribution: mine, my_recorded: mine > 0, swatch_held: false, location: { known: false },
  rewards: TH.map((threshold, i) => {
    const reached = mine >= threshold, top = i === 4;
    return { threshold, amount: top ? 75 : [10000, 1, 50, 1][i], template_id: OWN[i], reached, delivered: false,
      shown_template_id: top ? 'Alexin_Placeables_Swatch' : OWN[i], shown_amount: top ? 1 : [10000, 1, 50, 1][i], swatch: top,
      reward_state: states[threshold] || (reached ? 'no_row' : 'not_reached') };
  }),
  ...extra,
});
const board = (extra = {}) => ({
  term: { term_id: 7, start_time: 100, end_time: 200, reigning_faction_id: 1, active_decree_id: 1, elected_decree_id: 1 },
  terms: [{ term_id: 7, start_time: 100, end_time: 200 }],
  earlier_terms: [],
  factions: [{ id: 1, name: 'Atreides' }, { id: 2, name: 'Harkonnen' }],
  character: { player_id: 1, name: 'Tester', faction_id: 1 },
  decrees: [{ id: 1, decree_name: 'Decree A', disabled: 0 }], rotation: [],
  tasks: [
    house(903, 'Wayku', 6500, { completed: 1, winning_faction_id: 1, contributions: [{ faction_id: 1, amount: 4000 }, { faction_id: 2, amount: 1500 }] }, { 1200: 'delivered', 2400: 'delivered', 3600: 'waiting', 4800: 'waiting', 6000: 'waiting' }),
    house(900, 'Alexin', 0, { contributions: [{ faction_id: 2, amount: 900 }] }),
    house(901, 'Thorvald', 4700, { contributions: [{ faction_id: 1, amount: 3000 }, { faction_id: 2, amount: 1000 }] }),
    house(902, 'Hurata', 1300, { completed: 1, winning_faction_id: 2, contributions: [{ faction_id: 2, amount: 4000 }] }),
  ],
  ...extra,
});
// the read padded with Houses of no contribution to the 25 of a complete board: the board is then drawn with its controls and its cards
const complete = (data) => {
  const used = new Set(data.tasks.map((t) => t.board_index)), free = [...Array(25).keys()].filter((i) => !used.has(i));
  return { ...data, tasks: [...data.tasks, ...free.map((i) => ({ ...house(1000 + i, 'F' + i, 0), board_index: i }))] };
};
async function open(data = board(), { layout = 'list', full = false } = {}) {
  const ui = await boot({ get: { '/api/landsraad': full ? complete(data) : data }, local: { tab: 'landsraad', 'lsb.layout': layout } });
  await ui.settle();
  return ui;
}
const rows = (ui) => [...ui.doc.querySelectorAll('tr.lcc-bd-row')];
const rowOf = (ui, id) => ui.doc.querySelector(`tr.lcc-bd-row[data-id="${id}"]`);
const nameOf = (r) => r.querySelector('.lcc-bd-namebtn').textContent;
const text = (el) => el.textContent.replace(/\s+/g, ' ').trim();
const click = (ui, sel) => { const el = typeof sel === 'string' ? ui.doc.querySelector(sel) : sel; assert.ok(el, 'no ' + sel); el.dispatchEvent(new ui.w.MouseEvent('click', { bubbles: true, cancelable: true })); };
// the House looked at: the Inspector opens on it
const look = async (ui, id) => { click(ui, `[data-fk="ht:${id}"]`); await ui.settle(); return ui.doc.querySelector('.lcc-bd-insp'); };
// the last toast: the board says what a click did (a reason, a hint) as a toast, there is no result line (#943)
const toastText = (ui) => { const t = [...ui.doc.querySelectorAll('#toast .d-toast')].pop(); return t ? text(t.querySelector('span')) : ''; };
// records what the board asks the Saved map to show (the board calls window.liveMapShow before it changes the tab)
const spyMap = (ui) => { const calls = []; const real = ui.w.liveMapShow; ui.w.liveMapShow = (...a) => { calls.push(a); real(...a); }; return calls; };
const css = () => fs.readFileSync(new URL('../internal/web/static/app.css', import.meta.url), 'utf8');

test('the board lists every house by name with its state, tiers and next reward; it writes nothing', async () => {
  const ui = await open();
  assert.deepEqual(rows(ui).map(nameOf), ['Alexin', 'Hurata', 'Thorvald', 'Wayku'], 'a list that is not a complete board is by name');
  const cell = (n, c) => text(rows(ui).find((r) => nameOf(r) === n).querySelector('.lcc-bd-c-' + c));
  assert.equal(cell('Alexin', 'state'), 'Not started');
  assert.equal(cell('Thorvald', 'state'), 'In progress');
  assert.equal(cell('Wayku', 'state'), 'Top tier');
  assert.equal(cell('Thorvald', 'tiers'), '3 of 5 tiers earned');
  // next tier: the amount and the missing amount, the reward is in its hover (named by the game's item name); the top tier says so
  assert.equal(cell('Thorvald', 'next'), '4,800 (100 more)');
  assert.equal(cell('Wayku', 'next'), 'top tier earned');
  assert.ok(text(rowOf(ui, 901).querySelector('.hcard')).includes('Next reward4,800: 1 Improved Holtzman Actuator'), 'the card names the reward by the game\'s item name');
  assert.equal(rowOf(ui, 900).querySelector('.lcc-bd-c-state').title, 'No contribution of yours recorded. 0 of 5 tiers earned.', 'no contribution is said in words (#1056)');
  assert.equal(cell('Hurata', 'owner'), 'Harkonnen');
  assert.equal(cell('Wayku', 'owner'), 'Atreides');
  assert.equal(cell('Thorvald', 'owner'), 'Wavering');
  assert.deepEqual(ui.posted, [], 'the board only reads the save');
  assert.equal(ui.doc.querySelector('#lsb-lcc .lcc-bd-act, #lsb-lcc [data-fk^="b-"], #lsb-lcc [data-lb="tier"], #lsb-fac'), null, 'a read that is not a complete board has no control that writes');
});

test('a tier says what it gives, by the game’s name, and where it stands: not reached, waiting, delivered, or earned with no delivery recorded', async () => {
  const ui = await open(board(), { layout: 'grid', full: true });
  const insp = await look(ui, 903);
  const t = [...insp.querySelectorAll('.lcc-bd-tier')];
  assert.equal(t.length, 5);
  assert.deepEqual(t.map((x) => text(x.querySelector('.lcc-bd-tier-st'))), ['Earned, collected', 'Earned, collected', 'Earned, not collected', 'Earned, not collected', 'Earned, not collected']);
  assert.ok(text(t[0].querySelector('.lcc-bd-tier-rw')).startsWith('10,000 '), text(t[0].querySelector('.lcc-bd-tier-rw')));
  assert.ok(!text(insp).includes('SolarisCoin') && ![...insp.querySelectorAll('[title],[aria-label]')].some((e) => /SolarisCoin|Game id/.test((e.title || '') + (e.getAttribute('aria-label') || ''))), 'the internal name is shown nowhere, not even in a tooltip (#977)');
  assert.ok(t[0].classList.contains('dv') && t[2].classList.contains('wt'), 'collected and waiting are marked apart');
  assert.ok(text(t[4].querySelector('.lcc-bd-tier-rw')).includes('Placeables Swatch'), 'the top tier shows the swatch while it is not held');
  assert.ok(t[0].getAttribute('aria-label').includes('Tier 1, 1,200, reward 10,000'), t[0].getAttribute('aria-label'));
  // the card's own tier boxes say the same: the tooltip keeps the display name of the reward, the internal name is nowhere
  const box = ui.doc.querySelector('[data-fk="ct:903:1"]');
  assert.ok(box.title.startsWith('Reward: 10,000 Solari') && !box.title.includes('SolarisCoin'), box.title);
  assert.ok(box.getAttribute('aria-label').includes('collected'), box.getAttribute('aria-label'));
  assert.ok(ui.doc.querySelector('[data-fk="ct:903:3"]').getAttribute('aria-label').includes('not collected'));
  const thor = [...(await look(ui, 901)).querySelectorAll('.lcc-bd-tier-st')].map(text);
  assert.deepEqual(thor, ['Earned, delivery unknown', 'Earned, delivery unknown', 'Earned, delivery unknown', 'Not reached', 'Not reached']);
  // once the swatch is held the top tier is the task's own item
  const base = house(900, 'Alexin', 6500, { swatch_held: true });
  const held = board({ tasks: [{ ...base, rewards: base.rewards.map((r, i) => (i === 4 ? { ...r, shown_template_id: 'T6DiamodineBladeParts', shown_amount: 75, swatch: false } : r)) }] });
  const ui2 = await open(held, { layout: 'grid', full: true });
  const top = [...(await look(ui2, 900)).querySelectorAll('.lcc-bd-tier')].pop();
  assert.ok(text(top.querySelector('.lcc-bd-tier-rw')).startsWith('75 '), text(top.querySelector('.lcc-bd-tier-rw')));
});

test('the place: a house with no position says so and does not open anything; one with a position shows the area and the pin in its quick peek', async () => {
  const known = { known: true, map: 'Hagga Basin', map_key: 'HaggaBasin', area: 'The O’dham', place: 'north rim', x: 0.25, y: 0.5 };
  const ui = await open(board({ tasks: [house(900, 'Alexin', 0), house(901, 'Thorvald', 100, { location: known })] }));
  assert.ok(text(rowOf(ui, 900).querySelector('td.lcc-bd-loc')).includes('not mapped yet'));
  assert.ok(text(rowOf(ui, 901).querySelector('td.lcc-bd-loc')).includes('The O’dham'));
  assert.ok(text(rowOf(ui, 901).querySelector('.hcard')).includes('The O’dham (Hagga Basin)'), 'the card names the area and the map');
  assert.equal(rowOf(ui, 900).querySelector('td.lcc-bd-loc [data-lb=map]'), null, 'a house with no position has no link');
  assert.equal(rowOf(ui, 900).querySelector('.pin'), null, 'no position is drawn that is not known');
  assert.ok(text(rowOf(ui, 900).querySelector('.hcard')).includes('This house is not mapped yet'), 'the card explains it');
  const calls = spyMap(ui);
  // in the Inspector the place is a muted button: a click says why and opens nothing
  const insp = await look(ui, 900);
  const b = insp.querySelector('.lcc-bd-loc[data-lb="map"]');
  assert.equal(b.getAttribute('aria-disabled'), 'true');
  click(ui, b);
  assert.equal(ui.doc.querySelector('dialog'), null, 'no window is opened');
  assert.ok(toastText(ui).includes('not mapped yet'), 'the toast says why: ' + toastText(ui));
  assert.deepEqual(calls, [], 'nothing is opened on the Saved map');
  assert.ok(text(insp.querySelector('#lcc-bd-s5').parentElement).includes('Not mapped yet'), 'the Place part of the Inspector explains it too');
  // #1069: the picture is a window around the pin, so the pin is at the middle of the window; the small whole map carries the place itself (a quarter in, halfway down)
  const pop = rowOf(ui, 901).querySelector('td.lcc-bd-loc .loc > .pop');
  assert.equal(pop.querySelector('.minimap .mwin .pin').style.top, '50%');
  assert.equal(pop.querySelector('.minimap .mwin .pin').style.left, '50%');
  assert.equal(pop.querySelector('.minimap .mfull .mdot').style.top, '50%');
  assert.equal(pop.querySelector('.minimap .mfull .mdot').style.left, '25%');
  assert.ok(text(pop).includes('north rim'));
});

test('the house name card names the next reward, what waits, the faction standing and the place; it shows on hover and on keyboard focus, and the name looks at the house', async () => {
  const ui = await open();
  const hw = rowOf(ui, 903).querySelector('.hw');
  const card = text(hw.querySelector('.hcard'));
  assert.ok(card.includes('Not collected') && card.includes('Faction standing') && card.includes('Atreides 4,000 of 4,000') && card.includes('Pledged to Atreides') && card.includes('Place'), card);
  assert.ok(card.includes('top tier earned'));
  assert.match(css(), /\.lsb \.hw:hover>\.hcard\{display:block\}/, 'the card shows on hover');
  assert.match(css(), /\.lsb \.hw:has\(:focus-visible\)>\.hcard/, 'and on keyboard focus');
  assert.match(css(), /\.lsb \.hw\[data-esc\]>\.hcard\{display:none!important\}/, 'and Escape puts it away');
  const insp = await look(ui, 903); // the click on the name: the House is looked at in the Inspector
  assert.equal(insp.dataset.id, '903');
  assert.ok(text(insp).includes('House Wayku'));
});

test('the house name card lists each waiting reward on its own line, and says so in words when nothing waits (#666)', async () => {
  const data = board({ tasks: [
    house(903, 'Wayku', 6500, {}, { 1200: 'delivered', 2400: 'delivered', 3600: 'waiting', 4800: 'waiting', 6000: 'waiting' }),
    house(900, 'Alexin', 0),
    house(901, 'Thorvald', 4700, {}, { 1200: 'delivered', 2400: 'delivered', 3600: 'waiting' }),
  ] });
  const ui = await open(data);
  const waitingOf = (id) => [...rowOf(ui, id).querySelectorAll('.hcard dt')].find((d) => text(d) === 'Not collected').nextElementSibling;
  const dd = waitingOf(903);
  const lines = [...dd.querySelectorAll('.hrw')];
  assert.deepEqual(lines.map((l) => text(l)), ['50 Precision Range Finder', '1 Improved Holtzman Actuator', '1 House Alexin Placeables Swatch'], 'one line per waiting reward, in tier order');
  assert.equal(dd.children.length, 3, 'nothing but the reward lines inside');
  assert.ok(!dd.innerHTML.includes(', '), 'no comma-joined run is left');
  assert.ok(css().includes('.lsb .hcard dd .hrw{display:block}'), 'the stylesheet draws each reward as its own block');
  assert.equal(waitingOf(901).querySelectorAll('.hrw').length, 1);
  const none = waitingOf(900);
  assert.equal(none.querySelectorAll('.hrw').length, 0);
  assert.equal(text(none), 'nothing waiting');
});

test('earlier terms: the terms with a contribution of the character, each with its houses; a save with one term says so', async () => {
  const data = board({
    terms: [{ term_id: 7 }, { term_id: 6 }],
    earlier_terms: [{ term_id: 6, start_time: 0, end_time: 36000, houses: [
      { house_name: 'DA_HouseMaros', my_contribution: 6500, tiers_reached: 5, tier_count: 5, completed: 1, winning_faction_id: 1, paid_in_term: true },
      { house_name: 'DA_HouseKenola', my_contribution: 500, tiers_reached: 0, tier_count: 5, completed: 0, winning_faction_id: null, paid_in_term: false }] }],
  });
  const ui = await open(data);
  const hist = ui.doc.querySelector('#lsb-lcc-term .lcc-tm-hist');
  assert.ok(text(hist.querySelector('summary')).startsWith('Earlier terms'), 'the section is read-only and closed');
  assert.equal(hist.open, false);
  const term = hist.querySelector('details.lcc-tm-term');
  assert.ok(text(term.querySelector('summary')).startsWith('Term 6') && text(term.querySelector('summary')).includes('2 houses with your contribution'), text(term.querySelector('summary')));
  assert.ok(text(term.querySelector('summary')).includes('1 top tier') && text(term.querySelector('summary')).includes('1 pledged to your faction'));
  assert.deepEqual([...term.querySelectorAll('tbody tr')].map((r) => text(r.cells[0])), ['Maros', 'Kenola']);
  assert.ok(text(term).includes('Undecided at term end') && !text(term).includes('Play clock') && !text(term).includes('36,000 s'), 'no play-clock line under an earlier term (#981)');
  const one = await open(board());
  assert.ok(text(one.doc.querySelector('#lsb-lcc-term .lcc-tm-hist')).includes('only the current term'));
});

test('house names from the save are text, never markup', async () => {
  const ui = await open(board({ tasks: [house(900, '<img src=x onerror=alert(1)>', 0)] }));
  assert.equal(ui.doc.querySelector('#lsb img[onerror], #lsb img[src="x"]'), null);
  assert.ok(text(rowOf(ui, 900).querySelector('.lcc-bd-namebtn')).includes('<img'));
  const insp = await look(ui, 900);
  assert.equal(insp.querySelector('img[onerror]'), null);
  assert.ok(text(insp.querySelector('.lcc-bd-insp-name')).includes('<img'));
});

test('the place has a quick peek for a mouse only where the position is known', async () => {
  const known = { known: true, map: 'Hagga Basin', map_key: 'HaggaBasin', area: 'The O’dham', place: 'north rim', x: 0.25, y: 0.5 };
  const ui = await open(board({ tasks: [house(900, 'Alexin', 0), house(901, 'Thorvald', 100, { location: known })] }));
  assert.equal(rowOf(ui, 900).querySelector('td.lcc-bd-loc .pop'), null, 'no peek for a house with no position');
  const pop = rowOf(ui, 901).querySelector('td.lcc-bd-loc .loc > .pop');
  assert.ok(pop, 'the peek is in the place');
  assert.equal(pop.getAttribute('aria-hidden'), 'true', 'the peek is mouse only; the card and the Inspector are the way in for the keyboard and touch');
  assert.ok(text(pop).includes('The O’dham (Hagga Basin)') && text(pop).includes('north rim') && text(pop).includes('Click the place to open the Saved map'), text(pop));
  assert.equal(pop.querySelector('.mwin .pin').style.left, '50%', 'the pin is the middle of the picture window');
  assert.equal(pop.querySelector('.mfull .mdot').style.left, '25%', 'and the whole map holds the place itself');
  assert.ok(rowOf(ui, 901).querySelector('td.lcc-bd-loc [data-lb=map]').title.includes('Hover for a quick peek'));
});

test('a save with no houses in the term says so in place of the board', async () => {
  const ui = await open(board({ tasks: [] }));
  const b = ui.doc.querySelector('#lsb-board');
  assert.ok(text(b).includes('no Landsraad Houses in the current term, so there is nothing to show or change'), text(b).slice(-300));
  assert.equal(b.querySelector('tr.lcc-bd-row, .lcc-bd-cell, .lcc-bd-act, [data-lb="tier"], [data-lb="bulk"], #lsb-fac'), null, 'no House and no control is offered against an empty board');
  assert.deepEqual(ui.posted, []);
});

test('the stylesheet styles the status lines by the ids the board draws, and keeps the term facts apart from the Player tab\'s .kv cards', async () => {
  const js = fs.readFileSync(new URL('../internal/web/static/app.js', import.meta.url), 'utf8');
  const sheet = css();
  // the board has no result line (its results are toasts, #943): the count of the chosen Houses and the term panel's line are what is left
  for (const id of ['lsb-sum', 'lsb-tresult']) {
    assert.ok(js.includes('id="' + id + '"') || fs.readFileSync(new URL('../internal/web/static/lcc/lcc-term.js', import.meta.url), 'utf8').includes("id: '" + id + "'"), id + ' is drawn');
    assert.ok(sheet.includes('.lsb #' + id + '{'), id + ' is styled');
  }
  assert.ok(!/\.lsb \.kv\b/.test(sheet), 'the global .kv card (boxed, 11px values, words broken anywhere) must not meet the term facts');
});

test('a house with no place says so in its Place cell, with some places known as with none', async () => {
  const ui = await open();
  assert.ok(rows(ui).every((r) => text(r.querySelector('td.lcc-bd-loc')) === '— not mapped yet'), 'each Place cell says it in words, the same words as a single house without a place (#669)');
  assert.equal(ui.doc.querySelectorAll('td.lcc-bd-loc [data-lb=map]').length, 0, 'no place button in any row');
  const known = { known: true, map: 'Hagga Basin', area: 'The O’dham', place: 'rim', x: 0.25, y: 0.5 };
  const ui2 = await open(board({ tasks: [house(900, 'Alexin', 0, { location: known }), house(901, 'Thorvald', 100)] }));
  assert.ok(text(rowOf(ui2, 901).querySelector('td.lcc-bd-loc')).includes('not mapped yet'), 'with some places known, a house without one still says so');
  assert.ok(text(rowOf(ui2, 900).querySelector('td.lcc-bd-loc')).includes('The O’dham'));
});

test('the peek and the Inspector give the distance from you in plain words; a click on a place with a picture opens the Saved map there, with no window in between', async () => {
  const known = { known: true, map: 'Hagga Basin', map_key: 'HaggaBasin', area: 'The O’dham', place: 'north rim', x: 0.25, y: 0.5, from_you_cm: 281000 };
  const far = { known: true, map: 'Deep Desert', map_key: 'DeepDesert', area: 'The Sink', place: '', x: 0.5, y: 0.5 };
  const hub = { known: true, map: 'Arrakeen', map_key: '', area: 'Arrakeen', place: '', x: 0.5, y: 0.5 };
  const ui = await open(board({ tasks: [house(900, 'Alexin', 0, { location: known }), house(901, 'Thorvald', 100, { location: far }), house(902, 'Hurata', 0, { location: hub })] }));
  assert.ok(text(rowOf(ui, 900).querySelector('td.lcc-bd-loc .loc > .pop')).includes('From youabout 2.8 km'), text(rowOf(ui, 900).querySelector('td.lcc-bd-loc .loc > .pop')));
  assert.ok(text(await look(ui, 900)).includes('From you: about 2.8 km.'));
  const far1 = text(rowOf(ui, 901).querySelector('td.lcc-bd-loc .loc > .pop'));
  assert.ok(!far1.includes('another map') && !/\d km/.test(far1), 'another map: no invented distance, and no long reason standing in the peek: ' + far1);
  assert.ok(!/not worked out/.test(ui.doc.body.textContent), 'the old wording is gone');
  assert.ok(!text(rowOf(ui, 902).querySelector('.hcard')).includes('From you'), 'a hub has no From you line');
  const calls = spyMap(ui);
  click(ui, rowOf(ui, 900).querySelector('td.lcc-bd-loc [data-lb=map]'));
  await ui.settle();
  assert.deepEqual(calls, [['HaggaBasin', 0.25, 0.5, 'Alexin', 900]], 'the Saved map is shown at the house, in one click');
  assert.equal(ui.doc.querySelector('dialog'), null, 'no window');
  assert.equal(ui.doc.querySelector('#lsb'), null, 'the Landsraad page is left');
  assert.equal(ui.doc.querySelector('[data-tab=livemap]').getAttribute('aria-current'), 'page', 'the Saved map tab is the open one');
});

test('a hub (a place the Saved map has no picture of) is muted, says why in its tooltip and name, and a click opens nothing', async () => {
  const hub = { known: true, map: 'Harko Village', map_key: '', area: 'Harko Village', place: '', cell: '', unconfirmed: false };
  const ui = await open(board({ tasks: [house(900, 'Alexin', 0, { location: hub })] }));
  const why = 'This place is outside the picture of Harko Village.';
  // the list cell is plain text with the reason, not a link
  const cellNote = rowOf(ui, 900).querySelector('td.lcc-bd-loc .lcc-bd-nopic');
  assert.equal(cellNote.title, why.replace(/\.$/, ''));
  assert.equal(rowOf(ui, 900).querySelector('td.lcc-bd-loc [data-lb=map]'), null, 'no link in the cell');
  assert.ok(text(cellNote).includes('no map picture'), 'a cue in words, not only a colour');
  // the card's place line is a muted button with the reason in its name and tooltip
  const btn = rowOf(ui, 900).querySelector('.hcard .hcard-place');
  assert.ok(btn.classList.contains('off'), 'muted, not link-styled');
  assert.equal(btn.getAttribute('aria-disabled'), 'true');
  assert.equal(btn.title, why);
  assert.ok(btn.getAttribute('aria-label').startsWith('House Alexin: Harko Village') && btn.getAttribute('aria-label').includes(why), btn.getAttribute('aria-label'));
  const hc = text(rowOf(ui, 900).querySelector('.hcard'));
  assert.ok(hc.includes(why) && !/open the Saved map/.test(hc));
  const pop = text(rowOf(ui, 900).querySelector('td.lcc-bd-loc .pop'));
  assert.ok(pop.includes(why) && !/Click the place to open the Saved map/.test(pop), pop);
  const calls = spyMap(ui);
  click(ui, btn);
  assert.deepEqual(calls, [], 'nothing is opened on the Saved map');
  assert.equal(ui.doc.querySelector('dialog'), null, 'no window');
  assert.ok(ui.doc.querySelector('#lsb'), 'the Landsraad page stays');
  assert.equal(toastText(ui), why, 'the toast says it');
  assert.equal(text(btn.closest('.loc').querySelector('.lnote')), why, 'and the reason is shown next to the place, where the click was');
  assert.equal(btn.closest('.loc').querySelector('.lnote').getAttribute('role'), 'status');
  // the Inspector: the same muted button with the cue, and the reason under the place
  const insp = await look(ui, 900);
  const ib = insp.querySelector('.lcc-bd-loc[data-lb="map"]');
  assert.ok(ib.classList.contains('off') && ib.getAttribute('aria-disabled') === 'true' && ib.title === why);
  assert.equal(text(ib.querySelector('.lcc-bd-loc-s')), 'no map picture', 'a cue in words, not only a colour');
  assert.equal(text(insp.querySelector('#lcc-bd-s5').parentElement.querySelector('.lcc-bd-cap')), why);
  assert.equal(insp.querySelector('.pin'), null, 'no pin is drawn on a made-up picture');
  assert.ok(!text(insp.querySelector('#lcc-bd-s5').parentElement).includes('From you'));
  const pn = text(insp.querySelector('.lcc-bd-identity .pop .pnote'));
  assert.ok(pn.includes('outside the frame of the picture of Harko Village') && pn.includes('no pin or map link'), pn); // #1069: the editor has the picture; the place is outside its frame
});

test('the click on a place that cannot be opened shows its reason next to it; so does a Saved map that is not ready', async () => {
  const known = { known: true, map: 'Hagga Basin', map_key: 'HaggaBasin', area: 'The O’dham', place: 'north rim', x: 0.25, y: 0.5 };
  const ui = await open(board({ tasks: [house(900, 'Alexin', 0, { location: { known: false } }), house(901, 'Thorvald', 100, { location: known })] }));
  const insp = await look(ui, 900);
  assert.equal(toastText(ui), '', 'nothing is said before a click');
  click(ui, insp.querySelector('.lcc-bd-loc[data-lb="map"]'));
  assert.ok(toastText(ui).includes('no map marker'), 'the toast says why: ' + toastText(ui)); // the Inspector's muted place sits in no .loc, so the reason is the toast; the card's and the list's places keep it next to them (below)
  assert.equal(rowOf(ui, 901).querySelector('td.lcc-bd-loc .lnote'), null, 'a place that opens has no note');
  ui.w.liveMapShow = undefined;
  click(ui, rowOf(ui, 901).querySelector('td.lcc-bd-loc [data-lb=map]'));
  assert.equal(text(rowOf(ui, 901).querySelector('td.lcc-bd-loc .lnote')), 'The Saved map is not ready yet.');
  assert.equal(toastText(ui), 'The Saved map is not ready yet.');
  assert.ok(ui.doc.querySelector('#lsb'), 'the page stays');
});

test('a place with a picture reads as a link to the Saved map: not muted, and its name and tooltip say it opens the Saved map', async () => {
  const known = { known: true, map: 'Hagga Basin', map_key: 'HaggaBasin', area: 'The O’dham', place: 'north rim', x: 0.25, y: 0.5 };
  const ui = await open(board({ tasks: [house(900, 'Alexin', 0, { location: known })] }));
  const btn = rowOf(ui, 900).querySelector('td.lcc-bd-loc [data-lb=map]');
  assert.ok(!btn.classList.contains('off') && !btn.hasAttribute('aria-disabled'));
  assert.equal(btn.tagName, 'BUTTON', 'a real button: Enter and Space work, and it takes focus');
  assert.ok(btn.getAttribute('aria-label').endsWith('Open in the Saved map'), btn.getAttribute('aria-label'));
  assert.ok(btn.title.startsWith('Open The O’dham in the Saved map'), btn.title);
  assert.ok(!btn.hasAttribute('aria-haspopup'), 'it opens no window');
});

test('the place button works from the keyboard: a button takes the Enter/Space click, and a muted place still takes focus so its reason can be read', async () => {
  const known = { known: true, map: 'Deep Desert', map_key: 'DeepDesert', area: 'Deep Desert', place: '', cell: 'B8', x: 0.5, y: 0.75 };
  const hub = { known: true, map: 'Arrakeen', map_key: '', area: 'Arrakeen', place: '' };
  const ui = await open(board({ tasks: [house(900, 'Maros', 0, { location: known }), house(901, 'Hurata', 0, { location: hub }), house(902, 'Alexin', 0)] }));
  const calls = spyMap(ui);
  // the muted places of the card (a hub) and of the Inspector (no position); each is found again when it is wanted, a click may draw the Inspector again
  await look(ui, 902);
  for (const find of [() => rowOf(ui, 901).querySelector('.hcard .hcard-place'), () => ui.doc.querySelector('.lcc-bd-insp .lcc-bd-loc[data-lb="map"]')]) {
    const b = find();
    b.focus();
    assert.equal(ui.doc.activeElement, b, 'the muted place still takes focus, so its reason can be read and heard');
    b.click(); // what Enter and Space do on a button
    await ui.settle();
  }
  assert.deepEqual(calls, []);
  assert.equal(ui.doc.querySelector('dialog'), null);
  const b = rowOf(ui, 900).querySelector('td.lcc-bd-loc [data-lb=map]');
  b.focus();
  b.click();
  await ui.settle();
  assert.deepEqual(calls, [['DeepDesert', 0.5, 0.75, 'Maros', 900]]);
});

test('a Deep Desert row names its grid cell; Tseida-like places say they are from public guides', async () => {
  const dd = { known: true, map: 'Deep Desert', map_key: 'DeepDesert', area: 'Deep Desert', place: '', cell: 'B8', unconfirmed: false, x: 0.5, y: 0.5 };
  const tr = { known: true, map: 'Hagga Basin', map_key: 'HaggaBasin', area: 'Eastern Vermillius Gap', place: 'The Anvil Tradepost', cell: '', unconfirmed: true, x: 0.5, y: 0.5 };
  const ui = await open(board({ tasks: [house(900, 'Maros', 0, { location: dd }), house(901, 'Tseida', 0, { location: tr })] }));
  assert.ok(text(rowOf(ui, 900).querySelector('td.lcc-bd-loc')).includes('B8'), 'the cell is in the row');
  assert.ok(text(rowOf(ui, 901).querySelector('td.lcc-bd-loc .pop')).includes('The Anvil Tradepost'), 'the place is in the peek');
  const i0 = text(await look(ui, 900));
  assert.ok(i0.includes('grid cell B8'), 'the Inspector names the cell');
  assert.ok(!i0.includes('public guides'), 'a confirmed place has no hedge');
  const i1 = text(await look(ui, 901));
  assert.ok(i1.includes('The Anvil Tradepost') && i1.includes('from public guides and is not confirmed in the game'));
  assert.ok(text(rowOf(ui, 901).querySelector('td.lcc-bd-loc .pop')).includes('not confirmed in the game'), 'the peek says it too');
});

test('Group by region draws one heading per region with the number of houses in it', async () => {
  const mk = (map, key) => ({ known: true, map, map_key: key, area: map, place: '', cell: '', unconfirmed: false, x: 0.5, y: 0.5 });
  const ui = await open(board({ tasks: [house(900, 'Alexin', 0, { location: mk('Harko Village', '') }), house(901, 'Thorvald', 0, { location: mk('Hagga Basin', 'HaggaBasin') }), house(902, 'Hurata', 0, { location: mk('Hagga Basin', 'HaggaBasin') })] }));
  assert.equal(ui.doc.querySelectorAll('tbody.lcc-bd-group').length, 0, 'no headings until asked');
  click(ui, '[data-fk="gp:region"]');
  const heads = () => [...ui.doc.querySelectorAll('tbody.lcc-bd-group th[scope="rowgroup"]')].map(text);
  assert.deepEqual(heads(), ['Hagga Basin · 2', 'Harko Village · 1']);
  click(ui, '[data-fk="gp:none"]');
  assert.equal(ui.doc.querySelectorAll('tbody.lcc-bd-group').length, 0, 'the headings go when grouping is off');
});

test('the faction box says which faction is the character\'s, the same way as the faction crests', async () => {
  const ui = await open(board(), { layout: 'grid', full: true });
  assert.equal(text(ui.doc.querySelector('#lsb-fac option[value="1"]')), 'Atreides (your faction)');
  assert.equal(text(ui.doc.querySelector('#lsb-fac option[value="2"]')), 'Harkonnen');
  assert.ok(ui.doc.querySelector('[data-fk="seat:1"]').getAttribute('aria-label').endsWith('your faction'));
});

// #842: a save that already holds a house won by a faction the Landsraad does not have (Smuggler, 4, from an earlier editor write). The card still says "Other faction" (or the number when the factions are not
// loaded); the board's own Pledged-to cell cannot name a faction it does not know and says Unreadable, so no number is ever shown as a name.
test('a house won by a faction the list does not offer reads "Other faction" in its card and Unreadable in the Pledged-to cell, never a number as a name', async () => {
  const b = board();
  b.tasks.push(house(904, 'Zeta', 0, { completed: 1, winning_faction_id: 4, contributions: [{ faction_id: 4, amount: 4000 }] }));
  const ui = await open(b);
  const zeta = rowOf(ui, 904);
  assert.ok(text(zeta.querySelector('.hcard')).includes('Pledged to Other faction'), text(zeta.querySelector('.hcard')));
  assert.equal(zeta.querySelector('.lcc-bd-c-owner .lcc-owner').title, 'Favor unreadable');
  assert.ok(text(zeta.querySelector('.lcc-bd-c-owner')).includes('Unreadable'));
  assert.ok(!/Faction 4/.test(text(ui.doc.querySelector('#lsb'))), 'the number of the faction is not shown as its name');
});

test('"Other faction" is said only when the list of the Landsraad factions is loaded and does not offer the id; with no list the number is shown (#842)', async () => {
  const b = board({ factions: [] });
  b.tasks.push(house(904, 'Zeta', 0, { completed: 1, winning_faction_id: 4, contributions: [{ faction_id: 4, amount: 4000 }] }));
  const ui = await open(b);
  const card = text(rowOf(ui, 904).querySelector('.hcard'));
  assert.ok(card.includes('Pledged to Faction 4') && !card.includes('Other faction'), card);
  assert.ok(!/Other faction/.test(text(ui.doc.querySelector('#lsb'))), 'no "Other faction" without a list');
});
