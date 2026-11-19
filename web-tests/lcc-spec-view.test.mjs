// The Specializations view of the Command Center (M5), drawn on its own from the synthetic cases of M1: five lanes with the clean path emblems, the dossier, the operations in two groups, the
// Houses of the path, and (only when the save has one) the line about a row no path owns. Names from a save are text, never markup; a track that is not identified has its XP controls off and its keystone buttons on.
import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { lccDir, read } from './lcc-load.mjs';
import { load } from './lcc-load.mjs';

const CASES = JSON.parse(read('docs', 'design', 'landsraad-command-center', 'fixtures', 'api-player-specs.cases.json')).cases;
const byId = (id) => JSON.parse(JSON.stringify(CASES.find((c) => c.id === id).response));
const HOUSES = [{ id: 11, name: 'DA_HouseAlexin', line: 'Crafting' }, { id: 12, name: 'DA_HouseEcaz', line: 'Crafting' }, { id: 13, name: 'DA_HouseHagal', line: 'Crafting' }, { id: 21, name: 'DA_HouseSor', line: 'Combat' }];

function draw(resp, houses = HOUSES, host = {}, sel = 'crafting') {
  const { dom, w } = load();
  dom.window.eval(fs.readFileSync(path.join(lccDir, 'lcc-spec.js'), 'utf8'));
  const S = w.LCCS;
  S.state.sel = sel;
  const box = w.document.createElement('div');
  w.document.body.appendChild(box);
  const vm = S.readSpecs(resp, houses);
  S.mount(box, vm, host);
  return { S, w, doc: w.document, box, vm, redraw: () => S.mount(box, vm, host) };
}
const q = (d, s) => d.box.querySelector(s);
const qa = (d, s) => [...d.box.querySelectorAll(s)];
const txt = (e) => (e ? e.textContent.replace(/\s+/g, ' ').trim() : null);

test('five lanes in the approved order, each a button with its path emblem, its level, its XP and its keystones, one of them pressed', () => {
  const d = draw(byId('mixed-five-tracks'));
  const lanes = qa(d, '.lcc-sp-lane');
  assert.deepEqual(lanes.map((l) => l.dataset.track), ['crafting', 'gathering', 'exploration', 'combat', 'sabotage']);
  for (const l of lanes) {
    assert.equal(l.tagName, 'BUTTON'); assert.equal(l.type, 'button');
    const img = l.querySelector('img');
    assert.match(img.getAttribute('src'), new RegExp('^/lcc/art/emblems/' + l.dataset.track + '\\.png$'), 'the clean emblem, not the scroll banner picture');
    assert.equal(img.getAttribute('alt'), '');
    assert.match(l.getAttribute('aria-label'), /level \d+ of 100, [\d,]+ XP, \d+ of \d+ keystones/);
  }
  assert.deepEqual(lanes.map((l) => l.getAttribute('aria-pressed')), ['true', 'false', 'false', 'false', 'false']);
});

test('choosing a lane moves the dossier and the operations to it, and only that: no request, no data-act on a lane', () => {
  const d = draw(byId('mixed-five-tracks'));
  const lane = q(d, '[data-lcs-lane="combat"]');
  assert.equal(lane.getAttribute('data-act'), null);
  lane.dispatchEvent(new d.w.MouseEvent('click', { bubbles: true }));
  assert.equal(txt(q(d, '#lcs-dos-h')), 'COMBAT');
  assert.equal(q(d, '[data-lcs-lane="combat"]').getAttribute('aria-pressed'), 'true');
  assert.equal(q(d, '[data-lcs-lane="crafting"]').getAttribute('aria-pressed'), 'false');
  assert.equal(q(d, '[data-act="spAddCC"]').dataset.t, 'Combat');
  assert.ok(q(d, '#spxCombat'));
  assert.equal(d.doc.activeElement, q(d, '[data-lcs-lane="combat"]') || null, 'the lane keeps the focus after the view is drawn again');
});

