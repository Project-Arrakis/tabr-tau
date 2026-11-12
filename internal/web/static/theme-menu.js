// The Theme menu and the bulb in the header of the editor (#570, slice 3; Colours: #587).
// look-boot.js (in the head) keeps and applies the choices; motion-boot.js keeps the Motion setting; this file is only the control. It builds, in #theme-host:
//   the bulb          a button that switches between Dark and Dayside (the page follows the system until it is pressed once)
//   the Theme button  opens a menu of five groups, each a list of radio items: Look (Fremen, Awakening, Sietch), Dark / Dayside (follow the system, Dark, Dayside), Colours (None, then the 18
//                     palettes of the design module under their group headings), Font (System, Dune Rise: headings) and Motion (Full, Calm, Off)
// A palette applies in its own theme only (16 are dark, 2 are Dayside palettes). One that is not in effect in the theme shown now stays pickable and says so: its item carries the words "Dayside only"
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
  // the Colours group: None, then each group of the module under its heading, in the module's order
  const buildPalettes = (box, headId) => {
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
  note.textContent = 'A palette applies in its own theme only: 16 are dark, 2 are Dayside palettes (Fremen Sietch, Mentat).';
  menu.appendChild(note);
  host.appendChild(bulb);
  host.appendChild(btn);
  host.appendChild(menu);

  const items = () => Array.prototype.slice.call(menu.querySelectorAll('[role="menuitemradio"]'));
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
    items().forEach((it) => {
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
  menu.addEventListener('keydown', (e) => {
    const list = items();
    const i = list.indexOf(doc.activeElement);
    const k = e.key;
    if (k === 'ArrowDown' || k === 'ArrowUp' || k === 'Home' || k === 'End') {
      e.preventDefault();
      focusItem(list, k === 'Home' ? 0 : k === 'End' ? list.length - 1 : (i + (k === 'ArrowDown' ? 1 : -1) + list.length) % list.length);
    } else if (k === 'Enter' || k === ' ') {
      e.preventDefault();
      if (list[i]) pick(list[i].getAttribute('data-thgroup'), list[i].getAttribute('data-thvalue'));
    } else if (k === 'Escape') {
      e.preventDefault();
      e.stopPropagation();
      close(true);
    } else if (k === 'Tab') close(false);
  });
  doc.addEventListener('click', (e) => { if (!menu.hidden && !menu.contains(e.target) && e.target !== btn) close(false); });
  doc.addEventListener('keydown', (e) => { if (e.key === 'Escape') close(true); });
  root.addEventListener('resize', () => { if (!menu.hidden) place(); });
  doc.addEventListener('taulook', refresh);
  doc.addEventListener('taumotion', refresh);
  refresh();
})(window);
