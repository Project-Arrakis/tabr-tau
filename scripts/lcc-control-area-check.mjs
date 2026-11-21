// Real-layout check of the Command Center control area (tau #974) in a headless Chromium-family browser. jsdom has no CSS, so what only a layout engine can say is measured here: after Cancel of a
// question the action keys are shown and the focus is on a Fill or More key (not on the page), the question is not clipped, the Show tabs are off while Houses are chosen, and Clear keeps its word at the
// panel width of the design. Same set-up as scripts/lcc-m7-sweep.mjs: the editor's static files over loopback, the synthetic Landsraad answer, the generated UI fixtures; every POST is answered with a bare
// "ok" and counted, nothing is written and no window is ever shown.
//
//   node scripts/lcc-control-area-check.mjs
//
// Needs Node 22 or later and a Chromium-family browser (LCC_BROWSER names one). The exit status is 1 when a check fails. The browser it starts is the only process it stops (by its own id).
import http from 'node:http';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { spawn } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const repo = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const staticDir = path.join(repo, 'internal', 'web', 'static');
const designDir = path.join(repo, 'design', 'assets');
const fixDir = path.join(repo, 'docs', 'design', 'landsraad-command-center', 'fixtures');
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
const clean = (o) => JSON.parse(JSON.stringify(o, (k, v) => (k.startsWith('_') ? undefined : v)));
const LANDSRAAD = clean(JSON.parse(fs.readFileSync(path.join(fixDir, 'api-landsraad.synthetic.json'), 'utf8')));
const SPECS = JSON.parse(fs.readFileSync(path.join(fixDir, 'api-player-specs.cases.json'), 'utf8')).cases.find((c) => c.id === 'mixed-five-tracks').response;
const FIX = process.env.UI_FIXTURES_FILE || path.join(process.env.UI_FIXTURES_DIR || path.join(repo, 'web-tests', 'fixtures'), 'ui-fixtures.json');
if (!fs.existsSync(FIX)) { console.error('UI fixtures not found at ' + FIX + '. Generate them first: UI_FIXTURES_DIR=web-tests/fixtures go test ./internal/web -run TestDumpUIFixtures'); process.exit(2); }
const FIXTURES = JSON.parse(fs.readFileSync(FIX, 'utf8'));
const types = { '.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css', '.webp': 'image/webp', '.svg': 'image/svg+xml', '.png': 'image/png', '.ttf': 'font/ttf', '.woff2': 'font/woff2', '.json': 'application/json' };
const findBrowser = () => [process.env.LCC_BROWSER, 'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe', 'C:/Program Files/Microsoft/Edge/Application/msedge.exe', 'C:/Program Files/Google/Chrome/Application/chrome.exe', '/usr/bin/chromium', '/usr/bin/chromium-browser', '/usr/bin/google-chrome'].find((p) => p && fs.existsSync(p));

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
  console.log('browser started by this script: pid ' + proc.pid + ', profile ' + profile);
  const f = path.join(profile, 'DevToolsActivePort');
  for (let i = 0; i < 150 && !fs.existsSync(f); i++) await sleep(100);
  if (!fs.existsSync(f)) { proc.kill(); throw new Error('the browser did not start'); }
  const [port] = fs.readFileSync(f, 'utf8').split('\n');
  const page = (await (await fetch(`http://127.0.0.1:${port}/json/list`)).json()).find((t) => t.type === 'page');
  const ws = new WebSocket(page.webSocketDebuggerUrl);
  await new Promise((r, j) => { ws.onopen = r; ws.onerror = j; });
  let id = 0; const pending = new Map();
  ws.onmessage = (m) => { const d = JSON.parse(m.data); if (d.id && pending.has(d.id)) { const { res, rej } = pending.get(d.id); pending.delete(d.id); d.error ? rej(new Error(JSON.stringify(d.error))) : res(d.result); } };
  const send = (method, params = {}) => new Promise((res, rej) => { const i = ++id; pending.set(i, { res, rej }); ws.send(JSON.stringify({ id: i, method, params })); });
  return { send, close: async () => { try { await send('Browser.close'); } catch (e) { /* closing */ } try { ws.close(); } catch (e) { /* closed */ } setTimeout(() => proc.kill(), 1500); } };
}

