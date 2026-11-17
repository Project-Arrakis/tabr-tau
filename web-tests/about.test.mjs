// The version label in the header and the About panel (#806, slice 1: the version stamp only). The label reads the editor's own /api/about; the panel shows the version, build,
// commit, date and channel, and the release notes when a notes.md sits next to the editor. Nothing is fetched from the network, and every string is text, never markup.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import { boot, closeAll } from './boot.mjs';

afterEach(closeAll);

const RELEASE = { version: '0.14.2', label: 'v0.14.2', line: 'v0.14.2 (build 317, a1b2c3d)', build: '317', commit: 'a1b2c3d', date: '2026-11-16', channel: 'stable', arch: 'windows/amd64', dev: false };
const DEV = { version: '0.14.2+5', label: 'v0.14.2+5', line: 'v0.14.2+5 (build 322, e4f5a6b)', build: '322', commit: 'e4f5a6b', date: '2026-11-17', channel: 'stable', arch: 'windows/amd64', dev: true };
const UNSTAMPED = { version: 'dev', label: 'dev', line: 'dev', build: '', commit: '', date: '', channel: 'stable', arch: 'windows/amd64', dev: true };

const label = (ui) => ui.doc.querySelector('#header #verlabel, header #verlabel');

test('the header shows the version after the title, on every tab', async () => {
  const ui = await boot({ get: { '/api/about': RELEASE } });
  const b = label(ui);
  assert.ok(b, 'the header has a version button');
  assert.equal(b.hidden, false);
  assert.equal(b.textContent, 'v0.14.2');
  assert.equal(b.getAttribute('aria-label'), 'v0.14.2, about this version', 'the name starts with the words shown');
  assert.equal(b.getAttribute('aria-haspopup'), 'dialog');
  assert.equal(b.previousElementSibling.tagName, 'H1', 'it sits right after the title');
  ui.click('[data-tab="player"]'); await ui.settle();
  assert.equal(label(ui).textContent, 'v0.14.2', 'still there on another tab');
});

test('a development build shows its build metadata, an unstamped one shows dev', async () => {
  let ui = await boot({ get: { '/api/about': DEV } });
  assert.equal(label(ui).textContent, 'v0.14.2+5');
  closeAll();
  ui = await boot({ get: { '/api/about': UNSTAMPED } });
  assert.equal(label(ui).textContent, 'dev');
});

test('without an answer from /api/about the header is as it was: no label', async () => {
  const ui = await boot({ get: { '/api/about': { __fail: 'no', __status: 500 } } });
  assert.equal(label(ui).hidden, true);
});

test('the panel lists version, build, commit, date, channel and platform, and says there are no notes when there are none', async () => {
  const ui = await boot({ get: { '/api/about': { ...RELEASE, notes: '', notesTruncated: false, notesFile: 'notes.md' } } });
  ui.click('#verlabel'); await ui.settle();
  const dlg = ui.doc.querySelector('#ask [role="dialog"]');
  assert.ok(dlg, 'the label opens a dialog');
  assert.equal(dlg.getAttribute('aria-modal'), 'true');
  assert.equal(dlg.querySelector('h3').textContent, 'About Tabr Tau');
  const items = [...dlg.querySelectorAll('ul li')].map((li) => li.textContent);
  assert.deepEqual(items, ['Version: 0.14.2', 'Build: 317', 'Commit: a1b2c3d', 'Date: 2026-11-16', 'Channel: stable', 'Platform: windows/amd64']);
  const text = dlg.textContent;
  assert.ok(text.includes('This is v0.14.2 (build 317, a1b2c3d).'), 'the one line of the build');
  assert.ok(!text.includes('development build'), 'a release is not called a development build');
  assert.ok(text.includes('No release notes were found next to the editor.'));
  assert.equal(dlg.querySelector('pre'), null, 'no notes box when there are no notes');
  assert.equal(dlg.querySelector('ul').getAttribute('aria-label'), 'This build');
  // one Close button, no OK/Cancel pair
  const buttons = [...dlg.querySelectorAll('button')].map((b) => b.textContent);
  assert.deepEqual(buttons, ['Close']);
  assert.ok(ui.fetchedUrls.includes('/api/about?notes=1'), 'the notes are asked for when the panel opens');
  assert.equal(ui.posted.length, 0, 'opening the panel writes nothing');
});

