// The text sweep (#426, docs/design/2026-10-26-ui-text-sweep.md): a permanent line must earn its place. State, results, errors and what is queued stay in the default view; an explanation, a limit or
// background is in a tooltip on its control or in a closed "About this", or is drawn only while it applies. The checks that hold on EVERY page are here; each page's own decisions are in its section.
import test from 'node:test';
import assert from 'node:assert/strict';
import { boot, closeAll } from './boot.mjs';
import { visibleText, movedText, eachPage } from './uitext.mjs';

test.afterEach(() => closeAll());

const SUMMARY = (vitals) => ({ name: 'Tester', status: 'Alive', map: 'Survival_1', faction: '', level: 3, xp: 100, skillPoints: null, intel: null, vitals, identity: {}, ids: {}, currency: [] });
const VITALS = { health: 100, healthMinMax: 100, hydration: 52.5, maxHydration: 100, spiceAddiction: 0, maxSpiceAddiction: 10 };
const LIQ = { water: { total: 3, notFull: 2, unknown: 0 }, bloodSacks: { total: 4, notFull: 1, unknown: 0, holdingOther: 0 }, vehicles: { total: 5, notFull: 3, unknown: 0, noFuelRecord: 0 } };
const open = async (ui) => { ui.click('[data-tab="player"]'); await ui.settle(); };

// ---- the mechanism, on every page

test('every tooltip has an id, a sentence, a role, and a control inside its wrapper that points at it; no id is used twice', async () => {
  const ui = await boot({});
  const seen = [];
  await eachPage(ui, (label) => {
    const ids = new Set();
    for (const el of ui.doc.querySelectorAll('[id]')) { assert.ok(!ids.has(el.id), `${label}: id ${el.id} is used twice`); ids.add(el.id); }
    for (const box of ui.doc.querySelectorAll('.tipbox')) {
      seen.push(box.id);
      assert.ok(box.id, `${label}: a tooltip has no id`);
      assert.equal(box.getAttribute('role'), 'tooltip', `${label}: ${box.id}`);
      assert.ok(box.textContent.trim().length > 10, `${label}: ${box.id} is empty`);
      const wrap = box.parentElement;
      assert.ok(wrap.classList.contains('tip'), `${label}: ${box.id} is not inside .tip`);
      const owner = [...wrap.querySelectorAll('[aria-describedby]')].find((c) => c.getAttribute('aria-describedby').split(/\s+/).includes(box.id));
      assert.ok(owner, `${label}: nothing in the wrapper of ${box.id} is described by it`);
    }
    for (const c of ui.doc.querySelectorAll('[aria-describedby]')) {
      for (const id of c.getAttribute('aria-describedby').split(/\s+/)) assert.ok(ui.doc.getElementById(id), `${label}: aria-describedby names ${id}, which is not on the page`);
    }
  });
  assert.ok(seen.length >= 10, 'the pages carry tooltips: ' + seen.length);
  ui.dom.window.close();
});

test('every About this is closed on a fresh load, is named, and is the last thing in its card', async () => {
  const ui = await boot({});
  let n = 0;
  await eachPage(ui, (label) => {
    for (const d of ui.doc.querySelectorAll('details.about')) {
      n++;
      assert.equal(d.open, false, `${label}: ${d.dataset.about} opens by itself`);
      assert.ok(d.dataset.about, `${label}: an About this has no key`);
      const s = d.querySelector('summary');
      assert.ok(s && s.textContent.startsWith('About this'), `${label}: ${d.dataset.about}`);
      assert.ok(s.querySelector('.sr'), `${label}: ${d.dataset.about} does not say which card it is about`);
      assert.equal(d.parentElement.lastElementChild, d, `${label}: ${d.dataset.about} is not the last thing in its card`);
    }
  });
  assert.ok(n >= 1, 'the pages carry About this: ' + n);
  ui.dom.window.close();
});

test('whether About this is open is kept per card in the storage, put back after a redraw, and forgotten when closed', async () => {
  const ui = await boot({ get: { '/api/player/summary': SUMMARY(VITALS), '/api/player/liquids': LIQ } });
  await open(ui);
  const d = () => ui.doc.querySelector('details[data-about="overview.fill"]');
  assert.equal(d().open, false);
  d().open = true;
  d().dispatchEvent(new ui.w.Event('toggle'));
  assert.equal(ui.w.localStorage.getItem('about.overview.fill'), '1');
  ui.click('[data-sub="player:crafting"]'); await ui.settle();
  ui.click('[data-sub="player:character"]'); await ui.settle();
  assert.equal(d().open, true, 'open again when the page is drawn again');
  d().open = false;
  d().dispatchEvent(new ui.w.Event('toggle'));
  assert.equal(ui.w.localStorage.getItem('about.overview.fill'), null);
  ui.dom.window.close();
});

test('a browser that blocks its storage still draws every page, with About this closed', async () => {
  const ui = await boot({ pre: (w) => { Object.defineProperty(w, 'localStorage', { get() { throw new w.DOMException('The operation is insecure.', 'SecurityError'); }, configurable: true }); } });
  await open(ui);
  const d = ui.doc.querySelector('details[data-about="overview.fill"]');
  assert.ok(d && d.open === false);
  d.open = true;
  d.dispatchEvent(new ui.w.Event('toggle')); // writing must not throw
  ui.dom.window.close();
});

