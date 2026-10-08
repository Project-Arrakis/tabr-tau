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

// ---------- tabs
const TABS = { player: 'Player', bases: 'Bases', vehicles: 'Vehicles', exchange: 'Exchange', landsraad: 'Landsraad', config: 'Config', db: 'Database' };
let tab = Object.hasOwn(TABS, localStorage.tab) ? localStorage.tab : 'player';
const sub = { player: 'overview', db: 'browse' };
const GROUPS = ['player', 'bases', 'db'];
function drawNav() {
  setHTML($('#nav'), html`${Object.entries(TABS).map(([k, v]) => html`<button data-tab="${k}" class="${k == tab ? 'on' : ''}">${v}</button>`)}`);
}
function subNav(group, items) {
  return html`<div class="sub">${items.map(([k, v]) => html`<button data-sub="${group}:${k}" class="${sub[group] == k ? 'on' : ''}">${v}</button>`)}</div>`;
}

// ---------- generic renderers
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
async function playerView() {
  const s = sub.player;
  const h = [subNav('player', [['overview', 'Overview'], ['inventory', 'Inventory'], ['progress', 'Progression'], ['journey', 'Journey'], ['recipes', 'Recipes']])];
  if (s == 'overview') {
    P = await api('/api/player');
    const a = P.actor || {}, acc = P.account || {}, j = P.journey || {};
    h.push(html`<div class="card"><h3>${P.name}</h3><div class="grid">${kv('Solari', P.solari.toLocaleString())}${kv('Map', a.map)}${kv('Position', `${fix(a.x)}, ${fix(a.y)}, ${fix(a.z)}`)}${kv('Account', acc.funcom_id)}${kv('Platform', (acc.platform_name || '') + ' ' + (acc.platform_id || ''))}${kv('Journey', `${j.done || 0} / ${j.total || 0} nodes done`)}${kv('Pawn / controller', P.pawnId + ' / ' + P.controllerId)}</div></div>
    <div class="card"><h3>Solari</h3><div class="row"><input id="solari" type="number" value="10000"><button class="b" data-act="solari">Add / remove</button><span class="mut">Negative removes. Solari is an item stack in your backpack.</span></div></div>
    <div class="card"><h3>Teleport</h3><div class="row">X<input id="tx" type="number" value="${Math.round(a.x)}">Y<input id="ty" type="number" value="${Math.round(a.y)}">Z<input id="tz" type="number" value="${Math.round(a.z)}"><button class="b" data-act="teleport">Teleport</button></div>
    <p class="mut">Moves the character in the save (applied on next load). Stay above the terrain (Z) or you may fall through.</p></div>
    <div class="card"><h3>Respawn points</h3>${tbl([{ k: 'group' }, { k: 'locator_name' }, { k: 'locator_actor_id' }, { k: 'map' }], P.respawns)}</div>`);
  } else if (s == 'inventory') {
    const d = await api('/api/player/inventory');
    const cat = await loadCatalog(d.templates);
    h.push(html`<div class="card"><h3>Give item</h3><div class="row">${itemPicker("gt", cat)}
    Qty<input id="gq" type="number" value="1" min="1">Grade<select id="gg">${[0, 1, 2, 3, 4, 5].map((n) => html`<option>${n}</option>`)}</select><button class="b" data-act="give">Give</button><button class="b sec" data-act="repair">Repair all gear</button><button class="b sec" data-act="refill">Refill containers</button></div>
    <p class="mut">Pick an item by its in-game name. Items already in your save that the catalog does not know are listed by their template id; any valid template id can also be typed.</p></div>
    <div class="card"><h3>Items (${d.items.length})</h3>${tbl([{ k: 'inventory_name', label: 'Inventory' }, { k: 'position_index', label: 'Slot' }, { k: 'template_id', label: 'Item', f: (r) => itemCell(r.template_id) },
      { k: 'stack_size', label: 'Stack', f: (r) => html`<input type="number" value="${r.stack_size}" min="1" data-item="${r.id}" data-field="stack_size" class="w90">` },
      { k: 'quality_level', label: 'Grade', f: (r) => html`<select data-item="${r.id}" data-field="quality">${[0, 1, 2, 3, 4, 5].map((n) => html`<option ${n == r.quality_level ? 'selected' : ''}>${n}</option>`)}</select>` },
      { k: 'durability', label: 'Durability', f: (r) => (r.durability == null ? '' : Number(r.durability).toFixed(1) + (r.max_durability ? ' / ' + Number(r.max_durability).toFixed(0) : '')) }], d.items,
      { actions: (r) => html`<button class="b bad sm" data-act="delItem" data-id="${r.id}" data-name="${itemName(r.template_id)}">Delete</button>` })}</div>`);
  } else if (s == 'progress') {
    const [f, sp, tu, tg] = await Promise.all(['factions', 'specs', 'tutorials', 'tags'].map((x) => api('/api/player/' + x)));
    h.push(html`<div class="card"><h3>Faction reputation</h3>${tbl([{ k: 'name', label: 'Faction' }, { k: 'reputation', label: 'Reputation', f: (r) => html`<input type="number" min="0" max="12474" value="${r.reputation}" data-faction="${r.faction_id}" class="w100">` }], f)}<p class="mut">Range 0–12474. Edit and press Tab/Enter.</p></div>
    <div class="card"><h3>Specialization tracks</h3>${tbl([{ k: 'track_type', label: 'Track' }, { k: 'xp_amount', label: 'XP' }, { k: 'level' }], sp, { actions: (r) => html`<button class="b sec sm" data-act="spec" data-t="${r.track_type}" data-xp="${r.xp_amount}" data-lv="${r.level}">Edit</button>` })}<div class="row"><span class="mut">Add/overwrite:</span>Track<input id="st" type="number" value="0" class="w70">XP<input id="sx" type="number" value="0">Level<input id="sl" type="number" step="0.1" value="1" class="w80"><button class="b" data-act="specSet">Set</button></div></div>
    <div class="card"><h3>Tutorials</h3>${tbl([{ k: 'id' }, { k: 'name' }, { k: 'state', label: 'State', f: (r) => (r.state == 2 ? html`<span class="ok">done</span>` : html`<span class="mut">not done</span>`) }], tu, { actions: (r) => (r.state == 2 ? html`<button class="b sec sm" data-act="tut" data-id="${r.id}" data-c="0">Reset</button>` : html`<button class="b sm" data-act="tut" data-id="${r.id}" data-c="1">Complete</button>`) })}</div>
    <div class="card"><h3>Player tags</h3>${tg.length ? tg.map((t) => html`<span class="tag">${t.tag} <a href="#" data-act="tagDel" data-tag="${t.tag}">×</a></span>`) : html`<span class="mut">none</span>`}<div class="row"><input id="tag" placeholder="Tag.Name" size="40"><button class="b" data-act="tagAdd">Add tag</button></div></div>`);
  } else if (s == 'journey') {
    h.push(html`<div class="card"><h3>Journey nodes</h3><div class="row"><input id="jq" placeholder="filter (e.g. DA_MQ)" size="30" value="${window.jq || ''}"><button class="b sec" data-act="jfilter">Filter</button><span class="mut">Completing a node also completes its children.</span></div><div id="jout"></div></div>`);
  } else if (s == 'recipes') {
    const r = await api('/api/player/recipes');
    h.push(html`<div class="card"><h3>Learned building sets (${r.learnedSets.length})</h3>${r.learnedSets.map((x) => html`<span class="tag">${x.name}</span>`)}</div><div class="card"><h3>New buildable pieces (${r.newPieces.length})</h3>${r.newPieces.map((x) => html`<span class="tag">${x.name}</span>`)}</div>`);
  }
  setHTML($('#main'), html`${h}`);
  if (s == 'journey') journeyList();
}
async function journeyList() {
  const rows = await api('/api/player/journey?q=' + encodeURIComponent(window.jq || ''));
  setHTML($('#jout'), html`<p class="mut">${rows.length} nodes${rows.length > 500 ? ' (showing first 500)' : ''}</p>${tbl([{ k: 'id', label: 'Node' }, { k: 'complete', label: 'State', f: (r) => (r.complete ? html`<span class="ok">complete</span>` : html`<span class="mut">open</span>`) }], rows.slice(0, 500),
    { actions: (r) => (r.complete ? html`<button class="b sec sm" data-act="jset" data-id="${r.id}" data-c="0">Reset</button>` : html`<button class="b sm" data-act="jset" data-id="${r.id}" data-c="1">Complete</button>`) })}`);
}

