// Headless sweep of the Command Center pages in a real browser engine (milestone M7). It draws the editor's own page (the embedded static files and the design module's files) with the
// synthetic answers of M1 for the Landsraad and the Specializations, and measures what a still image cannot: the smallest text, the smallest controls, anything wider than the window, anything
// clipped, and the number of Houses and seats. No game save is read, nothing is written (every POST is answered with a bare "ok" and counted), and no window is ever shown.
//
//   node scripts/lcc-m7-sweep.mjs [outDir]
//
// Needs Node 22 or later (the built-in WebSocket) and a Chromium-family browser (LCC_BROWSER names one; the usual Edge and Chrome places are tried). Pictures are written to the output folder
// (the argument, else LCC_SHOTS_OUT, else a folder in the system temporary directory), never inside the repository, together with sweep.json. The exit status is 1 when a page has a hard
// failure: a page error, a missing House or seat, a page wider than the window, text under 11 px, or a request to an address that is not this one.
//   LCC_SWEEP_ONLY   a substring: only the cases whose name contains it
//   LCC_SWEEP_SHOTS  1 to write a picture for every case (default: a few named ones)
//   LCC_SWEEP_SCROLL a CSS selector: scroll it to the top of the window before a picture is taken (for example .lcc-bd-tiles, the House cards)
import http from 'node:http';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { spawn } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const repo = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const out = path.resolve(process.argv[2] || process.env.LCC_SHOTS_OUT || path.join(os.tmpdir(), 'lcc-m7-sweep'));
fs.mkdirSync(out, { recursive: true });
const only = process.env.LCC_SWEEP_ONLY || '';
const allShots = process.env.LCC_SWEEP_SHOTS === '1';
const scrollTo = process.env.LCC_SWEEP_SCROLL || '';
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
const staticDir = path.join(repo, 'internal', 'web', 'static');
const designDir = path.join(repo, 'design', 'assets');
const fixDir = path.join(repo, 'docs', 'design', 'landsraad-command-center', 'fixtures');
const clean = (o) => JSON.parse(JSON.stringify(o, (k, v) => (k.startsWith('_') ? undefined : v)));
const LANDSRAAD = clean(JSON.parse(fs.readFileSync(path.join(fixDir, 'api-landsraad.synthetic.json'), 'utf8')));
const SPECS = JSON.parse(fs.readFileSync(path.join(fixDir, 'api-player-specs.cases.json'), 'utf8')).cases.find((c) => c.id === 'mixed-five-tracks').response;
// The other reads the page makes at start come from the generated UI fixtures (the same file the web tests use): UI_FIXTURES_DIR=web-tests/fixtures go test ./internal/web -run TestDumpUIFixtures
const FIX = process.env.UI_FIXTURES_FILE || path.join(process.env.UI_FIXTURES_DIR || path.join(repo, 'web-tests', 'fixtures'), 'ui-fixtures.json');
if (!fs.existsSync(FIX)) { console.error('UI fixtures not found at ' + FIX + '. Generate them first: UI_FIXTURES_DIR=web-tests/fixtures go test ./internal/web -run TestDumpUIFixtures'); process.exit(2); }
const FIXTURES = JSON.parse(fs.readFileSync(FIX, 'utf8'));
const types = { '.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css', '.webp': 'image/webp', '.svg': 'image/svg+xml', '.png': 'image/png', '.ttf': 'font/ttf', '.woff2': 'font/woff2', '.json': 'application/json' };

function findBrowser() {
  return [process.env.LCC_BROWSER, 'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe', 'C:/Program Files/Microsoft/Edge/Application/msedge.exe', 'C:/Program Files/Google/Chrome/Application/chrome.exe',
    '/usr/bin/chromium', '/usr/bin/chromium-browser', '/usr/bin/google-chrome'].find((p) => p && fs.existsSync(p));
}

// the editor's page, as the editor serves it: the embedded static files, the design module's files under /design/, and the two answers this sweep needs (any other read comes from the generated UI fixtures, else a 404)
const posts = [];
function serve() {
  return new Promise((resolve) => {
    const srv = http.createServer((req, res) => {
      const u = decodeURIComponent(req.url.split('?')[0]);
      const json = (code, o) => { res.statusCode = code; res.setHeader('content-type', 'application/json'); res.end(JSON.stringify(o)); };
      if (u.startsWith('/api/')) {
        if (req.method === 'POST') { posts.push(u); return json(200, { ok: true }); }
        if (u === '/api/landsraad') return json(200, LANDSRAAD);
        if (u === '/api/player/specs') return json(200, SPECS);
        const f = FIXTURES[u + (req.url.includes('?') ? '?' + req.url.split('?')[1] : '')] || FIXTURES[u];
        return f ? json(f.status, f.body) : json(404, { error: 'not in the fixtures: ' + u });
      }
      if (req.method !== 'GET') { res.statusCode = 405; return res.end(); }
      if (u === '/favicon.ico') { res.statusCode = 204; return res.end(); }
      const base = u.startsWith('/design/') ? designDir : staticDir;
      const rel = u === '/' ? 'index.html' : u.startsWith('/design/') ? u.slice('/design/'.length) : u.slice(1);
      const f = path.resolve(path.join(base, rel));
      if (!f.startsWith(base + path.sep) || !fs.existsSync(f) || fs.statSync(f).isDirectory()) { res.statusCode = 404; return res.end(); }
      res.setHeader('content-type', types[path.extname(f)] || 'application/octet-stream');
      res.end(fs.readFileSync(f));
    }).listen(0, '127.0.0.1', () => resolve(srv));
  });
}

