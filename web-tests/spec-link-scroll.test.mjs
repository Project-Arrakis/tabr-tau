// #1041: the specialization link of the House Inspector ("Gathering Specialization") opens the Specialization page with that path chosen AND the lanes and the dossier in view, not at the top of the page.
// A move to another page scrolls to the top before the new page draws (#1025), so the scroll to the path has to come after the draw. Real app.js and lcc scripts over the synthetic answer of M1;
// jsdom has no layout, so the test records the scrolls (window.scrollTo and scrollIntoView) in the order they happen. The real scroll position is checked in a browser engine by the sweep.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import { boot, closeAll } from './boot.mjs';
import { synthetic } from './lcc-fixture.mjs';

afterEach(closeAll);
const TRACKS = ['crafting', 'gathering', 'exploration', 'combat', 'sabotage'];
const click = (ui, el) => { assert.ok(el, 'no element'); el.dispatchEvent(new ui.w.MouseEvent('click', { bubbles: true, cancelable: true })); };

async function open(local = { tab: 'landsraad', 'lsb.view': 'board', 'lsb.layout': 'grid' }) {
  const log = [];
  const ui = await boot({
    get: { '/api/landsraad': synthetic() }, local,
    pre: (w) => {
      w.scrollTo = (x, y) => log.push(['top', y]);
      w.Element.prototype.scrollIntoView = function (o) { log.push(['into', this.className, this.closest && this.closest('#lcs-root') ? 'in-spec-page' : 'elsewhere', o && o.block]); };
    },
  });
  await ui.settle();
  return { ui, log };
}
// selects Houses on the board until the Inspector offers the link for this path
function linkFor(ui, track) {
  const n = ui.doc.querySelectorAll('.lcc-bd-cell .lcc-hc-select').length;
  for (let i = 0; i < n; i++) {
    click(ui, ui.doc.querySelectorAll('.lcc-bd-cell .lcc-hc-select')[i]); // the board is drawn again after a click, so the cards are looked up each time
    const g = ui.doc.querySelector(`[data-lcb="gopath"][data-track="${track}"]`);
    if (g) return g;
  }
  return null;
}

for (const track of TRACKS) {
  test(`the ${track} link opens Specializations with ${track} chosen and the lanes scrolled into view, after the page is drawn`, async () => {
    const { ui, log } = await open();
    const g = linkFor(ui, track);
    assert.ok(g, `a House on the ${track} path`);
    log.length = 0;
    click(ui, g);
    await ui.settle();
    assert.equal(ui.posted.length, 0, 'only moves between pages');
    assert.ok(ui.doc.querySelector('#lcs-mount .lcc-sp'), 'the Specializations view is drawn');
    const on = [...ui.doc.querySelectorAll('.lcc-sp-lane[aria-pressed="true"]')];
    assert.equal(on.length, 1);
    assert.equal(on[0].getAttribute('data-lcs-lane'), track, 'the path of the link is the chosen one');
    const into = log.filter((e) => e[0] === 'into');
    assert.equal(into.length, 1, 'one scroll into view: ' + JSON.stringify(log));
    assert.equal(into[0][2], 'in-spec-page', 'the thing scrolled to is on the Specialization page');
    assert.match(into[0][1], /lcc-sp-headrow/, 'the row that heads the lanes');
    assert.equal(into[0][3], 'start');
    const top = log.findIndex((e) => e[0] === 'top'), at = log.findIndex((e) => e[0] === 'into');
    assert.ok(top >= 0 && at > top, 'the move to the top comes first and the scroll to the path after it, so it is not undone: ' + JSON.stringify(log));
  });
}

test('opening the Specialization page from the tabs scrolls to its top and nowhere else, and a redraw does not scroll to the path again', async () => {
  const { ui, log } = await open({ tab: 'landsraad', 'lsb.view': 'board', 'lsb.layout': 'grid', 'spec.view': 'cc' });
  log.length = 0;
  click(ui, ui.doc.querySelector('[data-tab="player"]'));
  await ui.settle();
  click(ui, ui.doc.querySelector('[data-sub="player:specialization"]'));
  await ui.settle();
  assert.ok(ui.doc.querySelector('#lcs-mount .lcc-sp'), 'the page is drawn');
  assert.equal(log.filter((e) => e[0] === 'into').length, 0, 'a plain tab move leaves the page at its top: ' + JSON.stringify(log));
  // a link, then away and back by the tabs: the second visit is a plain one and is not scrolled to the path
  click(ui, ui.doc.querySelector('[data-tab="landsraad"]'));
  await ui.settle();
  click(ui, linkFor(ui, 'gathering'));
  await ui.settle();
  assert.equal(log.filter((e) => e[0] === 'into' && e[2] === 'in-spec-page').length, 1, 'the link scrolled once');
  log.length = 0;
  click(ui, ui.doc.querySelector('[data-tab="landsraad"]'));
  await ui.settle();
  click(ui, ui.doc.querySelector('[data-tab="player"]'));
  await ui.settle();
  assert.equal(log.filter((e) => e[0] === 'into' && e[2] === 'in-spec-page').length, 0, 'a later visit by the tabs is not scrolled to the path: ' + JSON.stringify(log));
});

test('a lane pressed on the Specialization page does not scroll the page', async () => {
  const { ui, log } = await open();
  const g = linkFor(ui, 'combat');
  click(ui, g);
  await ui.settle();
  log.length = 0;
  click(ui, ui.doc.querySelector('.lcc-sp-lane[data-lcs-lane="gathering"]'));
  assert.equal(ui.doc.querySelector('.lcc-sp-lane[aria-pressed="true"]').getAttribute('data-lcs-lane'), 'gathering');
  assert.deepEqual(log.filter((e) => e[0] === 'into' || e[0] === 'top'), [], 'choosing a lane by hand leaves the scroll where it is');
});
