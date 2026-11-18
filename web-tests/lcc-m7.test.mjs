// Command Center milestone M7: the regression net across the four pages (Board, Sysselraad Strategy, Term and Decrees, Specializations). What each page writes was compared with the classic page
// in lcc-board-write, lcc-board-requests, lcc-term-requests and lcc-spec-requests; this file ties them together: the list of routes that can write is fixed and every one of them is
// exercised by an equivalence test, looking at any of the four pages sends nothing, the text floor holds in every style file of the Command Center, and the files are plain public text with
// LF line ends. Real page code and the synthetic answers of M1; no game save is read.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { execFileSync } from 'node:child_process';
import { boot, closeAll } from './boot.mjs';
import { synthetic } from './lcc-fixture.mjs';
import { lccDir, read, root } from './lcc-load.mjs';

afterEach(closeAll);
const text = (e) => (e ? e.textContent.replace(/\s+/g, ' ').trim() : '');
const click = (ui, el) => { if (typeof el === 'string') el = ui.doc.querySelector(el); assert.ok(el, 'no element'); if (el.focus) el.focus(); el.dispatchEvent(new ui.w.MouseEvent('click', { bubbles: true, cancelable: true })); };
const SPECS = () => JSON.parse(JSON.stringify(JSON.parse(read('docs', 'design', 'landsraad-command-center', 'fixtures', 'api-player-specs.cases.json')).cases.find((c) => c.id === 'mixed-five-tracks').response));

// ---- the routes that write: a fixed list, each one exercised by an equivalence test ----
const WRITE_ROUTES = [
  '/api/landsraad/apply', '/api/landsraad/decree', '/api/landsraad/term',
  '/api/player/specs', '/api/player/specs/assign', '/api/player/specs/keystones/grant', '/api/player/specs/keystones/reset', '/api/player/specs/max', '/api/player/specs/reset',
  '/api/player/specs/reset-all', '/api/player/specs/xp',
];