async function launch(browser, profile) {
  fs.rmSync(profile, { recursive: true, force: true });
  fs.mkdirSync(profile, { recursive: true });
  const proc = spawn(browser, ['--headless=new', '--disable-gpu', '--no-first-run', '--no-default-browser-check', '--remote-debugging-port=0', '--user-data-dir=' + profile, 'about:blank'], { stdio: 'ignore', windowsHide: true });
  const f = path.join(profile, 'DevToolsActivePort');
  for (let i = 0; i < 150 && !fs.existsSync(f); i++) await sleep(100);
  if (!fs.existsSync(f)) { proc.kill(); throw new Error('the browser did not start'); }
  const [port] = fs.readFileSync(f, 'utf8').split('\n');
  const page = (await (await fetch(`http://127.0.0.1:${port}/json/list`)).json()).find((t) => t.type === 'page');
  const ws = new WebSocket(page.webSocketDebuggerUrl);
  await new Promise((r, j) => { ws.onopen = r; ws.onerror = j; });
  let id = 0; const pending = new Map(); const listeners = [];
  ws.onmessage = (m) => {
    const d = JSON.parse(m.data);
    if (d.id && pending.has(d.id)) { const { res, rej } = pending.get(d.id); pending.delete(d.id); d.error ? rej(new Error(JSON.stringify(d.error))) : res(d.result); } else if (d.method) listeners.forEach((l) => l(d));
  };
  const send = (method, params = {}) => new Promise((res, rej) => { const i = ++id; pending.set(i, { res, rej }); ws.send(JSON.stringify({ id: i, method, params })); });
  return { send, on: (l) => listeners.push(l), close: async () => { try { await send('Browser.close'); } catch (e) { /* closing */ } try { ws.close(); } catch (e) { /* closed */ } setTimeout(() => proc.kill(), 1500); } };
}

