// #1142: the words Atreides and Harkonnen under the two faction icons of the Landsraad action bar are gone. The accessible name (aria-label) and the mouse-over text stay, and so do the pressed
// state and the 44 px size of the seats. jsdom has no layout: the DOM and the style file are pinned here, the pictures are in the pull request.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { boot, closeAll } from './boot.mjs';
import { synthetic } from './lcc-fixture.mjs';

afterEach(closeAll);

const here = path.dirname(fileURLToPath(import.meta.url));
const STATIC = path.join(here, '..', 'internal', 'web', 'static');
const CSS = fs.readFileSync(path.join(STATIC, 'lcc', 'lcc-board.css'), 'utf8').replace(/\/\*[\s\S]*?\*\//g, '');
const text = (el) => el.textContent.replace(/\s+/g, ' ').trim();
async function open(layout = 'grid') {
  const ui = await boot({ get: { '/api/landsraad': synthetic() }, post: {}, local: { tab: 'landsraad', 'lsb.view': 'board', 'lsb.layout': layout } });
  await ui.settle();
  return ui;
}

test('#1142: the two faction seats of the action bar draw an icon, a lamp and the mouse-over text, and no visible word', async () => {
  for (const layout of ['grid', 'list']) {
    const ui = await open(layout);
    const seats = [...ui.doc.querySelectorAll('[data-lk="seat"]')];
    assert.equal(seats.length, 2, layout + ': two faction seats');
    for (const s of seats) {
      assert.deepEqual([...s.children].map((c) => c.className.split(' ')[0]), ['lcc-k-crest', 'lcc-k-lamp', 'lcc-k-tip'], layout + ': icon, lamp, mouse-over text, and no word element');
      assert.equal(s.querySelector('.lcc-k-sl'), null);
      const visible = s.cloneNode(true);
      for (const n of visible.querySelectorAll('.lcc-k-tip, [aria-hidden="true"]')) n.remove();
      assert.equal(text(visible), '', 'no word is drawn by the seat itself');
      // what stays: the accessible name, the mouse-over text, the pressed state, and the roving tab stop
      assert.match(s.getAttribute('aria-label'), /^(Atreides|Harkonnen)/, 'the accessible name names the faction');
      assert.match(text(s.querySelector('.lcc-k-tip')), /^(Atreides|Harkonnen).* Fill and Complete act for this faction\./);
      assert.ok(s.hasAttribute('aria-checked') && s.getAttribute('role') === 'radio');
    }
    assert.deepEqual(seats.map((s) => s.getAttribute('aria-checked')).sort(), ['false', 'true'], 'one seat is pressed');
    // the seats are only in the action bar: the filter strip, the cards and the panel have no faction icon with a word under it
    for (const s of seats) assert.ok(s.closest('.lcc-k-cmd'), 'in the action bar');
    assert.equal(ui.doc.querySelectorAll('.lcc-k-strip [data-lk="seat"]:not(.lcc-k-cmd *)').length, 0);
    assert.equal(ui.doc.querySelectorAll('.lcc-k-sl').length, 0, layout + ': no seat label anywhere on the page');
    ui.w.close();
  }
});

test('#1142: choosing the other seat still sets the faction (the pressed state moves, nothing is written)', async () => {
  const ui = await open('grid');
  const [a, b] = [...ui.doc.querySelectorAll('[data-lk="seat"]')];
  const off = a.getAttribute('aria-checked') === 'true' ? b : a;
  off.dispatchEvent(new ui.w.MouseEvent('click', { bubbles: true, cancelable: true }));
  const now = [...ui.doc.querySelectorAll('[data-lk="seat"]')].filter((s) => s.getAttribute('aria-checked') === 'true').map((s) => s.getAttribute('data-fid'));
  assert.deepEqual(now, [off.getAttribute('data-fid')]);
  assert.equal(ui.posted.length, 0);
});

test('#1142: the style sheet has no rule left for the seat word or for the room under the seats, and the seat keeps its 44 px', () => {
  assert.doesNotMatch(CSS, /\.lcc-k-sl\b/, 'no rule for the removed word');
  assert.doesNotMatch(CSS, /\.lcc-k-seats \{ margin-bottom/, 'no room is kept under the seats for a word that is not drawn');
  assert.match(CSS, /\.lcc-k-seat \{[^}]*width: 44px; height: 44px;/, 'the touch size of the seat is as it was');
  const app = fs.readFileSync(path.join(STATIC, 'app.js'), 'utf8');
  assert.doesNotMatch(app, /lcc-k-sl/, 'the page does not draw it either');
});
