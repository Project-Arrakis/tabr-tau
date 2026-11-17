// The Command Center's live board (milestone M3): the plaque, the five-by-five board of 25 Houses, the list of the same Houses and the House Inspector, drawn from the answer of the Landsraad read.
//
// What this file is: a drawing and a small amount of view state (which filter is on, the layout, the sort, the House that is looked at). Every figure it shows comes from lcc-model.js (readBoard),
// and every sentence about what a click would do comes from the page's own code through `host.ctl(id)`: the same functions that draw the classic list. It builds elements with createElement and
// textContent only, so a House or decree name from a save is never markup.
// What it is not: it never writes. A control that changes the save is a plain button that carries the same data-lb attributes as the classic list (data-lb="tier", "qa", "pick", "map"), so one click
// handler in app.js, one stale guard, one write queue and one backup serve both views; the request itself is made there. When the board cannot be trusted as a board (readBoard says it is not ready)
// the controls that write are not drawn at all and the list says why.
// The sysselraad column of a task is not read here, a lead is never called a win, and a figure the save does not hold is shown as "Not in the save".
(function (root) {
  'use strict';

  const L = () => root.LCC;
  const M = () => root.LCCM;
  const el = (tag, cls, text, attrs) => L().el(tag, cls, text, attrs);
  const fmt = (n) => Number(n).toLocaleString('en-US');
  const FACTION_WORD = Object.freeze({ a: 'Atreides', h: 'Harkonnen' });
  const TRACK_WORD = Object.freeze({ combat: 'Combat', crafting: 'Crafting', exploration: 'Exploration', gathering: 'Gathering', sabotage: 'Sabotage' });
  const OWNER_WORD = Object.freeze({ a: 'Atreides', h: 'Harkonnen', open: 'Open', unknown: 'Unreadable' });
  const NOT_IN_SAVE = 'Not in the save';

  // The view state. It lives here, not in the answer, so a redraw after a write keeps the filter, the layout and the House that was looked at.
  const state = { layout: 'grid', show: 'all', hs: 'any', spec: 'all', sort: 'position', dir: 'asc', group: '', sel: null };
  let last = null; // { container, answer, host } of the latest mount, so a change of the view can draw again without the page's help
  let inited = false;

  // ---- small helpers -------------------------------------------------------------------------------------------------------------------------------------------------------------
  function btn(cls, text, attrs) { return el('button', cls, text, Object.assign({ type: 'button' }, attrs || {})); }
  const trackWord = (t) => TRACK_WORD[t] || '';
  const pct = (v, goal) => (goal > 0 && v !== null ? Math.max(0, Math.min(100, Math.round((v / goal) * 100))) : 0);
  const ownerPill = (owner) => el('span', 'lcc-owner lcc-owner-' + (OWNER_WORD[owner] ? owner : 'unknown'), ({ a: 'A', h: 'H', open: '○', unknown: '?' }[owner] || '?') + ' ' + (OWNER_WORD[owner] || 'Unreadable'));

  // A figure that cannot be read is never a zero: it is "not known".
  const known = (v) => (v === null || v === undefined ? 'not known' : fmt(v));

  // ---- the plaque ----------------------------------------------------------------------------------------------------------------------------------------------------------------
  function plaque(board, host) {
    const ready = board.ready, c = board.counts;
    const a = ready ? c.a : null, h = ready ? c.h : null;
    const dec = host && host.decree ? host.decree() : null;
    let standing;
    if (!ready) standing = 'HOUSE COUNT UNAVAILABLE · BOARD NOT COMPLETE';
    else if (c.unknown > 0) standing = 'COUNT INCOMPLETE · ' + c.unknown + (c.unknown === 1 ? ' HOUSE' : ' HOUSES') + ' UNREADABLE · VOTE NOT CONFIRMED';
    return L().renderPlaque({ a, h, decree: dec && dec.name ? dec : null, holder: board.term ? board.term.reigning : null, contributors: null, standing });
  }

  // ---- the figures above the board -----------------------------------------------------------------------------------------------------------------------------------------
  function kpis(board) {
    const c = board.counts, r = board.rollup;
    const w = el('ul', 'lcc-bd-kpis', '', { role: 'list', 'aria-label': 'Figures of the term' });
    const item = (label, value, cls, note) => {
      const li = el('li', 'lcc-bd-kpi' + (cls ? ' ' + cls : ''));
      li.appendChild(el('span', 'lcc-bd-kpi-l', label));
      li.appendChild(el('b', 'lcc-bd-kpi-v', value));
      if (note) li.appendChild(el('span', 'lcc-bd-kpi-n', note));
      return li;
    };
    w.appendChild(item('Atreides Houses', String(c.a), 'lcc-bd-kpi-a'));
    w.appendChild(item('Harkonnen Houses', String(c.h), 'lcc-bd-kpi-h'));
    w.appendChild(item('Open Houses', String(c.open)));
    if (c.unknown) w.appendChild(item('Unreadable owner', String(c.unknown), 'lcc-bd-kpi-warn', 'Not counted for either side'));
    w.appendChild(item('Your tiers reached', r.tiers ? r.reached + ' of ' + r.tiers : 'not known', '', 'worked out from your contribution'));
    w.appendChild(item('Waiting to collect', String(r.waiting), '', 'at the house representatives'));
    w.appendChild(item('Delivery unknown', String(r.unknownDelivery), '', 'reached, no delivery row'));
    w.appendChild(item('Contributed in all', fmt(r.mine), '', 'your contribution, all Houses'));
    return w;
  }

  // ---- the toolbar ---------------------------------------------------------------------------------------------------------------------------------------------------------------
  function toolbar(board, host, matchIds) {
    const bar = el('div', 'lcc-bd-bar', '', { role: 'group', 'aria-label': 'Filter and look at the board' });
    const seg = (key, label, list, cur) => {
      const g = el('span', 'lcc-bd-seg', '', { role: 'group', 'aria-label': label });
      g.appendChild(el('span', 'lcc-bd-lbl', label));
      for (const [k, t] of list) g.appendChild(btn('lcc-bd-chip', t, { 'aria-pressed': cur === k ? 'true' : 'false', 'data-lcb': 'filter', 'data-key': key, 'data-val': k, 'data-fk': 'bf:' + key + ':' + k }));
      return g;
    };
    bar.appendChild(seg('show', 'Show', M().SHOWS, state.show));
    bar.appendChild(seg('hs', 'House', M().HOUSE_FILTERS, state.hs));
    const sel = (cls, label, id, opts, cur, key) => {
      const w = el('span', 'lcc-bd-sel');
      w.appendChild(el('label', 'lcc-bd-lbl', label, { for: id }));
      const s = el('select', cls, '', { id, 'data-lcb': key, 'data-fk': 'bf:' + key });
      for (const [v, t] of opts) { const o = el('option', '', t, { value: v }); if (String(v) === String(cur)) o.selected = true; s.appendChild(o); }
      w.appendChild(s);
      return w;
    };
    bar.appendChild(sel('lcc-bd-select', 'Path', 'lcc-bd-spec', [['all', 'All paths']].concat(M().TRACKS.map((t) => [t, trackWord(t)])), state.spec, 'spec'));
    const jumpOpts = [['', 'Choose a House']].concat(board.houses.map((h) => [String(h.id), (h.number ? h.number + ' · ' : '') + h.name]));
    bar.appendChild(sel('lcc-bd-select', 'Jump to House', 'lcc-bd-jump', jumpOpts, state.sel === null ? '' : state.sel, 'jump'));
    const lay = el('span', 'lcc-bd-seg', '', { role: 'group', 'aria-label': 'Layout' });
    lay.appendChild(el('span', 'lcc-bd-lbl', 'View'));
    const gridOk = board.ready;
    lay.appendChild(btn('lcc-bd-chip', 'Board', { 'aria-pressed': state.layout === 'grid' && gridOk ? 'true' : 'false', 'aria-disabled': gridOk ? null : 'true', title: gridOk ? null : 'The board cannot be drawn as 5 by 5: ' + board.problems[0], 'data-lcb': 'layout', 'data-val': 'grid', 'data-fk': 'bf:layout:grid' }));
    lay.appendChild(btn('lcc-bd-chip', 'List', { 'aria-pressed': state.layout === 'list' || !gridOk ? 'true' : 'false', 'data-lcb': 'layout', 'data-val': 'list', 'data-fk': 'bf:layout:list' }));
    bar.appendChild(lay);
    if (state.layout === 'list' || !gridOk) bar.appendChild(sel('lcc-bd-select', 'Group', 'lcc-bd-group', [['', 'No groups'], ['region', 'By region'], ['spec', 'By path']], state.group, 'group'));
    bar.appendChild(el('span', 'lcc-bd-shown', matchIds.size + ' of ' + board.houses.length + ' Houses shown', { role: 'status', id: 'lcc-bd-shown' }));
    return bar;
  }

  // ---- the board: five by five ---------------------------------------------------------------------------------------------------------------------------------------------------
  // The faction that holds more than half of the Houses on a board that passed its checks ('a' or 'h'), or null. This is a count of the Houses as the save shows them now: the save keeps no
  // snapshot of the votes, so a count is never called a win, here or anywhere else on the board.
  function majorityOf(board) {
    if (!board || !board.ready || !board.counts) return null;
    const c = board.counts, need = Math.floor(board.houses.length / 2) + 1;
    return c.a >= need ? 'a' : c.h >= need ? 'h' : null;
  }
  function tile(h, matchIds, host, editable, picks, major) {
    const li = el('li', 'lcc-bd-cell', '', { 'data-id': h.id, 'data-index': h.index });
    const dom = !!major && h.owner === major;
    const card = L().renderHouseCard({ name: h.name, owner: h.owner, track: h.track, dominion: dom, selected: state.sel === h.id, dim: !matchIds.has(h.id), compact: true, number: h.number, waiting: h.waiting, meter: h.tierCount ? { reached: h.reached, total: h.tierCount } : null });
    card.setAttribute('data-lcb', 'sel');
    card.setAttribute('data-id', h.id);
    card.setAttribute('data-fk', 'ht:' + h.id);
    card.setAttribute('title', 'House ' + h.name + ', ' + (h.pos ? h.pos.words + '. ' : '') + (dom ? 'One of the ' + (major === 'a' ? 'Atreides' : 'Harkonnen') + ' Houses, who hold more than half of the Houses now: a count of Houses, not a vote result. ' : '') + 'Click to see its tiers and actions.');
    li.appendChild(card);
    if (editable) li.appendChild(pickBox(h, picks));
    return li;
  }
  function pickBox(h, picks) {
    const l = el('label', 'lcc-bd-pickl');
    const i = el('input', 'lcc-bd-pick', '', { type: 'checkbox', 'data-lb': 'pick', 'data-id': h.id, 'data-fk': 'pk:' + h.id, 'aria-label': 'Choose House ' + h.name + ' for the actions on several Houses' });
    if (picks && picks.has(h.id)) i.checked = true;
    l.appendChild(i);
    return l;
  }
  function grid(board, matchIds, host, editable, picks) {
    const ul = el('ul', 'lcc-bd-tiles', '', { role: 'list', 'aria-label': 'The 25 Houses of the board, five rows of five, numbered 01 to 25 from the top left' });
    const major = majorityOf(board);
    for (const h of board.grid) ul.appendChild(tile(h, matchIds, host, editable, picks, major));
    ul.addEventListener('keydown', (e) => {
      const d = { ArrowLeft: -1, ArrowRight: 1, ArrowUp: -M().COLS, ArrowDown: M().COLS }[e.key];
      const c = e.target && e.target.closest ? e.target.closest('.lcc-bd-cell') : null;
      if (!d || !c || (e.target.tagName !== 'BUTTON')) return;
      const to = Number(c.getAttribute('data-index')) + d;
      const n = ul.querySelector('.lcc-bd-cell[data-index="' + to + '"] .lcc-house');
      if (n) { e.preventDefault(); n.focus(); }
    });
    return ul;
  }

  // ---- the list ------------------------------------------------------------------------------------------------------------------------------------------------------------------------
  const COLS = [
    { k: 'position', t: 'No.', sort: 'position' }, { k: 'house', t: 'House', sort: 'house' }, { k: 'owner', t: 'Owner', sort: 'status' }, { k: 'path', t: 'Path' },
    { k: 'location', t: 'Location', sort: 'location' }, { k: 'state', t: 'Your progress', sort: 'state' }, { k: 'tiers', t: 'Tiers', sort: 'tiers' }, { k: 'next', t: 'Next tier', sort: 'next' }, { k: 'waiting', t: 'Waiting' },
  ];
  function locCell(h) {
    const td = el('td', 'lcc-bd-loc');
    if (!h.loc.known) { td.appendChild(el('span', 'lcc-bd-mut', 'not mapped yet')); return td; }
    const text = [h.loc.area, h.loc.cell ? 'cell ' + h.loc.cell : ''].filter(Boolean).join(' · ');
    if (h.loc.map_key) td.appendChild(btn('lcc-bd-linkb', text, { 'data-lb': 'map', 'data-id': h.id, 'data-fk': 'lm:' + h.id, title: 'Open ' + (h.loc.area || h.loc.map) + ' in the Saved map', 'aria-label': 'House ' + h.name + ': ' + text + '. Open in the Saved map' }));
    else { td.appendChild(el('span', '', text)); td.appendChild(el('span', 'lcc-bd-mut', ' · no map picture')); }
    return td;
  }
  function row(h, host, editable, picks, cols) {
    const tr = el('tr', 'lcc-bd-row', '', { 'data-id': h.id, 'aria-current': state.sel === h.id ? 'true' : null });
    if (editable) { const td = el('td', 'lcc-bd-c-pick'); td.appendChild(pickBox(h, picks)); tr.appendChild(td); }
    tr.appendChild(el('td', 'lcc-bd-c-no', h.number || '—'));
    const nameTd = el('td', 'lcc-bd-c-house');
    nameTd.appendChild(btn('lcc-bd-linkb lcc-bd-namebtn', h.name, { 'data-lcb': 'sel', 'data-id': h.id, 'data-fk': 'ht:' + h.id, 'aria-label': 'House ' + h.name + ': look at its tiers and actions' }));
    tr.appendChild(nameTd);
    const ow = el('td', 'lcc-bd-c-owner'); ow.appendChild(ownerPill(h.owner)); tr.appendChild(ow);
    tr.appendChild(el('td', 'lcc-bd-c-path', trackWord(h.track) || '—'));
    tr.appendChild(locCell(h));
    tr.appendChild(el('td', 'lcc-bd-c-state', ({ new: 'Not started', part: 'In progress', done: 'Top tier' })[h.state]));
    tr.appendChild(el('td', 'lcc-bd-c-tiers', h.tierCount ? h.reached + ' of ' + h.tierCount : 'no tiers'));
    tr.appendChild(el('td', 'lcc-bd-c-next', h.next ? fmt(h.next.threshold) + ' (' + fmt(h.next.remaining) + ' more)' : h.tierCount ? 'top tier reached' : '—'));
    tr.appendChild(el('td', 'lcc-bd-c-wait', h.waiting ? String(h.waiting) : '0'));
    return tr;
  }
  function list(board, matchIds, host, editable, picks) {
    const wrap = el('div', 'lcc-bd-listwrap');
    const t = el('table', 'lcc-bd-list');
    t.appendChild(el('caption', 'lcc-bd-cap', 'The Houses of the term' + (board.ready ? ', by default in board order (number 01 to 25)' : ', by name: the board could not be checked, so no number is given')));
    const th = el('thead'), hr = el('tr');
    if (editable) hr.appendChild(el('th', 'lcc-bd-c-pick', 'Choose', { scope: 'col' }));
    for (const c of COLS) {
      const sortable = !!c.sort && (board.ready || c.sort !== 'position'); // the number is a position on a board that was checked: with no such board it is not a column to sort by
      const cell = el('th', '', '', { scope: 'col', 'aria-sort': sortable && state.sort === c.sort ? (state.dir === 'asc' ? 'ascending' : 'descending') : null });
      if (sortable) cell.appendChild(btn('lcc-bd-sortb', c.t, { 'data-lcb': 'sort', 'data-col': c.sort, 'data-fk': 'bs:' + c.sort }));
      else cell.textContent = c.t;
      hr.appendChild(cell);
    }
    th.appendChild(hr); t.appendChild(th);
    const cols = COLS.length + (editable ? 1 : 0);
    const me = board.me.key;
    const sortKey = board.ready ? state.sort : state.sort === 'position' ? 'house' : state.sort;
    const rows = M().sortHouses(board.houses.filter((h) => matchIds.has(h.id)), { sort: sortKey, dir: state.dir, group: state.group }, me);
    if (!rows.length) {
      const tb = el('tbody'), tr = el('tr'), td = el('td', 'lcc-bd-empty', 'No House matches the filters.', { colspan: cols });
      tr.appendChild(td); tb.appendChild(tr); t.appendChild(tb);
    } else if (state.group) {
      const groups = [];
      for (const h of rows) { const g = M().groupOf(h, state.group) || ''; if (!groups.length || groups[groups.length - 1].g !== g) groups.push({ g, rows: [] }); groups[groups.length - 1].rows.push(h); }
      for (const g of groups) {
        const tb = el('tbody', 'lcc-bd-group');
        const tr = el('tr', 'lcc-bd-grouprow'), td = el('th', '', '', { scope: 'rowgroup', colspan: cols });
        td.textContent = (g.g ? (state.group === 'spec' ? trackWord(g.g) || g.g : g.g) : state.group === 'region' ? 'Location unknown' : 'No path') + ' · ' + g.rows.length;
        tr.appendChild(td); tb.appendChild(tr);
        for (const h of g.rows) tb.appendChild(row(h, host, editable, picks, cols));
        t.appendChild(tb);
      }
    } else {
      const tb = el('tbody');
      for (const h of rows) tb.appendChild(row(h, host, editable, picks, cols));
      t.appendChild(tb);
    }
    wrap.appendChild(t);
    return wrap;
  }

  // ---- the House Inspector --------------------------------------------------------------------------------------------------------------------------------------------------------
  function bar(v, goal, key, label) {
    const w = el('div', 'lcc-bd-fbar lcc-bd-fbar-' + key);
    w.appendChild(el('span', 'lcc-bd-fbar-l', label));
    const track = el('span', 'lcc-bd-fbar-t', '', { role: 'img', 'aria-label': label + ' progress ' + (v === null ? 'not known' : fmt(v)) + (goal !== null ? ' of ' + fmt(goal) : '') });
    const i = el('i'); i.style.setProperty('--lcc-v', pct(v, goal) + '%');
    track.appendChild(i);
    w.appendChild(track);
    w.appendChild(el('span', 'lcc-bd-fbar-v', v === null ? 'not known' : fmt(v)));
    return w;
  }
  function actionBtn(label, kind, h, c, fk, extra) {
    const b = btn('lcc-bd-act', label, { 'data-lb': 'qa', 'data-do': kind, 'data-id': h.id, 'data-fk': fk, 'aria-disabled': c.off ? 'true' : 'false', title: c.tip });
    if (c.off) b.setAttribute('data-why', c.tip);
    if (extra) for (const k of Object.keys(extra)) b.setAttribute(k, extra[k]);
    return b;
  }
  function inspector(board, h, host, editable) {
    const side = el('aside', 'lcc-bd-insp', '', { 'aria-label': 'House Inspector' });
    side.appendChild(el('h3', 'lcc-bd-insp-h', 'House Inspector', { id: 'lcc-bd-insp-h' }));
    if (!h) {
      side.appendChild(el('p', 'lcc-bd-mut', 'Choose a House on the board or in the list to see its tiers, its progress and what each action would do.'));
      return side;
    }
    side.setAttribute('data-id', h.id);
    const head = el('div', 'lcc-bd-insp-head');
    const crest = el('span', 'lcc-house-crest lcc-bd-insp-crest');
    const art = L().houseAsset(h.name);
    if (art) { const img = el('img', 'lcc-house-img', '', { alt: '', width: 40, height: 40, src: art.src, decoding: 'async' }); img.addEventListener('error', () => { img.hidden = true; }); crest.appendChild(img); }
    else crest.appendChild(el('span', 'lcc-house-initials', L().houseTitle(h.name).slice(0, 2).toUpperCase()));
    head.appendChild(crest);
    const hb = el('div', 'lcc-bd-insp-title');
    hb.appendChild(el('h4', 'lcc-bd-insp-name', 'House ' + L().houseTitle(h.name)));
    const meta = el('p', 'lcc-bd-insp-meta');
    if (h.number) meta.appendChild(el('span', 'lcc-bd-no', 'No. ' + h.number, { title: h.pos.words }));
    meta.appendChild(ownerPill(h.owner));
    if (h.track) meta.appendChild(el('span', 'lcc-bd-path', 'Path: ' + trackWord(h.track)));
    hb.appendChild(meta);
    head.appendChild(hb);
    side.appendChild(head);

    // who holds it: only what the save's own two fields say; a lead on the faction goal is a lead and never a win
    const st = el('p', 'lcc-bd-status');
    if (h.owner === 'a' || h.owner === 'h') st.textContent = 'Taken by ' + FACTION_WORD[h.owner] + ' (completed, with a winner the save records).';
    else if (h.owner === 'open') st.textContent = 'Open: no faction has taken it.' + (h.leader ? ' ' + FACTION_WORD[h.leader] + ' is ahead on the faction goal: a lead, not a win.' : '');
    else st.textContent = 'The owner cannot be read from the save (the completed flag and the winner do not agree), so this House is counted for neither side.';
    side.appendChild(st);

    // the two scales, kept apart: the factions' progress against the house goal, then the character's own tiers
    const s1 = el('section', 'lcc-bd-sec', '', { 'aria-labelledby': 'lcc-bd-s1' });
    s1.appendChild(el('h5', 'lcc-bd-sec-h', 'Faction progress', { id: 'lcc-bd-s1' }));
    s1.appendChild(L().badge('save', 'FROM THE SAVE'));
    s1.appendChild(bar(h.prog.a, h.goal, 'a', 'Atreides'));
    s1.appendChild(bar(h.prog.h, h.goal, 'h', 'Harkonnen'));
    s1.appendChild(el('p', 'lcc-bd-cap', 'House goal: ' + known(h.goal) + '. A faction’s progress is the sum of its contributions to this House.'));
    s1.appendChild(el('p', 'lcc-bd-none', 'Top contributors: ' + NOT_IN_SAVE + '.'));
    s1.appendChild(el('p', 'lcc-bd-none', 'Missions: ' + NOT_IN_SAVE + '.'));
    side.appendChild(s1);

    // the tiers (the contribution of the character, cumulative) and their rewards
    const c = editable && host && host.ctl ? host.ctl(h.id) : null;
    const s2 = el('section', 'lcc-bd-sec', '', { 'aria-labelledby': 'lcc-bd-s2' });
    s2.appendChild(el('h5', 'lcc-bd-sec-h', 'Your tiers and rewards', { id: 'lcc-bd-s2' }));
    s2.appendChild(L().badge('derived', 'WORKED OUT HERE'));
    s2.appendChild(el('p', 'lcc-bd-line', 'Your contribution: ' + (h.mine > 0 ? fmt(h.mine) : 'none recorded') + (h.next ? ' · next tier ' + fmt(h.next.threshold) + ', ' + fmt(h.next.remaining) + ' more' : h.tierCount ? ' · top tier reached' : '') + '.'));
    const ol = el('ol', 'lcc-bd-tiers', '', { role: 'list' });
    h.tiers.forEach((t) => {
      const li = el('li', 'lcc-bd-tier-li');
      const ct = c && c.tiers ? c.tiers[t.n - 1] : null;
      const reward = ct ? ct.reward : '';
      if (ct) {
        const b = btn('lcc-bd-tier' + (t.reached ? ' on' : '') + (t.reached && t.state === 'waiting' ? ' wt' : '') + (t.reached && t.state === 'delivered' ? ' dv' : ''), '', { 'data-lb': 'tier', 'data-id': h.id, 'data-t': t.n, 'data-fk': 'tc:' + h.id + ':' + t.n, 'aria-pressed': t.reached ? 'true' : 'false', 'aria-disabled': ct.off ? 'true' : 'false', title: ct.tip, 'aria-label': 'Tier ' + t.n + ', ' + fmt(t.threshold) + ', reward ' + reward + ', ' + t.word + (ct.off ? ', off' : '. ' + ct.tip) });
        if (ct.off) b.setAttribute('data-why', ct.tip);
        b.appendChild(el('span', 'lcc-bd-tier-n', 'Tier ' + t.n));
        b.appendChild(el('span', 'lcc-bd-tier-th', fmt(t.threshold)));
        b.appendChild(el('span', 'lcc-bd-tier-rw', reward));
        b.appendChild(el('span', 'lcc-bd-tier-st', t.word));
        if (ct.grant) b.appendChild(el('span', 'lcc-bd-tier-gr', ct.grant));
        else if (c.ownF && t.n === h.reached) b.appendChild(el('span', 'lcc-bd-tier-gr lcc-bd-rm', 'click removes it; takes back nothing it paid'));
        li.appendChild(b);
      } else {
        li.appendChild(el('span', 'lcc-bd-tier lcc-bd-tier-ro' + (t.reached ? ' on' : ''), `Tier ${t.n} · ${fmt(t.threshold)} · ${t.word}`));
      }
      ol.appendChild(li);
    });
    if (!h.tiers.length) s2.appendChild(el('p', 'lcc-bd-none', 'This House has no tiers in the save.'));
    s2.appendChild(ol);
    if (!h.tiersOrdered && h.tiers.length) s2.appendChild(el('p', 'lcc-bd-warn', 'The tier amounts are not in rising order in the save; the reached count is taken as the save states it.'));
    if (c && c.caption) s2.appendChild(el('p', 'lcc-bd-cap', c.caption));
    side.appendChild(s2);

    // the reward ledger of this House: what is waiting, what was collected, what cannot be told
    const s3 = el('section', 'lcc-bd-sec', '', { 'aria-labelledby': 'lcc-bd-s3' });
    s3.appendChild(el('h5', 'lcc-bd-sec-h', 'Reward ledger', { id: 'lcc-bd-s3' }));
    s3.appendChild(L().badge('save', 'FROM THE SAVE'));
    s3.appendChild(el('p', 'lcc-bd-line', h.reached + ' of ' + h.tierCount + ' reached · ' + h.waiting + ' waiting to collect · ' + h.collected + ' collected · ' + h.unknownDelivery + ' delivery unknown.'));
    if (h.unknownDelivery) s3.appendChild(el('p', 'lcc-bd-cap', '“Delivery unknown” means the tier is reached and the save has no delivery row for it this term: it is not shown as collected.'));
    side.appendChild(s3);

    // the actions on the House, for the faction chosen in the action bar below the board
    if (c) {
      const s4 = el('section', 'lcc-bd-sec', '', { 'aria-labelledby': 'lcc-bd-s4' });
      s4.appendChild(el('h5', 'lcc-bd-sec-h', 'Actions', { id: 'lcc-bd-s4' }));
      s4.appendChild(L().badge('unverified', 'NOT CHECKED IN THE GAME'));
      const who = el('p', 'lcc-bd-line');
      who.appendChild(root.document.createTextNode('Acting for ' + c.facName + (c.ownF ? ' (your faction)' : '') + '. '));
      who.appendChild(btn('lcc-bd-linkb', 'Change the faction', { 'data-lcb': 'gofac', 'data-fk': 'gofac' }));
      s4.appendChild(who);
      const acts = el('div', 'lcc-bd-acts', '', { role: 'group', 'aria-label': 'Actions on House ' + h.name });
      acts.appendChild(actionBtn('Fill', 'fill', h, c.fill, 'qf:' + h.id));
      acts.appendChild(actionBtn('Fill faction goal', 'fillgoal', h, c.fillgoal, 'af:' + h.id));
      acts.appendChild(actionBtn('Complete', 'complete', h, c.complete, 'ac:' + h.id));
      acts.appendChild(actionBtn('Reopen', 'reopen', h, c.reopen, 'ar:' + h.id));
      s4.appendChild(acts);
      s4.appendChild(el('p', 'lcc-bd-cap', 'Each click is written at once with a backup first, and refused if this House is no longer as shown. Complete on a House another faction took replaces that win; the question says so first.'));
      side.appendChild(s4);
    } else if (!editable) {
      side.appendChild(el('p', 'lcc-bd-warn', 'Read-only: the board could not be checked, so nothing can be written from this view. The classic list stays available.'));
    }

    // where it is
    const s5 = el('section', 'lcc-bd-sec', '', { 'aria-labelledby': 'lcc-bd-s5' });
    s5.appendChild(el('h5', 'lcc-bd-sec-h', 'Where', { id: 'lcc-bd-s5' }));
    if (!h.loc.known) s5.appendChild(el('p', 'lcc-bd-none', 'Not mapped yet: the save holds no map marker for this House’s representative, and the editor does not guess a place.'));
    else {
      s5.appendChild(el('p', 'lcc-bd-line', [h.loc.area, h.loc.cell ? 'grid cell ' + h.loc.cell : '', h.loc.place, h.loc.map && h.loc.map !== h.loc.area ? h.loc.map : ''].filter(Boolean).join(' · ')));
      if (h.loc.unconfirmed) s5.appendChild(el('p', 'lcc-bd-cap', 'This place is from public guides and is not confirmed in the game.'));
      if (h.loc.map_key) s5.appendChild(btn('lcc-bd-act', 'Open in the Saved map', { 'data-lb': 'map', 'data-id': h.id, 'data-fk': 'lm:' + h.id }));
      else s5.appendChild(el('p', 'lcc-bd-cap', 'The Saved map has no picture of ' + h.loc.map + ' yet.'));
    }
    side.appendChild(s5);
    return side;
  }

  // ---- the whole board -----------------------------------------------------------------------------------------------------------------------------------------------------------
  function build(board, host) {
    const editable = board.ready;
    const w = el('section', 'lcc lcc-bd', '', { 'aria-labelledby': 'lcc-bd-h', 'data-ready': board.ready ? 'true' : 'false' });
    const head = el('div', 'lcc-bd-head');
    head.appendChild(el('h3', 'lcc-bd-title', board.ready ? 'The 25 Houses' : 'The Houses', { id: 'lcc-bd-h' }));
    head.appendChild(el('span', 'lcc-bd-term', board.term ? 'Term ' + board.term.id : 'No term'));
    head.appendChild(L().badge('save', 'FROM THE SAVE'));
    w.appendChild(head);
    if (!board.ready) {
      const p = el('div', 'lcc-bd-problems', '', { role: 'status' });
      p.appendChild(el('p', 'lcc-bd-warn', 'The board cannot be drawn as a fixed five by five board, and nothing can be written from this view. The Houses that could be read are in the list. The classic list has the same Houses and its actions.'));
      const ul = el('ul', 'lcc-bd-why', '', { role: 'list' });
      for (const t of board.problems) ul.appendChild(el('li', '', t));
      p.appendChild(ul);
      w.appendChild(p);
    }
    w.appendChild(plaque(board, host));
    if (!board.houses.length) { w.appendChild(el('p', 'lcc-bd-none', 'This save has no Landsraad Houses in the current term, so there is nothing to show or change here.')); return w; }
    w.appendChild(kpis(board));
    if (state.sel !== null && !board.byId.has(state.sel)) state.sel = null;
    const matchIds = new Set(board.houses.filter((h) => M().matches(h, state)).map((h) => h.id));
    w.appendChild(toolbar(board, host, matchIds));
    const picks = host && host.picks ? host.picks() : null;
    const main = el('div', 'lcc-bd-main');
    const left = el('div', 'lcc-bd-left');
    if (board.ready && state.layout === 'grid') left.appendChild(grid(board, matchIds, host, editable, picks));
    else left.appendChild(list(board, matchIds, host, editable, picks));
    main.appendChild(left);
    main.appendChild(inspector(board, state.sel === null ? null : board.byId.get(state.sel), host, editable));
    w.appendChild(main);
    return w;
  }

  // ---- mounting and the view's own events ---------------------------------------------------------------------------------------------------------------------------------------
  function focusKey() {
    const d = root.document, a = d && d.activeElement;
    return a && a.getAttribute ? a.getAttribute('data-fk') || '' : '';
  }
  function mount(container, answer, host) {
    if (!inited) { inited = true; const v = host && host.get ? host.get('layout') : null; if (v === 'grid' || v === 'list') state.layout = v; }
    last = { container, answer, host };
    const fk = container.contains(root.document.activeElement) ? focusKey() : '';
    const board = M().readBoard(answer);
    container.textContent = '';
    container.appendChild(build(board, host));
    if (!container.__lccWired) { container.__lccWired = true; container.addEventListener('click', onClick); container.addEventListener('change', onChange); }
    if (fk) { const n = container.querySelector('[data-fk="' + fk + '"]'); if (n && n.focus) n.focus({ preventScroll: true }); }
    return board;
  }
  function redraw() { if (last) return mount(last.container, last.answer, last.host); return null; }
  // After a filter changes, a House the filter leaves out is no longer chosen for a bulk action (the same rule as the classic list: what is not shown is never written).
  // visible(answer): the ids of the Houses the filters let through now (the page asks for them when it chooses "all Houses": only what the board shows is chosen).
  function visible(answer) { return new Set(M().readBoard(answer).houses.filter((h) => M().matches(h, state)).map((h) => h.id)); }
  function filtered() {
    if (!last) return;
    if (last.host && last.host.filtered) last.host.filtered(visible(last.answer));
  }
  function onClick(e) {
    const t = e.target && e.target.closest ? e.target.closest('[data-lcb]') : null;
    if (!t || !last) return;
    const k = t.getAttribute('data-lcb');
    if (k === 'sel') {
      e.preventDefault();
      const id = Number(t.getAttribute('data-id'));
      state.sel = state.sel === id ? null : id;
      redraw();
      const insp = last.container.querySelector('.lcc-bd-insp');
      if (state.sel !== null && insp && insp.scrollIntoView) insp.scrollIntoView({ block: 'nearest' });
      return;
    }
    if (k === 'filter') {
      e.preventDefault();
      state[t.getAttribute('data-key')] = t.getAttribute('data-val');
      filtered(); redraw();
      return;
    }
    if (k === 'layout') {
      e.preventDefault();
      if (t.getAttribute('aria-disabled') === 'true') return;
      state.layout = t.getAttribute('data-val') === 'list' ? 'list' : 'grid';
      if (last.host && last.host.set) last.host.set('layout', state.layout);
      redraw();
      return;
    }
    if (k === 'sort') {
      e.preventDefault();
      const c = t.getAttribute('data-col');
      state.dir = state.sort === c && state.dir === 'asc' ? 'desc' : 'asc';
      state.sort = c;
      redraw();
      return;
    }
    if (k === 'gofac') { e.preventDefault(); if (last.host && last.host.focusFac) last.host.focusFac(); }
  }
  function onChange(e) {
    const t = e.target, k = t && t.getAttribute ? t.getAttribute('data-lcb') : '';
    if (!k) return;
    if (k === 'spec') { state.spec = M().TRACKS.includes(t.value) ? t.value : 'all'; filtered(); redraw(); return; }
    if (k === 'group') { state.group = t.value === 'region' || t.value === 'spec' ? t.value : ''; redraw(); return; }
    if (k === 'jump') {
      state.sel = t.value === '' ? null : Number(t.value);
      if (state.sel !== null) { // a jump shows the House: the filters that would hide it are cleared, as the classic list's jump does
        state.show = 'all'; state.hs = 'any'; state.spec = 'all';
      }
      redraw();
      if (state.sel !== null) {
        const c = last.container.querySelector('[data-fk="ht:' + state.sel + '"]');
        if (c && c.focus) { c.focus(); if (c.scrollIntoView) c.scrollIntoView({ block: 'nearest' }); }
      }
    }
  }

  const api = { state, mount, redraw, visible, build, majorityOf, plaque, kpis, grid, list, inspector, NOT_IN_SAVE };
  if (typeof module !== 'undefined' && module.exports) module.exports = api;
  else root.LCCB = api;
})(typeof window !== 'undefined' ? window : globalThis);