// ---- what runs inside the page ----
// roots: the three surfaces of the Command Center. Text: every visible text node's own size. Targets: every control a person presses. Clipped: a box whose content is wider or taller than it
// is and that does not scroll. Name-like text (a House name, a lane name, a decree name, a button label) must be 12 px or more, any other text 11 px or more.
const MEASURE = `(() => {
  const roots = [...document.querySelectorAll('#lsb-lcc, #lcc-term, #lcs-root')];
  const vis = (el) => { const r = el.getBoundingClientRect(); const cs = getComputedStyle(el); return r.width > 0 && r.height > 0 && cs.visibility !== 'hidden' && cs.display !== 'none' && !(cs.position === 'absolute' && cs.clip && cs.clip !== 'auto' && r.width <= 1); };
  const label = (el) => (el.className && el.className.baseVal === undefined ? '.' + String(el.className).trim().split(/\\s+/).slice(0, 2).join('.') : el.tagName.toLowerCase());
  const small = {}, between = {}, tiny = [], nodes = { n: 0 };
  const nameLike = (el) => /(^|-)(n|name|nm|title|h|hn|btn|b|lane-n|label|tag)(-|$)/.test(String(el.className && el.className.baseVal === undefined ? el.className : '').split(/\\s+/).join(' ')) || /^(H1|H2|H3|H4|BUTTON|LABEL|SUMMARY|TH)$/.test(el.tagName);
  for (const root of roots) {
    const w = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
    for (let n; (n = w.nextNode());) {
      if (!n.nodeValue.trim()) continue;
      const el = n.parentElement;
      if (!el || !vis(el) || el.closest('svg, .sr, [hidden], script, style')) continue;
      nodes.n++;
      const px = parseFloat(getComputedStyle(el).fontSize);
      if (px < 11) { small[label(el)] = Math.min(small[label(el)] || 99, px); tiny.push({ at: label(el), px, text: n.nodeValue.trim().slice(0, 30) }); }
      else if (px < 12 && nameLike(el)) between[label(el)] = px;
    }
  }
  // the hit area of a control is its box plus any absolutely placed ::before / ::after that extends it (an invisible extension of a small box, #906)
  const hit = (el, r) => {
    let l = r.left, t = r.top, rt = r.right, b = r.bottom;
    for (const p of ['::before', '::after']) {
      const cs = getComputedStyle(el, p);
      if (cs.content === 'none' || cs.position !== 'absolute' || cs.pointerEvents === 'none') continue;
      const w = parseFloat(cs.width), h = parseFloat(cs.height), pl = parseFloat(cs.left), pt = parseFloat(cs.top);
      if (!(w > 0 && h > 0) || isNaN(pl) || isNaN(pt)) continue;
      const bl = r.left + el.clientLeft, bt = r.top + el.clientTop;
      l = Math.min(l, bl + pl); t = Math.min(t, bt + pt); rt = Math.max(rt, bl + pl + w); b = Math.max(b, bt + pt + h);
    }
    return { l, t, r: rt, b };
  };
  const targets = [];
  for (const root of roots) for (const el of root.querySelectorAll('button, a[href], input:not([type=hidden]), select, textarea, [role=button], summary')) {
    if (!vis(el) || el.closest('.sr, [hidden]')) continue;
    const r = el.getBoundingClientRect();
    if (el.type === 'checkbox' && el.closest('label')) { const lr = el.closest('label').getBoundingClientRect(); if (lr.height >= r.height) { targets.push({ at: label(el), w: Math.round(lr.width), h: Math.round(lr.height) }); continue; } }
    const hr = hit(el, r);
    targets.push({ at: label(el), w: Math.round(hr.r - hr.l), h: Math.round(hr.b - hr.t), vw: Math.round(r.width), vh: Math.round(r.height), tier: el.classList.contains('lcc-hc-t') });
  }
  // two tier boxes of one House whose hit areas overlap sideways (a tap would be ambiguous), and the gap between hit areas
  let tierOverlap = 0, tierGapMin = 999;
  for (const grp of document.querySelectorAll('#lsb-lcc .lcc-hc-tiers')) {
    const bs = [...grp.querySelectorAll('.lcc-hc-t')].map((b) => hit(b, b.getBoundingClientRect())).filter((x) => x.r > x.l);
    for (let i = 1; i < bs.length; i++) { const d = bs[i].l - bs[i - 1].r; if (d < -0.5) tierOverlap++; tierGapMin = Math.min(tierGapMin, d); }
  }
  const clipped = [];
  for (const root of roots) for (const el of root.querySelectorAll('*')) {
    if (!vis(el) || el.closest('svg')) continue;
    const cs = getComputedStyle(el);
    if (el.scrollWidth > el.clientWidth + 2 && el.clientWidth > 0 && !/(auto|scroll)/.test(cs.overflowX) && (cs.overflowX === 'hidden' || cs.overflowX === 'clip')) clipped.push({ at: label(el), sw: el.scrollWidth, cw: el.clientWidth, text: (el.textContent || '').trim().slice(0, 24) });
  }
  // the owner line of a House card ("Pledged to Harkonnen") keeps 4 px or more from both card edges and is not wider than its box (#1083)
  const ownerBad = [];
  for (const o of document.querySelectorAll('#lsb-lcc .lcc-hc-owner')) { const c = o.closest('.lcc-hc'); if (!c || !vis(o)) continue; const cr = c.getBoundingClientRect(), rg = document.createRange(); rg.selectNodeContents(o); const t = rg.getBoundingClientRect(); if (o.scrollWidth > o.clientWidth + 1 || t.left < cr.left + 4 || t.right > cr.right - 4) ownerBad.push({ text: o.textContent, card: Math.round(cr.width), left: Math.round(t.left - cr.left), right: Math.round(cr.right - t.right) }); }
  const over = [];
  for (const root of roots) for (const el of root.querySelectorAll('*')) { if (!vis(el) || el.closest('svg')) continue; const r = el.getBoundingClientRect(); if (r.right > innerWidth + 1 && !el.closest('[class*=scroll], .lcc-bd-listwrap, table')) over.push({ at: label(el), right: Math.round(r.right) }); }
  // a contributor line of a banner (#972) is shown whole or not at all: it is inside its list, or out of sight in the list's second column (the wide layout), never cut through; its amount is inside the banner
  const cutEntries = [];
  for (const li of document.querySelectorAll('#lsb-lcc .lcc-standard-list > li')) {
    const o = li.parentElement.getBoundingClientRect(), r = li.getBoundingClientRect(), a = li.querySelector('.lcc-standard-amt');
    const inside = r.left >= o.left - 1 && r.right <= o.right + 1 && r.top >= o.top - 1 && r.bottom <= o.bottom + 1;
    const away = r.left >= o.right - 1 || r.top >= o.bottom - 1;
    const ar = a && a.getBoundingClientRect();
    if (!inside && !away) cutEntries.push((li.textContent || '').trim().slice(0, 30));
    else if (inside && ar && (ar.right > o.right + 1 || ar.left < o.left - 1)) cutEntries.push('amount: ' + (li.textContent || '').trim().slice(0, 30));
  }
  // the rank numeral of each shown line of a banner (#1066): the drawn text is 1. 2. 3. ..., at the size of the name (12 px or more), and the name starts on the same line as its numeral
  const numeralProblems = [];
  for (const ol of document.querySelectorAll('#lsb-lcc .lcc-standard-list')) {
    const o = ol.getBoundingClientRect();
    [...ol.children].forEach((li, i) => {
      const r = li.getBoundingClientRect(); if (r.left >= o.right - 1 || r.top >= o.bottom - 1) return; // out of sight in the second column: not drawn
      const b = getComputedStyle(li, '::before'), n = li.querySelector('.lcc-standard-name'), nr = n && n.getClientRects()[0];
      const own = getComputedStyle(li), counted = own.counterIncrement.replace(/ 1$/, '') === 'lcc-n' && getComputedStyle(ol).counterReset.replace(/ 0$/, '') === 'lcc-n';
      if (b.content !== 'counter(lcc-n) "."' || !counted) numeralProblems.push((i + 1) + ': drawn as ' + b.content + ' (counter ' + own.counterIncrement + ' / ' + getComputedStyle(ol).counterReset + ')');
      else if (b.display === 'none' || b.visibility === 'hidden' || parseFloat(b.fontSize) < 12) numeralProblems.push((i + 1) + ': hidden or under 12 px (' + b.fontSize + ')');
      else if (!nr || Math.abs(nr.top - r.top) > 4) numeralProblems.push((i + 1) + ': the name does not start beside its numeral ' + (nr ? Math.round(nr.top - r.top) + ' / ' + Math.round(nr.left - r.left) : 'no box'));
    });
  }
  const standardsShown = [...document.querySelectorAll('#lsb-lcc .lcc-standard-list')].map((ol) => { const o = ol.getBoundingClientRect(); return [...ol.children].filter((li) => li.getBoundingClientRect().left < o.right - 1).length; });
  const trk = document.querySelector('#lsb-lcc .lcc-track'); const tr = trk && trk.getBoundingClientRect();
  const seatsInTrack = tr ? [...document.querySelectorAll('#lsb-lcc .lcc-seat')].filter((e) => { const r = e.getBoundingClientRect(); return r.width > 0 && r.left >= tr.left - 1 && r.right <= tr.right + 1 && r.top >= tr.top - 1 && r.bottom <= tr.bottom + 1; }).length : null;
  // a selected switch (a chip, the Grid / List and view buttons) in forced colours must keep a label that differs from its plate: the browser lays a Canvas plate behind the label of a control it adjusts, so a HighlightText label on a Highlight fill was black on black (#998)
  const selected = [];
  for (const root of roots) for (const el of root.querySelectorAll('.lcc-bd-chip[aria-pressed="true"], .lcc-bd-layout button[aria-pressed="true"], .lcc-sp-vbtn[aria-pressed="true"]')) { if (!vis(el)) continue; const cs = getComputedStyle(el); selected.push({ at: label(el), text: (el.textContent || '').trim().slice(0, 16), adjust: cs.forcedColorAdjust, color: cs.color, bg: cs.backgroundColor }); }
  const count = (sel) => document.querySelectorAll(sel).length;
  return {
    innerWidth, scrollWidth: document.documentElement.scrollWidth, roots: roots.map((r) => r.id), textNodes: nodes.n,
    small, tiny: tiny.slice(0, 8), between, selected, targets, tierOverlap, tierGapMin, clipped: clipped.slice(0, 12), ownerBad: ownerBad.slice(0, 4), over: over.slice(0, 8), cutEntries, numeralProblems, standardsShown,
    seatsInTrack, firstHouseTop: (() => { const h = document.querySelector('#lsb-lcc .lcc-hc'); return h ? Math.round(h.getBoundingClientRect().top + scrollY) : null; })(), // how far down the page the first House starts (#933)
    layout: (() => { const r = (s) => { const e = document.querySelector(s); if (!e) return null; const b = e.getBoundingClientRect(); return { top: Math.round(b.top + scrollY), h: Math.round(b.height) }; }; return { plaque: r('#lsb-lcc .lcc-plaque'), term: r('#lcc-term'), grid: r('#lsb-lcc .lcc-bd-grid'), termOpen: !!(document.querySelector('#lcc-tm-all') || {}).open }; })(),
    houses: count('#lsb-lcc [data-fk^="ht:"]'), seats: count('#lsb-lcc .lcc-seat'), centerSeats: count('#lsb-lcc .lcc-seat-center'), lanes: count('#lcs-root [data-lcs-lane]'),
    injected: !!window.__pwned || document.querySelectorAll('img[src=x]').length,
    broken: [...document.images].filter((i) => i.complete && i.naturalWidth === 0 && vis(i)).map((i) => i.getAttribute('src')),
  };
})()`;

