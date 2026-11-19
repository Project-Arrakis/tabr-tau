// #980: the Help tab. The pages of docs/help are embedded in the program and read from its own /api/help (these tests use the real answer of the Go server, from the fixtures), drawn with
// createElement and textContent only, and work with no network. The standing explanations that moved there are gone from the Command Center. Real app.js, help.js and lcc scripts, the synthetic
// Landsraad answer; no game save is read.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import { boot, closeAll } from './boot.mjs';
import { synthetic } from './lcc-fixture.mjs';

afterEach(closeAll);
const text = (el) => (el ? el.textContent.replace(/\s+/g, ' ').trim() : '');
async function open(extra = {}, local = {}) {
  const ui = await boot(Object.assign({ get: { '/api/landsraad': synthetic() }, local: Object.assign({ tab: 'help' }, local) }, extra));
  await ui.settle();
  return ui;
}
const click = (ui, el) => { if (typeof el === 'string') el = ui.doc.querySelector(el); assert.ok(el, 'no element'); el.dispatchEvent(new ui.w.MouseEvent('click', { bubbles: true, cancelable: true })); };
const PAGES = [{ id: 'index', file: 'README.md', title: 'Help', body: '# Help\n\n| Page | What |\n|---|---|\n| [Board](board.md) | The board |\n\nSee [the board](board.md) and [elsewhere](https://example.invalid/x).\n' },
  { id: 'board', file: 'board.md', title: 'The board', body: '# The board\n\n## Marks\n\n- **Collected** is `✓`\n- Not <b>HTML</b>\n\n1. one\n2. two\n' }];

test('the Help tab is in the navigation and shows the embedded pages: the contents first, then one button per page', async () => {
  const ui = await open();
  const nav = [...ui.doc.querySelectorAll('#nav button')].map(text);
  assert.ok(nav.includes('Help'), nav.join(','));
  assert.equal(ui.doc.querySelector('#nav button[data-tab="help"]').getAttribute('aria-current'), 'page');
  const pages = [...ui.doc.querySelectorAll('.helpnav [data-help]')].map(text);
  assert.equal(pages[0], 'Contents');
  for (const p of ['The Landsraad board', 'Fill, Complete and tiers', 'Term and Decrees', 'Specializations', 'Checked in the game']) assert.ok(pages.includes(p), 'a page button: ' + p);
  assert.match(text(ui.doc.querySelector('#helpbody')), /This is the Help of the editor/);
  assert.equal(ui.doc.title, 'Help - Tabr Tau');
});

