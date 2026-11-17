// Headless captures of the Command Center component gallery (milestone M2). Fixture data only; the editor is not involved.
//
//   node scripts/lcc-gallery-shots.mjs [outDir]
//
// Needs Node 22 or later (the built-in WebSocket) and a Chromium-family browser. It starts the browser itself in headless mode (no window is ever shown), serves the repository over a
// loopback address with the gallery's own content security policy, drives the page over the DevTools protocol, and writes PNG files plus a report.
//   LCC_BROWSER   path of the browser program (default: the usual Edge or Chrome install places)
//   LCC_REF_IMAGE path of the approved reference picture, to build the side-by-side crops (optional; without it those are not made, and the report says so)
//   LCC_SHOTS_ONLY  a substring: only the captures whose name contains it
// The output folder is the argument, else LCC_SHOTS_OUT, else a folder in the system temporary directory. Nothing is written inside the repository.
import http from 'node:http';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { spawn } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const repo = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const out = path.resolve(process.argv[2] || process.env.LCC_SHOTS_OUT || path.join(os.tmpdir(), 'lcc-shots'));
fs.mkdirSync(out, { recursive: true });
const only = process.env.LCC_SHOTS_ONLY || '';
const refImage = process.env.LCC_REF_IMAGE && fs.existsSync(process.env.LCC_REF_IMAGE) ? path.resolve(process.env.LCC_REF_IMAGE) : '';
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
const CSP = "default-src 'none'; script-src 'self'; style-src 'self'; img-src 'self'; font-src 'self'";
const GALLERY = '/docs/design/landsraad-command-center/gallery/index.html';

function findBrowser() {
  const c = [process.env.LCC_BROWSER,
    'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe', 'C:/Program Files/Microsoft/Edge/Application/msedge.exe',
    'C:/Program Files/Google/Chrome/Application/chrome.exe', 'C:/Program Files (x86)/Google/Chrome/Application/chrome.exe',
    '/usr/bin/chromium', '/usr/bin/chromium-browser', '/usr/bin/google-chrome', '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome'];
  return c.find((p) => p && fs.existsSync(p));
}

// ---- the static server: the repository only, GET only, the gallery's policy on every answer ----
const types = { '.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css', '.webp': 'image/webp', '.svg': 'image/svg+xml', '.png': 'image/png', '.ttf': 'font/ttf', '.woff2': 'font/woff2', '.json': 'application/json' };
const virtual = new Map(); // /__name -> { type, body }
function serve(port0) {
  return new Promise((resolve) => {
    const srv = http.createServer((req, res) => {
      res.setHeader('content-security-policy', CSP);
      if (req.method !== 'GET') { res.statusCode = 405; return res.end(); }
      const u = decodeURIComponent(req.url.split('?')[0]);
      if (u === '/favicon.ico') { res.statusCode = 204; return res.end(); } // the browser asks for it by itself; the pages name none
      if (virtual.has(u)) { const v = virtual.get(u); res.setHeader('content-type', v.type); return res.end(v.body); }
      const f = path.resolve(path.join(repo, u));
      if (f !== repo && !f.startsWith(repo + path.sep)) { res.statusCode = 404; return res.end(); }
      if (!fs.existsSync(f) || fs.statSync(f).isDirectory()) { res.statusCode = 404; return res.end(); }
      res.setHeader('content-type', types[path.extname(f)] || 'application/octet-stream');
      res.end(fs.readFileSync(f));
    }).listen(port0, '127.0.0.1', () => resolve(srv));
  });
}

// ---- the browser, over the DevTools protocol ----
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
    if (d.id && pending.has(d.id)) { const { res, rej } = pending.get(d.id); pending.delete(d.id); d.error ? rej(new Error(JSON.stringify(d.error))) : res(d.result); }
    else if (d.method) listeners.forEach((l) => l(d));
  };
  const send = (method, params = {}) => new Promise((res, rej) => { const i = ++id; pending.set(i, { res, rej }); ws.send(JSON.stringify({ id: i, method, params })); });
  return { send, on: (l) => listeners.push(l), close: async () => { try { await send('Browser.close'); } catch (e) { /* closing */ } try { ws.close(); } catch (e) { /* closed */ } setTimeout(() => proc.kill(), 1500); } };
}