const STATES = {
  'board-grid': { local: { tab: 'landsraad', 'lsb.view': 'board', 'lsb.layout': 'grid' }, root: '#lsb-lcc' },
  'board-list': { local: { tab: 'landsraad', 'lsb.view': 'board', 'lsb.layout': 'list' }, root: '#lsb-lcc' },
  'board-inspector': { local: { tab: 'landsraad', 'lsb.view': 'board', 'lsb.layout': 'grid' }, root: '#lsb-lcc', pre: '#lsb-lcc [data-fk^="ht:"]', after: `document.querySelector('#lsb-lcc [data-fk^="ht:"]').click()` },
  'term-section': { local: { tab: 'landsraad', 'lsb.view': 'board', 'lsb.layout': 'grid' }, root: '#lcc-term', pre: '#lcc-tm-all', after: `document.querySelector('#lcc-tm-all').open = true` }, // on a phone the section is a closed disclosure (#933): the sweep opens it, so its text and controls are measured
  'spec-cc': { local: { tab: 'player', 'spec.view': 'cc' }, root: '#lcs-root', pre: '[data-sub="player:specialization"]', after: `document.querySelector('[data-sub="player:specialization"]').click()` },
};
const WIDTHS = [320, 390, 640, 768, 1024, 1280, 1600];
const plan = [];
for (const s of Object.keys(STATES)) for (const theme of ['dark', 'light']) for (const w of WIDTHS) plan.push({ name: `${s}_${theme}_${w}`, state: s, theme, w });
for (const s of Object.keys(STATES)) for (const [w, z] of [[640, 'zoom200-of-1280'], [320, 'zoom400-of-1280']]) plan.push({ name: `${s}_dark_${z}`, state: s, theme: 'dark', w, dsf: 2 });
for (const s of Object.keys(STATES)) for (const w of [390, 1280]) { plan.push({ name: `${s}_forced-colors_${w}`, state: s, theme: 'dark', w, media: [{ name: 'forced-colors', value: 'active' }] }); plan.push({ name: `${s}_reduced-motion_${w}`, state: s, theme: 'dark', w, media: [{ name: 'prefers-reduced-motion', value: 'reduce' }] }); }
const SHOT_NAMES = /^(board-grid|board-inspector|term-section|spec-cc)_(dark|light)_(390|1280)$|^board-grid_forced-colors_1280$|^board-grid_dark_zoom200/;

