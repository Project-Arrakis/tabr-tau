// The tier boxes of a House card keep the look they had before #1071 (#1082): earned and not collected is a gold box with a diamond, collected is the green check box, no delivery record is the
// question-mark box, not reached is the numbered box. #1071 asked only for the ledger rows of House Intelligence to change (teal edge and check, empty ring, dimmed); those rows are pinned in
// lcc-inspector-pass.test.mjs and are not touched here.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { boot, closeAll } from './boot.mjs';
import { synthetic } from './lcc-fixture.mjs';
import { lccDir } from './lcc-load.mjs';

afterEach(closeAll);

const text = (el) => el.textContent.replace(/\s+/g, ' ').trim();

test('each state of a card tier box has its own mark and class: check, diamond, question mark, number', async () => {
  const a = synthetic();
  a.tasks[7].rewards[0].reward_state = 'delivered';
  a.tasks[7].rewards[1].reward_state = 'no_row';
  const ui = await boot({ get: { '/api/landsraad': a }, local: { tab: 'landsraad', 'lsb.view': 'board' } });
  await ui.settle();
  const card = [...ui.doc.querySelectorAll('.lcc-bd-cell')].find((c) => c.querySelector('.lcc-hc-name').textContent.toLowerCase() === 'house wallach');
  const boxes = [...card.querySelectorAll('.lcc-hc-t')];
  const state = (b) => ['delivered', 'waiting', 'uncertain', 'off'].find((c) => b.classList.contains(c));
  assert.deepEqual(boxes.map((b) => [state(b), text(b)]), [['delivered', '✓'], ['uncertain', '?'], ['waiting', '◆'], ['off', '4'], ['off', '5']]);
  assert.doesNotMatch(card.textContent, /○ \d/, 'no ring count on the card');
  // the words a screen reader hears are the state words, as before
  assert.match(boxes[2].getAttribute('aria-label'), /not collected/);
  assert.match(boxes[0].getAttribute('aria-label'), /collected/);
});

test('the waiting card box is drawn as the gold box: the gold fill, edge and dark ink tokens, not an empty ring', () => {
  const css = fs.readFileSync(path.join(lccDir, 'lcc-board.css'), 'utf8').replace(/\r/g, '');
  const rule = css.split('\n').find((l) => l.startsWith('.lcc-hc-t.waiting {'));
  assert.ok(rule, 'the rule of the waiting box is there');
  assert.match(rule, /color: var\(--lcc-wait-ink\)/);
  assert.match(rule, /background: var\(--lcc-wait-bg\)/);
  assert.match(rule, /border-color: var\(--lcc-wait-edge\)/);
  assert.doesNotMatch(rule, /transparent/);
  assert.match(fs.readFileSync(path.join(lccDir, 'lcc.css'), 'utf8'), /--lcc-wait-bg: #f5c878; --lcc-wait-edge: #ffdc8a; --lcc-wait-ink: #22160a;/);
});
