// #1040: no surface of the page shows the game's decree id (SpecialVendorActive_Vehicles and the others) beside the decree's name and effect: the plaque's ACTIVE DECREE block, the Command Center Term
// and Decrees section (the facts, the pool, the drop-down line), the classic panel and every title or label. The name and the effect stay, each on a line of its own (so copied text does not run together).
// Real app.js and lcc scripts over the synthetic answer of M1; no game save is read.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { boot, closeAll } from './boot.mjs';
import { synthetic } from './lcc-fixture.mjs';
import { lccDir } from './lcc-load.mjs';

afterEach(closeAll);
const text = (el) => (el ? el.textContent.replace(/\s+/g, ' ').trim() : '');

async function open(view) {
  const ui = await boot({ get: { '/api/landsraad': synthetic() }, local: { tab: 'landsraad', 'lsb.view': view, 'lsb.layout': 'grid' } });
  await ui.settle();
  return ui;
}
const ids = () => synthetic().decrees.map((d) => d.decree_name).filter((n) => /_|[a-z][A-Z]/.test(n));

for (const view of ['board', 'classic']) {
  test(`${view} view: no decree id in the text, a title, a label or any attribute of the page (#1040)`, async () => {
    const ui = await open(view);
    const html = ui.doc.body.outerHTML;
    assert.ok(ids().length > 3, 'the synthetic save holds decrees with game ids');
    for (const id of ids()) {
      assert.ok(!ui.doc.body.textContent.includes(id), `${id} is not in the text of the page`);
      assert.ok(!html.includes(id), `${id} is not in any attribute of the page`);
    }
    assert.equal(ui.doc.querySelectorAll('.did, .lcc-tm-did').length, 0, 'no line for a game id');
  });
}

test('the active decree block of the plaque is the name and the effect, in two elements, and no id', async () => {
  const ui = await open('board');
  const t = ui.doc.querySelector('.lcc-plaque .lcc-pl-decree');
  assert.ok(t, 'the plaque has its decree block');
  assert.deepEqual([...t.children].map((c) => c.tagName + '.' + c.className), ['H3.lcc-pl-title', 'P.lcc-pl-effect'], 'the name and the effect are two block elements, nothing else');
  assert.equal(text(t.querySelector('.lcc-pl-title')), 'Exclusive Vehicle Import Rights');
  assert.equal(text(t.querySelector('.lcc-pl-effect')), 'Access to a Landsraad Vehicle Vendor');
});

test('the active decree fact of the Term and Decrees section is the name and the effect, each its own block, and no id', async () => {
  const ui = await open('board');
  const f = ui.doc.querySelector('#lcc-term [data-fact="active"], #lcc-term .lcc-tm-fact-decree');
  assert.ok(f, 'the active decree fact');
  const box = f.querySelector('.lcc-tm-dtext');
  assert.deepEqual([...box.children].map((c) => c.className), ['lcc-tm-v lcc-tm-dn', 'lcc-tm-de'], 'the name, then the effect: no third line for an id');
  assert.equal(text(box.querySelector('.lcc-tm-dn')), 'Exclusive Vehicle Import Rights');
  assert.equal(text(box.querySelector('.lcc-tm-de')), 'Access to a Landsraad Vehicle Vendor');
});

test('the name, the effect and the note are blocks of their own in every stylesheet that draws a decree', () => {
  const term = fs.readFileSync(path.join(lccDir, 'lcc-term.css'), 'utf8');
  const lcc = fs.readFileSync(path.join(lccDir, 'lcc.css'), 'utf8');
  const app = fs.readFileSync(new URL('../internal/web/static/app.css', import.meta.url), 'utf8');
  assert.match(term, /\.lcc-tm-v \{ display: block;/);
  assert.match(term, /\.lcc-tm-de \{ display: block;/);
  assert.match(lcc, /\.lcc-pl-decree \.lcc-pl-effect \{/, 'the plaque effect is a paragraph under the title');
  assert.match(app, /\.lsb \.de\{display:block/);
});
