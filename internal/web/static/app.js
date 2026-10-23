// Tabr Tau UI. All HTML is built with html`...` (escaped by construction, see html.js) and written with setHTML.
// Nothing here knows about the session: it is an HttpOnly cookie the browser sends on its own.
const { html, setHTML } = window.TabrHTML;
const $ = (s, r = document) => r.querySelector(s);

// What the editor said about the last edit it was sent: 'written' (it is in the game's file), 'queued' (accepted, written when the file can be), or null when the
// answer carried none (nothing changed, or the request failed). A message about an edit reads this and never guesses from an older status.
let lastOutcome = null;
// A refusal carries a stable code (changed_on_disk, conflict, no_room, queue_held, queued, game_running, read_only, bad_argument, not_found, bad_request, and for faults integrity_failed and write_failed) and the HTTP status; the words
// of the message are for the person and are never matched.
async function api(path, body) {
  const opt = { credentials: 'same-origin' };
  if (body !== undefined) {
    opt.method = 'POST';
    lastOutcome = null;
    opt.headers = { 'Content-Type': 'application/json' };
    opt.body = JSON.stringify(body);
  }
  const r = await fetch(path, opt);
  const j = await r.json().catch(() => ({ error: r.statusText }));
  // The editor answers a signed-out page with a sentence of its own (code not_signed_in). This is only the fallback for a 403 that carries none.
  if (r.status === 403 && !j.error) throw new Error('This window is no longer signed in to the editor. Close tabr-tau completely and start it again: the editor\'s own window opens signed in; in a browser, open the new link printed in the terminal.');
  if (!r.ok || j.error) { const e = new Error(j.error || r.statusText); e.code = j.code; e.status = r.status; throw e; }
  if (body !== undefined && j && (j.outcome === 'written' || j.outcome === 'queued')) lastOutcome = j.outcome;
  return j;
}
function toast(msg, err, ms = 3500) {
  const d = document.createElement('div');
  const part = err === 'part'; // part of a give went in: a warning that stays until it is dismissed, not a failure
  d.setAttribute('role', err && !part ? 'alert' : 'status');
  if (err) d.className = part ? 'part' : 'err';
  const t = document.createElement('span');
  t.textContent = msg;
  d.append(t);
  if (err) { // an error stays until it is dismissed: a message that vanishes in seconds is easy to miss
    const x = document.createElement('button');
    x.className = 'x';
    x.textContent = '\u00d7';
    x.title = 'Dismiss';
    x.onclick = () => d.remove();
    d.append(x);
  } else setTimeout(() => d.remove(), ms);
  $('#toast').append(d);
}
// What a give did, in words: okText when all of it went in, else how much did and why not more (a give puts in as much as fits). The answer carries requested, given and reason.
function giveToast(r, name, okText) {
  const n = (v) => Number(v).toLocaleString();
  if (r && Number(r.given) < Number(r.requested)) toast(`Gave ${n(r.given)} of ${n(r.requested)} ${name}: ${r.reason || 'not all of it fitted'}.` + NOT_SAVED, 'part');
  else toast(okText + NOT_SAVED);
}
// An edit of the save is written to the game's file as it is made, so a message has nothing to add; act() adds a hint when it finds the edit queued instead (the game is
// in a single-player session and rewrites the file). NOT_SAVED is that hint for the messages that are built by hand: it reads as the hint while edits are being queued
// (the last status said so) and as nothing otherwise.
const QUEUED_HINT = ' (queued: written when the game closes)';
const NOT_SAVED = { toString: () => (lastOutcome ? (lastOutcome === 'queued' ? QUEUED_HINT : '') : saveBlocked || lastDirty ? QUEUED_HINT : '') };
// immediate: the change is written at once (a config file, a setting) and never waits in the queue, so it never gets the hint.
async function act(fn, okMsg, refresh = true, immediate = false) {
  try {
    lastOutcome = null; // an older edit's outcome must not colour this message
    const r = await fn();
    let dirty = false;
    if (lastOutcome) dirty = lastOutcome === 'queued'; // the answer to the edit said which
    else if (!immediate) try { dirty = (await status()).dirty; } catch (e) { /* the poll reports a lost connection; the edit itself succeeded */ }
    if (okMsg) toast(okMsg + (dirty ? QUEUED_HINT : ''));
    if (refresh) await render();
    return r;
  }
  catch (e) { toast(e.message, true); }
}
// What a box edited in place said before it was changed, and the sentence that reports the change: "Spice Melange: stack 5 → 10. To undo, set it back to 5."
// There is no undo of its own: the old value is shown so that the person can type it again, and the backup holds the file as it was.
const shownBefore = (t) => (t.tagName === 'SELECT' ? ((([...t.options].find((o) => o.defaultSelected)) || {}).text || '') : (t.dataset.orig !== undefined ? t.dataset.orig : t.defaultValue));
const brief = (v) => { const s = String(v == null ? '' : v); return s === '' ? '(empty)' : s.length > 40 ? s.slice(0, 37) + '...' : s; };
function changedMsg(t, what) {
  const was = shownBefore(t), now = t.tagName === 'SELECT' ? (t.selectedIndex >= 0 ? t.options[t.selectedIndex].text : '') : t.value;
  // the old value is shown whole to put back (a cut-short one could not be typed again); a value too long to show says so instead of giving half of it
  const w = String(was == null ? '' : was);
  const back = w === '' ? '(empty)' : w.length <= 200 ? w : null;
  return `${what} ${brief(was)} → ${brief(now)}. To undo, ${back === null ? 'set it back to the value it had (too long to show here; the backup has it)' : 'set it back to ' + back}.`;
}
// A box edited in place keeps what was typed until the server answers. If the edit is refused the box goes back to the value it had, so the
// page never shows a value the save does not hold (UX-14); if it is accepted, that value is the one to go back to next time.
async function editInput(t, fn, okMsg, immediate = false) {
  let failed = false;
  await act(async () => { try { return await fn(); } catch (e) { failed = true; throw e; } }, okMsg, false, immediate);
  if (failed) {
    if (t.tagName === 'SELECT') { const i = [...t.options].findIndex((o) => o.defaultSelected); if (i >= 0) t.selectedIndex = i; }
    else t.value = t.dataset.orig !== undefined ? t.dataset.orig : t.defaultValue;
  } else if (t.tagName === 'SELECT') {
    [...t.options].forEach((o) => { o.defaultSelected = o.selected; });
  } else {
    t.defaultValue = t.value;
    if (t.dataset.orig !== undefined) t.dataset.orig = t.value;
  }
}
const val = (id) => $('#' + id).value;

// ---------- item names: one catalog, one picker, used by every place that asks for or shows an item
let CATALOG = null; // { list: [{ id, label }], byLabel: Map(lower label -> id), byId: Map(id -> name) }
let EXTRA_IDS = new Set(); // ids in this save that the catalog lacks
async function loadCatalog(extraIds = []) {
  if (!CATALOG) {
    const items = await api('/api/catalog/items');
    const count = new Map();
    const idsLower = new Map(items.map((it) => [it.id.toLowerCase(), it.id]));
    for (const it of items) count.set(it.name, (count.get(it.name) || 0) + 1);
    // A name is ambiguous when several items share it, or when it spells another item's id (a typed id must win).
    const ambiguous = (it) => count.get(it.name) > 1 || (idsLower.has(it.name.toLowerCase()) && idsLower.get(it.name.toLowerCase()) !== it.id);
    const list = items.map((it) => ({ id: it.id, name: it.name, label: ambiguous(it) ? `${it.name} (${it.id})` : it.name }));
    CATALOG = { list, byLabel: new Map(list.map((e) => [e.label.toLowerCase(), e.id])), byId: new Map(list.map((e) => [e.id, e.name])) };
  }
  // ids that exist in this save but not in the catalog stay selectable by their id
  const extra = extraIds.filter((id) => id && !CATALOG.byId.has(id)).map((id) => ({ id, label: id }));
  EXTRA_IDS = new Set(extra.map((e) => e.id));
  return { list: extra.length ? CATALOG.list.concat(extra) : CATALOG.list };
}
const itemName = (id) => (CATALOG && CATALOG.byId.get(id)) || id;
// An item cell: the in-game name, with the template id on hover.
const itemCell = (id) => html`<span title="${id}">${itemName(id)}</span>`;
// The picker: a text box with a drop-down of in-game names. Each option also carries its template id as its label, so
// typing either the name or the id finds it; a name, a label or a raw template id are all accepted.
function itemPicker(inputId, cat, placeholder = 'Item name, e.g. Spice Melange') {
  return html`<input id="${inputId}" list="${inputId}-dl" placeholder="${placeholder}" size="34"><datalist id="${inputId}-dl">${cat.list.map((e) => html`<option value="${e.label}" label="${e.id}"></option>`)}</datalist>`;
}
// What the picker holds -> the template id to send. An exact template id always wins; otherwise a shown label.
// The id of the item typed or picked in an item box, or '' when the text is not an item: a catalog name, a catalog id or an id already in
// this save. Free text is never sent on as an id (typing "Spice" is not an item; the item is Spice Melange).
function pickedItemId(inputId) {
  const v = val(inputId).trim();
  if (!CATALOG) return '';
  if (CATALOG.byId.has(v) || EXTRA_IDS.has(v)) return v;
  return CATALOG.byLabel.get(v.toLowerCase()) || '';
}
// Like pickedItemId, but says what to do when the box does not hold an item.
function needItem(inputId) {
  const t = pickedItemId(inputId);
  if (!t) {
    const v = val(inputId).trim();
    throw new Error(v ? `"${v}" is not an item. Type part of its name and choose it from the list.` : 'Pick an item first: type part of its name and choose it from the list.');
  }
  return t;
}

// Augments (#96). The slots are one drop-down each, with the augments that fit the item (from the console's compatibility data) and their
// best-grade effects; "applied" are the ids already on the item.
let augSeq = 0; // the newest request wins when the picker changes quickly
async function showGiveAugments() {
  const box = $('#gaug'), t = pickedItemId('gt'), seq = ++augSeq;
  if (!box) return;
  setHTML(box, html``);
  if (!t) return;
  try {
    const fit = await api('/api/items/augment-options', { template_id: t });
    fit.options.forEach((o) => AUG_NAMES.set(o.id, o.name));
    if (seq !== augSeq || !fit.limit || !$('#gaug')) return;
    setHTML($('#gaug'), html`<b>Augments</b> ${augSlots('ga', fit, [])}${augGradeSel('gag')}`);
  } catch (e) { /* an unknown item simply has no augment slots */ }
}
const augName = (id) => (AUG_NAMES.get(id) || id);
const AUG_NAMES = new Map(); // augment id -> name, filled from the options the server sent
const augLabel = (o) => (o.effects && o.effects.length ? `${o.name} (${o.effects.join('; ')})` : o.name);
const augSlots = (prefix, fit, applied) => html`${Array.from({ length: fit.limit }, (_, i) => html`<label>Slot ${i + 1} <select id="${prefix}${i}"><option value="">(none)</option>${fit.options.map((o) => html`<option value="${o.id}" ${applied[i] === o.id ? 'selected' : ''}>${augLabel(o)}</option>`)}</select></label> `)}`;
const augPicked = (prefix, n) => Array.from({ length: n }, (_, i) => { const e = $('#' + prefix + i); return e ? e.value : ''; }).filter(Boolean);
const augGradeSel = (id) => html`<label>Grade of added augments <select id="${id}">${[1, 2, 3, 4, 5].map((n) => html`<option ${n == 5 ? 'selected' : ''}>${n}</option>`)}</select></label>`;
// ---------- tabs
const TABS = { player: 'Player', livemap: 'Saved map', landsraad: 'Landsraad', config: 'Config', db: 'Database' };
// The Player tab mirrors the Dune Docker console's Players > Player Name view: the same tabs in the same order (#96). Bases and
// Vehicles used to be top-level tabs and are two of these.
const PTABS = [['character', 'Character'], ['crafting', 'Crafting'], ['research', 'Research'], ['buildingsets', 'Building sets'], ['customizations', 'Customizations'],
  ['skills', 'Skills'], ['specialization', 'Specialization'], ['journey', 'Journey'], ['bases', 'Bases'], ['vehicles', 'Vehicles'], ['admin', 'Admin']];
// The browser's storage is a convenience (the tab last open): a browser that blocks it, or a private window, throws on the first touch, which must not stop the page from starting.
const browserStore = {
  get(k) { try { return localStorage.getItem(k); } catch (e) { return null; } },
  set(k, v) { try { localStorage.setItem(k, v); } catch (e) { /* the choice lasts until the page is closed */ } },
};
const savedTab = browserStore.get('tab');
let tab = Object.hasOwn(TABS, savedTab) ? savedTab : 'player';
const sub = { player: ['bases', 'vehicles'].includes(savedTab) ? savedTab : 'character', char: 'overview', db: 'browse' };
const GROUPS = ['player', 'char', 'bases', 'db'];
let invG = null, invQ = ''; let giveQueue = loadQueue(); let augItem = null; // augItem: the inventory item whose augments are open for editing // Character > Inventory: the chosen group (null: the first one with items) and the filter text
function drawNav() {
  setHTML($('#nav'), html`${Object.entries(TABS).map(([k, v]) => html`<button data-tab="${k}" class="${k == tab ? 'on' : ''}" aria-current="${k == tab ? 'page' : 'false'}">${v}</button>`)}`);
}
function subNav(group, items) {
  return html`<div class="sub">${items.map(([k, v]) => html`<button data-sub="${group}:${k}" class="${sub[group] == k ? 'on' : ''}" aria-current="${sub[group] == k ? 'true' : 'false'}">${v}</button>`)}</div>`;
}

// ---------- generic renderers
// Every table sorts by the header that is clicked: once ascending, again descending, a third time back to the original order. The rows
// already on screen are sorted by what each cell shows (a number sorts as a number, an editable cell by its value, other text naturally),
// so it works for every table without each view knowing about it, and the choice is kept for a table with the same headers when it is
// drawn again after an edit. The Database viewer shows a page at a time, so it sorts that page.
const SORTS = new Map(); // header names joined -> { col, dir }
const sortKey = (t) => [...t.tHead.rows[0].cells].map((th) => th.textContent.trim()).join('|');
function cellValue(td) {
  if (!td) return '';
  const f = td.querySelector('input, select');
  if (f) return f.tagName === 'SELECT' ? (f.selectedIndex >= 0 ? f.options[f.selectedIndex].text : '') : f.value;
  return td.textContent.trim();
}
const leadingNumber = (s) => { const m = String(s).replace(/,/g, '').match(/^\s*(-?\d+(?:\.\d+)?)/); return m ? parseFloat(m[1]) : NaN; };
function compareCells(a, b) {
  if (a === '' || b === '') return a === b ? 0 : a === '' ? 1 : -1; // empty cells go last
  const na = leadingNumber(a), nb = leadingNumber(b);
  if (!Number.isNaN(na) && !Number.isNaN(nb) && na !== nb) return na - nb;
  return String(a).localeCompare(String(b), undefined, { numeric: true, sensitivity: 'base' });
}
function sortTable(t, col, dir) {
  const body = t.tBodies[0];
  if (!body) return;
  const rows = [...body.rows];
  rows.forEach((r, i) => { if (r.sortOrder === undefined) r.sortOrder = i; });
  rows.sort((a, b) => (dir ? compareCells(cellValue(a.cells[col]), cellValue(b.cells[col])) * (dir === 'asc' ? 1 : -1) : 0) || a.sortOrder - b.sortOrder);
  for (const r of rows) body.appendChild(r);
  [...t.tHead.rows[0].cells].forEach((th, i) => {
    th.classList.toggle('sa', i === col && dir === 'asc'); th.classList.toggle('sd', i === col && dir === 'desc');
    if (th.hasAttribute('aria-sort')) th.setAttribute('aria-sort', i === col && dir ? (dir === 'asc' ? 'ascending' : 'descending') : 'none');
  });
}
function headerClick(th) {
  const t = th.closest('table');
  if (!t || !t.tHead || !t.tBodies[0] || !th.textContent.trim()) return false;
  const key = sortKey(t), col = th.cellIndex, cur = SORTS.get(key);
  const dir = cur && cur.col === col ? (cur.dir === 'asc' ? 'desc' : null) : 'asc';
  if (dir) SORTS.set(key, { col, dir }); else SORTS.delete(key);
  sortTable(t, col, dir);
  sayAloud(dir ? `Sorted by ${th.textContent.trim()}, ${dir === 'asc' ? 'ascending' : 'descending'}.` : 'Back to the original order.');
  return true;
}
// A table that has just been drawn gets the sort remembered for its headers.
function applySorts() {
  for (const t of document.querySelectorAll('table')) {
    if (t.classList.contains('sortd') || !t.tHead || !t.tHead.rows[0]) continue;
    t.classList.add('sortd');
    for (const th of t.tHead.rows[0].cells) if (th.textContent.trim()) { th.setAttribute('tabindex', '0'); th.setAttribute('aria-sort', 'none'); } // a header that sorts can be reached and used from the keyboard
    const s = SORTS.get(sortKey(t));
    if (s) sortTable(t, s.col, s.dir);
  }
}
if (typeof MutationObserver !== 'undefined') new MutationObserver(applySorts).observe(document.body, { childList: true, subtree: true });
// A column with no label is named from its key in words ("time_stored" is "Time stored"); every column the person sees has a name.
const heading = (k) => capFirst(String(k).replace(/_/g, ' ').trim());
// What a count counts: "3 items" or "1 item" (the number with its noun).
const nn = (n, one, many) => `${Number(n).toLocaleString()} ${n === 1 ? one : many}`;
// "12 nodes" while nothing narrows the list; "3 of 12 nodes match" while a filter does. "12 of 12" says nothing the 12 did not.
const countText = (shown, total, one, many, filtered) => (filtered ? `${Number(shown).toLocaleString()} of ${Number(total).toLocaleString()} ${many} match` : nn(total, one, many));
// A time the save keeps as seconds (or milliseconds) since 1970, as the person's own date and time; anything that is not a plausible time stays as it is.
const fmtEpoch = (v) => {
  const n = Number(v);
  if (v == null || v === '') return '';
  if (!Number.isFinite(n) || n < 1e8) return String(v);
  const d = new Date(n > 1e12 ? n : n * 1000);
  return isNaN(d) ? String(v) : d.toLocaleString();
};
function tbl(cols, rows, { actions, empty } = {}) {
  if (!rows || !rows.length) return html`<p class="mut">${empty || 'Nothing to show here.'}</p>`;
  return html`<div class="scroll"><table><thead><tr>${cols.map((c) => html`<th>${c.label || heading(c.k)}</th>`)}${actions ? html`<th></th>` : ''}</tr></thead><tbody>${
    rows.map((r) => html`<tr>${cols.map((c) => html`<td class="${c.cls || ''}">${c.f ? c.f(r) : r[c.k]}</td>`)}${actions ? html`<td>${actions(r)}</td>` : ''}</tr>`)}</tbody></table></div>`;
}
const kv = (k, v) => html`<div class="kv"><b>${k}</b><span>${v}</span></div>`;
const fix = (n) => (n == null ? '' : Math.round(n).toLocaleString());
const fmtBytes = (n) => (n >= 1048576 ? (n / 1048576).toFixed(1) + ' MB' : n >= 1024 ? Math.round(n / 1024) + ' KB' : n + ' B');
const fmtDate = (s) => { const d = new Date(s); return isNaN(d) ? String(s || '') : d.toLocaleString(); };

// ---------- the Give queue survives a refresh
// The items queued to be given are only in the page, so a reload (the window refreshing, a restart of the page) emptied them. They are kept in the browser's local
// storage and read back through a check of every field, as nothing in storage is trusted.
function loadQueue() {
  try {
    const a = JSON.parse(localStorage.getItem('giveQueue') || '[]');
    if (!Array.isArray(a)) return [];
    const out = [];
    for (const e of a.slice(0, 200)) {
      if (!e || typeof e !== 'object' || typeof e.template_id !== 'string' || !e.template_id || e.template_id.length > 120) continue;
      const q = { template_id: e.template_id, quantity: Math.min(1e6, Math.max(1, Math.floor(Number(e.quantity)) || 1)), quality: Math.min(5, Math.max(0, Math.floor(Number(e.quality)) || 0)) };
      if (Array.isArray(e.augments) && e.augments.length && e.augments.length <= 3 && e.augments.every((x) => typeof x === 'string' && x.length <= 120)) {
        q.augments = e.augments.slice();
        q.grade = Math.min(5, Math.max(0, Math.floor(Number(e.grade)) || 0));
      }
      out.push(q);
    }
    return out;
  } catch (e) { return []; }
}
function keepQueue() {
  try { if (giveQueue.length) localStorage.setItem('giveQueue', JSON.stringify(giveQueue)); else localStorage.removeItem('giveQueue'); } catch (e) { /* storage is off: the queue lives only as long as the page */ }
}

