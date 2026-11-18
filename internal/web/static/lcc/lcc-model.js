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

  const api = { SIZE, COLS, TRACKS, KNOWN_STATES, TIER_WORD, SHOWS, HOUSE_FILTERS, SORTS, position, positions, checkTasks, readFactions, readBoard, matches, sortKey, sortHouses, groupOf };
  if (typeof module !== 'undefined' && module.exports) module.exports = api;
  else root.LCCM = api;
})(typeof window !== 'undefined' ? window : globalThis);
