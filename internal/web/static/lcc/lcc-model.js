// The Command Center's source adapter (milestone M3): turns the answer of the Landsraad read into plain values the board draws, and nothing else.
//
// What this file is: a pure reader. It takes the object the server sent, checks that it is a board it can trust (25 Houses, each place 0 to 24 once, each House once, a term, known
// factions), and gives back one value per House and the totals that come from the same pass, so the header, the board, the list and the inspector cannot disagree. It also holds the
// filters, the sort and the grouping of the list, as functions of those values.
// What it is not: it has no DOM, no network, no storage and no write. A write is made by the page's own code (app.js, through the editor's one apply route); this file only names the Houses that
// code is asked about. It holds no House, number or goal of its own: every figure is read from the answer, a figure that cannot be read is null, and null is shown as "not known".
//
// What it will not claim (decisions D02, D03, D05, D10a of docs/design/landsraad-command-center/DECISIONS.md): the `sysselraad` column of a task is carried and never read as a result;
// a count lead is a lead and never a won vote; a House whose owner cannot be read is neither side's and is counted apart.
(function (root) {
  'use strict';

  const SIZE = 25; // the board has 25 Houses (a 5 by 5 grid, board_index 0 to 24)
  const COLS = 5;
  const TRACKS = Object.freeze(['combat', 'crafting', 'exploration', 'gathering', 'sabotage']);
  const KNOWN_STATES = Object.freeze(['not_reached', 'waiting', 'delivered', 'no_row']);
  const num = (v) => (typeof v === 'number' && Number.isFinite(v) ? v : typeof v === 'string' && v.trim() !== '' && Number.isFinite(Number(v)) ? Number(v) : null);
  const int = (v) => { const n = num(v); return n !== null && Number.isInteger(n) ? n : null; };
  const obj = (v) => (v && typeof v === 'object' && !Array.isArray(v) ? v : null);
  const pad2 = (n) => (n < 10 ? '0' : '') + n;
  const houseName = (raw) => String(raw == null ? '' : raw).replace(/^DA_House/, '');
  const nameKey = (raw) => houseName(raw).trim().toLowerCase();

  // ---- positions ---------------------------------------------------------------------------------------------------------------------------------------------------------------
  // position(index): where a board_index sits on the 5 by 5 board. The number is 01 to 25, row by row from the top left, as the game numbers its tiles.
  function position(index) {
    if (!Number.isInteger(index) || index < 0 || index >= SIZE) return null;
    return { index, number: pad2(index + 1), n: index + 1, row: Math.floor(index / COLS) + 1, col: (index % COLS) + 1, words: 'position ' + (index + 1) + ' of ' + SIZE + ', row ' + (Math.floor(index / COLS) + 1) + ', column ' + ((index % COLS) + 1) };
  }

  // checkTasks(tasks): the reasons a list of tasks is not a board. Empty when it is one. Each reason is a sentence for the page.
  function checkTasks(tasks) {
    const why = [];
    if (!Array.isArray(tasks)) return ['The Landsraad read has no list of Houses.'];
    if (tasks.length !== SIZE) why.push(tasks.length < SIZE ? 'Board incomplete: ' + tasks.length + ' of ' + SIZE + ' Houses are in the read.' : 'Board not usable: ' + tasks.length + ' Houses are in the read, more than the ' + SIZE + ' of one term.');
    const seenIdx = new Map(), seenId = new Map(), seenName = new Map();
    let outside = 0, noId = 0, noName = 0;
    for (const t of tasks) {
      const r = obj(t);
      if (!r) { noId++; continue; }
      const id = int(r.id), bi = int(r.board_index), nk = nameKey(r.house_name);
      if (id === null) noId++; else seenId.set(id, (seenId.get(id) || 0) + 1);
      if (bi === null || bi < 0 || bi >= SIZE) outside++; else seenIdx.set(bi, (seenIdx.get(bi) || 0) + 1);
      if (!nk) noName++; else seenName.set(nk, (seenName.get(nk) || 0) + 1);
    }
    if (noId) why.push(noId + (noId === 1 ? ' House has' : ' Houses have') + ' no readable id.');
    if (outside) why.push(outside + (outside === 1 ? ' House has' : ' Houses have') + ' a board position outside 0 to ' + (SIZE - 1) + ' or none at all.');
    const dupIdx = [...seenIdx].filter(([, n]) => n > 1).map(([k]) => k);
    if (dupIdx.length) why.push('Board position ' + dupIdx.slice(0, 4).join(', ') + (dupIdx.length > 4 ? ' and more' : '') + ' is used by more than one House.');
    if ([...seenId].some(([, n]) => n > 1)) why.push('A House id appears twice.');
    if (noName) why.push(noName + (noName === 1 ? ' House has' : ' Houses have') + ' no name.');
    if ([...seenName].some(([, n]) => n > 1)) why.push('A House name appears twice.');
    return why;
  }

  // positions(tasks): Map(task id -> position) for a list that is a board, an empty Map for one that is not (no number is ever drawn from a board that failed its checks).
  function positions(tasks) {
    const m = new Map();
    if (checkTasks(tasks).length) return m;
    for (const t of tasks) m.set(t.id, position(t.board_index));
    return m;
  }

  // ---- factions ----------------------------------------------------------------------------------------------------------------------------------------------------------------
  // The Landsraad has two factions. 'a' is Atreides and 'h' is Harkonnen: found by the name the save gives, else by the two ids the editor knows (1 and 2). A faction that is neither, or a second
  // one that claims the same side, is left out, and a House that names it is read as unreadable.
  function readFactions(list) {
    const out = { a: null, h: null, byId: new Map(), list: [], problems: [] };
    if (!Array.isArray(list)) { out.problems.push('The factions of the Landsraad could not be read.'); return out; }
    for (const f of list) {
      const r = obj(f);
      const id = r ? int(r.id) : null;
      if (id === null) continue;
      const name = String(r.name == null ? '' : r.name);
      const low = name.toLowerCase();
      const key = low.includes('atreides') ? 'a' : low.includes('harkonnen') ? 'h' : id === 1 ? 'a' : id === 2 ? 'h' : null;
      const entry = { id, name: name || 'Faction ' + id, key };
      out.list.push(entry);
      if (key && !out[key]) { out[key] = entry; out.byId.set(id, key); }
    }
    if (!out.a || !out.h) out.problems.push('The Landsraad read does not name both Atreides and Harkonnen.');
    return out;
  }

  // ---- one House ---------------------------------------------------------------------------------------------------------------------------------------------------------------
  // The owner of a House, read strictly (D10a): completed with a winner the Landsraad has is that faction's; not completed with no winner is open; anything else cannot be read.
  function ownerOf(t, F) {
    const c = t.completed === 1 || t.completed === true ? 1 : t.completed === 0 || t.completed === false ? 0 : null;
    const w = t.winning_faction_id == null ? null : int(t.winning_faction_id);
    if (c === 1 && w !== null && F.byId.has(w)) return F.byId.get(w);
    if (c === 0 && t.winning_faction_id == null) return 'open';
    return 'unknown';
  }

  const TIER_WORD = Object.freeze({ not_reached: 'Not reached', waiting: 'Reached, waiting to collect', delivered: 'Reached, collected', no_row: 'Reached, delivery unknown', unknown: 'Reached, delivery unknown' });

  function readHouse(t, F, me, pos) {
    const goal = num(t.goal_amount);
    const prog = { a: null, h: null }, progOther = [];
    for (const c of Array.isArray(t.contributions) ? t.contributions : []) {
      const r = obj(c);
      if (!r) continue;
      const k = F.byId.get(int(r.faction_id)), a = num(r.amount);
      if (k && a !== null) prog[k] = (prog[k] || 0) + a; else if (a !== null) progOther.push(a);
    }
    const mine = num(t.my_contribution) || 0;
    const tiers = [];
    let ordered = true, prev = -Infinity;
    (Array.isArray(t.rewards) ? t.rewards : []).forEach((x, i) => {
      const r = obj(x) || {};
      const th = num(r.threshold);
      if (th === null || th <= prev) ordered = false;
      if (th !== null) prev = th;
      const reached = th !== null && mine >= th;
      let state = typeof r.reward_state === 'string' && r.reward_state ? r.reward_state : reached ? 'no_row' : 'not_reached';
      if (!KNOWN_STATES.includes(state)) state = 'unknown';
      else if (reached ? state === 'not_reached' : state !== 'not_reached') state = 'unknown'; // the state and the contribution disagree: not trusted
      tiers.push({ n: i + 1, threshold: th, amount: num(r.shown_amount != null ? r.shown_amount : r.amount), template: String(r.shown_template_id || r.template_id || ''), swatch: !!r.swatch, reached, state, word: TIER_WORD[state] });
    });
    const reached = tiers.filter((x) => x.reached).length;
    const collected = tiers.filter((x) => x.reached && x.state === 'delivered').length;
    const waiting = tiers.filter((x) => x.reached && x.state === 'waiting').length;
    const unknownDelivery = tiers.filter((x) => x.reached && (x.state === 'no_row' || x.state === 'unknown')).length;
    const top = tiers.length && tiers[tiers.length - 1].threshold !== null ? tiers[tiers.length - 1].threshold : null;
    const nextT = tiers.find((x) => !x.reached && x.threshold !== null) || null;
    const owner = ownerOf(t, F);
    // who is ahead on the faction goal: one side with more than the other; equal or empty is nobody
    const pa = prog.a || 0, ph = prog.h || 0;
    const leader = pa > ph ? 'a' : ph > pa ? 'h' : null;
    const loc = obj(t.location) || { known: false };
    const line = typeof t.line === 'string' ? t.line.trim().toLowerCase() : '';
    return {
      id: t.id, raw: t, name: houseName(t.house_name), key: nameKey(t.house_name), index: pos ? pos.index : null, pos,
      number: pos ? pos.number : '', owner, leader, taken: !!t.completed || t.winning_faction_id != null,
      goal, prog, progOther, mine, recorded: !!t.my_recorded, canGive: me.factionId !== null,
      tiers, tiersOrdered: ordered, reached, collected, waiting, unknownDelivery, notReached: tiers.length - reached, tierCount: tiers.length, top, next: nextT ? { n: nextT.n, threshold: nextT.threshold, remaining: Math.max(0, nextT.threshold - mine) } : null,
      state: mine <= 0 ? 'new' : tiers.length && reached === tiers.length ? 'done' : 'part',
      track: TRACKS.includes(line) ? line : '', loc, held: !!t.swatch_held, sysselraad: t.sysselraad === undefined ? null : t.sysselraad,
    };
  }

  // ---- the board -----------------------------------------------------------------------------------------------------------------------------------------------------------------
  // readBoard(answer): { ready, problems, term, factions, me, grant, houses, grid, counts, rollup }.
  //   ready    the answer is a complete, valid board: only then is `grid` set and a count or a position claimed; a write from the board needs it
  //   houses   every readable House, by board position when ready and by name when not
  //   grid     25 Houses by board_index (null when not ready)
  //   counts   { a, h, open, unknown, total }: a + h + open + unknown is the number of Houses read, and 25 when ready
  function readBoard(l) {
    const a = obj(l) || {};
    const problems = [];
    const term = obj(a.term);
    const termId = term ? int(term.term_id) : null;
    if (termId === null) problems.push('The Landsraad term could not be read, so no House can be tied to it.');
    problems.push(...checkTasks(a.tasks));
    const F = readFactions(a.factions);
    problems.push(...F.problems);
    const ch = obj(a.character);
    const meId = ch ? int(ch.faction_id) : null;
    const me = { factionId: meId, key: meId !== null && F.byId.has(meId) ? F.byId.get(meId) : null, name: ch && ch.name != null ? String(ch.name) : '' };
    const ready = problems.length === 0;
    const tasks = Array.isArray(a.tasks) ? a.tasks.filter((t) => obj(t) && int(t.id) !== null) : [];
    const houses = tasks.map((t) => readHouse(t, F, me, ready ? position(int(t.board_index)) : null));
    if (ready) houses.sort((x, y) => x.index - y.index); else houses.sort((x, y) => x.name.localeCompare(y.name));
    const grid = ready ? houses.slice() : null;
    const counts = { a: 0, h: 0, open: 0, unknown: 0, total: houses.length };
    for (const h of houses) counts[h.owner]++;
    const rollup = { reached: 0, tiers: 0, collected: 0, waiting: 0, unknownDelivery: 0, notReached: 0, mine: 0 };
    for (const h of houses) { rollup.reached += h.reached; rollup.tiers += h.tierCount; rollup.collected += h.collected; rollup.waiting += h.waiting; rollup.unknownDelivery += h.unknownDelivery; rollup.notReached += h.notReached; rollup.mine += h.mine; }
    const reigning = term && term.reigning_faction_id != null ? F.byId.get(int(term.reigning_faction_id)) || null : null;
    return {
      ready, problems, term: term ? { id: termId, start: term.start_time == null ? null : term.start_time, end: term.end_time == null ? null : term.end_time, reigning, activeDecreeId: term.active_decree_id == null ? null : term.active_decree_id } : null,
      factions: F, me, grant: obj(a.grant), houses, grid, byId: new Map(houses.map((h) => [h.id, h])), counts, rollup,
      // the unreadable owners and the count are said together, so a header never shows a count as complete when a House could not be read
      countComplete: ready && counts.unknown === 0 && counts.a + counts.h + counts.open === SIZE,
    };
  }

  // ---- the twelve lines (milestone M4) ----------------------------------------------------------------------------------------------------------------------------------------
  // The five rows, the five columns and the two diagonals of the fixed 5 by 5 board, by board_index and never by the order of a sorted or filtered list. Frozen: nothing can reorder them.
  const MAJORITY = Math.floor(SIZE / 2) + 1; // 13 of 25: the Houses a faction needs so that the other cannot reach as many
  const DIAGONALS = Object.freeze([['diag-main', 'Main diagonal', Object.freeze([0, 6, 12, 18, 24])], ['diag-anti', 'Opposite diagonal', Object.freeze([4, 8, 12, 16, 20])]]);
  const LINES = (() => {
    const out = [];
    for (let r = 0; r < COLS; r++) out.push(Object.freeze({ id: 'row' + r, kind: 'row', n: r + 1, name: 'Row ' + (r + 1), indexes: Object.freeze([0, 1, 2, 3, 4].map((c) => r * COLS + c)) }));
    for (let c = 0; c < COLS; c++) out.push(Object.freeze({ id: 'col' + c, kind: 'column', n: c + 1, name: 'Column ' + (c + 1), indexes: Object.freeze([0, 1, 2, 3, 4].map((r) => r * COLS + c)) }));
    for (const [id, name, indexes] of DIAGONALS) out.push(Object.freeze({ id, kind: 'diagonal', n: 0, name, indexes }));
    return Object.freeze(out);
  })();

  // lineStatus(owners, indexes): the status of one line for a list of 25 owners ('a', 'h', 'open', anything else is unreadable). One word each:
  //   unknown      a House of the line cannot be read: nothing is inferred about the line
  //   blocked      both factions hold a House of it: neither can complete it without a House being reopened
  //   complete-a / complete-h   five Houses of one faction
  //   one-away-a / one-away-h   four Houses of one faction and one open
  //   open-a / open-h           Houses of one faction only, more than one still open
  //   open-any     nothing claimed on it yet
  function lineStatus(owners, indexes) {
    let a = 0, h = 0, o = 0, u = 0;
    for (const i of indexes) { const v = owners[i]; if (v === 'a') a++; else if (v === 'h') h++; else if (v === 'open') o++; else u++; }
    if (u > 0) return 'unknown';
    if (a > 0 && h > 0) return 'blocked';
    if (a === indexes.length) return 'complete-a';
    if (h === indexes.length) return 'complete-h';
    if (a === indexes.length - 1 && o === 1) return 'one-away-a';
    if (h === indexes.length - 1 && o === 1) return 'one-away-h';
    if (a > 0) return 'open-a';
    if (h > 0) return 'open-h';
    return 'open-any';
  }
  const sideOf = (status) => (/-a$/.test(status) ? 'a' : /-h$/.test(status) ? 'h' : null);
  const OWNER_KEYS = Object.freeze(['a', 'h', 'open']);

  // evaluateOwners(owners, evidence, meta): the whole reading of a board of 25 owners, as separate facts. It is pure: the same owners give the same answer, and nothing is written or drawn.
  //   owners    25 values in board_index order: 'a', 'h', 'open' or anything else (unreadable). A list of another length is not a board.
  //   evidence  null (the save holds no record of the start of the Voting Session: this is what the page always passes) or { verified: true, owners: [25 values of a, h or open] }, the
  //             Houses as they stood at a verified cutoff. A snapshot that is not exactly that is no evidence. The page has no source for it; it exists so the rule is fixed and tested.
  //   meta      { termId, why }: the term the answer belongs to, and the reasons a board is not a board (when it is not)
  // Facts, never a merged yes or no: counts, lines (each with its status, its held Houses and its missing Houses), the lines held by one faction, the lines one House away, the lines still
  // possible, the count leader, whether a faction has 13 or more Houses, the cutoff reading, and a verdict that names which of these decides. A count lead is never a result without a
  // verified cutoff (D05), two factions with a line each is a conflict and nobody is chosen (D04), an unreadable House is neither side's and never an open one (D10a).
  function evaluateOwners(owners, evidence, meta) {
    meta = meta || {};
    const valid = Array.isArray(owners) && owners.length === SIZE;
    const base = {
      valid, termId: meta.termId == null ? null : meta.termId, why: valid ? [] : (Array.isArray(meta.why) && meta.why.length ? meta.why.slice() : ['The board is not 25 Houses that can be read.']), key: '',
      counts: null, countComplete: false, lines: [], held: [], oneAway: [], potential: { total: null, a: null, h: null, exact: false }, blockedLines: null, unknownLines: null,
      leader: null, margin: 0, majority: null, thresholdReached: false, vote: { phase: 'cutoff_unknown', verified: false, counts: null, winner: null, reason: 'The save holds no record of the start of the Voting Session.' },
      verdict: 'unknown', winner: null, conflict: false, conflictReason: '', majorityAlsoRecorded: false,
    };
    if (!valid) { base.lines = LINES.map((l) => ({ ...l, status: 'unknown', side: null, held: false, by: { a: [], h: [], open: [], unknown: [] }, missing: [] })); return base; }
    const own = owners.map((v) => (OWNER_KEYS.includes(v) ? v : 'unknown'));
    const counts = { a: 0, h: 0, open: 0, unknown: 0 };
    for (const v of own) counts[v]++;
    base.counts = counts;
    base.countComplete = counts.unknown === 0;
    base.key = (base.termId == null ? 'no-term' : String(base.termId)) + ':' + own.map((v) => ({ a: 'A', h: 'H', open: 'O', unknown: '?' })[v]).join('');
    base.lines = LINES.map((l) => {
      const by = { a: [], h: [], open: [], unknown: [] };
      for (const i of l.indexes) by[own[i]].push(i);
      const status = lineStatus(own, l.indexes);
      return { ...l, status, side: sideOf(status), held: status === 'complete-a' || status === 'complete-h', by, missing: status.startsWith('one-away') || status.startsWith('open') ? by.open.slice() : [] };
    });
    base.held = base.lines.filter((l) => l.held);
    base.oneAway = base.lines.filter((l) => l.status === 'one-away-a' || l.status === 'one-away-h');
    base.blockedLines = base.lines.filter((l) => l.status === 'blocked').length;
    base.unknownLines = base.lines.filter((l) => l.status === 'unknown').length;
    // the lines a faction can still complete: every line that is neither blocked, nor held, nor unreadable. A line nothing is claimed on counts for both factions, and once in the total.
    const open = base.lines.filter((l) => /^(open|one-away)/.test(l.status));
    base.potential = { total: open.length, a: open.filter((l) => l.side === 'a' || l.side === null).length, h: open.filter((l) => l.side === 'h' || l.side === null).length, exact: counts.unknown === 0 };
    if (counts.a !== counts.h) { base.leader = counts.a > counts.h ? 'a' : 'h'; base.margin = Math.abs(counts.a - counts.h); }
    base.thresholdReached = base.leader !== null && counts[base.leader] >= MAJORITY;
    base.majority = base.thresholdReached ? base.leader : null; // 13 or more Houses: a count that cannot be overtaken on this board, and still not a vote result
    // the cutoff
    const snap = evidence && evidence.verified === true && Array.isArray(evidence.owners) && evidence.owners.length === SIZE && evidence.owners.every((v) => OWNER_KEYS.includes(v)) ? evidence.owners : null;
    if (snap) {
      const k = { a: 0, h: 0, open: 0 };
      for (const v of snap) k[v]++;
      const w = k.a > k.h ? 'a' : k.h > k.a ? 'h' : null;
      base.vote = { phase: 'cutoff_verified', verified: true, counts: k, winner: w, reason: w ? 'More Houses than the other faction at the verified start of the Voting Session.' : 'The factions held the same number of Houses at the verified start of the Voting Session.' };
    }
    // the verdict: which fact decides, and never a faction chosen by order
    const heldBy = { a: base.held.filter((l) => l.side === 'a').length, h: base.held.filter((l) => l.side === 'h').length };
    if (heldBy.a > 0 && heldBy.h > 0) { base.verdict = 'conflict'; base.conflict = true; base.conflictReason = 'Both factions hold a line of five in the save.'; return base; }
    const lineSide = heldBy.a > 0 ? 'a' : heldBy.h > 0 ? 'h' : null;
    if (lineSide) {
      if (base.vote.verified && base.vote.winner && base.vote.winner !== lineSide) { base.verdict = 'conflict'; base.conflict = true; base.conflictReason = 'One faction holds a line of five and the other had more Houses at the verified start of the Voting Session.'; return base; }
      base.verdict = 'line_held'; base.winner = lineSide; base.majorityAlsoRecorded = base.vote.verified && base.vote.winner === lineSide;
      return base;
    }
    if (base.vote.verified) { if (base.vote.winner) { base.verdict = 'majority_at_cutoff'; base.winner = base.vote.winner; } else base.verdict = 'none'; return base; }
    if (counts.unknown > 0) { base.verdict = 'unknown'; return base; }
    base.verdict = base.leader ? 'pending_vote' : base.oneAway.length ? 'possible' : 'none';
    return base;
  }

  // evaluate(board, evidence): the same reading for the answer of readBoard. A board that is not a board (readBoard.ready is false) has no counts, no lines and the verdict unknown, with the
  // reasons readBoard gave. The owners are taken from the 25 Houses by board_index, the order readBoard fixes.
  function evaluate(board, evidence) {
    const b = board || {};
    const termId = b.term && b.term.id != null ? b.term.id : null;
    if (!b.ready || !Array.isArray(b.grid) || b.grid.length !== SIZE) return evaluateOwners(null, null, { termId, why: Array.isArray(b.problems) ? b.problems : [] });
    return evaluateOwners(b.grid.map((h) => h.owner), evidence, { termId });
  }

  // ---- filters, sort, groups -----------------------------------------------------------------------------------------------------------------------------------------------
  const SHOWS = Object.freeze([['all', 'All'], ['left', 'What is left'], ['new', 'Not started'], ['part', 'In progress'], ['done', 'Top tier']]);
  const HOUSE_FILTERS = Object.freeze([['any', 'Any'], ['open', 'Open'], ['taken', 'Claimed']]);
  const showMatch = (h, k) => (k === 'all' ? true : k === 'left' ? h.state !== 'done' : h.state === k);
  const houseMatch = (h, k) => (k === 'any' ? true : (k === 'taken') === h.taken);
  const trackMatch = (h, k) => (k === 'all' || k === '' ? true : h.track === k);
  // matches(h, f): f = { show, hs, spec }. Anything outside the fixed lists matches nothing but "all" / "any", never throws.
  function matches(h, f) {
    f = f || {};
    return showMatch(h, SHOWS.some((s) => s[0] === f.show) ? f.show : 'all') && houseMatch(h, HOUSE_FILTERS.some((s) => s[0] === f.hs) ? f.hs : 'any') && trackMatch(h, f.spec === 'all' || TRACKS.includes(f.spec) ? f.spec : 'all');
  }

  const SORTS = Object.freeze(['house', 'location', 'position', 'state', 'tiers', 'next', 'status']);
  // The key of a sort column, as the editor's list works it out: a House with no place sorts last, the state is Not started < In progress < Top tier, the status puts what is taken by the
  // character's own faction first, then what is taken by the other, then what is open with the character's faction ahead, then open with the other ahead, then open.
  function sortKey(h, col, me) {
    switch (col) {
      case 'location': return h.loc.known && h.loc.area ? String(h.loc.area) : '￿';
      case 'position': return h.index === null ? 99 : h.index;
      case 'state': return { new: 0, part: 1, done: 2 }[h.state];
      case 'tiers': return h.reached;
      case 'next': return h.next ? h.next.remaining : 1e9;
      case 'status': return h.owner === 'a' || h.owner === 'h' ? (h.owner === me ? 0 : 1) : h.leader && h.leader === me ? 2 : h.leader ? 3 : 4;
      default: return h.name;
    }
  }
  const groupOf = (h, g) => (g === 'region' ? (h.loc.known && h.loc.map ? String(h.loc.map) : '') : g === 'spec' ? h.track : '');
  // sortHouses(houses, { sort, dir, group }, meKey): a new array. The tie is always the name, ascending, so the order is stable; the group heading comes first.
  function sortHouses(houses, o, me) {
    const col = SORTS.includes(o && o.sort) ? o.sort : 'house', desc = o && o.dir === 'desc', g = o && (o.group === 'region' || o.group === 'spec') ? o.group : '';
    return houses.slice().sort((x, y) => {
      if (g) { const d = (groupOf(x, g) || '￿').localeCompare(groupOf(y, g) || '￿'); if (d) return d; }
      const a = sortKey(x, col, me), b = sortKey(y, col, me);
      let c = typeof a === 'string' ? a.localeCompare(b) : a - b;
      if (desc) c = -c;
      return c || x.name.localeCompare(y.name);
    });
  }

  const api = { SIZE, COLS, MAJORITY, LINES, TRACKS, KNOWN_STATES, TIER_WORD, SHOWS, HOUSE_FILTERS, SORTS, position, positions, checkTasks, readFactions, readBoard, lineStatus, evaluateOwners, evaluate, matches, sortKey, sortHouses, groupOf };
  if (typeof module !== 'undefined' && module.exports) module.exports = api;
  else root.LCCM = api;
})(typeof window !== 'undefined' ? window : globalThis);
