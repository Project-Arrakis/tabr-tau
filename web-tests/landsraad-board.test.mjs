// The new Landsraad board (#599): one list of the houses, alphabetical, collapsed rows that open to the tiers with their rewards, the filters, the sort, the jump and the earlier terms.
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
async function open(data = board()) {
  const ui = await boot({ get: { '/api/landsraad': data } });
  ui.click('[data-tab="landsraad"]'); await ui.settle();
  return ui;
}
const rows = (ui) => [...ui.doc.querySelectorAll('#lsb-rows > li.hr')];
const visible = (ui) => rows(ui).filter((r) => !r.hidden).map((r) => r.querySelector('.hn b').textContent);
const text = (el) => el.textContent.replace(/\s+/g, ' ').trim();
const click = (ui, sel) => { const el = ui.doc.querySelector(sel); assert.ok(el, 'no ' + sel); el.dispatchEvent(new ui.w.MouseEvent('click', { bubbles: true, cancelable: true })); };
// records what the board asks the Saved map to show (the board calls window.liveMapShow before it changes the tab)
const spyMap = (ui) => { const calls = []; const real = ui.w.liveMapShow; ui.w.liveMapShow = (...a) => { calls.push(a); real(...a); }; return calls; };

test('the board lists every house alphabetically with its state, tiers and next reward; it writes nothing', async () => {
  const ui = await open();
  assert.deepEqual(rows(ui).map((r) => r.querySelector('.hn b').textContent), ['Alexin', 'Hurata', 'Thorvald', 'Wayku']);
  const by = (n) => rows(ui).find((r) => r.querySelector('.hn b').textContent === n);
  assert.equal(by('Alexin').dataset.my, 'new');
  assert.equal(by('Thorvald').dataset.my, 'part');
  assert.equal(by('Wayku').dataset.my, 'done');
  assert.ok(text(by('Wayku').querySelector('.st')).includes('Top tier'));
  // next tier: the reward is named by the game's item name, the missing amount is its own piece
  const nx = [...by('Thorvald').querySelectorAll('.js-next .nx')].map((e) => e.textContent);
  assert.equal(nx.length, 2);
  assert.ok(/^next 4,800 · 1 Pincushion Gauntlets|^next 4,800 · 1 /.test(nx[0]) && nx[0].endsWith(','), nx[0]);
  assert.equal(nx[1], '100 more');
  assert.ok(text(by('Wayku').querySelector('.js-next')).includes('above the top tier'));
  assert.ok(text(by('Alexin').querySelector('.amt')).includes('none recorded'));
  assert.ok(text(by('Hurata').querySelector('.c-hs')).includes('Taken · Harkonnen'));
  assert.ok(text(by('Thorvald').querySelector('.c-hs')).includes('Open · Atreides ahead'));
  assert.deepEqual(ui.posted, [], 'the board only reads the save');
  assert.equal(ui.doc.querySelector('#lsb-shown').textContent, '4 of 4 houses shown');
});

test('the counts add up to the houses and the three cards, the bar and the Show buttons filter the list', async () => {
  const ui = await open();
  const n = (cls) => +ui.doc.querySelector('.glance .tile.' + cls + ' .n').textContent;
  assert.deepEqual([n('done'), n('part'), n('new')], [1, 2, 1]);
  click(ui, '.glance .tile.done');
  assert.deepEqual(visible(ui), ['Wayku']);
  assert.equal(ui.doc.querySelector('#lsb-shown').textContent, '1 of 4 houses shown');
  assert.equal(ui.doc.querySelector('.glance .tile.done').getAttribute('aria-current'), 'true');
  click(ui, '.stack a.u');
  assert.deepEqual(visible(ui), ['Alexin']);
  click(ui, '.seg [data-show="left"]');
  assert.deepEqual(visible(ui), ['Alexin', 'Hurata', 'Thorvald']);
  click(ui, '.back a');
  assert.equal(visible(ui).length, 4);
  // the house filter works with the Show filter
  const taken = ui.doc.querySelector('[data-lb=hs][value=taken]'); taken.checked = true; taken.dispatchEvent(new ui.w.Event('change', { bubbles: true }));
  assert.deepEqual(visible(ui), ['Hurata', 'Wayku']);
  click(ui, '.seg [data-show="part"]');
  assert.deepEqual(visible(ui), ['Hurata']);
  click(ui, '[data-lb=showall]');
  assert.equal(visible(ui).length, 4);
});