// ---------- status bar
let saveBlocked = false, saveBlockWhy = '', offline = false, lastDirty = false;
let gwKey = '', lastPending = []; // the banner as last drawn, and the queued changes in words
const KEEP = { qlist: false, raw: false }; // which of the banner's toggles are open: kept across redraws
// The reasons the editor gives for not writing, as a sentence. The wording the tools and checks use stays available under Details.
const capFirst = (s) => String(s || '').replace(/^\s*(.)/, (m, c) => c.toUpperCase());
const endSentence = (s) => (/[.!?]$/.test(s) ? s : s + '.');
function humanReason(raw) {
  const r = String(raw || '').trim();
  const L = r.toLowerCase();
  const map = (text) => ({ text, raw: r });
  if (L.startsWith('could not check whether the game is running')) return map('Could not tell whether Dune: Awakening is running.');
  if (L.includes('cannot tell whether a single-player session is active')) return map('Dune: Awakening is running and the editor cannot tell whether it is in single-player.');
  if (L.includes('single-player session is active')) return map('A single-player session is running, and the game rewrites the save while it runs.');
  if (L.includes('single-player session just ended')) return map('The single-player session has just ended; the game is still finishing its write of the save.');
  if (L.includes('cannot tell whether the game is using it')) return map('This save is on a network share or a removable drive, so the editor cannot tell whether the game is using it.');
  if (L.includes('the game saved while the edits were being written')) return map('The game saved while the changes were being written; they are put back onto its file next.');
  return { text: endSentence(capFirst(r)), raw: '' };
}
// A queued change in the words of the page. Most descriptions already read well (they come from the editor); the short internal ones are spelled out.
function queueLine(desc) {
  const d = String(desc || '');
  let m;
  const num = (v) => Number(v).toLocaleString();
  if ((m = d.match(/^solari ([+-]\d+)$/))) return +m[1] >= 0 ? `Add ${num(m[1])} Solari to the backpack` : `Remove ${num(-m[1])} Solari from the backpack`;
  if ((m = d.match(/^xp \+(\d+)$/))) return `Add ${m[1]} XP`;
  if ((m = d.match(/^intel \+(\d+)$/))) return `Add ${m[1]} Intel`;
  if ((m = d.match(/^unspent skill points = (\d+)$/))) return `Set the unspent skill points to ${m[1]}`;
  if ((m = d.match(/^decree (\d+) disabled=([01])$/))) return `${m[2] === '1' ? 'Disable' : 'Enable'} decree ${m[1]}`;
  if ((m = d.match(/^journey (\S+) (completed|reset) \((\d+) rows?\)$/))) return `Journey step ${m[1]}: ${m[2]}`;
  if ((m = d.match(/^moved vehicle (\d+) next to player$/))) return `Move vehicle ${m[1]} next to the character`;
  return capFirst(d);
}
// The game rewrites game.db about every twenty seconds while a session runs. With nothing queued the editor reads the new file by itself and
// redraws the page where it was, so what is on screen is what the game has (a building placed in the game shows up here without a click). It
// waits while a question is open or a field is being typed in, and never touches queued edits (they are put back onto the new file when they are written). The checkbox in the header turns it off.
let autoBusy = false, autoAt = 0;
const autoRefreshOn = () => { try { return localStorage.autoRefresh !== '0'; } catch (e) { return true; } };
async function autoRefresh(o) {
  if (autoBusy || !autoRefreshOn() || !o.changedOnDisk || o.pending.length || o.dirty || askState || Date.now() - autoAt < 15000) return; // at most every 15 seconds
  const a = document.activeElement;
  if (a && /^(INPUT|TEXTAREA|SELECT)$/.test(a.tagName) && a.id !== 'autorefresh') return;
  autoBusy = true;
  autoAt = Date.now();
  try {
    const y = window.scrollY;
    const r = await api('/api/save/discard', { ifClean: true });
    if (r && r.kept) return; // an edit was made while the status was being read: nothing is thrown away
    await render();
    window.scrollTo(0, y);
  } catch (e) { /* the next poll shows the banner and tries again */ } finally { autoBusy = false; }
}
async function status() {
  const o = await api('/api/save/state');
  offline = false;
  saveBlocked = !!o.saveBlocked;
  lastDirty = !!o.dirty;
  saveBlockWhy = o.blockReason || '';
  $('#path').textContent = String(o.path || '').split(/[\\/]/).pop();
  $('#path').title = o.path || '';
  const gw = $('#gamewarn');
  const note = o.gameMode === 'menu_or_multiplayer';
  const q = o.pending.length;
  const last = o.lastWrite;
  const why = (last && !last.saved && last.why) || saveBlockWhy;
  // When the game saves and nothing is queued, the editor reloads the file by itself (autoRefresh below), so the out-of-date banner is only for automatic refresh switched
  // off. Queued edits are put back onto the game's newer file when they are written, so a changed file needs no word from the person.
  const changed = !!o.changedOnDisk && !q && !autoRefreshOn();
  const restored = o.restored && (o.restored.applied > 0 || (o.restored.conflicts || []).length > 0) ? o.restored : null;
  gw.className = saveBlocked || q || changed || restored || o.writeLogProblem ? 'warn' : note ? 'info' : 'hide';
  const changedMsg = changed ? html`<div><b>The game saved since this editor opened.</b> What you see here is out of date. <button class="b sm" data-act="reload">Reload now</button></div>` : html``;
  // One banner for the whole state. The reason is said once, as a sentence; the tool's own text, when the sentence replaces it, sits under Details.
  // An edit is written as it is made. While the file cannot be written (a single-player session is running, the location is not confirmed) edits are queued, and the queue is
  // written by itself the moment the file can be.
  const reason = why ? humanReason(why) : { text: '', raw: '' };
  const n = (one, many) => (q === 1 ? one : many);
  // a queue the editor could not put back onto the game's newer file: none of it was written
  const heldMsg = o.held ? html`<div><b>${q} queued change${q > 1 ? 's were' : ' was'} not written.</b> The game saved changes that conflict with ${q > 1 ? 'them' : 'it'}, so the editor left the game's file as it is.
    ${(o.held.conflicts || []).map((c) => html`<div>${c.desc}: ${c.why}</div>`)} <button class="b sm" data-act="discardQueue">Discard queue</button></div>` : html``;
  let mainMsg = html``;
  if (o.locationUnconfirmed) mainMsg = html`<div><b>Changes cannot be written here yet.</b> ${reason.text} You can keep editing: changes wait in the queue. <button class="b sm" data-act="confirmLocation">Allow writing here</button></div>`;
  else if (o.gameCheckError && saveBlocked) mainMsg = html`<div><b>Changes cannot be written.</b> ${reason.text}${q ? html` ${q} ${n('change waits', 'changes wait')} in the queue and ${n('is written by itself', 'are written by themselves')} once the check works.` : ''} <button class="b sm" data-act="writeNow">Write now</button></div>`;
  else if (q && !o.held && last && last.stuck) mainMsg = html`<div><b>${q} queued change${n(' cannot', 's cannot')} be written.</b> The game saved after ${n('it was', 'they were')} made, and the editor did not keep ${n('it', 'them')}${o.journalOff ? ' (' + o.journalOff + ')' : ''}, so ${n('it', 'they')} cannot be put back onto the game's file, which is left as it is. Discard the queue and make ${n('the change', 'the changes')} again. <button class="b sm" data-act="discardQueue">Discard queue</button></div>`;
  else if (q && !o.held) mainMsg = html`<div><b>${q} change${n(' is', 's are')} queued.</b> ${reason.text} ${n('It is written to the game file by itself', 'They are written to the game file by themselves')} as soon as it can be written. <button class="b sm" data-act="writeNow">Write now</button></div>`;
  else if (saveBlocked && !q) mainMsg = html`<div><b>The game is in a single-player session.</b> It rewrites the save while it runs, so changes you make now are queued and written when you go to its menu or close it.</div>`;
  else if (note) mainMsg = html`<div>Dune: Awakening is open (${(o.gameProcesses || []).join(', ')}) but not in single-player, so changes are written at once.</div>`;
  // what became of the edits an earlier run (or a reload) put back, and the plain fact when they are not being kept
  // A change that could not be put back is dropped for good: its notice stays, even after the others are written, until it is dismissed.
  const restoredHead = !restored ? html`` : restored.applied > 0
    ? html`<b>${restored.applied} change${restored.applied === 1 ? ' was' : 's were'} put back${restored.rebased ? ' onto the newer file the game saved' : ''}.</b> ${restored.written ? (restored.applied === 1 ? 'It has been written.' : 'They have been written.') : 'They are written like any other change.'}`
    : html`<b>${(restored.conflicts || []).length === 1 ? 'A queued change was' : 'Queued changes were'} not kept.</b>`;
  const restoredMsg = restored ? html`<div>${restoredHead}
    ${(restored.conflicts || []).map((c) => html`<div>Could not put back: ${c.desc} (${c.why}). Make it again.</div>`)} <button class="b sm" data-act="dismissRestored">OK</button></div>` : html``;
  const journalMsg = o.journalOff && o.dirty ? html`<div class="mut">These queued changes are not kept if the editor is closed: ${o.journalOff}.</div>` : html``;
  const logMsg = o.writeLogProblem ? html`<div class="mut">${capFirst(o.writeLogProblem)}. The game's file was written; only the record of it is missing.</div>` : html``;
  const lines = o.pending.map(queueLine);
  const listMsg = q ? html`<details class="bn-list" ${KEEP.qlist ? 'open' : ''}><summary data-keep="qlist">What is queued (${q})</summary><ul>${lines.map((l) => html`<li>${l}</li>`)}</ul></details>` : html``;
  const rawMsg = reason.raw ? html`<details class="bn-raw" ${KEEP.raw ? 'open' : ''}><summary data-keep="raw">Details (include this if you report a problem)</summary><div class="mono">${reason.raw}</div></details>` : html``;
  lastPending = lines;
  const banner = html`${changedMsg}${heldMsg}${mainMsg}${restoredMsg}${journalMsg}${logMsg}${listMsg}${rawMsg}`;
  // redrawn only when it changed: a redraw every few seconds would take the keyboard focus off its buttons
  const key = String(banner).replace(/ ?open>/g, '>').replace(/ >/g, '>'); // a toggle the person opened is already open in the page: that alone is no reason to redraw
  if (key !== gwKey) { gwKey = key; setHTML(gw, banner); }
  $('#pending').textContent = q ? `${q} queued` : '';
  $('#pending').title = lines.join('\n');
  $('#btnDiscard').hidden = !o.dirty;
  const ro = $('#rowarn');
  // A queue that is not kept on disk (the journal is off) is lost when the browser tab closes: ask first (the native window asks through its own close handler).
  window.onbeforeunload = o.dirty && o.journalOff ? (e) => { e.preventDefault(); e.returnValue = ''; } : null;
  ro.className = o.readOnly ? 'warn' : 'hide';
  autoRefresh(o);
  setHTML(ro, o.readOnly ? html`<b>This save is read-only.</b> ${o.readOnly}<br>You can browse it, but every edit is refused. This usually means the file contains database objects the real game does not create (for example extra triggers) - do not edit a save from an untrusted source.` : html``);
  return o;
}
// ---------- confirmations
// Tiers: reversible edits need nothing (they are written as they are made, with a backup first). A hard-to-undo
// or far-reaching action asks first with its consequence in words. The far-reaching ones (bulk changes, raw SQL
// writes, restoring a backup over the save) also make the person type a word, so a stray click cannot start them.
let askState = null, askFrom = null;
function ask({ title, body, pre = '', list = [], ok = 'Confirm', typed = '', danger = false, gap = false }) {
  return new Promise((resolve) => { if (askState) askState.resolve(false); askFrom = askFrom || document.activeElement; askState = { title, body, pre, list, ok, typed, danger, gap, at: Date.now(), resolve }; drawAsk(); });
}
// A question opened with gap: true ignores an OK that comes sooner than ARM_GAP_MS after it opened (the second half of a double click on the button that opened it, a key that repeats).
const tooSoon = () => !!(askState && askState.gap && Date.now() - askState.at < ARM_GAP_MS);
// While a question is open the page behind it is inert (not reachable by Tab, by a screen reader's browse keys or by a click) and does not scroll.
const BEHIND = ['header', '#gamewarn', '#rowarn', '#nav', '#main'];
function setBehind(on) {
  for (const s of BEHIND) { const e = $(s); if (e) { if (on) e.setAttribute('inert', ''); else e.removeAttribute('inert'); } }
  document.body.classList.toggle('noscroll', on);
}
function drawAsk() {
  const el = $('#ask');
  setBehind(!!askState);
  if (!askState) { el.className = 'hide'; setHTML(el, html``); return; }
  const a = askState;
  el.className = 'modal top';
  setHTML(el, html`<div class="mbox small" role="dialog" aria-modal="true" aria-labelledby="atitle" aria-describedby="abody"><h3 class="big" id="atitle" tabindex="-1">${a.title}</h3><p id="abody">${a.body}</p>${a.list.length ? html`<ul class="qlist" tabindex="0" aria-label="The changes">${a.list.map((l) => html`<li>${l}</li>`)}</ul>` : ''}${a.pre ? html`<pre class="sqlpre" tabindex="0" aria-label="The statement">${a.pre}</pre>` : ''}
    ${a.typed ? html`<p class="mut">Type <b>${a.typed}</b> to continue.</p><input id="atype" spellcheck="false" size="20">` : ''}
    <div class="row mt12"><button class="b ${a.danger ? 'bad' : ''}" id="aok" data-act="askOk" ${a.typed ? 'disabled' : ''}>${a.ok}</button><button class="b sec" data-act="askNo">Cancel</button></div></div>`);
  (a.typed ? $('#atype') : $('#atitle')).focus();
}
function endAsk(v) {
  const a = askState, back = askFrom;
  askState = null; askFrom = null;
  drawAsk();
  if (back && back.isConnected && typeof back.focus === 'function') back.focus(); // return to where the person was
  if (a) a.resolve(v);
}
document.addEventListener('input', (e) => { if (askState && e.target.id === 'atype') $('#aok').disabled = e.target.value.trim().toLowerCase() !== askState.typed.toLowerCase(); });

// ---------- one-button confirmation
// A button with data-confirm acts on every base at once and is written at once, so the first press only asks: the same button turns into "Yes, ..." for a few seconds and
// the second press runs it. Pressing anything else, leaving the button or pressing Escape cancels. It is inline on purpose (no dialog to trap the keyboard) and the
// change of wording is also said aloud through the page's live region.
const ARM_MS = 15000;
const ARM_GAP_MS = 400; // a second press sooner than this after the first is the same movement of the hand (a double click, a bouncing switch), not a decision
let armed = null; // { btn, label, timer, at }
const sayAloud = (msg) => { const r = $('#sr'); if (r) r.textContent = msg; };
function disarm() {
  if (!armed) return;
  clearTimeout(armed.timer);
  const { btn, label } = armed;
  armed = null;
  if (btn.isConnected) { btn.textContent = label; btn.removeAttribute('data-armed'); }
  sayAloud('');
}
function arm(btn) {
  disarm();
  armed = { btn, label: btn.textContent, timer: setTimeout(disarm, ARM_MS), at: Date.now() };
  btn.dataset.armed = '1';
  btn.textContent = btn.dataset.confirm;
  sayAloud(`${btn.dataset.confirm}? Press the button again within ${ARM_MS / 1000} seconds to do it, or press Escape to cancel.`);
}

// ---------- reset buttons (#339, part of #112)
// One button per area (Skills, Level and XP, Journey) and never one for everything; pressing each of them is the "reset everything". A reset has two steps: the
// button opens a question that says what changes and that the backup puts it back, and asks for a sentence naming the area and the number, typed in full (an OK that
// comes sooner than ARM_GAP_MS after the question opened is ignored); then the request carries that number, the editor reads the save again inside the edit and
// refuses with a conflict (409) when the number is no longer the one that was confirmed, and the page is redrawn so that the new number is on it.
const UNDO_NOTE = 'A backup of the save is taken before the change is written to the game file, and Restore on the Database tab (Save backups) puts it back.';
async function resetArea({ count, title, body, list = [], typed, ok, path, extra = {}, done, none }) {
  if (!(await ask({ title, body: `${body} ${UNDO_NOTE}`, list, ok, typed, danger: true, gap: true }))) return;
  await act(async () => {
    try {
      const r = await api(path, { expected: count, ...extra });
      toast((r.noop ? none : done(r)) + NOT_SAVED);
    } catch (e) { if (e.code === 'conflict') setTimeout(render, 0); throw e; }
  }, null);
}

// ---------- the queue: an edit is written to the game's file as it is made; while the file cannot be written (the game is in a single-player session) edits wait here
$('#btnDiscard').onclick = async () => { if (await ask({ title: 'Discard the queued changes?', body: `These ${lastPending.length} ${lastPending.length === 1 ? 'change is' : 'changes are'} waiting for the game to close and would be dropped. The game file is not touched.`, list: lastPending, ok: 'Discard', danger: true })) act(() => api('/api/save/discard', {}), 'Discarded', true, true); };

// ---------- clear a container (#338)
// One row under a container's item list: a box for the currency, and a button that asks twice (the two-press rule of data-confirm, then a typed count). The count
// the person types is the number of items the clear will remove, and it is sent with the request: the editor refuses when the container holds another number by then.
const SOLARI = 'SolarisCoin';
function clearRow(inv, name, rows) {
  const all = rows.length, kept = rows.filter((r) => r.template_id === SOLARI).length, plain = all - kept;
  const note = all ? `Removes the ${nn(plain, 'item', 'items')} in ${name}, with everything attached to them. The Solari Coin stack${kept === 1 ? ' is' : 's are'} kept unless the box is ticked. Two presses, then type the count.` : `${name} is empty: nothing to clear.`;
  return html`<div class="row"><label><input type="checkbox" id="clrCur"> Also remove currency</label><button class="b bad" data-act="clearInv" data-confirm="Yes, clear this container" data-inv="${inv}" data-name="${name}" data-plain="${plain}" data-all="${all}" aria-describedby="clrnote" ${all ? '' : 'disabled'}>Clear this container</button><span class="mut" id="clrnote">${note}</span></div>`;
}

// ---------- PLAYER
let P = null;
function host() { return $('#pbody') || $('#main'); }
// The Player Summary above the Player tabs, as in the console's player view: identity, level, XP, skill points, Intel, vitals,
// faction, database ids and the currency balances. Everything comes from the save.
// The identifiers that name the account (platform, Funcom and FLS ids) stay hidden until their Show button is pressed, so a screenshot taken
// for a bug report does not carry them (GRC-5). The value is only drawn while it is shown; the page masks the same ids.
let SUMMARY = null;
const SHOWN = new Set();
const SECRET_IDS = ['platformId', 'funcomId', 'flsId'];
const idKv = (key, label, v) => (v == null || v === '' ? kv(label, '-')
  : html`<div class="kv"><b>${label}</b><span>${SHOWN.has(key) ? v : '••••••••'} <button class="b sec sm" data-reveal="${key}" title="Account identifiers are hidden so a screenshot does not share them">${SHOWN.has(key) ? 'Hide' : 'Show'}</button></span></div>`);
