// The win tokens and the optional animated border (M3): --lcc-glow-win and --lcc-outline-win, the outline on the Houses of a faction that holds more than half of the Houses, and the rule that the
// animated border is only ever added when Motion is Full and the browser has not asked for reduced motion. The board is read from the synthetic answer of M1 (no game save is read).
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { boot, closeAll } from './boot.mjs';
import { load, lccDir } from './lcc-load.mjs';
import { synthetic } from './lcc-fixture.mjs';

afterEach(closeAll);

const css = fs.readFileSync(path.join(lccDir, 'lcc.css'), 'utf8').replace(/\r/g, '');
const bare = css.replace(/\/\*[\s\S]*?\*\//g, '');

test('the two win tokens exist, the glow has its own dial, and the winning Houses use both', () => {
  assert.match(bare, /--lcc-glow-win:\s*1;/);
  assert.match(bare, /--lcc-outline-win:\s*3px solid var\(--lcc-gold\);/);
  // the light of a win is not the plaque's softer dial: --lcc-glow-win is defined as a plain number and --lcc-glow-win is not derived from --lcc-glow
  assert.doesNotMatch(bare.match(/--lcc-glow-win:[^;]*;/)[0], /--lcc-glow\b(?!-win)/);
  for (const kind of ['win']) { // one win outline for every House that makes up a win (2026-11-18)
    const rule = bare.match(new RegExp('\\.lcc-house\\[data-victory="' + kind + '"\\](?:, \\.lcc-sp-lane\\[data-victory="' + kind + '"\\])?(?:, \\.lcc-bd-victory[^{]*)?\\s*\\{[^}]*\\}'))[0]; // the WON tag of the victory panel joins the list (#978)
    assert.match(rule, /outline:\s*var\(--lcc-outline-win\)/, kind);
    assert.match(rule, /var\(--lcc-glow-win\)/, kind);
    assert.doesNotMatch(rule, /var\(--lcc-glow\)/, kind);
  }
});

test('the animated border moves by transform only, and its style needs Full motion and no reduced-motion request', () => {
  const kf = bare.match(/@keyframes lcc-win-sweep\s*\{([^]*?\})\s*\}/);
  assert.ok(kf, 'the keyframes');
  assert.match(kf[1], /rotate:\s*360deg/);
  assert.doesNotMatch(kf[1], /\b(width|height|top|left|margin|padding|border)\b/, 'only transform properties are animated');
  // the sweep is hidden by default, and every rule that shows or moves it sits in the no-preference block and names Motion Full
  assert.match(bare, /\.lcc-win-sweep\s*\{\s*display:\s*none;\s*\}/);
  const block = bare.match(/@media \(prefers-reduced-motion: no-preference\)\s*\{([^]*?)\n\}\n/);
  assert.ok(block, 'the no-preference block');
  assert.match(block[1], /html\[data-motion="full"\] \.lcc-win-anim > \.lcc-win-sweep \{/);
  assert.match(block[1], /animation:\s*lcc-win-sweep 9s linear infinite/);
  assert.equal((bare.match(/lcc-win-sweep \d+s linear infinite/g) || []).length, 2, 'two places start the animation: the winning Houses, and the centre seat of a win by 13 Houses (both gated on Motion Full and no reduced-motion request)');
  // forced colours draw the still outline only
  assert.match(bare, /@media \(forced-colors: active\)[^]*html\[data-motion\] \.lcc-win-anim > \.lcc-win-sweep \{ display: none; \}/);
});

// the class is added by the script only in the one case the setting allows it
function cardWith(motion, { reduce = false, noMatchMedia = false } = {}) {
  const { w, L, doc } = load();
  if (motion != null) doc.documentElement.setAttribute('data-motion', motion);
  if (!noMatchMedia) w.matchMedia = (q) => ({ matches: reduce && q === '(prefers-reduced-motion: reduce)', media: q });
  return { L, card: L.renderHouseCard({ name: 'DA_HouseVernius', owner: 'a', inLine: true, win: true }) };
}
const animated = (card) => card.classList.contains('lcc-win-anim') || !!card.querySelector('.lcc-win-sweep');

test('the animation class is added with Motion Full, and not with Calm, Off, no setting, or a reduced-motion request', () => {
  assert.equal(animated(cardWith('full').card), true);
  assert.equal(cardWith('full').card.querySelector('.lcc-win-sweep').getAttribute('aria-hidden'), 'true');
  assert.equal(animated(cardWith('calm').card), false, 'Motion Calm');
  assert.equal(animated(cardWith('off').card), false, 'Motion Off');
  assert.equal(animated(cardWith(null).card), false, 'no Motion setting');
  assert.equal(animated(cardWith('full', { reduce: true }).card), false, 'Motion Full, but reduced motion is requested');
  assert.equal(animated(cardWith('calm', { reduce: true }).card), false, 'both');
  // the still outline is there in every one of those cases
  for (const c of [cardWith('calm').card, cardWith('off').card, cardWith('full', { reduce: true }).card]) assert.equal(c.getAttribute('data-victory'), 'win');
});

test('a card that stops being a winner loses the border, and a card of no victory never has it', () => {
  const { L, card } = cardWith('full');
  assert.equal(animated(card), true);
  L.victory(card, null);
  assert.equal(animated(card), false);
  assert.equal(card.getAttribute('data-victory'), null);
  assert.equal(animated(cardWith('full').L.renderHouseCard({ name: 'DA_HouseVernius', owner: 'a' })), false);
  // drawn twice, it has one sweep, not two
  L.victory(card, true); L.victory(card, true);
  assert.equal(card.querySelectorAll('.lcc-win-sweep').length, 1);
});

test('the winning count: a faction with more than half of the Houses gets the outline on its Houses; with fewer, nobody does', async () => {
  const open = async (answer, motion) => {
    const ui = await boot({ get: { '/api/landsraad': answer }, local: { tab: 'landsraad', 'lsb.view': 'board' } });
    await ui.settle();
    if (motion) ui.doc.documentElement.setAttribute('data-motion', motion);
    return ui;
  };
  const a12 = synthetic();
  assert.equal(a12.tasks.filter((t) => t.completed === 1 && t.winning_faction_id === 1).length, 12);
  let ui = await open(a12);
  assert.equal(ui.doc.querySelectorAll('.lcc-bd-cell [data-victory]').length, 0, '12 of 25 is not more than half');
  closeAll();

  const a13 = synthetic();
  const spare = a13.tasks.find((t) => t.completed === 0);
  spare.completed = 1; spare.winning_faction_id = 1;
  ui = await open(a13, 'full');
  // the board is drawn when the tab opens; draw it again with Motion Full
  await ui.w.landsraadView(); await ui.settle();
  const won = [...ui.doc.querySelectorAll('.lcc-bd-cell .lcc-hc')].filter((c) => c.getAttribute('data-victory') === 'win');
  assert.equal(won.length, 13);
  assert.ok(won.every((c) => c.getAttribute('data-owner') === 'a'), 'only the Houses of the faction with the count');
  assert.equal(ui.doc.querySelectorAll('.lcc-bd-cell .lcc-hc[data-owner="h"][data-victory]').length, 0);
  assert.ok(won.every((c) => c.classList.contains('lcc-win-anim')), 'Motion Full: the border is added');
  // 13 Houses is a win by the game's rule (the operator's decision of 2026-11-18): the words say it, the card carries the outline and no text flag (#961), and the save is said not to hold the vote
  for (const c of won) {
    const pick = c.querySelector('button.lcc-hc-select');
    assert.match(pick.getAttribute('title') || pick.getAttribute('data-says'), /who hold 13 or more Houses\. Sysselraad secured by Atreides, by 13 Houses\. The save does not record the vote/);
    assert.equal(c.querySelector('.lcc-flag'), null, 'no text flag on a winning House, a 13-House win included (the way it was won is in the plaque and the panel)');
  }
  // Calm: the still outline stays, the animation class does not
  ui.doc.documentElement.setAttribute('data-motion', 'calm');
  await ui.w.landsraadView(); await ui.settle();
  const calm = [...ui.doc.querySelectorAll('.lcc-bd-cell .lcc-hc[data-victory="win"]')];
  assert.equal(calm.length, 13);
  assert.equal(calm.some((c) => c.classList.contains('lcc-win-anim') || c.querySelector('.lcc-win-sweep')), false);
});

test('a board that did not pass its checks outlines nobody', async () => {
  const bad = synthetic();
  bad.tasks.forEach((t) => { if (t.completed === 0) { t.completed = 1; t.winning_faction_id = 1; } });
  bad.tasks.pop();
  const ui = await boot({ get: { '/api/landsraad': bad }, local: { tab: 'landsraad', 'lsb.view': 'board' } });
  await ui.settle();
  assert.equal(ui.doc.querySelectorAll('[data-victory]').length, 0);
});
