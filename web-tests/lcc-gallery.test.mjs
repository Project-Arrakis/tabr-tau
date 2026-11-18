// The component gallery of the Command Center (M2) is a fixture page outside the editor: it is labelled as demo, built only from the helpers, and the editor never loads it.
// The page is run in jsdom with its own html file, so the test sees what a browser would build.
import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { JSDOM } from 'jsdom';
import { root, lccDir, read } from './lcc-load.mjs';

const gdir = path.join(root, 'docs', 'design', 'landsraad-command-center', 'gallery');
const lccJs = fs.readFileSync(path.join(lccDir, 'lcc-model.js'), 'utf8') + ';' + fs.readFileSync(path.join(lccDir, 'lcc.js'), 'utf8'); // the model holds the words of the Sysselraad; the page loads it first
const galleryJs = fs.readFileSync(path.join(gdir, 'gallery.js'), 'utf8');
const page = fs.readFileSync(path.join(gdir, 'index.html'), 'utf8').replace(/<script[^>]*src=[^>]*><\/script>/g, '');

function open(query = '') {
  const dom = new JSDOM(page, { url: 'http://127.0.0.1:8090/gallery/index.html' + query, runScripts: 'outside-only' });
  dom.window.eval(lccJs);
  dom.window.eval(galleryJs);
  return dom.window;
}

test('the gallery says it is fixture data, on the page and in every section', () => {
  const w = open();
  const d = w.document;
  assert.match(d.querySelector('.lccg-note').textContent, /FIXTURE \/ DEMO/);
  assert.match(d.title, /fixture data/);
  assert.equal(d.documentElement.getAttribute('data-gallery-ready'), 'true');
  const sections = [...d.querySelectorAll('section[data-scene]')];
  assert.deepEqual(sections.map((s) => s.getAttribute('data-scene')), ['plaques', 'lanes', 'houses', 'primitives', 'art']);
  for (const s of sections) assert.match(s.querySelector('h2 .lcc-badge').textContent, /FIXTURE/, s.getAttribute('data-scene'));
  assert.match(galleryJs, /^\/\/ FIXTURE \/ DEMO ONLY/);
});

test('the plaque section shows the six cases, each with 25 seats (or none for counts that cannot be read), and the 13th is gold', () => {
  const d = open().document;
  const figs = [...d.querySelectorAll('figure[data-case]')];
  assert.deepEqual(figs.map((f) => f.getAttribute('data-case')), ['a12h4', 'a13h12', 'a12h13', 'a6h10', 'a0h0', 'long', 'bad']);
  for (const f of figs) {
    const seats = f.querySelectorAll('.lcc-seat');
    const id = f.getAttribute('data-case');
    if (id === 'bad') { assert.equal(f.querySelectorAll('.lcc-seat-a, .lcc-seat-h').length, 0, 'nothing lit when the counts cannot be read'); continue; }
    assert.equal(seats.length, 25, id);
    assert.equal(f.querySelectorAll('.lcc-seat[data-center="true"]').length, 1, id);
    assert.equal(seats[12].getAttribute('data-center'), 'true', id + ': seat 13 is the gold one');
    assert.equal(f.querySelectorAll('.lcc-seat-halo').length, 1, id + ': only the 13th has the gold halo');
    assert.equal(f.querySelectorAll('.lcc-badge-demo').length >= 1, true, id + ' carries the demo badge');
  }
});

test('lanes: six fixture cards plus the unidentified one; houses: six states, a line of five and six majority Houses; the art section lists every picture', () => {
  const d = open().document;
  assert.equal(d.querySelectorAll('section[data-scene="lanes"] .lcc-lane').length, 7);
  assert.equal(d.querySelectorAll('section[data-scene="houses"] .lccg-grid-houses')[0].querySelectorAll('.lcc-house').length, 6);
  assert.equal(d.querySelectorAll('section[data-scene="houses"] .lccg-row5 .lcc-house[data-victory="circuit"]').length, 5);
  assert.equal(d.querySelectorAll('section[data-scene="houses"] .lccg-grid-houses')[1].querySelectorAll('.lcc-house[data-victory="dominion"]').length, 6);
  const imgs = [...d.querySelectorAll('section[data-scene="art"] img')];
  assert.equal(imgs.length, 25 + 2 + 10 + 4 + 12, 'houses + the two spellings + lanes and banners + the four faction pictures + decree ids');
  for (const i of imgs) assert.match(i.getAttribute('src'), /^\.\.\/\.\.\/\.\.\/\.\.\/internal\/web\/static\/lcc\/art\/(houses|lanes|banners|factions|decrees)\/[a-z-]+\.(webp|svg)$/);
});