test('every heading sorts, a second click reverses, the name breaks ties, and a house with no place yet sorts last', async () => {
  const ui = await open(board({ tasks: board().tasks.map((t) => (t.id === 901 ? { ...t, location: { known: true, map: 'Hagga Basin', area: 'Hagga Basin South', place: 'rift', x: 0.2, y: 0.3 } } : t)) }));
  const order = () => rows(ui).map((r) => r.querySelector('.hn b').textContent);
  click(ui, '[data-lb=sort][data-col=tiers]');
  assert.deepEqual(order(), ['Alexin', 'Hurata', 'Thorvald', 'Wayku'], 'tiers ascending: 0, 1, 3, 5');
  click(ui, '[data-lb=sort][data-col=tiers]');
  assert.deepEqual(order(), ['Wayku', 'Thorvald', 'Hurata', 'Alexin']);
  assert.equal(ui.doc.querySelector('[data-col=tiers]').getAttribute('aria-sort'), 'descending');
  assert.equal(ui.doc.querySelector('[data-col=house]').getAttribute('aria-sort'), 'none');
  click(ui, '[data-lb=sort][data-col=location]');
  assert.equal(order()[0], 'Thorvald', 'the only house with a place comes first; the others follow by name');
  assert.deepEqual(order().slice(1), ['Alexin', 'Hurata', 'Wayku']);
  click(ui, '[data-lb=sort][data-col=state]');
  assert.equal(order()[0], 'Alexin');
  click(ui, '[data-lb=sort][data-col=next]');
  assert.equal(order()[0], 'Thorvald', 'the nearest next tier first (100 more)');
  click(ui, '[data-lb=sort][data-col=status]');
  assert.equal(order()[0], 'Wayku', 'taken by your faction first');
  click(ui, '[data-lb=sort][data-col=house]');
  assert.deepEqual(order(), ['Alexin', 'Hurata', 'Thorvald', 'Wayku']);
});

test('Jump to house shows only that house, opened, and Show all houses brings the list back', async () => {
  const ui = await open();
  const j = ui.doc.querySelector('#lsb-jump');
  assert.deepEqual([...j.options].map((o) => o.textContent), ['Choose a house', 'Alexin', 'Hurata', 'Thorvald', 'Wayku']);
  j.value = '901'; j.dispatchEvent(new ui.w.Event('change', { bubbles: true }));
  assert.deepEqual(visible(ui), ['Thorvald']);
  assert.equal(ui.doc.querySelector('#lsx-901').checked, true);
  assert.equal(ui.doc.querySelector('#lsb-only').hidden, false);
  assert.equal(ui.doc.querySelector('#lsb-onlyname').textContent, 'Thorvald');
  click(ui, '#lsb-only [data-lb=showall]');
  assert.equal(visible(ui).length, 4);
  assert.equal(ui.doc.querySelector('#lsb-only').hidden, true);
});

test('Expand all and Collapse all open and close every row, and a row keeps its state when the list is reordered', async () => {
  const ui = await open();
  click(ui, '[data-lb=expand]');
  assert.ok([...ui.doc.querySelectorAll('.xc')].every((x) => x.checked));
  assert.equal(ui.doc.querySelector('#lsb').hasAttribute('data-all'), true);
  click(ui, '[data-lb=collapse]');
  assert.ok([...ui.doc.querySelectorAll('.xc')].every((x) => !x.checked));
  ui.doc.querySelector('#lsx-902').checked = true; ui.doc.querySelector('#lsx-902').dispatchEvent(new ui.w.Event('change', { bubbles: true }));
  click(ui, '[data-lb=sort][data-col=tiers]');
  assert.equal(ui.doc.querySelector('#lsx-902').checked, true);
});

