// The Look, the Dark / Dayside choice and the heading font (#570, slice 3), read before the first paint.
// This is a file in the head of the page, not an inline script (the content security policy blocks those). It comes after the stylesheets of the default look, so that it can swap the first two
// for those of another look before anything is drawn, and it writes the choices on the root element, where the design module's stylesheets read them:
//   html[data-look="fremen" | "awakening" | "sietch"]      the look; Fremen when nothing is chosen. The two look links are swapped to match
//   html[data-theme="dark" | "light"]                       set only when the person chose; without it the page follows the system's dark / light setting (light is called Dayside)
//   html[data-heading-font="dune-rise" | "system"]          the face of the headings; Dune Rise when nothing is chosen
// The choices are kept in three keys, tauLook, tauTheme and tauFont, each holding one word of a fixed list. Anything else in a key is ignored: a stored string is only ever compared with the
// list, never used as it is (not as a file name, a selector or markup). The file names come from the table below, which is the design module's own (design.LookFiles).
// Every read and write of storage is in try/catch: where storage is off or throws, a choice lasts until the page is closed.
(function (root) {
  'use strict';
  const doc = root.document;
  const html = doc.documentElement;
  const KEYS = Object.freeze({ look: 'tauLook', theme: 'tauTheme', font: 'tauFont' });
  const LOOKS = ['fremen', 'awakening', 'sietch'];
  const THEMES = ['dark', 'light'];
  const FONTS = ['dune-rise', 'system'];
  const DEFAULT_LOOK = 'fremen';
  const DEFAULT_FONT = 'dune-rise';
  // the two stylesheets of each look that change with it (the module's components.css and motion.css are the same for every look)
  const FILES = {
    fremen: ['/design/tokens-fremen.css', '/design/components-fremen.css'],
    awakening: ['/design/tokens-awakening.css', '/design/components-awakening.css'],
    sietch: ['/design/tokens-sietch.css', '/design/components-sietch.css'],
  };
  // the two links are found by the start of their href (the module's tokens-LOOK.css and components-LOOK.css; components.css has no dash); a link that is loading its replacement carries data-look-next
  const SELECTORS = ['link[rel="stylesheet"][href^="/design/tokens-"]:not([data-look-next])', 'link[rel="stylesheet"][href^="/design/components-"]:not([data-look-next])'];

  const stored = (key, list) => {
    try {
      const v = root.localStorage.getItem(key);
      return list.indexOf(v) >= 0 ? v : null;
    } catch (e) { return null; /* storage is off: nothing was chosen */ }
  };
  const keep = (key, value) => {
    try { root.localStorage.setItem(key, value); } catch (e) { /* storage is off: the choice lasts until the page is closed */ }
  };
  const forget = (key) => {
    try { root.localStorage.removeItem(key); } catch (e) { /* storage is off */ }
  };
  const announce = () => {
    try { doc.dispatchEvent(new root.CustomEvent('taulook', { detail: { look: look(), theme: theme(), picked: themePicked(), font: font() } })); } catch (e) { /* no listener can be told: the next read gets the attribute */ }
  };

  // what the page shows now, from the attributes (the single truth for every script and for the stylesheets)
  const look = () => { const v = html.getAttribute('data-look'); return LOOKS.indexOf(v) >= 0 ? v : DEFAULT_LOOK; };
  const font = () => { const v = html.getAttribute('data-heading-font'); return FONTS.indexOf(v) >= 0 ? v : DEFAULT_FONT; };
  const themePicked = () => THEMES.indexOf(html.getAttribute('data-theme')) >= 0;
  const systemLight = () => { try { return !!(root.matchMedia && root.matchMedia('(prefers-color-scheme: light)').matches); } catch (e) { return false; } };
  const theme = () => (themePicked() ? html.getAttribute('data-theme') : systemLight() ? 'light' : 'dark'); // the theme in use, picked or by the system

  // Before the first paint the two parser-inserted links are changed where they stand (the page waits for them). While the page is open a new link is put in beside the old one and the old
  // one goes when the new one has loaded, so that nothing is ever drawn without a look.
  const links = () => SELECTORS.map((q) => doc.querySelector(q));
  const swapLinks = (name, early) => {
    const files = FILES[name];
    links().forEach((link, i) => {
      if (!link) return;
      const stale = link.nextElementSibling; // a switch that is still loading is dropped for this one
      if (stale && stale.hasAttribute('data-look-next')) stale.remove();
      if (link.getAttribute('href') === files[i]) return;
      if (early) { link.setAttribute('href', files[i]); return; }
      const next = doc.createElement('link');
      next.rel = 'stylesheet';
      next.setAttribute('data-look-next', '');
      next.setAttribute('href', files[i]);
      const done = () => { link.remove(); next.removeAttribute('data-look-next'); };
      next.addEventListener('load', done);
      next.addEventListener('error', () => next.remove()); // the old look stays when the new one cannot load
      link.parentNode.insertBefore(next, link.nextSibling);
    });
  };
  const applyLook = (name, early) => {
    const v = LOOKS.indexOf(name) >= 0 ? name : DEFAULT_LOOK;
    html.setAttribute('data-look', v);
    swapLinks(v, early);
  };
  const applyTheme = (name) => {
    if (THEMES.indexOf(name) >= 0) html.setAttribute('data-theme', name); else html.removeAttribute('data-theme');
  };
  const applyFont = (name) => { html.setAttribute('data-heading-font', FONTS.indexOf(name) >= 0 ? name : DEFAULT_FONT); };

  // the person chose: keep it and use it. Another window of the editor is told by the storage event below.
  const setLook = (v) => {
    if (LOOKS.indexOf(v) < 0) return false;
    keep(KEYS.look, v);
    applyLook(v, false);
    announce();
    return true;
  };
  const setTheme = (v) => {
    if (THEMES.indexOf(v) < 0) return false;
    keep(KEYS.theme, v);
    applyTheme(v);
    announce();
    return true;
  };
  const setFont = (v) => {
    if (FONTS.indexOf(v) < 0) return false;
    keep(KEYS.font, v);
    applyFont(v);
    announce();
    return true;
  };
  // back to what the system gives: no theme chosen
  const followSystem = () => { forget(KEYS.theme); applyTheme(null); announce(); };

  applyLook(stored(KEYS.look, LOOKS), true);
  applyTheme(stored(KEYS.theme, THEMES));
  applyFont(stored(KEYS.font, FONTS));
  root.addEventListener('storage', (e) => {
    if (e.key !== null && e.key !== KEYS.look && e.key !== KEYS.theme && e.key !== KEYS.font) return;
    applyLook(stored(KEYS.look, LOOKS), false);
    applyTheme(stored(KEYS.theme, THEMES));
    applyFont(stored(KEYS.font, FONTS));
    announce();
  });
  try { root.matchMedia('(prefers-color-scheme: light)').addEventListener('change', announce); } catch (e) { /* no matchMedia: the bulb is read when it is pressed */ }

  root.TauLook = Object.freeze({
    KEYS, LOOKS, THEMES, FONTS, DEFAULT_LOOK, DEFAULT_FONT, FILES,
    look, theme, themePicked, font, setLook, setTheme, setFont, followSystem,
  });
})(window);
