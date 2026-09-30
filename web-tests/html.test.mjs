import test from 'node:test';
import assert from 'node:assert/strict';
import { createRequire } from 'node:module';
const require = createRequire(import.meta.url);
const { html, raw, setHTML, esc, Safe } = require('../internal/web/static/html.js');

const HOSTILE = `"><img src=x onerror=window.__pwned=1>'&<b>x</b>\`${'${7*7}'}`;

test('escapes every HTML-significant character, including backtick', () => {
  assert.equal(esc(`& < > " ' \``), '&amp; &lt; &gt; &quot; &#39; &#96;');
});

test('null and undefined render as empty; other primitives are stringified then escaped', () => {
  assert.equal(String(html`[${null}][${undefined}][${0}][${false}][${'<'}]`), '[][][0][false][&lt;]');
});

test('hostile text cannot break out of a quoted attribute or a text node', () => {
  const s = String(html`<input value="${HOSTILE}"><p>${HOSTILE}</p>`);
  assert.ok(!s.includes('<img'), s);
  assert.equal(s.split('<').length - 1, 3, 'only the template\'s own markup (<input, <p>, </p>) may contain <; none may come from the data: ' + s);
  assert.ok(s.includes('&quot;&gt;&lt;img src=x onerror=window.__pwned=1&gt;'), s);
});

test('nested html`` results are not escaped twice; arrays are flattened element by element', () => {
  const inner = html`<b>${'<i>'}</b>`;
  assert.equal(String(html`<p>${inner}</p>`), '<p><b>&lt;i&gt;</b></p>');
  assert.equal(String(html`<ul>${['<a>', html`<li>ok</li>`, null, 3]}</ul>`), '<ul>&lt;a&gt;<li>ok</li>3</ul>');
});

test('a plain string that looks like HTML is always escaped, only html`` output is trusted', () => {
  assert.equal(String(html`${'<script>alert(1)</script>'}`), '&lt;script&gt;alert(1)&lt;/script&gt;');
  const fake = { toString: () => '<script>alert(1)</script>' }; // duck-typed object must NOT pass as Safe
  assert.ok(!(fake instanceof Safe));
  assert.equal(String(html`${fake}`), '&lt;script&gt;alert(1)&lt;/script&gt;');
});

test('setHTML refuses anything that did not come from html``', () => {
  const el = { innerHTML: 'unchanged' };
  for (const bad of ['<b>x</b>', 42, null, undefined, {}, [], { s: '<b>', toString: () => '<b>' }, ['<b>']]) {
    assert.throws(() => setHTML(el, bad), TypeError);
    assert.equal(el.innerHTML, 'unchanged');
  }
  setHTML(el, html`<b>${'<ok>'}</b>`);
  assert.equal(el.innerHTML, '<b>&lt;ok&gt;</b>');
});

test('raw() is the only way to insert markup, and it is explicit', () => {
  assert.equal(String(html`${raw('<hr>')}${'<hr>'}`), '<hr>&lt;hr&gt;');
});