test('a tier says what it gives, by the game’s name, and where it stands: not reached, waiting, delivered, or reached with no delivery recorded', async () => {
  const ui = await open();
  const wayku = ui.doc.querySelector('#lsrow-903');
  const t = [...wayku.querySelectorAll('.tiers .tc')];
  assert.equal(t.length, 5);
  assert.deepEqual(t.map((x) => text(x.querySelector('.pl'))), ['Collected', 'Collected', 'Not collected', 'Not collected', 'Not collected']);
  assert.ok(text(t[0].querySelector('.rw')).startsWith('10,000 '), text(t[0].querySelector('.rw')));
  assert.ok(t[0].title.includes('Game id: SolarisCoin'), 'the internal id is only in the tooltip');
  assert.ok(!text(t[0]).includes('SolarisCoin'));
  assert.ok(t[2].classList.contains('wt') && t[0].classList.contains('dv'));
  assert.ok(text(t[4].querySelector('.rw')).includes('Placeables Swatch'), 'the top tier shows the swatch while it is not held');
  assert.ok(wayku.querySelector('.swl').textContent.includes('Swatch not held yet'));
  const thor = [...ui.doc.querySelectorAll('#lsrow-901 .tiers .tc .pl')].map(text);
  assert.deepEqual(thor, ['Reached, no delivery recorded', 'Reached, no delivery recorded', 'Reached, no delivery recorded', 'Not reached', 'Not reached']);
  assert.ok(t[0].getAttribute('aria-label').includes('Tier 1, 1,200, reward 10,000'), t[0].getAttribute('aria-label'));
  // once the swatch is held the top tier is the task's own item
  const held = board({ tasks: [house(900, 'Alexin', 6500, { swatch_held: true }).rewards ? { ...house(900, 'Alexin', 6500, { swatch_held: true }), rewards: house(900, 'Alexin', 6500).rewards.map((r, i) => (i === 4 ? { ...r, shown_template_id: 'T6DiamodineBladeParts', shown_amount: 75, swatch: false } : r)) } : null] });
  const ui2 = await open(held);
  assert.ok(text(ui2.doc.querySelector('#lsrow-900 .tiers .tc.top .rw')).startsWith('75 '), text(ui2.doc.querySelector('#lsrow-900 .tiers .tc.top .rw')));
  assert.ok(ui2.doc.querySelector('#lsrow-900 .swl').textContent.includes('Swatch held'));
});

test('the place: a house with no position says so and does not open anything; one with a position shows the area and the pin in its opened row', async () => {
  const known = { known: true, map: 'Hagga Basin', map_key: 'HaggaBasin', area: 'The O’dham', place: 'north rim', x: 0.25, y: 0.5 };
  const ui = await open(board({ tasks: [house(900, 'Alexin', 0), house(901, 'Thorvald', 100, { location: known })] }));
  assert.ok(text(ui.doc.querySelector('#lsrow-900 .c-loc')).includes('not mapped yet'));
  assert.ok(text(ui.doc.querySelector('#lsrow-901 .c-loc')).includes('The O’dham') && text(ui.doc.querySelector('#lsrow-901 .c-loc')).includes('Hagga Basin'));
  const calls = spyMap(ui);
  click(ui, '#lsrow-900 [data-lb=map]');
  assert.equal(ui.doc.querySelector('dialog'), null, 'no window is opened');
  assert.ok(text(ui.doc.querySelector('#lsb-result')).includes('no map marker'), 'the status line says why');
  assert.deepEqual(calls, [], 'nothing is opened on the Saved map');
  assert.ok(ui.doc.querySelector('#lsrow-900 .more').textContent.includes('This house is not mapped yet'), 'the opened row explains it too');
  assert.equal(ui.doc.querySelector('#lsrow-900 .more .pin'), null, 'no position is drawn that is not known');
  assert.equal(ui.doc.querySelector('#lsrow-901 .more .minimap .pin').style.top, '50%');
  assert.equal(ui.doc.querySelector('#lsrow-901 .more .minimap .pin').style.left, '25%');
  assert.ok(text(ui.doc.querySelector('#lsrow-901 .more')).includes('north rim'));
});

test('the house name card names the next reward, what waits, the faction race and the place, and opens on a click', async () => {
  const ui = await open();
  const hw = ui.doc.querySelector('#lsrow-903 .hw');
  const card = text(hw.querySelector('.hcard'));
  assert.ok(card.includes('Not collected') && card.includes('Faction race') && card.includes('Atreides 4,000 of 4,000') && card.includes('Finished by Atreides'), card);
  assert.ok(text(hw.querySelector('.hcard')).includes('top tier reached'));
  click(ui, '#lsrow-903 .hn');
  assert.ok(hw.classList.contains('open'));
  assert.equal(hw.querySelector('.hn').getAttribute('aria-expanded'), 'true');
  click(ui, '#lsrow-903 .hcl');
  assert.ok(!hw.classList.contains('open'));
});

