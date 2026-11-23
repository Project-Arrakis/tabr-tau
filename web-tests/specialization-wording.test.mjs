// #1061: one word for the thing on the Specialization page: "specialization". A player never reads "path" or "track" for it, in the Command Center view, in the Landsraad
// Inspector's link or in the Help. "Track" stays only in the Advanced cards under the view (the track-number editor), which name the save's own XP track numbers
// (marked data-advanced). The Classic list these words were first pinned against was retired (#1105). Real app.js and lcc scripts over the synthetic answers; no game save.
//
// The words of the confirmation questions and of the result lines are recorded in web-tests/lcc-spec-requests.golden.json (asked / said). The reset-all question, its typed sentence and its result line
// say "specialization" (the golden file was re-recorded for them, #1061); the results of the two Advanced track-number editors still say "track number" (that card names the save's own numbers).
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import { boot, closeAll } from './boot.mjs';
import { synthetic } from './lcc-fixture.mjs';
import { read } from './lcc-load.mjs';

afterEach(closeAll);
const CASES = JSON.parse(read('docs', 'design', 'landsraad-command-center', 'fixtures', 'api-player-specs.cases.json')).cases;
const SPECS = (id) => JSON.parse(JSON.stringify(CASES.find((c) => c.id === id).response));
const text = (e) => (e ? e.textContent.replace(/\s+/g, ' ').trim() : '');
const BAN = /\b(paths?|tracks?)\b/i;
const ATTRS = ['title', 'aria-label', 'aria-description', 'placeholder', 'alt'];
const ids = ['mixed-five-tracks', 'conflicting-learned-number-leaves-combat-unresolved', 'unassigned-row-number-9', 'both-complete', 'xp-44182-cap-zero-keystones', 'raw-row-xp-44182-level-10-mismatch', 'keystone-total-zero-no-map', 'xp-0-keystones-0-of-41'];
const noNumbers = () => { const s = SPECS('mixed-five-tracks'); for (const r of s.rows) r.number = null; s.unassigned = [{ number: 7, xp: 900, level: 4 }]; s.resetCount = 0; return s; };

// every piece of player-facing text under root, outside the Advanced cards: its own text nodes and the attributes a person or a screen reader reads
function said(root) {
  const out = [];
  const walk = (e) => {
    if (e.hasAttribute && e.hasAttribute('data-advanced')) return;
    if (['SCRIPT', 'STYLE', 'SVG', 'svg'].includes(e.tagName)) return;
    for (const a of ATTRS) if (e.getAttribute && e.getAttribute(a)) out.push(e.getAttribute(a));
    for (const n of e.childNodes) { if (n.nodeType === 3 && n.nodeValue.trim()) out.push(n.nodeValue.replace(/\s+/g, ' ').trim()); else if (n.nodeType === 1) walk(n); }
  };
  walk(root);
  return out;
}
const offences = (root) => said(root).filter((s) => BAN.test(s));

async function openSpec(local, specs) {
  const ui = await boot({ get: { '/api/player/specs': specs, '/api/landsraad': synthetic() }, local: { tab: 'player', ...local } });
  ui.click('[data-sub="player:specialization"]'); await ui.settle();
  return ui;
}

test('Command Center Specializations: no "path" and no "track" in any text, tooltip or screen-reader label, for every lane chosen, in every state of the save', async () => {
  for (const id of ids.concat('no-numbers')) {
    const ui = await openSpec({}, id === 'no-numbers' ? noNumbers() : SPECS(id));
    const root = ui.doc.querySelector('#lcs-root');
    assert.ok(root, id + ': the view is drawn');
    for (const lane of [...ui.doc.querySelectorAll('[data-lcs-lane]')].map((l) => l.getAttribute('data-lcs-lane'))) {
      ui.click('[data-lcs-lane="' + lane + '"]'); await ui.settle();
      assert.deepEqual(offences(ui.doc.querySelector('#lcs-root')), [], id + ' / ' + lane);
    }
    closeAll();
  }
});

test('The whole Specializations page: no "path" and no "track" outside the Advanced cards, with every number known, with a number missing, with a row of the save that no specialization owns, and with none known', async () => {
  for (const id of ['mixed-five-tracks', 'conflicting-learned-number-leaves-combat-unresolved', 'unassigned-row-number-9', 'no-numbers']) {
    const ui = await openSpec({}, id === 'no-numbers' ? noNumbers() : SPECS(id));
    const page = ui.doc.querySelector('#main');
    assert.ok(page.querySelector('#lcs-root'), id + ': the Command Center view is drawn');
    assert.equal(page.querySelector('table'), null, id + ': no Classic table');
    assert.deepEqual(offences(page), [], id);
    closeAll();
  }
});

