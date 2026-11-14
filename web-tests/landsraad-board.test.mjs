// The new Landsraad board (#599): one list of the houses, alphabetical, collapsed rows that open to the tiers with their rewards, the filters, the sort, the jump and the earlier terms.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
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
const rows = (ui) => [...ui.doc.querySelectorAll('#lsb-rows > li')];
const visible = (ui) => rows(ui).filter((r) => !r.hidden).map((r) => r.querySelector('.hn b').textContent);
const text = (el) => el.textContent.replace(/\s+/g, ' ').trim();
const click = (ui, sel) => { const el = ui.doc.querySelector(sel); assert.ok(el, 'no ' + sel); el.dispatchEvent(new ui.w.MouseEvent('click', { bubbles: true, cancelable: true })); };

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
  assert.deepEqual(t.map((x) => text(x.querySelector('.pl'))), ['Delivered', 'Delivered', 'Waiting to collect', 'Waiting to collect', 'Waiting to collect']);
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

test('the place: a house with no position says so and its map dialog explains; one with a position shows the area and the pin', async () => {
  const known = { known: true, map: 'Hagga Basin', area: 'The O’dham', place: 'north rim', x: 0.25, y: 0.5 };
  const ui = await open(board({ tasks: [house(900, 'Alexin', 0), house(901, 'Thorvald', 100, { location: known })] }));
  assert.ok(text(ui.doc.querySelector('#lsrow-900 .c-loc')).includes('not mapped yet'));
  assert.ok(text(ui.doc.querySelector('#lsrow-901 .c-loc')).includes('The O’dham') && text(ui.doc.querySelector('#lsrow-901 .c-loc')).includes('Hagga Basin'));
  click(ui, '#lsrow-900 [data-lb=map]');
  assert.ok(text(ui.doc.querySelector('#lsb-mapbody')).includes('not in the editor yet'));
  assert.equal(ui.doc.querySelector('#lsb-mapbody .pin'), null, 'no position is drawn that is not known');
  click(ui, '#lsrow-901 [data-lb=map]');
  assert.equal(ui.doc.querySelector('#lsb-mapbody .pin').style.left, '25%');
  assert.ok(text(ui.doc.querySelector('#lsb-mapbody')).includes('north rim'));
  assert.equal(ui.doc.querySelector('#lsrow-901 .more .minimap .pin').style.top, '50%');
});

test('the house name card names the next reward, what waits, the faction race and the place, and opens on a click', async () => {
  const ui = await open();
  const hw = ui.doc.querySelector('#lsrow-903 .hw');
  const card = text(hw.querySelector('.hcard'));
  assert.ok(card.includes('Waiting to collect') && card.includes('Faction race') && card.includes('Atreides 4,000 of 4,000') && card.includes('Finished by Atreides'), card);
  assert.ok(text(hw.querySelector('.hcard')).includes('top tier reached'));
  click(ui, '#lsrow-903 .hn');
  assert.ok(hw.classList.contains('open'));
  assert.equal(hw.querySelector('.hn').getAttribute('aria-expanded'), 'true');
  click(ui, '#lsrow-903 .hcl');
  assert.ok(!hw.classList.contains('open'));
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

test('the place has a quick peek for a mouse (the lines of the dialog) only where the position is known', async () => {
  const known = { known: true, map: 'Hagga Basin', area: 'The O’dham', place: 'north rim', x: 0.25, y: 0.5 };
  const ui = await open(board({ tasks: [house(900, 'Alexin', 0), house(901, 'Thorvald', 100, { location: known })] }));
  assert.equal(ui.doc.querySelector('#lsrow-900 .loc .pop'), null, 'no peek for a house with no position');
  const pop = ui.doc.querySelector('#lsrow-901 .loc > .pop');
  assert.ok(pop, 'the peek is in the place');
  assert.equal(pop.getAttribute('aria-hidden'), 'true', 'the peek is mouse only; the dialog is the way in for the keyboard and touch');
  assert.ok(text(pop).includes('The O’dham (Hagga Basin)') && text(pop).includes('north rim') && text(pop).includes('Click the place for the full map'), text(pop));
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
  for (const id of ['lsb-sum', 'lsb-result', 'lsb-pendbar', 'lsb-tresult']) {
    assert.ok(js.includes('id="' + id + '"'), id + ' is drawn');
    assert.ok(css.includes('.lsb #' + id + '{'), id + ' is styled');
  }
  assert.ok(!/\.lsb \.kv\b/.test(css), 'the global .kv card (boxed, 11px values, words broken anywhere) must not meet the term facts');
});
