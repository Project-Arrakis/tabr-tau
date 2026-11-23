// #1145: the specialization emblem of the House card (the same picture, from the same helper) also stands beside the specialization line of House Intelligence and to the left of the name in the List's
// Specialization column. The words, the Level text, the link and the column stay. A House with no track keeps the dash and gets no icon. jsdom has no layout: the DOM and the style file are pinned here,
// the widths at 1280 px are in the pull request.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { boot, closeAll } from './boot.mjs';
import { synthetic } from './lcc-fixture.mjs';

afterEach(closeAll);

const here = path.dirname(fileURLToPath(import.meta.url));
const CSS = fs.readFileSync(path.join(here, '..', 'internal', 'web', 'static', 'lcc', 'lcc-board.css'), 'utf8').replace(/\/\*[\s\S]*?\*\//g, '');
const text = (el) => el.textContent.replace(/\s+/g, ' ').trim();
const click = (ui, el) => { if (typeof el === 'string') el = ui.doc.querySelector(el); assert.ok(el, 'no element'); el.focus(); el.dispatchEvent(new ui.w.MouseEvent('click', { bubbles: true, cancelable: true })); };
const idOf = (a, name) => a.tasks.find((t) => t.house_name === 'DA_House' + name).id;
async function open(layout, a = synthetic()) {
  const ui = await boot({ get: { '/api/landsraad': a }, post: {}, local: { tab: 'landsraad', 'lsb.view': 'board', 'lsb.layout': layout } });
  await ui.settle();
  return ui;
}

test('#1145: House Intelligence shows the emblem of the path beside the specialization line, the same picture as the card', async () => {
  const a = synthetic();
  const ui = await open('list', a);
  click(ui, `[data-fk="ht:${idOf(a, 'Wallach')}"]`);
  const line = ui.doc.querySelector('.lcc-bd-insp .lcc-bd-spec-line');
  assert.ok(line, 'the specialization line');
  const img = line.querySelector('img.lcc-bd-spec-img');
  assert.ok(img, 'the emblem');
  assert.match(img.getAttribute('src'), /emblems\/exploration\.png$/, img.getAttribute('src'));
  assert.equal(img.getAttribute('alt'), '', 'a picture for the eye: the words beside it name the path');
  assert.equal(img.getAttribute('width'), '28');
  // the line, the Level text and the link behaviour are as they were
  const go = line.querySelector('.lcc-bd-gopath');
  assert.equal(text(go), 'Exploration Specialization');
  assert.equal(go.getAttribute('data-lcb'), 'gopath');
  assert.equal(go.getAttribute('data-track'), 'exploration');
  assert.match(text(line.querySelector('.lcc-bd-lvl')), /^Level /);
  assert.equal(line.firstElementChild, img, 'the emblem is to the left of the line');
  assert.equal(line.lastElementChild.className, 'lcc-bd-spec-txt', 'the link and the Level text stay together in one block beside it');
  assert.equal(line.querySelectorAll('img').length, 1, 'one picture');
  // the same picture as the card of the grid
  const grid = await open('grid', a);
  const card = grid.doc.querySelector(`li.lcc-bd-cell[data-id="${idOf(a, 'Wallach')}"] .lcc-hc-spec-img`);
  assert.equal(card.getAttribute('src'), img.getAttribute('src'), 'the card and the panel draw the same file');
});

test('#1145: the List\'s Specialization column has the small emblem to the left of the name; the words stay; the column keeps its single text node', async () => {
  const a = synthetic();
  const ui = await open('list', a);
  const rows = [...ui.doc.querySelectorAll('tr.lcc-bd-row')];
  assert.equal(rows.length, 25);
  let withIcon = 0;
  for (const r of rows) {
    const td = r.querySelector('td.lcc-bd-c-path');
    const img = td.querySelector('img');
    const word = text(td);
    if (word === '—') { assert.equal(img, null, 'a House with no track: the dash and no icon'); continue; }
    withIcon++;
    assert.ok(img && img.classList.contains('lcc-bd-spec-img-s'), 'a small emblem: ' + word);
    assert.equal(img.getAttribute('width'), '20');
    assert.equal(img.getAttribute('alt'), '');
    assert.ok(img.getAttribute('src').endsWith('emblems/' + word.toLowerCase() + '.png'), img.getAttribute('src'));
    assert.equal(td.firstChild, img, 'to the left of the name');
    assert.equal(td.querySelectorAll('*').length, 1, 'nothing else in the cell: no new element to wrap');
  }
  assert.ok(withIcon > 0, 'some Houses have a track');
  const head = [...ui.doc.querySelectorAll('.lcc-bd-list th')].find((t) => /^Specialization/.test(text(t)));
  assert.ok(head && !head.hasAttribute('aria-sort') && !head.querySelector('button'), 'the column heading and its (non-)sort are as they were');
});

test('#1145: a House with no track keeps the dash and gets no icon in the panel and in the list', async () => {
  const a = synthetic();
  const t = a.tasks.find((x) => x.house_name === 'DA_HouseSor');
  t.line = '';
  const ui = await open('list', a);
  const row = ui.doc.querySelector(`tr.lcc-bd-row[data-id="${t.id}"]`);
  assert.equal(text(row.querySelector('td.lcc-bd-c-path')), '—');
  assert.equal(row.querySelector('td.lcc-bd-c-path img'), null);
  click(ui, `[data-fk="ht:${t.id}"]`);
  assert.equal(ui.doc.querySelector('.lcc-bd-insp .lcc-bd-spec-img'), null, 'no icon in the panel');
  assert.equal(ui.doc.querySelector('.lcc-bd-insp .lcc-bd-spec-line'), null, 'and no line: the dash stands as it did');
  assert.equal(text(ui.doc.querySelector('.lcc-bd-insp .lcc-bd-bloc')), '—');
});

test('#1145: the style sheet draws the emblem in the card\'s tint with no literal colour, keeps the touch floor of the line and keeps the one word and its icon together', () => {
  const rules = [...CSS.matchAll(/([^{}]+)\{([^{}]*)\}/g)].map((m) => [m[1].trim(), m[2]]);
  const rule = (sel) => { const r = rules.find(([s]) => s === sel); assert.ok(r, 'rule ' + sel); return r[1]; };
  assert.match(rule('.lcc-bd-spec-img'), /filter: var\(--lcc-crest-filter\)/, 'the same tint token as the card');
  assert.match(rule('.lcc-bd-spec-img'), /width: 28px; height: 28px/);
  assert.match(rule('.lcc-bd-spec-img-s'), /width: 20px; height: 20px/);
  for (const s of ['.lcc-bd-spec-img', '.lcc-bd-spec-img-s', '.lcc-bd-spec-line']) assert.doesNotMatch(rule(s), /#[0-9a-f]{3,8}\b|rgb\(|hsl\(/i, 'no literal colour in ' + s);
  assert.match(rule('.lcc-bd-spec-img[hidden]'), /display: none/, 'a picture that fails to load is hidden');
  assert.match(CSS, /\.lcc-bd-list td\.lcc-bd-c-path \{ white-space: nowrap; \}/);
  // the line keeps the 36 px (44 px on touch) of the link: the gopath rules are not touched
  assert.match(rule('.lcc-bd-gopath'), /min-height: 36px/);
  assert.match(CSS, /\.lcc-bd-gopath, \.lcc-bd-loc \{ min-height: 44px; \}/);
});