test('the house name card lists each waiting reward on its own line, and says so in words when nothing waits (#666)', async () => {
  const data = board({ tasks: [
    house(903, 'Wayku', 6500, {}, { 1200: 'delivered', 2400: 'delivered', 3600: 'waiting', 4800: 'waiting', 6000: 'waiting' }),
    house(900, 'Alexin', 0),
    house(901, 'Thorvald', 4700, {}, { 1200: 'delivered', 2400: 'delivered', 3600: 'waiting' }),
  ] });
  const ui = await open(data);
  const waitingOf = (id) => {
    const dts = [...ui.doc.querySelectorAll('#lsrow-' + id + ' .hcard dt')];
    return dts.find((d) => text(d) === 'Not collected').nextElementSibling;
  };
  const dd = waitingOf(903);
  const lines = [...dd.querySelectorAll('.hrw')];
  assert.deepEqual(lines.map((l) => text(l)), ['50 Precision Range Finder', '1 Improved Holtzman Actuator', '1 House Alexin Placeables Swatch'], 'one line per waiting reward, in tier order');
  assert.equal(dd.children.length, 3, 'nothing but the reward lines inside');
  assert.ok(!dd.innerHTML.includes(', '), 'no comma-joined run is left');
  const css = fs.readFileSync(new URL('../internal/web/static/app.css', import.meta.url), 'utf8');
  assert.ok(css.includes('.lsb .hcard dd .hrw{display:block}'), 'the stylesheet draws each reward as its own block');
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
  const term = ui.doc.querySelector('.hist details.term');
  assert.ok(text(term.querySelector('summary')).startsWith('Term 6') && text(term.querySelector('summary')).includes('2 houses with your contribution'), text(term.querySelector('summary')));
  assert.ok(text(term.querySelector('summary')).includes('1 top tier') && text(term.querySelector('summary')).includes('1 taken by your faction'));
  assert.deepEqual([...term.querySelectorAll('tbody tr')].map((r) => text(r.cells[0])), ['Maros', 'Kenola']);
  assert.ok(text(term).includes('Open at term end') && text(term).includes('Play clock 0 to 36,000 s (10 h of play time)'));
  const one = await open(board());
  assert.ok(text(one.doc.querySelector('.hist')).includes('only the current term'));
});

test('house names from the save are text, never markup', async () => {
  const ui = await open(board({ tasks: [house(900, '<img src=x onerror=alert(1)>', 0)] }));
  assert.equal(ui.doc.querySelector('#lsb img'), null);
  assert.ok(text(ui.doc.querySelector('#lsrow-900 .hn b')).includes('<img'));
});

test('the opened part of a row is hidden until the row is opened, and follows the box', async () => {
  const ui = await open();
  const more = (id) => ui.doc.querySelector(`#lsrow-${id} .more`);
  assert.ok(more(900).hidden && more(901).hidden);
  const x = ui.doc.querySelector('#lsx-901'); x.checked = true; x.dispatchEvent(new ui.w.Event('change', { bubbles: true }));
  assert.equal(more(901).hidden, false);
  assert.ok(more(900).hidden);
  click(ui, '[data-lb=expand]');
  assert.ok(!more(900).hidden && !more(903).hidden);
  click(ui, '[data-lb=collapse]');
  assert.ok(more(900).hidden && more(901).hidden);
});

test('the longer explanations sit in closed About disclosures', async () => {
  const ui = await open();
  const abouts = [...ui.doc.querySelectorAll('.lsb details.about')];
  assert.ok(abouts.length >= 3);
  assert.ok(abouts.every((d) => !d.open && /^About /.test(text(d.querySelector('summary')))));
});