// what runs in the page: small helpers, then one scenario that returns what it saw
const HELP = `
  const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
  const shown = (el) => { if (!el) return false; const r = el.getBoundingClientRect(), cs = getComputedStyle(el); return r.width > 0 && r.height > 0 && cs.display !== 'none' && cs.visibility !== 'hidden'; };
  const panel = () => document.querySelector('.lcc-bd-panel');
  const fk = () => (document.activeElement && document.activeElement.getAttribute && document.activeElement.getAttribute('data-fk')) || (document.activeElement ? document.activeElement.tagName : '');
  const keysShown = () => Object.fromEntries(['fill', 'complete', 'reopen'].map((d) => [d, shown(document.querySelector('.lcc-k-cmd [data-do="' + d + '"]'))]).concat([['clear', shown(document.querySelector('.lcc-k-cmd [data-lb="pickno"]'))], ['find', shown(document.querySelector('.lcc-k-cmd [data-lk="findbtn"]'))], ['count', shown(document.querySelector('.lcc-k-cmd [data-lk="n"]'))]]));
  const tick = async (n) => { const bs = [...document.querySelectorAll('.lcc-bd-pick')].filter((b) => !b.disabled && !b.checked).slice(0, n); for (const b of bs) { b.click(); await sleep(30); } await sleep(150); };
  const press = async (sel) => { const e = document.querySelector(sel); if (!e) throw new Error('no ' + sel); e.focus(); e.click(); await sleep(250); };
`;
const SCENARIO = `(async () => {
  ${HELP}
  const out = { w: innerWidth, panelW: Math.round(panel().getBoundingClientRect().width) };
  await tick(6);
  out.mode = panel().dataset.mode; out.state = panel().dataset.state;
  out.keys = keysShown();
  out.cmdH = Math.round(document.querySelector('.lcc-k-cmd').getBoundingClientRect().height);
  out.rects = [...document.querySelectorAll('.lcc-k-cmd .lcc-k-zone > *, .lcc-k-cmd > *:not(.lcc-bd-barslot), .lcc-k-cmd .barpart > *')].filter((e) => shown(e)).map((e) => { const r = e.getBoundingClientRect(); return (e.getAttribute('data-fk') || e.className.toString().split(' ')[0]) + ':' + Math.round(r.left) + '-' + Math.round(r.right) + '@' + Math.round(r.top); });
  const clr = document.querySelector('.lcc-k-cmd [data-lb="pickno"]'); const word = clr && [...clr.querySelectorAll(':scope > span')].find((s) => /^Clear$/.test(s.textContent.trim()));
  out.clearWord = !!word && shown(word) && word.getBoundingClientRect().width > 8; out.clearW = clr ? Math.round(clr.getBoundingClientRect().width) : 0;
  out.showOff = [...document.querySelectorAll('.lcc-k-tab[data-key="show"]')].map((t) => t.getAttribute('aria-disabled'));
  // Fill, then the question: nothing of it is cut off
  await press('.lcc-k-cmd [data-do="fill"]');
  const qd = document.querySelector('.lcc-k-ask .lcc-k-qd');
  out.ask = { state: panel().dataset.state, text: qd ? qd.textContent.trim().slice(0, 400) : null, clipped: qd ? qd.scrollHeight > qd.clientHeight + 1 : null, clamp: qd ? getComputedStyle(qd).webkitLineClamp : null, h: qd ? Math.round(qd.getBoundingClientRect().height) : 0, stripH: Math.round(document.querySelector('.lcc-k-strip').getBoundingClientRect().height), buttons: ['cyes', 'cno'].map((k) => shown(document.querySelector('.lcc-k-ask [data-lb="' + k + '"]'))), focus: fk() };
  // Cancel: the action keys are back and the focus is on Fill
  await press('.lcc-k-ask [data-lb="cno"]');
  out.afterCancel = { state: panel().dataset.state, keys: keysShown(), focus: fk(), cmdH: Math.round(document.querySelector('.lcc-k-cmd').getBoundingClientRect().height) };
  // Complete and fill (under More), then Cancel: the focus is on the More key
  await press('.lcc-k-cmd [data-lk="more"]');
  out.moreOpen = shown(document.querySelector('.lcc-k-moremenu')); out.moreExpanded = document.querySelector('.lcc-k-cmd [data-lk="more"]').getAttribute('aria-expanded');
  await press('.lcc-k-moremenu [data-do="cfill"]');
  out.cfillAsk = { state: panel().dataset.state, moreClosed: !shown(document.querySelector('.lcc-k-moremenu')) };
  await press('.lcc-k-ask [data-lb="cno"]');
  out.cfillCancel = { state: panel().dataset.state, keys: keysShown(), focus: fk() };
  // Escape on a question
  await press('.lcc-k-cmd [data-do="complete"]');
  document.activeElement.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true })); await sleep(250);
  out.escape = { state: panel().dataset.state, keys: keysShown(), focus: fk() };
  // a Show tab does nothing while Houses are chosen
  const before = document.querySelectorAll('.lcc-bd-pick:checked').length;
  await press('.lcc-k-tab[data-key="show"][aria-checked="false"]');
  out.showClick = { before, after: document.querySelectorAll('.lcc-bd-pick:checked').length };
  await press('.lcc-k-cmd [data-lb="pickno"]');
  out.cleared = { mode: panel().dataset.mode, showOn: [...document.querySelectorAll('.lcc-k-tab[data-key="show"]')].map((t) => t.getAttribute('aria-disabled')) };
  return out;
})()`;

