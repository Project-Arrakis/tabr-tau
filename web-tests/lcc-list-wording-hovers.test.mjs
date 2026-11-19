// The Command Center list pass (tau #952, #950, #953, #951, #940, #937), in the page's own code (jsdom), with the synthetic answer of M1 and no game save.
// #952: a faction wins a House's favor (it does not own or claim it); a House's place is its square; its track is its specialization. The words are written once, in lcc-model.js (WORDS).
// #950: the favor cell of the list shows ONE mark. #953: the list opens sorted by the House name, A to Z. #951: the list has the Classic mouse-overs. #940: the Place line of the House card is a link
// that opens the Saved map. #937: every reward of the inspector has a mouse-over with its item, except Solari.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { boot, closeAll } from './boot.mjs';
import { synthetic } from './lcc-fixture.mjs';
import { root as repoRoot } from './lcc-load.mjs';

afterEach(closeAll);

const text = (el) => el.textContent.replace(/\s+/g, ' ').trim();
const click = (ui, el) => { if (typeof el === 'string') el = ui.doc.querySelector(el); assert.ok(el, 'no element'); el.focus(); el.dispatchEvent(new ui.w.MouseEvent('click', { bubbles: true, cancelable: true })); };
const idOf = (a, name) => a.tasks.find((t) => t.house_name === 'DA_House' + name).id;
async function open(layout = 'list', view = 'board') {
  const ui = await boot({ get: { '/api/landsraad': synthetic() }, post: {}, local: { tab: 'landsraad', 'lsb.view': view, 'lsb.layout': layout } });
  await ui.settle();
  return ui;
}
const rowOf = (ui, name) => [...ui.doc.querySelectorAll('.lcc-bd-row')].find((r) => text(r.querySelector('.lcc-bd-namebtn')) === name);
const look = (ui, a, name) => click(ui, '[data-fk="ht:' + idOf(a, name) + '"]');
const src = (...p) => fs.readFileSync(path.join(repoRoot, 'internal', 'web', 'static', ...p), 'utf8');

// every word a person reads or hears in a part of the page: its text, and the title and aria-label of every element in it
function said(node) {
  const out = [text(node)];
  for (const e of node.querySelectorAll('*')) for (const k of ['title', 'aria-label', 'data-info', 'data-why']) if (e.getAttribute(k)) out.push(e.getAttribute(k));
  return out.join(' | ');
}

// ---- #952, #950 --------------------------------------------------------------------------------------------------------------------------------------------------------
test('#952: the list header says Square, House, Favor won by, Specialization', async () => {
  const ui = await open();
  const heads = [...ui.doc.querySelectorAll('.lcc-bd-list thead th')].map((h) => text(h)).filter((h) => h !== 'Choose');
  assert.deepEqual(heads.slice(0, 4), ['Square', 'House', 'Favor won by', 'Specialization']);
});

test('#950: the favor cell shows ONE mark: the faction name alone, or the open circle and Not won, never a letter and the name', async () => {
  const a = synthetic();
  const ui = await open();
  const pills = [...ui.doc.querySelectorAll('.lcc-bd-row .lcc-bd-c-owner .lcc-owner')];
  assert.equal(pills.length, 25);
  const seen = new Set(pills.map((p) => text(p)));
  assert.deepEqual([...seen].sort(), ['Atreides', 'Harkonnen', '○ Not won'].sort());
  for (const p of pills) assert.doesNotMatch(text(p), /^[AH] /, 'no letter before the name: it read as a second H over HARKONNEN');
  assert.equal(text(rowOf(ui, 'Vernius').querySelector('.lcc-owner')), 'Harkonnen', 'Vernius: favor won by Harkonnen');
  assert.equal(text(rowOf(ui, 'Richese').querySelector('.lcc-owner')), 'Atreides');
  assert.equal(text(rowOf(ui, 'Kenola').querySelector('.lcc-owner')), '○ Not won');
  assert.match(src('lcc', 'lcc-board.css'), /\.lcc-bd-c-owner \.lcc-owner \{ white-space: nowrap; \}/, 'one line, so a narrow cell cannot stack a letter over the name');
  void a;
});