test('The Advanced cards are marked, and are the only places that still say "track" (the track-number editor; the raw row card was removed in #1133)', async () => {
  const ui = await openSpec({}, SPECS('conflicting-learned-number-leaves-combat-unresolved'));
  const adv = [...ui.doc.querySelectorAll('#main [data-advanced]')];
  assert.equal(adv.length, 1, 'the track-number card');
  assert.deepEqual(adv.map((a) => text(a.querySelector('h3, summary'))), ['Advanced: specialization numbers']);
});

test('the Command Center says the words the Classic list said, where both had the thing', async () => {
  const cc = await openSpec({}, SPECS('unassigned-row-number-9'));
  const root = cc.doc.querySelector('#lcs-root');
  const ccNames = [...root.querySelectorAll('.lcc-sp-lane-name')].map((n) => text(n).toLowerCase());
  assert.deepEqual(ccNames.sort(), ['combat', 'crafting', 'exploration', 'gathering', 'sabotage'], 'the same five names');
  assert.match(text(root.querySelector('[data-act="spResetAll"]')), /^Reset XP of All Known Specializations \(\d+\)$/i);
  assert.match(text(root), /the editor cannot match yet (is|are) left alone\./);
  assert.match(text(root.querySelector('[data-act="spKeyAll"]')), /^Grant All Keystones$/i);
  closeAll();
  const none = await openSpec({}, noNumbers());
  const off = text(none.doc.querySelector('#lcs-root .lcc-sp-hint, #lcs-root #lcs-resetnote'));
  assert.match(off, /^Nothing to reset: none of the specializations the editor can match has XP./);
});

test('the fixed words of the Command Center page: the five specializations, choose a specialization, selected specialization, reset XP for this specialization, all specializations, complete', async () => {
  const ui = await openSpec({}, SPECS('both-complete'));
  const root = ui.doc.querySelector('#lcs-root');
  const all = text(root);
  for (const s of ['THE FIVE SPECIALIZATIONS OF ARRAKIS', 'CHOOSE A SPECIALIZATION', 'SELECTED SPECIALIZATION', 'THIS SPECIALIZATION', 'All Specializations', 'Reset XP of All Known Specializations', 'COMPLETE']) assert.ok(all.includes(s), s + ' in ' + all.slice(0, 400));
  assert.equal(root.querySelector('[data-act="spReset"]').getAttribute('aria-label'), 'Reset XP for this specialization');
  assert.equal(text(root.querySelector('[data-act="spReset"]')), 'Reset XP');
  assert.ok([...root.querySelectorAll('.lcc-sp-lane-badge')].some((b) => text(b) === '✦ COMPLETE'), 'the badge of a specialization with the cap and every keystone');
  assert.doesNotMatch(all, /FULL PATH|PATH IN PROGRESS|NOT IDENTIFIED.*TRACK/);
});

test('the Landsraad Inspector link and the House list say "specialization", never "path" or "track"', async () => {
  const ui = await boot({ get: { '/api/landsraad': synthetic() }, local: { tab: 'landsraad', 'lsb.view': 'board', 'lsb.layout': 'grid' } });
  await ui.settle();
  const n = ui.doc.querySelectorAll('.lcc-bd-cell .lcc-hc-select').length;
  let link = null;
  for (let i = 0; i < n && !link; i++) {
    ui.doc.querySelectorAll('.lcc-bd-cell .lcc-hc-select')[i].dispatchEvent(new ui.w.MouseEvent('click', { bubbles: true, cancelable: true }));
    link = ui.doc.querySelector('[data-lcb="gopath"]');
  }
  assert.ok(link, 'a House with the specialization link');
  assert.match(text(link), /Specialization$/);
  assert.deepEqual(said(link).filter((s) => BAN.test(s)), [], 'the link');
  assert.match(link.getAttribute('aria-label'), /Specialization\. Open \w+ in Specializations\./);
  const insp = ui.doc.querySelector('.lcc-bd-insp');
  assert.deepEqual(said(insp).filter((s) => /\b(paths?|tracks?)\b/i.test(s)), [], 'the whole Inspector');
});

test('Help: the Specializations page says "specialization", and "track" only where it names the save\'s own track numbers', () => {
  const md = read('docs', 'help', 'specializations.md');
  const readme = read('docs', 'help', 'README.md');
  assert.doesNotMatch(readme.split('\n').filter((l) => l.includes('specializations.md')).join(' '), BAN);
  assert.doesNotMatch(md.replace(/## Technical notes[\s\S]*$/, ''), BAN, 'outside the section about the save\'s track numbers');
  assert.doesNotMatch(md, /\bpaths?\b/i, '"path" nowhere');
  for (const f of ['landsraad-board.md', 'term-and-decrees.md', 'fill-and-tiers.md', 'checked-in-the-game.md']) assert.doesNotMatch(read('docs', 'help', f), /\b(spec\w* )?(path|track)s? of a (house|specialization)/i, f);
});
