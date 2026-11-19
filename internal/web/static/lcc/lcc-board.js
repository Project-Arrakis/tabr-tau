// The Command Center's live board (milestone M3): the plaque, the place of the Term and Decrees section, the five-by-five board of 25 Houses, the list of the same Houses, the Imperial Victory Analysis and the
// House Intelligence panel, drawn from the answer of the Landsraad read.
//
// What this file is: a drawing and a small amount of view state (which filter is on, the layout, the sort, the House that is looked at). Every figure it shows comes from
// lcc-model.js (readBoard), and every sentence about what a click would do comes from the page's own code through `host.ctl(id)`: the same functions that draw the classic list. It builds elements with
// createElement and textContent only, so a House or decree name from a save is never markup.
// What it is not: it never writes. A control that changes the save is a plain button that carries the same data-lb attributes as the classic list (data-lb="tier", "qa", "pick", "map"), so one click
// handler in app.js, one stale guard, one write queue and one backup serve both views; the request itself is made there. When the board cannot be trusted as a board (readBoard says it is not ready)
// the controls that write are not drawn at all and the list says why. The plaque shows the term’s own active decree and nothing else.
// The sysselraad column of a task is not read here, a win is worked out from the Houses by the one winner function of lcc-model.js (WIN), and a figure the save does not hold is shown as "Not in the save".
(function (root) {
  'use strict';

  const L = () => root.LCC;
  const M = () => root.LCCM;
  const el = (tag, cls, text, attrs) => L().el(tag, cls, text, attrs);
  const fmt = (n) => Number(n).toLocaleString('en-US');
  const FACTION_WORD = Object.freeze({ a: 'Atreides', h: 'Harkonnen' });
  const TRACK_WORD = Object.freeze({ combat: 'Combat', crafting: 'Crafting', exploration: 'Exploration', gathering: 'Gathering', sabotage: 'Sabotage' });
  const W = () => M().WORDS; // the words of favor, square and specialization are written once, in lcc-model.js (#952)
  const OWNER_WORD = { a: 'Atreides', h: 'Harkonnen', get open() { return W().notWon; }, get unknown() { return W().unreadable; } };
  const NOT_IN_SAVE = 'Not in the save';
  const CHECKED = 'Each click is checked against the save first.';
  // A House the save says is taken but that shows nothing of the character's own (#911): said in words, never as "0/5 EARNED". The tiers are the character's own ladder, so with no row of the
  // character none is earned, and with no tier rows at all there is no ladder to show.
  // A House whose favor is won with no tier rows in the save says so. A won House with tiers but none of the character's own shows the count of any other House (0/5 EARNED, the check and the diamond), #1022.
  const TAKEN = { get noTiers() { return W().favorWon + ' - no tier record in the save'; },
    get noTiersWhy() { return 'This House’s favor is won, and the save holds no reward tiers for it.'; } };
  const takenWhy = (h) => (h.takenNoTiers ? TAKEN.noTiersWhy : '');
  // the contributors of a list, as the text of one line: the biggest first, "name amount"
  const contribLine = (list, n) => list.slice(0, n).map((c) => c.label + ' ' + fmt(c.amount)).join(' · ');
  // the banner's three, whole, for its mouse-over text: a narrow cloth may show fewer of them, never a part of one (#972)
  // A contributor the save names no guild for is shown as "Unnamed contributor"; the save's own number for it (which means nothing as a rank) is only said in the mouse-over text, so two of them can be told apart (#919)
  const unnamedNote = (list, n) => { const u = list.slice(0, n).filter((c) => c.unnamed).map((c) => c.id); return u.length ? ' Unnamed contributor: the save gives no player or guild name for it (save number' + (u.length > 1 ? 's ' : ' ') + u.join(', ') + ').' : ''; };
  const contribRank = (list) => 'Top ' + Math.min(3, list.length) + ': ' + list.slice(0, 3).map((c, i) => (i + 1) + '. ' + c.label).join('; ') + '.';
  // The five states of one reward tier, as the approved design shows them: the class of the box, its mark, and the words. A tier the character has not reached is always "not reached", whatever the
  // reward row says; a reached tier with no delivery record (or a record that disagrees with the contribution) is a question mark and is never counted as collected.
  const TIER_CLASS = Object.freeze({ delivered: 'delivered', waiting: 'waiting', no_row: 'uncertain', unknown: 'uncertain' });
  const TIER_MARK = Object.freeze({ delivered: '✓', waiting: '◆', no_row: '?', unknown: '?' });
  const TIER_LABEL = Object.freeze({ not_reached: 'NOT REACHED', waiting: 'NOT COLLECTED', delivered: 'COLLECTED', no_row: 'NO DELIVERY RECORD', unknown: 'NO DELIVERY RECORD' });
  const tierKind = (t) => (t.reached ? (TIER_CLASS[t.state] ? t.state : 'unknown') : 'not_reached');

  // The view state. It lives here, not in the answer, so a redraw after a write keeps the filter, the layout and the House that was looked at.
  const state = { layout: 'grid', show: 'all', hs: 'any', spec: 'all', sort: 'house', dir: 'asc', group: '', sel: null };
  let last = null; // { container, answer, host } of the latest mount, so a change of the view can draw again without the page's help
  let inited = false;
  // What a redraw keeps for the person (display only): where the House Intelligence panel was scrolled to (for the House it was on) and which disclosures were opened.
  const memo = { insp: null, open: new Set() };

  // ---- small helpers -------------------------------------------------------------------------------------------------------------------------------------------------------------
  function btn(cls, text, attrs) { return el('button', cls, text, Object.assign({ type: 'button' }, attrs || {})); }
  const trackWord = (t) => TRACK_WORD[t] || '';
  const pct = (v, goal) => (goal > 0 && v !== null ? Math.max(0, Math.min(100, Math.round((v / goal) * 100))) : 0);
  // The list cell of who won a House's favor: ONE mark, the faction's name in the faction's colour (#950). A letter before the name read as a second H (H over HARKONNEN) once the cell wrapped, so there is none;
  // a House whose favor is not won says so with the open circle, and a House that cannot be read with the question mark, each on one line.
  const ownerPill = (owner) => el('span', 'lcc-owner lcc-owner-' + (OWNER_WORD[owner] ? owner : 'unknown'), owner === 'a' || owner === 'h' ? OWNER_WORD[owner] : owner === 'open' ? '○ ' + W().notWon : '? ' + W().unreadable);
  // Classic words a tier "reached"; the board says "earned" (the one pair of words, earned and collected: WORDS in lcc-model.js). "not reached" is the state word of a tier not earned yet and stays.
  const earnedWords = (s) => (typeof s === 'string' ? s.replace(/ tiers reached\./g, ' ' + W().tiersEarned + '.').replace(/\(reached, /g, '(' + W().earned + ', ').replace(/top tier reached/g, W().topTierEarned).replace(/\b(is|are) reached\b/g, '$1 ' + W().earned).replace(/: reached, /g, ': ' + W().earned + ', ').replace(/highest reached tier/g, 'highest ' + W().earned + ' tier') : s);
  // The sentences of what a click on a tier does come from the page, in Classic's words; the tier boxes of the board get them with "earned" for "reached" (#962).
  const ctlOf = (host, id) => {
    const c = host && typeof host.ctl === 'function' ? host.ctl(id) : null;
    if (!c || !Array.isArray(c.tiers)) return c;
    return Object.assign({}, c, { tiers: c.tiers.map((t) => (t ? Object.assign({}, t, { tip: earnedWords(t.tip), title: earnedWords(t.title), item: earnedWords(t.item) }) : t)) });
  };
  const houseWord = (name) => 'House ' + L().houseTitle(name);

  // ---- the reward mouse-over (#977) ---------------------------------------------------------------------------------------------------------------------------------------------
  // The tier boxes of a House card, the tier rows of the Inspector and the reward cells of the list show the reward as a small picture card: the item's picture (from the board's own fixed list of the
  // pictures that were embedded, so no request is made for a file that does not exist, and none to any other address), its display name, the quantity and the tier with its state. A reward with no picture
  // shows the name only. The game's internal item name is never shown or spoken: the item id is only a data attribute that finds the picture. One small element serves the whole board; it is drawn by script
  // because a native title text cannot hold a picture, it never takes the pointer (so it cannot cover or steal its target), it is hidden from a screen reader (the control's own accessible name says the same
  // words), and it closes on Escape, on leaving and on a click. It is placed under its target, or above it when there is no room below, inside the board's own box.
  const RTIP = new WeakMap();
  let tipEl = null, tipOn = null;
  function rtipSet(node, d) {
    node.setAttribute('data-rtip', '');
    if (d.rows.length === 1 && d.rows[0].item) node.setAttribute('data-item', d.rows[0].item);
    RTIP.set(node, d);
  }
  function rewardRow(r, big) {
    const row = el('div', 'lcc-rtip-row');
    const a = r.item ? L().itemAsset(r.item) : null;
    if (a) {
      const img = el('img', 'lcc-rtip-img', '', { alt: '', width: big ? 48 : 32, height: big ? 48 : 32, src: a.src, decoding: 'async' });
      img.addEventListener('error', () => { img.hidden = true; });
      row.appendChild(img);
    }
    const tx = el('div', 'lcc-rtip-tx');
    tx.appendChild(el('strong', 'lcc-rtip-name', r.name));
    tx.appendChild(el('span', 'lcc-rtip-sub', 'Quantity ' + r.qty + (r.swatch ? ' · the house swatch' : '')));
    tx.appendChild(el('span', 'lcc-rtip-sub', earnedWords('Tier ' + r.tier + ' needs ' + r.need + ': ' + r.state)));
    row.appendChild(tx);
    return row;
  }
  function tipHide() { if (tipEl) tipEl.hidden = true; tipOn = null; }
  function tipShow(node) {
    const d = RTIP.get(node);
    if (!d || !tipEl || !tipEl.parentNode) { tipHide(); return; }
    if (tipOn === node) return;
    tipEl.textContent = '';
    for (const r of d.rows) tipEl.appendChild(rewardRow(r, !!d.big));
    if (d.note) tipEl.appendChild(el('p', 'lcc-rtip-note', d.note));
    tipEl.hidden = false;
    tipOn = node;
    const wrap = tipEl.parentNode, wr = wrap.getBoundingClientRect(), r = node.getBoundingClientRect(), vh = root.innerHeight || 800, tw = tipEl.offsetWidth, th = tipEl.offsetHeight;
    let left = r.left - wr.left, top = r.bottom - wr.top + 6;
    left = Math.max(0, Math.min(left, Math.max(0, wr.width - tw)));
    if (r.bottom + 6 + th > vh && r.top - 6 - th >= 0) top = r.top - wr.top - th - 6;
    tipEl.style.left = Math.round(left) + 'px';
    tipEl.style.top = Math.round(top) + 'px';
  }
  // what a control's reward mouse-over holds (rows, the note under them and the sentence the native title used to carry), for the page's tests and for tools; null when it has none
  const tipOf = (node) => { const d = node && RTIP.get(node); return d ? { rows: d.rows.map((r) => Object.assign({}, r)), note: d.note || '', title: d.title || '' } : null; };
  const rtipOf = (e) => (e.target && e.target.closest ? e.target.closest('[data-rtip]') : null);
  // the list's reward cells: with rows from the page the cell takes the picture mouse-over in place of its native title; a cell with no rows keeps its sentence
  function rtipCell(raw, key, td) {
    const rows = raw && raw.rows && Array.isArray(raw.rows[key]) ? raw.rows[key].filter(Boolean) : [];
    if (rows.length) { const title = td.getAttribute('title'); td.removeAttribute('title'); rtipSet(td, { rows, title }); }
    return td;
  }

  // A figure that cannot be read is never a zero: it is "not known".
  const known = (v) => (v === null || v === undefined ? 'not known' : fmt(v));

  // ---- the plaque and the place of the Term and Decrees section ---------------------------------------------------------------------------------------------------------------
  // The decree the plaque shows is the term's own active decree, read by the page (host.decree). There is no sample: the plaque follows what the save holds, after a write as well.
  function shownDecree(host) {
    const dec = host && host.decree ? host.decree() : null;
    return dec && dec.name ? dec : null;
  }
  function plaque(board, host) {
    const ready = board.ready, c = board.counts;
    const a = ready ? c.a : null, h = ready ? c.h : null;
    const win = M().evaluate(board, null).win; // the headline and the tally line are the winner function's words (lcc-model.js, WIN), the same on every surface
    // the contributors the save records under each faction, the biggest first (worked out here from the rows of the term: the save holds amounts, not a ranking); none is "Not in the save"
    const cs = board.contributors || { a: [], h: [] };
    const lines = (k) => (cs[k].length ? cs[k].slice(0, 3).map((c) => ({ name: c.label })) : null);
    const node = L().renderPlaque({ a, h, decree: shownDecree(host), holder: board.term ? board.term.reigning : null, contributors: { a: lines('a'), h: lines('h') }, win });
    for (const st of node.querySelectorAll ? node.querySelectorAll('.lcc-standard') : []) {
      const k = st.classList.contains('lcc-f-h') ? 'h' : 'a', hd = st.querySelector('.lcc-standard-h');
      if (hd) hd.setAttribute('title', cs[k].length ? 'The biggest contributors under ' + FACTION_WORD[k] + ' in the contribution rows of this term in the save, worked out here: the save holds amounts, not a ranking. The amounts are added up over all the Houses of the term, and equal amounts are listed by name, A to Z. Progress that the save records without a contributor is not named here. ' + contribRank(cs[k]) + unnamedNote(cs[k], 3) : 'The save holds no contribution rows under ' + FACTION_WORD[k] + ' for the Houses of this term.');
    }
    return node;
  }
  // ---- the twelve lines in words (milestone M4) ------------------------------------------------------------------------------------------------------------------------------------
  // Everything the page says about a line comes from M().evaluate (a pure reading of the 25 owners): the words here only name what it found. The wording rule (the operator's decision of 2026-11-18, which
  // replaces the earlier "never a win" rule): the phrases of a win come from WIN in lcc-model.js and are never built here; Imperial Dominion is never confirmed (D03), and the sysselraad column is not evidence (D02).
  const lineWord = (l) => l.name.toLowerCase(); // 'row 3', 'column 2', 'main diagonal', 'opposite diagonal'
  const listWords = (xs) => (xs.length < 2 ? xs.join('') : xs.slice(0, -1).join(', ') + ' and ' + xs[xs.length - 1]);

  // ---- the panel head, the filters and the legend ----------------------------------------------------------------------------------------------------------------------------
  function panelHead(board) {
    const head = el('div', 'lcc-bd-panel-head');
    const t = el('div', 'lcc-bd-panel-title');
    t.appendChild(el('p', 'lcc-bd-eyebrow', 'THE GREAT HOUSES'));
    t.appendChild(el('h3', 'lcc-bd-h', board.ready ? 'The Landsraad' : 'The Houses', { id: 'lcc-bd-h' }));
    // just "Term N" (#1033): that the board is a fixed 5 by 5 read from board_index is in Help (The Landsraad board), not in the header
    const subline = board.ready ? (board.term ? 'Term ' + board.term.id : '') : (board.term ? 'Term ' + board.term.id + ' · ' : '') + 'Not a fixed board: the Houses are listed as read';
    if (subline) t.appendChild(el('p', 'lcc-bd-subline', subline));
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
    // the House names only, A to Z (#979): the square number is not in the drop-down (the board and the list show it; the jump is by the House)
    const jumpOpts = [['', 'Choose a House…']].concat(board.houses.slice().sort((a, b) => String(a.name).localeCompare(String(b.name), 'en')).map((h) => [String(h.id), h.name]));
    // Jump to House and the buttons that choose Houses are one group: they stay together on a line and wrap together. The page seats its choose buttons in the slot (host.seat); with no page
    // (or a board that did not pass its checks) the slot stays empty and is not drawn.
    const jl = el('span', 'lcc-bd-jumpline');
    jl.appendChild(sel('Jump to House', 'lcc-bd-jump', jumpOpts, state.sel === null ? '' : state.sel, 'jump'));
    jl.appendChild(el('span', 'lcc-bd-barslot lcc-bd-chooseslot', '', { 'data-lcc-slot': 'choose' }));
    bar.appendChild(jl);
    if (state.layout === 'list' || !board.ready) bar.appendChild(sel('Group', 'lcc-bd-group', [['', 'No groups'], ['region', 'By region'], ['spec', W().bySpecialization]], state.group, 'group'));
    // The count of the Houses shown is said only while a filter narrows the board (#1044): with nothing chosen it would say "25 of 25" and tell nothing. The node stays, empty, as the polite live region.
    const narrowed = state.show !== 'all' || state.hs !== 'any' || state.spec !== 'all';
    bar.appendChild(el('span', 'lcc-bd-shown', narrowed ? matchIds.size + ' of ' + board.houses.length + ' Houses shown' : '', { role: 'status', id: 'lcc-bd-shown' }));
    // the line of the actions on the chosen Houses (Fill, Complete, Reopen, the faction for them and the count of the chosen Houses), seated by the page
    bar.appendChild(el('div', 'lcc-bd-barslot lcc-bd-actslot', '', { 'data-lcc-slot': 'act' }));
    return bar;
  }
  function legend(board, ev) {
    ev = ev || M().evaluate(board, null);
    const w = el('ul', 'lcc-bd-headlines', '', { role: 'list', 'aria-label': 'Key to the board' });
    const k = (cls, text, title) => { const li = el('li', '', '', { title }); li.appendChild(el('i', 'lcc-bd-key lcc-bd-key-' + cls, '', { 'aria-hidden': 'true' })); li.appendChild(root.document.createTextNode(text)); return li; };
    w.appendChild(k('a', 'ATREIDES ' + W().favorWon.toUpperCase()));
    w.appendChild(k('h', 'HARKONNEN ' + W().favorWon.toUpperCase()));
    w.appendChild(k('n', W().favorNotWon.toUpperCase()));
    w.appendChild(k('g', 'WINNING HOUSES · ' + M().WIN.TEXT.won, 'The Houses that make up a win have a gold outline: the five Houses of a line of five held by one faction, or the 13 or more Houses of a faction. ' + M().WIN.TEXT.facts));
    if (board.counts.unknown) w.appendChild(k('u', W().favorUnreadable.toUpperCase(), 'The completed flag and the winner of this House do not agree, so it is counted for neither faction.'));
    return w;
  }
  // what the character has done on this board, in one line under it (the figures the first version of the board kept above it)
  function totals(board) {
    const r = board.rollup;
    const p = el('p', 'lcc-bd-totals');
    p.appendChild(el('b', '', 'Your rewards on this board: '));
    p.appendChild(root.document.createTextNode((r.tiers ? r.reached + ' of ' + r.tiers + ' ' + W().tiersEarned : 'tiers not known') + ' · ' + r.collected + ' of ' + r.reached + ' collected' + (r.waiting ? ' (' + r.waiting + ' waiting at the house representatives)' : '') + (r.unknownDelivery ? ' · ' + r.unknownDelivery + ' earned with no delivery record' : '') + ' · ' + fmt(r.mine) + ' contributed in all Houses.'));
    return p;
  }

  // ---- the board: five by five ---------------------------------------------------------------------------------------------------------------------------------------------------
  // The faction that holds more than half of the Houses on a board that passed its checks ('a' or 'h'), or null. The count of the Houses as the save shows them now; 13 or more is a win
  // (the game works the winner out from the Houses, WIN in lcc-model.js), and the words for it come from that one table.
  function majorityOf(board) {
    if (!board || !board.ready || !board.counts) return null;
    return M().evaluate(board, null).majority; // 13 of 25 or more, decided in one place (the evaluator of lcc-model.js)
  }

  // One tier box of a House card: the approved five boxes. With a host that answers for the House the box is the same control as the Inspector's tier row (same data-lb, same id, same tier number,
  // its own focus key), so the one click handler of the page writes it; without one it is only a picture of the state.
  function tierBox(h, t, ct) {
    const kind = tierKind(t);
    const cls = 'lcc-hc-t ' + (kind === 'not_reached' ? 'off' : TIER_CLASS[kind]);
    const mark = kind === 'not_reached' ? String(t.n) : TIER_MARK[kind];
    const say = 'House ' + h.name + ', tier ' + t.n + ', ' + (t.threshold !== null ? fmt(t.threshold) + ', ' : '') + TIER_LABEL[kind].toLowerCase();
    if (!ct) return el('span', cls, mark, { role: 'img', 'aria-label': say, title: 'Tier ' + t.n + ': ' + TIER_LABEL[kind] });
    // the mouse-over is the Classic list's own text, built once by the page (ct.title, #938); the accessible name keeps the check against the save (#945)
    const b = btn(cls, mark, { 'data-lb': 'tier', 'data-id': h.id, 'data-t': t.n, 'data-fk': 'ct:' + h.id + ':' + t.n, 'aria-pressed': t.reached ? 'true' : 'false', 'aria-disabled': ct.off ? 'true' : 'false', 'aria-label': say + (ct.off ? ', off' : '. ' + ct.tip + ' ' + CHECKED) + (ct.rw && ct.reward ? ' Reward: ' + ct.reward + '.' : ''), title: ct.title || 'Tier ' + t.n + ': ' + TIER_LABEL[kind] + '. ' + ct.tip });
    if (ct.off) b.setAttribute('data-why', ct.tip);
    if (ct.rw) { const title = b.getAttribute('title'); b.removeAttribute('title'); rtipSet(b, { rows: [ct.rw], note: ct.tip, title, big: true }); } // the picture mouse-over in place of the native text (#977); Solari has no rw and keeps its text
    return b;
  }
  // One House card, as the approved design draws it: the number and the specialization emblem on top, the crest, HOUSE NAME, the specialization, the owner, the five tier boxes and the line "N/5 EARNED"
  // with the collected and the waiting counts. A House whose specialization the save does not give has no emblem and says so in words.
  function houseCard(h, matchIds, host, editable, picks, major, ev) {
    // a House of a line of five held by its faction, and a House of a faction with 13 or more Houses, get the one gold win outline (the words are the plaque's, the victory panel's, the legend's, and the House's tooltip and aria text; a card carries no text flag, #961).
    // Both are read from the evaluator, from the Houses as they stand now.
    const lines = ev && h.index !== null ? ev.held.filter((l) => l.indexes.includes(h.index)) : [];
    const inLine = lines.length > 0;
    const dom = !inLine && !!major && h.owner === major;
    const win = ev ? ev.win : null;
    const dim = !matchIds.has(h.id);
    const owner = OWNER_WORD[h.owner] ? h.owner : 'unknown';
    const card = el('article', 'lcc-hc lcc-hc-' + owner, '', { 'data-owner': owner, 'data-selected': state.sel === h.id ? 'true' : null, 'data-dim': dim ? 'true' : null, 'data-compact': 'true', 'data-lines': inLine ? lines.map((l) => l.id).join(' ') : null });
    if (inLine || dom) L().victory(card, true); // the one win outline: the five Houses of a held line and the Houses of the faction with 13 or more
    const lineSays = inLine ? FACTION_WORD[h.owner] + ' hold this House in a line of five: ' + listWords(lines.map(lineWord)) : '';
    const winSays = win && (inLine || dom) ? M().WIN.say(win) : '';
    const sel = btn('lcc-hc-select', '', { 'data-lcb': 'sel', 'data-id': h.id, 'data-fk': 'ht:' + h.id, 'aria-pressed': state.sel === h.id ? 'true' : 'false' });
    const top = el('span', 'lcc-hc-top');
    top.appendChild(el('span', 'lcc-hc-pos', h.number, { 'aria-hidden': 'true' }));
    // the specialization emblem (#963, #976): the path's clean emblem picture (only the emblem, cut from the banner picture) drawn in one tint like the House crest; no frame, no progress edge
    const banner = L().emblemAsset(h.track);
    if (banner) {
      const g = el('span', 'lcc-hc-spec', '', { title: trackWord(h.track) + ' specialization', 'data-track': h.track });
      const img = el('img', 'lcc-hc-spec-img', '', { alt: '', width: 28, height: 28, src: banner.src, decoding: 'async' });
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
    sel.appendChild(el('span', 'lcc-hc-owner', owner === 'a' || owner === 'h' ? '● ' + OWNER_WORD[owner] : owner === 'open' ? '○ ' + W().notWon : '? ' + W().favorUnreadable, { 'data-g': ({ a: 'A', h: 'H', open: '○', unknown: '?' })[owner] }));
    const notes = [];
    if (h.waiting > 0) notes.push(h.waiting + ' reward' + (h.waiting === 1 ? '' : 's') + ' not collected');
    sel.setAttribute('aria-label', [(h.number ? W().square + ' ' + h.number + ', ' : '') + h.name + ', ' + (owner === 'a' || owner === 'h' ? W().favorWon.toLowerCase() + ' by ' + OWNER_WORD[owner] : owner === 'open' ? W().favorNotWon.toLowerCase() : W().favorUnreadable.toLowerCase())].concat(h.track ? [W().specialization.toLowerCase() + ' ' + trackWord(h.track)] : ['specialization not in the save'], h.takenNoTiers ? [W().favorWon.toLowerCase() + ', no tier record in the save'] : h.tierCount ? [h.reached + ' of ' + h.tierCount + ' ' + W().tiersEarned, h.collected + ' collected'] : ['no tiers in the save'], notes, h.unknownDelivery ? [h.unknownDelivery + ' with no delivery record'] : [], inLine ? ['part of a line of five held by ' + FACTION_WORD[h.owner] + ': ' + listWords(lines.map(lineWord)), winSays] : [], dom ? ['one of the 13 or more Houses of ' + FACTION_WORD[major], winSays] : [], dim ? ['not in the current filter'] : []).join(', '));
    sel.setAttribute('title', 'House ' + h.name + ', ' + (h.pos ? h.pos.words + '. ' : '') + (inLine ? lineSays + '. ' + winSays + ' ' : '') + (dom ? 'One of the ' + FACTION_WORD[major] + ' Houses, who hold 13 or more Houses. ' + winSays + ' ' : '') + (takenWhy(h) ? takenWhy(h) + ' ' : '') + 'Click to see its tiers and actions.');
    card.appendChild(sel);
    const ctl = editable ? ctlOf(host, h.id) : null;
    const tiers = el('div', 'lcc-hc-tiers', '', { role: 'group', 'aria-label': 'House ' + h.name + ' reward tiers' + (ctl && ctl.tiers ? '. ' + CHECKED : '') });
    h.tiers.forEach((t, i) => tiers.appendChild(tierBox(h, t, ctl && ctl.tiers ? ctl.tiers[i] : null)));
    card.appendChild(tiers);
    const count = el('div', 'lcc-hc-count');
    if (h.takenNoTiers) count.appendChild(el('b', '', TAKEN.noTiers, { title: TAKEN.noTiersWhy, 'data-taken': 'no-tiers' }));
    else if (h.tierCount) {
      const sp = () => count.appendChild(root.document.createTextNode(' '));
      count.appendChild(el('b', '', h.reached + '/' + h.tierCount + ' EARNED', { title: 'Reward tiers of yours earned in this House: ' + h.reached + ' of ' + h.tierCount + '. Earned is not collected: the counts beside it say how many of them you have collected.' }));
      sp();
      count.appendChild(el('span', 'lcc-hc-q lcc-hc-q-ok', '✓ ' + h.collected, { title: h.collected + ' collected' }));
      sp();
      count.appendChild(el('span', 'lcc-hc-q lcc-hc-q-wait', '◆ ' + h.waiting, { title: h.waiting + ' not collected (waiting at the representative)' }));
      if (h.unknownDelivery) { sp(); count.appendChild(el('span', 'lcc-hc-q lcc-hc-q-unk', '? ' + h.unknownDelivery, { title: h.unknownDelivery + ' earned with no delivery record: not counted as collected or waiting' })); }
    } else count.appendChild(el('b', '', 'NO TIERS IN THE SAVE'));
    card.appendChild(count);
    if (editable) card.appendChild(pickBox(h, picks, dim));
    return card;
  }
  function tile(h, matchIds, host, editable, picks, major, ev) {
    const li = el('li', 'lcc-bd-cell', '', { 'data-id': h.id, 'data-index': h.index });
    li.appendChild(houseCard(h, matchIds, host, editable, picks, major, ev));
    // The square shows the same House hover card as the list rows (#975, #951): built once by the page (host.card), open while the pointer or the keyboard focus is on the square (the page's style sheet
    // and its Escape handling are the list's own: the cell is the card's .hw). It sits beside the card, not inside it: the card's clip path and filter would cut or move it. The card says what the
    // native text of the square said, so the square keeps that text as a data attribute only (two overlays would stack); a touch screen has no hover and keeps the Inspector.
    const card = host && typeof host.card === 'function' ? host.card(h.id) : null;
    if (card) {
      li.classList.add('hw');
      li.appendChild(card);
      const sel = li.querySelector('.lcc-hc-select');
      if (sel && sel.hasAttribute('title')) { sel.setAttribute('data-says', sel.getAttribute('title')); sel.removeAttribute('title'); }
    }
    return li;
  }
  // dim: the card is out of the current filter (#881). Its box is off and never ticked: what is not shown is never chosen for the actions on several Houses.
  function pickBox(h, picks, dim) {
    const l = el('label', 'lcc-bd-pickl');
    const i = el('input', 'lcc-bd-pick', '', { type: 'checkbox', 'data-lb': 'pick', 'data-id': h.id, 'data-fk': 'pk:' + h.id, autocomplete: 'off', 'aria-label': 'Choose House ' + h.name + ' for the actions on several Houses' + (dim ? ' (off: not in the current filter)' : '') }); // autocomplete off: the page draws the box from its own list, never from the browser's saved form state (#921)
    if (dim) { i.disabled = true; l.setAttribute('title', 'Not in the current filter, so it cannot be chosen. Change the filter to choose it.'); }
    else if (picks && picks.has(h.id)) i.checked = true;
    l.appendChild(i);
    return l;
  }
  function grid(board, matchIds, host, editable, picks, ev) {
    const ul = el('ul', 'lcc-bd-tiles', '', { role: 'list', 'aria-label': 'The 25 Houses of the board, five rows of five, numbered 01 to 25 from the top left' });
    ev = ev || M().evaluate(board, null);
    const major = ev.majority;
    for (const h of board.grid) ul.appendChild(tile(h, matchIds, host, editable, picks, major, ev));
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
  const COLS = () => [
    { k: 'position', t: W().square, sort: 'position' }, { k: 'house', t: 'House', sort: 'house' }, { k: 'owner', t: W().wonBy, sort: 'status' }, { k: 'path', t: W().specialization },
    { k: 'location', t: 'Location', sort: 'location' }, { k: 'state', t: 'Your progress', sort: 'state' }, { k: 'tiers', t: 'Tiers', sort: 'tiers' }, { k: 'next', t: 'Next tier', sort: 'next' }, { k: 'waiting', t: 'Not collected' },
  ];
  function locCell(h, host) {
    const td = el('td', 'lcc-bd-loc');
    if (!h.loc.known) { td.appendChild(el('span', 'lcc-bd-mut', 'not mapped yet')); return td; }
    const text = [h.loc.area, h.loc.cell ? 'cell ' + h.loc.cell : ''].filter(Boolean).join(' · ');
    const peek = host && typeof host.peek === 'function' ? host.peek(h.id) : null; // the Classic quick peek (the map picture), shown in the mouse-over only
    const wrap = el('span', 'loc');
    if (h.loc.map_key) wrap.appendChild(btn('lcc-bd-linkb', text, { 'data-lb': 'map', 'data-id': h.id, 'data-fk': 'lm:' + h.id, title: 'Open ' + (h.loc.area || h.loc.map) + ' in the Saved map' + (peek ? '. Hover for a quick peek.' : ''), 'aria-label': 'House ' + h.name + ': ' + text + '. Open in the Saved map' }));
    else { wrap.appendChild(el('span', '', text)); wrap.appendChild(el('span', 'lcc-bd-mut', ' · no map picture')); }
    if (peek) wrap.appendChild(peek);
    td.appendChild(wrap);
    return td;
  }
  function row(h, host, editable, picks, cols) {
    const tr = el('tr', 'lcc-bd-row', '', { 'data-id': h.id, 'aria-current': state.sel === h.id ? 'true' : null });
    if (editable) { const td = el('td', 'lcc-bd-c-pick'); td.appendChild(pickBox(h, picks)); tr.appendChild(td); }
    tr.appendChild(el('td', 'lcc-bd-c-no', h.number || '—'));
    const nameTd = el('td', 'lcc-bd-c-house');
    // the House name carries the Classic hover card (next reward, not collected, faction standing, the place as a link), built once by the page (host.card) and open while the pointer or the focus is on it (#951)
    const hw = el('span', 'hw');
    hw.appendChild(btn('lcc-bd-linkb lcc-bd-namebtn', h.name, { 'data-lcb': 'sel', 'data-id': h.id, 'data-fk': 'ht:' + h.id, 'aria-label': 'House ' + h.name + ': look at its tiers and actions' }));
    const card = host && typeof host.card === 'function' ? host.card(h.id) : null;
    if (card) hw.appendChild(card);
    nameTd.appendChild(hw);
    tr.appendChild(nameTd);
    const raw = host && typeof host.tips === 'function' ? host.tips(h.id) : null;
    const tips = raw ? Object.fromEntries(Object.entries(raw).map(([k, v]) => [k, earnedWords(v)])) : null; // Classic says reached where the board says earned (#962)
    // the mouse-overs of the cells, in the Classic list's words (one source, the page)
    const ow = el('td', 'lcc-bd-c-owner'); ow.appendChild(ownerPill(h.owner)); tr.appendChild(ow);
    tr.appendChild(el('td', 'lcc-bd-c-path', trackWord(h.track) || '—'));
    tr.appendChild(locCell(h, host));
    tr.appendChild(el('td', 'lcc-bd-c-state', h.takenNoTiers ? W().favorWon + ', none of yours' : ({ new: 'Not started', part: 'In progress', done: 'Top tier' })[h.state], takenWhy(h) ? { title: takenWhy(h) } : tips ? { title: tips.progress } : undefined));
    tr.appendChild(rtipCell(raw, 'tiers', el('td', 'lcc-bd-c-tiers', h.takenNoTiers ? 'no tier record' : h.tierCount ? h.reached + ' of ' + h.tierCount + ' ' + W().tiersEarned : 'no tiers', tips ? { title: tips.tiers } : undefined)));
    tr.appendChild(rtipCell(raw, 'next', el('td', 'lcc-bd-c-next', h.next ? fmt(h.next.threshold) + ' (' + fmt(h.next.remaining) + ' more)' : h.tierCount ? W().topTierEarned : '—', tips ? { title: tips.next } : undefined)));
    tr.appendChild(rtipCell(raw, 'waiting', el('td', 'lcc-bd-c-wait', h.waiting ? String(h.waiting) : '0', tips ? { title: tips.waiting } : undefined)));
    return tr;
  }
  function list(board, matchIds, host, editable, picks) {
    const wrap = el('div', 'lcc-bd-listwrap');
    const t = el('table', 'lcc-bd-list');
    t.appendChild(el('caption', 'lcc-bd-cap', 'The Houses of the term' + (board.ready ? ', by default by House name, A to Z' : ', by name: the board could not be checked, so no square is given')));
    const th = el('thead'), hr = el('tr');
    if (editable) hr.appendChild(el('th', 'lcc-bd-c-pick', 'Choose', { scope: 'col' }));
    const cols0 = COLS();
    for (const c of cols0) {
      const sortable = !!c.sort && (board.ready || c.sort !== 'position'); // the square is a position on a board that was checked: with no such board it is not a column to sort by
      const cell = el('th', '', '', { scope: 'col', 'aria-sort': sortable && state.sort === c.sort ? (state.dir === 'asc' ? 'ascending' : 'descending') : null });
      if (sortable) cell.appendChild(btn('lcc-bd-sortb', c.t, { 'data-lcb': 'sort', 'data-col': c.sort, 'data-fk': 'bs:' + c.sort }));
      else cell.textContent = c.t;
      hr.appendChild(cell);
    }
    th.appendChild(hr); t.appendChild(th);
    const cols = cols0.length + (editable ? 1 : 0);
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
        td.textContent = (g.g ? (state.group === 'spec' ? trackWord(g.g) || g.g : g.g) : state.group === 'region' ? 'Location unknown' : W().noSpecialization) + ' · ' + g.rows.length;
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
  // Read-only. What it says is read from the pure evaluator (lcc-model.js) over the 25 Houses as they stand: the counts, the lines of five that one faction holds, and who has won. The save keeps no
  // record of the vote (the operator's decision of 2026-11-18: the game works the winner out from the Houses, a line of five or 13 Houses, so that is what is said). Every phrase comes from WIN in
  // lcc-model.js; this function adds only the Houses and the lines it names. The panel no longer speaks of Imperial Dominion, and its long explanation is gone: the words are the tag, the title, the
  // how-line (BY 13 HOUSES, BY FIVE IN A LINE) and the counts; the full sentence, with the facts line, is for a screen reader and for the tooltip.
  function victoryWords(board, ev) {
    const W = M().WIN, d = ev.win, T = W.TEXT;
    const tag = d.state === 'won' ? T.won : d.state === 'contested' ? T.contested : T.undecidable;
    const say = W.say(d);
    if (!board.ready) return { state: 'unavailable', faction: '', tag, head: W.tally(d), foot: '', say };
    const c = board.counts;
    if (d.state === 'won') {
      const f = d.winner, n = d.lines.length;
      const how = [];
      if (d.routes.includes('line')) how.push(FACTION_WORD[f] + ' hold five Houses in ' + (n === 1 ? 'one line' : n + ' lines') + ': ' + listWords(d.lines.map((id) => lineWord(ev.lines.find((l) => l.id === id)))) + '.');
      if (d.routes.includes('houses')) how.push(FACTION_WORD[f] + ' hold ' + c[f] + ' of the 25 Houses.');
      return { state: d.routes.includes('line') ? 'line' : 'count', faction: f, tag, head: W.headline(d), foot: W.how(d) + (d.conflictPossible ? ' · ' + T.tallyMaybe : ''), say: say + ' ' + how.join(' ') };
    }
    if (d.why === 'both') return { state: 'conflict', faction: '', tag, head: W.tally(d), foot: '', say };
    if (d.why === 'unreadable') return { state: 'unreadable', faction: '', tag, head: W.tally(d), foot: countLine(board, ev), say: say + ' A House that cannot be read is counted for neither faction.' };
    return { state: 'contested', faction: '', tag, head: W.tally(d), foot: countLine(board, ev), say: say + ' ' + T.rule };
  }
  // The House count, in one line (the foot of a panel with no winner).
  function countLine(board, ev) {
    const c = board.counts;
    if (!board.ready) return '';
    if (!board.countComplete) return 'House count incomplete: ' + c.unknown + (c.unknown === 1 ? ' House' : ' Houses') + ' could not be read, so no comparison is made.';
    if (!ev.leader) return 'House count: even at ' + c.a + ' each.';
    return 'House count: ' + FACTION_WORD[ev.leader] + ' ahead ' + c[ev.leader] + ' to ' + c[ev.leader === 'a' ? 'h' : 'a'] + '.';
  }
  // The faction's own crest, filled, in the faction's colour: drawn only when a faction has won. With no picture (a failed load) the same place shows a filled diamond in the same colour.
  function winnerEmblem(f) {
    const e = el('div', 'lcc-bd-emblem lcc-bd-emblem-' + f, '', { 'aria-hidden': 'true', 'data-faction': f });
    const asset = L().factionAsset(f, 'mark');
    const fallback = () => { e.textContent = '◆'; e.setAttribute('data-fallback', 'true'); };
    if (!asset || !asset.src) { fallback(); return e; }
    const img = el('img', 'lcc-bd-emblem-img', '', { alt: '', width: 68, height: 68, src: asset.src, decoding: 'async' });
    img.addEventListener('error', () => { img.remove(); fallback(); });
    e.appendChild(img);
    return e;
  }
  // How many of the twelve lines each faction can still make (the unblocked lines that nobody holds yet; a line with nothing won on it counts for both). With a House that cannot be read the count is
  // incomplete and no number is given: a figure is never guessed. This is the figure that the row of four figures under the plaque used to show as POTENTIAL LINES (#935).
  function linesLeft(board, ev) {
    const p = ev.potential;
    if (!p.exact) return 'Lines still open: count incomplete · ' + board.counts.unknown + (board.counts.unknown === 1 ? ' House' : ' Houses') + ' unreadable';
    return 'Lines still open: ' + p.total + ' · Atreides ' + p.a + ' · Harkonnen ' + p.h + (ev.held.length ? ' · ' + ev.held.length + ' held, not counted' : '');
  }
  function victoryPanel(board, ev) {
    ev = ev || M().evaluate(board, null);
    const v = victoryWords(board, ev);
    const s = el('section', 'lcc-bd-surface lcc-bd-victory', '', { 'aria-label': 'Imperial Victory Analysis', 'aria-live': 'polite', 'data-state': v.state, 'data-faction': v.faction || null, title: v.say });
    s.appendChild(el('p', 'lcc-bd-eyebrow', 'IMPERIAL VICTORY ANALYSIS'));
    s.appendChild(el('div', 'lcc-bd-vtag', v.tag));
    if (v.faction) s.appendChild(winnerEmblem(v.faction)); // a faction has won: its crest; no winner: no icon at all
    s.appendChild(el('h3', 'lcc-bd-vhead', v.head));
    s.appendChild(el('p', 'lcc-bd-sr', v.say)); // the sentence for a screen reader, in the same words
    const c = board.counts;
    s.appendChild(el('div', 'lcc-bd-well', board.ready ? 'Atreides ' + c.a + ' · Harkonnen ' + c.h + ' · Open ' + c.open + (c.unknown ? ' · Unreadable ' + c.unknown : '') : 'Houses read: ' + board.houses.length));
    if (v.foot) s.appendChild(el('p', 'lcc-bd-vcount', v.foot));
    if (board.ready) s.appendChild(el('p', 'lcc-bd-vlines', linesLeft(board, ev), { 'data-lines': ev.potential.exact ? 'exact' : 'incomplete' }));
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
  // The actions block. Presentation only: every button is the same control as before (data-lb="qa", same ids and keys), the faction box drives the board's own faction box, and
  // Apply tier presses the tier box of the ledger that the choice means, so a tier set here goes through the same checks, question and backup as a click on that box.
  function editor(h, c) {
    const s4 = el('section', 'lcc-bd-sec lcc-bd-editor', '', { 'aria-labelledby': 'lcc-bd-s4' });
    // no badge, note or standing sentence here (#948, #980): what the tier boxes, Set tier and the faction box do is in the Help tab, and what a click would write is in its tooltip and its question
    const head = el('div', 'lcc-bd-editor-head');
    head.appendChild(secHead('ACTIONS', 'lcc-bd-s4'));
    s4.appendChild(head);
    const r1 = el('div', 'lcc-bd-erow', '', { role: 'group', 'aria-label': 'Actions on House ' + h.name });
    if (c.factions && c.factions.length) {
      const lab = el('label', 'lcc-bd-elab', 'Faction', { for: 'lcc-bd-fac' });
      const fs = el('select', 'lcc-bd-select lcc-bd-eselect', '', { id: 'lcc-bd-fac', 'data-lcb': 'fac', 'data-fk': 'fac-i', 'data-was': String(c.F), title: 'Acting for ' + c.facName + (c.ownF ? ' (your faction)' : '') });
      for (const f of c.factions) { const o = el('option', '', f.name, { value: String(f.id) }); if (String(f.id) === String(c.F)) o.selected = true; fs.appendChild(o); }
      if (c.busy) fs.disabled = true; // a redraw during a write: the box is built off, as lbRunning turned it off (#882)
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
    if (!h.tierCount || c.busy) ts.disabled = true;
    tl.appendChild(root.document.createTextNode(' '));
    tl.appendChild(ts);
    r2.appendChild(tl);
    r2.appendChild(btn('lcc-bd-act', 'Apply tier', { 'data-lcb': 'settier', 'data-id': h.id, 'data-r': h.reached, 'data-fk': 'at:' + h.id, title: 'Sets your contribution to the chosen tier, exactly as a click on that numbered box does: raises to a higher tier, or removes the highest earned tier.' }));
    r2.appendChild(el('span', 'lcc-bd-etip', W().favorWon + ' ≠ earned ≠ collected'));
    s4.appendChild(r2);
    return s4;
  }

  // The place line under the House name (#939): the same as the Classic list's. The place is the link that opens the Saved map, with the Classic mouse-over text ("Click the place to open the Saved map."
  // or the reason it cannot be opened), and the sub-line says the grid cell, the place and the area. The page builds the words once for both views (host.loc); a click is the Classic click (data-lb="map").
  function locLine(h, host) {
    const m = host && typeof host.loc === 'function' ? host.loc(h.id) : null;
    if (!m) return el('span', 'lcc-bd-bloc lcc-bd-locline', h.loc.known ? [h.loc.area, h.loc.cell ? 'cell ' + h.loc.cell : ''].filter(Boolean).join(' · ') : 'Representative location not mapped yet');
    const b = btn('lcc-bd-loc' + (m.onMap ? '' : ' off'), '', { 'data-lb': 'map', 'data-id': h.id, 'data-fk': 'lm:' + h.id, title: m.hint, 'aria-label': m.aria, 'aria-disabled': m.onMap ? null : 'true', 'data-why': m.onMap ? null : m.why });
    b.appendChild(el('span', 'lcc-bd-loc-a', m.area));
    if (m.sub) b.appendChild(el('span', 'lcc-bd-loc-s', m.sub));
    if (m.hub) b.appendChild(el('span', 'lcc-bd-loc-s', 'no map picture'));
    const peek = typeof host.peek === 'function' ? host.peek(h.id) : null; // the Classic quick peek: the map picture is in the mouse-over only (#940)
    if (!peek) return b;
    b.setAttribute('title', m.hint + (m.onMap ? ' Hover for a quick peek.' : ''));
    const wrap = el('span', 'loc lcc-bd-locwrap');
    wrap.appendChild(b); wrap.appendChild(peek);
    return wrap;
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
    hb.appendChild(el('h4', 'lcc-bd-insp-name', houseWord(h.name), h.number ? { title: W().square + ' ' + h.number + (h.pos && h.pos.words ? ' · ' + h.pos.words : '') } : undefined));
    hb.appendChild(locLine(h, host));
    // the specialization is one line: its name is the button that opens its track in the Specializations tab (it only moves between pages: nothing is written)
    if (h.track && host && typeof host.goPath === 'function') hb.appendChild(btn('lcc-bd-linkb lcc-bd-gopath', trackWord(h.track) + ' Specialization', { 'data-lcb': 'gopath', 'data-track': h.track, 'data-fk': 'gp:' + h.id,
      'aria-label': trackWord(h.track) + ' Specialization. Open ' + trackWord(h.track) + ' in Specializations. It only moves between pages: nothing is written.' }));
    else hb.appendChild(el('span', 'lcc-bd-bloc', h.track ? trackWord(h.track) + ' Specialization' : 'Specialization not in the save'));
    idn.appendChild(hb);
    side.appendChild(idn);

    // the actions on the House come first, directly under the name: they are what the panel is for, and a short window shows them without scrolling the panel. The order of the rest is the approved design's.
    const c = editable ? ctlOf(host, h.id) : null;
    if (c) {
      side.appendChild(editor(h, c));
      // directly under the buttons: where the page draws the question about this House (it moves its own question here), so it stands next to the buttons that asked. A question waiting for an answer is
      // not a toast. The results of a click are toasts (#943): no line of them is kept in the panel.
      const pin = el('div', 'lcc-bd-pin');
      pin.appendChild(el('div', 'lcc-bd-ask', '', { 'data-lcc-slot': 'ask' }));
      side.appendChild(pin);
    } else if (!editable) {
      side.appendChild(el('p', 'lcc-bd-warn', 'Read-only: the board could not be checked, so nothing can be written from this view. The classic list stays available.'));
    }

    // who holds it: only what the save's own two fields say; a lead on the faction goal is a lead and never a win
    const ban = el('div', 'lcc-bd-banner');
    ban.appendChild(el('span', 'lcc-bd-control lcc-bd-control-' + (OWNER_WORD[h.owner] ? h.owner : 'unknown'), h.owner === 'a' || h.owner === 'h' ? W().favorWon.toUpperCase() + ' · ' + FACTION_WORD[h.owner].toUpperCase() : h.owner === 'open' ? W().favorNotWon.toUpperCase() : W().favorUnreadable.toUpperCase()));
    // The approved design has no sentence under the band. What the sentence said stays reachable: as a tooltip and as text for a screen reader. Only a favor the save cannot settle keeps a visible sentence.
    const said = h.owner === 'a' || h.owner === 'h' ? W().favorWon + ' by ' + FACTION_WORD[h.owner] + ' (completed, with a winner the save records).'
      : h.owner === 'open' ? W().favorNotWon + ': no faction has won it.' + (h.leader ? ' ' + FACTION_WORD[h.leader] + ' is ahead on the faction goal: a lead, not a win.' : '')
        : 'Who won the favor of this House cannot be read from the save (the completed flag and the winner do not agree), so this House is counted for neither side.';
    ban.firstChild.setAttribute('title', said);
    const ownerSettled = h.owner === 'a' || h.owner === 'h' || h.owner === 'open';
    if (ownerSettled) ban.appendChild(el('span', 'lcc-bd-sr', said));
    side.appendChild(ban);
    if (!ownerSettled) side.appendChild(el('p', 'lcc-bd-status', said));

    // the two scales, kept apart: the factions' progress against the house goal, then the character's own tiers
    const s1 = el('section', 'lcc-bd-sec', '', { 'aria-labelledby': 'lcc-bd-s1' });
    // the names of the contributors, the biggest first, with no amounts and no placeholder for one the save gives no name for (#1058); the amounts stay in the save and in the plaque's tooltip only
    const named = h.contributors.filter((c) => !c.unnamed).slice(0, 5).map((c) => c.label);
    const s1note = 'House goal: ' + known(h.goal) + '. A faction’s progress is the sum of its contributions to this House. Top contributors: ' + (named.length ? named.join(', ') : h.contributors.length ? 'no names in the save' : NOT_IN_SAVE + (h.owner === 'a' || h.owner === 'h' ? ' (the save has no contributor row for this House, whose favor is won)' : '')) + '. Missions: ' + NOT_IN_SAVE + '.';
    const s1h = secHead('FACTION HOUSE CONTROL', 'lcc-bd-s1', 'SAVE');
    s1h.setAttribute('title', s1note);
    s1.appendChild(s1h);
    s1.appendChild(fcRow('a', 'HOUSE ATREIDES', h.prog.a, h.goal));
    s1.appendChild(fcRow('h', 'HOUSE HARKONNEN', h.prog.h, h.goal));
    s1.appendChild(el('span', 'lcc-bd-sr', s1note));
    if (named.length) {
      const cb = el('div', 'lcc-bd-contrib', '', { title: 'The order is worked out here: the save holds the amounts, not a ranking. A guild is shown by the identifier the save gives it, with capital letters added here (the save holds no display name). A contributor the save gives no name for is left out.' });
      cb.appendChild(el('p', 'lcc-bd-contrib-h', 'TOP CONTRIBUTORS'));
      const cl = el('ol', 'lcc-bd-contrib-l', '', { role: 'list' });
      for (const n of named) cl.appendChild(el('li', '', n));
      cb.appendChild(cl);
      s1.appendChild(cb);
    } else if (!h.contributors.length) s1.appendChild(el('p', 'lcc-bd-mut lcc-bd-nosave', 'Top contributors: not in the save.' + (h.owner === 'a' || h.owner === 'h' ? ' The favor of this House is won but the save holds no contributor row for it (for example when it was completed from this editor).' : '')));
    side.appendChild(s1);

    const top = h.tierCount ? h.top : null;
    const s2 = el('section', 'lcc-bd-sec', '', { 'aria-labelledby': 'lcc-bd-s2' });
    s2.appendChild(secHead('YOUR CONTRIBUTION', 'lcc-bd-s2', 'SAVE'));
    const nums = el('div', 'lcc-bd-pc');
    nums.appendChild(el('strong', '', h.mine > 0 || top !== null ? fmt(h.mine) + (top !== null ? ' / ' + fmt(top) : '') : 'none recorded'));
    nums.appendChild(el('small', '', h.next ? fmt(h.next.remaining) + ' to T' + h.next.n : h.tierCount ? W().topPersonalTierEarned.toUpperCase() : 'no tiers in the save'));
    s2.appendChild(nums);
    const mineSaid = 'Your contribution: ' + (h.mine > 0 ? fmt(h.mine) : 'none recorded') + (top !== null ? ' of ' + fmt(top) : '') + (h.next ? ' · next tier ' + fmt(h.next.threshold) + ', ' + fmt(h.next.remaining) + ' more' : h.tierCount ? ' · ' + W().topTierEarned : '') + '.';
    const pt = el('div', 'lcc-bd-track lcc-bd-track-me', '', { role: 'img', 'aria-label': mineSaid, title: mineSaid });
    const pf = el('i', 'lcc-bd-fill lcc-bd-fill-me'); pf.style.setProperty('--lcc-v', pct(h.mine, top) + '%');
    pt.appendChild(pf);
    s2.appendChild(pt);
    side.appendChild(s2);

    // the ledger: the tiers of the character (cumulative), their rewards and what became of them. The rows are the same controls as the tier boxes of the card.
    const s3 = el('section', 'lcc-bd-sec lcc-bd-ledger', '', { 'aria-labelledby': 'lcc-bd-s3' });
    const lh = el('div', 'lcc-bd-ledger-head');
    lh.appendChild(secHead('PERSONAL REWARD LEDGER', 'lcc-bd-s3', 'SAVE + DERIVED'));
    // the "N OF 5 EARNED" tag is gone (#1059): the Tiers earned tile below says it. Only a House with no tier record keeps a tag, since it has no tiles.
    if (h.takenNoTiers) lh.appendChild(el('span', 'lcc-bd-ledger-n', W().favorWon.toUpperCase() + ' · NO TIER RECORD', { title: takenWhy(h) }));
    s3.appendChild(lh);
    // two figures (#1046): the tiers earned of the five, and how many of the earned tiers are collected. What is not collected is the difference, said in the tooltip and for a screen reader.
    const sumSaid = h.reached + ' of ' + h.tierCount + ' ' + W().tiersEarned + ' · ' + h.collected + ' of ' + h.reached + ' earned ' + (h.reached === 1 ? 'tier' : 'tiers') + ' collected' + (h.waiting ? ', ' + h.waiting + ' waiting at the house representative' : '') + (h.unknownDelivery ? ', ' + h.unknownDelivery + ' with no delivery record' : '') + '.';
    const sum = el('div', 'lcc-bd-stats', '', { role: 'group', 'aria-label': 'House rewards summary: ' + sumSaid, title: sumSaid });
    const stat = (cls, v, l) => { const d = el('div', 'lcc-bd-stat' + (cls ? ' ' + cls : '')); d.appendChild(el('strong', '', v)); d.appendChild(el('span', '', l)); return d; };
    sum.appendChild(stat('', h.reached + ' / ' + h.tierCount, W().tiersEarnedHead));
    sum.appendChild(stat('ok', h.collected + ' / ' + h.reached, 'Collected'));
    if (h.takenNoTiers) s3.appendChild(el('p', 'lcc-bd-mut lcc-bd-taken-note', takenWhy(h)));
    else s3.appendChild(sum);
    if (h.unknownDelivery) s3.appendChild(el('p', 'lcc-bd-alert', '“Delivery unknown” means the tier is earned and the save has no delivery row for it this term: it is not counted as collected or as not collected.'));
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
        for (const [k, v] of Object.entries({ 'data-lb': 'tier', 'data-id': h.id, 'data-t': t.n, 'data-fk': 'tc:' + h.id + ':' + t.n, 'aria-pressed': t.reached ? 'true' : 'false', 'aria-disabled': ct.off ? 'true' : 'false', title: ct.tip, 'aria-label': 'Tier ' + t.n + ', ' + fmt(t.threshold) + ', reward ' + reward + ', ' + t.word + (ct.off ? ', off' : '. ' + ct.tip) + (ct.item ? '. ' + ct.item.split('\n').join('. ') : '') })) b.setAttribute(k, v);
        if (ct.rw) { b.removeAttribute('title'); rtipSet(b, { rows: [ct.rw], note: ct.tip, title: ct.tip, big: true }); } // the item of this reward as the picture mouse-over (#977, it was text drawn by the style sheet, #937); Solari has none
        if (ct.off) b.setAttribute('data-why', ct.tip);
        if (ct.grant) it.appendChild(el('span', 'lcc-bd-tier-gr', ct.grant));
        else if (c.ownF && t.n === h.reached && ct.rm) it.appendChild(el('span', 'lcc-bd-tier-gr lcc-bd-rm', ct.rm)); // what a click does to this reward, from the server's outcome for the tier (the page builds it)
        li.appendChild(b);
      } else li.appendChild(mk('div').b);
      ol.appendChild(li);
    });
    if (!h.tiers.length) s3.appendChild(el('p', 'lcc-bd-none', 'This House has no tiers in the save.'));
    s3.appendChild(ol);
    if (!h.tiersOrdered && h.tiers.length) s3.appendChild(el('p', 'lcc-bd-warn', 'The tier amounts are not in rising order in the save; the earned count is taken as the save states it.'));
    side.appendChild(s3); // no caption, mark legend or note under the ledger (#980): every tier row names its own state, and the Help tab explains the marks, Fill and the three separate states

    // where it is
    const s5 = el('section', 'lcc-bd-sec', '', { 'aria-labelledby': 'lcc-bd-s5' });
    s5.appendChild(secHead('WHERE', 'lcc-bd-s5'));
    if (!h.loc.known) s5.appendChild(el('p', 'lcc-bd-none', 'Not mapped yet: the save holds no map marker for this House’s representative, and the editor does not guess a place.'));
    else {
      s5.appendChild(el('p', 'lcc-bd-line', [h.loc.area, h.loc.cell ? 'grid cell ' + h.loc.cell : '', h.loc.place, h.loc.map && h.loc.map !== h.loc.area ? h.loc.map : ''].filter(Boolean).join(' · ')));
      if (h.loc.unconfirmed) s5.appendChild(el('p', 'lcc-bd-cap', 'This place is from public guides and is not confirmed in the game.'));
      const lm = host && typeof host.loc === 'function' ? host.loc(h.id) : null;
      if (lm && lm.from) s5.appendChild(el('p', 'lcc-bd-line', 'From you: ' + lm.from + '.'));
      if (h.loc.map_key) s5.appendChild(btn('lcc-bd-act', 'Open in the Saved map', { 'data-lb': 'map', 'data-id': h.id, 'data-fk': 'lw:' + h.id }));
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
    const ev = M().evaluate(board, null); // no record of the start of the Voting Session exists in the save: the evidence is always null (D03); the winner is worked out from the Houses (WIN)
    w.appendChild(plaque(board, host));
    // the Term and Decrees section: the page puts its own section here (host.seat), once, in place of what used to be the decree preview strip (#923)
    w.appendChild(el('div', 'lcc-bd-termslot', '', { 'data-lcc-slot': 'term' }));
    if (!board.houses.length) { w.appendChild(el('p', 'lcc-bd-none', 'This save has no Landsraad Houses in the current term, so there is nothing to show or change here.')); return w; }
    if (state.sel !== null && !board.byId.has(state.sel)) state.sel = null;
    const matchIds = new Set(board.houses.filter((h) => M().matches(h, state)).map((h) => h.id));
    const picks = host && host.picks ? host.picks() : null;
    const grid_ = el('div', 'lcc-bd-grid');
    const panel = el('section', 'lcc-bd-surface lcc-bd-panel', '', { 'aria-labelledby': 'lcc-bd-h' });
    panel.appendChild(panelHead(board));
    panel.appendChild(filters(board, matchIds));
    // directly under the control bar: the question before a write, the result line and the note on a write (the page puts them here, see host.seat)
    panel.appendChild(el('div', 'lcc-bd-slot', '', { 'data-lcc-slot': 'bar' }));
    panel.appendChild(legend(board, ev));
    const stage = el('div', 'lcc-bd-stage');
    if (board.ready && state.layout === 'grid') stage.appendChild(grid(board, matchIds, host, editable, picks, ev));
    else stage.appendChild(list(board, matchIds, host, editable, picks));
    panel.appendChild(stage);
    const foot = el('div', 'lcc-bd-foot');
    foot.appendChild(totals(board));
    panel.appendChild(foot);
    grid_.appendChild(panel);
    const side = el('div', 'lcc-bd-side');
    side.appendChild(victoryPanel(board, ev));
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
    const built = build(board, host);
    container.appendChild(built);
    tipEl = el('div', 'lcc-rtip', '', { 'aria-hidden': 'true', hidden: '' }); // the one reward mouse-over of this board (#977)
    built.appendChild(tipEl);
    tipOn = null;
    if (!container.__lccWired) {
      container.__lccWired = true;
      container.addEventListener('mouseover', (e) => { const n = rtipOf(e); if (n) tipShow(n); else tipHide(); });
      container.addEventListener('mouseleave', tipHide);
      container.addEventListener('focusin', (e) => { const n = rtipOf(e); if (n) tipShow(n); else tipHide(); });
      container.addEventListener('focusout', tipHide);
      container.addEventListener('keydown', (e) => { if (e.key === 'Escape') tipHide(); });
      container.addEventListener('click', tipHide, true);
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
  function focusNode(fk) { const n = last && last.container.querySelector('[data-fk="' + fk + '"]'); if (n && n.focus) n.focus({ preventScroll: true }); }
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
  // Choose a House and bring its panel into view: a click on the card's name button chooses it or, when it is the chosen one, lets it go (toggle); a tap of a tier box on a touch screen only chooses it.
  function chooseHouse(id, toggle) {
    state.sel = toggle && state.sel === id ? null : id; memo.insp = null;
    redraw();
    const insp = last.container.querySelector('.lcc-bd-insp');
    if (state.sel !== null && insp && insp.scrollIntoView) insp.scrollIntoView({ block: 'nearest' });
  }
  // After a filter changes, a House the filter leaves out is no longer chosen for a bulk action (the same rule as the classic list: what is not shown is never written).
  // visible(answer): the ids of the Houses the filters let through now (the page asks for them when it chooses "all Houses": only what the board shows is chosen).
  function visible(answer) { return new Set(M().readBoard(answer).houses.filter((h) => M().matches(h, state)).map((h) => h.id)); }
  function filtered() {
    if (!last) return;
    if (last.host && last.host.filtered) last.host.filtered(visible(last.answer));
  }
  // A touch screen (a coarse pointer, #880): a tap on a tier box of a House card cannot show what it would pay (the mouse-over text is never shown), so it only chooses the House and opens it in the House
  // Intelligence panel, whose tier rows say what each tier grants and write through the panel's own buttons (the same questions and requests as before). Nothing is written by the tap. A mouse still writes
  // on a click, as before, with the mouse-over card as the preview; the keyboard (Enter or Space on a box, a click with detail 0) and a click sent by script keep writing on every device.
  function coarsePointer() {
    try { return !!(root.matchMedia && root.matchMedia('(pointer: coarse)').matches); } catch (err) { return false; }
  }
  function onClick(e) {
    const box = e.target && e.target.closest ? e.target.closest('.lcc-hc-t[data-lb="tier"]') : null;
    if (box && last && e.detail > 0 && coarsePointer()) {
      e.preventDefault(); e.stopPropagation(); // the page's own click handler (the one that writes) never sees this tap
      chooseHouse(Number(box.getAttribute('data-id')), false);
      return;
    }
    const t = e.target && e.target.closest ? e.target.closest('[data-lcb]') : null;
    if (!t || !last) return;
    const k = t.getAttribute('data-lcb');
    if (k === 'sel') {
      e.preventDefault();
      chooseHouse(Number(t.getAttribute('data-id')), true);
      return;
    }
    if (k === 'back') {
      e.preventDefault();
      const card = last.container.querySelector('[data-fk="ht:' + t.getAttribute('data-id') + '"]');
      if (card) { if (card.scrollIntoView) card.scrollIntoView({ block: 'center' }); card.focus({ preventScroll: true }); }
      return;
    }
    if (k === 'gopath') { e.preventDefault(); if (last.host && typeof last.host.goPath === 'function') last.host.goPath(t.getAttribute('data-track')); return; }
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
      const insp = t.closest('.lcc-bd-insp'), sel = insp && insp.querySelector('#lcc-bd-tierset');
      const n = sel ? Number(sel.value) : NaN, r = Number(t.getAttribute('data-r'));
      const tell = (m) => { if (m && last.host && typeof last.host.say === 'function') last.host.say(m); }; // a reason is a toast of the page, not a line kept in the panel (#943)
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
      box.setAttribute('data-focusfk', t.getAttribute('data-fk') || ''); // after the write the focus stays on Apply tier, not on the ledger row it pressed (#883)
      try { box.click(); } finally { box.removeAttribute('data-focusfk'); }
    }
  }
  function onChange(e) {
    const t = e.target, k = t && t.getAttribute ? t.getAttribute('data-lcb') : '';
    if (!k) return;
    if (k === 'spec') { state.spec = M().TRACKS.includes(t.value) ? t.value : 'all'; filtered(); redraw(); return; }
    if (k === 'group') { state.group = t.value === 'region' || t.value === 'spec' ? t.value : ''; redraw(); return; }
    if (k === 'fac') { // the board's own faction box takes the choice; when it does not (it is off during a write) the panel's box goes back to the faction that is used, so it never shows one that was not applied (#882)
      const took = last && last.host && last.host.setFac ? last.host.setFac(t.value) : false;
      if (took === false) { const was = t.getAttribute('data-was'); if (was !== null) t.value = was; }
      return;
    }
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

  const api = { state, mount, redraw, keepInView, visible, build, tipOf, majorityOf, victoryWords, countLine, plaque, grid, houseCard, list, inspector, victoryPanel, tierKind, NOT_IN_SAVE };
  if (typeof module !== 'undefined' && module.exports) module.exports = api;
  else root.LCCB = api;
})(typeof window !== 'undefined' ? window : globalThis);
