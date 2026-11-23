// Back up now (#1168): the button in Database > Save backups, its confirm line, the one request it sends, the messages for success and for a refusal, and the tag in the list.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import { boot, closeAll } from './boot.mjs';

afterEach(closeAll);

const text = (el) => el.textContent.replace(/\s+/g, ' ').trim();
const ROWS = [
  { name: 'game-20261123-101500Z-000000001-manual.db', size: 2048, modified: '2026-11-23T10:15:00Z', copy: false, manual: true },
  { name: 'game-20261123-100000Z-000000001-auto.db', size: 2048, modified: '2026-11-23T10:00:00Z', copy: false },
];
const ANSWER = { ok: true, name: 'game-20261123-101500Z-000000002-manual.db', size: 2048, sha256: 'ab12cd34ef56' + '0'.repeat(52), at: '2026-11-23T10:15:00Z' };

async function openBackups(extra = {}) {
  const ui = await boot({ get: { '/api/save/backups': ROWS }, ...extra });
  ui.click('[data-tab="db"]'); await ui.settle();
  ui.click('[data-sub="db:backups"]'); await ui.settle();
  return ui;
}
const sent = (ui) => ui.posted.filter((p) => p.path === '/api/save/backup-now');

test('the Save backups card has a Back up now button, and the list tags a manual backup', async () => {
  const ui = await openBackups();
  const b = ui.doc.querySelector('[data-act="backupNow"]');
  assert.ok(b, 'the button is there');
  assert.equal(text(b), 'Back up now');
  const trs = [...ui.doc.querySelectorAll('tbody tr')];
  assert.equal(trs.length, 2);
  assert.ok(text(trs[0]).includes('manual backup'), text(trs[0]));
  assert.ok(!text(trs[1]).includes('manual backup'), text(trs[1]));
  assert.ok(ui.doc.querySelector('[data-act="restore"][data-n="' + ROWS[0].name + '"]'), 'Restore is on the manual backup like on any other');
});

test('the button asks first, says that the game may be running and that the save is not changed, and sends nothing before OK', async () => {
  const ui = await openBackups();
  ui.click('[data-act="backupNow"]'); await ui.settle();
  const q = text(ui.doc.querySelector('#ask'));
  assert.ok(q.includes('as it is now') && q.includes('the game is running') && q.includes('what the game last wrote') && q.includes('Nothing in the save is changed'), q);
  assert.equal(sent(ui).length, 0, 'nothing is sent by the first press');
  ui.doc.dispatchEvent(new ui.w.KeyboardEvent('keydown', { key: 'Escape', bubbles: true })); await ui.settle();
  assert.equal(sent(ui).length, 0, 'Cancel sends nothing');
});

test('OK sends one request, however often the button is pressed, and the toast names the backup, its size and its checksum', async () => {
  const ui = await openBackups({ post: { '/api/save/backup-now': ANSWER } });
  ui.click('[data-act="backupNow"]'); await ui.settle();
  ui.click('[data-act="backupNow"]'); await ui.settle(); // a second press while the question is open
  ui.click('[data-act="askOk"]'); await ui.settle();
  assert.equal(sent(ui).length, 1);
  assert.deepEqual(sent(ui)[0].body, {});
  const t = text(ui.doc.querySelector('#toast'));
  assert.ok(t.includes(ANSWER.name) && t.includes('2 KB') && t.includes('SHA-256 ab12cd34ef56') && t.includes('Manual backup taken'), t);
  assert.equal(ui.doc.querySelector('#toast .err'), null, 'a success is not shown as an error');
  assert.ok(ui.fetched.filter((p) => p === '/api/save/backups').length >= 2, 'the list is read again');
});

test('a refused backup shows the reason as an error and sends one request', async () => {
  const ui = await openBackups({ post: { '/api/save/backup-now': { __fail: 'the save file changed while it was being copied (the game wrote to it); nothing was kept, try again', __code: 'write_failed', __status: 500 } } });
  ui.click('[data-act="backupNow"]'); await ui.settle();
  ui.click('[data-act="askOk"]'); await ui.settle();
  assert.equal(sent(ui).length, 1);
  const el = ui.doc.querySelector('#toast .err');
  assert.ok(el, 'an error toast');
  assert.ok(text(el).includes('nothing was kept'), text(el));
  // and it can be tried again
  ui.click('[data-act="backupNow"]'); await ui.settle();
  ui.click('[data-act="askOk"]'); await ui.settle();
  assert.equal(sent(ui).length, 2);
});
