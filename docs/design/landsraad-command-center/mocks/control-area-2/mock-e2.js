// Control area mock, options E2 and E3 (tau #974, round 2). Behaviour for option-e2.html and option-e3.html only (E3: the lens row turns into the action bar when Houses are chosen); mock.js (options C, D, E) is not used on this page because E2 models two separate things:
// what is VISIBLE (the Show tabs, the House chips and the drawer) and what is CHOSEN (the ticks, Choose shown, Clear). The counts on the plate are chosen AND visible, so a House that a filter
// hides is never counted and never acted on. Fixture data only, nothing is read or written.
// Query: ?state=idle|chosen|confirm, ?theme=dark|light, ?bare=1, ?drawer=open, ?layout=list, ?plate=full (keep the full plate when nothing is chosen), ?annotate=1|phone (callouts for the
// three parts, for the annotated pictures only), ?show=left|notstarted|progress|top, ?house=undecided|pledged (open with a filter on).
(function () {
  'use strict';
  var doc = document, root = doc.documentElement, page = doc.querySelector('.k-page');
  var V = page && page.getAttribute('data-variant'); if (V !== 'e2' && V !== 'e3') return;
  var E3 = V === 'e3';
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
  var plate = q('[data-k="plate"]'), stubs = qa('.k-stub li');
  stubs.forEach(function (li, k) { li.setAttribute('data-id', String(k + 1)); });

  function radioIndex(label) { var g = q('[role="radiogroup"][aria-label="' + label + '"]'); var all = qa('[role="radio"]', g); for (var k = 0; k < all.length; k++) if (all[k].getAttribute('aria-checked') === 'true') return k; return 0; }
  function visible() {
    var s = radioIndex('Show'), h = radioIndex('House'), sp = q('select[aria-label="Specialization"]').selectedIndex;
    var out = {}; IDS.forEach(function (id) { if ((!SHOW[s] || SHOW[s][id]) && (!HOUSE[h] || HOUSE[h][id]) && (sp === 0 || id % 5 === sp - 1)) out[id] = true; });
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
    if (E3) page.setAttribute('data-mode', on ? 'action' : 'filters'); else plate.classList.toggle('k-slim', !on && params.get('plate') !== 'full');
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
    var badge = q('[data-k="fcount"]'), cnt = 0;
    qa('select', q('[data-k="drawer"]')).forEach(function (s) { if (s.selectedIndex > 0 && !s.closest('[hidden]')) cnt++; });
    badge.textContent = String(cnt); badge.hidden = cnt === 0;
    layoutNotes();
  }
  function layoutNotes() { if (root.classList.contains('k-annot')) annotate(); }

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
    if (role === 'drawerbtn') { var d = q('[data-k="drawer"]'), open = d.hidden; d.hidden = !open; t.setAttribute('aria-expanded', open ? 'true' : 'false'); layoutNotes(); return; }
    if (t.getAttribute('aria-disabled') === 'true') { toast(role === 'act' ? (q('[data-k="n"]').textContent === '0' ? 'Choose at least one House first' : 'Nothing to change among the chosen Houses that are shown') : role === 'choose' ? 'Every House that is shown is chosen' : 'Nothing is chosen'); return; }
    if (role === 'choose') { var v = visible(); IDS.forEach(function (id) { if (v[id]) chosen[id] = true; }); update(false); }
    else if (role === 'clear') { chosen = {}; update(false); if (E3) { var cb2 = q('[data-k="choose"]'); if (cb2) cb2.focus(); } }
    else if (role === 'act') { pending = t.getAttribute('data-do'); confirmOpen = true; update(true); q('[data-k="confirm"]').focus(); }
    else if (role === 'cancel') closeAsk(true);
    else if (role === 'yes') { closeAsk(true); toast('Mock only: nothing was written.'); }
  });
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
    var parts = E3 ? [
      { sel: '[data-part="lens"]', n: '1', t: 'Lens row (nothing chosen)', l: ['Show tabs', 'House chips', 'Filters button (badge)', 'Count of Houses shown', 'Choose shown (N) link'], on: 'filters' },
      { sel: '[data-part="drawer"]', n: '2', t: 'Drawer', l: ['Specialization', 'Jump to House', 'Group (list layout only)'], on: 'filters' },
      { sel: '[data-part="action"]', n: '3', t: 'Action bar (Houses chosen)', l: ['Count medallion, chosen', 'Faction crests', 'Fill, Complete, Reopen (counts)', 'Clear'], on: 'action' }
    ] : [
      { sel: '[data-part="lens"]', n: '1', t: 'Lens row', l: ['Show tabs', 'House chips', 'Filters button (badge: drawer filters that are on)', 'Count of Houses shown'] },
      { sel: '[data-part="drawer"]', n: '2', t: 'Drawer', l: ['Specialization', 'Jump to House', 'Group (list layout only)'] },
      { sel: '[data-part="dock"]', n: '3', t: 'Docked plate', l: ['Count medallion (chosen and shown)', 'Choose shown (N), Clear', 'Faction crests', 'Fill, Complete, Reopen (counts)'] }
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
  if (params.get('faction') === 'h') choose(q('[data-k="seat"][data-f="h"]'), false);
  var SHOWKEY = { left: 1, notstarted: 2, progress: 3, top: 4 }, HOUSEKEY = { undecided: 1, pledged: 2 };
  if (SHOWKEY[params.get('show')]) choose(qa('[role="radio"]', q('[role="radiogroup"][aria-label="Show"]'))[SHOWKEY[params.get('show')]], false);
  if (HOUSEKEY[params.get('house')]) choose(qa('[role="radio"]', q('[role="radiogroup"][aria-label="House"]'))[HOUSEKEY[params.get('house')]], false);
  if (params.get('layout') === 'list') { var lb = q('.k-view [aria-label="List"]'); if (lb) choose(lb, false); }
  if (params.get('drawer') === 'open') { var db = q('[data-k="drawerbtn"]'); q('[data-k="drawer"]').hidden = false; db.setAttribute('aria-expanded', 'true'); }
  if (want === 'confirm') confirmOpen = true;
  update(true);
  if (params.get('annotate')) {
    root.classList.add('k-annot'); if (params.get('annotate') === 'phone') root.classList.add('k-annot-phone');
    var go = function () { annotate(); }; if (doc.fonts && doc.fonts.ready) doc.fonts.ready.then(go); go(); window.addEventListener('resize', go);
  }
})();