test('LEVEL 100 with no keystone: the gold treatment and the words of the XP cap, never "all keystones" or "full path"', () => {
  const d = draw(byId('xp-44182-cap-zero-keystones'), HOUSES, {}, 'combat');
  const lane = q(d, '[data-lcs-lane="combat"]');
  assert.equal(lane.dataset.kind, 'cap');
  assert.equal(txt(lane.querySelector('.lcc-sp-lane-badge')), '✦ XP CAP REACHED');
  assert.equal(txt(q(d, '#lcs-mastery')), 'LEVEL 100 · XP CAP REACHED');
  assert.equal(txt(q(d, '#lcs-keys')), '0 / 41');
  assert.equal(txt(q(d, '#lcs-xp')), '44,182 / 44,182');
  assert.equal(txt(q(d, '#lcs-xpremain')), 'XP CAP REACHED — NO FURTHER XP LEVELS');
  assert.equal(q(d, '.lcc-sp-dossier').dataset.kind, 'cap');
  assert.doesNotMatch(d.box.textContent, /FULL PATH|KEYSTONES COMPLETE/);
});

test('every keystone at level 0, and both complete, are different from the cap', () => {
  const k = draw(byId('xp-0-all-keystones-41-of-41'), HOUSES, {}, 'combat');
  assert.equal(txt(q(k, '#lcs-mastery')), 'KEYSTONES COMPLETE · 41/41');
  assert.equal(txt(q(k, '.lcc-sp-level')), '0/100');
  assert.equal(q(k, '[data-lcs-lane="combat"]').dataset.kind, 'keys');
  const b = draw(byId('both-complete'), HOUSES, {}, 'combat');
  assert.match(txt(q(b, '#lcs-mastery')), /^FULL PATH COMPLETE/);
  assert.equal(q(b, '[data-lcs-lane="combat"]').dataset.kind, 'full');
});

test('a save with no keystone map shows "Not in the save", not 0 of 0 and not a completed bar', () => {
  const d = draw(byId('keystone-total-zero-no-map'), HOUSES, {}, 'combat');
  assert.equal(txt(q(d, '#lcs-keys')), 'Not in the save');
  assert.doesNotMatch(txt(q(d, '[data-lcs-lane="combat"]')), /0\/0/);
  assert.equal(txt(q(d, '#lcs-keycount')), 'Not in the save');
});

test('a level the save stores that is not the level of its XP is shown with a warning and the save is not corrected', () => {
  const d = draw(byId('raw-row-xp-44182-level-10-mismatch'), HOUSES, {}, 'combat');
  const w = q(d, '#lcs-mismatch');
  assert.ok(w);
  assert.match(txt(w), /stores level 10 for this track, but its XP is level 100/);
  assert.match(txt(w), /Nothing was changed/);
  assert.equal(qa(d, '[data-act]').filter((b) => /^(spAdd|spMax|spReset)/.test(b.dataset.act)).every((b) => !b.disabled), true);
});

test('a track that is not identified: no XP, no level, XP controls off with the reason, the keystone buttons on, the unknown row named read-only', () => {
  const d = draw(byId('conflicting-learned-number-leaves-combat-unresolved'), HOUSES, {}, 'combat');
  const lane = q(d, '[data-lcs-lane="combat"]');
  assert.equal(lane.dataset.kind, 'unresolved');
  assert.match(lane.getAttribute('aria-label'), /track not identified, no XP shown/);
  assert.equal(txt(q(d, '.lcc-sp-level')), '—');
  assert.equal(txt(q(d, '#lcs-xp')), 'Not in the save');
  for (const sel of ['[data-act="spAddCC"]', '[data-act="spMax"]', '[data-act="spReset"]', '#spxCombat']) {
    const e = q(d, sel); assert.ok(e, sel); assert.equal(e.disabled, true, sel);
    assert.match(e.getAttribute('aria-describedby'), /lcs-nonum/, sel + ' says why');
  }
  assert.match(txt(q(d, '#lcs-nonum')), /No track number of the save is tied to this path, so XP editing is off/);
  assert.doesNotMatch(d.box.textContent, /Track mapping|advanced editor/i, 'no pointer to the removed section');
  assert.equal(q(d, '[data-act="spKeyAll"]').disabled, false);
  assert.equal(q(d, '[data-act="spKeyReset"]').disabled, false);
  assert.match(txt(q(d, '#lcs-unknown-row')), /^Unknown specialization row in the save \(not edited\): number \d+\.$/);
  assert.equal(q(d, '#lcs-unknown-row').querySelector('button, input, select, a'), null, 'the line is read-only');
});