// ---------- BASES
async function basesView() {
  const s = sub.bases || 'overview';
  const h = [subNav('bases', [['overview', 'Overview'], ['storage', 'Storage'], ['parts', 'Pieces & placeables']])];
  if (s == 'overview') {
    const b = await api('/api/bases');
    const p = b.pieces || {};
    h.push(html`<div class="card"><h3>Bases (land claims)</h3>${tbl([{ k: 'totem_id', label: 'Totem' }, { k: 'map' }, { k: 'x', f: (r) => fix(r.x) }, { k: 'y', f: (r) => fix(r.y) }, { k: 'z', f: (r) => fix(r.z) }, { k: 'level' }], b.totems, { actions: (r) => html`<button class="b sec sm" data-act="tpTo" data-x="${r.x}" data-y="${r.y}" data-z="${r.z + 300}">Teleport here</button>` })}</div>
    <div class="card"><h3>Structure health</h3><div class="grid">${kv('Building pieces', p.n)}${kv('Lowest health', fix(p.minh))}${kv('Average health', fix(p.avgh))}${kv('Total sand buildup', fix(p.sand))}</div>
    <div class="row"><button class="b" data-act="repairB">Repair all to max</button><button class="b sec" data-act="sand">Clear sand buildup</button><span class="mut">Repair sets each piece to the highest health seen for its type.</span></div></div>
    <div class="card"><h3>Permissions</h3>${tbl([{ k: 'actor_id' }, { k: 'actor_name' }, { k: 'actor_type' }, { k: 'access_level' }, { k: 'is_child' }], b.permissions)}</div>`);
  } else if (s == 'storage') {
    const st = await api('/api/bases/storage');
    h.push(html`<div class="card"><h3>Containers, machines &amp; vehicles</h3>${tbl([{ k: 'name', label: 'Object' }, { k: 'actor_id', label: 'Actor' }, { k: 'inventory_id', label: 'Inv' }, { k: 'inventory_type', label: 'Type' }, { k: 'items' }, { k: 'total', label: 'Total units' }, { k: 'max_item_count', label: 'Slots' }], st,
      { actions: (r) => html`<button class="b sec sm" data-act="openInv" data-id="${r.inventory_id}">Open</button>` })}</div><div id="inv"></div>`);
  } else {
    const b = await api('/api/bases');
    h.push(html`<div class="card"><h3>Placeables (${b.placeables.length})</h3>${tbl([{ k: 'id' }, { k: 'building_type', label: 'Type' }, { k: 'health', f: (r) => html`<input type="number" value="${r.health}" data-hp="placeable" data-id="${r.id}" class="w90">` }], b.placeables)}</div>
    <div class="card"><h3>Building piece types</h3>${tbl([{ k: 'building_type', label: 'Type' }, { k: 'n', label: 'Count' }, { k: 'minh', label: 'Min health' }, { k: 'maxh', label: 'Max health' }, { k: 'avgh', label: 'Avg' }], b.types)}</div>`);
  }
  setHTML($('#main'), html`${h}`);
}

