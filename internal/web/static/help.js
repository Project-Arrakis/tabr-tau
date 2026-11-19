// The Help tab's page renderer (#980). The pages are the Markdown files of docs/help, embedded in the program and read from the editor's own /api/help: nothing is fetched from the network.
//
// What this file is: a small, safe Markdown reader. It builds elements with createElement and textContent only, so a page is never markup: a "<" in a page is a "<" on the screen, nothing is passed to
// innerHTML, and no raw HTML of a page is ever used. It reads headings (# ## ###), paragraphs, lists (- and 1.), tables, **bold**, `code` and links. A link to another Help page ([text](file.md)) becomes a
// button that opens that page inside the editor; any other link is shown as its text only, so a page cannot send the person anywhere. No dependency.
(function (root) {
  'use strict';

  const doc = () => root.document;
  function make(tag, cls, text) {
    const n = doc().createElement(tag);
    if (cls) n.className = cls;
    if (text !== undefined && text !== null && text !== '') n.textContent = text;
    return n;
  }
  // Inline: `code`, **bold** and [text](target). Everything else is text, exactly as written.
  const INLINE = /(`[^`]+`)|(\*\*[^*]+\*\*)|(\[[^\]]+\]\([^)\s]+\))/g;
  function inline(text, opts, into) {
    let at = 0, m;
    INLINE.lastIndex = 0;
    while ((m = INLINE.exec(text)) !== null) {
      if (m.index > at) into.appendChild(doc().createTextNode(text.slice(at, m.index)));
      const tok = m[0];
      if (m[1]) into.appendChild(make('code', '', tok.slice(1, -1)));
      else if (m[2]) into.appendChild(make('strong', '', tok.slice(2, -2)));
      else {
        const k = tok.indexOf('](');
        const label = tok.slice(1, k), target = tok.slice(k + 2, -1);
        const id = opts.files && Object.prototype.hasOwnProperty.call(opts.files, target) ? opts.files[target] : null;
        if (id) {
          const b = make('button', 'linkb helplink', label);
          b.type = 'button';
          b.setAttribute('data-help', id);
          into.appendChild(b);
        } else into.appendChild(doc().createTextNode(label));
      }
      at = m.index + tok.length;
    }
    if (at < text.length) into.appendChild(doc().createTextNode(text.slice(at)));
    return into;
  }
  const cells = (line) => line.trim().replace(/^\|/, '').replace(/\|$/, '').split('|').map((c) => c.trim());
  const isRule = (line) => /^\s*\|?\s*:?-{3,}:?\s*(\|\s*:?-{3,}:?\s*)*\|?\s*$/.test(line);

  // render(markdown, { files: { 'page.md': 'id' }, base: 3 }) -> a DocumentFragment. base is the heading level of "#".
  function render(md, opts) {
    opts = opts || {};
    const base = opts.base || 3;
    const out = doc().createDocumentFragment();
    const lines = String(md == null ? '' : md).replace(/\r\n?/g, '\n').split('\n');
    let i = 0;
    while (i < lines.length) {
      const line = lines[i];
      if (!line.trim()) { i++; continue; }
      const h = /^(#{1,3})\s+(.*)$/.exec(line);
      if (h) { inline(h[2].trim(), opts, out.appendChild(make('h' + (base + h[1].length - 1), 'helph'))); i++; continue; }
      if (line.trim().startsWith('|') && i + 1 < lines.length && isRule(lines[i + 1])) {
        const wrap = make('div', 'scroll helptable', '');
        wrap.setAttribute('tabindex', '0');
        wrap.setAttribute('role', 'region');
        const head = cells(line);
        wrap.setAttribute('aria-label', 'Table: ' + head.join(', '));
        const t = make('table', '');
        const tr0 = make('tr', '');
        for (const c of head) { const th = make('th', '', ''); th.setAttribute('scope', 'col'); inline(c, opts, th); tr0.appendChild(th); }
        const thead = make('thead', ''); thead.appendChild(tr0); t.appendChild(thead);
        const tb = make('tbody', '');
        i += 2;
        while (i < lines.length && lines[i].trim().startsWith('|')) {
          const tr = make('tr', '');
          for (const c of cells(lines[i])) inline(c, opts, tr.appendChild(make('td', '')));
          tb.appendChild(tr);
          i++;
        }
        t.appendChild(tb);
        wrap.appendChild(t);
        out.appendChild(wrap);
        continue;
      }
      const li = /^(\s*)([-*]|\d+\.)\s+(.*)$/.exec(line);
      if (li) {
        const ordered = /\d/.test(li[2]);
        const list = make(ordered ? 'ol' : 'ul', 'helplist');
        while (i < lines.length) {
          const m = /^(\s*)([-*]|\d+\.)\s+(.*)$/.exec(lines[i]);
          if (!m) break;
          let text = m[3];
          i++;
          while (i < lines.length && lines[i].trim() && /^\s{2,}\S/.test(lines[i]) && !/^\s*([-*]|\d+\.)\s+/.test(lines[i])) { text += ' ' + lines[i].trim(); i++; }
          inline(text, opts, list.appendChild(make('li', '')));
        }
        out.appendChild(list);
        continue;
      }
      const para = [];
      while (i < lines.length && lines[i].trim() && !/^#{1,3}\s/.test(lines[i]) && !/^\s*([-*]|\d+\.)\s+/.test(lines[i]) && !(lines[i].trim().startsWith('|') && i + 1 < lines.length && isRule(lines[i + 1]))) { para.push(lines[i].trim()); i++; }
      if (!para.length) { para.push(line.trim()); i++; } // a line no rule above took is a paragraph of its own: the loop always moves on
      inline(para.join(' '), opts, out.appendChild(make('p', '')));
    }
    return out;
  }

  const api = { render };
  if (typeof module !== 'undefined' && module.exports) module.exports = api;
  else root.TabrHelp = api;
})(typeof window !== 'undefined' ? window : globalThis);