test('the place has a quick peek for a mouse (the same lines as the opened row) only where the position is known', async () => {
  const known = { known: true, map: 'Hagga Basin', map_key: 'HaggaBasin', area: 'The O’dham', place: 'north rim', x: 0.25, y: 0.5 };
  const ui = await open(board({ tasks: [house(900, 'Alexin', 0), house(901, 'Thorvald', 100, { location: known })] }));
  assert.equal(ui.doc.querySelector('#lsrow-900 .loc .pop'), null, 'no peek for a house with no position');
  const pop = ui.doc.querySelector('#lsrow-901 .loc > .pop');
  assert.ok(pop, 'the peek is in the place');
  assert.equal(pop.getAttribute('aria-hidden'), 'true', 'the peek is mouse only; the opened row is the way in for the keyboard and touch');
  assert.ok(text(pop).includes('The O’dham (Hagga Basin)') && text(pop).includes('north rim') && text(pop).includes('Click the place to open the Saved map'), text(pop));
  assert.equal(pop.querySelector('.pin').style.left, '25%');
  assert.ok(ui.doc.querySelector('#lsrow-901 [data-lb=map]').title.includes('Hover for a quick peek'));
});

test('a save with no houses in the term says so in place of the board controls', async () => {
  const ui = await open(board({ tasks: [] }));
  const b = ui.doc.querySelector('#lsb-board');
  assert.ok(b.querySelector('.nodata') && text(b).includes('no Landsraad houses'), text(b));
  assert.equal(b.querySelectorAll('button, select, input').length, 0, 'no control is offered against an empty board');
});

test('the stylesheet styles the status lines by the ids the board draws, and keeps the term facts apart from the Player tab\'s .kv cards', async () => {
  const fs = await import('node:fs');
  const css = fs.readFileSync(new URL('../internal/web/static/app.css', import.meta.url), 'utf8');
  const js = fs.readFileSync(new URL('../internal/web/static/app.js', import.meta.url), 'utf8');
  for (const id of ['lsb-sum', 'lsb-result', 'lsb-tresult']) {
    assert.ok(js.includes('id="' + id + '"'), id + ' is drawn');
    assert.ok(css.includes('.lsb #' + id + '{'), id + ' is styled');
  }
  assert.ok(!/\.lsb \.kv\b/.test(css), 'the global .kv card (boxed, 11px values, words broken anywhere) must not meet the term facts');
});

test('when no house has a place yet the board says so once: no place in every row, Group by region and the Location sort are off with the reason', async () => {
  const ui = await open();
  assert.ok(text(ui.doc.querySelector('#lsb-nolocs')).includes('No house has a place'));
  assert.equal(ui.doc.querySelectorAll('#lsb-rows .loc-t').length, 0, 'no place button in any row');
  assert.equal(ui.doc.querySelectorAll('.lsb .more h4').length, rows(ui).length * 2, 'no Where block per opened row');
  const g = ui.doc.querySelector('[data-lb=group]');
  assert.ok(g.disabled && ui.doc.querySelector('#lsb-grp-hint'), 'Group by region is off with a hint');
  assert.equal(text(ui.doc.querySelector('#lsb-grp-hint')), 'Group by region is off: it needs a place for the houses.', 'the note is a full sentence (#670)');
  assert.ok(rows(ui).every((r) => text(r.querySelector('.c-loc')) === 'not mapped yet'), 'each Location cell says it, not a bare dash (#669), the same words as a single house without a place');
  assert.ok(ui.doc.querySelector('[data-col=location] .sorth').disabled);
  const known = { known: true, map: 'Hagga Basin', area: 'The O’dham', place: 'rim', x: 0.25, y: 0.5 };
  const ui2 = await open(board({ tasks: [house(900, 'Alexin', 0, { location: known }), house(901, 'Thorvald', 100)] }));
  assert.equal(ui2.doc.querySelector('#lsb-nolocs'), null);
  assert.ok(!ui2.doc.querySelector('[data-lb=group]').disabled);
  assert.ok(text(ui2.doc.querySelector('#lsrow-901 .c-loc')).includes('not mapped yet'), 'with some places known, a house without one still says so');
});