test('#952: the board and the inspector never say owner, claimed or taken, and never call a track a path or a square a number', async () => {
  const a = synthetic();
  for (const layout of ['grid', 'list']) {
    const ui = await open(layout);
    look(ui, a, 'Vernius'); // favor won by Harkonnen, with a ladder and none of the character's
    const all = said(ui.doc.querySelector('.lcc-bd'));
    assert.doesNotMatch(all, /\b(owner|owners|owned|claim|claims|claimed|unclaimed)\b/i, layout + ': ' + (all.match(/.{30}\b(owner|owned|claim|claimed)\b.{30}/i) || [''])[0]);
    const around = (word) => { const at = all.toLowerCase().indexOf(word); return at < 0 ? '' : all.slice(Math.max(0, at - 40), at + word.length + 40); };
    // "taken" is still in the page's own sentences that the Classic list shares (a result line such as "house taken by Atreides", the tooltip of Complete, the earlier terms): the recorded
    // results are golden files and stay as they are. Everything the board draws itself says favor won.
    const own = [...ui.doc.querySelectorAll('.lcc-hc, .lcc-bd-list, .lcc-bd-headlines, .lcc-bd-banner, .lcc-bd-ledger, .lcc-bd-victory')].map(said).join(' | ');
    assert.doesNotMatch(own, /\btaken\b/i, layout + ': ' + (own.match(/.{0,40}\btaken\b.{0,40}/i) || [''])[0]);
    assert.doesNotMatch(all, /\bpath\b/i, layout + ': ' + around('path'));
    assert.doesNotMatch(all, /\bNo\. ?\d/, layout);
    assert.match(all, /favor won/i, layout);
    ui.doc.querySelector('.lcc-bd-insp');
    look(ui, a, 'Kenola'); // favor not won
    assert.match(text(ui.doc.querySelector('.lcc-bd-banner')), /FAVOR NOT WON/);
    look(ui, a, 'Vernius');
    assert.match(text(ui.doc.querySelector('.lcc-bd-banner')), /FAVOR WON · HARKONNEN/);
    assert.match(ui.doc.querySelector('.lcc-bd-control').getAttribute('title'), /^Favor won by Harkonnen/);
  }
});

test('#952: the words are written once, in the wording table of lcc-model.js, and the Command Center files carry no copy of them', async () => {
  const ui = await open();
  const W = ui.w.LCCM.WORDS;
  assert.deepEqual([W.wonBy, W.square, W.specialization, W.notWon, W.favorWon, W.favorNotWon], ['Favor won by', 'Square', 'Specialization', 'Not won', 'Favor won', 'Favor not won']);
  assert.ok(Object.isFrozen(W));
  assert.equal(JSON.stringify(ui.w.LCCM.HOUSE_FILTERS.map((f) => f[1])), JSON.stringify(['Any', 'Not won', 'Favor won']));
  for (const f of ['lcc-board.js', 'lcc.js']) {
    const s = src('lcc', f);
    for (const old of ["'Owner'", "'Path'", "'No.'", "'Claimed'", "'CLAIMED", "NOT CLAIMED", "'By path'", "'No path'", "Taken - no"]) assert.ok(!s.includes(old), f + ' still has ' + old);
  }
});

// ---- #953 ---------------------------------------------------------------------------------------------------------------------------------------------------------------
test('#953: the list opens sorted by the House name, A to Z, and says so; a click on Square sorts by the square', async () => {
  const ui = await open();
  const names = () => [...ui.doc.querySelectorAll('.lcc-bd-row .lcc-bd-namebtn')].map((b) => text(b));
  assert.deepEqual(names(), [...names()].sort((x, y) => x.localeCompare(y)));
  assert.match(text(ui.doc.querySelector('.lcc-bd-list caption')), /by default by House name, A to Z/);
  assert.equal(text(ui.doc.querySelector('.lcc-bd-list th[aria-sort="ascending"]')), 'House ▲'.replace(' ▲', ''));
  click(ui, '[data-fk="bs:position"]');
  assert.deepEqual([...ui.doc.querySelectorAll('.lcc-bd-row .lcc-bd-c-no')].map((b) => text(b))[0], '01');
});

