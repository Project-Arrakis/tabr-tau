// The Theme menu and the bulb in the header of the editor (#570, slice 3; Colours: #587).
// look-boot.js (in the head) keeps and applies the choices; motion-boot.js keeps the Motion setting; this file is only the control. It builds, in #theme-host:
//   the bulb          a button that switches between Dark and Dayside (the page follows the system until it is pressed once)
//   the Theme button  opens a menu of five groups, each a list of radio items: Look (Fremen, Awakening, Sietch), Dark / Dayside (follow the system, Dark, Dayside), Colours (a filter box, None, then the 60
//                     palettes of the design module under their group headings), Font (System, Dune Rise: headings) and Motion (Full, Calm, Off)
// The filter box of Colours (#683) keeps the palettes whose name, group or theme word (Dark, Dayside) holds every word typed, in any case; a group with none left loses its heading, the chosen palette and
// None always stay (the chosen one is marked "(chosen)" when it is shown only for that), and a line says "N of M palettes", or what to do when none match (a polite live region). Arrow Down and Up walk
// through the box and the items, Enter in the box moves to the first match (it picks nothing), Escape clears the box first and closes the menu on the next press, a Clear button (44 px) empties the box, and
// a letter typed on a palette item moves to the box. The box stays at the top of the menu while the list scrolls, and an item the arrows reach is scrolled clear of it (WCAG 2.4.11).
// A palette applies in its own theme only (58 are dark, 2 are Dayside palettes). One that is not in effect in the theme shown now stays pickable and says so: its item carries the words "Dayside only"
// or "Dark only" and a tooltip, and the button's own tooltip says the palette is not applied.
// The menu is a role="menu" popover: Arrow Up and Down, Home and End move, Enter or Space picks (the menu stays open, so a combination can be tried), Escape closes it and gives the focus back to
// the button, Tab leaves it, a click outside closes it. It is placed under the button inside the window and scrolls when it is taller than the window; its items are 44 px high on a touch screen.
// Built with createElement and textContent only (no markup from a string); the icon is an inline SVG built here, so it does not wait for the module's sprite file.
(function (root) {
  'use strict';
  const doc = root.document;
  const L = root.TauLook;
  const M = root.TauMotion;
  const host = doc && doc.getElementById('theme-host');
  if (!host || !L || host.__themeBound) return;
  host.__themeBound = true;
  const SVG = 'http://www.w3.org/2000/svg';

  const LOOK_NAMES = { fremen: 'Fremen', awakening: 'Awakening', sietch: 'Sietch' };
  const LOOK_ORDER = ['fremen', 'awakening', 'sietch'];
  const THEME_NAMES = { system: 'Follow the system', dark: 'Dark', light: 'Dayside' };
  const FONT_NAMES = { system: 'System', 'dune-rise': 'Dune Rise: headings' };
  const PALETTES = L.PALETTES;
  const PALETTE_NAMES = Object.fromEntries(PALETTES.map((p) => [p.id, p.name]));
  const THEME_WORD = { dark: 'Dark', light: 'Dayside' };
  const MOTION_NAMES = { full: 'Full', calm: 'Calm', off: 'Off' };
  const MOTION_TIPS = {
    full: 'Every animation: the count that bumps, the row that lights up, toasts that fade and fill. The default when the system does not ask for less',
    calm: 'Short fades only: no bumps, no highlights, no slides. The default when the system asks for less motion',
    off: 'No animation at all and every change is instant',
  };
  const GROUPS = [
    { id: 'look', title: 'Look', values: LOOK_ORDER, names: LOOK_NAMES },
    { id: 'theme', title: 'Dark / Dayside', values: ['system', 'dark', 'light'], names: THEME_NAMES },
    { id: 'palette', title: 'Colours', palettes: true },
    { id: 'font', title: 'Font', values: ['system', 'dune-rise'], names: FONT_NAMES },
    { id: 'motion', title: 'Motion', values: ['full', 'calm', 'off'], names: MOTION_NAMES, tips: MOTION_TIPS },
  ];

  // what each group shows as chosen now
  const current = (group) => {
    if (group === 'look') return L.look();
    if (group === 'theme') return L.themePicked() ? L.theme() : 'system';
    if (group === 'palette') return L.palette() || 'none';
    if (group === 'font') return L.font();
    return M ? M.mode() : 'full';
  };
  const pick = (group, value) => {
    if (group === 'look') L.setLook(value);
    else if (group === 'theme') { if (value === 'system') L.followSystem(); else L.setTheme(value); }
    else if (group === 'font') L.setFont(value);
    else if (group === 'palette') L.setPalette(value === 'none' ? null : value);
    else if (M) M.set(value);
  };

  // the bulb: a lit bulb is Dayside, an unlit one is Dark (the icons of the module, built here)
  const icon = (lit) => {
    const svg = doc.createElementNS(SVG, 'svg');
    svg.setAttribute('class', 'd-i');
    svg.setAttribute('viewBox', '0 0 24 24');
    svg.setAttribute('aria-hidden', 'true');
    const add = (d, fill) => {
      const p = doc.createElementNS(SVG, 'path');
      p.setAttribute('d', d);
      if (fill) p.setAttribute('fill', 'currentColor');
      svg.appendChild(p);
    };
    if (lit) {
      add('M12 4.5a5.2 5.2 0 0 0-3 9.5c.6.5.9 1.2.9 2v.5h4.2V16c0-.8.3-1.5.9-2a5.2 5.2 0 0 0-3-9.5z', true);
      add('M9.5 18.5h5M10.5 21h3M12 .9v1.4M4.2 4l1 1M19.8 4l-1 1M2 10.5h1.4M20.6 10.5H22');
    } else {
      add('M12 3.5a6 6 0 0 0-3.4 10.9c.6.5.9 1.2.9 2v.6h5v-.6c0-.8.3-1.5.9-2A6 6 0 0 0 12 3.5z');
      add('M9.5 18.5h5M10.5 21h3');
    }
    return svg;
  };

  const bulb = doc.createElement('button');
  bulb.type = 'button';
  bulb.id = 'theme-bulb';
  bulb.className = 'd-btn icon quiet';
  const btn = doc.createElement('button');
  btn.type = 'button';
  btn.id = 'theme-btn';
  btn.className = 'd-btn sm quiet';
  btn.setAttribute('aria-haspopup', 'menu');
  btn.setAttribute('aria-expanded', 'false');
  btn.setAttribute('aria-controls', 'theme-menu');
  const menu = doc.createElement('div');
  menu.id = 'theme-menu';
  menu.className = 'x-thmenu';
  menu.setAttribute('role', 'menu');
  menu.setAttribute('aria-label', 'Theme: look, dark or dayside, colours, font, motion');
  menu.hidden = true;

  // one radio item: a dot and a name (and, for a palette, the tag that says it is not in effect now)
  const radio = (group, value, label) => {
    const it = doc.createElement('button');
    it.type = 'button';
    it.setAttribute('role', 'menuitemradio');
    it.setAttribute('data-thgroup', group);
    it.setAttribute('data-thvalue', value);
    it.setAttribute('aria-checked', 'false');
    it.tabIndex = -1;
    const dot = doc.createElement('span');
    dot.className = 'x-thdot';
    dot.setAttribute('aria-hidden', 'true');
    const name = doc.createElement('span');
    name.className = 'x-thname';
    name.textContent = label;
    it.appendChild(dot);
    it.appendChild(name);
    return it;
  };
  // the filter of the Colours group: what each palette is searched by (name, group, theme word), and the box and the line that counts
  const fold = (s) => String(s).toLowerCase().replace(/['\u2019]/g, '').replace(/[^a-z0-9]+/g, ' ').trim();
  const HAY = Object.fromEntries(PALETTES.map((p) => [p.id, fold(p.name + ' ' + p.group + ' ' + THEME_WORD[p.theme] + (p.theme === 'light' ? ' light' : ''))]));
  let search = null;
  let count = null;
  let clear = null;
  // the Colours group: the filter, None, then each group of the module under its heading, in the module's order
  const buildPalettes = (outer, headId) => {
    const row = doc.createElement('div');
    row.className = 'x-thfindrow';
    search = doc.createElement('input');
    search.type = 'search';
    search.id = 'theme-filter';
    search.className = 'd-input x-thfind';
    search.placeholder = 'Filter colours';
    search.autocomplete = 'off';
    search.spellcheck = false;
    search.tabIndex = -1;
    search.setAttribute('aria-label', 'Filter colours by name, group, Dark or Dayside');
    count = doc.createElement('p');
    count.className = 'x-thcount';
    count.setAttribute('role', 'status');
    count.setAttribute('aria-live', 'polite');
    clear = doc.createElement('button');
    clear.type = 'button';
    clear.className = 'd-btn quiet x-thclear';
    clear.textContent = 'Clear';
    clear.setAttribute('aria-label', 'Clear filter');
    clear.tabIndex = -1;
    clear.hidden = true;
    const field = doc.createElement('div');
    field.className = 'x-thfindbox';
    field.appendChild(search);
    field.appendChild(clear);
    row.appendChild(field);
    row.appendChild(count);
    outer.appendChild(row);
    const box = doc.createElement('div');
    box.id = 'theme-g-palette-list';
    box.setAttribute('role', 'presentation');
    outer.appendChild(box);
    box.appendChild(radio('palette', 'none', 'None (the look\u2019s own)'));
    let inner = null;
    let last = null;
    PALETTES.forEach((p) => {
      if (p.group !== last) {
        last = p.group;
        const sub = doc.createElement('div');
        sub.className = 'x-thsub';
        sub.id = headId + '-' + (box.querySelectorAll('[role="group"]').length + 1);
        sub.setAttribute('role', 'presentation');
        sub.textContent = p.group;
        inner = doc.createElement('div');
        inner.setAttribute('role', 'group');
        inner.setAttribute('aria-labelledby', sub.id);
        box.appendChild(sub);
        box.appendChild(inner);
      }
      const it = radio('palette', p.id, p.name);
      it.setAttribute('data-ththeme', p.theme);
      const tag = doc.createElement('span');
      tag.className = 'x-thtag';
      tag.hidden = true;
      it.appendChild(tag);
      const kept = doc.createElement('span');
      kept.className = 'x-thtag x-thkept';
      kept.textContent = '(chosen)';
      kept.hidden = true;
      it.appendChild(kept);
      inner.appendChild(it);
    });
  };

  GROUPS.forEach((g) => {
    const head = doc.createElement('div');
    head.className = 'x-thg';
    head.id = 'theme-g-' + g.id;
    head.setAttribute('role', 'presentation');
    head.textContent = g.title;
    menu.appendChild(head);
    const box = doc.createElement('div');
    box.setAttribute('role', 'group');
    box.setAttribute('aria-labelledby', head.id);
    if (g.palettes) {
      buildPalettes(box, head.id);
      menu.appendChild(box);
      return;
    }
    g.values.forEach((v) => {
      const it = radio(g.id, v, g.names[v]);
      if (g.tips) it.title = g.tips[v];
      box.appendChild(it);
    });
    menu.appendChild(box);
  });
  const note = doc.createElement('p');
  note.className = 'x-thnote';
  const lightNames = PALETTES.filter((p) => p.theme === 'light').map((p) => p.name);
  note.textContent = 'A palette applies in its own theme only: ' + (PALETTES.length - lightNames.length) + ' are dark, ' + lightNames.length + ' are Dayside palettes (' + lightNames.join(', ') + ').';
  menu.appendChild(note);
  host.appendChild(bulb);
  host.appendChild(btn);
  host.appendChild(menu);

  const allItems = () => Array.prototype.slice.call(menu.querySelectorAll('[role="menuitemradio"]'));
  const items = () => Array.prototype.slice.call(menu.querySelectorAll('[role="menuitemradio"]:not([hidden])'));
  // the places the arrows walk through: the filter box and the items that are shown, in the order of the page
  const ring = () => Array.prototype.slice.call(menu.querySelectorAll('input.x-thfind, [role="menuitemradio"]:not([hidden])'));
  const applyFilter = () => {
    if (!search) return;
    const terms = fold(search.value).split(' ').filter(Boolean);
    let shown = 0;
    allItems().forEach((it) => {
      const id = it.getAttribute('data-thvalue');
      if (it.getAttribute('data-thgroup') !== 'palette' || id === 'none') return;
      const hit = terms.every((t) => HAY[id].indexOf(t) >= 0);
      if (hit) shown += 1;
      const only = !hit && it.getAttribute('aria-checked') === 'true'; // shown for being the chosen one
      it.hidden = !(hit || only);
      it.setAttribute('data-thonly', String(only));
      const mark = it.querySelector('.x-thkept');
      if (mark) mark.hidden = !only;
    });
    clear.hidden = !search.value;
    Array.prototype.forEach.call(menu.querySelectorAll('.x-thsub'), (sub) => {
      const inner = sub.nextElementSibling;
      sub.hidden = inner.hidden = !inner.querySelector('[role="menuitemradio"]:not([hidden])');
    });
    const typed = search.value.trim();
    count.textContent = terms.length ? (shown ? shown + ' of ' + PALETTES.length + ' palettes' : 'No palette matches \u201c' + (typed.length > 24 ? typed.slice(0, 24) + '\u2026' : typed) + '\u201d. Clear the box to see all ' + PALETTES.length + '.') : PALETTES.length + ' palettes';
  };
  const chosen = () => PALETTES.find((p) => p.id === L.palette()) || null;
  const colours = () => { const p = chosen(); return p ? p.name + (L.paletteShows() ? '' : ' (not applied: it shows in ' + THEME_WORD[p.theme] + ' only)') : 'None'; };
  const words = () => 'Look: ' + LOOK_NAMES[L.look()] + '. ' + (L.themePicked() ? (L.theme() === 'light' ? 'Dayside' : 'Dark') : 'Dark or Dayside by the system') + '. Colours: ' + colours() + '. Font: ' + FONT_NAMES[L.font()] + '. Motion: ' + MOTION_NAMES[current('motion')] + '.';
  const refresh = () => {
    const dayside = L.theme() === 'light';
    bulb.replaceChildren(icon(dayside));
    const to = dayside ? 'Dark' : 'Dayside';
    bulb.setAttribute('aria-label', 'Switch to ' + to);
    bulb.title = 'Switch to ' + to + (L.themePicked() ? '' : ' (now it follows the system)');
    btn.textContent = 'Theme: ' + LOOK_NAMES[L.look()] + (chosen() ? ' \u00b7 ' + chosen().name : '');
    btn.setAttribute('aria-label', 'Theme. ' + words() + ' Open the menu');
    btn.title = words();
    allItems().forEach((it) => {
      it.setAttribute('aria-checked', String(it.getAttribute('data-thvalue') === current(it.getAttribute('data-thgroup'))));
      // a palette that is not in effect in the theme shown now says so
      const own = it.getAttribute('data-ththeme');
      if (own) {
        const off = own !== L.theme();
        const tag = it.querySelector('.x-thtag');
        it.classList.toggle('x-thoff', off);
        tag.hidden = !off;
        tag.textContent = off ? THEME_WORD[own] + ' only' : '';
        if (off) it.title = PALETTE_NAMES[it.getAttribute('data-thvalue')] + ' is a ' + THEME_WORD[own] + ' palette: it shows in ' + THEME_WORD[own] + ' only. Now the look\u2019s own colours show; the choice is kept.';
        else it.removeAttribute('title');
      }
    });
    applyFilter();
  };

  const place = () => {
    const r = btn.getBoundingClientRect();
    const vw = doc.documentElement.clientWidth || root.innerWidth || 800;
    const vh = root.innerHeight || 600;
    const w = Math.min(288, vw - 16);
    const left = Math.max(8, Math.min(r.right - w, vw - w - 8));
    const top = Math.min(r.bottom + 4, Math.max(8, vh - 120));
    menu.style.left = left + 'px';
    menu.style.top = top + 'px';
    menu.style.maxHeight = Math.max(160, Math.min(560, vh * 0.78, vh - top - 8)) + 'px';
  };
  const focusItem = (list, i) => {
    list.forEach((x) => { x.tabIndex = -1; });
    list[i].tabIndex = 0;
    list[i].focus();
    const row = menu.querySelector('.x-thfindrow');
    if (row && row.offsetHeight) menu.style.setProperty('--thfind-h', (row.offsetHeight + 8) + 'px'); // the row of the box covers the top of the list: an item is scrolled below it
    try { list[i].scrollIntoView({ block: 'nearest' }); } catch (e) { /* no layout */ }
  };
  const open = (last) => {
    refresh();
    menu.hidden = false;
    btn.setAttribute('aria-expanded', 'true');
    place();
    const list = items();
    const at = last ? list.length - 1 : Math.max(0, list.findIndex((x) => x.getAttribute('aria-checked') === 'true'));
    focusItem(list, at);
  };
  const close = (back) => {
    if (menu.hidden) return;
    menu.hidden = true;
    btn.setAttribute('aria-expanded', 'false');
    if (search && search.value) { search.value = ''; applyFilter(); }
    if (back) btn.focus();
  };

  btn.addEventListener('click', (e) => { e.stopPropagation(); if (menu.hidden) open(false); else close(true); });
  btn.addEventListener('keydown', (e) => {
    if (e.key === 'ArrowDown') { e.preventDefault(); open(false); } else if (e.key === 'ArrowUp') { e.preventDefault(); open(true); }
  });
  bulb.addEventListener('click', () => { L.setTheme(L.theme() === 'light' ? 'dark' : 'light'); });
  menu.addEventListener('click', (e) => {
    const it = e.target && e.target.closest ? e.target.closest('[role="menuitemradio"]') : null;
    if (!it) return;
    pick(it.getAttribute('data-thgroup'), it.getAttribute('data-thvalue'));
    it.focus();
  });
  if (search) search.addEventListener('input', applyFilter);
  if (clear) clear.addEventListener('click', () => { search.value = ''; applyFilter(); search.focus(); });
  menu.addEventListener('keydown', (e) => {
    const k = e.key;
    if (search && e.target === search) {
      // in the filter box the keys type; the arrows leave it, Enter picks nothing, Escape clears it first
      const around = ring();
      const at = around.indexOf(search);
      if (k === 'ArrowDown' || k === 'ArrowUp') { e.preventDefault(); focusItem(around, (at + (k === 'ArrowDown' ? 1 : -1) + around.length) % around.length); }
      else if (k === 'Enter') {
        e.preventDefault(); // Enter picks nothing: it moves to the first match
        const first = allItems().find((it) => it.getAttribute('data-thgroup') === 'palette' && it.getAttribute('data-thvalue') !== 'none' && !it.hidden && it.getAttribute('data-thonly') !== 'true');
        if (first) focusItem(ring(), ring().indexOf(first));
      }
      else if (k === 'Escape') {
        e.preventDefault();
        e.stopPropagation();
        if (search.value) { search.value = ''; applyFilter(); } else close(true);
      } else if (k === 'Tab') close(false);
      return;
    }
    const list = ring();
    const i = list.indexOf(doc.activeElement);
    if (k === 'ArrowDown' || k === 'ArrowUp' || k === 'Home' || k === 'End') {
      e.preventDefault();
      focusItem(list, k === 'Home' ? 0 : k === 'End' ? list.length - 1 : (i + (k === 'ArrowDown' ? 1 : -1) + list.length) % list.length);
    } else if (k === 'Enter' || k === ' ') {
      e.preventDefault();
      if (list[i] && list[i] !== search) pick(list[i].getAttribute('data-thgroup'), list[i].getAttribute('data-thvalue'));
    } else if (k === 'Escape') {
      e.preventDefault();
      e.stopPropagation();
      close(true);
    } else if (k === 'Tab') close(false);
    else if (k.length === 1 && /[a-z0-9]/i.test(k) && !e.ctrlKey && !e.metaKey && !e.altKey && search && doc.activeElement && doc.activeElement.getAttribute('data-thgroup') === 'palette') search.focus(); // the letter goes on into the box
  });
  doc.addEventListener('click', (e) => { if (!menu.hidden && !menu.contains(e.target) && e.target !== btn) close(false); });
  doc.addEventListener('keydown', (e) => { if (e.key === 'Escape') close(true); });
  root.addEventListener('resize', () => { if (!menu.hidden) place(); });
  doc.addEventListener('taulook', refresh);
  doc.addEventListener('taumotion', refresh);
  refresh();
})(window);