test('every picture the gallery names is a file on disk', () => {
  const d = open().document;
  for (const i of d.querySelectorAll('img')) {
    const rel = i.getAttribute('src');
    assert.ok(fs.existsSync(path.resolve(gdir, rel)), rel);
  }
});

test('a scene in the address shows only that scene; the settings in the address are only ever picked from the lists', () => {
  const w = open('?scene=lanes');
  assert.deepEqual([...w.document.querySelectorAll('section[data-scene]')].map((s) => s.getAttribute('data-scene')), ['lanes']);
  const one = open('?scene=plaque-a13h12').document;
  assert.deepEqual([...one.querySelectorAll('figure[data-case]')].map((f) => f.getAttribute('data-case')), ['a13h12']);
  const set = open('?look=sietch&theme=light&palette=cold-slate&motion=off').document.documentElement;
  assert.equal(set.getAttribute('data-look'), 'sietch');
  assert.equal(set.getAttribute('data-theme'), 'light');
  assert.equal(set.getAttribute('data-palette'), 'cold-slate');
  assert.equal(set.getAttribute('data-motion'), 'off');
  const bad = open('?look=../../x&theme=%3Cscript%3E&palette=evil&motion=fast').document;
  assert.equal(bad.documentElement.getAttribute('data-look'), 'fremen');
  assert.equal(bad.documentElement.getAttribute('data-theme'), null);
  assert.equal(bad.documentElement.getAttribute('data-palette'), null);
  assert.equal(bad.documentElement.getAttribute('data-motion'), 'full');
  const hrefs = [...bad.querySelectorAll('link[data-look-link]')].map((l) => l.getAttribute('href'));
  assert.deepEqual(hrefs, ['../../../../design/assets/tokens-fremen.css', '../../../../design/assets/components-fremen.css']);
  const nothing = open('?scene=%3Cimg%20src=x%3E').document;
  assert.equal(nothing.querySelectorAll('section[data-scene]').length, 0);
  assert.equal(nothing.querySelectorAll('img[src="x"]').length, 0);
});

test('each of the three looks points at design files that exist', () => {
  for (const look of ['fremen', 'awakening', 'sietch']) {
    const d = open('?look=' + look).document;
    for (const l of d.querySelectorAll('link[data-look-link]')) assert.ok(fs.existsSync(path.resolve(gdir, l.getAttribute('href'))), l.getAttribute('href'));
  }
});

test('the gallery page uses no inline script, no inline style, no external address; and no editor file refers to it', () => {
  const raw = fs.readFileSync(path.join(gdir, 'index.html'), 'utf8');
  assert.doesNotMatch(raw.replace(/<script[^>]*\ssrc="[^"]*"[^>]*><\/script>/g, ''), /<script|\sstyle=|\son[a-z]+=|<style/i);
  assert.doesNotMatch(raw, /https?:\/\//);
  assert.match(raw, /Content-Security-Policy/);
  assert.doesNotMatch(galleryJs, /fetch\(|XMLHttpRequest|WebSocket|sendBeacon|localStorage|sessionStorage|document\.cookie|eval\(|new Function|innerHTML|insertAdjacentHTML/);
  const editorFiles = ['app.js', 'index.html'].map((f) => read('internal', 'web', 'static', f)).concat(fs.readdirSync(path.join(root, 'internal', 'web', 'static')).filter((f) => f.endsWith('.js') && f !== 'app.js').map((f) => read('internal', 'web', 'static', f)));
  for (const t of editorFiles) { assert.doesNotMatch(t, /component gallery|gallery\/index\.html|lccg-/); }
});
