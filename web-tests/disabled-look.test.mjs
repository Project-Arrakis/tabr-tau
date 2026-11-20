// #1049: one standard look for every control that is off, in every style file of the editor. The style sheets are read as text: a rule whose selector names a disabled control may not draw a
// dashed edge and may not fade (an opacity under 1), and the one look is written once, in tokens, at the top of app.css. What a layout engine shows (the colours on a real page) is in the proof
// pictures of the pull request and in scripts/lcc-m7-sweep.mjs.
import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const staticDir = path.join(here, '..', 'internal', 'web', 'static');
const FILES = ['app.css', 'lcc/lcc.css', 'lcc/lcc-board.css', 'lcc/lcc-spec.css', 'lcc/lcc-term.css'];
const css = Object.fromEntries(FILES.map((f) => [f, fs.readFileSync(path.join(staticDir, f), 'utf8').replace(/\r\n/g, '\n').replace(/\/\*[\s\S]*?\*\//g, '')]));

// every rule as { sel, body, media }, one level of @media included
function rules(text) {
  const out = [];
  const walk = (src, media) => {
    let i = 0;
    while (i < src.length) {
      const open = src.indexOf('{', i);
      if (open < 0) break;
      const head = src.slice(i, open).trim();
      let depth = 1, j = open + 1;
      while (j < src.length && depth) { if (src[j] === '{') depth++; else if (src[j] === '}') depth--; j++; }
      const body = src.slice(open + 1, j - 1);
      if (head.startsWith('@media') || head.startsWith('@supports')) walk(body, head);
      else if (!head.startsWith('@')) out.push({ sel: head, body, media });
      i = j;
    }
  };
  walk(text, '');
  return out;
}
const isOff = (sel) => /:disabled|\[aria-disabled=["']?true["']?\]|\[disabled\]/.test(sel);

test('#1049: no rule for a disabled control draws a dashed edge', () => {
  const bad = [];
  for (const f of FILES) for (const r of rules(css[f])) if (isOff(r.sel) && /dashed|dotted/.test(r.body)) bad.push(f + ': ' + r.sel);
  assert.deepEqual(bad, [], 'a disabled control takes the one look of app.css, never a dashed edge');
});

test('#1049: no rule for a disabled control fades it (an opacity under 1)', () => {
  const bad = [];
  for (const f of FILES) for (const r of rules(css[f])) {
    if (!isOff(r.sel)) continue;
    for (const m of r.body.matchAll(/(?:^|[;\s])opacity\s*:\s*([^;]+)/g)) if (Number(m[1]) < 1) bad.push(f + ': ' + r.sel + ' opacity ' + m[1]);
    if (/saturate\(/.test(r.body)) bad.push(f + ': ' + r.sel + ' filter saturate');
  }
  assert.deepEqual(bad, [], 'a faded label loses its contrast: the quiet colours do the telling');
});

test('#1049: the one look is three tokens and one rule, for every kind of control, with the not-allowed pointer', () => {
  const app = css['app.css'];
  assert.match(app, /--dis-fg:\s*var\(--c-dim\)/);
  assert.match(app, /--dis-bg:\s*transparent/);
  assert.match(app, /--dis-edge:\s*var\(--c-hair\)/);
  const one = rules(app).find((r) => r.sel.includes('[aria-disabled="true"]') && r.sel.includes('button') && r.body.includes('var(--dis-fg)'));
  assert.ok(one, 'the shared rule');
  for (const kind of ['button', 'select', 'input', 'textarea', '[role="tab"]', '[role="switch"]', '.d-btn', '.d-toggle', ':disabled']) assert.ok(one.sel.includes(kind), 'covers ' + kind);
  for (const d of ['color:var(--dis-fg)', 'background-color:var(--dis-bg)', 'border-color:var(--dis-edge)', 'box-shadow:none', 'cursor:not-allowed']) assert.ok(one.body.replace(/\s+/g, '').includes(d), d);
  assert.doesNotMatch(one.body, /dashed|opacity:\s*\.\d/);
  // the checkbox and the switch follow the same colours
  assert.ok(rules(app).some((r) => r.sel.includes('input[type=checkbox]') && r.body.includes('accent-color:var(--dis-fg)')), 'a disabled checkbox');
  assert.ok(rules(app).some((r) => r.sel.includes('.d-toggle') && r.sel.includes('::after') && r.body.includes('var(--dis-fg)')), 'a disabled switch');
});

test('#1049: the other style files point at the tokens and do not define their own disabled colours', () => {
  for (const f of ['lcc/lcc-board.css', 'lcc/lcc-spec.css', 'lcc/lcc-term.css', 'lcc/lcc.css']) {
    for (const r of rules(css[f])) {
      if (!isOff(r.sel) || /forced-colors/.test(r.media)) continue;
      assert.doesNotMatch(r.body, /#[0-9a-f]{3,8}\b|rgba?\(/i, f + ': ' + r.sel + ' sets a colour of its own');
    }
  }
  // the places the audit found: none of them is left
  for (const gone of ['.lcc-bd-select:disabled', '.lcc-bd-chip[aria-disabled="true"]', '.lcc-bd-layout button[aria-disabled="true"]', '.lcc-bd-act[aria-disabled="true"]', '.lcc-tm-select:disabled', '.lcc-tm-btn[aria-disabled="true"]', '.lcc-sp-input:disabled', '.lcc-sp-btn:disabled']) {
    for (const f of ['lcc/lcc-board.css', 'lcc/lcc-spec.css', 'lcc/lcc-term.css']) {
      const own = rules(css[f]).filter((r) => r.sel === gone && !/forced-colors/.test(r.media));
      assert.deepEqual(own, [], f + ' still has its own rule for ' + gone);
    }
  }
  for (const gone of ['.lsb .qa[aria-disabled="true"]', '.lsb .tc[aria-disabled="true"]', '.lsb .sorth:disabled']) assert.deepEqual(rules(css['app.css']).filter((r) => r.sel === gone), [], gone);
});

test('#1049: a tier box keeps its state colours and takes the quiet edge and the pointer only', () => {
  const t = rules(css['app.css']).find((r) => r.sel.includes('.lcc-bd-tier') && r.sel.includes('[aria-disabled="true"]') && r.body.includes('border-style:solid'));
  assert.ok(t, 'the tier rule');
  assert.match(t.body, /cursor:not-allowed/);
  assert.doesNotMatch(t.body, /dashed|opacity|color:var\(--dis-fg\)\s*;?\s*$/);
});

// ---- the exact button of the operator's screenshot: Reopen in the Inspector Actions, next to Complete House and Fill 5 tiers ---------------------------------------------------------------
test('#1049: the disabled Reopen button of the Inspector has no dashed border: no rule that matches it draws one, and the shared disabled rule does match it', async () => {
  const { boot, closeAll } = await import('./boot.mjs');
  const { synthetic } = await import('./lcc-fixture.mjs');
  try {
    const a = synthetic();
    const ui = await boot({ get: { '/api/landsraad': a }, local: { tab: 'landsraad', 'lsb.view': 'board', 'lsb.layout': 'grid' } });
    await ui.settle();
    const open = a.tasks.find((t) => t.house_name === 'DA_HouseKenola'); // an open House: Reopen has nothing to reopen
    ui.doc.querySelector('[data-fk="ht:' + open.id + '"]').click();
    await ui.settle();
    const reopen = ui.doc.querySelector('.lcc-bd-insp .lcc-bd-act[data-do="reopen"]');
    assert.ok(reopen, 'the Reopen button');
    assert.equal(reopen.getAttribute('aria-disabled'), 'true', 'Reopen is off on an open House');
    for (const n of ['complete', 'fill']) assert.ok(ui.doc.querySelector('.lcc-bd-insp .lcc-bd-act[data-do="' + n + '"]'), 'its neighbours are there: ' + n);
    let sharedMatches = 0;
    const dashed = [];
    for (const f of FILES) for (const r of rules(css[f])) {
      if (/forced-colors/.test(r.media)) continue;
      for (const sel of r.sel.split(/,(?![^(]*\))/).map((x) => x.trim())) {
        let hit = false;
        try { hit = reopen.matches(sel); } catch (e) { hit = false; }
        if (!hit) continue;
        if (/dashed|dotted/.test(r.body)) dashed.push(f + ': ' + sel);
        if (f === 'app.css' && r.body.includes('var(--dis-edge)') && r.body.includes('cursor:not-allowed')) sharedMatches++;
      }
    }
    assert.deepEqual(dashed, [], 'a rule that matches Reopen draws a dashed or dotted edge');
    assert.ok(sharedMatches >= 1, 'the one shared disabled rule of app.css matches the Reopen button');
  } finally { closeAll(); }
});
