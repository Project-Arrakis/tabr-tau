// The Command Center's Term and Decrees section (milestone M6): the facts of the current term, the controls that set its active decree and reigning faction, the decree pool, and the earlier terms,
// drawn from a plain model the page gives (host.term()).
//
// What this file is: a drawing. It builds elements with createElement and textContent only, so a decree name or a House name from a save is never markup. It holds no request, no route and no save
// logic. Every control that changes the save is a plain button or select that carries the same data-lb attributes and the same focus key as the classic panel's control (data-lb="tapply", "tmk",
// "tflag", "tad", "trf"), so the page's one click handler, one request builder, one write function and one result line (#lsb-tresult) serve both views. Choosing a decree in a select or pressing a
// button here changes nothing by itself: only Apply, Make active and Set flag write, exactly as before.
// What it does not know: who the top contributors are and which missions are on offer. The save does not hold them, and the section says so in words.
(function (root) {
  'use strict';

  const L = () => root.LCC;
  const el = (tag, cls, text, attrs) => L().el(tag, cls, text, attrs);
  const btn = (cls, text, attrs) => el('button', cls, text, Object.assign({ type: 'button' }, attrs || {}));
  const NOT_IN_SAVE = 'Not in the save';
  const SRC = 'FROM THE SAVE';
  const MISSIONS = 'Active Landsraad missions are not recorded in this local save.';
  const CONTRIBUTORS = 'Top contributors are not recorded in this local save: the save holds your own contribution to each House, not a ranking of players.';
  const FLAG_TIP = 'The save\'s own disabled column, shown as stored. It does not say whether a decree is in effect.';

  // The picture of a decree: from the fixed list of the helpers (matched to the decree's id, never to a name), or the neutral picture. The name and the effect are always text beside it, so a picture
  // that is missing loses nothing. A decree with no picture of its own says so in its tooltip.
  function art(d, size) {
    const a = L().decreeAsset(d.key);
    const img = el('img', 'lcc-tm-art' + (a.fallback ? ' lcc-tm-art-fb' : ''), '', { alt: a.fallback ? '' : 'Picture for ' + d.name, width: size, height: size, src: a.src, decoding: 'async', 'data-fallback': a.fallback ? 'true' : null,
      title: a.fallback ? 'This decree has no picture of its own' : null });
    img.addEventListener('error', () => { img.hidden = true; });
    return img;
  }
  function badge(text, cls, title) { return el('span', 'lcc-tm-badge' + (cls ? ' ' + cls : ''), text, title ? { title } : undefined); }
  const src = (title) => el('span', 'lcc-tm-src', SRC, { title: title || 'Read from the save' });

  // ---- the facts of the term ---------------------------------------------------------------------------------------------------------------------------------------------------
  function decreeFact(label, d, key) {
    const li = el('li', 'lcc-tm-fact lcc-tm-fact-decree', '', { 'data-fact': key });
    li.appendChild(el('span', 'lcc-tm-k', label));
    if (!d) { li.appendChild(el('strong', 'lcc-tm-v', 'None')); return li; }
    const row = el('div', 'lcc-tm-dref');
    row.appendChild(art(d, 44));
    const t = el('div', 'lcc-tm-dtext');
    t.appendChild(el('strong', 'lcc-tm-v lcc-tm-dn', d.name, d.tip ? { title: d.tip } : undefined));
    if (d.effect) t.appendChild(el('small', 'lcc-tm-de', d.effect));
    if (d.note) t.appendChild(el('small', 'lcc-tm-du', d.note));
    t.appendChild(el('small', 'lcc-tm-did', d.key, { title: 'The game\'s internal id, kept for support' }));
    row.appendChild(t);
    li.appendChild(row);
    return li;
  }
  function facts(vm) {
    const t = vm.term;
    const ul = el('ul', 'lcc-tm-facts', '', { role: 'list', 'aria-label': 'The current term' });
    const simple = (key, label, value, sub) => {
      const li = el('li', 'lcc-tm-fact', '', { 'data-fact': key });
      li.appendChild(el('span', 'lcc-tm-k', label));
      li.appendChild(el('strong', 'lcc-tm-v', value));
      if (sub) li.appendChild(el('small', 'lcc-tm-sub', sub));
      ul.appendChild(li);
    };
    simple('term', 'TERM', String(t.id));
    simple('start', 'STARTS', t.start, 'play clock, in seconds');
    simple('end', 'ENDS', t.end, 'play clock; no calendar date is stored');
    const rf = el('li', 'lcc-tm-fact', '', { 'data-fact': 'reigning' });
    rf.appendChild(el('span', 'lcc-tm-k', 'REIGNING FACTION'));
    const r = el('div', 'lcc-tm-faction');
    const crest = L().factionAsset(t.reigning, 'mark');
    if (!crest.fallback) { const i = el('img', 'lcc-tm-crest', '', { alt: '', width: 28, height: 28, src: crest.src, decoding: 'async' }); i.addEventListener('error', () => { i.hidden = true; }); r.appendChild(i); }
    r.appendChild(el('strong', 'lcc-tm-v', t.reigning || 'None'));
    rf.appendChild(r);
    ul.appendChild(rf);
    ul.appendChild(decreeFact('ACTIVE DECREE', vm.active, 'active'));
    ul.appendChild(decreeFact('ELECTED DECREE', vm.elected, 'elected'));
    return ul;
  }

  // ---- the two controls and Apply ----------------------------------------------------------------------------------------------------------------------------------------------
  // The drop-downs keep the markup the page's change handler reads: a label that holds the select and the three small lines (.did the id, .de the effect, .du the note) it sets the text of.
  function controls(vm) {
    const g = el('div', 'lcc-tm-ctl', '', { role: 'group', 'aria-labelledby': 'lcc-tm-ctl-h' });
    g.appendChild(el('h4', 'lcc-tm-h4', 'Set the active decree and the reigning faction', { id: 'lcc-tm-ctl-h' }));
    g.appendChild(el('p', 'lcc-tm-scope', 'Current term only. Apply sets both together; an empty choice clears that field. Nothing changes until you press Apply, Make active or Set flag.'));
    const row = el('div', 'lcc-tm-ctlrow');
    const a = el('label', 'lcc-tm-fld');
    a.appendChild(el('span', 'lcc-tm-lbl', 'Active decree'));
    const sa = el('select', 'lcc-tm-select', '', { id: 'lsb-tad', 'data-lb': 'tad' });
    sa.appendChild(el('option', '', 'none', { value: '' }));
    for (const o of vm.options) { const op = el('option', '', o.label, { value: String(o.id) }); if (String(o.id) === vm.sel.ad) op.defaultSelected = true; sa.appendChild(op); }
    a.appendChild(sa);
    a.appendChild(el('small', 'did lcc-tm-did', vm.selD ? vm.selD.key : '', { title: 'The game\'s internal id, kept for support' }));
    a.appendChild(el('small', 'de lcc-tm-de', vm.selD && vm.selD.effect ? vm.selD.effect : ''));
    a.appendChild(el('small', 'mut du lcc-tm-du', vm.selD ? vm.selD.note : ''));
    row.appendChild(a);
    const f = el('label', 'lcc-tm-fld');
    f.appendChild(el('span', 'lcc-tm-lbl', 'Reigning faction'));
    const sf = el('select', 'lcc-tm-select', '', { id: 'lsb-trf', 'data-lb': 'trf' });
    sf.appendChild(el('option', '', 'none', { value: '' }));
    for (const o of vm.factions) { const op = el('option', '', o.label, { value: String(o.id) }); if (String(o.id) === vm.sel.rf) op.defaultSelected = true; sf.appendChild(op); }
    f.appendChild(sf);
    f.appendChild(el('small', 'lcc-tm-hint', 'any faction can be chosen'));
    row.appendChild(f);
    const ap = el('span', 'lcc-tm-apply');
    ap.appendChild(btn('lcc-tm-btn is-primary', 'Apply', { 'data-lb': 'tapply', 'data-fk': 'tapply' }));
    row.appendChild(ap);
    g.appendChild(row);
    g.appendChild(el('p', 'lcc-tm-result' + (vm.bad ? ' bad' : ''), vm.say || '', { id: 'lsb-tresult', role: 'status' }));
    return g;
  }

  // ---- the decree pool -------------------------------------------------------------------------------------------------------------------------------------------------------------
  // One row per decree. The three states of the term (Active, Elected, In rotation) and the save's own pool flag are shown apart: the flag is not a state of the term.
  function poolRow(d) {
    const li = el('li', 'lcc-tm-row', '', { 'data-decree': d.id, 'data-active': d.active ? 'true' : null });
    li.appendChild(art(d, 56));
    const body = el('div', 'lcc-tm-rbody');
    const nm = el('div', 'lcc-tm-rname');
    nm.appendChild(el('strong', 'lcc-tm-dn', d.name, d.tip ? { title: d.tip } : undefined));
    nm.appendChild(el('span', 'lcc-tm-id', 'id ' + d.id));
    body.appendChild(nm);
    if (d.effect) body.appendChild(el('small', 'lcc-tm-de', d.effect));
    else body.appendChild(el('small', 'lcc-tm-de lcc-tm-de-none', 'Effect: ' + NOT_IN_SAVE));
    if (d.note) body.appendChild(el('small', 'lcc-tm-du', d.note));
    body.appendChild(el('small', 'lcc-tm-did', d.key, { title: 'The game\'s internal id, kept for support' }));
    const tg = el('div', 'lcc-tm-tags', '', { 'aria-label': 'In this term' });
    tg.appendChild(el('span', 'lcc-tm-k', 'IN THIS TERM'));
    if (d.tags.length) for (const x of d.tags) tg.appendChild(badge(x, 'is-' + x.toLowerCase().replace(/\s+/g, '-')));
    else tg.appendChild(el('span', 'lcc-tm-none', 'not in this term'));
    body.appendChild(tg);
    li.appendChild(body);
    const side = el('div', 'lcc-tm-rside');
    const fl = el('div', 'lcc-tm-flag');
    fl.appendChild(el('span', 'lcc-tm-k', 'POOL FLAG'));
    fl.appendChild(el('b', 'fl', String(d.disabled ? 1 : 0), { title: FLAG_TIP }));
    side.appendChild(fl);
    const n = d.disabled ? 0 : 1;
    side.appendChild(btn('lcc-tm-btn', 'Set flag to ' + n, { 'data-lb': 'tflag', 'data-id': d.id, 'data-d': n, 'data-fk': 'tf:' + d.id, 'aria-label': 'Set flag to ' + n + ': ' + d.name,
      title: 'Sets the pool flag of this one row to ' + n + '. It does not make the decree active; Make active on the row or the Active decree above does.' }));
    if (d.makeOn) side.appendChild(el('span', 'lcc-tm-mkon', '✓ Active', { title: d.makeTip }));
    else side.appendChild(btn('lcc-tm-btn is-make', d.makeLabel, { 'data-lb': 'tmk', 'data-id': d.id, 'data-fk': 'tm:' + d.id, 'aria-disabled': d.makeOff ? 'true' : 'false', 'aria-label': d.makeLabel + ': ' + d.name, title: d.makeTip }));
    li.appendChild(side);
    return li;
  }
  function pool(vm) {
    const sec = el('div', 'lcc-tm-pool');
    const shown = vm.pool.filter((d) => !d.hidden), hidden = vm.pool.filter((d) => d.hidden);
    const dt = el('details', 'lcc-tm-disc', '', { id: 'lsb-tdp' });
    if (vm.open) dt.open = true;
    dt.appendChild(el('summary', '', 'Decree pool · ' + vm.pool.length + (vm.pool.length === 1 ? ' decree' : ' decrees')));
    const inner = el('div', 'lcc-tm-disc-in');
    inner.appendChild(el('p', 'lcc-tm-note', 'Active, Elected and In rotation are read from the current term. The pool flag is the save\'s own disabled column, shown as stored: a Set flag button changes that one row only and never makes a decree active.'));
    if (!vm.pool.length) inner.appendChild(el('p', 'lcc-tm-none', 'This save has no decrees.'));
    const ul = el('ul', 'lcc-tm-rows', '', { role: 'list', 'aria-label': 'Decree pool' });
    for (const d of shown) ul.appendChild(poolRow(d));
    inner.appendChild(ul);
    if (hidden.length) {
      const h = el('details', 'lcc-tm-disc lcc-tm-disc-hid', '', { id: 'lsb-dphid' });
      if (vm.hidOpen) h.open = true;
      h.appendChild(el('summary', '', 'Hidden: the plain SpecialVendorActive row (no game name, no category, effect unverified)'));
      const hin = el('div', 'lcc-tm-disc-in');
      hin.appendChild(el('p', 'lcc-tm-note lcc-tm-warn', 'The game\'s decree table has no row for this decree, so its name is the editor\'s and its effect is not verified. It is kept with the same controls in case the game uses it.'));
      const hu = el('ul', 'lcc-tm-rows', '', { role: 'list', 'aria-label': 'Hidden decree' });
      for (const d of hidden) hu.appendChild(poolRow(d));
      hin.appendChild(hu);
      h.appendChild(hin);
      inner.appendChild(h);
    }
    dt.appendChild(inner);
    sec.appendChild(dt);
    return sec;
  }

  // ---- what the save does not hold -------------------------------------------------------------------------------------------------------------------------------------------
  function notHeld() {
    const w = el('div', 'lcc-tm-nosave', '', { role: 'group', 'aria-label': 'What this save does not hold' });
    w.appendChild(el('span', 'lcc-tm-src lcc-tm-src-no', 'NOT IN THE SAVE'));
    w.appendChild(el('p', '', MISSIONS + ' ' + CONTRIBUTORS));
    return w;
  }

  // ---- earlier terms: read-only -----------------------------------------------------------------------------------------------------------------------------------------------
  function history(vm) {
    const h = vm.history;
    const dt = el('details', 'lcc-tm-disc lcc-tm-hist', '', { 'data-key': 'tm-hist' });
    dt.appendChild(el('summary', '', 'Earlier terms · read-only'));
    const inner = el('div', 'lcc-tm-disc-in');
    inner.appendChild(el('p', 'lcc-tm-note', h.note));
    for (const t of h.terms) {
      const d = el('details', 'lcc-tm-term');
      const s = el('summary');
      s.appendChild(el('b', '', 'Term ' + t.id));
      s.appendChild(el('span', 'lcc-tm-sum', t.summary));
      d.appendChild(s);
      const wrap = el('div', 'lcc-tm-tablewrap');
      const tb = el('table', 'lcc-tm-table');
      tb.appendChild(el('caption', 'lcc-tm-sr', 'Term ' + t.id + ': the Houses with a contribution of yours'));
      const hr = el('tr');
      for (const c of ['House', 'Your contribution', 'Tiers', 'House status', 'Last payout']) hr.appendChild(el('th', '', c, { scope: 'col' }));
      const th = el('thead'); th.appendChild(hr); tb.appendChild(th);
      const bd = el('tbody');
      for (const r of t.rows) {
        const tr = el('tr');
        tr.appendChild(el('td', '', r.name));
        tr.appendChild(el('td', 'lcc-tm-n', r.mine));
        const tc = el('td');
        tc.appendChild(el('span', 'lcc-tm-meter' + (r.full ? ' is-full' : ''), '', { role: 'img', 'aria-label': r.reached + ' of ' + r.count + ' tiers reached' }));
        const m = tc.firstChild;
        for (let i = 0; i < r.count; i++) m.appendChild(el('i', i < r.reached ? 'on' : ''));
        tc.appendChild(el('span', 'lcc-tm-sub', ' ' + r.reached + ' of ' + r.count));
        tr.appendChild(tc);
        tr.appendChild(el('td', '', r.status));
        tr.appendChild(el('td', '', r.paid));
        bd.appendChild(tr);
      }
      tb.appendChild(bd);
      wrap.appendChild(tb);
      d.appendChild(wrap);
      d.appendChild(el('p', 'lcc-tm-note', t.clock));
      inner.appendChild(d);
    }
    dt.appendChild(inner);
    return dt;
  }

  // ---- the section ---------------------------------------------------------------------------------------------------------------------------------------------------------------------
  function build(vm) {
    const w = el('section', 'lcc lcc-tm', '', { id: 'lcc-term', 'aria-labelledby': 'lcc-tm-h' });
    const surf = el('div', 'lcc-tm-surface');
    const head = el('div', 'lcc-tm-head');
    const t = el('div');
    t.appendChild(el('p', 'lcc-tm-eyebrow', 'THE REIGNING COURT'));
    t.appendChild(el('h3', 'lcc-tm-h', 'Term and Decrees', { id: 'lcc-tm-h', tabindex: '-1' }));
    t.appendChild(el('p', 'lcc-tm-lead', vm.none ? 'This save has no Landsraad term, so there is no decree to show or change.' : 'The decree in force, who reigns, and the decree pool. This is the editor\'s own term control: the plaque above only shows what these hold.'));
    head.appendChild(t);
    head.appendChild(src('Read from the save: landsraad_decree_term and landsraad_decrees'));
    surf.appendChild(head);
    if (vm.none) { w.appendChild(surf); return w; }
    surf.appendChild(facts(vm));
    surf.appendChild(controls(vm));
    surf.appendChild(pool(vm));
    surf.appendChild(notHeld());
    surf.appendChild(history(vm));
    w.appendChild(surf);
    return w;
  }
  function mount(container, vm) {
    if (!container) return null;
    container.textContent = '';
    const n = build(vm);
    container.appendChild(n);
    return n;
  }
  const api = { build, mount, art, MISSIONS, CONTRIBUTORS, NOT_IN_SAVE };
  if (typeof module !== 'undefined' && module.exports) module.exports = api;
  else root.LCCT = api;
})(typeof window !== 'undefined' ? window : globalThis);
