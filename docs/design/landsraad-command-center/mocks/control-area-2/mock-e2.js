// Control area mock, options E2 to E6 (tau #974, round 2). Behaviour for option-e2.html to option-e6.html only (E3 and E6: the lens row turns into the action bar when Houses are chosen; E4: the filters are a second row, behind one Filters (n) key on a phone;
// E5 and E6: one Filter menu with the categories House, Specialization and Group, and chips for what is on, a bottom sheet on a phone); mock.js (options C, D, E) is not used on this page because E2 models two separate things:
// what is VISIBLE (the Show tabs, the House chips and the drawer) and what is CHOSEN (the ticks, Choose shown, Clear). The counts on the plate are chosen AND visible, so a House that a filter
// hides is never counted and never acted on. Fixture data only, nothing is read or written.
// Query: ?state=idle|chosen|confirm, ?theme=dark|light, ?bare=1, ?drawer=open, ?layout=list, ?plate=full (keep the full plate when nothing is chosen), ?annotate=1|phone (callouts for the
// three parts, for the annotated pictures only), ?show=left|notstarted|progress|top, ?house=undecided|pledged (open with a filter on).
// E4: ?filters=open (the phone disclosure open). E5 and E6: ?menu=open, ?cat=house|spec|group, ?spec=4,5 (specializations 1 to 5), ?pick=Alexin,Hagal (E5: Houses to show), ?q=text (the search box).
// E6 only: ?tick=1,2,3 or ?tick=all (Houses ticked), ?find=open (the Find House list), ?group=region|spec (list layout, a Group chip). E6: the Filter menu's House list TICKS squares (it never hides one); Find House scrolls to one House and ticks nothing.
(function () {
  'use strict';
  var doc = document, root = doc.documentElement, page = doc.querySelector('.k-page');
  var V = page && page.getAttribute('data-variant'); if (['e2', 'e3', 'e4', 'e5', 'e6'].indexOf(V) < 0) return;
  var E3 = V === 'e3' || V === 'e6'; // the bar variants: the lens row turns into the action bar
  var E4 = V === 'e4', MENU = V === 'e5' || V === 'e6'; // E4: a filter row; E5 and E6: the categorized Filter menu
  var q = function (s, r) { return (r || doc).querySelector(s); };
  var qa = function (s, r) { return Array.prototype.slice.call((r || doc).querySelectorAll(s)); };
  var params = new URLSearchParams(location.search);
  if (params.get('theme') === 'light' || params.get('theme') === 'dark') root.setAttribute('data-theme', params.get('theme'));
  if (params.get('bare') === '1') root.classList.add('k-bare');
  qa('.k-dev a[data-q]').forEach(function (a) { var kv = a.getAttribute('data-q').split('='); if ((params.get(kv[0]) || (kv[0] === 'state' ? 'idle' : '')) === kv[1]) a.setAttribute('aria-current', 'true'); });

  // ---- fixture: 25 Houses; which ones each filter shows, and which ones each action would change (the same totals as the earlier options: Fill 21, Complete 14, Reopen 16 of 25) ----
  var IDS = []; for (var i = 1; i <= 25; i++) IDS.push(i);
  var set = function (a) { var o = {}; a.forEach(function (x) { o[x] = true; }); return o; };
  var TOP = [3, 8, 12, 17, 20, 21, 25], NOTSTARTED = [1, 2, 4, 5, 9, 13, 15, 19, 24];
  var SHOW = [null, IDS.filter(function (x) { return TOP.indexOf(x) < 0; }), NOTSTARTED, IDS.filter(function (x) { return TOP.indexOf(x) < 0 && NOTSTARTED.indexOf(x) < 0; }), TOP].map(function (a) { return a && set(a); });
  var HOUSE = [null, set(IDS.filter(function (x) { return x <= 11; })), set(IDS.filter(function (x) { return x > 11; }))];
  var CHG = { fill: set(IDS.filter(function (x) { return [6, 10, 14, 18].indexOf(x) < 0; })), complete: set([1, 2, 4, 5, 7, 9, 11, 13, 15, 16, 19, 22, 23, 24]), reopen: set([2, 3, 5, 6, 8, 9, 10, 12, 13, 14, 17, 18, 20, 21, 22, 25]) };
  var WORD = { fill: 'Fill', complete: 'Complete', reopen: 'Reopen' };
  var chosen = {}, pending = 'fill', confirmOpen = false, toastTimer = 0;
  var specPick = {}, housePick = {}, searchQ = '', menuOpen = false, flashTimer = 0, SPECN = ['', 'Crafting', 'Gathering', 'Exploration', 'Combat', 'Sabotage'], PLN = ['Any', 'Undecided', 'Pledged'], GBN = ['None', 'By region', 'By specialization'];
  var pop = q('[data-k="pop"]'), fbtn = q('[data-k="fbtn"]'), scrim = q('[data-k="scrim"]'), phoneMq = window.matchMedia('(max-width: 700px)');
  var plate = q('[data-k="plate"]'), stubs = qa('.k-stub li');
  stubs.forEach(function (li, k) { li.setAttribute('data-id', String(k + 1)); });

  function radioIndex(label) { var g = q('[role="radiogroup"][aria-label="' + label + '"]'); var all = qa('[role="radio"]', g); for (var k = 0; k < all.length; k++) if (all[k].getAttribute('aria-checked') === 'true') return k; return 0; }
  function visible() {
    var s = radioIndex('Show'), h = radioIndex('House'), sp = MENU ? 0 : q('select[aria-label="Specialization"]').selectedIndex, nsp = Object.keys(specPick).length, nhp = Object.keys(housePick).length, out = {};
    IDS.forEach(function (id) { var specOk = MENU ? (!nsp || specPick[(id % 5) + 1]) : (sp === 0 || id % 5 === sp - 1); if ((!SHOW[s] || SHOW[s][id]) && (!HOUSE[h] || HOUSE[h][id]) && specOk && (!nhp || housePick[id])) out[id] = true; });
    return out;
  }
  function faction() { var s = q('[data-k="seat"][aria-checked="true"]'); return s ? s.getAttribute('data-name') : 'Atreides'; }
  function plural(n, w) { return n + ' ' + w + (n === 1 ? '' : 's'); }
  function toast(msg) { var t = q('[data-k="toast"]'); if (!t) return; t.textContent = msg; t.hidden = false; clearTimeout(toastTimer); toastTimer = setTimeout(function () { t.hidden = true; }, 3200); }
  function totals(n) { // the Yes key's mouse-over: the totals the long paragraph used to carry
    var m = Math.round(n * 107 / 21), f = function (v) { return Math.round(v * n / 21 / 500) * 500; }, fx = function (v) { return Math.round(v * n / 21); };
    return 'Writes ' + m + ' missions: +' + f(642000).toLocaleString('en-US') + ' Solari, +' + f(107000).toLocaleString('en-US') + ' House Scrip, +' + fx(182) + ' Combat XP, +' + f(22500).toLocaleString('en-US') + ' Crafting XP. ' + fx(92) + ' rewards waiting at the House representative. Written at once, with a backup first.';
  }

  function update(keepConfirm) {
    var vis = visible(), vc = IDS.filter(function (id) { return vis[id]; });
    var cv = vc.filter(function (id) { return chosen[id]; });
    var counts = {}; ['fill', 'complete', 'reopen'].forEach(function (k) { counts[k] = cv.filter(function (id) { return CHG[k][id]; }).length; });
    if (confirmOpen && !(keepConfirm && cv.length && counts[pending])) confirmOpen = false;
    var on = cv.length > 0;
    page.setAttribute('data-state', confirmOpen ? 'confirm' : (on ? 'chosen' : 'idle'));
    plate.classList.toggle('k-lit', on);
    if (E3) { page.setAttribute('data-mode', on && !menuOpen ? 'action' : 'filters'); } else plate.classList.toggle('k-slim', !on && params.get('plate') !== 'full');
    q('[data-k="n"]').textContent = String(cv.length);
    q('[data-k="nsr"]').textContent = on ? plural(cv.length, 'House') + ' chosen' : 'No Houses chosen';
    q('[data-k="shown"]').textContent = String(vc.length);
    var shownSr = q('.k-count .k-sr'); if (shownSr) shownSr.textContent = vc.length + ' of 25 Houses shown';
    var shownTip = q('.k-count .k-tip'); if (shownTip) shownTip.textContent = vc.length + ' of 25 Houses shown';
    var allIn = vc.length > 0 && vc.every(function (id) { return chosen[id]; });
    var cs = q('[data-k="choose"]'); q('span', cs).textContent = 'Choose shown (' + vc.length + ')';
    if (!vc.length || allIn) cs.setAttribute('aria-disabled', 'true'); else cs.removeAttribute('aria-disabled');
    var clear = q('[data-k="clear"]'); if (on) clear.removeAttribute('aria-disabled'); else clear.setAttribute('aria-disabled', 'true');
    qa('[data-k="act"]').forEach(function (b) {
      var k = b.getAttribute('data-do'), c = q('[data-k="chg"]', b), tip = q('[data-k="atip"]', b), n = counts[k];
      if (n > 0) b.removeAttribute('aria-disabled'); else b.setAttribute('aria-disabled', 'true');
      if (tip) tip.textContent = !on ? 'Choose at least one House first' : n > 0 ? WORD[k] + ' changes ' + n + ' of the ' + cv.length + ' chosen Houses that are shown. A question shows the totals before anything is written.' : 'Nothing to change among the chosen Houses that are shown';
      c.textContent = n > 0 ? String(n) : ''; c.hidden = !(n > 0);
      b.classList.toggle('k-asking', confirmOpen && k === pending);
    });
    stubs.forEach(function (li, k) { var id = k + 1; li.classList.toggle('k-dim', !vis[id]); li.classList.toggle('k-on', !!(chosen[id] && vis[id])); });
    var c = q('[data-k="confirm"]'); c.hidden = !confirmOpen;
    if (confirmOpen) {
      var n2 = counts[pending];
      q('[data-k="qh"]', c).textContent = WORD[pending] + ' ' + plural(n2, 'House') + (pending === 'reopen' ? '' : ' for ' + faction()) + '?';
      q('[data-k="yes"]', c).setAttribute('aria-label', 'Yes, ' + WORD[pending].toLowerCase() + ' ' + plural(n2, 'House') + (pending === 'reopen' ? '' : ' for ' + faction()));
      q('[data-k="ytip"]', c).textContent = totals(n2);
    }
    if (MENU) renderMenu();
    else if (E4) { var n4 = radioIndex('House') > 0 ? 1 : 0; ['Specialization', 'Group'].forEach(function (l) { var s4 = q('select[aria-label="' + l + '"]'); if (s4 && s4.selectedIndex > 0 && !s4.closest('[hidden]')) n4++; }); q('[data-k="fcount"]').textContent = '(' + n4 + ')'; } // Jump to House is not a filter: it is not counted
    else { var badge = q('[data-k="fcount"]'), cnt = 0; qa('select', q('[data-k="drawer"]')).forEach(function (s) { if (s.selectedIndex > 0 && !s.closest('[hidden]')) cnt++; }); badge.textContent = String(cnt); badge.hidden = cnt === 0; }
    layoutNotes();
  }
  function layoutNotes() { if (root.classList.contains('k-annot')) annotate(); }

  // ---- E5 and E6: the Filter menu (a popover; a bottom sheet at 700 px and under) ----
  function isPhone() { return phoneMq.matches; }
  function listLayout() { var c = q('[data-k="cat"][data-cat="group"]'); return !!c && !c.hidden; }
  function renderMenu() {
    var chips = [], pl = radioIndex('House'), gb = listLayout() ? radioIndex('Group by') : 0, nsp0 = Object.keys(specPick).length;
    if (V === 'e6') { // one chip per category, never per item: Pledge, Specialization, Group. A House has no chip (picking one ticks its square).
      if (pl > 0) chips.push(['pledge', PLN[pl], 'Pledge: ' + PLN[pl]]);
      if (nsp0) { var sn = []; for (var k0 = 1; k0 <= 5; k0++) if (specPick[k0]) sn.push(SPECN[k0]); chips.push(['spec', nsp0 >= 3 ? 'Specialization: ' + nsp0 + ' chosen' : sn.join(' + '), 'Specialization: ' + sn.join(', ')]); }
      if (gb > 0) chips.push(['group', GBN[gb], 'Group: ' + GBN[gb]]);
    } else {
      if (pl > 0) chips.push(['pledge', 'House: ' + PLN[pl], 'House: ' + PLN[pl]]);
      IDS.forEach(function (id) { if (housePick[id]) { var t5 = 'House: ' + q('[data-k="hpick"][data-id="' + id + '"]').getAttribute('data-name'); chips.push(['house:' + id, t5, t5]); } });
      for (var k = 1; k <= 5; k++) if (specPick[k]) { var t6 = 'Specialization: ' + SPECN[k]; chips.push(['spec:' + k, t6, t6]); }
      if (gb > 0) chips.push(['group', 'Group: ' + GBN[gb], 'Group: ' + GBN[gb]]);
    }
    var box = q('[data-k="chips"]'); box.textContent = '';
    chips.forEach(function (c) { var b = doc.createElement('button'); b.type = 'button'; b.className = 'k-chip'; b.setAttribute('data-k', 'chip'); b.setAttribute('data-chip', c[0]); b.setAttribute('aria-label', 'Remove ' + c[2]); b.title = 'Remove ' + c[2];
      var s = doc.createElement('span'); s.textContent = c[1]; b.appendChild(s); var ns = 'http://www.w3.org/2000/svg', sv = doc.createElementNS(ns, 'svg'), u = doc.createElementNS(ns, 'use'); sv.setAttribute('class', 'k-ic'); sv.setAttribute('aria-hidden', 'true'); u.setAttribute('href', '#k-x'); sv.appendChild(u); b.appendChild(sv); box.appendChild(b); });
    q('[data-k="clearf"]').hidden = !chips.length;
    var nsp = Object.keys(specPick).length, nh = V === 'e6' ? 0 : Object.keys(housePick).length, vis6 = V === 'e6' ? visible() : null, nt = V === 'e6' ? IDS.filter(function (id) { return chosen[id] && vis6[id]; }).length : 0;
    var fc = q('[data-k="fcnt"]'); if (fc) { fc.textContent = String(chips.length); fc.hidden = !chips.length; }
    qa('[data-k="spick"]').forEach(function (b) { var i = +b.getAttribute('data-i'); b.setAttribute('aria-checked', i === 0 ? String(!nsp) : String(!!specPick[i])); });
    var shownN = 0, needle = searchQ.trim().toLowerCase();
    qa('[data-k="hpick"]').forEach(function (b) { var id = +b.getAttribute('data-id'), hit = !needle || b.getAttribute('data-name').toLowerCase().indexOf(needle) >= 0; b.hidden = !hit; if (hit) shownN++; if (V === 'e6') { b.setAttribute('aria-checked', String(!!chosen[id] && !!vis6[id])); if (vis6[id]) b.removeAttribute('aria-disabled'); else b.setAttribute('aria-disabled', 'true'); } else b.setAttribute('aria-checked', String(!!housePick[id])); });
    q('[data-k="hn"]').textContent = !needle ? '25 Houses' + (nh + nt ? ', ' + (nh + nt) + ' chosen' : '') : shownN ? shownN + ' of 25 Houses match' : 'No House matches the search.';
    qa('[data-k="cat"]').forEach(function (b) { var c = b.getAttribute('data-cat'), n = c === 'house' ? (pl > 0 ? 1 : 0) + nh : c === 'spec' ? nsp : gb > 0 ? 1 : 0; q('[data-k="cc"]', b).textContent = n ? String(n) : ''; });
    rove(q('[data-list="h"]')); rove(q('[data-list="s"]'));
    if (V === 'e6') fitChips();
    if (menuOpen) place();
  }
  function fitChips() { // E6: the strip stays one line; if the chips would be cut, Find House shows its icon only (its name stays as the accessible name and in the tooltip)
    var lens = q('[data-part="lens"]'); lens.removeAttribute('data-tight');
    var cut = qa('.k-chip > span', lens).some(function (e) { return e.scrollWidth > e.clientWidth + 0.5; });
    if (cut) lens.setAttribute('data-tight', '1');
  }
  function rove(list) { var o = qa('.k-opt:not([hidden])', list); if (!o.length) return; var cur = o.filter(function (b) { return b.tabIndex === 0; })[0] || o.filter(function (b) { return b.getAttribute('aria-checked') === 'true'; })[0] || o[0]; qa('.k-opt', list).forEach(function (b) { b.tabIndex = b === cur ? 0 : -1; }); }
  function showCat(name, focusIt) {
    qa('[data-k="cat"]').forEach(function (b) { var on = b.getAttribute('data-cat') === name; b.setAttribute('aria-selected', on ? 'true' : 'false'); b.tabIndex = on ? 0 : -1; if (on) q('[data-k="ptitle"]').textContent = q('span', b).textContent; });
    qa('.k-p5-panel', pop).forEach(function (p) { p.hidden = p.getAttribute('data-cat') !== name; });
    pop.setAttribute('data-view', 'items'); 
    if (menuOpen) place();
  }
  function placeAt(el, btn) { // E6: the popovers are fixed under their key so the sticky strip and the popover stay together while the page scrolls
    if (isPhone()) { el.style.left = ''; el.style.top = ''; return; }
    var br = btn.getBoundingClientRect(); el.style.left = Math.max(8, Math.min(br.left, window.innerWidth - el.offsetWidth - 8)) + 'px'; el.style.top = Math.round(br.bottom + 6) + 'px';
  }
  function place() {
    if (V === 'e6') { placeAt(pop, fbtn); return; }
    if (!isPhone()) { var pr = page.getBoundingClientRect(), br = fbtn.getBoundingClientRect(); pop.style.left = Math.max(0, Math.min(br.left - pr.left, pr.width - pop.offsetWidth)) + 'px'; pop.style.top = Math.round(br.bottom - pr.top + 6) + 'px'; }
    else { pop.style.left = ''; pop.style.top = ''; }
  }
  function openMenu(focusIt) {
    if (menuOpen) return; menuOpen = true; pop.hidden = false; scrim.hidden = !isPhone(); fbtn.setAttribute('aria-expanded', 'true');
    if (isPhone()) pop.setAttribute('data-view', 'cats'); else { var sel = q('[data-k="cat"][aria-selected="true"]'); showCat(sel ? sel.getAttribute('data-cat') : 'house'); }
    if (E3) update(true);
    place(); layoutNotes(); if (focusIt !== false) { var t = isPhone() ? q('[data-k="cat"]:not([hidden])', pop) : q('[data-k="cat"][aria-selected="true"]', pop); if (t) t.focus(); }
  }
  function closeMenu(refocus) {
    if (!MENU || !menuOpen) return; menuOpen = false; pop.hidden = true; scrim.hidden = true; fbtn.setAttribute('aria-expanded', 'false');
    if (E3) { update(true); if (refocus) { if (page.getAttribute('data-mode') === 'action') { var fl0 = q('[data-k="act"][data-do="fill"]'); if (fl0) fl0.focus(); } else fbtn.focus(); } return; }
    layoutNotes(); if (refocus) fbtn.focus();
  }
  function flash(id) { var li = stubs[id - 1]; if (!li) return; li.classList.add('k-flash'); if (li.scrollIntoView) li.scrollIntoView({ block: 'center', behavior: window.matchMedia('(prefers-reduced-motion: reduce)').matches ? 'auto' : 'smooth' }); clearTimeout(flashTimer); flashTimer = setTimeout(function () { qa('.k-flash').forEach(function (e) { e.classList.remove('k-flash'); }); }, 1800); }
  function resetRadio(label) { var all = qa('[role="radio"]', q('[role="radiogroup"][aria-label="' + label + '"]')); choose(all[0], false); }
  function clearFilters() { resetRadio('House'); if (q('[role="radiogroup"][aria-label="Group by"]')) resetRadio('Group by'); specPick = {}; housePick = {}; searchQ = ''; var s = q('[data-k="search"]'); if (s) s.value = ''; update(false); fbtn.focus(); }
  function menuClick(t, role) {
    if (role === 'findbtn') { if (findOpen) closeFind(true); else openFind(t); return true; }
    if (role === 'fclose') { closeFind(true); return true; }
    if (role === 'fpick') { var fid = +t.getAttribute('data-id'); closeFind(true); flash(fid); return true; }
    if (role === 'fbtn') { if (menuOpen) closeMenu(true); else openMenu(true); return true; }
    if (role === 'cat') { showCat(t.getAttribute('data-cat'), true); return true; }
    if (role === 'back') { pop.setAttribute('data-view', 'cats'); var c = q('[data-k="cat"][aria-selected="true"]', pop); if (c) c.focus(); return true; }
    if (role === 'popclose') { closeMenu(true); return true; }
    if (role === 'spick') { var k = +t.getAttribute('data-i'); if (k === 0) specPick = {}; else if (specPick[k]) delete specPick[k]; else specPick[k] = true; update(false); return true; }
    if (role === 'hpick' && V === 'e6') { var id6 = +t.getAttribute('data-id'); if (!visible()[id6]) { toast('Not shown, so it cannot be chosen'); return true; } if (chosen[id6]) delete chosen[id6]; else chosen[id6] = true; update(false); return true; } // E6: ticks the square, nothing else (no scroll, no outline)
    if (role === 'hpick') { var id = +t.getAttribute('data-id'), added = !housePick[id]; if (added) housePick[id] = true; else delete housePick[id]; update(false); if (added) flash(id); return true; }
    if (role === 'chip') { var key = t.getAttribute('data-chip'), p = key.split(':'); if (key === 'pledge') resetRadio('House'); else if (key === 'group') resetRadio('Group by'); else if (key === 'spec') specPick = {}; else if (p[0] === 'house') delete housePick[+p[1]]; else if (p[0] === 'spec') delete specPick[+p[1]]; update(false); fbtn.focus(); return true; }
    if (role === 'clearf') { clearFilters(); return true; }
    return false;
  }
  function popKey(e) {
    var t = e.target, key = e.key;
    if (key === 'Escape') { e.stopPropagation(); e.preventDefault(); if (t.getAttribute && t.getAttribute('data-k') === 'search' && t.value) { t.value = ''; searchQ = ''; update(false); } else closeMenu(true); return; }
    var cat = t.closest ? t.closest('[data-k="cat"]') : null, opt = t.closest ? t.closest('.k-opt') : null, vis = function (r) { return qa(r, pop).filter(function (b) { return !b.hidden; }); };
    if (cat && (key === 'ArrowDown' || key === 'ArrowUp' || key === 'Home' || key === 'End')) { var cs = vis('[data-k="cat"]'), k = cs.indexOf(cat), nk = key === 'Home' ? 0 : key === 'End' ? cs.length - 1 : (k + (key === 'ArrowDown' ? 1 : -1) + cs.length) % cs.length; showCat(cs[nk].getAttribute('data-cat')); if (isPhone()) pop.setAttribute('data-view', 'cats'); cs[nk].focus(); e.preventDefault(); return; }
    if (cat && key === 'ArrowRight') { var f = q('.k-p5-panel:not([hidden]) [role="radio"][aria-checked="true"], .k-p5-panel:not([hidden]) .k-opt[tabindex="0"]', pop); if (f) { f.focus(); e.preventDefault(); } return; }
    if (opt && (key === 'ArrowDown' || key === 'ArrowUp' || key === 'Home' || key === 'End')) { var list = opt.closest('[data-list]'), os = qa('.k-opt:not([hidden])', list), i = os.indexOf(opt);
      if (key === 'ArrowUp' && i === 0 && list.getAttribute('data-list') === 'h') { q('[data-k="search"]').focus(); e.preventDefault(); return; }
      var ni = key === 'Home' ? 0 : key === 'End' ? os.length - 1 : Math.max(0, Math.min(os.length - 1, i + (key === 'ArrowDown' ? 1 : -1))); os.forEach(function (b) { b.tabIndex = b === os[ni] ? 0 : -1; }); os[ni].focus(); e.preventDefault(); return; }
    if (t.getAttribute && t.getAttribute('data-k') === 'search' && (key === 'ArrowDown')) { var o1 = qa('.k-opt:not([hidden])', q('[data-list="h"]'))[0]; if (o1) { o1.focus(); e.preventDefault(); } return; }
    if ((opt || (t.getAttribute && t.getAttribute('data-k') === 'search')) && key === 'ArrowLeft' && !(t.getAttribute && t.getAttribute('data-k') === 'search')) { var sc = q('[data-k="cat"][aria-selected="true"]', pop); if (sc) { sc.focus(); e.preventDefault(); } }
  }

  // ---- E6: Find House (a small single-pick list; scrolls to one House and outlines it; it changes no filter and ticks nothing) ----
  var fpop = q('[data-k="fpop"]'), findOpen = false, findBtn = null;
  if (fpop) { var fl = q('[data-list="f"]', fpop); qa('[data-k="hpick"]').forEach(function (h) { var o = doc.createElement('button'); o.type = 'button'; o.className = 'k-opt k-opt-find'; o.setAttribute('role', 'option'); o.tabIndex = -1; o.setAttribute('data-k', 'fpick'); o.setAttribute('data-id', h.getAttribute('data-id')); o.textContent = h.getAttribute('data-name'); fl.appendChild(o); }); }
  function findRender() { var needle = q('[data-k="fsearch"]').value.trim().toLowerCase(), n = 0; qa('.k-opt', fpop).forEach(function (b) { var hit = !needle || b.textContent.toLowerCase().indexOf(needle) >= 0; b.hidden = !hit; if (hit) n++; }); q('[data-k="fn"]').textContent = !needle ? '25 Houses' : n ? n + ' of 25 Houses match' : 'No House matches the search.'; var vo = qa('.k-opt:not([hidden])', fpop); qa('.k-opt', fpop).forEach(function (b) { b.tabIndex = vo[0] === b ? 0 : -1; }); }
  function openFind(btn) {
    if (findOpen) return; if (menuOpen) closeMenu(false); findOpen = true; findBtn = btn; q('[data-k="fsearch"]').value = ''; findRender(); fpop.hidden = false; scrim.hidden = !isPhone(); qa('[data-k="findbtn"]').forEach(function (b) { b.setAttribute('aria-expanded', 'true'); });
    placeAt(fpop, btn); layoutNotes(); q('[data-k="fsearch"]').focus();
  }
  function closeFind(refocus) {
    if (!findOpen) return; findOpen = false; fpop.hidden = true; scrim.hidden = true; qa('[data-k="findbtn"]').forEach(function (b) { b.setAttribute('aria-expanded', 'false'); });
    layoutNotes(); if (refocus && findBtn) findBtn.focus();
  }
  function findKey(e) {
    var t = e.target, key = e.key, opts = qa('.k-opt:not([hidden])', fpop), isBox = t.getAttribute && t.getAttribute('data-k') === 'fsearch', i = opts.indexOf(t);
    if (key === 'Escape') { e.stopPropagation(); e.preventDefault(); closeFind(true); return; }
    if (isBox && key === 'ArrowDown') { if (opts[0]) { opts[0].focus(); e.preventDefault(); } return; }
    if (isBox && key === 'Enter') { if (opts[0]) { e.preventDefault(); opts[0].click(); } return; }
    if (i >= 0 && (key === 'ArrowDown' || key === 'ArrowUp' || key === 'Home' || key === 'End')) {
      if (key === 'ArrowUp' && i === 0) { q('[data-k="fsearch"]').focus(); e.preventDefault(); return; }
      var ni = key === 'Home' ? 0 : key === 'End' ? opts.length - 1 : Math.max(0, Math.min(opts.length - 1, i + (key === 'ArrowDown' ? 1 : -1))); opts.forEach(function (b) { b.tabIndex = b === opts[ni] ? 0 : -1; }); opts[ni].focus(); e.preventDefault();
    }
  }
  if (fpop) {
    fpop.addEventListener('keydown', findKey);
    fpop.addEventListener('input', function (e) { if (e.target.getAttribute('data-k') === 'fsearch') findRender(); });
    doc.addEventListener('click', function (e) { if (findOpen && e.target.closest && !e.target.closest('[data-k="fpop"]') && !e.target.closest('[data-k="findbtn"]')) closeFind(false); });
    doc.addEventListener('focusout', function (e) { if (findOpen && e.relatedTarget && !fpop.contains(e.relatedTarget) && !(e.relatedTarget.closest && e.relatedTarget.closest('[data-k="findbtn"]'))) closeFind(false); });
    window.addEventListener('resize', function () { if (findOpen) { scrim.hidden = !isPhone(); placeAt(fpop, findBtn); } });
    window.addEventListener('scroll', function () { if (findOpen) placeAt(fpop, findBtn); if (menuOpen && V === 'e6') place(); }, { passive: true });
  }

  // radio groups: one tab stop, arrows move and choose
  function choose(btn, focus) {
    var g = btn.closest('[role="radiogroup"]');
    qa('[role="radio"]', g).forEach(function (b) { var on = b === btn; b.setAttribute('aria-checked', on ? 'true' : 'false'); b.tabIndex = on ? 0 : -1; });
    if (g.getAttribute('aria-label') === 'Board layout') { var list = btn.getAttribute('aria-label') === 'List'; qa('[data-group]').forEach(function (e) { e.hidden = !list; }); }
    if (focus) btn.focus();
  }
  function closeAsk(refocus) { confirmOpen = false; update(false); if (refocus) { var a = q('[data-k="act"][data-do="' + pending + '"]'); if (a) a.focus(); } }
  doc.addEventListener('click', function (e) {
    var li = e.target.closest ? e.target.closest('.k-stub li') : null;
    if (li) { var id = +li.getAttribute('data-id'); if (!li.classList.contains('k-dim')) { chosen[id] = !chosen[id]; update(false); } else toast('Not shown, so it cannot be chosen'); return; }
    var t = e.target.closest ? e.target.closest('button') : null; if (!t) return;
    var role = t.getAttribute('data-k');
    if (role === 'tab' || role === 'seat') { choose(t, false); update(role === 'seat'); return; }
    if (role === 'f4btn') { var o4 = !page.classList.contains('k-f4open'); page.classList.toggle('k-f4open', o4); t.setAttribute('aria-expanded', o4 ? 'true' : 'false'); layoutNotes(); return; }
    if (MENU && menuClick(t, role)) return;
    if (role === 'drawerbtn') { var d = q('[data-k="drawer"]'), open = d.hidden; d.hidden = !open; t.setAttribute('aria-expanded', open ? 'true' : 'false'); layoutNotes(); return; }
    if (t.getAttribute('aria-disabled') === 'true') { toast(role === 'act' ? (q('[data-k="n"]').textContent === '0' ? 'Choose at least one House first' : 'Nothing to change among the chosen Houses that are shown') : role === 'choose' ? 'Every House that is shown is chosen' : 'Nothing is chosen'); return; }
    if (role === 'choose') { var v = visible(); IDS.forEach(function (id) { if (v[id]) chosen[id] = true; }); update(false); }
    else if (role === 'clear') { chosen = {}; update(false); if (E3) { var cb2 = q('[data-k="choose"]'); if (cb2) cb2.focus(); } }
    else if (role === 'act') { pending = t.getAttribute('data-do'); confirmOpen = true; update(true); q('[data-k="confirm"]').focus(); }
    else if (role === 'cancel') closeAsk(true);
    else if (role === 'yes') { closeAsk(true); toast('Mock only: nothing was written.'); }
  });
  if (MENU) {
    doc.addEventListener('click', function (e) { if (menuOpen && e.target.closest && !e.target.closest('[data-k="pop"]') && !e.target.closest('[data-k="fwrap"]')) closeMenu(false); });
    pop.addEventListener('keydown', popKey);
    pop.addEventListener('focusin', function (e) { var o = e.target.closest && e.target.closest('.k-opt'); if (o) { var l = o.closest('[data-list]'); qa('.k-opt', l).forEach(function (b) { b.tabIndex = b === o ? 0 : -1; }); } });
    doc.addEventListener('focusout', function (e) { if (menuOpen && e.relatedTarget && !pop.contains(e.relatedTarget) && !q('[data-k="fwrap"]').contains(e.relatedTarget)) closeMenu(false); });
    doc.addEventListener('input', function (e) { if (e.target && e.target.getAttribute && e.target.getAttribute('data-k') === 'search') { searchQ = e.target.value; update(false); } });
    window.addEventListener('resize', function () { if (V === 'e6') fitChips(); if (menuOpen) { scrim.hidden = !isPhone(); place(); } });
  }
  doc.addEventListener('change', function (e) { if (e.target && e.target.matches && e.target.matches('select')) update(false); });
  doc.addEventListener('keydown', function (e) {
    if (e.key === 'Escape' && confirmOpen) { closeAsk(true); return; }
    if (e.key === 'Escape' && E3 && page.getAttribute('data-mode') === 'action') { chosen = {}; update(false); var cb = q('[data-k="choose"]'); if (cb) cb.focus(); return; }
    var r = e.target.closest ? e.target.closest('[role="radio"]') : null;
    if (r && (e.key === 'ArrowRight' || e.key === 'ArrowLeft' || e.key === 'ArrowDown' || e.key === 'ArrowUp')) {
      var g = r.closest('[role="radiogroup"]'), all = qa('[role="radio"]', g), k = all.indexOf(r), step = (e.key === 'ArrowRight' || e.key === 'ArrowDown') ? 1 : -1;
      choose(all[(k + step + all.length) % all.length], true); update(r.getAttribute('data-k') === 'seat'); e.preventDefault();
    }
  });

  // ---- annotate (pictures only): three callouts and thin leader lines; nothing of it exists in the normal page ----
  var annNode = null;
  function annotate() {
    if (annNode) annNode.remove();
    var NS = 'http://www.w3.org/2000/svg', sx = window.pageXOffset, sy = window.pageYOffset, wrap = q('.k-wrap').getBoundingClientRect();
    var W = Math.max(doc.documentElement.scrollWidth, window.innerWidth), H = Math.max(doc.documentElement.scrollHeight, window.innerHeight);
    annNode = doc.createElement('div'); annNode.className = 'k-an'; annNode.setAttribute('aria-hidden', 'true'); annNode.style.width = W + 'px'; annNode.style.height = H + 'px';
    var svg = doc.createElementNS(NS, 'svg'); svg.setAttribute('width', W); svg.setAttribute('height', H); annNode.appendChild(svg); doc.body.appendChild(annNode);
    var LENS_E2 = { sel: '[data-part="lens"]', n: '1', t: 'Lens row' }, DOCK = { sel: '[data-part="dock"]', n: '3', t: 'Docked plate', l: ['Count medallion (chosen and shown)', 'Choose shown (N), Clear', 'Faction crests', 'Fill, Complete, Reopen (counts)'] };
    var parts = V === 'e3' ? [
      { sel: '[data-part="lens"]', n: '1', t: 'Lens row (nothing chosen)', l: ['Show tabs', 'House chips', 'Filters button (badge)', 'Count of Houses shown', 'Choose shown (N) link'], on: 'filters' },
      { sel: '[data-part="drawer"]', n: '2', t: 'Drawer', l: ['Specialization', 'Jump to House', 'Group (list layout only)'], on: 'filters' },
      { sel: '[data-part="action"]', n: '3', t: 'Action bar (Houses chosen)', l: ['Count medallion, chosen', 'Faction crests', 'Fill, Complete, Reopen (counts)', 'Clear'], on: 'action' }
    ] : V === 'e4' ? [
      { sel: '[data-part="lens"]', n: '1', t: 'Lens row', l: ['Show tabs', 'House chips'] },
      { sel: '[data-part="row2"]', n: '2', t: 'Filter row (always visible)', l: ['Specialization', 'Jump to House', 'Group (list layout only)', 'Count of Houses shown', 'Phone: one Filters (n) key'] },
      DOCK
    ] : V === 'e5' ? [
      { sel: '[data-part="lens"]', n: '1', t: 'Lens row', l: ['Show tabs', 'Filter key', 'Chips for what is on, Clear filters', 'Count of Houses shown'] },
      { sel: '[data-part="menu"]', n: '2', t: 'Filter menu', l: ['Categories: House, Specialization, Group', 'House: pledge, search, the 25 Houses', 'Specialization (several)', 'Group (list layout only)'] },
      DOCK
    ] : V === 'e6' ? [
      { sel: '[data-part="lens"]', n: '1', t: 'Strip, filter mode (nothing chosen)', l: ['Show tabs', 'Filter key, Find House key', 'Chips (Pledge, Specialization, Group), Clear filters', 'Count of Houses shown', 'Choose shown (N) link'], on: 'filters' },
      { sel: '[data-part="menu"]', n: '2', t: 'Filter menu', l: ['Categories: House, Specialization, Group', 'House: pledge, search, a tick per House', 'Specialization (several)'], on: 'filters' },
      { sel: '[data-part="action"]', n: '3', t: 'Action bar (Houses chosen)', l: ['Count medallion, chosen', 'Faction crests', 'Fill, Complete, Reopen (counts)', 'Clear (the menu is away)'], on: 'action' }
    ] : [
      { sel: '[data-part="lens"]', n: '1', t: 'Lens row', l: ['Show tabs', 'House chips', 'Filters button (badge: drawer filters that are on)', 'Count of Houses shown'] },
      { sel: '[data-part="drawer"]', n: '2', t: 'Drawer', l: ['Specialization', 'Jump to House', 'Group (list layout only)'] },
      DOCK
    ];
    var x0 = Math.round(wrap.right + sx + 22), cw = Math.min(300, W - x0 - 12), lastBottom = 8;
    parts.forEach(function (p) {
      var el = q(p.sel); if (!el || el.hidden || (p.on && page.getAttribute('data-mode') !== p.on)) return; var r = el.getBoundingClientRect(); if (!r.width) return;
      var rx = r.left + sx - 3, ry = r.top + sy - 3, rw = r.width + 6, rh = r.height + 6;
      var box = doc.createElementNS(NS, 'rect'); box.setAttribute('x', rx); box.setAttribute('y', ry); box.setAttribute('width', rw); box.setAttribute('height', rh); box.setAttribute('rx', 3); box.setAttribute('class', 'k-an-box'); svg.appendChild(box);
      var c = doc.createElement('div'); c.className = 'k-an-co'; c.style.left = x0 + 'px'; c.style.width = cw + 'px';
      c.innerHTML = '<b><i>' + p.n + '</i>' + p.t + '</b><span>' + p.l.join('</span><span>') + '</span>'; annNode.appendChild(c);
      var ch = c.offsetHeight, top = Math.max(Math.min(ry + rh / 2 - ch / 2, window.innerHeight - ch - 8), lastBottom + 8); c.style.top = top + 'px'; lastBottom = top + c.offsetHeight;
      var ex = rx + rw, ey = ry + rh / 2, cy = top + c.offsetHeight / 2;
      var ln = doc.createElementNS(NS, 'polyline'); ln.setAttribute('points', ex + ',' + ey + ' ' + (x0 - 12) + ',' + ey + ' ' + (x0 - 12) + ',' + cy + ' ' + x0 + ',' + cy); ln.setAttribute('class', 'k-an-ln'); svg.appendChild(ln);
      var dot = doc.createElementNS(NS, 'circle'); dot.setAttribute('cx', ex); dot.setAttribute('cy', ey); dot.setAttribute('r', 3); dot.setAttribute('class', 'k-an-dot'); svg.appendChild(dot);
    });
  }

  // ---- tooltips: the same text is the control's accessible description (aria-describedby), it opens on hover and on keyboard focus (CSS), and a long press shows it on touch ----
  var tipN = 0;
  qa('.k-tip').forEach(function (t) {
    var ctl = t.closest('button, [role="radio"], [tabindex]');
    if (!t.id) t.id = 'k-tp-' + (++tipN); t.removeAttribute('aria-hidden'); t.setAttribute('role', 'tooltip');
    if (ctl) { var d = (ctl.getAttribute('aria-describedby') || '').split(' ').filter(Boolean); if (d.indexOf(t.id) < 0) d.push(t.id); ctl.setAttribute('aria-describedby', d.join(' ')); }
  });
  var holdTimer = 0, dropTimer = 0;
  function dropHeld() { qa('.k-held').forEach(function (e) { e.classList.remove('k-held'); }); }
  doc.addEventListener('touchstart', function (e) {
    clearTimeout(holdTimer); clearTimeout(dropTimer); var h = e.target.closest ? e.target.closest('.k-tipped') : null;
    if (!h) { dropHeld(); return; }
    holdTimer = setTimeout(function () { dropHeld(); h.classList.add('k-held'); }, 500);
  }, { passive: true });
  ['touchend', 'touchcancel'].forEach(function (n) { doc.addEventListener(n, function () { clearTimeout(holdTimer); dropTimer = setTimeout(dropHeld, 2600); }, { passive: true }); });
  doc.addEventListener('contextmenu', function (e) { if (e.target.closest && e.target.closest('.k-held, .k-tipped') && window.matchMedia('(hover: none)').matches) e.preventDefault(); });

  // ---- start ----
  var want = params.get('state');
  if (want === 'chosen' || want === 'confirm') IDS.forEach(function (id) { if (!E3 || [6, 10, 14, 18].indexOf(id) < 0) chosen[id] = true; }); // E3 opens with 21 ticked, as the brief's example
  if (V === 'e6' && params.get('tick')) { (params.get('tick') === 'all' ? IDS : params.get('tick').split(',').map(Number)).forEach(function (n) { if (n >= 1 && n <= 25) chosen[n] = true; }); }
  if (params.get('faction') === 'h') choose(q('[data-k="seat"][data-f="h"]'), false);
  var SHOWKEY = { left: 1, notstarted: 2, progress: 3, top: 4 }, HOUSEKEY = { undecided: 1, pledged: 2 };
  if (SHOWKEY[params.get('show')]) choose(qa('[role="radio"]', q('[role="radiogroup"][aria-label="Show"]'))[SHOWKEY[params.get('show')]], false);
  if (HOUSEKEY[params.get('house')]) choose(qa('[role="radio"]', q('[role="radiogroup"][aria-label="House"]'))[HOUSEKEY[params.get('house')]], false);
  if (params.get('layout') === 'list') { var lb = q('.k-view [aria-label="List"]'); if (lb) choose(lb, false); }
  if (V === 'e6' && (params.get('group') === 'region' || params.get('group') === 'spec')) { var lb2 = q('.k-view [aria-label="List"]'); if (lb2) choose(lb2, false); var gr = qa('[role="radio"]', q('[role="radiogroup"][aria-label="Group by"]')); choose(gr[params.get('group') === 'region' ? 1 : 2], false); }
  if (params.get('drawer') === 'open') { var db = q('[data-k="drawerbtn"]'); q('[data-k="drawer"]').hidden = false; db.setAttribute('aria-expanded', 'true'); }
  if (want === 'confirm') confirmOpen = true;
  if (E4) { var hg = qa('.k-lens-tray .k-grp')[1], slot = q('[data-k="houseslot"]'), tray4 = q('.k-lens-tray'); var place4 = function () { if (phoneMq.matches) slot.appendChild(hg); else tray4.appendChild(hg); layoutNotes(); }; phoneMq.addEventListener('change', place4); place4(); // on a phone the House chips sit under the House heading of the disclosure
    if (params.get('filters') === 'open') { page.classList.add('k-f4open'); q('[data-k="f4btn"]').setAttribute('aria-expanded', 'true'); } }
  if (MENU) {
    (params.get('spec') || '').split(',').forEach(function (n) { if (+n >= 1 && +n <= 5) specPick[+n] = true; });
    (params.get('pick') || '').split(',').forEach(function (nm) { var b = qa('[data-k="hpick"]').filter(function (x) { return x.getAttribute('data-name') === nm; })[0]; if (b) housePick[+b.getAttribute('data-id')] = true; });
    if (params.get('q')) { searchQ = params.get('q'); q('[data-k="search"]').value = searchQ; }
  }
  update(true);
  if (V === 'e6' && doc.fonts && doc.fonts.ready) doc.fonts.ready.then(function () { fitChips(); });
  if (V === 'e6' && params.get('find') === 'open') openFind(page.getAttribute('data-mode') === 'action' ? q('.k-findbar') : q('.k-lens [data-k="findbtn"]'));
  if (MENU && params.get('menu') === 'open') { openMenu(false); var cw = params.get('cat'); if (cw === 'house' || cw === 'spec' || (cw === 'group' && listLayout())) showCat(cw); }
  if (params.get('annotate')) {
    root.classList.add('k-annot'); if (params.get('annotate') === 'phone') root.classList.add('k-annot-phone');
    var go = function () { annotate(); }; if (doc.fonts && doc.fonts.ready) doc.fonts.ready.then(go); go(); window.addEventListener('resize', go);
  }
})();