test('the peek and the opened row give the distance from you in plain words; a click on a place with a picture opens the Saved map there, with no window in between', async () => {
  const known = { known: true, map: 'Hagga Basin', map_key: 'HaggaBasin', area: 'The O’dham', place: 'north rim', x: 0.25, y: 0.5, from_you_cm: 281000 };
  const far = { known: true, map: 'Deep Desert', map_key: 'DeepDesert', area: 'The Sink', place: '', x: 0.5, y: 0.5 };
  const hub = { known: true, map: 'Arrakeen', map_key: '', area: 'Arrakeen', place: '', x: 0.5, y: 0.5 };
  const ui = await open(board({ tasks: [house(900, 'Alexin', 0, { location: known }), house(901, 'Thorvald', 100, { location: far }), house(902, 'Hurata', 0, { location: hub })] }));
  assert.ok(text(ui.doc.querySelector('#lsrow-900 .loc > .pop')).includes('From youabout 2.8 km'), text(ui.doc.querySelector('#lsrow-900 .loc > .pop')));
  assert.ok(text(ui.doc.querySelector('#lsrow-900 .more')).includes('From youabout 2.8 km'));
  const far1 = text(ui.doc.querySelector('#lsrow-901 .more'));
  assert.ok(far1.includes('From younot known (your character is on another map, or has no saved position)'), 'another map: no invented distance, in plain words: ' + far1);
  assert.ok(!/not worked out/.test(ui.doc.body.textContent), 'the old wording is gone');
  assert.ok(!text(ui.doc.querySelector('#lsrow-902 .more')).includes('From you'), 'a hub has no From you line');
  const calls = spyMap(ui);
  click(ui, '#lsrow-900 [data-lb=map]');
  await ui.settle();
  assert.deepEqual(calls, [['HaggaBasin', 0.25, 0.5, 'Alexin']], 'the Saved map is shown at the house, in one click');
  assert.equal(ui.doc.querySelector('dialog'), null, 'no window');
  assert.equal(ui.doc.querySelector('#lsb'), null, 'the Landsraad page is left');
  assert.equal(ui.doc.querySelector('[data-tab=livemap]').getAttribute('aria-current'), 'page', 'the Saved map tab is the open one');
});

test('a hub (a place the Saved map has no picture of) is muted, says why in its tooltip and name, and a click opens nothing', async () => {
  const hub = { known: true, map: 'Harko Village', map_key: '', area: 'Harko Village', place: '', cell: '', unconfirmed: false };
  const ui = await open(board({ tasks: [house(900, 'Alexin', 0, { location: hub })] }));
  const btn = ui.doc.querySelector('#lsrow-900 .loc-t');
  const why = 'The Saved map has no picture of Harko Village yet.';
  assert.ok(btn.classList.contains('off'), 'muted, not link-styled');
  assert.equal(btn.getAttribute('aria-disabled'), 'true');
  assert.equal(btn.title, why);
  assert.ok(btn.getAttribute('aria-label').startsWith('House Alexin: Harko Village') && btn.getAttribute('aria-label').includes(why), btn.getAttribute('aria-label'));
  assert.ok(text(ui.doc.querySelector('#lsrow-900 .pop')).includes(why) && !/open the Saved map/.test(text(ui.doc.querySelector('#lsrow-900 .pop'))));
  assert.ok(text(ui.doc.querySelector('#lsrow-900 .hcard')).includes(why) && !/open the Saved map/.test(text(ui.doc.querySelector('#lsrow-900 .hcard'))));
  const calls = spyMap(ui);
  click(ui, '#lsrow-900 [data-lb=map]');
  assert.deepEqual(calls, [], 'nothing is opened on the Saved map');
  assert.equal(ui.doc.querySelector('dialog'), null, 'no window');
  assert.ok(ui.doc.querySelector('#lsb'), 'the Landsraad page stays');
  assert.equal(text(ui.doc.querySelector('#lsb-result')), why, 'the status line says it');
  assert.ok(ui.doc.querySelector('#lsb-result').classList.contains('warn'), 'in the warning colour, not the green of a done write');
  assert.equal(text(ui.doc.querySelector('#lsrow-900 .loc .lnote')), why, 'and the reason is shown next to the place, where the click was, not only at the top of the page');
  assert.ok(ui.doc.querySelector('#lsrow-900 .loc .lnote').getAttribute('role') === 'status');
  assert.ok(ui.doc.querySelector('#lsrow-900 .loc-t svg'), 'the pin icon stays on the dimmed place');
  assert.ok(text(ui.doc.querySelector('#lsrow-900 .loc-t .cue')) === 'no map picture', 'a cue in words, not only a colour');
  const more = text(ui.doc.querySelector('#lsrow-900 .more'));
  assert.ok(more.includes(why) && more.includes('no pin or map link') && !more.includes('From you'), more);
  assert.equal(ui.doc.querySelector('#lsrow-900 .more .pin'), null, 'no pin is drawn on a made-up picture');
});