// ---- #951, #940 --------------------------------------------------------------------------------------------------------------------------------------------------------
test('#951: the House name of a row carries the Classic hover card, and the cells carry the Classic mouse-over words', async () => {
  const ui = await open();
  const r = rowOf(ui, 'Alexin');
  const card = r.querySelector('.hw > .hcard');
  assert.ok(card, 'the card is in the name cell');
  const t = text(card);
  for (const w of ['House Alexin', 'Next reward', 'Not collected', 'Faction race', 'Place']) assert.ok(t.includes(w), w + ' in ' + t);
  assert.ok(r.querySelector('.hw > .lcc-bd-namebtn'), 'the name stays the button that looks at the House');
  assert.equal(r.querySelector('.hw .hn'), null);
  // the same card as the Classic list's, from one builder
  const c = await open('list', 'classic');
  assert.equal(text(c.doc.querySelector('#lsrow-' + idOf(synthetic(), 'Alexin') + ' .hcard')), t, 'the Classic card and the board card say the same');
  // the cells
  const tip = (cls) => { const n = r.querySelector('.' + cls); return n.getAttribute('title') || ui.w.LCCB.tipOf(n).title; }; // the reward cells carry the picture mouse-over (#977), which keeps the sentence
  assert.match(tip('lcc-bd-c-state'), /^(Your contribution: [\d,]+\. |No contribution of yours recorded\. )\d+ of 5 tiers earned.$/);
  assert.match(tip('lcc-bd-c-tiers'), /^Tier 1, [\d,]+: .+ \((not reached|earned, .+)\)(\nTier \d, .+)+$/);
  assert.match(tip('lcc-bd-c-next'), /^Next reward: |top tier reached/);
  assert.match(tip('lcc-bd-c-wait'), /^(Not collected: .+|Nothing waiting at the house representative\.)$/);
  assert.match(tip('lcc-bd-c-wait'), /^Not collected: /, 'Alexin has a reward waiting in the fixture');
});

test('#951: the Location cell is the Classic place link with the quick peek (the map picture) in the mouse-over, and the peek is for the mouse only', async () => {
  const a = synthetic();
  const ui = await open();
  const r = rowOf(ui, 'Kenola');
  const loc = r.querySelector('.lcc-bd-loc .loc');
  assert.ok(loc);
  const pop = loc.querySelector(':scope > .pop');
  assert.ok(pop, 'the peek');
  assert.equal(pop.getAttribute('aria-hidden'), 'true');
  assert.ok(pop.querySelector('.minimap'), 'the map picture, in the mouse-over only');
  assert.match(text(pop), /Click the place to open the Saved map/);
  assert.match(loc.querySelector('[data-lb="map"]').getAttribute('title'), /in the Saved map\. Hover for a quick peek\.$/);
  assert.equal(rowOf(ui, 'Thorvald').querySelector('.lcc-bd-loc .pop'), null, 'no peek for a House that is not mapped');
  // the picture is in no panel: the inspector carries the peek only in the mouse-over, never as a mini-map of its own
  look(ui, a, 'Kenola');
  const insp = ui.doc.querySelector('.lcc-bd-insp');
  assert.equal(insp.querySelectorAll('.minimap').length, 1);
  assert.ok(insp.querySelector('.loc > .pop .minimap'), 'the one picture is the one in the mouse-over');
  assert.equal(insp.querySelector('.loc > .pop').getAttribute('aria-hidden'), 'true');
});

test('#940: the Place line of the House card is a link that opens the Saved map, in the Classic list and in the Command Center list, and the card stays while the focus is on it', async () => {
  const a = synthetic();
  for (const [layout, view, sel] of [['list', 'board', '.lcc-bd-row'], ['list', 'classic', '.hr']]) {
    const ui = await open(layout, view);
    const calls = []; const real = ui.w.liveMapShow; ui.w.liveMapShow = (...x) => { calls.push(x); real(...x); };
    const r = [...ui.doc.querySelectorAll(sel)].find((x) => /Kenola/.test(text(x)));
    const place = r.querySelector('.hcard .hcard-place');
    assert.ok(place, view + ': the Place line is a button');
    assert.equal(place.tagName, 'BUTTON');
    assert.equal(place.getAttribute('data-lb'), 'map', 'the same handler as the place link of the row');
    assert.match(place.getAttribute('title'), /^Click the place to open the Saved map\.$/);
    assert.match(place.getAttribute('aria-label'), /Open in the Saved map$/);
    assert.ok(place.closest('.loc').querySelector(':scope > .pop .minimap'), 'with the Classic peek (the map picture) under it');
    click(ui, place);
    assert.deepEqual(calls.map((c) => [c[0], c[1], c[2], c[3]]), [['HaggaBasin', 0.25, 0.5, 'Kenola']], view);
  }
  // a place the Saved map has no picture of says why, as the row's link does
  const ui = await open('list', 'board');
  const hub = rowOf(ui, 'Alexin').querySelector('.hcard .hcard-place');
  assert.equal(hub.getAttribute('aria-disabled'), 'true');
  assert.equal(hub.getAttribute('title'), 'The Saved map has no picture of Arrakeen yet.');
  // not mapped: plain words, no link
  assert.equal(rowOf(ui, 'Thorvald').querySelector('.hcard .hcard-place'), null);
  assert.match(text(rowOf(ui, 'Thorvald').querySelector('.hcard')), /Placenot mapped yet/);
  void a;
});

