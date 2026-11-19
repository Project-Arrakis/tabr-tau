// Control area mock (tau #974): the little behaviour the two mocks need to be judged by hand. Synthetic numbers only (the same as the fixture board: 25 Houses chosen, Fill would change 21,
// Complete 14, Reopen 16). Nothing is read or written. ?state=idle|chosen|confirm opens a given state (the picture sheets use it).
(function () {
  'use strict';
  var doc = document, page = doc.querySelector('.cam-page');
  if (!page) return;
  var q = function (s, r) { return (r || doc).querySelector(s); };
  var qa = function (s, r) { return Array.prototype.slice.call((r || doc).querySelectorAll(s)); };
  var N = 25, CHG = { fill: 21, complete: 14, reopen: 16 }, WORD = { fill: 'Fill', complete: 'Complete', reopen: 'Reopen' };
  var pending = '';
  // The question keeps the wording the board asks today (the Fill sample is the real question over the fixture board, with Houses capitalised); the other two are short samples of the same shape.
  var QUESTION = {
    fill: 'Fill {n} Houses? Writes 107 missions: +642,000 Solari, +107,000 House Scrip, +182 Combat XP, +22,500 Crafting XP, +30,000 Exploration XP, +37,500 Gathering XP, 92 rewards waiting at the House representative. Houses: Alexin, Dyvetz, Hurata, Imota, Kenola, Lindaren, Maros, Mikkarol and 13 more. 4 other chosen Houses are left alone: nothing to change for them. Written at once, with a backup first.',
    complete: 'Complete {n} Houses for {f}? Written at once, with a backup first.',
    reopen: 'Reopen {n} Houses? Written at once, with a backup first.'
  };
  function faction() { var s = q('[data-cam="faction"]'); return s ? s.options[s.selectedIndex].text.replace(' (your faction)', '') : 'Atreides'; }
  function set(state, focusConfirm) {
    page.setAttribute('data-state', state);
    var on = state !== 'idle';
    var sum = q('[data-cam="sum"]'), say = q('[data-cam="say"]');
    if (sum) sum.textContent = on ? N + ' Houses chosen' : 'No Houses chosen';
    qa('[data-cam="act"]').forEach(function (b) {
      var k = b.getAttribute('data-do'), c = q('.cam-chg', b);
      if (on) b.removeAttribute('aria-disabled'); else b.setAttribute('aria-disabled', 'true');
      b.setAttribute('title', on ? WORD[k] + ' changes ' + CHG[k] + ' of the ' + N + ' chosen Houses. A question shows the totals before anything is written.' : 'Choose at least one House first');
      if (c) { c.textContent = on ? String(CHG[k]) : ''; c.hidden = !on; }
    });
    qa('[data-cam="clear"], [data-cam="choose"]').forEach(function (l) {
      var isClear = l.getAttribute('data-cam') === 'clear';
      if (isClear && !on) l.setAttribute('aria-disabled', 'true'); else l.removeAttribute('aria-disabled');
    });
    qa('[data-cam="confirm"]').forEach(function (c) {
      c.hidden = state !== 'confirm';
      if (state === 'confirm') {
        q('[data-cam="q"]', c).textContent = QUESTION[pending].replace('{n}', CHG[pending]).replace('{f}', faction());
        q('[data-cam="yes"]', c).textContent = WORD[pending] + ' ' + CHG[pending] + ' Houses';
        if (focusConfirm) c.focus();
      }
    });
    if (say && state !== 'confirm') say.textContent = '';
  }
  doc.addEventListener('click', function (e) {
    var t = e.target.closest ? e.target.closest('button') : null;
    if (!t) return;
    var seg = t.closest('.cam-seg'), role = t.getAttribute('data-cam');
    if (seg) { qa('button', seg).forEach(function (b) { b.setAttribute('aria-pressed', b === t ? 'true' : 'false'); }); return; }
    if (t.getAttribute('aria-disabled') === 'true') return; // the real board answers a disabled click with a toast that gives the reason
    if (role === 'choose') set('chosen');
    else if (role === 'clear') set('idle');
    else if (role === 'act') { pending = t.getAttribute('data-do'); set('confirm', true); }
    else if (role === 'cancel') set('chosen');
    else if (role === 'yes') { set('chosen'); var s = q('[data-cam="say"]'); if (s) s.textContent = 'Mock only: nothing was written.'; }
  });
  doc.addEventListener('keydown', function (e) { if (e.key === 'Escape' && page.getAttribute('data-state') === 'confirm') { set('chosen'); } });
  var want = (new URLSearchParams(location.search)).get('state');
  pending = 'fill';
  set(want === 'chosen' || want === 'confirm' ? want : 'idle', false);
})();
