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
  // A House the save says is taken but that shows nothing of the character's own (#911): said in words, never as "0 of 5 reached". The tiers are the character's own ladder, so with no row of the
  // character none is reached, and with no tier rows at all there is no ladder to show.
  const TAKEN_NO_TIERS = 'Taken - no tier record in the save';
  const TAKEN_NO_MINE = 'Taken - no contribution of yours';
  const TAKEN_NO_TIERS_WHY = 'This House is taken, and the save holds no reward tiers for it.';
  const TAKEN_NO_MINE_WHY = 'This House is taken. The save holds no contribution of yours to it, so none of your five tiers is reached: that is your own ladder, not the faction’s.';
  const takenWhy = (h) => (h.takenNoTiers ? TAKEN_NO_TIERS_WHY : h.takenNoMine ? TAKEN_NO_MINE_WHY : '');
  // the contributors of a list, as the text of one line: the biggest first, "name amount"
  const contribLine = (list, n) => list.slice(0, n).map((c) => c.label + ' ' + fmt(c.amount)).join(' · ');
  // The five states of one reward tier, as the approved design shows them: the class of the box, its mark, and the words. A tier the character has not reached is always "not reached", whatever the
  // reward row says; a reached tier with no delivery record (or a record that disagrees with the contribution) is a question mark and is never counted as collected.
  const TIER_CLASS = Object.freeze({ delivered: 'delivered', waiting: 'waiting', no_row: 'uncertain', unknown: 'uncertain' });
  const TIER_MARK = Object.freeze({ delivered: '✓', waiting: '◆', no_row: '?', unknown: '?' });
  const TIER_LABEL = Object.freeze({ not_reached: 'NOT REACHED', waiting: 'TO COLLECT', delivered: 'COLLECTED', no_row: 'NO DELIVERY RECORD', unknown: 'NO DELIVERY RECORD' });
  const tierKind = (t) => (t.reached ? (TIER_CLASS[t.state] ? t.state : 'unknown') : 'not_reached');

  // The view state. It lives here, not in the answer, so a redraw after a write keeps the filter, the layout and the House that was looked at.
  const state = { layout: 'grid', show: 'all', hs: 'any', spec: 'all', sort: 'position', dir: 'asc', group: '', sel: null, preview: '', strategy: false, line: '', lineSort: 'board' };
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
    // the contributors the save records under each faction, the biggest first (worked out here from the rows of the term: the save holds amounts, not a ranking); none is "Not in the save"
    const cs = board.contributors || { a: [], h: [] };
    const lines = (k) => (cs[k].length ? cs[k].slice(0, 3).map((c) => c.label + ' · ' + fmt(c.amount)) : null);
    const node = L().renderPlaque({ a, h, decree: shownDecree(host), holder: board.term ? board.term.reigning : null, contributors: { a: lines('a'), h: lines('h') }, standing, demo: state.preview !== '' });
    for (const st of node.querySelectorAll ? node.querySelectorAll('.lcc-standard') : []) {
      const k = st.classList.contains('lcc-f-h') ? 'h' : 'a', hd = st.querySelector('.lcc-standard-h');
      if (hd) hd.setAttribute('title', cs[k].length ? 'The biggest contributors under ' + FACTION_WORD[k] + ' in the contribution rows of this term in the save, worked out here: the save holds amounts, not a ranking.' : 'The save holds no contribution rows under ' + FACTION_WORD[k] + ' for the Houses of this term.');
    }
    return node;
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
    w.appendChild(btn('lcc-bd-btn lcc-bd-toterm', 'Term and decrees ↓', { 'data-lcb': 'toterm', 'data-fk': 'bf:toterm', title: "The editor's own controls for the active decree and the reigning faction are in the Term and Decrees section below the board" }));
    const shown = state.preview !== '' ? decs.find((x) => x.id === state.preview) : null;
    w.appendChild(el('span', 'lcc-bd-preview-n', shown ? 'Showing “' + shown.name + '” as a sample · not written to a save' : decs.length ? 'Sample decree only · not written to a save' : 'No decrees are known to this page · nothing to preview', { role: 'status', id: 'lcc-bd-preview-n' }));
    return w;
  }

  // ---- the four figures under the plaque -----------------------------------------------------------------------------------------------------------------------------------
  // TERM STATUS, OPEN HOUSES, POTENTIAL LINES, SPECIALIZATION CAP. The first two are worked out from the same read as the board; the third by the twelve-line check of milestone M4; the fourth is the
  // game's rule for one path (a reference, not a figure of the save).
  // ---- the twelve lines in words (milestone M4) ------------------------------------------------------------------------------------------------------------------------------------
  // Everything the page says about a line comes from M().evaluate (a pure reading of the 25 owners): the words here only name what it found. The wording rule: "five in a line" and "N Houses held",
  // never a win and never a confirmed vote, because the save holds no record of the start of the Voting Session (D03) and the sysselraad column is not evidence (D02).
  const lineWord = (l) => l.name.toLowerCase(); // 'row 3', 'column 2', 'main diagonal', 'opposite diagonal'
  const houseNums = (l) => l.indexes.map((i) => String(i + 1).padStart(2, '0')).join(', ');
  const listWords = (xs) => (xs.length < 2 ? xs.join('') : xs.slice(0, -1).join(', ') + ' and ' + xs[xs.length - 1]);
  const heldWords = (ev, side) => listWords(ev.held.filter((l) => l.side === side).map(lineWord));
  const NOT_VERIFIED = 'not verified: the save does not record the start of the Voting Session';
  // The word for one line of the list: where it stands, for whom, and which House is missing (by name, from the board, never guessed).
  function lineSaid(l, board) {
    const nm = (i) => (board.grid && board.grid[i] ? houseWord(board.grid[i].name) : 'a House');
    const missing = l.missing.map(nm);
    switch (l.status) {
      case 'complete-a': case 'complete-h': return FACTION_WORD[l.side] + ' hold all five Houses: five in a line';
      case 'one-away-a': case 'one-away-h': return FACTION_WORD[l.side] + ' 4 of 5, one House away: ' + missing[0] + ' is still open';
      case 'open-a': case 'open-h': return FACTION_WORD[l.side] + ' ' + l.by[l.side].length + ' of 5, still open: ' + listWords(missing);
      case 'open-any': return 'Nothing claimed on it yet';
      case 'blocked': return 'Blocked: both factions hold a House on it';
      default: return 'Cannot be read: ' + l.by.unknown.length + (l.by.unknown.length === 1 ? ' House' : ' Houses') + ' on it could not be read';
    }
  }

  function metrics(board, ev) {
    ev = ev || M().evaluate(board, null);
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
    // a line of five held now is said as that (the Houses as they stand, not a vote); two factions with a line each is a conflict and neither is chosen
    if (!board.ready) w.appendChild(item('TERM STATUS', 'UNAVAILABLE', 'The board could not be checked', '', { 'data-metric': 'term' }));
    else if (ev.verdict === 'conflict') w.appendChild(item('TERM STATUS', 'CONFLICT', 'Both factions hold a line of five · vote not recorded', '', { 'data-metric': 'term' }));
    else if (ev.verdict === 'line_held') w.appendChild(item('TERM STATUS', 'FIVE IN A LINE', FACTION_WORD[ev.winner] + ' · ' + heldWords(ev, ev.winner) + ' · vote not recorded', '', { 'data-metric': 'term' }));
    else w.appendChild(item('TERM STATUS', 'CONTESTED', 'Vote outcome is not yet established', '', { 'data-metric': 'term' }));
    w.appendChild(board.ready
      ? item('OPEN HOUSES', String(c.open), c.unknown ? 'Available to contest · ' + c.unknown + ' unreadable, not counted' : 'Available to contest', '', { 'data-metric': 'open' })
      : item('OPEN HOUSES', '—', 'Not counted: the board could not be checked', '', { 'data-metric': 'open' }));
    // POTENTIAL LINES: the lines of the twelve that are not blocked and not yet held, so a faction could still complete them (the approved design's own rule). A House that cannot be read
    // puts a line out of reach of the check, so the count is then said to be incomplete and no number is shown: a figure is never guessed.
    if (!board.ready) w.appendChild(item('POTENTIAL LINES', '—', 'Not counted: the board could not be checked', '', { 'data-metric': 'lines' }));
    else if (!ev.potential.exact) w.appendChild(item('POTENTIAL LINES', '—', 'Count incomplete · ' + c.unknown + (c.unknown === 1 ? ' House' : ' Houses') + ' unreadable', '', { 'data-metric': 'lines', 'data-incomplete': 'true' }));
    else w.appendChild(item('POTENTIAL LINES', String(ev.potential.total), 'Unblocked five-House alignments · Atreides ' + ev.potential.a + ' · Harkonnen ' + ev.potential.h + (ev.held.length ? ' · ' + ev.held.length + ' held, not counted' : ''), '', { 'data-metric': 'lines', title: 'Lines of the twelve (five rows, five columns, two diagonals) that no faction has blocked and none holds yet. A line with nothing claimed on it counts for both factions and once in the total.' }));
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
    act.appendChild(btn('lcc-bd-btn lcc-bd-strat', 'Sysselraad Strategy: ' + (state.strategy && gridOk ? 'On' : 'Off'), { 'aria-pressed': state.strategy && gridOk ? 'true' : 'false', 'aria-disabled': gridOk ? null : 'true', 'data-lcb': 'strategy', 'data-fk': 'bf:strategy',
      title: gridOk ? 'Shows the twelve lines of the board as a list: click one to mark its Houses on the board. A preview is a reading of the Houses as they stand, never a result. Nothing is written.' : 'The board cannot be drawn as 5 by 5, so there are no lines to show' }));
    act.appendChild(btn('lcc-bd-btn', 'Clear highlight', { 'data-lcb': 'clearsel', 'data-fk': 'bf:clear', title: 'Stops highlighting the House that is looked at. The outlines of a line of five and of a faction with 13 or more Houses are the Houses as they stand, and stay.' }));
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
  function legend(board, ev) {
    ev = ev || M().evaluate(board, null);
    const w = el('ul', 'lcc-bd-headlines', '', { role: 'list', 'aria-label': 'What the colours of a House mean' });
    const k = (cls, text, title) => { const li = el('li', '', '', { title }); li.appendChild(el('i', 'lcc-bd-key lcc-bd-key-' + cls, '', { 'aria-hidden': 'true' })); li.appendChild(root.document.createTextNode(text)); return li; };
    w.appendChild(k('a', 'ATREIDES CLAIMED'));
    w.appendChild(k('h', 'HARKONNEN CLAIMED'));
    w.appendChild(k('n', 'OPEN'));
    w.appendChild(k('g', 'VICTORY LINE', 'A line of five Houses held by one faction (a row, a column or a diagonal) has a bright gold outline and the words LINE OF FIVE. A faction that holds 13 or more Houses has a gold outline on those Houses (MAJORITY HOUSE): a count of Houses now, ' + NOT_VERIFIED + '.'));
    if (state.strategy && board.ready) w.appendChild(k('p', 'PREVIEW LINE · POTENTIAL', 'A violet dashed outline marks the Houses of the line chosen in the Sysselraad Strategy list when it is not held. It is a preview of a line one faction could still complete, not a result.'));
    if (ev.majority) w.appendChild(k('m', 'MAJORITY HOUSE · COUNT NOT VERIFIED', FACTION_WORD[ev.majority] + ' hold ' + board.counts[ev.majority] + ' Houses now: a count, ' + NOT_VERIFIED + '.'));
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
    const b = btn(cls, mark, { 'data-lb': 'tier', 'data-id': h.id, 'data-t': t.n, 'data-fk': 'ct:' + h.id + ':' + t.n, 'aria-pressed': t.reached ? 'true' : 'false', 'aria-disabled': ct.off ? 'true' : 'false', 'aria-label': say + (ct.off ? ', off' : '. ' + ct.tip), title: 'Tier ' + t.n + ': ' + TIER_LABEL[kind] + '. ' + ct.tip });
    if (ct.off) b.setAttribute('data-why', ct.tip);
    return b;
  }
  // One House card, as the approved design draws it: the number and the specialization badge on top, the crest, HOUSE NAME, the specialization, the owner, the five tier boxes and the line "N/5 REACHED"
  // with the collected and the waiting counts. A House whose specialization the save does not give has no badge and says so in words.
  function houseCard(h, matchIds, host, editable, picks, major, ev) {
    // a House of a line of five held by its faction gets the circuit outline; a House of a faction with 13 or more Houses gets the majority outline unless it is already in a line (the line is the
    // stronger of the two statements). Both are read from the evaluator and are a reading of the Houses as they stand: neither says a vote was held.
    const lines = ev && h.index !== null ? ev.held.filter((l) => l.indexes.includes(h.index)) : [];
    const inLine = lines.length > 0;
    const dom = !inLine && !!major && h.owner === major;
    const dim = !matchIds.has(h.id);
    const owner = OWNER_WORD[h.owner] ? h.owner : 'unknown';
    const route = routeOf(h, ev);
    const card = el('article', 'lcc-hc lcc-hc-' + owner, '', { 'data-owner': owner, 'data-selected': state.sel === h.id ? 'true' : null, 'data-dim': dim ? 'true' : null, 'data-compact': 'true', 'data-lines': inLine ? lines.map((l) => l.id).join(' ') : null, 'data-route': route ? route.kind : null, 'data-route-role': route ? route.role : null });
    if (inLine) L().victory(card, 'circuit'); else if (dom) L().victory(card, 'dominion');
    const lineSays = inLine ? FACTION_WORD[h.owner] + ' hold this House in a line of five: ' + listWords(lines.map(lineWord)) : '';
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
    if (inLine) sel.appendChild(el('span', 'lcc-flag lcc-flag-line lcc-hc-badge', L().VICTORY.circuit));
    else if (dom) sel.appendChild(el('span', 'lcc-flag lcc-flag-dom lcc-hc-badge', L().VICTORY.dominion));
    if (route && !inLine) sel.appendChild(el('span', 'lcc-flag lcc-flag-route lcc-hc-badge', route.badge));
    const notes = [];
    if (h.waiting > 0) notes.push(h.waiting + ' reward' + (h.waiting === 1 ? '' : 's') + ' waiting');
    sel.setAttribute('aria-label', [(h.number ? 'Number ' + h.number + ', ' : '') + h.name + ', ' + (owner === 'a' || owner === 'h' ? 'claimed by ' + OWNER_WORD[owner] : owner === 'open' ? 'open' : 'owner unreadable')].concat(h.track ? ['path ' + trackWord(h.track)] : ['specialization not in the save'], h.takenNoTiers ? ['taken, no tier record in the save'] : h.takenNoMine ? ['taken, no contribution of yours in the save, so none of your tiers is reached'] : h.tierCount ? [h.reached + ' of ' + h.tierCount + ' tiers reached', h.collected + ' collected'] : ['no tiers in the save'], notes, h.unknownDelivery ? [h.unknownDelivery + ' with no delivery record'] : [], inLine ? ['part of a line of five held by ' + FACTION_WORD[h.owner] + ': ' + listWords(lines.map(lineWord))] : [], dom ? ['one of the Houses of the faction that holds more than half'] : [], route ? [route.say] : [], dim ? ['not in the current filter'] : []).join(', '));
    sel.setAttribute('title', 'House ' + h.name + ', ' + (h.pos ? h.pos.words + '. ' : '') + (route ? route.say + '. ' : '') + (inLine ? lineSays + '. The Houses as they stand now, not a vote result. ' : '') + (dom ? 'One of the ' + FACTION_WORD[major] + ' Houses, who hold more than half of the Houses now: a count of Houses, not a vote result, and ' + NOT_VERIFIED + '. ' : '') + (takenWhy(h) ? takenWhy(h) + ' ' : '') + 'Click to see its tiers and actions.');
    card.appendChild(sel);
    const ctl = editable && host && host.ctl ? host.ctl(h.id) : null;
    const tiers = el('div', 'lcc-hc-tiers', '', { role: 'group', 'aria-label': 'House ' + h.name + ' reward tiers' });
    h.tiers.forEach((t, i) => tiers.appendChild(tierBox(h, t, ctl && ctl.tiers ? ctl.tiers[i] : null)));
    card.appendChild(tiers);
    const count = el('div', 'lcc-hc-count');
    if (h.takenNoTiers) count.appendChild(el('b', '', TAKEN_NO_TIERS, { title: TAKEN_NO_TIERS_WHY, 'data-taken': 'no-tiers' }));
    else if (h.takenNoMine) count.appendChild(el('b', '', TAKEN_NO_MINE, { title: TAKEN_NO_MINE_WHY, 'data-taken': 'no-mine' }));
    else if (h.tierCount) {
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
  function tile(h, matchIds, host, editable, picks, major, ev) {
    const li = el('li', 'lcc-bd-cell', '', { 'data-id': h.id, 'data-index': h.index });
    li.appendChild(houseCard(h, matchIds, host, editable, picks, major, ev));
    return li;
  }
  function pickBox(h, picks) {
    const l = el('label', 'lcc-bd-pickl');
    const i = el('input', 'lcc-bd-pick', '', { type: 'checkbox', 'data-lb': 'pick', 'data-id': h.id, 'data-fk': 'pk:' + h.id, 'aria-label': 'Choose House ' + h.name + ' for the actions on several Houses' });
    if (picks && picks.has(h.id)) i.checked = true;
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
    tr.appendChild(el('td', 'lcc-bd-c-state', h.takenNoTiers || h.takenNoMine ? 'Taken, none of yours' : ({ new: 'Not started', part: 'In progress', done: 'Top tier' })[h.state], takenWhy(h) ? { title: takenWhy(h) } : undefined));
    tr.appendChild(el('td', 'lcc-bd-c-tiers', h.takenNoTiers ? 'no tier record' : h.takenNoMine ? 'none of yours' : h.tierCount ? h.reached + ' of ' + h.tierCount : 'no tiers'));
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

  // ---- Sysselraad Strategy (milestone M6) --------------------------------------------------------------------------------------------------------------------------------------------
  // A list of the twelve lines of the board and the way to look at one of them: its Houses are marked on the board, the House that is missing can be jumped to. Everything is read from the evaluator
  // (lcc-model.js) over the Houses as they stand, and nothing is written. A line that is not held is a PREVIEW (a violet dashed outline and the word POTENTIAL): it says what one faction could
  // still complete, never that it will, and never in the words of a result. The board itself is never reordered; only this list can be sorted.
  const LINE_BADGE = Object.freeze({ held: 'FIVE IN A LINE', oneaway: 'ONE AWAY', potential: 'POTENTIAL', blocked: 'BLOCKED', unknown: 'UNREADABLE' });
  function lineKind(l) {
    if (l.held) return 'held';
    if (l.status === 'blocked') return 'blocked';
    if (l.status === 'unknown') return 'unknown';
    return /^one-away/.test(l.status) ? 'oneaway' : 'potential';
  }
  const lineBadge = (l) => LINE_BADGE[lineKind(l)];
  // the order of the list: the board's own order, or the lines closest to done first (held, one away, then the fewest missing, then nothing claimed, blocked and unreadable last); ties keep board order
  function lineOrder(lines, how) {
    const rank = (l) => ({ held: 0, oneaway: 1, blocked: 90, unknown: 91 }[lineKind(l)] ?? (l.status === 'open-any' ? 80 : 10 + l.missing.length));
    return how === 'near' ? lines.map((l, i) => [l, i]).sort((x, y) => rank(x[0]) - rank(y[0]) || x[1] - y[1]).map((x) => x[0]) : lines.slice();
  }
  // what the chosen line says about one House of the board: its outline kind and its word (a House the line misses is MISSING, a claimed House of a line that is not held is POTENTIAL)
  function routeOf(h, ev) {
    if (!state.strategy || !state.line || !ev || !ev.valid || h.index === null || h.index === undefined) return null;
    const l = ev.lines.find((x) => x.id === state.line);
    if (!l || !l.indexes.includes(h.index)) return null;
    const kind = lineKind(l), open = h.owner === 'open';
    if (kind === 'held') return { kind, role: 'held', badge: LINE_BADGE.held, say: 'in the line you are looking at (' + lineWord(l) + '), held by ' + FACTION_WORD[l.side] };
    if (kind === 'blocked') return { kind, role: 'blocked', badge: LINE_BADGE.blocked, say: 'in the line you are looking at (' + lineWord(l) + '), which is blocked' };
    if (kind === 'unknown') return { kind, role: 'unknown', badge: LINE_BADGE.unknown, say: 'in the line you are looking at (' + lineWord(l) + '), which cannot be read' };
    return { kind: 'potential', role: open ? 'missing' : 'claimed', badge: open ? 'MISSING' : 'POTENTIAL', say: open ? 'a missing House of the line you are looking at (' + lineWord(l) + '): a preview, not a result' : 'in the line you are looking at (' + lineWord(l) + '): a preview, not a result' };
  }
  function strategyPanel(board, ev) {
    const s = el('section', 'lcc-bd-surface lcc-bd-strat-panel', '', { 'aria-labelledby': 'lcc-bd-strat-h', 'data-lines': ev.valid ? '12' : '0' });
    s.appendChild(el('p', 'lcc-bd-eyebrow', 'SYSSELRAAD STRATEGY'));
    s.appendChild(el('h3', 'lcc-bd-strat-h', 'Twelve lines', { id: 'lcc-bd-strat-h' }));
    s.appendChild(el('p', 'lcc-bd-strat-lead', 'Five rows, five columns, two diagonals. Choose a line to mark its Houses on the board. A mixed line is blocked. A line one faction does not hold yet is only a preview: the save does not record the start of the Voting Session, and a lead in the House count is not a result.'));
    const sw = el('div', 'lcc-bd-lsort', '', { role: 'group', 'aria-label': 'Order of the list' });
    sw.appendChild(el('span', 'lcc-bd-lbl', 'LIST ORDER'));
    for (const [v, t] of [['board', 'Board order'], ['near', 'Closest first']]) sw.appendChild(btn('lcc-bd-chip', t, { 'data-lcb': 'lsort', 'data-val': v, 'data-fk': 'bf:lsort:' + v, 'aria-pressed': state.lineSort === v ? 'true' : 'false' }));
    s.appendChild(sw);
    s.appendChild(lineFoot(board, ev));
    const ul = el('ul', 'lcc-bd-lrows', '', { role: 'list', 'aria-label': 'The twelve lines of the board' });
    for (const l of lineOrder(ev.lines, state.lineSort)) {
      const li = el('li', 'lcc-bd-lrow-li');
      const kind = lineKind(l), on = state.line === l.id;
      const b = btn('lcc-bd-lrow', '', { 'data-lcb': 'line', 'data-pline': l.id, 'data-kind': kind, 'data-fk': 'bf:line:' + l.id, 'aria-pressed': on ? 'true' : 'false' });
      const top = el('span', 'lcc-bd-lrow-top');
      top.appendChild(el('b', '', l.name + ' · Houses ' + houseNums(l)));
      top.appendChild(el('span', 'lcc-bd-lbadge', LINE_BADGE[kind]));
      b.appendChild(top);
      const pips = el('span', 'lcc-bd-lpips', '', { 'aria-hidden': 'true' }); // the five Houses of the line as they stand; the words below say the same, so the squares are only a picture
      for (const i of l.indexes) { const h = board.grid[i]; pips.appendChild(el('i', '', '', { 'data-owner': h ? h.owner : 'open' })); }
      b.appendChild(pips);
      b.appendChild(el('span', 'lcc-bd-lsay', lineSaid(l, board)));
      li.appendChild(b);
      ul.appendChild(li);
    }
    s.appendChild(ul);
    return s;
  }
  // the line that is chosen, what it is, the way to its missing Houses and the way to let go of it: above the list, so it is in sight where the list is long
  function lineFoot(board, ev) {
    const cur = state.line ? ev.lines.find((l) => l.id === state.line) : null;
    const foot = el('div', 'lcc-bd-lfoot', '', { role: 'status', 'aria-live': 'polite' });
    if (cur) {
      foot.appendChild(el('p', 'lcc-bd-lcur', cur.name + ': ' + lineBadge(cur) + (lineKind(cur) === 'potential' || lineKind(cur) === 'oneaway' ? ' · a preview, not a result' : '')));
      const acts = el('div', 'lcc-bd-lacts');
      for (const i of cur.missing) { const h = board.grid[i]; if (h) acts.appendChild(btn('lcc-bd-btn', 'Jump to missing House ' + L().houseTitle(h.name), { 'data-lcb': 'jumpmissing', 'data-id': h.id, 'data-fk': 'bf:jm:' + h.id })); }
      acts.appendChild(btn('lcc-bd-btn', 'Clear line', { 'data-lcb': 'clearline', 'data-fk': 'bf:clearline' }));
      foot.appendChild(acts);
    } else foot.appendChild(el('p', 'lcc-bd-lcur', 'No line chosen. Escape clears the choice.'));
    return foot;
  }

  // ---- Imperial Victory Analysis -------------------------------------------------------------------------------------------------------------------------------------------------
  // Read-only. What it says is read from the pure evaluator (lcc-model.js) over the 25 Houses as they stand: the counts, the lines of five that one faction holds, and the lines still possible. The save
  // keeps no record of the start of the Voting Session (D03) and the sysselraad column is not evidence (D02), so a count lead is a lead, a House count is "not verified", and no sentence says that a
  // vote was won. A line of five is said as "five in a line" and a count as "N Houses held".
  function victoryWords(board, ev) {
    if (!board.ready) return { state: 'unavailable', faction: '', tag: 'COUNT UNAVAILABLE', head: 'NO COUNT TO SHOW', body: 'The board could not be checked, so no count of Houses is shown and nothing is confirmed.' };
    const c = board.counts;
    const heldSides = ['a', 'h'].filter((f) => ev.held.some((l) => l.side === f));
    const also = ev.majority && heldSides.length === 1 && heldSides[0] === ev.majority ? ' ' + FACTION_WORD[ev.majority] + ' also hold ' + c[ev.majority] + ' Houses: a count, ' + NOT_VERIFIED + '.' : '';
    if (ev.verdict === 'conflict') return { state: 'conflict', faction: '', tag: 'CONFLICT', head: 'BOTH FACTIONS HOLD A LINE', body: FACTION_WORD.a + ' hold ' + heldWords(ev, 'a') + ' and ' + FACTION_WORD.h + ' hold ' + heldWords(ev, 'h') + '. The save does not record which came first or the result of a vote, so neither faction is chosen.' };
    if (ev.verdict === 'line_held') {
      const f = ev.winner, n = ev.held.length;
      return { state: 'line', faction: f, tag: 'FIVE IN A LINE', head: FACTION_WORD[f] + ' · FIVE IN A LINE', body: FACTION_WORD[f] + ' hold five Houses in ' + (n === 1 ? 'one line' : n + ' lines') + ': ' + heldWords(ev, f) + '. Read from the Houses as they stand now; the save does not record the result of a vote.' + also };
    }
    if (ev.majority) return { state: 'count', faction: ev.majority, tag: 'IMPERIAL DOMINION · NOT VERIFIED', head: FACTION_WORD[ev.majority] + ' · ' + c[ev.majority] + ' HOUSES HELD', body: 'A count of Houses now, not a vote result. The save does not record when the Voting Session starts, so a House-count lead cannot be confirmed here.' };
    return { state: 'contested', faction: '', tag: 'AWAITING VOTING SESSION', head: 'SYSSELRAAD CONTESTED', body: 'No confirmed winner. A five-House line secures victory; otherwise compare claimed Houses at the start of the Voting Session.' };
  }
  // The House count set beside the Dominion rule (more Houses than the other faction at the start of the Voting Session): said as a comparison of the Houses now and never as a result.
  function countLine(board, ev) {
    const c = board.counts;
    if (!board.ready) return '';
    if (!board.countComplete) return 'House count incomplete: ' + c.unknown + (c.unknown === 1 ? ' House' : ' Houses') + ' could not be read, so no comparison is made.';
    if (!ev.leader) return 'House count: even at ' + c.a + ' each · ' + NOT_VERIFIED + '.';
    return 'House count: ' + FACTION_WORD[ev.leader] + ' ahead ' + c[ev.leader] + ' to ' + c[ev.leader === 'a' ? 'h' : 'a'] + ' · ' + NOT_VERIFIED + '.';
  }
  function victoryPanel(board, ev) {
    ev = ev || M().evaluate(board, null);
    const v = victoryWords(board, ev);
    const s = el('section', 'lcc-bd-surface lcc-bd-victory', '', { 'aria-label': 'Imperial Victory Analysis', 'aria-live': 'polite', 'data-state': v.state, 'data-faction': v.faction || null });
    s.appendChild(el('p', 'lcc-bd-eyebrow', 'IMPERIAL VICTORY ANALYSIS'));
    s.appendChild(el('div', 'lcc-bd-vtag', v.tag));
    s.appendChild(el('div', 'lcc-bd-emblem', '◇', { 'aria-hidden': 'true' }));
    s.appendChild(el('h3', 'lcc-bd-vhead', v.head));
    s.appendChild(el('p', 'lcc-bd-vbody', v.body));
    const c = board.counts;
    s.appendChild(el('div', 'lcc-bd-well', board.ready ? 'Atreides ' + c.a + ' · Harkonnen ' + c.h + ' · Open ' + c.open + (c.unknown ? ' · Unreadable ' + c.unknown : '') : 'Houses read: ' + board.houses.length));
    if (board.ready) {
      s.appendChild(el('p', 'lcc-bd-vcount', countLine(board, ev)));
      // the twelve lines, one sentence each, in the board's fixed order (rows, columns, the two diagonals); never sorted, so a line is always where it was
      const ul = el('ul', 'lcc-bd-lines', '', { role: 'list', 'aria-label': 'The twelve lines of the board' });
      for (const l of ev.lines) {
        const li = el('li', 'lcc-bd-line', '', { 'data-status': l.status, 'data-line': l.id });
        li.appendChild(el('b', '', l.name + ' · Houses ' + houseNums(l)));
        li.appendChild(el('span', '', lineSaid(l, board)));
        ul.appendChild(li);
      }
      s.appendChild(disc('lines', 'The twelve lines (' + ev.held.length + ' held, ' + (ev.potential.exact ? ev.potential.total + ' still possible' : 'count incomplete') + ')', ul));
    }
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
    if (h.track && host && typeof host.goPath === 'function') hb.appendChild(btn('lcc-bd-btn lcc-bd-gopath', 'Open ' + trackWord(h.track) + ' in Specializations', { 'data-lcb': 'gopath', 'data-track': h.track, 'data-fk': 'gp:' + h.id,
      title: 'Opens the ' + trackWord(h.track) + ' path in the Specialization tab. It only moves between pages: nothing is written.' }));
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
    const s1note = 'House goal: ' + known(h.goal) + '. A faction’s progress is the sum of its contributions to this House. Top contributors: ' + (h.contributors.length ? contribLine(h.contributors, 5) : NOT_IN_SAVE + (h.owner === 'a' || h.owner === 'h' ? ' (the save has no contributor row for this taken House)' : '')) + '. Missions: ' + NOT_IN_SAVE + '.';
    const s1h = secHead('FACTION HOUSE CONTROL', 'lcc-bd-s1', 'SAVE');
    s1h.setAttribute('title', s1note);
    s1.appendChild(s1h);
    s1.appendChild(fcRow('a', 'HOUSE ATREIDES', h.prog.a, h.goal));
    s1.appendChild(fcRow('h', 'HOUSE HARKONNEN', h.prog.h, h.goal));
    s1.appendChild(el('span', 'lcc-bd-sr', s1note));
    if (h.contributors.length) {
      s1.appendChild(el('p', 'lcc-bd-mut lcc-bd-contrib', 'Contributors to this House, from the save, the biggest first: ' + contribLine(h.contributors, 5) + '.', { title: 'The order is worked out here: the save holds the amounts, not a ranking. A guild is shown by the name the save gives it.' }));
      s1.appendChild(el('p', 'lcc-bd-mut lcc-bd-nosave', 'Active Landsraad missions are not recorded in this local save.'));
    } else s1.appendChild(el('p', 'lcc-bd-mut lcc-bd-nosave', 'Top contributors: not in the save. ' + (h.owner === 'a' || h.owner === 'h' ? 'This House is taken but the save holds no contributor row for it (for example when it was completed from this editor). ' : '') + 'Active Landsraad missions are not recorded in this local save.'));
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
    lh.appendChild(el('span', 'lcc-bd-ledger-n', h.takenNoTiers ? 'TAKEN · NO TIER RECORD IN THE SAVE' : h.takenNoMine ? 'TAKEN · NO CONTRIBUTION OF YOURS' : h.reached + ' OF ' + h.tierCount + ' REACHED', takenWhy(h) ? { title: takenWhy(h) } : undefined));
    s3.appendChild(lh);
    const sumSaid = h.reached + ' of ' + h.tierCount + ' reached · ' + h.waiting + ' waiting to collect · ' + h.collected + ' collected · ' + h.unknownDelivery + ' delivery unknown.';
    const sum = el('div', 'lcc-bd-stats', '', { role: 'group', 'aria-label': 'House rewards summary: ' + sumSaid, title: sumSaid });
    const stat = (cls, v, l) => { const d = el('div', 'lcc-bd-stat' + (cls ? ' ' + cls : '')); d.appendChild(el('strong', '', v)); d.appendChild(el('span', '', l)); return d; };
    sum.appendChild(stat('', h.reached + '/' + h.tierCount, 'Tiers reached'));
    sum.appendChild(stat('ok', String(h.collected), 'Collected'));
    sum.appendChild(stat('wait', String(h.waiting), 'To collect'));
    sum.appendChild(stat('rest', String(h.notReached), 'Not reached'));
    if (h.takenNoTiers || h.takenNoMine) s3.appendChild(el('p', 'lcc-bd-mut lcc-bd-taken-note', takenWhy(h)));
    else s3.appendChild(sum);
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
    const ev = M().evaluate(board, null); // no record of the start of the Voting Session exists in the save: the evidence is always null (D03)
    w.appendChild(plaque(board, host));
    w.appendChild(preview(board, host));
    w.appendChild(metrics(board, ev));
    if (!board.houses.length) { w.appendChild(el('p', 'lcc-bd-none', 'This save has no Landsraad Houses in the current term, so there is nothing to show or change here.')); return w; }
    if (state.sel !== null && !board.byId.has(state.sel)) state.sel = null;
    const matchIds = new Set(board.houses.filter((h) => M().matches(h, state)).map((h) => h.id));
    const picks = host && host.picks ? host.picks() : null;
    const grid_ = el('div', 'lcc-bd-grid');
    const panel = el('section', 'lcc-bd-surface lcc-bd-panel', '', { 'aria-labelledby': 'lcc-bd-h' });
    panel.appendChild(panelHead(board));
    panel.appendChild(filters(board, matchIds));
    panel.appendChild(legend(board, ev));
    const stage = el('div', 'lcc-bd-stage');
    if (board.ready && state.layout === 'grid') stage.appendChild(grid(board, matchIds, host, editable, picks, ev));
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
    side.appendChild(victoryPanel(board, ev));
    if (state.strategy && board.ready) side.appendChild(strategyPanel(board, ev));
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
      container.addEventListener('click', onClick); container.addEventListener('change', onChange); container.addEventListener('keydown', onKey);
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
  function onKey(e) {
    if (e.key !== 'Escape' || !state.line || !last) return;
    const w = e.target && e.target.closest ? e.target.closest('.lcc-bd-strat-panel, .lcc-bd-tiles') : null;
    if (!w) return;
    const was = state.line; state.line = ''; redraw();
    focusNode(w.classList.contains('lcc-bd-strat-panel') ? 'bf:line:' + was : 'bf:strategy');
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
    if (k === 'toterm') {
      e.preventDefault();
      const n = root.document.getElementById('lcc-tm-h'), s = root.document.getElementById('lcc-term');
      if (s && s.scrollIntoView) s.scrollIntoView({ block: 'start' });
      if (n && n.focus) n.focus({ preventScroll: true });
      return;
    }
    if (k === 'gopath') { e.preventDefault(); if (last.host && typeof last.host.goPath === 'function') last.host.goPath(t.getAttribute('data-track')); return; }
    if (k === 'strategy') {
      e.preventDefault();
      if (t.getAttribute('aria-disabled') === 'true') return;
      state.strategy = !state.strategy;
      if (!state.strategy) state.line = '';
      redraw();
      focusNode('bf:strategy');
      return;
    }
    if (k === 'line') {
      e.preventDefault();
      const id = t.getAttribute('data-pline');
      state.line = state.line === id ? '' : id;
      redraw();
      return;
    }
    if (k === 'lsort') { e.preventDefault(); state.lineSort = t.getAttribute('data-val') === 'near' ? 'near' : 'board'; redraw(); return; }
    if (k === 'clearline') { e.preventDefault(); const was = state.line; state.line = ''; redraw(); focusNode('bf:line:' + was); return; }
    if (k === 'jumpmissing') {
      e.preventDefault();
      state.sel = Number(t.getAttribute('data-id')); memo.insp = null;
      state.show = 'all'; state.hs = 'any'; state.spec = 'all'; // the same rule as Jump to House: the filters that would hide it are cleared
      redraw();
      const c = last.container.querySelector('[data-fk="ht:' + state.sel + '"]');
      if (c && c.focus) { c.focus(); if (c.scrollIntoView) c.scrollIntoView({ block: 'nearest' }); }
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

  const api = { state, mount, redraw, routeOf, strategyPanel, lineOrder, lineBadge, keepInView, visible, build, majorityOf, victoryWords, countLine, lineSaid, plaque, preview, metrics, grid, houseCard, list, inspector, victoryPanel, knownDecrees, tierKind, NOT_IN_SAVE };
  if (typeof module !== 'undefined' && module.exports) module.exports = api;
  else root.LCCB = api;
})(typeof window !== 'undefined' ? window : globalThis);
