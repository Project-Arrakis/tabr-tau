// Escaping by construction (audit finding F-03 / #4: stored XSS from save contents).
//
// html`...${value}...` escapes every interpolated value unless it is itself the result of html`` (a Safe), an
// array (each element is treated the same way), or null/undefined (empty). setHTML() is the ONLY place in the UI
// allowed to assign innerHTML, and it throws for anything that did not come from html``. Attribute values must
// always be quoted in templates: escaping covers & < > " ' and `.
(function (root) {
  'use strict';

  class Safe {
    constructor(s) { this.s = s; }
    toString() { return this.s; }
  }

  const MAP = { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;', '`': '&#96;' };
  const esc = (v) => (v == null ? '' : String(v).replace(/[&<>"'`]/g, (c) => MAP[c]));

  function ins(v) {
    if (v instanceof Safe) return v.s;
    if (Array.isArray(v)) return v.map(ins).join('');
    return esc(v);
  }

  function html(strings, ...vals) {
    let out = strings[0];
    for (let i = 0; i < vals.length; i++) out += ins(vals[i]) + strings[i + 1];
    return new Safe(out);
  }

  // raw marks a literal, trusted fragment. Use only for constants written in the source, never for data.
  const raw = (s) => new Safe(String(s));

  function setHTML(el, v) {
    if (!(v instanceof Safe)) throw new TypeError('setHTML needs html`...` output, got ' + typeof v);
    el.innerHTML = v.s;
  }

  const api = { Safe, esc, html, raw, setHTML };
  if (typeof module !== 'undefined' && module.exports) module.exports = api;
  else root.TabrHTML = api;
})(typeof window !== 'undefined' ? window : globalThis);