// ---------- VEHICLES
async function vehiclesView() {
  const v = await api('/api/vehicles');
  const h = [html`<div class="card"><h3>Vehicles in the world (${v.vehicles.length})</h3>${tbl([{ k: 'name', label: 'Vehicle' }, { k: 'id' }, { k: 'map' }, { k: 'x', f: (r) => fix(r.x) }, { k: 'y', f: (r) => fix(r.y) }, { k: 'z', f: (r) => fix(r.z) },
    { k: 'modules', label: 'Modules', f: (r) => (r.modules || []).map((m) => html`<span class="tag">${m.template_id}</span>`) }], v.vehicles,
    { actions: (r) => html`<button class="b sm" data-act="bring" data-id="${r.id}">Bring to me</button>` })}</div>
  <div class="card"><h3>Recovered / stored vehicles (${v.recovered.length})</h3>${tbl([{ k: 'vehicle_id', label: 'Id' }, { k: 'vehicle_name', label: 'Name' }, { k: 'chassis_durability', label: 'Chassis durability' }, { k: 'time_stored' }, { k: 'reason' }], v.recovered,
    { actions: (r) => html`<button class="b sec sm" data-act="dur" data-id="${r.vehicle_id}" data-v="${r.chassis_durability}">Set durability</button>` })}</div>`];
  if (!v.vehicles.length && !v.recovered.length) h.push(html`<p class="mut">No vehicles in this save yet. Vehicle fuel is stored in an opaque binary blob and is not editable.</p>`);
  setHTML($('#main'), html`${h}`);
}