function summaryCard(m) {
  return html`<div id="sumcard">${summaryBody(m)}</div>`;
}
function summaryBody(m) {
  if (!m) return html``;
  const num = (v) => (v == null ? '-' : Number(v).toLocaleString());
  const f1 = (v) => (v == null ? '-' : Number(v).toFixed(1));
  const id = m.identity || {}, ids = m.ids || {}, v = m.vitals || {}, sp = m.skillPoints, it = m.intel;
  return html`<div class="card"><h3>Player summary</h3><div class="grid">
    ${kv('Character', m.name)}${kv('Status', m.status)}${kv('Map', m.map)}${kv('Faction', m.faction || 'Neutral')}
    ${kv('Level', m.level == null ? '-' : m.level)}${kv('XP', num(m.xp))}${kv('Skill points', sp ? `${num(sp.unspent)} unspent, ${num(sp.total)} earned` : '-')}${kv('Intel', it ? `${num(it.points)} of ${num(it.max)}` : '-')}
    ${kv('Health', v.health == null ? '-' : `${f1(v.health)} (max ${Math.ceil(Math.max(v.healthMinMax, v.health))} or more)`)}${kv('Hydration', v.hydration == null ? '-' : `${f1(v.hydration)} of ${v.maxHydration}`)}${kv('Spice addiction', v.spiceAddiction == null ? '-' : `${f1(v.spiceAddiction)} of ${v.maxSpiceAddiction}`)}
    ${(m.currency || []).map((c) => kv(c.label, num(c.balance)))}</div>
    <details class="ids"><summary>Account and database ids</summary><div class="grid">
    ${idKv('platformId', (id.platform || 'Platform') + ' ID', id.platformId)}${idKv('funcomId', 'Funcom ID', id.funcomId)}${idKv('flsId', 'FLS ID', id.flsId)}
    ${kv('DB player', ids.actor)}${kv('Account', ids.account)}${kv('Player controller', ids.controller)}${kv('Player state', ids.playerState)}</div></details></div>`;
}
async function playerView() {
  let sm = null;
  try { sm = await api('/api/player/summary'); } catch (e) { sm = null; } // the tabs still work without it
  SUMMARY = sm;
  SHOWN.clear(); // hidden again every time the view is drawn
  setHTML($('#main'), html`${summaryCard(sm)}${subNav('player', PTABS)}<div id="pbody"></div>`);
  if (sub.player == 'bases') return basesView();
  if (sub.player == 'vehicles') return vehiclesView();
  return playerSection(sub.player);
}
const NO_TRACK_NUMBER = 'This track\'s number is not known yet: see the note above';
// The reset of the character's level and XP (#339): on Character > Overview, beside Give XP. The number it asks the person to confirm is the XP the summary above shows.
function levelResetCard() {
  const xp = SUMMARY && Number.isInteger(SUMMARY.xp) ? SUMMARY.xp : null;
  const note = xp === null ? 'This save has no level record for the character.' : xp > 0 ? 'The XP goes back to 0 and the level with it, and the skill points the levels gave are taken back. Skills you bought stay. Asks first.' : 'Nothing to reset: the character has no XP.';
  return html`<div class="card"><h3>Reset level and XP</h3><div class="row"><button class="b bad" data-act="lvReset" aria-describedby="lvresetnote" ${xp ? '' : 'disabled'}>Reset level and XP (${Number(xp || 0).toLocaleString()} XP)</button><span class="mut" id="lvresetnote">${note}</span></div></div>`;
}
async function playerSection(s) {
  const h = s == 'character' ? [subNav('char', [['overview', 'Overview'], ['inventory', 'Inventory'], ['reputation', 'Reputation']])] : [];
  const c = s == 'character' ? sub.char || 'overview' : s;
  if (c == 'overview') {
    P = await api('/api/player');
    const a = P.actor || {}, j = P.journey || {};
    h.push(html`<div class="card"><h3>${P.name}</h3><div class="grid">${kv('Solari in backpack', P.solari.toLocaleString())}${kv('Position', `${fix(a.x)}, ${fix(a.y)}, ${fix(a.z)}`)}${kv('Journey', `${(j.done || 0).toLocaleString()} of ${(j.total || 0).toLocaleString()} nodes done`)}</div><p class="mut">Map, account, platform and the database ids are in the summary above.</p></div>
    <div class="card"><h3>Money</h3><div class="row"><b>Solari in backpack</b><input id="solari" type="number" value="10000"><button class="b" data-act="solari">Add or remove</button><span class="mut">A negative number removes. These are the Solari stacks in your backpack.</span></div>
    <div class="row"><b>Give to a wallet</b><select id="curSel"><option value="0">Solari in wallet</option><option value="1">House credit in wallet</option></select><input id="curAmt" type="number" value="100" min="1"><button class="b" data-act="currency">Give</button><span class="mut">A wallet balance, kept apart from the Solari stacks in your backpack.</span></div></div>
    <div class="card"><h3>Quick rewards</h3><div class="row"><b>Give XP</b><input id="xpAmt" type="number" value="1000" min="1"><button class="b" data-act="xp">Give</button><span class="mut">Raises the level as the XP grows and adds one skill point per level gained; stops at level 200.</span></div><div class="row"><b>Give Intel</b><input id="intelAmt" type="number" value="100" min="1"><button class="b" data-act="intel">Give</button><span class="mut">Adds Intel for the research tree, never past 2,779. All of these apply when the game next loads the save.</span></div></div>
    ${levelResetCard()}`);
  } else if (c == 'inventory') {
    const d = await api('/api/player/inventory');
    const cat = await loadCatalog(d.templates);
    // The console's four groups of the character's carried inventories, with a filter over name, item id, template and slot.
    const G = [['backpack', 'Backpack'], ['character', 'Character'], ['loadout', 'Loadout'], ['schematics', 'Unique schematics']];
    const groupOf = (r) => (r.inventory_type == 1 ? 'character' : r.inventory_type == 15 ? 'loadout' : r.inventory_type == 30 ? 'schematics' : 'backpack');
    const counts = Object.fromEntries(G.map(([k]) => [k, d.items.filter((r) => groupOf(r) == k).length]));
    const grp = invG || (G.find(([k]) => counts[k]) || G[0])[0];
    const terms = invQ.toLowerCase().split(' ').filter(Boolean);
    const rows = d.items.filter((r) => groupOf(r) == grp && terms.every((t) => `${itemName(r.template_id)} ${r.template_id} ${r.id} ${r.position_index}`.toLowerCase().includes(t)));
    const cap = (d.inventories || []).find((i) => groupOf(i) == grp);
    const slots = cap && cap.max_item_count > 0 ? `, ${counts[grp]} of ${cap.max_item_count} slots used` : '';
    const invCard = html`<div class="card"><h3>Inventory: ${nn(d.items.length, 'item', 'items')} carried</h3>
      <div class="sub">${G.map(([k, v]) => html`<button data-invg="${k}" class="${grp == k ? 'on' : ''}">${v} (${counts[k]})</button>`)}</div>
      <div class="row"><input id="invq" placeholder="Filter by name, item ID, or template" size="34" value="${invQ}"><button class="b sec" data-act="invfilter">Filter</button><button class="b sec" data-act="invclear">Clear</button><span class="mut">${countText(rows.length, counts[grp], 'item', 'items', terms.length > 0)}${slots}</span></div>
      ${cap && grp != 'schematics' ? clearRow(cap.id, G.find(([k]) => k == grp)[1], d.items.filter((r) => r.inventory_id == cap.id)) : ''}
      ${tbl([{ k: 'position_index', label: 'Slot' }, { k: 'template_id', label: 'Item', f: (r) => itemCell(r.template_id) },
        { k: 'stack_size', label: 'Stack', f: (r) => html`<input type="number" value="${r.stack_size}" min="1" data-item="${r.id}" data-field="stack_size" data-name="${itemName(r.template_id)}" class="w90" aria-label="Stack of ${itemName(r.template_id)}">` },
        { k: 'quality_level', label: 'Grade', f: (r) => html`<select data-item="${r.id}" data-field="quality" data-name="${itemName(r.template_id)}" aria-label="Grade of ${itemName(r.template_id)}">${[0, 1, 2, 3, 4, 5].map((n) => html`<option ${n == r.quality_level ? 'selected' : ''}>${n}</option>`)}</select>` },
        { k: 'durability', label: 'Durability', f: (r) => (r.durability == null ? '' : Number(r.durability).toFixed(1) + (r.max_durability ? ' / ' + Number(r.max_durability).toFixed(0) : '')) },
        { k: 'augments', label: 'Augments', f: (r) => (r.augments || []).map((a) => html`<span class="tag" title="${a.name}${a.quality == null ? '' : ' (quality ' + a.quality + ')'}">${itemName(a.name)}</span>`) }], rows,
        { actions: (r) => html`${r.aug_limit ? html`<button class="b sec sm" data-act="augEdit" data-id="${r.id}">Augments</button>` : ''}<button class="b bad sm" data-act="delItem" data-id="${r.id}" data-name="${itemName(r.template_id)}">Delete</button>`, empty: terms.length ? 'No item in this group matches the filter.' : 'Nothing in this group.' })}</div>`;
    const augCard = augItem ? html`<div class="card"><h3>Augments: ${itemName(augItem.template_id)}</h3>
      <p class="mut">Holds up to ${augItem.limit}. Pick what to apply; an augment already on the item keeps its grade, new ones get the grade below. Choose (none) in every slot to remove them all.</p>
      <div class="row">${augSlots('au', augItem, augItem.applied.map((a) => a.id))}</div>
      <div class="row">${augGradeSel('augG')}<label><input type="checkbox" id="augU" checked> Also unlock the augment slots (specialization keystones)</label>
      <button class="b" data-act="augApply" data-id="${augItem.item_id}">Apply</button><button class="b sec" data-act="augCancel">Cancel</button></div></div>` : '';
    h.push(augCard);
    h.push(html`<div class="card"><h3>Repair</h3><div class="row"><button class="b" data-act="repair">Repair all worn items and loadouts</button><span class="mut">Everything you wear and everything in your loadout is made as good as new: full durability, with a maximum that repairs wore down restored. The backpack is not touched.</span></div></div>`);
    h.push(html`<div class="card"><h3>Give item</h3><div class="row">${itemPicker("gt", cat)}
    Qty<input id="gq" type="number" value="1" min="1">Grade<select id="gg">${[0, 1, 2, 3, 4, 5].map((n) => html`<option>${n}</option>`)}</select><button class="b" data-act="give">Give</button><button class="b sec" data-act="queueAdd">Add to queue</button><button class="b sec" data-act="giveWater" title="Puts 10 Cups of Water in the backpack; it does not fill the character's own water (use Fill hydration)">Give water</button><button class="b sec" data-act="fillWater" title="Fill the character's own hydration to the maximum">Fill hydration</button><button class="b sec" data-act="refill">Refill Literjons</button></div>
    <div class="row" id="gaug"></div>
    ${giveQueue.length ? html`<div class="row"><b>Queue (${giveQueue.length})</b>${giveQueue.map((q, i) => html`<span class="tag">${q.quantity} x ${itemName(q.template_id)}${q.quality ? ' (grade ' + q.quality + ')' : ''}${q.augments && q.augments.length ? ' + ' + q.augments.map((a) => augName(a)).join(', ') : ''} <button class="b sec sm" data-act="queueDel" data-id="${i}" title="Remove from the queue">x</button></span>`)}<button class="b" data-act="queueGive">Give queued items</button><button class="b sec" data-act="queueClear">Clear</button></div>` : ''}
    <p class="mut">Pick an item by its in-game name. Items already in your save that the catalog does not know are listed by their template id; any valid template id can also be typed.</p></div>
    ${invCard}`);
  } else if (c == 'reputation') {
    const f = await api('/api/player/factions');
    h.push(html`<div class="card"><h3>Faction reputation</h3>${tbl([{ k: 'name', label: 'Faction' }, { k: 'reputation', label: 'Reputation', f: (r) => html`<input type="number" min="0" max="12474" value="${r.reputation}" data-faction="${r.faction_id}" data-name="${r.name}" class="w100" aria-label="Reputation with ${r.name}">` }], f, { empty: 'This save has no factions.' })}<p class="mut">Range 0–12474. Edit and press Tab/Enter.</p></div>`);
  } else if (s == 'specialization') {
    const sp = await api('/api/player/specs');
    const known = (r) => r.number != null;
    const keystone = (r) => (r.granted ? html`<span class="ok" title="${r.keystoneOwned} of ${r.keystoneTotal} keystones">Granted</span>` : r.keystoneOwned > 0 ? html`<span title="${r.keystoneOwned} of ${r.keystoneTotal} keystones bought">${r.keystoneOwned}/${r.keystoneTotal}</span>` : html`<span class="mut">-</span>`);
    h.push(html`<div class="card"><h3>Specialization tracks</h3>
      <div class="row"><button class="b" data-act="spKeyAll">Grant all keystones</button><button class="b bad" data-act="spKeyReset">Reset all keystones</button></div>
      <div class="row"><button class="b bad" data-act="spResetAll" data-n="${sp.resetCount || 0}" data-left="${sp.unassigned.length}" aria-describedby="spresetnote" ${sp.resetCount ? '' : 'disabled'}>Reset XP of all known tracks (${sp.resetCount || 0})</button>
      <span class="mut" id="spresetnote">${sp.resetCount ? 'Removes the XP of every track whose number is known (the rows the table below shows XP for), in one edit, after you type a confirmation. Keystones are not touched.' : 'Nothing to reset: no track whose number is known has XP.'}${sp.unassigned.length ? ` ${nn(sp.unassigned.length, 'row', 'rows')} of the save whose track is not known yet ${sp.unassigned.length === 1 ? 'is' : 'are'} left alone.` : ''}</span></div>
      ${sp.rows.some(known) ? '' : html`<div class="info" id="spnone"><b>No track has a number in this save yet</b>, so the XP controls below are off. Earn a little XP in a specialization track in the game, let it save, then come back to this tab: the track appears under Track numbers, where you assign it. Keystones need no numbers and work now.</div>`}
      <p class="mut">Changes are written to the game file as you make them and show in the game after it loads the save. Max XP is ${Number(sp.maxXp).toLocaleString()}.</p>
      ${tbl([{ k: 'track', label: 'Track' }, { k: 'xp', label: 'XP', f: (r) => (known(r) ? Number(r.xp).toLocaleString() : html`<span class="mut" title="This track's number in your save is not known yet; see below">?</span>`) },
        { k: 'level', label: 'Level', f: (r) => (known(r) ? Math.floor(r.level) : html`<span class="mut">?</span>`) }, { k: 'keystoneOwned', label: 'Keystone', f: keystone },
        { k: 'add', label: 'Add XP', f: (r) => html`<input id="spx${r.track}" type="number" value="1000" class="w90" aria-label="XP to add to ${r.track}" ${known(r) ? '' : 'disabled'}><button class="b sm" data-act="spAdd" data-t="${r.track}" ${known(r) ? '' : 'disabled'} title="${known(r) ? '' : NO_TRACK_NUMBER}">Add XP</button>` }],
        sp.rows, { empty: 'This save has no specialization tracks.', actions: (r) => html`<button class="b sec sm" data-act="spMax" data-t="${r.track}" ${known(r) ? '' : 'disabled'} title="${known(r) ? '' : NO_TRACK_NUMBER}">Grant max</button><button class="b bad sm" data-act="spReset" data-t="${r.track}" ${known(r) ? '' : 'disabled'} title="${known(r) ? '' : NO_TRACK_NUMBER}">Reset</button>` })}</div>
    <div class="card"><h3>Track numbers</h3>
      <p class="mut">The game's own database names its specialization tracks. Your save stores only a number for each one and does not say which number is which, so the XP controls above need to know the number of each track. It is learned from your own save: earn a little XP in a track in game, save, reload this tab, and the new row appears below to assign to the track you used. Keystones need no numbers.</p>
      ${sp.unassigned.length ? html`${sp.unassigned.map((u) => html`<div class="row"><span>Track number <b>${u.number}</b>: ${Number(u.xp).toLocaleString()} XP, level ${Math.floor(u.level)}</span> is
        <select id="spa${u.number}">${sp.tracks.map((t) => html`<option>${t}</option>`)}</select><button class="b" data-act="spAssign" data-n="${u.number}">Assign</button></div>`)}` : html`<p class="mut">No specialization XP in your save is waiting to be assigned.</p>`}
      <div class="row"><span class="mut">Known a track's number already?</span><select id="spmt">${sp.tracks.map((t) => html`<option>${t}</option>`)}</select> number <input id="spmn" type="number" min="0" max="255" class="w70"><button class="b sec" data-act="spAssignHand">Set</button></div>
      <p class="mut">Assigned: ${sp.rows.filter(known).map((r) => `${r.track} = ${r.number}`).join(', ') || 'none yet'}.</p></div>
    <div class="card"><details><summary>Advanced: set a raw row</summary><p class="mut">Writes the XP and level you type for a track number directly. A wrong number changes another track.</p><div class="row"><span class="mut">Track number</span><input id="st" type="number" value="0" class="w70">XP<input id="sx" type="number" value="0">Level<input id="sl" type="number" step="0.1" value="1" class="w80"><button class="b sec" data-act="specSet">Set</button></div></details></div>`);
  } else if (s == 'journey') {
    JB = await api('/api/player/journey/browse');
    const tg = await api('/api/player/tags');
    h.push(html`<div class="card"><h3>Journey browser</h3><div id="jhead"></div><div id="jout"></div></div>`);
    h.push(html`<div class="card"><h3>Player tags</h3>${tg.length ? tg.map((t) => html`<span class="tag">${t.tag} <a href="#" data-act="tagDel" data-tag="${t.tag}" aria-label="Remove tag ${t.tag}" title="Remove tag ${t.tag}">×</a></span>`) : html`<span class="mut">none</span>`}<div class="row"><input id="tag" placeholder="Tag.Name" size="40"><button class="b" data-act="tagAdd">Add tag</button></div></div>`);
  } else if (s == 'skills') {
    SK = await api('/api/player/skills');
    h.push(html`<div class="card"><h3>Skill point controls</h3>
      <div class="row"><span>Earned in total <b>${SK.total}</b></span><span>Spent in skills <b>${SK.spent}</b></span><span>Unspent <b>${SK.unspent}</b></span></div>
      <div class="row">Unspent points<input id="skPts" type="number" min="0" max="100000" value="${SK.unspent}" class="w90"><button class="b" data-act="skPoints">Set unspent points</button></div>
      <p class="mut">The save does not keep spent plus unspent equal to the total (some skills are granted free), so these three numbers are shown as stored and not worked out from each other.</p></div>
    <div class="card"><h3>Skill browser</h3><div id="skhead"></div><div id="skout"></div></div>`);
  } else if (s == 'research') {
    RS = (await api('/api/player/research')).rows;
    h.push(html`<div class="card"><h3>Research</h3><div class="row"><button class="b" data-act="rsAll">Unlock all research</button><span class="mut">Buys every entry that unlocks a recipe or a building set and adds what it unlocks.</span></div><div id="rshead"></div><div id="rsout"></div></div>`);
  } else if (s == 'crafting') {
    CR = (await api('/api/player/crafting')).rows;
    h.push(html`<div class="card"><h3>Crafting recipes</h3><div class="row"><button class="b" data-act="crAll">Unlock all recipes</button><span class="mut">Adds every recipe your research tree offers that you do not know yet.</span></div><div id="crhead"></div><div id="crout"></div></div>`);
  } else if (s == 'buildingsets') {
    CT = await api('/api/player/building-sets');
    CT.kind = 'bs';
    ctGroup = '';
    h.push(html`<div class="card"><h3>Building sets</h3><div class="row"><button class="b" data-act="bsAll">Unlock all building sets</button><span class="mut">Teaches every set below except the experimental ones, without putting the items in your backpack.</span></div><div id="cthead"></div><div id="ctout"></div></div>
      <div class="card"><h3>New buildable pieces (${CT.newPieces.length})</h3>${CT.newPieces.map((x) => html`<span class="tag">${x.name}</span>`)}</div>`);
  } else if (s == 'customizations') {
    CT = await api('/api/player/customizations');
    CT.kind = 'cu';
    ctGroup = '';
    h.push(html`<div class="card"><h3>Customizations</h3>
      ${CT.libraryAvailable ? html`<div class="row"><button class="b" data-act="ctUnlockSet" data-id="all" data-name="all five sets">Unlock all sets</button><span class="mut">Adds the cosmetics of the five sets to your character's customization library (${CT.libraryCount} unlocked now): no backpack space needed. Store and DLC cosmetics are skipped, because the game removes the ones your account does not own.</span></div>` : html`<p class="mut">This save has no customization library for the character, so cosmetics can only be given as items.</p>`}
      <div class="row"><button class="b sec" data-act="ctGrantSet" data-id="all" data-name="all five sets">Give all sets as items</button><span class="mut">Puts one cosmetic item of each in your backpack instead, all or nothing: there are more of them than the backpack holds, so give one set at a time.</span></div><div id="cthead"></div><div id="ctout"></div></div>`);
  } else if (s == 'admin') {
    P = await api('/api/player');
    const a = P.actor || {};
    const fa = await api('/api/player/faction');
    h.push(html`<div class="card"><h3>Admin actions</h3><p class="mut">Faction, teleport and the vendor purchase limits. Repair is on Character &gt; Inventory (gear) and on Vehicles.</p></div>
    <div class="card"><h3>Faction assignment</h3><div class="row"><b>Current</b><span>${fa.current ? fa.current.name : 'none recorded'}</span>
      <select id="facSel">${fa.options.map((o) => html`<option value="${o.id}" ${fa.current && fa.current.id == o.id ? 'selected' : ''}>${o.name}</option>`)}</select><button class="b" data-act="faction">Change faction</button>
      <span class="mut">Changes which faction the character belongs to. Reputation is under Character &gt; Reputation.</span></div></div>
    <div class="card"><h3>Repair</h3><div class="row"><b>Gear and loadouts</b><button class="b sec" data-goto="character:inventory">Open Character &gt; Inventory</button><b>Vehicles</b><button class="b sec" data-goto="vehicles">Open Vehicles</button></div></div>
    <div class="card"><h3>Movement</h3><div class="row"><b>Teleport to</b>X<input id="tx" type="number" value="${Math.round(a.x)}">Y<input id="ty" type="number" value="${Math.round(a.y)}">Z<input id="tz" type="number" value="${Math.round(a.z)}">Facing <input id="tdyaw" type="number" step="1" placeholder="degrees" class="w90"><button class="b" data-act="teleport">Teleport</button></div>
    <div class="row"><b>Go to a place</b><select id="tdkind"><option value="base">Base</option><option value="respawn">Respawn point</option><option value="vehicle">My vehicle</option><option value="marker">Discovered map marker</option></select><input id="tdq" placeholder="filter, e.g. cave" class="w150"><button class="b sec" data-act="tdFind">Find</button><span id="tdout" class="mut"></span></div>
    <p class="mut">Moves the character in the save. The game keeps its own copy of the save in memory while single-player runs and overwrites the file, so the move shows when you next enter single-player. Choosing a place fills X, Y and Z. Places are on the map the character is on now. Facing is optional: 0 looks along +X; leave it empty to keep the way the character faces. If you type a Z yourself, stay above the terrain or you may fall through.</p>
    </div>`);
    const ve = await api('/api/vendors');
    await loadCatalog(ve.stock.map((x) => x.template_id));
    h.push(html`<div class="card"><h3>Vendor purchase limits</h3><p class="mut">Vendors limit how much you can buy per restock cycle. Resetting clears those counters.</p><div class="row"><button class="b" data-act="vreset" data-v="">Reset all vendors</button></div>
      ${tbl([{ k: 'vendor_id', label: 'Vendor ID' }, { k: 'template_id', label: 'Item', f: (r) => itemCell(r.template_id) }, { k: 'amount_bought', label: 'Bought' }], ve.stock, { actions: (r) => html`<button class="b sec sm" data-act="vreset" data-v="${r.vendor_id}">Reset vendor</button>`, empty: 'No purchases are counted against any vendor, so there is nothing to reset.' })}
      <h3 class="mt14">Restock cycles</h3>${tbl([{ k: 'vendor_id', label: 'Vendor ID' }, { k: 'last_interacted_timestamp', label: 'Last interaction', f: (r) => fmtEpoch(r.last_interacted_timestamp) }], ve.cycles, { empty: 'No vendor has been visited yet.' })}</div>`);
  } else {
    h.push(html`<div class="card"><p class="mut">There is nothing on this page.</p></div>`);
  }
  setHTML(host(), html`${h}`);
  if (s == 'journey') journeyList();
  if (s == 'skills') skillsList();
  if (s == 'research') researchList();
  if (s == 'buildingsets' || s == 'customizations') catalogList();
  if (s == 'crafting') craftingList();
}
// The Journey browser, as the console's: Story, Contract, Codex and Tutorial lists with readable names, the tree depth, and status.
// Story, Codex and Tutorial rows can be completed or reset (a story or codex node takes its children with it); contracts are read-only here.
let JB = null, jgrp = 'story'; // JB: the last browse result
const JGROUPS = [['story', 'Story'], ['contract', 'Contract'], ['codex', 'Codex'], ['tutorial', 'Tutorial']];
const JMAX = 400;
function journeyList() {
  if (!JB || !$('#jout')) return;
  const q = (window.jq || '').trim().toLowerCase();
  const all = JB[jgrp] || [];
  const rows = q ? all.filter((r) => r.name.toLowerCase().includes(q) || r.id.toLowerCase().includes(q)) : all;
  const jn = JB.resetNodes || 0, jt = JB.resetTutorials || 0;
  setHTML($('#jhead'), html`<div class="sub">${JGROUPS.map(([k, v]) => html`<button data-jgrp="${k}" class="${jgrp == k ? 'on' : ''}">${v} (${(JB[k] || []).length})</button>`)}</div>
    <div class="row"><button class="b bad" data-act="jReset" data-n="${JB.resetCount || 0}" data-nodes="${jn}" data-tuts="${jt}" aria-describedby="jresetnote" ${JB.resetCount ? '' : 'disabled'}>Reset journey (${nn(jn, 'step', 'steps')}, ${nn(jt, 'tutorial', 'tutorials')})</button>
    <span class="mut" id="jresetnote">${JB.resetCount ? 'Every finished Story and Codex step goes back to not finished and every started or finished tutorial back to not started, in one edit, after you type a confirmation. Waiting rewards, tags and contracts are not touched.' : 'Nothing to reset: no Story or Codex step is finished and no tutorial is started.'}</span></div>
    <div class="row"><input id="jq" placeholder="Filter by name or node id" size="34" value="${window.jq || ''}"><button class="b sec" data-act="jfilter">Filter</button><button class="b sec" data-act="jclear">Clear</button>
    <span class="mut">${countText(rows.length, all.length, 'node', 'nodes', !!q)}${rows.length > JMAX ? `, the first ${JMAX} shown: narrow the filter to see others` : ''}. ${jgrp == 'contract' ? 'Contracts are shown read-only; a contract is complete when all its tags are on the character.' : 'Completing a node completes its children; resetting clears it and its children.'}</span></div>`);
  const stat = (r) => (r.status == 'Complete' ? html`<span class="ok">Complete</span>` : r.status == 'Revealed' || r.status == 'Started' ? html`<span class="warn">${r.status}</span>` : html`<span class="mut">${r.status}</span>`);
  const act = (r) => {
    if (!r.actionable) return '';
    const kind = jgrp == 'tutorial' ? 'tut' : 'jset';
    return r.complete ? html`<button class="b sec sm" data-act="${kind}" data-id="${r.id}" data-c="0">Reset</button>` : html`<button class="b sm" data-act="${kind}" data-id="${r.id}" data-c="1">Complete</button>`;
  };
  setHTML($('#jout'), tbl([{ k: 'name', label: 'Name', f: (r) => html`<span class="ind${Math.min(r.depth || 0, 8)}" title="${r.id}">${r.name}</span>${r.pendingReward ? html` <span class="tag">reward pending</span>` : ''}` },
    { k: 'status', label: 'Status', f: stat }, { k: 'tags', label: 'Tags', f: (r) => (r.tags ? r.tags : '') }], rows.slice(0, JMAX), { actions: act, empty: q ? 'No node matches the filter.' : 'This list is empty.' }));
}

// The Skill Browser, as the console's: a school at a time, each skill with a rank bar and a rank control. A change is one edit in the
// the queue. By default it sets the skill only, as Dune Docker's control does; "Pay for it" also takes the points from, or gives them
// back to, the unspent points.
let SK = null, skSchool = 'Trooper', skCharge = false;
const SKOTHER = ['Hidden', 'Other'];
function skillsList() {
  if (!SK || !$('#skout')) return;
  const groups = [...SK.schools.map((x) => [x.key, x.label]), ['Other', 'Other']];
  const inGroup = (m) => (skSchool == 'Other' ? SKOTHER.includes(m.school) : m.school == skSchool);
  const rows = SK.modules.filter(inGroup);
  const count = (k) => SK.modules.filter((m) => (k == 'Other' ? SKOTHER.includes(m.school) : m.school == k) && m.rank > 0).length;
  setHTML($('#skhead'), html`<div class="sub">${groups.map(([k, v]) => html`<button data-skgrp="${k}" class="${skSchool == k ? 'on' : ''}">${v} (${count(k)})</button>`)}</div>
    <div class="row"><label><input type="checkbox" id="skCharge" ${skCharge ? 'checked' : ''}> Pay for changes from the unspent points (and give them back when lowering)</label>
    ${skSchool != 'Other' ? html`<button class="b sec" data-act="skStarter">Restore starter skills</button>` : ''}
    ${skSchool != 'Other' ? html`<button class="b" data-act="skMax" data-school="${skSchool}">Max all ${groups.find(([k]) => k == skSchool)[1]} skills</button>` : ''}<button class="b" data-act="skMax" data-school="all">Max all skills of all schools</button>
    <span class="mut">${nn(rows.length, 'skill', 'skills')}, ${rows.filter((m) => m.rank > 0).length} learned.</span></div>
    <div class="row"><button class="b bad" data-act="skReset" aria-describedby="skresetnote" ${SK.resetCount ? '' : 'disabled'}>Reset all skills to rank 0 (${SK.resetCount || 0})</button>
    <span class="mut" id="skresetnote">${SK.resetCount ? 'Every skill of the five schools that has points goes back to rank 0, in one edit, after you type a confirmation. Hidden skills and skills the catalog does not know are left alone.' : 'Nothing to reset: every skill of the five schools is already at rank 0.'}</span></div>`);
  const bar = (m) => (m.known ? html`<span class="rankbar" title="rank ${m.rank} of ${m.max}">${'●'.repeat(m.rank)}${'○'.repeat(Math.max(0, m.max - m.rank))}</span>` : html`<span class="mut" title="not in the catalog: ${m.spent} points spent">?</span>`);
  setHTML($('#skout'), tbl([{ k: 'name', label: 'Skill', f: (m) => html`<span title="${m.id}">${m.name}</span>` }, { k: 'kind', label: 'Type' }, { k: 'rank', label: 'Rank', f: bar },
    { k: 'spent', label: 'Points', f: (m) => m.spent },
    { k: 'max', label: 'Set rank', f: (m) => (m.known ? html`<select data-skill="${m.id}" data-name="${m.name}" aria-label="Rank of ${m.name}">${Array.from({ length: m.max + 1 }, (_, i) => html`<option ${i == m.rank ? 'selected' : ''}>${i}</option>`)}</select>` : html`<span class="mut">catalog does not know its ranks</span>`) }], rows, { empty: 'No skills in this school.' }));
}

// Research and crafting recipes, as in Dune Docker's Research and Crafting tabs: a category, a filter, and an Unlock on each entry that is not
// there yet. Unlocking research marks it purchased and adds the recipe or building set it unlocks; a purchased entry whose recipe or set is
// missing shows Repair unlock.
let RS = null, CR = null, rsCat = '', rsGroup = '', rsQ = '', crCat = '', crQ = '';
const RCATS = ['Water Discipline', 'Combat', 'Construction', 'Exploration', 'Vehicles', 'Augmentations', 'Uniques', 'Essentials'];
const LISTMAX = 400;
function researchList() {
  if (!RS || !$('#rsout')) return;
  const inCat = RS.filter((r) => !rsCat || r.category == rsCat);
  const groups = [...new Set(inCat.map((r) => r.productGroup))].sort();
  const words = rsQ.toLowerCase().split(/\s+/).filter(Boolean);
  const rows = inCat.filter((r) => (!rsGroup || r.productGroup == rsGroup) && words.every((w) => [r.name, r.itemKey, r.type, r.productGroup].join(' ').toLowerCase().includes(w)));
  const done = RS.filter((r) => r.unlocked).length;
  const rsFiltered = !!(rsQ.trim() || rsGroup);
  setHTML($('#rshead'), html`<div class="sub"><button data-rscat="" class="${rsCat == '' ? 'on' : ''}">All (${RS.length})</button>${RCATS.map((c) => html`<button data-rscat="${c}" class="${rsCat == c ? 'on' : ''}">${c} (${RS.filter((r) => r.category == c).length})</button>`)}</div>
    <div class="row">${rsCat ? html`<select id="rsGrp"><option value="">All product groups</option>${groups.map((g) => html`<option ${g == rsGroup ? 'selected' : ''}>${g}</option>`)}</select>` : ''}
    <input id="rsq" placeholder="Filter by name, key, type or group" size="34" value="${rsQ}"><button class="b sec" data-act="rsfilter">Filter</button><button class="b sec" data-act="rsclear">Clear</button>
    <span class="mut">${countText(rows.length, inCat.length, 'entry', 'entries', rsFiltered)}${rows.length > LISTMAX ? `, the first ${LISTMAX} shown` : ''}; ${done} of ${RS.length} entries unlocked in all. Unlocking adds the linked recipe or building set; group markers cannot be unlocked as one entry.</span></div>
    <div class="row"><button class="b bad" disabled aria-describedby="rsresetnote" title="The save does not say which research a new character already has (its starting recipes and building sets), so there is no state to reset to, and the editor has no safe way to take a recipe or building set back out. Restore a backup from before the research was unlocked to undo it.">Reset research</button><span class="mut" id="rsresetnote">Not available: The save does not say which research a new character already has (its starting recipes and building sets), so there is no state to reset to, and the editor has no safe way to take a recipe or building set back out. Restore a backup from before the research was unlocked to undo it.</span></div>`);
  const state = (r) => (r.unlocked ? html`<span class="ok">Unlocked</span>` : r.purchased ? html`<span class="warn">Purchased, ${r.unlockKind} missing</span>` : html`<span class="mut">Not purchased</span>`);
  const act = (r) => (!r.actionable ? html`<button class="b sec sm" disabled title="A group marker; unlock its individual entries">Group</button>` : r.unlocked ? '' :
    html`<button class="b sm" data-act="rsUnlock" data-key="${r.itemKey}">${r.needsRepair ? 'Repair unlock' : 'Unlock'}</button>`);
  setHTML($('#rsout'), tbl([{ k: 'name', label: 'Research', f: (r) => html`<span title="${r.itemKey}">${r.name}</span>` }, { k: 'itemKey', label: 'Key', cls: 'mut' }, { k: 'type', label: 'Type' },
    { k: 'productGroup', label: 'Product group' }, { k: 'state', label: 'State', f: state }], rows.slice(0, LISTMAX), { actions: act, empty: 'No research entry matches.' }));
}
function craftingList() {
  if (!CR || !$('#crout')) return;
  const inCat = CR.filter((r) => !crCat || r.category == crCat);
  const words = crQ.toLowerCase().split(/\s+/).filter(Boolean);
  const rows = inCat.filter((r) => words.every((w) => [r.name, r.recipeId, r.category, r.source].join(' ').toLowerCase().includes(w)));
  const known = CR.filter((r) => r.known).length;
  setHTML($('#crhead'), html`<div class="sub"><button data-crcat="" class="${crCat == '' ? 'on' : ''}">All (${CR.length})</button>${RCATS.filter((c) => c != 'Augmentations' && c != 'Uniques').map((c) => html`<button data-crcat="${c}" class="${crCat == c ? 'on' : ''}">${c} (${CR.filter((r) => r.category == c).length})</button>`)}</div>
    <div class="row"><input id="crq" placeholder="Filter by name, recipe id or source" size="34" value="${crQ}"><button class="b sec" data-act="crfilter">Filter</button><button class="b sec" data-act="crclear">Clear</button>
    <span class="mut">${countText(rows.length, inCat.length, 'recipe', 'recipes', !!crQ.trim())}${rows.length > LISTMAX ? `, the first ${LISTMAX} shown` : ''}; ${known} of ${CR.length} known. The recipes listed are the ones you know plus the ones your research tree can still give.</span></div>`);
  setHTML($('#crout'), tbl([{ k: 'name', label: 'Recipe', f: (r) => html`<span title="${r.recipeId}">${r.name}</span>${r.limited ? html` <span class="tag">limited use (${r.uses})</span>` : ''}` }, { k: 'recipeId', label: 'Recipe id', cls: 'mut' },
    { k: 'category', label: 'Category' }, { k: 'source', label: 'Source' }, { k: 'known', label: 'State', f: (r) => (r.known ? html`<span class="ok">Known</span>` : html`<span class="mut">Not known</span>`) }], rows.slice(0, LISTMAX),
    { actions: (r) => (r.known ? '' : html`<button class="b sm" data-act="crUnlock" data-id="${r.recipeId}">Unlock</button>`), empty: 'No recipe matches.' }));
}

// Building Sets and Customizations, as in Dune Docker's tabs: the catalog's items with what the save says you have, and a Give that puts the
// item in the backpack (the game teaches the set or unlocks the cosmetic when it is used). The save keeps no list of owned customizations
// that tabr-tau can read, so those show only whether the item is in an inventory.
let CT = null, ctFilter = 'all', ctQ = '', ctGroup = '', ctExp = true;
function catalogList() {
  if (!CT || !$('#ctout')) return;
  const bs = CT.kind == 'bs';
  const have = (r) => r.learned || r.unlocked || r.inInventory;
  const visible = CT.rows.filter((r) => bs ? (ctExp || !r.experimental) : true);
  const groupNames = bs ? [...new Set(visible.map((r) => r.group))].sort() : [];
  const rows = visible.filter((r) => (ctFilter == 'all' || (ctFilter == 'have' ? have(r) : !have(r))) && (!ctGroup || (bs ? r.group == ctGroup : r.groupId == ctGroup))
    && ctQ.split(/\s+/).filter(Boolean).every((w) => (r.name + ' ' + r.id + ' ' + r.group).toLowerCase().includes(w.toLowerCase())));
  const nHave = visible.filter(have).length;
  const cards = bs ? '' : html`<div class="row">${CT.groups.map((g) => html`<div class="card setcard ${ctGroup == g.id ? 'on' : ''}"><button class="b sec sm" data-ctg="${g.id}">${g.name}</button>
      <div class="mut">${g.count} cosmetics${g.requirement ? ' · ' + g.requirement : ''}</div>${CT.libraryAvailable ? html`<button class="b sm" data-act="ctUnlockSet" data-id="${g.id}" data-name="${g.name}">Unlock set</button> ` : ''}<button class="b sec sm" data-act="ctGrantSet" data-id="${g.id}" data-name="${g.name}">Give as items</button></div>`)}</div>`;
  setHTML($('#cthead'), html`${cards}<div class="sub">${[['all', 'All', visible.length], ['have', bs ? 'Learned or in inventory' : 'Unlocked or in inventory', nHave], ['missing', 'Not owned', visible.length - nHave]].map(([k, v, n]) => html`<button data-ctf="${k}" class="${ctFilter == k ? 'on' : ''}">${v} (${n})</button>`)}</div>
    <div class="row"><input id="ctq" placeholder="Filter by name, item id or group" size="34" value="${ctQ}"><button class="b sec" data-act="ctfilter">Filter</button><button class="b sec" data-act="ctclear">Clear</button>
    ${bs ? html`<select id="ctgrp"><option value="">All groups</option>${groupNames.map((g) => html`<option ${g == ctGroup ? 'selected' : ''}>${g}</option>`)}</select><label><input type="checkbox" id="ctexp" ${ctExp ? 'checked' : ''}> Show experimental</label>` : (ctGroup ? html`<button class="b sec" data-ctg="">All sets</button>` : '')}
    <span class="mut">${countText(rows.length, visible.length, 'item', 'items', !!(ctQ.trim() || ctGroup || ctFilter != 'all'))}${rows.length > LISTMAX ? `, the first ${LISTMAX} shown` : ''}. Give adds the item to your backpack.${bs ? '' : ' Unlock adds the cosmetic to your customization library without an item. Neither gives a DLC or an account entitlement: the game removed a store cosmetic that was unlocked this way (a plain dye pack stayed), so set unlocks skip those.'}</span></div>`);
  setHTML($('#ctout'), tbl([{ k: 'name', label: bs ? 'Building set' : 'Customization', f: (r) => html`<span title="${r.id}">${r.name}</span>${r.experimental ? html` <span class="tag">experimental</span>` : ''}` }, { k: 'id', label: 'Item id', cls: 'mut' },
    { k: 'group', label: bs ? 'Group' : 'Set' }, { k: 'requiredDlc', label: 'Requires', f: (r) => r.requiredDlc || (r.entitlement ? 'Account entitlement' : '') },
    { k: 'learned', label: 'State', f: (r) => (r.learned ? html`<span class="ok">Learned</span>` : r.unlocked ? html`<span class="ok">Unlocked</span>` : r.inInventory ? html`<span class="warn">In inventory</span>` : html`<span class="mut">Not owned</span>`) }], rows.slice(0, LISTMAX),
    { empty: bs ? 'No building set matches the filter.' : 'No customization matches the filter.', actions: (r) => html`${!bs && CT.libraryAvailable && r.unlockable && !r.unlocked ? html`<button class="b sm" data-act="ctUnlock" data-id="${r.id}" data-name="${r.name}" data-ent="${r.entitlement ? '1' : ''}" title="${r.entitlement ? 'Needs an account entitlement: the game removes it if your account does not own it' : ''}">Unlock</button> ` : ''}${r.inCatalog ? html`<button class="b ${!bs && CT.libraryAvailable && r.unlockable ? 'sec ' : ''}sm" data-act="ctGive" data-id="${r.id}">Give</button>` : ''}` }));
}

// ---------- BASES
// ---------- BASES (the console's Bases panel inside Player > Bases: the base list, and for one base Power, Water, Inventory and Land Claim Editor)
// The automatic refill switch, shown on both the Power and the Water tab because it refills both: water storage (cisterns, windtraps, Blood Purifiers) and power (generators, wind turbines, windtraps).
function autoRefillCard(prefs) {
  return html`<div class="card"><h3>Automatic refill of water and power</h3><div class="row"><span>When the editor opens: <b>${prefs.autoRefillOnOpen ? 'on' : 'off'}</b></span><button class="b sec" data-act="autoref" data-on="${prefs.autoRefillOnOpen ? '1' : ''}">${prefs.autoRefillOnOpen ? 'Turn off' : 'Turn on'}</button></div>${prefs.last ? html`<p class="mut">Last automatic save: ${prefs.last}</p>` : ''}
    ${prefs.problem ? html`<p class="bad">${prefs.problem}</p>` : ''}
    <p class="mut">When on, opening the editor refills water storage (Water Cisterns, Windtraps, Blood Purifiers) and power (generators, wind turbines, windtraps) and <b>saves straight away</b>, like every other change, so it applies the next time the game loads. It is skipped while a single-player session is running. The previous file is backed up first. Remembered between runs. The same switch is on the Power and Water tabs.</p></div>`;
}
// The console's Land Claim Editor: the claim as a grid of cells turned by the base's own yaw (north at the top), with the totem's cell,
// the cells it has, and dotted "available" cells next to it. Click an available cell to add it, a chosen one to take it back (what is
// chosen must stay connected edge to edge), pick a vertical level, then Apply. Clicking an existing cell that holds no building piece or placeable marks it
// for removal (shown as red), and Remove takes the marked cells out; the claim must stay connected, and the server refuses a cell with pieces in it as well.
let LC = null; // { claim, sel: Set of "x,y" to add, rm: Set of "x,y" to remove, level }
let TD = []; // the places the last Find returned
const lcKey = (x, y) => `${x},${y}`;
const lcNeighbours = (x, y) => [[x + 1, y], [x - 1, y], [x, y + 1], [x, y - 1]];
const lcCoords = (k) => k.split(',').map(Number);
function lcReachable(cells) {
  const seen = new Set(['0,0']), queue = [[0, 0]];
  while (queue.length) {
    const [x, y] = queue.shift();
    for (const [nx, ny] of lcNeighbours(x, y)) { const k = lcKey(nx, ny); if (cells.has(k) && !seen.has(k)) { seen.add(k); queue.push([nx, ny]); } }
  }
  return seen;
}
function lcToggle(k) {
  const c = LC.claim, existing = new Set(['0,0', ...(c.segments || []).map((s) => lcKey(s.x, s.y))]);
  if (!LC.sel.has(k)) { LC.sel.add(k); return; }
  LC.sel.delete(k);
  const reached = lcReachable(new Set([...existing, ...LC.sel]));
  LC.sel = new Set([...LC.sel].filter((e) => reached.has(e))); // taking a cell back also drops the chosen cells that hung from it
}
// The compass point a direction in the map's own terms points to (east to the right, south down, as the world's x and y run): the grid's cell x runs along the base's yaw,
// the cell y a quarter turn after it.
const COMPASS8 = ['North', 'North-east', 'East', 'South-east', 'South', 'South-west', 'West', 'North-west'];
function lcDirection(deg) {
  const bearing = (((deg + 90) % 360) + 360) % 360; // from north, turning clockwise
  return COMPASS8[Math.round(bearing / 45) % 8];
}
// A click on a cell of the grid: an existing cell is marked for removal or unmarked, any other is added or taken back.
function lcCellClick(k) {
  const existing = new Set((LC.claim.segments || []).map((s) => lcKey(s.x, s.y)));
  if (existing.has(k)) { if (LC.rm.has(k)) LC.rm.delete(k); else LC.rm.add(k); return; }
  lcToggle(k);
}
function claimEditorDraw() {
  const host = $('#lced');
  if (!host || !LC) return;
  const c = LC.claim, yaw = Number(c.yaw) || 0;
  const existing = new Set(['0,0', ...(c.segments || []).map((s) => lcKey(s.x, s.y))]);
  const pieces = new Map((c.grid || []).map((g) => [lcKey(g.x, g.y), g.n]));
  // the cells that stay when the marked ones go, and whether the claim stays connected to the totem
  const kept = new Set([...existing].filter((k) => !LC.rm.has(k)));
  const reachedBefore = lcReachable(existing), reachedAfter = lcReachable(kept);
  const cutOff = LC.rm.size ? [...kept].filter((k) => reachedBefore.has(k) && !reachedAfter.has(k)) : [];
  const occupied = new Set([...existing, ...LC.sel]);
  const frontier = new Set();
  for (const k of occupied) {
    const [x, y] = lcCoords(k);
    for (const [nx, ny] of lcNeighbours(x, y)) { const nk = lcKey(nx, ny); if (!occupied.has(nk) && Math.abs(nx) <= 128 && Math.abs(ny) <= 128) frontier.add(nk); }
  }
  const plotted = [...new Set([...occupied, ...frontier])].map(lcCoords);
  const rad = (yaw * Math.PI) / 180;
  const world = plotted.map(([x, y]) => [x * Math.cos(rad) - y * Math.sin(rad), x * Math.sin(rad) + y * Math.cos(rad)]);
  const xs = world.map((w) => w[0]), ys = world.map((w) => w[1]);
  const minX = Math.min(...xs) - 0.7, minY = Math.min(...ys) - 0.7;
  const width = Math.max(2, Math.max(...xs) + 0.7 - minX), height = Math.max(2, Math.max(...ys) + 0.7 - minY);
  // room around the grid for the compass points, and the viewBox with it
  const pad = 1.3, vx = minX - pad, vy = minY - pad, vw = width + 2 * pad, vh = height + 2 * pad;
  const compass = [['N', vx + vw / 2, vy + 0.8], ['S', vx + vw / 2, vy + vh - 0.35], ['W', vx + 0.55, vy + vh / 2 + 0.3], ['E', vx + vw - 0.55, vy + vh / 2 + 0.3]]
    .map(([t, x, y]) => html`<text x="${x}" y="${y}" class="lc-compass ${t === 'N' ? 'lc-compass-n' : ''}">${t}</text>`);
  const dirX = lcDirection(yaw), dirY = lcDirection(yaw + 90);
  const maxLevel = Math.max(c.level, c.maxLevel);
  const dirty = LC.sel.size > 0 || LC.level !== c.level;
  const cells = plotted.map(([x, y]) => {
    const k = lcKey(x, y), origin = k === '0,0', sel = LC.sel.has(k), have = existing.has(k) && !origin, avail = frontier.has(k);
    const removable = have && !pieces.get(k), marked = removable && LC.rm.has(k);
    const kind = origin ? 'lc-o' : sel ? 'lc-s' : marked ? 'lc-r' : have ? 'lc-x' : 'lc-a';
    const label = origin ? 'T' : sel ? '+' : have && pieces.get(k) ? pieces.get(k) : '';
    return html`<rect x="${x - 0.44}" y="${y - 0.44}" width="0.88" height="0.88" rx="0.08" class="lc-cell ${kind}" ${avail || sel || removable ? html`data-cell="${k}"` : ''}><title>cell ${x}, ${y}${origin ? ' (the totem)' : have && pieces.get(k) ? ` (holds ${pieces.get(k)} building pieces or placeables: cannot be removed)` : removable ? (marked ? ' (marked for removal: click to keep it)' : ' (click to mark it for removal)') : ''}</title></rect>${label !== '' ? html`<text x="${x}" y="${y + 0.09}" transform="rotate(${-yaw} ${x} ${y})" class="lc-label">${label}</text>` : ''}`;
  });
  setHTML(host, html`<div class="card"><h3>Land claim editor</h3>
    <div class="grid">${kv('Totem ID', c.totem_id)}${kv('Original yaw', Math.round(yaw * 10) / 10 + '°')}${kv('Horizontal segments', c.cells - 1)}${kv('Vertical level', c.level + ' / ' + c.maxLevel)}</div>
    <div class="lc-work"><div class="lc-gridcard"><b>Horizontal claim</b><p class="mut">Click a dotted cell to add it. New cells must stay connected edge to edge. A number is how many building pieces a cell holds.</p>
      <div class="lc-north">&#9650; North is up. Column <b>+x</b> runs <b>${dirX}</b>, row <b>+y</b> runs <b>${dirY}</b> (the base's yaw is ${Math.round(yaw * 10) / 10}°).</div>
      <svg class="lc-svg" viewBox="${vx} ${vy} ${vw} ${vh}">${compass}<g transform="rotate(${yaw} 0 0)">${cells}
        <path d="M0.5 0 H1.9" class="lc-axis lc-axis-x"></path><path d="M1.9 0 l-0.25 -0.14 v0.28 z" class="lc-axis-head lc-axis-x"></path><text x="2.35" y="0.12" transform="rotate(${-yaw} 2.35 0)" class="lc-axis-t lc-axis-x">+x</text>
        <path d="M0 0.5 V1.9" class="lc-axis lc-axis-y"></path><path d="M0 1.9 l-0.14 -0.25 h0.28 z" class="lc-axis-head lc-axis-y"></path><text x="0" y="2.5" transform="rotate(${-yaw} 0 2.5)" class="lc-axis-t lc-axis-y">+y</text></g></svg>
      <p class="mut"><span class="lc-key lc-o"></span>Totem <span class="lc-key lc-x"></span>Existing <span class="lc-key lc-s"></span>New <span class="lc-key lc-a"></span>Available <span class="lc-key lc-r"></span>To remove</p>
      <p class="mut">World-oriented view: north stays at the top and the grid is turned by the base's original yaw; the saved cell coordinates are not changed.</p></div>
    <div class="lc-side"><b>Expansion</b>
      <label>Vertical level <select id="lcLvl">${Array.from({ length: maxLevel - c.level + 1 }, (_, i) => c.level + i).map((n) => html`<option ${n === LC.level ? 'selected' : ''}>${n}</option>`)}</select></label>
      <p class="mut">Vertical expansion grows equally up and down. The game caps it at level ${c.maxLevel}.</p>
      <div class="kv"><b>Pending horizontal</b><span>${LC.sel.size}</span></div><div class="kv"><b>Pending vertical</b><span>${LC.level === c.level ? 'No change' : c.level + ' → ' + LC.level}</span></div>
      <div class="row"><button class="b" data-act="lcApply" ${dirty ? '' : 'disabled'}>Apply changes</button>${LC.sel.size || LC.rm.size ? html`<button class="b sec" data-act="lcClear">Clear selection</button>` : ''}</div>
      <div class="kv"><b>Marked for removal</b><span>${LC.rm.size}</span></div>
      ${cutOff.length ? html`<p class="bad">Removing those would cut ${cutOff.length} cell${cutOff.length === 1 ? '' : 's'} that stay off from the totem; the game needs the claim connected edge to edge. Mark them too, or unmark a cell.</p>` : ''}
      <div class="row"><button class="b" data-act="lcRemove" ${LC.rm.size && !cutOff.length ? '' : 'disabled'}>Remove marked cells</button></div>
      <p class="mut">Click an existing cell to mark it. A cell with building pieces or placeables in it (its number is shown) cannot be removed, and neither can the totem's own cell.</p>
      <p class="mut">The edit is written at once, after the usual backup. Applied when the game next loads the save.</p></div></div>
    <p class="mut">This editor bypasses the game's normal staking checks. Claiming a protected area does not guarantee that the game lets you build there.</p></div>`);
}

let bsTab = 'power', bsOpen = null, bsGroup = 'storage';
async function basesView() {
  const list = await api('/api/bases/list');
  const id = list.some((x) => x.base_id == bsOpen) ? bsOpen : (list[0] ? list[0].base_id : null);
  const base = list.find((x) => x.base_id == id);
  const h = [html`<div class="card"><h3>Bases (${list.length})</h3>${tbl([{ k: 'base_id', label: 'ID' }, { k: 'name', label: 'Base name' }, { k: 'base_type', label: 'Base type' }, { k: 'owner', label: 'Owner' }, { k: 'map', label: 'Map' },
    { k: 'generators', label: 'Generators' }, { k: 'pieces', label: 'Building pieces' }, { k: 'placeables', label: 'Placeables' }, { k: 'x', label: 'Coordinates', f: (r) => `${fix(r.x)}, ${fix(r.y)}, ${fix(r.z)}` }], list,
    { actions: (r) => (r.base_id == id ? html`<span class="tag">Open below</span>` : html`<button class="b sec sm" data-bsopen="${r.base_id}">Open</button>`), empty: 'No bases yet: build one in the game, then reload.' })}</div>`];
  if (base) {
    const tabs = [['power', 'Power'], ['water', 'Water'], ['inventory', 'Inventory'], ['claim', 'Land claim editor'], ['structures', 'Structures']];
    h.push(html`<div class="card"><h3>${base.name} (#${base.base_id})</h3><div class="sub">${tabs.map(([k, v]) => html`<button data-bstab="${k}" class="${bsTab == k ? 'on' : ''}">${v}</button>`)}</div></div>`);
    if (bsTab == 'power') {
      const pw = await api('/api/bases/power?base=' + id);
      const prefs = await api('/api/settings');
      h.push(html`<div class="card"><h3>Power</h3>${pw.types.length ? tbl([{ k: 'name', label: 'Building' }, { k: 'devices', label: 'Built' },
        { k: 'queued', label: 'Holds', f: (r) => Number(r.queued).toLocaleString() + (r.fuel ? ' ' + itemName(r.fuel) : '') }, { k: 'capacity', label: 'Full', f: (r) => (r.capacity ? Number(r.capacity).toLocaleString() : '-') }, { k: 'empty', label: 'Empty' }], pw.types) : html`<p class="mut">No generators, wind turbines or windtraps built at this base.</p>`}
        <div class="row"><button class="b" data-act="refillGen" data-confirm="Yes, refill power on every base">Refill power (every base)</button><span class="mut">Tops up Fuel-Powered and Spice-Powered Generators (499), Omnidirectional Wind Turbines (Low-grade Lubricant) and Directional Wind Turbines (Industrial-grade Lubricant), five stacks of 100 and 499 in all, and Windtraps and Large Windtraps (5 filters, always the best filter for the machine). Buildings that are only placeholders are left alone. </span></div></div>
    ${autoRefillCard(prefs)}`);
    } else if (bsTab == 'water') {
      const w = await api('/api/bases/water?base=' + id);
      h.push(html`<div class="card"><h3>Water</h3>${w.types.length ? tbl([{ k: 'name', label: 'Building' }, { k: 'devices', label: 'Built' }, { k: 'stored', label: 'Water stored', f: (r) => Number(r.stored).toLocaleString() },
        { k: 'capacity', label: 'Capacity', f: (r) => (r.capacity ? Number(r.capacity).toLocaleString() : '-') }, { k: 'fillPercent', label: 'Fill', f: (r) => (r.fillPercent == null ? '-' : Math.round(r.fillPercent) + '%') }], w.types) : html`<p class="mut">No water storage at this base.</p>`}
        <div class="row"><button class="b" data-act="refillWater" data-confirm="Yes, refill water on every base">Refill base water (every base)</button><span class="mut">Fills every Water Cistern, Medium Water Cistern, Large Water Cistern, Windtrap, Large Windtrap and Blood Purifier to its capacity. The blood in a Blood Purifier is not touched, and a building that is only a placeholder is left alone.</span></div></div>
    ${autoRefillCard(await api('/api/settings'))}`);
    } else if (bsTab == 'inventory') {
      const inv = await api('/api/bases/inventory?base=' + id);
      const grp = inv.groups.some((g) => g.id == bsGroup && g.count) ? bsGroup : (inv.groups.find((g) => g.count) || inv.groups[0]).id;
      const rows = inv.containers.filter((c) => c.group == grp);
      h.push(html`<div class="card"><h3>Inventory</h3><div class="sub">${inv.groups.map((g) => html`<button data-bsgrp="${g.id}" class="${grp == g.id ? 'on' : ''}">${g.label} (${g.count})</button>`)}</div>
        ${tbl([{ k: 'name', label: 'Container' }, { k: 'type', label: 'Type' }, { k: 'items', label: 'Items' }, { k: 'max_item_count', label: 'Slots used', f: (r) => `${r.items} of ${r.max_item_count}` }], rows,
        { actions: (r) => html`<button class="b sec sm" data-act="openInv" data-id="${r.inventory_id}" data-name="${r.name}">Open</button>`, empty: 'No containers in this group.' })}
        <p class="mut">Slots used is the items held out of the container's capacity. Only containers that belong to this base are listed.</p></div><div id="inv"></div>`);
    } else if (bsTab == 'structures') {
      // tabr-tau's own base tools (the console's base view has no equivalent): structure health, repair all, sand, placeables, piece types.
      const b = await api('/api/bases');
      const p = b.pieces || {};
      h.push(html`<div class="card"><h3>Structure health</h3><div class="grid">${kv('Building pieces', p.n)}${kv('Lowest health', fix(p.minh))}${kv('Average health', fix(p.avgh))}${kv('Total sand buildup', fix(p.sand))}</div>
        <div class="row"><button class="b" data-act="repairB" data-confirm="Yes, repair every base">Repair every base to max</button><button class="b sec" data-act="sand" data-confirm="Yes, clear sand on every base">Clear sand on every base</button><span class="mut">These two act on all of your bases, not only the one open above. Repair sets each piece to the highest health seen for its type.</span></div></div>`);
      h.push(html`<div class="card"><h3>Placeables (${b.placeables.length})</h3>${tbl([{ k: 'id' }, { k: 'building_type', label: 'Type' }, { k: 'health', label: 'Health', f: (r) => (r.health == null ? '' : fix(r.health)) }], b.placeables, { empty: 'No placeables in this save.' })}</div>
        <div class="card"><h3>Building piece types</h3>${tbl([{ k: 'building_type', label: 'Type' }, { k: 'n', label: 'Count' }, { k: 'minh', label: 'Min health' }, { k: 'maxh', label: 'Max health' }, { k: 'avgh', label: 'Avg' }], b.types, { empty: 'No building pieces in this save.' })}</div>`);
    } else {
      const claims = (await api('/api/bases/claim')).filter((c) => c.totem_id == id);
      LC = claims.length ? { claim: claims[0], sel: new Set(), rm: new Set(), level: claims[0].level } : null;
      if (LC) h.push(html`<div id="lced"></div>`);
      h.push(html`${claims.length ? html`<div class="card"><h3>Resize by square</h3>${claims.map((c) => html`<div class="row"><b>${c.name} ${c.totem_id}</b><span>${c.cells} cell${c.cells == 1 ? '' : 's'}${c.irregular ? ' (irregular shape)' : ` (${2 * c.rings + 1} x ${2 * c.rings + 1} square)`}, vertical level ${c.level}</span>
      Grow to <select id="cr${c.totem_id}"><option value="0">no change</option>${Array.from({ length: c.maxRings }, (_, i) => i + 1).filter((n) => n > c.rings).map((n) => html`<option value="${n}">${2 * n + 1} x ${2 * n + 1}</option>`)}</select>
      Level <select id="cv${c.totem_id}">${Array.from({ length: c.maxLevel + 1 }, (_, i) => i).filter((n) => n >= c.level).map((n) => html`<option value="${n}" ${n == c.level ? 'selected' : ''}>${n}</option>`)}</select>
      <button class="b" data-act="claimGrow" data-id="${c.totem_id}" data-level="${c.level}">Expand claim</button></div>
      ${c.cells > 1 ? html`<div class="row"><span class="mut">${c.piecesInClaim == null ? '' : `${c.piecesInClaim} pieces inside the claim, ${c.piecesOutside} outside.`}</span>
        Shrink to <select id="cs${c.totem_id}"><option value="0">the totem's own cell only</option>${Array.from({ length: c.maxRings }, (_, i) => i + 1).filter((n) => (c.irregular ? n <= c.rings : n < c.rings)).map((n) => html`<option value="${n}">${2 * n + 1} x ${2 * n + 1} square</option>`)}</select>
        <button class="b sec" data-act="claimShrink" data-id="${c.totem_id}">Shrink claim</button><span class="mut">Cells that hold building pieces are never removed.</span></div>` : html``}`)}
    <p class="mut">One cell is 10 x 10 foundations, around the totem. Expanding only adds cells and never removes them, and the vertical level can only go up. Applied when the game next loads the save.</p></div>` : html``}
`);
      if (!claims.length) h.push(html`<p class="mut">This base has no land claim data in the save.</p>`);
    }
  }
  setHTML(host(), html`${h}`);
  if (bsTab == 'claim') claimEditorDraw();
}

// ---------- VEHICLES
// The player's own vehicles as in the console's Vehicles tab: type, lowest condition, fuel and location per vehicle, and for the open
// one its components with their condition and its cargo hold (read-only here; removing cargo comes later).
let vhOpen = null;
const pctCell = (v) => (v == null ? html`<span class="mut">-</span>` : html`<span class="${v < 30 ? 'bad' : v < 60 ? 'warn' : 'ok'}">${Math.round(v)}%</span>`);
const compName = (t) => String(t || '').replace(/_\d+$/, '').replace(/([a-z])([A-Z])/g, '$1 $2');
async function vehiclesView() {
  const v = await api('/api/vehicles');
  const open = v.vehicles.find((r) => r.id == vhOpen) || v.vehicles[0];
  if (open && open.cargo && open.cargo.items.length) await loadCatalog(open.cargo.items.map((i) => i.template_id));
  const h = [html`<div class="card"><h3>Your vehicles (${v.vehicles.length})</h3>${tbl([{ k: 'name', label: 'Vehicle', f: (r) => html`<b title="${r.name}">${r.type || r.name}</b> <span class="mut">${r.map}</span>` }, { k: 'type', label: 'Type' },
    { k: 'condition', label: 'Lowest condition', f: (r) => pctCell(r.condition) }, { k: 'fuel', label: 'Fuel', f: (r) => (r.fuel == null ? '-' : Number(r.fuel).toFixed(1)) },
    { k: 'x', label: 'Location', f: (r) => `${fix(r.x)}, ${fix(r.y)}` }], v.vehicles,
    { actions: (r) => html`<button class="b ${open && open.id == r.id ? '' : 'sec'} sm" data-vhopen="${r.id}">Details</button><button class="b sm" data-act="repairOne" data-id="${r.id}">Repair</button><button class="b sm" data-act="bring" data-id="${r.id}">Bring to me</button>`, empty: 'You have no vehicles in this save yet.' })}
    <div class="row"><button class="b" data-act="repairV">Repair all my vehicles</button><label>Only modules below <input id="rvPct" type="number" min="1" max="100" value="100" class="w70"> % of their full durability</label></div>
    <p class="mut">Makes every module as good as new: full durability, and the maximum that repairs at a workbench wore down is restored too. 100 repairs every module that is not already as good as new; a lower number repairs only the modules below that share, and says when there were none. A module whose full durability is not known is raised to the maximum it records, or skipped when it records none.</p></div>`];
  if (open) {
    const cargo = open.cargo;
    h.push(html`<div class="card"><h3>${open.type || open.name}: ${(open.components || []).length} components</h3>${tbl([{ k: 'template_id', label: 'Component', f: (r) => html`<span title="${r.template_id}">${compName(r.template_id)}</span>` },
      { k: 'percent', label: 'Condition', f: (r) => pctCell(r.percent) }, { k: 'current', label: 'Durability', f: (r) => (r.current == null ? '-' : Number(r.current).toFixed(0) + (r.max ? ' / ' + Number(r.max).toFixed(0) : '')) }], open.components || [], { empty: 'This vehicle has no components in the save.' })}
      <p class="mut">Condition is each module's durability against its current maximum; a module with no stored maximum shows only its value. The fuel tank size is not stored in the save, so fuel is the current amount.</p></div>`);
    h.push(html`<div class="card"><h3>Cargo hold</h3>${cargo ? html`<p class="mut">${cargo.slots == null ? nn(cargo.items.length, 'item', 'items') : `${cargo.items.length} of ${cargo.slots} slots used`}</p>${cargo.items.length ? tbl([{ k: 'position_index', label: 'Slot' },
      { k: 'template_id', label: 'Item', f: (r) => itemCell(r.template_id) }, { k: 'stack_size', label: 'Quantity' }, { k: 'quality_level', label: 'Grade' }], cargo.items) : html`<p class="mut">The cargo hold is empty.</p>`}` : html`<p class="mut">This vehicle has no cargo hold.</p>`}</div>`);
  }
  h.push(html`<div class="card"><h3>Recovered and stored vehicles (${v.recovered.length})</h3>${tbl([{ k: 'vehicle_id', label: 'ID' }, { k: 'vehicle_name', label: 'Name' }, { k: 'chassis_durability', label: 'Chassis durability' }, { k: 'time_stored', label: 'Stored at', f: (r) => fmtEpoch(r.time_stored) }, { k: 'reason', label: 'Reason code' }], v.recovered, { empty: 'No vehicle has been recovered or stored.' })}</div>`);
  if (v.hidden) h.push(html`<p class="mut">${v.hidden} other vehicle${v.hidden == 1 ? '' : 's'} in the world (not yours) ${v.hidden == 1 ? 'is' : 'are'} not shown.</p>`);
  if (!v.vehicles.length && !v.recovered.length) h.push(html`<p class="mut">None of your vehicles are in this save yet. Vehicle fuel is stored in an opaque binary blob and is not editable.</p>`);
  setHTML(host(), html`${h}`);
}

// ---------- LANDSRAAD
async function landsraadView() {
  const l = await api('/api/landsraad');
  await loadCatalog(l.tasks.flatMap((x) => (x.rewards || []).map((y) => y.template_id)));
  const t = l.term || {};
  const fname = (id) => (l.factions.find((f) => f.id == id) || {}).name || id || '—';
  const facOpts = (sel) => l.factions.map((f) => html`<option value="${f.id}" ${f.id == sel ? 'selected' : ''}>${f.name}</option>`);
  const dec = l.decrees;
  setHTML($('#main'), html`<div class="card"><h3>Current term ${t.term_id}</h3><div class="grid">${kv('Start', fmtEpoch(t.start_time))}${kv('End', fmtEpoch(t.end_time))}${kv('Reigning faction', fname(t.reigning_faction_id))}${kv('Active decree', (dec.find((d) => d.id == t.active_decree_id) || {}).decree_name || '—')}${kv('Elected decree', (dec.find((d) => d.id == t.elected_decree_id) || {}).decree_name || '—')}</div>
  <div class="row">Active decree<select id="ad"><option value="">none</option>${dec.map((d) => html`<option value="${d.id}" ${d.id == t.active_decree_id ? 'selected' : ''}>${d.decree_name}</option>`)}</select>
  Reigning<select id="rf"><option value="">none</option>${facOpts(t.reigning_faction_id)}</select><button class="b" data-act="term">Apply</button></div></div>
  <div class="card"><h3>Decree pool</h3>${tbl([{ k: 'id', label: 'ID' }, { k: 'decree_name', label: 'Decree' }, { k: 'disabled', label: 'State', f: (r) => (r.disabled ? html`<span class="mut">disabled</span>` : html`<span class="ok">enabled</span>`) }], dec, { empty: 'This save has no decrees.', actions: (r) => html`<button class="b sec sm" data-act="decree" data-id="${r.id}" data-d="${r.disabled ? 0 : 1}">${r.disabled ? 'Enable' : 'Disable'}</button>` })}</div>
  <div class="card"><h3>Task board (${l.tasks.length})</h3><div class="row">Faction<select id="lf">${facOpts()}</select><span class="mut">used by the buttons below</span></div>${tbl([{ k: 'board_index', label: '#' }, { k: 'house_name', label: 'House', f: (r) => String(r.house_name || '').replace('DA_House', '') }, { k: 'goal_amount', label: 'Goal' },
    { k: 'contributions', label: 'Progress', f: (r) => ((r.contributions || []).length ? (r.contributions || []).map((c) => html`<span class="tag">${fname(c.faction_id)}: ${fix(c.amount)}</span>`) : html`<span class="mut">0</span>`) },
    { k: 'completed', label: 'Status', f: (r) => (r.completed ? html`<span class="ok">done (${fname(r.winning_faction_id)})</span>` : 'open') },
    { k: 'rewards', label: 'Rewards', f: (r) => html`<details><summary>${nn((r.rewards || []).length, 'reward', 'rewards')}</summary>${(r.rewards || []).map((x) => html`<div>At ${x.threshold}: ${Number(x.amount).toLocaleString()} × ${itemName(x.template_id)}</div>`)}</details>` }], l.tasks,
    { empty: 'This save has no Landsraad tasks.', actions: (r) => html`<button class="b sm" data-act="tfill" data-id="${r.id}" data-goal="${r.goal_amount}">Fill goal</button> <button class="b sm" data-act="tdone" data-id="${r.id}" data-r="${r.completed ? 1 : 0}">${r.completed ? 'Reopen' : 'Complete'}</button>` })}</div>`);
}

// ---------- CONFIG (.ini)
let cfgFile = null;
async function configView() {
  const val_ = await api('/api/config/validate');
  if (!val_.dirExists) {
    setHTML($('#main'), html`<div class="warn">${val_.error}<br>Start the game once so it creates its config, or pass <code>--config &lt;folder&gt;</code>.</div>`);
    return;
  }
  const problems = val_.files.filter((f) => !f.exists || (f.missingSections || []).length);
  const files = await api('/api/config/files');
  if (!cfgFile) cfgFile = (files.find((f) => f.name == 'ServerCustomSettings.ini') || files[0] || {}).name;
  const h = [problems.length
    ? html`<div class="warn"><b>Config check:</b><ul class="tight">${problems.map((f) => html`<li><code>${f.name}</code> — ${f.exists ? 'missing sections: ' + (f.missingSections || []).join(', ') : 'file not found'} <button class="b sm" data-act="cfgcreate" data-n="${f.name}">${f.exists ? 'Add sections' : 'Create'}</button></li>`)}</ul>Missing files are normally created by the game on first run or when a setting is changed in-game.</div>`
    : html`<div class="mut mb8">✓ All expected config files present: ${val_.files.map((f) => f.name).join(', ')}</div>`];
  h.push(html`<div class="warn">The game rewrites its config files when it exits. Close Dune before saving changes here. Each save keeps a backup in a <code>tabr-tau-backups</code> folder next to the file.</div>
  <div class="row"><select id="cfgsel">${files.map((f) => html`<option value="${f.name}" ${f.name == cfgFile ? 'selected' : ''}>${f.name}${f.empty ? ' (empty)' : ''}</option>`)}</select></div>`);
  if (cfgFile) {
    const c = await api('/api/config/file?name=' + encodeURIComponent(cfgFile));
    const bk = await api('/api/config/backups?name=' + encodeURIComponent(cfgFile));
    let last = null;
    const entryRows = c.entries.map((e) => {
      const sec = e.section !== last ? e.section : '';
      last = e.section;
      return html`<tr><td class="mut mono">${sec}</td><td class="mono">${e.key}</td><td><input class="cfgv" value="${e.value}" data-line="${e.line}" data-key="${e.key}" data-section="${e.section}"></td><td><a href="#" data-act="cfgdel" data-line="${e.line}" data-key="${e.key}" class="bad">delete</a></td></tr>`;
    });
    h.push(html`<div class="card"><h3>${cfgFile}</h3>${c.entries.length ? html`<div class="scroll"><table><thead><tr><th>Section</th><th>Key</th><th>Value</th><th></th></tr></thead><tbody>${entryRows}</tbody></table></div>` : html`<p class="mut">This file is empty.</p>`}
    <div class="row mt12"><b>Add key</b><input id="cs" placeholder="[Section]" list="secs"><datalist id="secs">${[...new Set(c.entries.map((e) => e.section))].map((s) => html`<option value="${s}">`)}</datalist><input id="ck" placeholder="Key"><input id="cv" placeholder="Value"><button class="b" data-act="cfgadd">Add</button></div></div>
    <div class="card"><details><summary>Edit raw text</summary><textarea id="raw" rows="18" spellcheck="false">${c.text}</textarea><div class="row"><button class="b" data-act="cfgraw">Save raw text</button></div></details></div>
    <div class="card"><details><summary>Backups (${bk.length})</summary>${tbl([{ k: 'name', label: 'Backup' }, { k: 'size', label: 'Size', f: (r) => fmtBytes(r.size) }], bk, { empty: 'No backups of this file yet. One is made before a change is written.', actions: (r) => html`<button class="b sec sm" data-act="cfgrestore" data-b="${r.name}">Restore</button>` })}</details></div>`);
  }
  setHTML($('#main'), html`${h}`);
}

// ---------- DATABASE
let dbTable = null, dbOff = 0, dbQ = '';
async function dbView() {
  const s = sub.db || 'browse';
  const h = [subNav('db', [['browse', 'Tables'], ['sql', 'SQL'], ['backups', 'Save backups']])];
  if (s == 'browse') {
    const t = await api('/api/db/tables');
    if (!dbTable) dbTable = t[0].name;
    h.push(html`<div class="row"><select id="dbsel">${t.map((x) => html`<option value="${x.name}" ${x.name == dbTable ? 'selected' : ''}>${x.name} (${x.rows})</option>`)}</select><input id="dbq" placeholder="Search this table" value="${dbQ}"><button class="b sec" data-act="dbgo">Search</button>
      <a href="/api/db/export?name=${encodeURIComponent(dbTable)}&amp;format=csv" data-export="csv">CSV</a><a href="/api/db/export?name=${encodeURIComponent(dbTable)}&amp;format=json" data-export="json">JSON</a></div><div id="dbout"></div>`);
    setHTML($('#main'), html`${h}`);
    const d = await api(`/api/db/table?name=${encodeURIComponent(dbTable)}&q=${encodeURIComponent(dbQ)}&offset=${dbOff}&limit=100`);
    const cols = d.columns.slice(1);
    const locked = new Map(Object.entries(d.locked || {})); // a column name from the save is only ever a Map key
    setHTML($('#dbout'), html`<p class="mut">${d.total === 0 ? (dbQ ? 'No rows match the search' : 'No rows in this table') : `${nn(d.total, 'row', 'rows')}, showing ${dbOff + 1}–${Math.min(dbOff + 100, d.total)}`}${d.tableLocked ? html`. Read-only: ${d.tableLocked}.` : locked.size ? html`. Plain text columns are read-only (primary keys, links to other tables and account identity); hover for the reason.` : ''}</p><div class="scroll"><table><thead><tr>${cols.map((c) => html`<th>${c}</th>`)}</tr></thead><tbody>${d.rows.map((r) => html`<tr>${r.slice(1).map((v, i) => {
      const b = typeof v === 'string' && v.startsWith('<blob');
      const why = d.tableLocked || locked.get(cols[i]);
      return html`<td>${b ? html`<span class="mut mono">${v}</span>` : why ? html`<span class="mono" title="${why}">${v}</span>` : html`<input class="dbc" value="${v}" data-rowid="${r[0]}" data-kind="${d.columns[0]}" data-col="${cols[i]}" data-orig="${v}" ${v === null ? 'data-null' : ''} size="${Math.max(6, Math.min(40, String(v ?? '').length + 2))}">`}</td>`;
    })}</tr>`)}</tbody></table></div>
      <div class="row"><button class="b sec" data-act="dbprev" ${dbOff <= 0 ? 'disabled' : ''}>Previous</button><button class="b sec" data-act="dbnext" ${dbOff + 100 >= d.total ? 'disabled' : ''}>Next</button></div>`);
    return;
  } else if (s == 'sql') {
    h.push(html`<div class="card"><h3>SQL</h3><textarea id="sql" rows="5" placeholder="select * from items limit 20">${window.lastSql || 'select name from sqlite_master'}</textarea><div class="row"><button class="b" data-act="runsql">Run (read-only)</button><button class="b bad" data-act="execsql">Run as write</button><span class="mut">Write mode runs one or more statements atomically. They are written to game.db at once, after a backup. If the file cannot be written (for example a single-player session is running) they are queued and written by themselves when it can be.</span></div><div id="sqlout"></div></div>`);
  } else {
    const b = await api('/api/save/backups');
    h.push(html`<div class="card"><h3>Save backups</h3><p class="mut">A backup of game.db is taken before changes are written (one backup covers the changes made within two minutes) and before every restore. Restore requires the game to be closed.</p>${tbl([{ k: 'name', label: 'Backup' }, { k: 'size', label: 'Size', f: (r) => fmtBytes(r.size) }, { k: 'modified', label: 'Saved', f: (r) => fmtDate(r.modified) }], b, { actions: (r) => html`<button class="b sec sm" data-act="restore" data-n="${r.name}">Restore</button>`, empty: 'No backups yet. One is made before the first change is written.' })}</div>`);
  }
  setHTML($('#main'), html`${h}`);
}

// ---------- render + events
// What a vehicle repair says. "Repaired 0" alone does not tell whether nothing was below the threshold or nothing could be repaired (UX-03).
function repairMsg(r, scope, pct) {
  const skipped = r.withoutKnownMax ? ` ${r.withoutKnownMax} modules have no known maximum and were left alone${(r.withoutKnownMaxTemplates || []).length ? ' (' + r.withoutKnownMaxTemplates.join(', ') + ')' : ''}.` : '';
  if (r.repaired) return `Repaired ${r.repaired} ${scope}.` + skipped;
  if (!r.modules) return 'Nothing to repair: no vehicle modules were found.' + skipped;
  return (pct && pct < 100 ? `Nothing to repair: none of the ${r.modules} modules is below ${pct}% of its full durability.` : `Nothing to repair: all ${r.modules} modules are already as good as new.`) + skipped;
}
// The browser tab names the page that is open: "Player - Inventory - Tabr Tau".
function pageTitle() {
  const parts = [TABS[tab] || 'Tabr Tau'];
  if (tab === 'player') {
    const p = PTABS.find(([k]) => k === sub.player);
    if (p) parts.push(p[1]);
  }
  if (parts[0] !== 'Tabr Tau') parts.push('Tabr Tau');
  return parts.join(' - ');
}
// The page is drawn again after almost every action, which destroys the control that had the keyboard focus. focusSpec notes which control it was
// (by id, or by its data-* attributes and label) and refocus gives the focus back to the same control in the new page, unless something else took it
// meanwhile (register 2026-10-15, UX-07).
function focusSpec(el) {
  if (!el || el === document.body || !el.closest || !el.closest('#main, #nav')) return null;
  const data = [...el.attributes].filter((a) => a.name.startsWith('data-')).map((a) => [a.name, a.value]);
  if (!el.id && !data.length) return null;
  return { id: el.id, tag: el.tagName, data, label: (el.getAttribute('aria-label') || el.textContent || '').trim().slice(0, 60) };
}
function refocus(s) {
  if (!s || (document.activeElement && document.activeElement !== document.body)) return;
  const pool = s.id ? [document.getElementById(s.id)] : [...document.querySelectorAll('#main ' + s.tag + ', #nav ' + s.tag)];
  const same = pool.filter((n) => n && s.data.every(([k, v]) => n.getAttribute(k) === v));
  const n = same.find((x) => (x.getAttribute('aria-label') || x.textContent || '').trim().slice(0, 60) === s.label) || same[0];
  if (n && typeof n.focus === 'function') n.focus({ preventScroll: true });
}
async function render() {
  const keep = focusSpec(document.activeElement);
  document.title = pageTitle();
  drawNav();
  try {
    await ({ player: playerView, livemap: liveMapView, landsraad: landsraadView, config: configView, db: dbView })[tab]();
  } catch (e) {
    setHTML($('#main'), html`<div class="card bad">${e.message}</div><div class="row mt12"><button class="b" data-act="retry">Try again</button></div>`);
  }
  refocus(keep);
}

const D = (e) => e.target.closest('[data-act]');
// A failed SQL statement shows its reason where the result goes, and the earlier result is cleared so it cannot be read as the answer (register 2026-10-07, UX-5).
function sqlFailed(e) { const out = $('#sqlout'); if (out) setHTML(out, html`<p class="bad">${e.message}</p>`); }

const A = {
  retry: () => render(),
  askOk: () => { if (tooSoon()) return; if (askState && askState.typed && $('#atype').value.trim().toLowerCase() !== askState.typed.toLowerCase()) return; endAsk(true); }, // the typed word is checked here too, not only by the disabled button
  askNo: () => endAsk(false),
  confirmLocation: async () => (await ask({ title: 'Allow saving to this location?', body: 'The editor can only see the game on this computer, and this save is somewhere it cannot vouch for (a network share, a removable drive, or a folder other than the one the game itself uses). Saving while the game, on this or another computer, has the file open can corrupt it or lose your work. Continue only if the game is closed everywhere that can open this save.', ok: 'Allow saving', danger: true })) && act(() => api('/api/save/confirm-location', {}), 'Saving is allowed for this save until the editor closes'),
  reload: async () => (await ask({ title: 'Reload from disk?', body: 'The file is read again. Any changes still queued are put back onto it where they still apply; one that no longer applies is listed so you can make it again.', ok: 'Reload' })) && act(async () => {
    const r = await api('/api/save/rebase', {});
    const x = r && r.restored;
    toast(x ? `Reloaded: ${x.applied} change${x.applied === 1 ? '' : 's'} kept` + ((x.conflicts || []).length ? `, ${x.conflicts.length} could not be put back` : '') : 'Reloaded from disk.');
  }, null),
  dismissRestored: () => act(async () => { await api('/api/save/restored/dismiss', {}); }, null),
  discardQueue: () => $('#btnDiscard').click(),
  writeNow: () => act(async () => {
    const r = await api('/api/save/flush', {});
    if (r.saved) toast('Written to the game file' + (r.warning ? '. ' + r.warning : '.'), !!r.warning);
    else toast(r.queued ? 'Still waiting: ' + (r.why || 'the file cannot be written yet') : 'Nothing was waiting.', !!r.queued);
  }, null, true, true),
  solari: () => act(() => api('/api/player/solari', { amount: +val('solari') }), 'Solari updated'),
  xp: () => act(async () => { const r = await api('/api/player/xp', { amount: +val('xpAmt') }); toast(r.applied ? `XP ${Number(r.before).toLocaleString()} → ${Number(r.after).toLocaleString()}, level ${r.levelBefore} → ${r.levelAfter}` + (r.skillPointsGained ? `, +${r.skillPointsGained} skill points` : '') + (r.capped ? ' (stopped at level 200)' : '') + NOT_SAVED : 'Already at the last level (200).'); }, null),
  currency: () => act(async () => { const r = await api('/api/player/currency', { currency: +val('curSel'), amount: +val('curAmt') }); toast(`${r.currency == 0 ? 'Solari in wallet' : 'House credit in wallet'} ${Number(r.before).toLocaleString()} → ${Number(r.after).toLocaleString()}` + NOT_SAVED); }, null),
  intel: () => act(async () => { const r = await api('/api/player/intel', { amount: +val('intelAmt') }); toast(r.applied ? `Intel ${Number(r.before).toLocaleString()} → ${Number(r.after).toLocaleString()}` + (r.capped ? ' (capped at 2,779)' : '') + NOT_SAVED : 'Intel is already at the cap (2,779).'); }, null),
  teleport: () => act(() => { const body = { x: +val('tx'), y: +val('ty'), z: +val('tz') }; if (val('tdyaw') !== '') body.yaw = +val('tdyaw'); return api('/api/player/teleport', body); }, 'Teleport set'),
  tdFind: async () => {
    try {
      const r = await api(`/api/player/destinations?kind=${encodeURIComponent(val('tdkind'))}&q=${encodeURIComponent(val('tdq'))}`);
      TD = r.destinations;
      const more = r.total > TD.length ? ` (nearest ${TD.length} of ${r.total}; filter to narrow)` : '';
      setHTML($('#tdout'), TD.length ? html`<select id="tdsel"><option value="">${TD.length} place${TD.length == 1 ? '' : 's'} on ${r.map}, nearest first${more}</option>${TD.map((d, i) => html`<option value="${i}">${d.label} - ${Math.round(d.distance / 100).toLocaleString()} m away</option>`)}</select>` : html`None on ${r.map}.`);
    } catch (e) { toast(e.message, true); }
  },
  tpTo: (d) => act(() => api('/api/player/teleport', { x: +d.x, y: +d.y, z: +d.z }), 'Teleport set'),
  augEdit: (d) => act(async () => { const r = await api('/api/items/augment-options', { item_id: +d.id }); r.options.forEach((o) => AUG_NAMES.set(o.id, o.name)); augItem = { ...r, item_id: +d.id }; render(); }, null, false),
  augCancel: () => { augItem = null; render(); },
  augApply: (d) => act(async () => { const r = await api('/api/items/augment', { item_id: +d.id, augments: augPicked('au', augItem.limit), grade: +val('augG'), unlock_slots: $('#augU').checked }); augItem = null; toast(`Augments set (${r.augments})` + (r.slotsUnlocked ? `, ${r.slotsUnlocked} slot keystones unlocked` : '') + NOT_SAVED); }, null),
  queueAdd: () => { let t; try { t = needItem('gt'); } catch (e) { return toast(e.message, 'err'); } const e = { template_id: t, quantity: Math.max(1, +val('gq') || 1), quality: +val('gg') }; const ids = augPicked('ga', 3); if (ids.length) { e.augments = ids; e.grade = +val('gag'); } giveQueue.push(e); keepQueue(); render(); },
  queueDel: (d) => { giveQueue.splice(+d.id, 1); keepQueue(); render(); },
  queueClear: () => { giveQueue = []; keepQueue(); render(); },
  queueGive: () => act(async () => { const r = await api('/api/player/give-items', { items: giveQueue }); giveQueue = []; keepQueue(); giveToast(r, 'units of the queued items', `Added ${r.count} items`); }, null),
  give: () => act(async () => {
    const ids = augPicked('ga', 3), tid = needItem('gt');
    const r = ids.length ? await api('/api/player/give-items', { items: [{ template_id: tid, quantity: +val('gq'), quality: +val('gg'), augments: ids, grade: +val('gag') }] })
      : await api('/api/player/give', { template_id: tid, quantity: +val('gq'), quality: +val('gg') });
    giveToast(r, itemName(tid), 'Item added');
  }, null),
  giveWater: () => act(async () => { const r = await api('/api/player/give', { template_id: 'WaterPack_Consumable', quantity: 10, quality: 0 }); giveToast(r, 'Cup of Water', '10 x Cup of Water added'); }, null),
  faction: () => act(async () => { const r = await api('/api/player/faction', { faction_id: +val('facSel') }); toast(`Faction set to ${r.faction}` + NOT_SAVED); }, null),
  repair: () => act(async () => { const r = await api('/api/player/repair', {}); toast(`Repaired ${r.repaired} items` + NOT_SAVED); }, null),
  fillWater: () => act(async () => { const r = await api('/api/player/hydration', {}); toast(`Hydration ${Math.round(r.before * 10) / 10} → ${r.after}` + NOT_SAVED); }, null),
  refill: () => act(async () => { const r = await api('/api/player/refill', {}); const sk = (r.skippedUnknown || []).length ? ` Left alone, capacity unknown: ${r.skippedUnknown.join(', ')}.` : ''; toast(`Filled ${r.filled} Literjons (${r.alreadyFull} already full).${sk}` + NOT_SAVED); }, null),
  delItem: async (d) => (await ask({ title: 'Delete item?', body: 'Delete ' + d.name + ' and everything attached to it (its stats and links). It is written at once, after a backup. If the file cannot be written (for example a single-player session is running) it is queued and written when it can be.', ok: 'Delete', danger: true })) && act(() => api('/api/items/delete', { id: +d.id }), 'Deleted'),
  clearInv: async (d) => {
    const cur = !!($('#clrCur') && $('#clrCur').checked), n = +(cur ? d.all : d.plain);
    if (!n) { toast(+d.all ? 'Nothing to clear: this container holds only currency. Tick the box to remove it too.' : 'Nothing to clear: this container is empty.'); return; }
    const body = `This removes ${nn(n, 'item', 'items')} from ${d.name}, with everything attached to them (stats, links, and what is inside a bag). ${cur ? 'The currency is removed too.' : 'The Solari Coin stack is kept.'} It is one edit, written at once after a backup. If the file cannot be written (for example a single-player session is running) it is queued and written when it can be. Items added to the container meanwhile make the editor refuse and change nothing.`;
    if (!(await ask({ title: 'Clear ' + d.name + '?', body, ok: 'Clear container', typed: String(n), danger: true }))) return;
    act(async () => { const r = await api('/api/inventories/clear', { inventory_id: +d.inv, count: n, include_currency: cur }); toast(r.removed ? `Cleared ${d.name}: ${nn(r.removed, 'item', 'items')} removed` + NOT_SAVED : r.message); }, null);
  },
  spec: (d) => { $('#st').value = d.t; $('#sx').value = d.xp; $('#sl').value = d.lv; },
  spKeyAll: async () => (await ask({ title: 'Grant all keystones?', body: 'Every specialization keystone of every track is bought. One edit.', ok: 'Grant all', danger: true })) && act(async () => { const r = await api('/api/player/specs/keystones/grant', {}); toast(r.granted ? `${r.granted} keystones granted` + NOT_SAVED : 'All keystones were already granted.'); }, null),
  spKeyReset: async () => (await ask({ title: 'Reset all keystones?', body: 'Every specialization keystone you have bought is removed. One edit.', ok: 'Reset all', danger: true })) && act(async () => { const r = await api('/api/player/specs/keystones/reset', {}); toast(r.removed ? `${r.removed} keystones reset` + NOT_SAVED : 'There were no keystones to reset.'); }, null),
  spAdd: (d) => act(async () => { const r = await api('/api/player/specs/xp', { track: d.t, amount: +val('spx' + d.t) }); toast(`${d.t}: ${Number(r.before).toLocaleString()} → ${Number(r.after).toLocaleString()} XP, level ${Math.floor(r.level)}` + NOT_SAVED); }, null),
  spMax: async (d) => (await ask({ title: 'Grant max ' + d.t + '?', body: d.t + ' goes to the XP of its last level (level 100). One edit.', ok: 'Grant max', danger: true })) && act(async () => { await api('/api/player/specs/max', { track: d.t }); toast(d.t + ' set to max' + NOT_SAVED); }, null),
  spReset: async (d) => (await ask({ title: 'Reset ' + d.t + '?', body: d.t + " goes back to 0 XP. Its keystones are not changed. One edit.", ok: 'Reset', danger: true })) && act(async () => { const r = await api('/api/player/specs/reset', { track: d.t }); toast(r.removed ? d.t + ' reset' + NOT_SAVED : d.t + ' had no XP to reset; nothing changed.'); }, null),
  spAssign: (d) => act(async () => { const t = val('spa' + d.n); await api('/api/player/specs/assign', { track: t, number: +d.n }); toast(`Track number ${d.n} is ${t}`); }, null, false),
  spAssignHand: () => act(async () => { const t = val('spmt'), n = val('spmn'); if (n === '') throw new Error('Type the track number first.'); await api('/api/player/specs/assign', { track: t, number: +n }); toast(`${t} is track number ${n}`); }, null, false),
  specSet: async () => (await ask({ title: 'Set this raw row?', body: `Track number ${+val('st')} is set to ${(+val('sx')).toLocaleString()} XP at level ${+val('sl')}. If that number belongs to another track than you mean, the other track changes. One edit.`, ok: 'Set the row', danger: true })) && act(() => api('/api/player/specs', { track_type: +val('st'), xp: +val('sx'), level: +val('sl') }), 'Specialization set'),
  tut: (d) => act(async () => { await api('/api/player/tutorials', { id: +d.id, complete: d.c == '1' }); JB = await api('/api/player/journey/browse'); journeyList(); toast('Tutorial updated' + NOT_SAVED); }, null),
  tagAdd: () => act(() => api('/api/player/tags', { tag: val('tag').trim(), add: true }), 'Tag added'),
  tagDel: (d) => act(() => api('/api/player/tags', { tag: d.tag, add: false }), 'Tag removed'),
  invfilter: () => { invQ = val('invq'); return render(); },
  invclear: () => { invQ = ''; render(); },
  skPoints: () => act(async () => { const r = await api('/api/player/skills/points', { points: +val('skPts') }); toast(`Unspent skill points ${r.before} → ${r.after}` + NOT_SAVED); }, null),
  skStarter: () => act(async () => { const r = await api('/api/player/skills/starter', { school: skSchool }); toast(r.changed ? `Restored ${r.changed} starter skills` + NOT_SAVED : 'The starter skills are already learned.'); }, null),
  ctfilter: () => { ctQ = val('ctq'); catalogList(); },
  ctclear: () => { ctQ = ''; catalogList(); },
  ctGive: (d) => act(async () => { await api('/api/player/give-items', { items: [{ template_id: d.id, quantity: 1, quality: 0 }] }); toast('Item added to your backpack' + NOT_SAVED); }, null),
  bsAll: async () => (await ask({ title: 'Unlock all building sets?', body: 'Every building set in the list below, except the experimental ones, is added to the sets you have learned. This is one edit.', ok: 'Unlock all' })) && act(async () => { const r = await api('/api/player/building-sets/unlock-all', {}); toast(r.added ? `Learned ${r.added} building sets` + NOT_SAVED : 'All building sets are already learned.'); }, null),
  rsAll: async () => (await ask({ title: 'Unlock all research?', body: 'Every research entry that unlocks a recipe or a building set is bought, and the recipes and sets they unlock are added. One edit.', ok: 'Unlock all' })) && act(async () => { const r = await api('/api/player/research/unlock-all', {}); toast(r.bought || r.repaired ? `Research: ${r.bought} bought, ${r.repaired} repaired, ${r.unlocksAdded} recipes or sets added` + NOT_SAVED : 'All research is already unlocked.'); }, null),
  crAll: async () => (await ask({ title: 'Unlock all recipes?', body: 'Every crafting recipe your research tree offers that you do not know yet is added to your known recipes. One edit.', ok: 'Unlock all' })) && act(async () => { const r = await api('/api/player/crafting/unlock-all', {}); toast(r.added ? `Added ${r.added} recipes` + NOT_SAVED : 'You already know every recipe.'); }, null),
  lvReset: () => {
    const xp = SUMMARY && Number.isInteger(SUMMARY.xp) ? SUMMARY.xp : 0;
    if (!xp) { toast('The character has no XP, so there is nothing to reset.'); return; }
    const lvl = SUMMARY.level, sp = SUMMARY.skillPoints;
    const fmt = (n) => Number(n).toLocaleString();
    return resetArea({ count: xp, title: `Reset level ${lvl} and ${fmt(xp)} XP to 0?`, ok: 'Reset level and XP', typed: `reset ${xp} xp`,
      body: `The character's XP goes to 0 and the level with it. The skill points the levels gave are taken back, as many as the level (${lvl}) from the total${sp ? ` (now ${fmt(sp.total)})` : ''} and from the unspent points${sp ? ` (now ${fmt(sp.unspent)})` : ''}, never below 0. Skills you bought are not touched: reset them on the Skills tab first, with the box ticked so their points come back, to be left with none. One edit.`,
      path: '/api/player/level/reset', done: (r) => `Level ${r.levelBefore} and ${fmt(r.changed)} XP reset to 0`, none: 'The character has no XP, so there was nothing to reset.' });
  },
  jReset: (d) => {
    const n = +d.n || 0, nodes = +d.nodes || 0, tuts = +d.tuts || 0;
    if (!n) { toast('No Story or Codex step is finished and no tutorial is started, so there is nothing to reset.'); return; }
    return resetArea({ count: n, title: `Reset the journey (${nn(nodes, 'step', 'steps')} and ${nn(tuts, 'tutorial', 'tutorials')})?`, ok: 'Reset journey', typed: `reset ${n} journey ${n === 1 ? 'item' : 'items'}`,
      body: `Finished Story and Codex steps (${nodes}) go back to not finished, the same as the Reset button of each row: a step and its children are marked not finished and revealed. Started or finished tutorials (${tuts}) go back to not started. Rewards that are waiting stay as they are (whether the game gives a reward again when a step is finished again is not known), and tags and contracts are not touched. One edit.`,
      path: '/api/player/journey/reset', done: (r) => `Journey reset: ${nn(r.nodes, 'step', 'steps')} and ${nn(r.tutorials, 'tutorial', 'tutorials')}`, none: 'No Story or Codex step is finished and no tutorial is started, so there was nothing to reset.' });
  },
  spResetAll: (d) => {
    const n = +d.n || 0, left = +d.left || 0;
    if (!n) { toast('No track whose number is known has XP, so there is nothing to reset.'); return; }
    return resetArea({ count: n, title: `Reset the XP of ${nn(n, 'track', 'tracks')}?`, ok: 'Reset XP', typed: `reset ${n} ${n === 1 ? 'track' : 'tracks'}`,
      body: `The XP and level of every specialization track whose number the editor knows go back to 0, the same as the Reset button of each track. Keystones are not touched.${left ? ` ${nn(left, 'row', 'rows')} of the save whose track is not known yet ${left === 1 ? 'is' : 'are'} left alone, since removing ${left === 1 ? 'it' : 'them'} could clear the wrong track.` : ''} One edit.`,
      path: '/api/player/specs/reset-all', done: (r) => `${nn(r.changed, 'track', 'tracks')} reset to 0 XP`, none: 'No track whose number is known has XP, so there was nothing to reset.' });
  },
  skReset: () => {
    const n = (SK && SK.resetCount) || 0;
    if (!n) { toast('Every skill of the five schools is already at rank 0, so there is nothing to reset.'); return; }
    const back = skCharge; // the box above the list: the same choice as for a single skill
    return resetArea({ count: n, title: `Reset ${nn(n, 'skill', 'skills')} to rank 0?`, ok: 'Reset skills', typed: `reset ${n} ${n === 1 ? 'skill' : 'skills'}`,
      body: `Every skill of the five schools that has points goes back to rank 0. ${back ? 'The points they cost are given back to your unspent skill points (the box above the list is ticked).' : 'Your unspent skill points do not change (the box above the list is not ticked).'} Your level and XP, the Hidden skills and the skills the catalog does not know are not touched. One edit.`,
      path: '/api/player/skills/reset', extra: { charge: back }, done: (r) => `${nn(r.changed, 'skill', 'skills')} reset to rank 0`, none: 'Every skill of the five schools is already at rank 0.' });
  },
  skMax: async (d) => (await ask({ title: 'Max skills?', body: d.school == 'all' ? 'Every skill of the five schools is set to its highest rank. The unspent points are not changed. One edit.' : 'Every skill of this school is set to its highest rank. The unspent points are not changed.', ok: 'Max skills' })) && act(async () => { const r = await api('/api/player/skills/max', { school: d.school }); toast(r.changed ? `${r.changed} skills raised to their highest rank` + NOT_SAVED : 'Those skills are already at their highest rank.'); }, null),
  ctUnlockSet: async (d) => (await ask({ title: 'Unlock ' + d.name + '?', body: 'Every ordinary cosmetic of ' + d.name + ' that is not unlocked yet is added to your character\'s customization library, with no item in your backpack. One edit. Store and DLC cosmetics are skipped: they need an account entitlement, and the game removes the ones your account does not own.', ok: 'Unlock' })) && act(async () => { const r = await api('/api/player/customizations/unlock', { group: d.id }); const sk = r.skippedEntitlement ? ` (${r.skippedEntitlement} store or DLC cosmetics skipped: they need an account entitlement)` : ''; toast((r.added ? `Unlocked ${r.added} customizations` + NOT_SAVED : 'Nothing new to unlock in that set') + sk); }, null),
  ctUnlock: async (d) => (await ask({ title: 'Unlock ' + d.name + '?', body: 'This cosmetic is added to your character\'s customization library, with no item in your backpack. One edit.' + (d.ent ? ' It needs an account entitlement: if your account does not own it, the game removes the unlock when it loads the save (it did for a store cosmetic in a test).' : ''), ok: 'Unlock' })) && act(async () => { const r = await api('/api/player/customizations/unlock', { id: d.id }); toast(r.added ? `Unlocked ${r.added} customization${r.added > 1 ? 's' : ''}` + NOT_SAVED + (r.warning ? ' ' + r.warning : '') : 'That cosmetic is already unlocked.', !!r.warning, r.warning ? 9000 : 3500); }, null),
  ctGrantSet: async (d) => (await ask({ title: 'Give ' + d.name + ' as items?', body: 'One of each cosmetic item that is not already in your inventory goes into your backpack, all or nothing, so it needs room for them all. Unlock set avoids that. This does not give a DLC or an account entitlement a set may need.', ok: 'Give' })) && act(async () => { const r = await api('/api/player/customizations/grant', { group: d.id }); toast(r.granted ? `Added ${r.granted} cosmetics to your backpack` + NOT_SAVED : 'All of that set is already in your inventory.'); }, null),
  lcClear: () => { LC.sel = new Set(); LC.rm = new Set(); claimEditorDraw(); },
  lcRemove: async () => {
    const c = LC.claim, cells = [...LC.rm].map(lcCoords).sort((p, q) => p[1] - q[1] || p[0] - q[0]).map(([x, y]) => ({ x, y }));
    if (!(await ask({ title: 'Remove land claim cells?', body: `Remove ${cells.length} cell${cells.length === 1 ? '' : 's'} from the claim. A cell that holds a building piece or a placeable is never removed, and the claim must stay connected to the totem; the request is refused whole otherwise. It is written at once, after the usual backup; the game loads it next time.`, ok: 'Remove cells' }))) return;
    act(async () => { const r = await api('/api/bases/claim/remove', { totem_id: c.totem_id, cells }); LC = null; toast(`Land claim: ${r.removed} cell${r.removed === 1 ? '' : 's'} removed, ${r.remaining} left` + NOT_SAVED); }, null);
  },
  lcApply: async () => {
    const c = LC.claim, cells = [...LC.sel].map(lcCoords).sort((p, q) => p[1] - q[1] || p[0] - q[0]).map(([x, y]) => ({ x, y }));
    const body = { totem_id: c.totem_id, cells };
    if (LC.level !== c.level) body.level = LC.level;
    if (!(await ask({ title: 'Edit land claim?', body: `Add ${cells.length} horizontal cell${cells.length === 1 ? '' : 's'}${LC.level !== c.level ? ` and raise the vertical level from ${c.level} to ${LC.level}` : ''}. It is written at once, after the usual backup; the game loads it next time.`, ok: 'Apply changes' }))) return;
    act(async () => { const r = await api('/api/bases/claim/apply', body); LC = null; toast(`Land claim: ${r.added} cell${r.added === 1 ? '' : 's'} added` + (r.levelRaised ? `, vertical level ${r.level}` : '') + NOT_SAVED); }, null);
  },
  rsfilter: () => { rsQ = val('rsq'); researchList(); },
  rsclear: () => { rsQ = ''; rsGroup = ''; researchList(); },
  rsUnlock: (d) => act(async () => { const r = await api('/api/player/research/unlock', { item_key: d.key }); toast((r.repaired ? 'Unlock repaired' : r.alreadyPurchased ? 'Already purchased' : 'Research unlocked') + NOT_SAVED); }, null),
  crfilter: () => { crQ = val('crq'); craftingList(); },
  crclear: () => { crQ = ''; craftingList(); },
  crUnlock: (d) => act(async () => { const r = await api('/api/player/crafting/unlock', { recipe_id: d.id }); toast((r.alreadyKnown ? 'Already known' : 'Recipe unlocked') + NOT_SAVED); }, null),
  jfilter: () => { window.jq = val('jq'); journeyList(); },
  jclear: () => { window.jq = ''; journeyList(); },
  jset: (d) => act(async () => { await api('/api/player/journey', { node_id: d.id, complete: d.c == '1' }); JB = await api('/api/player/journey/browse'); journeyList(); toast('Journey updated' + NOT_SAVED); }, null),
  autoref: async (d) => {
    if (!d.on && !(await ask({ title: 'Turn on automatic refill?', body: 'From now on, opening the editor refills water storage and power buildings and saves to your game file immediately, like every other change. The previous file is backed up each time, and nothing happens while a single-player session is running. It applies to whichever save file the editor opens.', ok: 'Turn on' }))) return;
    act(async () => { const r = await api('/api/settings', { autoRefillOnOpen: !d.on }); toast('Automatic refill when the editor opens is now ' + (r.autoRefillOnOpen ? 'on' : 'off')); }, null);
  },
  claimShrink: async (d) => {
    const rings = +val('cs' + d.id);
    const size = rings ? `${2 * rings + 1} x ${2 * rings + 1}` : 'the totem\'s own cell';
    if (!(await ask({ title: 'Shrink this land claim?', body: `Removes every stored cell outside ${size}. A cell that holds building pieces is never removed. It is written at once, after the usual backup.`, ok: 'Shrink claim' }))) return;
    act(async () => { const r = await api('/api/bases/claim/shrink', { totem_id: +d.id, rings }); toast(`Removed ${r.removed} cells; the claim now has ${r.remaining}` + NOT_SAVED); });
  },
  claimGrow: async (d) => {
    const rings = +val('cr' + d.id), level = +val('cv' + d.id);
    const body = { totem_id: +d.id };
    if (rings > 0) body.rings = rings;
    if (level > +d.level) body.level = level;
    if (body.rings === undefined && body.level === undefined) { toast('Nothing to change: pick a bigger size or a higher level', true); return; }
    const what = [body.rings ? `every missing cell up to a ${2 * rings + 1} x ${2 * rings + 1} square around the totem` : '', body.level !== undefined ? `the vertical level raised to ${level}` : ''].filter(Boolean).join(' and ');
    if (!(await ask({ title: 'Expand this land claim?', body: `Adds ${what}. It is written at once, after the usual backup.`, ok: 'Expand claim' }))) return;
    act(async () => { const r = await api('/api/bases/claim/expand', body); toast(`Claim now ${r.totalCells} cells, level ${r.level} (+${r.added} cells)` + NOT_SAVED); });
  },
  refillWater: () => act(async () => { const r = await api('/api/bases/refill-water', {}); const sk = (r.skippedUnknown || []).length ? ` Left alone, capacity unknown: ${r.skippedUnknown.join(', ')}.` : ''; toast((r.filled ? `Refilled ${r.summary} (${r.alreadyFull} already full).` : `All ${r.alreadyFull} water buildings were already full.`) + sk + NOT_SAVED); }, null),
  refillGen: () => act(async () => { const r = await api('/api/bases/refill-generators', {}); toast((r.filled ? `Refilled ${r.summary} (${r.alreadyFull} already full).` : `All ${r.alreadyFull} power buildings were already full.`) + ((r.skipped || []).length ? ` No free slot: ${r.skipped.join(', ')}.` : '') + NOT_SAVED); }, null),
  repairOne: (d) => act(async () => { const r = await api('/api/vehicles/repair', { vehicle_id: +d.id }); toast(repairMsg(r, 'modules of this vehicle') + (r.repaired ? NOT_SAVED : '')); }, null),
  repairV: () => act(async () => { const pct = $('#rvPct'); const n = pct ? +pct.value : 100; const r = await api('/api/vehicles/repair', pct && n < 100 ? { threshold: n } : {}); toast(repairMsg(r, 'vehicle modules', n) + (r.repaired ? NOT_SAVED : '')); }, null),
  repairB: () => act(async () => { const r = await api('/api/bases/repair', {}); toast(`Repaired ${r.pieces} pieces, ${r.placeables} placeables` + NOT_SAVED); }, null),
  sand: () => act(async () => { const r = await api('/api/bases/clear-sand', {}); toast(`Cleared ${r.pieces} pieces` + NOT_SAVED); }, null),
  openInv: async (d) => {
    const it = await api('/api/bases/storage/items?inventory=' + encodeURIComponent(d.id));
    const cat = await loadCatalog(it.map((r) => r.template_id));
    setHTML($('#inv'), html`<div class="card"><h3>Inventory ${d.id}</h3>${tbl([{ k: 'position_index', label: 'Slot' }, { k: 'template_id', label: 'Item', f: (r) => itemCell(r.template_id) }, { k: 'stack_size', label: 'Stack', f: (r) => html`<input type="number" min="1" value="${r.stack_size}" data-item="${r.id}" data-field="stack_size" data-name="${itemName(r.template_id)}" class="w90" aria-label="Stack of ${itemName(r.template_id)}">` }], it, { empty: 'This container is empty.', actions: (r) => html`<button class="b bad sm" data-act="delItem" data-id="${r.id}" data-name="${itemName(r.template_id)}">Delete</button>` })}
    <div class="row">${itemPicker("bt", cat)}Qty<input id="bq" type="number" value="1" min="1"><button class="b" data-act="giveInv" data-id="${d.id}" data-name="${d.name || ''}">Add item</button></div>${clearRow(d.id, d.name || 'this container', it)}</div>`);
  },
  giveInv: (d) => act(async () => { const tid = needItem('bt'); const r = await api('/api/bases/give', { inventory_id: +d.id, template_id: tid, quantity: +val('bq') }); giveToast(r, itemName(tid), 'Item added'); await A.openInv(d); }, null, false),
  bring: (d) => act(() => api('/api/vehicles/bring', { id: +d.id }), 'Vehicle moved next to you'),
  vreset: async (d) => (await ask({ title: d.v ? 'Reset purchase limits for this vendor?' : 'Reset purchase limits for every vendor?', body: d.v ? 'Resets the purchase counters of vendor ' + d.v + '.' : 'Resets the purchase counters of every vendor in the save, not one.', ok: 'Reset' })) && act(() => api('/api/vendors/reset', { vendor_id: d.v }), 'Purchase limits reset'),
  term: () => act(async () => { await api('/api/landsraad/term', { active_decree_id: val('ad'), reigning_faction_id: val('rf') }); }, 'Term updated'),
  decree: (d) => act(() => api('/api/landsraad/decree', { id: +d.id, disabled: d.d == '1' }), 'Decree updated'),
  tfill: (d) => act(() => api('/api/landsraad/progress', { task_id: +d.id, faction_id: +val('lf'), amount: +d.goal }), 'Progress set'),
  tdone: async (d) => (await ask({ title: d.r == '1' ? 'Reset this task?' : 'Complete this task?', body: 'Changes the Landsraad task for the chosen faction.', ok: d.r == '1' ? 'Reset' : 'Complete' })) && act(() => api('/api/landsraad/complete', { task_id: +d.id, faction_id: +val('lf'), reset: d.r == '1' }), 'Task updated'),
  cfgcreate: (d) => act(() => api('/api/config/create', { name: d.n }), 'Created ' + d.n + ' now', true, true),
  cfgadd: () => act(() => api('/api/config/set', { name: cfgFile, section: val('cs').replace(/^\[|\]$/g, ''), key: val('ck').trim(), value: val('cv') }), 'Added to the config file now', true, true),
  cfgdel: async (d) => (await ask({ title: 'Delete config line?', body: 'Delete ' + d.key + ' from ' + cfgFile + '. A backup of the file is made first.', ok: 'Delete', danger: true })) && act(() => api('/api/config/delete', { name: cfgFile, line: +d.line, key: d.key }), 'Deleted from the config file now', true, true),
  cfgraw: async () => (await ask({ title: 'Replace the whole file?', body: 'Everything in ' + cfgFile + ' is replaced by the text in the box. It is written now, after a backup of the file is made. Close the game first.', ok: 'Replace file', danger: true })) && act(() => api('/api/config/raw', { name: cfgFile, text: val('raw') }), 'File written now (the previous text is in Backups)', true, true),
  cfgrestore: async (d) => (await ask({ title: 'Restore config backup?', body: 'Replace ' + cfgFile + ' with ' + d.b + '. Close the game first.', ok: 'Restore', danger: true })) && act(() => api('/api/config/restore', { name: cfgFile, backup: d.b }), 'Config file restored now', true, true),
  dbgo: () => { dbQ = val('dbq'); dbOff = 0; return render(); },
  dbprev: () => { dbOff = Math.max(0, dbOff - 100); render(); },
  dbnext: () => { dbOff += 100; render(); },
  runsql: async () => {
    window.lastSql = val('sql');
    try {
      const r = await api('/api/db/sql', { sql: window.lastSql });
      setHTML($('#sqlout'), html`<p class="mut">${r.rows.length.toLocaleString()} ${r.rows.length === 1 ? 'row' : 'rows'}${r.truncated ? '. Only the first 1,000 rows are shown: add LIMIT or a WHERE to see others.' : ''}</p><div class="scroll"><table><thead><tr>${r.columns.map((c) => html`<th>${c}</th>`)}</tr></thead><tbody>${r.rows.map((x) => html`<tr>${x.map((v) => html`<td>${v}</td>`)}</tr>`)}</tbody></table></div>`);
    } catch (e) { toast(e.message, true); sqlFailed(e); }
  },
  execsql: async () => {
    window.lastSql = val('sql');
    if (!(await ask({ title: 'Run this SQL as a write?', body: 'This statement is written to the game file at once, after the usual backup. If the file cannot be written (for example a single-player session is running) it waits in the queue and is written when it can be:', pre: window.lastSql, ok: 'Run', typed: 'write', danger: true }))) return;
    try {
      const r = await api('/api/db/exec', { sql: window.lastSql });
      toast(r.changes + (r.changes === 1 ? ' row changed' : ' rows changed'));
      let queued = r.outcome === 'queued';
      if (!r.outcome) try { queued = (await status()).dirty; } catch (e) { /* the poll reports a lost connection; the statement itself ran */ }
      setHTML($('#sqlout'), html`<p class="ok">${r.changes} ${r.changes === 1 ? 'row' : 'rows'} changed, ${queued ? 'and queued: it is written to game.db as soon as the file can be written.' : 'and written to game.db.'}</p>`);
    } catch (e) { toast(e.message, true); sqlFailed(e); }
  },
  restore: async (d) => (await ask({ title: 'Restore this backup over the save?', body: 'Replace game.db with ' + d.n + '. The current file is backed up first. Close the game first.', ok: 'Restore', typed: 'restore', danger: true })) && act(() => api('/api/save/restore', { name: d.n }), 'Restored'),
};
document.addEventListener('click', (e) => {
  const cf0 = e.target.closest('[data-confirm]');
  if (armed && !(cf0 && cf0 === armed.btn)) disarm(); // anything but the second press of the same button cancels
  if (cf0 && !cf0.dataset.armed) { e.preventDefault(); arm(cf0); return; }
  if (cf0 && armed && cf0 === armed.btn && (e.detail > 1 || Date.now() - armed.at < ARM_GAP_MS)) { e.preventDefault(); e.stopPropagation(); return; } // the second half of a double click, or two presses too close to be two decisions: still asking
  if (cf0) disarm(); // the second press: the label goes back and the action below runs
  const keep = e.target.closest('summary[data-keep]');
  if (keep && Object.hasOwn(KEEP, keep.dataset.keep)) KEEP[keep.dataset.keep] = !keep.parentElement.open; // the toggle is about to flip
  const hth = e.target.closest('thead th');
  if (hth && !e.target.closest('button, a, input, select') && headerClick(hth)) return;
  const t = e.target.closest('[data-tab]');
  if (t) { if (Object.hasOwn(TABS, t.dataset.tab)) { tab = t.dataset.tab; browserStore.set('tab', tab); render(); } return; }
  const rv = e.target.closest('[data-reveal]');
  if (rv) {
    const k = rv.dataset.reveal;
    if (SECRET_IDS.includes(k) && $('#sumcard')) { if (SHOWN.has(k)) SHOWN.delete(k); else SHOWN.add(k); setHTML($('#sumcard'), summaryBody(SUMMARY)); }
    return;
  }
  const gt = e.target.closest('[data-goto]'); // a link to the one place where something is done
  if (gt) { const [a, b] = gt.dataset.goto.split(':'); if (PTABS.some(([k]) => k === a)) { sub.player = a; if (a === 'character' && ['overview', 'inventory', 'reputation'].includes(b)) sub.char = b; render(); } return; }
  const cg = e.target.closest('[data-ctg]');
  if (cg) { ctGroup = cg.dataset.ctg; catalogList(); return; }
  const lcc = e.target.closest('[data-cell]');
  if (lcc && LC) { lcCellClick(lcc.getAttribute('data-cell')); claimEditorDraw(); return; }
  const cf = e.target.closest('[data-ctf]');
  if (cf) { ctFilter = cf.dataset.ctf; catalogList(); return; }
  const rc = e.target.closest('[data-rscat]');
  if (rc) { rsCat = rc.dataset.rscat; rsGroup = ''; researchList(); return; }
  const cc = e.target.closest('[data-crcat]');
  if (cc) { crCat = cc.dataset.crcat; craftingList(); return; }
  const sg = e.target.closest('[data-skgrp]');
  if (sg) { skSchool = sg.dataset.skgrp; skillsList(); return; }
  const jg = e.target.closest('[data-jgrp]');
  if (jg) { if (JGROUPS.some(([k]) => k == jg.dataset.jgrp)) { jgrp = jg.dataset.jgrp; journeyList(); } return; }
  const vo = e.target.closest('[data-vhopen]');
  if (vo) { vhOpen = +vo.dataset.vhopen; render(); return; }
  const bo = e.target.closest('[data-bsopen]');
  if (bo) { bsOpen = +bo.dataset.bsopen; render(); return; }
  const bt = e.target.closest('[data-bstab]');
  if (bt) { bsTab = bt.dataset.bstab; render(); return; }
  const bg = e.target.closest('[data-bsgrp]');
  if (bg) { bsGroup = bg.dataset.bsgrp; render(); return; }
  const ig = e.target.closest('[data-invg]');
  if (ig) { invG = ig.dataset.invg; render(); return; }
  const s = e.target.closest('[data-sub]');
  if (s) { const [g, k] = s.dataset.sub.split(':'); if (GROUPS.includes(g)) { sub[g] = k; render(); } return; }
  const a = D(e);
  if (a) { e.preventDefault(); if (Object.hasOwn(A, a.dataset.act)) Promise.resolve().then(() => A[a.dataset.act](a.dataset)).catch((err) => toast(err.message, true)); }
  const x = e.target.closest('[data-export]');
  if (x) {
    e.preventDefault();
    // An error answer is JSON with the reason; it is shown, not saved as the table's file (register 2026-10-07, UX-6).
    fetch(x.href, { credentials: 'same-origin' }).then(async (r) => {
      if (!r.ok) { const j = await r.json().catch(() => ({})); throw new Error(j.error || r.statusText || 'The export failed'); }
      return r.blob();
    }).then((b) => { const u = URL.createObjectURL(b), l = document.createElement('a'); l.href = u; l.download = dbTable + '.' + x.dataset.export; l.click(); setTimeout(() => URL.revokeObjectURL(u), 60000); /* the browser reads the blob after the click returns */ }).catch((err) => toast(err.message, true));
  }
});
document.addEventListener('change', (e) => {
  const t = e.target;
  if (t.id == 'gt') { showGiveAugments(); return; }
  if (t.id == 'skCharge') { skCharge = t.checked; return; }
  if (t.id == 'rsGrp') { rsGroup = t.value; researchList(); return; }
  if (t.id == 'tdsel') { const d = TD[+t.value]; if (d) { $('#tx').value = Math.round(d.x); $('#ty').value = Math.round(d.y); $('#tz').value = Math.round(d.z); } return; }
  if (t.id == 'lcLvl' && LC) { LC.level = +t.value; claimEditorDraw(); return; }
  if (t.id == 'ctgrp') { ctGroup = t.value; catalogList(); return; }
  if (t.id == 'ctexp') { ctExp = t.checked; if (!ctExp && CT.rows.some((r) => r.experimental && r.group == ctGroup)) ctGroup = ''; catalogList(); return; }
  if (t.dataset.skill) { const said = changedMsg(t, (t.dataset.name || 'Skill') + ': rank'); act(async () => { const r = await api('/api/player/skills/module', { module: t.dataset.skill, level: +t.value, charge: skCharge }); toast(`${said} Skill points ${r.pointsBefore} → ${r.pointsAfter}` + NOT_SAVED); }, null); return; }
  if (t.dataset.item) editInput(t, () => api('/api/items/update', { id: +t.dataset.item, [t.dataset.field]: +t.value }), changedMsg(t, (t.dataset.name || 'Item') + ': ' + (t.dataset.field == 'quality' ? 'grade' : 'stack')));
  else if (t.dataset.faction) editInput(t, () => api('/api/player/factions', { faction_id: +t.dataset.faction, amount: +t.value }), changedMsg(t, (t.dataset.name || 'Faction') + ': reputation'));
  else if (t.classList.contains('cfgv')) editInput(t, () => api('/api/config/set', { name: cfgFile, section: t.dataset.section, key: t.dataset.key, value: t.value, line: +t.dataset.line }), changedMsg(t, t.dataset.key + ' written to the config file now:'), true);
  else if (t.classList.contains('dbc')) editInput(t, async () => {
    try { return await api('/api/db/update', { table: dbTable, ...(t.dataset.kind == '_key_' ? { key: t.dataset.rowid } : { rowid: +t.dataset.rowid }), column: t.dataset.col, value: t.value === '' && t.hasAttribute('data-null') ? null : t.value, expected: t.dataset.orig }); }
    catch (e) { if (e.code === 'conflict') setTimeout(render, 0); throw e; } // the table is out of date: show it as it is now
  }, changedMsg(t, 'Database ' + dbTable + '.' + t.dataset.col + ':'));
  else if (t.id == 'cfgsel') { cfgFile = t.value; render(); }
  else if (t.id == 'dbsel') { dbTable = t.value; dbOff = 0; dbQ = ''; render(); }
});
// keeps the sticky warning banner just below the sticky header, whatever height the header wraps to
const hdrEl = $('header');
const setHdrH = () => document.documentElement.style.setProperty('--hdr-h', hdrEl.offsetHeight + 'px');
if (hdrEl) { setHdrH(); if (window.ResizeObserver) new ResizeObserver(setHdrH).observe(hdrEl); else window.addEventListener('resize', setHdrH); }
const autoBox = $('#autorefresh');
autoBox.checked = autoRefreshOn();
autoBox.addEventListener('change', () => { try { localStorage.autoRefresh = autoBox.checked ? '1' : '0'; } catch (e) { /* the choice lasts until the page is closed */ } status(); });
status().then(render).then(async () => {
  try { const r = await api('/api/startup'); if (r.note) toast(r.note, !!r.warn, 15000); } catch (e) { /* the note is optional */ }
}).catch((e) => { // the editor did not answer: "Loading the save..." would stay for ever
  if ($('#loading')) setHTML($('#main'), html`<div class="card bad">The editor did not answer: ${e.message}</div><div class="row mt12"><button class="b" data-act="retry">Try again</button></div>`);
});
// Keeps the game-running warning current. A failed poll is shown, not hidden: writing stays off until contact returns.
let pollBusy = false; // a poll that is still waiting for an answer (a slow save, a stalled editor) is not asked again on top of itself
setInterval(async () => {
  if (document.hidden || pollBusy) return;
  pollBusy = true;
  try { await status(); } catch (e) {
    offline = true;
    const gw = $('#gamewarn');
    gw.className = 'warn';
    gwKey = '';
    setHTML(gw, html`<b>Lost contact with the editor.</b> Changes are not written until it answers again. If you closed the editor, nothing more will be written.`);
  } finally { pollBusy = false; }
}, 5000);
document.addEventListener('focusout', (e) => { if (armed && e.target === armed.btn) disarm(); });
// Enter in a filter box does what its Filter button does, and the cursor stays in the box (the page may have drawn the box again).
const FILTER_ENTER = { invq: 'invfilter', jq: 'jfilter', rsq: 'rsfilter', crq: 'crfilter', ctq: 'ctfilter', dbq: 'dbgo', tdq: 'tdFind' };
async function runFilter(id) {
  try { await A[FILTER_ENTER[id]]({}); } catch (err) { toast(err.message, true); }
  const box = $('#' + id);
  if (box) { box.focus(); try { box.setSelectionRange(box.value.length, box.value.length); } catch (err) { /* a box with no cursor */ } }
}
document.addEventListener('keydown', (e) => {
  if (armed && e.key === 'Escape') disarm();
  if (e.key === 'Enter' && !e.isComposing && !askState && e.target && Object.hasOwn(FILTER_ENTER, e.target.id)) { e.preventDefault(); runFilter(e.target.id); return; }
  if ((e.key === 'Enter' || e.key === ' ') && !askState && e.target && e.target.matches && e.target.matches('thead th[tabindex]')) { e.preventDefault(); headerClick(e.target); return; }
  if (askState) { // the question owns the keyboard: Tab stays inside it, Enter completes a typed answer, Escape cancels
    if (e.key === 'Escape') endAsk(false);
    else if (e.key === 'Enter' && e.target.id === 'atype' && !$('#aok').disabled) { if (!tooSoon()) endAsk(true); }
    else if (e.key === 'Tab') {
      const f = [...$('#ask').querySelectorAll('input,button,[tabindex="0"]')].filter((x) => !x.disabled);
      if (!f.length) return;
      const i = f.indexOf(document.activeElement);
      const n = e.shiftKey ? (i <= 0 ? f.length - 1 : i - 1) : (i === f.length - 1 ? 0 : i + 1);
      e.preventDefault();
      f[n].focus();
    }
    return;
  }
});
document.addEventListener('mousedown', (e) => { if (askState && e.target === $('#ask')) endAsk(false); });