test('the click on a place that cannot be opened shows its reason next to it, in the warning colour; so does a Saved map that is not ready', async () => {
  const known = { known: true, map: 'Hagga Basin', map_key: 'HaggaBasin', area: 'The O’dham', place: 'north rim', x: 0.25, y: 0.5 };
  const none = { known: false };
  const ui = await open(board({ tasks: [house(900, 'Alexin', 0, { location: none }), house(901, 'Thorvald', 100, { location: known })] }));
  assert.equal(text(ui.doc.querySelector('#lsrow-900 .loc .lnote')), '', 'nothing is said before a click');
  click(ui, '#lsrow-900 [data-lb=map]');
  assert.ok(text(ui.doc.querySelector('#lsrow-900 .loc .lnote')).includes('no map marker'));
  assert.ok(ui.doc.querySelector('#lsb-result').classList.contains('warn'));
  assert.equal(ui.doc.querySelector('#lsrow-901 .loc .lnote'), null, 'a place that opens has no note');
  ui.w.liveMapShow = undefined;
  click(ui, '#lsrow-901 [data-lb=map]');
  assert.equal(text(ui.doc.querySelector('#lsrow-901 .loc .lnote')), 'The Saved map is not ready yet.');
  assert.equal(text(ui.doc.querySelector('#lsb-result')), 'The Saved map is not ready yet.');
  assert.ok(ui.doc.querySelector('#lsb-result').classList.contains('warn'));
  assert.ok(ui.doc.querySelector('#lsb'), 'the page stays');
});

test('the About text of the board says what a click on a place does', async () => {
  const ui = await open();
  assert.ok(text(ui.doc.querySelector('.lsb')).includes('A click on a place opens the Saved map at the house'));
});

test('a place with a picture reads as a link to the Saved map: not muted, and its name and tooltip say it opens the Saved map', async () => {
  const known = { known: true, map: 'Hagga Basin', map_key: 'HaggaBasin', area: 'The O’dham', place: 'north rim', x: 0.25, y: 0.5 };
  const ui = await open(board({ tasks: [house(900, 'Alexin', 0, { location: known })] }));
  const btn = ui.doc.querySelector('#lsrow-900 .loc-t');
  assert.ok(!btn.classList.contains('off') && !btn.hasAttribute('aria-disabled'));
  assert.equal(btn.tagName, 'BUTTON', 'a real button: Enter and Space work, and it takes focus');
  assert.ok(btn.getAttribute('aria-label').endsWith('Open in the Saved map'), btn.getAttribute('aria-label'));
  assert.ok(btn.title.startsWith('Open The O’dham (Hagga Basin) in the Saved map'), btn.title);
  assert.ok(!btn.hasAttribute('aria-haspopup'), 'it opens no window');
});

test('the place button works from the keyboard: a button takes the Enter/Space click, with and without a picture', async () => {
  const known = { known: true, map: 'Deep Desert', map_key: 'DeepDesert', area: 'Deep Desert', place: '', cell: 'B8', x: 0.5, y: 0.75 };
  const hub = { known: true, map: 'Arrakeen', map_key: '', area: 'Arrakeen', place: '' };
  const ui = await open(board({ tasks: [house(900, 'Maros', 0, { location: known }), house(901, 'Hurata', 0, { location: hub }), house(902, 'Alexin', 0)] }));
  const calls = spyMap(ui);
  for (const id of [901, 902]) {
    const b = ui.doc.querySelector('#lsrow-' + id + ' .loc-t');
    b.focus();
    assert.equal(ui.doc.activeElement, b, 'the muted place still takes focus, so its reason can be read and heard');
    b.click(); // what Enter and Space do on a button
  }
  assert.deepEqual(calls, []);
  assert.equal(ui.doc.querySelector('dialog'), null);
  const b = ui.doc.querySelector('#lsrow-900 .loc-t');
  b.focus();
  b.click();
  await ui.settle();
  assert.deepEqual(calls, [['DeepDesert', 0.5, 0.75, 'Maros']]);
});

