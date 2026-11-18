// A jsdom window with the Command Center helpers (internal/web/static/lcc/lcc.js, and lcc-model.js before it) loaded, for the lcc-*.test.mjs files. Nothing else of the editor is loaded: the helpers stand alone.
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { JSDOM } from 'jsdom';

const here = path.dirname(fileURLToPath(import.meta.url));
export const root = path.join(here, '..');
export const lccDir = path.join(root, 'internal', 'web', 'static', 'lcc');
export const read = (...p) => fs.readFileSync(path.join(root, ...p), 'utf8');

export function load() {
  const dom = new JSDOM('<!doctype html><body></body>', { url: 'http://127.0.0.1:8090/', runScripts: 'outside-only' });
  dom.window.eval(fs.readFileSync(path.join(lccDir, 'lcc-model.js'), 'utf8')); // the words of the Sysselraad are in the model (WIN): lcc.js takes them from there
  dom.window.eval(fs.readFileSync(path.join(lccDir, 'lcc.js'), 'utf8'));
  return { dom, w: dom.window, L: dom.window.LCC, doc: dom.window.document };
}