test('the panel calls a development build one, and says what is not recorded', async () => {
  const ui = await boot({ get: { '/api/about': UNSTAMPED } });
  ui.click('#verlabel'); await ui.settle();
  const text = ui.doc.querySelector('#ask [role="dialog"]').textContent;
  assert.ok(text.includes('This is a development build, not a tagged release.'));
  assert.ok(text.includes('Build: not recorded') && text.includes('Commit: not recorded') && text.includes('Date: not recorded'));
  assert.ok(text.includes('Version: dev'));
});

test('release notes are shown as plain text in a labelled box, markup stays inert, a cut file says so', async () => {
  const notes = '## [0.14.2]\n- Fixed <img src=x onerror="window.pwned=1"> and <b>bold</b>.';
  const ui = await boot({ get: { '/api/about': { ...RELEASE, notes, notesTruncated: true, notesFile: 'notes.md' } } });
  ui.click('#verlabel'); await ui.settle();
  const dlg = ui.doc.querySelector('#ask [role="dialog"]');
  const pre = dlg.querySelector('pre');
  assert.ok(pre, 'the notes are in a text box');
  assert.equal(pre.getAttribute('aria-label'), 'Release notes');
  assert.ok(pre.textContent.startsWith('## [0.14.2]\n- Fixed <img src=x onerror="window.pwned=1"> and <b>bold</b>.'), 'the text is shown as it is');
  assert.ok(pre.textContent.includes('(cut: the file is longer than the editor shows)'));
  assert.equal(dlg.querySelectorAll('img, b').length, 0, 'no element was made from the notes');
  assert.equal(ui.w.pwned, undefined);
  assert.ok(dlg.textContent.includes('Release notes (notes.md, next to the editor):'));
});

test('Close and Escape end the panel and the focus goes back to the label; nothing behind was changed', async () => {
  const ui = await boot({ get: { '/api/about': RELEASE } });
  const b = label(ui);
  b.focus();
  ui.click('#verlabel'); await ui.settle();
  assert.ok(ui.doc.querySelector('#ask [role="dialog"]'));
  assert.equal(ui.doc.querySelector('header').hasAttribute('inert'), true, 'the page behind is inert while it is open');
  ui.click('#aok'); await ui.settle();
  assert.equal(ui.doc.querySelector('#ask [role="dialog"]'), null);
  assert.equal(ui.doc.querySelector('header').hasAttribute('inert'), false);
  assert.equal(ui.doc.activeElement, b, 'the focus returns to the label');
  ui.click('#verlabel'); await ui.settle();
  ui.doc.dispatchEvent(new ui.w.KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
  await ui.settle();
  assert.equal(ui.doc.querySelector('#ask [role="dialog"]'), null, 'Escape closes it');
});

test('if the notes cannot be read when the panel opens, it still shows the version it already had', async () => {
  const ui = await boot({ get: { '/api/about': RELEASE } });
  // the first read (no notes) worked; make the second fail
  const real = ui.w.fetch;
  ui.w.fetch = (p, o) => (String(p).includes('notes=1') ? Promise.resolve({ ok: false, status: 500, statusText: 'x', json: async () => ({ error: 'x' }) }) : real(p, o));
  ui.click('#verlabel'); await ui.settle();
  const dlg = ui.doc.querySelector('#ask [role="dialog"]');
  assert.ok(dlg && dlg.textContent.includes('Version: 0.14.2'));
});

test('the page makes no request to anywhere but the editor itself', async () => {
  const ui = await boot({ get: { '/api/about': RELEASE } });
  ui.click('#verlabel'); await ui.settle();
  const src = (await import('node:fs')).readFileSync(new URL('../internal/web/static/app.js', import.meta.url), 'utf8');
  assert.ok(!/https?:\/\//.test(src.split('---------- the version label')[1].split('status().then')[0]), 'the label and panel code names no web address');
  for (const u of ui.fetchedUrls) assert.ok(u.startsWith('/'), `${u} is the editor's own path`);
});
