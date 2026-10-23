// A small jsdom boot of the real UI code for tests that follow one flow (the reset buttons). Answers come from the generated fixtures
// (internal/web/fixtures_test.go) unless the test gives its own: get[path] for a GET, post[path] for a POST, and { __fail, __code, __status }
// for a refusal. Every POST is recorded in `posted`.
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { JSDOM } from 'jsdom';

const here = path.dirname(fileURLToPath(import.meta.url));
const static_ = path.join(here, '..', 'internal', 'web', 'static');
const FIX = process.env.UI_FIXTURES_FILE || path.join(process.env.UI_FIXTURES_DIR || path.join(here, 'fixtures'), 'ui-fixtures.json');
if (!fs.existsSync(FIX)) throw new Error(`UI fixtures not found at ${FIX}. Generate them first:\n  UI_FIXTURES_DIR=${path.dirname(FIX)} go test ./internal/web -run TestDumpUIFixtures`);
const fixtures = JSON.parse(fs.readFileSync(FIX, 'utf8'));
const read = (n) => fs.readFileSync(path.join(static_, n), 'utf8');

const open = [];
// Closes every window a test opened, passed or failed: the page polls on a timer, and a window left open keeps the test process alive.
export function closeAll() { while (open.length) open.pop().window.close(); }

export async function boot({ post = {}, get = {} } = {}) {
  const dom = new JSDOM(read('index.html'), { url: 'http://127.0.0.1:8090/', runScripts: 'dangerously', pretendToBeVisual: true });
  const w = dom.window;
  open.push(dom);
  let inflight = 0;
  const posted = [], fetched = [];
  w.fetch = async (p, opt = {}) => {
    inflight++;
    try {
      await Promise.resolve();
      const u = new URL(p, 'http://127.0.0.1:8090');
      const fail = (b) => ({ ok: false, status: b.__status || 400, statusText: 'Bad Request', json: async () => ({ error: b.__fail, code: b.__code }) });
      if ((opt.method || 'GET') === 'POST') {
        const b = post[u.pathname] ?? { ok: true };
        posted.push({ path: u.pathname, body: opt.body ? JSON.parse(opt.body) : null });
        if (b.__fail) return fail(b);
        return { ok: true, status: 200, statusText: 'OK', json: async () => b };
      }
      fetched.push(u.pathname);
      const g = get[u.pathname];
      if (g && g.__fail) return fail(g);
      if (g) return { ok: true, status: 200, statusText: 'OK', json: async () => g, blob: async () => new w.Blob(['x']) };
      const f = fixtures[u.pathname];
      const status = f ? f.status : 404;
      return { ok: status < 400, status, statusText: f ? 'OK' : 'Not Found', json: async () => (f ? f.body : { error: 'not in fixtures: ' + u.pathname }), blob: async () => new w.Blob(['x']) };
    } finally { inflight--; }
  };
  w.eval(read('html.js'));
  w.eval(read('livemap.js'));
  w.eval(read('app.js'));
  const settle = async () => { for (let i = 0; i < 40; i++) { await new Promise((r) => w.setTimeout(r, 0)); if (inflight === 0 && i > 4) return; } };
  await settle();
  const click = (sel) => { const el = w.document.querySelector(sel); assert.ok(el, `no element for ${sel}`); el.dispatchEvent(new w.MouseEvent('click', { bubbles: true, cancelable: true })); };
  // types into the question's box the way a person does: the input event fires
  const type = (text) => { const el = w.document.querySelector('#atype'); assert.ok(el, 'no typed-confirmation box'); el.value = text; el.dispatchEvent(new w.Event('input', { bubbles: true })); };
  const wait = (ms) => new Promise((r) => w.setTimeout(r, ms));
  return { dom, w, doc: w.document, settle, posted, fetched, click, type, wait };
}

// The minimum gap a second press must respect (app.js ARM_GAP_MS is 400 ms); tests wait a little longer.
export const GAP = 450;