test('the track mapping and advanced editor section is gone (#986): no heading, no eyebrow, no disclosure, no track numbers, no assign, no hand entry, no raw row; the XP field keeps the id the handlers read', () => {
  for (const id of ['unassigned-row-number-9', 'mixed-five-tracks', 'conflicting-learned-number-leaves-combat-unresolved']) {
    const d = draw(byId(id));
    assert.equal(q(d, '.lcc-sp-adv'), null, id);
    assert.equal(q(d, '.lcc-sp-details'), null, id);
    assert.equal(q(d, '#lcs-adv-h'), null, id);
    for (const sel of ['#spmt', '#spmn', '#st', '#sx', '#sl', '[id^="spa"]', '[data-act="spAssign"]', '[data-act="spAssignHand"]', '[data-act="specSet"]', 'select', 'summary']) assert.equal(q(d, sel), null, id + ' ' + sel);
    assert.doesNotMatch(d.box.textContent, /EXISTING EDITOR FEATURES|RETAINED|Track mapping|advanced editor|Track numbers|Exact raw row|Know a track|waiting to be assigned/i, id);
  }
  assert.equal(q(draw(byId('mixed-five-tracks')), '#spxCrafting').value, '1000');
});

test('a save where every row belongs to a path has no line about an unknown row; a save with one has exactly one, read-only', () => {
  assert.equal(q(draw(byId('mixed-five-tracks')), '#lcs-unknown-row'), null);
  assert.equal(q(draw(byId('xp-0-keystones-0-of-41')), '#lcs-unknown-row'), null);
  const d = draw(byId('unassigned-row-number-9'));
  assert.equal(qa(d, '#lcs-unknown-row').length, 1);
  assert.equal(txt(q(d, '#lcs-unknown-row')), 'Unknown specialization row in the save (not edited): number 9.');
  const two = byId('unassigned-row-number-9');
  two.unassigned.push({ number: 12, xp: 5, level: 0 });
  assert.equal(txt(q(draw(two), '#lcs-unknown-row')), 'Unknown specialization rows in the save (not edited): numbers 9, 12.');
});

test('a path at level 100 (the XP cap or the full path) is marked for the one win outline of lcc.css; a path below 100 is not (#988)', () => {
  const d = draw(byId('mixed-five-tracks'));
  const win = qa(d, '.lcc-sp-lane').filter((l) => l.getAttribute('data-victory') === 'win').map((l) => l.dataset.track);
  assert.deepEqual(win, ['crafting'], 'only the lane at 44,182 XP');
  for (const l of qa(d, '.lcc-sp-lane')) if (l.dataset.track !== 'crafting') assert.equal(l.getAttribute('data-victory'), null, l.dataset.track + ' keeps a normal border');
  const cap = draw(byId('xp-44182-cap-zero-keystones'), HOUSES, {}, 'combat');
  assert.equal(q(cap, '[data-lcs-lane="combat"]').getAttribute('data-victory'), 'win', 'cap without keystones');
  assert.equal(q(cap, '[data-lcs-lane="combat"]').getAttribute('aria-pressed'), 'true');
  const near = draw(byId('xp-44181-near-cap-keystones-20-of-41'), HOUSES, {}, 'combat');
  assert.equal(q(near, '[data-lcs-lane="combat"]').getAttribute('data-victory'), null, 'one XP short of the cap');
  const keys = draw(byId('xp-0-all-keystones-41-of-41'), HOUSES, {}, 'combat');
  assert.equal(q(keys, '[data-lcs-lane="combat"]').getAttribute('data-victory'), null, 'all keystones at level 0');
});