test('Escape puts a tooltip away while the focus is on its control, and it comes back when the focus leaves and returns', async () => {
  const ui = await boot({});
  await open(ui);
  const b = ui.doc.querySelector('[data-act="xp"]');
  const tipEl = b.closest('.tip');
  assert.ok(tipEl, 'the Give XP button has a tooltip');
  b.focus();
  ui.doc.dispatchEvent(new ui.w.KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
  assert.ok(tipEl.classList.contains('closed'));
  b.blur();
  assert.ok(!tipEl.classList.contains('closed'), 'back after the focus left');
  ui.dom.window.close();
});

// ---- Player > Character > Overview

test('Overview: the explanations moved to tooltips and the Fill card\'s About this; state and the way to act stay in view', async () => {
  const ui = await boot({ get: { '/api/player/summary': SUMMARY(VITALS), '/api/player/liquids': LIQ, '/api/player/blood-sacks': { total: 4, notFull: 3, unknown: 2, holdingOther: 0 } } });
  await open(ui);
  const shown = visibleText(ui.doc.querySelector('#main'));
  const moved = movedText(ui.doc.querySelector('#main'));
  const gone = [
    'A negative number removes', 'kept apart from the Solari stacks', 'The currency of the Scourge event', 'Raises the level as the XP grows', 'never past 2,779',
    'Fills the character\'s own water to the maximum', 'Fills the sacks you carry to the most blood seen', 'Fills the Literjons, stillsuits and other water containers you carry to their capacity',
    'Fills your Literjons, stillsuits and other water containers, your blood sacks and', 'The fill has not been checked in the game yet',
    'The XP goes back to 0 and the level with it',
  ];
  for (const s of gone) { assert.ok(!shown.includes(s), 'still shown: ' + s); assert.ok(moved.includes(s), 'lost, not moved: ' + s); }
  // the one sentence about the blood fill that cannot be missed is also in the card's About this, which is one click away
  assert.ok(ui.doc.querySelector('details[data-about="overview.fill"]').textContent.includes('has not been checked in the game yet'));
  // deleted for good: the summary above has it
  assert.ok(!shown.includes('Map, account, platform and the database ids are in the summary above'));
  assert.ok(!movedText(ui.doc).includes('Map, account, platform and the database ids are in the summary above'));
  assert.ok(!shown.includes('All of these apply when the game next loads the save'));
  // what stays
  for (const s of ['Solari in backpack', 'Blood Marks in backpack', 'Give XP', 'Give Intel', 'Fill hydration', 'Fill blood sacks', 'Refill Literjons', 'Fill all liquids', 'Also refuel my vehicles',
    '3 of 4 blood sacks are not full', '2 blood sacks of an unknown kind will be left alone', 'Not full: 2 water containers, 1 blood sack, 3 vehicles.', 'Reset level and XP (100 XP)']) {
    assert.ok(shown.includes(s), 'missing from the default view: ' + s);
  }
  ui.dom.window.close();
});

test('Overview: the hydration line is drawn only when there is no hydration to fill from, and the reset line only when the reset is off', async () => {
  let ui = await boot({ get: { '/api/player/summary': SUMMARY(VITALS) } });
  await open(ui);
  assert.equal(ui.doc.querySelector('#hydnote'), null, 'the number is the summary tile\'s');
  assert.equal(ui.doc.querySelector('#lvresetnote'), null, 'the reset is on: its explanation is the tooltip');
  assert.ok(ui.doc.querySelector('[data-act="lvReset"]').closest('.tip'));
  assert.equal(ui.doc.querySelector('[data-act="fillWater"]').getAttribute('aria-describedby'), 'hydtip');
  ui.dom.window.close();

  ui = await boot({ get: { '/api/player/summary': SUMMARY({ ...VITALS, hydration: null }) } });
  await open(ui);
  assert.ok(visibleText(ui.doc.querySelector('#main')).includes('The summary shows no hydration for the character, so this may be refused.'));
  assert.deepEqual(ui.doc.querySelector('[data-act="fillWater"]').getAttribute('aria-describedby').split(' '), ['hydnote', 'hydtip']);
  ui.dom.window.close();

  ui = await boot({ get: { '/api/player/summary': { ...SUMMARY(VITALS), xp: 0, level: 0 } } });
  await open(ui);
  assert.ok(visibleText(ui.doc.querySelector('#main')).includes('Nothing to reset: the character has no XP.'), 'why the button is off stays');
  assert.equal(ui.doc.querySelector('[data-act="lvReset"]').closest('.tip'), null, 'nothing to explain while it is off');
  ui.dom.window.close();
});

test('Overview: a fill that is queued while the game runs is still said where the result goes, not only in a tooltip', async () => {
  const ui = await boot({ get: { '/api/player/summary': SUMMARY(VITALS) }, post: { '/api/player/hydration': { ok: true, before: 0, after: 100, outcome: 'queued' } } });
  await open(ui);
  ui.click('[data-act="fillWater"]'); await ui.settle();
  const warn = ui.doc.querySelector('#toast .part');
  assert.ok(warn && warn.textContent.includes('queued, not applied') && warn.textContent.includes('will not help right now'), ui.doc.querySelector('#toast').textContent);
  ui.dom.window.close();
});
