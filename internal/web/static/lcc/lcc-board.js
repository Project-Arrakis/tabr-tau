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
  const OWNER_WORD = { a: 'Atreides', h: 'Harkonnen', get open() { return W().undecided; }, get unknown() { return W().unreadable; } };
  const NOT_IN_SAVE = 'Not in the save';
  const CHECKED = 'Each click is checked against the save first.';
  // A House the save says is taken but that shows nothing of the character's own (#911): said in words, never as "0/5 EARNED". The tiers are the character's own ladder, so with no row of the
  // character none is earned, and with no tier rows at all there is no ladder to show.
  // A House pledged to a faction with no tier rows in the save says so. A pledged House with tiers but none of the character's own shows the count of any other House (0/5 EARNED, the check and the diamond), #1022.
  const TAKEN = { get noTiers() { return W().pledged + ' - no tier record in the save'; },
    get noTiersWhy() { return 'This House is pledged to a faction, and the save holds no reward tiers for it.'; } };
  const takenWhy = (h) => (h.takenNoTiers ? TAKEN.noTiersWhy : '');
  // the banner's three, whole, for its mouse-over text: a narrow cloth may show fewer of them, never a part of one (#972)
  // A contributor the save names no guild for is shown as "Unnamed contributor"; the save's own number for it (which means nothing as a rank) is only said in the mouse-over text, so two of them can be told apart (#919)
  const unnamedNote = (list, n) => (list.slice(0, n).some((c) => c.unnamed) ? ' Unnamed contributor: the save gives no player or guild name for it.' : ''); // no save number or field name in a tooltip (#1057)
  const contribRank = (list) => 'Top ' + Math.min(3, list.length) + ': ' + list.slice(0, 3).map((c, i) => (i + 1) + '. ' + c.label).join('; ') + '.';
  // The five states of one reward tier, as the approved design shows them: the class of the box, its mark, and the words. A tier the character has not reached is always "not reached", whatever the
  // reward row says; a reached tier with no delivery record (or a record that disagrees with the contribution) is a question mark and is never counted as collected.
  const TIER_CLASS = Object.freeze({ delivered: 'delivered', waiting: 'waiting', no_row: 'uncertain', unknown: 'uncertain' });
  const TIER_MARK = Object.freeze({ delivered: '✓', waiting: '◆', no_row: '?', unknown: '?' });
  const TIER_LABEL = Object.freeze({ not_reached: 'NOT REACHED', waiting: 'NOT COLLECTED', delivered: 'COLLECTED', no_row: 'NO DELIVERY RECORD', unknown: 'NO DELIVERY RECORD' });
  const tierKind = (t) => (t.reached ? (TIER_CLASS[t.state] ? t.state : 'unknown') : 'not_reached');

  // The view state. It lives here, not in the answer, so a redraw after a write keeps the filter, the layout and the House that was looked at.
  const state = { layout: 'grid', show: 'all', hs: 'any', spec: 'all', sort: 'house', dir: 'asc', group: '', sel: null, specs: [] }; // specs: the specializations the Filter menu has on (none is all)
  let last = null; // { container, answer, host } of the latest mount, so a change of the view can draw again without the page's help
  let inited = false;
  // What a redraw keeps for the person (display only): where the House Intelligence panel was scrolled to (for the House it was on) and which disclosures were opened.
  const memo = { insp: null, open: new Set() };

  // ---- small helpers -------------------------------------------------------------------------------------------------------------------------------------------------------------
  function btn(cls, text, attrs) { return el('button', cls, text, Object.assign({ type: 'button' }, attrs || {})); }
  const trackWord = (t) => TRACK_WORD[t] || '';
  const pct = (v, goal) => (goal > 0 && v !== null ? Math.max(0, Math.min(100, Math.round((v / goal) * 100))) : 0);
  // The list cell of the faction a House is pledged to: ONE mark, the faction's name in the faction's colour (#950). A letter before the name read as a second H (H over HARKONNEN) once the cell wrapped, so there is none;
  // a House no faction has claimed says Undecided (or Wavering) in words only (the open circle before it went in #1083), and a House that cannot be read the question mark, each on one line (#1073).
  const ownerPill = (h) => { const owner = OWNER_WORD[h.owner] ? h.owner : 'unknown', p = M().pledgeOf(h); return el('span', 'lcc-owner lcc-owner-' + owner, owner === 'a' || owner === 'h' || owner === 'open' ? p.word : '? ' + p.word, { title: p.line }); };
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
      if (hd) hd.setAttribute('title', cs[k].length ? 'The biggest contributors under ' + FACTION_WORD[k] + ' this term. The amounts are added up over all the Houses of the term, and equal amounts are listed by name, A to Z. Progress with no contributor recorded is not named here. ' + contribRank(cs[k]) + unnamedNote(cs[k], 3) : 'The save holds no contribution rows under ' + FACTION_WORD[k] + ' for the Houses of this term.');
    }
    return node;
  }
  // ---- the twelve lines in words (milestone M4) ------------------------------------------------------------------------------------------------------------------------------------
  // Everything the page says about a line comes from M().evaluate (a pure reading of the 25 owners): the words here only name what it found. The wording rule (the operator's decision of 2026-11-18, which
  // replaces the earlier "never a win" rule): the phrases of a win come from WIN in lcc-model.js and are never built here; Imperial Dominion is never confirmed (D03), and the sysselraad column is not evidence (D02).
  const lineWord = (l) => l.name.toLowerCase(); // 'row 3', 'column 2', 'main diagonal', 'opposite diagonal'
  const listWords = (xs) => (xs.length < 2 ? xs.join('') : xs.slice(0, -1).join(', ') + ' and ' + xs[xs.length - 1]);

  // ---- the panel head, the filters and the legend ----------------------------------------------------------------------------------------------------------------------------
  // The character's level in a House's specialization (#646): 'Level 57', or a dash when the level is not known (the title says why). One wording on the card's mouse-over, the emblem's and the House panel.
  const LEVEL_WHY = 'Not read yet: the level comes from the Specializations tab, which reads it when it is opened';
  const levelText = (h) => 'Level ' + (h.level === null || h.level === undefined ? '—' : h.level);
  // The summary line of the term (#440): how many Houses are at the top tier, in progress and not started, counted from the same House state the Show tabs filter by (M().matches); each count is a key that sets that Show tab.
  // Nothing is drawn for a character in no faction (every House would read as not started). It writes nothing: it only changes which Houses are shown.
  function summary(board) {
    if (board.me.factionId === null || !board.houses.length) return null;
    const n = { done: 0, part: 0, new: 0 };
    for (const k of Object.keys(n)) n[k] = board.houses.filter((h) => M().matches(h, { show: k })).length;
    const names = Object.fromEntries(M().SHOWS);
    const w = el('div', 'lcc-bd-sum', '', { role: 'group', 'aria-label': 'Your progress this term, by House' });
    ['done', 'part', 'new'].forEach((k, i) => {
      if (i) w.appendChild(el('span', 'lcc-bd-sum-sep', '·', { 'aria-hidden': 'true' }));
      const b = btn('lcc-bd-linkb lcc-bd-sumb', '', { 'data-lcb': 'filter', 'data-key': 'show', 'data-val': k, 'data-fk': 'sm:' + k, 'aria-pressed': state.show === k ? 'true' : 'false',
        title: 'Show only the ' + n[k] + (n[k] === 1 ? ' House' : ' Houses') + ' at ' + names[k] + '. The same as the Show tab ' + names[k] + '. It changes nothing in the save.' });
      b.appendChild(el('b', '', String(n[k])));
      b.appendChild(root.document.createTextNode(' ' + names[k].toLowerCase()));
      w.appendChild(b);
    });
    return w;
  }
  function panelHead(board) {
    const head = el('div', 'lcc-bd-panel-head');
    const t = el('div', 'lcc-bd-panel-title');
    t.appendChild(el('p', 'lcc-bd-eyebrow', 'THE GREAT HOUSES'));
    t.appendChild(el('h3', 'lcc-bd-h', board.ready ? 'The Landsraad' : 'The Houses', { id: 'lcc-bd-h', title: totals(board) }));
    // just "Term N" (#1033): that the board is a fixed 5 by 5 read from board_index is in Help (The Landsraad board), not in the header
    const subline = board.ready ? (board.term ? 'Term ' + board.term.id : '') : (board.term ? 'Term ' + board.term.id + ' · ' : '') + 'The save gives no 5 by 5 board, so the Houses are listed';
    if (subline) t.appendChild(el('p', 'lcc-bd-subline', subline));
    const sum = summary(board);
    if (sum) t.appendChild(sum);
    head.appendChild(t);
    const act = el('div', 'lcc-bd-panel-actions');
    const gridOk = board.ready;
    const lay = el('div', 'lcc-bd-layout', '', { role: 'group', 'aria-label': 'Board layout' });
    lay.appendChild(btn('', 'Grid', { 'aria-pressed': state.layout === 'grid' && gridOk ? 'true' : 'false', 'aria-disabled': gridOk ? null : 'true', title: gridOk ? null : 'The board cannot be drawn as 5 by 5: ' + board.problems[0], 'data-lcb': 'layout', 'data-val': 'grid', 'data-fk': 'bf:layout:grid' }));
    lay.appendChild(btn('', 'List', { 'aria-pressed': state.layout === 'list' || !gridOk ? 'true' : 'false', 'data-lcb': 'layout', 'data-val': 'list', 'data-fk': 'bf:layout:list' }));
    act.appendChild(lay);
    head.appendChild(act);
    return head;
  }
  // ---- the control area (tau #974, the approved design "E6 v2") ----------------------------------------------------------------------------------------------------------------
  // ONE sticky strip at the top of the board panel, in two modes in the same place. Nothing chosen: the Show tabs, the Filter key (a categorized menu: House with the pledge row and the 25 Houses as tickable items,
  // Specialization, Group), the Find House key, one chip per active filter category (never one per House: a House in the menu TICKS its square), the count and Choose shown. Houses chosen: the action bar (the count with
  // "chosen", the faction crests, Fill, Complete, Reopen, Clear); the Filter key and the chips are away until the choice is cleared, and the filters stay applied. The question before a write takes the whole bar.
  // The controls that write are the page's own (data-lb "pickall", "pickno", "bulk", "cyes", "cno" and the faction box): this file only places and restyles them (host.seat) and shows the mode. The strip swaps when the Filter menu
  // CLOSES (Done, Escape, a click outside, Tab out) or at once when a square is ticked on the board; a House the filters hide is never counted and never written (the page's rule, lbDropHidden).
  const NS = 'http://www.w3.org/2000/svg';
  const ICONS = Object.freeze({
    sliders: [['path', 'M3.5 7h8M16.5 7h4M3.5 17h3M11.5 17h9'], ['circle', 14, 7, 2.3], ['circle', 9, 17, 2.3]],
    chev: [['path', 'M6 9.5l6 6 6-6']],
    pin: [['path', 'M12 21.5S5.5 15.8 5.5 10.2a6.5 6.5 0 0 1 13 0c0 5.6-6.5 11.3-6.5 11.3z'], ['circle', 12, 10.2, 2.4]],
    eye: [['path', 'M2.5 12S6 5.8 12 5.8 21.5 12 21.5 12 18 18.2 12 18.2 2.5 12 2.5 12z'], ['circle', 12, 12, 3]],
    x: [['path', 'M6 6l12 12M18 6L6 18']],
    fill: [['path', 'M5 4h14l-1.4 14.2a2 2 0 0 1-2 1.8H8.4a2 2 0 0 1-2-1.8z'], ['path', 'M5.9 11.2c2-1.6 4 1.6 6.1 0s4.1 1.6 6.1 0']],
    done: [['circle', 12, 12, 8.8], ['path', 'M7.8 12.4l3 3 5.4-6.6']],
    reopen: [['path', 'M4.4 12a7.6 7.6 0 1 0 2.3-5.4'], ['path', 'M4.2 3.8v4.6h4.6']],
    more: [['path', 'M5 12h.01M12 12h.01M19 12h.01']],
  });
  function icon(name, extra) {
    const s = root.document.createElementNS(NS, 'svg');
    s.setAttribute('class', 'lcc-k-ic' + (extra ? ' ' + extra : '')); s.setAttribute('viewBox', '0 0 24 24'); s.setAttribute('aria-hidden', 'true');
    for (const p of ICONS[name] || []) {
      const n = root.document.createElementNS(NS, p[0]);
      if (p[0] === 'path') n.setAttribute('d', p[1]); else { n.setAttribute('cx', p[1]); n.setAttribute('cy', p[2]); n.setAttribute('r', p[3]); }
      s.appendChild(n);
    }
    return s;
  }
  // the view state of the control area that must outlive a redraw: the Filter menu (open or not, its category, the House search), the Find House list and its search
  const ui = { menu: false, cat: 'house', q: '', view: 'cats', find: false, fq: '' };
  const phone = () => { try { return !!(root.matchMedia && root.matchMedia('(max-width: 700px)').matches); } catch (err) { return false; } };
  const reducedMotion = () => { try { return !!(root.matchMedia && root.matchMedia('(prefers-reduced-motion: reduce)').matches); } catch (err) { return false; } };
  const lamp = () => el('span', 'lcc-k-lamp', '', { 'aria-hidden': 'true' });
  // a mouse-over text that keyboard focus shows too (the kit's .lcc-k-tip); a visible label comes first, the tip only says more and is never needed to operate the control
  function tipped(node, text, pos) {
    node.classList.add('lcc-k-tipped');
    for (const p of String(pos || '').split(' ').filter(Boolean)) node.classList.add('lcc-k-tip-' + p);
    node.appendChild(el('span', 'lcc-k-tip' + (text.length > 40 ? ' lcc-k-tip-wide' : ''), text, { 'aria-hidden': 'true' }));
    return node;
  }
  function kbtn(cls, label, attrs, ic, tip, pos) {
    const b = btn('lcc-k-btn lcc-k-fr ' + cls, '', attrs);
    if (ic) b.appendChild(icon(ic));
    b.appendChild(el('span', 'lcc-k-bl', label));
    return tip ? tipped(b, tip, pos) : b;
  }
  const SHOW_OFF = 'Not available while Houses are chosen. Clear the choice to change which Houses are shown.';
  const SPEC_LIST = Object.freeze(['crafting', 'gathering', 'exploration', 'combat', 'sabotage']);
  const groupOn = (board) => !!state.group && (state.layout === 'list' || !board.ready);
  // one chip per active filter category (House, Specialization, Group), never one per House; Show is not a chip (its tabs are always on the strip)
  function activeFilters(board) {
    const out = [];
    if (state.hs !== 'any') { const t = state.hs === 'open' ? W().undecided : W().pledged; out.push({ key: 'hs', text: 'House: ' + t, full: 'House: ' + t }); }
    if (state.specs.length) {
      const names = state.specs.map(trackWord);
      out.push({ key: 'spec', text: 'Specialization: ' + (names.length >= 3 ? names.length + ' chosen' : names.join(' + ')), full: 'Specialization: ' + names.join(', ') });
    }
    if (groupOn(board)) { const t = state.group === 'region' ? 'By region' : W().bySpecialization; out.push({ key: 'group', text: 'Group: ' + t, full: 'Group: ' + t }); }
    return out;
  }
  function chip(f) {
    const c = el('span', 'lcc-k-chip', '', { 'data-lk': 'chip', 'data-key': f.key, title: f.full });
    c.appendChild(el('span', '', f.text));
    const x = btn('lcc-k-chipx', '', { 'data-lk': 'chipx', 'data-key': f.key, 'data-fk': 'cx:' + f.key, 'aria-label': 'Remove ' + f.full, title: 'Remove ' + f.full });
    x.appendChild(icon('x'));
    c.appendChild(x);
    return c;
  }
  const letterOf = (name) => (/^atreides/i.test(String(name)) ? 'a' : /^harkonnen/i.test(String(name)) ? 'h' : '');
  function strip(board, matchIds) {
    const s = el('div', 'lcc-k-strip', '', { 'data-part': 'strip' });
    const lens = el('div', 'lcc-k-lens lcc-k-pane lcc-k-pane-f', '', { 'data-part': 'lens' });
    const tray = el('div', 'lcc-k-tray lcc-k-fr lcc-k-lens-tray');
    const grp = el('div', 'lcc-k-grp');
    grp.appendChild(tipped(el('span', 'lcc-k-gl', 'Show'), 'Show changes only which Houses are shown. It never changes which Houses are chosen.', 'below start'));
    const tabs = el('div', 'lcc-k-tabs', '', { role: 'radiogroup', 'aria-label': 'Show' });
    for (const [k, t] of M().SHOWS) {
      const on = state.show === k;
      const b = el('button', 'lcc-k-tab lcc-k-fr', t, { type: 'button', role: 'radio', 'aria-checked': on ? 'true' : 'false', tabindex: on ? '0' : '-1', 'data-lcb': 'filter', 'data-key': 'show', 'data-val': k, 'data-fk': 'bf:show:' + k });
      b.appendChild(lamp());
      tabs.appendChild(b);
    }
    grp.appendChild(tabs); tray.appendChild(grp); lens.appendChild(tray);
    const act = activeFilters(board);
    const fw = el('span', 'lcc-k-fwrap');
    const fb = kbtn('lcc-k-fbtn', 'Filter', { 'aria-haspopup': 'dialog', 'aria-expanded': ui.menu ? 'true' : 'false', 'aria-controls': 'lcc-bd-menu', 'data-lk': 'fbtn', 'data-fk': 'fbtn' }, 'sliders', 'Opens the Filter menu: House, Specialization and Group (list layout). What is filtered shows as chips beside this key.', 'below');
    fb.appendChild(el('span', 'lcc-k-fcnt', String(act.length), { 'data-lk': 'fcnt', hidden: act.length ? null : '' }));
    fb.appendChild(icon('chev', 'lcc-k-chev')); fb.appendChild(lamp());
    fw.appendChild(fb); lens.appendChild(fw);
    const dw = el('span', 'lcc-k-fwrap lcc-k-findwrap');
    const fd = kbtn('lcc-k-fbtn lcc-k-findbtn', 'Find House', { 'aria-haspopup': 'dialog', 'aria-expanded': ui.find ? 'true' : 'false', 'aria-controls': 'lcc-bd-find', 'aria-label': 'Find House', 'data-lk': 'findbtn', 'data-fk': 'findbtn' }, 'pin', 'Find House: scrolls the board to one House', 'below');
    fd.appendChild(lamp());
    dw.appendChild(fd); lens.appendChild(dw);
    lens.appendChild(el('i', 'lcc-k-brk', '', { 'aria-hidden': 'true' }));
    const chips = el('span', 'lcc-k-chips', '', { role: 'group', 'aria-label': 'Active filters', 'data-lk': 'chips' });
    for (const f of act) chips.appendChild(chip(f));
    lens.appendChild(chips);
    const cg = el('span', 'lcc-k-cg');
    const count = tipped(el('span', 'lcc-k-count', '', { role: 'status', id: 'lcc-bd-shown' }), 'Houses shown by the filters. A House that is not shown is never counted and never written.', 'below end');
    count.insertBefore(icon('eye'), count.firstChild);
    const nums = el('span', '', '', { 'aria-hidden': 'true' });
    nums.appendChild(el('b', '', String(matchIds.size), { 'data-lk': 'shown' })); nums.appendChild(root.document.createTextNode(' / ')); nums.appendChild(el('b', '', String(board.houses.length)));
    count.appendChild(nums);
    count.appendChild(el('span', 'lcc-k-sr', matchIds.size + ' of ' + board.houses.length + ' Houses shown', { 'data-lk': 'shownsr' }));
    cg.appendChild(count);
    cg.appendChild(el('span', 'lcc-k-hid', '', { 'data-lk': 'hid', hidden: '' }));
    cg.appendChild(el('span', 'lcc-bd-barslot lcc-k-chooseslot', '', { 'data-lcc-slot': 'choose' }));
    lens.appendChild(cg);
    s.appendChild(lens);
    const cmd = el('section', 'lcc-k-cmd lcc-k-fr lcc-k-pane lcc-k-pane-a', '', { 'data-part': 'action', role: 'group', 'aria-label': 'Actions on the chosen Houses' });
    cmd.appendChild(el('i', 'lcc-k-hair', '', { 'aria-hidden': 'true' }));
    cmd.appendChild(kbtn('lcc-k-findbtn lcc-k-findbar', 'Find House', { 'aria-haspopup': 'dialog', 'aria-expanded': 'false', 'aria-controls': 'lcc-bd-find', 'aria-label': 'Find House', 'data-lk': 'findbtn', 'data-fk': 'findbtn2' }, 'pin', 'Find House: scrolls the board to one House', 'below start'));
    cmd.appendChild(el('span', 'lcc-bd-barslot lcc-k-actslot', '', { 'data-lcc-slot': 'act' }));
    cmd.appendChild(el('span', 'lcc-bd-barslot lcc-k-askslot', '', { 'data-lcc-slot': 'askbar' }));
    s.appendChild(cmd);
    return s;
  }
  // the Filter menu: categories at the left, their items at the right, a footer with Clear filters, the number of Houses chosen and Done. On a phone it is a bottom sheet (categories first, then the items of one).
  function menu(board) {
    const p = el('div', 'lcc-k-pop5 lcc-k-fr', '', { id: 'lcc-bd-menu', role: 'dialog', 'aria-label': 'Filter', 'data-lk': 'pop', 'data-view': ui.view, hidden: ui.menu ? null : '' });
    p.appendChild(el('div', 'lcc-k-p5-head')).appendChild(el('span', '', 'Filter'));
    const body = el('div', 'lcc-k-p5-body');
    const cats = el('div', 'lcc-k-p5-cats', '', { role: 'tablist', 'aria-orientation': 'vertical', 'aria-label': 'Filter by' });
    const gridOnly = state.layout !== 'list' && board.ready; // Group is a list-layout control: in the grid its row stays, off, and says why
    const n = { house: state.hs !== 'any' ? 1 : 0, spec: state.specs.length, group: groupOn(board) ? 1 : 0 };
    const catDefs = [['house', 'House'], ['spec', W().specialization], ['group', 'Group']];
    if (gridOnly && ui.cat === 'group') ui.cat = 'house';
    for (const [k, t] of catDefs) {
      const off = k === 'group' && gridOnly;
      const c = el('button', 'lcc-k-cat', '', { type: 'button', role: 'tab', 'aria-selected': ui.cat === k ? 'true' : 'false', tabindex: ui.cat === k ? '0' : '-1', 'data-lk': 'cat', 'data-cat': k, 'data-fk': 'mn:cat:' + k, 'aria-disabled': off ? 'true' : null });
      c.appendChild(el('span', '', t));
      if (off) c.appendChild(el('span', 'lcc-k-cond', 'List layout only'));
      c.appendChild(el('span', 'lcc-k-c', n[k] ? String(n[k]) : '', { 'data-lk': 'cc' }));
      cats.appendChild(c);
    }
    body.appendChild(cats);
    const items = el('div', 'lcc-k-p5-items', '', { role: 'tabpanel', 'aria-label': 'Choices' });
    const back = el('div', 'lcc-k-p5-back');
    back.appendChild(btn('lcc-k-link', 'Back', { 'data-lk': 'back', 'data-fk': 'mn:back' }));
    back.appendChild(el('span', 'lcc-k-p5-t', (catDefs.find((c) => c[0] === ui.cat) || catDefs[0])[1], { 'data-lk': 'ptitle' }));
    items.appendChild(back);
    // House: the pledge row, the search and the 25 Houses (each a box: ticking it ticks the square on the board, it never hides the others)
    const ph = el('div', 'lcc-k-p5-panel', '', { 'data-cat': 'house', hidden: ui.cat === 'house' ? null : '' });
    const pledge = el('div', 'lcc-k-tabs', '', { role: 'radiogroup', 'aria-label': 'House' });
    for (const [k, t] of M().HOUSE_FILTERS) {
      const on = state.hs === k;
      const b = el('button', 'lcc-k-tab lcc-k-fr', t, { type: 'button', role: 'radio', 'aria-checked': on ? 'true' : 'false', tabindex: on ? '0' : '-1', 'data-lcb': 'filter', 'data-key': 'hs', 'data-val': k, 'data-fk': 'bf:hs:' + k });
      b.appendChild(lamp());
      if (k === 'open') tipped(b, 'No faction has the favor of the House yet. Wavering: both factions have progress on it.', 'below');
      if (k === 'taken') tipped(b, 'The House is pledged to a faction: ' + W().pledgedTo + ' Atreides or ' + W().pledgedTo + ' Harkonnen.', 'below end');
      pledge.appendChild(b);
    }
    ph.appendChild(pledge);
    ph.appendChild(tipped(el('p', 'lcc-k-p5-sub', 'Houses', { id: 'lcc-bd-houses-h' }), 'Ticks the House on the board. A ticked House is chosen. It does not hide the others.', 'below start'));
    ph.appendChild(el('input', 'lcc-k-search', '', { type: 'search', 'data-lk': 'hsearch', 'data-fk': 'mn:search', 'aria-label': 'Search Houses', placeholder: 'Search Houses', autocomplete: 'off', value: ui.q }));
    ph.appendChild(el('p', 'lcc-k-p5-n', '', { role: 'status', 'data-lk': 'hn' }));
    const hl = el('div', 'lcc-k-p5-list', '', { role: 'group', 'aria-label': 'Houses', 'aria-describedby': 'lcc-bd-houses-h', 'data-list': 'h' });
    for (const h of board.houses.slice().sort((a, b) => String(a.name).localeCompare(String(b.name), 'en'))) hl.appendChild(btn('lcc-k-opt', L().houseTitle(h.name), { role: 'checkbox', 'aria-checked': 'false', tabindex: '-1', 'data-lk': 'hpick', 'data-id': h.id, 'data-fk': 'hp:' + h.id }));
    ph.appendChild(hl);
    items.appendChild(ph);
    const ps = el('div', 'lcc-k-p5-panel', '', { 'data-cat': 'spec', hidden: ui.cat === 'spec' ? null : '' });
    ps.appendChild(el('p', 'lcc-k-p5-sub', W().specialization));
    const sl = el('div', 'lcc-k-p5-list lcc-k-p5-one', '', { role: 'group', 'aria-label': 'Specializations', 'data-list': 's' });
    sl.appendChild(btn('lcc-k-opt', 'All', { role: 'checkbox', 'aria-checked': state.specs.length ? 'false' : 'true', tabindex: '-1', 'data-lk': 'spick', 'data-val': 'all', 'data-i': '0', 'data-fk': 'sp:all' }));
    SPEC_LIST.forEach((k, i) => sl.appendChild(btn('lcc-k-opt', trackWord(k), { role: 'checkbox', 'aria-checked': state.specs.includes(k) ? 'true' : 'false', tabindex: '-1', 'data-lk': 'spick', 'data-val': k, 'data-i': String(i + 1), 'data-fk': 'sp:' + k })));
    ps.appendChild(sl);
    items.appendChild(ps);
    const pg = el('div', 'lcc-k-p5-panel', '', { 'data-cat': 'group', hidden: ui.cat === 'group' ? null : '' });
    pg.appendChild(el('p', 'lcc-k-p5-sub', 'Group'));
    const gt = el('div', 'lcc-k-tabs', '', { role: 'radiogroup', 'aria-label': 'Group by' });
    for (const [v, t] of [['', 'None'], ['region', 'By region'], ['spec', W().bySpecialization]]) {
      const on = (state.group || '') === v;
      const b = el('button', 'lcc-k-tab lcc-k-fr', t, { type: 'button', role: 'radio', 'aria-checked': on ? 'true' : 'false', tabindex: on ? '0' : '-1', 'data-lk': 'gpick', 'data-val': v, 'data-fk': 'gp:' + (v || 'none') });
      b.appendChild(lamp());
      gt.appendChild(b);
    }
    pg.appendChild(gt);
    items.appendChild(pg);
    body.appendChild(items);
    p.appendChild(body);
    const foot = el('div', 'lcc-k-p5-foot');
    foot.appendChild(btn('lcc-k-link lcc-k-clearf', 'Clear filters', { 'data-lk': 'clearf', 'data-fk': 'mn:clear', hidden: activeFilters(board).length ? null : '' }));
    foot.appendChild(el('span', 'lcc-k-spacer'));
    foot.appendChild(el('span', 'lcc-k-p5-cnt', '', { role: 'status', 'data-lk': 'pcnt' }));
    foot.appendChild(kbtn('lcc-k-pri', 'Done', { 'data-lk': 'popclose', 'data-fk': 'mn:done' }));
    p.appendChild(foot);
    return p;
  }
  // Find House: a single-pick searchable list. It scrolls the board to the House and outlines it for 1.8 s; it changes no filter and ticks nothing.
  function findList(board) {
    const p = el('div', 'lcc-k-pop5 lcc-k-popf lcc-k-fr', '', { id: 'lcc-bd-find', role: 'dialog', 'aria-label': 'Find House', 'data-lk': 'fpop', hidden: ui.find ? null : '' });
    const head = el('div', 'lcc-k-p5-head');
    head.appendChild(el('span', '', 'Find House'));
    head.appendChild(btn('lcc-k-link', 'Done', { 'data-lk': 'fclose', 'data-fk': 'fd:done' }));
    p.appendChild(head);
    p.appendChild(el('input', 'lcc-k-search', '', { type: 'search', 'data-lk': 'fsearch', 'data-fk': 'fd:search', 'aria-label': 'Search Houses', placeholder: 'Search Houses', autocomplete: 'off', value: ui.fq }));
    p.appendChild(el('p', 'lcc-k-p5-n', '', { role: 'status', 'data-lk': 'fn' }));
    const l = el('div', 'lcc-k-p5-list lcc-k-p5-one lcc-k-fl', '', { role: 'listbox', 'aria-label': 'Houses', 'data-list': 'f' });
    for (const h of board.houses.slice().sort((a, b) => String(a.name).localeCompare(String(b.name), 'en'))) l.appendChild(btn('lcc-k-opt lcc-k-opt-find', L().houseTitle(h.name), { role: 'option', tabindex: '-1', 'data-lk': 'fpick', 'data-id': h.id, 'data-fk': 'fp:' + h.id }));
    p.appendChild(l);
    return p;
  }
  function legend(board, ev) {
    ev = ev || M().evaluate(board, null);
    const w = el('ul', 'lcc-bd-headlines', '', { role: 'list', 'aria-label': 'Key to the board' });
    const k = (cls, text, title) => { const li = el('li', '', '', { title }); li.appendChild(el('i', 'lcc-bd-key lcc-bd-key-' + cls, '', { 'aria-hidden': 'true' })); li.appendChild(root.document.createTextNode(text)); return li; };
    w.appendChild(k('a', (W().pledgedTo + ' ' + FACTION_WORD.a).toUpperCase()));
    w.appendChild(k('h', (W().pledgedTo + ' ' + FACTION_WORD.h).toUpperCase()));
    w.appendChild(k('n', W().undecided.toUpperCase(), 'No faction has the favor of this House yet. Wavering: both factions have progress on it.'));
    w.appendChild(k('g', 'WINNING HOUSES · ' + M().WIN.TEXT.won, 'The Houses that make up a win have a gold outline: the five Houses of a line of five held by one faction, or the 13 or more Houses of a faction. ' + M().WIN.TEXT.facts));
    if (board.counts.unknown) w.appendChild(k('u', W().favorUnreadable.toUpperCase(), 'The completed flag and the winner of this House do not agree, so it is counted for neither faction.'));
    return w;
  }
  // what the character has done on this board, in one line under it (the figures the first version of the board kept above it)
  // The line under the board is gone (#1062: it repeated the Inspector's tiles); its sentence is the hover text of the heading "The Landsraad".
  function totals(board) {
    const r = board.rollup;
    return 'Your rewards on this board: ' + ((r.tiers ? r.reached + ' of ' + r.tiers + ' ' + W().tiersEarned : 'tiers not known') + ' · ' + r.collected + ' of ' + r.reached + ' collected' + (r.waiting ? ' (' + r.waiting + ' waiting at the house representatives)' : '') + (r.unknownDelivery ? ' · ' + r.unknownDelivery + ' earned with no delivery record' : '') + ' · ' + fmt(r.mine) + ' contributed in all Houses.');
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
      const g = el('span', 'lcc-hc-spec', '', { title: trackWord(h.track) + ' specialization, ' + levelText(h), 'data-track': h.track });
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
    sel.appendChild(el('small', 'lcc-hc-track' + (h.track ? '' : ' lcc-hc-track-unk'), h.track ? trackWord(h.track) : '—', h.track ? undefined : { title: 'Specialization not in the save' }));
    const pl = M().pledgeOf(h);
    sel.appendChild(el('span', 'lcc-hc-owner', owner === 'a' || owner === 'h' || owner === 'open' ? pl.line : '? ' + pl.line, { 'data-g': ({ a: 'A', h: 'H', open: '○', unknown: '?' })[owner] }));
    const notes = [];
    if (h.waiting > 0) notes.push(h.waiting + ' reward' + (h.waiting === 1 ? '' : 's') + ' not collected');
    sel.setAttribute('aria-label', [(h.number ? W().square + ' ' + h.number + ', ' : '') + h.name + ', ' + pl.line.charAt(0).toLowerCase() + pl.line.slice(1)].concat(h.track ? [W().specialization.toLowerCase() + ' ' + trackWord(h.track)] : ['specialization not in the save'], h.takenNoTiers ? [W().pledged.toLowerCase() + ', no tier record in the save'] : h.tierCount ? [h.reached + ' of ' + h.tierCount + ' ' + W().tiersEarned, h.collected + ' collected'] : ['no tiers in the save'], notes, h.unknownDelivery ? [h.unknownDelivery + ' with no delivery record'] : [], inLine ? ['part of a line of five held by ' + FACTION_WORD[h.owner] + ': ' + listWords(lines.map(lineWord)), winSays] : [], dom ? ['one of the 13 or more Houses of ' + FACTION_WORD[major], winSays] : [], dim ? ['not in the current filter'] : []).join(', '));
    sel.setAttribute('title', 'House ' + h.name + ', ' + pl.house.charAt(0).toLowerCase() + pl.house.slice(1) + '. ' + (h.pos ? h.pos.words + '. ' : '') + (inLine ? lineSays + '. ' + winSays + ' ' : '') + (dom ? 'One of the ' + FACTION_WORD[major] + ' Houses, who hold 13 or more Houses. ' + winSays + ' ' : '') + (takenWhy(h) ? takenWhy(h) + ' ' : '') + (h.track ? trackWord(h.track) + ' specialization, ' + levelText(h) + '. ' : '') + 'Click to see its tiers and actions.');
    card.appendChild(sel);
    const ctl = editable ? ctlOf(host, h.id) : null;
    const tiers = el('div', 'lcc-hc-tiers', '', { role: 'group', 'aria-label': 'House ' + h.name + ' reward tiers' + (ctl && ctl.tiers ? '. ' + CHECKED : '') });
    h.tiers.forEach((t, i) => tiers.appendChild(tierBox(h, t, ctl && ctl.tiers ? ctl.tiers[i] : null)));
    card.appendChild(tiers);
    const count = el('div', 'lcc-hc-count');
    if (h.takenNoTiers) count.appendChild(el('b', '', TAKEN.noTiers, { title: TAKEN.noTiersWhy, 'data-taken': 'no-tiers' }));
    else if (h.tierCount) {
      // the collected and not collected counts are the hover of the figure, not a second line on every card (#1078); the accessible name of the card says them in words
      const said = h.reached > 0 ? ' Collected: ' + h.collected + '. Not collected (waiting at the representative): ' + h.waiting + '.' + (h.unknownDelivery ? ' With no delivery record, so in neither count: ' + h.unknownDelivery + '.' : '') : '';
      count.appendChild(el('b', '', h.reached + '/' + h.tierCount + ' EARNED', { title: 'Reward tiers of yours earned in this House: ' + h.reached + ' of ' + h.tierCount + '. Earned is not collected.' + said }));
    } else count.appendChild(el('b', '', 'NO TIERS', { title: 'No reward tiers are recorded for this House.' }));
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
    { k: 'position', t: W().square, sort: 'position' }, { k: 'house', t: 'House', sort: 'house' }, { k: 'owner', t: W().pledgeBy, sort: 'status' }, { k: 'path', t: W().specialization },
    { k: 'location', t: 'Place', sort: 'location' }, { k: 'state', t: 'Your progress', sort: 'state' }, { k: 'tiers', t: 'Tiers', sort: 'tiers' }, { k: 'next', t: 'Next tier', sort: 'next' }, { k: 'waiting', t: 'Not collected' },
  ];
  function locCell(h, host) {
    const td = el('td', 'lcc-bd-loc');
    // a House with no place is an em dash, not three wrapped words (#1059); the words stay for a screen reader and in the mouse-over
    if (!h.loc.known) { const d = el('span', 'lcc-bd-mut lcc-bd-dash', '—', { title: 'Not mapped yet: the save holds no map marker for this House’s representative' }); d.appendChild(el('span', 'lcc-bd-sr', ' not mapped yet')); td.appendChild(d); return td; }
    const text = [h.loc.area, h.loc.cell ? 'cell ' + h.loc.cell : ''].filter(Boolean).join(' · ');
    const peek = host && typeof host.peek === 'function' ? host.peek(h.id) : null; // the Classic quick peek (the map picture), shown in the mouse-over only
    const wrap = el('span', 'loc');
    if (h.loc.map_key) wrap.appendChild(btn('lcc-bd-linkb', text, { 'data-lb': 'map', 'data-id': h.id, 'data-fk': 'lm:' + h.id, title: 'Open ' + (h.loc.area || h.loc.map) + ' in the Saved map' + (peek ? '. Hover for a quick peek.' : ''), 'aria-label': 'House ' + h.name + ': ' + text + '. Open in the Saved map' }));
    else { const t = el('span', 'lcc-bd-nopic', text, { title: M().noPictureSays(h.loc).replace(/\.$/, '') }); t.appendChild(el('span', 'lcc-bd-sr', ' · no map picture')); wrap.appendChild(t); } // the words "no map picture" are for a screen reader and the mouse-over; they wrapped the cell to three lines (#1059)
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
    const ow = el('td', 'lcc-bd-c-owner'); ow.appendChild(ownerPill(h)); tr.appendChild(ow);
    tr.appendChild(el('td', 'lcc-bd-c-path', trackWord(h.track) || '—'));
    tr.appendChild(locCell(h, host));
    tr.appendChild(el('td', 'lcc-bd-c-state', h.takenNoTiers ? W().pledged + ', none of yours' : ({ new: 'Not started', part: 'In progress', done: 'Top tier' })[h.state], takenWhy(h) ? { title: takenWhy(h) } : tips ? { title: tips.progress } : undefined));
    // "2 of 5" on the page, " tiers earned" for a screen reader (the column heading says Tiers): "2 of 5 tiers earned" wrapped to three lines (#1059)
    const tiersTd = el('td', 'lcc-bd-c-tiers', h.takenNoTiers ? 'no tier record' : h.tierCount ? h.reached + ' of ' + h.tierCount : 'no tiers', tips ? { title: tips.tiers } : undefined);
    if (!h.takenNoTiers && h.tierCount) tiersTd.appendChild(el('span', 'lcc-bd-sr', ' ' + W().tiersEarned));
    tr.appendChild(rtipCell(raw, 'tiers', tiersTd));
    tr.appendChild(rtipCell(raw, 'next', el('td', 'lcc-bd-c-next', h.next ? fmt(h.next.threshold) + ' (' + fmt(h.next.remaining) + ' more)' : h.tierCount ? W().topTierEarned : '—', tips ? { title: tips.next } : undefined)));
    tr.appendChild(rtipCell(raw, 'waiting', el('td', 'lcc-bd-c-wait', h.waiting ? String(h.waiting) : '0', tips ? { title: tips.waiting } : undefined)));
    return tr;
  }
  function list(board, matchIds, host, editable, picks) {
    const wrap = el('div', 'lcc-bd-listwrap');
    const t = el('table', 'lcc-bd-list');
    t.appendChild(el('caption', 'lcc-bd-cap' + (board.ready ? ' lcc-bd-sr' : ''), 'The Houses of the term' + (board.ready ? ', by default by House name, A to Z' : ', by name: the board could not be checked, so no square is given'))); // read out, not drawn, while the board is a board (#1062)
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
        td.textContent = (g.g ? (state.group === 'spec' ? trackWord(g.g) || g.g : g.g) : state.group === 'region' ? 'Place unknown' : W().noSpecialization) + ' · ' + g.rows.length;
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
    if (d.why === 'unreadable') return { state: 'unreadable', faction: '', tag, head: W.tally(d), foot: '', say: say + ' A House that cannot be read is counted for neither faction.' };
    return { state: 'contested', faction: '', tag, head: W.tally(d), foot: '', say: say + ' ' + T.rule };
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
  // Said plainly (#1052): "Lines of five still possible: Atreides 4 · Harkonnen 0". The total, the lines held and a count that cannot be given are not said again: the headline and the plaque say what is known, and a count that
  // cannot be made (a House cannot be read) shows no line at all.
  function linesLeft(ev) {
    const p = ev.potential;
    return p.exact ? 'Lines of five still possible: Atreides ' + p.a + ' · Harkonnen ' + p.h : '';
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
    // the House count is said once, in the headline above (and on the plaque): no second row of the same three figures and no "House count" sentence (#1052). Only a board that is not a board says how many Houses were read.
    if (!board.ready) s.appendChild(el('div', 'lcc-bd-well', 'Houses read: ' + board.houses.length));
    if (v.foot) s.appendChild(el('p', 'lcc-bd-vcount', v.foot));
    if (board.ready && linesLeft(ev)) s.appendChild(el('p', 'lcc-bd-vlines', linesLeft(ev), { 'data-lines': 'exact' }));
    return s;
  }

  // ---- House Intelligence (the Inspector) ------------------------------------------------------------------------------------------------------------------------------------------
  function fcRow(key, label, v, goal) {
    const w = el('div', 'lcc-bd-fc');
    const head = el('div', 'lcc-bd-fc-head');
    head.appendChild(el('span', 'lcc-bd-fc-l lcc-bd-fc-' + key, label));
    // a faction with no contribution row in the save reads 0, like a House with no contribution of yours (#1048); the difference is only in the hover text
    const at = v === null ? 0 : v, gl = goal === null ? '—' : fmt(goal), glSaid = goal === null ? 'not known' : fmt(goal);
    head.appendChild(el('b', 'lcc-bd-fc-v', fmt(at) + ' / ' + gl, Object.assign({ 'aria-label': label + ' ' + fmt(at) + ' of ' + glSaid }, v === null ? { title: 'No record of this faction in the save for this House.' } : {})));
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
  // a section heading is only its words: the labels FROM THE SAVE, SAVE and SAVE + DERIVED are gone (#1057); Help (The Landsraad board) says where the figures come from, once
  function secHead(text, id) {
    return el('h5', 'lcc-bd-sec-h', text, { id });
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
    const tl = el('label', 'lcc-bd-elab', 'Set tier', { for: 'lcc-bd-tierset', title: W().pledged + ' ≠ earned ≠ collected: pledged is a faction, earned is your tier, collected is the reward taken. Help, The Landsraad board, explains the three.' }); // the standing line is a hover text (#1062)
    const ts = el('select', 'lcc-bd-select lcc-bd-eselect', '', { id: 'lcc-bd-tierset', 'data-fk': 'ts:' + h.id });
    for (let n = 0; n <= h.tierCount; n++) { const o = el('option', '', n === 0 ? '0 — none' : n + ' / ' + h.tierCount, { value: String(n) }); if (n === h.reached) o.selected = true; ts.appendChild(o); }
    if (!h.tierCount || c.busy) ts.disabled = true;
    tl.appendChild(root.document.createTextNode(' '));
    tl.appendChild(ts);
    r2.appendChild(tl);
    r2.appendChild(btn('lcc-bd-act', 'Apply tier', { 'data-lcb': 'settier', 'data-id': h.id, 'data-r': h.reached, 'data-fk': 'at:' + h.id, title: 'Sets your contribution to the chosen tier, exactly as a click on that numbered box does: raises to a higher tier, or removes the highest earned tier.' }));
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
    side.appendChild(head);
    if (!h) {
      side.appendChild(el('p', 'lcc-bd-mut', board.ready && state.layout === 'grid' ? 'Choose a House on the board to see its tiers and progress.' : 'Choose a House in the list to see its tiers and progress.'));
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
    else hb.appendChild(el('span', 'lcc-bd-bloc', h.track ? trackWord(h.track) + ' Specialization' : '—', h.track ? undefined : { title: 'Specialization not in the save' }));
    if (h.track) hb.appendChild(el('span', 'lcc-bd-lvl', levelText(h), h.level === null ? { title: LEVEL_WHY } : undefined));
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

    // who it is pledged to: only what the save's own two fields say; a lead on the faction goal is a lead and never a pledge
    const ban = el('div', 'lcc-bd-banner');
    ban.appendChild(el('span', 'lcc-bd-control lcc-bd-control-' + (OWNER_WORD[h.owner] ? h.owner : 'unknown'), (h.owner === 'a' || h.owner === 'h' ? M().pledgeOf(h).house : h.owner === 'open' ? M().pledgeOf(h).word : W().favorUnreadable).toUpperCase()));
    // The approved design has no sentence under the band. What the sentence said stays reachable: as a tooltip and as text for a screen reader. Only a favor the save cannot settle keeps a visible sentence.
    const said = h.owner === 'a' || h.owner === 'h' ? M().pledgeOf(h).house + ' (completed, with a faction the save records).'
      : h.owner === 'open' ? M().pledgeOf(h).word + ': no faction has the favor of this House yet.' + (M().pledgeOf(h).kind === 'wavering' ? ' Both factions have progress on it.' : '') + (h.leader ? ' ' + FACTION_WORD[h.leader] + ' is ahead on the faction goal: a lead, not a pledge.' : '')
        : 'Which faction this House is pledged to cannot be read from the save (the completed flag and the faction do not agree), so this House is counted for neither side.';
    ban.firstChild.setAttribute('title', said);
    const ownerSettled = h.owner === 'a' || h.owner === 'h' || h.owner === 'open';
    if (ownerSettled) ban.appendChild(el('span', 'lcc-bd-sr', said));
    side.appendChild(ban);
    if (!ownerSettled) side.appendChild(el('p', 'lcc-bd-status', said));

    // the two scales, kept apart: the factions' progress against the house goal, then the character's own tiers
    const s1 = el('section', 'lcc-bd-sec', '', { 'aria-labelledby': 'lcc-bd-s1' });
    // the Inspector names no contributors (#1077): the biggest ones of each faction are on the plaque
    const s1note = 'House goal: ' + known(h.goal) + '. A faction’s progress is the sum of its contributions to this House. Missions: ' + NOT_IN_SAVE + '.';
    const s1h = secHead('FACTION HOUSE CONTROL', 'lcc-bd-s1');
    s1h.setAttribute('title', s1note);
    s1.appendChild(s1h);
    s1.appendChild(fcRow('a', 'HOUSE ATREIDES', h.prog.a, h.goal));
    s1.appendChild(fcRow('h', 'HOUSE HARKONNEN', h.prog.h, h.goal));
    s1.appendChild(el('span', 'lcc-bd-sr', s1note));
    side.appendChild(s1);

    const top = h.tierCount ? h.top : null;
    const s2 = el('section', 'lcc-bd-sec', '', { 'aria-labelledby': 'lcc-bd-s2' });
    s2.appendChild(secHead('YOUR CONTRIBUTION', 'lcc-bd-s2'));
    const nums = el('div', 'lcc-bd-pc');
    nums.appendChild(el('strong', '', h.mine > 0 || top !== null ? fmt(h.mine) + (top !== null ? ' / ' + fmt(top) : '') : '0'));
    nums.appendChild(el('small', '', h.next ? fmt(h.next.remaining) + ' to T' + h.next.n : h.tierCount ? W().topPersonalTierEarned.toUpperCase() : '—', h.next || h.tierCount ? undefined : { title: 'No reward tiers are recorded for this House.' }));
    s2.appendChild(nums);
    const mineSaid = 'Your contribution: ' + (h.mine > 0 ? fmt(h.mine) : '0 (none recorded in the save)') + (top !== null ? ' of ' + fmt(top) : '') + (h.next ? ' · next tier ' + fmt(h.next.threshold) + ', ' + fmt(h.next.remaining) + ' more' : h.tierCount ? ' · ' + W().topTierEarned : '') + '.';
    const pt = el('div', 'lcc-bd-track lcc-bd-track-me', '', { role: 'img', 'aria-label': mineSaid, title: mineSaid });
    const pf = el('i', 'lcc-bd-fill lcc-bd-fill-me'); pf.style.setProperty('--lcc-v', pct(h.mine, top) + '%');
    pt.appendChild(pf);
    s2.appendChild(pt);
    side.appendChild(s2);

    // the ledger: the tiers of the character (cumulative), their rewards and what became of them. The rows are the same controls as the tier boxes of the card.
    const s3 = el('section', 'lcc-bd-sec lcc-bd-ledger', '', { 'aria-labelledby': 'lcc-bd-s3' });
    const lh = el('div', 'lcc-bd-ledger-head');
    lh.appendChild(secHead('PERSONAL REWARD LEDGER', 'lcc-bd-s3'));
    // the "N OF 5 EARNED" tag is gone (#1059): the Tiers earned tile below says it. Only a House with no tier record keeps a tag, since it has no tiles.
    if (h.takenNoTiers) lh.appendChild(el('span', 'lcc-bd-ledger-n', W().pledged.toUpperCase() + ' · NO TIER RECORD', { title: takenWhy(h) }));
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
    for (const t of ['TIER', 'REWARD', 'STATE']) heading.appendChild(el('span', '', t));
    s3.appendChild(heading);
    const ol = el('ol', 'lcc-bd-tiers', '', { role: 'list' });
    h.tiers.forEach((t) => {
      const li = el('li', 'lcc-bd-tier-li');
      const ct = c && c.tiers ? c.tiers[t.n - 1] : null;
      const kind = tierKind(t), reward = ct ? ct.reward : '';
      const marked = kind === 'delivered' || kind === 'waiting' || kind === 'not_reached'; // the three states of the ladder are a marker, not words (#1071); a tier with no delivery record keeps its words
      const mk = (tg) => {
        const b = el(tg, 'lcc-bd-tier ' + kind + (marked ? ' mk' : '') + (t.reached ? ' on' : '') + (t.reached && t.state === 'waiting' ? ' wt' : '') + (t.reached && t.state === 'delivered' ? ' dv' : ''));
        const m = el('span', 'lcc-bd-tier-m'); m.appendChild(el('strong', 'lcc-bd-tier-n', 'T' + t.n)); m.appendChild(el('span', 'lcc-bd-tier-th', t.threshold !== null ? fmt(t.threshold) : '—', t.threshold !== null ? undefined : { title: 'Not in the save' }));
        b.appendChild(m);
        const it = el('span', 'lcc-bd-tier-rw'); if (reward) it.appendChild(el('span', '', reward));
        b.appendChild(it);
        if (kind === 'delivered' || kind === 'waiting') b.appendChild(el('span', 'lcc-bd-tier-s mk ' + kind, kind === 'delivered' ? '✓' : '', { 'aria-hidden': 'true' })); // the check in a filled circle, or the empty ring
        else if (!marked) b.appendChild(el('span', 'lcc-bd-tier-s ' + kind, '? ' + TIER_LABEL[kind]));
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
    if (!h.tiers.length) s3.appendChild(el('p', 'lcc-bd-none', 'This House has no reward tiers.'));
    s3.appendChild(ol);
    if (!h.tiersOrdered && h.tiers.length) s3.appendChild(el('p', 'lcc-bd-warn', 'The tier amounts are not in rising order in the save; the earned count is taken as the save states it.'));
    side.appendChild(s3); // no caption, mark legend or note under the ledger (#980): every tier row names its own state, and the Help tab explains the marks, Fill and the three separate states

    // where it is
    const s5 = el('section', 'lcc-bd-sec', '', { 'aria-labelledby': 'lcc-bd-s5' });
    s5.appendChild(secHead('PLACE', 'lcc-bd-s5'));
    if (!h.loc.known) s5.appendChild(el('p', 'lcc-bd-none', 'Not mapped yet: the save holds no map marker for this House’s representative, and the editor does not guess a place.'));
    else {
      s5.appendChild(el('p', 'lcc-bd-line', [h.loc.area, h.loc.cell ? 'grid cell ' + h.loc.cell : '', h.loc.place, h.loc.map && h.loc.map !== h.loc.area ? h.loc.map : ''].filter(Boolean).join(' · ')));
      if (h.loc.unconfirmed) s5.appendChild(el('p', 'lcc-bd-cap', 'This place is from public guides and is not confirmed in the game.'));
      const lm = host && typeof host.loc === 'function' ? host.loc(h.id) : null;
      if (lm && lm.from) s5.appendChild(el('p', 'lcc-bd-line', 'From you: ' + lm.from + '.', lm.fromWhy ? { title: lm.fromWhy } : undefined));
      if (h.loc.map_key) s5.appendChild(btn('lcc-bd-act', 'Open in the Saved map', { 'data-lb': 'map', 'data-id': h.id, 'data-fk': 'lw:' + h.id }));
      else s5.appendChild(el('p', 'lcc-bd-cap', M().noPictureSays(h.loc)));
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
    panel.className += ' lcc-k-page lcc-k-e2 lcc-k-e3 lcc-k-m5 lcc-k-e6'; panel.setAttribute('data-mode', 'filters'); panel.setAttribute('data-state', 'idle');
    panel.appendChild(strip(board, matchIds));
    // directly under the control bar: the question before a write, the result line and the note on a write (the page puts them here, see host.seat)
    panel.appendChild(el('div', 'lcc-bd-slot', '', { 'data-lcc-slot': 'bar' }));
    panel.appendChild(legend(board, ev));
    const stage = el('div', 'lcc-bd-stage');
    if (board.ready && state.layout === 'grid') stage.appendChild(grid(board, matchIds, host, editable, picks, ev));
    else stage.appendChild(list(board, matchIds, host, editable, picks));
    panel.appendChild(stage);
    grid_.appendChild(panel);
    const side = el('div', 'lcc-bd-side');
    side.appendChild(victoryPanel(board, ev));
    side.appendChild(inspector(board, state.sel === null ? null : board.byId.get(state.sel), host, editable));
    grid_.appendChild(side);
    w.appendChild(grid_);
    w.classList.add('lcc-k-page', 'lcc-k-e2', 'lcc-k-e3', 'lcc-k-m5', 'lcc-k-e6'); // the Filter menu and the Find House list are not inside the panel: they take the same kit rules from the board's own box
    w.appendChild(menu(board)); w.appendChild(findList(board)); w.appendChild(el('div', 'lcc-k-scrim', '', { 'data-lk': 'scrim', hidden: '' })); // the Filter menu and the Find House list sit outside the panel (fixed, or a sheet on a phone)
    return w;
  }

  // ---- the control area: what it does after it is drawn (tau #974) -------------------------------------------------------------------------------------------------------------------
  const qsel = (sel) => (last ? last.container.querySelector(sel) : null);
  const panelNode = () => qsel('.lcc-bd-panel.lcc-k-page');
  const picksNow = () => { const h = last && last.host; const s = h && typeof h.picks === 'function' ? h.picks() : null; return s ? [...s] : []; };
  // what is shown now, what is chosen and shown, and what is chosen but not shown by the filters (a House the filters hide is never counted and never written)
  function counts() {
    const board = last.board, shown = new Set(board.houses.filter((h) => M().matches(h, state)).map((h) => h.id));
    const ids = new Set(picksNow()), chosen = [...ids].filter((id) => shown.has(id)).length;
    return { board, shown, chosen, hidden: ids.size - chosen, ids };
  }
  const setText = (root_, sel, t) => { const n = root_.querySelector(sel); if (n && n.textContent !== t) n.textContent = t; };
  // the question before a write is drawn in the strip as a pocket of the bar, with the crest of the faction it is for
  function skinConfirm() {
    const cf = qsel('.lcc-k-cmd #lsb-confirm');
    if (!cf || cf.classList.contains('lcc-k-ask')) return;
    const f = cf.getAttribute('data-f') || '';
    cf.className = 'lcc-k-ask';
    const qc = el('span', 'lcc-k-qcrest', '', { 'data-f': f, 'aria-hidden': 'true' });
    qc.appendChild(el('span', 'lcc-k-crest'));
    cf.insertBefore(qc, cf.firstChild);
    // the question's own words are not changed: its first sentence is the line of the bar, what it writes (the totals, the Houses, the Houses left alone) is the small text under it (two lines, the whole of it in the mouse-over text and in the accessible name)
    const b = cf.querySelector('b'), tx = el('div', 'lcc-k-qtx');
    if (b) {
      const full = b.textContent, i = full.indexOf('? ');
      b.classList.add('lcc-k-qh');
      if (i > 0) { b.textContent = full.slice(0, i + 1); tx.setAttribute('title', full); }
      b.replaceWith(tx); tx.appendChild(b);
      if (i > 0) tx.appendChild(el('p', 'lcc-k-qd', full.slice(i + 2)));
    }
    const bt = el('div', 'lcc-k-qbtn');
    for (const n of [...cf.querySelectorAll('button')]) { n.className = 'lcc-k-btn lcc-k-fr' + (n.getAttribute('data-lb') === 'cyes' ? ' lcc-k-pri' : ''); bt.appendChild(n); }
    cf.appendChild(bt);
  }
  // the one place that says which mode the strip is in and what the counts are; the page calls it (host.syncStrip is LCCB.syncStrip) on every change of the choice
  function syncStrip() {
    if (!last || !last.board) return;
    const panel = panelNode();
    if (!panel) return;
    skinConfirm();
    const c = counts(), asking = !!panel.querySelector('.lcc-k-cmd #lsb-confirm');
    const mode = asking || (c.chosen > 0 && !ui.menu) ? 'action' : 'filters';
    panel.setAttribute('data-mode', mode); panel.setAttribute('data-state', asking ? 'confirm' : c.chosen > 0 ? 'chosen' : 'idle');
    // the filters stay unavailable while Houses are chosen, on every width (on a phone the Show tabs are outside the strip, so the strip's own swap does not hide them)
    for (const t of panel.querySelectorAll('.lcc-k-tab[data-key="show"]')) { if (mode === 'action') { t.setAttribute('aria-disabled', 'true'); t.setAttribute('title', SHOW_OFF); } else { t.removeAttribute('aria-disabled'); t.removeAttribute('title'); } }
    setText(panel, '[data-lk="shown"]', String(c.shown.size)); setText(panel, '[data-lk="shownsr"]', c.shown.size + ' of ' + c.board.houses.length + ' Houses shown');
    const hid = panel.querySelector('[data-lk="hid"]');
    if (hid) { hid.textContent = c.hidden ? c.hidden + ' chosen, not shown' : ''; hid.hidden = !c.hidden; }
    const hid2 = panel.querySelector('[data-lk="hid2"]');
    if (hid2) { hid2.textContent = c.hidden ? c.hidden + ' not shown' : ''; hid2.hidden = !c.hidden; }
    setText(panel, '[data-lk="n"]', String(c.chosen));
    setText(panel, '[data-lk="nsr"]', c.chosen + (c.chosen === 1 ? ' House' : ' Houses') + ' chosen' + (c.hidden ? ' (' + c.hidden + ' not shown)' : ''));
    setText(panel, '[data-lk="chooselabel"]', 'Choose shown (' + c.shown.size + ')');
    const m = last.container.querySelector('#lcc-bd-menu');
    if (m) {
      setText(m, '[data-lk="pcnt"]', c.chosen + ' chosen' + (c.hidden ? ', ' + c.hidden + ' not shown' : ''));
      for (const o of m.querySelectorAll('.lcc-k-opt[data-lk="hpick"]')) {
        const id = Number(o.getAttribute('data-id')), off = !c.shown.has(id);
        o.setAttribute('aria-checked', c.ids.has(id) && !off ? 'true' : 'false');
        if (off) { o.setAttribute('aria-disabled', 'true'); o.setAttribute('title', 'Not shown, so it cannot be chosen'); } else { o.removeAttribute('aria-disabled'); o.removeAttribute('title'); }
      }
      const ph = m.querySelector('.lcc-k-p5-panel[data-cat="house"]'), hl = ph && ph.querySelector('.lcc-k-p5-list');
      if (hl) filterList(hl, ui.q, ph.querySelector('[data-lk="hn"]'), c.chosen ? ', ' + c.chosen + ' chosen' : '');
    }
  }
  function filterList(list, text, nEl, extra) {
    const t = String(text || '').trim().toLowerCase();
    let n = 0, total = 0;
    for (const o of list.querySelectorAll('.lcc-k-opt')) { total++; const hit = !t || o.textContent.toLowerCase().includes(t); o.hidden = !hit; if (hit) n++; }
    if (nEl) nEl.textContent = !t ? total + ' Houses' + (extra || '') : n ? n + ' of ' + total + ' Houses match' : 'No House matches the search.';
    return n;
  }
  const visibleFind = () => { const p = panelNode(); return p ? [...p.querySelectorAll('[data-lk="findbtn"]')].find((b) => b.offsetParent) || null : null; };
  // the popovers are fixed under their key on a desktop width and bottom sheets (a style rule) on a phone
  function placeAll() {
    const panel = panelNode();
    if (!panel) return;
    const pr = panel.getBoundingClientRect();
    const place = (pop, anchor, wide) => {
      if (!pop || pop.hidden || !anchor) return;
      if (phone()) { pop.style.left = ''; pop.style.top = ''; pop.style.width = ''; return; }
      const a = anchor.getBoundingClientRect(), w = Math.min(wide ? 640 : 360, pr.width), top = Math.round(a.bottom + 6);
      const left = Math.max(pr.left, wide ? pr.right - w : Math.min(a.left, pr.right - w));
      pop.style.width = w + 'px'; pop.style.left = left + 'px'; pop.style.top = top + 'px';
      const r = pop.getBoundingClientRect(), dx = r.left - left, dy = r.top - top; // a containing block of its own would shift a fixed box: take that off
      if (Math.abs(dx) > 1 || Math.abs(dy) > 1) { pop.style.left = (left - dx) + 'px'; pop.style.top = (top - dy) + 'px'; }
    };
    place(qsel('#lcc-bd-menu'), qsel('[data-lk="fbtn"]'), true);
    place(qsel('#lcc-bd-find'), visibleFind(), false);
  }
  function layersSync() {
    if (!last) return;
    const m = qsel('#lcc-bd-menu'), f = qsel('#lcc-bd-find'), sc = qsel('[data-lk="scrim"]');
    if (m) { m.hidden = !ui.menu; m.setAttribute('data-view', ui.view); }
    if (f) f.hidden = !ui.find;
    if (sc) sc.hidden = !(phone() && (ui.menu || ui.find));
    for (const b of last.container.querySelectorAll('[data-lk="fbtn"]')) b.setAttribute('aria-expanded', ui.menu ? 'true' : 'false');
    for (const b of last.container.querySelectorAll('[data-lk="findbtn"]')) b.setAttribute('aria-expanded', ui.find ? 'true' : 'false');
  }
  function focusFk(fk) { const n = qsel('[data-fk="' + fk + '"]'); if (n && n.focus) { n.focus({ preventScroll: true }); return true; } return false; }
  function setMenu(open, o) {
    o = o || {};
    if (!last || ui.menu === open) return;
    ui.menu = open;
    if (open) { ui.find = false; ui.view = 'cats'; }
    layersSync(); placeAll(); syncStrip();
    if (open) focusFk('mn:cat:' + ui.cat);
    else if (o.focus) { if (!(counts().chosen > 0 && focusFk('b-fill'))) focusFk('fbtn'); } // Done and Escape: the focus goes to Fill when the strip swapped to the bar, else back to Filter
  }
  function setFind(open, back) {
    if (!last || ui.find === open) return;
    if (open && ui.menu) { ui.menu = false; }
    ui.find = open;
    if (!open) { ui.fq = ''; const i = qsel('[data-lk="fsearch"]'); if (i) i.value = ''; }
    layersSync(); placeAll(); syncStrip();
    const f = qsel('#lcc-bd-find'), l = f && f.querySelector('.lcc-k-p5-list');
    if (open && f) { if (l) filterList(l, ui.fq, f.querySelector('[data-lk="fn"]')); const i = f.querySelector('[data-lk="fsearch"]'); if (i) i.focus({ preventScroll: true }); }
    else if (back) { const k = visibleFind(); if (k) k.focus({ preventScroll: true }); }
  }
  // Find House: scroll to the House and outline it for 1.8 s (reduced motion: no smooth scroll, the outline only). It changes no filter, ticks nothing and chooses no House.
  function findHouse(id) {
    if (!last) return;
    const c = last.container, target = c.querySelector('.lcc-bd-cell[data-id="' + id + '"]') || c.querySelector('tr.lcc-bd-row[data-id="' + id + '"]');
    if (!target) return;
    if (target.scrollIntoView) target.scrollIntoView({ block: 'center', behavior: reducedMotion() ? 'auto' : 'smooth' });
    for (const n of c.querySelectorAll('.lcc-k-flash')) n.classList.remove('lcc-k-flash');
    target.classList.add('lcc-k-flash');
    root.setTimeout(() => target.classList.remove('lcc-k-flash'), 1800);
  }
  function stuckSync() {
    const p = panelNode(), s = p && p.querySelector('.lcc-k-strip');
    if (!s) return;
    const lim = parseFloat(root.getComputedStyle(s).top) || 0;
    p.setAttribute('data-stuck', s.getBoundingClientRect().top <= lim + 0.5 && p.getBoundingClientRect().top < lim - 1 ? '1' : '0');
  }
  // the 25 squares are ONE tab stop (the box that was last used, else the first ticked, else the first), arrows move between the boxes
  function rove() {
    if (!last) return;
    const boxes = [...last.container.querySelectorAll('.lcc-bd-pick')], ok = boxes.filter((b) => !b.disabled);
    if (!ok.length) return;
    const keep = ok.find((b) => b.getAttribute('data-id') === ui.rove) || ok.find((b) => b.checked) || ok[0];
    for (const b of boxes) b.setAttribute('tabindex', b === keep ? '0' : '-1');
  }
  function pickKey(e) {
    const t = e.target;
    if (!t || !t.classList || !t.classList.contains('lcc-bd-pick')) return false;
    if (e.key === 'Enter') { e.preventDefault(); t.click(); return true; }
    const grid = !!last.container.querySelector('.lcc-bd-tiles');
    const d = { ArrowLeft: -1, ArrowRight: 1, ArrowUp: grid ? -M().COLS : -1, ArrowDown: grid ? M().COLS : 1 }[e.key];
    if (!d) return false;
    const boxes = [...last.container.querySelectorAll('.lcc-bd-pick')];
    let i = boxes.indexOf(t) + d;
    while (i >= 0 && i < boxes.length && boxes[i].disabled) i += d;
    e.preventDefault();
    if (i >= 0 && i < boxes.length) { ui.rove = boxes[i].getAttribute('data-id'); boxes[i].focus(); rove(); }
    return true;
  }
  function radioKey(e) {
    const t = e.target;
    if (!t || !t.getAttribute || t.getAttribute('role') !== 'radio') return false;
    const d = { ArrowRight: 1, ArrowDown: 1, ArrowLeft: -1, ArrowUp: -1 }[e.key], g = t.closest('[role="radiogroup"]');
    if (!d || !g) return false;
    const rs = [...g.querySelectorAll('[role="radio"]:not([aria-disabled="true"])')], n = rs[(rs.indexOf(t) + d + rs.length) % rs.length];
    e.preventDefault(); if (n) n.click();
    return true;
  }
  const firstIn = (panel) => panel.querySelector('.lcc-k-tab[tabindex="0"], .lcc-k-search, .lcc-k-opt:not([hidden])');
  function menuKey(e) {
    const t = e.target, m = t && t.closest ? t.closest('#lcc-bd-menu') : null;
    if (!m) return false;
    if (t.classList.contains('lcc-k-cat')) {
      const cats = [...m.querySelectorAll('.lcc-k-cat:not([aria-disabled="true"])')], i = cats.indexOf(t);
      if (e.key === 'ArrowDown' || e.key === 'ArrowUp') { const n = cats[(i + (e.key === 'ArrowDown' ? 1 : cats.length - 1)) % cats.length]; e.preventDefault(); if (n) { n.click(); n.focus(); } return true; }
      if (e.key === 'ArrowRight') { const p = m.querySelector('.lcc-k-p5-panel:not([hidden])'), f = p && firstIn(p); if (f) { e.preventDefault(); f.focus(); } return true; }
      return false;
    }
    if (t.classList.contains('lcc-k-opt')) {
      const list = t.closest('.lcc-k-p5-list'), opts = [...list.querySelectorAll('.lcc-k-opt:not([hidden])')], i = opts.indexOf(t);
      const step = { ArrowDown: 1, ArrowRight: 1, ArrowUp: -1, ArrowLeft: -1 }[e.key];
      if (step) {
        e.preventDefault();
        if (i + step < 0) { const s = m.querySelector('.lcc-k-search'); if (s && list.getAttribute('data-list') === 'h') s.focus(); else { const c = m.querySelector('.lcc-k-cat[aria-selected="true"]'); if (c) c.focus(); } } else if (opts[i + step]) opts[i + step].focus();
        return true;
      }
      if (e.key === 'Home' || e.key === 'End') { e.preventDefault(); (e.key === 'Home' ? opts[0] : opts[opts.length - 1]).focus(); return true; }
      return false;
    }
    if (t.getAttribute('data-lk') === 'hsearch' && e.key === 'ArrowDown') { const f = m.querySelector('.lcc-k-p5-panel[data-cat="house"] .lcc-k-opt:not([hidden])'); if (f) { e.preventDefault(); f.focus(); } return true; }
    return false;
  }
  function ctlKey(e) {
    if (!last) return;
    if (e.key === 'Escape') {
      const t = e.target, inFind = t.closest && t.closest('#lcc-bd-find'), inMenu = t.closest && t.closest('#lcc-bd-menu');
      if (ui.find && (inFind || !ui.menu)) { if (t.getAttribute && t.getAttribute('data-lk') === 'fsearch' && t.value) { t.value = ''; ui.fq = ''; ctlInput(e); } else setFind(false, true); e.preventDefault(); e.stopPropagation(); return; }
      if (ui.menu) { if (t.getAttribute && t.getAttribute('data-lk') === 'hsearch' && t.value) { t.value = ''; ui.q = ''; ctlInput(e); } else setMenu(false, { focus: true }); e.preventDefault(); e.stopPropagation(); return; }
      const mo = qsel('.lcc-k-moremenu:not([hidden])');
      if (mo) { mo.hidden = true; const mb = qsel('[data-lk="more"]'); if (mb) mb.setAttribute('aria-expanded', 'false'); if (mb) mb.focus(); e.preventDefault(); return; }
      if (inMenu || !t.closest || !(t.closest('.lcc-k-strip') || t.closest('.lcc-bd-pick'))) return;
      const no = qsel('.lcc-k-cmd [data-lb="cno"]'); // the question first (the page's own Cancel returns the focus to Fill), then the choice
      if (no) { e.preventDefault(); no.click(); return; }
      const p = panelNode();
      if (p && p.getAttribute('data-mode') === 'action') { const cl = p.querySelector('[data-lb="pickno"]'); if (cl) { e.preventDefault(); cl.click(); } }
      return;
    }
    if (pickKey(e) || radioKey(e) || menuKey(e)) return;
  }
  function ctlInput(e) {
    const t = e.target, k = t && t.getAttribute ? t.getAttribute('data-lk') : '';
    if (k === 'hsearch') { ui.q = t.value; const m = qsel('#lcc-bd-menu'), l = m && m.querySelector('.lcc-k-p5-panel[data-cat="house"] .lcc-k-p5-list'); if (l) filterList(l, ui.q, m.querySelector('[data-lk="hn"]'), counts().chosen ? ', ' + counts().chosen + ' chosen' : ''); }
    if (k === 'fsearch') { ui.fq = t.value; const f = qsel('#lcc-bd-find'), l = f && f.querySelector('.lcc-k-p5-list'); if (l) filterList(l, ui.fq, f.querySelector('[data-lk="fn"]')); }
  }
  // a click on one of this area's own keys; the Show tabs and the pledge row use the board's own filter path (data-lcb="filter") and are not handled here
  function ctlClick(e) {
    const t = e.target && e.target.closest ? e.target.closest('[data-lk]') : null;
    if (!t || !last) return false;
    const k = t.getAttribute('data-lk');
    const reset = (key) => { if (key === 'hs') state.hs = 'any'; else if (key === 'spec') { state.specs = []; state.spec = 'all'; } else if (key === 'group') state.group = ''; };
    if (k === 'fbtn') { e.preventDefault(); setMenu(!ui.menu, { focus: true }); return true; }
    if (k === 'findbtn') { e.preventDefault(); setFind(!ui.find, true); return true; }
    if (k === 'popclose') { e.preventDefault(); setMenu(false, { focus: true }); return true; }
    if (k === 'fclose') { e.preventDefault(); setFind(false, true); return true; }
    if (k === 'cat') {
      e.preventDefault();
      if (t.getAttribute('aria-disabled') === 'true') return true;
      ui.cat = t.getAttribute('data-cat'); if (phone()) ui.view = 'items';
      const m = qsel('#lcc-bd-menu');
      for (const c of m.querySelectorAll('.lcc-k-cat')) { const on = c === t; c.setAttribute('aria-selected', on ? 'true' : 'false'); c.setAttribute('tabindex', on ? '0' : '-1'); }
      for (const p of m.querySelectorAll('.lcc-k-p5-panel')) p.hidden = p.getAttribute('data-cat') !== ui.cat;
      setText(m, '[data-lk="ptitle"]', (t.firstChild ? t.firstChild.textContent : ''));
      layersSync();
      return true;
    }
    if (k === 'back') { e.preventDefault(); ui.view = 'cats'; layersSync(); focusFk('mn:cat:' + ui.cat); return true; }
    if (k === 'hpick') { // ticks the square on the board, the same tick the board gives; no chip, no scroll, nothing is hidden
      e.preventDefault();
      const cb = last.container.querySelector('.lcc-bd-pick[data-id="' + t.getAttribute('data-id') + '"]');
      if (cb && !cb.disabled && t.getAttribute('aria-disabled') !== 'true') { ui.ticking = true; try { cb.click(); } finally { ui.ticking = false; } } // the click of the square must not close the menu it came from
      return true;
    }
    if (k === 'spick') {
      e.preventDefault();
      const v = t.getAttribute('data-val'), set = new Set(state.specs);
      if (v === 'all') set.clear(); else if (set.has(v)) set.delete(v); else set.add(v);
      state.specs = SPEC_LIST.filter((s) => set.has(s)); state.spec = 'all';
      filtered(); redraw();
      return true;
    }
    if (k === 'gpick') { e.preventDefault(); state.group = t.getAttribute('data-val') === 'region' || t.getAttribute('data-val') === 'spec' ? t.getAttribute('data-val') : ''; redraw(); return true; }
    if (k === 'clearf') { e.preventDefault(); reset('hs'); reset('spec'); reset('group'); filtered(); redraw(); focusFk('mn:done'); return true; }
    if (k === 'chipx') { e.preventDefault(); reset(t.getAttribute('data-key')); filtered(); redraw(); focusFk('fbtn'); return true; }
    if (k === 'fpick') { e.preventDefault(); const id = Number(t.getAttribute('data-id')); setFind(false, true); findHouse(id); return true; }
    if (k === 'seat') { e.preventDefault(); if (last.host && last.host.setFac) last.host.setFac(t.getAttribute('data-fid')); return true; }
    if (k === 'more') { e.preventDefault(); const mm = qsel('.lcc-k-moremenu'); if (mm) { mm.hidden = !mm.hidden; t.setAttribute('aria-expanded', mm.hidden ? 'false' : 'true'); } return true; }
    if (k === 'scrim') { setMenu(false, {}); setFind(false, false); return true; }
    return false;
  }
  // a click outside an open menu or list closes it (and the strip swaps when Houses are ticked); Tab out of it does the same
  function outside(e) {
    if (!last || !last.container.isConnected) return;
    const t = e.target;
    if (ui.ticking) return;
    if (ui.menu && !(t.closest && (t.closest('#lcc-bd-menu') || t.closest('[data-lk="fbtn"]')))) setMenu(false, {});
    if (ui.find && !(t.closest && (t.closest('#lcc-bd-find') || t.closest('[data-lk="findbtn"]')))) setFind(false, false);
    const mm = qsel('.lcc-k-moremenu:not([hidden])');
    if (mm && !(t.closest && (t.closest('.lcc-k-moremenu') || t.closest('[data-lk="more"]')))) { mm.hidden = true; const mb = qsel('[data-lk="more"]'); if (mb) mb.setAttribute('aria-expanded', 'false'); }
  }
  function leaving(e) {
    if (!last) return;
    const to = e.relatedTarget;
    if (!to) return;
    if (ui.menu && e.target.closest && e.target.closest('#lcc-bd-menu') && !to.closest('#lcc-bd-menu') && !to.closest('[data-lk="fbtn"]')) setMenu(false, {});
    if (ui.find && e.target.closest && e.target.closest('#lcc-bd-find') && !to.closest('#lcc-bd-find') && !to.closest('[data-lk="findbtn"]')) setFind(false, false);
  }
  // after every drawing of the board (mount): the crests, the mode, the roving box, the popovers and what the menu keeps
  function afterMount() {
    const panel = panelNode();
    if (!panel) return;
    for (const ph of [...last.container.querySelectorAll('.lcc-k-icph')]) ph.replaceWith(icon(ph.getAttribute('data-ic'))); // the icons of the page's keys (the page draws a placeholder; the board draws the picture)
    for (const f of ['a', 'h']) { const a = L().factionAsset ? L().factionAsset(f, 'mark') : null; if (a && a.src) panel.style.setProperty('--lcc-k-crest-' + f, 'url("' + a.src + '")'); }
    // on a phone the Show tabs are not part of the sticky strip (they scroll away, just above it); on a wider window they are the first thing in its filter row
    const tray = panel.querySelector('.lcc-k-lens-tray'), st = panel.querySelector('.lcc-k-strip'), lens = st && st.querySelector('.lcc-k-lens');
    if (tray && st && lens) {
      if (phone() && tray.parentNode === lens) { tray.classList.add('lcc-k-tray-out'); st.parentNode.insertBefore(tray, st); } else if (!phone() && tray.parentNode !== lens) { tray.classList.remove('lcc-k-tray-out'); lens.insertBefore(tray, lens.firstChild); }
    }
    syncStrip(); layersSync(); rove(); placeAll(); stuckSync();
    const f = qsel('#lcc-bd-find'), l = f && f.querySelector('.lcc-k-p5-list');
    if (f && l) filterList(l, ui.fq, f.querySelector('[data-lk="fn"]'));
  }

  // ---- mounting and the view's own events ---------------------------------------------------------------------------------------------------------------------------------------
  function focusKey() {
    const d = root.document, a = d && d.activeElement;
    return a && a.getAttribute ? a.getAttribute('data-fk') || '' : '';
  }
  function mount(container, answer, host) {
    if (!inited) { inited = true; const v = host && host.get ? host.get('layout') : null; if (v === 'grid' || v === 'list') state.layout = v; }
    if (!last || last.container !== container) { ui.menu = false; ui.find = false; ui.fq = ''; } // a new board (a return to the tab) starts with the Filter menu and Find House closed; a redraw of the same board keeps them
    last = { container, answer, host };
    const fk = container.contains(root.document.activeElement) ? focusKey() : '';
    if (host && host.unseat) host.unseat(); // what the page put into the board goes back to its own place before the board is drawn again
    const board = M().readBoard(answer, host && typeof host.specLevels === 'function' ? host.specLevels() : null);
    last.board = board;
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
      container.addEventListener('keydown', (e) => { if (e.key === 'Escape') tipHide(); ctlKey(e); });
      container.addEventListener('input', ctlInput); container.addEventListener('focusout', leaving);
      container.addEventListener('focusin', (e) => { const b = e.target; if (b && b.classList && b.classList.contains('lcc-bd-pick')) { ui.rove = b.getAttribute('data-id'); rove(); } });
      if (!root.__lccBdGlobals) { // the page and the window get these listeners once, not once per container: a tab switch builds a new container, and the handlers read the board that is mounted now (last)
        root.__lccBdGlobals = true;
        root.document.addEventListener('click', outside); root.addEventListener('scroll', () => { stuckSync(); if (ui.menu || ui.find) placeAll(); }, { passive: true }); let wasPhone = phone(); root.addEventListener('resize', () => { if (!last || !last.container.isConnected) return; if (phone() !== wasPhone) { wasPhone = phone(); redraw(); return; } layersSync(); placeAll(); });
      }
      container.addEventListener('click', tipHide, true);
      container.addEventListener('click', onClick); container.addEventListener('change', onChange);
      container.addEventListener('scroll', onScroll, true); container.addEventListener('toggle', onToggle, true); // neither event bubbles
    }
    if (host && host.seat) host.seat(container);
    afterMount();
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
  // a key of the More disclosure closes it; the page's own handler then asks or writes as before
  function closeMoreFor(e) {
    const mi = e.target && e.target.closest ? e.target.closest('.lcc-k-moremenu [data-lb="bulk"]') : null;
    if (mi) { const mm = mi.closest('.lcc-k-moremenu'), mb = qsel('[data-lk="more"]'); mm.hidden = true; if (mb) mb.setAttribute('aria-expanded', 'false'); }
  }
  function onClick(e) {
    closeMoreFor(e);
    if (ctlClick(e)) return; // the keys of the control area (the Filter menu, Find House, the chips, the faction crests)
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
      if (t.getAttribute('aria-disabled') === 'true') { if (last.host && typeof last.host.say === 'function') last.host.say(SHOW_OFF); return; }
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
    if (k === 'fac') { // the board's own faction box takes the choice; when it does not (it is off during a write) the panel's box goes back to the faction that is used, so it never shows one that was not applied (#882)
      const took = last && last.host && last.host.setFac ? last.host.setFac(t.value) : false;
      if (took === false) { const was = t.getAttribute('data-was'); if (was !== null) t.value = was; }
      return;
    }
  }

  // The path of the large House symbol (the 128 px picture of the Command Center art) for the hover card that the page builds (#1069): the page may not reach into the art itself (a test keeps it to LCCM, LCCB, LCCS, LCCT and LCC_HOST),
  // so it asks here. The name is looked up in the art's fixed list of Houses (houseAsset): a name that is not on it, or no art, gives an empty answer and the card has no picture.
  const houseSymbol = (name) => { const a = L() && L().houseAsset ? L().houseAsset(name) : null; return a ? a.src : ''; };
  const api = { state, ui, syncStrip, mount, redraw, keepInView, visible, build, tipOf, majorityOf, victoryWords, plaque, grid, houseCard, list, inspector, victoryPanel, tierKind, NOT_IN_SAVE, houseSymbol };
  if (typeof module !== 'undefined' && module.exports) module.exports = api;
  else root.LCCB = api;
})(typeof window !== 'undefined' ? window : globalThis);