// ---- the capture plan ----
const WIDTHS = [390, 768, 1280, 1600];
const PLAQUES = ['a12h4', 'a13h12', 'a12h13', 'a6h10'];
const plan = [];
const add = (c) => plan.push(c);
for (const theme of ['dark', 'light']) {
  for (const w of WIDTHS) {
    for (const p of PLAQUES) add({ name: `LCC_M2_plaque_${p}_${theme}_${w}`, w, theme, q: `scene=plaque-${p}`, sel: `figure[data-case="${p}"]` });
    for (const s of ['lanes', 'houses']) add({ name: `LCC_M2_${s}_${theme}_${w}`, w, theme, q: `scene=${s}`, sel: `section[data-scene="${s}"]` });
  }
  add({ name: `LCC_M2_primitives_${theme}_1280`, w: 1280, theme, q: 'scene=primitives', sel: 'section[data-scene="primitives"]' });
  add({ name: `LCC_M2_art_${theme}_1280`, w: 1280, theme, q: 'scene=art', sel: 'section[data-scene="art"]' });
}
for (const p of ['a0h0', 'bad']) for (const w of [390, 1280]) add({ name: `LCC_M2_plaque_${p}_dark_${w}`, w, theme: 'dark', q: `scene=plaque-${p}`, sel: `figure[data-case="${p}"]` });
for (const look of ['awakening', 'sietch']) {
  for (const theme of ['dark', 'light']) {
    add({ name: `LCC_M2_look-${look}_plaque_a13h12_${theme}_1280`, w: 1280, theme, q: `look=${look}&scene=plaque-a13h12`, sel: 'figure[data-case="a13h12"]' });
    add({ name: `LCC_M2_look-${look}_houses_${theme}_1280`, w: 1280, theme, q: `look=${look}&scene=houses`, sel: 'section[data-scene="houses"]' });
    add({ name: `LCC_M2_look-${look}_lanes_${theme}_1280`, w: 1280, theme, q: `look=${look}&scene=lanes`, sel: 'section[data-scene="lanes"]' });
  }
}
for (const m of ['calm', 'off']) add({ name: `LCC_M2_motion-${m}_houses_dark_1280`, w: 1280, theme: 'dark', q: `motion=${m}&scene=houses`, sel: 'section[data-scene="houses"]' });
add({ name: 'LCC_M2_reduced-motion_houses_dark_1280', w: 1280, theme: 'dark', q: 'scene=houses', sel: 'section[data-scene="houses"]', media: [{ name: 'prefers-reduced-motion', value: 'reduce' }] });
add({ name: 'LCC_M2_forced-colors_plaque_a13h12_1280', w: 1280, theme: 'dark', q: 'scene=plaque-a13h12', sel: 'figure[data-case="a13h12"]', media: [{ name: 'forced-colors', value: 'active' }] });
add({ name: 'LCC_M2_forced-colors_houses_1280', w: 1280, theme: 'dark', q: 'scene=houses', sel: 'section[data-scene="houses"]', media: [{ name: 'forced-colors', value: 'active' }] });
add({ name: 'LCC_M2_forced-colors_lanes_1280', w: 1280, theme: 'dark', q: 'scene=lanes', sel: 'section[data-scene="lanes"]', media: [{ name: 'forced-colors', value: 'active' }] });
add({ name: 'LCC_M2_forced-colors_plaque_a13h12_390', w: 390, theme: 'dark', q: 'scene=plaque-a13h12', sel: 'figure[data-case="a13h12"]', media: [{ name: 'forced-colors', value: 'active' }] });
// 200% zoom is a browser window half as wide in CSS pixels at twice the pixel density
for (const [css, label] of [[640, '1280'], [800, '1600']]) {
  add({ name: `LCC_M2_zoom200_plaque_a13h12_dark_${label}`, w: css, dsf: 2, theme: 'dark', q: 'scene=plaque-a13h12', sel: 'figure[data-case="a13h12"]' });
  add({ name: `LCC_M2_zoom200_houses_dark_${label}`, w: css, dsf: 2, theme: 'dark', q: 'scene=houses', sel: 'section[data-scene="houses"]' });
  add({ name: `LCC_M2_zoom200_lanes_dark_${label}`, w: css, dsf: 2, theme: 'dark', q: 'scene=lanes', sel: 'section[data-scene="lanes"]' });
}
add({ name: 'LCC_M2_gallery_full_dark_1280', w: 1280, theme: 'dark', q: '', full: true });

// The five risky crops (fractions of the plaque picture: x0, y0, x1, y1), captured at 2x with the plaque 1450 CSS pixels wide, the width of the approved reference.
const CROPS = {
  'left-point': [0.12, 0.38, 0.36, 0.7],
  'right-point': [0.64, 0.38, 0.88, 0.7],
  'center-gold': [0.38, 0.36, 0.62, 0.72],
  'footer': [0.1, 0.7, 0.9, 0.99],
  'held-by-divider': [0.66, 0.0, 0.8, 0.3],
};
// where the plaque sits in the 1510-pixel-wide approved reference picture (x, y, width, height)
const REF_BOX = { x: 30, y: 280, w: 1450, h: 482 };