test('it works offline: the only request is the editor\'s own /api/help, no network address is fetched, and nothing is posted', async () => {
  const ui = await open();
  click(ui, '.helpnav [data-help="term-and-decrees"]');
  await ui.settle();
  assert.ok(ui.fetchedUrls.includes('/api/help'));
  assert.equal(ui.fetchedUrls.filter((u) => u === '/api/help').length, 1, 'read once, not again for every page');
  assert.ok(ui.fetchedUrls.every((u) => u.startsWith('/api/') || u.startsWith('/')), ui.fetchedUrls.join(','));
  assert.equal(ui.posted.length, 0);
  assert.match(text(ui.doc.querySelector('#helpbody')), /Next term's decree/);
});

test('a page button opens its page, the last page is remembered, and a link between pages opens the other page', async () => {
  const ui = await open({ get: { '/api/landsraad': synthetic(), '/api/help': { pages: PAGES, repo: 'https://github.com/Project-Arrakis/tabr-tau/blob/main/docs/help/' } } });
  assert.equal(text(ui.doc.querySelector('.helpnav .on')), 'Contents');
  click(ui, '.helpbody .helplink[data-help="board"]');
  await ui.settle();
  assert.equal(text(ui.doc.querySelector('.helpnav .on')), 'The board');
  assert.equal(text(ui.doc.querySelector('#helpbody h3')), 'The board');
  assert.equal(ui.w.localStorage.getItem('help.page'), 'board');
  const again = await open({ get: { '/api/landsraad': synthetic(), '/api/help': { pages: PAGES, repo: 'https://github.com/Project-Arrakis/tabr-tau/blob/main/docs/help/' } } }, { 'help.page': 'board' });
  assert.equal(text(again.doc.querySelector('#helpbody h3')), 'The board');
});

test('the page is drawn as text: markup in a page stays text, a table and lists are drawn, a link to an address is only its words', async () => {
  const ui = await open({ get: { '/api/landsraad': synthetic(), '/api/help': { pages: PAGES, repo: 'https://github.com/Project-Arrakis/tabr-tau/blob/main/docs/help/' } } });
  const b = ui.doc.querySelector('#helpbody');
  assert.ok(b.querySelector('.helptable table th'), 'a table');
  assert.equal(text(b.querySelector('.helptable [data-help]')), 'Board');
  assert.equal(b.querySelector('a'), null, 'an address is never a link');
  assert.match(text(b), /elsewhere/);
  assert.doesNotMatch(b.innerHTML, /example\.invalid/);
  click(ui, '.helpnav [data-help="board"]');
  await ui.settle();
  const p = ui.doc.querySelector('#helpbody');
  assert.equal(p.querySelectorAll('li').length, 4);
  assert.equal(p.querySelector('b'), null, 'no element from the page text');
  assert.match(text(p), /Not <b>HTML<\/b>/, 'the markup is shown as it was written');
  assert.equal(text(p.querySelector('strong')), 'Collected');
  assert.equal(text(p.querySelector('code')), '✓');
  assert.equal(p.querySelector('ol').children.length, 2);
});

test('hostile text in a page is text: no element, no handler, no link, nothing is run', async () => {
  const bad = '# <img src=x onerror="window.__hit=1">\n\n<script>window.__hit=1</script> [x](javascript:window.__hit=1) [y](unknown.md) **<i>b</i>** `<u>`\n\n| <a> | b |\n|---|---|\n| <svg onload="window.__hit=1"> | [z](javascript:1) |\n';
  const ui = await open({ get: { '/api/landsraad': synthetic(), '/api/help': { pages: [{ id: 'index', file: 'README.md', title: 'Help', body: bad }], repo: 'https://github.com/Project-Arrakis/tabr-tau/blob/main/docs/help/' } } });
  const b = ui.doc.querySelector('#helpbody');
  assert.equal(b.querySelector('img, script, svg, a, i, u'), null);
  assert.equal(ui.w.__hit, undefined);
  assert.match(text(b), /<img src=x/);
  assert.match(text(b), /<script>/);
});

test('"Also on GitHub" is a plain link to the same page, drawn only for the project\'s own address', async () => {
  const ui = await open();
  const a = ui.doc.querySelector('.helpgh a');
  assert.ok(a);
  assert.equal(text(a), 'Also on GitHub');
  assert.equal(a.getAttribute('href'), 'https://github.com/Project-Arrakis/tabr-tau/blob/main/docs/help/README.md');
  assert.equal(a.getAttribute('rel'), 'noopener noreferrer');
  const evil = await open({ get: { '/api/landsraad': synthetic(), '/api/help': { pages: PAGES, repo: 'https://evil.example/docs/help/' } } });
  assert.equal(evil.doc.querySelector('.helpgh'), null, 'no link to another address');
});

// ---- what moved out of the Command Center -------------------------------------------------------------------------------------------------------------------------------------------------------
const GONE = ['About Fill and the tiers', 'What the marks mean', 'Important: three separate states', 'three separate states', 'How each click is written', 'What the colours of a House mean',
  'Writes at once, with a backup first', 'a raise also pays its missions', 'Not checked in the game yet', 'Each click is written at once with a backup first',
  'The decree in force, who reigns', 'NOT IN THE SAVE', 'Active Landsraad missions are not recorded', 'A ranking of contributors is not recorded', 'Current term only. Apply sets both together',
  'are read from the current term. The pool flag', 'Play clock', 'Five disciplines. One progression dossier'];
async function board() {
  const ui = await open({}, { tab: 'landsraad', 'lsb.view': 'board', 'lsb.layout': 'grid' });
  click(ui, ui.doc.querySelector('.lcc-hc .lcc-hc-select'));
  await ui.settle();
  return ui;
}
function standing(ui) {
  const root = ui.doc.querySelector('#lsb');
  const t = [text(root)];
  for (const n of root.querySelectorAll('[title],[aria-label]')) t.push(n.getAttribute('title') || '', n.getAttribute('aria-label') || '');
  return t.join(' | ');
}

test('the Landsraad board and its House panel hold none of the standing explanations that moved to the Help, in text, tooltip or screen-reader text', async () => {
  const ui = await board();
  assert.ok(ui.doc.querySelector('.lcc-bd-insp[data-id]'), 'a House is open');
  const all = standing(ui);
  for (const g of GONE) assert.ok(!all.includes(g), 'still on the board: ' + g);
  assert.equal(ui.doc.querySelectorAll('#lsb details.lcc-bd-disc').length, 0, 'no disclosure left in the House panel');
  assert.equal(ui.doc.querySelector('#lsb-legend'), null);
  assert.equal(ui.doc.querySelector('#lsb .lcc-bd-legend'), null);
  assert.equal(ui.doc.querySelector('#lsb .lcc-bd-note'), null);
  assert.equal(ui.doc.querySelector('#lsb .lcc-bd-tip'), null);
  assert.equal(ui.doc.querySelector('#lcc-bd-s4d'), null, 'the sentence for a screen reader under ACTIONS is gone');
  assert.equal(ui.doc.querySelector('#lcc-bd-s4').getAttribute('title'), null);
});

test('what stays: the short labels, the key to the board, the question before a write and the refusals', async () => {
  const ui = await board();
  assert.equal(ui.doc.querySelector('.lcc-bd-etip'), null, 'the standing line under Set tier is a hover text (#1062)');
  assert.match(ui.doc.querySelector('label[for="lcc-bd-tierset"]').getAttribute('title'), /favor won ≠ earned ≠ collected/i);
  assert.ok(ui.doc.querySelector('.lcc-bd-headlines[aria-label="Key to the board"]'));
  assert.ok(ui.doc.querySelector('.lcc-bd-act[data-do="fill"]'));
  const t = ui.doc.querySelector('.lcc-bd-insp .lcc-bd-tier[data-lb="tier"]');
  assert.ok(t.getAttribute('title').length > 10, 'the tooltip of a tier box stays');
});

test('the classic list has no legend line and no "Not checked in the game yet. Going back:" paragraph either', async () => {
  const ui = await open({}, { tab: 'landsraad', 'lsb.view': 'classic' });
  const t = text(ui.doc.querySelector('#lsb'));
  for (const d of ui.doc.querySelectorAll('#lsb details')) d.open = true;
  const all = text(ui.doc.querySelector('#lsb'));
  assert.doesNotMatch(all, /Writes at once, with a backup first|Not checked in the game yet\. Going back|Play clock/);
  assert.equal(ui.doc.querySelector('#lsb-legend'), null);
  assert.ok(t.length > 100);
});

test('the Specialization view has no lead sentence, no independence line, no standing operations line and no marks line', async () => {
  const ui = await open({}, { tab: 'player', 'sub.player': 'specialization' });
  ui.click('[data-sub="player:specialization"]');
  await ui.settle();
  const root = ui.doc.querySelector('#main');
  const all = text(root);
  for (const g of ['Five disciplines. One progression dossier', 'XP cap and keystones are independent', 'Every change asks first, is written after a backup', 'Reference marks along the 100 levels. They are not unlock levels.']) assert.ok(!all.includes(g), g);
});
