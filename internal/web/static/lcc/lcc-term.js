// The Command Center's Term and Decrees section (milestone M6): the facts of the current term, the controls that set its active decree and reigning faction, the decree pool, and the earlier terms,
// drawn from a plain model the page gives (host.term()).
//
// What this file is: a drawing. It builds elements with createElement and textContent only, so a decree name or a House name from a save is never markup. It holds no request, no route and no save
// logic. Every control that changes the save is a plain button or select that carries the same data-lb attributes and the same focus key as the classic panel's control (data-lb="tapply", "tmk",
// "tflag", "tad", "trf"), so the page's one click handler, one request builder, one write function and one result line (#lsb-tresult) serve both views. Choosing a decree in a select or pressing a
// button here changes nothing by itself: only Apply, Make active and Set flag write, exactly as before.
// Time: the save holds the term's start and end as play-clock seconds, and where the play clock stood when the save was written (with the real time of that write). The section says how far the start and the end are from that point, in days, hours and minutes of play, and the real time of the save in the viewer's local time; it never works out a calendar date for the start or the end, because the save does not hold what that needs (#922).
// What it does not know: which missions are on offer, and any ranking of the contributors (the save holds the amounts each gave; the board works the order out). The section says so in words.
(function (root) {
  'use strict';

  const L = () => root.LCC;
  const el = (tag, cls, text, attrs) => L().el(tag, cls, text, attrs);
  const btn = (cls, text, attrs) => el('button', cls, text, Object.assign({ type: 'button' }, attrs || {}));
  const NOT_IN_SAVE = 'Not in the save';
  const SRC = 'FROM THE SAVE';
  const TIERS_EARNED = () => (root.LCCM && root.LCCM.WORDS ? root.LCCM.WORDS.tiersEarned : 'tiers earned'); // the one pair of words, earned and collected, is in WORDS (lcc-model.js, #962)
  const MISSIONS = 'Active Landsraad missions are not recorded in this local save.';
  const CONTRIBUTORS = 'A ranking of contributors is not recorded: the save holds the amount each contributor gave to each House, and the banners show the biggest, worked out here from those amounts.';
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
  // ---- time: days, hours and minutes on the play clock (#922) ----------------------------------------------------------------------------------------------------------------------
  // span(s): whole seconds as "2 d 4 h 15 min" (days only when there are some, hours when there are days or hours, minutes always); under a minute it is "less than a minute".
  function span(sec) {
    const m = Math.floor(Math.abs(Number(sec)) / 60);
    if (!(m >= 1)) return 'less than a minute';
    const d = Math.floor(m / 1440), h = Math.floor((m % 1440) / 60);
    return (d ? d + ' d ' : '') + (d || h ? h + ' h ' : '') + (m % 60) + ' min';
  }
  // away(sec, now): where a play-clock second lies against the play clock at the save, in words. Both are whole play-clock seconds; null when either is not a number.
  function away(sec, now) {
    if (!Number.isFinite(sec) || !Number.isFinite(now)) return null;
    const d = sec - now;
    if (Math.abs(d) < 60) return 'at the save (less than a minute away)';
    return span(d) + ' of play ' + (d > 0 ? 'after' : 'before') + ' the save';
  }
  // the real time of the save, in the viewer's own time zone
  function localTime(ms) {
    const d = new Date(Number(ms));
    return Number.isNaN(d.getTime()) ? '' : d.toLocaleString();
  }
  const secs = (n) => Number(n).toLocaleString('en-US') + ' s';
  function clockNote(t, c) {
    const len = Number.isFinite(t.startSec) && Number.isFinite(t.endSec) && t.endSec >= t.startSec ? ' The term is ' + span(t.endSec - t.startSec) + ' long on the play clock.' : '';
    if (!c) return 'The save does not hold where the play clock stood when it was written, so the start and the end are shown as play-clock seconds.' + len + ' No calendar date is stored.';
    const when = localTime(c.savedAt);
    return (when ? 'Saved ' + when + ' (your local time), with the play clock at ' + secs(c.play) + '.' : 'The play clock stood at ' + secs(c.play) + ' when the save was written.') + len
      + ' The start and the end are shown as play time before or after that point. The save holds no calendar date for them: the play clock stops while the game is closed, and the save does not say when.';
  }
  function facts(vm) {
    const t = vm.term, c = vm.clock || null;
    const ul = el('ul', 'lcc-tm-facts', '', { role: 'list', 'aria-label': 'The current term' });
    const simple = (key, label, value, sub) => {
      const li = el('li', 'lcc-tm-fact', '', { 'data-fact': key });
      li.appendChild(el('span', 'lcc-tm-k', label));
      li.appendChild(el('strong', 'lcc-tm-v', value));
      if (sub) li.appendChild(el('small', 'lcc-tm-sub', sub));
      ul.appendChild(li);
    };
    simple('term', 'TERM', String(t.id));
    // a start or end the save does not hold is "Not in the save"; with the position of the save on the play clock it is play time before or after the save, else the play-clock seconds as stored
    const timed = (key, label, text, sec) => {
      const a = c ? away(sec, c.play) : null;
      if (text === '' || text === null || text === undefined || text === '—') simple(key, label, NOT_IN_SAVE);
      else if (a) simple(key, label, a, 'play clock ' + text);
      else simple(key, label, text, 'play clock, in seconds');
      const li = ul.lastChild;
      if (a) li.querySelector('.lcc-tm-v').setAttribute('title', 'Play clock ' + text + (c ? '; the save was written at ' + secs(c.play) : ''));
    };
    timed('start', 'STARTS', t.start, t.startSec);
    timed('end', 'ENDS', t.end, t.endSec);
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
  function clockLine(vm) {
    return el('p', 'lcc-tm-clock', clockNote(vm.term, vm.clock || null), { id: 'lcc-tm-clock' });
  }

  // ---- the two controls and Apply ----------------------------------------------------------------------------------------------------------------------------------------------
  // The drop-downs keep the markup the page's change handler reads: a label that holds the select and the three small lines (.did the id, .de the effect, .du the note) it sets the text of.
  function controls(vm) {
    const g = el('div', 'lcc-tm-ctl', '', { role: 'group', 'aria-labelledby': 'lcc-tm-ctl-h' });
    g.appendChild(el('h4', 'lcc-tm-h4', 'Set the active decree and the reigning faction', { id: 'lcc-tm-ctl-h' }));
    g.appendChild(el('p', 'lcc-tm-scope', 'Current term only. Apply sets both together; none for the decree clears the decree. A decree needs a reigning faction, so the faction cannot be left empty (Apply needs no choice only when the save holds a faction this list does not offer, which then stays as it is, or when the decree is cleared on a term with no faction). Nothing changes until you press Apply, Make active or Set flag.'));
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
    a.appendChild(el('small', 'lcc-tm-pending', vm.pending || '', { id: 'lcc-tm-pending', role: 'status' })); // a choice that is only a draft says so until it is written (#925)
    row.appendChild(a);
    const f = el('label', 'lcc-tm-fld');
    f.appendChild(el('span', 'lcc-tm-lbl', 'Reigning faction'));
    const sf = el('select', 'lcc-tm-select', '', { id: 'lsb-trf', 'data-lb': 'trf' });
    if (vm.sel.rf === '') { const ph = el('option', '', 'Choose a faction', { value: '', disabled: '' }); ph.defaultSelected = true; sf.appendChild(ph); } // no 'none': a decree needs a faction, and this placeholder cannot be chosen or applied (#960)
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
  // a disclosure that is closed until it is asked for: the summary names the block and carries a small Show / Hide control; the content is kept in the page (closed) and its state survives a redraw
  function shut(dt, name, count) {
    const sm = el('summary', 'lcc-tm-sum2');
    sm.appendChild(el('span', 'lcc-tm-sumt', name + (count ? ' · ' + count : '')));
    const sh = el('span', 'lcc-tm-showhide', dt.open ? 'Hide' : 'Show');
    sm.appendChild(sh);
    dt.appendChild(sm);
    dt.addEventListener('toggle', () => { sh.textContent = dt.open ? 'Hide' : 'Show'; });
    return sm;
  }
  function pool(vm) {
    const sec = el('div', 'lcc-tm-pool');
    const shown = vm.pool.filter((d) => !d.hidden), hidden = vm.pool.filter((d) => d.hidden);
    const dt = el('details', 'lcc-tm-disc', '', { id: 'lsb-tdp' });
    if (vm.open) dt.open = true;
    shut(dt, 'Decree pool', vm.pool.length + (vm.pool.length === 1 ? ' decree' : ' decrees'));
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
    if (vm.histOpen) dt.open = true;
    shut(dt, 'Earlier terms', 'read-only');
    const inner = el('div', 'lcc-tm-disc-in');
    inner.appendChild(el('p', 'lcc-tm-note', h.note));
    for (const t of h.terms) {
      const d = el('details', 'lcc-tm-term');
      const s = el('summary');
      s.appendChild(el('b', '', 'Term ' + t.id));
      s.appendChild(el('span', 'lcc-tm-sum', t.summary));
      d.appendChild(s);
      const wrap = el('div', 'lcc-tm-tablewrap', '', { tabindex: '0', role: 'region', 'aria-label': 'Term ' + t.id + ', the table of Houses: scrolls sideways when the window is narrow' });
      d.appendChild(el('p', 'lcc-tm-scrollhint', 'Scroll the table sideways for House status and Last payout.'));
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
        tc.appendChild(el('span', 'lcc-tm-meter' + (r.full ? ' is-full' : ''), '', { role: 'img', 'aria-label': r.reached + ' of ' + r.count + ' ' + TIERS_EARNED() }));
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
    surf.appendChild(clockLine(vm));
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
  const api = { build, mount, art, span, away, MISSIONS, CONTRIBUTORS, NOT_IN_SAVE };
  if (typeof module !== 'undefined' && module.exports) module.exports = api;
  else root.LCCT = api;
})(typeof window !== 'undefined' ? window : globalThis);
