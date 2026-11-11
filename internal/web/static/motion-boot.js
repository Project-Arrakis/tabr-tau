// The Motion setting (#571): Full, Calm or Off, read before the first paint.
// This is a file in the head of the page, not an inline script (the content security policy blocks those). It runs before the stylesheets are used and before any other script, and it
// writes the choice on the root element, where the design module's motion.css reads it:
//   html[data-motion="full" | "calm" | "off"]   what the page does; when nothing is chosen: Calm if the system asks for reduced motion, else Full
//   html[data-motion-src="user"]                set only when the person chose a value: that choice then wins over the system setting
//   html[data-doc-hidden]                       set while the document is hidden: motion.css pauses every running animation, and the page's own loops stop
// The choice is kept in one key, tauMotion, holding one of the three words. Anything else in it is ignored: it is read against the list and never used as it is.
// Every read and write of storage is in try/catch: where storage is off or throws, the setting lasts until the page is closed.
(function (root) {
  'use strict';
  const KEY = 'tauMotion';
  const VALUES = ['full', 'calm', 'off'];
  const FRAME_MS = 33; // a decorative loop that the page draws itself runs at most 30 frames a second
  const doc = root.document;
  const html = doc.documentElement;

  // what the page does until the person chooses: the system's own wish ("reduce motion" asks for Calm), else Full
  const defaultMode = () => {
    try { return root.matchMedia('(prefers-reduced-motion: reduce)').matches ? 'calm' : 'full'; } catch (e) { return 'full'; /* no media query: nothing asks for less */ }
  };
  const stored = () => {
    try {
      const v = root.localStorage.getItem(KEY);
      return VALUES.indexOf(v) >= 0 ? v : null;
    } catch (e) { return null; /* storage is off: nothing was chosen */ }
  };
  // what the page does now, read from the attribute (the single truth for every script and for the stylesheet)
  const mode = () => {
    const v = html.getAttribute('data-motion');
    return VALUES.indexOf(v) >= 0 ? v : 'full'; // a page that never ran this script has no limit of its own: the module's rules are the full ones
  };
  const picked = () => html.getAttribute('data-motion-src') === 'user';
  const apply = (pick) => {
    html.setAttribute('data-motion', pick || defaultMode());
    if (pick) html.setAttribute('data-motion-src', 'user'); else html.removeAttribute('data-motion-src');
  };
  const announce = () => {
    try { doc.dispatchEvent(new root.CustomEvent('taumotion', { detail: { mode: mode(), picked: picked() } })); } catch (e) { /* no listener can be told: the next draw reads the attribute */ }
  };
  // The person chose: keep it and use it.
  const set = (v) => {
    if (VALUES.indexOf(v) < 0) return false;
    try { root.localStorage.setItem(KEY, v); } catch (e) { /* storage is off: the choice lasts until the page is closed */ }
    apply(v);
    announce();
    return true;
  };

  apply(stored());
  if (doc.hidden) html.setAttribute('data-doc-hidden', '');
  doc.addEventListener('visibilitychange', () => {
    if (doc.hidden) html.setAttribute('data-doc-hidden', ''); else html.removeAttribute('data-doc-hidden');
  });
  root.addEventListener('storage', (e) => { // another tab of the editor chose
    if (e.key !== KEY && e.key !== null) return;
    apply(stored());
    announce();
  });

  try { // the system's wish changes while nothing is chosen: follow it
    const mq = root.matchMedia('(prefers-reduced-motion: reduce)');
    if (mq && mq.addEventListener) mq.addEventListener('change', () => { if (!picked()) { apply(null); announce(); } });
  } catch (e) { /* no media query: the default stays */ }

  root.TauMotion = Object.freeze({
    KEY, VALUES, FRAME_MS, defaultMode, mode, picked, set,
    // decorative loops (a shimmer, a pulse that repeats): Full only, and never while the document is hidden
    loops: () => mode() === 'full' && !doc.hidden,
    // things that ease from one state to the next (a highlight that fades, a check that is drawn): Full and Calm; Off makes every change instant
    eases: () => mode() !== 'off',
    // one-shot attention marks (a count that bumps, a row that lights up): Full only
    marks: () => mode() === 'full',
  });
})(window);