test('the server registers exactly these write routes for the Landsraad and the Specializations, and the page sends only to them', () => {
  const go = read('internal', 'web', 'server.go');
  const registered = [...go.matchAll(/s\.post\("(\/api\/(?:landsraad|player\/specs)[^"]*)"/g)].map((m) => m[1]).sort();
  assert.deepEqual(registered, [...WRITE_ROUTES].sort(), 'a write route was added or removed: it needs its own review, its own equivalence test and a row in the M7 document');
  const app = read('internal', 'web', 'static', 'app.js');
  const used = new Set([...app.matchAll(/api\('(\/api\/(?:landsraad|player\/specs)[^']*)'/g)].map((m) => m[1]));
  for (const r of used) assert.ok(r === '/api/landsraad' || r === '/api/player/specs' || WRITE_ROUTES.includes(r), 'the page reaches ' + r);
  // the GET reads of the two pages are the same two routes in every view
  const gets = [...go.matchAll(/s\.get\("(\/api\/(?:landsraad|player\/specs)[^"]*)"/g)].map((m) => m[1]).sort();
  assert.deepEqual(gets, ['/api/landsraad', '/api/player/specs']);
});

test('every write route is played by an equivalence test: the board against the classic list, the Term and Decrees section and the Specializations view against recordings of the classic page', () => {
  const sources = ['lcc-board-write.test.mjs', 'lcc-board-requests.test.mjs', 'lcc-term-requests.test.mjs', 'lcc-spec-requests.test.mjs', 'lcc-board-requests.golden.json', 'lcc-term-requests.golden.json', 'lcc-spec-requests.golden.json']
    .map((f) => read('web-tests', f)).join('\n');
  for (const r of WRITE_ROUTES) {
    const where = r === '/api/landsraad/apply' ? /landsraad\/apply/ : new RegExp(r.replace(/[/]/g, '\\/') + '(?![\\w/-])');
    assert.match(sources, where, 'no equivalence test sends ' + r);
  }
  for (const f of ['lcc-board-requests.golden.json', 'lcc-term-requests.golden.json', 'lcc-spec-requests.golden.json']) {
    const g = JSON.parse(read('web-tests', f));
    assert.ok(Object.keys(g).length >= 2, f + ' has recorded scenarios');
    for (const [name, v] of Object.entries(g)) assert.ok(Array.isArray(v.sent), f + ' ' + name + ' records what was sent');
  }
});

// ---- looking sends nothing, on any of the four pages ----
const SAFE = ['layout', 'strategy', 'line', 'jumpmissing', 'clearline', 'group', 'lsort', 'sort', 'toterm', 'back', 'clearsel', 'gopath'];
test('looking at the board, the strategy, the term section and the specializations sends nothing: every House, layout, sort, line, link and lane', async () => {
  const a = synthetic();
  const ui = await boot({ get: { '/api/landsraad': a, '/api/player/specs': SPECS() }, local: { tab: 'landsraad', 'lsb.view': 'board', 'lsb.layout': 'grid', 'spec.view': 'cc' } });
  await ui.settle();
  assert.ok(ui.doc.querySelector('#lcc-term'), 'the term section is on the board page');
  // the board: every House, then every safe control that is there
  for (const card of [...ui.doc.querySelectorAll('#lsb-lcc [data-fk^="ht:"]')]) { const c = ui.doc.querySelector('[data-fk="' + card.dataset.fk + '"]'); if (c) { click(ui, c); await ui.settle(); } }
  assert.equal(ui.doc.querySelectorAll('#lsb-lcc [data-fk^="ht:"]').length, 25);
  for (const kind of SAFE) {
    for (let i = 0; i < 14; i++) {
      const els = [...ui.doc.querySelectorAll('#lsb-lcc [data-lcb="' + kind + '"], #lcc-term [data-lcb="' + kind + '"]')].filter((e) => !e.disabled);
      if (!els[i]) break;
      click(ui, els[i]);
      await ui.settle();
    }
  }
  ui.doc.dispatchEvent(new ui.w.KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
  // the term section: open every disclosure
  for (const d of ui.doc.querySelectorAll('#lcc-term details')) d.open = true;
  // the Specializations view: every lane, the switch to the classic list and back
  click(ui, '[data-tab="player"]'); await ui.settle();
  click(ui, '[data-sub="player:specialization"]'); await ui.settle();
  const lanes = [...ui.doc.querySelectorAll('[data-lcs-lane]')];
  assert.equal(lanes.length, 5);
  for (const l of lanes) { click(ui, ui.doc.querySelector('[data-lcs-lane="' + l.dataset.lcsLane + '"]')); await ui.settle(); }
  for (const go of [...ui.doc.querySelectorAll('[data-lcs-go]')].slice(0, 3)) { click(ui, go); await ui.settle(); click(ui, '[data-tab="player"]'); await ui.settle(); click(ui, '[data-sub="player:specialization"]'); await ui.settle(); }
  click(ui, '[data-spview="classic"]'); await ui.settle();
  click(ui, '[data-spview="cc"]'); await ui.settle();
  assert.deepEqual(ui.posted, [], 'a request was sent while only looking: ' + JSON.stringify(ui.posted.map((p) => p.path)));
});

// ---- the text floor in every style file of the Command Center ----
// A rule with a size in px keeps it for the cascade: the last plain rule of a selector wins and a rule inside a query is checked on its own. Names and key labels are 12 px or more, other text 11 px or
// more. The rows of KNOWN are open issues, each one a size that is below the agreement today; the test fails when a new one appears and when a listed one is fixed (so the list is cleaned up).
const KNOWN = new Map([['lcc.css .lcc-flag', '#907: a rule only the preview gallery reaches']]);
function rulesOf(src) {
  const bare = src.replace(/\r/g, '').replace(/\/\*[\s\S]*?\*\//g, '');
  const out = []; let i = 0;
  const walk = (end, at) => {
    while (i < end) {
      const open = bare.indexOf('{', i);
      if (open < 0 || open >= end) break;
      const head = bare.slice(i, open).trim();
      let depth = 1, j = open + 1;
      while (depth && j < end) { if (bare[j] === '{') depth++; else if (bare[j] === '}') depth--; j++; }
      const body = bare.slice(open + 1, j - 1);
      if (/^@(media|container|supports)/.test(head)) { i = open + 1; walk(j - 1, head); i = j; } else { if (!head.startsWith('@')) out.push({ at, sel: head, body }); i = j; }
    }
  };
  walk(bare.length, '');
  return out;
}
const sizeOf = (body) => { const m = /(?:^|;)\s*font-size:\s*([\d.]+)px/.exec(body) || /(?:^|;)\s*font:\s*(?:italic\s+|bold\s+|\d+\s+)*(?:clamp\()?([\d.]+)px/.exec(body); return m ? Number(m[1]) : null; };

test('text floor over every style file of the Command Center: no size under 11 px survives the cascade, names and key labels are 12 px or more', () => {
  const files = fs.readdirSync(lccDir).filter((f) => f.endsWith('.css')).sort();
  assert.deepEqual(files, ['lcc-board.css', 'lcc-spec.css', 'lcc-term.css', 'lcc.css']);
  const low = new Map(); const fixed = [];
  for (const f of files) {
    const winner = new Map();
    for (const r of rulesOf(fs.readFileSync(path.join(lccDir, f), 'utf8'))) {
      const px = sizeOf(r.body);
      if (px === null) continue;
      for (const s of r.sel.split(',').map((x) => x.trim())) {
        if (r.at === '') winner.set(s, px);
        else if (px < 11) low.set(f + ' ' + r.at.slice(0, 40) + ' ' + s, px);
      }
    }
    for (const [s, px] of winner) if (px < 11) low.set(f + ' ' + s, px);
    for (const [k, why] of KNOWN) if (k.startsWith(f + ' ') && !low.has(k)) fixed.push(k + ' (' + why + ')');
    for (const s of ['.lcc-bd-name', '.lcc-hc-name', '.lcc-tm-name', '.lcc-sp-name', '.lcc-sp-btn', '.lcc-tm-btn']) if (winner.has(s)) assert.ok(winner.get(s) >= 12, f + ' ' + s + ' ' + winner.get(s));
  }
  const fresh = [...low].filter(([k]) => !KNOWN.has(k));
  assert.deepEqual(fresh, [], 'under 11 px after the cascade: ' + fresh.map(([k, px]) => k + ' ' + px).join(' | '));
  assert.deepEqual(fixed, [], 'a known size was fixed: take it off the list');
});

test('every touch rule keeps a floor: a coarse pointer or a narrow window gets controls of 44 px in each of the three pages that have controls of their own', () => {
  for (const [f, sel] of [['lcc-board.css', /min-height:\s*44px/], ['lcc-spec.css', /min-height:\s*44px/], ['lcc-term.css', /min-height:\s*44px/]]) {
    const css = fs.readFileSync(path.join(lccDir, f), 'utf8').replace(/\r/g, '');
    const inTouch = rulesOf(css).filter((r) => /pointer:\s*coarse|max-width:\s*(640|900)px/.test(r.at) && sel.test(r.body));
    assert.ok(inTouch.length >= 1, f + ' has no 44 px rule for touch or narrow windows');
  }
});

// ---- plain public text, LF line ends ----
function walk(dir, keep, out = []) {
  for (const e of fs.readdirSync(dir, { withFileTypes: true })) {
    const p = path.join(dir, e.name);
    if (e.isDirectory()) { if (e.name !== 'node_modules' && e.name !== 'fixtures' || p.includes('landsraad-command-center')) walk(p, keep, out); } else if (keep(p)) out.push(p);
  }
  return out;
}
const TEXT = /\.(md|json|csv|html|js|mjs|css|go)$/;
const files = () => [
  ...walk(lccDir, (p) => TEXT.test(p)),
  ...walk(path.join(root, 'docs', 'design', 'landsraad-command-center'), (p) => TEXT.test(p)),
  ...fs.readdirSync(path.join(root, 'web-tests')).filter((f) => /^lcc-/.test(f) && TEXT.test(f)).map((f) => path.join(root, 'web-tests', f)),
  ...fs.readdirSync(path.join(root, 'scripts')).filter((f) => /^lcc-/.test(f)).map((f) => path.join(root, 'scripts', f)),
];

test('the Command Center files, its notes, its tests and its scripts are stored with LF line ends (the committed blobs; a Windows checkout may show CRLF in the working copy)', () => {
  const list = files().map((p) => path.relative(root, p).split(path.sep).join('/'));
  assert.ok(list.length > 40, 'files found: ' + list.length);
  let eol;
  try { eol = execFileSync('git', ['ls-files', '--eol', '--', ...list], { cwd: root, encoding: 'utf8', timeout: 60000 }); } catch (e) { return; } // not a git checkout: nothing to read
  const rows = eol.split(/\r?\n/).filter(Boolean).map((l) => ({ idx: /^i\/(\S+)/.exec(l)[1], file: l.split(/\t/)[1] }));
  if (!rows.length) return;
  const bad = rows.filter((r) => r.idx === 'crlf' || r.idx === 'mixed');
  assert.deepEqual(bad.map((r) => r.file), []);
});

test('the Command Center files name no local path, no user folder, no drive letter and no game-internal asset file name', () => {
  const found = [];
  const rules = [
    [/\b[A-Za-z]:[\\/](?:Users|Windows)\b/, 'a local path'],
    [/(?:^|[\s"'(=])\/(?:Users|home)\/[A-Za-z0-9_.-]+/m, 'a user folder'],
    [/\\Users\\|AppData|\.claude\b|\bscratchpad\b/, 'a user folder'],
    [/\.uasset\b|\.umap\b|\/Game\/[A-Z]|\bT_[A-Z][A-Za-z0-9]+_[A-Za-z0-9_]+\.(?:png|webp|tga)\b/, 'a game-internal asset name'],
  ];
  for (const p of files()) {
    if (path.basename(p) === 'lcc-m7.test.mjs') continue; // holds the patterns
    const s = fs.readFileSync(p, 'utf8');
    for (const [re, what] of rules) { const m = re.exec(s); if (m) found.push(path.relative(root, p) + ': ' + what + ' (' + m[0].trim().slice(0, 30) + ')'); }
  }
  assert.deepEqual(found, []);
});

test('no picture or file of a personal save is among the Command Center files', () => {
  const art = walk(path.join(lccDir, 'art'), () => true).map((p) => path.relative(lccDir, p).replace(/\\/g, '/'));
  assert.ok(art.length > 40);
  for (const p of art) assert.match(p, /^art\/(houses|lanes|banners|decrees|factions|standards|specs)\/[a-z0-9-]+\.(webp|png|svg)$/, 'an unexpected file in the art folder: ' + p);
  const stray = walk(path.join(root, 'docs', 'design', 'landsraad-command-center'), (p) => /\.(png|jpe?g|webp|gif|db|sqlite3?|bak|ini|log|zip)$/i.test(p));
  assert.deepEqual(stray.map((p) => path.relative(root, p)), [], 'a picture or save-like file among the notes');
});

// ---- Workstream B: an independent reading of the twelve lines, written here from the rules and not from the evaluator, against the evaluator and against the cards on the page ----
// A seeded generator, so the boards are the same on every run. The reference below knows only: the lines are the rows, the columns and the two diagonals of the 5 by 5 board; a line is held with
// five Houses of one faction; a line is blocked by a House of each faction; an unreadable House makes the line unreadable; 13 Houses of a faction that has more than the other is a count; both
// factions with a line is a conflict; nothing is ever called a result without a verified cutoff, and the page always passes none.
function lcg(seed) { let s = seed >>> 0; return () => { s = (Math.imul(s, 1664525) + 1013904223) >>> 0; return s / 4294967296; }; }
const LINES = [];
for (let r = 0; r < 5; r++) LINES.push([0, 1, 2, 3, 4].map((c) => r * 5 + c));
for (let c = 0; c < 5; c++) LINES.push([0, 1, 2, 3, 4].map((r) => r * 5 + c));
LINES.push([0, 6, 12, 18, 24], [4, 8, 12, 16, 20]);
function reference(letters) {
  const n = { A: 0, H: 0, O: 0, '?': 0 };
  for (const c of letters) n[c]++;
  const lines = LINES.map((idx) => {
    const k = { A: 0, H: 0, O: 0, '?': 0 };
    for (const i of idx) k[letters[i]]++;
    if (k['?']) return 'unreadable';
    if (k.A && k.H) return 'blocked';
    if (k.A === 5) return 'held-A';
    if (k.H === 5) return 'held-H';
    if (k.O === 1) return k.A ? 'one-away-A' : 'one-away-H';
    return 'open';
  });
  const heldA = lines.some((l) => l === 'held-A'), heldH = lines.some((l) => l === 'held-H');
  const lead = n.A > n.H ? 'A' : n.H > n.A ? 'H' : null;
  const count = lead && n[lead] >= 13 ? lead : null;
  const verdict = heldA && heldH ? 'conflict' : heldA || heldH ? 'line_held' : n['?'] ? 'unknown' : lead ? 'pending_vote' : lines.some((l) => l.startsWith('one-away')) ? 'possible' : 'none';
  const state = heldA && heldH ? 'conflict' : heldA || heldH ? 'line' : count ? 'count' : 'contested';
  const circuit = [...new Set(LINES.filter((_, i) => lines[i].startsWith('held')).flat())].sort((a, b) => a - b);
  const dominion = count && !(heldA && heldH) ? [...letters].map((c, i) => [c, i]).filter(([c, i]) => c === count && !circuit.includes(i)).map(([, i]) => i) : [];
  const potential = lines.filter((l) => l === 'open' || l.startsWith('one-away')).length;
  return { n, lines, verdict, state, circuit, dominion, potential, unreadable: n['?'] > 0 };
}
function boards(count) {
  const rnd = lcg(20261118), out = [];
  const mixes = [[.34, .33, .33, 0], [.6, .1, .3, 0], [.1, .6, .3, 0], [.5, .5, 0, 0], [.2, .2, .56, .04], [.7, .05, .25, 0], [.04, .04, .92, 0], [.45, .45, .08, .02]];
  for (let b = 0; b < count; b++) {
    const mix = mixes[b % mixes.length];
    const s = [];
    for (let i = 0; i < 25; i++) { const r = rnd(); s.push(r < mix[0] ? 'A' : r < mix[0] + mix[1] ? 'H' : r < mix[0] + mix[1] + mix[2] ? 'O' : '?'); }
    // a third of the boards get a full line of one faction put in, so the held lines are well tested
    if (b % 3 === 0) { const line = LINES[Math.floor(rnd() * 12)], f = rnd() < .5 ? 'A' : 'H'; for (const i of line) s[i] = f; }
    out.push(s.join(''));
  }
  return out;
}

test('the evaluator agrees with an independent reading of 3000 seeded boards: counts, every line, potential lines and the verdict', async () => {
  const { createRequire } = await import('node:module');
  const M = createRequire(import.meta.url)(path.join(lccDir, 'lcc-model.js'));
  const OWN = { A: 'a', H: 'h', O: 'open', '?': 'unknown' };
  let held = 0, conflicts = 0, counts13 = 0;
  for (const b of boards(3000)) {
    const ref = reference(b), got = M.evaluateOwners([...b].map((c) => OWN[c]), null, { termId: 7 });
    const lines = got.lines.map((l) => (l.status === 'unknown' ? 'unreadable' : l.status.startsWith('complete') ? 'held-' + l.status.slice(-1).toUpperCase() : l.status.startsWith('one-away') ? 'one-away-' + l.status.slice(-1).toUpperCase() : l.status.startsWith('open') ? 'open' : l.status));
    assert.deepEqual(lines, ref.lines, b);
    assert.deepEqual([got.counts.a, got.counts.h, got.counts.open, got.counts.unknown], [ref.n.A, ref.n.H, ref.n.O, ref.n['?']], b);
    assert.equal(got.verdict, ref.verdict, b);
    assert.equal(got.countComplete, !ref.unreadable, b);
    if (!ref.unreadable) assert.equal(got.potential.total, ref.potential, b);
    assert.equal(got.vote.verified, false, 'no cutoff is ever claimed: ' + b);
    assert.notEqual(got.verdict, 'majority_at_cutoff', b);
    if (ref.verdict === 'line_held') held++; if (ref.verdict === 'conflict') conflicts++; if (ref.state === 'count') counts13++;
  }
  assert.ok(held > 200 && conflicts > 20 && counts13 > 100, 'the seeded boards reach the states: ' + [held, conflicts, counts13]);
});

test('the cards and the panel on the page agree with the independent reading on 48 seeded boards: the circuit outline, the majority outline, the state of the panel and the number of potential lines', async () => {
  const picked = boards(480).filter((_, i) => i % 10 === 3).slice(0, 48);
  for (const b of picked) {
    const a = synthetic();
    for (const t of a.tasks) { const c = b[t.board_index]; t.completed = c === 'A' || c === 'H' || c === '?' ? 1 : 0; t.winning_faction_id = c === 'A' ? 1 : c === 'H' ? 2 : null; }
    const ui = await boot({ get: { '/api/landsraad': a }, local: { tab: 'landsraad', 'lsb.view': 'board', 'lsb.layout': 'grid' } });
    await ui.settle();
    const ref = reference(b);
    const cards = [...ui.doc.querySelectorAll('.lcc-bd-cell .lcc-hc')];
    assert.equal(cards.length, 25, b);
    const at = (kind) => cards.map((c, i) => [i, c.getAttribute('data-victory')]).filter((x) => x[1] === kind).map((x) => x[0]);
    assert.deepEqual(at('circuit'), ref.circuit, 'circuit ' + b);
    assert.deepEqual(at('dominion'), ref.dominion, 'dominion ' + b);
    assert.equal(ui.doc.querySelector('.lcc-bd-victory').getAttribute('data-state'), ref.state, 'panel ' + b);
    const lines = text(ui.doc.querySelector('[data-metric="lines"]'));
    if (ref.unreadable) assert.match(lines, /Count incomplete/i, b); else assert.match(lines, new RegExp('^POTENTIAL LINES' + ref.potential + '(?!\d)'), b + ' ' + lines);
    assert.deepEqual(ui.posted, [], 'looking sends nothing');
    closeAll();
  }
});

// ---- Workstream D: switching tabs and jumping 50 times leaks no timer, no listener on the page and no element ----
test('fifty tab switches and fifty House jumps leave no new timer, no new listener on the page or the window, no extra element, and send nothing', async () => {
  const timers = new Set(); let listeners = 0;
  const ui = await boot({
    get: { '/api/landsraad': synthetic(), '/api/player/specs': SPECS() },
    local: { tab: 'landsraad', 'lsb.view': 'board', 'lsb.layout': 'grid', 'spec.view': 'cc' },
    pre: (w) => {
      const si = w.setInterval.bind(w), ci = w.clearInterval.bind(w);
      w.setInterval = (...a) => { const id = si(...a); timers.add(id); return id; };
      w.clearInterval = (id) => { timers.delete(id); return ci(id); };
      for (const target of [w, w.document]) { const add = target.addEventListener.bind(target); target.addEventListener = (...a) => { listeners++; return add(...a); }; }
    },
  });
  await ui.settle();
  const nodes = () => ui.doc.getElementsByTagName('*').length;
  const jump = async (i) => { const s = ui.doc.querySelector('[data-lcb="jump"]'); const opts = [...s.options].filter((o) => o.value !== ''); s.value = opts[i % opts.length].value; s.dispatchEvent(new ui.w.Event('change', { bubbles: true })); await ui.settle(); };
  const cycle = async (i) => {
    click(ui, '[data-tab="player"]'); await ui.settle();
    click(ui, '[data-sub="player:specialization"]'); await ui.settle();
    click(ui, '[data-tab="landsraad"]'); await ui.settle();
    await jump(i);
  };
  await cycle(0); await cycle(1); // the first rounds set everything up once
  const base = { timers: timers.size, listeners, nodes: nodes() };
  for (let i = 0; i < 50; i++) await cycle(i + 2);
  await cycle(1); // the same House as when the base was taken
  assert.equal(timers.size, base.timers, 'timers');
  assert.equal(listeners, base.listeners, 'listeners added to the page or the window by the redraws');
  assert.equal(nodes(), base.nodes, 'elements on the page');
  assert.equal(ui.doc.querySelectorAll('#lsb-lcc [data-fk^="ht:"]').length, 25);
  assert.deepEqual(ui.posted, []);
});
