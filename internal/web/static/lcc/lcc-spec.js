// The Command Center's Specializations view (milestone M5): the model of the five tracks read from the save, and the page built from it.
//
// What this file is: a reader and a renderer. readSpecs() turns the answer of the specialization read of the editor (and, for the Houses of each path, the Houses of the Landsraad read) into plain values;
// mount() draws them with createElement and textContent (never a string of HTML), so a name from a save cannot become markup.
// What this file is not: it holds no demo data, it fetches nothing and it writes nothing. Every button it draws carries the data-act of a control the editor already has (spAdd, spMax,
// spReset, spKeyAll, spKeyReset, spResetAll), and the page's own handlers for those send the same requests with the same questions; the one new
// name, spAddCC, only checks the number typed and then runs spAdd. The track-number mapping and the raw-row writer are not drawn here any more (#986): they stay in the classic list. A value the save does not hold is shown as "Not in the save", never made up.
(function (root) {
  'use strict';

  // The XP at which each level starts, level 0 to 100: the table the editor writes levels with (internal/ops/specialization.go, specCumulativeXP). A test reads both and compares them.
  const LEVEL_XP = Object.freeze([
    0, 100, 205, 315, 431, 553, 681, 816, 958, 1107, 1264,
    1428, 1599, 1777, 1963, 2157, 2359, 2570, 2790, 3019, 3258,
    3504, 3757, 4017, 4285, 4561, 4845, 5137, 5438, 5748, 6067,
    6393, 6727, 7069, 7419, 7777, 8143, 8518, 8902, 9295, 9697,
    10107, 10525, 10951, 11385, 11827, 12277, 12736, 13204, 13681, 14167,
    14661, 15163, 15673, 16191, 16717, 17251, 17794, 18346, 18907, 19477,
    20052, 20632, 21217, 21807, 22402, 23002, 23608, 24220, 24838, 25462,
    26086, 26710, 27334, 27958, 28582, 29206, 29830, 30454, 31078, 31702,
    32326, 32950, 33574, 34198, 34822, 35446, 36070, 36694, 37318, 37942,
    38566, 39190, 39814, 40438, 41062, 41686, 42310, 42934, 43558, 44182,
  ]);
  const MAX_LEVEL = LEVEL_XP.length - 1;
  // the lanes are drawn in the design's order
  const ORDER = Object.freeze(['crafting', 'gathering', 'exploration', 'combat', 'sabotage']);
  const LABEL = Object.freeze({ combat: 'Combat', crafting: 'Crafting', exploration: 'Exploration', gathering: 'Gathering', sabotage: 'Sabotage' });
  const NOT_IN_SAVE = 'Not in the save';
  const NONE = '—'; // what a figure the save does not hold shows (#1056): a dash, never the words; the reason is the hover text (NOT_IN_SAVE)
  const SCALE = Object.freeze([0, 25, 50, 75, 100]);

  const fmt = (n) => Number(n).toLocaleString('en-US');
  const isInt = (n) => typeof n === 'number' && Number.isInteger(n);
  const isNum = (n) => typeof n === 'number' && Number.isFinite(n);
  const key = (t) => (typeof t === 'string' ? t.trim().toLowerCase() : '');

  // The level an amount of XP stands on, from the table: the last level whose start is at or below the XP. No linear formula.
  function levelOf(xp) {
    if (!isNum(xp) || xp <= 0) return 0;
    let lo = 0;
    for (let i = 0; i < LEVEL_XP.length; i++) { if (LEVEL_XP[i] <= xp) lo = i; else break; }
    return lo;
  }
  // XP still needed for the next level (0 at the last level)
  const xpToNext = (xp) => { const l = levelOf(xp); return l >= MAX_LEVEL ? 0 : LEVEL_XP[l + 1] - Math.max(0, xp); };

  // What the editor refuses before it sends, said before it is sent: a whole number from 1 to the largest XP of a track. XP is added here and never removed (#1032): a minus sign, 0 and
  // anything that is not a whole number are refused, and so is a number past the largest XP. The Go write path refuses the same amounts (internal/ops specialization.go, AddSpecXP).
  // It judges the number the page will send (the classic list sends +value, so 1e3 and 1.0 are 1000 and 1 there), not the spelling: any decimal spelling of a whole number passes (#899).
  function amountProblem(text, maxXp) {
    const lim = isInt(maxXp) && maxXp > 0 ? maxXp : LEVEL_XP[MAX_LEVEL];
    const msg = 'Type a whole number of XP from 1 to ' + fmt(lim) + '. XP is added here, never removed (use Reset to take it back). Nothing was sent.';
    const s = typeof text === 'string' ? text.trim() : '';
    if (!/^\+?(\d+\.?\d*|\.\d+)(e[+-]?\d+)?$/i.test(s)) return msg;
    const n = Number(s);
    if (!Number.isSafeInteger(n) || n < 1 || n > lim) return msg;
    return '';
  }

  // The model. resp is the answer of the specialization read; houses (optional) is [{ id, name, line }] from the Landsraad read, or null when that read is not there.
  function readSpecs(resp, houses) {
    const out = { ok: false, problem: '', maxXp: 0, levelsTrusted: false, resetCount: 0, rows: [], unassigned: [], housesKnown: Array.isArray(houses), totals: null, anyUnidentified: false };
    if (!resp || typeof resp !== 'object' || !Array.isArray(resp.rows) || !Array.isArray(resp.unassigned)) { out.problem = 'The save gave no specialization tracks this page can read.'; return out; }
    if (!isInt(resp.maxXp) || resp.maxXp <= 0) { out.problem = 'The save gave no largest XP for a track, so the progress cannot be shown.'; return out; }
    out.maxXp = resp.maxXp;
    out.levelsTrusted = resp.maxXp === LEVEL_XP[MAX_LEVEL];
    out.resetCount = isInt(resp.resetCount) && resp.resetCount >= 0 ? resp.resetCount : 0;
    for (const u of resp.unassigned) if (u && isInt(u.number)) out.unassigned.push({ number: u.number, xp: isNum(u.xp) ? u.xp : null, level: isNum(u.level) ? u.level : null });
    const seen = new Set();
    for (const r of resp.rows) {
      if (!r || typeof r.track !== 'string' || seen.has(key(r.track))) continue;
      seen.add(key(r.track));
      const k = key(r.track), known = isInt(r.number);
      const xpOk = known && isNum(r.xp) && r.xp >= 0;
      const xp = xpOk ? r.xp : null;
      const total = isInt(r.keystoneTotal) && r.keystoneTotal > 0 ? r.keystoneTotal : 0;
      const owned = isInt(r.keystoneOwned) && r.keystoneOwned >= 0 ? r.keystoneOwned : 0;
      const capped = xp !== null && xp >= out.maxXp;
      const derived = xp !== null && out.levelsTrusted ? levelOf(xp) : null;
      const stored = known && isNum(r.level) ? r.level : null;
      const level = derived !== null ? derived : stored === null ? null : Math.floor(stored);
      const mismatch = derived !== null && stored !== null && Math.floor(stored) !== derived;
      const keyComplete = total > 0 && owned >= total;
      const own = Array.isArray(houses) ? houses.filter((h) => h && key(h.line) === k).sort((a, b) => String(a.name).localeCompare(String(b.name))) : null;
      out.rows.push({
        track: r.track, key: k, label: LABEL[k] || r.track, known, number: known ? r.number : null, xp, level, storedLevel: stored, levelMismatch: mismatch,
        capped, toNext: derived !== null && !capped ? xpToNext(xp) : null, toCap: xp !== null ? Math.max(0, out.maxXp - xp) : null,
        keyOwned: owned, keyTotal: total, keyKnown: total > 0, keyComplete, full: capped && keyComplete,
        kind: !known ? 'unresolved' : capped && keyComplete ? 'full' : capped ? 'cap' : keyComplete ? 'keys' : 'progress',
        houses: own,
      });
      if (!known) out.anyUnidentified = true;
    }
    if (out.unassigned.length) out.anyUnidentified = true;
    out.rows.sort((a, b) => { const ia = ORDER.indexOf(a.key), ib = ORDER.indexOf(b.key); return (ia < 0 ? 99 : ia) - (ib < 0 ? 99 : ib); });
    const known = out.rows.filter((r) => r.known);
    out.totals = {
      tracks: out.rows.length, atCap: known.filter((r) => r.capped).length,
      owned: out.rows.reduce((s, r) => s + r.keyOwned, 0), total: out.rows.reduce((s, r) => s + r.keyTotal, 0),
      keysPartial: out.rows.some((r) => !r.keyKnown) || out.rows.some((r) => !r.known), // a track whose number is not known is counted by its keystones only: the total is partial then too
      keysWhy: out.rows.some((r) => !r.keyKnown) ? 'total' : out.rows.some((r) => !r.known) ? 'unidentified' : '',
    };
    out.ok = out.rows.length > 0;
    if (!out.ok) out.problem = 'This save has no specialization tracks.';
    return out;
  }

  // The words of a track's state, kept apart so XP and keystones are never one word: LEVEL 100 is the XP cap and says nothing about keystones, "full path" needs both numbers.
  function masteryText(r) {
    if (!r.known) return 'TRACK NOT IDENTIFIED';
    if (r.kind === 'full') return 'FULL PATH COMPLETE'; // the level (the plate above), the XP and the keystone count (the bars below) are not said again here (#1053)
    if (r.kind === 'cap') return 'XP CAP REACHED';
    if (r.kind === 'keys') return 'KEYSTONES COMPLETE';
    return 'XP CAP NOT REACHED';
  }
  const laneBadge = (r) => (!r.known ? 'TRACK NOT IDENTIFIED' : r.kind === 'full' ? '✦ FULL PATH COMPLETE' : r.kind === 'cap' ? '✦ XP CAP REACHED' : r.kind === 'keys' ? 'KEYSTONES COMPLETE' : 'PATH IN PROGRESS');
  function xpRemainText(vm, r) {
    if (r.xp === null) return NONE;
    if (r.capped) return 'XP CAP REACHED — NO FURTHER XP LEVELS';
    if (r.toNext === null) return fmt(r.toCap) + ' XP TO CAP';
    return fmt(r.toNext) + ' XP TO LEVEL ' + (r.level + 1) + ' · ' + fmt(r.toCap) + ' XP TO CAP';
  }
  const keysText = (r) => (r.keyKnown ? r.keyOwned + ' / ' + r.keyTotal : NONE);
  const pctOf = (n, d) => (isNum(n) && d > 0 ? Math.max(0, Math.min(100, (n / d) * 100)) : 0);
  const storedText = (r) => String(Math.round(r.storedLevel * 100) / 100);
  const laneLabel = (vm, r) => r.label + ', ' + (r.known ? 'level ' + (r.level === null ? 'unknown' : r.level) + ' of 100, ' + fmt(r.xp) + ' XP, ' + (r.keyKnown ? r.keyOwned + ' of ' + r.keyTotal + ' keystones' : 'keystones not known') + (r.levelMismatch ? ', the save stores level ' + storedText(r) : '') + (r.capped ? ', XP cap reached' : '') + (r.keyComplete ? ', keystones complete' : '') : 'track not identified, no XP shown');

  // ---- the page ----------------------------------------------------------------------------------------------------------------------------------------------------------------
  // sel is the chosen path; xp holds what was typed in the XP field of each path, so choosing another lane and coming back, or drawing the view again, does not put 1000 back (#899)
  const state = { sel: 'crafting', xp: {} };
  const doc = () => root.document;
  function el(tag, cls, text, attrs) {
    const e = doc().createElement(tag);
    if (cls) e.className = cls;
    if (text != null && text !== '') e.textContent = String(text);
    if (attrs) for (const k of Object.keys(attrs)) if (attrs[k] != null && attrs[k] !== false) e.setAttribute(k, attrs[k] === true ? '' : String(attrs[k]));
    return e;
  }
  const add = (p, ...kids) => { for (const k of kids) if (k) p.appendChild(k); return p; };
  const LC = () => root.LCC || null;
  function pic(cls, track, size) {
    const a = LC() && LC().emblemAsset ? LC().emblemAsset(track) : null; // the clean path emblem, no scroll and no frame (#989)
    if (!a) return null;
    const img = el('img', cls, '', { alt: '', width: size, height: size, decoding: 'async', src: a.src });
    img.addEventListener('error', () => { img.hidden = true; });
    return img;
  }
  function houseCrest(name) {
    const a = LC() && LC().houseAsset ? LC().houseAsset(name) : null;
    if (!a) return el('span', 'lcc-sp-hc-none');
    const img = el('img', 'lcc-sp-hc-img', '', { alt: '', width: 37, height: 37, decoding: 'async', src: a.src });
    img.addEventListener('error', () => { img.hidden = true; });
    return img;
  }
  const bar = (cls, pct, label, now, max) => {
    const b = el('div', cls, '', { role: 'progressbar', 'aria-label': label, 'aria-valuemin': 0, 'aria-valuemax': max, 'aria-valuenow': now });
    const i = el('span', cls + '-fill');
    i.style.setProperty('--lcc-v', pct.toFixed(2) + '%'); // a run-time value goes through the style object, never a style attribute in markup (the content security policy)
    return add(b, i);
  };
  const btn = (text, cls, attrs) => el('button', 'lcc-sp-btn' + (cls ? ' ' + cls : ''), text, Object.assign({ type: 'button' }, attrs));

  function heroOf(vm) {
    const h = el('header', 'lcc-sp-hero', '', { 'aria-label': 'Specializations overview' });
    add(h, el('div', 'lcc-sp-hero-art', '', { 'aria-hidden': 'true' }), null);
    h.firstChild.appendChild(el('span', '', '✥'));
    const t = el('div', 'lcc-sp-hero-title');
    add(t, el('p', 'lcc-sp-eyebrow', 'THE FIVE PATHS OF ARRAKIS · IMPERIAL ARCHIVE'));
    const h2 = el('h2', 'lcc-sp-h2', 'SPECIALIZATION ', { id: 'lcs-title' });
    h2.appendChild(el('em', '', 'COMMAND'));
    add(t, h2); // no lead sentence (#980): what the view is and how its operations work are in the Help tab
    const s = el('div', 'lcc-sp-hero-stats', '', { role: 'group', 'aria-label': 'Progression overview' });
    const stat = (v, k, id, title) => add(el('div', 'lcc-sp-stat'), el('b', '', v, { id, title: v === NONE ? NOT_IN_SAVE : null }), el('small', '', k, title ? { title } : undefined));
    const T = vm.totals;
    add(s, stat(T.tracks ? T.atCap + ' / ' + T.tracks : NONE, 'SPECIALIZATIONS COMPLETED', 'lcs-capcount', 'Paths whose XP has reached the cap (level 100). Keystones are counted apart.'), // renamed from TRACKS AT XP CAP (#1042): the count is the same
      stat(T.total > 0 ? T.owned + ' / ' + T.total + (T.keysPartial ? '*' : '') : NONE, T.keysPartial && T.total > 0 ? 'KEYSTONES (* ' + (T.keysWhy === 'total' ? 'A TRACK’S TOTAL IS NOT IN THE SAVE' : 'A TRACK IS NOT IDENTIFIED YET') + ')' : 'KEYSTONES', 'lcs-keycount')); // two totals: the third, the largest XP of a track, is dropped (#1042); the XP ceiling is still on the dossier
    return add(h, t, s);
  }

  function laneOf(vm, r) {
    const sel = r.key === state.sel;
    const b = el('button', 'lcc-sp-lane', '', { type: 'button', 'aria-pressed': sel ? 'true' : 'false', 'data-lcs-lane': r.key, 'data-track': r.key, 'data-kind': r.kind, 'data-mismatch': r.levelMismatch ? 'true' : null, 'data-victory': r.kind === 'cap' || r.kind === 'full' ? 'win' : null, 'aria-label': laneLabel(vm, r) }); // level 100 wears the one win outline of lcc.css (#988)
    const ic = el('span', 'lcc-sp-lane-ic', '', { 'aria-hidden': 'true' });
    add(ic, pic('lcc-sp-lane-img', r.key, 72));
    const lv = el('span', 'lcc-sp-lane-lv', '', { 'aria-hidden': 'true' });
    add(lv, el('b', '', r.known && r.level !== null ? 'LV ' + r.level : 'LV —'), el('span', '', ' / 100'));
    // the level of the lane is the one of its XP; when the save stores another level the lane says so, not only the dossier of the chosen path (#899)
    const mm = r.levelMismatch ? el('span', 'lcc-sp-lane-mm', '⚠ SAVE STORES LV ' + storedText(r), { 'aria-hidden': 'true' }) : null;
    add(b, ic, el('strong', 'lcc-sp-lane-name', r.label.toUpperCase(), { 'aria-hidden': 'true' }), lv,
      bar('lcc-sp-lane-bar', pctOf(r.xp, vm.maxXp), 'XP of this path toward the cap', r.xp === null ? 0 : r.xp, vm.maxXp),
      el('span', 'lcc-sp-lane-nums', (r.xp === null ? NONE + ' XP' : fmt(r.xp) + ' XP') + ' · ' + (r.keyKnown ? r.keyOwned + '/' + r.keyTotal + ' keystones' : NONE + ' keystones'), { 'aria-hidden': 'true' }),
      el('span', 'lcc-sp-lane-badge', laneBadge(r), { 'aria-hidden': 'true' }), mm);
    b.querySelector('.lcc-sp-lane-bar').setAttribute('aria-hidden', 'true');
    return b;
  }

  function dossierOf(vm, r) {
    const d = el('section', 'lcc-sp-dossier lcc-sp-surface', '', { 'aria-labelledby': 'lcs-dos-h', 'data-kind': r.kind });
    const head = el('div', 'lcc-sp-dtitle');
    const ic = add(el('div', 'lcc-sp-dossier-ic', '', { 'aria-hidden': 'true' }), pic('lcc-sp-dossier-img', r.key, 96));
    const hd = el('div', 'lcc-sp-dheading');
    add(hd, el('p', 'lcc-sp-eyebrow', 'PROGRESSION DOSSIER · SELECTED PATH'), el('h2', 'lcc-sp-h2s', r.label.toUpperCase(), { id: 'lcs-dos-h' }),
      r.known ? null : el('p', 'lcc-sp-small', 'This save has XP in a track number that is not tied to this path yet.')); // a path is picked by its name: the number behind a known path is not shown (#708)
    add(head, ic, hd, el('span', 'lcc-sp-flag' + (r.capped ? ' is-cap' : ''), !r.known ? 'TRACK NOT IDENTIFIED' : r.capped ? 'LEVEL 100 · XP CAP REACHED' : 'LEVEL ' + (r.level === null ? '—' : r.level)));
    add(d, head);

    if (!r.known) {
      add(d, el('p', 'lcc-sp-note lcc-sp-note-warn', 'XP editing is off for this path: no track number of the save is tied to it. The keystone buttons still work: keystones need no track numbers.', { id: 'lcs-unres-note' }));
    }
    const g = el('div', 'lcc-sp-prog');
    const plate = el('div', 'lcc-sp-plate');
    const lvl = el('div', 'lcc-sp-level', r.level === null || !r.known ? '—' : String(r.level));
    if (r.known && r.level !== null) lvl.appendChild(el('span', '', '/100', { 'aria-hidden': 'true' }));
    add(plate, el('span', 'lcc-sp-k', 'SPECIALIZATION LEVEL'), lvl, el('div', 'lcc-sp-mastery' + (r.kind === 'full' || r.kind === 'cap' ? ' is-on' : ''), masteryText(r), { id: 'lcs-mastery' }));
    const st = el('div', 'lcc-sp-status');
    const line = (k, v, id) => add(el('div', 'lcc-sp-line'), el('span', '', k), el('strong', '', v, { id, title: v === NONE ? NOT_IN_SAVE : null }));
    add(st, line('XP', r.xp === null ? NONE : fmt(r.xp) + ' / ' + fmt(vm.maxXp), 'lcs-xp'),
      bar('lcc-sp-xpbar', pctOf(r.xp, vm.maxXp), 'Experience of the selected path', r.xp === null ? 0 : r.xp, vm.maxXp),
      el('div', 'lcc-sp-sub', xpRemainText(vm, r), { id: 'lcs-xpremain' }));
    if (r.levelMismatch) add(st, el('p', 'lcc-sp-note lcc-sp-note-warn', 'The save stores level ' + (Math.round(r.storedLevel * 100) / 100) + ' for this track, but its XP is level ' + r.level + ' on the XP table. Nothing was changed; the level the save stores is left as it is.', { id: 'lcs-mismatch' }));
    if (!vm.levelsTrusted) add(st, el('p', 'lcc-sp-note lcc-sp-note-warn', 'The save’s largest XP (' + fmt(vm.maxXp) + ') is not the one this page’s level table ends on, so the level shown is the one the save stores.', { id: 'lcs-table-note' }));
    add(st, line('KEYSTONES', keysText(r), 'lcs-keys'),
      bar('lcc-sp-keybar', r.keyKnown ? pctOf(r.keyOwned, r.keyTotal) : 0, 'Keystones purchased on the selected path', r.keyOwned, r.keyKnown ? r.keyTotal : 0)); // "XP cap and keystones are independent" is in the Help tab (#980)
    add(g, plate, st);
    add(d, g);

    const wp = el('div', 'lcc-sp-scale');
    add(wp, add(el('div', 'lcc-sp-scale-h'), el('span', 'lcc-sp-k', 'LEVEL SCALE', { title: 'Reference marks along the 100 levels. They are not unlock levels.' })));
    const row = el('ol', 'lcc-sp-marks', '', { 'aria-label': 'Reference marks along the 100 levels' });
    for (const n of SCALE) {
      const li = el('li', 'lcc-sp-mark' + (r.level !== null && r.level >= n ? ' is-past' : '') + (r.level !== null && (n === 100 ? r.level === 100 : r.level >= n && r.level < SCALE[SCALE.indexOf(n) + 1]) ? ' is-here' : ''));
      add(li, el('span', 'lcc-sp-dot', '', { 'aria-hidden': 'true' }), el('b', '', 'LV ' + n));
      row.appendChild(li);
    }
    add(wp, row);
    return add(d, wp); // no row of three fact tiles (#1053): the Houses are the panel below, the XP ceiling the bar's own end, the keystones the bar above
  }

  function opsOf(vm, r) {
    const a = el('aside', 'lcc-sp-ops lcc-sp-surface', '', { 'aria-labelledby': 'lcs-ops-h' });
    const top = el('div', 'lcc-sp-ops-top');
    add(top, el('p', 'lcc-sp-eyebrow', 'THE EDITOR’S OWN OPERATIONS'), el('h3', 'lcc-sp-h3', 'Specialization Operations', { id: 'lcs-ops-h' })); // every change asks first and is written after a backup: the question and the result say so, and the Help tab explains it (#980)
    add(a, top);

    const off = !r.known;
    const why = off ? 'lcs-nonum' : null;
    const g1 = el('div', 'lcc-sp-group');
    add(g1, add(el('div', 'lcc-sp-glabel'), el('strong', '', r.label.toUpperCase() + (off ? ' · NOT IDENTIFIED' : '')), el('span', '', 'INDIVIDUAL TRACK')));
    if (off) add(g1, el('p', 'lcc-sp-note lcc-sp-note-warn', 'No track number of the save is tied to this path, so XP editing is off.', { id: 'lcs-nonum' }));
    add(g1, el('label', 'lcc-sp-lbl', 'Add XP', { for: 'spx' + r.track }));
    const inp = el('input', 'lcc-sp-input', '', { id: 'spx' + r.track, type: 'number', step: 1, min: 1, max: vm.maxXp, value: Object.prototype.hasOwnProperty.call(state.xp, r.track) ? state.xp[r.track] : 1000, 'aria-describedby': 'lcs-xphint lcs-xperr' + (why ? ' ' + why : ''), disabled: off });
    inp.addEventListener('keydown', (e) => { if (e.key === '-' || e.key === 'Subtract') e.preventDefault(); }); // no minus sign: XP is not removed here (#1032)
    const xt = tipOf('lcs-xphint', 'About the XP amount', 'Whole number from 1 to ' + fmt(vm.maxXp) + '; zero and negative numbers are not accepted (to take XP back, use Reset). The total stays at most ' + fmt(vm.maxXp) + '. The editor shows the XP before and after.');
    add(g1, wireTip(add(el('div', 'lcc-sp-inline lcc-tt'), inp, btn('Apply XP', 'is-primary', { 'data-act': 'spAddCC', 'data-t': r.track, disabled: off, 'aria-describedby': 'lcs-xphint' + (why ? ' ' + why : '') }), xt.mark, xt.tip)));
    add(g1, el('p', 'lcc-sp-note lcc-sp-note-warn', off ? '' : amountProblem(String(inp.value), vm.maxXp), { id: 'lcs-xperr', 'aria-live': 'polite' }));
    g1.lastChild.hidden = !g1.lastChild.textContent; // an empty note takes no room
    inp.setAttribute('aria-invalid', g1.lastChild.hidden ? 'false' : 'true');
    add(g1, add(el('div', 'lcc-sp-grid2'),
      btn('✦ Grant LV 100', '', { 'data-act': 'spMax', 'data-t': r.track, disabled: off, 'aria-describedby': 'lcs-trkhint' + (why ? ' ' + why : '') }),
      btn('Reset Track XP', 'is-danger', { 'data-act': 'spReset', 'data-t': r.track, disabled: off, 'aria-describedby': 'lcs-trkhint' + (why ? ' ' + why : '') })));
    add(g1, el('p', 'lcc-sp-hint', 'Grant LV 100 sets this track to level 100 and its XP cap. Reset Track XP sets its XP and level back to 0. Neither changes any keystone.', { id: 'lcs-trkhint' }));
    add(a, g1);

    const g2 = el('div', 'lcc-sp-group lcc-sp-global', '', { role: 'group', 'aria-labelledby': 'lcs-glob-h' });
    add(g2, add(el('div', 'lcc-sp-glabel'), el('strong', '', 'All Five Tracks', { id: 'lcs-glob-h' }), el('span', '', 'GLOBAL OPERATIONS')));
    const T = vm.totals;
    add(g2, el('p', 'lcc-sp-scope', T.total > 0 ? 'Keystones now: ' + T.owned + ' of ' + T.total + ' purchased across all ' + T.tracks + ' tracks. The two keystone buttons change every track, not the selected one.' : 'The two keystone buttons change every track, not the selected one. The save does not say how many keystones there are.', { id: 'lcs-keyscope' }));
    add(g2, add(el('div', 'lcc-sp-grid2'),
      btn('Grant All Keystones', '', { 'data-act': 'spKeyAll', 'aria-describedby': 'lcs-keyscope' }),
      btn('Reset All Keystones', 'is-danger', { 'data-act': 'spKeyReset', 'aria-describedby': 'lcs-keyscope' })));
    const left = vm.unassigned.length;
    const resetOff = !vm.resetCount;
    const resetBtn = btn('Reset XP of All Known Tracks (' + vm.resetCount + ')', 'is-danger lcc-sp-wide', { 'data-act': 'spResetAll', 'data-n': vm.resetCount, 'data-left': left, disabled: resetOff, 'aria-describedby': 'lcs-resetnote' });
    const resetSay = (resetOff ? 'Nothing to reset: no track whose number is known has XP.' + (left ? ' ' + left + (left === 1 ? ' row of the save whose track is not known yet is left alone.' : ' rows of the save whose track is not known yet are left alone.') : '')
      : 'Resets the XP and level of ' + vm.resetCount + (vm.resetCount === 1 ? ' track' : ' tracks') + ' whose number is known, after you type a confirmation. Keystones are not touched.' + (left ? ' ' + left + (left === 1 ? ' row whose track is not known yet is left alone.' : ' rows whose track is not known yet are left alone.') : ''));
    if (resetOff) add(g2, resetBtn, el('p', 'lcc-sp-hint', resetSay, { id: 'lcs-resetnote' }));
    else {
      const rt = tipOf('lcs-resetnote', 'About resetting all tracks', resetSay);
      add(g2, wireTip(add(el('div', 'lcc-sp-resetrow lcc-tt'), resetBtn, rt.mark, rt.tip)));
    }
    return add(a, g2);
  }

  // A hover/focus tip for a long explanation (#1030): the page keeps no always-visible sentence for it. The tip is the existing .lcc-tt/.lcc-tip pattern of lcc.js (real text, role tooltip, shown on hover and on keyboard focus by lcc.css);
  // the controls and the info mark point to it with aria-describedby, and the info mark is a focusable button so a touch screen can open it. Escape closes it while the mouse or the focus stays.
  function tipOf(id, name, text) {
    const mark = el('button', 'lcc-sp-info', '', { type: 'button', 'aria-label': name, 'aria-describedby': id });
    add(mark, el('span', 'lcc-sp-info-i', 'i', { 'aria-hidden': 'true' })); // the circle is smaller than the 44 px target around it
    mark.addEventListener('click', () => mark.focus());
    const tip = el('span', 'lcc-tip', text, { role: 'tooltip', id });
    return { mark, tip };
  }
  function wireTip(w) {
    w.addEventListener('keydown', (e) => { if (e.key === 'Escape') w.classList.add('dismissed'); });
    const back = () => w.classList.remove('dismissed');
    w.addEventListener('mouseleave', back);
    w.addEventListener('focusout', back);
    return w;
  }

  function housesOf(vm, r) {
    const s = el('section', 'lcc-sp-houses lcc-sp-surface', '', { 'aria-labelledby': 'lcs-hs-h' });
    add(s, el('p', 'lcc-sp-eyebrow', 'LANDSRAAD INTELLIGENCE'), el('h3', 'lcc-sp-h3', 'Associated Houses · ' + r.label, { id: 'lcs-hs-h', title: 'The Houses of this path, from a public community tracker, not from the game or the save: treat it as a reference. Choosing a House opens it on the Landsraad board; nothing is written.' }));
    const list = el('ul', 'lcc-sp-hlist', '', { role: 'list' });
    if (r.houses === null) add(s, el('p', 'lcc-sp-note', 'The Landsraad of this save could not be read, so the Houses of this path are ' + NOT_IN_SAVE.toLowerCase() + '.'));
    else if (!r.houses.length) add(s, el('p', 'lcc-sp-note', 'No House of this term is listed under this path.'));
    else {
      for (const h of r.houses) {
        const title = LC() && LC().houseTitle ? LC().houseTitle(h.name) : String(h.name);
        const b = el('button', 'lcc-sp-house', '', { type: 'button', 'data-lcs-go': h.id, 'aria-label': 'Open House ' + title + ' on the Landsraad board' });
        add(b, add(el('span', 'lcc-sp-hc', '', { 'aria-hidden': 'true' }), houseCrest(h.name)), add(el('span', 'lcc-sp-hn', '', { 'aria-hidden': 'true' }), el('span', '', 'HOUSE'), el('strong', '', title.toUpperCase())), el('span', 'lcc-sp-arrow', '↗', { 'aria-hidden': 'true' }));
        add(list, add(el('li', ''), b));
      }
      add(s, list);
    }
    return s;
  }

  // The one line that is left of the old mapping section (#986): shown only when the save holds a row of specialization XP that no path owns. Read-only: nothing here writes.
  function unknownRowOf(vm) {
    if (!vm.unassigned.length) return null;
    const nums = vm.unassigned.map((u) => u.number);
    return el('p', 'lcc-sp-note lcc-sp-note-warn', (nums.length === 1 ? 'Unknown specialization row in the save (not edited): number ' : 'Unknown specialization rows in the save (not edited): numbers ') + nums.join(', ') + '.', { id: 'lcs-unknown-row' });
  }

  function emptyOf(msg) {
    return add(el('section', 'lcc-sp-surface lcc-sp-empty', '', { role: 'status' }), el('h2', 'lcc-sp-h2s', 'Specializations'), el('p', 'lcc-sp-note lcc-sp-note-warn', msg));
  }

  // mount(container, vm, host): draws the view into the container. host.goHouse(id) opens a House on the Landsraad board (it only moves between pages and writes nothing).
  function mount(container, vm, host) {
    if (!container) return null;
    container.textContent = '';
    const rootEl = el('div', 'lcc lcc-sp', '', { id: 'lcs-root', 'aria-label': 'Specializations Command Center' });
    container.appendChild(rootEl);
    if (!vm || !vm.ok) { rootEl.appendChild(emptyOf(vm && vm.problem ? vm.problem : 'The specialization tracks could not be read.')); return rootEl; }
    if (!vm.rows.some((x) => x.key === state.sel)) state.sel = vm.rows[0].key;
    const r = vm.rows.find((x) => x.key === state.sel);
    add(rootEl, heroOf(vm));
    const head = el('div', 'lcc-sp-headrow');
    add(head, add(el('div'), el('p', 'lcc-sp-eyebrow', 'CHOOSE A PATH'), el('h3', 'lcc-sp-h3', 'Specialization Lanes')));
    add(rootEl, head);
    const lanes = el('div', 'lcc-sp-lanes', '', { role: 'group', 'aria-label': 'Five specialization lanes' });
    for (const x of vm.rows) lanes.appendChild(laneOf(vm, x));
    add(rootEl, lanes);
    const two = el('div', 'lcc-sp-two');
    add(two, dossierOf(vm, r), opsOf(vm, r));
    add(rootEl, two, housesOf(vm, r), unknownRowOf(vm));
    rootEl.addEventListener('input', (e) => {
      const t = e.target;
      if (t && t.classList && t.classList.contains('lcc-sp-input') && typeof t.id === 'string' && t.id.indexOf('spx') === 0) {
        state.xp[t.id.slice(3)] = t.value;
        const bad = amountProblem(String(t.value), vm.maxXp); // the same refusal as the one on Apply, said where the number is typed
        const p = rootEl.querySelector('#lcs-xperr');
        if (p) { p.textContent = bad; p.hidden = !bad; }
        t.setAttribute('aria-invalid', bad ? 'true' : 'false');
      }
    });
    rootEl.addEventListener('click', (e) => {
      const t = e.target && e.target.closest ? e.target : null;
      const lane = t && t.closest('[data-lcs-lane]');
      if (lane) { state.sel = lane.getAttribute('data-lcs-lane'); mount(container, vm, host); const n = container.querySelector('[data-lcs-lane="' + state.sel + '"]'); if (n && n.focus) n.focus({ preventScroll: true }); return; }
      const go = t && t.closest('[data-lcs-go]');
      if (go && host && typeof host.goHouse === 'function') host.goHouse(Number(go.getAttribute('data-lcs-go')));
    });
    return rootEl;
  }

  const api = { LEVEL_XP, MAX_LEVEL, ORDER, SCALE, NOT_IN_SAVE, state, levelOf, xpToNext, amountProblem, readSpecs, masteryText, laneBadge, xpRemainText, mount };
  if (typeof module !== 'undefined' && module.exports) module.exports = api;
  else root.LCCS = api;
})(typeof window !== 'undefined' ? window : globalThis);