const CMP_JS = `(function(){var p=new URLSearchParams(location.search),f=p.get('f').split(',').map(Number),S=1.5,R=${JSON.stringify(REF_BOX)};
function mk(cap,src,iw,ih,ox,oy,wide,high){var c=document.createElement('figure');c.style.margin='0 12px 12px 0';c.style.display='inline-block';c.style.font='12px sans-serif';c.style.color='#ddd';
var d=document.createElement('div');d.style.width=wide+'px';d.style.height=high+'px';d.style.overflow='hidden';d.style.position='relative';d.style.background='#222';
var i=document.createElement('img');i.src=src;i.style.position='absolute';i.style.width=iw+'px';if(ih)i.style.height=ih+'px';i.style.left=(-ox)+'px';i.style.top=(-oy)+'px';d.appendChild(i);c.appendChild(d);
var t=document.createElement('figcaption');t.textContent=cap;c.appendChild(t);document.body.appendChild(c);}
var cw=(f[2]-f[0])*R.w*S,ch=(f[3]-f[1])*R.h*S;document.body.style.margin='8px';document.body.style.background='#111';document.body.style.width=(cw*2+40)+'px';
mk('APPROVED REFERENCE (V20)','/__ref.png',1510*S,0,(R.x+f[0]*R.w)*S,(R.y+f[1]*R.h)*S,cw,ch);
mk('THIS BUILD','/__mine/'+p.get('n')+'.png',R.w*S,R.h*S,f[0]*R.w*S,f[1]*R.h*S,cw,ch);
document.documentElement.dataset.cmpReady='1';})();`;

