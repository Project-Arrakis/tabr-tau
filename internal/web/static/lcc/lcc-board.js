// The Command Center's live board (milestone M3): the plaque, the decree preview, the four figures, the five-by-five board of 25 Houses, the list of the same Houses, the Imperial Victory Analysis and the
// House Intelligence panel, drawn from the answer of the Landsraad read.
//
// What this file is: a drawing and a small amount of view state (which filter is on, the layout, the sort, the House that is looked at, the decree that is previewed). Every figure it shows comes from
// lcc-model.js (readBoard), and every sentence about what a click would do comes from the page's own code through `host.ctl(id)`: the same functions that draw the classic list. It builds elements with
// createElement and textContent only, so a House or decree name from a save is never markup.
// What it is not: it never writes. A control that changes the save is a plain button that carries the same data-lb attributes as the classic list (data-lb="tier", "qa", "pick", "map"), so one click
// handler in app.js, one stale guard, one write queue and one backup serve both views; the request itself is made there. When the board cannot be trusted as a board (readBoard says it is not ready)
// the controls that write are not drawn at all and the list says why. The decree preview changes only what the plaque shows.
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
  // The five states of one reward tier, as the approved design shows them: the class of the box, its mark, and the words. A tier the character has not reached is always "not reached", whatever the
  // reward row says; a reached tier with no delivery record (or a record that disagrees with the contribution) is a question mark and is never counted as collected.
  const TIER_CLASS = Object.freeze({ delivered: 'delivered', waiting: 'waiting', no_row: 'uncertain', unknown: 'uncertain' });
  const TIER_MARK = Object.freeze({ delivered: '✓', waiting: '◆', no_row: '?', unknown: '?' });
  const TIER_LABEL = Object.freeze({ not_reached: 'NOT REACHED', waiting: 'TO COLLECT', delivered: 'COLLECTED', no_row: 'NO DELIVERY RECORD', unknown: 'NO DELIVERY RECORD' });
  const tierKind = (t) => (t.reached ? (TIER_CLASS[t.state] ? t.state : 'unknown') : 'not_reached');

  // The view state. It lives here, not in the answer, so a redraw after a write keeps the filter, the layout and the House that was looked at.
  const state = { layout: 'grid', show: 'all', hs: 'any', spec: 'all', sort: 'position', dir: 'asc', group: '', sel: null, preview: '' };
  let last = null; // { container, answer, host } of the latest mount, so a change of the view can draw again without the page's help
  let inited = false;
  // What a redraw keeps for the person (display only): where the House Intelligence panel was scrolled to (for the House it was on) and which disclosures were opened.
  const memo = { insp: null, open: new Set() };

  // ---- small helpers -------------------------------------------------------------------------------------------------------------------------------------------------------------
  function btn(cls, text, attrs) { return el('button', cls, text, Object.assign({ type: 'button' }, attrs || {})); }
  // A long explanation behind a plain summary: a native disclosure, so the keyboard opens it and a screen reader says whether it is open. What was opened stays open over a redraw.
  function disc(key, summary, ...nodes) {
    const d = el('details', 'lcc-bd-disc', '', { 'data-key': key });
    d.appendChild(el('summary', '', summary));
    for (const n of nodes) d.appendChild(n);
    if (memo.open.has(key)) d.open = true;
    return d;
  }
  const trackWord = (t) => TRACK_WORD[t] || '';
  const pct = (v, goal) => (goal > 0 && v !== null ? Math.max(0, Math.min(100, Math.round((v / goal) * 100))) : 0);
  const ownerPill = (owner) => el('span', 'lcc-owner lcc-owner-' + (OWNER_WORD[owner] ? owner : 'unknown'), ({ a: 'A', h: 'H', open: '○', unknown: '?' }[owner] || '?') + ' ' + (OWNER_WORD[owner] || 'Unreadable'));
  const houseWord = (name) => 'House ' + L().houseTitle(name);

  // A figure that cannot be read is never a zero: it is "not known".
  const known = (v) => (v === null || v === undefined ? 'not known' : fmt(v));

  // ---- the plaque and the decree preview -----------------------------------------------------------------------------------------------------------------------------------
  // The decrees the page knows by name, for the preview: [{ id, name, effect }], read-only, each name once. An empty list when the page gives none.
  function knownDecrees(host) {
    const out = [], seen = new Set();
    const list = host && typeof host.decrees === 'function' ? host.decrees() : [];
    for (const d of Array.isArray(list) ? list : []) {
      const name = d && typeof d.name === 'string' ? d.name : '';
      if (!name || seen.has(name)) continue;
      seen.add(name);
      out.push({ id: String(d.id), name, effect: typeof d.effect === 'string' ? d.effect : '' });
    }
    return out;
  }
  // The decree the plaque shows: the one chosen in the preview (a sample, never written) or else the term's own active decree.
  function shownDecree(host) {
    if (state.preview !== '') { const d = knownDecrees(host).find((x) => x.id === state.preview); if (d) return d; state.preview = ''; }
    const dec = host && host.decree ? host.decree() : null;
    return dec && dec.name ? dec : null;
  }
  function plaque(board, host) {
    const ready = board.ready, c = board.counts;
    const a = ready ? c.a : null, h = ready ? c.h : null;
    let standing;
    if (!ready) standing = 'HOUSE COUNT UNAVAILABLE · BOARD NOT COMPLETE';
    else if (c.unknown > 0) standing = 'COUNT INCOMPLETE · ' + c.unknown + (c.unknown === 1 ? ' HOUSE' : ' HOUSES') + ' UNREADABLE';
    return L().renderPlaque({ a, h, decree: shownDecree(host), holder: board.term ? board.term.reigning : null, contributors: null, standing });
  }
  // DECREE PREVIEW: a read-only chooser that shows how a decree looks in the plaque. It has no data-lb and sends nothing: the save's active decree is changed only on the Term and decrees panel.
  function preview(board, host) {
    const w = el('div', 'lcc-bd-preview', '', { role: 'group', 'aria-label': 'Decree preview' });
    w.appendChild(el('label', 'lcc-bd-preview-l', 'DECREE PREVIEW', { for: 'lcc-bd-decree' }));
    const decs = knownDecrees(host), act = host && host.decree ? host.decree() : null;
    const s = el('select', 'lcc-bd-select', '', { id: 'lcc-bd-decree', 'data-lcb': 'preview', 'data-fk': 'bf:preview' });
    s.appendChild(el('option', '', act && act.name ? act.name : 'No active decree', act && act.name ? { value: '', title: 'The active decree of the save' } : { value: '' }));
    for (const d of decs) { const o = el('option', '', d.name, { value: d.id }); if (d.id === state.preview) o.selected = true; s.appendChild(o); }
    if (!decs.length) s.disabled = true;
    w.appendChild(s);
    const shown = state.preview !== '' ? decs.find((x) => x.id === state.preview) : null;
    w.appendChild(el('span', 'lcc-bd-preview-n', shown ? 'Showing “' + shown.name + '” as a sample · not written to a save' : decs.length ? 'Sample decree only · not written to a save' : 'No decrees are known to this page · nothing to preview', { role: 'status', id: 'lcc-bd-preview-n' }));
    return w;
  }

  // ---- the four figures under the plaque -----------------------------------------------------------------------------------------------------------------------------------
  // TERM STATUS, OPEN HOUSES, POTENTIAL LINES, SPECIALIZATION CAP. The first two are worked out from the same read as the board; the third waits for the line check of milestone M4; the fourth is the
  // game's rule for one path (a reference, not a figure of the save).
  function metrics(board) {
    const c = board.counts;
    const w = el('ul', 'lcc-bd-metrics', '', { role: 'list', 'aria-label': 'Figures of the term' });
    const item = (kicker, value, sub, cls, attrs, tag) => {
      const li = el('li', 'lcc-bd-metric' + (cls ? ' ' + cls : ''), '', attrs);
      const k = el('span', 'lcc-bd-metric-k', kicker);
      if (tag) k.appendChild(el('span', 'lcc-bd-src', tag));
      li.appendChild(k);
      li.appendChild(el('strong', '', value));
      li.appendChild(el('small', '', sub));
      return li;
    };
    // no vote-start snapshot is in the save, so a count is a count and the outcome is never called: the word is CONTESTED while the board is a board, and UNAVAILABLE when it is not
    w.appendChild(board.ready
      ? item('TERM STATUS', 'CONTESTED', 'Vote outcome is not yet established', '', { 'data-metric': 'term' })
      : item('TERM STATUS', 'UNAVAILABLE', 'The board could not be checked', '', { 'data-metric': 'term' }));
    w.appendChild(board.ready
      ? item('OPEN HOUSES', String(c.open), c.unknown ? 'Available to contest · ' + c.unknown + ' unreadable, not counted' : 'Available to contest', '', { 'data-metric': 'open' })
      : item('OPEN HOUSES', '—', 'Not counted: the board could not be checked', '', { 'data-metric': 'open' }));
    w.appendChild(item('POTENTIAL LINES', '—', 'Counted when the twelve lines are checked (milestone M4)', '', { 'data-metric': 'lines' }));
    w.appendChild(item('SPECIALIZATION CAP', 'LV 100', '44,182 XP · separate from keystones', 'accent', { 'data-metric': 'cap', title: 'The cap of every path in the game: a rule, not a figure read from your save. Your own levels are on the Player tab.' }, 'RULE'));
    return w;
  }

  // ---- the panel head, the filters and the legend ----------------------------------------------------------------------------------------------------------------------------
  function panelHead(board) {
    const head = el('div', 'lcc-bd-panel-head');
    const t = el('div', 'lcc-bd-panel-title');
    t.appendChild(el('p', 'lcc-bd-eyebrow', 'THE GREAT HOUSES'));
    t.appendChild(el('h3', 'lcc-bd-h', board.ready ? 'The Landsraad' : 'The Houses', { id: 'lcc-bd-h' }));
    t.appendChild(el('p', 'lcc-bd-subline', (board.term ? 'Term ' + board.term.id + ' · ' : '') + (board.ready ? 'Fixed 5 × 5 board · game positions read from board_index' : 'Not a fixed board: the Houses are listed as read')));
    head.appendChild(t);
    const act = el('div', 'lcc-bd-panel-actions');
    const w = el('label', 'lcc-bd-sel');
    w.appendChild(el('span', 'lcc-bd-sr', 'Specialization filter'));
    const s = el('select', 'lcc-bd-select', '', { id: 'lcc-bd-spec', 'data-lcb': 'spec', 'data-fk': 'bf:spec' });
    for (const [v, tx] of [['all', 'All specializations'], ['crafting', 'Crafting'], ['gathering', 'Gathering'], ['exploration', 'Exploration'], ['combat', 'Combat'], ['sabotage', 'Sabotage']]) { const o = el('option', '', tx, { value: v }); if (v === state.spec) o.selected = true; s.appendChild(o); }
    w.appendChild(s);
    act.appendChild(w);
    const gridOk = board.ready;
    const lay = el('div', 'lcc-bd-layout', '', { role: 'group', 'aria-label': 'Board layout' });
    lay.appendChild(btn('', 'Grid', { 'aria-pressed': state.layout === 'grid' && gridOk ? 'true' : 'false', 'aria-disabled': gridOk ? null : 'true', title: gridOk ? null : 'The board cannot be drawn as 5 by 5: ' + board.problems[0], 'data-lcb': 'layout', 'data-val': 'grid', 'data-fk': 'bf:layout:grid' }));
    lay.appendChild(btn('', 'List', { 'aria-pressed': state.layout === 'list' || !gridOk ? 'true' : 'false', 'data-lcb': 'layout', 'data-val': 'list', 'data-fk': 'bf:layout:list' }));
    act.appendChild(lay);
    act.appendChild(btn('lcc-bd-btn', 'Clear highlight', { 'data-lcb': 'clearsel', 'data-fk': 'bf:clear', title: 'Stops highlighting the House that is looked at. Highlights of a line come with the line check (milestone M4).' }));
    head.appendChild(act);
    return head;
  }
  function filters(board, matchIds) {
    const bar = el('div', 'lcc-bd-filters', '', { role: 'group', 'aria-label': 'Filter and look at the board' });
    const seg = (key, label, list, cur) => {
      const g = el('span', 'lcc-bd-seg', '', { role: 'group', 'aria-label': label });
      g.appendChild(el('span', 'lcc-bd-lbl', label.toUpperCase()));
      for (const [k, t] of list) g.appendChild(btn('lcc-bd-chip', t, { 'aria-pressed': cur === k ? 'true' : 'false', 'data-lcb': 'filter', 'data-key': key, 'data-val': k, 'data-fk': 'bf:' + key + ':' + k }));
      return g;
    };
    bar.appendChild(seg('show', 'Show', M().SHOWS, state.show));
    bar.appendChild(seg('hs', 'House', M().HOUSE_FILTERS, state.hs));
    const sel = (label, id, opts, cur, key) => {
      const w = el('span', 'lcc-bd-sel');
      w.appendChild(el('label', 'lcc-bd-lbl', label.toUpperCase(), { for: id }));
      const s = el('select', 'lcc-bd-select', '', { id, 'data-lcb': key, 'data-fk': 'bf:' + key });
      for (const [v, t] of opts) { const o = el('option', '', t, { value: v }); if (String(v) === String(cur)) o.selected = true; s.appendChild(o); }
      w.appendChild(s);
      return w;
    };
    const jumpOpts = [['', 'Choose a House…']].concat(board.houses.map((h) => [String(h.id), (h.number ? h.number + ' · ' : '') + h.name]));
    bar.appendChild(sel('Jump to House', 'lcc-bd-jump', jumpOpts, state.sel === null ? '' : state.sel, 'jump'));
    if (state.layout === 'list' || !board.ready) bar.appendChild(sel('Group', 'lcc-bd-group', [['', 'No groups'], ['region', 'By region'], ['spec', 'By path']], state.group, 'group'));
    bar.appendChild(el('span', 'lcc-bd-shown', matchIds.size + ' of ' + board.houses.length + ' Houses shown', { role: 'status', id: 'lcc-bd-shown' }));
    return bar;
  }
  function legend(board) {
    const w = el('ul', 'lcc-bd-headlines', '', { role: 'list', 'aria-label': 'What the colours of a House mean' });
    const k = (cls, text, title) => { const li = el('li', '', '', { title }); li.appendChild(el('i', 'lcc-bd-key lcc-bd-key-' + cls, '', { 'aria-hidden': 'true' })); li.appendChild(root.document.createTextNode(text)); return li; };
    w.appendChild(k('a', 'ATREIDES CLAIMED'));
    w.appendChild(k('h', 'HARKONNEN CLAIMED'));
    w.appendChild(k('n', 'OPEN'));
    w.appendChild(k('g', 'VICTORY LINE', 'A line of five is drawn only when the line check of milestone M4 is built. The gold outline on this board marks the Houses of a faction that holds more than half of the Houses now: a count, not a vote result.'));
    if (board.counts.unknown) w.appendChild(k('u', 'OWNER UNREADABLE', 'The completed flag and the winner of this House do not agree, so it is counted for neither faction.'));
    return w;
  }
  // what the character has done on this board, in one line under it (the figures the first version of the board kept above it)
  function totals(board) {
    const r = board.rollup;
    const p = el('p', 'lcc-bd-totals');
    p.appendChild(el('b', '', 'Your rewards on this board: '));
    p.appendChild(root.document.createTextNode((r.tiers ? r.reached + ' of ' + r.tiers + ' tiers reached' : 'tiers not known') + ' · ' + r.waiting + ' waiting to collect at the house representatives · ' + r.collected + ' collected · ' + r.unknownDelivery + ' reached with no delivery record · ' + fmt(r.mine) + ' contributed in all Houses.'));
    return p;
  }

  // ---- the board: five by five ---------------------------------------------------------------------------------------------------------------------------------------------------
  // The faction that holds more than half of the Houses on a board that passed its checks ('a' or 'h'), or null. This is a count of the Houses as the save shows them now: the save keeps no
  // snapshot of the votes, so a count is never called a win, here or anywhere else on the board.
  function majorityOf(board) {
    if (!board || !board.ready || !board.counts) return null;
    const c = board.counts, need = Math.floor(board.houses.length / 2) + 1;
    return c.a >= need ? 'a' : c.h >= need ? 'h' : null;
  }

  // One tier box of a House card: the approved five boxes. With a host that answers for the House the box is the same control as the Inspector's tier row (same data-lb, same id, same tier number,
  // its own focus key), so the one click handler of the page writes it; without one it is only a picture of the state.
  function tierBox(h, t, ct) {
    const kind = tierKind(t);
    const cls = 'lcc-hc-t ' + (kind === 'not_reached' ? 'off' : TIER_CLASS[kind]);
    const mark = kind === 'not_reached' ? String(t.n) : TIER_MARK[kind];
    const say = 'House ' + h.name + ', tier ' + t.n + ', ' + (t.threshold !== null ? fmt(t.threshold) + ', ' : '') + TIER_LABEL[kind].toLowerCase();
    if (!ct) return el('span', cls, mark, { role: 'img', 'aria-label': say, title: 'Tier ' + t.n + ': ' + TIER_LABEL[kind] });
    const b = btn(cls, mark, { 'data-lb': 'tier', 'data-id': h.id, 'data-t': t.n, 'data-fk': 'ct:' + h.id + ':' + t.n, 'aria-pressed': t.reached ? 'true' : 'false', 'aria-disabled': ct.off ? 'true' : 'false', 'aria-label': say + (ct.off ? ', off' : '. ' + ct.tip), title: 'Tier ' + t.n + ': ' + TIER_LABEL[kind] + '. ' + ct.tip });
    if (ct.off) b.setAttribute('data-why', ct.tip);
    return b;
  }
  // One House card, as the approved design draws it: the number and the specialization badge on top, the crest, HOUSE NAME, the specialization, the owner, the five tier boxes and the line "N/5 REACHED"
  // with the collected and the waiting counts. A House whose specialization the save does not give has no badge and says so in words.
  function houseCard(h, matchIds, host, editable, picks, major) {
    const dom = !!major && h.owner === major;
    const dim = !matchIds.has(h.id);
    const owner = OWNER_WORD[h.owner] ? h.owner : 'unknown';
    const card = el('article', 'lcc-hc lcc-hc-' + owner, '', { 'data-owner': owner, 'data-selected': state.sel === h.id ? 'true' : null, 'data-dim': dim ? 'true' : null, 'data-compact': 'true' });
    if (dom) L().victory(card, 'dominion');
    const sel = btn('lcc-hc-select', '', { 'data-lcb': 'sel', 'data-id': h.id, 'data-fk': 'ht:' + h.id, 'aria-pressed': state.sel === h.id ? 'true' : 'false' });
    const top = el('span', 'lcc-hc-top');
    top.appendChild(el('span', 'lcc-hc-pos', h.number, { 'aria-hidden': 'true' }));
    const banner = L().specAsset(h.track);
    if (banner) {
      const g = el('span', 'lcc-hc-spec', '', { title: trackWord(h.track) + ' specialization', 'data-track': h.track });
      const img = el('img', 'lcc-hc-spec-img', '', { alt: '', width: 26, height: 26, src: banner.src, decoding: 'async' });
      img.addEventListener('error', () => { img.hidden = true; });
      g.appendChild(img);
      top.appendChild(g);
    }
    sel.appendChild(top);
    const crest = el('span', 'lcc-hc-crest');
    const art = L().houseAsset(h.name);
    if (art) { const img = el('img', 'lcc-hc-crest-img', '', { alt: '', width: 48, height: 48, src: art.src, decoding: 'async' }); img.addEventListener('error', () => { img.hidden = true; }); crest.appendChild(img); }
    else crest.appendChild(el('span', 'lcc-house-initials', L().houseTitle(h.name).slice(0, 2).toUpperCase()));
    sel.appendChild(crest);
    sel.appendChild(el('strong', 'lcc-hc-name', houseWord(h.name)));
    sel.appendChild(el('small', 'lcc-hc-track' + (h.track ? '' : ' lcc-hc-track-unk'), h.track ? trackWord(h.track) : 'Specialization not in the save'));
    sel.appendChild(el('span', 'lcc-hc-owner', owner === 'a' || owner === 'h' ? '● ' + OWNER_WORD[owner] : owner === 'open' ? '○ Open' : '? Owner unreadable', { 'data-g': ({ a: 'A', h: 'H', open: '○', unknown: '?' })[owner] }));
    if (dom) sel.appendChild(el('span', 'lcc-flag lcc-flag-dom lcc-hc-badge', L().VICTORY.dominion));
    const notes = [];
    if (h.waiting > 0) notes.push(h.waiting + ' reward' + (h.waiting === 1 ? '' : 's') + ' waiting');
    sel.setAttribute('aria-label', [(h.number ? 'Number ' + h.number + ', ' : '') + h.name + ', ' + (owner === 'a' || owner === 'h' ? 'claimed by ' + OWNER_WORD[owner] : owner === 'open' ? 'open' : 'owner unreadable')].concat(h.track ? ['path ' + trackWord(h.track)] : ['specialization not in the save'], h.tierCount ? [h.reached + ' of ' + h.tierCount + ' tiers reached', h.collected + ' collected'] : ['no tiers in the save'], notes, h.unknownDelivery ? [h.unknownDelivery + ' with no delivery record'] : [], dom ? ['one of the Houses of the faction that holds more than half'] : [], dim ? ['not in the current filter'] : []).join(', '));
    sel.setAttribute('title', 'House ' + h.name + ', ' + (h.pos ? h.pos.words + '. ' : '') + (dom ? 'One of the ' + FACTION_WORD[major] + ' Houses, who hold more than half of the Houses now: a count of Houses, not a vote result. ' : '') + 'Click to see its tiers and actions.');
    card.appendChild(sel);
    const ctl = editable && host && host.ctl ? host.ctl(h.id) : null;
    const tiers = el('div', 'lcc-hc-tiers', '', { role: 'group', 'aria-label': 'House ' + h.name + ' reward tiers' });
    h.tiers.forEach((t, i) => tiers.appendChild(tierBox(h, t, ctl && ctl.tiers ? ctl.tiers[i] : null)));
    card.appendChild(tiers);
    const count = el('div', 'lcc-hc-count');
    if (h.tierCount) {
      const sp = () => count.appendChild(root.document.createTextNode(' '));
      count.appendChild(el('b', '', h.reached + '/' + h.tierCount + ' REACHED'));
      sp();
      count.appendChild(el('span', 'lcc-hc-q lcc-hc-q-ok', '✓ ' + h.collected, { title: h.collected + ' collected' }));
      sp();
      count.appendChild(el('span', 'lcc-hc-q lcc-hc-q-wait', '◆ ' + h.waiting, { title: h.waiting + ' waiting at the representative' }));
      if (h.unknownDelivery) { sp(); count.appendChild(el('span', 'lcc-hc-q lcc-hc-q-unk', '? ' + h.unknownDelivery, { title: h.unknownDelivery + ' reached with no delivery record: not counted as collected or waiting' })); }
    } else count.appendChild(el('b', '', 'NO TIERS IN THE SAVE'));
    card.appendChild(count);
    if (editable) card.appendChild(pickBox(h, picks));
    return card;
  }
  function tile(h, matchIds, host, editable, picks, major) {
    const li = el('li', 'lcc-bd-cell', '', { 'data-id': h.id, 'data-index': h.index });
    li.appendChild(houseCard(h, matchIds, host, editable, picks, major));
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
      if (!d || !c || !e.target.classList.contains('lcc-hc-select')) return;
      const to = Number(c.getAttribute('data-index')) + d;
      const n = ul.querySelector('.lcc-bd-cell[data-index="' + to + '"] .lcc-hc-select');
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

  // ---- Imperial Victory Analysis -------------------------------------------------------------------------------------------------------------------------------------------------
  // Read-only in M3. The save keeps no snapshot of the votes at the start of the voting session, so this says what the count is and that nothing is confirmed; the five lines of logic are milestone M4.
  function victoryPanel(board) {
    const s = el('section', 'lcc-bd-surface lcc-bd-victory', '', { 'aria-label': 'Imperial Victory Analysis', 'aria-live': 'polite' });
    s.appendChild(el('p', 'lcc-bd-eyebrow', 'IMPERIAL VICTORY ANALYSIS'));
    s.appendChild(el('div', 'lcc-bd-vtag', board.ready ? 'AWAITING VOTING SESSION' : 'COUNT UNAVAILABLE'));
    s.appendChild(el('div', 'lcc-bd-emblem', '◇', { 'aria-hidden': 'true' }));
    s.appendChild(el('h3', 'lcc-bd-vhead', board.ready ? 'SYSSELRAAD CONTESTED' : 'NO COUNT TO SHOW'));
    s.appendChild(el('p', 'lcc-bd-vbody', board.ready ? 'No confirmed winner. A five-House line secures victory; otherwise compare claimed Houses at the start of the Voting Session.' : 'The board could not be checked, so no count of Houses is shown and nothing is confirmed.'));
    const c = board.counts;
    s.appendChild(el('div', 'lcc-bd-well', board.ready ? 'Atreides ' + c.a + ' · Harkonnen ' + c.h + ' · Open ' + c.open + (c.unknown ? ' · Unreadable ' + c.unknown : '') : 'Houses read: ' + board.houses.length));
    return s;
  }

  // ---- House Intelligence (the Inspector) ------------------------------------------------------------------------------------------------------------------------------------------
  function fcRow(key, label, v, goal) {
    const w = el('div', 'lcc-bd-fc');
    const head = el('div', 'lcc-bd-fc-head');
    head.appendChild(el('span', 'lcc-bd-fc-l lcc-bd-fc-' + key, label));
    head.appendChild(el('b', 'lcc-bd-fc-v', (v === null ? 'not known' : fmt(v)) + ' / ' + (goal === null ? 'not known' : fmt(goal)), { 'aria-label': label + ' ' + (v === null ? 'not known' : fmt(v)) + ' of ' + (goal === null ? 'not known' : fmt(goal)) }));
    w.appendChild(head);
    const track = el('div', 'lcc-bd-track', '', { role: 'img', 'aria-label': label + ' progress ' + pct(v, goal) + ' percent' });
    const i = el('i', 'lcc-bd-fill lcc-bd-fill-' + key); i.style.setProperty('--lcc-v', pct(v, goal) + '%');
    track.appendChild(i);
    w.appendChild(track);
    return w;
  }
  function actionBtn(label, kind, h, c, fk, extra) {
    const b = btn('lcc-bd-act', label, { 'data-lb': 'qa', 'data-do': kind, 'data-id': h.id, 'data-fk': fk, 'aria-disabled': c.off ? 'true' : 'false', title: c.tip });
    if (c.off) b.setAttribute('data-why', c.tip);
    if (extra) for (const k of Object.keys(extra)) b.setAttribute(k, extra[k]);
    return b;
  }
  const tag = (t) => el('span', 'lcc-bd-src', t);
  function secHead(text, id, src) {
    const h = el('h5', 'lcc-bd-sec-h', text, { id });
    if (src) h.appendChild(tag(src));
    return h;
  }
  // The note of the approved design, in our words: what the save says, and the rule for a tier with no delivery record.
  function importantNote() {
    const n = el('div', 'lcc-bd-note');
    n.appendChild(el('strong', '', 'Important:'));
    n.appendChild(root.document.createTextNode(' A House claimed by a faction, a personal reward tier reached, and a reward collected are '));
    n.appendChild(el('em', '', 'three separate states'));
    n.appendChild(root.document.createTextNode('. Thresholds and reward records are read from the save; a tier with no delivery record is never counted as collected or waiting.'));
    return n;
  }
  // The actions block. Presentation only: every button is the same control as before (data-lb="qa", same ids and keys), the faction box drives the board's own faction box, and
  // Apply tier presses the tier box of the ledger that the choice means, so a tier set here goes through the same checks, question and backup as a click on that box.
  function editor(h, c) {
    const s4 = el('section', 'lcc-bd-sec lcc-bd-editor', '', { 'aria-labelledby': 'lcc-bd-s4' });
    const head = el('div', 'lcc-bd-editor-head');
    head.appendChild(secHead('ACTIONS', 'lcc-bd-s4'));
    head.appendChild(L().badge('unverified', 'NOT CHECKED IN THE GAME'));
    s4.appendChild(head);
    s4.appendChild(el('p', 'lcc-bd-cap', 'The numbered tier boxes of the board and of the ledger are the same controls as Set tier. Choosing a faction writes nothing.'));
    const r1 = el('div', 'lcc-bd-erow', '', { role: 'group', 'aria-label': 'Actions on House ' + h.name });
    if (c.factions && c.factions.length) {
      const lab = el('label', 'lcc-bd-elab', 'Faction', { for: 'lcc-bd-fac' });
      const fs = el('select', 'lcc-bd-select lcc-bd-eselect', '', { id: 'lcc-bd-fac', 'data-lcb': 'fac', 'data-fk': 'fac-i', title: 'Acting for ' + c.facName + (c.ownF ? ' (your faction)' : '') });
      for (const f of c.factions) { const o = el('option', '', f.name, { value: String(f.id) }); if (String(f.id) === String(c.F)) o.selected = true; fs.appendChild(o); }
      lab.appendChild(root.document.createTextNode(' '));
      lab.appendChild(fs);
      r1.appendChild(lab);
    } else r1.appendChild(el('span', 'lcc-bd-elab', 'Acting for ' + c.facName + (c.ownF ? ' (your faction)' : '')));
    r1.appendChild(actionBtn(h.tierCount ? 'Fill ' + h.tierCount + ' tiers' : 'Fill', 'fill', h, c.fill, 'qf:' + h.id));
    r1.appendChild(actionBtn('Fill faction goal', 'fillgoal', h, c.fillgoal, 'af:' + h.id));
    r1.appendChild(actionBtn('Complete House', 'complete', h, c.complete, 'ac:' + h.id));
    r1.appendChild(actionBtn('Reopen', 'reopen', h, c.reopen, 'ar:' + h.id, { class: 'lcc-bd-act lcc-bd-act-danger' }));
    s4.appendChild(r1);
    const r2 = el('div', 'lcc-bd-erow');
    const tl = el('label', 'lcc-bd-elab', 'Set tier', { for: 'lcc-bd-tierset' });
    const ts = el('select', 'lcc-bd-select lcc-bd-eselect', '', { id: 'lcc-bd-tierset', 'data-fk': 'ts:' + h.id });
    for (let n = 0; n <= h.tierCount; n++) { const o = el('option', '', n === 0 ? '0 — none' : n + ' / ' + h.tierCount, { value: String(n) }); if (n === h.reached) o.selected = true; ts.appendChild(o); }
    if (!h.tierCount) ts.disabled = true;
    tl.appendChild(root.document.createTextNode(' '));
    tl.appendChild(ts);
    r2.appendChild(tl);
    r2.appendChild(btn('lcc-bd-act', 'Apply tier', { 'data-lcb': 'settier', 'data-id': h.id, 'data-r': h.reached, 'data-fk': 'at:' + h.id, title: 'Sets your contribution to the chosen tier, exactly as a click on that numbered box does: raises to a higher tier, or removes the highest reached tier.' }));
    r2.appendChild(el('span', 'lcc-bd-etip', 'Claimed ≠ earned ≠ collected'));
    s4.appendChild(r2);
    s4.appendChild(el('p', 'lcc-bd-cap', '', { role: 'status', id: 'lcc-bd-eset' }));
    s4.appendChild(disc('tip', 'How each click is written', el('p', 'lcc-bd-tip', 'Each click is written at once with a backup first, and refused if this House is no longer as shown. Complete on a House another faction took replaces that win; the question says so first.')));
    return s4;
  }

  function inspector(board, h, host, editable) {
    const side = el('section', 'lcc-bd-surface lcc-bd-insp', '', { 'aria-label': 'House Intelligence' });
    const head = el('div', 'lcc-bd-insp-top');
    head.appendChild(el('h3', 'lcc-bd-eyebrow lcc-bd-insp-h', 'HOUSE INTELLIGENCE', { id: 'lcc-bd-insp-h' }));
    head.appendChild(el('span', 'lcc-bd-source', 'FROM THE SAVE'));
    side.appendChild(head);
    if (!h) {
      side.appendChild(el('p', 'lcc-bd-mut', 'Choose a House on the board or in the list to see its tiers, its progress and what each action would do.'));
      return side;
    }
    side.setAttribute('data-id', h.id);
    // shown only where the panel is not beside the board (a phone, a narrow window): it takes the person back to the card of this House
    side.appendChild(btn('lcc-bd-back', 'Back to board', { 'data-lcb': 'back', 'data-id': h.id, 'data-fk': 'bk:' + h.id, title: 'Scroll back to the card of this House' }));
    const idn = el('div', 'lcc-bd-identity');
    const sigil = el('span', 'lcc-bd-sigil');
    const art = L().houseAsset(h.name);
    if (art) { const img = el('img', 'lcc-hc-crest-img', '', { alt: '', width: 48, height: 48, src: art.src, decoding: 'async' }); img.addEventListener('error', () => { img.hidden = true; }); sigil.appendChild(img); }
    else sigil.appendChild(el('span', 'lcc-house-initials', L().houseTitle(h.name).slice(0, 2).toUpperCase()));
    idn.appendChild(sigil);
    const hb = el('div', 'lcc-bd-insp-title');
    // the position of the House on the board stays reachable as a tooltip on the name (the approved design shows no number here)
    hb.appendChild(el('h4', 'lcc-bd-insp-name', houseWord(h.name), h.number ? { title: 'No. ' + h.number + (h.pos && h.pos.words ? ' · ' + h.pos.words : '') } : undefined));
    hb.appendChild(el('span', 'lcc-bd-bloc', h.track ? trackWord(h.track) + ' · Associated specialization' : 'Specialization not in the save'));
    idn.appendChild(hb);
    side.appendChild(idn);

    // the actions on the House come first, directly under the name: they are what the panel is for, and a short window shows them without scrolling the panel. The order of the rest is the approved design's.
    const c = editable && host && host.ctl ? host.ctl(h.id) : null;
    if (c) {
      side.appendChild(editor(h, c));
      // directly under the buttons: where the page draws the question about this House (it moves its own question here) and a copy of its result line, so both stand next to the buttons that asked;
      // where the panel scrolls on its own they stay at its top while the ledger scrolls under them
      const pin = el('div', 'lcc-bd-pin');
      pin.appendChild(el('div', 'lcc-bd-ask', '', { 'data-lcc-slot': 'ask' }));
      pin.appendChild(el('p', 'lcc-bd-eres', '', { id: 'lcc-bd-eres', 'aria-hidden': 'true' }));
      side.appendChild(pin);
    } else if (!editable) {
      side.appendChild(el('p', 'lcc-bd-warn', 'Read-only: the board could not be checked, so nothing can be written from this view. The classic list stays available.'));
    }

    // who holds it: only what the save's own two fields say; a lead on the faction goal is a lead and never a win
    const ban = el('div', 'lcc-bd-banner');
    ban.appendChild(el('span', 'lcc-bd-control lcc-bd-control-' + (OWNER_WORD[h.owner] ? h.owner : 'unknown'), h.owner === 'a' || h.owner === 'h' ? 'CLAIMED · ' + FACTION_WORD[h.owner].toUpperCase() : h.owner === 'open' ? 'NOT CLAIMED' : 'OWNER UNREADABLE'));
    ban.appendChild(el('span', 'lcc-bd-where', h.loc.known ? [h.loc.area, h.loc.cell ? 'cell ' + h.loc.cell : ''].filter(Boolean).join(' · ') : 'Representative location not mapped yet'));
    // The approved design has no sentence under the band. What the sentence said stays reachable: as a tooltip and as text for a screen reader. Only an owner the save cannot settle keeps a visible sentence.
    const said = h.owner === 'a' || h.owner === 'h' ? 'Claimed by ' + FACTION_WORD[h.owner] + ' (completed, with a winner the save records).'
      : h.owner === 'open' ? 'Open: no faction has taken it.' + (h.leader ? ' ' + FACTION_WORD[h.leader] + ' is ahead on the faction goal: a lead, not a win.' : '')
        : 'The owner cannot be read from the save (the completed flag and the winner do not agree), so this House is counted for neither side.';
    ban.firstChild.setAttribute('title', said);
    const ownerSettled = h.owner === 'a' || h.owner === 'h' || h.owner === 'open';
    if (ownerSettled) ban.appendChild(el('span', 'lcc-bd-sr', said));
    side.appendChild(ban);
    if (!ownerSettled) side.appendChild(el('p', 'lcc-bd-status', said));

    // the two scales, kept apart: the factions' progress against the house goal, then the character's own tiers
    const s1 = el('section', 'lcc-bd-sec', '', { 'aria-labelledby': 'lcc-bd-s1' });
    const s1note = 'House goal: ' + known(h.goal) + '. A faction’s progress is the sum of its contributions to this House. Top contributors: ' + NOT_IN_SAVE + '. Missions: ' + NOT_IN_SAVE + '.';
    const s1h = secHead('FACTION HOUSE CONTROL', 'lcc-bd-s1', 'SAVE');
    s1h.setAttribute('title', s1note);
    s1.appendChild(s1h);
    s1.appendChild(fcRow('a', 'HOUSE ATREIDES', h.prog.a, h.goal));
    s1.appendChild(fcRow('h', 'HOUSE HARKONNEN', h.prog.h, h.goal));
    s1.appendChild(el('span', 'lcc-bd-sr', s1note));
    side.appendChild(s1);

    const top = h.tierCount ? h.top : null;
    const s2 = el('section', 'lcc-bd-sec', '', { 'aria-labelledby': 'lcc-bd-s2' });
    s2.appendChild(secHead('YOUR CONTRIBUTION', 'lcc-bd-s2', 'SAVE'));
    const nums = el('div', 'lcc-bd-pc');
    nums.appendChild(el('strong', '', h.mine > 0 || top !== null ? fmt(h.mine) + (top !== null ? ' / ' + fmt(top) : '') : 'none recorded'));
    nums.appendChild(el('small', '', h.next ? fmt(h.next.remaining) + ' to T' + h.next.n : h.tierCount ? 'TOP PERSONAL TIER REACHED' : 'no tiers in the save'));
    s2.appendChild(nums);
    const mineSaid = 'Your contribution: ' + (h.mine > 0 ? fmt(h.mine) : 'none recorded') + (top !== null ? ' of ' + fmt(top) : '') + (h.next ? ' · next tier ' + fmt(h.next.threshold) + ', ' + fmt(h.next.remaining) + ' more' : h.tierCount ? ' · top tier reached' : '') + '.';
    const pt = el('div', 'lcc-bd-track lcc-bd-track-me', '', { role: 'img', 'aria-label': mineSaid, title: mineSaid });
    const pf = el('i', 'lcc-bd-fill lcc-bd-fill-me'); pf.style.setProperty('--lcc-v', pct(h.mine, top) + '%');
    pt.appendChild(pf);
    s2.appendChild(pt);
    side.appendChild(s2);

    // the ledger: the tiers of the character (cumulative), their rewards and what became of them. The rows are the same controls as the tier boxes of the card.
    const s3 = el('section', 'lcc-bd-sec lcc-bd-ledger', '', { 'aria-labelledby': 'lcc-bd-s3' });
    const lh = el('div', 'lcc-bd-ledger-head');
    lh.appendChild(secHead('PERSONAL REWARD LEDGER', 'lcc-bd-s3', 'SAVE + DERIVED'));
    lh.appendChild(el('span', 'lcc-bd-ledger-n', h.reached + ' OF ' + h.tierCount + ' REACHED'));
    s3.appendChild(lh);
    const sumSaid = h.reached + ' of ' + h.tierCount + ' reached · ' + h.waiting + ' waiting to collect · ' + h.collected + ' collected · ' + h.unknownDelivery + ' delivery unknown.';
    const sum = el('div', 'lcc-bd-stats', '', { role: 'group', 'aria-label': 'House rewards summary: ' + sumSaid, title: sumSaid });
    const stat = (cls, v, l) => { const d = el('div', 'lcc-bd-stat' + (cls ? ' ' + cls : '')); d.appendChild(el('strong', '', v)); d.appendChild(el('span', '', l)); return d; };
    sum.appendChild(stat('', h.reached + '/' + h.tierCount, 'Tiers reached'));
    sum.appendChild(stat('ok', String(h.collected), 'Collected'));
    sum.appendChild(stat('wait', String(h.waiting), 'To collect'));
    sum.appendChild(stat('rest', String(h.notReached), 'Not reached'));
    s3.appendChild(sum);
    if (h.unknownDelivery) s3.appendChild(el('p', 'lcc-bd-alert', '“Delivery unknown” means the tier is reached and the save has no delivery row for it this term: it is not shown as collected or waiting.'));
    const heading = el('div', 'lcc-bd-tier-heading', '', { 'aria-hidden': 'true' });
    for (const t of ['TIER & THRESHOLD', 'REWARD', 'STATE']) heading.appendChild(el('span', '', t));
    s3.appendChild(heading);
    const ol = el('ol', 'lcc-bd-tiers', '', { role: 'list' });
    h.tiers.forEach((t) => {
      const li = el('li', 'lcc-bd-tier-li');
      const ct = c && c.tiers ? c.tiers[t.n - 1] : null;
      const kind = tierKind(t), reward = ct ? ct.reward : '';
      const stateText = (kind === 'delivered' ? '✓ ' : kind === 'waiting' ? '◆ ' : kind === 'not_reached' ? '○ ' : '? ') + TIER_LABEL[kind];
      const mk = (tg) => {
        const b = el(tg, 'lcc-bd-tier ' + kind + (t.reached ? ' on' : '') + (t.reached && t.state === 'waiting' ? ' wt' : '') + (t.reached && t.state === 'delivered' ? ' dv' : ''));
        const m = el('span', 'lcc-bd-tier-m'); m.appendChild(el('strong', 'lcc-bd-tier-n', 'T' + t.n)); m.appendChild(el('span', 'lcc-bd-tier-th', t.threshold !== null ? fmt(t.threshold) : 'not known'));
        b.appendChild(m);
        const it = el('span', 'lcc-bd-tier-rw'); if (reward) it.appendChild(el('span', '', reward));
        b.appendChild(it);
        b.appendChild(el('span', 'lcc-bd-tier-s ' + kind, stateText));
        b.appendChild(el('span', 'lcc-bd-tier-st', t.word));
        return { b, it };
      };
      if (ct) {
        const { b, it } = mk('button');
        b.setAttribute('type', 'button');
        for (const [k, v] of Object.entries({ 'data-lb': 'tier', 'data-id': h.id, 'data-t': t.n, 'data-fk': 'tc:' + h.id + ':' + t.n, 'aria-pressed': t.reached ? 'true' : 'false', 'aria-disabled': ct.off ? 'true' : 'false', title: ct.tip, 'aria-label': 'Tier ' + t.n + ', ' + fmt(t.threshold) + ', reward ' + reward + ', ' + t.word + (ct.off ? ', off' : '. ' + ct.tip) })) b.setAttribute(k, v);
        if (ct.off) b.setAttribute('data-why', ct.tip);
        if (ct.grant) it.appendChild(el('span', 'lcc-bd-tier-gr', ct.grant));
        else if (c.ownF && t.n === h.reached) it.appendChild(el('span', 'lcc-bd-tier-gr lcc-bd-rm', 'click removes it; takes back nothing it paid'));
        li.appendChild(b);
      } else li.appendChild(mk('div').b);
      ol.appendChild(li);
    });
    if (!h.tiers.length) s3.appendChild(el('p', 'lcc-bd-none', 'This House has no tiers in the save.'));
    s3.appendChild(ol);
    if (!h.tiersOrdered && h.tiers.length) s3.appendChild(el('p', 'lcc-bd-warn', 'The tier amounts are not in rising order in the save; the reached count is taken as the save states it.'));
    if (c && c.caption) s3.appendChild(disc('caption', 'About Fill and the tiers', el('p', 'lcc-bd-cap', c.caption)));
    const lg = el('div', 'lcc-bd-legend');
    for (const t of ['✓ Collected', '◆ Waiting at representative', '○ Not reached', '? No delivery record']) lg.appendChild(el('span', '', t));
    s3.appendChild(disc('legend', 'What the marks mean', lg));
    side.appendChild(s3);
    side.appendChild(disc('note', 'Important: three separate states', importantNote()));

    // where it is
    const s5 = el('section', 'lcc-bd-sec', '', { 'aria-labelledby': 'lcc-bd-s5' });
    s5.appendChild(secHead('WHERE', 'lcc-bd-s5'));
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
    if (!board.ready) {
      const p = el('div', 'lcc-bd-problems', '', { role: 'status' });
      p.appendChild(el('p', 'lcc-bd-warn', 'The board cannot be drawn as a fixed five by five board, and nothing can be written from this view. The Houses that could be read are in the list. The classic list has the same Houses and its actions.'));
      const ul = el('ul', 'lcc-bd-why', '', { role: 'list' });
      for (const t of board.problems) ul.appendChild(el('li', '', t));
      p.appendChild(ul);
      w.appendChild(p);
    }
    w.appendChild(plaque(board, host));
    w.appendChild(preview(board, host));
    w.appendChild(metrics(board));
    if (!board.houses.length) { w.appendChild(el('p', 'lcc-bd-none', 'This save has no Landsraad Houses in the current term, so there is nothing to show or change here.')); return w; }
    if (state.sel !== null && !board.byId.has(state.sel)) state.sel = null;
    const matchIds = new Set(board.houses.filter((h) => M().matches(h, state)).map((h) => h.id));
    const picks = host && host.picks ? host.picks() : null;
    const grid_ = el('div', 'lcc-bd-grid');
    const panel = el('section', 'lcc-bd-surface lcc-bd-panel', '', { 'aria-labelledby': 'lcc-bd-h' });
    panel.appendChild(panelHead(board));
    panel.appendChild(filters(board, matchIds));
    panel.appendChild(legend(board));
    const stage = el('div', 'lcc-bd-stage');
    if (board.ready && state.layout === 'grid') stage.appendChild(grid(board, matchIds, host, editable, picks));
    else stage.appendChild(list(board, matchIds, host, editable, picks));
    panel.appendChild(stage);
    const foot = el('div', 'lcc-bd-foot');
    foot.appendChild(el('p', 'lcc-bd-caption', 'Select a House to inspect faction contributions, your five reward tiers, earned and collected rewards, and remaining progress.' + (editable ? ' The numbered boxes are the same controls as the tiers in the House Intelligence panel: each click is checked against the save first.' : '')));
    foot.appendChild(totals(board));
    panel.appendChild(foot);
    grid_.appendChild(panel);
    // the place of the page's own action bar: directly under the board, so the order of the keys is the order of the eyes (the page puts the bar here, see host.seat)
    grid_.appendChild(el('div', 'lcc-bd-slot', '', { 'data-lcc-slot': 'bar' }));
    const side = el('div', 'lcc-bd-side');
    side.appendChild(victoryPanel(board));
    side.appendChild(inspector(board, state.sel === null ? null : board.byId.get(state.sel), host, editable));
    grid_.appendChild(side);
    w.appendChild(grid_);
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
    if (host && host.unseat) host.unseat(); // what the page put into the board goes back to its own place before the board is drawn again
    const board = M().readBoard(answer);
    container.textContent = '';
    container.appendChild(build(board, host));
    if (!container.__lccWired) {
      container.__lccWired = true;
      container.addEventListener('click', onClick); container.addEventListener('change', onChange);
      container.addEventListener('scroll', onScroll, true); container.addEventListener('toggle', onToggle, true); // neither event bubbles
    }
    if (host && host.seat) host.seat(container);
    const insp = container.querySelector('.lcc-bd-insp');
    // the panel is drawn again on every select, filter and tier change: it is put back where it was scrolled to when it is still the same House
    if (insp && memo.insp && memo.insp.top > 0 && memo.insp.id === insp.getAttribute('data-id')) insp.scrollTop = memo.insp.top;
    if (fk) { const n = container.querySelector('[data-fk="' + fk + '"]'); if (n && n.focus) { n.focus({ preventScroll: true }); keepInView(n); } }
    return board;
  }
  // A control that has the focus inside the scrolling panel is never left out of sight in it, nor under the question and the result line that stay at the top of the panel.
  function keepInView(n) {
    const sc = n && n.closest ? n.closest('.lcc-bd-insp') : null;
    if (!sc || !n.getBoundingClientRect || sc.scrollHeight <= sc.clientHeight + 1) return;
    const a = n.getBoundingClientRect(), b = sc.getBoundingClientRect(), pin = sc.querySelector('.lcc-bd-pin');
    if (!a.height) return;
    const top = b.top + (pin && !n.closest('.lcc-bd-pin') && getComputedStyle(pin).position === 'sticky' ? pin.offsetHeight : 0);
    if (a.top < top) sc.scrollTop -= top - a.top + 8; else if (a.bottom > b.bottom) sc.scrollTop += a.bottom - b.bottom + 8;
  }
  function onScroll(e) {
    const t = e.target;
    if (t && t.classList && t.classList.contains('lcc-bd-insp')) memo.insp = { id: t.getAttribute('data-id'), top: t.scrollTop };
  }
  function onToggle(e) {
    const t = e.target;
    if (!t || !t.classList || !t.classList.contains('lcc-bd-disc')) return;
    const k = t.getAttribute('data-key');
    if (t.open) memo.open.add(k); else memo.open.delete(k);
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
      state.sel = state.sel === id ? null : id; memo.insp = null;
      redraw();
      const insp = last.container.querySelector('.lcc-bd-insp');
      if (state.sel !== null && insp && insp.scrollIntoView) insp.scrollIntoView({ block: 'nearest' });
      return;
    }
    if (k === 'back') {
      e.preventDefault();
      const card = last.container.querySelector('[data-fk="ht:' + t.getAttribute('data-id') + '"]');
      if (card) { if (card.scrollIntoView) card.scrollIntoView({ block: 'center' }); card.focus({ preventScroll: true }); }
      return;
    }
    if (k === 'clearsel') { e.preventDefault(); state.sel = null; memo.insp = null; redraw(); return; }
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
    if (k === 'settier') {
      e.preventDefault();
      // Set tier N is the click of a numbered box: a higher N presses box N; N one below the reached tier presses the highest reached box (it removes that tier); anything else changes nothing here
      const insp = t.closest('.lcc-bd-insp'), sel = insp && insp.querySelector('#lcc-bd-tierset'), say = insp && insp.querySelector('#lcc-bd-eset');
      const n = sel ? Number(sel.value) : NaN, r = Number(t.getAttribute('data-r'));
      const tell = (m) => { if (say) say.textContent = m; };
      let want = 0, msg = '';
      if (!(n >= 0)) msg = 'Choose a tier first.';
      else if (n > r) want = n;
      else if (n === r) msg = n === 0 ? 'No tier is reached: nothing to change.' : 'Tier ' + n + ' is already the highest reached tier: nothing to change.';
      else if (n === r - 1) want = r;
      else msg = 'Tiers come off one at a time, the highest first: choose ' + (r - 1) + ' to remove tier ' + r + '.';
      if (!want) { tell(msg); return; }
      const box = insp.querySelector('.lcc-bd-tier[data-lb="tier"][data-t="' + want + '"]');
      if (!box) { tell('The tier boxes cannot be used here.'); return; }
      tell('');
      box.click();
    }
  }
  function onChange(e) {
    const t = e.target, k = t && t.getAttribute ? t.getAttribute('data-lcb') : '';
    if (!k) return;
    if (k === 'spec') { state.spec = M().TRACKS.includes(t.value) ? t.value : 'all'; filtered(); redraw(); return; }
    if (k === 'group') { state.group = t.value === 'region' || t.value === 'spec' ? t.value : ''; redraw(); return; }
    if (k === 'fac') { if (last && last.host && last.host.setFac) last.host.setFac(t.value); return; }
    if (k === 'preview') { state.preview = knownDecrees(last && last.host).some((d) => d.id === t.value) ? t.value : ''; redraw(); return; }
    if (k === 'jump') {
      state.sel = t.value === '' ? null : Number(t.value);
      memo.insp = null;
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

  const api = { state, mount, redraw, keepInView, visible, build, majorityOf, plaque, preview, metrics, grid, houseCard, list, inspector, victoryPanel, knownDecrees, tierKind, NOT_IN_SAVE };
  if (typeof module !== 'undefined' && module.exports) module.exports = api;
  else root.LCCB = api;
})(typeof window !== 'undefined' ? window : globalThis);
