// Control area mock, round 2 (tau #974): the little behaviour the three mocks need to be judged by hand. Synthetic numbers only (the same as the fixture board: 25 Houses chosen, Fill would change 21,
// Complete 14, Reopen 16). Nothing is read or written. ?state=idle|chosen|confirm opens a given state, ?theme=dark|light a look, ?bare=1 hides the mock's own notes (the picture sheets use all three),
// ?drawer=open (option E) and ?fly=show|house|spec|jump (option D) open one of the pop-up parts.
(function () {
  'use strict';
  var doc = document, root = doc.documentElement, page = doc.querySelector('.k-page');
  if (!page) return;
  var q = function (s, r) { return (r || doc).querySelector(s); };
  var qa = function (s, r) { return Array.prototype.slice.call((r || doc).querySelectorAll(s)); };
  var params = new URLSearchParams(location.search);
  if (params.get('theme') === 'light' || params.get('theme') === 'dark') root.setAttribute('data-theme', params.get('theme'));
  if (params.get('bare') === '1') root.classList.add('k-bare');
  qa('.k-dev a[data-q]').forEach(function (a) { var kv = a.getAttribute('data-q').split('='); if ((params.get(kv[0]) || (kv[0] === 'state' ? 'idle' : '')) === kv[1]) a.setAttribute('aria-current', 'true'); });

  var N = 25, CHG = { fill: 21, complete: 14, reopen: 16 }, WORD = { fill: 'Fill', complete: 'Complete', reopen: 'Reopen' };
  var pending = 'fill', toastTimer = 0;
  // The question keeps the wording the board asks today (the Fill sample is the real question over the fixture board, with Houses capitalised); the other two are short samples of the same shape.
  var QUESTION = {
    fill: { h: 'Fill {n} Houses?', b: 'Writes 107 missions: +642,000 Solari, +107,000 House Scrip, +182 Combat XP, +22,500 Crafting XP, +30,000 Exploration XP, +37,500 Gathering XP, 92 rewards waiting at the House representative. Houses: Alexin, Dyvetz, Hurata, Imota, Kenola, Lindaren, Maros, Mikkarol and 13 more. 4 other chosen Houses are left alone: nothing to change for them.' },
    complete: { h: 'Complete {n} Houses for {f}?', b: '' },
    reopen: { h: 'Reopen {n} Houses?', b: '' }
  };
  function faction() { var s = q('[data-k="seat"][aria-checked="true"]'); return s ? s.getAttribute('data-name') : 'Atreides'; }
  function toast(msg) {
    var t = q('[data-k="toast"]'); if (!t) return;
    t.textContent = msg; t.hidden = false; clearTimeout(toastTimer);
    toastTimer = setTimeout(function () { t.hidden = true; }, 3200);
  }
  // radio groups (Show, House, layout, faction, a pop-up list): one tab stop, arrows move and choose
  function choose(btn, focus) {
    var g = btn.closest('[role="radiogroup"]');
    qa('[role="radio"]', g).forEach(function (b) { var on = b === btn; b.setAttribute('aria-checked', on ? 'true' : 'false'); b.tabIndex = on ? 0 : -1; });
    if (g.getAttribute('aria-label') === 'Board layout') { var list = btn.getAttribute('aria-label') === 'List'; qa('[data-group]').forEach(function (e) { e.hidden = !list; }); }
    if (focus) btn.focus();
  }
  function set(state, focusAsk) {
    page.setAttribute('data-state', state);
    var on = state !== 'idle';
    var plate = q('[data-k="plate"]');
    if (plate) plate.classList.toggle('k-lit', on);
    var n = q('[data-k="n"]'); if (n) n.textContent = String(on ? N : 0);
    var nsr = q('[data-k="nsr"]'); if (nsr) nsr.textContent = on ? N + ' Houses chosen' : 'No Houses chosen';
    qa('[data-k="act"]').forEach(function (b) {
      var k = b.getAttribute('data-do'), c = q('[data-k="chg"]', b);
      if (on) b.removeAttribute('aria-disabled'); else b.setAttribute('aria-disabled', 'true');
      b.setAttribute('title', on ? WORD[k] + ' changes ' + CHG[k] + ' of the ' + N + ' chosen Houses. A question shows the totals before anything is written.' : 'Choose at least one House first');
      if (c) { c.textContent = on ? String(CHG[k]) : ''; c.hidden = !on; }
    });
    var clear = q('[data-k="clear"]'); if (clear) { if (on) clear.removeAttribute('aria-disabled'); else clear.setAttribute('aria-disabled', 'true'); }
    var all = q('[data-k="choose"]'); if (all) { if (on) all.setAttribute('aria-disabled', 'true'); else all.removeAttribute('aria-disabled'); }
    var c = q('[data-k="confirm"]');
    if (c) {
      c.hidden = state !== 'confirm';
      if (state === 'confirm') {
        var Q = QUESTION[pending], f = function (s) { return s.replace('{n}', CHG[pending]).replace('{f}', faction()); };
        q('[data-k="qh"]', c).textContent = f(Q.h);
        var qb = q('[data-k="qb"]', c); qb.textContent = f(Q.b); qb.hidden = !Q.b;
        q('[data-k="yes"]', c).textContent = WORD[pending] + ' ' + CHG[pending] + ' Houses';
        if (focusAsk) c.focus();
      }
    }
    qa('[data-k="act"]').forEach(function (b) { b.classList.toggle('k-asking', state === 'confirm' && b.getAttribute('data-do') === pending); });
  }
  function closeFlies(except) {
    qa('[data-fly]').forEach(function (w) {
      if (w === except) return;
      var f = q('.k-fly', w), b = q('[data-k="fb"]', w);
      if (f && !f.hidden) { f.hidden = true; b.setAttribute('aria-expanded', 'false'); }
    });
  }
  function filterBadge() {
    var d = q('[data-k="drawer"]'), badge = q('[data-k="fcount"]');
    if (!d || !badge) return;
    var cnt = 0;
    qa('.k-tabs', d).forEach(function (g) { var first = q('[role="radio"]', g); if (first && first.getAttribute('aria-checked') !== 'true') cnt++; });
    qa('select', d).forEach(function (s) { if (s.selectedIndex > 0) cnt++; });
    badge.textContent = String(cnt); badge.hidden = cnt === 0;
  }
  function flySet(opt) {
    var w = opt.closest('[data-fly]'), f = q('.k-fly', w);
    w.setAttribute('data-set', opt === q('[data-k="opt"]', f) ? '0' : '1');
    var fb = q('[data-k="fb"]', w), tp = q('.k-tip', fb);
    if (tp) tp.textContent = fb.getAttribute('aria-label') + ': ' + opt.firstChild.textContent;
  }
  doc.addEventListener('click', function (e) {
    var t = e.target.closest ? e.target.closest('button') : null;
    if (!t) { closeFlies(null); return; }
    var role = t.getAttribute('data-k');
    if (role === 'tab' || role === 'seat') { choose(t, false); filterBadge(); return; }
    if (role === 'fb') {
      var w = t.closest('[data-fly]'), f = q('.k-fly', w), open = f.hidden;
      closeFlies(w); f.hidden = !open; t.setAttribute('aria-expanded', open ? 'true' : 'false');
      if (open) { var sel = q('[aria-checked="true"]', f) || q('button', f); if (sel) sel.focus(); }
      return;
    }
    if (role === 'opt') {
      var w2 = t.closest('[data-fly]'), f2 = q('.k-fly', w2);
      choose(t, false); flySet(t);
      f2.hidden = true; var fb = q('[data-k="fb"]', w2); fb.setAttribute('aria-expanded', 'false'); fb.focus(); return;
    }
    if (role === 'drawerbtn') {
      var d = q('[data-k="drawer"]'), open2 = d.hidden; d.hidden = !open2; t.setAttribute('aria-expanded', open2 ? 'true' : 'false'); return;
    }
    closeFlies(null);
    if (t.getAttribute('aria-disabled') === 'true') { if (role === 'act') toast('Choose at least one House first'); return; } // the real board answers a disabled click with a toast that gives the reason
    if (role === 'choose') set('chosen');
    else if (role === 'clear') set('idle');
    else if (role === 'act') { pending = t.getAttribute('data-do'); set('confirm', true); }
    else if (role === 'cancel') { set('chosen'); var a = q('[data-k="act"][data-do="' + pending + '"]'); if (a) a.focus(); }
    else if (role === 'yes') { set('chosen'); toast('Mock only: nothing was written.'); var a2 = q('[data-k="act"][data-do="' + pending + '"]'); if (a2) a2.focus(); }
  });
  doc.addEventListener('change', function (e) { if (e.target && e.target.matches && e.target.matches('select')) filterBadge(); });
  doc.addEventListener('keydown', function (e) {
    if (e.key === 'Escape') {
      if (page.getAttribute('data-state') === 'confirm') { set('chosen'); var a = q('[data-k="act"][data-do="' + pending + '"]'); if (a) a.focus(); return; }
      var open = qa('.k-fly').filter(function (f) { return !f.hidden; })[0];
      if (open) { var w = open.closest('[data-fly]'); closeFlies(null); q('[data-k="fb"]', w).focus(); return; }
    }
    var r = e.target.closest ? e.target.closest('[role="radio"]') : null;
    if (r && (e.key === 'ArrowRight' || e.key === 'ArrowLeft' || e.key === 'ArrowDown' || e.key === 'ArrowUp')) {
      var g = r.closest('[role="radiogroup"]'), all = qa('[role="radio"]', g), i = all.indexOf(r);
      var step = (e.key === 'ArrowRight' || e.key === 'ArrowDown') ? 1 : -1;
      var next = all[(i + step + all.length) % all.length];
      choose(next, true);
      if (next.getAttribute('data-k') === 'opt') flySet(next);
      filterBadge(); e.preventDefault();
    }
  });
  var want = params.get('state');
  set(want === 'chosen' || want === 'confirm' ? want : 'idle', false);
  if (params.get('drawer') === 'open') { var db = q('[data-k="drawerbtn"]'); if (db) { q('[data-k="drawer"]').hidden = false; db.setAttribute('aria-expanded', 'true'); } }
  var fly = params.get('fly');
  if (fly) { var w4 = q('[data-fly="' + fly + '"]'); if (w4) { q('.k-fly', w4).hidden = false; q('[data-k="fb"]', w4).setAttribute('aria-expanded', 'true'); } }
  if (params.get('layout') === 'list') { var lb = q('.k-view [aria-label="List"]'); if (lb) choose(lb, false); }
  filterBadge();
})();
