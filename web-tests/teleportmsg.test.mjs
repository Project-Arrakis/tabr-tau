// The message after a teleport (#501) is the one the editor sends, in its own words for each state of the game; a warning (the move waits, or a running game will not follow it)
// stays on the screen until dismissed, and an answer without a message still says "Teleport set".
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import { boot, closeAll } from './boot.mjs';

afterEach(closeAll);

async function teleport(answer) {
  const ui = await boot({ post: { '/api/player/teleport': answer } });
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="player:admin"]'); await ui.settle();
  ui.click('[data-act="teleport"]'); await ui.settle();
  const t = ui.doc.querySelector('#toast');
  return { ui, text: t.textContent.replace(/\s+/g, ' ').trim(), warn: !!t.querySelector('div.part, div.err'), posted: ui.posted.filter((p) => p.path === '/api/player/teleport') };
}

test('a teleport shows the message the editor sent', async () => {
  const msg = 'Position saved. Press Continue to load it.';
  const r = await teleport({ ok: true, outcome: 'written', message: msg, warn: false });
  assert.equal(r.posted.length, 1);
  assert.ok(r.text.includes(msg), r.text);
  assert.ok(!r.text.includes('Teleport set'), r.text);
  assert.equal(r.warn, false);
});

test('a teleport while a world is running shows the warning, and it stays until dismissed', async () => {
  const msg = 'Position queued, but the running game keeps its own copy of the save: it will not move you now, and the game may overwrite it when it saves. Leave the world, wait about 30 seconds, then teleport and press Continue.';
  const r = await teleport({ ok: true, outcome: 'queued', message: msg, warn: true });
  assert.ok(r.text.includes(msg), r.text);
  assert.equal(r.warn, true);
});

test('an answer with no message still says Teleport set', async () => {
  const r = await teleport({ ok: true });
  assert.ok(r.text.includes('Teleport set'), r.text);
});