async function main() {
  const browser = findBrowser();
  if (!browser) { console.error('no browser found; set LCC_BROWSER'); process.exit(2); }
  const srv = await serve(0);
  const port = srv.address().port;
  const base = `http://127.0.0.1:${port}`;
  const b = await launch(browser, path.join(os.tmpdir(), 'lcc-shots-profile-' + process.pid));
  const report = { browser: path.basename(browser), viewports: [...new Set(plan.map((c) => c.w + 'x@' + (c.dsf || 1)))], shots: [], consoleProblems: [], network: [], notRun: [] };
  let current = '';
  b.on((d) => {
    if (d.method === 'Runtime.exceptionThrown') report.consoleProblems.push({ shot: current, kind: 'exception', text: JSON.stringify(d.params.exceptionDetails).slice(0, 300) });
    else if (d.method === 'Runtime.consoleAPICalled' && ['error', 'warning'].includes(d.params.type)) report.consoleProblems.push({ shot: current, kind: 'console.' + d.params.type, text: JSON.stringify(d.params.args).slice(0, 300) });
    else if (d.method === 'Log.entryAdded' && ['error', 'warning'].includes(d.params.entry.level)) report.consoleProblems.push({ shot: current, kind: 'log.' + d.params.entry.level, text: (d.params.entry.text + ' ' + (d.params.entry.url || '')).slice(0, 300) });
    else if (d.method === 'Network.requestWillBeSent') { const u = d.params.request.url; if (!u.startsWith(base + '/') && !u.startsWith('about:')) report.network.push({ shot: current, kind: 'not local', url: u }); }
    else if (d.method === 'Network.responseReceived' && d.params.response.status >= 400) report.network.push({ shot: current, kind: 'status ' + d.params.response.status, url: d.params.response.url });
    else if (d.method === 'Network.loadingFailed') report.network.push({ shot: current, kind: 'failed ' + d.params.errorText, url: d.params.requestId });
  });
  for (const dom of ['Runtime', 'Log', 'Page', 'Network']) await b.send(dom + '.enable');

  const shoot = async (c) => {
    current = c.name;
    await b.send('Emulation.setDeviceMetricsOverride', { width: c.w, height: c.h || 900, deviceScaleFactor: c.dsf || 1, mobile: false });
    await b.send('Emulation.setEmulatedMedia', { features: [{ name: 'prefers-color-scheme', value: c.theme || 'dark' }, ...(c.media || [])] });
    await b.send('Page.navigate', { url: `${base}${c.page || GALLERY}${c.q ? '?' + c.q : ''}` });
    for (let i = 0; i < 60; i++) { const r = await b.send('Runtime.evaluate', { expression: 'document.documentElement.dataset.galleryReady||document.documentElement.dataset.cmpReady||""', returnByValue: true }); if (r.result.value) break; await sleep(100); }
    await sleep(c.settle || 1400);
    if (c.prep) await b.send('Runtime.evaluate', { expression: c.prep });
    let clip;
    if (c.sel) {
      const r = await b.send('Runtime.evaluate', { expression: `(()=>{const e=document.querySelector(${JSON.stringify(c.sel)});if(!e)return '';const r=e.getBoundingClientRect();return JSON.stringify({x:r.x+scrollX,y:r.y+scrollY,width:r.width,height:r.height,scale:1})})()`, returnByValue: true });
      if (!r.result.value) throw new Error('selector not found: ' + c.sel);
      clip = JSON.parse(r.result.value);
      if (c.crop) { const k = c.crop; clip = { x: clip.x + clip.width * k[0], y: clip.y + clip.height * k[1], width: clip.width * (k[2] - k[0]), height: clip.height * (k[3] - k[1]), scale: 1 }; }
    } else {
      const r = await b.send('Runtime.evaluate', { expression: 'document.documentElement.scrollHeight', returnByValue: true });
      clip = { x: 0, y: 0, width: c.w, height: Math.min(r.result.value, 6000), scale: 1 };
    }
    const sh = await b.send('Page.captureScreenshot', { format: 'png', clip, captureBeyondViewport: true });
    const buf = Buffer.from(sh.data, 'base64');
    const dst = c.mine ? path.join(out, 'mine') : out;
    fs.mkdirSync(dst, { recursive: true });
    fs.writeFileSync(path.join(dst, c.name + '.png'), buf);
    report.shots.push({ file: c.name + '.png', viewport: `${c.w}x${c.h || 900}@${c.dsf || 1}`, theme: c.theme || 'dark', media: (c.media || []).map((m) => m.name + '=' + m.value).join(','), bytes: buf.length });
    console.log('captured', c.name);
    return buf;
  };

  for (const c of plan) {
    if (only && !c.name.includes(only)) continue;
    try { await shoot(c); } catch (e) { report.notRun.push({ name: c.name, why: String(e.message || e).slice(0, 200) }); console.log('NOT RUN', c.name, e.message); }
  }

  // the five crops: the plaque of the a13h12 case at the reference width, then a side-by-side with the approved picture when it is given
  if (!only || only.includes('crop')) {
    const mine = new Map();
    const crop = { name: 'LCC_M2_plaque_a12h4_dark_1450', w: 1700, dsf: 2, theme: 'dark', q: 'scene=plaque-a12h4', sel: 'figure[data-case="a12h4"] .lcc-plaque-host', mine: true,
      prep: "(function(){var h=document.querySelector('figure[data-case=\"a12h4\"] .lcc-plaque-host');h.style.width='1450px';h.style.maxWidth='1450px';})()" };
    try {
      const buf = await shoot(crop);
      mine.set(crop.name, buf);
      for (const [k, f] of Object.entries(CROPS)) {
        await shoot({ ...crop, name: `LCC_M2_crop_${k}_dark_2x`, crop: f, mine: false });
      }
      if (refImage) {
        virtual.set('/__ref.png', { type: 'image/png', body: fs.readFileSync(refImage) });
        virtual.set('/__mine/' + crop.name + '.png', { type: 'image/png', body: buf });
        virtual.set('/__cmp.js', { type: 'text/javascript', body: CMP_JS });
        virtual.set('/__cmp.html', { type: 'text/html', body: '<!doctype html><meta charset=utf-8><title>compare</title><body><script src="/__cmp.js"></script>' });
        for (const [k, f] of Object.entries(CROPS)) {
          const cw = Math.round((f[2] - f[0]) * REF_BOX.w * 1.5), ch = Math.round((f[3] - f[1]) * REF_BOX.h * 1.5);
          await shoot({ name: `LCC_M2_compare_${k}`, w: cw * 2 + 60, h: ch + 60, theme: 'dark', page: '/__cmp.html', q: `n=${crop.name}&f=${f.join(',')}`, settle: 700, sel: 'body' });
        }
      } else report.notRun.push({ name: 'side-by-side crops against the approved picture', why: 'LCC_REF_IMAGE not given' });
    } catch (e) { report.notRun.push({ name: 'risky crops', why: String(e.message || e).slice(0, 200) }); console.log('NOT RUN crops', e.message); }
  }

  fs.writeFileSync(path.join(out, 'LCC_M2_report.json'), JSON.stringify(report, null, 2));
  console.log(`shots: ${report.shots.length}; console problems: ${report.consoleProblems.length}; network problems: ${report.network.length}; not run: ${report.notRun.length}`);
  await b.close(); srv.close();
  process.exit(report.consoleProblems.length || report.network.length || report.notRun.length ? 1 : 0);
}
main().catch((e) => { console.error(e); process.exit(2); });