test('a Deep Desert row names its grid cell; Tseida-like places say they are from public guides', async () => {
  const dd = { known: true, map: 'Deep Desert', map_key: 'DeepDesert', area: 'Deep Desert', place: '', cell: 'B8', unconfirmed: false, x: 0.5, y: 0.5 };
  const tr = { known: true, map: 'Hagga Basin', map_key: 'HaggaBasin', area: 'Eastern Vermillius Gap', place: 'The Anvil Tradepost', cell: '', unconfirmed: true, x: 0.5, y: 0.5 };
  const ui = await open(board({ tasks: [house(900, 'Maros', 0, { location: dd }), house(901, 'Tseida', 0, { location: tr })] }));
  assert.ok(text(ui.doc.querySelector('#lsrow-900 .c-loc .lm')).includes('Grid cell B8'), 'the cell is in the row');
  assert.ok(text(ui.doc.querySelector('#lsrow-901 .c-loc .lm')).includes('The Anvil Tradepost'), 'the place is in the row');
  assert.ok(text(ui.doc.querySelector('#lsrow-900 .more')).includes('Grid cell') && text(ui.doc.querySelector('#lsrow-900 .more')).includes('B8'));
  assert.ok(!text(ui.doc.querySelector('#lsrow-900 .more')).includes('public guides'), 'a confirmed place has no hedge');
  assert.ok(text(ui.doc.querySelector('#lsrow-901 .more')).includes('from public guides and is not confirmed in the game'));
  assert.ok(text(ui.doc.querySelector('#lsrow-901 .pop')).includes('not confirmed in the game'), 'the peek says it too');
});

test('Group by region draws one heading per region with the number of houses in it', async () => {
  const mk = (map, key) => ({ known: true, map, map_key: key, area: map, place: '', cell: '', unconfirmed: false, x: 0.5, y: 0.5 });
  const ui = await open(board({ tasks: [house(900, 'Alexin', 0, { location: mk('Harko Village', '') }), house(901, 'Thorvald', 0, { location: mk('Hagga Basin', 'HaggaBasin') }), house(902, 'Hurata', 0, { location: mk('Hagga Basin', 'HaggaBasin') })] }));
  assert.equal(ui.doc.querySelectorAll('#lsb-rows li.grp').length, 0, 'no headings until asked');
  click(ui, '[data-lb=group]');
  const heads = [...ui.doc.querySelectorAll('#lsb-rows li.grp h4')].map((h) => text(h));
  assert.deepEqual(heads, ['Hagga Basin 2', 'Harko Village 1']);
  click(ui, '[data-lb=group]');
  assert.equal(ui.doc.querySelectorAll('#lsb-rows li.grp').length, 0, 'the headings go when grouping is off');
});

test('the glance line says what its numbers count: houses taken by each faction, your faction named, then the houses still open (#665)', async () => {
  const ui = await open();
  const facts = [...ui.doc.querySelectorAll('.lsb .facts > span')].map((e) => text(e));
  assert.ok(facts[0].startsWith('Houses taken by each faction: Atreides 1 (your faction) · Harkonnen 1 · 2 still open'), facts[0]);
  assert.ok(!facts[0].includes('By faction, not by'), 'no contrast against something not yet seen');
  assert.ok(!facts[0].includes('(you)'), 'no bare (you)');
  const items = [...ui.doc.querySelectorAll('.lsb .facts > span:first-child .fi')].map((e) => text(e));
  assert.deepEqual(items, ['Atreides 1 (your faction) ·', 'Harkonnen 1 ·', '2 still open'], 'each count stays whole when the line wraps');
  assert.ok(text(ui.doc.querySelector('#lsb-actbar, .actbar #lsb-fac')).includes('Atreides (your faction)'), 'the faction drop-down says it the same way');
});

test('the current-term glance cards come before the Term and decrees panel, then the board and the earlier terms (#668)', async () => {
  const ui = await open();
  const order = [...ui.doc.querySelectorAll('#lsb > section')].map((e) => e.getAttribute('aria-labelledby'));
  assert.deepEqual(order.slice(0, 4), ['lsb-glance', 'lsb-term-h', 'lsb-board', 'lsb-hist']);
});
