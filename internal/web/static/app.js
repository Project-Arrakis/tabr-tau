// Tabr Tau UI. All HTML is built with html`...` (escaped by construction, see html.js) and written with setHTML.
// Nothing here knows about the session: it is an HttpOnly cookie the browser sends on its own.
const { html, setHTML } = window.TabrHTML;
const $ = (s, r = document) => r.querySelector(s);

async function api(path, body) {
  const opt = { credentials: 'same-origin' };
  if (body !== undefined) {
    opt.method = 'POST';
    opt.headers = { 'Content-Type': 'application/json' };
    opt.body = JSON.stringify(body);
  }
  const r = await fetch(path, opt);
  const j = await r.json().catch(() => ({ error: r.statusText }));
  if (r.status === 403 && !j.error) throw new Error('This browser is no longer signed in. Close tabr-tau and start it again, then open the new link from the terminal.');
  if (!r.ok || j.error) { const e = new Error(j.error || r.statusText); e.code = j.code; throw e; }
  return j;
}
function toast(msg, err, ms = 3500) {
  const d = document.createElement('div');
  if (err) d.className = 'err';
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
const NOT_SAVED = ' (not saved yet)';
async function act(fn, okMsg, refresh = true) {
  try {
    const r = await fn();
    let dirty = false;
    try { dirty = (await status()).dirty; } catch (e) { /* the poll reports a lost connection; the edit itself succeeded */ }
    if (okMsg) toast(okMsg + (dirty ? NOT_SAVED : ''));
    if (refresh) await render();
    return r;
  }
  catch (e) { toast(e.message, true); }
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
function itemPicker(inputId, cat, placeholder = 'Item name, e.g. Spice') {
  return html`<input id="${inputId}" list="${inputId}-dl" placeholder="${placeholder}" size="34"><datalist id="${inputId}-dl">${cat.list.map((e) => html`<option value="${e.label}" label="${e.id}"></option>`)}</datalist>`;
}
// What the picker holds -> the template id to send. An exact template id always wins; otherwise a shown label.
function pickedItemId(inputId) {
  const v = val(inputId).trim();
  if (!CATALOG || CATALOG.byId.has(v) || EXTRA_IDS.has(v)) return v;
  return CATALOG.byLabel.get(v.toLowerCase()) || v;
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
const TABS = { player: 'Player', livemap: 'Live Map', landsraad: 'Landsraad', config: 'Config', db: 'Database' };
// The Player tab mirrors the Dune Docker console's Players > Player Name view: the same tabs in the same order (#96). Bases and
// Vehicles used to be top-level tabs and are two of these.
const PTABS = [['character', 'Character'], ['crafting', 'Crafting'], ['research', 'Research'], ['buildingsets', 'Building Sets'], ['customizations', 'Customizations'],
  ['skills', 'Skills'], ['specialization', 'Specialization'], ['journey', 'Journey'], ['blueprints', 'Blueprints'], ['bases', 'Bases'], ['vehicles', 'Vehicles'], ['admin', 'Admin']];
// The console tabs tabr-tau does not have yet; each says so instead of being left out.
const SOON = { blueprints: 'Blueprints' };
let tab = Object.hasOwn(TABS, localStorage.tab) ? localStorage.tab : 'player';
const sub = { player: ['bases', 'vehicles'].includes(localStorage.tab) ? localStorage.tab : 'character', char: 'overview', db: 'browse' };
const GROUPS = ['player', 'char', 'bases', 'db'];
let invG = null, invQ = ''; let giveQueue = []; let augItem = null; // augItem: the inventory item whose augments are open for editing // Character > Inventory: the chosen group (null: the first one with items) and the filter text
function drawNav() {
  setHTML($('#nav'), html`${Object.entries(TABS).map(([k, v]) => html`<button data-tab="${k}" class="${k == tab ? 'on' : ''}">${v}</button>`)}`);
}
function subNav(group, items) {
  return html`<div class="sub">${items.map(([k, v]) => html`<button data-sub="${group}:${k}" class="${sub[group] == k ? 'on' : ''}">${v}</button>`)}</div>`;
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
  [...t.tHead.rows[0].cells].forEach((th, i) => { th.classList.toggle('sa', i === col && dir === 'asc'); th.classList.toggle('sd', i === col && dir === 'desc'); });
}
function headerClick(th) {
  const t = th.closest('table');
  if (!t || !t.tHead || !t.tBodies[0] || !th.textContent.trim()) return false;
  const key = sortKey(t), col = th.cellIndex, cur = SORTS.get(key);
  const dir = cur && cur.col === col ? (cur.dir === 'asc' ? 'desc' : null) : 'asc';
  if (dir) SORTS.set(key, { col, dir }); else SORTS.delete(key);
  sortTable(t, col, dir);
  return true;
}
// A table that has just been drawn gets the sort remembered for its headers.
function applySorts() {
  for (const t of document.querySelectorAll('table')) {
    if (t.classList.contains('sortd') || !t.tHead || !t.tHead.rows[0]) continue;
    t.classList.add('sortd');
    const s = SORTS.get(sortKey(t));
    if (s) sortTable(t, s.col, s.dir);
  }
}
if (typeof MutationObserver !== 'undefined') new MutationObserver(applySorts).observe(document.body, { childList: true, subtree: true });
function tbl(cols, rows, { actions } = {}) {
  if (!rows || !rows.length) return html`<p class="mut">Nothing here.</p>`;
  return html`<div class="scroll"><table><thead><tr>${cols.map((c) => html`<th>${c.label || c.k}</th>`)}${actions ? html`<th></th>` : ''}</tr></thead><tbody>${
    rows.map((r) => html`<tr>${cols.map((c) => html`<td class="${c.cls || ''}">${c.f ? c.f(r) : r[c.k]}</td>`)}${actions ? html`<td>${actions(r)}</td>` : ''}</tr>`)}</tbody></table></div>`;
}
const kv = (k, v) => html`<div class="kv"><b>${k}</b><span>${v}</span></div>`;
const fix = (n) => (n == null ? '' : Math.round(n).toLocaleString());

// ---------- status bar
let saveBlocked = false, saveBlockWhy = '', offline = false;
async function status() {
  const o = await api('/api/save/state');
  offline = false;
  saveBlocked = !!o.saveBlocked;
  saveBlockWhy = o.blockReason || '';
  $('#path').textContent = o.path;
  const gw = $('#gamewarn');
  const note = o.gameMode === 'menu_or_multiplayer';
  gw.className = saveBlocked ? 'warn' : note ? 'info' : 'hide';
  setHTML(gw, o.gameCheckError && saveBlocked ? html`<b>Saving is blocked.</b> ${saveBlockWhy}. You can keep editing; if this keeps happening, report this text.` : saveBlocked ? html`<b>Saving is blocked.</b> ${saveBlockWhy}. You can keep editing; a save made during a single-player session would be overwritten by the game's own autosave.` : note ? html`Dune: Awakening is open (${(o.gameProcesses || []).join(', ')}) but not in single-player, so saving is allowed.` : html``);
  syncReview();
  const n = o.pending.length;
  $('#pending').textContent = n ? `${n} unsaved change${n > 1 ? 's' : ''}` : '';
  $('#btnSave').disabled = $('#btnDiscard').disabled = !o.dirty;
  $('#btnSave').title = o.pending.join('\n');
  const ro = $('#rowarn');
  // Ask before the browser tab is closed with unsaved edits (the native window asks through its own close handler).
  window.onbeforeunload = o.dirty ? (e) => { e.preventDefault(); e.returnValue = ''; } : null;
  ro.className = o.readOnly ? 'warn' : 'hide';
  setHTML(ro, o.readOnly ? html`<b>This save is read-only.</b> ${o.readOnly}<br>You can browse it, but every edit is refused. This usually means the file contains database objects the real game does not create (for example extra triggers) - do not edit a save from an untrusted source.` : html``);
  return o;
}
// ---------- confirmations
// Tiers: reversible edits need nothing (they are pending, listed in the review, and Discard undoes them). A hard-to-undo
// or far-reaching action asks first with its consequence in words. The far-reaching ones (bulk changes, raw SQL
// writes, restoring a backup over the save) also make the person type a word, so a stray click cannot start them.
let askState = null, askFrom = null;
function ask({ title, body, ok = 'Confirm', typed = '', danger = false }) {
  return new Promise((resolve) => { if (askState) askState.resolve(false); askFrom = askFrom || document.activeElement; askState = { title, body, ok, typed, danger, resolve }; drawAsk(); });
}
function drawAsk() {
  const el = $('#ask');
  if (!askState) { el.className = 'hide'; setHTML(el, html``); return; }
  const a = askState;
  el.className = 'modal top';
  setHTML(el, html`<div class="mbox small" role="dialog" aria-modal="true"><h3 class="big" id="atitle" tabindex="-1">${a.title}</h3><p>${a.body}</p>
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

// ---------- review pane: the only way to write the save
let R = null; // the open review: { token, ops, diff, error }
const val1 = (v) => { const t = v === undefined ? 'undefined' : JSON.stringify(v); return t.length > 120 ? t.slice(0, 117) + '...' : t; };
function reviewBody() {
  const d = R.diff, tabs = d.tables || [];
  const cut = tabs.some((t) => (t.modified || []).length < t.numModified);
  return html`<div class="mbox" role="dialog" aria-modal="true"><h3 class="big" id="mtitle" tabindex="-1">Review changes</h3>
    <div id="mmsg">${R.error ? (R.stale ? html`<div class="warn"><b>The save changed on disk.</b> ${R.error}<br>The game (or another program) rewrote game.db after you opened it, so these edits cannot be applied on top of it. Reload the file, then redo the edits listed below.<div class="row"><button class="b" data-act="reload">Reload from disk (discards these edits)</button></div></div>` : html`<div class="warn">${R.error}</div>`) : ''}</div>
    <h3>Edits you made (${R.ops.length})</h3>
    ${R.ops.length ? html`<ul class="tight">${R.ops.map((o) => html`<li>${o.desc}</li>`)}</ul>` : html`<p class="mut">No edits.</p>`}
    <h3>What changes inside game.db</h3>
    ${tabs.length ? html`<div class="scroll"><table><thead><tr><th>Table</th><th>Added</th><th>Removed</th><th>Changed</th></tr></thead><tbody>${tabs.map((t) => html`<tr><td>${t.name}</td><td>${t.numAdded}</td><td>${t.numRemoved}</td><td>${t.numModified}</td></tr>`)}</tbody></table></div>
      <details><summary>Show changed values</summary>${cut ? html`<p class="mut">Only the first changed rows of each table are listed here; the counts above are complete.</p>` : ''}${tabs.map((t) => (t.modified || []).length ? html`<p class="mono"><b>${t.name}</b></p>${t.modified.map((m) => html`<div class="mono">row ${val1(m.key)}: ${m.changes.map((c) => html`<div>${c.path}: ${val1(c.before)} \u2192 ${val1(c.after)}</div>`)}</div>`)}` : '')}</details>
      ${d.redacted ? html`<p class="mut">Account and character identifiers are hidden in this view.</p>` : ''}`
      : html`<p class="mut">The working copy is identical to the file on disk.</p>`}
    <div class="row mt12"><button class="b" data-act="doSave" id="btnDoSave">Save to game</button><button class="b sec" data-act="closeReview">Back</button><span id="savewhy" class="mut"></span></div></div>`;
}
function syncReview() {
  const m = $('#modal');
  if (!R) { m.className = 'hide'; document.body.classList.remove('noscroll'); return; }
  m.className = 'modal';
  document.body.classList.toggle('noscroll', true);
  const b = $('#btnDoSave');
  if (!b) return;
  b.disabled = offline || saveBlocked || !R.dirty || !!R.stale;
  $('#savewhy').textContent = offline ? 'Lost contact with the editor.' : saveBlocked ? 'Saving is blocked: ' + saveBlockWhy + '.' : R.stale ? 'Reload the file first.' : !R.dirty ? 'Nothing to save.' : '';
}
function drawReview(focus) {
  setHTML($('#modal'), R ? reviewBody() : html``);
  syncReview();
  if (R && focus) $('#mtitle').focus(); // keyboard users land in the pane
}
let opening = false;
async function openReview() {
  if (opening) return;
  opening = true;
  try {
    const r = await api('/api/save/review');
    R = { token: r.token, dirty: !!r.dirty, ops: r.ops || [], diff: r.diff || { tables: [] } };
  } catch (e) { toast(e.message, true); return; } finally { opening = false; }
  drawReview(true);
}
$('#btnSave').onclick = openReview;
$('#btnDiscard').onclick = async () => { if (await ask({ title: 'Discard all unsaved changes?', body: 'Every edit since the file was loaded or last saved is dropped. The file on disk is not touched.', ok: 'Discard', danger: true })) act(() => api('/api/save/discard', {}), 'Discarded'); };

// ---------- PLAYER
let P = null;
function host() { return $('#pbody') || $('#main'); }
// The Player Summary above the Player tabs, as in the console's player view: identity, level, XP, skill points, Intel, vitals,
// faction, database ids and the currency balances. Everything comes from the save.
function summaryCard(m) {
  if (!m) return html``;
  const num = (v) => (v == null ? '-' : Number(v).toLocaleString());
  const f1 = (v) => (v == null ? '-' : Number(v).toFixed(1));
  const id = m.identity || {}, ids = m.ids || {}, v = m.vitals || {}, sp = m.skillPoints, it = m.intel;
  return html`<div class="card"><h3>Player Summary</h3><div class="grid">
    ${kv('Character', m.name)}${kv('Status', m.status)}${kv('Map', m.map)}${kv('Faction', m.faction || 'Neutral')}
    ${kv('Level', m.level)}${kv('XP', num(m.xp))}${kv('Skill points', sp ? `${num(sp.unspent)} / ${num(sp.total)}` : '-')}${kv('Intel', it ? `${num(it.points)} / ${num(it.max)}` : '-')}
    ${kv('Health', v.health == null ? '-' : `${f1(v.health)} (max at least ${Math.ceil(Math.max(v.healthMinMax, v.health))})`)}${kv('Hydration', v.hydration == null ? '-' : `${f1(v.hydration)} / ${v.maxHydration}`)}${kv('Spice addiction', v.spiceAddiction == null ? '-' : `${f1(v.spiceAddiction)} / ${v.maxSpiceAddiction}`)}
    ${kv((id.platform || 'Platform') + ' ID', id.platformId)}${kv('Funcom ID', id.funcomId)}${kv('FLS ID', id.flsId)}
    ${kv('DB player', ids.actor)}${kv('Account', ids.account)}${kv('Player controller', ids.controller)}${kv('Player state', ids.playerState)}
    ${(m.currency || []).map((c) => kv(c.label, num(c.balance)))}</div></div>`;
}
async function playerView() {
  let sm = null;
  try { sm = await api('/api/player/summary'); } catch (e) { sm = null; } // the tabs still work without it
  setHTML($('#main'), html`${summaryCard(sm)}${subNav('player', PTABS)}<div id="pbody"></div>`);
  if (sub.player == 'bases') return basesView();
  if (sub.player == 'vehicles') return vehiclesView();
  return playerSection(sub.player);
}
async function playerSection(s) {
  const h = s == 'character' ? [subNav('char', [['overview', 'Overview'], ['inventory', 'Inventory'], ['reputation', 'Reputation']])] : [];
  const c = s == 'character' ? sub.char || 'overview' : s;
  if (c == 'overview') {
    P = await api('/api/player');
    const a = P.actor || {}, j = P.journey || {};
    h.push(html`<div class="card"><h3>${P.name}</h3><div class="grid">${kv('Solari in backpack', P.solari.toLocaleString())}${kv('Position', `${fix(a.x)}, ${fix(a.y)}, ${fix(a.z)}`)}${kv('Journey', `${j.done || 0} / ${j.total || 0} nodes done`)}</div><p class="mut">Map, account, platform and the database ids are in the summary above.</p></div>
    <div class="card"><h3>Solari</h3><div class="row"><input id="solari" type="number" value="10000"><button class="b" data-act="solari">Add / remove</button><span class="mut">Negative removes. Solari is an item stack in your backpack.</span></div></div>
    <div class="card"><h3>Quick rewards</h3><div class="row"><b>Give XP</b><input id="xpAmt" type="number" value="1000" min="1"><button class="b" data-act="xp">Give</button><span class="mut">Raises the level as the XP grows and adds one skill point per level gained; stops at level 200.</span></div><div class="row"><b>Give currency</b><select id="curSel"><option value="0">Solari Credit</option><option value="1">House Credit</option></select><input id="curAmt" type="number" value="100" min="1"><button class="b" data-act="currency">Give</button><span class="mut">The virtual wallets, not the Solari in your backpack (that is the Solari card above).</span></div><div class="row"><b>Give Intel</b><input id="intelAmt" type="number" value="100" min="1"><button class="b" data-act="intel">Give</button><span class="mut">Adds Intel for the research tree, never past 2,779. All of these apply when the game next loads the save.</span></div></div>`);
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
    const slots = cap && cap.max_item_count > 0 ? ` - ${counts[grp]} / ${cap.max_item_count} slots used` : '';
    const invCard = html`<div class="card"><h3>Inventory (${d.items.length})</h3>
      <div class="sub">${G.map(([k, v]) => html`<button data-invg="${k}" class="${grp == k ? 'on' : ''}">${v} (${counts[k]})</button>`)}</div>
      <div class="row"><input id="invq" placeholder="Filter by name, item ID, or template" size="34" value="${invQ}"><button class="b sec" data-act="invfilter">Filter</button><button class="b sec" data-act="invclear">Clear</button><span class="mut">${rows.length} of ${counts[grp]}${slots}</span></div>
      ${tbl([{ k: 'position_index', label: 'Slot' }, { k: 'template_id', label: 'Item', f: (r) => itemCell(r.template_id) },
        { k: 'stack_size', label: 'Stack', f: (r) => html`<input type="number" value="${r.stack_size}" min="1" data-item="${r.id}" data-field="stack_size" class="w90">` },
        { k: 'quality_level', label: 'Grade', f: (r) => html`<select data-item="${r.id}" data-field="quality">${[0, 1, 2, 3, 4, 5].map((n) => html`<option ${n == r.quality_level ? 'selected' : ''}>${n}</option>`)}</select>` },
        { k: 'durability', label: 'Durability', f: (r) => (r.durability == null ? '' : Number(r.durability).toFixed(1) + (r.max_durability ? ' / ' + Number(r.max_durability).toFixed(0) : '')) },
        { k: 'augments', label: 'Augments', f: (r) => (r.augments || []).map((a) => html`<span class="tag" title="${a.name}${a.quality == null ? '' : ' (quality ' + a.quality + ')'}">${itemName(a.name)}</span>`) }], rows,
        { actions: (r) => html`${r.aug_limit ? html`<button class="b sec sm" data-act="augEdit" data-id="${r.id}">Augments</button>` : ''}<button class="b bad sm" data-act="delItem" data-id="${r.id}" data-name="${itemName(r.template_id)}">Delete</button>` })}</div>`;
    const augCard = augItem ? html`<div class="card"><h3>Augments: ${itemName(augItem.template_id)}</h3>
      <p class="mut">Holds up to ${augItem.limit}. Pick what to apply; an augment already on the item keeps its grade, new ones get the grade below. Choose (none) in every slot to remove them all.</p>
      <div class="row">${augSlots('au', augItem, augItem.applied.map((a) => a.id))}</div>
      <div class="row">${augGradeSel('augG')}<label><input type="checkbox" id="augU" checked> Also unlock the augment slots (specialization keystones), as the console does</label>
      <button class="b" data-act="augApply" data-id="${augItem.item_id}">Apply</button><button class="b sec" data-act="augCancel">Cancel</button></div></div>` : '';
    h.push(augCard);
    h.push(html`<div class="card"><h3>Repair</h3><div class="row"><button class="b" data-act="repair">Repair all worn items and loadouts</button><span class="mut">Everything you wear and everything in your loadout goes to full durability. The backpack is not touched.</span></div></div>`);
    h.push(html`<div class="card"><h3>Give item</h3><div class="row">${itemPicker("gt", cat)}
    Qty<input id="gq" type="number" value="1" min="1">Grade<select id="gg">${[0, 1, 2, 3, 4, 5].map((n) => html`<option>${n}</option>`)}</select><button class="b" data-act="give">Give</button><button class="b sec" data-act="queueAdd">Add to queue</button><button class="b sec" data-act="refill">Refill containers</button></div>
    <div class="row" id="gaug"></div>
    ${giveQueue.length ? html`<div class="row"><b>Queue (${giveQueue.length})</b>${giveQueue.map((q, i) => html`<span class="tag">${q.quantity} x ${itemName(q.template_id)}${q.quality ? ' (grade ' + q.quality + ')' : ''}${q.augments && q.augments.length ? ' + ' + q.augments.map((a) => augName(a)).join(', ') : ''} <button class="b sec sm" data-act="queueDel" data-id="${i}" title="Remove from the queue">x</button></span>`)}<button class="b" data-act="queueGive">Give queued items</button><button class="b sec" data-act="queueClear">Clear</button></div>` : ''}
    <p class="mut">Pick an item by its in-game name. Items already in your save that the catalog does not know are listed by their template id; any valid template id can also be typed.</p></div>
    ${invCard}`);
  } else if (c == 'reputation') {
    const f = await api('/api/player/factions');
    h.push(html`<div class="card"><h3>Faction reputation</h3>${tbl([{ k: 'name', label: 'Faction' }, { k: 'reputation', label: 'Reputation', f: (r) => html`<input type="number" min="0" max="12474" value="${r.reputation}" data-faction="${r.faction_id}" class="w100">` }], f)}<p class="mut">Range 0–12474. Edit and press Tab/Enter.</p></div>`);
  } else if (s == 'specialization') {
    const sp = await api('/api/player/specs');
    const known = (r) => r.number != null;
    const keystone = (r) => (r.granted ? html`<span class="ok" title="${r.keystoneOwned} of ${r.keystoneTotal} keystones">Granted</span>` : r.keystoneOwned > 0 ? html`<span title="${r.keystoneOwned} of ${r.keystoneTotal} keystones bought">${r.keystoneOwned}/${r.keystoneTotal}</span>` : html`<span class="mut">-</span>`);
    h.push(html`<div class="card"><h3>Specialization Tracks</h3>
      <div class="row"><button class="b" data-act="spKeyAll">Grant All Keystones</button><button class="b bad" data-act="spKeyReset">Reset All Keystones</button></div>
      <p class="mut">Changes go through the review pane and show in game after the save is loaded. Max XP is ${Number(sp.maxXp).toLocaleString()}.</p>
      ${tbl([{ k: 'track', label: 'Track' }, { k: 'xp', label: 'XP', f: (r) => (known(r) ? Number(r.xp).toLocaleString() : html`<span class="mut" title="This track's number in your save is not known yet; see below">?</span>`) },
        { k: 'level', label: 'Level', f: (r) => (known(r) ? Math.floor(r.level) : html`<span class="mut">?</span>`) }, { k: 'keystoneOwned', label: 'Keystone', f: keystone },
        { k: 'add', label: 'Add XP', f: (r) => html`<input id="spx${r.track}" type="number" value="1000" class="w90" ${known(r) ? '' : 'disabled'}><button class="b sm" data-act="spAdd" data-t="${r.track}" ${known(r) ? '' : 'disabled'}>Add XP</button>` }],
        sp.rows, { actions: (r) => html`<button class="b sec sm" data-act="spMax" data-t="${r.track}" ${known(r) ? '' : 'disabled'}>Grant Max</button><button class="b bad sm" data-act="spReset" data-t="${r.track}" ${known(r) ? '' : 'disabled'}>Reset</button>` })}</div>
    <div class="card"><h3>Track numbers</h3>
      <p class="mut">The console's database names its specialization tracks. Your save stores only a number for each one and does not say which number is which, so the XP controls above need to know the number of each track. It is learned from your own save: earn a little XP in a track in game, save, reload this tab, and the new row appears below to assign to the track you used. Keystones need no numbers.</p>
      ${sp.unassigned.length ? html`${sp.unassigned.map((u) => html`<div class="row"><span>Track number <b>${u.number}</b>: ${Number(u.xp).toLocaleString()} XP, level ${Math.floor(u.level)}</span> is
        <select id="spa${u.number}">${sp.tracks.map((t) => html`<option>${t}</option>`)}</select><button class="b" data-act="spAssign" data-n="${u.number}">Assign</button></div>`)}` : html`<p class="mut">No specialization XP in your save is waiting to be assigned.</p>`}
      <div class="row"><span class="mut">Known a track's number already?</span><select id="spmt">${sp.tracks.map((t) => html`<option>${t}</option>`)}</select> number <input id="spmn" type="number" min="0" max="255" class="w70"><button class="b sec" data-act="spAssignHand">Set</button></div>
      <p class="mut">Assigned: ${sp.rows.filter(known).map((r) => `${r.track} = ${r.number}`).join(', ') || 'none yet'}.</p></div>
    <div class="card"><h3>Advanced: set a raw row</h3><div class="row"><span class="mut">Track number</span><input id="st" type="number" value="0" class="w70">XP<input id="sx" type="number" value="0">Level<input id="sl" type="number" step="0.1" value="1" class="w80"><button class="b sec" data-act="specSet">Set</button></div></div>`);
  } else if (s == 'journey') {
    JB = await api('/api/player/journey/browse');
    const tg = await api('/api/player/tags');
    h.push(html`<div class="card"><h3>Journey Browser</h3><div id="jhead"></div><div id="jout"></div></div>`);
    h.push(html`<div class="card"><h3>Player tags</h3>${tg.length ? tg.map((t) => html`<span class="tag">${t.tag} <a href="#" data-act="tagDel" data-tag="${t.tag}">×</a></span>`) : html`<span class="mut">none</span>`}<div class="row"><input id="tag" placeholder="Tag.Name" size="40"><button class="b" data-act="tagAdd">Add tag</button></div></div>`);
  } else if (s == 'skills') {
    SK = await api('/api/player/skills');
    h.push(html`<div class="card"><h3>Skill Point Controls</h3>
      <div class="row"><span>Earned in total <b>${SK.total}</b></span><span>Spent in skills <b>${SK.spent}</b></span><span>Unspent <b>${SK.unspent}</b></span></div>
      <div class="row">Unspent points<input id="skPts" type="number" min="0" max="100000" value="${SK.unspent}" class="w90"><button class="b" data-act="skPoints">Set unspent points</button></div>
      <p class="mut">The save does not keep spent plus unspent equal to the total (some skills are granted free), so these three numbers are shown as stored and not worked out from each other.</p></div>
    <div class="card"><h3>Skill Browser</h3><div id="skhead"></div><div id="skout"></div></div>`);
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
    h.push(html`<div class="card"><h3>Building Sets</h3><div class="row"><button class="b" data-act="bsAll">Unlock all building sets</button><span class="mut">Teaches every set below except the experimental ones, without putting the items in your backpack.</span></div><div id="cthead"></div><div id="ctout"></div></div>
      <div class="card"><h3>New buildable pieces (${CT.newPieces.length})</h3>${CT.newPieces.map((x) => html`<span class="tag">${x.name}</span>`)}</div>`);
  } else if (s == 'customizations') {
    CT = await api('/api/player/customizations');
    CT.kind = 'cu';
    ctGroup = '';
    h.push(html`<div class="card"><h3>Customizations</h3><div class="row"><button class="b" data-act="ctGrantSet" data-id="all" data-name="all five sets">Grant all sets</button><span class="mut">Puts one of each cosmetic of the five sets in your backpack, all or nothing (it needs room for them).</span></div><div id="cthead"></div><div id="ctout"></div></div>`);
  } else if (s == 'admin') {
    P = await api('/api/player');
    const a = P.actor || {};
    const fa = await api('/api/player/faction');
    const na = (what) => html`<button class="b" disabled title="Not in tabr-tau yet">${what}</button>`;
    h.push(html`<div class="card"><h3>Player Admin Actions</h3><p class="mut">The console's Admin tab. Actions the save file cannot do, or that only make sense on a server (kick, ban, login queue, recovering a deleted character), are left out; the others that tabr-tau does not have yet are shown disabled.</p></div>
    <div class="card"><h3>Faction Assignment</h3><div class="row"><b>Current</b><span>${fa.current ? fa.current.name : 'none recorded'}</span>
      <select id="facSel">${fa.options.map((o) => html`<option value="${o.id}" ${fa.current && fa.current.id == o.id ? 'selected' : ''}>${o.name}</option>`)}</select><button class="b" data-act="faction">Change Faction</button>
      <span class="mut">Changes which faction the character belongs to. Reputation is under Character &gt; Reputation.</span></div></div>
    <div class="card"><h3>Repair</h3>
      <div class="row"><b>Repair Faction</b>${na('Repair Faction')}<span class="mut">Not in tabr-tau yet.</span></div>
      <div class="row"><b>Repair Landsraad Quests</b>${na('Repair Quests')}<span class="mut">Not in tabr-tau yet.</span></div>
      <div class="row"><b>Repair Gear</b><button class="b" data-act="repair">Repair Gear</button><span class="mut">Worn items and loadouts, to full durability.</span></div>
      <div class="row"><b>Repair Vehicle Durability</b><button class="b" data-act="repairV">Repair Vehicles</button><label>Repair Below <input id="rvPct" type="number" min="1" max="100" value="50" class="w70"> %</label><span class="mut">Raises the modules of your vehicles whose durability is below this share of their current maximum back to that maximum. Wear that lowered the maximum itself is not undone, and modules with no recorded maximum are skipped.</span></div></div>
    <div class="card"><h3>Danger Zone</h3><div class="row">${na('Wipe Inventory')}${na('Reset Progression')}<span class="mut">Not in tabr-tau yet.</span></div></div>
    <div class="card"><h3>Movement / Vehicles</h3><div class="row"><b>Teleport To</b>X<input id="tx" type="number" value="${Math.round(a.x)}">Y<input id="ty" type="number" value="${Math.round(a.y)}">Z<input id="tz" type="number" value="${Math.round(a.z)}"><button class="b" data-act="teleport">Teleport</button></div>
    <p class="mut">Moves the character in the save (applied on next load). Stay above the terrain (Z) or you may fall through.</p>
    <div class="row"><b>Spawn Vehicle</b>${na('Spawn')}<span class="mut">Not in tabr-tau yet.</span></div></div>`);
    const ve = await api('/api/vendors');
    h.push(html`<div class="card"><h3>Vendor purchase limits</h3><p class="mut">tabr-tau's own tool (the console has none). Vendors limit how much you can buy per restock cycle. Resetting clears those counters.</p><div class="row"><button class="b" data-act="vreset" data-v="">Reset all vendors</button></div>
      ${tbl([{ k: 'vendor_id', label: 'Vendor' }, { k: 'template_id', label: 'Item', f: (r) => itemCell(r.template_id) }, { k: 'amount_bought', label: 'Bought' }], ve.stock, { actions: (r) => html`<button class="b sec sm" data-act="vreset" data-v="${r.vendor_id}">Reset vendor</button>` })}
      <h3 class="mt14">Restock cycles</h3>${tbl([{ k: 'vendor_id' }, { k: 'last_interacted_timestamp', label: 'Last interaction', f: (r) => new Date(r.last_interacted_timestamp * 1000).toLocaleString() }], ve.cycles)}</div>`);
  } else {
    h.push(html`<div class="card"><h3>${SOON[s] || s}</h3><p class="mut">Not in tabr-tau yet. The console has this tab; it is planned (#96).</p></div>`);
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
  setHTML($('#jhead'), html`<div class="sub">${JGROUPS.map(([k, v]) => html`<button data-jgrp="${k}" class="${jgrp == k ? 'on' : ''}">${v} (${(JB[k] || []).length})</button>`)}</div>
    <div class="row"><input id="jq" placeholder="Filter by name or node id" size="34" value="${window.jq || ''}"><button class="b sec" data-act="jfilter">Filter</button><button class="b sec" data-act="jclear">Clear</button>
    <span class="mut">${rows.length} of ${all.length}${rows.length > JMAX ? ` (showing the first ${JMAX}, narrow the filter)` : ''}. ${jgrp == 'contract' ? 'Contracts are shown read-only; a contract is complete when all its tags are on the character.' : 'Completing a node completes its children; resetting clears it and its children.'}</span></div>`);
  const stat = (r) => (r.status == 'Complete' ? html`<span class="ok">Complete</span>` : r.status == 'Revealed' || r.status == 'Started' ? html`<span class="warn">${r.status}</span>` : html`<span class="mut">${r.status}</span>`);
  const act = (r) => {
    if (!r.actionable) return '';
    const kind = jgrp == 'tutorial' ? 'tut' : 'jset';
    return r.complete ? html`<button class="b sec sm" data-act="${kind}" data-id="${r.id}" data-c="0">Reset</button>` : html`<button class="b sm" data-act="${kind}" data-id="${r.id}" data-c="1">Complete</button>`;
  };
  setHTML($('#jout'), tbl([{ k: 'name', label: 'Name', f: (r) => html`<span class="ind${Math.min(r.depth || 0, 8)}" title="${r.id}">${r.name}</span>${r.pendingReward ? html` <span class="tag">reward pending</span>` : ''}` },
    { k: 'status', label: 'Status', f: stat }, { k: 'tags', label: 'Tags', f: (r) => (r.tags ? r.tags : '') }], rows.slice(0, JMAX), { actions: act }));
}

// The Skill Browser, as the console's: a school at a time, each skill with a rank bar and a rank control. A change is one edit in the
// review pane. By default it sets the skill only, as the console's control does; "Pay for it" also takes the points from, or gives them
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
    <span class="mut">${rows.length} skills; ${rows.filter((m) => m.rank > 0).length} learned.</span></div>`);
  const bar = (m) => (m.known ? html`<span class="rankbar" title="rank ${m.rank} of ${m.max}">${'●'.repeat(m.rank)}${'○'.repeat(Math.max(0, m.max - m.rank))}</span>` : html`<span class="mut" title="not in the catalog: ${m.spent} points spent">?</span>`);
  setHTML($('#skout'), tbl([{ k: 'name', label: 'Skill', f: (m) => html`<span title="${m.id}">${m.name}</span>` }, { k: 'kind', label: 'Type' }, { k: 'rank', label: 'Rank', f: bar },
    { k: 'spent', label: 'Points', f: (m) => m.spent },
    { k: 'max', label: 'Set rank', f: (m) => (m.known ? html`<select data-skill="${m.id}">${Array.from({ length: m.max + 1 }, (_, i) => html`<option ${i == m.rank ? 'selected' : ''}>${i}</option>`)}</select>` : html`<span class="mut">catalog does not know its ranks</span>`) }], rows));
}

// Research and crafting recipes, as the console's Research and Crafting tabs: a category, a filter, and an Unlock on each entry that is not
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
  setHTML($('#rshead'), html`<div class="sub"><button data-rscat="" class="${rsCat == '' ? 'on' : ''}">All (${RS.length})</button>${RCATS.map((c) => html`<button data-rscat="${c}" class="${rsCat == c ? 'on' : ''}">${c} (${RS.filter((r) => r.category == c).length})</button>`)}</div>
    <div class="row">${rsCat ? html`<select id="rsGrp"><option value="">All product groups</option>${groups.map((g) => html`<option ${g == rsGroup ? 'selected' : ''}>${g}</option>`)}</select>` : ''}
    <input id="rsq" placeholder="Filter by name, key, type or group" size="34" value="${rsQ}"><button class="b sec" data-act="rsfilter">Filter</button><button class="b sec" data-act="rsclear">Clear</button>
    <span class="mut">${rows.length} of ${inCat.length}${rows.length > LISTMAX ? `, showing the first ${LISTMAX}` : ''}; ${done} of ${RS.length} unlocked. Unlocking adds the linked recipe or building set; group markers cannot be unlocked as one entry.</span></div>`);
  const state = (r) => (r.unlocked ? html`<span class="ok">Unlocked</span>` : r.purchased ? html`<span class="warn">Purchased, ${r.unlockKind} missing</span>` : html`<span class="mut">Not purchased</span>`);
  const act = (r) => (!r.actionable ? html`<button class="b sec sm" disabled title="A group marker; unlock its individual entries">Group</button>` : r.unlocked ? '' :
    html`<button class="b sm" data-act="rsUnlock" data-key="${r.itemKey}">${r.needsRepair ? 'Repair unlock' : 'Unlock'}</button>`);
  setHTML($('#rsout'), tbl([{ k: 'name', label: 'Research', f: (r) => html`<span title="${r.itemKey}">${r.name}</span>` }, { k: 'itemKey', label: 'Key', cls: 'mut' }, { k: 'type', label: 'Type' },
    { k: 'productGroup', label: 'Product group' }, { k: 'state', label: 'State', f: state }], rows.slice(0, LISTMAX), { actions: act }));
}
function craftingList() {
  if (!CR || !$('#crout')) return;
  const inCat = CR.filter((r) => !crCat || r.category == crCat);
  const words = crQ.toLowerCase().split(/\s+/).filter(Boolean);
  const rows = inCat.filter((r) => words.every((w) => [r.name, r.recipeId, r.category, r.source].join(' ').toLowerCase().includes(w)));
  const known = CR.filter((r) => r.known).length;
  setHTML($('#crhead'), html`<div class="sub"><button data-crcat="" class="${crCat == '' ? 'on' : ''}">All (${CR.length})</button>${RCATS.filter((c) => c != 'Augmentations' && c != 'Uniques').map((c) => html`<button data-crcat="${c}" class="${crCat == c ? 'on' : ''}">${c} (${CR.filter((r) => r.category == c).length})</button>`)}</div>
    <div class="row"><input id="crq" placeholder="Filter by name, recipe id or source" size="34" value="${crQ}"><button class="b sec" data-act="crfilter">Filter</button><button class="b sec" data-act="crclear">Clear</button>
    <span class="mut">${rows.length} of ${inCat.length}${rows.length > LISTMAX ? `, showing the first ${LISTMAX}` : ''}; ${known} known. The recipes listed are the ones you know plus the ones your research tree can still give.</span></div>`);
  setHTML($('#crout'), tbl([{ k: 'name', label: 'Recipe', f: (r) => html`<span title="${r.recipeId}">${r.name}</span>${r.limited ? html` <span class="tag">limited use (${r.uses})</span>` : ''}` }, { k: 'recipeId', label: 'Recipe id', cls: 'mut' },
    { k: 'category', label: 'Category' }, { k: 'source', label: 'Source' }, { k: 'known', label: 'State', f: (r) => (r.known ? html`<span class="ok">Known</span>` : html`<span class="mut">Not known</span>`) }], rows.slice(0, LISTMAX),
    { actions: (r) => (r.known ? '' : html`<button class="b sm" data-act="crUnlock" data-id="${r.recipeId}">Unlock</button>`) }));
}

// Building Sets and Customizations, as the console's tabs: the catalog's items with what the save says you have, and a Give that puts the
// item in the backpack (the game teaches the set or unlocks the cosmetic when it is used). The save keeps no list of owned customizations
// that tabr-tau can read, so those show only whether the item is in an inventory.
let CT = null, ctFilter = 'all', ctQ = '', ctGroup = '', ctExp = true;
function catalogList() {
  if (!CT || !$('#ctout')) return;
  const bs = CT.kind == 'bs';
  const have = (r) => r.learned || r.inInventory;
  const visible = CT.rows.filter((r) => bs ? (ctExp || !r.experimental) : true);
  const groupNames = bs ? [...new Set(visible.map((r) => r.group))].sort() : [];
  const rows = visible.filter((r) => (ctFilter == 'all' || (ctFilter == 'have' ? have(r) : !have(r))) && (!ctGroup || (bs ? r.group == ctGroup : r.groupId == ctGroup))
    && ctQ.split(/\s+/).filter(Boolean).every((w) => (r.name + ' ' + r.id + ' ' + r.group).toLowerCase().includes(w.toLowerCase())));
  const nHave = visible.filter(have).length;
  const cards = bs ? '' : html`<div class="row">${CT.groups.map((g) => html`<div class="card setcard ${ctGroup == g.id ? 'on' : ''}"><button class="b sec sm" data-ctg="${g.id}">${g.name}</button>
      <div class="mut">${g.count} cosmetics${g.requirement ? ' · ' + g.requirement : ''}</div><button class="b sm" data-act="ctGrantSet" data-id="${g.id}" data-name="${g.name}">Grant set</button></div>`)}</div>`;
  setHTML($('#cthead'), html`${cards}<div class="sub">${[['all', 'All', visible.length], ['have', bs ? 'Learned or in inventory' : 'In inventory', nHave], ['missing', 'Not owned', visible.length - nHave]].map(([k, v, n]) => html`<button data-ctf="${k}" class="${ctFilter == k ? 'on' : ''}">${v} (${n})</button>`)}</div>
    <div class="row"><input id="ctq" placeholder="Filter by name, item id or group" size="34" value="${ctQ}"><button class="b sec" data-act="ctfilter">Filter</button><button class="b sec" data-act="ctclear">Clear</button>
    ${bs ? html`<select id="ctgrp"><option value="">All groups</option>${groupNames.map((g) => html`<option ${g == ctGroup ? 'selected' : ''}>${g}</option>`)}</select><label><input type="checkbox" id="ctexp" ${ctExp ? 'checked' : ''}> Show experimental</label>` : (ctGroup ? html`<button class="b sec" data-ctg="">All sets</button>` : '')}
    <span class="mut">${rows.length} of ${visible.length}${rows.length > LISTMAX ? `, showing the first ${LISTMAX}` : ''}. Give adds the item to your backpack.${bs ? '' : ' The save does not list owned customizations, so only items in an inventory are marked. Granting does not grant a DLC or account entitlement.'}</span></div>`);
  setHTML($('#ctout'), tbl([{ k: 'name', label: bs ? 'Building set' : 'Customization', f: (r) => html`<span title="${r.id}">${r.name}</span>${r.experimental ? html` <span class="tag">experimental</span>` : ''}` }, { k: 'id', label: 'Item id', cls: 'mut' },
    { k: 'group', label: bs ? 'Group' : 'Set' }, { k: 'requiredDlc', label: 'Requires', f: (r) => r.requiredDlc || (r.entitlement ? 'Account entitlement' : '') },
    { k: 'learned', label: 'State', f: (r) => (r.learned ? html`<span class="ok">Learned</span>` : r.inInventory ? html`<span class="warn">In inventory</span>` : html`<span class="mut">Not owned</span>`) }], rows.slice(0, LISTMAX),
    { actions: (r) => (r.inCatalog ? html`<button class="b sm" data-act="ctGive" data-id="${r.id}">Give</button>` : '') }));
}

// ---------- BASES
// ---------- BASES (the console's Bases panel inside Player > Bases: the base list, and for one base Power, Water, Inventory and Land Claim Editor)
// The automatic refill switch, shown on both the Power and the Water tab because it refills both: the water devices and the generators.
function autoRefillCard(prefs) {
  return html`<div class="card"><h3>Automatic refill of water and power</h3><div class="row"><span>When the editor opens: <b>${prefs.autoRefillOnOpen ? 'on' : 'off'}</b></span><button class="b sec" data-act="autoref" data-on="${prefs.autoRefillOnOpen ? '1' : ''}">${prefs.autoRefillOnOpen ? 'Turn off' : 'Turn on'}</button></div>${prefs.last ? html`<p class="mut">Last automatic save: ${prefs.last}</p>` : ''}
    <p class="mut">When on, opening the editor refills base water and generators and <b>saves straight away</b>, without the review step, so it applies the next time the game loads. It is skipped while a single-player session is running. The previous file is backed up first. Remembered between runs. The same switch is on the Power and Water tabs.</p></div>`;
}
// The console's Land Claim Editor: the claim as a grid of cells turned by the base's own yaw (north at the top), with the totem's cell,
// the cells it has, and dotted "available" cells next to it. Click an available cell to add it, a chosen one to take it back (what is
// chosen must stay connected edge to edge), pick a vertical level, then Apply. Existing cells are never removed here.
let LC = null; // { claim, sel: Set of "x,y", level }
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
function claimEditorDraw() {
  const host = $('#lced');
  if (!host || !LC) return;
  const c = LC.claim, yaw = Number(c.yaw) || 0;
  const existing = new Set(['0,0', ...(c.segments || []).map((s) => lcKey(s.x, s.y))]);
  const pieces = new Map((c.grid || []).map((g) => [lcKey(g.x, g.y), g.n]));
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
  const maxLevel = Math.max(c.level, c.maxLevel);
  const dirty = LC.sel.size > 0 || LC.level !== c.level;
  const cells = plotted.map(([x, y]) => {
    const k = lcKey(x, y), origin = k === '0,0', sel = LC.sel.has(k), have = existing.has(k) && !origin, avail = frontier.has(k);
    const kind = origin ? 'lc-o' : sel ? 'lc-s' : have ? 'lc-x' : 'lc-a';
    const label = origin ? 'T' : sel ? '+' : have && pieces.get(k) ? pieces.get(k) : '';
    return html`<rect x="${x - 0.44}" y="${y - 0.44}" width="0.88" height="0.88" rx="0.08" class="lc-cell ${kind}" ${avail || sel ? html`data-cell="${k}"` : ''}></rect>${label !== '' ? html`<text x="${x}" y="${y + 0.09}" transform="rotate(${-yaw} ${x} ${y})" class="lc-label">${label}</text>` : ''}`;
  });
  setHTML(host, html`<div class="card"><h3>Land Claim Editor</h3>
    <div class="grid">${kv('Totem ID', c.totem_id)}${kv('Original yaw', Math.round(yaw * 10) / 10 + '°')}${kv('Horizontal segments', c.cells - 1)}${kv('Vertical level', c.level + ' / ' + c.maxLevel)}</div>
    <div class="lc-work"><div class="lc-gridcard"><b>Horizontal claim</b><p class="mut">Click a dotted cell to add it. New cells must stay connected edge to edge. A number is how many building pieces a cell holds.</p>
      <div class="lc-north">&#9650; North</div>
      <svg class="lc-svg" viewBox="${minX} ${minY} ${width} ${height}"><g transform="rotate(${yaw} 0 0)">${cells}</g></svg>
      <p class="mut"><span class="lc-key lc-o"></span>Totem <span class="lc-key lc-x"></span>Existing <span class="lc-key lc-s"></span>New <span class="lc-key lc-a"></span>Available</p>
      <p class="mut">World-oriented view: north stays at the top and the grid is turned by the base's original yaw; the saved cell coordinates are not changed.</p></div>
    <div class="lc-side"><b>Expansion</b>
      <label>Vertical level <select id="lcLvl">${Array.from({ length: maxLevel - c.level + 1 }, (_, i) => c.level + i).map((n) => html`<option ${n === LC.level ? 'selected' : ''}>${n}</option>`)}</select></label>
      <p class="mut">Vertical expansion grows equally up and down. The game caps it at level ${c.maxLevel}.</p>
      <div class="kv"><b>Pending horizontal</b><span>${LC.sel.size}</span></div><div class="kv"><b>Pending vertical</b><span>${LC.level === c.level ? 'No change' : c.level + ' → ' + LC.level}</span></div>
      <div class="row"><button class="b" data-act="lcApply" ${dirty ? '' : 'disabled'}>Apply changes</button>${LC.sel.size ? html`<button class="b sec" data-act="lcClear">Clear selection</button>` : ''}</div>
      <p class="mut">The edit goes through the review pane and the usual backup. Applied when the game next loads the save.</p></div></div>
    <p class="mut">This editor bypasses the game's normal staking checks. Claiming a protected area does not guarantee that the game lets you build there.</p></div>`);
}

let bsTab = 'power', bsOpen = null, bsGroup = 'storage';
async function basesView() {
  const list = await api('/api/bases/list');
  const id = list.some((x) => x.base_id == bsOpen) ? bsOpen : (list[0] ? list[0].base_id : null);
  const base = list.find((x) => x.base_id == id);
  const h = [html`<div class="card"><h3>Bases (${list.length})</h3>${tbl([{ k: 'base_id', label: 'ID' }, { k: 'name', label: 'Base Name' }, { k: 'base_type', label: 'Base Type' }, { k: 'owner', label: 'Owner' }, { k: 'map', label: 'Map' },
    { k: 'generators', label: 'Generators' }, { k: 'pieces', label: 'Building Pieces' }, { k: 'placeables', label: 'Placeables' }, { k: 'x', label: 'Coordinates', f: (r) => `${fix(r.x)}, ${fix(r.y)}, ${fix(r.z)}` }], list,
    { actions: (r) => (r.base_id == id ? html`<span class="tag">Open below</span>` : html`<button class="b sec sm" data-bsopen="${r.base_id}">Open</button>`) })}</div>`];
  if (!base) {
    h.push(html`<p class="mut">No base in this save yet.</p>`);
  } else {
    const tabs = [['power', 'Power'], ['water', 'Water'], ['inventory', 'Inventory'], ['claim', 'Land Claim Editor'], ['structures', 'Structures']];
    h.push(html`<div class="card"><h3>${base.name} (#${base.base_id})</h3><div class="sub">${tabs.map(([k, v]) => html`<button data-bstab="${k}" class="${bsTab == k ? 'on' : ''}">${v}</button>`)}</div></div>`);
    if (bsTab == 'power') {
      const pw = await api('/api/bases/power?base=' + id);
      const prefs = await api('/api/settings');
      h.push(html`<div class="card"><h3>Power</h3>${pw.types.length ? tbl([{ k: 'name', label: 'Type' }, { k: 'devices', label: 'Devices' },
        { k: 'queued', label: 'Holds', f: (r) => Number(r.queued).toLocaleString() + (r.fuel ? ' ' + itemName(r.fuel) : '') }, { k: 'capacity', label: 'Full', f: (r) => (r.capacity ? Number(r.capacity).toLocaleString() : '-') }, { k: 'empty', label: 'Empty' }], pw.types) : html`<p class="mut">No generators or windtraps built at this base.</p>`}
        <div class="row"><button class="b" data-act="refillGen">Refill generators</button><span class="mut">Tops fuel and spice generators up to a full stack. Wind turbines (lubricant) and windtrap filters are shown but not refilled.</span></div></div>
    ${autoRefillCard(prefs)}`);
    } else if (bsTab == 'water') {
      const w = await api('/api/bases/water?base=' + id);
      h.push(html`<div class="card"><h3>Water</h3>${w.types.length ? tbl([{ k: 'name', label: 'Type' }, { k: 'devices', label: 'Containers' }, { k: 'stored', label: 'Water stored', f: (r) => Number(r.stored).toLocaleString() },
        { k: 'capacity', label: 'Capacity', f: (r) => (r.capacity ? Number(r.capacity).toLocaleString() : '-') }, { k: 'fillPercent', label: 'Fill', f: (r) => (r.fillPercent == null ? '-' : Math.round(r.fillPercent) + '%') }], w.types) : html`<p class="mut">No water storage at this base.</p>`}
        <div class="row"><button class="b" data-act="refillWater">Refill base water</button><span class="mut">Fills every water device to its capacity.</span></div></div>
    ${autoRefillCard(await api('/api/settings'))}`);
    } else if (bsTab == 'inventory') {
      const inv = await api('/api/bases/inventory?base=' + id);
      const grp = inv.groups.some((g) => g.id == bsGroup && g.count) ? bsGroup : (inv.groups.find((g) => g.count) || inv.groups[0]).id;
      const rows = inv.containers.filter((c) => c.group == grp);
      h.push(html`<div class="card"><h3>Inventory</h3><div class="sub">${inv.groups.map((g) => html`<button data-bsgrp="${g.id}" class="${grp == g.id ? 'on' : ''}">${g.label} (${g.count})</button>`)}</div>
        ${tbl([{ k: 'name', label: 'Container' }, { k: 'type', label: 'Type' }, { k: 'items', label: 'Items' }, { k: 'max_item_count', label: 'Slots', f: (r) => `${r.items} / ${r.max_item_count}` }], rows,
        { actions: (r) => html`<button class="b sec sm" data-act="openInv" data-id="${r.inventory_id}">Open</button>` })}
        <p class="mut">Slots shows items held out of the container's capacity. Only containers that belong to this base are listed.</p></div><div id="inv"></div>`);
    } else if (bsTab == 'structures') {
      // tabr-tau's own base tools (the console's base view has no equivalent): structure health, repair all, sand, placeables, piece types.
      const b = await api('/api/bases');
      const p = b.pieces || {};
      h.push(html`<div class="card"><h3>Structure health</h3><div class="grid">${kv('Building pieces', p.n)}${kv('Lowest health', fix(p.minh))}${kv('Average health', fix(p.avgh))}${kv('Total sand buildup', fix(p.sand))}</div>
        <div class="row"><button class="b" data-act="repairB">Repair all to max</button><button class="b sec" data-act="sand">Clear sand buildup</button><span class="mut">Repair sets each piece to the highest health seen for its type.</span></div></div>`);
      h.push(html`<div class="card"><h3>Placeables (${b.placeables.length})</h3>${tbl([{ k: 'id' }, { k: 'building_type', label: 'Type' }, { k: 'health', f: (r) => html`<input type="number" value="${r.health}" data-hp="placeable" data-id="${r.id}" class="w90">` }], b.placeables)}</div>
        <div class="card"><h3>Building piece types</h3>${tbl([{ k: 'building_type', label: 'Type' }, { k: 'n', label: 'Count' }, { k: 'minh', label: 'Min health' }, { k: 'maxh', label: 'Max health' }, { k: 'avgh', label: 'Avg' }], b.types)}</div>`);
    } else {
      const claims = (await api('/api/bases/claim')).filter((c) => c.totem_id == id);
      LC = claims.length ? { claim: claims[0], sel: new Set(), level: claims[0].level } : null;
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
    { actions: (r) => html`<button class="b ${open && open.id == r.id ? '' : 'sec'} sm" data-vhopen="${r.id}">Details</button><button class="b sm" data-act="repairOne" data-id="${r.id}">Repair</button><button class="b sm" data-act="bring" data-id="${r.id}">Bring to me</button>` })}
    <div class="row"><button class="b" data-act="repairV">Repair all my vehicles</button><span class="mut">Raises every module of your vehicles to its current maximum. For a "repair below %" threshold use Admin.</span></div></div>`];
  if (open) {
    const cargo = open.cargo;
    h.push(html`<div class="card"><h3>${open.type || open.name}: ${(open.components || []).length} components</h3>${tbl([{ k: 'template_id', label: 'Component', f: (r) => html`<span title="${r.template_id}">${compName(r.template_id)}</span>` },
      { k: 'percent', label: 'Condition', f: (r) => pctCell(r.percent) }, { k: 'current', label: 'Durability', f: (r) => (r.current == null ? '-' : Number(r.current).toFixed(0) + (r.max ? ' / ' + Number(r.max).toFixed(0) : '')) }], open.components || [])}
      <p class="mut">Condition is each module's durability against its current maximum; a module with no stored maximum shows only its value. The fuel tank size is not stored in the save, so fuel is the current amount.</p></div>`);
    h.push(html`<div class="card"><h3>Cargo hold</h3>${cargo ? html`<p class="mut">${cargo.items.length} / ${cargo.slots == null ? '?' : cargo.slots} slots used</p>${cargo.items.length ? tbl([{ k: 'position_index', label: 'Slot' },
      { k: 'template_id', label: 'Item', f: (r) => itemCell(r.template_id) }, { k: 'stack_size', label: 'Quantity' }, { k: 'quality_level', label: 'Grade' }], cargo.items) : html`<p class="mut">The cargo hold is empty.</p>`}` : html`<p class="mut">This vehicle has no cargo hold.</p>`}</div>`);
  }
  h.push(html`<div class="card"><h3>Recovered / stored vehicles (${v.recovered.length})</h3>${tbl([{ k: 'vehicle_id', label: 'Id' }, { k: 'vehicle_name', label: 'Name' }, { k: 'chassis_durability', label: 'Chassis durability' }, { k: 'time_stored' }, { k: 'reason' }], v.recovered,
    { actions: (r) => html`<button class="b sec sm" data-act="dur" data-id="${r.vehicle_id}" data-v="${r.chassis_durability}">Set durability</button>` })}</div>`);
  if (v.hidden) h.push(html`<p class="mut">${v.hidden} other vehicle${v.hidden == 1 ? '' : 's'} in the world (not yours) ${v.hidden == 1 ? 'is' : 'are'} not shown.</p>`);
  if (!v.vehicles.length && !v.recovered.length) h.push(html`<p class="mut">None of your vehicles are in this save yet. Vehicle fuel is stored in an opaque binary blob and is not editable.</p>`);
  setHTML(host(), html`${h}`);
}

// ---------- LANDSRAAD
async function landsraadView() {
  const l = await api('/api/landsraad');
  const t = l.term || {};
  const fname = (id) => (l.factions.find((f) => f.id == id) || {}).name || id || '—';
  const facOpts = (sel) => l.factions.map((f) => html`<option value="${f.id}" ${f.id == sel ? 'selected' : ''}>${f.name}</option>`);
  const dec = l.decrees;
  setHTML($('#main'), html`<div class="card"><h3>Current term ${t.term_id}</h3><div class="grid">${kv('Start', t.start_time)}${kv('End', t.end_time)}${kv('Reigning faction', fname(t.reigning_faction_id))}${kv('Active decree', (dec.find((d) => d.id == t.active_decree_id) || {}).decree_name || '—')}${kv('Elected decree', (dec.find((d) => d.id == t.elected_decree_id) || {}).decree_name || '—')}</div>
  <div class="row">Active decree<select id="ad"><option value="">none</option>${dec.map((d) => html`<option value="${d.id}" ${d.id == t.active_decree_id ? 'selected' : ''}>${d.decree_name}</option>`)}</select>
  Reigning<select id="rf"><option value="">none</option>${facOpts(t.reigning_faction_id)}</select><button class="b" data-act="term">Apply</button></div></div>
  <div class="card"><h3>Decree pool</h3>${tbl([{ k: 'id' }, { k: 'decree_name', label: 'Decree' }, { k: 'disabled', label: 'State', f: (r) => (r.disabled ? html`<span class="mut">disabled</span>` : html`<span class="ok">enabled</span>`) }], dec, { actions: (r) => html`<button class="b sec sm" data-act="decree" data-id="${r.id}" data-d="${r.disabled ? 0 : 1}">${r.disabled ? 'Enable' : 'Disable'}</button>` })}</div>
  <div class="card"><h3>Task board (${l.tasks.length})</h3><div class="row">Faction<select id="lf">${facOpts()}</select><span class="mut">used by the buttons below</span></div>${tbl([{ k: 'board_index', label: '#' }, { k: 'house_name', label: 'House', f: (r) => String(r.house_name || '').replace('DA_House', '') }, { k: 'goal_amount', label: 'Goal' },
    { k: 'contributions', label: 'Progress', f: (r) => ((r.contributions || []).length ? (r.contributions || []).map((c) => html`<span class="tag">${fname(c.faction_id)}: ${fix(c.amount)}</span>`) : html`<span class="mut">0</span>`) },
    { k: 'completed', label: 'Status', f: (r) => (r.completed ? html`<span class="ok">done (${fname(r.winning_faction_id)})</span>` : 'open') },
    { k: 'rewards', label: 'Rewards', f: (r) => html`<details><summary>${(r.rewards || []).length}</summary>${(r.rewards || []).map((x) => html`<div class="mono">${x.threshold}: ${x.amount}× ${x.template_id}</div>`)}</details>` }], l.tasks,
    { actions: (r) => html`<button class="b sm" data-act="tfill" data-id="${r.id}" data-goal="${r.goal_amount}">Fill goal</button> <button class="b sm" data-act="tdone" data-id="${r.id}" data-r="${r.completed ? 1 : 0}">${r.completed ? 'Reopen' : 'Complete'}</button>` })}</div>`);
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
    <div class="card"><details><summary>Backups (${bk.length})</summary>${tbl([{ k: 'name' }, { k: 'size' }], bk, { actions: (r) => html`<button class="b sec sm" data-act="cfgrestore" data-b="${r.name}">Restore</button>` })}</details></div>`);
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
    h.push(html`<div class="row"><select id="dbsel">${t.map((x) => html`<option value="${x.name}" ${x.name == dbTable ? 'selected' : ''}>${x.name} (${x.rows})</option>`)}</select><input id="dbq" placeholder="search" value="${dbQ}"><button class="b sec" data-act="dbgo">Search</button>
      <a href="/api/db/export?name=${encodeURIComponent(dbTable)}&amp;format=csv" data-export="csv">CSV</a><a href="/api/db/export?name=${encodeURIComponent(dbTable)}&amp;format=json" data-export="json">JSON</a></div><div id="dbout"></div>`);
    setHTML($('#main'), html`${h}`);
    const d = await api(`/api/db/table?name=${encodeURIComponent(dbTable)}&q=${encodeURIComponent(dbQ)}&offset=${dbOff}&limit=100`);
    const cols = d.columns.slice(1);
    const locked = new Map(Object.entries(d.locked || {})); // a column name from the save is only ever a Map key
    setHTML($('#dbout'), html`<p class="mut">${d.total} rows — showing ${dbOff + 1}–${Math.min(dbOff + 100, d.total)}${d.tableLocked ? html`. Read-only: ${d.tableLocked}.` : locked.size ? html`. Plain text columns are read-only (primary keys, links to other tables and account identity); hover for the reason.` : ''}</p><div class="scroll"><table><thead><tr>${cols.map((c) => html`<th>${c}</th>`)}</tr></thead><tbody>${d.rows.map((r) => html`<tr>${r.slice(1).map((v, i) => {
      const b = typeof v === 'string' && v.startsWith('<blob');
      const why = d.tableLocked || locked.get(cols[i]);
      return html`<td>${b ? html`<span class="mut mono">${v}</span>` : why ? html`<span class="mono" title="${why}">${v}</span>` : html`<input class="dbc" value="${v}" data-rowid="${r[0]}" data-col="${cols[i]}" data-orig="${v}" size="${Math.max(6, Math.min(40, String(v ?? '').length + 2))}">`}</td>`;
    })}</tr>`)}</tbody></table></div>
      <div class="row"><button class="b sec" data-act="dbprev" ${dbOff <= 0 ? 'disabled' : ''}>Previous</button><button class="b sec" data-act="dbnext" ${dbOff + 100 >= d.total ? 'disabled' : ''}>Next</button></div>`);
    return;
  } else if (s == 'sql') {
    h.push(html`<div class="card"><h3>SQL</h3><textarea id="sql" rows="5" placeholder="select * from items limit 20">${window.lastSql || 'select name from sqlite_master'}</textarea><div class="row"><button class="b" data-act="runsql">Run (read-only)</button><button class="b bad" data-act="execsql">Run as write</button><span class="mut">Write mode runs one or more statements atomically on your working copy. Nothing reaches game.db until you press <b>Save to game</b>, which backs up first.</span></div><div id="sqlout"></div></div>`);
  } else {
    const b = await api('/api/save/backups');
    h.push(html`<div class="card"><h3>Save backups</h3><p class="mut">A backup of game.db is made every time you save from here. Restore requires the game to be closed.</p>${tbl([{ k: 'name' }, { k: 'size' }, { k: 'modified' }], b, { actions: (r) => html`<button class="b sec sm" data-act="restore" data-n="${r.name}">Restore</button>` })}</div>`);
  }
  setHTML($('#main'), html`${h}`);
}

// ---------- render + events
async function render() {
  drawNav();
  try {
    await ({ player: playerView, livemap: liveMapView, landsraad: landsraadView, config: configView, db: dbView })[tab]();
  } catch (e) {
    setHTML($('#main'), html`<div class="card bad">${e.message}</div>`);
  }
}

const D = (e) => e.target.closest('[data-act]');
const A = {
  closeReview: () => { R = null; drawReview(); },
  askOk: () => { if (askState && askState.typed && $('#atype').value.trim().toLowerCase() !== askState.typed.toLowerCase()) return; endAsk(true); }, // the typed word is checked here too, not only by the disabled button
  askNo: () => endAsk(false),
  reload: async () => (await ask({ title: 'Reload from disk?', body: 'Your edits are dropped and the file is read again. The edits listed in the review are the ones to redo.', ok: 'Reload', danger: true })) && act(async () => { await api('/api/save/discard', {}); R = null; drawReview(); }, 'Reloaded from disk. Redo your edits.'),
  doSave: async () => {
    if (!R) return;
    let r;
    try {
      r = await api('/api/save/commit', { reviewed: R.token });
    } catch (e) {
      if (e.code === 'review_changed') { // the edits moved on since this pane was drawn: show the current ones
        await openReview();
        if (R) { R.error = e.message; drawReview(true); }
        return;
      }
      R.error = e.message;
      R.stale = e.code === 'changed_on_disk';
      drawReview(true);
      return;
    }
    R = null; drawReview();
    toast(r.saved ? 'Saved. Backup: ' + r.backup : 'Nothing to save', false, 15000);
    if (r.warning) toast(r.warning, true);
    try { await status(); await render(); } catch (e) { toast('Saved, but the screen could not refresh: ' + e.message, true); }
  },
  solari: () => act(() => api('/api/player/solari', { amount: +val('solari') }), 'Solari updated'),
  xp: () => act(async () => { const r = await api('/api/player/xp', { amount: +val('xpAmt') }); toast(r.applied ? `XP ${Number(r.before).toLocaleString()} to ${Number(r.after).toLocaleString()}, level ${r.levelBefore} to ${r.levelAfter}` + (r.skillPointsGained ? `, +${r.skillPointsGained} skill points` : '') + (r.capped ? ' (stopped at level 200)' : '') + NOT_SAVED : 'Already at the last level (200).'); }, null),
  currency: () => act(async () => { const r = await api('/api/player/currency', { currency: +val('curSel'), amount: +val('curAmt') }); toast(`${r.currency == 0 ? 'Solari Credit' : 'House Credit'} ${Number(r.before).toLocaleString()} to ${Number(r.after).toLocaleString()}` + NOT_SAVED); }, null),
  intel: () => act(async () => { const r = await api('/api/player/intel', { amount: +val('intelAmt') }); toast(r.applied ? `Intel ${Number(r.before).toLocaleString()} to ${Number(r.after).toLocaleString()}` + (r.capped ? ' (capped at 2,779)' : '') + NOT_SAVED : 'Intel is already at the cap (2,779).'); }, null),
  teleport: () => act(() => api('/api/player/teleport', { x: +val('tx'), y: +val('ty'), z: +val('tz') }), 'Teleport queued'),
  tpTo: (d) => act(() => api('/api/player/teleport', { x: +d.x, y: +d.y, z: +d.z }), 'Teleport queued'),
  augEdit: (d) => act(async () => { const r = await api('/api/items/augment-options', { item_id: +d.id }); r.options.forEach((o) => AUG_NAMES.set(o.id, o.name)); augItem = { ...r, item_id: +d.id }; render(); }, null, false),
  augCancel: () => { augItem = null; render(); },
  augApply: (d) => act(async () => { const r = await api('/api/items/augment', { item_id: +d.id, augments: augPicked('au', augItem.limit), grade: +val('augG'), unlock_slots: $('#augU').checked }); augItem = null; toast(`Augments set (${r.augments})` + (r.slotsUnlocked ? `, ${r.slotsUnlocked} slot keystones unlocked` : '') + NOT_SAVED); }, null),
  queueAdd: () => { const t = pickedItemId('gt'); if (!t) return toast('Pick an item first', 'err'); const e = { template_id: t, quantity: Math.max(1, +val('gq') || 1), quality: +val('gg') }; const ids = augPicked('ga', 3); if (ids.length) { e.augments = ids; e.grade = +val('gag'); } giveQueue.push(e); render(); },
  queueDel: (d) => { giveQueue.splice(+d.id, 1); render(); },
  queueClear: () => { giveQueue = []; render(); },
  queueGive: () => act(async () => { const r = await api('/api/player/give-items', { items: giveQueue }); giveQueue = []; toast(`Added ${r.count} items` + NOT_SAVED); }, null),
  give: () => act(async () => {
    const ids = augPicked('ga', 3);
    if (ids.length) await api('/api/player/give-items', { items: [{ template_id: pickedItemId('gt'), quantity: +val('gq'), quality: +val('gg'), augments: ids, grade: +val('gag') }] });
    else await api('/api/player/give', { template_id: pickedItemId('gt'), quantity: +val('gq'), quality: +val('gg') });
    toast('Item added' + NOT_SAVED);
  }, null),
  faction: () => act(async () => { const r = await api('/api/player/faction', { faction_id: +val('facSel') }); toast(`Faction set to ${r.faction}` + NOT_SAVED); }, null),
  repair: () => act(async () => { const r = await api('/api/player/repair', {}); toast(`Repaired ${r.repaired} items` + NOT_SAVED); }, null),
  refill: () => act(async () => { const r = await api('/api/player/refill', {}); const sk = (r.skippedUnknown || []).length ? ` Left alone, capacity unknown: ${r.skippedUnknown.join(', ')}.` : ''; toast(`Filled ${r.filled} containers (${r.alreadyFull} already full).${sk}` + NOT_SAVED); }, null),
  delItem: async (d) => (await ask({ title: 'Delete item?', body: 'Delete ' + d.name + ' and everything attached to it (its stats and links). Discard undoes it until you save.', ok: 'Delete', danger: true })) && act(() => api('/api/items/delete', { id: +d.id }), 'Deleted'),
  spec: (d) => { $('#st').value = d.t; $('#sx').value = d.xp; $('#sl').value = d.lv; },
  spKeyAll: async () => (await ask({ title: 'Grant all keystones?', body: 'Every specialization keystone of every track is bought. One edit in the review pane.', ok: 'Grant all', danger: true })) && act(async () => { const r = await api('/api/player/specs/keystones/grant', {}); toast(r.granted ? `${r.granted} keystones granted` + NOT_SAVED : 'All keystones were already granted.'); }, null),
  spKeyReset: async () => (await ask({ title: 'Reset all keystones?', body: 'Every specialization keystone you have bought is removed. One edit in the review pane.', ok: 'Reset all', danger: true })) && act(async () => { const r = await api('/api/player/specs/keystones/reset', {}); toast(r.removed ? `${r.removed} keystones reset` + NOT_SAVED : 'There were no keystones to reset.'); }, null),
  spAdd: (d) => act(async () => { const r = await api('/api/player/specs/xp', { track: d.t, amount: +val('spx' + d.t) }); toast(`${d.t}: ${Number(r.before).toLocaleString()} to ${Number(r.after).toLocaleString()} XP, level ${Math.floor(r.level)}` + NOT_SAVED); }, null),
  spMax: async (d) => (await ask({ title: 'Grant max ' + d.t + '?', body: d.t + ' goes to the XP of its last level (level 100). One edit in the review pane.', ok: 'Grant max', danger: true })) && act(async () => { await api('/api/player/specs/max', { track: d.t }); toast(d.t + ' set to max' + NOT_SAVED); }, null),
  spReset: async (d) => (await ask({ title: 'Reset ' + d.t + '?', body: d.t + " goes back to 0 XP. Its keystones are not changed. One edit in the review pane.", ok: 'Reset', danger: true })) && act(async () => { await api('/api/player/specs/reset', { track: d.t }); toast(d.t + ' reset' + NOT_SAVED); }, null),
  spAssign: (d) => act(async () => { const t = val('spa' + d.n); await api('/api/player/specs/assign', { track: t, number: +d.n }); toast(`Track number ${d.n} is ${t}`); }, null, false),
  spAssignHand: () => act(async () => { const t = val('spmt'), n = val('spmn'); if (n === '') throw new Error('Type the track number first.'); await api('/api/player/specs/assign', { track: t, number: +n }); toast(`${t} is track number ${n}`); }, null, false),
  specSet: () => act(() => api('/api/player/specs', { track_type: +val('st'), xp: +val('sx'), level: +val('sl') }), 'Specialization set'),
  tut: (d) => act(async () => { await api('/api/player/tutorials', { id: +d.id, complete: d.c == '1' }); JB = await api('/api/player/journey/browse'); journeyList(); toast('Tutorial updated' + NOT_SAVED); }, null),
  tagAdd: () => act(() => api('/api/player/tags', { tag: val('tag').trim(), add: true }), 'Tag added'),
  tagDel: (d) => act(() => api('/api/player/tags', { tag: d.tag, add: false }), 'Tag removed'),
  invfilter: () => { invQ = val('invq'); render(); },
  invclear: () => { invQ = ''; render(); },
  skPoints: () => act(async () => { const r = await api('/api/player/skills/points', { points: +val('skPts') }); toast(`Unspent skill points ${r.before} to ${r.after}` + NOT_SAVED); }, null),
  skStarter: () => act(async () => { const r = await api('/api/player/skills/starter', { school: skSchool }); toast(r.changed ? `Restored ${r.changed} starter skills` + NOT_SAVED : 'The starter skills are already learned.'); }, null),
  ctfilter: () => { ctQ = val('ctq'); catalogList(); },
  ctclear: () => { ctQ = ''; catalogList(); },
  ctGive: (d) => act(async () => { await api('/api/player/give-items', { items: [{ template_id: d.id, quantity: 1, quality: 0 }] }); toast('Item added to your backpack' + NOT_SAVED); }, null),
  bsAll: async () => (await ask({ title: 'Unlock all building sets?', body: 'Every building set in the list below, except the experimental ones, is added to the sets you have learned. This is one edit in the review pane.', ok: 'Unlock all' })) && act(async () => { const r = await api('/api/player/building-sets/unlock-all', {}); toast(r.added ? `Learned ${r.added} building sets` + NOT_SAVED : 'All building sets are already learned.'); }, null),
  rsAll: async () => (await ask({ title: 'Unlock all research?', body: 'Every research entry that unlocks a recipe or a building set is bought, and the recipes and sets they unlock are added. One edit in the review pane.', ok: 'Unlock all' })) && act(async () => { const r = await api('/api/player/research/unlock-all', {}); toast(r.bought || r.repaired ? `Research: ${r.bought} bought, ${r.repaired} repaired, ${r.unlocksAdded} recipes or sets added` + NOT_SAVED : 'All research is already unlocked.'); }, null),
  crAll: async () => (await ask({ title: 'Unlock all recipes?', body: 'Every crafting recipe your research tree offers that you do not know yet is added to your known recipes. One edit in the review pane.', ok: 'Unlock all' })) && act(async () => { const r = await api('/api/player/crafting/unlock-all', {}); toast(r.added ? `Added ${r.added} recipes` + NOT_SAVED : 'You already know every recipe.'); }, null),
  skMax: async (d) => (await ask({ title: 'Max skills?', body: d.school == 'all' ? 'Every skill of the five schools is set to its highest rank. The unspent points are not changed. One edit in the review pane.' : 'Every skill of this school is set to its highest rank. The unspent points are not changed.', ok: 'Max skills' })) && act(async () => { const r = await api('/api/player/skills/max', { school: d.school }); toast(r.changed ? `${r.changed} skills raised to their highest rank` + NOT_SAVED : 'Those skills are already at their highest rank.'); }, null),
  ctGrantSet: async (d) => (await ask({ title: 'Grant ' + d.name + '?', body: 'One of each cosmetic that is not already in your inventory goes into your backpack, all or nothing. This does not grant a DLC or an account entitlement a set may need.', ok: 'Grant' })) && act(async () => { const r = await api('/api/player/customizations/grant', { group: d.id }); toast(r.granted ? `Added ${r.granted} cosmetics to your backpack` + NOT_SAVED : 'All of that set is already in your inventory.'); }, null),
  lcClear: () => { LC.sel = new Set(); claimEditorDraw(); },
  lcApply: async () => {
    const c = LC.claim, cells = [...LC.sel].map(lcCoords).sort((p, q) => p[1] - q[1] || p[0] - q[0]).map(([x, y]) => ({ x, y }));
    const body = { totem_id: c.totem_id, cells };
    if (LC.level !== c.level) body.level = LC.level;
    if (!(await ask({ title: 'Edit land claim?', body: `Add ${cells.length} horizontal cell${cells.length === 1 ? '' : 's'}${LC.level !== c.level ? ` and raise the vertical level from ${c.level} to ${LC.level}` : ''}. It goes through the review pane and the usual backup; the game loads it next time.`, ok: 'Apply changes' }))) return;
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
    if (!d.on && !(await ask({ title: 'Turn on automatic refill?', body: 'From now on, opening the editor refills base water and generators and saves to your game file immediately, without the Review & save step. The previous file is backed up each time, and nothing happens while a single-player session is running. It applies to whichever save file the editor opens.', ok: 'Turn on' }))) return;
    act(async () => { const r = await api('/api/settings', { autoRefillOnOpen: !d.on }); toast('Automatic refill when the editor opens is now ' + (r.autoRefillOnOpen ? 'on' : 'off')); }, null);
  },
  claimShrink: async (d) => {
    const rings = +val('cs' + d.id);
    const size = rings ? `${2 * rings + 1} x ${2 * rings + 1}` : 'the totem\'s own cell';
    if (!(await ask({ title: 'Shrink this land claim?', body: `Removes every stored cell outside ${size}. A cell that holds building pieces is never removed. This cannot be undone from here (restore a backup to go back).`, ok: 'Shrink claim' }))) return;
    act(async () => { const r = await api('/api/bases/claim/shrink', { totem_id: +d.id, rings }); toast(`Removed ${r.removed} cells; the claim now has ${r.remaining}` + NOT_SAVED); });
  },
  claimGrow: async (d) => {
    const rings = +val('cr' + d.id), level = +val('cv' + d.id);
    const body = { totem_id: +d.id };
    if (rings > 0) body.rings = rings;
    if (level > +d.level) body.level = level;
    if (body.rings === undefined && body.level === undefined) { toast('Nothing to change: pick a bigger size or a higher level', true); return; }
    const what = [body.rings ? `every missing cell up to a ${2 * rings + 1} x ${2 * rings + 1} square around the totem` : '', body.level !== undefined ? `the vertical level raised to ${level}` : ''].filter(Boolean).join(' and ');
    if (!(await ask({ title: 'Expand this land claim?', body: `Adds ${what}. This cannot be undone from here (restore a backup to go back).`, ok: 'Expand claim' }))) return;
    act(async () => { const r = await api('/api/bases/claim/expand', body); toast(`Claim now ${r.totalCells} cells, level ${r.level} (+${r.added} cells)` + NOT_SAVED); });
  },
  refillWater: () => act(async () => { const r = await api('/api/bases/refill-water', {}); const sk = (r.skippedUnknown || []).length ? ` Left alone, capacity unknown: ${r.skippedUnknown.join(', ')}.` : ''; toast(`Filled ${r.filled} water devices (${r.alreadyFull} already full).${sk}` + NOT_SAVED); }, null),
  refillGen: () => act(async () => { const r = await api('/api/bases/refill-generators', {}); toast(`Filled ${r.filled} generators (${r.alreadyFull} already full)` + NOT_SAVED); }, null),
  repairOne: (d) => act(async () => { const r = await api('/api/vehicles/repair', { vehicle_id: +d.id }); toast(`Repaired ${r.repaired} modules of this vehicle.` + (r.withoutKnownMax ? ` ${r.withoutKnownMax} modules have no recorded maximum and were left alone.` : '') + NOT_SAVED); }, null),
  repairV: () => act(async () => { const pct = $('#rvPct'); const r = await api('/api/vehicles/repair', pct ? { threshold: +pct.value } : {}); toast(`Repaired ${r.repaired} vehicle modules.` + (r.withoutKnownMax ? ` ${r.withoutKnownMax} modules have no recorded maximum and were left alone.` : '') + NOT_SAVED); }, null),
  repairB: () => act(async () => { const r = await api('/api/bases/repair', {}); toast(`Repaired ${r.pieces} pieces, ${r.placeables} placeables` + NOT_SAVED); }, null),
  sand: () => act(async () => { const r = await api('/api/bases/clear-sand', {}); toast(`Cleared ${r.pieces} pieces` + NOT_SAVED); }, null),
  openInv: async (d) => {
    const it = await api('/api/bases/storage/items?inventory=' + encodeURIComponent(d.id));
    const cat = await loadCatalog(it.map((r) => r.template_id));
    setHTML($('#inv'), html`<div class="card"><h3>Inventory ${d.id}</h3>${tbl([{ k: 'position_index', label: 'Slot' }, { k: 'template_id', label: 'Item', f: (r) => itemCell(r.template_id) }, { k: 'stack_size', label: 'Stack', f: (r) => html`<input type="number" min="1" value="${r.stack_size}" data-item="${r.id}" data-field="stack_size" class="w90">` }], it, { actions: (r) => html`<button class="b bad sm" data-act="delItem" data-id="${r.id}" data-name="${itemName(r.template_id)}">Delete</button>` })}
    <div class="row">${itemPicker("bt", cat)}Qty<input id="bq" type="number" value="1" min="1"><button class="b" data-act="giveInv" data-id="${d.id}">Add item</button></div></div>`);
  },
  giveInv: (d) => act(async () => { await api('/api/bases/give', { inventory_id: +d.id, template_id: pickedItemId('bt'), quantity: +val('bq') }); toast('Item added' + NOT_SAVED); await A.openInv(d); }, null, false),
  bring: (d) => act(() => api('/api/vehicles/bring', { id: +d.id }), 'Vehicle moved next to you'),
  dur: (d) => { const v = prompt('Chassis durability', d.v); if (v !== null) act(() => api('/api/vehicles/durability', { vehicle_id: +d.id, chassis_durability: +v }), 'Updated'); },
  vreset: async (d) => (await ask({ title: 'Reset purchase limits?', body: 'Resets the purchase limits of this vendor.', ok: 'Reset' })) && act(() => api('/api/vendors/reset', { vendor_id: d.v }), 'Purchase limits reset'),
  term: () => act(async () => { await api('/api/landsraad/term', { active_decree_id: val('ad'), reigning_faction_id: val('rf') }); }, 'Term updated'),
  decree: (d) => act(() => api('/api/landsraad/decree', { id: +d.id, disabled: d.d == '1' }), 'Decree updated'),
  tfill: (d) => act(() => api('/api/landsraad/progress', { task_id: +d.id, faction_id: +val('lf'), amount: +d.goal }), 'Progress set'),
  tdone: async (d) => (await ask({ title: d.r == '1' ? 'Reset this task?' : 'Complete this task?', body: 'Changes the Landsraad task for the chosen faction.', ok: d.r == '1' ? 'Reset' : 'Complete' })) && act(() => api('/api/landsraad/complete', { task_id: +d.id, faction_id: +val('lf'), reset: d.r == '1' }), 'Task updated'),
  cfgcreate: (d) => act(() => api('/api/config/create', { name: d.n }), 'Created ' + d.n),
  cfgadd: () => act(() => api('/api/config/set', { name: cfgFile, section: val('cs').replace(/^\[|\]$/g, ''), key: val('ck').trim(), value: val('cv') }), 'Added', true),
  cfgdel: async (d) => (await ask({ title: 'Delete config line?', body: 'Delete ' + d.key + ' from ' + cfgFile + '. A backup of the file is made first.', ok: 'Delete', danger: true })) && act(() => api('/api/config/delete', { name: cfgFile, line: +d.line, key: d.key }), 'Deleted'),
  cfgraw: () => act(() => api('/api/config/raw', { name: cfgFile, text: val('raw') }), 'File written'),
  cfgrestore: async (d) => (await ask({ title: 'Restore config backup?', body: 'Replace ' + cfgFile + ' with ' + d.b + '. Close the game first.', ok: 'Restore', danger: true })) && act(() => api('/api/config/restore', { name: cfgFile, backup: d.b }), 'Restored'),
  dbgo: () => { dbQ = val('dbq'); dbOff = 0; render(); },
  dbprev: () => { dbOff = Math.max(0, dbOff - 100); render(); },
  dbnext: () => { dbOff += 100; render(); },
  runsql: async () => {
    window.lastSql = val('sql');
    try {
      const r = await api('/api/db/sql', { sql: window.lastSql });
      setHTML($('#sqlout'), html`<p class="mut">${r.rows.length} rows</p><div class="scroll"><table><thead><tr>${r.columns.map((c) => html`<th>${c}</th>`)}</tr></thead><tbody>${r.rows.map((x) => html`<tr>${x.map((v) => html`<td>${v}</td>`)}</tr>`)}</tbody></table></div>`);
    } catch (e) { toast(e.message, true); }
  },
  execsql: async () => {
    window.lastSql = val('sql');
    if (!(await ask({ title: 'Run this SQL as a write?', body: window.lastSql.slice(0, 400), ok: 'Run', typed: 'write', danger: true }))) return;
    try {
      const r = await api('/api/db/exec', { sql: window.lastSql });
      toast(r.changes + ' row(s) changed');
      await status();
      setHTML($('#sqlout'), html`<p class="ok">${r.changes} row(s) changed. Press "Review &amp; save" to write them to game.db.</p>`);
    } catch (e) { toast(e.message, true); }
  },
  restore: async (d) => (await ask({ title: 'Restore this backup over the save?', body: 'Replace game.db with ' + d.n + '. The current file is backed up first. Close the game first.', ok: 'Restore', typed: 'restore', danger: true })) && act(() => api('/api/save/restore', { name: d.n }), 'Restored'),
};
document.addEventListener('click', (e) => {
  const hth = e.target.closest('thead th');
  if (hth && !e.target.closest('button, a, input, select') && headerClick(hth)) return;
  const t = e.target.closest('[data-tab]');
  if (t) { if (Object.hasOwn(TABS, t.dataset.tab)) { tab = t.dataset.tab; localStorage.tab = tab; render(); } return; }
  const cg = e.target.closest('[data-ctg]');
  if (cg) { ctGroup = cg.dataset.ctg; catalogList(); return; }
  const lcc = e.target.closest('[data-cell]');
  if (lcc && LC) { lcToggle(lcc.getAttribute('data-cell')); claimEditorDraw(); return; }
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
    fetch(x.href, { credentials: 'same-origin' }).then((r) => r.blob()).then((b) => { const u = URL.createObjectURL(b), l = document.createElement('a'); l.href = u; l.download = dbTable + '.' + x.dataset.export; l.click(); URL.revokeObjectURL(u); });
  }
});
document.addEventListener('change', (e) => {
  const t = e.target;
  if (t.id == 'gt') { showGiveAugments(); return; }
  if (t.id == 'skCharge') { skCharge = t.checked; return; }
  if (t.id == 'rsGrp') { rsGroup = t.value; researchList(); return; }
  if (t.id == 'lcLvl' && LC) { LC.level = +t.value; claimEditorDraw(); return; }
  if (t.id == 'ctgrp') { ctGroup = t.value; catalogList(); return; }
  if (t.id == 'ctexp') { ctExp = t.checked; if (!ctExp && CT.rows.some((r) => r.experimental && r.group == ctGroup)) ctGroup = ''; catalogList(); return; }
  if (t.dataset.skill) { act(async () => { const r = await api('/api/player/skills/module', { module: t.dataset.skill, level: +t.value, charge: skCharge }); toast(`Skill set: ${r.pointsBefore} to ${r.pointsAfter} points` + NOT_SAVED); }, null); return; }
  if (t.dataset.item) act(() => api('/api/items/update', { id: +t.dataset.item, [t.dataset.field]: +t.value }), 'Item updated', false);
  else if (t.dataset.faction) act(() => api('/api/player/factions', { faction_id: +t.dataset.faction, amount: +t.value }), 'Reputation set', false);
  else if (t.dataset.hp) act(() => api('/api/bases/health', { kind: 'placeable', id: +t.dataset.id, health: +t.value }), 'Health set', false);
  else if (t.classList.contains('cfgv')) act(() => api('/api/config/set', { name: cfgFile, section: t.dataset.section, key: t.dataset.key, value: t.value, line: +t.dataset.line }), 'Saved ' + t.dataset.key, false);
  else if (t.classList.contains('dbc')) act(() => api('/api/db/update', { table: dbTable, rowid: +t.dataset.rowid, column: t.dataset.col, value: t.value === '' && t.dataset.orig === '' ? null : t.value }), 'Cell updated', false);
  else if (t.id == 'cfgsel') { cfgFile = t.value; render(); }
  else if (t.id == 'dbsel') { dbTable = t.value; dbOff = 0; dbQ = ''; render(); }
});
status().then(render).then(async () => {
  try { const r = await api('/api/startup'); if (r.note) toast(r.note, !!r.warn); } catch (e) { /* the note is optional */ }
});
// Keeps the game-running warning current. A failed poll is shown, not hidden: Save stays off until contact returns.
setInterval(async () => {
  if (document.hidden) return;
  try { await status(); } catch (e) {
    offline = true;
    const gw = $('#gamewarn');
    gw.className = 'warn';
    setHTML(gw, html`<b>Lost contact with the editor.</b> Saving is off until it answers again. If you closed the editor, nothing more will be written.`);
    syncReview();
  }
}, 5000);
document.addEventListener('keydown', (e) => {
  if (askState) { // the question owns the keyboard: Tab stays inside it, Enter completes a typed answer, Escape cancels
    if (e.key === 'Escape') endAsk(false);
    else if (e.key === 'Enter' && e.target.id === 'atype' && !$('#aok').disabled) endAsk(true);
    else if (e.key === 'Tab') {
      const f = [...$('#ask').querySelectorAll('input,button')].filter((x) => !x.disabled);
      if (!f.length) return;
      const i = f.indexOf(document.activeElement);
      const n = e.shiftKey ? (i <= 0 ? f.length - 1 : i - 1) : (i === f.length - 1 ? 0 : i + 1);
      e.preventDefault();
      f[n].focus();
    }
    return;
  }
  if (e.key === 'Escape' && R) A.closeReview();
});
document.addEventListener('mousedown', (e) => { if (askState && e.target === $('#ask')) endAsk(false); else if (R && !askState && e.target === $('#modal')) A.closeReview(); });
