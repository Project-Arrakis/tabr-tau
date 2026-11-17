// FIXTURE / DEMO ONLY. The component gallery of the Command Center's visual foundation (milestone M2).
// Everything in the FIX table below is made up for the preview: it is not read from a save, it is never sent anywhere, and nothing in the editor imports this file.
// The page builds its parts with the same helpers the editor will use (internal/web/static/lcc/lcc.js) and adds nothing of its own to them.
(function () {
  'use strict';
  const L = window.LCC;
  L.config({ artBase: '../../../../internal/web/static/lcc/art/' });
  const doc = document, html = doc.documentElement;
  const el = (tag, cls, text, attrs) => {
    const e = doc.createElement(tag);
    if (cls) e.className = cls;
    if (text != null && text !== '') e.textContent = text;
    if (attrs) for (const k of Object.keys(attrs)) e.setAttribute(k, attrs[k]);
    return e;
  };

  // ---- the fixed lists the controls choose from (a value from the address is only ever compared with these) ----
  const LOOKS = { fremen: ['tokens-fremen.css', 'components-fremen.css'], awakening: ['tokens-awakening.css', 'components-awakening.css'], sietch: ['tokens-sietch.css', 'components-sietch.css'] };
  const THEMES = ['system', 'dark', 'light'];
  const PALETTES = ['none', 'warm-charcoal', 'navy-blue', 'cold-slate', 'sand-linen'];
  const MOTIONS = ['full', 'calm', 'off'];
  const DESIGN = '../../../../design/assets/';

  // ---- the fixtures ----
  const FIX = {
    plaques: [
      { id: 'a12h4', label: 'Atreides 12, Harkonnen 4 (9 open)', a: 12, h: 4, holder: 'h' },
      { id: 'a13h12', label: 'Atreides 13, Harkonnen 12 (every seat taken, the 13th is Atreides)', a: 13, h: 12, holder: 'a' },
      { id: 'a12h13', label: 'Atreides 12, Harkonnen 13 (every seat taken, the 13th is Harkonnen)', a: 12, h: 13, holder: 'h' },
      { id: 'a6h10', label: 'Atreides 6, Harkonnen 10 (9 open)', a: 6, h: 10, holder: null },
      { id: 'a0h0', label: 'No House taken (the gold seat stays gold)', a: 0, h: 0, holder: null },
      { id: 'bad', label: 'Counts that cannot be read (shown as unavailable, nothing lit)', a: 30, h: 2, holder: null },
    ],
    decree: { name: 'Fixture decree: demo wording', effect: 'Demo effect line, not a game value' },
    contributors: { a: ['Fixture guild 1', 'Fixture guild 2', 'Fixture guild 3', 'Fixture guild 4', 'Fixture guild 5'], h: ['Fixture guild A', 'Fixture guild B', 'Fixture guild C', 'Fixture guild D', 'Fixture guild E'] },
    lanes: [
      { track: 'crafting', level: 0, xp: 0, ks: 0, note: 'Level 0, keystones 0 of 41' },
      { track: 'gathering', level: 0, xp: 0, ks: 41, note: 'Level 0, keystones 41 of 41' },
      { track: 'exploration', level: 60, xp: 21000, ks: 0, note: 'Level 60, keystones 0 of 41' },
      { track: 'combat', level: 60, xp: 21000, ks: 41, note: 'Level 60, keystones 41 of 41' },
      { track: 'sabotage', level: 100, xp: 44182, ks: 0, note: 'Level 100 (XP cap), keystones 0 of 41' },
      { track: 'crafting', level: 100, xp: 44182, ks: 41, note: 'Level 100 and keystones 41 of 41' },
    ],
    houses: {
      states: [
        { name: 'DA_HouseAlexin', owner: 'open', track: 'gathering', label: 'Open' },
        { name: 'DA_HouseEcaz', owner: 'a', track: 'combat', label: 'Atreides claimed' },
        { name: 'DA_HouseHagal', owner: 'h', track: 'crafting', label: 'Harkonnen claimed' },
        { name: 'DA_HouseKenola', owner: 'open', track: 'sabotage', label: 'One away from a line', oneAway: true },
        { name: 'DA_HouseMaros', owner: 'a', track: 'exploration', label: 'Selected, 2 rewards waiting', selected: true, waiting: 2 },
        { name: 'DA_HouseSor', owner: 'unknown', track: null, label: 'Owner unreadable' },
      ],
      line: ['DA_HouseArgosaz', 'DA_HouseDyvetz', 'DA_HouseHurata', 'DA_HouseImota', 'DA_HouseLindaren'],
      dominion: ['DA_HouseMikkarol', 'DA_HouseMoritani', 'DA_HouseMutelli', 'DA_HouseNovebruns', 'DA_HouseRichese', 'DA_HouseSpinette'],
    },
    decreeIds: ['ExperienceRateIncrease', 'RangedDamageIncreased', 'MeleeDamageIncreased', 'CraftingCostReduced', 'DropInventoryOnDefeatActive', 'RepairAndRefiningTimes', 'SpecialVendorActive_Vehicles', 'SpecialVendorActive_Weapons', 'SpecialVendorActive_Armor', 'SpecialVendorActive_Utilities', 'SpecialVendorActive', 'NotADecreeId'],
  };

  // ---- the page ----
  const root = doc.getElementById('gallery');
  const params = new URLSearchParams(location.search);
  const pick = (key, list, def) => { const v = params.get(key); return list.indexOf(v) >= 0 ? v : def; };
  const lookKeys = Object.keys(LOOKS);
  const state = { look: pick('look', lookKeys, 'fremen'), theme: pick('theme', THEMES, 'system'), palette: pick('palette', PALETTES, 'none'), motion: pick('motion', MOTIONS, 'full'), scene: params.get('scene') || 'all' };

  function apply() {
    const links = doc.querySelectorAll('link[data-look-link]');
    links[0].setAttribute('href', DESIGN + LOOKS[state.look][0]);
    links[1].setAttribute('href', DESIGN + LOOKS[state.look][1]);
    html.setAttribute('data-look', state.look);
    if (state.theme === 'system') html.removeAttribute('data-theme'); else html.setAttribute('data-theme', state.theme);
    if (state.palette === 'none') html.removeAttribute('data-palette'); else html.setAttribute('data-palette', state.palette);
    html.setAttribute('data-motion', state.motion);
    html.setAttribute('data-motion-src', 'user');
  }

  function controls() {
    const f = doc.getElementById('controls');
    f.addEventListener('submit', (e) => e.preventDefault());
    const add = (key, label, list) => {
      const w = el('label', 'lccg-ctl');
      w.appendChild(el('span', '', label));
      const s = el('select');
      for (const v of list) { const o = el('option', '', v, { value: v }); if (state[key] === v) o.selected = true; s.appendChild(o); }
      s.addEventListener('change', () => { state[key] = s.value; apply(); });
      w.appendChild(s);
      f.appendChild(w);
    };
    add('look', 'Look', lookKeys); add('theme', 'Theme', THEMES); add('palette', 'Palette', PALETTES); add('motion', 'Motion', MOTIONS);
    f.appendChild(el('span', 'lccg-hint', 'Forced colours and reduced motion follow the browser (the preview adds no switch of its own for them).'));
  }

  function section(id, title, intro) {
    const s = el('section', 'lcc-scope lccg-sec', '', { 'aria-labelledby': 'h-' + id, 'data-scene': id });
    const h = el('h2', '', title, { id: 'h-' + id });
    h.appendChild(doc.createTextNode(' '));
    h.appendChild(L.badge('demo'));
    s.appendChild(h);
    if (intro) s.appendChild(el('p', 'lccg-intro', intro));
    return s;
  }
  const want = (id) => state.scene === 'all' || state.scene === id;
  const wantPlaque = (id) => state.scene === 'plaques' || state.scene === 'all' || state.scene === 'plaque-' + id;

  function plaques() {
    if (!(want('plaques') || FIX.plaques.some((p) => wantPlaque(p.id)))) return;
    const s = section('plaques', 'Decree and voting plaque', 'Drawn at the width of its box: the approved artwork above 900 px, a deliberate narrow layout below it. Seats fill from the left for Atreides and from the right for Harkonnen; the 13th seat is always the gold middle seat.');
    for (const p of FIX.plaques) {
      if (!wantPlaque(p.id)) continue;
      const fig = el('figure', 'lccg-fig', '', { 'data-case': p.id });
      fig.appendChild(el('figcaption', '', p.label));
      fig.appendChild(L.renderPlaque({ a: p.a, h: p.h, holder: p.holder, decree: p.id === 'a0h0' ? null : FIX.decree, contributors: p.id === 'a13h12' ? FIX.contributors : null, demo: true }));
      s.appendChild(fig);
    }
    root.appendChild(s);
  }

  function lanes() {
    if (!want('lanes')) return;
    const s = section('lanes', 'Specialization lanes', 'The XP cap (level 100) and the keystones are two separate facts with two separate tags. The five scroll icons are the game’s own pictures. Fixture values, not the game’s level table.');
    const g = el('div', 'lccg-grid lccg-grid-lanes');
    FIX.lanes.forEach((l, i) => {
      const w = el('figure', 'lccg-cell');
      w.appendChild(L.renderLaneCard({ track: l.track, level: l.level, maxLevel: 100, xp: l.xp, maxXp: 44182, keystonesOwned: l.ks, keystonesTotal: 41, selected: i === 4 }));
      w.appendChild(el('figcaption', '', l.note + (i === 4 ? ' (selected)' : '')));
      g.appendChild(w);
    });
    const un = el('figure', 'lccg-cell');
    un.appendChild(L.renderLaneCard({ track: 'gathering', level: 12, maxLevel: 100, xp: 900, maxXp: 44182, keystonesOwned: 0, keystonesTotal: 0, unresolved: true }));
    un.appendChild(el('figcaption', '', 'Track not identified, keystone total unknown'));
    g.appendChild(un);
    s.appendChild(g);
    root.appendChild(s);
  }

  function houses() {
    if (!want('houses')) return;
    const s = section('houses', 'House cards', 'State is always text as well as colour and shape. Pictures: the game’s House symbols and the path banner of each House.');
    const g = el('div', 'lccg-grid lccg-grid-houses');
    for (const h of FIX.houses.states) {
      const w = el('figure', 'lccg-cell');
      w.appendChild(L.renderHouseCard(h));
      w.appendChild(el('figcaption', '', h.label));
      g.appendChild(w);
    }
    s.appendChild(g);
    s.appendChild(el('h3', '', 'Five in a row: one outline on each of five connected Houses'));
    const tracks = ['combat', 'crafting', 'exploration', 'gathering', 'sabotage'];
    const line = el('div', 'lccg-row5', '', { role: 'group', 'aria-label': 'A line of five Houses (fixture)' });
    FIX.houses.line.forEach((n, i) => line.appendChild(L.renderHouseCard({ name: n, owner: 'a', inLine: true, track: tracks[i] })));
    s.appendChild(line);
    s.appendChild(el('h3', '', 'Majority: every House of the winning side double-bordered, not connected'));
    const dom = el('div', 'lccg-grid lccg-grid-houses', '', { role: 'group', 'aria-label': 'Majority Houses (fixture)' });
    FIX.houses.dominion.forEach((n, i) => dom.appendChild(L.renderHouseCard({ name: n, owner: 'h', dominion: true, track: tracks[i % 5] })));
    s.appendChild(dom);
    root.appendChild(s);
  }

  function primitives() {
    if (!want('primitives')) return;
    const s = section('primitives', 'Primitives', 'The small parts the cards and the plaque are made of.');
    const g = el('div', 'lccg-grid lccg-grid-prim');
    const cell = (cap, node) => { const w = el('figure', 'lccg-cell'); w.appendChild(node); w.appendChild(el('figcaption', '', cap)); g.appendChild(w); };
    cell('Cut-corner gold frame', L.frame([el('p', '', 'Panel content inside the frame.')]));
    cell('Gold nameplate', L.pill('House Atreides'));
    cell('Gold nameplate', L.pill('House Harkonnen'));
    const held = el('div', 'lccg-heldcell');
    held.appendChild(L.heldBy('HOUSE HARKONNEN', 'h'));
    held.appendChild(L.ruleV());
    held.appendChild(L.heldBy('HOUSE ATREIDES', 'a'));
    cell('HELD BY with the single vertical rule', held);
    cell('Atreides standard (empty state)', L.standard('a', null));
    cell('Harkonnen standard (fixture list)', L.standard('h', FIX.contributors.h));
    const badges = el('div', 'lccg-badges');
    for (const k of ['demo', 'readonly', 'ref']) badges.appendChild(L.badge(k));
    cell('Badges', badges);
    cell('Reachable tooltip (hover or focus)', L.richTip('What is this?', 'A tooltip is real text beside its trigger, reachable with the keyboard.'));
    s.appendChild(g);
    root.appendChild(s);
  }

  function art() {
    if (!want('art')) return;
    const s = section('art', 'Art inventory', 'Every picture below comes from a fixed list; the page builds no file name from a name. Game artwork that belongs to Funcom and is outside this repository’s licence (docs/art-notice.md).');
    const grid = (cls) => el('div', 'lccg-art ' + cls);
    const tile = (src, cap, cls) => {
      const f = el('figure', 'lccg-tile ' + (cls || ''));
      f.appendChild(el('img', '', '', { alt: '', width: '56', height: '56', src }));
      f.appendChild(el('figcaption', '', cap));
      return f;
    };
    s.appendChild(el('h3', '', 'Houses (25), with the spelling alias'));
    const hg = grid('lccg-art-h');
    for (const k of L.HOUSES) hg.appendChild(tile(L.houseAsset(k).src, k));
    hg.appendChild(tile(L.houseAsset('Mikkarol').src, 'save: Mikkarol'));
    hg.appendChild(tile(L.houseAsset('mikarrol').src, 'reference: mikarrol'));
    s.appendChild(hg);
    s.appendChild(el('h3', '', 'Lane scrolls (Specializations) and path banners (Landsraad)'));
    const lg = grid('lccg-art-l');
    for (const k of L.TRACKS) { lg.appendChild(tile(L.laneAsset(k).src, 'lane ' + k, 'big')); lg.appendChild(tile(L.bannerAsset(k).src, 'banner ' + k, 'big')); }
    s.appendChild(lg);
    s.appendChild(el('h3', '', 'Decree pictures by decree id (the last two use the neutral picture)'));
    const dg = grid('lccg-art-d');
    for (const id of FIX.decreeIds) { const a = L.decreeAsset(id); dg.appendChild(tile(a.src, id + (a.fallback ? ' (neutral)' : ''), a.fallback ? 'fb' : '')); }
    s.appendChild(dg);
    root.appendChild(s);
  }

  controls();
  apply();
  plaques(); lanes(); houses(); primitives(); art();
  html.setAttribute('data-gallery-ready', 'true');
})();