// ---------- EXCHANGE
async function exchangeView() {
  const e = await api('/api/exchange');
  setHTML($('#main'), html`<div class="card"><h3>Solari</h3><div class="grid">${kv('Balance', e.solari.toLocaleString())}</div><div class="row"><input id="solari" type="number" value="10000"><button class="b" data-act="solari">Add / remove</button></div></div>
  <div class="card"><h3>Vendor purchase limits</h3><p class="mut">Vendors limit how much you can buy per restock cycle. Resetting clears those counters.</p><div class="row"><button class="b" data-act="vreset" data-v="">Reset all vendors</button></div>
  ${tbl([{ k: 'vendor_id', label: 'Vendor' }, { k: 'template_id', label: 'Item', f: (r) => itemCell(r.template_id) }, { k: 'amount_bought', label: 'Bought' }], e.stock, { actions: (r) => html`<button class="b sec sm" data-act="vreset" data-v="${r.vendor_id}">Reset vendor</button>` })}
  <h3 class="mt14">Restock cycles</h3>${tbl([{ k: 'vendor_id' }, { k: 'last_interacted_timestamp', label: 'Last interaction', f: (r) => new Date(r.last_interacted_timestamp * 1000).toLocaleString() }], e.cycles)}</div>`);
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
    setHTML($('#dbout'), html`<p class="mut">${d.total} rows — showing ${dbOff + 1}–${Math.min(dbOff + 100, d.total)}</p><div class="scroll"><table><thead><tr>${cols.map((c) => html`<th>${c}</th>`)}</tr></thead><tbody>${d.rows.map((r) => html`<tr>${r.slice(1).map((v, i) => {
      const b = typeof v === 'string' && v.startsWith('<blob');
      return html`<td>${b ? html`<span class="mut mono">${v}</span>` : html`<input class="dbc" value="${v}" data-rowid="${r[0]}" data-col="${cols[i]}" data-orig="${v}" size="${Math.max(6, Math.min(40, String(v ?? '').length + 2))}">`}</td>`;
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
    await ({ player: playerView, bases: basesView, vehicles: vehiclesView, exchange: exchangeView, landsraad: landsraadView, config: configView, db: dbView })[tab]();
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
  teleport: () => act(() => api('/api/player/teleport', { x: +val('tx'), y: +val('ty'), z: +val('tz') }), 'Teleport queued'),
  tpTo: (d) => act(() => api('/api/player/teleport', { x: +d.x, y: +d.y, z: +d.z }), 'Teleport queued'),
  give: () => act(() => api('/api/player/give', { template_id: pickedItemId('gt'), quantity: +val('gq'), quality: +val('gg') }), 'Item added'),
  repair: () => act(async () => { const r = await api('/api/player/repair', {}); toast(`Repaired ${r.repaired} items` + NOT_SAVED); }, null),
  refill: () => act(async () => { const r = await api('/api/player/refill', {}); const sk = (r.skippedUnknown || []).length ? ` Left alone, capacity unknown: ${r.skippedUnknown.join(', ')}.` : ''; toast(`Filled ${r.filled} containers (${r.alreadyFull} already full).${sk}` + NOT_SAVED); }, null),
  delItem: async (d) => (await ask({ title: 'Delete item?', body: 'Delete ' + d.name + ' and everything attached to it (its stats and links). Discard undoes it until you save.', ok: 'Delete', danger: true })) && act(() => api('/api/items/delete', { id: +d.id }), 'Deleted'),
  spec: (d) => { $('#st').value = d.t; $('#sx').value = d.xp; $('#sl').value = d.lv; },
  specSet: () => act(() => api('/api/player/specs', { track_type: +val('st'), xp: +val('sx'), level: +val('sl') }), 'Specialization set'),
  tut: (d) => act(() => api('/api/player/tutorials', { id: +d.id, complete: d.c == '1' }), 'Tutorial updated'),
  tagAdd: () => act(() => api('/api/player/tags', { tag: val('tag').trim(), add: true }), 'Tag added'),
  tagDel: (d) => act(() => api('/api/player/tags', { tag: d.tag, add: false }), 'Tag removed'),
  jfilter: () => { window.jq = val('jq'); journeyList(); },
  jset: (d) => act(() => api('/api/player/journey', { node_id: d.id, complete: d.c == '1' }), 'Journey updated', false).then(journeyList),
  repairB: async () => (await ask({ title: 'Repair every base piece?', body: 'Sets the health of all building pieces and placeables in the save to full.', ok: 'Repair all', typed: 'repair', danger: true })) && act(async () => { const r = await api('/api/bases/repair', {}); toast(`Repaired ${r.pieces} pieces, ${r.placeables} placeables` + NOT_SAVED); }, null),
  sand: async () => (await ask({ title: 'Clear sand build-up?', body: 'Removes the sand coverage from every building piece in the save.', ok: 'Clear sand', typed: 'clear', danger: true })) && act(async () => { const r = await api('/api/bases/clear-sand', {}); toast(`Cleared ${r.pieces} pieces` + NOT_SAVED); }, null),
  openInv: async (d) => {
    const it = await api('/api/bases/storage/items?inventory=' + encodeURIComponent(d.id));
    const cat = await loadCatalog(it.map((r) => r.template_id));
    setHTML($('#inv'), html`<div class="card"><h3>Inventory ${d.id}</h3>${tbl([{ k: 'position_index', label: 'Slot' }, { k: 'template_id', label: 'Item', f: (r) => itemCell(r.template_id) }, { k: 'stack_size', label: 'Stack', f: (r) => html`<input type="number" min="1" value="${r.stack_size}" data-item="${r.id}" data-field="stack_size" class="w90">` }], it, { actions: (r) => html`<button class="b bad sm" data-act="delItem" data-id="${r.id}" data-name="${itemName(r.template_id)}">Delete</button>` })}
    <div class="row">${itemPicker("bt", cat)}Qty<input id="bq" type="number" value="1" min="1"><button class="b" data-act="giveInv" data-id="${d.id}">Add item</button></div></div>`);
  },
  giveInv: (d) => act(async () => { await api('/api/bases/give', { inventory_id: +d.id, template_id: pickedItemId('bt'), quantity: +val('bq') }); toast('Item added' + NOT_SAVED); await A.openInv(d); }, null, false),
  bring: (d) => act(() => api('/api/vehicles/bring', { id: +d.id }), 'Vehicle moved next to you'),
  dur: (d) => { const v = prompt('Chassis durability', d.v); if (v !== null) act(() => api('/api/vehicles/durability', { vehicle_id: +d.id, chassis_durability: +v }), 'Updated'); },
  vreset: async (d) => (await ask({ title: 'Reset purchase limits?', body: 'Resets the purchase limits of this vendor.', ok: 'Reset' })) && act(() => api('/api/exchange/reset', { vendor_id: d.v }), 'Purchase limits reset'),
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
  const t = e.target.closest('[data-tab]');
  if (t) { if (Object.hasOwn(TABS, t.dataset.tab)) { tab = t.dataset.tab; localStorage.tab = tab; render(); } return; }
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
  if (t.dataset.item) act(() => api('/api/items/update', { id: +t.dataset.item, [t.dataset.field]: +t.value }), 'Item updated', false);
  else if (t.dataset.faction) act(() => api('/api/player/factions', { faction_id: +t.dataset.faction, amount: +t.value }), 'Reputation set', false);
  else if (t.dataset.hp) act(() => api('/api/bases/health', { kind: 'placeable', id: +t.dataset.id, health: +t.value }), 'Health set', false);
  else if (t.classList.contains('cfgv')) act(() => api('/api/config/set', { name: cfgFile, section: t.dataset.section, key: t.dataset.key, value: t.value, line: +t.dataset.line }), 'Saved ' + t.dataset.key, false);
  else if (t.classList.contains('dbc')) act(() => api('/api/db/update', { table: dbTable, rowid: +t.dataset.rowid, column: t.dataset.col, value: t.value === '' && t.dataset.orig === '' ? null : t.value }), 'Cell updated', false);
  else if (t.id == 'cfgsel') { cfgFile = t.value; render(); }
  else if (t.id == 'dbsel') { dbTable = t.value; dbOff = 0; dbQ = ''; render(); }
});
status().then(render);
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