test('#940: the card stays open while the pointer or the keyboard focus is on anything inside it, and Escape closes it', async () => {
  const css = src('app.css');
  assert.match(css, /\.lsb \.hw:hover>\.hcard/, 'open on the pointer, anywhere on the card (it is inside the wrapper)');
  assert.match(css, /\.lsb \.hw:has\(:focus-visible\)>\.hcard/, 'open while the focus is on the name or on the place link inside');
  assert.match(css, /\.lsb \.hw\[data-esc\]>\.hcard\{display:none!important\}/);
  const ui = await open();
  const r = rowOf(ui, 'Kenola');
  const hw = r.querySelector('.hw');
  const place = hw.querySelector('.hcard-place');
  place.focus();
  place.dispatchEvent(new ui.w.KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
  assert.equal(hw.hasAttribute('data-esc'), true, 'Escape closes the card');
  assert.ok(ui.doc.activeElement === hw.querySelector('.lcc-bd-namebtn'), 'the focus returns to the row');
  // the card is placed next to its name by the page, on the pointer and on the focus
  const name = hw.querySelector('.lcc-bd-namebtn');
  name.dispatchEvent(new ui.w.MouseEvent('mouseover', { bubbles: true }));
  assert.ok(hw.style.getPropertyValue('--fx') !== '', 'the card is placed');
});

// ---- #937 ---------------------------------------------------------------------------------------------------------------------------------------------------------------
test('#937: each reward of the inspector has the item in its mouse-over and on focus, in the Classic words, and Solari has none', async () => {
  const a = synthetic();
  const ui = await open('grid');
  look(ui, a, 'Alexin');
  const tiers = [...ui.doc.querySelectorAll('.lcc-bd-insp .lcc-bd-tier')];
  assert.equal(tiers.length, 5);
  assert.equal(tiers[0].hasAttribute('data-rtip'), false, 'tier 1 of the fixture is Solari: no reward mouse-over');
  assert.equal(ui.w.LCCB.tipOf(tiers[0]), null);
  assert.doesNotMatch(tiers[0].getAttribute('aria-label'), /Game id/);
  const t2 = tiers[1];
  const d2 = ui.w.LCCB.tipOf(t2);
  assert.ok(d2, 'tier 2 has the reward mouse-over');
  assert.equal(d2.rows.length, 1);
  assert.equal(d2.rows[0].item, 'SYNTH_SCHEMATIC_1', 'the item id only finds the picture');
  assert.equal(t2.getAttribute('data-item'), 'SYNTH_SCHEMATIC_1', 'and is kept as a data attribute only');
  assert.match(d2.rows[0].name, /\S/, 'the display name');
  assert.doesNotMatch(d2.rows[0].name, /SYNTH_SCHEMATIC/, 'the internal name is not the display name');
  assert.doesNotMatch(t2.getAttribute('aria-label'), /Game id/, 'no "Game id" in the words a screen reader says (#977; an item the editor has no name for is called by its id, as everywhere on the page)');
  assert.equal(t2.getAttribute('data-info'), null, 'the text drawn by the style sheet is gone');
  assert.doesNotMatch(src('lcc', 'lcc-board.css'), /data-info/);
  // the click sentence rides in the mouse-over card, no longer in a native title
  assert.equal(t2.getAttribute('title'), null);
  assert.match(d2.note, /Give tier 2|Remove tier 2|Tier 2 is reached|Off while/);
});
