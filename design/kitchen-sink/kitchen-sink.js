// Behaviour of the kitchen sink: just enough to try every part of the module by hand. No framework, no network, no storage, no HTML from strings:
// the page is changed with setAttribute, classList, textContent and style.setProperty only. A test reads this file.
(function () {
  'use strict';
  var doc = document, root = doc.documentElement;
  function all(sel, from) { return Array.prototype.slice.call((from || doc).querySelectorAll(sel)); }

  // The look files are a fixed table, never a stored or typed string (the same names as design.LookFiles; a test compares them).
  var FILES = {
    fremen: ['tokens-fremen.css', 'components-fremen.css'],
    awakening: ['tokens-awakening.css', 'components-awakening.css'],
    sietch: ['tokens-sietch.css', 'components-sietch.css']
  };
  var VALUES = { look: Object.keys(FILES), motion: ['full', 'calm', 'off'], 'heading-font': ['dune-rise', 'system'] };

  function setLook(look) {
    if (VALUES.look.indexOf(look) < 0) return;
    var t = doc.getElementById('lk-tokens'), c = doc.getElementById('lk-components');
    t.setAttribute('href', '../assets/' + FILES[look][0]);
    c.setAttribute('href', '../assets/' + FILES[look][1]);
    root.setAttribute('data-look', look);
  }
  function setValue(group, value) {
    if (VALUES[group].indexOf(value) < 0) return;
    if (group === 'look') setLook(value);
    else if (group === 'motion') { root.setAttribute('data-motion', value); root.setAttribute('data-motion-src', 'user'); }
    else root.setAttribute('data-heading-font', value);
  }

  all('[data-switch]').forEach(function (seg) {
    var group = seg.getAttribute('data-switch');
    seg.addEventListener('click', function (e) {
      var b = e.target.closest('button[data-value]');
      if (!b) return;
      setValue(group, b.getAttribute('data-value'));
      all('button[data-value]', seg).forEach(function (o) { o.setAttribute('aria-pressed', String(o === b)); });
    });
  });

  all('[data-theme-toggle]').forEach(function (b) {
    b.addEventListener('click', function () {
      var light = root.getAttribute('data-theme') === 'light' || (!root.hasAttribute('data-theme') && window.matchMedia('(prefers-color-scheme: light)').matches);
      root.setAttribute('data-theme', light ? 'dark' : 'light');
    });
  });

  // meters: the value is data, the bar reads a custom property
  all('[data-v]').forEach(function (m) { m.style.setProperty('--v', String(Math.max(0, Math.min(100, Number(m.getAttribute('data-v')) || 0))) + '%'); });
  all('[data-indeterminate]').forEach(function (i) { i.indeterminate = true; });

  // folding heads
  all('button.d-head[aria-controls]').forEach(function (b) {
    b.addEventListener('click', function () {
      var open = b.getAttribute('aria-expanded') !== 'true', body = doc.getElementById(b.getAttribute('aria-controls'));
      b.setAttribute('aria-expanded', String(open));
      if (body) { if (open) body.removeAttribute('hidden'); else body.setAttribute('hidden', ''); }
    });
  });

  // tabs, with the sliding underline
  all('.d-seg[data-seg]').forEach(function (seg) {
    var ind = seg.querySelector('.d-ind');
    function place() {
      var sel = seg.querySelector('[aria-selected="true"]');
      if (ind && sel) { ind.style.setProperty('--x', sel.offsetLeft + 'px'); ind.style.setProperty('--w', sel.offsetWidth + 'px'); }
    }
    seg.addEventListener('click', function (e) {
      var b = e.target.closest('[role="tab"]');
      if (!b) return;
      all('[role="tab"]', seg).forEach(function (o) {
        var on = o === b, pane = doc.getElementById(o.getAttribute('data-pane'));
        o.setAttribute('aria-selected', String(on));
        if (pane) { if (on) pane.removeAttribute('hidden'); else pane.setAttribute('hidden', ''); }
      });
      place();
    });
    place();
  });

  // the popover, the tooltip's Esc, the rail's collapse tab and the dialog
  all('[data-pop]').forEach(function (b) {
    b.addEventListener('click', function () {
      var pop = doc.getElementById(b.getAttribute('data-pop'));
      if (pop) pop.hidden = !pop.hidden;
    });
  });
  all('[data-collapse]').forEach(function (b) {
    b.addEventListener('click', function () {
      var rail = doc.getElementById(b.getAttribute('data-collapse'));
      if (!rail) return;
      var shut = rail.classList.toggle('shut');
      b.setAttribute('aria-expanded', String(!shut));
    });
  });
  all('[data-open-dialog]').forEach(function (b) {
    b.addEventListener('click', function () {
      var d = doc.getElementById(b.getAttribute('data-open-dialog'));
      if (d && d.showModal) d.showModal();
    });
  });
  // Esc hides the tooltip that is showing until the pointer or the focus leaves its part (the module's .dismissed)
  doc.addEventListener('keydown', function (e) {
    if (e.key !== 'Escape') return;
    all('.d-tt').forEach(function (t) { if (t.matches(':hover, :focus-within')) t.classList.add('dismissed'); });
    all('.d-pop:not([hidden])').forEach(function (p) { p.hidden = true; });
  });
  all('.d-tt').forEach(function (t) {
    function clear() { t.classList.remove('dismissed'); }
    t.addEventListener('mouseleave', clear);
    t.addEventListener('focusout', clear);
  });
})();