const fails = [];
const must = (ok, what) => { if (!ok) fails.push(what); };
async function main() {
  const browser = findBrowser();
  if (!browser) { console.error('no browser found; set LCC_BROWSER'); process.exit(2); }
  const srv = await serve();
  const base = `http://127.0.0.1:${srv.address().port}`;
  const b = await launch(browser, path.join(os.tmpdir(), 'lcc-ca-check-profile-' + process.pid));
  for (const dom of ['Runtime', 'Page']) await b.send(dom + '.enable');
  const evalIn = async (expression) => { const r = await b.send('Runtime.evaluate', { expression, returnByValue: true, awaitPromise: true }); if (r.exceptionDetails) throw new Error(JSON.stringify(r.exceptionDetails).slice(0, 400)); return r.result.value; };
  const results = [];
  try {
    for (const w of [1280, 1024, 390]) {
      const touch = w <= 900;
      await b.send('Emulation.setDeviceMetricsOverride', { width: w, height: 900, deviceScaleFactor: 1, mobile: false });
      await b.send('Emulation.setTouchEmulationEnabled', { enabled: touch, maxTouchPoints: touch ? 5 : 1 });
      await b.send('Page.navigate', { url: base + '/design/tokens.css' });
      await sleep(150);
      await evalIn(`localStorage.clear(); const s = { tab: 'landsraad', 'lsb.view': 'board', 'lsb.layout': 'grid', tauTheme: 'dark' }; for (const k in s) localStorage.setItem(k, s[k]); true`);
      await b.send('Page.navigate', { url: base + '/?w=' + w });
      let ready = false;
      for (let i = 0; i < 100 && !ready; i++) { await sleep(150); ready = await evalIn(`!!document.querySelector('.lcc-bd-panel .lcc-bd-pick') && !document.querySelector('#loading')`).catch(() => false); }
      must(ready, w + ': the board did not draw');
      if (!ready) continue;
      const o = await evalIn(SCENARIO);
      results.push(o);
      const t = w + ' px (panel ' + o.panelW + ' px): ';
      must(o.mode === 'action' && o.state === 'chosen', t + 'the strip is the action bar with Houses chosen');
      if (w > 700) must(o.cmdH <= 104, t + 'the action bar is one row (height ' + o.cmdH + ')');
      must(Object.values(o.keys).every(Boolean), t + 'the action keys are shown: ' + JSON.stringify(o.keys));
      if (o.panelW >= 800 && w > 700) must(o.clearWord, t + 'Clear keeps its word (width ' + o.clearW + ')');
      must(o.showOff.length > 0 && o.showOff.every((a) => a === 'true'), t + 'the Show tabs are off while Houses are chosen: ' + JSON.stringify(o.showOff));
      must(o.ask.state === 'confirm' && o.ask.text, t + 'Fill asks a question');
      must(o.ask.text && /Houses:/.test(o.ask.text) && /backup first/.test(o.ask.text), t + 'the question shows the Houses and the backup sentence: ' + o.ask.text);
      must(o.ask.clipped === false && o.ask.clamp === 'none', t + 'the question is not clipped (clipped ' + o.ask.clipped + ', clamp ' + o.ask.clamp + ')');
      must(o.ask.buttons.every(Boolean), t + 'Yes and Cancel are shown');
      must(o.afterCancel.state === 'chosen' && Object.values(o.afterCancel.keys).every(Boolean), t + 'after Cancel the action keys are shown: ' + JSON.stringify(o.afterCancel));
      must(o.afterCancel.focus === 'b-fill', t + 'after Cancel the focus is on Fill, it is on ' + o.afterCancel.focus);
      must(o.moreOpen && o.moreExpanded === 'true', t + 'More opens');
      must(o.cfillAsk.state === 'confirm' && o.cfillAsk.moreClosed, t + 'Complete and fill asks and More is closed');
      must(o.cfillCancel.state === 'chosen' && Object.values(o.cfillCancel.keys).every(Boolean) && o.cfillCancel.focus === 'b-more', t + 'after Cancel of Complete and fill the keys are shown and the focus is on More: ' + JSON.stringify(o.cfillCancel));
      must(o.escape.state === 'chosen' && Object.values(o.escape.keys).every(Boolean) && o.escape.focus === 'b-complete', t + 'Escape on a question gives the keys back and the focus to the key that asked (Complete): ' + JSON.stringify(o.escape));
      must(o.showClick.before === o.showClick.after, t + 'a Show tab changes nothing while Houses are chosen');
      must(o.cleared.mode === 'filters' && o.cleared.showOn.every((a) => a === null), t + 'Clear brings the filter row and the Show tabs back');
    }
    must(posts.length === 0, 'something was written: ' + posts.join(' '));
  } finally { await b.close(); srv.close(); }
  if (process.env.LCC_CA_DUMP) for (const r of results) console.log(r.w, JSON.stringify(r.rects));
  for (const r of results) console.log(JSON.stringify({ w: r.w, panelW: r.panelW, cmdH: r.cmdH, askH: r.ask.h, stripH: r.ask.stripH, clearW: r.clearW, clearWord: r.clearWord, cancelFocus: r.afterCancel.focus, moreFocus: r.cfillCancel.focus }));
  console.log(fails.length ? fails.length + ' check(s) failed:\n - ' + fails.join('\n - ') : 'all checks passed');
  process.exit(fails.length ? 1 : 0);
}
main().catch((e) => { console.error(e); process.exit(1); });
