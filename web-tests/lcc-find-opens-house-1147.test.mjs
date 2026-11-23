// #1147: Find House also chooses the found House, as a click on its square (grid) or its name (List) does: House Intelligence opens on it. The scroll and the outline stay; no filter changes and no box is
// ticked (the CHOOSE column is the multi-select for actions, not the selection). A House the List hides under the filter has no row: nothing happens, as before.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import { boot, closeAll } from './boot.mjs';
import { synthetic } from './lcc-fixture.mjs';

afterEach(closeAll);

const text = (el) => el.textContent.replace(/\s+/g, ' ').trim();
const click = (ui, el) => { if (typeof el === 'string') el = ui.doc.querySelector(el); assert.ok(el, 'no element'); el.focus(); el.dispatchEvent(new ui.w.MouseEvent('click', { bubbles: true, cancelable: true })); };
async function open(layout) {
  const ui = await boot({ get: { '/api/landsraad': synthetic() }, post: {}, local: { tab: 'landsraad', 'lsb.view': 'board', 'lsb.layout': layout } });
  await ui.settle();
  ui.scrolled = [];
  Object.defineProperty(ui.w.HTMLElement.prototype, 'offsetParent', { configurable: true, get() { return this.closest('[hidden]') ? null : this.parentNode; } }); // jsdom lays nothing out: the page asks which Find House key is drawn
  ui.w.HTMLElement.prototype.scrollIntoView = function () { ui.scrolled.push(this); };
  return ui;
}
const find = (ui, name) => {
  click(ui, '[data-lk="findbtn"]');
  const o = [...ui.doc.querySelectorAll('[data-lk="fpick"]')].find((n) => text(n) === name);
  assert.ok(o, 'no Find House entry for ' + name);
  click(ui, o);
};
const insp = (ui) => ui.doc.querySelector('.lcc-bd-insp-name');
const marked = (ui) => [...ui.doc.querySelectorAll('.lcc-k-flash')];

for (const layout of ['grid', 'list']) {
  test(`#1147 (${layout}): Find House opens the found House in House Intelligence, scrolls to it and outlines it, and changes no filter and ticks no box`, async () => {
    const ui = await open(layout);
    assert.equal(insp(ui), null, 'nothing is chosen at first');
    find(ui, 'Sor');
    assert.ok(insp(ui), 'House Intelligence is open');
    assert.match(text(insp(ui)), /Sor/, 'on the House that was found: ' + text(insp(ui)));
    // the same marks as a click on the square or the name
    if (layout === 'grid') assert.equal(ui.doc.querySelector('.lcc-hc[data-selected="true"] .lcc-hc-name').textContent.trim().toLowerCase(), 'house sor');
    else assert.equal(text(ui.doc.querySelector('tr.lcc-bd-row[aria-current="true"] .lcc-bd-namebtn')), 'Sor');
    // the scroll and the outline are on the found House, once, after the redraw that the choice causes
    assert.equal(ui.scrolled.length, 1, 'one scroll: to the House');
    assert.equal(marked(ui).length, 1, 'one outlined element');
    const found = layout === 'grid' ? '.lcc-bd-cell' : 'tr.lcc-bd-row';
    assert.ok(marked(ui)[0].matches(found), 'the card or the row: ' + marked(ui)[0].className);
    assert.equal(ui.scrolled[0], marked(ui)[0], 'the element that is scrolled to is the one that is outlined (the new one, not the one the redraw dropped)');
    // the focus is back on the Find House key that is drawn now, and quiet (#1134): the redraw made a new key
    assert.equal(ui.doc.activeElement.getAttribute('data-lk'), 'findbtn', 'the focus is on the Find House key');
    assert.ok(ui.doc.activeElement.classList.contains('lcc-k-tip-quiet'), 'and its mouse-over text stays away');
    // no filter, no box
    assert.equal(ui.doc.querySelectorAll('.lcc-bd-pick:checked').length, 0, 'ticks nothing');
    assert.equal(ui.doc.querySelectorAll('.lcc-k-strip [data-lk="chips"] .lcc-k-chip').length, 0, 'no filter chip');
    assert.equal(ui.posted.length, 0, 'nothing is written');
  });

  test(`#1147 (${layout}): a second Find House moves the panel to the new House; finding the chosen House keeps it chosen`, async () => {
    const ui = await open(layout);
    find(ui, 'Sor');
    find(ui, 'Wallach');
    assert.match(text(insp(ui)), /Wallach/);
    find(ui, 'Wallach');
    assert.ok(insp(ui), 'finding the chosen House does not let it go (a click on it would)');
    assert.match(text(insp(ui)), /Wallach/);
    assert.equal(marked(ui).length, 1);
  });
}

test('#1147: in the List a House the filter hides has no row: Find House does nothing, as before (it opens no panel and changes no filter)', async () => {
  const ui = await open('list');
  click(ui, '[data-fk="bf:hs:taken"]'); // Show: Pledged
  const shown = new Set([...ui.doc.querySelectorAll('tr.lcc-bd-row')].map((r) => r.getAttribute('data-id')));
  assert.ok(shown.size > 0 && shown.size < 25, 'the filter hides some Houses: ' + shown.size);
  click(ui, '[data-lk="findbtn"]');
  const hidden = [...ui.doc.querySelectorAll('[data-lk="fpick"]')].find((n) => !shown.has(n.getAttribute('data-id')));
  assert.ok(hidden, 'a House that is not in the list');
  click(ui, hidden);
  assert.equal(insp(ui), null, 'no panel opened');
  assert.equal(marked(ui).length, 0, 'no outline');
  assert.equal(ui.scrolled.length, 0, 'no scroll');
  assert.equal(ui.doc.querySelector('[data-fk="bf:hs:taken"]').getAttribute('aria-checked'), 'true', 'the filter is as it was');
});

test('#1147: the mouse-over text of Find House says it goes to the House and opens it in House Intelligence', async () => {
  const ui = await open('grid');
  const keys = [...ui.doc.querySelectorAll('[data-lk="findbtn"]')];
  assert.equal(keys.length, 2, 'the strip key and the action bar key');
  for (const k of keys) {
    const t = text(k.closest('.lcc-k-tipw, .lcc-k-fwrap, span, section') || k.parentNode);
    assert.match(t, /Find House: goes to one House and opens it in House Intelligence/, 'the tip: ' + t);
    assert.doesNotMatch(t, /scrolls the board/);
  }
  assert.doesNotMatch(ui.doc.body.textContent, /scrolls the board to one House/);
});
