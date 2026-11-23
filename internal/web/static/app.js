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
// ---------- motion (#571)
// window.TauMotion comes from /motion-boot.js, a file in the head: what the page does is Full, Calm or Off, written on the root element, where the design module's motion.css reads it.
// A mark that asks for the eye (a count that bumps, a row that lights up) is for Full only; a change that eases (a toast that fades) is for Full and Calm; in Off every change is
// instant. A page without that file (a test) has no limit of its own and is treated as Full. Nothing here loops, nothing moves the layout, and nothing changes what is written to a save.
const motionOn = (kind) => { const m = window.TauMotion; return !m || (kind === 'ease' ? m.eases() : m.marks()); };
// Plays a one-shot animation class on an element and takes it off again (after ms, so that the same mark can play the next time too).
function playMark(el, cls, ms) {
  if (!el || !motionOn('mark')) return;
  el.classList.remove(cls);
  void el.offsetWidth; // a class that is put back at once only plays again after a reflow
  el.classList.add(cls);
  setTimeout(() => el.classList.remove(cls), ms);
}
// The result icons of a toast and of the button that was pressed (the design module's fill icons, motion.css .d-fx): a drop (water), a heart (health), a bolt (stamina), a ring (the fills
// together), a can (fuel) or a shield (repair) whose level rises while the toast comes in and settles with a check. The shapes are the module's; the animation is its CSS. They are for
// Motion = Full only (fullMotion): Calm and Off show the plain check. The level starts at --lvl-from, set through the CSSOM (the policy has no inline style).
const SVGNS = 'http://www.w3.org/2000/svg';
const FX_SHAPE = { water: 'M12 2.5C12 2.5 5.5 9.4 5.5 14a6.5 6.5 0 0 0 13 0C18.5 9.4 12 2.5 12 2.5Z', fuel: 'M6.5 3.5h8l3 3v14h-11z', repair: 'M12 2.8l7 2.9v5.6c0 4.3-2.9 7.7-7 9.3-4.1-1.6-7-5-7-9.3V5.7z',
  health: 'M12 20.4C12 20.4 3.6 15 3.6 8.9A4.4 4.4 0 0 1 12 7a4.4 4.4 0 0 1 8.4 1.9C20.4 15 12 20.4 12 20.4Z', stamina: 'M13.4 2.8L5.8 13.4h5.1l-1.1 7.8 7.7-10.6h-5.1z', all: 'M12 3.2a8.8 8.8 0 1 0 0 17.6 8.8 8.8 0 0 0 0-17.6Z' };
// Which icon each fill, repair and refuel action plays, keyed by the ACTION that was pressed (its data-act), never by the words or the numbers of the result. A new fill or repair
// action has to be added here: web-tests/fillfx.test.mjs reads the action table and fails for one that is missing. Fill all liquids is the three fills together (water, blood, fuel): the ring.
const FX_OF = { fillWater: 'water', refill: 'water', fillBlood: 'water', fillLiquids: 'all', refillWater: 'water', refillGen: 'fuel', refuelV: 'fuel', repair: 'repair', repairOne: 'repair', repairV: 'repair', repairB: 'repair', repairAll: 'repair' };
// Motion = Full is the only mode that plays the fill icons (the setting, or the system's reduced motion, makes it Calm). A page without /motion-boot.js has no limit of its own.
const fullMotion = () => { const m = window.TauMotion; return !m || m.mode() === 'full'; };
const MARK_PATHS = { ok: ['M5 12.5l4.5 4.5L19 7.5'], warn: ['M12 4l9 16H3z', 'M12 10v4.5M12 17.4v.3'], bad: ['M6 6l12 12M18 6L6 18'], queued: ['M12 4a8 8 0 1 0 0 16a8 8 0 1 0 0-16z', 'M12 8v4.5l3 1.8'] };
let fxUid = 0;
function svgEl(tag, attrs, parent) {
  const e = document.createElementNS(SVGNS, tag);
  for (const k of Object.keys(attrs)) e.setAttribute(k, attrs[k]);
  if (parent) parent.appendChild(e);
  return e;
}
function fxIcon(kind, from) {
  const id = 'd-fx-clip-' + ++fxUid;
  const svg = svgEl('svg', { class: 'd-fx ' + kind, viewBox: '0 0 24 24', 'aria-hidden': 'true' });
  const clip = svgEl('clipPath', { id }, svgEl('defs', {}, svg));
  svgEl('path', { d: FX_SHAPE[kind] }, clip);
  const lvl = svgEl('rect', { class: 'lvl', x: '0', y: '0', width: '24', height: '24' }, svgEl('g', { 'clip-path': 'url(#' + id + ')' }, svg));
  lvl.style.setProperty('--lvl-from', 100 - Math.max(0, Math.min(100, from === undefined ? 30 : from)) + '%');
  svgEl('path', { class: 'shell', d: FX_SHAPE[kind] }, svg);
  svgEl('path', { class: 'chk', d: 'M8.6 13.4l2.7 2.7 4.5-5.1' }, svg);
  return svg;
}
// a shape as well as a colour for what a toast says: a check, a warning triangle or a cross
function markIcon(kind) {
  const svg = svgEl('svg', { class: 'd-i d-pop-in', viewBox: '0 0 24 24', 'aria-hidden': 'true' });
  for (const d of MARK_PATHS[kind]) svgEl('path', { d }, svg);
  return svg;
}
// A toast leaves with the module's d-out (a fade); in Off it is gone at once. The timer is the way out where no animation ends (a window that is not shown).
function leaveToast(d) {
  if (!d.isConnected) return;
  if (!motionOn('ease')) { d.remove(); return; }
  d.classList.add('out');
  setTimeout(() => d.remove(), 260);
}
// A toast: what an action did, in words. err: true (an error, red: stays until it is dismissed), 'part' (a warning that stays: part of a give went in), 'note' (an amber notice with no icon that
// leaves by itself: the action wrote nothing, because everything was already as it should be) or false. fx is { kind, from } with a kind of FX_SHAPE: a result that went well and filled or repaired
// something shows the fill icon of that kind instead of the check, in Motion = Full only; never on a warning or an error.
// The Command Center board says its results here (#943): one at a time (a new one takes the place of the last: data-one), several lines where a result has several parts, and the time on screen
// follows the length of the text (toastMs). A write that fails stays until it is dismissed, as every error does.
const toastMs = (msg) => Math.min(20000, Math.max(3500, 2500 + String(msg).length * 40));
function toast(msg, err, ms = 3500, fx = null, opts = null) {
  const d = document.createElement('div');
  const one = opts && opts.one; // the toast a call raises itself says so (opts), nothing is held between calls: an unrelated toast (a poll, a lost connection) is never taken for the write's
  const stays = (o) => o.classList.contains('err') || o.classList.contains('part'); // an error or a warning that stays until it is dismissed
  // a new result takes the place of the last of its kind; a soft one (the answer to a click that wrote nothing) never takes away an error or a warning that is still showing, it stands next to it
  if (one) { for (const o of [...$('#toast').children]) if (o.getAttribute('data-one') === one && !(opts.soft && stays(o))) o.remove(); d.setAttribute('data-one', one); }
  if (opts && opts.lines) d.setAttribute('data-lines', 'true');
  const part = err === 'part', note = err === 'note';
  const queued = !err && !!(opts && opts.queued); // work that waits (a refill queued behind the game): a neutral clock, never the check of a result that went through (#1123)
  const kind = queued ? 'queued' : !err ? 'ok' : part || note ? 'warn' : 'bad';
  d.setAttribute('role', err && !part && !note ? 'alert' : 'status');
  d.className = 'd-toast' + (kind === 'ok' ? '' : ' ' + kind) + (err ? (part ? ' part' : note ? ' note' : ' err') : '');
  const fxKind = kind === 'ok' && fx && Object.hasOwn(FX_SHAPE, fx.kind) && fullMotion() ? fx.kind : '';
  if (fxKind) d.setAttribute('data-fx', fxKind);
  d.append(fxKind ? fxIcon(fxKind, fx.from) : markIcon(kind));
  const t = document.createElement('span');
  t.textContent = msg;
  d.append(t);
  if (err && !note) { // an error stays until it is dismissed: a message that vanishes in seconds is easy to miss
    const x = document.createElement('button');
    x.className = 'x';
    x.textContent = '\u00d7';
    x.title = 'Dismiss';
    x.setAttribute('aria-label', 'Dismiss this message');
    x.onclick = () => d.remove();
    d.append(x);
  } else setTimeout(() => leaveToast(d), note ? Math.max(ms, 6000) : ms);
  $('#toast').append(d);
}
// The note the editor leaves about the automatic pass. A refill that is only queued behind the game is said in one short sentence with a neutral queued mark; the whole reason is in the banner and in the
// list "What is queued" (#1123). Every other note is shown as it is.
const REFILL_QUEUED = 'Automatic refill queued: ';
const PASS_QUEUED = 'Queued: '; // the answer of "Run the automatic pass now" when the pass waits for the save (#1135): the neutral queued mark, never the check of a result that went through
function startupToast(r) {
  if (!r.warn && String(r.note).startsWith(REFILL_QUEUED)) { toast('Refill queued; it runs when the save is free.', false, 6000, null, { queued: true }); return; }
  toast(r.note, !!r.warn, 15000);
}
// The same icon on the button that was pressed (Motion = Full only). The button has no icon of its own to take the place of, so for a moment the icon sits in a round chip in the middle of the button
// and the label is made clear (colour only: the words, the title and the aria are untouched), then the button is as it was. The chip is out of the flow (absolute), so nothing moves. The motion is
// the toast's: the module's CSS. One timer takes the overlay away; a second press starts it again.
const FX_HOLD_MS = 2200; // the drop fills in about 1.2 s and the check is drawn after it
const fxOnButton = new WeakMap(); // button -> { svg, timer }
function clearButtonFx(btn) {
  const cur = fxOnButton.get(btn);
  if (!cur) return;
  clearTimeout(cur.timer);
  cur.svg.remove();
  btn.classList.remove('x-fxing');
  fxOnButton.delete(btn);
}
function buttonFx(btn, kind) {
  if (!btn || !btn.isConnected || !Object.hasOwn(FX_SHAPE, kind) || !fullMotion()) return false;
  clearButtonFx(btn);
  const svg = fxIcon(kind, 30);
  svg.classList.add('x-btnfx');
  btn.classList.add('x-fxing');
  btn.appendChild(svg);
  fxOnButton.set(btn, { svg, timer: setTimeout(() => clearButtonFx(btn), FX_HOLD_MS) });
  return true;
}
// What was pressed last (the data-act and the id of the row), and the icon that is waiting for the page to be drawn again: an action redraws the page after it has said what it did, which
// replaces the button, so the icon is played on the button that is there when the redraw is done (act()).
let lastPress = null, fxPending = null;
// The toast of a fill, a refill, a refuel or a repair. name is the data-act of the action, and the icon comes from it (FX_OF), never from the result. wrote: something was written. When it
// was, the result is a success (green, with the icon, even where the rest was already full); when nothing was, it is an amber notice with no icon (level 'note', or 'part' for one that stays).
function fillToast(name, wrote, msg, level = 'note') {
  if (!wrote) { toast(msg, level); return; }
  toast(msg, false, undefined, { kind: FX_OF[name] });
  fxPending = { act: name, id: lastPress && lastPress.act === name ? lastPress.id : undefined };
}
function playFx() {
  const p = fxPending;
  fxPending = null;
  if (!p || !FX_OF[p.act]) return;
  const btn = [...document.querySelectorAll('[data-act="' + p.act + '"]')].find((b) => p.id === undefined || b.dataset.id === p.id);
  if (btn) buttonFx(btn, FX_OF[p.act]);
}
// What a give did, in words: okText when all of it went in, else how much did and why not more (a give puts in as much as fits). The answer carries requested, given and reason.
function giveToast(r, name, okText) {
  const n = (v) => Number(v).toLocaleString();
  const note = r && r.volumeNote ? ' ' + r.volumeNote : ''; // e.g. "Volume was not checked: the editor has no volume for this item."
  if (r && Number(r.given) < Number(r.requested)) toast(`Gave ${n(r.given)} of ${n(r.requested)} ${name}: ${r.reason || 'not all of it fitted'}.` + note + NOT_SAVED, 'part');
  else toast(okText + note + NOT_SAVED);
}
// An edit of the save is written to the game's file as it is made, so a message has nothing to add; act() adds a hint when it finds the edit queued instead (the game is
// in a single-player session and rewrites the file). NOT_SAVED is that hint for the messages that are built by hand: it reads as the hint while edits are being queued
// (the last status said so) and as nothing otherwise.
const QUEUED_HINT = ' (queued: written as soon as the save can be written)';
// The one sentence that says a blood-sack fill is not proven in the game. It is in the tooltip and in About this, and in the result of every fill that changed a sack, because the fill does not ask first (CR-4).
const BLOOD_UNCHECKED = ' The fill of blood sacks has not been checked in the game yet.';
// A disabled button cannot take the keyboard focus, so a reason that is only in its title is never reached from the keyboard or by a screen reader (CR-5). The reason is drawn as text
// next to the control, and the control is described by it: describedBy(id) is that attribute for a control that is off, and nothing for one that is on. The title stays for the pointer.
const describedBy = (id) => (id ? html`aria-describedby="${id}"` : '');
const NOT_SAVED = { toString: () => (lastOutcome ? (lastOutcome === 'queued' ? QUEUED_HINT : '') : saveBlocked || lastDirty ? QUEUED_HINT : '') };
// What Fill hydration says (#371). It edits the save. When the file can be written the edit is in it at once; while the game runs the edit is only queued and is applied
// when the game closes and loads the save again, so it cannot refill a character who is thirsty now. That case is said plainly and is not a success message: it stays
// on the screen until it is dismissed and it names what to do.
function hydrationMessage(r) {
  const was = Math.round(r.before * 10) / 10;
  if (String(NOT_SAVED) === '') return { text: `Hydration ${was} → ${r.after}`, warn: false };
  return { warn: true, text: `Fill hydration is queued, not applied. The game is running, so the water changes only after the game closes and loads this save again (${was} → ${r.after} then). It will not help right now: a character who is thirsty now needs another source of water now.` };
}
// immediate: the change is written at once (a config file, a setting) and never waits in the queue, so it never gets the hint.
async function act(fn, okMsg, refresh = true, immediate = false, toastOpts = null) {
  try {
    lastOutcome = null; // an older edit's outcome must not colour this message
    const r = await fn();
    let dirty = false;
    if (lastOutcome) dirty = lastOutcome === 'queued'; // the answer to the edit said which
    else if (!immediate) try { dirty = (await status()).dirty; } catch (e) { /* the poll reports a lost connection; the edit itself succeeded */ }
    if (okMsg) toast(okMsg + (dirty ? QUEUED_HINT : ''), false, undefined, null, toastOpts);
    if (refresh) await render(true);
    return r;
  }
  catch (e) { fxPending = null; toast(e.message, true, undefined, null, toastOpts); }
  finally { playFx(); }
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
  if (!failed) playMark(t.closest('tr') || t, 'd-flash', 1100); // the change was applied: its row lights up and fades (Full only)
}
const val = (id) => $('#' + id).value;

// ---------- item names: one catalog, one picker, used by every place that asks for or shows an item
let CATALOG = null; // { list: [{ id, name, kind, label }], byLabel: Map(lower label -> id), byId: Map(id -> name), kindOf: Map(id -> 'item' | 'schematic'), both: Set(names with an item and a schematic) }
let EXTRA_IDS = new Set(); // ids in this save that the catalog lacks
// An item and its schematic are two catalog entries with one name (the catalog files the schematic under the category "schematics"). The picker row of a
// schematic says so, so that the two rows are never alike: "Item name" and "Item name (schematic)". Several of one kind with one name carry their template id too.
const itemKind = (it) => (it.category === 'schematics' ? 'schematic' : 'item');
const pickerLabel = (name, kind, id, ambiguous) => (kind === 'schematic' ? (ambiguous ? `${name} (schematic, ${id})` : `${name} (schematic)`) : (ambiguous ? `${name} (${id})` : name));
async function loadCatalog(extraIds = []) {
  if (!CATALOG) {
    const items = await api('/api/catalog/items');
    const count = new Map(), nameCount = new Map(), kinds = new Map();
    const idsLower = new Map(items.map((it) => [it.id.toLowerCase(), it.id]));
    for (const it of items) {
      const k = itemKind(it);
      count.set(k + '|' + it.name, (count.get(k + '|' + it.name) || 0) + 1);
      nameCount.set(it.name, (nameCount.get(it.name) || 0) + 1);
      if (!kinds.has(it.name)) kinds.set(it.name, new Set());
      kinds.get(it.name).add(k);
    }
    // A name is ambiguous when several items of the same kind share it, or when it spells another item's id (a typed id must win).
    const ambiguous = (it) => count.get(itemKind(it) + '|' + it.name) > 1 || (idsLower.has(it.name.toLowerCase()) && idsLower.get(it.name.toLowerCase()) !== it.id);
    const list = items.map((it) => ({ id: it.id, name: it.name, kind: itemKind(it), label: pickerLabel(it.name, itemKind(it), it.id, ambiguous(it)) }));
    const byLabel = new Map(list.map((e) => [e.label.toLowerCase(), e.id]));
    // a schematic whose name no other entry has is still found by the bare name, as before the rows said what they are
    for (const e of list) if (e.kind === 'schematic' && nameCount.get(e.name) === 1 && !byLabel.has(e.name.toLowerCase())) byLabel.set(e.name.toLowerCase(), e.id);
    CATALOG = { list, byLabel, byId: new Map(list.map((e) => [e.id, e.name])), kindOf: new Map(list.map((e) => [e.id, e.kind])), both: new Set([...kinds].filter(([, v]) => v.size > 1).map(([n]) => n)) };
  }
  // ids that exist in this save but not in the catalog stay selectable by their id
  const extra = extraIds.filter((id) => id && !CATALOG.byId.has(id)).map((id) => ({ id, label: id }));
  EXTRA_IDS = new Set(extra.map((e) => e.id));
  return { list: extra.length ? CATALOG.list.concat(extra) : CATALOG.list };
}
const itemName = (id) => (CATALOG && CATALOG.byId.get(id)) || id;
// The name for the queued line and the result of a give: a schematic says so, and an item that also has a schematic says it is the item.
function itemKindName(id) {
  const n = itemName(id), k = CATALOG && CATALOG.kindOf.get(id);
  return k === 'schematic' ? n + ' (schematic)' : k === 'item' && CATALOG.both.has(n) ? n + ' (item)' : n;
}
const isSchematicId = (id) => !!(CATALOG && CATALOG.kindOf.get(id) === 'schematic');
// An item cell: the in-game name, with the template id on hover.
const itemCell = (id) => html`<span title="${id}">${itemName(id)}</span>`;
// The picker: a text box with a drop-down of in-game names. Each option also carries its template id as its label, so
// typing either the name or the id finds it; a name, a label or a raw template id are all accepted.
// wide: the browser's drop-down is as wide as its box, so the box that is to show whole names (and the kind of the row) is made wide; it never exceeds the screen.
function itemPicker(inputId, cat, placeholder = 'Item name, e.g. Spice Melange', wide = false) {
  return html`<input id="${inputId}" list="${inputId}-dl" aria-label="${placeholder}" placeholder="${placeholder}" size="34"${wide ? html` class="pick-wide"` : ''}><datalist id="${inputId}-dl">${cat.list.map((e) => html`<option value="${e.label}" label="${e.id}"></option>`)}</datalist>`;
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
// The Give item row (picker gt, box gaug, slots ga, grade gag) and the care package editor's row (kitItem, kitAug, ka, kag) use the same slots.
// keep: the values the controls of the block had before the page was drawn again (the snapshot of a redraw, { '#ga0|0': [value, default], ... }); they are put back once the block is drawn (#1148).
async function showGiveAugments(pick = 'gt', boxId = 'gaug', prefix = 'ga', gradeId = 'gag', keep = null) {
  const box = $('#' + boxId), t = pickedItemId(pick), seq = ++augSeq;
  if (!box) return;
  setHTML(box, html``);
  if (!t) return;
  try {
    const fit = await api('/api/items/augment-options', { template_id: t });
    fit.options.forEach((o) => AUG_NAMES.set(o.id, o.name));
    if (seq !== augSeq || !fit.limit || !$('#' + boxId)) return;
    setHTML($('#' + boxId), augBlock(prefix, fit, [], gradeId));
    if (keep) for (const e of $('#' + boxId).querySelectorAll('select')) { const c = keep['#' + e.id + '|0']; if (c && [...e.options].some((o) => o.value === c[0])) e.value = c[0]; }
  } catch (e) { /* an unknown item simply has no augment slots */ }
}
// The Augments block is drawn by showGiveAugments when the item picker changes, never by the card itself, so a redraw of the page (the page's own refresh when the game saved, #696; a redraw after
// another action) gave back the picker with its item and an empty block, and the slots and the grade were lost (#1148). After a redraw that keeps what was entered, the block is drawn again for the
// item that is in the picker and its controls get back what they held. Nothing is asked of the server but the item's augment options, as when the picker changes; nothing is written.
const AUG_ROWS = [['gt', 'gaug', 'ga', 'gag'], ['kitItem', 'kitAug', 'ka', 'kag']]; // Give item, and the care package editor's item row
function restoreAugBlocks(place) {
  if (!place || place.tab !== tab) return;
  for (const [pick, box, prefix, gradeId] of AUG_ROWS) if ($('#' + box) && $('#' + pick) && pickedItemId(pick)) showGiveAugments(pick, box, prefix, gradeId, place.ctl);
}
const augName = (id) => (AUG_NAMES.get(id) || id);
const AUG_NAMES = new Map(); // augment id -> name, filled from the options the server sent
const augLabel = (o) => (o.effects && o.effects.length ? `${o.name} (${o.effects.join('; ')})` : o.name);
const augSlots = (prefix, fit, applied) => html`${Array.from({ length: fit.limit }, (_, i) => html`<label>Slot ${i + 1} <select id="${prefix}${i}"><option value="">(none)</option>${fit.options.map((o) => html`<option value="${o.id}" ${applied[i] === o.id ? 'selected' : ''}>${augLabel(o)}</option>`)}</select></label> `)}`;
// One layout for both places that draw the pair (#1136): a heading row with the one grade of the added augments beside it, then one slot per line under it, all the same width.
const augBlock = (prefix, fit, applied, gradeId) => html`<div class="augbox"><div class="aug-head"><b>Augments</b>${augGradeSel(gradeId)}</div><div class="aug-slots">${augSlots(prefix, fit, applied)}</div></div>`;
const augPicked = (prefix, n) => Array.from({ length: n }, (_, i) => { const e = $('#' + prefix + i); return e ? e.value : ''; }).filter(Boolean);
const augGradeSel = (id) => html`<label>Grade of added augments <select id="${id}">${[1, 2, 3, 4, 5].map((n) => html`<option ${n == 5 ? 'selected' : ''}>${n}</option>`)}</select></label>`;
// ---------- tabs
const TABS = { player: 'Player', livemap: 'Saved map', landsraad: 'Landsraad', config: 'Config', db: 'Database', help: 'Help' };
// The Player tab mirrors the Dune Docker console's Players > Player Name view: the same tabs in the same order (#96). Bases and
// Vehicles used to be top-level tabs and are two of these.
const PTABS = [['character', 'Character'], ['crafting', 'Crafting'], ['research', 'Research'], ['buildingsets', 'Building sets'], ['customizations', 'Customizations'],
  ['skills', 'Skills'], ['specialization', 'Specialization'], ['journey', 'Journey'], ['bases', 'Bases'], ['vehicles', 'Vehicles'], ['admin', 'Admin']];
// The browser's storage is a convenience (the tab last open): a browser that blocks it, or a private window, throws on the first touch, which must not stop the page from starting.
const browserStore = {
  get(k) { try { return localStorage.getItem(k); } catch (e) { return null; } },
  set(k, v) { try { localStorage.setItem(k, v); } catch (e) { /* the choice lasts until the page is closed */ } },
  remove(k) { try { localStorage.removeItem(k); } catch (e) { /* nothing was kept */ } },
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

// ---------- text that is not shown all the time (#426, docs/design/2026-10-26-ui-text-sweep.md)
// A permanent line must earn its place: the state of a control, a result, an error, what is queued and the one thing that can hurt stay on the page. An explanation, a limit or
// background goes in one of two places. tip() puts the sentence or two that belong to ONE control in a tooltip that shows on hover and on keyboard focus; the control carries
// aria-describedby="<id>" so a screen reader reads it as the control's description whether or not it is drawn (the caller writes the attribute; a test checks that every tip has one).
// about() puts what belongs to a card as a whole in a closed "About this" disclosure; whether it is open is kept per card in the browser's storage.
function tip(id, text, control) { return html`<span class="tip">${control}<span class="tipbox" role="tooltip" id="${id}">${text}</span></span>`; }
const aboutKey = (id) => 'about.' + id;
// What a decree row is in the current term (#448): the term's active decree, its elected decree, a decree in the rotation; read from the term and the rotation by the decree id.
function about(id, title, content) { return html`<details class="about" data-about="${id}" ${browserStore.get(aboutKey(id)) === '1' ? 'open' : ''}><summary>About this<span class="sr"> (${title})</span></summary>${content}</details>`; }
document.addEventListener('toggle', (e) => {
  const d = e.target;
  if (d && d.matches && d.matches('details.about[data-about]')) { if (d.open) browserStore.set(aboutKey(d.dataset.about), '1'); else browserStore.remove(aboutKey(d.dataset.about)); }
}, true);
// Where a tooltip goes (#536). A tooltip is drawn with position:fixed, so the scroll box a table sits in neither clips it nor counts it in its scroll size, and nothing moves when
// it opens. This pure function computes where: all numbers are in viewport pixels. anchor is the control's rectangle {left, top, right, bottom}, box the tooltip's size {width, height},
// view the window {width, height}. The tooltip goes below the control (overlapping its edge by 2 px, so the pointer can travel into the tooltip), flips above when it would run
// past the bottom and there is more room above, and is clamped to the window on every side (margin 8 px). A tooltip taller than the window is cut by the CSS max-height.
function placeTip(anchor, box, view, margin = 8, overlap = 2) {
  const maxLeft = Math.max(margin, view.width - box.width - margin);
  const left = Math.min(Math.max(anchor.left, margin), maxLeft);
  const below = anchor.bottom - overlap, above = anchor.top + overlap - box.height;
  const roomBelow = view.height - margin - below, roomAbove = anchor.top + overlap - margin;
  const flip = box.height > roomBelow && roomAbove > roomBelow;
  let top = flip ? above : below;
  top = Math.min(Math.max(top, margin), Math.max(margin, view.height - box.height - margin));
  return { left, top, side: flip ? 'above' : 'below' };
}
// One tooltip is open at a time (hover or keyboard focus on its control). It is placed from the rectangle of its wrapper when it opens and put away on blur, on Escape and
// when anything scrolls or the window is resized (it would no longer be next to its control). Escape puts it away without moving the focus or the pointer; it comes back
// when the pointer or the focus leaves and returns.
let tipOpen = null;
function closeTip() { if (tipOpen) { tipOpen.classList.remove('open'); tipOpen = null; } }
function openTip(t) {
  if (tipOpen === t || t.classList.contains('closed')) return;
  closeTip();
  const box = t.querySelector('.tipbox');
  if (!box) return;
  t.classList.add('open');
  tipOpen = t;
  box.style.left = '0px'; box.style.top = '0px'; // measured at the corner, where it has the most room to wrap
  const r = t.getBoundingClientRect(), s = box.getBoundingClientRect(), de = document.documentElement;
  const p = placeTip(r, { width: s.width, height: s.height }, { width: de.clientWidth || window.innerWidth, height: de.clientHeight || window.innerHeight });
  box.style.left = p.left + 'px'; box.style.top = p.top + 'px';
}
const tipOf = (e) => (e.target && e.target.closest ? e.target.closest('.tip') : null);
document.addEventListener('mouseover', (e) => { const t = tipOf(e); if (t) openTip(t); });
document.addEventListener('focusin', (e) => { const t = tipOf(e); if (t) openTip(t); });
document.addEventListener('keydown', (e) => {
  if (e.key !== 'Escape') return;
  document.querySelectorAll('.tip').forEach((t) => { if (t.contains(document.activeElement) || t.matches(':hover') || t === tipOpen) t.classList.add('closed'); });
  closeTip();
});
const leaveTip = (e) => {
  const t = tipOf(e);
  if (!t || t.contains(e.relatedTarget)) return;
  t.classList.remove('closed');
  if (t === tipOpen && !(e.type === 'mouseout' && t.contains(document.activeElement)) && !(e.type === 'focusout' && t.matches(':hover'))) closeTip();
};
document.addEventListener('focusout', leaveTip);
document.addEventListener('mouseout', leaveTip);
window.addEventListener('scroll', (e) => { if (tipOpen && !(e.target && e.target.nodeType === 1 && tipOpen.contains(e.target))) closeTip(); }, true);
window.addEventListener('resize', closeTip);

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
  if (!t || t.closest('.lcc') || !t.tHead || !t.tBodies[0] || !th.textContent.trim()) return false; // a list of the Command Center sorts by its own rule (lcc-model.js), never by what its cells say
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
    if (t.classList.contains('sortd') || t.closest('.lcc') || !t.tHead || !t.tHead.rows[0]) continue;
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
// The fill level of a liquid container (read-only; the server works it out from the item's stats). Empty for an item that holds no liquid.
function fillCell(r) {
  const f = r.fill;
  if (!f) return '';
  const lit = (n) => Number(n).toLocaleString();
  const what = f.kind || 'Liquid';
  if (f.selfFilled) return html`<span class="mut" title="The game puts the water the suit recovers into it. Fill leaves it alone.">${what} ${lit(Math.round(f.amount))} / ${lit(f.capacity)} (${f.percent}%), the suit fills itself</span>`;
  if (f.state == 'empty') return html`<span class="mut">${what} empty</span>`;
  if (f.capacity == null) return html`<span class="warn" title="The capacity of this container is not known to the editor.">${what} ${lit(f.amount)} (capacity unknown)</span>`;
  return html`<span class="${f.state == 'full' ? 'ok' : f.state == 'over' ? 'warn' : ''}" title="${f.state == 'over' ? 'More than the capacity the editor knows.' : ''}">${what} ${lit(f.amount)} / ${lit(f.capacity)} (${f.percent}%)</span>`;
}
// The card header's count of liquid containers in a list of items, and how many of them are not full.
function fillTotals(items) {
  const c = items.filter((r) => r.fill && !r.fill.selfFilled); // a worn stillsuit is not a container the person fills
  if (!c.length) return '';
  const notFull = c.filter((r) => r.fill.state == 'empty' || r.fill.state == 'partial').length, unk = c.filter((r) => r.fill.state == 'unknown').length;
  return `${nn(c.length, 'liquid container', 'liquid containers')}, ${notFull} not full${unk ? `, ${unk} of unknown capacity` : ''}.`;
}
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
// The open file is not the game's own save (a copy): said in the header, the tab title and the banner, so a copy is never taken for the live save (#498). The native window's title says the same.
const NOT_LIVE = 'NOT LIVE: a copy';
let notLive = false;
let gwKey = '', lastPending = []; // the banner as last drawn, and the queued changes in words
let lastQueued = null, lastWriteAt = null; // how many changes waited at the last poll, and when the last write that went through was made (null: no poll yet)
const KEEP = { qlist: false, raw: false, vcparts: false }; // which of the banner's toggles are open: kept across redraws
// The reasons the editor gives for not writing, as a sentence. The wording the tools and checks use stays available under Details.
const capFirst = (s) => String(s || '').replace(/^\s*(.)/, (m, c) => c.toUpperCase());
const endSentence = (s) => (/[.!?]$/.test(s) ? s : s + '.');
function humanReason(raw) {
  const r = String(raw || '').trim();
  const L = r.toLowerCase();
  const map = (text) => ({ text, raw: r });
  if (L.startsWith('could not check whether the game is running')) return map('Could not tell whether Dune: Awakening is running.');
  if (L.includes('cannot tell whether the game is using it')) return map('This save is on a network share or a removable drive, so the editor cannot tell whether the game is using it.');
  if (L.includes('cannot be sure that no other program or computer is writing this file')) return map('This file is on a network share or a removable drive, so the editor cannot be sure that no other program or computer is writing it.');
  if (L.includes('the game saved while the edits were being written')) return map('The game saved while the changes were being written; they are put back onto its file next.');
  return { text: endSentence(capFirst(r)), raw: '' };
}
// The name of a Journey step for a queue line (#618): the browser's own list has it once the Journey tab has been opened; until then the line has no name and the step's id is in its tooltip (queueTip).
function journeyStepName(id) {
  if (!JB) return '';
  for (const g of JGROUPS) { const r = (JB[g[0]] || []).find((x) => x.id === id); if (r) return r.name; }
  return '';
}
// A queued Give of several items is one edit: "Give 2 items: 3 x A (grade 1); 1 x B (grade 5, augments: C at grade 3)". Its items, one to a line, in the banner list (#1149); null for any other edit.
function queueGiveItems(desc) {
  const m = String(desc || '').match(/^Give (\d+) items(?: \([^)]*\))?: (.+)$/);
  return m ? m[2].split('; ') : null;
}
// The id that a queue line leaves out, for its tooltip (#618). A queued Give of several items says all of them in its tooltip too, on one line (#1149).
function queueTip(desc) {
  let m;
  if (queueGiveItems(desc)) return String(desc);
  if ((m = String(desc || '').match(/^journey (\S+) (?:completed|reset) \(/))) return `Journey step id ${m[1]}`;
  if ((m = String(desc || '').match(/^moved vehicle (\d+) next to player$/))) return `Vehicle number ${m[1]}`;
  return '';
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
  if ((m = d.match(/^decree (\d+) disabled=([01])$/))) return `Set the pool flag of decree ${m[1]} to ${m[2]}`;
  if ((m = d.match(/^journey (\S+) (completed|reset) \((\d+) rows?\)$/))) { const jn = journeyStepName(m[1]); return jn ? `Journey step ${jn}: ${m[2]}` : `Journey step: ${m[2]}`; }
  if ((m = d.match(/^moved vehicle (\d+) next to player$/))) return 'Move a vehicle next to the character';
  // Landsraad edits name their houses and factions (#611): several houses, then one house
  // "landsraad: 3 houses completed for Atreides; replaces ...; houses: A, B, C": what was done and the short facts first, the list of houses last
  if ((m = d.match(/^landsraad: (\d+) houses? (completed|reopened|changed from the board)(?: for ([^;]+))?((?:; .*)?)$/))) {
    // the list of houses is the last "houses: " part: a name that holds the marker itself does not cut the line short (#775)
    const at = m[4].lastIndexOf('; houses: '), hm = at >= 0 ? { index: at, 1: m[4].slice(at + 10) } : null, facts = hm ? m[4].slice(0, hm.index) : m[4], n = +m[1];
    const verb = m[2] === 'completed' ? 'Complete' : m[2] === 'reopened' ? 'Reopen' : 'Change';
    return `${verb} ${n} Landsraad ${n === 1 ? 'house' : 'houses'}${m[2] === 'changed from the board' ? ' from the board' : ''}${m[3] ? ' for ' + m[3] : ''}${hm ? ': ' + hm[1] : ''}${facts}`;
  }
  if ((m = d.match(/^landsraad (?!term |task )([^:]+): (.+)$/))) return `${m[1]} (Landsraad): ${capFirst(m[2])}`;
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
    const r = await api('/api/save/discard', { ifClean: true });
    if (r && r.kept) return; // an edit was made while the status was being read: nothing is thrown away
    await render(true, true); // the page's own redraw: the focus goes back, the page is not moved (#696)
  } catch (e) { /* the next poll shows the banner and tries again */ } finally { autoBusy = false; }
}
// A write that went through since the last poll: a check is drawn by the status bar for a moment (the words stay with the check, so Off still says it).
let writtenTimer = 0;
function showWritten() {
  const m = $('#savedmark');
  if (!m) return;
  m.hidden = false; // shown again, the check is drawn again
  clearTimeout(writtenTimer);
  writtenTimer = setTimeout(() => { m.hidden = true; }, 3000);
}
async function status() {
  const o = await api('/api/save/state');
  offline = false;
  saveBlocked = !!o.saveBlocked;
  lastDirty = !!o.dirty;
  saveBlockWhy = o.blockReason || '';
  $('#path').textContent = String(o.path || '').split(/[\\/]/).pop();
  notLive = o.live === false;
  $('#savename').hidden = !$('#path').textContent;
  $('#path').title = (notLive ? "Not the game's save: " : '') + (o.path || '');
  const nl = $('#notlive');
  nl.classList.toggle('hide', !notLive);
  nl.textContent = notLive ? NOT_LIVE : '';
  nl.title = notLive ? "This file is not the game's save. Edits are written at once and do not reach the game unless it is installed as its game.db." : '';
  if (document.title !== pageTitle()) document.title = pageTitle();
  const gw = $('#gamewarn');
  const q = o.pending.length;
  const last = o.lastWrite;
  const why = (last && !last.saved && last.why) || saveBlockWhy;
  // When the game saves and nothing is queued, the editor reloads the file by itself (autoRefresh below), so the out-of-date banner is only for automatic refresh switched
  // off. Queued edits are put back onto the game's newer file when they are written, so a changed file needs no word from the person.
  const changed = !!o.changedOnDisk && !q && !autoRefreshOn();
  const restored = o.restored && (o.restored.applied > 0 || (o.restored.conflicts || []).length > 0) ? o.restored : null;
  // Some table is not defined as the editor expects (the game was probably updated): said at open and kept, editing continues.
  const schemaNote = o.schemaNote ? String(o.schemaNote) : '';
  // Waiting to be written with nothing listed (#592): said as a plain fact. There is nothing to discard, so no Discard button is offered for it.
  const unlisted = !!o.dirty && !q && !o.held;
  const refill = o.autoRefill && o.autoRefill.waiting ? o.autoRefill : null; // an automatic refill that waits for the game (#1123); listed below with the queue
  gw.className = saveBlocked || q || refill || unlisted || changed || restored || o.writeLogProblem || o.notesProblem || o.recordNotice ? 'warn' : schemaNote || notLive ? 'info' : 'hide';
  const gameFile = notLive ? 'the file' : 'the game file';
  const notLiveMsg = notLive ? html`<div><b>This is a copy, not the game's save:</b> edits are written at once and do not reach the game unless this file is installed as its game.db. <span class="mono" title="${o.path}">${String(o.path || '').split(/[\\/]/).pop()}</span></div>` : html``;
  const changedMsg = changed ? html`<div><b>The game saved since this editor opened.</b> What you see here is out of date. <button class="b sm d-btn primary sm" data-act="reload">Reload now</button></div>` : html``;
  // One banner for the whole state. The reason is said once, as a sentence; the tool's own text, when the sentence replaces it, sits under Details.
  // An edit is written as it is made. While the file cannot be written (a single-player session is running, the location is not confirmed) edits are queued, and the queue is
  // written by itself the moment the file can be.
  const reason = why ? humanReason(why) : { text: '', raw: '' };
  const n = (one, many) => (q === 1 ? one : many);
  // a queue the editor could not put back onto the game's newer file: none of it was written
  const unlistedMsg = unlisted ? html`<div><b>Unlisted changes are waiting to be written.</b> They are written by themselves as soon as the file can be written. <button class="b sm d-btn primary sm" data-act="writeNow">Write now</button></div>` : html``;
  const heldMsg = o.held ? html`<div><b>${q} queued change${q > 1 ? 's were' : ' was'} not written.</b> The game saved changes that conflict with ${q > 1 ? 'them' : 'it'}, so the editor left the game's file as it is. Discard the queue and make the change${q > 1 ? 's' : ''} again.
    ${(o.held.conflicts || []).map((c) => html`<div>${queueLine(c.desc)}: ${c.why}</div>`)} <button class="b sm d-btn primary sm" data-act="discardQueue">Discard queue</button></div>` : html``;
  let mainMsg = html``;
  if (o.locationUnconfirmed) mainMsg = html`<div><b>Changes cannot be written here yet.</b> ${reason.text} You can keep editing: changes wait in the queue. <button class="b sm d-btn primary sm" data-act="confirmLocation">Allow writing here</button></div>`;
  else if (o.gameCheckError && saveBlocked) mainMsg = html`<div><b>Changes cannot be written.</b> ${reason.text}${q ? html` ${q} ${n('change waits', 'changes wait')} in the queue and ${n('is written by itself', 'are written by themselves')} once the check works.` : ''} The editor checks again by itself every few seconds. <button class="b sm d-btn primary sm" data-act="checkAgain">Check again</button> <button class="b sm d-btn primary sm" data-act="writeNow">Write now</button></div>`;
  else if (q && !o.held && last && last.stuck) mainMsg = html`<div><b>${q} queued change${n(' cannot', 's cannot')} be written.</b> The game saved after ${n('it was', 'they were')} made, and the editor did not keep ${n('it', 'them')}${o.journalOff ? ' (' + o.journalOff + ')' : ''}, so ${n('it', 'they')} cannot be put back onto the game's file, which is left as it is. Discard the queue and make ${n('the change', 'the changes')} again. <button class="b sm d-btn primary sm" data-act="discardQueue">Discard queue</button></div>`;
  else if (q && !o.held) mainMsg = html`<div><b>${q} change${n(' is', 's are')} queued.</b> ${reason.text} ${n('It is written to ' + gameFile + ' by itself', 'They are written to ' + gameFile + ' by themselves')} as soon as it can be written. <button class="b sm d-btn primary sm" data-act="writeNow">Write now</button></div>`;
  else if (saveBlocked && !q) mainMsg = html`<div><b>Changes you make now are queued.</b> ${reason.text} They are written by themselves as soon as that clears.</div>`;
  // what became of the edits an earlier run (or a reload) put back, and the plain fact when they are not being kept
  // A change that could not be put back is dropped for good: its notice stays, even after the others are written, until it is dismissed.
  const restoredHead = !restored ? html`` : restored.applied > 0
    ? html`<b>${restored.applied} change${restored.applied === 1 ? ' was' : 's were'} put back${restored.rebased ? ' onto the newer file the game saved' : ''}.</b> ${restored.written ? (restored.applied === 1 ? 'It has been written.' : 'They have been written.') : 'They are written like any other change.'}`
    : html`<b>${(restored.conflicts || []).length === 1 ? 'A queued change was' : 'Queued changes were'} not kept.</b>`;
  const restoredMsg = restored ? html`<div>${restoredHead}
    ${(restored.conflicts || []).map((c) => html`<div>Could not put back: ${queueLine(c.desc)} (${c.why}). Make it again.</div>`)} <button class="b sm d-btn primary sm" data-act="dismissRestored">OK</button></div>` : html``;
  const journalMsg = o.journalOff && o.dirty ? html`<div class="mut">These queued changes are not kept if the editor is closed: ${o.journalOff}.</div>` : html``;
  const schemaMsg = schemaNote ? html`<div><b>This save is not quite the one this editor knows.</b> ${schemaNote}</div>` : html``;
  const logMsg = o.writeLogProblem ? html`<div class="mut">${capFirst(o.writeLogProblem)}. The game's file was written; only the record of it is missing.</div>` : html``;
  const notesMsg = o.notesProblem ? html`<div class="mut">${capFirst(o.notesProblem)}. The game's file was written; only the editor's own record of the ground a Landsraad raise paid is behind.</div>` : html``;
  // the editor's own record of the Landsraad ground it paid was not usable and the old file was kept aside (#838): the file is named, the folder is the editor's own
  const recordMsg = o.recordNotice ? html`<div class="mut">${capFirst(o.recordNotice)}. The game's file is not touched; a raise now counts only the ground the new record holds.</div>` : html``;
  const lines = o.pending.map(queueLine);
  // An automatic refill that waits for the game (#1123) is a job of its own, not an edit: it is listed with the edits (what it is, what it waits for, a way to cancel it) and counted by the chip, but it is not one of the
  // q queued changes (the banner sentences above count edits) and Discard queue does not touch it.
  const nq = q + (refill ? 1 : 0);
  const refillWhy = refill ? (refill.why ? capFirst(refill.why) : 'The game may still hold the save') + '.' : '';
  const refillLi = refill ? html`<li class="bn-refill"><b>Automatic refill.</b> The jobs switched on in Settings, run as one edit. ${refillWhy} It runs by itself, on the file as the game left it, as soon as that clears. <button class="b sm d-btn sm" data-act="cancelRefill" title="Cancel this refill. Nothing is written and no setting changes; the editor asks again the next time it opens.">Cancel refill</button></li>` : html``;
  const listMsg = nq ? html`<details class="bn-list" ${KEEP.qlist ? 'open' : ''}><summary data-keep="qlist">What is queued (${nq})</summary><ul>${lines.map((l, i) => { const gi = queueGiveItems(o.pending[i]); return html`<li title="${queueTip(o.pending[i])}">${gi ? html`${l.slice(0, l.indexOf(':') + 1)}${gi.map((x) => html`<br>${x}`)}` : l}</li>`; })}${refillLi}</ul></details>` : html``;
  const rawMsg = reason.raw ? html`<details class="bn-raw" ${KEEP.raw ? 'open' : ''}><summary data-keep="raw">Details (include this if you report a problem)</summary><div class="mono">${reason.raw}</div></details>` : html``;
  lastPending = lines;
  const banner = html`${notLiveMsg}${changedMsg}${heldMsg}${mainMsg}${unlistedMsg}${restoredMsg}${schemaMsg}${journalMsg}${logMsg}${notesMsg}${recordMsg}${listMsg}${rawMsg}`;
  // redrawn only when it changed: a redraw every few seconds would take the keyboard focus off its buttons
  const key = String(banner).replace(/ ?open>/g, '>').replace(/ >/g, '>'); // a toggle the person opened is already open in the page: that alone is no reason to redraw
  if (key !== gwKey) { gwKey = key; setHTML(gw, banner); }
  $('#pending').textContent = nq ? `${nq} queued` : '';
  $('#pending').setAttribute('aria-label', nq ? `${nq} queued, show the queued changes` : 'Show the queued changes'); // starts with the visible words and carries the count (#619 D9-2)
  $('#pending').title = nq ? lines.concat(refill ? ['Automatic refill: ' + refillWhy] : []).join('\n') + '\n(click to open the list)' : '';
  if (lastQueued !== null && nq > lastQueued) playMark($('#pending'), 'x-bump', 450); // a change joined the queue: the count bumps once (Full only)
  lastQueued = nq;
  const wroteAt = last && last.saved && last.at ? String(last.at) : '';
  if (wroteAt) { if (lastWriteAt !== null && lastWriteAt !== wroteAt) showWritten(); lastWriteAt = wroteAt; } else if (lastWriteAt === null) lastWriteAt = '';
  $('#btnDiscard').hidden = !(q > 0 || o.held); // only when something is listed to drop (or a held queue); a queue with nothing listed has the notice above instead
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
// close: true makes it a notice with one Close button (the About panel); listLabel and preLabel name the list and the text block for a screen reader.
function ask({ title, body, pre = '', list = [], ok = 'Confirm', typed = '', danger = false, gap = false, close = false, listLabel = 'The changes', preLabel = 'The statement' }) {
  return new Promise((resolve) => { if (askState) askState.resolve(false); askFrom = askFrom || document.activeElement; askState = { title, body, pre, list, ok, typed, danger, gap, close, listLabel, preLabel, at: Date.now(), resolve }; drawAsk(); });
}
// A question opened with gap: true ignores an OK that comes sooner than ARM_GAP_MS after it opened (the second half of a double click on the button that opened it, a key that repeats).
const tooSoon = () => !!(askState && askState.gap && Date.now() - askState.at < ARM_GAP_MS);
// While a question is open the page behind it is inert (not reachable by Tab, by a screen reader's browse keys or by a click) and does not scroll.
const BEHIND = ['header', '#gamewarn', '#launchpass', '#rowarn', '#nav', '#main'];
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
  setHTML(el, html`<div class="mbox d-panel small" role="dialog" aria-modal="true" aria-labelledby="atitle" aria-describedby="abody"><h3 class="big" id="atitle" tabindex="-1">${a.title}</h3><p id="abody">${a.body}</p>${a.list.length ? html`<ul class="qlist" tabindex="0" aria-label="${a.listLabel}">${a.list.map((l) => html`<li>${l}</li>`)}</ul>` : ''}${a.pre ? html`<pre class="sqlpre" tabindex="0" aria-label="${a.preLabel}">${a.pre}</pre>` : ''}
    ${a.typed ? html`<p class="mut">Type <b>${a.typed}</b> to continue.</p><input id="atype" spellcheck="false" size="20">` : ''}
    <div class="row mt12">${a.close ? html`<button class="b d-btn primary" id="aok" data-act="askNo">Close</button>` : html`<button class="b ${a.danger ? 'bad' : ''} d-btn ${a.danger ? 'danger' : 'primary'}" id="aok" data-act="askOk" ${a.typed ? 'disabled' : ''}>${a.ok}</button><button class="b sec d-btn" data-act="askNo">Cancel</button>`}</div></div>`);
  (a.typed ? $('#atype') : $('#atitle')).focus();
}
function endAsk(v) {
  const a = askState, back = askFrom;
  askState = null; askFrom = null;
  drawAsk();
  if (back && back.isConnected && typeof back.focus === 'function') back.focus(); // return to where the person was
  if (a) a.resolve(v);
}
document.addEventListener('input', (e) => { goKeep(e.target); if (askState && e.target.id === 'atype') $('#aok').disabled = e.target.value.trim().toLowerCase() !== askState.typed.toLowerCase(); });

// ---------- one-button confirmation
// A button with data-confirm destroys or resets something (clear a container, clear sand, put a setting back to its default), so the first press only asks: the same button turns into "Yes, ..." for a few seconds and
// the second press runs it. A top-up (a refill or a repair) never asks: it is written as it is made, the result line says what changed and the backup taken first is the undo. Pressing anything else, leaving the button or pressing Escape cancels. It is inline on purpose (no dialog to trap the keyboard) and the
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
const undoNote = () => `A backup of the save is taken before the change is written to ${notLive ? 'this file' : 'the game file'}, and Restore on the Database tab (Save backups) puts it back.`;
async function resetArea({ count, title, body, list = [], typed, ok, path, extra = {}, done, none }) {
  if (!(await ask({ title, body: `${body} ${undoNote()}`, list, ok, typed, danger: true, gap: true }))) return;
  await act(async () => {
    try {
      const r = await api(path, { expected: count, ...extra });
      toast((r.noop ? none : done(r)) + NOT_SAVED);
    } catch (e) { if (e.code === 'conflict') setTimeout(() => render('scroll'), 0); throw e; }
  }, null);
}

// ---------- the queue: an edit is written to the game's file as it is made; while the file cannot be written (the game is in a single-player session) edits wait here
// the "N queued" chip opens the list of queued changes in the banner (#597); it is hidden when nothing is queued
$('#pending').onclick = () => { KEEP.qlist = true; const d = $('#gamewarn .bn-list'); if (!d) return; d.open = true; d.scrollIntoView({ block: 'nearest' }); d.querySelector('summary').focus(); };
$('#btnDiscard').onclick = async () => { if (await ask({ title: 'Discard the queued changes?', body: `${lastPending.length === 1 ? 'This change is' : lastPending.length > 1 ? `These ${lastPending.length} changes are` : 'The queued changes are'} waiting for the game to close and would be dropped. The game file is not touched.`, list: lastPending, ok: 'Discard', danger: true })) act(() => api('/api/save/discard', {}), 'Discarded', true, true); };

// ---------- clear a container (#338)
// One row under a container's item list: a box for the currency, and a button that asks twice (the two-press rule of data-confirm, then a typed count). The count
// the person types is the number of items the clear will remove, and it is sent with the request: the editor refuses when the container holds another number by then.
const SOLARI = 'SolarisCoin';
function clearRow(inv, name, rows) {
  const all = rows.length, kept = rows.filter((r) => r.template_id === SOLARI).length, plain = all - kept;
  const btn = html`<button class="b bad d-btn danger" data-act="clearInv" data-confirm="Yes, clear this container" data-inv="${inv}" data-name="${name}" data-plain="${plain}" data-all="${all}" aria-describedby="clrnote" ${all ? '' : 'disabled'}>Clear this container</button>`;
  const note = `Removes the ${nn(plain, 'item', 'items')} in ${name}, with everything attached to them. The Solari Coin stack${kept === 1 ? ' is' : 's are'} kept unless the box is ticked. Two presses, then type the count.`;
  // what it does is the tooltip; that there is nothing to clear is state and stays drawn
  return html`<div class="row"><label><input type="checkbox" id="clrCur"> Also remove currency</label>${all ? tip('clrnote', note, btn) : btn}${all ? '' : html`<span class="mut" id="clrnote">${name} is empty: nothing to clear.</span>`}</div>`;
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
  : html`<div class="kv"><b>${label}</b><span>${SHOWN.has(key) ? v : '••••••••'} <button class="b sec sm d-btn sm" data-reveal="${key}" title="Account identifiers are hidden so a screenshot does not share them">${SHOWN.has(key) ? 'Hide' : 'Show'}</button></span></div>`);
function summaryCard(m) {
  return html`<div id="sumcard">${summaryBody(m)}</div>`;
}
function summaryBody(m) {
  if (!m) return html``;
  const num = (v) => (v == null ? '-' : Number(v).toLocaleString());
  const f1 = (v) => (v == null ? '-' : Number(v).toFixed(1));
  const id = m.identity || {}, ids = m.ids || {}, v = m.vitals || {}, sp = m.skillPoints, it = m.intel;
  return html`<div class="card d-panel"><h3 class="d-head">Player summary</h3><div class="grid">
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
// The Command Center view (lcc/lcc-spec.js) is the only view of the Specialization tab (the Classic list was retired, #1105). Every write goes through the handlers below (spAddCC, spMax, spReset, spKeyAll, spKeyReset, spResetAll, spAssign, spAssignHand, specSet).
// A view chosen in an earlier version of the editor (the browser kept it under spec.view) is dropped here: the Command Center view is the default and there is no other view to choose.
browserStore.remove('spec.view');
const SPV = { vm: null, lv: null, stale: false, ls: null, n: 0, lvAt: 0, lsAt: 0, wrAt: 0 }; // lv: the specialization answer as the last visit to the Specializations tab read it, the one source of the levels the Landsraad board shows; stale: a Landsraad write gave specialization XP after it was read, so its levels are no longer the save's (#646); ls: the levels the Landsraad answer itself carries (#1108); lvAt, lsAt, wrAt: when each was read or written, so the newest read after the last write wins
// The levels the board shows come from the newest read that is not older than the last write that gave specialization XP: the Landsraad answer's own specialization_levels (whole numbers by line, read only,
// #1108) or the Specializations tab's answer (#646). A save that holds no track row for a line leaves it out of the Landsraad answer, and the board then shows a dash for that line, never a made-up number.
function spvNoteBoardRead(l) {
  const sl = l && l.specialization_levels, o = {};
  if (sl && typeof sl === 'object') for (const k of Object.keys(sl)) if (Number.isInteger(sl[k]) && sl[k] >= 0) o[k.trim().toLowerCase()] = sl[k];
  SPV.ls = Object.keys(o).length ? o : null; SPV.lsAt = ++SPV.n; if (SPV.ls) SPV.stale = false; // levels after the write are the save's again
}
function spvLevels() {
  const fromTab = SPV.lv && SPV.lv.ok && SPV.lvAt > SPV.wrAt, fromBoard = SPV.ls && SPV.lsAt > SPV.wrAt;
  if (fromTab && (!fromBoard || SPV.lvAt > SPV.lsAt)) { const o = {}; for (const r of SPV.lv.rows) if (r.known && Number.isInteger(r.level)) o[r.key] = r.level; return o; }
  return fromBoard ? { ...SPV.ls } : null;
}
const specCCOn = () => !!(window.LCCS && window.LCC); // false only when the Command Center scripts did not load (specMissing)
const specMissing = () => html`<section class="card d-panel" id="lcs-missing" aria-labelledby="lcs-missing-h"><h2 class="d-head" id="lcs-missing-h" tabindex="-1">Specializations</h2><p class="nodata" role="alert">The Specializations page could not be loaded, so the specializations cannot be shown. Reload the page (Ctrl+F5). If this stays, close the editor and start it again.</p></section>`;
// Opening a House of a specialization on the Landsraad board only moves between pages: the board shows it selected, with its filters cleared, and nothing is written.
function specGoHouse(id) {
  if (!Number.isInteger(id)) return;
  if (window.LCC && window.LCCM && window.LCCB) { const st = window.LCCB.state; st.sel = id; st.show = 'all'; st.hs = 'any'; st.spec = 'all'; }
  tab = 'landsraad'; browserStore.set('tab', tab); askReveal(() => lbRevealHouse(id)); render();
}
// The way back from the Saved map to a House: the Landsraad tab with the House chosen on the board. It only moves between pages and writes nothing (#1060).
function lbOpenHouseHere(id) {
  if (!Number.isInteger(id)) return;
  if (lbBoardOn()) { const st = window.LCCB.state; st.sel = id; st.show = 'all'; st.hs = 'any'; st.spec = 'all'; }
  tab = 'landsraad'; browserStore.set('tab', tab); askReveal(() => lbRevealHouse(id)); render();
}
// And back: the specialization of a House opens that specialization in the Specialization tab's Command Center view. It only moves between pages (the chosen specialization is the view's own state) and writes nothing.
function specGoPath(track) {
  if (!(window.LCCS && window.LCC) || !Array.isArray(window.LCCS.ORDER) || !window.LCCS.ORDER.includes(track)) return;
  window.LCCS.state.sel = track;
  sub.player = 'specialization'; tab = 'player'; browserStore.set('tab', tab); askReveal(specReveal); render();
}
const SPV_HOST = { goHouse: specGoHouse };
// The lanes and the chosen specialization's dossier are below the page's header and totals. A link that opens a specialization (specGoPath) brings them into view once the page is drawn (askReveal). The sticky header is cleared by the page's scroll-padding-top.
function specReveal() { const t = $('#lcs-root .lcc-sp-headrow'); if (t && typeof t.scrollIntoView === 'function') t.scrollIntoView({ block: 'start' }); }
function specMount() {
  const m = $('#lcs-mount');
  if (m && SPV.vm && window.LCCS) window.LCCS.mount(m, SPV.vm, SPV_HOST);
}
// The card the Specialization tab keeps under the Command Center view: the numbers the save stores for the tracks (Advanced: specialization numbers, drawn only when a track has no number or a number assigned by hand is not the game's own). It sends the requests it always sent (spAssign, spAssignHand).
function specAdvanced(sp) {
  const known = (r) => r.number != null;
  // Every specialization has a number (the ones the game uses, or the ones assigned earlier): a person picks one by name and the numbers stay out of sight. Only a save whose numbers cannot be told gets the assignment card.
  const allKnown = sp.rows.length > 0 && sp.rows.every(known);
  const asMeasured = sp.rows.every((r) => !known(r) || MEASURED_TRACK_NUMBERS[r.track] === r.number); // a number assigned by hand that is not the game's own leaves no way back, so the card stays
  return html`${allKnown && asMeasured ? '' : html`<div class="card d-panel" data-advanced="true"><h3 class="d-head">Advanced: specialization numbers</h3>
    ${sp.unassigned.length ? html`${sp.unassigned.map((u) => html`<div class="row"><span>Track number <b>${u.number}</b>: ${Number(u.xp).toLocaleString()} XP, level ${Math.floor(u.level)}</span> is
      <select id="spa${u.number}" aria-label="Which specialization track is number ${u.number}">${sp.tracks.map((t) => html`<option>${t}</option>`)}</select><button class="b d-btn primary" data-act="spAssign" data-n="${u.number}">Assign</button></div>`)}` : html`<p class="mut">No specialization XP in your save is waiting to be assigned.</p>`}
    <div class="row"><span class="mut">Known a track's number already?</span><select id="spmt" aria-label="Specialization track">${sp.tracks.map((t) => html`<option>${t}</option>`)}</select> number <input id="spmn" aria-label="Track number" type="number" min="0" max="255" class="w70"><button class="b sec d-btn" data-act="spAssignHand">Set</button></div>
    <p class="mut">Assigned: ${sp.rows.filter(known).map((r) => `${r.track} = ${r.number}`).join(', ') || 'none yet'}.</p>
    ${about('specialization.numbers', 'Specialization numbers', html`<p class="mut">Your save stores only a number for each specialization track and does not say which number is which. The editor knows the numbers the game uses (Crafting 1, Gathering 2, Exploration 3, Combat 4, Sabotage 5), so this card is normally not on the page. It is here because a track has no number, or a number assigned earlier is not the game's own. If the XP of one track shows under another track's name, the other track holds its number: set that track again to its own number in the box above. If the game changes a number, earn a little XP in that track, save, reload this tab, and assign the new row. Keystones need no numbers.</p>`)}</div>`}`;
}
// The numbers the game uses for the five tracks (measuredTrackNumbers of internal/ops/landsraad_grant.go; a test compares the two). A track shown with another number was assigned by hand: the Track numbers card is drawn then, so it can be set right.
const MEASURED_TRACK_NUMBERS = { Crafting: 1, Gathering: 2, Exploration: 3, Combat: 4, Sabotage: 5 };
// The reset of the character's level and XP (#339): on Character > Overview, beside Give XP. The number it asks the person to confirm is the XP the summary above shows.
function levelResetCard() {
  const xp = SUMMARY && Number.isInteger(SUMMARY.xp) ? SUMMARY.xp : null;
  // why the button is off is state and stays; what it does when it is on is the button's tooltip
  const off = xp === null ? 'This save has no level record for the character.' : xp > 0 ? '' : 'Nothing to reset: the character has no XP.';
  const btn = html`<button class="b bad d-btn danger" data-act="lvReset" aria-describedby="${off ? 'lvresetnote' : 'lvresettip'}" ${xp ? '' : 'disabled'}>Reset level and XP (${Number(xp || 0).toLocaleString()} XP)</button>`;
  return html`<div class="card d-panel"><h3 class="d-head">Reset level and XP</h3><div class="row">${off ? btn : tip('lvresettip', 'The XP goes back to 0 and the level with it, and the skill points the levels gave are taken back as far as they are unspent. Skills you bought stay, with their points. Asks first.', btn)}${off ? html`<span class="mut" id="lvresetnote">${off}</span>` : ''}</div></div>`;
}
// The Fill card on Player > Character > Overview (#415): what used to be spread over Player > Inventory (Fill hydration, Fill blood sacks #60, Refill Literjons, Fill all liquids #349),
// in one place. The wording, the counts, the disabled states and the data-act keys are those the buttons had on the Inventory tab. The two counts are read here, when the tab is
// drawn; a count that cannot be read leaves its button usable and says so, and a save with no hydration record or no sacks still draws the card.
// Fill blood sacks: the count says how many of the character's blood sacks are not full. Only the kinds whose capacity was seen in a save are filled; the others are left alone and counted.
function bloodSacksRow(b) {
  const total = b ? Number(b.total) || 0 : 0, notFull = b ? Number(b.notFull) || 0 : 0, unknown = b ? Number(b.unknown) || 0 : 0;
  const count = !b ? 'The blood sacks could not be counted.' : total || unknown ? `${notFull} of ${total} blood sacks are not full.` : 'No blood sacks in the backpack, worn gear or loadout.';
  const left = unknown ? ` ${unknown} blood ${unknown == 1 ? 'sack' : 'sacks'} of an unknown kind will be left alone.` : '';
  return html`<div class="row">${tip('bloodtip', 'Fills the sacks you carry to the capacity of their kind, taken from the game\'s item data, without changing anything else about them. Water containers are not touched. The fill has not been checked in the game yet.', html`<button class="b sec d-btn" data-act="fillBlood" aria-describedby="bloodnote bloodtip" ${b && !notFull ? 'disabled' : ''}>Fill blood sacks</button>`)}<span class="mut" id="bloodnote">${count}${left}</span></div>`;
}
// Fill all liquids (issue #349): the water refill, the blood-sack fill and, with the box ticked, the refuel of the character's vehicles, as one edit.
// The box stays as the person left it until the page is reloaded.
let liqVeh = true;
function liquidsRow(l) {
  const n = (k, f) => (l && l[k] ? Number(l[k][f]) || 0 : 0);
  const waterN = n('water', 'notFull'), bloodN = n('bloodSacks', 'notFull'), vehN = n('vehicles', 'notFull');
  const count = !l ? 'The containers could not be counted.' : `Not full: ${nn(waterN, 'water container', 'water containers')}, ${nn(bloodN, 'blood sack', 'blood sacks')}, ${nn(vehN, 'vehicle', 'vehicles')}.`;
  return html`<div class="row">${tip('liqtip', 'Fills your Literjons, stillsuits and other water containers, your blood sacks and (with the box ticked) the fuel of your own vehicles, as one edit with one backup, by the same rules as the three buttons (Refill Literjons, Fill blood sacks, Refuel all my vehicles). Kinds whose size is not known are left alone and counted.', html`<button class="b d-btn primary" data-act="fillLiquids" aria-describedby="liqnote liqtip">Fill all liquids</button>`)}<label><input type="checkbox" id="liqVeh" ${liqVeh ? 'checked' : ''}> Also refuel my vehicles</label><span class="mut" id="liqnote">${count}</span></div>`;
}
async function fillCard() {
  const get = async (path) => { try { return await api(path); } catch (e) { return null; } };
  const [b, l] = await Promise.all([get('/api/player/blood-sacks'), get('/api/player/liquids')]);
  const hyd = SUMMARY && SUMMARY.vitals && SUMMARY.vitals.hydration != null ? SUMMARY.vitals : null;
  // the number is the summary tile's; the line is drawn only when there is none to fill from
  const hydNote = hyd ? '' : 'The summary shows no hydration for the character, so this may be refused.';
  return html`<div class="card d-panel"><h3 class="d-head">Fill</h3>
    <div class="row">${tip('hydtip', 'Fills the character\'s own water to the maximum in the save. While the game is running this is queued until the game closes and loads the save again: it will not help a thirsty character right now.', html`<button class="b sec d-btn" data-act="fillWater" aria-describedby="${hydNote ? 'hydnote hydtip' : 'hydtip'}">Fill hydration</button>`)}${hydNote ? html`<span class="mut" id="hydnote">${hydNote}</span>` : ''}</div>
    ${bloodSacksRow(b)}
    <div class="row">${tip('refilltip', 'Fills the Literjons, stillsuits and other water containers you carry to their capacity.', html`<button class="b sec d-btn" data-act="refill" aria-describedby="refilltip">Refill Literjons</button>`)}</div>
    ${liquidsRow(l)}
    ${about('overview.fill', 'Fill', html`<p class="mut">The fill of blood sacks has not been checked in the game yet. A fill is written to the save at once; while the game is running it is queued and applied when the game closes and loads the save again, so it does not help a character who is thirsty now.</p>`)}</div>`;
}
// ---------- Care package (#417): named kits of items, XP and skill points, kept in the editor's own settings (not in a save, so a new save starts with them).
// The draft is the kit being made or changed; it lives here, not in the boxes, because the page is drawn again after every action.
let KITS = null; // the last answer of /api/kits: { kits, limits, problem, notice }
let kitApplied = new Set(); // the kits applied since this page was opened: applying one again asks with that said (the kit itself remembers nothing about any save)
let kitDraft = null; // { was, name, items: [{ item, quantity, grade, augments, augmentGrade }], xp, skillPoints }; was is the name of the kit being changed ('' for a new one)
const kitSpec = (l) => `${Number(l.quantity).toLocaleString()} x ${itemName(l.item)}${l.grade ? ' (grade ' + l.grade + ')' : ''}${l.augments && l.augments.length ? ' + ' + l.augments.map((a) => augName(a)).join(', ') : ''}`;
const kitContents = (k) => (k.items.length ? k.items.slice(0, 4).map(kitSpec).join('; ') + (k.items.length > 4 ? `; and ${k.items.length - 4} more` : '') : 'No items');
// Reads the boxes into the draft before the page is drawn again.
function kitSync() {
  if (!kitDraft || !$('#kitName')) return;
  const whole = (id) => Math.max(0, Math.floor(+val(id)) || 0);
  kitDraft.name = val('kitName');
  kitDraft.xp = whole('kitXp');
  kitDraft.skillPoints = whole('kitSp');
  document.querySelectorAll('[data-kitqty]').forEach((e) => { const l = kitDraft.items[+e.dataset.kitqty]; if (l) l.quantity = Math.max(1, Math.floor(+e.value) || 1); });
}
function kitsCard() {
  const list = KITS.kits, lim = KITS.limits, editing = !!kitDraft;
  const why = editing ? 'Finish or cancel the kit you are editing first.' : '';
  return html`<div class="card d-panel"><h3 class="d-head">Care packages</h3>
    ${KITS.problem ? html`<div class="info" role="alert">${KITS.problem}</div>` : ''}
    ${KITS.notice ? html`<div class="info" role="status" id="settingsnotice">${KITS.notice}</div>` : ''}
    ${list.length >= lim.kits ? html`<p class="mut" id="kitfull">The most kits are kept (${lim.kits}): delete one first.</p>` : ''}
    ${editing ? html`<p class="mut" id="kitediting">${why}</p>` : ''}
    <div class="row"><button class="b d-btn primary" data-act="kitNew" ${describedBy(list.length >= lim.kits ? 'kitfull' : editing ? 'kitediting' : '')} ${editing || list.length >= lim.kits ? 'disabled' : ''} title="${list.length >= lim.kits ? 'The most kits are kept: delete one first.' : why}">New kit</button>
      <label>Import a kit file <input type="file" id="kitFile" ${describedBy(editing ? 'kitediting' : '')} ${editing ? 'disabled' : ''}></label>
      <button class="b sec d-btn" data-act="kitExport" data-name="" ${list.length ? '' : 'disabled'}>Export all kits</button></div>
    ${tbl([{ k: 'name', label: 'Kit', f: (k) => html`<b>${k.name}</b>${k.problem ? html` <span class="warn" id="kitprob${list.indexOf(k)}">Cannot be used: ${k.problem}</span>` : ''}` },
      { k: 'items', label: 'Items', f: (k) => kitContents(k) }, { k: 'xp', label: 'XP', f: (k) => (k.xp ? Number(k.xp).toLocaleString() : '') },
      { k: 'skillPoints', label: 'Skill points', f: (k) => (k.skillPoints ? Number(k.skillPoints).toLocaleString() : '') }], list,
      { empty: 'No kits yet. Make one with New kit, or import a kit file.', actions: (k) => html`<button class="b sm d-btn primary sm" data-act="kitApply" data-name="${k.name}" aria-label="Apply kit ${k.name} to the character" ${describedBy(k.problem ? 'kitprob' + list.indexOf(k) : '')} ${k.problem ? 'disabled' : ''} title="${k.problem ? 'This kit cannot be used: ' + k.problem : ''}">Apply</button><button class="b sec sm d-btn sm" data-act="kitEdit" data-name="${k.name}" aria-label="Edit kit ${k.name}" ${describedBy(editing ? 'kitediting' : '')} ${editing ? 'disabled' : ''} title="${why}">Edit</button><button class="b sec sm d-btn sm" data-act="kitDup" data-name="${k.name}" aria-label="Duplicate kit ${k.name}">Duplicate</button><button class="b sec sm d-btn sm" data-act="kitExport" data-name="${k.name}" aria-label="Export kit ${k.name}">Export</button><button class="b bad sm d-btn danger sm" data-act="kitDel" data-name="${k.name}" aria-label="Delete kit ${k.name}">Delete</button>` })}
    ${about('carepackage.list', 'Care packages', html`<p class="mut">A care package is a named kit of items, XP and skill points. Kits are kept in the editor's own settings, not in a save, so a new save starts with them. Apply gives a kit to the character in this save as one edit: the items by the rules of Give item (slots, stacks, volume), then the XP and the skill points. It shows what would be given and what would not fit before it asks. Up to ${lim.kits} kits, each with up to ${lim.items} items. A kit file you export is a small text file; one you import is checked first and never replaces a kit you have.</p>`)}</div>`;
}
function kitEditor(cat) {
  const d = kitDraft, lim = KITS.limits;
  return html`<div class="card d-panel"><h3>${d.was ? 'Edit kit: ' + d.was : 'New kit'}</h3>
    <div class="row">${tip('kitnametip', html`Up to ${lim.name} letters, digits, spaces and - _ . , ' ( ) + &amp; # !`, html`<label>Name <input id="kitName" value="${d.name}" size="32" aria-describedby="kitnametip"></label>`)}</div>
    <div class="row"><b>Add an item</b>${itemPicker('kitItem', cat)}<label>Qty <input id="kitQty" type="number" value="1" min="1" max="${lim.quantity}"></label><label>Grade <select id="kitGrade">${[0, 1, 2, 3, 4, 5].map((n) => html`<option>${n}</option>`)}</select></label><button class="b d-btn primary" data-act="kitAddLine" ${d.items.length >= lim.items ? 'disabled' : ''}>Add to kit</button></div>
    <div class="row" id="kitAug"></div>
    ${tbl([{ k: 'item', label: 'Item', f: (l) => itemCell(l.item) },
      { k: 'quantity', label: 'Quantity', f: (l) => html`<input type="number" min="1" max="${lim.quantity}" value="${l.quantity}" data-kitqty="${d.items.indexOf(l)}" class="w90" aria-label="Quantity of ${itemName(l.item)}">` },
      { k: 'grade', label: 'Grade' }, { k: 'augments', label: 'Augments', f: (l) => (l.augments || []).map((a) => html`<span class="tag">${augName(a)}</span>`) }], d.items,
      { empty: 'No items yet.', actions: (l) => html`<button class="b bad sm d-btn danger sm" data-act="kitDelLine" data-id="${d.items.indexOf(l)}" aria-label="Remove ${itemName(l.item)} from the kit">Remove</button>` })}
    <div class="row">${tip('kitxptip', 'XP raises the level and adds a skill point per level gained, as Give XP does. Skill points are added on top.', html`<label>XP <input id="kitXp" type="number" min="0" max="${lim.xp}" value="${d.xp}" class="w90" aria-describedby="kitxptip"></label>`)}<label>Skill points <input id="kitSp" type="number" min="0" max="${lim.skillPoints}" value="${d.skillPoints}" class="w90"></label></div>
    <div class="row"><button class="b d-btn primary" data-act="kitSave">Save kit</button><button class="b sec d-btn" data-act="kitCancel">Cancel</button></div></div>`;
}
// What an apply of a kit gave or would give, line by line, for the question that asks first and for the message that reports it.
function kitLines(r) {
  const n = (v) => Number(v).toLocaleString();
  const out = r.lines.map((l) => (Number(l.given) >= Number(l.requested) ? `${n(l.given)} x ${l.name}` : `${n(l.given)} of ${n(l.requested)} x ${l.name}: ${n(l.requested - l.given)} did not fit (${l.reason || 'no room'})`));
  if (r.xp) out.push(r.xp.applied ? `+${n(r.xp.applied)} XP, level ${r.xp.levelBefore} to ${r.xp.levelAfter}${r.xp.capped ? `; ${n(r.xp.requested - r.xp.applied)} XP past the last level left out` : ''}` : 'No XP: the character is at the last level already');
  if (r.skillPoints) out.push(r.skillPoints.applied ? `+${n(r.skillPoints.applied)} skill points${r.skillPoints.capped ? `; ${n(r.skillPoints.requested - r.skillPoints.applied)} more would pass the most that can be unspent` : ''}` : 'No skill points: the most that can be unspent is reached already');
  return out;
}
async function kitDownload(name) {
  const j = await api('/api/kits/export' + (name ? '?name=' + encodeURIComponent(name) : ''));
  const b = new Blob([JSON.stringify(j, null, 2) + '\n'], { type: 'application/json' });
  const u = URL.createObjectURL(b), l = document.createElement('a');
  l.href = u;
  l.download = (name ? 'tabr-tau-kit-' + name.replace(/[^A-Za-z0-9._-]+/g, '_') : 'tabr-tau-kits') + '.json';
  l.click();
  setTimeout(() => URL.revokeObjectURL(u), 60000); // the browser reads the blob after the click returns
}
function kitImport(input) {
  const f = input.files && input.files[0];
  input.value = '';
  if (!f) return;
  if (f.size > KITS.limits.fileBytes) { toast(`That file is larger than ${KITS.limits.fileBytes / 1024} KiB: a kit file is small, so this is not one.`, true); return; }
  act(async () => {
    const r = await api('/api/kits/import', { text: await f.text() });
    toast(`Imported ${nn(r.imported, 'kit', 'kits')}` + (r.renamed ? `; ${nn(r.renamed, 'name was', 'names were')} taken, so ${r.renamed == 1 ? 'that kit has' : 'those kits have'} "(imported)" after the name` : ''));
  }, null, true, true);
}
async function playerSection(s) {
  const h = s == 'character' ? [subNav('char', [['overview', 'Overview'], ['inventory', 'Inventory'], ['reputation', 'Reputation'], ['carepackage', 'Care package']])] : [];
  const c = s == 'character' ? sub.char || 'overview' : s;
  if (c == 'overview') {
    P = await api('/api/player');
    const a = P.actor || {}, j = P.journey || {};
    h.push(html`<div class="card d-panel"><h3>${P.name}</h3><div class="grid">${kv('Solari in backpack', P.solari.toLocaleString())}${kv('Position', `${fix(a.x)}, ${fix(a.y)}, ${fix(a.z)}`)}${kv('Journey', `${(j.done || 0).toLocaleString()} of ${(j.total || 0).toLocaleString()} nodes done`)}</div></div>
    <div class="card d-panel"><h3 class="d-head">Money</h3><div class="row"><b>Solari in backpack</b><input id="solari" aria-label="Solari to add or remove" type="number" value="10000">${tip('solaritip', 'A negative number removes. These are the Solari stacks in your backpack.', html`<button class="b d-btn primary" data-act="solari" aria-describedby="solaritip">Add or remove</button>`)}</div>
    <div class="row"><b>Give to a wallet</b><select id="curSel" aria-label="Wallet currency"><option value="0">Solari in wallet</option><option value="1">House credit in wallet</option></select><input id="curAmt" aria-label="Amount to give to the wallet" type="number" value="100" min="1">${tip('wallettip', 'A wallet balance, kept apart from the Solari stacks in your backpack.', html`<button class="b d-btn primary" data-act="currency" aria-describedby="wallettip">Give</button>`)}</div>
    <div class="row"><b title="Blood Marks (item HalloweenCoin): the currency of the Scourge event">Blood Marks in backpack</b><b id="bmHave">${Number(P.bloodMarks || 0).toLocaleString()}</b><input id="bmAmt" type="number" value="100" min="1" max="${P.bloodMarksMax || 50000}" aria-label="Blood Marks to give">${tip('bmtip', 'The currency of the Scourge event (item HalloweenCoin). It is added to the stack in your backpack, and to a new stack when it does not fit; as much as fits is given.', html`<button class="b d-btn primary" data-act="bloodMarks" aria-describedby="bmtip">Give Blood Marks</button>`)}</div></div>
    <div class="card d-panel"><h3 class="d-head">Quick rewards</h3><div class="row"><b>Give XP</b><input id="xpAmt" aria-label="XP to give" type="number" value="1000" min="1">${tip('xptip', 'Raises the level as the XP grows and adds one skill point per level gained; stops at level 200.', html`<button class="b d-btn primary" data-act="xp" aria-describedby="xptip">Give</button>`)}</div><div class="row"><b>Give Intel</b><input id="intelAmt" aria-label="Intel to give" type="number" value="100" min="1">${tip('inteltip', 'Adds Intel for the research tree, never past 2,779.', html`<button class="b d-btn primary" data-act="intel" aria-describedby="inteltip">Give</button>`)}</div></div>
    ${await fillCard()}
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
    const invCard = html`<div class="card d-panel"><h3>Inventory: ${nn(d.items.length, 'item', 'items')} carried</h3>
      <div class="sub">${G.map(([k, v]) => html`<button data-invg="${k}" class="${grp == k ? 'on' : ''}">${v} (${counts[k]})</button>`)}</div>
      <div class="row"><input id="invq" aria-label="Filter the inventory" placeholder="Filter by name, item ID, or template" size="34" value="${invQ}"><button class="b sec d-btn" data-act="invfilter">Filter</button><button class="b sec d-btn" data-act="invclear">Clear</button><span class="mut">${countText(rows.length, counts[grp], 'item', 'items', terms.length > 0)}${slots}</span>${fillTotals(d.items.filter((r) => groupOf(r) == grp)) ? html`<span class="mut" id="invfill">${fillTotals(d.items.filter((r) => groupOf(r) == grp))}</span>` : ''}</div>
      ${cap && grp != 'schematics' ? clearRow(cap.id, G.find(([k]) => k == grp)[1], d.items.filter((r) => r.inventory_id == cap.id)) : ''}
      ${tbl([{ k: 'position_index', label: 'Slot' }, { k: 'template_id', label: 'Item', f: (r) => itemCell(r.template_id) },
        { k: 'stack_size', label: 'Stack', f: (r) => html`<input type="number" value="${r.stack_size}" min="1" data-item="${r.id}" data-field="stack_size" data-name="${itemName(r.template_id)}" class="w90" aria-label="Stack of ${itemName(r.template_id)}">` },
        { k: 'quality_level', label: 'Grade', f: (r) => html`<select data-item="${r.id}" data-field="quality" data-name="${itemName(r.template_id)}" aria-label="Grade of ${itemName(r.template_id)}">${[0, 1, 2, 3, 4, 5].map((n) => html`<option ${n == r.quality_level ? 'selected' : ''}>${n}</option>`)}</select>` },
        { k: 'fill', label: 'Liquid', f: (r) => fillCell(r) },
        { k: 'durability', label: 'Durability', f: (r) => (r.durability == null ? '' : Number(r.durability).toFixed(1) + (r.max_durability ? ' / ' + Number(r.max_durability).toFixed(0) : '')) },
        { k: 'augments', label: 'Augments', f: (r) => (r.augments || []).map((a) => html`<span class="tag" title="${a.name}${a.quality == null ? '' : ' (quality ' + a.quality + ')'}">${itemName(a.name)}</span>`) }], rows,
        { actions: (r) => html`${r.aug_limit ? html`<button class="b sec sm d-btn sm" data-act="augEdit" data-id="${r.id}">Augments</button>` : ''}<button class="b bad sm d-btn danger sm" data-act="delItem" data-id="${r.id}" data-name="${itemName(r.template_id)}">Delete</button>`, empty: terms.length ? 'No item in this group matches the filter.' : 'Nothing in this group.' })}
      ${about('inventory.card', 'Inventory', html`<p class="mut">The fill actions (Fill hydration, Fill blood sacks, Refill Literjons, Fill all liquids) are on Character &gt; Overview. <button class="b sec sm d-btn sm" data-goto="character:overview">Go to Character</button></p>`)}</div>`;
    const augCard = augItem ? html`<div class="card d-panel"><h3>Augments: ${itemName(augItem.template_id)}</h3>
      <p class="mut">Holds up to ${augItem.limit}.</p>
      ${augBlock('au', augItem, augItem.applied.map((a) => a.id), 'augG')}
      <div class="row"><label><input type="checkbox" id="augU" checked> Also unlock the augment slots (specialization keystones)</label>
      ${tip('augtip', 'Pick what to apply; an augment already on the item keeps its grade, new ones get the grade above. Choose (none) in every slot to remove them all.', html`<button class="b d-btn primary" data-act="augApply" data-id="${augItem.item_id}" aria-describedby="augtip">Apply</button>`)}<button class="b sec d-btn" data-act="augCancel">Cancel</button></div></div>` : '';
    h.push(augCard);
    h.push(html`<div class="card d-panel"><h3 class="d-head">Repair</h3><div class="row">${tip('repairtip', 'Everything you wear, everything in your loadout and everything in your backpack is made as good as new: full durability, with a maximum that repairs wore down restored. Chests and the items of other characters are not touched.', html`<button class="b d-btn primary" data-act="repair" aria-describedby="repairtip">Repair all worn items, loadouts and backpack</button>`)}</div></div>`);
    h.push(html`<div class="card d-panel"><h3 class="d-head">Give item</h3><div class="row">${itemPicker("gt", cat, undefined, true)}
    Qty<input id="gq" aria-label="Quantity to give" type="number" value="1" min="1">Grade<select id="gg" aria-label="Grade of the item to give">${[0, 1, 2, 3, 4, 5].map((n) => html`<option>${n}</option>`)}</select><button class="b d-btn primary" data-act="give">Give</button><button class="b sec d-btn" data-act="queueAdd">Add to queue</button></div>
    <div class="row" id="gaug"></div>
    ${giveQueue.length ? html`<div class="row"><b>Queue (${giveQueue.length})</b>${giveQueue.map((q, i) => html`<span class="tag">${q.quantity} x ${itemKindName(q.template_id)}${q.quality ? ' (grade ' + q.quality + ')' : ''}${q.augments && q.augments.length ? ' + ' + q.augments.map((a) => augName(a)).join(', ') : ''} <button class="b sec sm d-btn sm" data-act="queueDel" data-id="${i}" title="Remove from the queue">x</button></span>`)}<button class="b d-btn primary" data-act="queueGive">Give queued items</button><button class="b sec d-btn" data-act="queueClear">Clear</button></div>` : ''}
    ${about('inventory.give', 'Give item', html`<p class="mut">Pick an item by its in-game name. Items already in your save that the catalog does not know are listed by their template id; any valid template id can also be typed.</p>`)}</div>
    ${invCard}`);
  } else if (c == 'carepackage') {
    KITS = await api('/api/kits');
    const cat = await loadCatalog([]);
    h.push(kitsCard());
    if (kitDraft) h.push(kitEditor(cat));
  } else if (c == 'reputation') {
    const f = await api('/api/player/factions');
    h.push(html`<div class="card d-panel"><h3 class="d-head">Faction reputation</h3>${tbl([{ k: 'name', label: 'Faction' }, { k: 'reputation', label: 'Reputation', f: (r) => html`<input type="number" min="0" max="12474" value="${r.reputation}" data-faction="${r.faction_id}" data-name="${r.name}" class="w100" aria-label="Reputation with ${r.name}" title="0 to 12474">` }], f, { empty: 'This save has no factions.' })}</div>`);
  } else if (s == 'specialization') {
    if (!specCCOn()) { SPV.vm = null; SPV.lv = null; SPV.stale = false; SPV.lvAt = ++SPV.n; h.push(specMissing()); } else {
      const sp = await api('/api/player/specs');
      let hs = null;
      try { const ls = await api('/api/landsraad'); hs = !ls || !Array.isArray(ls.tasks) ? null : ls.tasks.map((t) => ({ id: t.id, name: lbNameOf(t), line: t.line || '' })); } catch (e) { hs = null; } // the Houses of a path come from the Landsraad read; without it (or without a task list in the answer) they are "Not in the save"
      SPV.vm = window.LCCS.readSpecs(sp, hs); SPV.lv = SPV.vm; SPV.stale = false; SPV.lvAt = ++SPV.n; // the Landsraad board shows the levels of this answer (#646)
      h.push(html`<div id="lcs-mount"></div>`, specAdvanced(sp));
    }
  } else if (s == 'journey') {
    JB = await api('/api/player/journey/browse');
    const tg = await api('/api/player/tags');
    h.push(html`<div class="card d-panel"><h3 class="d-head">Journey browser</h3><div id="jhead"></div><div id="jout"></div>${about('journey.card', 'Journey browser', html`<p class="mut">Completing a node completes its children; resetting clears it and its children.</p>`)}</div>`);
    h.push(html`<div class="card d-panel"><h3 class="d-head">Player tags</h3>${tg.length ? tg.map((t) => html`<span class="tag">${t.tag} <a href="#" data-act="tagDel" data-tag="${t.tag}" aria-label="Remove tag ${t.tag}" title="Remove tag ${t.tag}">×</a></span>`) : html`<span class="mut">none</span>`}<div class="row"><input id="tag" aria-label="Tag name" placeholder="Tag.Name" size="40"><button class="b d-btn primary" data-act="tagAdd">Add tag</button></div></div>`);
  } else if (s == 'skills') {
    SK = await api('/api/player/skills');
    h.push(html`<div class="card d-panel"><h3 class="d-head">Skill point controls</h3>
      <div class="row"><span>Earned in total <b>${SK.total}</b></span><span>Spent in skills <b>${SK.spent}</b></span><span>Unspent <b>${SK.unspent}</b></span></div>
      <div class="row">Unspent points<input id="skPts" aria-label="Unspent skill points" type="number" min="0" max="100000" value="${SK.unspent}" class="w90"><button class="b d-btn primary" data-act="skPoints">Set unspent points</button></div>
      ${about('skills.points', 'Skill point controls', html`<p class="mut">The save does not keep spent plus unspent equal to the total (some skills are granted free), so these three numbers are shown as stored and not worked out from each other.</p>`)}</div>
    <div class="card d-panel"><h3 class="d-head">Skill browser</h3><div id="skhead"></div><div id="skout"></div></div>`);
  } else if (s == 'research') {
    const rsAns = await api('/api/player/research');
    RS = rsAns.rows;
    RSRESET = { entries: rsAns.resetCount || 0, recipes: rsAns.resetRecipes || 0, sets: rsAns.resetSets || 0 };
    h.push(html`<div class="card d-panel"><h3 class="d-head">Research</h3><div class="row">${tip('rstip', 'Buys every entry that unlocks a recipe or a building set and adds what it unlocks.', html`<button class="b d-btn primary" data-act="rsAll" aria-describedby="rstip">Unlock all research</button>`)}</div><div id="rshead"></div><div id="rsout"></div></div>`);
  } else if (s == 'crafting') {
    CR = (await api('/api/player/crafting')).rows;
    h.push(html`<div class="card d-panel"><h3 class="d-head">Crafting recipes</h3><div class="row">${tip('crtip', 'Adds every recipe your research tree offers that you do not know yet.', html`<button class="b d-btn primary" data-act="crAll" aria-describedby="crtip">Unlock all recipes</button>`)}</div><div id="crhead"></div><div id="crout"></div>${about('crafting.card', 'Crafting recipes', html`<p class="mut">The recipes listed are the ones you know plus the ones your research tree can still give.</p>`)}</div>`);
  } else if (s == 'buildingsets') {
    CT = await api('/api/player/building-sets');
    CT.kind = 'bs';
    ctGroup = '';
    h.push(html`<div class="card d-panel"><h3 class="d-head">Building sets</h3><div class="row">${tip('bstip', 'Teaches every set below except the experimental ones, without putting the items in your backpack.', html`<button class="b d-btn primary" data-act="bsAll" aria-describedby="bstip">Unlock all building sets</button>`)}</div><div id="cthead"></div><div id="ctout"></div>${about('buildingsets.card', 'Building sets', html`<p class="mut">Give adds the item to your backpack.</p>`)}</div>
      <div class="card d-panel"><h3>New buildable pieces (${CT.newPieces.length})</h3>${CT.newPieces.map((x) => html`<span class="tag">${x.name}</span>`)}</div>`);
  } else if (s == 'customizations') {
    CT = await api('/api/player/customizations');
    CT.kind = 'cu';
    ctGroup = '';
    h.push(html`<div class="card d-panel"><h3 class="d-head">Customizations</h3>
      ${CT.libraryAvailable ? html`<div class="row">${tip('cutip', 'Adds the cosmetics of the five sets to your character\'s customization library: no backpack space needed. Store and DLC cosmetics are skipped, because the game removes the ones your account does not own.', html`<button class="b d-btn primary" data-act="ctUnlockSet" data-id="all" data-name="all five sets" aria-describedby="cutip">Unlock all sets</button>`)}<span class="mut">${CT.libraryCount} unlocked now.</span></div>` : html`<p class="mut">This save has no customization library for the character, so cosmetics can only be given as items.</p>`}
      <div class="row">${tip('cugivetip', 'Puts one cosmetic item of each in your backpack instead, all or nothing: there are more of them than the backpack holds, so give one set at a time.', html`<button class="b sec d-btn" data-act="ctGrantSet" data-id="all" data-name="all five sets" aria-describedby="cugivetip">Give all sets as items</button>`)}</div><div id="cthead"></div><div id="ctout"></div>${about('customizations.card', 'Customizations', html`<p class="mut">Give adds the item to your backpack. Unlock adds the cosmetic to your customization library without an item. Neither gives a DLC or an account entitlement: the game removed a store cosmetic that was unlocked this way (a plain dye pack stayed), so set unlocks skip those.</p>`)}</div>`);
  } else if (s == 'admin') {
    P = await api('/api/player');
    const a = P.actor || {};
    goSavedMap(a.map || '');
    const fa = await api('/api/player/faction');
    h.push(html`<div class="card d-panel"><h3 class="d-head">Faction assignment</h3><div class="row"><b>Current</b><span>${fa.current ? fa.current.name : 'none recorded'}</span>
      <select id="facSel" aria-label="Faction">${fa.options.map((o) => html`<option value="${o.id}" ${fa.current && fa.current.id == o.id ? 'selected' : ''}>${o.name}</option>`)}</select>${tip('factip', 'Changes which faction the character belongs to. Reputation is under Character > Reputation.', html`<button class="b d-btn primary" data-act="faction" aria-describedby="factip">Change faction</button>`)}</div></div>
    <div class="card d-panel"><h3 class="d-head">Repair</h3><div class="row"><b>Gear and loadouts</b><button class="b sec d-btn" data-goto="character:inventory">Open Character &gt; Inventory</button><b>Vehicles</b><button class="b sec d-btn" data-goto="vehicles">Open Vehicles</button></div></div>
    <div class="card d-panel"><h3 class="d-head">Movement</h3><div class="row"><b>Teleport to</b>X<input id="tx" aria-label="Teleport to X" type="number" value="${goVal('tx', Math.round(a.x))}">Y<input id="ty" aria-label="Teleport to Y" type="number" value="${goVal('ty', Math.round(a.y))}">Z${tip('tztip', 'If you type a Z yourself, stay above the terrain or you may fall through.', html`<input id="tz" aria-label="Teleport to Z" type="number" value="${goVal('tz', Math.round(a.z))}" aria-describedby="tztip">`)}Facing ${tip('tyawtip', 'Facing is optional: 0 looks along +X; leave it empty to keep the way the character faces.', html`<input id="tdyaw" aria-label="Facing in degrees" type="number" step="1" placeholder="degrees" value="${goVal('tdyaw', '')}" class="w90" aria-describedby="tyawtip">`)}Map ${tip('tmaptip', 'Places are on the map chosen in Map (the one the character is on now unless you change it).', html`<select id="tmap" aria-label="Map to move to" aria-describedby="tmaptip"><option value="" ${goVal('tmap', '') === '' ? 'selected' : ''}>This map (${a.map ? mapName(a.map) : 'unknown'})</option>${TRAVEL_MAPS.map(([k, v]) => html`<option value="${k}" ${goVal('tmap', '') === k ? 'selected' : ''}>${v}</option>`)}</select>`)}<button class="b d-btn primary" data-act="teleport">Teleport</button><span id="tdpick" class="mut" role="status"></span></div>
    ${a.parked ? html`<p class="mut" id="tparked">This save keeps the character's body away from the player: on another map${a.bodyMap && a.bodyMap !== a.map ? ` (${mapName(a.bodyMap)})` : ''}, or not standing in the world. This map and the X, Y and Z above are where the player is.</p>` : html``}
    <div id="tmapnote"></div>
    <div class="row"><b>Go to a place</b><span id="tdout" class="mut"></span></div>
    ${about('admin.movement', 'Movement', html`<p class="mut">Moves the character in the save. The game keeps its own copy of the save in memory while single-player runs and overwrites the file, so the move shows when you next enter single-player.</p><p class="mut">${TRAVEL_NOTE}</p><p class="mut">Go to a place lists the trading posts you have found, your own bases that stand in the world, your own vehicles that stand in the world, your respawn points and the taxis, under those headings, nearest first. Taxis are the three Deep Desert taxi service points while the Deep Desert is the map (from the game's level data, named by their grid cell, whether or not your save has found them). The two hubs, Arrakeen and Harko Village, are another map from every map: they are listed last, under "${OTHER_MAPS_HEAD}", and the count at the top of the list does not include them. Choosing a place fills X, Y and Z.</p><p class="mut">${PLACE_NOTE}</p><p class="mut">If you type a Z yourself, stay above the terrain or you may fall through.</p>`)}
    </div>`);
    const ve = await api('/api/vendors');
    await loadCatalog(ve.stock.map((x) => x.template_id));
    // The save has no name for a vendor (#618): each is "Vendor 1", "Vendor 2" in the order they first appear, the same in both tables, with its id in the tooltip and in the list under the tables.
    const vnum = new Map();
    for (const r of [...ve.stock, ...ve.cycles]) if (!vnum.has(r.vendor_id)) vnum.set(r.vendor_id, vnum.size + 1);
    const vendorCell = (r) => html`<span title="Vendor id ${r.vendor_id}">Vendor ${vnum.get(r.vendor_id)}</span>`;
    h.push(html`<div class="card d-panel"><h3 class="d-head">Vendor purchase limits</h3><div class="row">${tip('vendtip', 'Vendors limit how much you can buy per restock cycle. Resetting clears those counters.', html`<button class="b d-btn primary" data-act="vreset" data-v="" aria-describedby="vendtip">Reset all vendors</button>`)}</div>
      ${tbl([{ k: 'vendor_id', label: 'Vendor', f: vendorCell }, { k: 'template_id', label: 'Item', f: (r) => itemCell(r.template_id) }, { k: 'amount_bought', label: 'Bought' }], ve.stock, { actions: (r) => html`<button class="b sec sm d-btn sm" data-act="vreset" data-v="${r.vendor_id}" data-vn="Vendor ${vnum.get(r.vendor_id)}">Reset vendor</button>`, empty: 'No purchases are counted against any vendor, so there is nothing to reset.' })}
      <h3 class="mt14 d-head">Restock cycles</h3>${tbl([{ k: 'vendor_id', label: 'Vendor', f: vendorCell }, { k: 'last_interacted_timestamp', label: 'Last interaction', f: (r) => fmtEpoch(r.last_interacted_timestamp) }], ve.cycles, { empty: 'No vendor has been visited yet.' })}
      ${vnum.size ? html`<details class="ids"><summary>Vendor ids</summary><div class="grid">${[...vnum].map(([id, n]) => kv('Vendor ' + n, id))}</div></details>` : ''}</div>`);
  } else {
    h.push(html`<div class="card d-panel"><p class="mut">There is nothing on this page.</p></div>`);
  }
  setHTML(host(), html`${h}`);
  if (s == 'specialization') specMount();
  if (s == 'journey') journeyList();
  if (s == 'skills') skillsList();
  if (s == 'research') researchList();
  if (s == 'buildingsets' || s == 'customizations') catalogList();
  if (s == 'crafting') craftingList();
}
// The Journey browser, as the console's: Story, Contract, Codex and Tutorial lists with readable names, the tree depth, and status.
// Story, Codex and Tutorial rows can be completed or reset (a story or codex node takes its children with it); contracts are read-only here.
let JB = null, jgrp = 'story', journeyQ = ''; // journeyQ: the filter text; it is not window.jq, which a page with an element of the id jq resolves to that element // JB: the last browse result
const JGROUPS = [['story', 'Story'], ['contract', 'Contract'], ['codex', 'Codex'], ['tutorial', 'Tutorial']];
const JMAX = 400;
function journeyList() {
  if (!JB || !$('#jout')) return;
  const q = journeyQ.trim().toLowerCase();
  const all = JB[jgrp] || [];
  const rows = q ? all.filter((r) => r.name.toLowerCase().includes(q) || r.id.toLowerCase().includes(q)) : all;
  const jn = JB.resetNodes || 0, jt = JB.resetTutorials || 0;
  setHTML($('#jhead'), html`<div class="sub">${JGROUPS.map(([k, v]) => html`<button data-jgrp="${k}" class="${jgrp == k ? 'on' : ''}">${v} (${(JB[k] || []).length})</button>`)}</div>
    <div class="row">${(() => {
        const b = (off) => html`<button class="b bad d-btn danger" data-act="jReset" data-n="${JB.resetCount || 0}" data-nodes="${jn}" data-tuts="${jt}" aria-describedby="jresetnote" ${off ? 'disabled' : ''}>Reset journey (${nn(jn, 'step', 'steps')}, ${nn(jt, 'tutorial', 'tutorials')})</button>`;
        return JB.resetCount ? tip('jresetnote', 'Every finished Story and Codex step goes back to not finished and every started or finished tutorial back to not started, in one edit, after you type a confirmation. Waiting rewards, tags and contracts are not touched, and the 95 steps and 4 tutorials a new character already has finished are left alone.', b(false)) : html`${b(true)}<span class="mut" id="jresetnote">Nothing to reset: no Story or Codex step is finished and no tutorial is started.</span>`;
      })()}</div>
    <div class="row"><input id="jq" aria-label="Filter the journey" placeholder="Filter by name or node id" size="34" value="${journeyQ}"><button class="b sec d-btn" data-act="jfilter">Filter</button><button class="b sec d-btn" data-act="jclear">Clear</button>
    <span class="mut">${countText(rows.length, all.length, 'node', 'nodes', !!q)}${rows.length > JMAX ? `, the first ${JMAX} shown: narrow the filter to see others` : ''}.${jgrp == 'contract' ? ' Contracts are shown read-only; a contract is complete when all its tags are on the character.' : ''}</span></div>`);
  const stat = (r) => (r.status == 'Complete' ? html`<span class="ok">Complete</span>` : r.status == 'Revealed' || r.status == 'Started' ? html`<span class="warn">${r.status}</span>` : html`<span class="mut">${r.status}</span>`);
  const act = (r) => {
    if (!r.actionable) return '';
    const kind = jgrp == 'tutorial' ? 'tut' : 'jset';
    return r.complete ? html`<button class="b sec sm d-btn sm" data-act="${kind}" data-id="${r.id}" data-c="0">Reset</button>` : html`<button class="b sm d-btn primary sm" data-act="${kind}" data-id="${r.id}" data-c="1">Complete</button>`;
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
    ${skSchool != 'Other' ? html`<button class="b sec d-btn" data-act="skStarter">Restore starter skills</button>` : ''}
    ${skSchool != 'Other' ? html`<button class="b d-btn primary" data-act="skMax" data-school="${skSchool}">Max all ${groups.find(([k]) => k == skSchool)[1]} skills</button>` : ''}<button class="b d-btn primary" data-act="skMax" data-school="all">Max all skills of all schools</button>
    <span class="mut">${nn(rows.length, 'skill', 'skills')}, ${rows.filter((m) => m.rank > 0).length} learned.</span></div>
    <div class="row">${SK.resetCount ? tip('skresetnote', 'Every skill of the five schools that has points goes back to rank 0, in one edit, after you type a confirmation. Hidden skills and skills the catalog does not know are left alone.', html`<button class="b bad d-btn danger" data-act="skReset" aria-describedby="skresetnote">Reset all skills to rank 0 (${SK.resetCount})</button>`) : html`<button class="b bad d-btn danger" data-act="skReset" aria-describedby="skresetnote" disabled>Reset all skills to rank 0 (0)</button><span class="mut" id="skresetnote">Nothing to reset: every skill of the five schools is already at rank 0.</span>`}</div>`);
  const bar = (m) => (m.known ? html`<span class="rankbar" title="rank ${m.rank} of ${m.max}">${'●'.repeat(m.rank)}${'○'.repeat(Math.max(0, m.max - m.rank))}</span>` : html`<span class="mut" title="not in the catalog: ${m.spent} points spent">?</span>`);
  setHTML($('#skout'), tbl([{ k: 'name', label: 'Skill', f: (m) => html`<span title="${m.id}">${m.name}</span>` }, { k: 'kind', label: 'Type' }, { k: 'rank', label: 'Rank', f: bar },
    { k: 'spent', label: 'Points', f: (m) => m.spent },
    { k: 'max', label: 'Set rank', f: (m) => (m.known ? html`<select data-skill="${m.id}" data-name="${m.name}" aria-label="Rank of ${m.name}">${Array.from({ length: m.max + 1 }, (_, i) => html`<option ${i == m.rank ? 'selected' : ''}>${i}</option>`)}</select>` : html`<span class="mut">catalog does not know its ranks</span>`) }], rows, { empty: 'No skills in this school.' }));
}

// Research and crafting recipes, as in Dune Docker's Research and Crafting tabs: a category, a filter, and an Unlock on each entry that is not
// there yet. Unlocking research marks it purchased and adds the recipe or building set it unlocks; a purchased entry whose recipe or set is
// missing shows Repair unlock.
let RS = null, RSRESET = { entries: 0, recipes: 0, sets: 0 }, CR = null, rsCat = '', rsGroup = '', rsQ = '', crCat = '', crQ = '';
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
    <input id="rsq" aria-label="Filter the research" placeholder="Filter by name, key, type or group" size="34" value="${rsQ}"><button class="b sec d-btn" data-act="rsfilter">Filter</button><button class="b sec d-btn" data-act="rsclear">Clear</button>
    <span class="mut">${countText(rows.length, inCat.length, 'entry', 'entries', rsFiltered)}${rows.length > LISTMAX ? `, the first ${LISTMAX} shown` : ''}; ${done} of ${RS.length} entries unlocked in all.</span></div>
    <div class="row">${RSRESET.entries ? tip('rsresetnote', `Puts the research back to what a new character has: every bought entry except the four a new character starts with goes back to not bought, and the ${nn(RSRESET.recipes, 'recipe', 'recipes')} and ${nn(RSRESET.sets, 'building set', 'building sets')} they gave are taken out again. After a typed confirmation, in one edit. Intel is not given back.`, html`<button class="b bad d-btn danger" data-act="rsReset" aria-describedby="rsresetnote">Reset research (${nn(RSRESET.entries, 'entry', 'entries')})</button>`) : html`<button class="b bad d-btn danger" data-act="rsReset" aria-describedby="rsresetnote" disabled>Reset research (${nn(RSRESET.entries, 'entry', 'entries')})</button><span class="mut" id="rsresetnote">Nothing to reset: no entry beyond the four a new character starts with is bought.</span>`}</div>`);
  const state = (r) => (r.unlocked ? html`<span class="ok">Unlocked</span>` : r.purchased ? html`<span class="warn">Purchased, ${r.unlockKind} missing</span>` : html`<span class="mut">Not purchased</span>`);
  const act = (r) => (!r.actionable ? html`<button class="b sec sm d-btn sm" disabled aria-describedby="rsg${String(r.itemKey).replace(/[^A-Za-z0-9_-]/g, '_')}" title="A group marker; unlock its individual entries">Group</button> <span class="mut" id="rsg${String(r.itemKey).replace(/[^A-Za-z0-9_-]/g, '_')}">A group marker; unlock its entries.</span>` : r.unlocked ? '' :
    html`<button class="b sm d-btn primary sm" data-act="rsUnlock" data-key="${r.itemKey}">${r.needsRepair ? 'Repair unlock' : 'Unlock'}</button>`);
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
    <div class="row"><input id="crq" aria-label="Filter the recipes" placeholder="Filter by name, recipe id or source" size="34" value="${crQ}"><button class="b sec d-btn" data-act="crfilter">Filter</button><button class="b sec d-btn" data-act="crclear">Clear</button>
    <span class="mut">${countText(rows.length, inCat.length, 'recipe', 'recipes', !!crQ.trim())}${rows.length > LISTMAX ? `, the first ${LISTMAX} shown` : ''}; ${known} of ${CR.length} known.</span></div>`);
  setHTML($('#crout'), tbl([{ k: 'name', label: 'Recipe', f: (r) => html`<span title="${r.recipeId}">${r.name}</span>${r.limited ? html` <span class="tag">limited use (${r.uses})</span>` : ''}` }, { k: 'recipeId', label: 'Recipe id', cls: 'mut' },
    { k: 'category', label: 'Category' }, { k: 'source', label: 'Source' }, { k: 'known', label: 'State', f: (r) => (r.known ? html`<span class="ok">Known</span>` : html`<span class="mut">Not known</span>`) }], rows.slice(0, LISTMAX),
    { actions: (r) => (r.known ? '' : html`<button class="b sm d-btn primary sm" data-act="crUnlock" data-id="${r.recipeId}">Unlock</button>`), empty: 'No recipe matches.' }));
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
  // Building sets: the word "learned" counts only the sets the save lists as learned; a set held only in an inventory is not learned until it is used in the game.
  const nLearned = bs ? visible.filter((r) => r.learned).length : 0, nOnlyInv = bs ? nHave - nLearned : 0;
  const cards = bs ? '' : html`<div class="row">${CT.groups.map((g) => html`<div class="card d-panel setcard ${ctGroup == g.id ? 'on' : ''}"><button class="b sec sm d-btn sm" data-ctg="${g.id}">${g.name}</button>
      <div class="mut">${g.count} cosmetics${g.requirement ? ' · ' + g.requirement : ''}</div>${CT.libraryAvailable ? html`<button class="b sm d-btn primary sm" data-act="ctUnlockSet" data-id="${g.id}" data-name="${g.name}">Unlock set</button> ` : ''}<button class="b sec sm d-btn sm" data-act="ctGrantSet" data-id="${g.id}" data-name="${g.name}">Give as items</button></div>`)}</div>`;
  setHTML($('#cthead'), html`${cards}<div class="sub">${[['all', 'All', visible.length], ['have', bs ? 'Learned or in inventory' : 'Unlocked or in inventory', nHave], ['missing', 'Not owned', visible.length - nHave]].map(([k, v, n]) => html`<button data-ctf="${k}" class="${ctFilter == k ? 'on' : ''}">${v} (${n})</button>`)}</div>
    <div class="row"><input id="ctq" aria-label="Filter the list" placeholder="Filter by name, item id or group" size="34" value="${ctQ}"><button class="b sec d-btn" data-act="ctfilter">Filter</button><button class="b sec d-btn" data-act="ctclear">Clear</button>
    ${bs ? html`<select id="ctgrp" aria-label="Show one group"><option value="">All groups</option>${groupNames.map((g) => html`<option ${g == ctGroup ? 'selected' : ''}>${g}</option>`)}</select><label><input type="checkbox" id="ctexp" ${ctExp ? 'checked' : ''}> Show experimental</label>` : (ctGroup ? html`<button class="b sec d-btn" data-ctg="">All sets</button>` : '')}
    <span class="mut">${countText(rows.length, visible.length, 'item', 'items', !!(ctQ.trim() || ctGroup || ctFilter != 'all'))}${rows.length > LISTMAX ? `, the first ${LISTMAX} shown` : ''}; ${bs ? `${nLearned.toLocaleString()} of ${visible.length.toLocaleString()} learned${nOnlyInv ? ` (${nOnlyInv.toLocaleString()} more only in an inventory)` : ''}` : `${nHave.toLocaleString()} of ${visible.length.toLocaleString()} ${CT.libraryAvailable ? 'unlocked or in inventory' : 'in inventory'}`}.</span></div>`);
  setHTML($('#ctout'), tbl([{ k: 'name', label: bs ? 'Building set' : 'Customization', f: (r) => html`<span title="${r.id}">${r.name}</span>${r.experimental ? html` <span class="tag">experimental</span>` : ''}` }, { k: 'id', label: 'Item id', cls: 'mut' },
    { k: 'group', label: bs ? 'Group' : 'Set' }, { k: 'requiredDlc', label: 'Requires', f: (r) => r.requiredDlc || (r.entitlement ? 'Account entitlement' : '') },
    { k: 'learned', label: 'State', f: (r) => (r.learned ? html`<span class="ok">Learned</span>` : r.unlocked ? html`<span class="ok">Unlocked</span>` : r.inInventory ? html`<span class="warn">In inventory</span>` : html`<span class="mut">Not owned</span>`) }], rows.slice(0, LISTMAX),
    { empty: bs ? 'No building set matches the filter.' : 'No customization matches the filter.', actions: (r) => html`${!bs && CT.libraryAvailable && r.unlockable && !r.unlocked ? html`<button class="b sm d-btn primary sm" data-act="ctUnlock" data-id="${r.id}" data-name="${r.name}" data-ent="${r.entitlement ? '1' : ''}" title="${r.entitlement ? 'Needs an account entitlement: the game removes it if your account does not own it' : ''}">Unlock</button> ` : ''}${r.inCatalog ? html`<button class="b ${!bs && CT.libraryAvailable && r.unlockable ? 'sec ' : ''}sm d-btn sm ${!bs && CT.libraryAvailable && r.unlockable ? '' : 'primary'}" data-act="ctGive" data-id="${r.id}">Give</button>` : ''}` }));
}

// ---------- BASES
// ---------- BASES (the console's Bases panel inside Player > Bases: the base list, and for one base Power, Water, Inventory and Land Claim Editor)
// The automatic refill and repair card, shown on both the Power and the Water tab because the pass covers both: a master switch and one checkbox per job (base water and power, repairs, fills).
// A job that is not available yet is listed, greyed, with the reason the server gives.
function autoRefillCard(prefs) {
  const jobs = prefs.jobs || [];
  return html`<div class="card d-panel"><h3 class="d-head">Automatic refill and repair when the editor opens</h3><div class="row"><span>When the editor opens: <b>${prefs.autoRefillOnOpen ? 'on' : 'off'}</b></span><button class="b sec d-btn" data-act="autoref" data-on="${prefs.autoRefillOnOpen ? '1' : ''}">${prefs.autoRefillOnOpen ? 'Turn off' : 'Turn on'}</button></div>
    ${jobs.length ? html`<div role="group" aria-label="What the automatic pass does">${jobs.map((j) => html`<div class="row">${tip('ajt-' + j.key, j.desc, html`<label class="${j.ready ? '' : 'mut'}"><input type="checkbox" data-autojob="${j.key}" aria-describedby="${j.ready ? 'ajt-' + j.key : 'aj-' + j.key + ' ajt-' + j.key}" ${j.on ? 'checked' : ''} ${j.ready ? '' : 'disabled'}> <b>${j.label}</b></label>`)}${j.ready ? '' : html`<span class="mut" id="aj-${j.key}">Not available yet: ${j.reason}.</span>`}</div>`)}</div>` : ''}
    ${prefs.copy ? html`<p class="warn" id="autocopy">Not run: this is a copy, not the game's save. The automatic pass only runs on the game's own save.</p>` : ''}
    ${prefs.last ? html`<p class="mut">Last automatic save: ${prefs.last}</p>` : ''}
    ${prefs.lastProblem ? html`<p class="bad">${prefs.lastProblem}</p>` : ''}
    ${prefs.problem ? html`<p class="bad">${prefs.problem}</p>` : ''}
    ${prefs.notice ? html`<p class="warn" role="status" id="settingsnotice">${prefs.notice}</p>` : ''}
    <div class="row"><button class="b d-btn primary" data-act="autoNow" title="Runs the jobs ticked above once, whatever the switch above says. If the game may still hold the save, it is queued and runs by itself once the game is closed and the save is free.">Run the automatic pass now</button></div>
    ${(prefs.autoRefillOnOpen || prefs.waitingByHand) && prefs.waiting && !prefs.copy ? html`<p class="mut" id="autowait">Waiting: ${prefs.waitingWhy || 'the game may still hold the save (see the banner above)'}. It runs by itself as soon as that clears.</p>` : ''}
    ${about('bases.autorefill', 'Automatic refill and repair', html`<p class="mut">When the master switch is on, opening the editor starts a countdown of 15 seconds at the top of the page; when it ends, the jobs ticked above run as <b>one edit</b> that <b>saves straight away</b>, like every other change, so it applies the next time the game loads. <b>Skip</b> ends the countdown for this run, <b>Turn it off</b> turns the switch off, and starting the editor with <b>-no-auto</b> skips the pass for that run. The countdown starts when the page first shows it, so nothing runs before you can stop it. <b>Run the automatic pass now</b> runs the ticked jobs once, whenever you like. Every job is ticked by default, the repairs too; the master switch is off until you turn it on. It never runs on a copy (a save that is not the game's own); there, use the Fill and Refill buttons by hand. It acts only on what your character owns or carries: your own vehicles, worn equipment, loadout, inventories and bases. It does not write while the game may still hold the save: a single-player session, the game having written its database a moment ago, or a session having ended a moment ago. It then waits and runs by itself once the game is closed and the save is free. Pressed by hand at the game's main menu a while after you left the world, it runs at once. When nothing needs doing nothing is written. If one job fails, nothing is changed and the error is shown here. The previous file is backed up first. Remembered between runs. The same card is on the Power and Water tabs.</p>`)}</div>`;
}
// The console's Land Claim Editor: the claim as a grid of cells turned by the base's own yaw (north at the top), with the totem's cell,
// the cells it has, and dotted "available" cells next to it. Click an available cell to add it, a chosen one to take it back (what is
// chosen must stay connected edge to edge), pick a vertical level, then Apply. Clicking an existing cell that holds no building piece or placeable marks it
// for removal (shown as red), and Remove takes the marked cells out; the claim must stay connected, and the server refuses a cell with pieces in it as well.
let LC = null; // { claim, sel: Set of "x,y" to add, rm: Set of "x,y" to remove, level }
let TD = []; // the places the drop-down of Go to a place holds
let TDR = null; // what the last answer said about them: { map, sameMap, total, groups } (groups: the headings that have a place, in order)
// The Movement card's boxes (#506). A redraw builds them again from the save, and the save moves on by itself (the game rewrites the character's position every time it saves),
// so what a person chose or typed here is kept in this object and the card is drawn from it: a value is in GO.vals once the person (or a choice in the list of places) has set it,
// and the draw uses it instead of the saved position. It is cleared by a successful Teleport (the boxes follow the saved position again) and when the character's map in the
// save changes while the boxes are meant for "this map". pick is the index in TD of the place chosen in the list, null when none (or the coordinates were typed over it).
const GO = { vals: {}, pick: null, savedMap: null };
const GO_IDS = ['tx', 'ty', 'tz', 'tdyaw', 'tmap'];
const goVal = (id, saved) => (Object.hasOwn(GO.vals, id) ? GO.vals[id] : saved);
function goClear() { GO.vals = {}; GO.pick = null; TD = []; TDR = null; TDSEEN = ''; }
function goSavedMap(m) {
  if (GO.savedMap !== null && GO.savedMap !== m && !GO.vals.tmap) goClear(); // the boxes meant "this map", and this map is another one now
  GO.savedMap = m;
}
function goKeep(t) { // a box was set by the person: remember it; a typed coordinate is no longer the place that was chosen
  if (!t || !GO_IDS.includes(t.id)) return;
  GO.vals[t.id] = String(t.value);
  if (t.id === 'tx' || t.id === 'ty' || t.id === 'tz') { GO.pick = null; const s = $('#tdsel'); if (s) s.value = ''; goNote(); }
}
function goSet(id, v) { GO.vals[id] = String(v); const el = $('#' + id); if (el) el.value = String(v); }
// How far a place is: only for a place on the map the character is on; a hub that is another map says so instead (#521).
const tdAway = (d) => (d.otherMap ? 'another map' : (TDR && TDR.sameMap) || d.map ? `${Math.round(d.distance / 100).toLocaleString()} m away` : '');
// Says which place Teleport is aimed at while a choice from the list stands.
function goNote() {
  const n = $('#tdpick'), d = GO.pick === null ? null : TD[GO.pick];
  if (n) { n.textContent = d ? `Selected: ${placeName(d)}${tdAway(d) ? ', ' + tdAway(d) : ''}` : ''; if (d && d.detail) n.title = d.detail; else n.removeAttribute('title'); }
  tmapSync(); // a hub chosen in the list is another map, whatever the Map box says (#521)
}
const OTHER_MAPS_HEAD = 'Other maps (moves there are untested)';
// A map in plain words, as the Map box names it ("Hagga Basin", "the Deep Desert"); a map the box does not offer keeps its own name.
const mapName = (key) => { const m = TRAVEL_MAPS.find(([k]) => k === key); return m ? m[1].replace(/^The /, 'the ') : key; };
// The drop-down of places, drawn from the last answer and the choice made in it, so a redraw of the page loses neither.
function tdDraw() {
  const out = $('#tdout');
  if (!out) return;
  if (!TDR) { setHTML(out, html``); return; }
  const more = TDR.total > TD.length ? ` (showing ${TD.length} of ${TDR.total})` : '';
  const opt = (d, i) => html`<option value="${i}" title="${d.detail || ''}" ${GO.pick === i ? 'selected' : ''}>${placeName(d)}${tdAway(d) ? ' - ' + tdAway(d) : ''}</option>`;
  // The places are in groups (Trading posts, Personal bases, Vehicles, Respawn points, Taxis; #500, #519), each under its heading; the value of an option is its place in TD.
  // A hub that is not on the chosen map (offList: it is another map than the one chosen, #521, #825; the hub the character stands in is still one when another map is chosen): it is counted apart and listed last under its own heading, so "N places on <map>" is the places of that map (#607).
  const groups = TDR.groups || [];
  const here = TD.filter((d) => !d.offList).length, other = TD.length - here;
  const mine = (g) => TD.map((d, i) => (!d.offList && (g === null || d.group === g.key) ? opt(d, i) : ''));
  const body = groups.length ? groups.filter((g) => TD.some((d) => !d.offList && d.group === g.key)).map((g) => html`<optgroup label="${g.label}">${mine(g)}</optgroup>`) : mine(null);
  const hubs = other ? html`<optgroup label="${OTHER_MAPS_HEAD}">${TD.map((d, i) => (d.offList ? opt(d, i) : ''))}</optgroup>` : '';
  // The number of other-map places comes before "nearest first": a phone cuts the closed box at its width, and that number is the part that must show.
  const count = `${here ? `${here} place${here == 1 ? '' : 's'}` : 'No places'} on ${mapName(TDR.map)}${other ? `, ${other} on other maps` : ''}${TDR.sameMap && here > 1 ? ', nearest first' : ''}${more}`;
  setHTML(out, TD.length ? html`<select id="tdsel" aria-label="Place to go to"><option value="">${count}</option>${body}${hubs}</select>` : html`No places on ${mapName(TDR.map)}.`);
}
// Go to a place (#519) is one drop-down with no filter box, no Find and no kind of place: it is filled at once from the chosen map (goPlaces), and what is chosen in it is kept by the
// place's name and heading.
const placeKey = (d) => d.group + '|' + d.label;
// What a place is shown as: its name with no id (#618). The label is its identity (placeKey) and carries the number of a base or a vehicle; the ids are in d.detail, for a tooltip.
const placeName = (d) => d.display || d.label;
let TDSEQ = 0; // the newest request for the places: an older answer that arrives later is dropped
let TDSEEN = ''; // the last answer as text: a refresh that finds nothing new draws nothing, so a drop-down that is open stays open
// Fills the drop-down from the chosen map. It runs when another map is chosen and each time the Movement card is drawn from the save (the game saves and the character moves, so the
// distances change); a chosen place stays chosen when it is still in the answer. A failure is shown when the person asked for the list, not on a refresh.
async function goPlaces(asked) {
  const seq = ++TDSEQ;
  try {
    const r = await api(`/api/player/destinations?map=${encodeURIComponent(goVal('tmap', ''))}`);
    if (seq !== TDSEQ) return;
    const text = JSON.stringify(r);
    if (text === TDSEEN && TDR) return;
    TDSEEN = text;
    const keep = GO.pick === null ? null : TD[GO.pick];
    TD = r.destinations; TDR = { map: r.map, sameMap: r.sameMap, total: r.total, groups: r.groups || [] };
    const at = keep ? TD.findIndex((d) => placeKey(d) === placeKey(keep)) : -1;
    GO.pick = at < 0 ? null : at;
    goSync();
  } catch (e) { if (asked) toast(e.message, true); }
}
function goSync() { tdDraw(); goNote(); }
const TRAVEL_NOTE = 'A move to another map writes the character\'s controller, body and player state on the new map. A move to the Deep Desert was checked in the game once, on a test save: the game loaded the character there. No other move was checked, so try it on a copy first. A position typed for the Deep Desert is not accepted for Hagga Basin.';
// What is drawn under Teleport while another map is chosen: the one thing that can hurt, in two sentences. The whole note (what is written, the position rule) is TRAVEL_NOTE, in the Movement card's About this (#426).
const PLACE_NOTE = "For trading posts and hubs the boxes are filled with the point where the game's own ornithopter trip would put you; for other places, with the place's own position. The distance shown is to the point that is used. Two of the trading-post points (Griffin's Reach and Crossroads) are computed from the game's level data and have not been seen in use; the other trading posts and the hubs match trips that were seen. A Deep Desert taxi is the spot beside it in the game's level data and has not been tried in the game yet. Whether a save edit moves the character into another map is not proven; do it on a copy first, with the game closed, and expect to need the in-game taxi if it does not work.";
const TRAVEL_WARN = 'Only one move between maps was checked in the game: to the Deep Desert (2026-10-27, one character, a throwaway save). No other move was checked, so try it on a copy first.';
const TRAVEL_MAPS = [['HaggaBasin', 'Hagga Basin'], ['DeepDesert', 'The Deep Desert']]; // the maps the editor moves the character to (the console offers the same two for an offline player)
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
// What a cell says to a screen reader (the <title> is only a tooltip): where it is and what pressing it does.
function lcCellName(x, y, sel, marked, removable) {
  if (sel) return `Cell ${x}, ${y}: chosen to be added. Press to take it back.`;
  if (marked) return `Cell ${x}, ${y}: marked for removal. Press to keep it.`;
  if (removable) return `Cell ${x}, ${y}: part of the claim. Press to mark it for removal.`;
  return `Cell ${x}, ${y}: available. Press to add it.`;
}
// A click on a cell of the grid: an existing cell is marked for removal or unmarked, any other is added or taken back.
function lcCellClick(k) {
  const existing = new Set((LC.claim.segments || []).map((s) => lcKey(s.x, s.y)));
  if (existing.has(k)) { if (LC.rm.has(k)) LC.rm.delete(k); else LC.rm.add(k); return; }
  lcToggle(k);
}
function claimEditorDraw(focusKey) {
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
    return html`<rect x="${x - 0.44}" y="${y - 0.44}" width="0.88" height="0.88" rx="0.08" class="lc-cell ${kind}" ${avail || sel || removable ? html`data-cell="${k}" tabindex="0" role="button" aria-pressed="${sel || marked ? 'true' : 'false'}" aria-label="${lcCellName(x, y, sel, marked, removable)}"` : ''}><title>cell ${x}, ${y}${origin ? ' (the totem)' : have && pieces.get(k) ? ` (holds ${pieces.get(k)} building pieces or placeables: cannot be removed)` : removable ? (marked ? ' (marked for removal: click to keep it)' : ' (click to mark it for removal)') : ''}</title></rect>${label !== '' ? html`<text x="${x}" y="${y + 0.09}" transform="rotate(${-yaw} ${x} ${y})" class="lc-label">${label}</text>` : ''}`;
  });
  setHTML(host, html`<div class="card d-panel"><h3 class="d-head">Land claim editor</h3>
    <div class="grid">${kv('Original yaw', Math.round(yaw * 10) / 10 + '°')}${kv('Horizontal segments', c.cells - 1)}${kv('Vertical level', c.level + ' / ' + c.maxLevel)}</div>
    <details class="ids"><summary>Claim ids</summary><div class="grid">${kv('Totem', c.totem_id)}</div></details>
    <div class="lc-work"><div class="lc-gridcard"><b>Horizontal claim</b>
      <svg class="lc-svg" viewBox="${vx} ${vy} ${vw} ${vh}">${compass}<g transform="rotate(${yaw} 0 0)">${cells}
        <path d="M0.5 0 H1.9" class="lc-axis lc-axis-x"></path><path d="M1.9 0 l-0.25 -0.14 v0.28 z" class="lc-axis-head lc-axis-x"></path><text x="2.35" y="0.12" transform="rotate(${-yaw} 2.35 0)" class="lc-axis-t lc-axis-x">+x</text>
        <path d="M0 0.5 V1.9" class="lc-axis lc-axis-y"></path><path d="M0 1.9 l-0.14 -0.25 h0.28 z" class="lc-axis-head lc-axis-y"></path><text x="0" y="2.5" transform="rotate(${-yaw} 0 2.5)" class="lc-axis-t lc-axis-y">+y</text></g></svg>
      <p class="mut"><span class="lc-key lc-o"></span>Totem <span class="lc-key lc-x"></span>Existing <span class="lc-key lc-s"></span>New <span class="lc-key lc-a"></span>Available <span class="lc-key lc-r"></span>To remove</p>
</div>
    <div class="lc-side"><b>Expansion</b>
      ${tip('lcleveltip', `Vertical expansion grows equally up and down. The game caps it at level ${c.maxLevel}.`, html`<label>Vertical level <select id="lcLvl" aria-describedby="lcleveltip">${Array.from({ length: maxLevel - c.level + 1 }, (_, i) => c.level + i).map((n) => html`<option ${n === LC.level ? 'selected' : ''}>${n}</option>`)}</select></label>`)}
      <div class="kv"><b>Pending horizontal</b><span>${LC.sel.size}</span></div><div class="kv"><b>Pending vertical</b><span>${LC.level === c.level ? 'No change' : c.level + ' → ' + LC.level}</span></div>
      <div class="row"><button class="b d-btn primary" data-act="lcApply" ${dirty ? '' : 'disabled'}>Apply changes</button>${LC.sel.size || LC.rm.size ? html`<button class="b sec d-btn" data-act="lcClear">Clear selection</button>` : ''}</div>
      <div class="kv"><b>Marked for removal</b><span>${LC.rm.size}</span></div>
      ${cutOff.length ? html`<p class="bad">Removing those would cut ${cutOff.length} cell${cutOff.length === 1 ? '' : 's'} that stay off from the totem; the game needs the claim connected edge to edge. Mark them too, or unmark a cell.</p>` : ''}
      <div class="row"><button class="b d-btn primary" data-act="lcRemove" ${LC.rm.size && !cutOff.length ? '' : 'disabled'}>Remove marked cells</button></div>
      ${LC.sel.size ? html`<p class="mut" id="lcbypass">This editor bypasses the game's normal staking checks. Claiming a protected area does not guarantee that the game lets you build there.</p>` : ''}</div></div>
    ${about('bases.claim', 'Land claim editor', html`<p class="mut">Click a dotted cell to add it. New cells must stay connected edge to edge. A number is how many building pieces a cell holds.</p>
      <p class="mut">North is up. Column <b>+x</b> runs <b>${dirX}</b>, row <b>+y</b> runs <b>${dirY}</b> (the base's yaw is ${Math.round(yaw * 10) / 10}°). World-oriented view: north stays at the top and the grid is turned by the base's original yaw; the saved cell coordinates are not changed.</p>
      <p class="mut">Click an existing cell to mark it. A cell with building pieces or placeables in it (its number is shown) cannot be removed, and neither can the totem's own cell.</p>
      <p class="mut">The edit is written at once, after the usual backup. Applied when the game next loads the save. This editor bypasses the game's normal staking checks. Claiming a protected area does not guarantee that the game lets you build there.</p>`)}</div>`);
  // The grid was drawn again, so the cell that had the keyboard focus is gone: put the focus on the same cell (or, when it is no longer a cell to press, on Apply).
  if (focusKey) {
    const again = [...host.querySelectorAll('[data-cell]')].find((c) => c.getAttribute('data-cell') === focusKey) || host.querySelector('[data-act="lcApply"]');
    if (again) again.focus();
  }
}

// ---------- BASE BACKUPS (#475, phase 1): the bases the game's own tool picked up, each with a button that saves it as a file. The export only reads the save.
// The file reaches the browser as text and is saved as it is, never parsed in the page: a 64-bit entity id in it stays exact. The browser's own save dialog says where it goes.
async function baseBackupDownload(d) {
  const r = await fetch('/api/bases/backups/export?backup=' + encodeURIComponent(d.id), { credentials: 'same-origin' });
  if (!r.ok) { const j = await r.json().catch(() => ({})); throw new Error(j.error || r.statusText || 'The export failed'); }
  const b = await r.blob();
  const u = URL.createObjectURL(b), l = document.createElement('a');
  l.href = u;
  l.download = 'tabr-tau-base-backup-' + String(d.id).replace(/[^0-9]/g, '') + '-' + (String(d.name || '').replace(/[^A-Za-z0-9._-]+/g, '_').replace(/^[._-]+|[._-]+$/g, '').slice(0, 40) || 'base') + '.json';
  l.click();
  setTimeout(() => URL.revokeObjectURL(u), 60000); // the browser reads the blob after the click returns
  toast(`Exported "${d.name}": ${Number(d.pieces).toLocaleString()} pieces, ${Number(d.placeables).toLocaleString()} placeables, ${Number(d.items).toLocaleString()} stored items. Nothing in the save changed.`);
}
// The Dune console's own file (#534), from a base that is still standing (Bases list) or from a base backup (Base backups card). The editor answers with the file and, in two headers,
// the counts of what the file holds and any notes about it; the file itself is saved as it is and never parsed here. d.kind is 'base' (d.id is the totem) or 'backup'.
const consoleFileName = (kind, id, name) => 'tabr-tau-console-' + kind + '-' + String(id).replace(/[^0-9]/g, '') + '-' + (String(name || '').replace(/[^A-Za-z0-9._-]+/g, '_').replace(/^[._-]+|[._-]+$/g, '').slice(0, 40) || 'base') + '.json';
async function baseConsoleDownload(d) {
  const url = d.kind === 'backup' ? '/api/bases/backups/export?format=console&backup=' + encodeURIComponent(d.id) : '/api/bases/console-export?base=' + encodeURIComponent(d.id);
  const r = await fetch(url, { credentials: 'same-origin' });
  if (!r.ok) { const j = await r.json().catch(() => ({})); throw new Error(j.error || r.statusText || 'The export failed'); }
  let counts = null, notes = [];
  try { counts = JSON.parse(r.headers.get('X-Base-Counts')); } catch (e) { /* the numbers of the row are used */ }
  try { notes = JSON.parse(decodeURIComponent(r.headers.get('X-Base-Notes') || '[]')); } catch (e) { notes = []; }
  const b = await r.blob();
  const u = URL.createObjectURL(b), l = document.createElement('a');
  l.href = u;
  l.download = consoleFileName(d.kind, d.id, d.name);
  l.click();
  setTimeout(() => URL.revokeObjectURL(u), 60000);
  const n = (v) => Number(v).toLocaleString();
  const c = counts || { pieces: d.pieces, placeables: d.placeables, storedItems: d.items };
  const items = c.storedItems == null ? '' : `, ${n(c.storedItems)} stored items`;
  toast(`Exported "${d.name}" for the Dune console: ${n(c.pieces)} pieces, ${n(c.placeables)} placeables${items}. Nothing in the save changed.${notes.length ? ' ' + notes.join(' ') : ''}`, null, notes.length ? 15000 : 3500);
}
// Import (phase 2): a base file is chosen, read in the browser as text, and sent to the editor, which checks it whole and answers with a preview (the import, rolled back). Nothing is
// written until Import is pressed; the file's text is kept in the page only so that Import can send the file that was previewed (with the hash the preview gave).
let bbImport = null; // { name, text, pv } after a preview, { name, err } after a refusal
const BASE_FILE_MAX = 16 * 1024 * 1024;
function baseImportPick(input) {
  const f = input.files && input.files[0];
  input.value = '';
  if (!f) return;
  if (f.size > BASE_FILE_MAX) { bbImport = { name: f.name, err: 'That file is larger than 16 MiB: a base file is smaller, so this is not one.' }; render(true); return; }
  (async () => {
    let text;
    try { text = await f.text(); } catch (e) { bbImport = { name: f.name, err: 'The file could not be read.' }; render(true); return; }
    try { bbImport = { name: f.name, text, pv: await api('/api/bases/backups/import-preview', { text }) }; } catch (e) { bbImport = { name: f.name, err: e.message }; }
    render(true);
  })();
}
function baseImportPanel() {
  const b = bbImport;
  if (!b) return '';
  if (b.err) return html`<div class="info" role="alert" id="baseImportPanel"><b class="bad">${b.name}</b>: ${b.err} <button class="b sec sm d-btn sm" data-act="baseImportCancel">Dismiss</button></div>`;
  const p = b.pv, s = p.summary, n = (v) => Number(v).toLocaleString();
  return html`<div class="info" role="status" id="baseImportPanel"><b>${b.name}: ${p.name}</b>
    ${p.from ? html`<p>${p.from}</p>` : ''}
    <p>Would add a new base backup of ${n(s.actors)} actors, ${n(s.pieces)} building pieces, ${n(s.placeables)} placeables and ${n(s.storedItems)} stored items (${n(s.rows)} rows) for ${p.owner.character}. Nothing in the save has changed.</p>
    ${p.warnings.length ? html`<ul>${p.warnings.map((w) => html`<li class="warn">${w}</li>`)}</ul>` : ''}
    <div class="row"><button class="b d-btn primary" data-act="baseImportGo">${p.patchesDiffer ? 'Import anyway' : 'Import'}</button><button class="b sec d-btn" data-act="baseImportCancel">Cancel</button></div></div>`;
}
async function baseBackupsCard() {
  let list;
  try { list = await api('/api/bases/backups'); } catch (e) { return html`<div class="card d-panel"><h3 class="d-head">Base backups</h3><p class="bad" role="alert">${e.message}</p></div>`; }
  const n = (v) => Number(v).toLocaleString();
  return html`<div class="card d-panel"><h3>Base backups (${list.length})</h3>${tbl([{ k: 'name', label: 'Name', f: (r) => html`<span title="Base backup number ${r.backup_id}">${r.name}</span>` }, { k: 'map', label: 'Map' }, { k: 'actors', label: 'Actors', f: (r) => n(r.actors) },
    { k: 'pieces', label: 'Building pieces', f: (r) => n(r.pieces) }, { k: 'placeables', label: 'Placeables', f: (r) => n(r.placeables) }, { k: 'stored_items', label: 'Stored items', f: (r) => n(r.stored_items) }], list,
    { actions: (r) => html`<button class="b sec sm d-btn sm" data-act="baseBackupExport" data-id="${r.backup_id}" data-name="${r.name}" data-pieces="${r.pieces}" data-placeables="${r.placeables}" data-items="${r.stored_items}">Export base backup</button>
      ${tip('bbctip' + r.backup_id, "Saves this backup as a Dune console base backup file, for your own self-hosted server only. Reads the save and changes nothing.", html`<button class="b sec sm d-btn sm" data-act="baseConsoleExport" data-kind="backup" data-id="${r.backup_id}" data-name="${r.name}" data-pieces="${r.pieces}" data-placeables="${r.placeables}" data-items="${r.stored_items}" aria-describedby="bbctip${r.backup_id}">Export for console</button>`)}`,
      empty: "No base backups in this save. Pick a base up with the game's base tool, then reload." })}
    <div class="row"><label>Import a base file <input type="file" id="baseFile"></label></div>
    ${baseImportPanel()}
    ${about('bases.backups', 'Base backups', html`<p class="mut">A base backup is what the game's base tool leaves when you pick a base up: its buildings stay in the save and the claim is gone. Export saves one as a <b>tabr-tau-base</b> file (version 1) with every row of the base and its original ids, a manifest of the counts, and a checksum. It only reads the save and changes nothing in it.</p>
      <p class="mut">Import reads a file like that and adds its base to this save as a new base backup, for the game's Base Reconstruction Tool to place; the editor does not put the pieces in the world. Choosing a file shows what would be added and changes nothing; Import then writes it as one edit, after a backup (or queued while the game runs). Every id in the file gets a new one, the character of this save becomes the owner, and nothing already in the save is changed or replaced. A file that is not a good base file is refused whole. Importing the same file twice adds two backups.</p>
      <p class="mut"><b>Export for console</b> saves a base as a <b>dune-base-backup</b> file (version 1), the base backup file of the self-hosted Dune console, which a self-hosted Dune server of your own can import. It is for your own self-hosted server only, not for a console game or a public or another person's server. On the Bases list it works for a base that is still standing: the file holds what a pick-up followed by an export in the Dune console would give (every actor of the base in the backed-up state, only the pieces its totem owns, its storage and crafting stations, the claim, and the owner as a placeholder that the importing player replaces). On a base backup it writes the backup as it is. It only reads the save. The editor reads the file it made before it hands it out; whether a game accepts it can only be tried in a game. A Solido blueprint file is not written yet.</p>
      <p class="mut">A <b>dune-base-backup</b> file, the base backup export of the self-hosted Dune console, is read the same way: the editor turns it into the same rows (the transform lists become plain columns, every id in it becomes a new id), says in the preview that it came from the Dune console, and refuses the whole file for anything it cannot place in a save (market, sinkchart or Solido entries, a field it does not know).</p>`)}</div>`;
}

let bsTab = 'power', bsOpen = null, bsGroup = 'storage';
let bsInvOpen = null; // { id, name }: the container opened with Open in the base's Inventory list. It stays open when the page is drawn again (after a delete, an edit or the page's own refresh) and goes with a move to another base, sub-tab, group or page (#731)
async function basesView() {
  const list = await api('/api/bases/list');
  const id = list.some((x) => x.base_id == bsOpen) ? bsOpen : (list[0] ? list[0].base_id : null);
  const base = list.find((x) => x.base_id == id);
  const h = [html`<div class="card d-panel"><h3>Bases (${list.length})</h3>${tbl([{ k: 'name', label: 'Base name', f: (r) => html`<span title="Base number ${r.base_id}">${r.name}</span>${r.picked_up ? html` <span class="tag" title="Picked up with the game's base tool: its buildings are still in the save, but nobody has claimed it.">Picked up</span>` : ''}` }, { k: 'base_type', label: 'Base type' }, { k: 'owner', label: 'Owner' }, { k: 'map', label: 'Map' },
    { k: 'generators', label: 'Generators' }, { k: 'pieces', label: 'Building pieces' }, { k: 'placeables', label: 'Placeables' }, { k: 'x', label: 'Coordinates', f: (r) => html`${fix(r.x)}, ${fix(r.y)}, ${fix(r.z)}${r.region ? html` <span class="mut">near ${r.region}</span>` : ''}` }], list,
    { actions: (r) => html`${r.base_id == id ? html`<span class="tag">Open below</span>` : html`<button class="b sec sm d-btn sm" data-bsopen="${r.base_id}">Open</button>`}${r.picked_up ? '' : tip('bcetip' + r.base_id, "Saves this base, with what is stored in it, as a Dune console base backup file, for your own self-hosted server only. Reads the save and changes nothing.", html`<button class="b sec sm d-btn sm" data-act="baseConsoleExport" data-kind="base" data-id="${r.base_id}" data-name="${r.name}" data-pieces="${r.pieces}" data-placeables="${r.placeables}" aria-describedby="bcetip${r.base_id}">Export for console</button>`)}`, empty: 'No bases yet: build one in the game, then reload.' })}</div>`];
  if (base) {
    const tabs = [['power', 'Power'], ['water', 'Water'], ['inventory', 'Inventory'], ['claim', 'Land claim editor'], ['structures', 'Structures']];
    h.push(html`<div class="card d-panel"><h3 title="Base number ${base.base_id}">${base.name}</h3><div class="sub">${tabs.map(([k, v]) => html`<button data-bstab="${k}" class="${bsTab == k ? 'on' : ''}">${v}</button>`)}</div></div>`);
    if (bsTab == 'power') {
      const pw = await api('/api/bases/power?base=' + id);
      const prefs = await api('/api/settings');
      h.push(html`<div class="card d-panel"><h3 class="d-head">Power</h3>${pw.types.length ? tbl([{ k: 'name', label: 'Building' }, { k: 'devices', label: 'Built' },
        { k: 'queued', label: 'Holds', f: (r) => Number(r.queued).toLocaleString() + (r.fuel ? ' ' + itemName(r.fuel) : '') }, { k: 'capacity', label: 'Full', f: (r) => (r.capacity ? Number(r.capacity).toLocaleString() : '-') }, { k: 'empty', label: 'Empty' }], pw.types) : html`<p class="mut">No generators, wind turbines or windtraps built at this base.</p>`}
        <div class="row">${tip('gentip', 'Tops up Fuel-Powered and Spice-Powered Generators (499), Omnidirectional Wind Turbines (Low-grade Lubricant) and Directional Wind Turbines (Industrial-grade Lubricant), five stacks of 100 and 499 in all, and Windtraps and Large Windtraps (5 filters, always the best filter for the machine). Buildings that are only placeholders are left alone.', html`<button class="b d-btn primary" data-act="refillGen" aria-describedby="gentip">Refill power (every base)</button>`)}</div></div>
    ${autoRefillCard(prefs)}`);
    } else if (bsTab == 'water') {
      const w = await api('/api/bases/water?base=' + id);
      h.push(html`<div class="card d-panel"><h3 class="d-head">Water</h3>${w.types.length ? tbl([{ k: 'name', label: 'Building' }, { k: 'devices', label: 'Built' }, { k: 'stored', label: 'Water stored', f: (r) => Number(r.stored).toLocaleString() },
        { k: 'capacity', label: 'Capacity', f: (r) => (r.capacity ? Number(r.capacity).toLocaleString() : '-') }, { k: 'fillPercent', label: 'Fill', f: (r) => (r.fillPercent == null ? '-' : Math.round(r.fillPercent) + '%') }], w.types) : html`<p class="mut">No water storage at this base.</p>`}
        <div class="row">${tip('watertip', 'Fills every Water Cistern, Medium Water Cistern, Large Water Cistern, Windtrap, Large Windtrap and Blood Purifier to its capacity. The blood in a Blood Purifier is not touched, and a building that is only a placeholder is left alone.', html`<button class="b d-btn primary" data-act="refillWater" aria-describedby="watertip">Refill base water (every base)</button>`)}</div></div>
    ${autoRefillCard(await api('/api/settings'))}`);
    } else if (bsTab == 'inventory') {
      const inv = await api('/api/bases/inventory?base=' + id);
      const grp = inv.groups.some((g) => g.id == bsGroup && g.count) ? bsGroup : (inv.groups.find((g) => g.count) || inv.groups[0]).id;
      const rows = inv.containers.filter((c) => c.group == grp);
      // the container that was open (#731) is built here, before the page is drawn, so the redraw shows it at once: it is not emptied and filled again (no flash, no jump of the page, and the Add item box is never missing while it is typed in)
      let open = '';
      if (bsInvOpen) {
        const c = inv.containers.find((x) => x.inventory_id == bsInvOpen.id);
        if (!c) bsInvOpen = null; // no longer one of this base's containers
        else { try { open = await baseInvCard({ id: c.inventory_id, name: c.name }); } catch (e) { bsInvOpen = null; } }
      }
      h.push(html`<div class="card d-panel"><h3 class="d-head">Inventory</h3><div class="sub">${inv.groups.map((g) => html`<button data-bsgrp="${g.id}" class="${grp == g.id ? 'on' : ''}">${g.label} (${g.count})</button>`)}</div>
        ${tbl([{ k: 'name', label: 'Container' }, { k: 'type', label: 'Type' }, { k: 'items', label: 'Items' }, { k: 'max_item_count', label: 'Slots used', f: (r) => `${r.items} of ${r.max_item_count}` }], rows,
        { actions: (r) => html`<button class="b sec sm d-btn sm" data-act="openInv" data-id="${r.inventory_id}" data-name="${r.name}">Open</button>`, empty: 'No containers in this group.' })}
        ${about('bases.inventory', 'Inventory', html`<p class="mut">Only containers that belong to this base are listed.</p>`)}</div><div id="inv">${open}</div>`);
    } else if (bsTab == 'structures') {
      // The open base's own structures (#526): the health card, the placeables and the piece types list only what this base's totem owns. Pieces no base owns are listed apart,
      // and the only whole-save figures and buttons are in the last card, labelled as such.
      const b = await api('/api/bases/structures?base=' + id);
      const p = b.pieces || {};
      const un = b.unowned || {}, up = un.pieces || {};
      const sv = b.save || {};
      const otherPieces = Math.max(0, (Number(sv.pieces) || 0) - (Number(p.n) || 0) - (Number(up.n) || 0));
      const otherPlaceables = Math.max(0, (Number(sv.placeables) || 0) - b.placeables.length - (un.placeables || []).length);
      h.push(html`<div class="card d-panel"><h3 class="d-head">Structure health</h3>${Number(p.n) ? html`<div class="grid">${kv('Building pieces', p.n)}${kv('Lowest health', fix(p.minh))}${kv('Average health', fix(p.avgh))}${kv('Total sand buildup', fix(p.sand))}</div>` : html`<p class="mut">This base has no building pieces.</p>`}
        <div class="row">${tip('repairbtip', 'Raises the pieces and placeables of this base to the highest health seen for their type on standing pieces of your bases (backups do not count, and a cap per type applies). Placeable holograms (unfinished placements) are left as they are. Other bases are not changed, and nothing is lowered.', html`<button class="b d-btn primary" data-act="repairB" data-id="${id}" aria-describedby="repairbtip">Repair this base to max</button>`)}<button class="b sec d-btn" data-act="sand" data-id="${id}" data-confirm="Yes, clear sand on this base">Clear sand on this base</button></div>
        ${about('bases.structures', 'Structures', html`<p class="mut">The health card, the placeables and the piece types list only what belongs to this base: the pieces and placeables whose owner is this base's totem. Pieces that no base owns are listed apart under Not part of any base and are never counted into a base. Repair this base to max and Clear sand on this base change only this base's rows, and both repairs leave placeable holograms (unfinished placements) as they are. The highest health of a type is looked up over the standing pieces of all your bases (not backups, and never above a cap per type), so a base whose pieces are all worn is still repaired. The last card has the whole-save counts and the two buttons that act on every base.</p>`)}</div>`);
      h.push(html`<div class="card d-panel"><h3>Placeables (${b.placeables.length})</h3>${tbl([{ k: 'building_type', label: 'Type', f: (r) => html`<span title="Placeable number ${r.id}">${r.building_type}</span>` }, { k: 'health', label: 'Health', f: (r) => (r.health == null ? '' : fix(r.health)) }], b.placeables, { empty: 'No placeables at this base.' })}</div>
        <div class="card d-panel"><h3 class="d-head">Building piece types</h3>${tbl([{ k: 'building_type', label: 'Type' }, { k: 'n', label: 'Count' }, { k: 'minh', label: 'Min health' }, { k: 'maxh', label: 'Max health' }, { k: 'avgh', label: 'Avg' }], b.types, { empty: 'No building pieces at this base.' })}</div>`);
      if (Number(up.n) || (un.placeables || []).length) {
        h.push(html`<div class="card d-panel"><h3 class="d-head">Not part of any base</h3><p class="mut">${Number(up.n) || 0} building pieces and ${(un.placeables || []).length} placeables have no owner that is a base. They are not counted above, and the buttons for this base leave them alone.</p>
          ${Number(up.n) ? tbl([{ k: 'building_type', label: 'Type' }, { k: 'n', label: 'Count' }, { k: 'minh', label: 'Min health' }, { k: 'maxh', label: 'Max health' }, { k: 'avgh', label: 'Avg' }], un.types || []) : ''}
          ${(un.placeables || []).length ? tbl([{ k: 'building_type', label: 'Type', f: (r) => html`<span title="Placeable number ${r.id}">${r.building_type}</span>` }, { k: 'health', label: 'Health', f: (r) => (r.health == null ? '' : fix(r.health)) }], un.placeables) : ''}</div>`);
      }
      h.push(html`<div class="card d-panel"><h3 class="d-head">Whole save</h3><p class="mut">The save has ${sv.pieces} building pieces and ${sv.placeables} placeables: ${p.n} building pieces and ${b.placeables.length} placeables at this base, ${otherPieces} and ${otherPlaceables} at other bases, ${Number(up.n) || 0} and ${(un.placeables || []).length} at no base.</p>
        <div class="row">${tip('repairalltip', 'Raises every building piece and placeable in the save, at every base and at no base, to the highest health seen for its type on standing pieces of your bases (backups do not count, and a cap per type applies). Placeable holograms (unfinished placements) are left as they are, and so are the pieces and placeables of a base backup.', html`<button class="b sec d-btn" data-act="repairAll" aria-describedby="repairalltip">Repair every base to max</button>`)}<button class="b sec d-btn" data-act="sandAll" data-confirm="Yes, clear sand on every base">Clear sand on every base</button></div></div>`);
    } else {
      const claims = (await api('/api/bases/claim')).filter((c) => c.totem_id == id);
      LC = claims.length ? { claim: claims[0], sel: new Set(), rm: new Set(), level: claims[0].level } : null;
      if (LC) h.push(html`<div id="lced"></div>`);
      h.push(html`${claims.length ? html`<div class="card d-panel"><h3 class="d-head">Resize by square</h3>${claims.map((c) => html`<div class="row"><b title="Totem ${c.totem_id}">Claim</b><span>${c.cells} cell${c.cells == 1 ? '' : 's'}${c.irregular ? ' (irregular shape)' : ` (${2 * c.rings + 1} x ${2 * c.rings + 1} square)`}, vertical level ${c.level}</span>
      Grow to <select id="cr${c.totem_id}"><option value="0">no change</option>${Array.from({ length: c.maxRings }, (_, i) => i + 1).filter((n) => n > c.rings).map((n) => html`<option value="${n}">${2 * n + 1} x ${2 * n + 1}</option>`)}</select>
      Level <select id="cv${c.totem_id}">${Array.from({ length: c.maxLevel + 1 }, (_, i) => i).filter((n) => n >= c.level).map((n) => html`<option value="${n}" ${n == c.level ? 'selected' : ''}>${n}</option>`)}</select>
      <button class="b d-btn primary" data-act="claimGrow" data-id="${c.totem_id}" data-level="${c.level}">Expand claim</button></div>
      ${c.cells > 1 ? html`<div class="row"><span class="mut">${c.piecesInClaim == null ? '' : `${c.piecesInClaim} pieces inside the claim, ${c.piecesOutside} outside.`}</span>
        Shrink to <select id="cs${c.totem_id}"><option value="0">the totem's own cell only</option>${Array.from({ length: c.maxRings }, (_, i) => i + 1).filter((n) => (c.irregular ? n <= c.rings : n < c.rings)).map((n) => html`<option value="${n}">${2 * n + 1} x ${2 * n + 1} square</option>`)}</select>
        ${tip('shrinktip' + c.totem_id, 'Cells that hold building pieces are never removed.', html`<button class="b sec d-btn" data-act="claimShrink" data-id="${c.totem_id}" aria-describedby="shrinktip${c.totem_id}">Shrink claim</button>`)}</div>` : html``}`)}
    ${about('bases.resize', 'Resize by square', html`<p class="mut">One cell is 10 x 10 foundations, around the totem. Expanding only adds cells and never removes them, and the vertical level can only go up. Applied when the game next loads the save. This editor bypasses the game's normal staking checks. Claiming a protected area does not guarantee that the game lets you build there.</p>`)}</div>` : html``}
`);
      if (!claims.length) h.push(html`<p class="mut">This base has no land claim data in the save.</p>`);
    }
  }
  h.push(await baseBackupsCard());
  setHTML(host(), html`${h}`);
  if (bsTab == 'claim') claimEditorDraw();
}

// ---------- VEHICLES
// The player's own vehicles as in the console's Vehicles tab: type, lowest condition, fuel and location per vehicle, and for the open
// one its components with their condition and its cargo hold (read-only here; removing cargo comes later).
let vhOpen = null;
const pctCell = (v) => (v == null ? html`<span class="mut">-</span>` : html`<span class="${v < 30 ? 'bad' : v < 60 ? 'warn' : 'ok'}">${Math.round(v)}%</span>`);
const compName = (t) => String(t || '').replace(/_\d+$/, '').replace(/([a-z])([A-Z])/g, '$1 $2');
// The kind of a vehicle from its blueprint name: BP_MediumOrnithopter_CHOAM_C is a Medium Ornithopter.
const vehKind = (c) => String(c || '').replace(/^BP_/, '').replace(/_C$/, '').replace(/_CHOAM$/i, '').replace(/([a-z])([A-Z])/g, '$1 $2');
// Refuel all my vehicles (issue #349): the count says how many of the character's vehicles are not full. Only the kinds whose full tank was measured in the game are filled; the others are left alone and counted.
function refuelCard(f) {
  const total = f ? Number(f.total) || 0 : 0, notFull = f ? Number(f.notFull) || 0 : 0, unknown = f ? Number(f.unknown) || 0 : 0, noRec = f ? Number(f.noFuelRecord) || 0 : 0;
  const count = !f ? 'The vehicles could not be counted.' : total ? `${notFull} of ${total} vehicles are not full.` : 'You have no vehicles in this save yet.';
  const left = unknown ? ` ${unknown} ${unknown == 1 ? 'vehicle' : 'vehicles'} of an unknown kind will be left alone.` : '';
  const none = noRec ? ` ${noRec} ${noRec == 1 ? 'vehicle has' : 'vehicles have'} no fuel record and will be left alone.` : '';
  return html`<div class="card d-panel"><h3 class="d-head">Fuel</h3><div class="row">${tip('refueltip', 'Fills the fuel of your own vehicles to a full tank, never lowering a fuel amount that is already at or above it. A vehicle of any other kind is left alone and counted. Vehicles of other owners and the world\'s vehicles are not touched.', html`<button class="b d-btn primary" data-act="refuelV" aria-describedby="refuelnote refueltip" ${f && !notFull ? 'disabled' : ''}>Refuel all my vehicles</button>`)}<span class="mut" id="refuelnote">${count}${left}${none}</span></div>
    ${about('vehicles.fuel', 'Fuel', html`<p class="mut">A tank follows the vehicle's chassis grade (from the game's module table): Sandbike 90 to 150, Buggy 220 to 300, Light Ornithopter 225 to 400, Medium Ornithopter 450 to 850, Sand Crawler 450 to 750, Transport Ornithopter 1350 to 2500.</p>`)}</div>`;
}
// ---------- Create a vehicle from a template (#401). The class, the tier (the Mk number of the parts), a module for any slot, the fuel and the items for the cargo hold are picked here; the rows are
// written by POST /api/vehicles/spawn as one edit, and POST /api/vehicles/spawn-preview answers what that would write and keeps nothing. The draft lives here, not in the boxes, because the page is drawn again after every action.
let VC = null; // the last answer of /api/vehicles/catalog: { build, classes: [{ key, label, slots: [{ slot, mandatory, modules }], templates: [{ name, tier, modules, maxHealth, fuel, cargoSlots, cargoVolume }] }] }
let vcDraft = null; // { cls, tpl, mods: { slot: module id or '' }, fuel (percent), items: [{ item, quantity }] }
let vcPreview = null; // the preview's answer for the draft as it was when it was asked; any change to the draft drops it
let vcPreviewKey = ''; // the request the shown preview was asked with (JSON); Create writes only when the form still makes the same request (#672)
let vcSeq = 0; // counts the changes that decide what the preview is: an answer or a put-back is used only when no change came after its request
let vcTimer = 0; // the wait after the last part picked in the preview table, so a key held or pressed twice asks once
let vcPicked = []; // the slots picked in the table whose answer is not in yet
let vcBusy = false; // a part was picked in the preview table (#672) and its preview is not in yet: Create waits
let vcCreating = false; // a Create is on its way to the editor and its answer is not in: a second press (a double click) must not write a second vehicle (#740)
let vcPartNote = null; // { slots, text }: why the last parts picked in the preview table were not taken, until the next change
const VC_PICK_WAIT = 600; // ms; the arrow keys on a closed list change the value at every press
let vcResult = null; // { r: the answer of the last create, note: what the queue adds to the sentence }
const SLOT_WORDS = { Chassis: 'Chassis', Generator: 'Generator', Engine: 'Engine', CockpitHull: 'Front hull', BodyHull: 'Rear hull', UtilitySlot1: 'Utility 1', UtilitySlot2: 'Utility 2', UtilitySlot3: 'Utility 3' };
const slotWord = (s) => SLOT_WORDS[s] || s.replace(/^Locomotion(\d*)$/, (m, n) => 'Locomotion ' + (n || 1)).replace(/^BodyHull(\d)$/, 'Hull $1');
// "Mk6 Combat" for T6_Combat, "Mk0" for T0, "Container" for the container vehicle's one template.
const tplWord = (name, tier) => (name === 'Container' ? name : 'Mk' + tier + (name.includes('_') ? ' ' + name.slice(name.indexOf('_') + 1) : ''));
const vcClass = () => VC.classes.find((c) => c.key === vcDraft.cls);
const vcTemplate = () => vcClass().templates.find((t) => t.name === vcDraft.tpl);
const vcDefaults = () => Object.fromEntries(vcClass().slots.map((s) => [s.slot, vcTemplate().modules[s.slot] || '']));
const vcTop = (c) => c.templates.reduce((a, b) => (b.tier > a.tier ? b : a)).name;
const vcModName = (m) => (m.name || compName(m.id)) + (m.unique ? ' (unique)' : '') + (m.cargoSlots ? ` (${m.cargoSlots} slots)` : '');
// What the boxes hold -> the draft.
function vcSync() {
  if (!vcDraft || !$('#vcFuel')) return;
  vcDraft.fuel = Math.min(100, Math.max(0, Number(val('vcFuel')) || 0));
  document.querySelectorAll('[data-vcslot]').forEach((e) => { vcDraft.mods[e.dataset.vcslot] = e.value; });
  document.querySelectorAll('[data-vcqty]').forEach((e) => { const l = vcDraft.items[+e.dataset.vcqty]; if (l) l.quantity = Math.max(1, Math.floor(+e.value) || 1); });
}
// The page is drawn again when the answer to a pick in the preview table lands (#738), and the person may have typed meanwhile. vcSync puts the fuel and the quantities into the draft; the item box and its Qty
// are never in the draft. So the text of the boxes the person changed, and the caret of the one that has the focus, are noted before the redraw and put back after it (render gives the focus back itself).
const VC_BOXES = '#vcItem, #vcQty, #vcFuel, [data-vcqty]';
const vcBoxKey = (e) => (e.id ? '#' + e.id : '[data-vcqty="' + e.dataset.vcqty + '"]');
function vcTypedTake() {
  const out = [];
  document.querySelectorAll(VC_BOXES).forEach((e) => {
    const typed = e.value !== e.defaultValue, focus = e === document.activeElement;
    if (!typed && !focus) return;
    let start = null, end = null;
    if (focus) { try { start = e.selectionStart; end = e.selectionEnd; } catch (err) { /* a number box has no caret to read */ } }
    out.push({ key: vcBoxKey(e), value: typed ? e.value : null, focus, start, end });
  });
  return out;
}
// The redraw waits for the editor's answers before it replaces the boxes, and keys typed in that time go to the old boxes: they are noted as they are typed, into the same list.
function vcTypedWatch(typed) {
  const on = (ev) => {
    const e = ev.target;
    if (!e || !e.matches || !e.matches(VC_BOXES)) return;
    const key = vcBoxKey(e);
    let t = typed.find((x) => x.key === key);
    if (!t) typed.push(t = { key });
    typed.forEach((x) => { x.focus = false; });
    let start = null, end = null;
    try { start = e.selectionStart; end = e.selectionEnd; } catch (err) { /* a number box has no caret to read */ }
    Object.assign(t, { value: e.value, focus: true, start, end });
  };
  document.addEventListener('input', on, true);
  return () => document.removeEventListener('input', on, true);
}
function vcTypedPut(typed) {
  for (const t of typed) {
    const e = $('#main') && $('#main').querySelector(t.key);
    if (!e) continue; // the box is gone (the new part has no cargo hold): nothing to put back
    if (t.value !== null && e.value !== t.value) e.value = t.value;
    if (!t.focus || e !== document.activeElement) continue;
    if (t.start !== null) { try { e.setSelectionRange(t.start, t.end); } catch (err) { /* a box with no caret */ } }
    else if (e.type === 'number' && e.value !== '') { const v = e.value; e.value = ''; e.value = v; } // a number box has no caret to set; typing goes on at its end, where the person was typing
  }
}
// A class, a tier or a part was changed in the boxes.
function vcChanged(t) {
  vcSync();
  if (t.id === 'vcClass') { vcDraft.cls = t.value; vcDraft.tpl = vcTop(vcClass()); vcDraft.mods = vcDefaults(); }
  else if (t.id === 'vcTpl') { vcDraft.tpl = t.value; vcDraft.mods = vcDefaults(); }
  vcDrop();
  render('scroll'); // the same page: the scroll offset and the focused box stay, and the boxes are drawn from the draft (#650)
}
// The options of one slot's part box, the same for the Change parts box and the preview table's (#672): the parts that fit the slot of this class, the cargo-hold and unique labels, and (empty) for an optional slot.
const vcSlotOptions = (s, cur) => html`${s.mandatory ? '' : html`<option value="" ${cur ? '' : 'selected'}>(empty)</option>`}${(s.modules || []).map((m) => html`<option value="${m.id}" ${m.id === cur ? 'selected' : ''}>${vcModName(m)}</option>`)}`;
// The rows of the preview table: the rows the answer lists, then an optional slot that holds no part, so a part can be put there from the table too.
function vcPartRows(p) {
  const rows = p.rows.vehicle_modules.slice(), have = new Set(rows.map((m) => m.slot)), order = vcClass().slots.map((s) => s.slot);
  for (const s of vcClass().slots) if (!have.has(s.slot) && !s.mandatory && (s.modules || []).length) rows.push({ slot: s.slot, template_id: '', name: '', durability: null });
  return rows.sort((a, b) => order.indexOf(a.slot) - order.indexOf(b.slot)); // the class's own order, so a row does not move when its part is set
}
// Whatever decides what the preview is changed (a class, a tier, a part in Change parts, an item, Cancel, a new Preview, a created vehicle): the preview goes, and so does every answer still on its way.
function vcDrop() {
  vcPreview = null;
  vcPreviewKey = '';
  vcPartNote = null;
  vcSeq++;
  clearTimeout(vcTimer);
  vcBusy = false;
  vcPicked = [];
}
// The page's part of "the preview is being asked for": the line that says why Create waits, and Create itself. The card is drawn again when the answer is in.
function vcBusyShow(on) {
  const w = $('#vcwait'), box = $('#vcpreview'), create = document.querySelector('[data-act="vcCreate"]');
  if (w) w.hidden = !on;
  if (box) box.setAttribute('aria-busy', on ? 'true' : 'false');
  if (create) { create.disabled = on || vcCreating; create.setAttribute('aria-describedby', 'vccreatetip' + (on ? ' vcwait' : '')); }
}
const vcPartName = (slot) => { const s = vcClass().slots.find((x) => x.slot === slot), m = s && (s.modules || []).find((x) => x.id === vcDraft.mods[slot]); return m ? vcModName(m) : 'empty'; };
// A part was picked in the preview table (#672). The draft takes it at once (and so does the same box in Change parts), and after a short wait with no other pick the preview is asked again for the whole draft;
// the answer replaces the table without leaving the preview. A refusal puts the parts back to the ones the old preview shows, keeps that preview and says the reason. Nothing is written either way. Create waits
// meanwhile, and an answer is used only when nothing changed after its request (vcSeq), so the table never shows parts the draft does not hold.
function vcPartPicked(t) {
  if (!vcDraft || !vcPreview) return;
  const slot = t.dataset.vcprev;
  vcSync();
  if ((t.value || '') === (vcDraft.mods[slot] || '')) return;
  vcDraft.mods[slot] = t.value;
  const same = document.querySelector('select[data-vcslot="' + slot + '"]');
  if (same) same.value = t.value; // Change parts shows the part at once; a later sync of its boxes must not undo the pick
  if (!vcPicked.includes(slot)) vcPicked.push(slot);
  vcPartNote = null;
  vcBusy = true;
  const seq = ++vcSeq;
  clearTimeout(vcTimer);
  vcTimer = setTimeout(() => vcAskPick(seq), VC_PICK_WAIT);
  vcBusyShow(true);
}
async function vcAskPick(seq) {
  const draft = vcDraft;
  if (!draft || seq !== vcSeq) return;
  vcSync();
  const asked = vcBody(), picked = vcPicked.slice();
  let answer = null, err = null;
  try { answer = await api('/api/vehicles/spawn-preview', asked); } catch (e) { err = e; }
  if (seq !== vcSeq || vcDraft !== draft) return; // something changed after this request: the change drew the page, and its own answer (if any) decides
  vcBusy = false;
  vcPicked = [];
  vcSync(); // the fuel and the quantities typed while the request was on its way go into the draft; Create then sees the form no longer matches the preview and asks for a new one (#738)
  const typed = vcTypedTake();
  if (answer) {
    vcPreview = answer;
    vcPreviewKey = JSON.stringify(asked);
  } else {
    const shown = Object.fromEntries(vcPreview.rows.vehicle_modules.map((m) => [m.slot, m.template_id]));
    for (const s of vcClass().slots) draft.mods[s.slot] = shown[s.slot] || '';
    vcPartNote = { slots: picked, text: err.message };
  }
  // The state is up to date either way, and the card is drawn from it when the person comes back. If the preview card is not on the page (another tab or sub-page was opened while the
  // answer was on its way), there is no table to say a toast about, and a redraw would drop what was typed on the page that is open now (#739).
  if (!$('#vcpreview')) return;
  if (answer) {
    const d = answer.derived, what = picked.length === 1 ? slotWord(picked[0]) + ' is now ' + vcPartName(picked[0]) : 'The parts are changed';
    toast(`${what}. Health ${Number(d.maxHealth).toLocaleString()}, ${d.cargoSlots ? 'cargo hold of ' + nn(d.cargoSlots, 'slot', 'slots') : 'no cargo hold'}.`);
  }
  await vcRedraw(typed, vcTypedWatch(typed));
}
// The redraw that keeps what was typed (#738, #813): the text is put back whether the redraw succeeds or not, and a redraw that fails says so in a toast and does not leave a rejection nobody handles.
async function vcRedraw(typed, unwatch) {
  try { await render('scroll'); } catch (e) { toast('The page could not be drawn again: ' + (e && e.message ? e.message : e), true); } finally { unwatch(); vcTypedPut(typed); }
}
// One id for each form (#740): Create sends it, and the editor gives a second request with the same id and the same body the first one's answer instead of writing another vehicle.
function vcRequestId() {
  try { if (globalThis.crypto && crypto.randomUUID) return crypto.randomUUID(); } catch (e) { /* an older browser: the fallback below */ }
  return 'f' + Date.now().toString(36) + Math.random().toString(36).slice(2, 12);
}
// The request the draft stands for: the template, and a module for each slot that differs from it.
function vcBody() {
  const t = vcTemplate(), uniques = {};
  for (const s of vcClass().slots) { const want = vcDraft.mods[s.slot] || '', def = t.modules[s.slot] || ''; if (want !== def) uniques[s.slot] = want; }
  const body = { class: vcDraft.cls, template: vcDraft.tpl, fuel: vcDraft.fuel / 100 };
  if (Object.keys(uniques).length) body.uniques = uniques;
  if (vcDraft.items.length) body.items = vcDraft.items.map((l) => ({ template_id: l.item, quantity: l.quantity }));
  return body;
}
// What an answer (a preview's or a create's) says about the vehicle, in one sentence.
function vcSentence(r) {
  const d = r.derived, n = (v) => Number(v).toLocaleString();
  const hold = d.cargoSlots ? `a cargo hold of ${nn(d.cargoSlots, 'slot', 'slots')} and volume ${n(d.cargoVolume)}` : 'no cargo hold';
  const items = r.requested ? `, ${n(r.given)} of ${n(r.requested)} items in the cargo hold` : '';
  return `${r.label}, ${tplWord(r.template, r.tier)}, ${nn(r.counts.vehicle_modules, 'part', 'parts')}, health ${n(d.maxHealth)}, fuel ${n(d.fuel)} of ${n(d.fuelCapacity)}, ${hold}${items}`;
}
// Where the vehicle goes (#480, #540): the answer's own words (distance and direction, and for an aircraft the height and that the game settles it), "next to you" when the answer has none.
const whereWords = (r) => (r.place && r.place.words) || 'next to you';
function vcShorts(r) { return (r.notGiven || []).map((g) => html`<li>${g.name}: ${Number(g.short).toLocaleString()} did not fit (${g.reason || 'no room'})</li>`); }
function vehicleCreateCard(cat) {
  const r = vcResult && vcResult.r;
  const result = r ? html`<div class="info" role="status" id="vcresult"><b title="Vehicle number ${r.id}">Created vehicle:</b> ${vcSentence(r)}. It goes ${whereWords(r)}. It is written to the save; the game has not loaded it yet.${vcResult.note}
    ${(r.notGiven || []).length || (r.notes || []).length ? html`<ul>${vcShorts(r)}${(r.notes || []).map((x) => html`<li>${x}</li>`)}</ul>` : ''}
    <button class="b sec sm d-btn sm" data-act="vcDismiss">Dismiss</button></div>` : '';
  const aboutText = about('vehicles.create', 'Create a vehicle', html`<p class="mut">Writes a new vehicle of yours into the save from the game's own vehicle tables (game build ${VC ? VC.build : '1.5.15.0'}): the same rows the game keeps for a vehicle, which Duplicate also writes. The tier is the Mk number of the parts, so Mk6 is the tier 6 template. A part can be changed for any part of the same slot, a unique or another grade. The vehicle is put next to you, or outside your base when you stand in one; an ornithopter is put 30 m ahead of you and 7 m up, in the air, and the game settles it. Nothing asks first: it is written at once after a backup, or queued while the game is running. A vehicle made this way has not been loaded in the game yet, so check it on a copy of your save first.</p>`);
  if (!vcDraft) return html`<div class="card d-panel"><h3 class="d-head">Create a vehicle</h3>${result}<div class="row"><button class="b d-btn primary" data-act="vcOpen">Create a vehicle...</button></div>${aboutText}</div>`;
  const c = vcClass(), t = vcTemplate();
  const mods = vcDraft.mods;
  const hold = c.slots.reduce((a, s) => { const m = (s.modules || []).find((x) => x.id === mods[s.slot]); return a + (m ? m.cargoSlots || 0 : 0); }, 0);
  const p = vcPreview;
  const picker = hold ? html`<div class="row"><b>Items for the cargo hold</b>${itemPicker('vcItem', cat)}<label>Qty <input id="vcQty" type="number" value="1" min="1" class="w90"></label><button class="b sec d-btn" data-act="vcAddItem">Add</button></div>
    ${vcDraft.items.length ? tbl([{ k: 'item', label: 'Item', f: (l) => itemCell(l.item) }, { k: 'quantity', label: 'Quantity', f: (l) => html`<input type="number" min="1" value="${l.quantity}" data-vcqty="${vcDraft.items.indexOf(l)}" class="w90" aria-label="Quantity of ${itemName(l.item)}">` }], vcDraft.items,
      { actions: (l) => html`<button class="b bad sm d-btn danger sm" data-act="vcDelItem" data-id="${vcDraft.items.indexOf(l)}" aria-label="Remove ${itemName(l.item)}">Remove</button>` }) : ''}` : html`<p class="mut" id="vcnohold">These parts give no cargo hold; pick a storage part under Change parts${p ? ' or in the preview table' : ''} to put items in.</p>`;
  return html`<div class="card d-panel"><h3 class="d-head">Create a vehicle</h3>${result}
    <div class="row"><label>Class <select id="vcClass">${VC.classes.map((x) => html`<option value="${x.key}" ${x.key === vcDraft.cls ? 'selected' : ''}>${x.label}</option>`)}</select></label>
      <label>Tier <select id="vcTpl">${c.templates.map((x) => html`<option value="${x.name}" ${x.name === vcDraft.tpl ? 'selected' : ''}>${tplWord(x.name, x.tier)}</option>`)}</select></label>
      <label>Fuel % <input id="vcFuel" type="number" min="0" max="100" value="${vcDraft.fuel}" class="w90"></label></div>
    <details class="vcparts" ${KEEP.vcparts ? 'open' : ''}><summary data-keep="vcparts">Change parts</summary>
      ${c.slots.map((s) => html`<div class="row"><label>${slotWord(s.slot)} <select data-vcslot="${s.slot}">${vcSlotOptions(s, mods[s.slot])}</select></label></div>`)}
    </details>
    ${picker}
    <div class="row">${tip('vcpreviewtip', 'Shows the rows that would be written. Nothing is changed.', html`<button class="b sec d-btn" data-act="vcPreview" aria-describedby="vcpreviewtip">Preview</button>`)}${tip('vccreatetip', 'Writes the vehicle to the save now, after a backup, or queues it while the game is running. Nothing asks first.', html`<button class="b d-btn primary" data-act="vcCreate" aria-describedby="vccreatetip${vcBusy ? ' vcwait' : ''}" ${vcBusy || vcCreating ? 'disabled' : ''}>Create</button>`)}<button class="b sec d-btn" data-act="vcCancel">Cancel</button></div>
    ${p ? html`<div class="info" id="vcpreview" aria-busy="${vcBusy ? 'true' : 'false'}"><div role="status"><b title="Vehicle number ${p.id}">Would create vehicle:</b> ${vcSentence(p)}. It goes ${whereWords(p)}.
      <p class="mut" id="vcwait" ${vcBusy ? '' : 'hidden'}>Updating the preview...</p>
      ${(p.notGiven || []).length || (p.notes || []).length ? html`<ul>${vcShorts(p)}${(p.notes || []).map((x) => html`<li>${x}</li>`)}</ul>` : ''}</div>
      ${vcPartNote ? html`<p class="bad" role="alert" id="vcpartnote">${vcPartNote.slots.map(slotWord).join(' and ')} ${vcPartNote.slots.length === 1 ? 'was' : 'were'} not changed. ${vcPartNote.text}</p>` : ''}
      <p class="mut">Pick a part in the Part column to swap it. The preview updates, and nothing is written until Create.</p>
      ${tbl([{ k: 'slot', label: 'Slot', f: (m) => slotWord(m.slot) }, { k: 'name', label: 'Part', f: (m) => html`<select data-vcprev="${m.slot}" aria-label="Part for ${slotWord(m.slot)}" aria-invalid="${vcPartNote && vcPartNote.slots.includes(m.slot) ? 'true' : 'false'}" aria-describedby="${vcPartNote && vcPartNote.slots.includes(m.slot) ? 'vcpartnote' : ''}">${vcSlotOptions(c.slots.find((x) => x.slot === m.slot) || {}, mods[m.slot])}</select>` }, { k: 'durability', label: 'Durability', f: (m) => fix(m.durability) }], vcPartRows(p))}</div>` : ''}${aboutText}</div>`;
}
async function vehiclesView() {
  const v = await api('/api/vehicles');
  let fuel = null;
  try { fuel = await api('/api/vehicles/fuel'); } catch (e) { fuel = null; }
  const open = v.vehicles.find((r) => r.id == vhOpen) || v.vehicles[0];
  const cat = (open && open.cargo) || vcDraft ? await loadCatalog(open && open.cargo ? open.cargo.items.map((i) => i.template_id) : []) : null;
  const h = [html`<div class="card d-panel"><h3>Your vehicles (${v.vehicles.length})</h3>${tbl([{ k: 'name', label: 'Vehicle', f: (r) => html`<b title="${r.name}">${r.type || r.name}</b> <span class="mut">${r.map}</span>${r.stored ? html` <span class="tag" title="The game keeps this vehicle in storage; it keeps only the place it was last in the world.">${r.storedLabel}</span> <span class="mut" id="vstored${r.id}">Stored: take it out of storage in the game first.</span>` : ''}` }, { k: 'type', label: 'Type' },
    { k: 'condition', label: 'Lowest condition', f: (r) => pctCell(r.condition) }, { k: 'fuel', label: 'Fuel', f: (r) => (r.fuel == null ? '-' : r.fuelMax ? html`${Number(r.fuel).toFixed(1)} of ${Number(r.fuelMax).toLocaleString()} <span class="mut">(${Math.round(r.fuelPercent)}%)</span>` : Number(r.fuel).toFixed(1)) },
    { k: 'x', label: 'Location', f: (r) => html`${fix(r.x)}, ${fix(r.y)}${r.region ? html` <span class="mut">near ${r.region}</span>` : ''}` }], v.vehicles,
    { actions: (r) => html`<button class="b ${open && open.id == r.id ? '' : 'sec'} sm d-btn sm ${open && open.id == r.id ? 'primary' : ''}" data-vhopen="${r.id}">Details</button><button class="b sm d-btn primary sm" data-act="repairOne" data-id="${r.id}">Repair</button><button class="b sm d-btn primary sm" data-act="bring" data-id="${r.id}" ${describedBy(r.stored ? 'vstored' + r.id : '')} ${r.stored ? 'disabled' : ''} title="${r.stored ? 'This vehicle is stored; take it out of storage in the game first.' : ''}">Bring to me</button><button class="b sec sm d-btn sm" data-act="dupV" data-id="${r.id}" data-name="${r.type || r.name}" aria-label="Duplicate ${r.type || r.name} ${r.id}" ${describedBy(r.stored ? 'vstored' + r.id : '')} ${r.stored ? 'disabled' : ''} title="${r.stored ? 'This vehicle is stored; take it out of storage in the game first.' : ''}">Duplicate</button>`, empty: 'You have no vehicles in this save yet.' })}
    <div class="row">${tip('repairvtip', 'Makes every module as good as new: full durability, and the maximum that repairs at a workbench wore down is restored too. 100 repairs every module that is not already as good as new; a lower number repairs only the modules below that share, and says when there were none. A module whose full durability is not known is raised to the maximum it records, or skipped when it records none.', html`<button class="b d-btn primary" data-act="repairV" aria-describedby="repairvtip">Repair all my vehicles</button>`)}<label>Only modules below <input id="rvPct" type="number" min="1" max="100" value="100" class="w70"> % of their full durability</label></div>
    ${v.hidden ? about('vehicles.list', 'Your vehicles', html`<p class="mut">${v.hidden} other vehicle${v.hidden == 1 ? '' : 's'} in the world (not yours) ${v.hidden == 1 ? 'is' : 'are'} not shown.</p>`) : ''}</div>`, refuelCard(fuel), vehicleCreateCard(cat)];
  if (open) {
    const cargo = open.cargo;
    h.push(html`<div class="card d-panel"><h3>${open.type || open.name}: ${(open.components || []).length} components</h3>${tbl([{ k: 'template_id', label: 'Component', f: (r) => html`<span title="${r.template_id}">${compName(r.template_id)}</span>` },
      { k: 'percent', label: 'Condition', f: (r) => pctCell(r.percent) }, { k: 'current', label: 'Durability', f: (r) => (r.current == null ? '-' : Number(r.current).toFixed(0) + (r.max ? ' / ' + Number(r.max).toFixed(0) : '')) }], open.components || [], { empty: 'This vehicle has no components in the save.' })}
      ${about('vehicles.components', 'Components', html`<p class="mut">Condition is each module's durability against its current maximum; a module with no stored maximum shows only its value. The tank size is not stored in the save; the Fuel column takes it from the vehicle's chassis in the game's module table.</p>`)}</div>`);
    // The cargo hold is the vehicle's own inventory (the console's "View Contents"): the stack of an item, Delete, Add item and Clear this container work as in a base container.
    // Its room comes from the vehicle's parts (#483): a vehicle with no storage part has none, so the card offers no Add item there, and says so; rows already in such a hold stay listed to be deleted or cleared.
    const hold = `${open.type || open.name} cargo hold`;
    const n = cargo ? cargo.items.length : 0, noHold = !!(cargo && cargo.noHold);
    const room = !cargo ? '' : noHold ? 'This vehicle has no cargo hold (no storage part).' + (n ? ` ${nn(n, 'item', 'items')} in a hold with no slots.` : '') : cargo.slots == null ? nn(n, 'item', 'items') : `${cargo.used == null ? n : cargo.used} of ${cargo.slots} slots used`;
    h.push(html`<div class="card d-panel"><h3 class="d-head">Cargo hold</h3>${cargo ? html`<p class="mut" id="cargoroom">${room}</p>${n ? tbl([{ k: 'position_index', label: 'Slot' },
      { k: 'template_id', label: 'Item', f: (r) => itemCell(r.template_id) }, { k: 'stack_size', label: 'Quantity', f: (r) => html`<input type="number" min="1" value="${r.stack_size}" data-item="${r.id}" data-field="stack_size" data-name="${itemName(r.template_id)}" class="w90" aria-label="Quantity of ${itemName(r.template_id)}">` },
      { k: 'quality_level', label: 'Grade' }], cargo.items, { actions: (r) => html`<button class="b bad sm d-btn danger sm" data-act="delItem" data-id="${r.id}" data-name="${itemName(r.template_id)}">Delete</button>` }) : noHold ? '' : html`<p class="mut">The cargo hold is empty.</p>`}
      ${noHold ? '' : html`<div class="row">${itemPicker('bt', cat)}Qty<input id="bq" aria-label="Quantity to add" type="number" value="1" min="1"><button class="b d-btn primary" data-act="giveCargo" data-id="${cargo.inventory_id}">Add item</button></div>`}${noHold && !n ? '' : clearRow(cargo.inventory_id, hold, cargo.items)}` : html`<p class="mut">This vehicle has no cargo hold.</p>`}</div>`);
  }
  // Every stored vehicle of the character in one list (#528): the recovered ones, the one in the vehicle backup and the ones whose state says stored; the count is the rows shown.
  const stored = v.stored || [];
  h.push(html`<div class="card d-panel"><h3>Recovered and stored vehicles (${stored.length})</h3>${tbl([{ k: 'label', label: 'Kind', f: (r) => html`<span title="Vehicle number ${r.vehicle_id}">${r.label}</span>` }, { k: 'type', label: 'Vehicle', f: (r) => (r.type || '-') }, { k: 'vehicle_name', label: 'Name', f: (r) => (r.vehicle_name == null ? '-' : r.vehicle_name) }, { k: 'chassis_durability', label: 'Chassis durability', f: (r) => (r.chassis_durability == null ? '-' : r.chassis_durability) }, { k: 'time_stored', label: 'Stored at', f: (r) => (r.time_stored == null ? '-' : fmtEpoch(r.time_stored)) }, { k: 'reason', label: 'Reason code', f: (r) => (r.reason == null ? '-' : r.reason) }], stored, { empty: 'No vehicle has been recovered or stored.' })}</div>`);
  setHTML(host(), html`${h}`);
}

// ---------- LANDSRAAD
let LD = null; // the Landsraad read the board shows
// ---------- LANDSRAAD board (tau #599, design of record: docs/design/mock/landsraad/a-board.html)
// The page reads the save through /api/landsraad and the Command Center scripts draw the board (lcc/lcc-board.js); the classic list that used to be the other view was retired (#1028). The filters, the sort and the
// grouping of the board are the board's own.
// Every click on the board writes at once (#641): there is no staged state. LP holds only what the page remembers between redraws: the faction chosen for Fill and Complete, the
// houses with a tick in their Choose box, the line that says what the last click did, whether a write is running (a second click waits) and an open bulk question ({ act, F, n, ids }).
const LP = { fac: null, pick: new Set(), say: '', bad: false, warn: false, busy: false, confirm: null };
// The Command Center board is the only view of the Landsraad tab (the Classic list was retired, #1028). Every write goes through lbItem, lbGo, lbSend and lbRun below. A view chosen in an earlier version of the
// editor (the browser kept it under lsb.view) is dropped here: the board is the default and there is no other view to choose.
browserStore.remove('lsb.view');
const lbBoardOn = () => !!(window.LCC && window.LCCM && window.LCCB); // false only when the Command Center scripts did not load (lbBoardMissing)
// the three states of a tier, in one pair of words in both views (#936, #1099): Not reached / Earned, not collected / Earned, collected (the Command Center board says the same). The longer explanation stays in the text for a screen reader.
const LB_STL = { not_reached: 'not reached', waiting: 'earned, not collected (waiting at the house representative)', delivered: 'earned, collected', no_row: 'earned, no delivery recorded' };
const lbF = (n) => Number(n).toLocaleString('en-US');
const lbNameOf = (r) => String(r.house_name || '').replace(/^DA_House/, '');
const LB_WAIT = 'A write is still running. Wait for it to finish, then click again.';
const LB_REOPEN_TIP = 'Reopen this house: clears the completion and the faction that holds its favor. Progress and your contribution stay as they are, so the faction’s progress stays at the goal and a house that was taken by another faction is not given back to it. Reopen writes or removes no reward.';
const crTipC = (fn) => `Complete for ${fn}: taken by ${fn}, progress to the goal`;
// The small marks (save, derived) beside the headings of the Landsraad pages, and the table and field names in their tooltips, are gone (#1057): Help (The Landsraad board) says where the figures come from, once.
function lbAbout(id, title, content) { return html`<details class="about" data-about="${id}" ${browserStore.get(aboutKey(id)) === '1' ? 'open' : ''}><summary>About ${title}</summary><div class="abt">${content}</div></details>`; }
// One house as the board needs it, from the task the server returned.
const lbTaken = (c) => !!c.done || c.win != null;
const lbCan = (l) => !!(l && l.character && l.character.faction_id != null); // the contribution is the character's, under its own faction
function lbHouse(r, l) {
  const goal = Number(r.goal_amount) || 0, prog = {};
  for (const c of r.contributions || []) prog[c.faction_id] = Number(c.amount) || 0;
  const mine = Number(r.my_contribution || 0), done = !!r.completed, win = r.winning_faction_id == null ? null : r.winning_faction_id;
  const tiers = (r.rewards || []).map((x) => ({ th: Number(x.threshold), amount: Number(x.shown_amount != null ? x.shown_amount : x.amount), tmpl: x.shown_template_id || x.template_id, swatch: !!x.swatch,
    reached: mine >= Number(x.threshold), state: x.reward_state || (x.reached ? 'no_row' : 'not_reached'),
    back: x.takeback || '', backWhy: x.takeback_why || '' })); // what removing this tier would do to its reward (take, collected, none, unsure), by the server's rule (landsraad_takeback.go)
  const reached = tiers.filter((t) => t.reached).length;
  const loc = r.location || { known: false };
  const state = mine <= 0 ? 'new' : tiers.length && reached === tiers.length ? 'done' : 'part';
  return { id: r.id, name: lbNameOf(r), task: r, mine, tiers, reached, state, goal, prog, win, done, taken: lbTaken({ done, win }),
    loc, region: loc.known ? loc.map : '', area: loc.known ? loc.area : '', held: !!r.swatch_held,
    unfill: r.unfill || null }; // what Unfill chosen would do to the rewards of every reached tier of this house at once (the server's rule, landsraad_takeback.go: boardUnfill), said before the click (#932)
}
// The name of a Landsraad reward item. An item the editor's list has no name for is "Unlisted item" everywhere on the Landsraad page (the board, the ledger, the list cells, the hovers and the results), never its internal id (#1055).
const LB_UNLISTED = 'Unlisted item';
const lbItemName = (id) => (id === LB_SOLARI ? 'Solari' : (CATALOG && CATALOG.byId.get(id)) || LB_UNLISTED);
const lbRText = (t) => lbF(t.amount) + ' ' + lbItemName(t.tmpl);
// the next-tier text is two pieces that never break inside: on a wide row they sit on one line, on a phone the second piece moves down
function lbNextParts(h) {
  for (const t of h.tiers) if (h.mine < t.th) return ['next ' + lbF(t.th) + ' · ' + lbRText(t) + ',', lbF(t.th - h.mine) + ' more'];
  const top = h.tiers.length ? h.tiers[h.tiers.length - 1].th : 0;
  return [h.mine > top ? 'above the top tier' : 'top tier earned'];
}
function lbNextReward(h) { for (const t of h.tiers) if (h.mine < t.th) return lbF(t.th) + ': ' + lbRText(t); return 'top tier earned'; }
// one reward per line (#666): each waiting reward is its own block, so a long item name wraps inside its own line and never runs into the next reward
const lbWaiting = (h) => { const w = h.tiers.filter((t) => t.state === 'waiting').map(lbRText); return w.length ? w.map((r) => html`<span class="hrw">${r}</span>`) : 'nothing waiting'; };
const lbTop = (h) => (h.tiers.length ? h.tiers[h.tiers.length - 1].th : 0);
// the faction of the list: the box on the page is the truth (a refresh puts a chosen value back into it), else what was chosen before, else the character's own faction, else the first
function lbFacId(l) {
  const sel = typeof document !== 'undefined' ? document.getElementById('lsb-fac') : null, v = sel && sel.value !== '' ? +sel.value : LP.fac;
  return v != null && (l.factions || []).some((f) => f.id == v) ? v : lbCan(l) ? l.character.faction_id : ((l.factions || [])[0] || {}).id;
}
const lbDoneFor = (h, F) => h.done && h.win == F && (h.prog[F] || 0) >= h.goal;
const lbFillOff = (h, l) => !lbCan(l) || lbTop(h) <= 0 || h.mine >= lbTop(h);
// your contribution counts for your own faction: with another faction chosen, Fill changes nothing of yours and writes no reward rows (Complete and Fill faction goal act for that faction)
const lbOwnF = (l, F) => lbCan(l) && F == l.character.faction_id;
const lbFillOffF = (h, l, F) => lbFillOff(h, l) || !lbOwnF(l, F);
// What one click asks the save for (the item of /api/landsraad/apply), worked out from the house as the page shows it: kind is fill, complete, reopen, fillgoal or tier (with i, the
// tier number). Returns null when the click would change nothing. Nothing is ever lowered except by the removal of the highest tier (lower: true).
function lbItem(h, l, kind, F, i) {
  const it = { task_id: h.id, was_mine: h.mine, was_done: h.done, was_win: h.win };
  // Complete and fill chosen is Fill faction goal on each chosen house, and only for the character's own faction: the fill is the character's own contribution, so with another faction chosen it is off
  // (a House is never completed alone by this button; Complete chosen is the button for that), #932
  if (kind === 'cfill') { if (!lbOwnF(l, F)) return null; kind = 'fillgoal'; }
  // Unfill chosen sets the character's own contribution to 0 (every reached tier comes off at once) and carries the number of waiting rewards the question said it would take back (was_taken), so the
  // server refuses it when it would take back another number now. A House with no contribution of yours is left out, and so is a read that does not say what an unfill would do (#932).
  if (kind === 'unfill') {
    if (!lbOwnF(l, F) || !(h.mine > 0) || !h.unfill) return null;
    it.mine_to = 0; it.lower = true; it.was_taken = (h.unfill.rows || []).filter((r) => r.outcome === 'take').length; // counted from the rows the question names, so the number sent is the number said
  }
  if (kind === 'tier') {
    if (!lbOwnF(l, F)) return null; // a tier sets your own contribution, which counts for your own faction only (#667)
    const ths = h.tiers.map((t) => t.th), n = h.reached;
    if (i > n) it.mine_to = ths[i - 1]; else if (i === n) { it.mine_to = i > 1 ? ths[i - 2] : 0; it.lower = true; } else return null;
  }
  if ((kind === 'fill' || kind === 'fillgoal') && !lbFillOffF(h, l, F)) it.mine_to = lbTop(h); // Fill is off at the top tier, so this never lowers
  if (it.mine_to !== undefined && typeof h.task.my_events === 'number') it.was_events = h.task.my_events; // the ground your own events cover, as shown: the server refuses the raise when it is another number now (#762)
  // a raise also carries the two other grounds its sentence was worked out from, as shown: what the editor itself paid before (and the ground whose reward rows are delivered), and what you reached
  // under another faction; the server refuses the raise when either is another number now (#805). A removal does not depend on them.
  if (it.mine_to !== undefined && !it.lower) {
    if (typeof h.task.my_paid === 'number') it.was_paid = h.task.my_paid;
    if (typeof h.task.my_paid_rows === 'number') it.was_paid_rows = h.task.my_paid_rows;
    if (typeof h.task.my_elsewhere === 'number') it.was_elsewhere = h.task.my_elsewhere;
  }
  if ((kind === 'complete' || kind === 'fillgoal') && !lbDoneFor(h, F)) it.complete_for = F;
  if (kind === 'reopen' && h.taken) it.reopen = true;
  return it.mine_to !== undefined || it.complete_for !== undefined || it.reopen ? it : null;
}
// the sentence for what a click changed, from the house as it was shown and the request that was sent
function lbChangeText(h, l, it) {
  const p = [];
  if (it.mine_to !== undefined) {
    const a = h.reached, b = h.tiers.filter((t) => t.th <= it.mine_to).length;
    const t = b > a ? ' (' + (b - a > 1 ? 'tiers ' + (a + 1) + ' to ' + b : 'tier ' + b) + ' reached)' : b < a ? ' (' + (a - b > 1 ? 'tiers ' + (b + 1) + ' to ' + a : 'tier ' + a) + ' removed)' : '';
    p.push('your contribution ' + lbF(h.mine) + ' → ' + lbF(it.mine_to) + t);
  }
  if (it.complete_for != null) {
    const have = h.prog[it.complete_for] || 0;
    if (have < h.goal) p.push(lbFacName(l, it.complete_for) + ' progress ' + lbF(have) + ' → ' + lbF(h.goal));
    p.push('house taken by ' + lbFacName(l, it.complete_for));
  }
  if (it.reopen) p.push('house reopened (progress stays as it is)');
  return p.join('; ');
}
// a faction the Landsraad does not have (a save that already holds the Smuggler faction from an earlier write) is named plainly, not by its number (#842)
// "Other faction" only when the list of the Landsraad's factions is loaded and the id is not in it; with no list (it failed to load, or is empty) nothing is known, so the number is shown
const lbFacName = (l, id) => ((l.factions || []).find((f) => f.id == id) || {}).name || (id == null ? '' : (l.factions || []).length ? 'Other faction' : 'Faction ' + id);
// ---- what a raise grants, said before the click (#652). The board read carries the size of a mission step, what a mission pays and the reason a raise would be refused (l.grant), and for
// every house its specialization line and the ground already paid: what its own events cover (my_events), what it reached under another faction (my_elsewhere) and what the editor paid itself (my_paid). The missions of a click are worked out here as the editor works them out when it writes
// (landsraad_grant.go): the mission steps the raise crosses beyond the ground already paid.
const LB_ASK_MISSIONS = 10; // a click on one house that would grant this many missions or more (60,000 Solari at the stock step) asks first; fewer write at once
const GRANT_UNCHECKED = 'The pay-out of a mission (Solari, House Scrip, specialization XP and rewards) has not been checked in the game yet.';
function lbGrantOf(h, l, from, to) {
  const g = l && l.grant;
  if (!g || !(to > from)) return null;
  const lo0 = Math.max(from, Number(h.task.my_events) || 0, Number(h.task.my_elsewhere) || 0); // the game's events, and what the character reached under another faction
  const lo = Math.max(lo0, Number(h.task.my_paid) || 0); // and the ground the editor itself paid before: a tier removal does not take back what its missions paid (#777)
  const loRows = Math.max(lo0, Number(h.task.my_paid_rows) || 0); // the ground whose reward rows are delivered: the rewards a removal took back are written again (#944)
  const rowsOf = () => h.tiers.filter((t) => t.th > loRows && t.th <= to && !(t.swatch && h.held)).length;
  const at0 = (x) => (x <= 0 ? 0 : Math.ceil(x / (g.step || 1) - 1e-9));
  const paidMissions = lo > lo0 && g.step > 0 ? Math.max(0, at0(Math.min(to, lo)) - at0(lo0)) : 0; // the missions this click would pay but for that record
  if (!(to > lo)) return { missions: 0, rewards: rowsOf(), solari: 0, scrip: 0, xpBy: {}, paidMissions }; // the character's own events and the editor's own record already cover this ground: no mission is paid, and only the rewards a removal took back are written
  if (g.problem) return { problem: g.problem };
  const line = h.task.line || '';
  const at = (x) => (x <= 0 ? 0 : Math.ceil(x / g.step - 1e-9)), missions = at(to) - at(lo);
  if (missions > g.max_missions) return { problem: `House ${h.name}: this click stands for ${lbF(missions)} missions, more than the ${lbF(g.max_missions)} one change may grant for one house, so nothing is changed. Click the tiers one at a time, or set LandsraadContributionMultiplier in ServerCustomSettings.ini (Config tab) nearer to 1.` };
  if (!line && g.xp > 0 && missions > 0) return { problem: `House ${h.name} has no specialization line in the editor, so a raise on it that pays a mission is refused and nothing is changed. Choose other houses, or use Complete for this one.` }; // the write refuses only when a mission with experience is written (#762)
  const room = g.room && line && g.room[line] != null ? Number(g.room[line]) : null; // the experience the track still has room for: the write adds no more than fits (#762)
  const xp = room != null ? Math.min(missions * g.xp, Math.max(room, 0)) : missions * g.xp;
  // a swatch won in this term is not delivered again (the write skips it), so the top tier does not count as waiting then
  return { missions, paidMissions, rewards: rowsOf(), solari: missions * g.solari, scrip: missions * g.scrip, xpBy: xp > 0 && line ? { [line]: xp } : {} };
}
function lbGrantSum(list) { // the grants of several houses as one
  const s = { missions: 0, rewards: 0, solari: 0, scrip: 0, xpBy: {}, paidMissions: 0 };
  for (const g of list) { if (!g || g.problem) continue; s.paidMissions += g.paidMissions || 0; s.missions += g.missions; s.rewards += g.rewards; s.solari += g.solari; s.scrip += g.scrip; for (const k of Object.keys(g.xpBy)) s.xpBy[k] = (s.xpBy[k] || 0) + g.xpBy[k]; }
  // two houses of one line share one track: together they add no more than the track has room for (#762)
  const room = LD && LD.grant && LD.grant.room;
  if (room) for (const k of Object.keys(s.xpBy)) if (room[k] != null) s.xpBy[k] = Math.min(s.xpBy[k], Math.max(Number(room[k]), 0));
  return s;
}
const lbGrantList = (g) => ['+' + lbF(g.solari) + ' Solari', '+' + lbF(g.scrip) + ' House Scrip'].concat(Object.keys(g.xpBy).sort().map((k) => '+' + lbF(g.xpBy[k]) + ' ' + k + ' XP')).join(', ');
// the sentence before the click: what the raise grants, or why it will be refused
function lbGrantSay(g) {
  if (!g) return '';
  if (g.problem) return 'Will be refused: ' + g.problem;
  const rw = g.rewards ? nn(g.rewards, 'reward', 'rewards') + ' waiting at the house representative' : '';
  const pb = g.paidMissions ? ' ' + lbPaidBefore(g.paidMissions) : ''; // ground the editor paid before is named, so that a click that pays less than it looks is not a surprise (#777)
  if (!g.missions) return 'Writes no missions' + (g.paidMissions ? '' : ': your own earlier progress already counts for this ground') + (rw ? '; ' + rw : '') + '.' + pb;
  return 'Writes ' + nn(g.missions, 'mission', 'missions') + ': ' + lbGrantList(g) + (rw ? ', ' + rw : '') + '.' + pb;
}
// the sentence for missions left unpaid because the editor paid that ground before (a tier removal takes back no Solari, House Scrip, XP or faction progress, so the same raise never pays them twice; it only writes again a reward a removal took back)
const lbPaidBefore = (n) => nn(n, 'mission', 'missions') + ' not paid: the editor recorded that ground as paid in an earlier raise, and removing a tier does not take back what its missions paid.';
const lbGrantShort = (g) => (!g ? '' : g.problem ? 'will be refused' : g.missions ? nn(g.missions, 'mission', 'missions') + ': +' + lbF(g.solari) + ' Solari' : 'no missions');
// The House words of the Command Center (lcc-model.js WORDS) (#1073): a House a faction has claimed is Pledged to it; one no faction has claimed is
// Undecided, or Wavering when both factions have progress on it (the save holds the progress of each faction).
// the House words are the model's (lcc-model.js WORDS); a page without the Command Center scripts says the same words from this copy (a test holds the two together)
const LB_WORDS = Object.freeze({ pledged: 'Pledged', pledgedTo: 'Pledged to', undecided: 'Undecided', wavering: 'Wavering' });
const lbW = () => (window.LCCM && window.LCCM.WORDS) || LB_WORDS;
const lbOpenWord = (h, l) => ((l.factions || []).length >= 2 && (l.factions || []).every((f) => (h.prog[f.id] || 0) > 0) ? lbW().wavering : lbW().undecided);
// a place on a map the Saved map has no picture of (Arrakeen, Harko Village): it has a name and no pin, distance or map link
const lbHub = (h) => h.loc.known && !h.loc.map_key;
// a click on the place goes to the Saved map only where the Saved map has a picture of the place; the other places say why they cannot
const lbOnMap = (h) => !!(h.loc.known && h.loc.map_key);
// two cases, each said truly (#1083): a map the editor has no picture of at all, and a place outside the frame of the picture of a map the editor has. A test holds this together with noPictureSays of lcc-model.js.
const lbNoPic = (h) => (LB_PIC_LABELS.includes(h.loc.map) ? (h.loc.area && h.loc.area !== h.loc.map ? h.loc.area : 'This place') + ' is outside the picture of ' + h.loc.map + '.' : 'The Saved map has no picture of ' + (h.loc.map || h.loc.area || 'this place') + ' yet.');
const lbNoMark = 'This house is not mapped yet: the save holds no map marker for its representative and the editor has no fixed place for it.';
const lbLocWhy = (h) => (!h.loc.known ? lbNoMark : lbHub(h) ? lbNoPic(h) : '');
// what a click on the place does, in words: used by the peek and the name card
const lbLocHint = (h) => (lbOnMap(h) ? 'Click the place to open the Saved map.' : lbLocWhy(h));
// the second line of the Location column: the Deep Desert's grid cell, a named place, and the map when the first line is an area of it
const lbSub = (h) => (h.loc.known ? [h.loc.cell ? 'Grid cell ' + h.loc.cell : '', h.loc.place, h.loc.area === h.loc.map ? '' : h.loc.map].filter(Boolean).join(' · ') : '');
function lbAreaFull(h) { return !h.loc.known ? 'not mapped yet' : h.loc.area === h.loc.map ? h.loc.area : h.loc.area + ' (' + h.loc.map + ')'; }
const lbTierWord = (t) => LB_STL[t.state];
const LB_REMOVE_NOTE = 'Rewards already collected stay, and so do the Solari, House Scrip and XP the missions of the tier paid, and raising the tier again pays no mission a second time. To go back, restore the backup (Database tab, Save backups); the editor does not restore its own record with it, so a raise after that may pay less than before.';
// What removing a tier does to its reward, in words (the server's rule, landsraad_takeback.go, said before the click): t is the tier of the house as lbHouse built it. A tier whose reward has no row says so, so a removal
// is never silent about the reward. Removing the tier also removes the tiers above it, but the page removes one tier at a time, the highest first.
function lbBackSay(t) {
  const r = lbRText(t) + (t.swatch ? ' (the house swatch)' : '');
  switch (t.back) {
    case 'take': return 'Takes back its reward, which still waits at the house representative: ' + r + '. Raising the tier again writes that reward once more.';
    case 'collected': return 'Its reward (' + r + ') is already collected and stays.';
    case 'earned': return 'A reward (' + r + ') waits for this tier and stays: ' + (t.backWhy || 'your own missions in the game cover this tier') + '.';
    case 'unsure': return 'Its reward (' + r + ') stays: it cannot be tied to this tier with certainty, because ' + (t.backWhy || 'the save does not say') + '.';
    default: return 'No reward of this tier waits at the house representative, so there is nothing to take back.';
  }
}
// a removal asks first when its reward is taken back or named as staying; a tier with no reward row to say anything about is written at once, as before
const lbBackAsks = (t) => !!t && (t.back === 'take' || t.back === 'collected' || t.back === 'earned' || t.back === 'unsure');
// the line under the highest reached tier box: what a click on it does to its reward, said from the server's outcome for the tier (t.back) and never as a fixed claim
function lbRemoveLine(t) {
  switch (t && t.back) {
    case 'take': return 'click removes it; takes back its waiting reward';
    case 'collected': return 'click removes it; its reward is collected and stays';
    case 'earned': case 'unsure': return 'click removes it; its waiting reward stays';
    default: return 'click removes it; no reward waits';
  }
}
// what a tier click does. withGrant: the sentence that says what a raise grants goes into the tip too (the tier box shows it as text, so its accessible name carries it as well)
// what the Command Center board says when a click on a lower reached tier cannot remove it: the board removes tiers with Set tier and Apply tier, one at a time, the highest first
const lbBoardRemove = (h) => 'choose tier ' + (h.reached - 1) + ' in Set tier and press Apply tier: tiers come off one at a time, the highest first.';
const lbRemoveHint = (h, board) => (board ? 'To remove tiers of House ' + h.name + ', ' + lbBoardRemove(h) : `To remove tiers of House ${h.name}, click the highest reached tier (tier ${h.reached}).`);
function lbTierTip(h, t, i, can, l, withGrant, F, board) {
  const n = h.reached;
  if (!can) return 'Your character is in no faction, so there is no contribution of yours to change.';
  if (!lbOwnF(l, F)) return `Off while ${lbFacName(l, F)} is chosen. Tiers change your own contribution, which counts for ${lbFacName(l, l.character.faction_id)}: choose ${lbFacName(l, l.character.faction_id)} above.`;
  if (i > n) return `Give tier ${i}: your contribution goes to ${lbF(t.th)}, so ${i === 1 ? 'tier 1 is' : 'tiers 1 to ' + i + ' are'} reached (contribution is cumulative). ${withGrant ? lbGrantSay(lbGrantOf(h, l, h.mine, t.th)) + ' ' : ''}Written at once.`;
  return i === n ? `Remove tier ${i}: your contribution drops to ${i > 1 ? lbF(h.tiers[i - 2].th) : 'none'}. ${lbBackSay(t)} ${LB_REMOVE_NOTE}`
    : board ? `Tier ${i} is reached. To remove tiers, ${lbBoardRemove(h)}` : `Tier ${i} is reached. To remove tiers, click the highest reached tier (tier ${n}).`;
}
// The mouse-over text of one tier box, built once for the cards, the rows and the Inspector of the board (#938): the reward, and what a click does or why the box is off. tip is what lbTierTip said.
function lbTierTitle(h, t, i, tip, off) {
  const rid = 'Reward: ' + lbRText(t) + (t.swatch ? ' (the house swatch, as the character does not hold it yet)' : '') + '.'; // the display name only: the game's internal name is shown nowhere (#977)
  return off ? tip : rid + ' ' + tip;
}
// The quick peek draws the real picture of the House's map (#1069): a window of that picture around the pin and, in a corner, the whole map with the same pin and the window marked. The pictures are the ones the Saved map draws, one fixed
// list by the map key of the read (a key is never turned into a path), with the map space of internal/ops/livemap.go (Width x Height): the read gives the pin as a fraction of the picture (x * Width, y * Height on the Saved map),
// so the pin is the same fraction here, and a test keeps this list in step with livemap.go. The files are the program's own and lazy (a picture inside a hidden peek is not asked for until it is shown).
const LB_PIC = {
  HaggaBasin: { src: '/maps/hagga-basin.png', w: 4096, h: 4096 },
  DeepDesert: { src: '/maps/deep-desert.png', w: 4096, h: 4096 },
  Arrakeen: { src: '/maps/arrakeen.png', w: 4096, h: 2342 },
  HarkoVillage: { src: '/maps/harko-village.png', w: 4096, h: 4096 },
};
const LB_PIC_LABELS = ['Hagga Basin', 'Deep Desert', 'Arrakeen', 'Harko Village']; // the map names of a read whose place has a picture (internal/ops houseMapKey)
const LB_PEEK_ZOOM = 0.25, LB_PEEK_RATIO = 16 / 9; // the window shows a quarter of the map's width, in a box 16 to 9
// the window: its place in the picture (left, top, width and height as fractions) and the pin inside it (fractions of the window). It stays inside the picture: a place near an edge is not centred.
function lbPeekWindow(pic, x, y) {
  const fw = LB_PEEK_ZOOM, fh = Math.min(1, fw * (pic.w / pic.h) / LB_PEEK_RATIO);
  const left = Math.min(1 - fw, Math.max(0, x - fw / 2)), top = Math.min(1 - fh, Math.max(0, y - fh / 2));
  return { fw, fh, left, top, px: (x - left) / fw, py: (y - top) / fh };
}
const lbPct = (n) => (n * 100).toFixed(2);
// why the peek is the schematic and not the picture: a map with no picture, or a place outside the frame of its picture (the read gives no map_key and no pin then)
function lbSchemWhy(h) {
  if (!h.loc.map_key) return 'Schematic only, not to scale: ' + (LB_PIC_LABELS.includes(h.loc.map) ? 'this place is outside the frame of the picture, so there is no pin.' : 'the editor has no picture of ' + h.loc.map + ' yet.');
  return 'Schematic only, not to scale: the editor has no picture of ' + h.loc.map + ' that this place can be pinned on.';
}
// what the note under a place with no pin says: the editor has the picture of the map and the place is outside its frame, or the editor has no picture of that map (#1069)
const lbHubNote = (h) => (LB_PIC_LABELS.includes(h.loc.map) ? 'This place is outside the frame of the picture of ' + h.loc.map + '.' : lbNoPic(h));
const lbSchematic = (h) => html`<span class="minimap"><svg viewBox="0 0 120 90" role="img" aria-label="Schematic of ${h.loc.map}, not to scale" focusable="false"><path class="m-fill" d="M8 34 L28 12 L66 6 L102 20 L114 50 L98 80 L56 86 L22 78 L6 56 Z"/></svg><span class="mwhy">${lbSchemWhy(h)}</span></span>`;
function lbMiniMap(h) {
  if (!h.loc.known) return '';
  const pic = LB_PIC[h.loc.map_key], x = Number(h.loc.x), y = Number(h.loc.y);
  if (!h.loc.map_key || !pic || !(x >= 0 && x <= 1 && y >= 0 && y <= 1)) return lbSchematic(h);
  const g = lbPeekWindow(pic, x, y), corner = (g.px > 0.62 ? 'l' : 'r') + (g.py > 0.62 ? 't' : 'b'); // the small whole map sits in the corner away from the pin
  const ar = pic.w + '/' + pic.h;
  return html`<span class="minimap mpic" role="img" aria-label="Map of ${h.loc.map}, zoomed on the place of House ${h.name}, with the whole map in the corner"><span class="mwin"><img class="mimg" src="${pic.src}" alt="" loading="lazy" decoding="async" data-box="${lbPct(-g.left / g.fw)},${lbPct(-g.top / g.fh)},${lbPct(1 / g.fw)},,${ar}"><span class="pin" data-px="${lbPct(g.px)}" data-py="${lbPct(g.py)}"></span></span><span class="mfull c-${corner}"><img src="${pic.src}" alt="" loading="lazy" decoding="async" data-box=",,,,${ar}"><span class="mview" data-box="${lbPct(g.left)},${lbPct(g.top)},${lbPct(g.fw)},${lbPct(g.fh)}"></span><span class="mdot" data-px="${lbPct(x)}" data-py="${lbPct(y)}"></span></span></span>`;
}
// the quick peek on the place (mouse hover only, see the .pop rules): the same lines as the opened row, drawn once per row; the opened row is the way in for a touch screen and the keyboard
const lbPeek = (h) => (h.loc.known ? html`<span class="pop" aria-hidden="true"><h4>House ${h.name}</h4>${lbPlaceDetails(h)}<span class="fine">Quick peek. ${lbLocHint(h)}</span></span>` : '');
// The House hover card, one builder for the cards and the List of the Command Center board (#940, #951). The Place line is a real button that opens the same Saved map view as the place link of the row (same data-lb="map"),
// with the same quick peek (the map picture) under it, and it stays inside the card, which stays open while the pointer or the keyboard focus is on it (the CSS keeps it open on :hover and :focus-visible of anything inside).
function lbPlaceLine(h) {
  if (!h.loc.known) return 'not mapped yet';
  const text = lbAreaFull(h) + (h.loc.cell ? ' · grid cell ' + h.loc.cell : '') + (h.loc.place ? ' · ' + h.loc.place : '') + (h.loc.unconfirmed ? ' (from public guides, not confirmed in the game)' : '');
  const on = lbOnMap(h);
  return html`<span class="loc"><button type="button" class="hcard-place${on ? '' : ' off'}" data-lb="map" data-id="${h.id}" data-fk="hp:${h.id}" ${on ? '' : html`aria-disabled="true" data-why="${lbLocWhy(h)}"`} aria-label="${lbLocAria(h)}" title="${lbLocHint(h)}">${text}</button>${on ? '' : html`<span class="lnote" role="status"></span>`}${lbPeek(h)}</span>`;
}
// The head of the House hover card (#1069): the large House symbol (the game's picture at 128 px, from the Command Center's art, shown at 64) beside the House name and its region. The path comes from the board's own lookup
// (LCCB.houseSymbol: a fixed list of Houses), the picture is lazy so the hidden card asks for nothing, and a House with no picture there (or no Command Center scripts) has the name and the region alone.
function lbCardHead(h) {
  const f = window.LCCB && window.LCCB.houseSymbol ? window.LCCB.houseSymbol(h.name) : '', region = h.loc.known ? lbAreaFull(h) : 'not mapped yet';
  return html`<span class="hhead">${f ? html`<img class="hsym" src="${f}" alt="" width="64" height="64" loading="lazy" decoding="async">` : ''}<span class="hht"><h4>House ${h.name}</h4><span class="hreg">${region}</span></span></span>`;
}
function lbHouseCard(h, l, noLocs) {
  return html`<span class="hcard" role="group" aria-label="House ${h.name}, details"><button type="button" class="hcl" aria-label="Close the card">×</button>${lbCardHead(h)}<dl><dt>Next reward</dt><dd>${lbNextReward(h)}</dd><dt>Not collected</dt><dd>${lbWaiting(h)}</dd><dt>Faction standing</dt><dd>${(l.factions || []).map((f) => html`${f.name} ${lbF(h.prog[f.id] || 0)} of ${lbF(h.goal)}<br>`)}${h.taken ? (h.win != null ? lbW().pledgedTo + ' ' + lbFacName(l, h.win) : lbW().pledged) : lbOpenWord(h, l)}</dd>${noLocs ? '' : html`<dt>Place</dt><dd>${lbPlaceLine(h)}</dd>`}</dl>${noLocs ? '' : html`<span class="fine">${lbLocHint(h)}</span>`}</span>`;
}
// A built piece of the page as a DOM node, for the Command Center scripts (they draw with the DOM, the page with html``): the pins of a place peek are put where the save says, as the page does after a draw.
function lbNode(safe) {
  const n = document.createElement('span');
  n.className = 'lsb-frag';
  setHTML(n, safe);
  lbStyles(n);
  return n.childElementCount === 1 ? n.firstElementChild : n;
}
// The mouse-over of the item a reward tier gives (#937, #977): what the editor holds about it, in the board’s words (the reward and the state; never the game's internal name), and nothing it does not know: what kind of item it is
// (a schematic, a resource, a part) is not in the save or the editor's item list, so it is not said. Solari has none (the amount is the whole story).
const LB_SOLARI = 'SolarisCoin';
function lbItemInfo(h, t, i) {
  if (!t || t.tmpl === LB_SOLARI) return '';
  return ['Reward of tier ' + i + ': ' + lbRText(t) + (t.swatch ? ' (the house swatch)' : ''), 'Tier ' + i + ' needs ' + lbF(t.th) + ': ' + lbTierWord(t)].join('\n');
}
// One reward of a tier as the picture mouse-over of the Command Center draws it (#977): the display name the game shows (never the internal name), the quantity, the tier and its state. `item` is the item id, handed
// over only so the board can look the picture up in its own fixed list (a data attribute, never text); the board draws no picture for an id it has none for. Solari (the amount is the whole reward) has no picture.
function lbRewardRow(h, t, i) {
  if (!t) return null;
  const solari = t.tmpl === LB_SOLARI;
  return { item: solari ? '' : String(t.tmpl), name: lbItemName(t.tmpl), qty: lbF(t.amount), swatch: !!t.swatch, tier: i, need: lbF(t.th), state: lbTierWord(t) };
}
// The mouse-overs of the cells of the Command Center list, in one source of words: your progress, the tiers, the next tier and the rewards waiting.
function lbListTips(h) {
  const n = h.tiers.length;
  return {
    progress: (h.mine > 0 ? 'Your contribution: ' + lbF(h.mine) + '. ' : 'No contribution of yours recorded. ') + h.reached + ' of ' + n + ' tiers earned.',
    tiers: n ? h.tiers.map((t, k) => 'Tier ' + (k + 1) + ', ' + lbF(t.th) + ': ' + lbRText(t) + ' (' + lbTierWord(t) + ')').join('\n') : 'No reward tiers.',
    next: 'Next reward: ' + lbNextReward(h) + (h.tiers.some((t) => h.mine < t.th) ? ', ' + lbNextParts(h)[1] : '') + '.',
    waiting: h.tiers.some((t) => t.state === 'waiting') ? 'Not collected: ' + h.tiers.filter((t) => t.state === 'waiting').map(lbRText).join('; ') : 'Nothing waiting at the house representative.',
    // the same mouse-overs with the item pictures (#977): the rows of the reward mouse-over of the Command Center list, one per tier; a cell with no rows keeps its sentence
    rows: {
      tiers: h.tiers.map((t, k) => lbRewardRow(h, t, k + 1)),
      next: h.tiers.map((t, k) => (h.mine < t.th ? lbRewardRow(h, t, k + 1) : null)).filter(Boolean).slice(0, 1),
      waiting: h.tiers.map((t, k) => (t.state === 'waiting' ? lbRewardRow(h, t, k + 1) : null)).filter(Boolean),
    },
  };
}
// the distance from the character to the house representative: the save holds the character's place in centimetres, and the house representative's marker its place, as a fraction of its map
function lbFromYouText(h) {
  const c = h.loc.from_you_cm;
  if (c == null) return 'not known';
  const m = Math.round(Number(c) / 100);
  return m >= 1000 ? `about ${(m / 1000).toFixed(1)} km` : `about ${m} m`;
}
const LB_FROM_YOU_WHY = 'Your character is on another map, or has no saved position.'; // the reason the distance is not known: the hover text, not the standing line
const lbFromYou = (h) => (h.loc.from_you_cm == null ? html`<span class="mut" title="${LB_FROM_YOU_WHY}">${lbFromYouText(h)}</span>` : lbFromYouText(h));
// The place line of a house as the board says it in its List and in House Intelligence, one builder (#939): the words of the link, its accessible name and its mouse-over. hint is the mouse-over:
// "Click the place to open the Saved map." or the reason the place cannot be opened.
const lbLocAria = (h) => 'House ' + h.name + ': ' + (h.loc.known ? lbAreaFull(h) + (lbSub(h) ? ', ' + lbSub(h) : '') + '. ' + (lbOnMap(h) ? 'Open in the Saved map' : lbLocWhy(h)) : 'not mapped yet. ' + lbNoMark);
function lbLocModel(h) {
  if (!h.loc.known) return { known: false, onMap: false, area: 'Representative place not mapped yet', sub: '', hub: false, hint: lbNoMark, why: lbNoMark, aria: lbLocAria(h), from: '' };
  return { known: true, onMap: lbOnMap(h), area: h.loc.area, sub: lbSub(h), hub: lbHub(h), hint: lbLocHint(h), why: lbLocWhy(h), aria: lbLocAria(h), from: lbHub(h) ? '' : lbFromYouText(h), fromWhy: !lbHub(h) && h.loc.from_you_cm == null ? LB_FROM_YOU_WHY : '' };
}
function lbPlaceDetails(h) {
  if (!h.loc.known) return html`<p class="cap">This house is not mapped yet: the save holds no map marker for its representative and the editor has no fixed place for it. Nothing to fix: the editor does not guess a place.</p>`;
  return html`${lbMiniMap(h)}<dl><dt>Location</dt><dd>${lbAreaFull(h)}${h.loc.place ? ' · ' + h.loc.place : ''}</dd>${h.loc.cell ? html`<dt>Grid cell</dt><dd>${h.loc.cell}</dd>` : ''}${lbHub(h) ? '' : html`<dt>From you</dt><dd>${lbFromYou(h)}</dd>`}</dl>${h.loc.unconfirmed ? html`<p class="pnote">This place is from public guides and is not confirmed in the game.</p>` : ''}${lbHub(h) ? html`<p class="pnote">${lbHubNote(h)} There is no pin or map link for this house.</p>` : ''}`;
}
// What the quick actions of one house say, and which are off, for the faction chosen. The Inspector and the quick actions of a row of the Command Center board all draw from this,
// so the two views cannot tell two stories about one click.
function lbQa(h, l, F) {
  const can = lbCan(l), fn = lbFacName(l, F), ownF = can && F == l.character.faction_id;
  const fillOff = lbFillOffF(h, l, F), fillGr = fillOff ? null : lbGrantOf(h, l, h.mine, lbTop(h)), nextGr = ownF && h.reached < h.tiers.length ? lbGrantOf(h, l, h.mine, h.tiers[h.reached].th) : null;
  const fillTip = fillOff ?(!can ? 'Your character is in no faction, so there is no contribution of yours to fill' : !ownF ? `Off while ${fn} is chosen: Fill raises your own contribution, which counts for ${lbFacName(l, l.character.faction_id)}. Complete or Fill faction goal act for ${fn}.` : 'Nothing to change: your contribution is already at the top tier') : 'Fill: raises your contribution to the top tier (never lowered). ' + lbGrantSay(fillGr) + ' Written at once.';
  const doneF = lbDoneFor(h, F), goalOff = (fillOff || !ownF) && doneF;
  const goalTip = goalOff ? 'Nothing to change for ' + fn : `Fill faction goal for ${fn}: progress to the goal, house taken by ${fn}` + (ownF ? ', your contribution to the top tier. ' + lbGrantSay(fillGr) + ' Written at once.' : '; your own contribution row is skipped');
  const crTip = h.taken ? LB_REOPEN_TIP : `Complete for ${fn}: taken by ${fn}, progress to the goal`;
  return { can, fn, ownF, fillOff, fillGr, nextGr, fillTip, doneF, goalOff, goalTip, crTip };
}
function lbHistory(l) {
  const e = l.earlier_terms || [], tm = l.terms || [];
  const rows = e.map((t) => {
    const hs = t.houses, done = hs.filter((x) => x.tier_count && x.tiers_reached === x.tier_count).length, mineF = l.character ? l.character.faction_id : null;
    return html`<details class="term"><summary><b>Term ${t.term_id}</b><span class="sum2"><span><b>${hs.length}</b> ${hs.length === 1 ? 'house' : 'houses'} with your contribution</span><span><b>${done}</b> top tier</span><span><b>${hs.length - done}</b> in progress</span><span><b>${hs.filter((x) => x.winning_faction_id != null && x.winning_faction_id == mineF).length}</b> pledged to your faction</span><span><b>${hs.filter((x) => x.paid_in_term).length}</b> last paid out in this term</span></span></summary>
      <table><thead><tr><th>House</th><th>Your contribution</th><th>Tiers</th><th>House status</th><th>Last payout</th></tr></thead><tbody>${hs.map((x) => html`<tr><td>${lbNameOf(x)}</td><td class="n">${lbF(x.my_contribution)}</td><td><span class="meter${x.tier_count && x.tiers_reached === x.tier_count ? ' full' : ''}" role="img" aria-label="${x.tiers_reached} of ${x.tier_count} tiers reached">${Array.from({ length: x.tier_count }, (_, i) => html`<i class="${i < x.tiers_reached ? 'on' : ''}${i === x.tier_count - 1 ? ' top' : ''}"></i>`)}</span> <span class="mut">${x.tiers_reached} of ${x.tier_count}</span></td><td>${x.completed || x.winning_faction_id != null ? 'Pledged' + (x.winning_faction_id != null ? ' to ' + lbFacName(l, x.winning_faction_id) : '') : 'Undecided at term end'}</td><td>${x.paid_in_term ? 'in this term' : html`<span class="mut">not in this term</span>`}</td></tr>`)}</tbody></table></details>`;
  });
  return html`<section class="card d-panel hist" aria-labelledby="lsb-hist"><h2 class="d-head" id="lsb-hist">Earlier terms <span class="cnt">opens on demand</span></h2>
  <p class="line1 mb8">${tm.length < 2 ? 'This save holds only the current term.' : e.length ? 'Terms with a row of yours; open one for its houses.' : 'No earlier term has a row of yours.'}</p>
  ${lbAbout('lsb.hist', 'earlier terms', html`<p>Terms with no row of yours are not listed. If a save holds only the current term this card says so instead. A term's rows say what the contribution was; they do not say which terms paid out. The last payout is the time of the last reward row written for the House, kept as one time per House: a reward collected in a later term moves it out of the earlier term, so "not in this term" does not always mean nothing was paid then.</p>`)}
  ${rows}</section>`;
}
// ---- the term and decrees panel (part 4): the term's facts, the Active decree and Reigning faction controls, and the decree pool. A write goes to the save at once, as the other
// term controls always did; the decree flag stays a button per row (the pool flag is shown as stored, and what the game does with it is not known, #634).
// A decree is shown by the name the game shows (#608): the server sends display_name and effect from the game's own decree table, name_known is false when the name is not the game's
// (the plain SpecialVendorActive row has no row in that table). An id the table does not have comes through with no display_name and shows as a readable form of the id; it never breaks the page.
function decreeLabel(d) {
  const o = d && typeof d === 'object' ? d : { decree_name: d };
  if (o.display_name) return String(o.display_name);
  const k = String(o.decree_name == null ? '' : o.decree_name);
  const [base, ...rest] = k.split('_'), small = new Set(['And', 'On', 'Of', 'The', 'In', 'To', 'For']);
  const b = base.replace(/(.)Active$/, '$1').replace(/([a-z0-9])([A-Z])/g, '$1 $2').split(' ').map((w, i) => (i && small.has(w) ? w.toLowerCase() : w)).join(' ');
  return rest.length ? b + ': ' + rest.join(' ') : b;
}
// What the page says about a name that is not the game's, in words (the same words in the note under a name, in a drop-down option and in the summary line and the result): the plain
// SpecialVendorActive row has a name the editor gave it, and an id the editor has never seen has no name at all (the page shows a readable form of the id).
function decreeNote(d, short) {
  if (!d || d.name_known !== false) return '';
  if (d.display_name) return short ? "not the game's name" : "editor's name, not the game's";
  return short ? 'no game name' : 'no game name for this id';
}
// The name with the short note after it, for the places that show the name as plain text: a drop-down option, the summary line, the Applied result.
const decreeFull = (d) => decreeLabel(d) + (decreeNote(d, true) ? ` (${decreeNote(d, true)})` : '');
// The tooltip of a decree's name. The effect and the note are also drawn as text (a tooltip does not reach touch, keyboard or a screen reader); the tooltip adds what only fits in a sentence.
function decreeTip(d) {
  if (!d) return '';
  const parts = [];
  if (d.variant && d.display_name) parts.push(`The game gives both decrees the name ${String(d.display_name).replace(/ \([^)]*\)$/, '')}; "${d.variant}" is added here to tell them apart`);
  if (d.name_known === false) parts.push(d.display_name ? "This name is not the game's: the game's decree table has no row for this decree" : "No game name is known for this decree id; the name shown is a readable form of the id");
  if (d.effect) parts.push('In game: ' + d.effect);
  return parts.join('. ');
}
const decreeNameEl = (d) => html`<span class="dn" ${decreeTip(d) ? html`title="${decreeTip(d)}"` : ''}>${decreeLabel(d)}</span>`;
const decreeUnsure = (d) => (d && d.name_known === false ? html` <small class="mut du">${decreeNote(d)}</small>` : '');
const decreeEffect = (d) => (d && d.effect ? html`<small class="de">${d.effect}</small>` : '');
// the two small lines under the Active decree drop-down: the effect and the note of the chosen decree (always drawn, so a change of the choice only sets their text). The game's id is not shown (#1040).
const decreeSelInfo = (d) => html`<small class="de">${d && d.effect ? d.effect : ''}</small><small class="mut du">${decreeNote(d)}</small>`;
// the drop-down's order: by the name a player knows, a name that is not the game's (and an unknown id) last
const decreeSorted = (dec) => [...dec].sort((x, y) => (x.name_known === false) - (y.name_known === false) || decreeLabel(x).localeCompare(decreeLabel(y)) || x.id - y.id);
// The plain SpecialVendorActive decree does nothing in the game and is not shown anywhere on the page (operator order, after #634 had put it in a closed list). Its row stays in the data and is
// never changed or deleted; only the page leaves it out: not in the pool, not in the Active decree drop-down, not by name. The five SpecialVendorActive rows are five ids; the four category vendors
// (Vehicles, Weapons, Armor, Utilities) stay. The one case the page cannot leave out is a term that holds this row as its active or elected decree: the page then says "Another decree" with no id
// and no effect, so the term never reads as having none, and the drop-down keeps that one value so that Apply cannot clear the decree by accident.
const LB_HIDDEN_DECREE = 'SpecialVendorActive';
const LB_HIDDEN_NAME = 'Another decree';
const lbIsHidden = (d) => !!d && d.decree_name === LB_HIDDEN_DECREE;
const lbMasked = (d) => (lbIsHidden(d) ? { id: d.id, decree_name: '', display_name: LB_HIDDEN_NAME, name_known: true, effect: '', active: d.active, elected: d.elected, in_rotation: d.in_rotation, disabled: d.disabled } : d);
const lbDecreeById = (l, id) => lbMasked(((l && l.decrees) || []).find((x) => String(x.id) === String(id)));
const lbShownDecrees = (l) => ((l && l.decrees) || []).filter((d) => !lbIsHidden(d));
// the Active decree drop-down's rows: the shown decrees, and the masked one only while the choice or the saved term holds it
const lbDecreeOptions = (l, sel) => {
  const hid = ((l && l.decrees) || []).filter((d) => lbIsHidden(d) && (String(d.id) === sel.ad || (l.term && l.term.active_decree_id != null && String(d.id) === String(l.term.active_decree_id))));
  return [...decreeSorted(lbShownDecrees(l)), ...hid.map(lbMasked)];
};
const LB_NEED_FACTION = 'Choose a reigning faction first: a decree is only in force because a faction won the term. Nothing was changed.';
const LT = { ad: undefined, rf: undefined, open: false, histOpen: false, flags: false }; // what the two drop-downs were set to (kept over a redraw); what Apply or Make active did is a toast, not a line that stays (#1120)
// the two drop-downs: what was chosen, else the term's active decree and the character's own faction (the term's reigning faction when the character has none); a faction the list does not offer is shown as none
function lbTermSel(l) {
  const t = l.term || {}, facs = (l.factions || []).map((f) => String(f.id)), decs = (l.decrees || []).map((d) => String(d.id));
  const own0 = lbCan(l) ? String(l.character.faction_id) : t.reigning_faction_id != null ? String(t.reigning_faction_id) : '', own = facs.includes(own0) ? own0 : '';
  const rf = LT.rf !== undefined && (LT.rf === '' || facs.includes(LT.rf)) ? LT.rf : own;
  const ad = LT.ad !== undefined && (LT.ad === '' || decs.includes(LT.ad)) ? LT.ad : t.active_decree_id != null ? String(t.active_decree_id) : '';
  return { ad, rf };
}
function lbTerm(l) {
  const t = l.term;
  if (!t) return html`<section class="card d-panel" aria-labelledby="lsb-term-h"><h2 class="d-head" id="lsb-term-h">Term and decrees <span class="cnt">this save has no Landsraad term</span></h2></section>`;
  const dec = lbShownDecrees(l), by = (i) => lbDecreeById(l, i), a = by(t.active_decree_id), e = by(t.elected_decree_id), sel = lbTermSel(l), nx = lbNextWord(l);
  const own = lbCan(l) ? l.character.faction_id : null, rfName = t.reigning_faction_id != null ? lbFacName(l, t.reigning_faction_id) : 'none';
  const fname = sel.rf !== '' ? lbFacName(l, +sel.rf) : '';
  const nm = (d) => (d ? decreeFull(d) : '—');
  const row = (d) => {
    const tg = [d.active && 'Active', d.elected && "Next term's decree", d.in_rotation && 'In rotation'].filter(Boolean), n = d.disabled ? 0 : 1, lab = decreeLabel(d);
    const on = d.id == t.active_decree_id && sel.rf !== '' && sel.rf === String(t.reigning_faction_id == null ? '' : t.reigning_faction_id);
    const mkTip = sel.rf !== '' ? `Makes ${lab} the active decree of the term for ${fname}, the same as the Active decree drop-down and Apply` : 'Choose a reigning faction above first';
    const mk = on
      ? html`<span class="mk-on" title="This decree is already the active decree of the term for ${fname}"><span aria-hidden="true">✓</span> Active</span>`
      : html`<button type="button" class="qa mk" data-lb="tmk" data-id="${d.id}" data-fk="tm:${d.id}" aria-disabled="${sel.rf === '' ? 'true' : 'false'}" aria-label="${sel.rf !== '' ? 'Make active for ' + fname : 'Make active (choose a faction)'}: ${lab}" title="${mkTip}">${sel.rf !== '' ? 'Make active for ' + fname : 'Make active (choose a faction)'}</button>`;
    // no raw row number (#1054); the pool flag and its button sit behind the one Advanced switch of the pool (data-flagbox, hidden until it is pressed)
    return html`<li><span class="dc">${decreeNameEl(d)}${decreeEffect(d)}${decreeUnsure(d)}</span><span class="tg">${tg.length ? tg.map((x) => html`<span class="tag">${x}</span> `) : ''}</span><span class="mkc">${mk}</span><span class="fgb" data-flagbox ${LT.flags ? '' : 'hidden'}><span class="mut">Pool flag</span><span class="fl mut" title="A switch the save keeps on this decree, shown as stored. It does not say whether the decree is in effect.">${d.disabled ? 1 : 0}</span><button type="button" class="qa" data-lb="tflag" data-id="${d.id}" data-d="${n}" data-fk="tf:${d.id}" aria-label="Set flag to ${n}: ${lab}" title="Sets the pool flag of this one row to ${n}. It does not make the decree active; Make active on the row or the Active decree above does.">Set flag to ${n}</button></span></li>`;
  };
  return html`<section class="card d-panel" aria-labelledby="lsb-term-h"><details class="tdp" id="lsb-tdp" ${LT.open ? 'open' : ''}><summary><h2 class="d-head" id="lsb-term-h">Term and decrees <span class="cnt">Term ${t.term_id} · ${rfName} · ${a ? nm(a) : 'no active decree'}</span></h2></summary>
  <div class="tbody">
  <div class="tkv"><div><span class="mut">Ends in</span><b title="${LB_ENDS_TIP}">${lbEndsIn(l)}</b></div><div><span class="mut">Reigning faction</span><b>${rfName}</b></div><div><span class="mut">Active decree</span>${a ? decreeNameEl(a) : html`<span class="dn">—</span>`}${decreeUnsure(a)}</div><div><span class="mut">Next term's decree</span>${e ? decreeNameEl(e) : html`<span class="dn" data-next="${nx.kind}" title="${nx.tip}">${nx.text}</span>`}${decreeUnsure(e)}</div></div>
  <div class="tctl" role="group" aria-label="Set the term's active decree and reigning faction">
    <label class="fld"><span class="lbl">Active decree</span><select id="lsb-tad" data-lb="tad"><option value="">none</option>${lbDecreeOptions(l, sel).map((d) => html`<option value="${d.id}" ${String(d.id) === sel.ad ? 'selected' : ''}>${decreeFull(d)}</option>`)}</select>${decreeSelInfo(by(sel.ad))}</label>
    <label class="fld"><span class="lbl">Reigning faction</span><select id="lsb-trf" data-lb="trf" title="Any faction can be chosen.">${sel.rf === '' ? html`<option value="" disabled selected>Choose a faction</option>` : ''}${(l.factions || []).map((f) => html`<option value="${f.id}" ${String(f.id) === sel.rf ? 'selected' : ''}>${f.name}${own != null && f.id == own ? ' (your faction)' : ''}</option>`)}</select></label>
    <span class="tapply"><button type="button" class="qa btnp" data-lb="tapply" data-fk="tapply">Apply</button></span>
  </div>
  <h3 class="d-head">Decree pool</h3>
  ${dec.length ? html`<p class="advrow"><button type="button" class="qa" data-lb="tflags" data-fk="tflags" aria-expanded="${LT.flags ? 'true' : 'false'}">${LT.flags ? 'Advanced: hide pool flags' : 'Advanced: show pool flags'}</button></p>` : ''}
  <ul class="dp" id="lsb-dp" role="list"><li class="dh" aria-hidden="true"><span class="dc">Decree</span><span class="tg">In this term</span><span class="mkc">Make active</span></li>${dec.map(row)}</ul>
  ${dec.length ? '' : html`<p class="mut">This save has no decrees.</p>`}
  ${lbAbout('lsb.term', 'term and decrees', html`<p><b>Apply</b> sets the term's active decree and its reigning faction together: one row, the current term in <code>landsraad_decree_term</code>, and none in the Active decree drop-down clears the decree. A decree is only in force for the faction that reigns, so the faction cannot be left empty: the Reigning faction drop-down offers Atreides and Harkonnen, starts on your own faction, and asks you to choose when you are in neither and the term has no reigning faction. Apply needs no choice in two cases: when the save holds a faction this list does not offer and you leave the drop-down alone, Apply changes the decree and leaves that faction as it is; and none for the decree on a term that has no faction clears the decree and writes no faction.</p><p><b>Make active for ...</b> on a pool row does the same as the Active decree drop-down and Apply for that row's decree, for the faction in the Reigning faction drop-down; a decree belongs to a faction (the reigning faction), not to a house.</p><p id="lsdecnote"><b>In this term</b> is read from the current term: <b>Active</b> is the term's active decree, <b>Next term's decree</b> the decree elected during this term, which takes effect in the next term, <b>In rotation</b> a decree in the save's decree rotation. <b>Advanced: show pool flags</b> adds a switch to each decree row: the flag the save keeps on that decree, shown as stored. It does not say whether a decree is in effect. A <b>Set flag</b> button changes the flag of its own row only; it does not make the decree active, the Active decree above does. The special vendor decree is five separate rows: the four Exclusive Import Rights decrees (Vehicle, Weapon, Armor and Tool) are in the list; the fifth, the plain one, does nothing in the game, so the page does not show it. It stays in the save as it is.</p><p><b>Names</b> are the ones the game shows, with the effect line the game gives under each; the id the save holds is the small line below, kept for support. The game gives the Ranged and Melee decrees of In Honor of the Fallen one name, so the page adds the word in brackets. The pool is in id order; the Active decree drop-down is sorted by name, with a name that is not the game's last.</p>`)}
  </div></details></section>`;
}
// The Command Center's Term and Decrees section (lcc/lcc-term.js) is drawn from this plain model of the same answer, with the same words and the same states as lbTerm above: the section
// carries the same data-lb attributes as every control of the page, so lbClick and lbChange (and so the requests) are the page’s own. Nothing here sends anything.
// The line under the Active decree drop-down while the choice is not the term's active decree yet (#925): the choice is only a draft until Apply or Make active writes it. '' when they agree.
function lbPendingText(l, ad) {
  const t = l.term;
  if (!t) return '';
  const saved = t.active_decree_id != null ? String(t.active_decree_id) : '';
  if (ad === saved) return '';
  const by = (i) => lbDecreeById(l, i), n = (i) => (i === '' || !by(i) ? 'none' : decreeLabel(by(i)));
  return 'Not applied yet: the term still has ' + n(saved) + ' as its active decree. Press Apply to set ' + n(ad) + '.';
}
// The line of "Next term's decree" by the state of the Landsraad (#1073), one function for the Command Center. The term row says who reigns (reigning_faction_id) and which decree was chosen
// for the next term (elected_decree_id); the two are read apart: a decree chosen -> the decree, as always. None chosen and no reigning faction -> the Landsraad is still divided. None chosen and a reigning faction
// -> that faction have swayed the Landsraad and the decree is yet to be chosen. A decree chosen that this save's list does not hold, or a faction the list does not name -> a dash: it cannot be read.
const LB_NEXT = Object.freeze({ divided: 'The Landsraad is still divided', dividedTip: 'No faction is in control of the Landsraad yet.', swayedTail: ' have swayed the Landsraad; their decree is yet to be chosen', swayedTip: 'The controlling faction has not selected next term’s decree.', dash: '—', dashTip: 'Not recorded in the save.' });
function lbNextWord(l) {
  const t = (l && l.term) || {}, e = t.elected_decree_id, d = e != null ? lbDecreeById(l, e) : null;
  if (e != null) return d ? { kind: 'decree', d } : { kind: 'unreadable', text: LB_NEXT.dash, tip: LB_NEXT.dashTip };
  if (t.reigning_faction_id == null) return { kind: 'divided', text: LB_NEXT.divided, tip: LB_NEXT.dividedTip };
  const f = ((l && l.factions) || []).find((x) => String(x.id) === String(t.reigning_faction_id));
  return f && f.name ? { kind: 'swayed', text: f.name + LB_NEXT.swayedTail, tip: LB_NEXT.swayedTip } : { kind: 'unreadable', text: LB_NEXT.dash, tip: LB_NEXT.dashTip };
}
function lbTermModel(l) {
  const t = l.term;
  if (!t) return { none: true };
  const dec = lbShownDecrees(l), by = (i) => lbDecreeById(l, i), sel = lbTermSel(l), own = lbCan(l) ? l.character.faction_id : null;
  const fname = sel.rf !== '' ? lbFacName(l, +sel.rf) : '';
  const one = (d) => (d ? { id: d.id, key: d.decree_name, name: decreeLabel(d), effect: d.effect || '', note: decreeNote(d), tip: decreeTip(d) } : null);
  const row = (d) => {
    const lab = decreeLabel(d), on = d.id == t.active_decree_id && sel.rf !== '' && sel.rf === String(t.reigning_faction_id == null ? '' : t.reigning_faction_id);
    return Object.assign(one(d), {
      tags: [d.active && 'Active', d.elected && "Next term's decree", d.in_rotation && 'In rotation'].filter(Boolean), disabled: d.disabled ? 1 : 0, makeOn: on, makeOff: sel.rf === '',
      makeLabel: sel.rf !== '' ? 'Make active for ' + fname : 'Make active (choose a faction)',
      makeTip: on ? 'This decree is already the active decree of the term for ' + fname : sel.rf !== '' ? 'Makes ' + lab + ' the active decree of the term for ' + fname + ', the same as the Active decree drop-down and Apply' : 'Choose a reigning faction above first',
    });
  };
  const e = l.earlier_terms || [], tm = l.terms || [], mineF = l.character ? l.character.faction_id : null;
  const terms = e.map((x) => {
    const hs = x.houses, done = hs.filter((y) => y.tier_count && y.tiers_reached === y.tier_count).length;
    return {
      id: x.term_id,
      summary: hs.length + (hs.length === 1 ? ' house' : ' houses') + ' with your contribution · ' + done + ' top tier · ' + (hs.length - done) + ' in progress · ' + hs.filter((y) => y.winning_faction_id != null && y.winning_faction_id == mineF).length + ' pledged to your faction · ' + hs.filter((y) => y.paid_in_term).length + ' last paid out in this term',
      rows: hs.map((y) => ({ name: lbNameOf(y), mine: lbF(y.my_contribution), reached: y.tiers_reached, count: y.tier_count, full: !!(y.tier_count && y.tiers_reached === y.tier_count),
        status: y.completed || y.winning_faction_id != null ? 'Pledged' + (y.winning_faction_id != null ? ' to ' + lbFacName(l, y.winning_faction_id) : '') : 'Undecided at term end', paid: y.paid_in_term ? 'in this term' : 'not in this term' })),
    };
  });
  return {
    none: false,
    term: { id: t.term_id, endsIn: lbEndsIn(l), endsTip: LB_ENDS_TIP, reigning: t.reigning_faction_id != null ? lbFacName(l, t.reigning_faction_id) : '' },
    active: one(by(t.active_decree_id)), elected: one(by(t.elected_decree_id)), next: (({ kind, text, tip }) => ({ kind, text, tip }))(lbNextWord(l)),
    pool: dec.map(row), options: lbDecreeOptions(l, sel).map((d) => ({ id: d.id, label: decreeFull(d) })),
    factions: (l.factions || []).map((f) => ({ id: f.id, label: f.name + (own != null && f.id == own ? ' (your faction)' : '') })),
    sel, selD: one(by(sel.ad)), pending: lbPendingText(l, sel.ad), open: LT.open, histOpen: LT.histOpen, flags: LT.flags,
    history: { note: tm.length < 2 ? 'This save holds only the current term.' : e.length ? '' : 'No earlier term has a row of yours.', terms },
  };
}
// A play-clock second as a number (null when the save holds none, or a value that is a real time, not the play clock: the same rule as lbEndsIn), and where the play clock stood when the save was written, with
// the real time of that write in ms (null when the read does not give it). Display only (#922).
const lbPlay = (v) => (v == null || v === '' || !Number.isFinite(Number(v)) || Number(v) < 0 || Number(v) >= 1e8 ? null : Number(v));
function lbSaveClock(l) {
  const c = l && l.clock;
  return c && Number.isFinite(Number(c.play_seconds)) && Number.isFinite(Number(c.saved_at)) ? { play: Number(c.play_seconds), savedAt: Number(c.saved_at) * 1000 } : null;
}
// How long the term has left, as the one value of the row ENDS IN: hours and minutes (#981). The term end and the play clock are both in play-clock seconds; the clock stops while the game is closed, so this is
// the value as of the last save. No seconds are shown, no start, no date. Edge cases in words: under an hour "37 m", under a minute "less than a minute", at or past the end "ended", unreadable a dash (#1056).
const LB_ENDS_TIP = 'Counts down only while the game is running; as of the last save.';
function lbEndsIn(l) {
  const t = l && l.term, end = t ? lbPlay(t.end_time) : null, c = lbSaveClock(l);
  if (end === null || !c) return '—'; // a dash, not words (#1056)
  const left = end - c.play;
  if (!(left > 0)) return 'ended';
  const m = Math.floor(left / 60), h = Math.floor(m / 60);
  if (m < 1) return 'less than a minute';
  return (h ? h + ' h ' : '') + (m % 60) + ' m';
}
const lbTermOn = () => !!window.LCCT;
// What the term panel says (Apply, Make active, the pool flag): a toast like every other result of the Landsraad page (#1120), never a line that stays. The partial-draft hint under the Active decree box
// ("Not applied yet: ...", lbPendingText) is not a result: it is drawn from the choice and stays until the choice is applied.
function lbTSay(t, kind) { lbTell(t, kind); }
// The board: it is drawn into #lsb-lcc by the Command Center scripts after this page is drawn (lbMountBoard). The action bar (the actions on several houses, the faction for Fill and Complete,
// the count of the chosen houses, the question before a write and the result line) is drawn only for a board that passed its checks: a board that did not is shown read-only, with no control that writes. It is drawn
// in three parts that are seated in the board's control bar (lbSeat), not under the board (#914).
function lbInner(l) {
  if (!lbBoardOn()) return lbBoardMissing();
  const hs = l.tasks.map((r) => lbHouse(r, l)), ready = window.LCCM.readBoard(l).ready;
  const who = l.character ? (lbCan(l) ? '' : 'Your character is in no faction, so there is no contribution of yours to change.') : 'This save has no character.'; // the line that named the faction is gone (#930): Fill and Complete name '(your faction)' where a faction is chosen
  return html`<section class="card d-panel" aria-labelledby="lsb-board-h" id="lsb-board"><h2 class="d-head" id="lsb-board-h" tabindex="-1">Houses <span class="cnt">${nn(hs.length, 'house', 'houses')}</span></h2>
  ${who ? html`<p class="line1">${who}</p>` : ''}
  <div id="lsb-lcc"></div>
  ${hs.length && ready ? lbActbar(hs, l, true) : ''}</section>${lbTermOn() ? html`<div id="lsb-lcc-term"></div>` : html`${lbTerm(l)}${lbHistory(l)}`}`; // with the Command Center scripts the term section is seated in the board under the plaque (lbSeat), drawn once (#923)
}
// What the page says when the Command Center scripts did not load: there is no other view of the Houses, so this says so and how to get the board back. Nothing can be written from here.
const lbBoardMissing = () => html`<section class="card d-panel" aria-labelledby="lsb-board-h" id="lsb-board"><h2 class="d-head" id="lsb-board-h" tabindex="-1">Houses</h2><p class="nodata" role="alert">The Landsraad board could not be loaded, so the Houses cannot be shown. Reload the page (Ctrl+F5). If this stays, close the editor and start it again.</p></section>`;
const lbPage = (l) => html`<div class="lsb" id="lsb">${lbInner(l)}</div>`;
function lbSumText(hs, l) {
  const sh = lbBoardOn() && window.LCCB ? window.LCCB.visible(l) : null; // chosen AND shown: a House the filters hide is never counted (tau #974)
  const ch = hs.filter((h) => LP.pick.has(h.id) && (!sh || sh.has(h.id))), F = lbFacId(l);
  if (!ch.length) return ''; // no standing "Nothing is chosen." (#1062); the live region speaks when something is chosen
  return `Chosen: ${nn(ch.length, 'house', 'houses')}. Fill chosen changes ${ch.filter((h) => !lbFillOffF(h, l, F)).length}, Complete chosen changes ${ch.filter((h) => !lbDoneFor(h, F)).length} for ${lbFacName(l, F)}, Complete and fill chosen changes ${ch.filter((h) => lbOwnF(l, F) && (!lbFillOffF(h, l, F) || !lbDoneFor(h, F))).length}, Reopen chosen changes ${ch.filter((h) => h.taken).length}, Unfill chosen changes ${ch.filter((h) => lbOwnF(l, F) && h.mine > 0 && h.unfill).length}.`;
}
const lbLetter = (name) => (/^atreides/i.test(String(name || '')) ? 'a' : /^harkonnen/i.test(String(name || '')) ? 'h' : '');
function lbActbar(hs, l) {
  const F = lbFacId(l), mineF = lbCan(l) ? l.character.faction_id : null, cf = LP.confirm;
  // The bar is drawn as two parts that the page seats in the board's control bar (lbSeat): the actions with the faction box and the count of the chosen houses, and the question, the result line and the note on a
  // write directly under it (#914). Choosing every shown House is the Filter menu's All (lbPickShown, through the host).
  const bulkWhy = (act) => lbBulkOff(act, l, F), bulkOff = (act) => (bulkWhy(act) ? html`aria-disabled="true" data-why="${bulkWhy(act)}"` : ''), bulkTip = (act, text) => bulkWhy(act) || text;
  const ask = !cf ? '' : html`<p id="lsb-confirm" class="confirm" data-f="${lbLetter(((l.factions || []).find((x) => x.id == cf.F) || {}).name)}" role="group" aria-label="Confirm: ${cf.q}"><b>${cf.q}</b> <button type="button" class="qa btnp" data-lb="cyes" data-fk="c-yes">${cf.yes}</button><button type="button" class="qa" data-lb="cno" data-fk="c-no">Cancel</button></p>`;
  {
    // The controls of the board (tau #974, the approved design E6 v2), drawn as parts that the board seats in its sticky strip (lbSeat):
    // The action mode: the count of the chosen Houses, the faction crests, Fill, Complete, Reopen (Complete and fill and Unfill are under More) and Clear; and the question. The faction box stays as the
    // engine behind the crests (hidden), so lbFacId and the board's faction path are unchanged. The counts on the keys are filled in by lbBarSync, the icons by the board (lcc-k-icph).
    const facs = l.factions || [], facName = (facs.find((x) => x.id == F) || {}).name || '', ic = (n) => html`<span class="lcc-k-icph" data-ic="${n}"></span>`;
    const seats = facs.map((f) => html`<button type="button" role="radio" class="lcc-k-seat lcc-k-fr lcc-k-tipped" data-lk="seat" data-fid="${f.id}" data-f="${lbLetter(f.name)}" aria-checked="${f.id == F ? 'true' : 'false'}" tabindex="${f.id == F ? '0' : '-1'}" aria-label="${f.name}${f.id == mineF ? ', your faction' : ''}" data-fk="seat:${f.id}"><span class="lcc-k-crest" aria-hidden="true"></span><span class="lcc-k-lamp" aria-hidden="true"></span><span class="lcc-k-tip lcc-k-tip-wide" aria-hidden="true">${f.name}${f.id == mineF ? ', your faction' : ''}. Fill and Complete act for this faction. It starts on your own faction. Choosing it writes nothing.</span></button>`);
    const key = (act, cls, label, icn, tip, pos) => html`<button type="button" class="lcc-k-btn lcc-k-fr ${cls} lcc-k-tipped ${pos}" data-lk="act" data-lb="bulk" data-do="${act}" data-fk="b-${act}" aria-label="${label}">${ic(icn)}<span class="lcc-k-bl">${act === 'fill' ? 'Fill' : act === 'complete' ? 'Complete' : 'Reopen'}</span><span class="lcc-k-n" data-lk="chg" hidden></span><span class="lcc-k-tip lcc-k-tip-wide" aria-hidden="true">${tip}</span></button>`;
    const more = (act, label, tip) => html`<button type="button" class="lcc-k-btn lcc-k-fr lcc-k-tipped" data-lb="bulk" data-do="${act}" data-fk="b-${act}" ${bulkOff(act)}>${label}<span class="lcc-k-tip lcc-k-tip-wide" aria-hidden="true">${bulkTip(act, tip)}</span></button>`;
    return html`<div class="actbar">
      <div class="barpart bp-act" data-bar="act" role="group" aria-label="Act on the chosen Houses">
        <div class="lcc-k-zone lcc-k-zone-sel"><div class="lcc-k-med lcc-k-fr lcc-k-tipped lcc-k-tip-below lcc-k-tip-start" role="status"><b data-lk="n" aria-hidden="true">0</b><span class="lcc-k-sr" data-lk="nsr">0 of ${hs.length} Houses chosen</span><span class="lcc-k-tip lcc-k-tip-wide" aria-hidden="true">Houses chosen and shown. A House that a filter hides is never counted and never acted on.</span></div><span class="lcc-k-chosen" aria-hidden="true"><span data-lk="ntotal">/ ${hs.length} chosen</span><span class="lcc-k-hid" data-lk="hid2" hidden></span></span></div>
        <i class="lcc-k-orn" aria-hidden="true"></i>
        <div class="lcc-k-zone lcc-k-zone-do"><div class="lcc-k-seats" role="radiogroup" aria-label="Faction for Fill and Complete">${seats}</div>
          ${key('fill', 'lcc-k-pri', 'Fill for ' + facName, 'fill', 'Raises your contribution on every chosen House to its highest reward tier (never lowered) and writes what the missions pay: Solari, House Scrip and specialization XP. The question shows the totals before anything is written. Does nothing while another faction is chosen.', '')}
          ${key('complete', 'lcc-k-pri', 'Complete for ' + facName, 'done', "Marks every chosen House complete for the faction chosen and sets that faction's progress on it to the House goal (never lowered).", '')}
          ${key('reopen', '', 'Reopen chosen Houses', 'reopen', 'Reopens the chosen Houses: clears the completion and the faction that holds their favor. Progress and contributions stay as they are, so this does not set back what Complete wrote.', 'lcc-k-tip-end')}
          <span class="lcc-k-morewrap"><button type="button" class="lcc-k-btn lcc-k-fr lcc-k-more-key lcc-k-tipped lcc-k-tip-end" data-lk="more" data-fk="b-more" aria-expanded="false" aria-controls="lsb-moremenu" aria-label="More actions on the chosen Houses">${ic('more')}<span class="lcc-k-bl">More</span><span class="lcc-k-tip lcc-k-tip-wide" aria-hidden="true">Complete and fill chosen, and Unfill chosen.</span></button><div class="lcc-k-moremenu" id="lsb-moremenu" role="group" aria-label="More actions on the chosen Houses" hidden>${more('cfill', 'Complete and fill chosen', 'Does Fill chosen and Complete chosen in one write, with one backup and one result line. Does nothing while another faction is chosen: use Complete for that.')}${more('unfill', 'Unfill chosen', 'Sets your contribution to 0 on every chosen House you have a contribution on, so its tiers go back to 0 of 5. The question shows what it would take back before anything is written.')}</div></span>
        </div>
        <span class="lcc-k-spacer"></span><button type="button" class="lcc-k-ghost lcc-k-tipped lcc-k-tip-end lcc-k-tip-below" data-lb="pickno" data-lk="clear" data-fk="b-none" aria-label="Clear">${ic('x')}<span>Clear</span><span class="lcc-k-tip lcc-k-tip-wide" aria-hidden="true">Clears the choice and brings the Filter key back. The Filter menu is not available while Houses are chosen.</span></button>
        <select id="lsb-fac" data-lb="fac" data-fk="fac" hidden tabindex="-1" aria-hidden="true">${facs.map((f) => html`<option value="${f.id}" ${f.id == F ? 'selected' : ''}>${f.name}${f.id == mineF ? ' (your faction)' : ''}</option>`)}</select>
        <p id="lsb-sum" class="lcc-k-sr" aria-live="polite">${lbSumText(hs, l)}</p>
      </div>
      <div class="barpart bp-ask" data-bar="ask">${ask}</div>
    </div>`;
  }
}
// Draw the board again from the loaded data and what is staged, and put the focus back on the control that had it.
function lbRedraw() {
  const root = $('#lsb');
  if (!root || !LD) return;
  const a = document.activeElement, fk = a && a.dataset ? a.dataset.fk : '';
  const sumOf = (d) => { const s = d.querySelector('summary'); return s ? s.textContent.trim() : ''; };
  const opened = new Set([...root.querySelectorAll('details[open]')].map(sumOf)); // a list the person opened stays open
  setHTML(root, lbInner(LD));
  lbMountBoard();
  root.querySelectorAll('details').forEach((d) => { if (opened.has(sumOf(d))) d.open = true; });
  lbStyles(root);
  lbApply();
  if (fk) { const n = root.querySelector('[data-fk="' + fk + '"]'); if (n && n.focus) n.focus(); }
}
// What a click on the board says (a reason, a hint, a cancel): the app's own toast, lower right, one at a time (#943).
function lbTell(t, kind) {
  const err = kind === true;
  toast(t, err ? true : 'note', err ? undefined : toastMs(t), null, { one: 'lbd', lines: true, soft: !err });
}
function lbSay(t, kind) { lbTell(t, kind); }
// One write from the board (#641). The pressed control is marked busy and the board ignores a second click until the answer is in; the page is then read again from the save, so the
// rows show what is stored. The line that says what happened goes through say(): the board's result line, or the term panel's. An error shows its text as the editor gave it.
function lbRunning(on, b) {
  const root = $('#lsb');
  if (!root) return;
  root.classList.toggle('running', on);
  if (on) root.setAttribute('aria-busy', 'true'); else root.removeAttribute('aria-busy');
  if (b) { b.classList.toggle('busy', on); if (on) { if (b.getAttribute('aria-disabled') === 'true') b.dataset.keepoff = '1'; b.setAttribute('aria-disabled', 'true'); } else { if (!b.dataset.keepoff) b.removeAttribute('aria-disabled'); delete b.dataset.keepoff; } }
  root.querySelectorAll('#lsb-fac, #lcc-bd-fac, #lcc-bd-tierset, #lsb-tad, #lsb-trf').forEach((x) => { x.disabled = on; });
}
async function lbRun(b, call, say, focusFk) {
  const inToast = (say === lbSay && lbBoardOn()) || say === lbTSay; // the results of the board are toasts, not a line on the page (#943), and so are the term panel's (#1120)
  if (LP.busy) { say('A write is still running. Wait for it to finish, then click again.'); return false; }
  LP.busy = true;
  lbRunning(true, b);
  const tOpts = inToast ? { one: 'lbd', lines: true } : null; // what this call's own toasts are tagged with (also the error that act() raises for it)
  if (!inToast) say('Writing…');
  let ok = false, conflict = false;
  try {
    await act(async () => {
      try {
        // call() answers with the sentence, or { text, short, warn }: short is the one line of the toast, warn marks a result that is not a plain success
        const r = await call(), o = typeof r === 'string' ? { text: r } : r, hint = String(NOT_SAVED), text = o.text + hint, warn = !!o.warn || hint !== '';
        ok = true;
        if (inToast) { // one result per line: what changed, what was not written and why, the warnings; the hint that the change is only queued is the last line
          const msg = (o.lines || [o.text]).concat(hint ? [hint.trim()] : []).join('\n');
          toast(msg, warn ? 'part' : false, toastMs(msg), null, tOpts); // a warning (queued, left out) stays until it is dismissed, as an error does
        } else {
          say(text, warn ? 'warn' : false); // the result line keeps the whole sentence, and the hint that the change is only queued
          sayAloud(text);
          toast((o.short || o.text) + hint, warn ? 'note' : false, warn ? 9000 : 3500); // a warning, or a change that is only queued, is not drawn as a success and stays longer
        }
      } catch (e) { conflict = e.code === 'conflict'; if (!inToast) { say(e.message, true); sayAloud(e.message); } throw e; } // on the board act() shows the error as a toast that stays until it is dismissed
    }, null, true, false, tOpts);
  } finally { LP.busy = false; }
  // a panel drawn while the write ran (the re-read after it, or a House looked at meanwhile) has its selects built off; the write is over, so they are on again
  document.querySelectorAll('#lsb #lcc-bd-fac, #lsb #lcc-bd-tierset').forEach((x) => { x.disabled = x.id === 'lcc-bd-tierset' && x.options.length < 2; }); // the redraw of a write that reads the board again runs while busy, so the selects of the new panel are built off (#1015): on again now, Set tier only when the House has tiers
  if (ok) { const r = $('#lsb'); if (r && r.classList.contains('running')) lbRunning(false, b); } // the board was not drawn again (the re-read failed): it is usable again
  else {
    lbRunning(false, b);
    if (conflict) { try { await render(true); } catch (e) { /* the message above stays */ } } else lbRedraw(); // a house that changed meanwhile is shown as it is now
  }
  // the focus goes back to the control that was pressed; a control that turned into another one (Complete into Reopen) or went away (Make active on the row that is now active) has a next best
  for (const fk of [].concat(focusFk || [])) { const n = document.querySelector('#lsb [data-fk="' + fk + '"]'); if (n && n.focus) { n.focus({ preventScroll: true }); break; } }
  return ok;
}
// What a raise wrote, as the answer carries it: the missions it stands for and what they paid (Solari, House Scrip, specialization XP by line). queued: the game is running, so nothing is in
// the save yet and the sentence says what is to be written. A raise that crossed no mission says so.
function lbGrantText(r, queued) {
  if (!r) return '';
  const pb = r.missions_paid_before > 0 ? ' ' + lbPaidBefore(r.missions_paid_before) : '';
  if (!r.missions) return r.contribution_raised ? 'No missions ' + (queued ? 'to write' : 'written') + (r.missions_paid_before > 0 ? '.' : ': your own earlier progress on this house already counts for this ground') + pb : '';
  const xp = Object.keys(r.specialization_xp || {}).sort().map((k) => '+' + lbF(r.specialization_xp[k]) + ' ' + k + ' XP');
  return nn(r.missions, 'mission', 'missions') + (queued ? ' to write' : ' written') + ': ' + ['+' + lbF(r.solari) + ' Solari', '+' + lbF(r.house_scrip) + ' House Scrip'].concat(xp).join(', ') + lbXpLeft(r, queued) + pb;
}
// What a removal did to the rewards of the removed tiers, as the answer carries it (landsraad_takeback.go): the waiting ones taken back, the collected ones that stay, and the ones that stay because they
// cannot be tied to the tier with certainty. A removal with no reward row to say anything about says so, so the result is never silent about rewards. queued: the game is running, so it is not done yet.
function lbBackText(r, queued) {
  const p = [], t = r.rewards_taken_back || 0, c = r.rewards_kept_collected || 0, u = r.rewards_kept_unsure || 0, e = r.rewards_kept_earned || 0;
  if (t) p.push((queued ? 'will take back ' : 'took back ') + nn(t, 'waiting reward', 'waiting rewards') + (r.swatches_taken_back ? ' (the house swatch among them)' : ''));
  if (c) p.push(nn(c, 'collected reward', 'collected rewards') + (c === 1 ? ' stays' : ' stay'));
  if (e) p.push(nn(e, 'waiting reward', 'waiting rewards') + (e === 1 ? ' stays' : ' stay') + ' (your own missions cover the tier)');
  if (u) p.push(nn(u, 'reward', 'rewards') + ' left alone (not certain ' + (u === 1 ? 'it belongs' : 'they belong') + ' to the removed tier)');
  return p.length ? p.join('; ') : 'no reward waited for the removed tier, so there was nothing to take back';
}
// the experience that did not fit because a specialization is at its last level, by line (#752): a raise that filled a house to level a line must not read as a success with no word about it
function lbXpLeft(r, queued) {
  const left = r && r.specialization_xp_not_added || {}, ks = Object.keys(left).filter((k) => left[k] > 0).sort();
  return ks.map((k) => '; ' + k + ' is at its last level, +' + lbF(left[k]) + ' XP ' + (queued ? 'will not be added' : 'not added')).join('');
}
// One request to /api/landsraad/apply for one house or several (one edit, one backup, all or none). hs: the houses as they were shown, by id.
async function lbSend(b, items, label, hs, focusFk) {
  const l = LD;
  if (!l.term) { const t = 'This save has no Landsraad term, so there is no board to change.'; if (lbBoardOn()) lbSay(t, true); else { lbSay(t); toast(t, true); } return false; }
  LP.confirm = null;
  const ok = await lbRun(b, async () => {
    const r = await api('/api/landsraad/apply', { term_id: l.term.term_id, houses: items });
    // a win taken from another faction, and the other completion times the clock repair moved (D1-3, D1-4, #611): said in the result line, also when nothing else changed
    const repairNote = r.repaired > 0 ? nn(r.repaired, 'other completion time', 'other completion times') + " put back inside the term's window" : '';
    // a write that only moved other completion times did change something: it does not say there was nothing to change (#775)
    if (!r.changed) return label + ': ' + (repairNote ? (items.length > 1 ? 'the houses were' : 'the house was') + ' already in that state; ' + repairNote : 'nothing to change, ' + (items.length > 1 ? 'the houses were' : 'the house was') + ' already in that state') + '.';
    const takenNote = (r.replaced || []).length ? 'replaces the win of ' + r.replaced.map((x) => x.previous_winner + ' on ' + x.house).join(', ') : '';
    const extra = [takenNote, repairNote].filter(Boolean).map((x) => '; ' + x).join('');
    const queued = String(NOT_SAVED) !== ''; // the game is running: the change is queued, so nothing here may say it is in the save
    const rows = r.reward_rows ? nn(r.reward_rows, 'reward', 'rewards') + (queued ? ' to leave' : ' now') + ' waiting at the house representative' : '';
    // what the missions of a raise write (landsraad_grant.go), and the warning when rewards were left out because the character's own events already crossed them (#656)
    // an unfill sets the contribution to 0 on every House it changes: it says so, not "a tier removed" (#932)
    const lowered = (n_) => (label === LB_BULK.unfill[0] ? 'your contribution set to 0 on ' + nn(n_, 'house', 'houses') + ' (every tier removed)' : 'lowered on ' + nn(n_, 'house', 'houses') + ' (a tier removed)');
    if (r.specialization_xp && Object.keys(r.specialization_xp).length) { SPV.stale = true; SPV.wrAt = ++SPV.n; } // the levels the board shows came from before this write (#646); the Landsraad read that follows brings the new ones (#1108)
    const grant = lbGrantText(r, queued), left = r.rewards_already_given, back = r.contribution_lowered ? lbBackText(r, queued) : '';
    const warn = left ? 'Warning: ' + nn(left, 'reward', 'rewards') + (left === 1 ? ' was' : ' were') + ' left out: earlier progress on this house already counted for ' + (left === 1 ? 'it' : 'them') + '. Nothing to do' : '';
    const tail = (r.contribution_lowered ? ' ' + LB_REMOVE_NOTE : '') + (r.missions ? ' ' + GRANT_UNCHECKED : '');
    let text;
    if (items.length === 1 && hs.get(items[0].task_id)) text = label + ': ' + lbChangeText(hs.get(items[0].task_id), l, items[0]) + (r.completed && items[0].complete_for == null ? '; house taken, its faction reached the goal' : '') + (rows ? '; ' + rows : '') + (back ? '; ' + back : '') + (grant ? '; ' + grant : '') + extra + (warn ? '; ' + warn : '') + '.' + tail; // the notes (a win taken away, times moved) come before the warning, so they are not inside its sentence (#775)
    else {
      const parts = [r.contribution_raised ? 'your contribution raised on ' + nn(r.contribution_raised, 'house', 'houses') : '', r.contribution_lowered ? lowered(r.contribution_lowered) : '',
        rows, back, grant, r.completed ? nn(r.completed, 'house', 'houses') + ' completed' : '', r.reopened ? nn(r.reopened, 'house', 'houses') + ' reopened' : ''].filter(Boolean);
      text = label + ': ' + parts.join(', ') + extra + (warn ? '; ' + warn : '') + '.' + tail;
    }
    // the board's toast says it in lines (#943): what changed, what was not written and why, the warnings, the notes
    const dot = (x) => (/[.!?]$/.test(x) ? x : x + '.');
    const h1 = items.length === 1 ? hs.get(items[0].task_id) : null;
    const lines = [h1
      ? dot(label + ': ' + lbChangeText(h1, l, items[0]) + (r.completed && items[0].complete_for == null ? '; house taken, its faction reached the goal' : '') + (rows ? '; ' + rows : '') + extra)
      : dot(label + ': ' + ([r.contribution_raised ? 'your contribution raised on ' + nn(r.contribution_raised, 'house', 'houses') : '', r.contribution_lowered ? lowered(r.contribution_lowered) : '', rows, r.completed ? nn(r.completed, 'house', 'houses') + ' completed' : '', r.reopened ? nn(r.reopened, 'house', 'houses') + ' reopened' : ''].filter(Boolean).join(', ') || 'written') + extra)];
    if (back) lines.push(dot(back.charAt(0).toUpperCase() + back.slice(1)));
    if (grant) lines.push(dot(grant));
    if (left) lines.push('Warning: ' + nn(left, 'reward', 'rewards') + (left === 1 ? ' was' : ' were') + ' left out: earlier progress on this house already counted for ' + (left === 1 ? 'it' : 'them') + '.');
    if (r.contribution_lowered) lines.push(LB_REMOVE_NOTE);
    if (r.missions) lines.push(GRANT_UNCHECKED);
        const short = r.missions ? label + ': ' + nn(r.missions, 'mission', 'missions') + (queued ? ' queued.' : ' written.') + (lbXpLeft(r, queued) ? ' Some XP ' + (queued ? 'will not be added' : 'not added') + ': a specialization is at its last level.' : '') : r.contribution_lowered ? label + ': tier removed; ' + back + '.' : '';
    return { text, short, warn: !!left, lines };
  }, lbSay, focusFk);
  // the result line is at the top of the board and the click may be far below it: bring it into view, and give it the focus when the control that was pressed is no longer there to take it
  const res = $('#lsb-result'), a = document.activeElement, lost = !a || a === document.body || (a.closest && a.closest('[hidden]'));
  if (res) {
    lbReveal(res);
    if (lost) res.focus({ preventScroll: true });
  } else if (lost && lbBoardOn()) { const hd = $('#lsb-board-h'); if (hd) hd.focus({ preventScroll: true }); } // the board has no result line: the focus goes to the heading of the board, not to the top of the page
  return ok;
}
const LB_BULK = { fill: ['Fill chosen', 'Fill'], complete: ['Complete chosen', 'Complete'], reopen: ['Reopen chosen', 'Reopen'], unfill: ['Unfill chosen', 'Unfill'], cfill: ['Complete and fill chosen', 'Complete and fill'] };
// Why Unfill chosen and Complete and fill chosen are off, or '' when they are on. Both change the character's own contribution, which counts for the character's own faction: with another faction chosen
// they do nothing, and the page says why instead of leaving the House out without a word (#932). Complete chosen is the button that acts for another faction.
function lbBulkOff(act_, l, F) {
  if (act_ !== 'unfill' && act_ !== 'cfill') return '';
  if (!lbCan(l)) return 'Your character is in no faction, so there is no contribution of yours to change.';
  if (lbOwnF(l, F)) return '';
  const own = lbFacName(l, l.character.faction_id), fn = lbFacName(l, F);
  return LB_BULK[act_][0] + ' is off while ' + fn + ' is chosen: it changes your own contribution, which counts for ' + own + '. Choose ' + own + ' above' + (act_ === 'cfill' ? ', or use Complete chosen for ' + fn : '') + '.';
}
// What Unfill chosen says before it writes (#932), from what the board's read says each House would do (h.unfill, the server's rule): how many Houses go to 0, the waiting rewards it takes back, the ones that
// stay and why. The numbers are the server's; the request carries the count of rewards taken back, so the write is refused when it differs.
function lbUnfillSay(items, hs) {
  const hl = items.map((it) => hs.get(it.task_id)), by = { take: [], collected: [], earned: [], unsure: [] }, why = new Set();
  for (const h of hl) for (const r of (h.unfill && h.unfill.rows) || []) {
    if (!by[r.outcome]) continue;
    by[r.outcome].push(lbF(r.amount) + ' ' + lbItemName(r.template_id) + (r.swatch ? ' (the house swatch)' : '') + ' on House ' + h.name);
    if (r.outcome === 'unsure' && r.why) why.add(r.why);
  }
  const list = (a) => (a.length > 6 ? a.slice(0, 6).join(', ') + ' and ' + (a.length - 6) + ' more' : a.join(', '));
  const p = ['Your contribution goes to 0 on ' + nn(hl.length, 'house', 'houses') + ': every tier you have reached there comes off.'];
  if (by.take.length) p.push('Takes back ' + nn(by.take.length, 'waiting reward', 'waiting rewards') + ': ' + list(by.take) + '.');
  if (by.collected.length) p.push(nn(by.collected.length, 'reward', 'rewards') + ' already collected ' + (by.collected.length === 1 ? 'stays' : 'stay') + ': ' + list(by.collected) + '.');
  if (by.earned.length) p.push(nn(by.earned.length, 'waiting reward', 'waiting rewards') + (by.earned.length === 1 ? ' stays' : ' stay') + ' (your own missions in the game cover ' + (by.earned.length === 1 ? 'its tier' : 'their tiers') + '): ' + list(by.earned) + '.');
  if (by.unsure.length) p.push(nn(by.unsure.length, 'reward', 'rewards') + (by.unsure.length === 1 ? ' stays' : ' stay') + ' (' + [...why].join('; ') + '): ' + list(by.unsure) + '.');
  if (!by.take.length && !by.collected.length && !by.earned.length && !by.unsure.length) p.push('No reward waits for the removed tiers, so nothing is taken back.');
  p.push('A House that is taken stays taken, and so does your faction’s progress.');
  return p.join(' ');
}
// What Complete and fill chosen says before it writes: how many Houses get your contribution raised and how many are taken, for the faction chosen (the missions the fill pays are said by lbGrantSay, as for Fill chosen).
function lbCfillSay(items, F) {
  const fill = items.filter((it) => it.mine_to !== undefined).length, done = items.filter((it) => it.complete_for != null).length;
  return 'Raises your contribution to the top tier on ' + nn(fill, 'house', 'houses') + ' (never lowered) and takes ' + nn(done, 'house', 'houses') + ' for ' + lbFacName(LD, F) + ': its progress goes to the goal.';
}
function lbBulkItems(act_, ids, F, i) {
  const hs = new Map(LD.tasks.filter((r) => ids.has(r.id)).map((r) => [r.id, lbHouse(r, LD)]));
  return { hs, items: [...hs.values()].map((h) => lbItem(h, LD, act_, F, i)).filter(Boolean) };
}
// What a set of requests grants, from the houses as they are shown: the sum, or the first reason one of them would be refused.
function lbGrantOfItems(items, hs) {
  const gs = items.map((it) => { const h = hs.get(it.task_id); return it.mine_to !== undefined && h && it.mine_to > h.mine ? lbGrantOf(h, LD, h.mine, it.mine_to) : null; });
  const bad = gs.find((g) => g && g.problem);
  if (bad) return bad;
  const sum = lbGrantSum(gs), max = LD.grant ? LD.grant.max_missions : 0;
  // the cap is on the whole request, counting every house in it
  if (max && sum.missions > max) return { problem: `This request stands for ${lbF(sum.missions)} missions in all, more than the ${lbF(max)} one change may grant, so nothing is changed. Fill fewer houses at once, or set LandsraadContributionMultiplier in ServerCustomSettings.ini (Config tab) nearer to 1.` };
  return sum;
}
// The question before a write and the name of the button that answers it: what it does, to how many houses, and for a raise what it grants. one: a click on one house (its name is in the
// question), not a bulk button.
function lbAskOf(act_, items, hs, F, i, one) {
  const n = items.length, h1 = hs.get(items[0].task_id);
  const removal = act_ === 'tier' && !!items[0].lower, rt = removal ? h1.tiers[h1.reached - 1] : null; // a removal removes the highest reached tier
  const what = one ? (act_ === 'tier' ? (removal ? 'Remove tier ' + i + ' on House ' + h1.name : 'Give tier ' + i + ' on House ' + h1.name) : act_ === 'fillgoal' ? 'Fill faction goal on House ' + h1.name : LB_BULK[act_][1] + ' House ' + h1.name) : LB_BULK[act_][1] + ' ' + nn(n, 'house', 'houses');
  const g = lbGrantOfItems(items, hs), raises = !!LD.grant && items.some((it) => it.mine_to !== undefined);
  const unf = act_ === 'unfill', cf = act_ === 'cfill';
  const q = what + (act_ === 'complete' || act_ === 'fillgoal' || cf ? ' for ' + lbFacName(LD, F) : '') + '?' + (removal && rt ? ' ' + lbBackSay(rt) + ' ' + LB_REMOVE_NOTE : '') + (unf ? ' ' + lbUnfillSay(items, hs) + ' ' + LB_REMOVE_NOTE : '') + (cf ? ' ' + lbCfillSay(items, F) : '') + (raises && !unf ? ' ' + lbGrantSay(g) : '') + (lbBoardOn() || unf || cf ? lbBoardAsk(items, hs, F, one) : '') + ' Written at once, with a backup first.';
  return { yes: what, q, missions: g.problem ? 0 : g.missions, removal: unf || (removal && lbBackAsks(rt)) }; // an unfill always asks: it takes rewards back
}
// The houses of a request whose win is taken from another faction: Complete (or Fill faction goal) for F on a house that another faction took.
const lbReplaced = (items, hs, F) => items.filter((it) => it.complete_for != null).map((it) => hs.get(it.task_id)).filter((h) => h && h.win != null && h.win != F);
// What the Command Center board's question says beyond the plain one: whose win a Complete replaces, which houses a question about several is about, and
// how many of the chosen houses are left out because nothing would change for them.
function lbBoardAsk(items, hs, F, one) {
  const p = [], rep = lbReplaced(items, hs, F);
  if (rep.length) p.push('Replaces the ' + (rep.length === 1 ? 'win' : 'wins') + ' of ' + rep.map((h) => lbFacName(LD, h.win) + ' on House ' + h.name).join(', ') + '.');
  if (!one && items.length > 1) {
    const nm = items.map((it) => hs.get(it.task_id).name).sort((a, b) => a.localeCompare(b));
    p.push('Houses: ' + (nm.length > 8 ? nm.slice(0, 8).join(', ') + ' and ' + (nm.length - 8) + ' more' : nm.join(', ')) + '.');
  }
  const left = LP.pick.size - items.length;
  if (!one && left > 0) p.push(nn(left, 'other chosen house is', 'other chosen houses are') + ' left alone: nothing to change for ' + (left === 1 ? 'it' : 'them') + '.');
  return p.length ? ' ' + p.join(' ') : '';
}
// The question is closed and the strip is told at once (LCCB.syncStrip): the panel keeps data-state="confirm", which hides the action keys, until the strip is synced; the focus calls that follow need the keys shown (tau #974).
function lbCancelConfirm() { if (!LP.confirm) return; LP.confirm = null; const c = $('#lsb-confirm'); if (c) c.remove(); if (window.LCCB && window.LCCB.syncStrip) window.LCCB.syncStrip(); }

// A question that named a house the board no longer shows is closed with a line that says nothing was written (#818). If Yes or Cancel of that question had the focus, it goes back to the bulk
// button that asked, not to the top of the page (#832).
function lbCloseStale() {
  const a = document.activeElement, held = !!(LP.confirm && a && a.closest && a.closest('#lsb-confirm')), back = LP.confirm ? LP.confirm.back : '';
  lbCancelConfirm();
  if (held && back) lbFocusFk(back);
  lbSay('The chosen houses changed, so the question was closed. Nothing was written.');
}
function lbFocusFk(fk) { let n = $('#lsb [data-fk="' + fk + '"]'); if (n && n.closest && n.closest('.lcc-k-moremenu[hidden]')) n = $('#lsb [data-fk="b-more"]') || n; /* Complete and fill and Unfill live in the closed More menu: the focus goes to the More key (tau #974) */ if (n && n.focus) { n.focus({ preventScroll: true }); if (window.LCCB) window.LCCB.keepInView(n); } }
// One click, or a bulk button: write it, or ask first. Several houses always ask; one house asks when its click would write LB_ASK_MISSIONS missions or more. The question is drawn in the
// action bar; the answer sends the request again, worked out from the board as it is then.
function lbGo(b, act_, items, hs, F, i, label, backFk, one) {
  const q = lbAskOf(act_, items, hs, F, i, one);
  // on the board a click that takes a win from another faction asks first too (it is asked, never written at once)
  // (a bulk button with several houses chosen always asks, even when only one of them would change: the question says the others are left alone, #879)
  if (items.length === 1 && (one || LP.pick.size <= 1) && q.missions < LB_ASK_MISSIONS && !q.removal && !(lbBoardOn() && lbReplaced(items, hs, F).length)) { lbSend(b, items, label, hs, backFk); return; }
  LP.confirm = { act: act_, F, i, ids: items.map((x) => x.task_id), n: items.length, q: q.q, yes: q.yes, label, back: backFk, one: !!one };
  lbRedraw();
  lbFocusFk('c-no'); // the safe answer has the focus, so a stray Enter does not write
  const c = $('#lsb-confirm');
  if (c) intoView(c);
}
// The ids of the houses the page shows now (the board's filters), or null when the page cannot say. The actions on several houses write only houses that are in it (#881): what
// is not shown is never written, whatever the choice list holds (a box ticked by a script or left over from an earlier view is dropped here, at the write, not only when a filter changes).
function lbShownIds() {
  return lbBoardOn() ? window.LCCB.visible(LD) : null;
}
// Takes the houses that are not shown out of the choice (and its boxes); how many went.
function lbDropHidden() {
  const shown = lbShownIds();
  if (!shown) return 0;
  let n = 0;
  for (const id of [...LP.pick]) if (!shown.has(id)) { LP.pick.delete(id); n++; }
  if (n) { document.querySelectorAll('#lsb .pick, #lsb .lcc-bd-pick').forEach((pk) => { const o = pk.closest('[data-id]'); if (o && !LP.pick.has(+o.dataset.id)) pk.checked = false; }); lbSumNow(); }
  return n;
}
function lbBulk(b, act_) {
  const F = lbFacId(LD), off = lbBulkOff(act_, LD, F);
  if (off) { lbSay(off); return; }
  const gone = lbDropHidden(), ids = new Set(LP.pick);
  if (!ids.size && gone) { lbSay('The chosen houses are not in the current filter, so nothing is chosen and nothing was written.'); return; }
  if (!ids.size) { lbSay('Nothing is chosen, so there is nothing to ' + (act_ === 'cfill' ? 'complete and fill' : act_) + '.'); return; }
  const { hs, items } = lbBulkItems(act_, ids, F);
  // the read must say what an unfill would do to the rewards of every House it lowers: without it the question could not be honest, so nothing is asked and nothing is written
  if (act_ === 'unfill' && [...hs.values()].some((h) => h.mine > 0 && !h.unfill)) { lbSay('Unfill chosen cannot say what it would do to the rewards, so nothing was changed. Reload the page and try again.', true); return; }
  if (!items.length) { lbSay(LB_BULK[act_][0] + ': nothing to change on the chosen houses.'); return; }
  lbGo(b, act_, items, hs, F, undefined, LB_BULK[act_][0], 'b-' + act_, false);
}
function lbConfirmYes() {
  const c = LP.confirm;
  if (!c) return;
  // a bulk question writes only houses that are chosen now: a filter or a jump unticks the rows it hides, and a house shown as not chosen is not written (#818). A click on one house names itself.
  if (!c.one) lbDropHidden(); // a house the page does not show is not written, even when the choice still holds it (#881)
  const ids = c.one ? c.ids : c.ids.filter((id) => LP.pick.has(id));
  const { hs, items } = lbBulkItems(c.act, new Set(ids), c.F, c.i);
  if (!items.length) { LP.confirm = null; lbSay('Nothing to change any more.'); lbRedraw(); lbFocusFk(c.back); return; }
  const q = lbAskOf(c.act, items, hs, c.F, c.i, c.one);
  if (items.length !== c.n || q.q !== c.q) { // the houses are not what the question named: ask again, with the count and the amounts as they are now
    LP.confirm = { ...c, ids: items.map((x) => x.task_id), n: items.length, q: q.q, yes: q.yes };
    lbSay('The houses changed since the question was asked. Check the question and answer again.'); lbRedraw(); lbFocusFk('c-no');
    return;
  }
  lbSend($('#lsb [data-fk="c-yes"]'), items, c.label, hs, c.back);
}
// Keeps the line that counts the chosen houses right: the board has no rows of its own to order here, its filters and its sort are the Command Center board's (lcc-board.js).
function lbApply() {
  if (!$('#lsb') || !LD) return;
  lbSumNow();
}
function lbSumNow() { const sm = $('#lsb-sum'); if (sm && LD) sm.textContent = lbSumText(LD.tasks.map((r) => lbHouse(r, LD)), LD); lbBarSync(); if (window.LCCB && window.LCCB.syncStrip) window.LCCB.syncStrip(); }
// the counts on Fill, Complete and Reopen (how many of the chosen and shown Houses each would change: the figures of the sentence in #lsb-sum); a key that would change none is dimmed (it still answers a click, in words)
function lbBarSync() {
  const bar = $('#lsb [data-bar="act"]');
  if (!bar || !LD || !window.LCCB) return;
  const hs = LD.tasks.map((r) => lbHouse(r, LD)), F = lbFacId(LD), sh = window.LCCB.visible(LD), ch = hs.filter((h) => LP.pick.has(h.id) && sh.has(h.id));
  const n = { fill: ch.filter((h) => !lbFillOffF(h, LD, F)).length, complete: ch.filter((h) => !lbDoneFor(h, F)).length, reopen: ch.filter((h) => h.taken).length };
  for (const k of Object.keys(n)) {
    const b = bar.querySelector('[data-lb="bulk"][data-do="' + k + '"]'), c = b && b.querySelector('[data-lk="chg"]');
    if (c) { c.textContent = String(n[k]); c.hidden = !ch.length; }
    if (b) b.setAttribute('data-zero', ch.length && !n[k] ? 'true' : 'false');
  }
}
// ---- where the parts of the board sit (presentation only: nothing here asks, answers or writes) ----
// The action bar is drawn after the board's box in two parts (data-bar) and the parts are seated in the board's control bar, the bar above the 5 by 5: the
// actions with the faction box and the count of the chosen houses, and the question, the result line and the note on a write directly under the bar, before the cards. So the
// order of the keys is the order of the eyes. The question about one house is drawn in the Actions block of the House Intelligence panel while that panel is on the same house, and the panel shows a copy
// of the result line next to its buttons. The board draws itself again on a select or a filter, so what was put into it goes back to its own place before (lbUnseat) and is put in its place again after
// (lbSeat). The parts and their slots: act and bar (the slot of the question and the result line).
const LB_PARTS = [['act', 'act'], ['ask', 'askbar']];
const lbPart = (k) => $('#lsb [data-bar="' + k + '"]');
const lbBarNode = () => $('#lsb-board > #lsb-result, #lsb .lcc-bd-slot > #lsb-result');
function lbUnseat() {
  const home = $('#lsb-board'), bar = $('#lsb-board > .actbar'), c = $('#lsb-confirm'), tm = $('#lsb-lcc-term');
  if (home && tm && tm.parentNode !== home.parentNode) home.after(tm); // the Term and Decrees section goes back to its own place before the board is drawn again (#923)
  if (bar) {
    bar.hidden = false;
    for (const [k] of LB_PARTS) { const p = lbPart(k); if (p && p.parentNode !== bar) bar.appendChild(p); }
    const ask = lbPart('ask');
    if (c && ask && c.parentNode !== ask) ask.prepend(c);
    return;
  }
  const res = lbBarNode();
  if (home && res && res.parentNode !== home) home.appendChild(res);
}
function lbSeat(container) {
  const lcc = container || $('#lsb-lcc');
  if (!lcc) return;
  const tm = $('#lsb-lcc-term'), tslot = lcc.querySelector('[data-lcc-slot="term"]');
  if (tm && tslot && tm.parentNode !== tslot) tslot.appendChild(tm); // under the plaque, where the decree preview strip was (#923)
  for (const [k, slotName] of LB_PARTS) { const p = lbPart(k), slot = lcc.querySelector('[data-lcc-slot="' + slotName + '"]'); if (p && slot && p.parentNode !== slot) slot.appendChild(p); }
  const home = $('#lsb-board > .actbar');
  if (home && !home.childElementCount) home.hidden = true; // nothing is left in it: no empty box under the board
  const res = lbBarNode(), slot = lcc.querySelector('[data-lcc-slot="bar"]');
  if (res && slot && res.parentNode !== slot) slot.appendChild(res);
  const c = $('#lsb-confirm'), ask = lcc.querySelector('.lcc-bd-insp [data-lcc-slot="ask"]'), insp = ask && ask.closest('.lcc-bd-insp'), q = LP.confirm;
  if (c && ask && q && q.one && q.ids.length === 1 && insp.getAttribute('data-id') === String(q.ids[0])) ask.appendChild(c);
}
// Bring the result line or the question into view, clear of the sticky header and of the banner under it however tall that banner is now (intoView measures them; a fixed scroll offset is only
// right for a banner of one height, #823).
function lbReveal(res) { if (res) intoView(res); }
// The board is drawn by the Command Center scripts, from the loaded read, into #lsb-lcc. host is how those scripts ask this page for what only this page knows (the sentences of a click, the
// houses chosen, the decree, where the layout is remembered); they write nothing themselves.
function lbMountBoard() {
  const m = $('#lsb-lcc');
  if (m && LD && window.LCCB) window.LCCB.mount(m, LD, window.LCC_HOST);
  if (m) lbBarSync();
  const tm = $('#lsb-lcc-term');
  if (tm && LD && window.LCCT) window.LCCT.mount(tm, lbTermModel(LD));
}
// A link that opened the Landsraad tab for one House (askReveal): the House's card on the board (the grid or the list of the board) is brought into view and given the focus,
// the way Jump to house does. Runs once, after the page is drawn. The board's own Inspector already shows the House; this is the part that moves the page to it.
function lbRevealHouse(id) {
  const n = $('#lsb-lcc [data-fk="ht:' + id + '"]');
  if (!n) return;
  const at = n.closest('.lcc-bd-cell, .lcc-bd-row, .hr') || n;
  if (typeof at.scrollIntoView === 'function') at.scrollIntoView({ block: 'center' });
  if (n.focus) n.focus({ preventScroll: true });
}
// After a filter of the board changes: a house the filter leaves out is no longer chosen for the actions on several houses (what is not shown is never written), and a question that named it closes.
function lbBoardFiltered(ids) {
  let dropped = false;
  for (const id of [...LP.pick]) if (!ids.has(id)) { LP.pick.delete(id); if (LP.confirm && !LP.confirm.one && LP.confirm.ids.includes(id)) dropped = true; }
  if (dropped) lbCloseStale();
  lbSumNow();
}
// What the Inspector of the board shows and says for one house: the words and the on/off of lbQa, lbTierTip and lbGrantOf, for the faction chosen.
function lbCtl(id) {
  const r = LD && LD.tasks.find((x) => x.id === id);
  if (!r) return null;
  const h = lbHouse(r, LD), F = lbFacId(LD), q = lbQa(h, LD, F), l = LD;
  return {
    F, facName: q.fn, ownF: q.ownF, can: q.can, busy: LP.busy, // busy: a write is running, so the panel is drawn with its selects off (#882)
    factions: (l.factions || []).map((f) => ({ id: f.id, name: f.name + (f.id == (l.character && l.character.faction_id) && q.can ? ' (your faction)' : '') })),
    fill: { off: q.fillOff, tip: q.fillTip },
    fillgoal: { off: q.goalOff, tip: q.goalTip },
    complete: { off: q.doneF, tip: q.doneF ? 'Already complete for ' + q.fn + ': nothing to change' : crTipC(q.fn) },
    reopen: { off: !h.taken, tip: h.taken ? LB_REOPEN_TIP : 'The house is open: nothing to change' },
    tiers: h.tiers.map((t, k) => { const gr = q.ownF && k + 1 > h.reached ? lbGrantOf(h, l, h.mine, t.th) : null, tip = lbTierTip(h, t, k + 1, q.can, l, true, F, true); return { reward: lbRText(t), off: !q.ownF, tip, title: lbTierTitle(h, t, k + 1, tip, !q.ownF), item: lbItemInfo(h, t, k + 1), rw: t.tmpl === LB_SOLARI ? null : lbRewardRow(h, t, k + 1), grant: gr ? lbGrantShort(gr) : '', rm: q.ownF && k + 1 === h.reached ? lbRemoveLine(t) : '' }; }),
    loc: lbLocModel(h),
  };
}
function lbDecreeNow() {
  const t = LD && LD.term, d = t && t.active_decree_id != null ? lbDecreeById(LD, t.active_decree_id) : null;
  return d ? { name: decreeLabel(d), effect: d.effect || '' } : null;
}
window.LCC_HOST = {
  ctl: lbCtl, card: (id) => { const r = LD && LD.tasks.find((x) => x.id === id); if (!r) return null; const h = lbHouse(r, LD); return lbNode(lbHouseCard(h, LD, LD.tasks.every((x) => !(x.location && x.location.known)))); },
  peek: (id) => { const r = LD && LD.tasks.find((x) => x.id === id), h = r ? lbHouse(r, LD) : null; return h && h.loc.known ? lbNode(lbPeek(h)) : null; },
  tips: (id) => { const r = LD && LD.tasks.find((x) => x.id === id); return r ? lbListTips(lbHouse(r, LD)) : null; },
  loc: (id) => { const r = LD && LD.tasks.find((x) => x.id === id); return r ? lbLocModel(lbHouse(r, LD)) : null; }, say: (m) => lbTell(m), decree: lbDecreeNow, picks: () => LP.pick, filtered: lbBoardFiltered, goPath: specGoPath,
  // the character's level in each specialization, from the answer the Specializations tab read (no request of its own, #646): null until that tab has been opened, and again after a write that gave specialization XP
  specLevels: () => spvLevels(),
  pickShown: lbPickShown, // the Filter menu's All chooses every shown House
  unseat: lbUnseat, seat: lbSeat, // presentation only: where the parts of the action bar, the question and a copy of the result line are drawn
  // the faction box of the Inspector's actions drives the board's own faction box (choosing a faction writes nothing)
  setFac: (v) => { const s = $('#lsb-fac'); if (!s || s.disabled) return false; s.value = String(v); s.dispatchEvent(new Event('change', { bubbles: true })); return true; }, // true when the board's own faction box took it (#882)
  get: (k) => browserStore.get('lsb.' + k), set: (k, v) => browserStore.set('lsb.' + k, v),
};
// In the Command Center the hover card of a House and the place peek are fixed to the window (see the .lcc-bd rule of the style sheet), so a box that scrolls or clips never cuts them. This puts them next to what they
// belong to: --fx is the left edge, --fy the top (below it) or --fb the bottom (above it, when there is no room below). Display only; a phone's bottom sheet ignores them.
function lbFloat(e) {
  const t = e.target && e.target.closest ? e.target : null;
  if (!t) return;
  for (const sel of ['.hw', '.loc']) {
    const w = t.closest(sel);
    if (!w || !w.closest('.lcc-bd')) continue;
    const trg = w.firstElementChild || w, r = trg.getBoundingClientRect(), vw = window.innerWidth || 1024, vh = window.innerHeight || 768;
    w.style.setProperty('--fx', Math.round(Math.max(8, Math.min(r.left, vw - 330))) + 'px');
    if (vh - r.bottom >= 300 || vh - r.bottom >= r.top) { w.style.setProperty('--fy', Math.round(r.bottom) + 'px'); w.style.setProperty('--fb', 'auto'); }
    else { w.style.setProperty('--fy', 'auto'); w.style.setProperty('--fb', Math.round(vh - r.top) + 'px'); }
  }
}
function lbCloseCards() { document.querySelectorAll('.lsb .hw.open').forEach((w) => { w.classList.remove('open'); const b = w.querySelector('.hn'); if (b) b.setAttribute('aria-expanded', 'false'); }); }
// the reason a place cannot be opened is said next to the place (the status line at the top of the board can be out of sight, on a phone above all) and in the status line for a screen reader
function lbLocSay(b, t) {
  lbSay(t, 'warn');
  const loc = b && b.closest ? b.closest('.loc') : null;
  if (!loc) return;
  let n = loc.querySelector('.lnote');
  if (!n) { n = document.createElement('span'); n.className = 'lnote'; n.setAttribute('role', 'status'); loc.appendChild(n); }
  n.textContent = t;
}
// a click on a place the Saved map has a picture of opens the Saved map centred on the house
function lbShowOnMap(h, b) {
  if (!h || !lbOnMap(h)) return;
  if (!window.liveMapShow) { lbLocSay(b, 'The Saved map is not ready yet.'); return; }
  window.liveMapShow(h.loc.map_key, Number(h.loc.x), Number(h.loc.y), h.name, h.id);
  tab = 'livemap'; browserStore.set('tab', tab); render();
}
// Chooses every House the board shows now (the Filter menu's All, through LCC_HOST.pickShown): a House a filter hides is not chosen.
function lbPickShown() {
  lbCancelConfirm();
  if (lbBoardOn()) { const ids = window.LCCB.visible(LD); document.querySelectorAll('#lsb .lcc-bd-pick').forEach((pk) => { if (ids.has(+pk.dataset.id)) { LP.pick.add(+pk.dataset.id); pk.checked = true; } }); }
  lbApply();
}
function lbClick(e) {
  const t = e.target.closest ? e.target : null;
  if (!t) return;
  if (!t.closest('.hw.open')) lbCloseCards();
  // the quick peek (the small map under a place) says "Click the place to open the Saved map": at a narrow window it is a sheet over the place link, and a click on the picture is a click on the place (#1025)
  const peek = t.closest('.loc > .pop'), pl = peek && peek.parentElement.querySelector('[data-lb="map"]');
  const b = pl || t.closest('[data-lb], .hn, .hcl');
  if (t.closest('.hcard') ) { e.preventDefault(); }
  if (!b) return;
  if (b.classList.contains('hn')) { e.preventDefault(); const hw = b.closest('.hw'), was = hw.classList.contains('open'); lbCloseCards(); if (!was) { hw.removeAttribute('data-esc'); hw.classList.add('open'); b.setAttribute('aria-expanded', 'true'); } return; }
  if (b.classList.contains('hcl')) { e.preventDefault(); const hb = b.closest('.hw').querySelector('.hn'); lbCloseCards(); if (hb) hb.focus(); return; }
  const k = b.dataset.lb;
  if (k === 'tier' || k === 'qa') {
    e.preventDefault(); // a button inside the row's label must not open or close the row
    const r = LD.tasks.find((x) => String(x.id) === b.dataset.id);
    if (!r) return;
    if (b.getAttribute('aria-disabled') === 'true') { lbSay(LP.busy ? LB_WAIT : b.dataset.why || b.title || 'Nothing to change'); return; }
    if (LP.busy) { lbSay(LB_WAIT); return; }
    const h = lbHouse(r, LD), F = lbFacId(LD), i = +b.dataset.t, kind = k === 'tier' ? 'tier' : b.dataset.do;
    const it = lbItem(h, LD, kind, F, i);
    if (!it) { lbSay(k === 'tier' ? lbRemoveHint(h, lbBoardOn()) : `House ${h.name}: nothing to change.`); return; }
    lbGo(b, kind, [it], new Map([[h.id, h]]), F, i, 'House ' + h.name, b.dataset.focusfk || b.dataset.fk, true); // focusfk: a press made on the box's behalf (Apply tier in the panel) keeps the focus on the control that was pressed (#883)
    return;
  }
  if (k === 'pickno') { lbCancelConfirm(); LP.pick.clear(); document.querySelectorAll('#lsb .lcc-bd-pick').forEach((pk) => { pk.checked = false; }); lbApply(); if (lbBoardOn() && !$('#lsb [data-lk="fbtn"][aria-expanded="true"]')) lbFocusFk('fbtn'); return; } // Clear brings the Filter key back and the focus goes to it; from the open Filter menu (All with every shown House ticked) the focus stays in the menu
  if (k === 'bulk') { e.preventDefault(); if (LP.busy) { lbSay(LB_WAIT); return; } if (b.getAttribute('aria-disabled') === 'true') { lbSay(b.dataset.why || b.title || 'Nothing to change'); return; } lbBulk(b, b.dataset.do); return; }
  if (k === 'cyes') { e.preventDefault(); if (LP.busy) { lbSay(LB_WAIT); return; } lbConfirmYes(); return; }
  if (k === 'cno') { e.preventDefault(); if (LP.busy) { lbSay(LB_WAIT); return; } const a = LP.confirm && LP.confirm.back; lbCancelConfirm(); lbSay('Cancelled. Nothing was written.'); if (a) lbFocusFk(a); return; }
  if (k === 'tapply' || k === 'tmk') {
    e.preventDefault();
    if (LP.busy) { lbTSay(LB_WAIT); return; } // first, before the button's own refusal too: the running Apply or Make active is drawn aria-disabled, and its title must not replace 'Writing...' (#1019); and before the choice is stored: a click on another row during a write changes nothing, so the drop-downs keep showing what the save holds (#904)
    if (b.getAttribute('aria-disabled') === 'true') { lbTSay(b.title); return; }
    const sel = lbTermSel(LD), ad = k === 'tmk' ? String(b.dataset.id) : sel.ad, rf = sel.rf, dn = (id) => { const d = lbDecreeById(LD, id); return d ? decreeFull(d) : 'none'; };
    // the save holds a reigning faction the list does not offer and the drop-down was left alone: the request leaves the faction as it is (the server refuses to set such a faction)
    // The same when the decree is cleared (none) on a term that has no faction: the request names the decree only, so a save the game wrote with a decree and no faction can still have its decree cleared (#960).
    const keepRf = rf === '' && ((LT.rf === undefined && LD.term && LD.term.reigning_faction_id != null) || (ad === '' && LD.term && LD.term.reigning_faction_id == null));
    // a decree is only in force because a faction won the term: no faction chosen cannot be applied (the server refuses it as well, #960)
    if (k === 'tapply' && rf === '' && !keepRf) { lbTSay(LB_NEED_FACTION, true); return; }
    LT.ad = ad; if (!keepRf) LT.rf = rf;
    const body = keepRf ? { active_decree_id: ad } : { active_decree_id: ad, reigning_faction_id: rf };
    lbRun(b, async () => { await api('/api/landsraad/term', body); return `Applied: active decree ${dn(ad)}, ${keepRf ? 'reigning faction left as it is' : 'reigning faction ' + lbFacName(LD, +rf)}.`; }, lbTSay, k === 'tmk' ? [b.dataset.fk, 'tf:' + b.dataset.id] : b.dataset.fk);
    return;
  }
  if (k === 'tflags') { e.preventDefault(); LT.flags = !LT.flags; for (const x of document.querySelectorAll('[data-flagbox]')) x.hidden = !LT.flags; for (const x of document.querySelectorAll('[data-lb="tflags"]')) { x.setAttribute('aria-expanded', LT.flags ? 'true' : 'false'); x.textContent = LT.flags ? 'Advanced: hide pool flags' : 'Advanced: show pool flags'; } return; } // the pool flags are an Advanced view: this only shows or hides them, nothing is written (#1054)
  if (k === 'tflag') { e.preventDefault(); const id = +b.dataset.id, n = b.dataset.d === '1'; lbRun(b, async () => { await api('/api/landsraad/decree', { id, disabled: n }); return `Decree ${id}: pool flag set to ${n ? 1 : 0}.`; }, lbTSay, b.dataset.fk); return; }
  if (k === 'map') {
    e.preventDefault();
    const h = LD.tasks.map((r) => lbHouse(r, LD)).find((x) => String(x.id) === b.dataset.id);
    if (!h) return;
    if (b.getAttribute('aria-disabled') === 'true') { lbLocSay(b, b.dataset.why || b.title); return; }
    lbShowOnMap(h, b);
    return;
  }
}
function lbChange(e) {
  const t = e.target, k = t.dataset ? t.dataset.lb : '';
  if (k === 'pick') { const id = +t.dataset.id; lbCancelConfirm(); if (t.checked) { const shown = lbShownIds(); if (t.disabled || (shown && !shown.has(id))) { t.checked = false; LP.pick.delete(id); lbApply(); lbSay('That House is not in the current filter, so it cannot be chosen.'); return; } } if (t.checked) LP.pick.add(id); else LP.pick.delete(id); lbApply(); return; }
  if (k === 'fac') { lbCancelConfirm(); LP.fac = +t.value; lbRedraw(); return; }
  if (k === 'tad') { LT.ad = t.value; const pn = $('#lcc-tm-pending'); if (pn && LD) pn.textContent = lbPendingText(LD, t.value); const d = lbDecreeById(LD, t.value), l = t.closest('label'); if (l) { const e2 = l.querySelector('.de'), u = l.querySelector('.du'); if (e2) e2.textContent = d && d.effect ? d.effect : ''; if (u) u.textContent = decreeNote(d); } return; }
  if (k === 'trf') { LT.rf = t.value; lbRedraw(); return; }
}
// The few measured values (a bar's width, a pin's place, a house symbol's colour) are set through the style object after the page is drawn: the page's policy allows no style attribute.
function lbStyles(root) {
  root.querySelectorAll('[data-w]').forEach((e) => { e.style.width = e.dataset.w + '%'; });
  root.querySelectorAll('[data-hue]').forEach((e) => { e.style.setProperty('--h', e.dataset.hue); });
  root.querySelectorAll('[data-px]').forEach((e) => { e.style.left = e.dataset.px + '%'; e.style.top = e.dataset.py + '%'; });
  root.querySelectorAll('[data-box]').forEach((e) => { const [l, t, w, h, ar] = e.dataset.box.split(','); if (l) e.style.left = l + '%'; if (t) e.style.top = t + '%'; if (w) e.style.width = w + '%'; if (h) e.style.height = h + '%'; if (ar) e.style.aspectRatio = ar; }); // a picture window (#1069): where the picture, the marked window and the pin sit
}
function lbWire() {
  const root = $('#lsb');
  if (!root) return;
  lbStyles(root);
  root.addEventListener('click', lbClick);
  root.addEventListener('change', lbChange);
  root.addEventListener('toggle', (e) => { if (e.target && e.target.id === 'lsb-tdp') LT.open = e.target.open; else if (e.target && e.target.getAttribute && e.target.getAttribute('data-key') === 'tm-hist') LT.histOpen = e.target.open; }, true); // the panel stays open over a write
  root.addEventListener('keydown', (e) => {
    if (e.key !== 'Escape') return;
    if (LP.confirm && !LP.busy) { const a = LP.confirm.back; lbCancelConfirm(); lbSay('Cancelled. Nothing was written.'); lbFocusFk(a); return; }
    const a = document.activeElement, w = a && a.closest ? a.closest('.hw') : null;
    const pw = a && a.closest ? a.closest('.loc') : null; if (pw && pw.querySelector(':scope > .pop')) pw.setAttribute('data-esc', ''); // Escape puts a place peek away while the focus stays on its link (#1069)
    lbCloseCards(); if (w) { w.setAttribute('data-esc', ''); const hb = w.querySelector('.hn, [data-lcb="sel"]'); // the name button of the Command Center list
    if (hb && a !== hb) hb.focus(); }
  });
  const clearEsc = (e) => { const w = e.target.closest ? e.target.closest('.hw') : null, pw = e.target.closest ? e.target.closest('.loc') : null; root.querySelectorAll('.hw[data-esc]').forEach((x) => { if (x !== w) x.removeAttribute('data-esc'); }); root.querySelectorAll('.loc[data-esc]').forEach((x) => { if (x !== pw) x.removeAttribute('data-esc'); }); };
  root.addEventListener('mouseover', clearEsc); root.addEventListener('focusin', clearEsc);
  root.addEventListener('mouseover', lbFloat); root.addEventListener('focusin', lbFloat);
  lbApply();
}

async function landsraadView() {
  const l = await api('/api/landsraad');
  await loadCatalog(l.tasks.flatMap((x) => (x.rewards || []).flatMap((y) => [y.template_id, y.shown_template_id])));
  const onBoard = new Set(l.tasks.map((x) => x.id));
  LD = l;
  spvNoteBoardRead(l);
  for (const id of [...LP.pick]) if (!onBoard.has(id)) LP.pick.delete(id);
  // a re-read draws the page again, which takes the focus from Yes or Cancel of an open question; if the board then closes that question (a house it named is hidden now, #818), the focus
  // goes back to the bulk button that asked it, not to the top of the page (#832)
  const a0 = document.activeElement, heldAsk = !!(LP.confirm && a0 && a0.closest && a0.closest('#lsb-confirm')), askBack = heldAsk ? LP.confirm.back : '';
  setHTML($('#main'), lbPage(l));
  lbMountBoard();
  // a write and re-read can leave a chosen House out of what the board shows (a filter, or a state the write changed): the choice is pruned as a click on a filter prunes it, so 'N chosen' and the phone bar never count a dimmed House (#1019)
  if (lbBoardOn()) lbBoardFiltered(window.LCCB.visible(LD));
  lbWire();
  if (heldAsk && !LP.confirm && askBack) lbFocusFk(askBack);
}

// ---------- CONFIG (.ini)
let cfgFile = null;
// The Default and Reset cells of one setting (a file with a table of defaults). A setting with no verified default says so and has no working Reset.
function defaultCells(e, d) {
  if (!d || !d.known) return html`<td class="mut" id="cfgdef${e.line}">default unknown</td><td><button class="b sec sm d-btn sm" disabled aria-describedby="cfgdef${e.line}" title="No verified default for ${e.key}: it is never reset" aria-label="Reset ${e.key} (default unknown)">Reset</button></td>`;
  const range = d.options ? 'one of ' + d.options.join(', ') : d.min != null || d.max != null ? `range ${d.min == null ? '' : d.min} to ${d.max == null ? 'no upper limit' : d.max}` : '';
  return html`<td class="mono" title="${range}">${d.default}${range ? html` <span class="mut">(${range})</span>` : ''}</td><td>${d.atDefault
    ? html`<button class="b sec sm d-btn sm" disabled aria-describedby="cfgat${e.line}" title="Already the default" aria-label="Reset ${e.key} (already the default)">Reset</button> <span class="mut" id="cfgat${e.line}">Already the default.</span>`
    : html`<button class="b sec sm d-btn sm" data-act="cfgReset" data-confirm="Yes, reset" data-line="${e.line}" data-key="${e.key}" data-was="${e.value}" data-to="${d.default}" aria-label="Reset ${e.key} to ${d.default}">Reset</button>`}</td>`;
}
// The row above the table: Reset all, with what it changes and what it leaves alone.
function resetAllRow(rc) {
  const n = rc.change || 0, left = (rc.noDefault || 0) + (rc.otherParts || 0);
  return html`<div class="row"><button class="b bad d-btn danger" data-act="cfgResetAll" data-n="${n}" aria-describedby="rstnote" ${n ? '' : 'disabled'}>Reset all to defaults</button><span class="mut" id="rstnote">${n ? `${nn(n, 'value differs', 'values differ')} from the default and would change.` : 'Every setting that has a known default is already at it.'} ${nn(rc.atDefault || 0, 'value is', 'values are')} already at the default${left ? `; ${nn(left, 'entry has', 'entries have')} no known default (or is not a game setting) and ${left == 1 ? 'is' : 'are'} left as ${left == 1 ? 'it is' : 'they are'}` : ''}.</span></div>`;
}
// ---------- the Help tab (#980): the pages of docs/help, embedded in the program and read from its own /api/help, drawn by help.js with createElement and textContent only. It works with no network;
// the optional "Also on GitHub" link is the same page in the repository and is the only thing here that needs one. The page last read is remembered in the browser's storage.
let HELP = null;
const HELP_REPO = /^https:\/\/github\.com\/Project-Arrakis\/tabr-tau\//;
function helpPageNow() {
  const pages = (HELP && HELP.pages) || [];
  const want = browserStore.get('help.page');
  return pages.find((p) => p.id === want) || pages[0] || null;
}
function helpDraw() {
  const cur = helpPageNow(), host = $('#helpbody');
  if (!cur || !host || !window.TabrHelp) return;
  const files = {};
  for (const p of HELP.pages) files[p.file] = p.id;
  host.textContent = '';
  host.appendChild(window.TabrHelp.render(cur.body, { files, base: 3 }));
}
async function helpView() {
  if (!HELP) HELP = await api('/api/help');
  const pages = (HELP && HELP.pages) || [], cur = helpPageNow();
  if (!cur || !window.TabrHelp) { setHTML($('#main'), html`<div class="card d-panel"><h2 class="d-head">Help</h2><p class="mut">The Help pages could not be read.</p></div>`); return; }
  const gh = typeof HELP.repo === 'string' && HELP_REPO.test(HELP.repo) ? HELP.repo + cur.file : '';
  setHTML($('#main'), html`<div class="card d-panel helpbox"><h2 class="d-head" id="help-h">Help</h2>
    <div class="sub helpnav" role="group" aria-label="Help pages">${pages.map((p) => html`<button type="button" data-help="${p.id}" class="${p.id === cur.id ? 'on' : ''}" aria-current="${p.id === cur.id ? 'page' : 'false'}">${p.id === 'index' ? 'Contents' : p.title}</button>`)}</div>
    <article class="helpbody" id="helpbody" aria-labelledby="help-h"></article>
    ${gh ? html`<p class="helpgh"><a href="${gh}" target="_blank" rel="noopener noreferrer" title="The same page in the repository. It needs a network; the Help here does not.">Also on GitHub</a></p>` : ''}</div>`);
  helpDraw();
}

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
    ? html`<div class="warn"><b>Config check:</b><ul class="tight">${problems.map((f) => html`<li><code>${f.name}</code> — ${f.exists ? 'missing sections: ' + (f.missingSections || []).join(', ') : 'file not found'} <button class="b sm d-btn primary sm" data-act="cfgcreate" data-n="${f.name}">${f.exists ? 'Add sections' : 'Create'}</button></li>`)}</ul></div>`
    : html`<div class="mut mb8">✓ All expected config files are present.</div>`];
  h.push(html`<div class="warn">The game rewrites its config files when it exits. Close Dune before saving changes here. Each save keeps a backup in a <code>tabr-tau-backups</code> folder next to the file (the Backups list is below).</div>
  <div class="row"><select id="cfgsel" aria-label="Config file">${files.map((f) => html`<option value="${f.name}" ${f.name == cfgFile ? 'selected' : ''}>${f.name}${f.empty ? ' (empty)' : ''}</option>`)}</select></div>`);
  if (cfgFile) {
    const c = await api('/api/config/file?name=' + encodeURIComponent(cfgFile));
    const bk = await api('/api/config/backups?name=' + encodeURIComponent(cfgFile));
    let last = null;
    const dmap = Object.fromEntries((c.defaults || []).map((x) => [x.line, x])), rc = c.resetCounts || {};
    const entryRows = c.entries.map((e) => {
      const sec = e.section !== last ? e.section : '';
      last = e.section;
      return html`<tr><td class="mut mono">${sec}</td><td class="mono">${e.key}</td><td><input class="cfgv" aria-label="Value of ${e.key}" value="${e.value}" data-line="${e.line}" data-key="${e.key}" data-section="${e.section}"></td>${c.defaults ? defaultCells(e, dmap[e.line]) : ''}<td><a href="#" data-act="cfgdel" data-line="${e.line}" data-key="${e.key}" class="bad">delete</a></td></tr>`;
    });
    h.push(html`<div class="card d-panel"><h3>${cfgFile}</h3>${c.defaults ? resetAllRow(rc) : ''}${c.entries.length ? html`<div class="scroll"><table><thead><tr><th>Section</th><th>Key</th><th>Value</th>${c.defaults ? html`<th>Default</th><th>Reset</th>` : ''}<th></th></tr></thead><tbody>${entryRows}</tbody></table></div>` : html`<p class="mut">This file is empty.</p>`}
    <div class="row mt12"><b>Add key</b><input id="cs" aria-label="Section of the new key" placeholder="[Section]" list="secs"><datalist id="secs">${[...new Set(c.entries.map((e) => e.section))].map((s) => html`<option value="${s}">`)}</datalist><input id="ck" aria-label="Name of the new key" placeholder="Key"><input id="cv" aria-label="Value of the new key" placeholder="Value"><button class="b d-btn primary" data-act="cfgadd">Add</button></div>${c.defaults ? about('config.defaults', cfgFile, html`<p class="mut">The defaults are the Dune Docker console's table for the game's settings (MIT, RedBlink); a setting without a verified default is never reset.</p>`) : ''}</div>
    <div class="card d-panel"><details><summary>Edit raw text</summary><textarea id="raw" aria-label="Raw text of the config file" rows="18" spellcheck="false">${c.text}</textarea><div class="row"><button class="b d-btn primary" data-act="cfgraw">Save raw text</button></div></details></div>
    <div class="card d-panel"><details><summary>Backups (${bk.length})</summary>${tbl([{ k: 'name', label: 'Backup' }, { k: 'size', label: 'Size', f: (r) => fmtBytes(r.size) }], bk, { empty: 'No backups of this file yet. One is made before a change is written.', actions: (r) => html`<button class="b sec sm d-btn sm" data-act="cfgrestore" data-b="${r.name}">Restore</button>` })}</details></div>`);
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
    h.push(html`<div class="row"><select id="dbsel" aria-label="Table">${t.map((x) => html`<option value="${x.name}" ${x.name == dbTable ? 'selected' : ''}>${x.name} (${x.rows})</option>`)}</select><input id="dbq" aria-label="Search this table" placeholder="Search this table" value="${dbQ}"><button class="b sec d-btn" data-act="dbgo">Search</button>
      <a href="/api/db/export?name=${encodeURIComponent(dbTable)}&amp;format=csv" data-export="csv">CSV</a><a href="/api/db/export?name=${encodeURIComponent(dbTable)}&amp;format=json" data-export="json">JSON</a></div><div id="dbout"></div>`);
    setHTML($('#main'), html`${h}`);
    const d = await api(`/api/db/table?name=${encodeURIComponent(dbTable)}&q=${encodeURIComponent(dbQ)}&offset=${dbOff}&limit=100`);
    const cols = d.columns.slice(1);
    const locked = new Map(Object.entries(d.locked || {})); // a column name from the save is only ever a Map key
    setHTML($('#dbout'), html`<p class="mut">${d.total === 0 ? (dbQ ? 'No rows match the search' : 'No rows in this table') : `${nn(d.total, 'row', 'rows')}, showing ${dbOff + 1}–${Math.min(dbOff + 100, d.total)}`}${d.tableLocked ? html`. Read-only: ${d.tableLocked}.` : ''}</p><div class="scroll"><table><thead><tr>${cols.map((c) => html`<th>${c}</th>`)}</tr></thead><tbody>${d.rows.map((r) => html`<tr>${r.slice(1).map((v, i) => {
      const b = typeof v === 'string' && v.startsWith('<blob');
      const why = d.tableLocked || locked.get(cols[i]);
      return html`<td>${b ? html`<span class="mut mono">${v}</span>` : why ? html`<span class="mono" title="${why}">${v}</span>` : html`<input class="dbc" aria-label="${cols[i]}, row ${r[0]}" value="${v}" data-rowid="${r[0]}" data-kind="${d.columns[0]}" data-col="${cols[i]}" data-orig="${v}" ${v === null ? 'data-null' : ''} size="${Math.max(6, Math.min(40, String(v ?? '').length + 2))}">`}</td>`;
    })}</tr>`)}</tbody></table></div>
      ${d.total > 100 || dbOff > 0 ? html`<div class="row"><button class="b sec d-btn" data-act="dbprev" ${dbOff <= 0 ? 'disabled' : ''}>Previous</button><button class="b sec d-btn" data-act="dbnext" ${dbOff + 100 >= d.total ? 'disabled' : ''}>Next</button></div>` : ''}
      ${!d.tableLocked && locked.size ? about('db.tables', 'Tables', html`<p class="mut">Plain text columns are read-only (primary keys, links to other tables and account identity); hover a value for the reason.</p>`) : ''}`);
    return;
  } else if (s == 'sql') {
    h.push(html`<div class="card d-panel"><h3 class="d-head">SQL</h3><textarea id="sql" rows="5" placeholder="select * from items limit 20">${window.lastSql || 'select name from sqlite_master'}</textarea><div class="row"><button class="b d-btn primary" data-act="runsql">Run (read-only)</button>${tip('sqltip', 'Write mode runs one or more statements atomically. They are written to game.db at once, after a backup. If the file cannot be written (for example the game is running) they are queued and written by themselves when it can be.', html`<button class="b bad d-btn danger" data-act="execsql" aria-describedby="sqltip">Run as write</button>`)}</div><div id="sqlout"></div></div>`);
  } else {
    const b = await api('/api/save/backups');
    h.push(html`<div class="card d-panel"><h3 class="d-head">Save backups</h3><p class="mut">A backup is taken before changes are written and before every restore. Restore needs the game to be closed.</p>${tbl([{ k: 'name', label: 'Backup', f: (r) => html`${r.name}${r.copy ? html` <span class="tag">made from a copy</span>` : ''}` }, { k: 'size', label: 'Size', f: (r) => fmtBytes(r.size) }, { k: 'modified', label: 'Saved', f: (r) => fmtDate(r.modified) }], b, { actions: (r) => html`<button class="b sec sm d-btn sm" data-act="restore" data-n="${r.name}" data-copy="${r.copy ? '1' : ''}">Restore</button>`, empty: 'No backups yet. One is made before the first change is written.' })}${about('db.backups', 'Save backups', html`<p class="mut">One backup covers the changes made within two minutes.</p>`)}</div>`);
  }
  setHTML($('#main'), html`${h}`);
}

// ---------- render + events
// What the gear repair says: the backpack items are counted apart from the worn and loadout items, and only when there are any.
function gearRepairText(r) {
  const b = r.backpackRepaired || 0;
  if (!b) return `Repaired ${r.repaired} items`;
  const bp = `${b} backpack ${b === 1 ? 'item' : 'items'}`;
  return r.repaired ? `Repaired ${r.repaired} worn or loadout ${r.repaired === 1 ? 'item' : 'items'} and ${bp}` : `Repaired ${bp}`;
}

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
  return (notLive ? NOT_LIVE + ' - ' : '') + parts.join(' - ');
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
// scroll is true only for a redraw that follows the person's own action; the page's own refresh (about every twenty seconds while the game runs) gives the focus back and never moves the page (#696).
function refocus(s, scroll) {
  if (!s || (document.activeElement && document.activeElement !== document.body)) return;
  const pool = s.id ? [document.getElementById(s.id)] : [...document.querySelectorAll('#main ' + s.tag + ', #nav ' + s.tag)];
  const same = pool.filter((n) => n && s.data.every(([k, v]) => n.getAttribute(k) === v));
  const n = same.find((x) => (x.getAttribute('aria-label') || x.textContent || '').trim().slice(0, 60) === s.label) || same[0];
  if (n && typeof n.focus === 'function') { n.focus({ preventScroll: true }); if (scroll) intoView(n); }
}
// The focus went back to a control that the kept offset left where it was; the page moves only when that control is off the screen or under the sticky header (#686).
function intoView(n) {
  const r = n.getBoundingClientRect();
  if (!r.width && !r.height) return; // not laid out (hidden, or no layout at all)
  let top = 0;
  document.querySelectorAll('header, #gamewarn, #launchpass, .lcc-k-e6[data-stuck="1"] .lcc-k-strip').forEach((b) => { top = Math.max(top, b.getBoundingClientRect().bottom); });
  const bottom = window.innerHeight; // the strip of the control area sticks to the top (the sticky header and the stuck strip are what covers the window there)
  const by = r.top < top ? r.top - top - 8 : r.bottom > bottom ? r.bottom - bottom + 8 : 0;
  if (by) window.scrollTo(window.scrollX, Math.max(0, window.scrollY + by));
}
// The page is drawn again after an action, and that must not cost the person their place (#365): the scroll offset of the page and of every scrolling table, the
// choices made in the page that the save does not hold (a ticked box, a chosen faction, text typed in a box) and the lists that were opened. viewSnapshot notes them before
// the redraw and viewApply puts them back after it. A control is put back only when what the page would show for it is what it showed before (its default is unchanged):
// a box whose value the save has changed meanwhile shows the new value. The same snapshot is kept in the session storage when the page is left, so a refresh of the page
// (F5) comes back to the same tab, place and choices.
const VIEW_KEY = 'tabr-tau-view';
const VIEW_SKIP = new Set(['button', 'submit', 'reset', 'hidden', 'file', 'password', 'image']);
function ctlKey(el) {
  if (el.id) return '#' + el.id;
  const data = [...el.attributes].filter((a) => a.name.startsWith('data-')).map((a) => a.name + '=' + a.value).join('&');
  return el.tagName + ':' + (el.type || '') + ':' + data + ':' + (el.getAttribute('aria-label') || el.name || '');
}
// Calls fn(kind, element, key) for every control, scrolling table and list in the page, with a key that stays the same across a redraw (the id, or the attributes and
// the label, and the order among the ones that are alike).
function eachViewPart(fn) {
  const main = $('#main');
  if (!main) return;
  const count = new Map();
  const nth = (k) => { const n = count.get(k) || 0; count.set(k, n + 1); return k + '|' + n; };
  main.querySelectorAll('.scroll').forEach((e) => { const h = e.closest('.card') && e.closest('.card').querySelector('h3'); fn('scroll', e, nth('S:' + (h ? h.textContent.trim().slice(0, 40) : ''))); });
  // The part boxes of Create a vehicle belong to its draft, which takes a change in the change event and draws the boxes from itself: a snapshot taken before a refused pick would put
  // the refused part back into them after a redraw that was already on its way (#742).
  main.querySelectorAll('input, select, textarea').forEach((e) => { if (!VIEW_SKIP.has(e.type) && !e.hasAttribute('data-vcprev') && !e.hasAttribute('data-vcslot')) fn('ctl', e, nth(ctlKey(e))); });
  main.querySelectorAll('details').forEach((e) => { const s = e.querySelector('summary'); fn('det', e, nth('D:' + (s ? s.textContent.trim().slice(0, 40) : ''))); });
}
const selDefault = (s) => { const o = [...s.options].find((x) => x.defaultSelected) || s.options[0]; return o ? o.value : ''; };
function viewSnapshot() {
  if (!$('#main') || $('#loading')) return null;
  const snap = { tab, sub: { player: sub.player, char: sub.char, db: sub.db }, y: window.scrollY || 0, boxes: {}, ctl: {}, det: {} };
  eachViewPart((kind, e, key) => {
    if (kind === 'scroll') { if (e.scrollTop || e.scrollLeft) snap.boxes[key] = [e.scrollTop, e.scrollLeft]; }
    else if (kind === 'det') snap.det[key] = e.open;
    else if (e.type === 'checkbox' || e.type === 'radio') { if (e.checked !== e.defaultChecked) snap.ctl[key] = [e.checked, e.defaultChecked]; }
    else if (e.tagName === 'SELECT') { if (e.selectedIndex >= 0 && e.value !== selDefault(e)) snap.ctl[key] = [e.value, selDefault(e)]; }
    else if (e.value !== e.defaultValue) snap.ctl[key] = [e.value, e.defaultValue];
  });
  return snap;
}
// The offsets of the table boxes that stay as they are after a redraw that shows other rows: only the box that holds the control the person pressed (its own rows are
// the same, and a list that jumps back to its top hides the button that was just used, #824).
function heldBoxes(boxes, from) {
  const kept = {};
  if (!from || !from.closest) return kept;
  eachViewPart((kind, e, key) => { if (kind === 'scroll' && e.contains(from) && Object.hasOwn(boxes, key)) kept[key] = boxes[key]; });
  return kept;
}
function viewApply(snap) {
  if (!snap || snap.tab !== tab) return;
  eachViewPart((kind, e, key) => {
    if (kind === 'scroll') { const b = snap.boxes[key]; if (b) { e.scrollTop = b[0]; e.scrollLeft = b[1]; } return; }
    if (kind === 'det') { if (Object.hasOwn(snap.det, key)) e.open = !!snap.det[key]; return; }
    const c = snap.ctl[key];
    if (!c) return;
    if (e.type === 'checkbox' || e.type === 'radio') { if (e.defaultChecked === c[1]) e.checked = c[0]; }
    else if (e.tagName === 'SELECT') { if (selDefault(e) === c[1] && [...e.options].some((o) => o.value === c[0])) e.value = c[0]; }
    else if (e.defaultValue === c[1]) e.value = c[0];
  });
  if (snap.y) window.scrollTo(0, snap.y);
}
function viewSave() {
  try { const s = viewSnapshot(); if (s) sessionStorage.setItem(VIEW_KEY, JSON.stringify(s)); } catch (e) { /* the place is not kept across a refresh */ }
}
// What the page left in the session storage when it was last left; a snapshot that does not read back as the shape above is dropped (the storage is not trusted).
function viewLoad() {
  try {
    const raw = sessionStorage.getItem(VIEW_KEY);
    sessionStorage.removeItem(VIEW_KEY);
    const s = JSON.parse(raw || 'null');
    const rec = (v) => v && typeof v === 'object' && !Array.isArray(v);
    if (!rec(s) || !rec(s.boxes) || !rec(s.ctl) || !rec(s.det) || !rec(s.sub) || typeof s.tab !== 'string' || !(Number(s.y) >= 0)) return null;
    for (const k of Object.keys(s.boxes)) if (!Array.isArray(s.boxes[k]) || s.boxes[k].length !== 2) delete s.boxes[k];
    for (const k of Object.keys(s.ctl)) if (!Array.isArray(s.ctl[k]) || s.ctl[k].length !== 2) delete s.ctl[k];
    s.y = Number(s.y);
    return s;
  } catch (e) { return null; }
}
let reloaded = viewLoad(); // the place the page had when it was refreshed; used once, by the first drawing
if (reloaded && reloaded.tab === tab) {
  if (PTABS.some(([k]) => k === reloaded.sub.player) && !['bases', 'vehicles'].includes(savedTab)) sub.player = reloaded.sub.player;
  if (['overview', 'inventory', 'reputation'].includes(reloaded.sub.char)) sub.char = reloaded.sub.char;
  if (['browse', 'sql', 'backups'].includes(reloaded.sub.db)) sub.db = reloaded.sub.db;
}
window.addEventListener('pagehide', viewSave);
document.addEventListener('visibilitychange', () => { if (document.hidden) viewSave(); });
// The note about a move between maps is drawn only while another map is chosen (#426).
function tmapSync() {
  const sel = $('#tmap'), box = $('#tmapnote');
  const hub = GO.pick !== null && TD[GO.pick] && TD[GO.pick].otherMap; // a hub chosen in the list of places is another map too (#521)
  if (box) setHTML(box, (sel && sel.value !== '') || hub ? html`<p class="warn" id="tmapwarn">${TRAVEL_WARN}</p>` : html``);
}
// render(true) is the redraw after an action (or the page's own refresh from the file): it keeps the place; render('scroll') keeps only the scroll offsets; render('rows') keeps the page offset and the offset of the table box that holds the control pressed (from, or the focused control), for a change that shows other rows in another table box (the old offset of that box is not put back onto them, #697; a list the person scrolled and pressed a button in stays where it is, #824). Any other call is a move to another page, which starts at the top.
// auto is the page's own refresh from the file: it keeps the place the same way, but the page is never scrolled to the refocused control (#696).
let shownTab = null; // the tab the page was last drawn for
// A link that opens a page at one of its parts (a House on the Landsraad tab, a specialization on the Specialization tab) cannot scroll there itself: the page is not drawn yet, and the move to another page starts at the top (#1025).
// It asks with askReveal(fn) before it calls render(); render() runs fn once, after the page is drawn and the view is put back, so the scroll to the part is the last scroll and nothing undoes it (#1041, #1072).
let revealOnce = null;
const askReveal = (fn) => { revealOnce = fn; };
async function render(keepPlace, auto, from) {
  const keep = focusSpec(document.activeElement);
  const place = keepPlace === true || keepPlace === 'scroll' || keepPlace === 'rows' ? viewSnapshot() : reloaded;
  if (place && keepPlace === 'rows') place.boxes = heldBoxes(place.boxes, from || document.activeElement); // other rows in the table boxes: they start at their top, except the box that holds the control just pressed (#824)
  if (place && (keepPlace === 'scroll' || keepPlace === 'rows')) place.ctl = {}; // a redraw to show what the save holds (a refused change): the boxes go back to it, only the place is kept
  reloaded = null;
  if (keepPlace !== true && keepPlace !== 'scroll' && keepPlace !== 'rows') bsInvOpen = null; // a move to another page: no base container is open there (#731)
  if (!place && shownTab !== null && shownTab !== tab) window.scrollTo(0, 0); // a move to another page starts at the top (the browser would keep the offset of the page just left, and a shorter page would end up scrolled to its bottom, #1025); done before the page draws, so a page that scrolls to its own part (the board to a chosen House) still does
  shownTab = tab;
  document.title = pageTitle();
  drawNav();
  try {
    await ({ player: playerView, livemap: liveMapView, landsraad: landsraadView, config: configView, db: dbView, help: helpView })[tab]();
  } catch (e) {
    setHTML($('#main'), html`<div class="card d-panel bad">${e.message}</div><div class="row mt12"><button class="b d-btn primary" data-act="retry">Try again</button></div>`);
  }
  viewApply(place);
  restoreAugBlocks(place);
  tmapSync();
  goSync();
  if (tab === 'player' && $('#tdout')) goPlaces(false);
  refocus(keep, !auto);
  const reveal = revealOnce; revealOnce = null; // asked for this one draw only: a later redraw, or a visit by the tabs, is not scrolled again
  if (reveal) reveal();
}

const D = (e) => e.target.closest('[data-act]');
// A new search or another page of rows starts at the top of the table's own box; the page offset is kept (#686).
function dbRowsTop() { const b = $('#dbout .scroll'); if (b) { b.scrollTop = 0; b.scrollLeft = 0; } }
// A failed SQL statement shows its reason where the result goes, and the earlier result is cleared so it cannot be read as the answer (register 2026-10-07, UX-5).
function sqlFailed(e) { const out = $('#sqlout'); if (out) setHTML(out, html`<p class="bad">${e.message}</p>`); }

// The card of a base container opened with Open (#731): its items, the box that adds one and the clear row. It is built before the page is drawn, so a redraw puts the container in at once and does not empty it first.
async function baseInvCard(d) {
  const it = await api('/api/bases/storage/items?inventory=' + encodeURIComponent(d.id));
  const cat = await loadCatalog(it.map((r) => r.template_id));
  return html`<div class="card d-panel"><h3 title="Inventory number ${d.id}">${d.name ? 'Inventory: ' + d.name : 'Inventory'}</h3>${tbl([{ k: 'position_index', label: 'Slot' }, { k: 'template_id', label: 'Item', f: (r) => itemCell(r.template_id) }, { k: 'stack_size', label: 'Stack', f: (r) => html`<input type="number" min="1" value="${r.stack_size}" data-item="${r.id}" data-field="stack_size" data-name="${itemName(r.template_id)}" class="w90" aria-label="Stack of ${itemName(r.template_id)}">` }, { k: 'fill', label: 'Liquid', f: (r) => fillCell(r) }], it, { empty: 'This container is empty.', actions: (r) => html`<button class="b bad sm d-btn danger sm" data-act="delItem" data-id="${r.id}" data-name="${itemName(r.template_id)}" aria-label="Delete ${itemName(r.template_id)}">Delete</button>` })}
    <div class="row">${itemPicker("bt", cat)}Qty<input id="bq" aria-label="Quantity to add" type="number" value="1" min="1"><button class="b d-btn primary" data-act="giveInv" data-id="${d.id}" data-name="${d.name || ''}">Add item</button></div>${clearRow(d.id, d.name || 'this container', it)}</div>`;
}

// the one request of Add XP; the handler spAddCC checks the number first (#1032). Not a handler of the table: a button reaches it only through spAddCC.
const specXpSend = (d) => act(async () => { const r = await api('/api/player/specs/xp', { track: d.t, amount: +val('spx' + d.t) }); toast(`${d.t}: ${Number(r.before).toLocaleString()} → ${Number(r.after).toLocaleString()} XP, level ${Math.floor(r.level)}` + NOT_SAVED); }, null);
const A = {
  retry: () => render(true),
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
  // Cancel refill (#1123): the automatic refill that waits for the game stops waiting. Nothing is written and no setting changes; the page is told at once.
  cancelRefill: () => act(async () => {
    const r = await api('/api/autopass/cancel', {});
    toast(r.cancelled ? 'Automatic refill cancelled. Nothing was written.' : 'No automatic refill was waiting.');
    try { await status(); } catch (e) { /* the poll reports a lost connection */ }
  }, null, true, true),
  // Check again: the game check is made at once (the process list is read again, never taken from the cache) and the answer is said in a sentence. The banner is redrawn from
  // that answer, so one that said the check could not be made goes away as soon as a check works.
  checkAgain: () => act(async () => {
    const c = await api('/api/game/check', {});
    toast(capFirst(c.message), !!c.failed);
    try { await status(); } catch (e) { /* the poll reports a lost connection */ }
  }, null, false, true),
  // Write now: the same check first, so there is always an answer to give even when nothing is waiting, then another attempt to write what waits.
  writeNow: () => act(async () => {
    const c = await api('/api/game/check', {});
    const r = await api('/api/save/flush', {});
    if (r.saved) toast((notLive ? 'Written to this file' : 'Written to the game file') + (r.warning ? '. ' + r.warning : '.'), !!r.warning);
    else if (r.queued) toast('Still waiting: ' + (c.failed ? c.message : r.why || 'the file cannot be written yet'), true);
    else toast(capFirst(c.message), !!c.failed);
    try { await status(); } catch (e) { /* the poll reports a lost connection */ }
  }, null, true, true),
  solari: () => act(() => api('/api/player/solari', { amount: +val('solari') }), 'Solari updated'),
  bloodMarks: () => act(async () => { const r = await api('/api/player/blood-marks', { amount: +val('bmAmt') }); giveToast(r, 'Blood Marks', `Gave ${Number(r.given).toLocaleString()} Blood Marks (${Number(r.before).toLocaleString()} → ${Number(r.after).toLocaleString()})`); }, null),
  xp: () => act(async () => { const r = await api('/api/player/xp', { amount: +val('xpAmt') }); toast(r.applied ? `XP ${Number(r.before).toLocaleString()} → ${Number(r.after).toLocaleString()}, level ${r.levelBefore} → ${r.levelAfter}` + (r.skillPointsGained ? `, +${r.skillPointsGained} skill points` : '') + (r.capped ? ' (stopped at level 200)' : '') + NOT_SAVED : 'Already at the last level (200).'); }, null),
  currency: () => act(async () => { const r = await api('/api/player/currency', { currency: +val('curSel'), amount: +val('curAmt') }); toast(`${r.currency == 0 ? 'Solari in wallet' : 'House credit in wallet'} ${Number(r.before).toLocaleString()} → ${Number(r.after).toLocaleString()}` + NOT_SAVED); }, null),
  intel: () => act(async () => { const r = await api('/api/player/intel', { amount: +val('intelAmt') }); toast(r.applied ? `Intel ${Number(r.before).toLocaleString()} → ${Number(r.after).toLocaleString()}` + (r.capped ? ' (capped at 2,779)' : '') + NOT_SAVED : 'Intel is already at the cap (2,779).'); }, null),
  teleport: () => act(async () => { const body = { x: +val('tx'), y: +val('ty'), z: +val('tz') }; if (val('tdyaw') !== '') body.yaw = +val('tdyaw'); const pick = GO.pick === null ? null : TD[GO.pick]; if (pick && pick.map) body.map = pick.map; else if (val('tmap') !== '') body.map = val('tmap'); const r = await api('/api/player/teleport', body); goClear(); toast(r.message || ('Teleport set' + NOT_SAVED), r.warn ? 'part' : false, 15000); return r; }, null),
  augEdit: (d) => act(async () => { const r = await api('/api/items/augment-options', { item_id: +d.id }); r.options.forEach((o) => AUG_NAMES.set(o.id, o.name)); augItem = { ...r, item_id: +d.id }; render('scroll'); }, null, false),
  augCancel: () => { augItem = null; render(true); },
  augApply: (d) => act(async () => { const r = await api('/api/items/augment', { item_id: +d.id, augments: augPicked('au', augItem.limit), grade: +val('augG'), unlock_slots: $('#augU').checked }); augItem = null; toast(`Augments set (${r.augments})` + (r.slotsUnlocked ? `, ${r.slotsUnlocked} slot keystones unlocked` : '') + NOT_SAVED); }, null),
  queueAdd: () => { let t; try { t = needItem('gt'); } catch (e) { return toast(e.message, 'err'); } const e = { template_id: t, quantity: Math.max(1, +val('gq') || 1), quality: +val('gg') }; const ids = augPicked('ga', 3); if (ids.length) { e.augments = ids; e.grade = +val('gag'); } giveQueue.push(e); keepQueue(); render('scroll'); },
  // Care package (#417). These change the editor's settings and never the save, so they are immediate: nothing is queued.
  kitNew: () => { kitDraft = { was: '', name: '', items: [], xp: 0, skillPoints: 0 }; render('scroll'); },
  kitEdit: (d) => {
    const k = KITS && KITS.kits.find((x) => x.name === d.name);
    if (!k) { toast('That kit is no longer in the list.', true); return render(true); }
    kitDraft = { was: k.name, name: k.name, items: k.items.map((l) => ({ ...l, augments: (l.augments || []).slice() })), xp: k.xp || 0, skillPoints: k.skillPoints || 0 };
    render('scroll');
  },
  kitCancel: () => { kitDraft = null; render(true); },
  kitAddLine: () => {
    kitSync();
    let item;
    try { item = needItem('kitItem'); } catch (e) { return toast(e.message, 'err'); }
    const line = { item, quantity: Math.max(1, Math.floor(+val('kitQty')) || 1), grade: +val('kitGrade') };
    const ids = augPicked('ka', 12);
    if (ids.length) { line.augments = ids; line.augmentGrade = +val('kag'); }
    kitDraft.items.push(line);
    render('scroll');
  },
  kitDelLine: (d) => { kitSync(); kitDraft.items.splice(+d.id, 1); render('scroll'); },
  kitSave: () => {
    kitSync();
    const d = kitDraft;
    return act(async () => { await api('/api/kits/save', { was: d.was, kit: { name: d.name, items: d.items, xp: d.xp, skillPoints: d.skillPoints } }); kitDraft = null; }, `Saved kit "${d.name.trim()}"`, true, true);
  },
  kitDup: (d) => act(async () => { const r = await api('/api/kits/duplicate', { name: d.name }); toast(`Made "${r.saved}"`); }, null, true, true),
  kitDel: async (d) => (await ask({ title: 'Delete kit?', body: `Delete the kit "${d.name}" from the editor's settings. Nothing in any save changes. Export it first to keep a copy.`, ok: 'Delete', danger: true })) && act(() => api('/api/kits/delete', { name: d.name }), `Deleted kit "${d.name}"`, true, true),
  kitExport: (d) => kitDownload(d.name),
  baseBackupExport: (d) => baseBackupDownload(d),
  baseConsoleExport: (d) => baseConsoleDownload(d),
  baseImportCancel: () => { bbImport = null; render(true); },
  // Import sends the text that was previewed with the hash the preview gave; the editor refuses it if the text is another file. It asks nothing: it only adds rows.
  baseImportGo: () => {
    const b = bbImport;
    if (!b || !b.pv || b.busy) return;
    b.busy = true;
    return act(async () => {
      try {
        const r = await api('/api/bases/backups/import', { text: b.text, fileHash: b.pv.fileHash });
        bbImport = null;
        const m = r.summary;
        toast(`Imported "${r.name}" as base backup ${r.ids.backup.first}: ${Number(m.pieces).toLocaleString()} pieces, ${Number(m.placeables).toLocaleString()} placeables, ${Number(m.storedItems).toLocaleString()} stored items. Place it with the Base Reconstruction Tool in the game.` + NOT_SAVED);
      } finally { b.busy = false; }
    }, null);
  },
  // Apply asks the editor what it would do (a dry run: nothing is written), shows that, and applies only after OK. The apply is one edit, written after a backup, or queued while the game runs.
  kitApply: async (d) => {
    let pv;
    try { pv = await api('/api/kits/preview', { name: d.name }); } catch (e) { toast(e.message, true); return; }
    if (!pv.changed) { toast(`Nothing would change: ${kitLines(pv).join('; ') || 'the kit is empty'}.`, 'part'); return; }
    const again = kitApplied.has(d.name) ? ' You applied this kit earlier in this session; applying it again gives all of it again.' : '';
    const where = saveBlocked ? 'The save cannot be written right now, so the edit is queued and written as soon as it can be.' : 'It is written at once, after a backup.';
    const ok = await ask({ title: `Apply kit "${d.name}" to the character?`, ok: 'Apply kit', gap: true, list: kitLines(pv),
      body: `This is what would be given${pv.complete ? '' : ', and what would not fit'}. It is one edit. ${where}${again}${pv.volumeNote ? ' ' + pv.volumeNote : ''}` });
    if (!ok) return;
    return act(async () => {
      const r = await api('/api/kits/apply', { name: d.name, kitHash: pv.kitHash });
      kitApplied.add(d.name);
      const lines = kitLines(r).join('; ');
      if (r.complete) toast(`Applied kit "${d.name}": ${lines}` + NOT_SAVED);
      else toast(`Applied kit "${d.name}", but not all of it fitted: ${lines}` + NOT_SAVED, 'part');
    }, null);
  },
  queueDel: (d) => { giveQueue.splice(+d.id, 1); keepQueue(); render(true); },
  queueClear: () => { giveQueue = []; keepQueue(); render(true); },
  queueGive: () => act(async () => { const r = await api('/api/player/give-items', { items: giveQueue }); const sc = giveQueue.filter((q) => isSchematicId(q.template_id)).length; giveQueue = []; keepQueue(); giveToast(r, 'units of the queued items', `Added ${r.count} items` + (sc ? ` (${sc} schematic${sc === 1 ? '' : 's'})` : '')); }, null),
  give: () => act(async () => {
    const ids = augPicked('ga', 3), tid = needItem('gt');
    const r = ids.length ? await api('/api/player/give-items', { items: [{ template_id: tid, quantity: +val('gq'), quality: +val('gg'), augments: ids, grade: +val('gag') }] })
      : await api('/api/player/give', { template_id: tid, quantity: +val('gq'), quality: +val('gg') });
    giveToast(r, itemKindName(tid), isSchematicId(tid) ? 'Schematic added' : 'Item added');
  }, null),
  faction: () => act(async () => { const r = await api('/api/player/faction', { faction_id: +val('facSel') }); toast(`Faction set to ${r.faction}` + NOT_SAVED); }, null),
  repair: () => act(async () => { const r = await api('/api/player/repair', {}); fillToast('repair', (r.repaired || 0) + (r.backpackRepaired || 0) > 0, gearRepairText(r) + NOT_SAVED); }, null),
  fillWater: () => act(async () => { const r = await api('/api/player/hydration', {}); const m = hydrationMessage(r); fillToast('fillWater', !m.warn && Math.round(r.before * 10) !== Math.round(r.after * 10), m.text, m.warn ? 'part' : 'note'); }, null),
  fillBlood: () => act(async () => {
    const r = await api('/api/player/fill-blood-sacks', {});
    const by = Object.entries(r.byTemplate || {}).sort().map(([k, n]) => `${n} x ${itemName(k)}`).join(', ');
    const un = r.unknown ? ` ${r.unknown} blood ${r.unknown == 1 ? 'sack' : 'sacks'} of an unknown kind ${r.unknown == 1 ? 'was' : 'were'} left alone${(r.skippedUnknown || []).length ? ` (${r.skippedUnknown.join(', ')})` : ''}.` : '';
    const other = r.holdingOther ? ` ${r.holdingOther} holding something other than blood ${r.holdingOther == 1 ? 'was' : 'were'} left alone.` : '';
    fillToast('fillBlood', r.filled > 0, (r.filled ? `Filled ${r.filled} blood ${r.filled == 1 ? 'sack' : 'sacks'} (${by}); ${r.alreadyFull} already full.` : `No blood sack needed filling (${r.alreadyFull} already full).`) + un + other + (r.filled ? BLOOD_UNCHECKED : '') + NOT_SAVED);
  }, null),
  fillLiquids: () => act(async () => {
    const box = $('#liqVeh');
    if (box) liqVeh = box.checked;
    const r = await api('/api/player/fill-all-liquids', { vehicles: liqVeh });
    const w = r.water || {}, b = r.bloodSacks || {}, v = r.vehicles || null;
    const parts = [`${nn(w.filled || 0, 'water container', 'water containers')}`, `${nn(b.filled || 0, 'blood sack', 'blood sacks')}`];
    if (v) parts.push(nn(v.filled || 0, 'vehicle', 'vehicles'));
    const left = (w.skippedUnknown || []).length + (b.unknown || 0) + (v ? v.unknown || 0 : 0);
    const any = (w.filled || 0) + (b.filled || 0) + (v ? v.filled || 0 : 0);
    fillToast('fillLiquids', any > 0, (any ? `Filled ${parts.join(', ')}.` : 'Nothing needed filling.') + (r.vehiclesIncluded ? '' : ' Vehicles were not refuelled.') + (left ? ' Some containers, sacks or vehicles of an unknown kind were left alone.' : '') + (b.filled ? BLOOD_UNCHECKED : '') + (any ? NOT_SAVED : ''));
  }, null),
  refill: () => act(async () => { const r = await api('/api/player/refill', {}); const sk = (r.skippedUnknown || []).length ? ` Left alone, capacity unknown: ${r.skippedUnknown.join(', ')}.` : ''; fillToast('refill', r.filled > 0, `Filled ${r.filled} Literjons (${r.alreadyFull} already full).${sk}` + NOT_SAVED); }, null),
  delItem: async (d) => {
    if (!(await ask({ title: 'Delete item?', body: 'Delete ' + d.name + ' and everything attached to it (its stats and links). It is written at once, after a backup. If the file cannot be written (for example the game is running) it is queued and written when it can be.', ok: 'Delete', danger: true }))) return false;
    // In a base container that stays open (#731) the row the focus was on is gone after the redraw: the focus goes to the Delete button of the row that took its place (the next one, or the last), else to the box that adds an item.
    const btns = () => [...document.querySelectorAll('#inv [data-act="delItem"]')];
    const was = btns().findIndex((b) => b.dataset.id == d.id), hadFocus = was >= 0 && btns()[was] === document.activeElement;
    const r = await act(() => api('/api/items/delete', { id: +d.id }), 'Deleted');
    if (hadFocus && (!document.activeElement || document.activeElement === document.body)) {
      const left = btns(), n = left[Math.min(was, left.length - 1)] || $('#inv #bt');
      if (n) { n.focus({ preventScroll: true }); intoView(n); }
    }
    return r;
  },
  clearInv: async (d) => {
    const cur = !!($('#clrCur') && $('#clrCur').checked);
    let pv; // what the clear would remove right now, as the editor reads it (the list on the page may be older): the numbers to confirm and the token that binds the items
    try { pv = await api('/api/inventories/clear-preview?inventory_id=' + encodeURIComponent(d.inv) + '&include_currency=' + cur); } catch (e) { toast(e.message, true); return; }
    const n = +pv.items, nested = +pv.nested, total = n + nested;
    if (!n) { toast(+pv.kept ? 'Nothing to clear: this container holds only currency. Tick the box to remove it too.' : 'Nothing to clear: this container is empty.'); return; }
    const inside = nested ? `, and the ${nn(nested, 'item', 'items')} inside them (in bags)` : '';
    const body = `This removes ${nn(n, 'item', 'items')} from ${d.name}${inside}, with everything else attached to them (stats, links)${nested ? `: ${nn(total, 'item', 'items')} in all` : ''}. ${cur ? 'The currency is removed too.' : 'The Solari Coin stack is kept.'} It is one edit, written at once after a backup. If the file cannot be written (for example the game is running) it is queued and written when it can be. If the container holds other items than these by then, the editor refuses and changes nothing.`;
    if (!(await ask({ title: 'Clear ' + d.name + '?', body, ok: 'Clear container', typed: String(total), danger: true }))) return;
    act(async () => { const r = await api('/api/inventories/clear', { inventory_id: +d.inv, count: n, nested, ids_hash: pv.token, include_currency: cur }); toast(r.removed ? `Cleared ${d.name}: ${nn(r.removed, 'item', 'items')} removed${r.nested ? ` (and ${nn(r.nested, 'item', 'items')} inside bags)` : ''}` + NOT_SAVED : r.message); }, null);
  },
  spKeyAll: async () => (await ask({ title: 'Grant all keystones?', body: 'Every keystone of every specialization is bought. One edit.', ok: 'Grant all', danger: true })) && act(async () => { const r = await api('/api/player/specs/keystones/grant', {}); toast(r.granted ? `${r.granted} keystones granted` + NOT_SAVED : 'All keystones were already granted.'); }, null),
  spKeyReset: async () => (await ask({ title: 'Reset all keystones?', body: 'Every specialization keystone you have bought is removed. One edit.', ok: 'Reset all', danger: true })) && act(async () => { const r = await api('/api/player/specs/keystones/reset', {}); toast(r.removed ? `${r.removed} keystones reset` + NOT_SAVED : 'There were no keystones to reset.'); }, null),
  spAddCC: (d) => { const bad = window.LCCS ? window.LCCS.amountProblem(val('spx' + d.t), SPV.vm ? SPV.vm.maxXp : 0) : ''; if (bad) { toast(bad, true); return; } return specXpSend(d); }, // the Command Center's Add XP: XP is added only (#1032), so a number the editor would refuse is not sent; the reason is shown instead
  spMax: async (d) => (await ask({ title: 'Grant max ' + d.t + '?', body: d.t + ' goes to the XP of its last level (level 100). One edit.', ok: 'Grant max', danger: true })) && act(async () => { await api('/api/player/specs/max', { track: d.t }); toast(d.t + ' set to max' + NOT_SAVED); }, null),
  spReset: async (d) => (await ask({ title: 'Reset ' + d.t + '?', body: d.t + " goes back to 0 XP. Its keystones are not changed. One edit.", ok: 'Reset', danger: true })) && act(async () => { const r = await api('/api/player/specs/reset', { track: d.t }); toast(r.removed ? d.t + ' reset' + NOT_SAVED : d.t + ' had no XP to reset; nothing changed.'); }, null),
  spAssign: (d) => act(async () => { const t = val('spa' + d.n); await api('/api/player/specs/assign', { track: t, number: +d.n }); toast(`Track number ${d.n} is ${t}`); }, null, false),
  spAssignHand: () => act(async () => { const t = val('spmt'), n = val('spmn'); if (n === '') throw new Error('Type the track number first.'); await api('/api/player/specs/assign', { track: t, number: +n }); toast(`${t} is track number ${n}`); }, null, false),
  tut: (d) => act(async () => { await api('/api/player/tutorials', { id: +d.id, complete: d.c == '1' }); JB = await api('/api/player/journey/browse'); journeyList(); toast('Tutorial updated' + NOT_SAVED); }, null),
  tagAdd: () => act(() => api('/api/player/tags', { tag: val('tag').trim(), add: true }), 'Tag added'),
  tagDel: (d) => act(() => api('/api/player/tags', { tag: d.tag, add: false }), 'Tag removed'),
  invfilter: () => { invQ = val('invq'); return render('rows'); },
  invclear: () => { invQ = ''; render('rows'); },
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
      body: `The character's XP goes to 0 and the level with it. The skill points the levels gave are taken back, up to the level (${lvl}), but only from the points you have not spent${sp ? ` (unspent now ${fmt(sp.unspent)}, total ${fmt(sp.total)})` : ''}: a point spent in a skill stays, so the total never drops below what your skills hold. Skills you bought are not touched: reset them on the Skills tab first, with the box ticked so their points come back, to take back all of them. One edit.`,
      path: '/api/player/level/reset', done: (r) => `Level ${r.levelBefore} and ${fmt(r.changed)} XP reset to 0` + (r.pointsKept ? `; ${nn(r.pointsKept, 'skill point', 'skill points')} could not be taken back because they are spent in skills` : ''), none: 'The character has no XP, so there was nothing to reset.' });
  },
  jReset: (d) => {
    const n = +d.n || 0, nodes = +d.nodes || 0, tuts = +d.tuts || 0;
    if (!n) { toast('No Story or Codex step is finished and no tutorial is started, so there is nothing to reset.'); return; }
    return resetArea({ count: n, title: `Reset the journey (${nn(nodes, 'step', 'steps')} and ${nn(tuts, 'tutorial', 'tutorials')})?`, ok: 'Reset journey', typed: `reset ${n} journey ${n === 1 ? 'item' : 'items'}`,
      body: `Finished Story and Codex steps (${nodes}) go back to not finished, the same as the Reset button of each row: a step and its children are marked not finished and revealed. Started or finished tutorials (${tuts}) go back to not started. The 95 steps and 4 tutorials a new character already has finished (read from a save made right after a character was created) are left alone. Rewards that are waiting stay as they are (whether the game gives a reward again when a step is finished again is not known), and tags and contracts are not touched. One edit.`,
      path: '/api/player/journey/reset', done: (r) => `Journey reset: ${nn(r.nodes, 'step', 'steps')} and ${nn(r.tutorials, 'tutorial', 'tutorials')}`, none: 'No Story or Codex step is finished and no tutorial is started, so there was nothing to reset.' });
  },
  spResetAll: (d) => {
    const n = +d.n || 0, left = +d.left || 0;
    if (!n) { toast('No specialization whose number is known has XP, so there is nothing to reset.'); return; }
    return resetArea({ count: n, title: `Reset the XP of ${nn(n, 'specialization', 'specializations')}?`, ok: 'Reset XP', typed: `reset ${n} ${n === 1 ? 'specialization' : 'specializations'}`,
      body: `The XP and level of every specialization the editor can match go back to 0, the same as the Reset button of each specialization. Keystones are not touched.${left ? ` ${nn(left, 'row', 'rows')} of the save that the editor cannot match to a specialization yet ${left === 1 ? 'is' : 'are'} left alone, since removing ${left === 1 ? 'it' : 'them'} could clear the wrong one.` : ''} One edit.`,
      path: '/api/player/specs/reset-all', done: (r) => `${nn(r.changed, 'specialization', 'specializations')} reset to 0 XP`, none: 'No specialization whose number is known has XP, so there was nothing to reset.' });
  },
  rsReset: () => {
    const n = RSRESET.entries, rc = RSRESET.recipes, bs = RSRESET.sets;
    if (!n) { toast('No research beyond the four entries a new character starts with is bought, so there is nothing to reset.'); return; }
    return resetArea({ count: n, title: `Reset ${nn(n, 'research entry', 'research entries')}?`, ok: 'Reset research', typed: `reset ${n} research ${n === 1 ? 'entry' : 'entries'}`,
      body: `Every bought research entry except the four a new character starts with goes back to not bought. The ${nn(rc, 'recipe', 'recipes')} those entries gave are taken out of your known recipes (unique schematics you found are among them, and you would have to find them again or unlock them in the tree) and the ${nn(bs, 'building set', 'building sets')} they gave out of your learned sets. Recipes you know from a schematic whose entry is not bought, recipes and sets the research tree does not list, your Intel, your items, the buildings you placed and your skills are not touched, and no Intel is given back. One edit.`,
      path: '/api/player/research/reset', done: (r) => `Research reset: ${nn(r.changed, 'entry', 'entries')} back to not bought, ${nn(r.recipes, 'recipe', 'recipes')} and ${nn(r.sets, 'building set', 'building sets')} taken out`, none: 'No research beyond the four entries a new character starts with is bought, so there was nothing to reset.' });
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
  jfilter: () => { journeyQ = val('jq'); journeyList(); },
  jclear: () => { journeyQ = ''; journeyList(); },
  jset: (d) => act(async () => { await api('/api/player/journey', { node_id: d.id, complete: d.c == '1' }); JB = await api('/api/player/journey/browse'); journeyList(); toast('Journey updated' + NOT_SAVED); }, null),
  launchSkip: () => act(async () => { const r = await api('/api/autopass/skip', {}); await launchPoll(); toast(r.skipped ? 'Automatic refill and repair skipped for this run' : 'The automatic pass had already started'); }, null, false, true),
  launchOff: () => act(async () => { await api('/api/settings', { autoRefillOnOpen: false }); await launchPoll(); toast('Automatic refill when the editor opens is now off'); }, null, true, true),
  autoNow: () => act(async () => { launchHide(); const r = await api('/api/autopass/run', {}); await launchPoll(); if (r.note) { const q = !r.warn && String(r.note).startsWith(PASS_QUEUED); toast(r.note, !!r.warn, q ? 6000 : 15000, null, q ? { queued: true } : null); } }, null, true, true),
  autoref: async (d) => {
    if (!d.on && !(await ask({ title: 'Turn on automatic refill and repair?', body: 'From now on, opening the editor runs the jobs that are ticked on the card (refills water storage and power buildings, repairs your vehicles and your worn equipment and loadout, fills your water containers and your water, and so on) as one edit and saves to your game file immediately, like every other change. The previous file is backed up each time, and nothing happens while the game is open, not even at its menu (it runs when the game has closed). It runs on the game\'s own save only, never on a copy.', ok: 'Turn on' }))) return;
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
  refillWater: () => act(async () => { const r = await api('/api/bases/refill-water', {}); const sk = (r.skippedUnknown || []).length ? ` Left alone, capacity unknown: ${r.skippedUnknown.join(', ')}.` : ''; fillToast('refillWater', r.filled > 0, (r.filled ? `Refilled ${r.summary} (${r.alreadyFull} already full).` : `All ${r.alreadyFull} water buildings were already full.`) + sk + NOT_SAVED); }, null),
  refillGen: () => act(async () => { const r = await api('/api/bases/refill-generators', {}); fillToast('refillGen', r.filled > 0, (r.filled ? `Refilled ${r.summary} (${r.alreadyFull} already full).` : `All ${r.alreadyFull} power buildings were already full.`) + ((r.skipped || []).length ? ` No free slot: ${r.skipped.join(', ')}.` : '') + NOT_SAVED); }, null),
  refuelV: () => act(async () => {
    const r = await api('/api/vehicles/refuel', {});
    const by = Object.entries(r.byClass || {}).sort().map(([k, n]) => `${n} x ${vehKind(k)}`).join(', ');
    const un = r.unknown ? ` ${r.unknown} ${r.unknown == 1 ? 'vehicle' : 'vehicles'} of an unknown kind ${r.unknown == 1 ? 'was' : 'were'} left alone${(r.skippedUnknown || []).length ? ` (${r.skippedUnknown.map((x) => vehKind(x.replace(/ x\d+$/, '')) + x.replace(/^.* (x\d+)$/, ' $1')).join(', ')})` : ''}.` : '';
    const none = r.noFuelRecord ? ` ${r.noFuelRecord} ${r.noFuelRecord == 1 ? 'vehicle has' : 'vehicles have'} no fuel record and ${r.noFuelRecord == 1 ? 'was' : 'were'} left alone.` : '';
    fillToast('refuelV', r.filled > 0, (r.filled ? `Refuelled ${r.filled} ${r.filled == 1 ? 'vehicle' : 'vehicles'} (${by}); ${r.alreadyFull} already full.` : r.total ? `No vehicle needed fuel (${r.alreadyFull} already full).` : 'You have no vehicles in this save.') + un + none + (r.filled ? NOT_SAVED : ''));
  }, null),
  repairOne: (d) => act(async () => { const r = await api('/api/vehicles/repair', { vehicle_id: +d.id }); fillToast('repairOne', r.repaired > 0, repairMsg(r, 'modules of this vehicle') + (r.repaired ? NOT_SAVED : '')); }, null),
  repairV: () => act(async () => { const pct = $('#rvPct'); const n = pct ? +pct.value : 100; const r = await api('/api/vehicles/repair', pct && n < 100 ? { threshold: n } : {}); fillToast('repairV', r.repaired > 0, repairMsg(r, 'vehicle modules', n) + (r.repaired ? NOT_SAVED : '')); }, null),
  repairB: (d) => act(async () => { const r = await api('/api/bases/repair', { base_id: +d.id }); fillToast('repairB', r.pieces + r.placeables > 0, `Repaired ${r.pieces} pieces, ${r.placeables} placeables at this base` + NOT_SAVED); }, null),
  repairAll: () => act(async () => { const r = await api('/api/bases/repair-all', {}); fillToast('repairAll', r.pieces + r.placeables > 0, `Repaired ${r.pieces} pieces, ${r.placeables} placeables in the whole save` + NOT_SAVED); }, null),
  sand: (d) => act(async () => { const r = await api('/api/bases/clear-sand', { base_id: +d.id }); toast(`Cleared ${r.pieces} pieces at this base` + NOT_SAVED); }, null),
  sandAll: () => act(async () => { const r = await api('/api/bases/clear-sand-all', {}); toast(`Cleared ${r.pieces} pieces in the whole save` + NOT_SAVED); }, null),
  openInv: async (d) => {
    const card = await baseInvCard(d);
    bsInvOpen = { id: d.id, name: d.name || '' };
    setHTML($('#inv'), card);
  },
  giveCargo: (d) => act(async () => { const tid = needItem('bt'); const r = await api('/api/bases/give', { inventory_id: +d.id, template_id: tid, quantity: +val('bq') }); giveToast(r, itemName(tid), 'Item added'); }, null),
  giveInv: (d) => act(async () => { const tid = needItem('bt'); const r = await api('/api/bases/give', { inventory_id: +d.id, template_id: tid, quantity: +val('bq') }); giveToast(r, itemName(tid), 'Item added'); await A.openInv(d); }, null, false),
  dupV: async (d) => (await ask({ title: `Duplicate this ${d.name}?`, body: `Makes a copy of this ${d.name} next to you (an ornithopter: 30 m ahead and 7 m up, in the air): the same kind, modules and condition, an empty cargo hold, and yours. Only your own permission is copied, not the permission of any other player on this vehicle. The serial number and the records of the modules are copied as they are. The copy is built from the rows the game keeps for a vehicle (its actor, permission record, entities, modules and inventories). The game loaded a copy of a medium ornithopter and it flew, and an earlier copy of a light ornithopter also worked (checked in the game once, 2026-10-27); other kinds of vehicle were not tried, so try it on a copy of your save first. ${undoNote()}`, ok: 'Duplicate', danger: true })) && act(async () => { const r = await api('/api/vehicles/duplicate', { id: +d.id }); toast(`Made vehicle ${r.id}: a copy of the ${d.name} ${whereWords(r)}, with ${nn(r.modules, 'module', 'modules')} and an empty cargo hold` + NOT_SAVED); }, null),
  // Create a vehicle (#401). The draft is kept here; Preview asks what Create would write and keeps nothing; Create writes at once (one edit, a backup first, or queued while the game runs) and asks nothing.
  vcOpen: async () => {
    if (!VC) VC = await api('/api/vehicles/catalog');
    const c = VC.classes.find((x) => x.key === 'Buggy') || VC.classes[0];
    vcDraft = { cls: c.key, tpl: vcTop(c), mods: {}, fuel: 100, items: [], requestId: vcRequestId() };
    vcDraft.mods = vcDefaults();
    vcDrop();
    render('scroll');
  },
  vcCancel: () => { vcDraft = null; vcDrop(); render('scroll'); },
  vcDismiss: () => { vcResult = null; render('scroll'); },
  vcAddItem: () => {
    vcSync();
    let item;
    try { item = needItem('vcItem'); } catch (e) { return toast(e.message, 'err'); }
    vcDraft.items.push({ item, quantity: Math.max(1, Math.floor(+val('vcQty')) || 1) });
    vcDrop();
    render('scroll');
  },
  vcDelItem: (d) => { vcSync(); vcDraft.items.splice(+d.id, 1); vcDrop(); render('scroll'); },
  vcPreview: async () => {
    vcSync();
    vcDrop();
    const seq = vcSeq, asked = vcBody();
    try { const answer = await api('/api/vehicles/spawn-preview', asked); if (seq !== vcSeq) return; vcPreview = answer; vcPreviewKey = JSON.stringify(asked); } catch (e) { if (seq !== vcSeq) return; vcPreview = null; toast(e.message, true); render('scroll'); return; }
    render('scroll');
  },
  vcCreate: () => {
    if (vcCreating) return;
    if (vcBusy) return toast('The preview is updating. Press Create when it has finished.', 'note');
    vcSync();
    // Create writes what the preview shows: when the form no longer makes the request the preview was asked with (a fuel or quantity edit, say), the preview goes and has to be asked again
    if (vcPreview && JSON.stringify(vcBody()) !== vcPreviewKey) {
      // the redraw takes the item and Qty boxes back to empty: what was typed in them stays (#738)
      const typed = vcTypedTake(), unwatch = vcTypedWatch(typed);
      vcDrop();
      vcRedraw(typed, unwatch);
      return toast('The form changed after the preview. Press Preview, then Create.', 'note');
    }
    const body = vcBody();
    body.requestId = vcDraft.requestId;
    // The button is off and a second press does nothing until the answer is in: one press, one vehicle (#740)
    vcCreating = true;
    const create = document.querySelector('[data-act="vcCreate"]');
    if (create) create.disabled = true;
    return act(async () => {
      const r = await api('/api/vehicles/spawn', body);
      vcResult = { r, note: String(NOT_SAVED) ? ' ' + String(NOT_SAVED).trim() : '' };
      vcDraft = null;
      vcDrop();
      toast(`Created vehicle: ${r.label} ${whereWords(r)}` + NOT_SAVED);
    }, null).finally(() => {
      vcCreating = false;
      const again = document.querySelector('[data-act="vcCreate"]');
      if (again) again.disabled = vcBusy;
    });
  },
  bring: (d) => act(async () => { const r = await api('/api/vehicles/bring', { id: +d.id }); toast(`Vehicle moved ${whereWords(r)}` + NOT_SAVED); }, null),
  vreset: async (d) => (await ask({ title: d.v ? 'Reset purchase limits for this vendor?' : 'Reset purchase limits for every vendor?', body: d.v ? 'Resets the purchase counters of ' + (d.vn || 'this vendor') + '.' : 'Resets the purchase counters of every vendor in the save, not one.', ok: 'Reset' })) && act(() => api('/api/vendors/reset', { vendor_id: d.v }), 'Purchase limits reset'),
  cfgcreate: (d) => act(() => api('/api/config/create', { name: d.n }), 'Created ' + d.n + ' now', true, true),
  cfgadd: () => act(() => api('/api/config/set', { name: cfgFile, section: val('cs').replace(/^\[|\]$/g, ''), key: val('ck').trim(), value: val('cv') }), 'Added to the config file now', true, true),
  cfgReset: (d) => act(async () => { await api('/api/config/reset', { name: cfgFile, line: +d.line, key: d.key }); toast(`${d.key}: ${brief(d.was)} → ${brief(d.to)} (the game's default). To undo, set it back to ${brief(d.was)}. The file was written now, after a backup.`); }, null, true, true),
  cfgResetAll: async (d) => {
    const n = +d.n;
    if (!(await ask({ title: `Reset ${nn(n, 'setting', 'settings')} to the game's default?`, body: 'In ' + cfgFile + ', ' + nn(n, 'value is', 'values are') + ' put back to the default. Values already at the default, entries without a known default and keys you added are not touched. It is one edit: the file is written now, after one backup. Close the game first.', ok: 'Reset ' + n, typed: `reset ${n} ${n === 1 ? 'setting' : 'settings'} in ${cfgFile}`, danger: true, gap: true }))) return;
    act(async () => { const r = (await api('/api/config/reset', { name: cfgFile, expect: n })).result; toast(r.changed ? `Reset ${nn(r.changed, 'setting', 'settings')} in ${cfgFile} to the game's default. The file was written now, after a backup.` : 'Nothing to reset: every setting with a known default is already at it.'); }, null, true, true);
  },
  cfgdel: async (d) => (await ask({ title: 'Delete config line?', body: 'Delete ' + d.key + ' from ' + cfgFile + '. A backup of the file is made first.', ok: 'Delete', danger: true })) && act(() => api('/api/config/delete', { name: cfgFile, line: +d.line, key: d.key }), 'Deleted from the config file now', true, true),
  cfgraw: async () => (await ask({ title: 'Replace the whole file?', body: 'Everything in ' + cfgFile + ' is replaced by the text in the box. It is written now, after a backup of the file is made. Close the game first.', ok: 'Replace file', danger: true })) && act(() => api('/api/config/raw', { name: cfgFile, text: val('raw') }), 'File written now (the previous text is in Backups)', true, true),
  cfgrestore: async (d) => (await ask({ title: 'Restore config backup?', body: 'Replace ' + cfgFile + ' with ' + d.b + '. Close the game first.', ok: 'Restore', danger: true })) && act(() => api('/api/config/restore', { name: cfgFile, backup: d.b }), 'Config file restored now', true, true),
  dbgo: async () => { dbQ = val('dbq'); dbOff = 0; await render('scroll'); dbRowsTop(); },
  dbprev: async () => { dbOff = Math.max(0, dbOff - 100); await render('scroll'); dbRowsTop(); },
  dbnext: async () => { dbOff += 100; await render('scroll'); dbRowsTop(); },
  runsql: async () => {
    window.lastSql = val('sql');
    try {
      const r = await api('/api/db/sql', { sql: window.lastSql });
      setHTML($('#sqlout'), html`<p class="mut">${r.rows.length.toLocaleString()} ${r.rows.length === 1 ? 'row' : 'rows'}${r.truncated ? '. Only the first 1,000 rows are shown: add LIMIT or a WHERE to see others.' : ''}</p><div class="scroll"><table><thead><tr>${r.columns.map((c) => html`<th>${c}</th>`)}</tr></thead><tbody>${r.rows.map((x) => html`<tr>${x.map((v) => html`<td>${v}</td>`)}</tr>`)}</tbody></table></div>`);
    } catch (e) { toast(e.message, true); sqlFailed(e); }
  },
  execsql: async () => {
    window.lastSql = val('sql');
    if (!(await ask({ title: 'Run this SQL as a write?', body: `This statement is written to ${notLive ? 'this file' : 'the game file'} at once, after the usual backup. If the file cannot be written (for example the game is running) it waits in the queue and is written when it can be:`, pre: window.lastSql, ok: 'Run', typed: 'write', danger: true }))) return;
    try {
      const r = await api('/api/db/exec', { sql: window.lastSql });
      toast(r.changes + (r.changes === 1 ? ' row changed' : ' rows changed'));
      let queued = r.outcome === 'queued';
      if (!r.outcome) try { queued = (await status()).dirty; } catch (e) { /* the poll reports a lost connection; the statement itself ran */ }
      setHTML($('#sqlout'), html`<p class="ok">${r.changes} ${r.changes === 1 ? 'row' : 'rows'} changed, ${queued ? 'and queued: it is written to ' + (notLive ? 'this file' : 'game.db') + ' as soon as the file can be written.' : 'and written to ' + (notLive ? 'this file' : 'game.db') + '.'}</p>`);
    } catch (e) { toast(e.message, true); sqlFailed(e); }
  },
  restore: async (d) => (await ask({ title: 'Restore this backup over the save?', body: d.copy ? 'Replace this file with ' + d.n + '. This backup was made from a copy, not from the game\'s save. The current file is backed up first.' : 'Replace game.db with ' + d.n + '. The current file is backed up first. Close the game first.', ok: 'Restore', typed: 'restore', danger: true })) && act(() => api('/api/save/restore', { name: d.n }), 'Restored'),
};
document.addEventListener('click', (e) => {
  const cf0 = e.target.closest('[data-confirm]');
  if (armed && !(cf0 && cf0 === armed.btn)) disarm(); // anything but the second press of the same button cancels
  if (cf0 && !cf0.dataset.armed) { e.preventDefault(); arm(cf0); return; }
  if (cf0 && armed && cf0 === armed.btn && Date.now() - armed.at < ARM_GAP_MS) { e.preventDefault(); e.stopPropagation(); return; } // two presses too close to be two decisions (the second half of a double click): still asking. Only the time counts: a press after the gap is a decision, whatever the system calls it
  if (cf0) disarm(); // the second press: the label goes back and the action below runs
  const keep = e.target.closest('summary[data-keep]');
  if (keep && Object.hasOwn(KEEP, keep.dataset.keep)) KEEP[keep.dataset.keep] = !keep.parentElement.open; // the toggle is about to flip
  const hth = e.target.closest('thead th');
  if (hth && !e.target.closest('button, a, input, select') && headerClick(hth)) return;
  const hp = e.target.closest('[data-help]');
  if (hp && HELP && HELP.pages.some((p) => p.id === hp.dataset.help)) { browserStore.set('help.page', hp.dataset.help); if (tab === 'help') render(); return; }
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
  if (vo) { vcSync(); vhOpen = +vo.dataset.vhopen; render('rows', false, vo); return; }
  const bo = e.target.closest('[data-bsopen]');
  if (bo) { bsOpen = +bo.dataset.bsopen; bsInvOpen = null; render('rows', false, bo); return; }
  const bt = e.target.closest('[data-bstab]');
  if (bt) { bsTab = bt.dataset.bstab; bsInvOpen = null; render('rows', false, bt); return; }
  const bg = e.target.closest('[data-bsgrp]');
  if (bg) { bsGroup = bg.dataset.bsgrp; bsInvOpen = null; render('rows', false, bg); return; }
  const ig = e.target.closest('[data-invg]');
  if (ig) { invG = ig.dataset.invg; render('rows', false, ig); return; }
  const s = e.target.closest('[data-sub]');
  if (s) { const [g, k] = s.dataset.sub.split(':'); if (GROUPS.includes(g)) { sub[g] = k; render(); } return; }
  const a = D(e);
  if (a) { e.preventDefault(); lastPress = { act: a.dataset.act, id: a.dataset.id }; if (Object.hasOwn(A, a.dataset.act)) Promise.resolve().then(() => A[a.dataset.act](a.dataset)).catch((err) => toast(err.message, true)); }
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
  goKeep(t);
  if (t.id == 'gt') { showGiveAugments(); return; }
  if (t.id == 'kitItem') { showGiveAugments('kitItem', 'kitAug', 'ka', 'kag'); return; }
  if (t.id == 'kitFile') { kitImport(t); return; }
  if (t.id == 'baseFile') { baseImportPick(t); return; }
  if (t.id == 'vcClass' || t.id == 'vcTpl' || t.dataset.vcslot !== undefined) { vcChanged(t); return; }
  if (t.dataset.vcprev !== undefined) { vcPartPicked(t); return; }
  if (t.id == 'skCharge') { skCharge = t.checked; return; }
  if (t.dataset.autojob) { // one job of the automatic pass switched on or off; the card is drawn again from what the server kept
    const key = t.dataset.autojob, on = t.checked, label = ((t.closest('label') || {}).textContent || key).trim();
    act(async () => { try { await api('/api/settings', { [key]: on }); } catch (e) { render('scroll'); throw e; } toast(label + ' is now ' + (on ? 'on' : 'off') + ' for the automatic pass'); }, null);
    return;
  }
  if (t.id == 'rsGrp') { rsGroup = t.value; researchList(); return; }
  if (t.id == 'tmap') { tmapSync(); goPlaces(true); return; }
  if (t.id == 'tdsel') { const d = t.value === '' ? null : TD[+t.value]; GO.pick = d ? +t.value : null; if (d) { goSet('tx', Math.round(d.x)); goSet('ty', Math.round(d.y)); goSet('tz', Math.round(d.z)); } goNote(); return; }
  if (t.id == 'lcLvl' && LC) { LC.level = +t.value; claimEditorDraw(); return; }
  if (t.id == 'ctgrp') { ctGroup = t.value; catalogList(); return; }
  if (t.id == 'ctexp') { ctExp = t.checked; if (!ctExp && CT.rows.some((r) => r.experimental && r.group == ctGroup)) ctGroup = ''; catalogList(); return; }
  if (t.dataset.skill) { const said = changedMsg(t, (t.dataset.name || 'Skill') + ': rank'); act(async () => { const r = await api('/api/player/skills/module', { module: t.dataset.skill, level: +t.value, charge: skCharge }); toast(`${said} Skill points ${r.pointsBefore} → ${r.pointsAfter}` + NOT_SAVED); }, null); return; }
  if (t.dataset.item) editInput(t, () => api('/api/items/update', { id: +t.dataset.item, [t.dataset.field]: +t.value }), changedMsg(t, (t.dataset.name || 'Item') + ': ' + (t.dataset.field == 'quality' ? 'grade' : 'stack')));
  else if (t.dataset.faction) editInput(t, () => api('/api/player/factions', { faction_id: +t.dataset.faction, amount: +t.value }), changedMsg(t, (t.dataset.name || 'Faction') + ': reputation'));
  else if (t.classList.contains('cfgv')) editInput(t, () => api('/api/config/set', { name: cfgFile, section: t.dataset.section, key: t.dataset.key, value: t.value, line: +t.dataset.line }), changedMsg(t, t.dataset.key + ' written to the config file now:'), true);
  else if (t.classList.contains('dbc')) editInput(t, async () => {
    try { return await api('/api/db/update', { table: dbTable, ...(t.dataset.kind == '_key_' ? { key: t.dataset.rowid } : { rowid: +t.dataset.rowid }), column: t.dataset.col, value: t.value === '' && t.hasAttribute('data-null') ? null : t.value, expected: t.dataset.orig }); }
    catch (e) { if (e.code === 'conflict') setTimeout(() => render('scroll'), 0); throw e; } // the table is out of date: show it as it is now
  }, changedMsg(t, 'Database ' + dbTable + '.' + t.dataset.col + ':'));
  else if (t.id == 'cfgsel') { cfgFile = t.value; render(); }
  else if (t.id == 'dbsel') { dbTable = t.value; dbOff = 0; dbQ = ''; render(); }
});
// keeps the sticky warning banner just below the sticky header, whatever height the header wraps to
const hdrEl = $('header');
const setHdrH = () => document.documentElement.style.setProperty('--hdr-h', hdrEl.offsetHeight + 'px');
if (hdrEl) { setHdrH(); if (window.ResizeObserver) new ResizeObserver(setHdrH).observe(hdrEl); else window.addEventListener('resize', setHdrH); }
// The room that the bars under the header take (--warn-h, #910): the warning bar can grow to 45 vh when many writes are queued, so a fixed 76 px let a jump or a focus land under it. The height is
// measured, as the header's is; 76 px stays as the value until a bar has been measured, and when no bar is shown the CSS rules that use it do not apply.
const warnBars = ['#gamewarn', '#launchpass'].map((s) => $(s)).filter(Boolean);
const setWarnH = () => { const h = warnBars.reduce((n, b) => n + (b.classList.contains('warn') || b.classList.contains('info') ? b.offsetHeight : 0), 0); document.documentElement.style.setProperty('--warn-h', (h || 76) + 'px'); };
if (warnBars.length) { setWarnH(); if (window.ResizeObserver) { const ro = new ResizeObserver(setWarnH); warnBars.forEach((b) => ro.observe(b)); } else window.addEventListener('resize', setWarnH); }
const autoBox = $('#autorefresh');
autoBox.checked = autoRefreshOn();
autoBox.addEventListener('change', () => { try { localStorage.autoRefresh = autoBox.checked ? '1' : '0'; } catch (e) { /* the choice lasts until the page is closed */ } status(); });
// The countdown before the automatic pass at launch (#460). The pass does not start until the page has shown this bar, and the bar has Skip and a way to turn the switch off. The seconds
// are counted down here and checked with the editor every few seconds, and again when they run out; the bar goes when the editor says no pass is counting or working.
let launchMode = '', launchLeft = 0, launchTicker = null, launchTicks = 0, launchBusy = false;
function launchHide() {
  if (launchTicker) { clearInterval(launchTicker); launchTicker = null; }
  launchMode = '';
  const b = $('#launchpass');
  if (b) { b.className = 'hide'; setHTML(b, html``); }
}
async function launchPoll() {
  if (launchBusy) return;
  launchBusy = true;
  try {
    let st;
    try { st = await api('/api/autopass/launch'); } catch (e) { launchHide(); return; }
    const b = $('#launchpass');
    const mode = st.pending ? 'count' : st.running ? 'run' : '';
    if (!mode) {
      const was = launchMode;
      launchHide();
      if (was) { // the countdown ended: the pass ran (or was switched off); say what it did
        try { const r = await api('/api/startup'); if (r.note) { startupToast(r); await status(); await render(true); } } catch (e) { /* the note is optional */ }
      }
      return;
    }
    launchLeft = st.secondsLeft;
    if (mode !== launchMode) {
      launchMode = mode;
      b.className = 'info';
      setHTML(b, mode === 'count'
        ? html`<b>Automatic refill and repair starts in <span id="launchsecs">${launchLeft}</span> s.</b> <button class="b sm d-btn primary sm" data-act="launchSkip">Skip</button> <button class="b sm sec d-btn sm" data-act="launchOff">Turn it off</button>`
        : html`<b>Automatic refill and repair is running...</b>`);
      const sr = $('#sr');
      if (sr) sr.textContent = mode === 'count' ? 'Automatic refill and repair starts in ' + launchLeft + ' seconds. Skip and Turn it off are at the top of the page.' : 'Automatic refill and repair is running.';
    } else if ($('#launchsecs')) $('#launchsecs').textContent = String(launchLeft);
    if (!launchTicker) {
      launchTicks = 0;
      launchTicker = setInterval(() => {
        launchTicks++;
        if (launchMode === 'count') { launchLeft = Math.max(0, launchLeft - 1); if ($('#launchsecs')) $('#launchsecs').textContent = String(launchLeft); }
        if (launchMode === 'run' || launchLeft <= 0 || launchTicks % 5 === 0) launchPoll();
      }, 1000);
    }
  } finally { launchBusy = false; }
}
// ---------- the version label and the About panel (#806, slice 1: the version stamp only; nothing here fetches anything from the network)
// The label is the version of this build, read from the editor's own /api/about. It opens a panel with the version, build number, commit, date and channel, and the release notes when a notes.md sits next to the editor.
let ABOUT = null;
async function loadAbout() {
  try { ABOUT = await api('/api/about'); } catch (e) { return; } // the label is optional: without it the header is as before
  const b = $('#verlabel');
  if (!b || !ABOUT || !ABOUT.label) return;
  b.textContent = String(ABOUT.label);
  b.setAttribute('aria-label', `${ABOUT.label}, about this version`);
  b.hidden = false;
}
function aboutLines(a) {
  const rows = [['Version', a.version], ['Build', a.build], ['Commit', a.commit], ['Date', a.date], ['Channel', a.channel], ['Platform', a.arch]];
  return rows.map(([k, v]) => `${k}: ${v || 'not recorded'}`);
}
function aboutBody(a) {
  const parts = [`This is ${a.line || a.label}.`];
  if (a.dev) parts.push('This is a development build, not a tagged release.');
  parts.push(a.notes ? `Release notes (${a.notesFile || 'notes.md'}, next to the editor):` : 'No release notes were found next to the editor.');
  return parts.join(' ');
}
async function openAbout() {
  let a;
  try { a = await api('/api/about?notes=1'); } catch (e) { a = ABOUT; }
  if (!a) { toast('The version could not be read.', true); return; }
  await ask({ title: 'About Tabr Tau', body: aboutBody(a), list: aboutLines(a), listLabel: 'This build', pre: a.notes ? a.notes + (a.notesTruncated ? '\n(cut: the file is longer than the editor shows)' : '') : '', preLabel: 'Release notes', close: true });
}
$('#verlabel').onclick = openAbout;
loadAbout();
status().then(render).then(async () => {
  try { const r = await api('/api/startup'); if (r.note) startupToast(r); } catch (e) { /* the note is optional */ }
  launchPoll();
}).catch((e) => { // the editor did not answer: "Loading the save..." would stay for ever
  if ($('#loading')) setHTML($('#main'), html`<div class="card d-panel bad">The editor did not answer: ${e.message}</div><div class="row mt12"><button class="b d-btn primary" data-act="retry">Try again</button></div>`);
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
const FILTER_ENTER = { invq: 'invfilter', jq: 'jfilter', rsq: 'rsfilter', crq: 'crfilter', ctq: 'ctfilter', dbq: 'dbgo' };
async function runFilter(id) {
  try { await A[FILTER_ENTER[id]]({}); } catch (err) { toast(err.message, true); }
  const box = $('#' + id);
  if (box) { box.focus(); try { box.setSelectionRange(box.value.length, box.value.length); } catch (err) { /* a box with no cursor */ } }
}
document.addEventListener('keydown', (e) => {
  if (armed && e.key === 'Escape') disarm();
  if (e.key === 'Enter' && !e.isComposing && !askState && e.target && Object.hasOwn(FILTER_ENTER, e.target.id)) { e.preventDefault(); runFilter(e.target.id); return; }
  if ((e.key === 'Enter' || e.key === ' ') && !askState && e.target && e.target.matches && e.target.matches('thead th[tabindex]')) { e.preventDefault(); headerClick(e.target); return; }
  // a cell of the land claim grid is a button: Enter and Space press it, as a click does, and the focus stays on it
  if ((e.key === 'Enter' || e.key === ' ') && !askState && LC && e.target && e.target.matches && e.target.matches('[data-cell]')) { e.preventDefault(); const k = e.target.getAttribute('data-cell'); lcCellClick(k); claimEditorDraw(k); return; }
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