test('the lane and dossier pictures are the clean emblems at the sizes asked for, with no box element around them (#989)', () => {
  const d = draw(byId('mixed-five-tracks'));
  for (const l of qa(d, '.lcc-sp-lane')) { const i = l.querySelector('img'); assert.equal(i.getAttribute('width'), '72'); assert.equal(i.getAttribute('height'), '72'); }
  const h = q(d, '.lcc-sp-dossier-img');
  assert.equal(h.getAttribute('src'), '/lcc/art/emblems/crafting.png');
  assert.equal(h.getAttribute('width'), '96');
  assert.doesNotMatch(d.box.innerHTML, /art\/lanes\//, 'the scroll pictures are not used in this view');
});

test('the controls: each carries the data-act of an existing control, the global ones are in their own labelled group, and the selected-track ones are not in it', () => {
  const d = draw(byId('mixed-five-tracks'));
  const acts = qa(d, '[data-act]').map((b) => b.dataset.act).sort();
  assert.deepEqual(acts, ['spAddCC', 'spKeyAll', 'spKeyReset', 'spMax', 'spReset', 'spResetAll']);
  const glob = q(d, '.lcc-sp-global');
  assert.equal(glob.getAttribute('role'), 'group');
  assert.equal(txt(glob.querySelector('#lcs-glob-h')), 'All Five Tracks');
  assert.deepEqual([...glob.querySelectorAll('[data-act]')].map((b) => b.dataset.act).sort(), ['spKeyAll', 'spKeyReset', 'spResetAll']);
  for (const b of glob.querySelectorAll('[data-act]')) assert.equal(b.getAttribute('data-t'), null, 'a global button names no track');
  assert.match(txt(q(d, '#lcs-keyscope')), /change every track, not the selected one/);
  const own = q(d, '.lcc-sp-group');
  assert.notEqual(own, glob);
  assert.deepEqual([...own.querySelectorAll('[data-act]')].map((b) => b.dataset.act + ':' + b.dataset.t).sort(), ['spAddCC:Crafting', 'spMax:Crafting', 'spReset:Crafting']);
});

test('Reset XP of all known tracks: the count and the rows left alone come from the save, and it is off with the reason when there is nothing to reset', () => {
  const d = draw(byId('unassigned-row-number-9'));
  const b = q(d, '[data-act="spResetAll"]');
  assert.equal(b.dataset.n, String(d.vm.resetCount));
  assert.equal(b.dataset.left, '1');
  assert.match(txt(b), new RegExp('\\(' + d.vm.resetCount + '\\)'));
  assert.match(txt(q(d, '#lcs-resetnote')), /row whose track is not known yet is left alone/);
  const none = draw(byId('xp-0-keystones-0-of-41'));
  const nb = q(none, '[data-act="spResetAll"]');
  assert.equal(nb.disabled, true); assert.equal(nb.dataset.n, '0');
  assert.match(txt(q(none, '#lcs-resetnote')), /Nothing to reset: no track whose number is known has XP/);
  assert.equal(nb.getAttribute('aria-describedby'), 'lcs-resetnote');
});

test('the Houses of a path are buttons that open the House on the Landsraad board, from the House\'s own id; nothing else is sent', () => {
  const got = [];
  const d = draw(byId('mixed-five-tracks'), HOUSES, { goHouse: (id) => got.push(id) });
  const hs = qa(d, '.lcc-sp-house');
  assert.equal(hs.length, 3);
  assert.deepEqual(hs.map((h) => txt(h.querySelector('strong'))), ['ALEXIN', 'ECAZ', 'HAGAL']);
  assert.match(hs[0].getAttribute('aria-label'), /^Open House Alexin on the Landsraad board$/);
  assert.match(hs[0].querySelector('img').getAttribute('src'), /^\/lcc\/art\/houses\/alexin\.webp$/);
  hs[1].dispatchEvent(new d.w.MouseEvent('click', { bubbles: true }));
  assert.deepEqual(got, [12]);
  assert.match(q(d, '.lcc-sp-houses #lcs-hs-h').getAttribute('title'), /public community tracker, not from the game or the save/, 'the source is the tooltip of the heading (#980)');
  assert.doesNotMatch(txt(q(d, '.lcc-sp-houses')), /public community tracker/);
});

test('with no Landsraad read the Houses are "Not in the save", and a path with no House says so', () => {
  const d = draw(byId('mixed-five-tracks'), null);
  assert.match(txt(q(d, '.lcc-sp-houses')), /could not be read, so the Houses of this path are not in the save/);
  assert.equal(qa(d, '.lcc-sp-house').length, 0);
  assert.equal(txt(qa(d, '.lcc-sp-fact strong')[0]), 'Not in the save');
  const e = draw(byId('mixed-five-tracks'), HOUSES, {}, 'sabotage');
  assert.match(txt(q(e, '.lcc-sp-houses')), /No House of this term is listed under this path/);
});

test('a name from a save is text: a hostile House name or track name never becomes markup', () => {
  const evil = '<img src=x onerror=window.__pwned=1>';
  const resp = byId('mixed-five-tracks');
  resp.rows[0].track = evil; // the row that is drawn first among unknown names
  const d = draw(resp, [{ id: 1, name: 'DA_House' + evil, line: 'Crafting' }]);
  assert.equal(d.box.querySelectorAll('img[src="x"]').length, 0);
  assert.equal(d.w.__pwned, undefined);
  assert.ok(d.box.textContent.includes(evil.toUpperCase()) || d.box.textContent.includes(evil));
});

test('every picture has an empty alt and a text beside it; nothing is loaded from outside this origin; no inline style attribute is written', () => {
  const d = draw(byId('mixed-five-tracks'));
  for (const i of qa(d, 'img')) { assert.equal(i.getAttribute('alt'), ''); assert.match(i.getAttribute('src'), /^\/lcc\/art\//); }
  const html = d.box.innerHTML;
  assert.doesNotMatch(html, /https?:|url\(|@import/);
  // a run-time value is set through the style object, as the content security policy asks: it is a custom property only
  for (const e of qa(d, '[style]')) assert.match(e.getAttribute('style'), /^--lcc-v:\s*[\d.]+%;?$/);
});

test('the bars are exact and the labelled ones carry their numbers', () => {
  const d = draw(byId('xp-44181-near-cap-keystones-20-of-41'), HOUSES, {}, 'combat');
  const xp = q(d, '.lcc-sp-xpbar');
  assert.equal(xp.getAttribute('role'), 'progressbar');
  assert.equal(xp.getAttribute('aria-valuenow'), '44181'); assert.equal(xp.getAttribute('aria-valuemax'), '44182');
  const ks = q(d, '.lcc-sp-keybar');
  assert.equal(ks.getAttribute('aria-valuenow'), '20'); assert.equal(ks.getAttribute('aria-valuemax'), '41');
  assert.equal(txt(q(d, '#lcs-xpremain')), '1 XP TO LEVEL 100 · 1 XP TO CAP');
  assert.equal(txt(q(d, '.lcc-sp-level')), '99/100');
});

test('the level scale is labelled as reference marks and marks where the level stands', () => {
  const d = draw(byId('xp-44181-near-cap-keystones-20-of-41'), HOUSES, {}, 'combat');
  assert.match(q(d, '.lcc-sp-scale .lcc-sp-k').getAttribute('title'), /Reference marks along the 100 levels\. They are not unlock levels\./, 'said in the tooltip of the label, not as a line (#980)');
  assert.doesNotMatch(txt(q(d, '.lcc-sp-scale')), /not unlock levels/);
  const marks = qa(d, '.lcc-sp-mark');
  assert.deepEqual(marks.map((m) => txt(m.querySelector('b'))), ['LV 0', 'LV 25', 'LV 50', 'LV 75', 'LV 100']);
  assert.deepEqual(marks.map((m) => m.classList.contains('is-past')), [true, true, true, true, false]);
  assert.deepEqual(marks.map((m) => m.classList.contains('is-here')), [false, false, false, true, false]);
});

test('a read that cannot be used draws one honest message and no control', () => {
  const d = draw({ rows: 'x' });
  assert.equal(qa(d, '[data-act]').length, 0);
  assert.match(txt(q(d, '.lcc-sp-empty')), /The save gave no specialization tracks this page can read\./);
});

test('the hero counts come from the save', () => {
  const d = draw(byId('both-complete'));
  assert.equal(txt(q(d, '#lcs-capcount')), '1 / 5');
  assert.equal(txt(q(d, '#lcs-maxxp')), '44,182');
});