async function main() {
  const browser = findBrowser();
  if (!browser) { console.error('no browser found; set LCC_BROWSER'); process.exit(2); }
  const srv = await serve();
  const base = `http://127.0.0.1:${srv.address().port}`;
  const b = await launch(browser, path.join(os.tmpdir(), 'lcc-m7-profile-' + process.pid));
  const report = { browser: path.basename(browser), cases: [], pageErrors: [], notLocal: [], failed404: [], hard: [] };
  let current = '';
  b.on((d) => {
    if (d.method === 'Runtime.exceptionThrown') report.pageErrors.push({ case: current, text: JSON.stringify(d.params.exceptionDetails).slice(0, 300) });
    else if (d.method === 'Network.requestWillBeSent') { const u = d.params.request.url; if (!u.startsWith(base + '/') && !u.startsWith('about:') && !u.startsWith('data:')) report.notLocal.push({ case: current, url: u }); }
    else if (d.method === 'Network.responseReceived' && d.params.response.status >= 400 && !/\/api\//.test(d.params.response.url)) report.failed404.push({ case: current, url: d.params.response.url, status: d.params.response.status });
  });
  for (const dom of ['Runtime', 'Page', 'Network']) await b.send(dom + '.enable');
  const evalIn = async (expression) => { const r = await b.send('Runtime.evaluate', { expression, returnByValue: true, awaitPromise: true }); if (r.exceptionDetails) throw new Error(JSON.stringify(r.exceptionDetails).slice(0, 300)); return r.result.value; };
  for (const c of plan) {
    if (only && !c.name.includes(only)) continue;
    current = c.name;
    const st = STATES[c.state];
    const touch = c.w <= 900;
    await b.send('Emulation.setDeviceMetricsOverride', { width: c.w, height: 900, deviceScaleFactor: c.dsf || 1, mobile: false });
    await b.send('Emulation.setTouchEmulationEnabled', { enabled: touch, maxTouchPoints: touch ? 5 : 1 });
    await b.send('Emulation.setEmulatedMedia', { media: '', features: [{ name: 'prefers-color-scheme', value: c.theme }, ...(c.media || [])] });
    const seed = JSON.stringify({ ...st.local, tauTheme: c.theme });
    await b.send('Page.navigate', { url: base + '/design/tokens.css' });
    await sleep(150);
    await evalIn(`localStorage.clear(); const s = ${seed}; for (const k in s) localStorage.setItem(k, s[k]); true`);
    await b.send('Page.navigate', { url: base + '/?c=' + encodeURIComponent(c.name) });
    const until = async (expr, tries = 100) => { for (let i = 0; i < tries; i++) { await sleep(150); if (await evalIn(expr).catch(() => false)) return true; } return false; };
    if (st.pre && (await until('!!document.querySelector(' + JSON.stringify(st.pre) + ')'))) { await evalIn(st.after).catch(() => null); await sleep(500); }
    const ready = await until('(() => { const r = document.querySelector(' + JSON.stringify(st.root) + '); return !!r && r.children.length > 0 && !document.querySelector("#loading"); })()');
    const m = ready ? await evalIn(MEASURE) : null;
    const coarse = await evalIn("matchMedia('(pointer: coarse)').matches").catch(() => null);
    const posted = posts.splice(0).length;
    const row = { name: c.name, ready, posted, coarse };
    if (m) {
      const floor = c.w <= 640 ? 44 : 36; // the agreed floors: 44 px at phone widths (640 and below), 36 px for touch above that and for a mouse (#906)
      const floorPx = floor;
      const smallBySel = {};
      for (const t of m.targets) if (t.h < floorPx || t.w < 24) { const k = (smallBySel[t.at] ||= { n: 0, minW: 999, minH: 999 }); k.n++; k.minW = Math.min(k.minW, t.w); k.minH = Math.min(k.minH, t.h); }
      Object.assign(row, { innerWidth: m.innerWidth, pageWiderBy: Math.max(0, m.scrollWidth - m.innerWidth), textNodes: m.textNodes, under11: m.small, tiny: m.tiny, nameBand11to12: m.between, controls: m.targets.length, under36: m.targets.filter((t) => t.h < 36).length, under44: m.targets.filter((t) => t.h < 44).length, floor, smallBySel, tierOverlap: m.tierOverlap, tierGapMin: m.tierGapMin, tierHit: (() => { const t = m.targets.filter((x) => x.tier); return t.length ? { n: t.length, hitW: Math.min(...t.map((x) => x.w)), hitH: Math.min(...t.map((x) => x.h)), visW: Math.min(...t.map((x) => x.vw)), visH: Math.min(...t.map((x) => x.vh)) } : null; })(), seatsInTrack: m.seatsInTrack, clipped: m.clipped, cutEntries: m.cutEntries, numeralProblems: m.numeralProblems, standardsShown: m.standardsShown, over: m.over, houses: m.houses, seats: m.seats, centerSeats: m.centerSeats, firstHouseTop: m.firstHouseTop, layout: m.layout, lanes: m.lanes, injected: m.injected, broken: m.broken });
      if (m.scrollWidth > m.innerWidth + 1) report.hard.push(c.name + ': the page is wider than the window by ' + (m.scrollWidth - m.innerWidth) + ' px');
      if (Object.keys(m.small).length) report.hard.push(c.name + ': text under 11 px in ' + Object.keys(m.small).join(', '));
      if ((c.media || []).some((x) => x.name === 'forced-colors')) for (const x of m.selected) if (x.adjust !== 'none') report.hard.push(c.name + ': a selected control is left to the automatic forced-colours adjustment (' + x.at + ' ' + x.text + ')'); else if (x.color === x.bg) report.hard.push(c.name + ': a selected control has the same label and plate colour (' + x.at + ' ' + x.text + ')');
      if (touch && Object.keys(smallBySel).length) report.hard.push(c.name + ': controls under the touch floor (' + floor + ' px high, 24 px wide): ' + Object.entries(smallBySel).map(([k, v]) => k + ' x' + v.n + ' ' + v.minW + 'x' + v.minH).join(', '));
      if (m.ownerBad.length) report.hard.push(c.name + ': the owner line of a House card touches or crosses the card edge: ' + JSON.stringify(m.ownerBad));
      if (m.tierOverlap) report.hard.push(c.name + ': ' + m.tierOverlap + ' tier boxes whose hit areas overlap their neighbour');
      if (m.cutEntries.length) report.hard.push(c.name + ': a contributor line of a banner is cut: ' + m.cutEntries.join(' | '));
      if (m.numeralProblems.length) report.hard.push(c.name + ': a banner rank numeral is wrong: ' + m.numeralProblems.join(' | '));
      if (m.standardsShown.some((n) => n < 1)) report.hard.push(c.name + ': a banner shows no contributor line though it has some');
      if (m.injected) report.hard.push(c.name + ': text from the save became markup or ran');
      if (m.broken.length) report.hard.push(c.name + ': a picture did not load: ' + m.broken.join(', '));
      if (c.state.startsWith('board-') && c.state !== 'board-list' && m.houses !== 25) report.hard.push(c.name + ': ' + m.houses + ' Houses drawn, not 25');
      if (c.state.startsWith('board-') && m.seatsInTrack !== null && m.seatsInTrack !== 25) report.hard.push(c.name + ': ' + m.seatsInTrack + ' of 25 seats inside the track');
      if (c.state === 'spec-cc' && m.lanes !== 5) report.hard.push(c.name + ': ' + m.lanes + ' lanes, not 5');
    } else report.hard.push(c.name + ': the page did not draw');
    if (allShots || SHOT_NAMES.test(c.name)) {
      if (scrollTo) { await evalIn(`(() => { const n = document.querySelector(${JSON.stringify(scrollTo)}); if (n) n.scrollIntoView({ block: 'start' }); return !!n; })()`).catch(() => null); await sleep(300); }
      const shot = await b.send('Page.captureScreenshot', { format: 'png', captureBeyondViewport: false });
      fs.writeFileSync(path.join(out, c.name + '.png'), Buffer.from(shot.data, 'base64'));
      row.picture = c.name + '.png';
    }
    report.cases.push(row);
    process.stdout.write('.');
  }
  // The tap of a tier box on a House card (#880): on a touch screen it chooses the House and writes nothing (no request, the panel of the House opens); with a mouse the click still writes (a request, or the question
  // that comes before it). The touch is a real touch event of the browser engine, the click a real mouse press.
  if (!only || 'tap-tier'.includes(only) || only.includes('tap')) {
    for (const mode of ['touch', 'mouse']) {
      current = 'tap-tier_' + mode;
      const w = mode === 'touch' ? 390 : 1280;
      await b.send('Emulation.setDeviceMetricsOverride', { width: w, height: 900, deviceScaleFactor: 1, mobile: false });
      await b.send('Emulation.setTouchEmulationEnabled', { enabled: mode === 'touch', maxTouchPoints: mode === 'touch' ? 5 : 1 });
      await b.send('Emulation.setEmulatedMedia', { media: '', features: [{ name: 'prefers-color-scheme', value: 'dark' }] });
      await b.send('Page.navigate', { url: base + '/design/tokens.css' });
      await sleep(150);
      await evalIn(`localStorage.clear(); const s = ${JSON.stringify({ ...STATES['board-grid'].local, tauTheme: 'dark' })}; for (const k in s) localStorage.setItem(k, s[k]); true`);
      await b.send('Page.navigate', { url: base + '/?c=tap-' + mode });
      let ok = false;
      for (let i = 0; i < 100 && !ok; i++) { await sleep(150); ok = await evalIn('!!document.querySelector("#lsb-lcc .lcc-hc-t[data-lb=tier]:not([aria-disabled=true])") && !document.querySelector("#loading")').catch(() => false); }
      if (!ok) { report.hard.push(current + ': the board did not draw'); continue; }
      posts.splice(0);
      const pt = await evalIn(`(() => { const n = document.querySelector("#lsb-lcc .lcc-hc-t[data-lb=tier]:not([aria-disabled=true])"); n.scrollIntoView({ block: 'center' }); const r = n.getBoundingClientRect(); return { x: r.left + r.width / 2, y: r.top + r.height / 2, coarse: matchMedia('(pointer: coarse)').matches, id: n.getAttribute('data-id') }; })()`);
      await sleep(200);
      if (mode === 'touch') {
        await b.send('Input.dispatchTouchEvent', { type: 'touchStart', touchPoints: [{ x: pt.x, y: pt.y }] });
        await b.send('Input.dispatchTouchEvent', { type: 'touchEnd', touchPoints: [] });
      } else {
        await b.send('Input.dispatchMouseEvent', { type: 'mousePressed', x: pt.x, y: pt.y, button: 'left', clickCount: 1 });
        await b.send('Input.dispatchMouseEvent', { type: 'mouseReleased', x: pt.x, y: pt.y, button: 'left', clickCount: 1 });
      }
      await sleep(900);
      const after = await evalIn(`({ insp: (document.querySelector('.lcc-bd-insp') || {}).dataset ? document.querySelector('.lcc-bd-insp').dataset.id : null, asked: !!document.querySelector('[data-fk="c-yes"]'), queued: (document.querySelector('#lsb-result') || {}).textContent || '' })`);
      const wrote = posts.length;
      const row = { name: current, coarse: pt.coarse, house: pt.id, panelOf: after.insp, postsSent: wrote, asked: after.asked };
      report.cases.push(row);
      if (mode === 'touch' && (!pt.coarse || wrote || after.asked || after.insp !== pt.id)) report.hard.push(current + ': a tap of a tier box must choose the House and write nothing: ' + JSON.stringify(row));
      if (mode === 'mouse' && (pt.coarse || (!wrote && !after.asked))) report.hard.push(current + ': a mouse click on a tier box must still write (or ask first): ' + JSON.stringify(row));
      process.stdout.write('t');
    }
  }
  // The result line and the question of a click scroll into view clear of the sticky header and of the banner under it, however tall that banner is (#823). The Classic list is scrolled to its last House,
  // the banner is made 220 px tall (a long list of queued changes does that), and lbReveal brings the result line back: it must land below the bottom of the banner, not under it.
  if (!only || only.includes('reveal')) {
    current = 'reveal-tall-banner';
    await b.send('Emulation.setDeviceMetricsOverride', { width: 1280, height: 900, deviceScaleFactor: 1, mobile: false });
    await b.send('Emulation.setTouchEmulationEnabled', { enabled: false, maxTouchPoints: 1 });
    await b.send('Page.navigate', { url: base + '/design/tokens.css' });
    await sleep(150);
    await evalIn(`localStorage.clear(); const s = ${JSON.stringify({ tab: 'landsraad', 'lsb.view': 'classic', tauTheme: 'dark' })}; for (const k in s) localStorage.setItem(k, s[k]); true`);
    await b.send('Page.navigate', { url: base + '/?c=reveal' });
    let ok = false;
    for (let i = 0; i < 100 && !ok; i++) { await sleep(150); ok = await evalIn('!!document.querySelector("#lsb-rows .hr") && !document.querySelector("#loading")').catch(() => false); }
    if (!ok) report.hard.push(current + ': the Classic list did not draw');
    else {
      const r = await evalIn(`(() => { const gw = document.querySelector('#gamewarn'); gw.style.minHeight = '220px'; document.querySelector('#lsb-rows .hr:last-child').scrollIntoView({ block: 'end' });
        lbReveal(document.querySelector('#lsb-result')); const bottom = (s) => { const e = document.querySelector(s); return e ? Math.round(e.getBoundingClientRect().bottom) : null; };
        const gs = getComputedStyle(gw); return new Promise((res) => setTimeout(() => res({ banner: bottom('#gamewarn'), sticky: gs.position === 'sticky', result: Math.round(document.querySelector('#lsb-result').getBoundingClientRect().top) }), 600)); })()`);
      report.cases.push({ name: current, ...r });
      if (!r.sticky) report.hard.push(current + ': the banner is not sticky here, so the check proves nothing: ' + JSON.stringify(r));
      else if (r.result < r.banner - 1) report.hard.push(current + ': the result line is under the banner: ' + JSON.stringify(r));
      process.stdout.write('r');
    }
  }
  // A place link of a House opens the Saved map scrolled to the map, with the House named, whatever offset the board had (#1025). The board is scrolled to its end, the link of a mapped House is pressed with a
  // real mouse press, and the Saved map must have its canvas inside the window (not scrolled past) and the line that names the House in view.
  if (!only || only.includes('maplink')) {
    const GRID_PRE = `(() => { const id = document.querySelector(".lcc-bd-cell .hcard-place:not(.off)").dataset.id; document.querySelector('[data-fk="ht:' + id + '"]').click(); return id; })()`; // the grid: a mapped House is chosen, and its inspector has the link
    for (const [view, layout, link, w, h, pre] of [['classic', 'grid', '#lsb-rows .loc-t[data-lb=map]:not(.off)', 1280, 800], ['board', 'list', 'td.lcc-bd-loc .lcc-bd-linkb[data-lb=map]', 1280, 800], ['board', 'grid', '.lcc-bd-insp .lcc-bd-act[data-lb=map]', 1280, 800, GRID_PRE], ['board', 'list', 'td.lcc-bd-loc .lcc-bd-linkb[data-lb=map]', 390, 800]]) {
      current = `maplink-${view}-${layout}-${w}`;
      await b.send('Emulation.setDeviceMetricsOverride', { width: w, height: h, deviceScaleFactor: 1, mobile: false });
      await b.send('Emulation.setTouchEmulationEnabled', { enabled: false, maxTouchPoints: 1 });
      await b.send('Page.navigate', { url: base + '/design/tokens.css' });
      await sleep(150);
      await evalIn(`localStorage.clear(); const s = ${JSON.stringify({ tab: 'landsraad', 'lsb.view': view, 'lsb.layout': layout, tauTheme: 'dark' })}; for (const k in s) localStorage.setItem(k, s[k]); true`);
      await b.send('Page.navigate', { url: base + '/?c=' + current });
      let ok = false;
      for (let i = 0; i < 100 && !ok; i++) { await sleep(150); ok = await evalIn('!!document.querySelector(' + JSON.stringify(pre ? '.lcc-bd-cell .hcard-place:not(.off)' : link) + ') && !document.querySelector("#loading")').catch(() => false); }
      if (!ok) { report.hard.push(current + ': no place link on the page'); continue; }
      await sleep(400);
      if (pre) { await evalIn(pre); await sleep(500); }
      const pt = await evalIn(`(() => { window.scrollTo(0, 1e6); const n = document.querySelector(${JSON.stringify(link)}); n.scrollIntoView({ block: 'center' }); const r = n.getBoundingClientRect(); return { x: r.left + r.width / 2, y: r.top + r.height / 2, y0: Math.round(scrollY) }; })()`);
      await sleep(200);
      await b.send('Input.dispatchMouseEvent', { type: 'mouseMoved', x: pt.x, y: pt.y });
      await sleep(150);
      await b.send('Input.dispatchMouseEvent', { type: 'mousePressed', x: pt.x, y: pt.y, button: 'left', clickCount: 1 });
      await b.send('Input.dispatchMouseEvent', { type: 'mouseReleased', x: pt.x, y: pt.y, button: 'left', clickCount: 1 });
      let drawn = false;
      for (let i = 0; i < 60 && !drawn; i++) { await sleep(150); drawn = await evalIn('!!document.querySelector("#lmc")').catch(() => false); }
      await sleep(500);
      const r = await evalIn(`(() => { const c = document.querySelector('#lmc'), l = document.querySelector('#lmhouse'); const cb = c && c.getBoundingClientRect(), lb = l && l.getBoundingClientRect(); const hd = document.querySelector('header'); const top = Math.max(hd ? hd.getBoundingClientRect().bottom : 0, 0);
        return { scrollY: Math.round(scrollY), docH: document.documentElement.scrollHeight, canvasTop: cb && Math.round(cb.top), canvasBottom: cb && Math.round(cb.bottom), vh: innerHeight, headerBottom: Math.round(top), line: lb ? l.textContent.trim().slice(0, 40) : null, lineTop: lb && Math.round(lb.top), focus: document.activeElement && document.activeElement.id }; })()`);
      report.cases.push({ name: current, from: pt.y0, ...r });
      if (!drawn) report.hard.push(current + ': the Saved map did not open');
      else if (!r.line) report.hard.push(current + ': the Saved map does not name the House: ' + JSON.stringify(r));
      else if (r.canvasTop < r.headerBottom - 1 || r.canvasTop > r.vh - 100) report.hard.push(current + ': the map is not in view (it starts at ' + r.canvasTop + ' of a window ' + r.vh + ' high, the header ends at ' + r.headerBottom + '): ' + JSON.stringify(r));
      else if (r.focus !== 'lmc') report.hard.push(current + ': the keyboard focus is not on the map: ' + r.focus);
      if (/(classic|board)-(grid|list)-1280/.test(current) && !allShots) { const shot = await b.send('Page.captureScreenshot', { format: 'png' }); fs.writeFileSync(path.join(out, current + '.png'), Buffer.from(shot.data, 'base64')); }
      // and the way back: "Back to House X" on that Saved map opens the Landsraad tab with the House chosen and its card, row or list line in view below the sticky header, not at the top of the page (#1072)
      if (drawn && r.line) {
        const bk = await evalIn(`(() => { const k = document.querySelector('#lmhouse [data-lmback]'); if (!k) return null; k.scrollIntoView({ block: 'center' }); const q = k.getBoundingClientRect(); return { x: Math.round(q.left + q.width / 2), y: Math.round(q.top + q.height / 2), id: k.dataset.lmback }; })()`);
        if (!bk) report.hard.push(current + ': the Saved map has no way back to the House');
        else {
          await b.send('Input.dispatchMouseEvent', { type: 'mouseMoved', x: bk.x, y: bk.y });
          await b.send('Input.dispatchMouseEvent', { type: 'mousePressed', x: bk.x, y: bk.y, button: 'left', clickCount: 1 });
          await b.send('Input.dispatchMouseEvent', { type: 'mouseReleased', x: bk.x, y: bk.y, button: 'left', clickCount: 1 });
          let back = null;
          for (let i = 0; i < 40 && !back; i++) { await sleep(150); back = await evalIn(`(() => document.querySelector('#lsb') ? 1 : 0)()`).catch(() => 0); }
          await sleep(700);
          const q = await evalIn(`(() => { const id = ${JSON.stringify(bk.id)}; const n = document.querySelector('#lsb-lcc') ? (document.querySelector('.lcc-bd-cell[data-id="' + id + '"]') || document.querySelector('.lcc-bd-row[data-id="' + id + '"]')) : document.querySelector('#lsrow-' + id); let hb = 0; document.querySelectorAll('header, #gamewarn, #launchpass').forEach((e) => { hb = Math.max(hb, e.getBoundingClientRect().bottom); });
            const r = n && n.getBoundingClientRect(); const insp = document.querySelector('.lcc-bd-insp'); return { scrollY: Math.round(scrollY), vh: innerHeight, headerBottom: Math.round(hb), found: !!n, top: r && Math.round(r.top), bottom: r && Math.round(r.bottom), board: !!document.querySelector('#lsb-lcc'), inspector: insp ? insp.getAttribute('data-id') : null, focus: document.activeElement && (document.activeElement.getAttribute('data-fk') || document.activeElement.className) }; })()`);
          report.cases.push({ name: current + '-back', ...q });
          const seen = q.found && q.bottom > q.headerBottom && q.top < q.vh;
          if (!seen) report.hard.push(current + ': Back to House did not bring the House into view: ' + JSON.stringify(q));
          else if (q.board && q.inspector !== bk.id) report.hard.push(current + ': Back to House did not choose the House on the board: ' + JSON.stringify(q));
          if (/(classic|board)-(grid|list)-1280/.test(current) && !allShots) { const shot = await b.send('Page.captureScreenshot', { format: 'png' }); fs.writeFileSync(path.join(out, current + '-back.png'), Buffer.from(shot.data, 'base64')); }
        }
      }
      process.stdout.write('m');
    }
  }
  await b.close(); srv.close();
  if (report.pageErrors.length) report.hard.push(report.pageErrors.length + ' page errors');
  if (report.notLocal.length) report.hard.push(report.notLocal.length + ' requests to an address that is not this one');
  fs.writeFileSync(path.join(out, 'sweep.json'), JSON.stringify(report, null, 1));
  console.log('\n' + report.cases.length + ' cases, ' + report.hard.length + ' hard failures; pageErrors ' + report.pageErrors.length + ', not-local ' + report.notLocal.length + ', 404 files ' + report.failed404.length + '. Report: ' + path.join(out, 'sweep.json'));
  for (const h of report.hard.slice(0, 40)) console.log(' - ' + h);
  process.exit(report.hard.length ? 1 : 0);
}
main().catch((e) => { console.error(e); process.exit(2); });
