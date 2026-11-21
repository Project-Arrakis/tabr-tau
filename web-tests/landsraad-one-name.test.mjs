// #1099: one name for each thing on the Command Center board. A tier is 'earned' (not 'reached'; the state word 'Not reached' stays), the House hover card says 'Place' (not 'Region') and carries no
// 'experimental' note. Real app.js and lcc scripts over the synthetic answer, the Grid and the List layout.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import { boot, closeAll } from './boot.mjs';
import { synthetic } from './lcc-fixture.mjs';

afterEach(closeAll);
const text = (e) => (e ? e.textContent.replace(/\s+/g, ' ').trim() : '');
// every word a person can read or hear on the page part: its text and the accessible names and hover texts inside it
const said = (root) => [text(root), ...[...root.querySelectorAll('[aria-label],[title]')].flatMap((e) => [e.getAttribute('aria-label'), e.getAttribute('title')])].filter(Boolean).join(' | ');

for (const layout of ['grid', 'list']) {
  test('Command Center board, ' + layout + ' layout: earned (never reached), Place (never Region) and no experimental note in the House cards, the tier boxes and the counts', async () => {
    const ui = await boot({ get: { '/api/landsraad': synthetic() }, local: { tab: 'landsraad', 'lsb.layout': layout } });
    await ui.settle();
    const board = ui.doc.querySelector('#lsb-lcc');
    const said_ = said(board);
    const cards = [...board.querySelectorAll('.hcard')];
    assert.ok(cards.length > 0, 'the House cards are drawn');
    const seen = text(board), left = seen.replace(/not reached/gi, '').match(/.{0,60}[ ]reached.{0,60}/gi);
    assert.equal(left, null, 'the words of the board say earned, left: ' + JSON.stringify(left));
    const labels = [...board.querySelectorAll('[data-lb="tier"]')].map((t) => t.getAttribute('aria-label')); // the List layout has no tier boxes
    assert.ok((layout === 'list' || labels.length > 0) && labels.every((l) => !/, reached[, ]/i.test(l)), 'the tier boxes say earned: ' + labels.find((l) => /, reached[, ]/i.test(l)));
    assert.doesNotMatch(text(ui.doc.querySelector('#lsb')).replace(/not reached/gi, ''), /tiers? reached/i, 'the counts say earned');
    assert.doesNotMatch(said_, /experimental/i, 'no experimental note on the card or its button');
    assert.ok(/\bearned\b/i.test(said_), 'the tier words are earned');
    for (const c of cards) {
      assert.ok(![...c.querySelectorAll('dt')].some((d) => /region/i.test(text(d))), 'the card has no Region label');
      assert.doesNotMatch(text(c), /Experimental card/i);
    }
    const places = [...board.querySelectorAll('.hcard dt')].map((d) => text(d));
    assert.ok(places.includes('Place'), 'the Place label is there: ' + places.join(','));
  });
}
