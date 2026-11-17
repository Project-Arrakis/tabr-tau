// The Command Center's visual foundation (milestone M2): art lookups from fixed lists, the 25-seat model of the voting track, and the rendering helpers for the
// plaque, the House card, the specialization lane card and the small parts they share.
//
// What this file is not: it holds no demo data, it fetches nothing, it writes nothing to the save and it is not linked from any live page yet. Every helper takes plain values
// and returns an element built with createElement / createElementNS and textContent (never a string of HTML), so a House name or a decree name from a save cannot become markup.
// The picture of a House, a lane, a banner or a decree is looked up in a fixed table by its key; a file name is never built from a name, so a strange name can only miss the table.
// Everything is served from this origin (the content security policy allows nothing else) and every picture has a text beside it, so a missing picture loses nothing.
(function (root) {
  'use strict';

  // ---- art: fixed lists ----------------------------------------------------------------------------------------------------------------------------------------------------------
  // The 25 Houses by the key the save's name gives once "DA_House" is taken off and the rest is lower-cased. The save and the editor spell one House Mikkarol; the reference
  // picture and the public trackers spell it Mikarrol. Both spellings find the one file, through the alias table below and nowhere else.
  const HOUSES = Object.freeze(['alexin', 'argosaz', 'dyvetz', 'ecaz', 'hagal', 'hurata', 'imota', 'kenola', 'lindaren', 'maros', 'mikkarol', 'moritani', 'mutelli', 'novebruns', 'richese', 'sor', 'spinette', 'taligari', 'thorvald', 'tseida', 'varota', 'vernius', 'wallach', 'wayku', 'wydras']);
  const HOUSE_ALIAS = Object.freeze({ mikarrol: 'mikkarol' });
  const TRACKS = Object.freeze(['combat', 'crafting', 'exploration', 'gathering', 'sabotage']);
  // The game's decree id (landsraad_decrees.decree_name) -> the picture that shows what the decree gives. Matched by hand against the editor's decree table: "name" means the picture is
  // named like the decree, "subject" means the picture shows what the decree is about. A decree that is not here (the plain SpecialVendorActive row has no picture of its own, and an id
  // the editor has never seen) gets the neutral picture, never a guessed one.
  const DECREE_ART = Object.freeze({
    experiencerateincrease: 'scholarly-collaboration',
    rangeddamageincreased: 'honor-of-the-fallen',
    meleedamageincreased: 'honor-of-the-fallen',
    craftingcostreduced: 'production-contract',
    dropinventoryondefeatactive: 'loot',
    repairandrefiningtimes: 'repair-cost-reduction',
    specialvendoractive_vehicles: 'vendor-vehicles',
    specialvendoractive_weapons: 'vendor-weapons',
    specialvendoractive_armor: 'vendor-armor',
    specialvendoractive_utilities: 'vendor-utilities',
  });
  const DECREE_FILES = Object.freeze(['scholarly-collaboration', 'honor-of-the-fallen', 'production-contract', 'repair-cost-reduction', 'loot', 'vendor-armor', 'vendor-utilities', 'vendor-vehicles', 'vendor-weapons']);
  const DECREE_FALLBACK = 'fallback';

  // Where the pictures are served from. The page that uses the helpers may move it (the preview page reads the files from the repository); it must be a relative or same-origin path that
  // ends in a slash, never an address with a scheme, so no helper can be pointed at another host.
  let artBase = '/lcc/art/';
  const BASE_OK = /^[A-Za-z0-9._/~-]*\/$/;
  function config(opts) {
    if (opts && typeof opts.artBase === 'string') {
      if (!BASE_OK.test(opts.artBase) || opts.artBase.indexOf('//') >= 0) throw new TypeError('artBase must be a path on this origin ending in a slash');
      artBase = opts.artBase;
    }
    return { artBase };
  }

  const KEY_OK = /^[a-z][a-z0-9_]{1,40}$/;
  const norm = (v) => (typeof v === 'string' ? v.trim().toLowerCase() : '');
  const hit = (folder, file, key) => Object.freeze({ key, file: folder + '/' + file + '.webp', src: artBase + folder + '/' + file + '.webp', fallback: false });

  // houseAsset('DA_HouseEcaz') / ('Mikkarol') / ('mikarrol') -> { key, file, src } or null. The prefix the save puts on the name is taken off, the rest is looked up, and nothing else is read.
  function houseAsset(name) {
    let k = norm(name).replace(/^da_house/, '');
    if (!KEY_OK.test(k)) return null;
    if (Object.prototype.hasOwnProperty.call(HOUSE_ALIAS, k)) k = HOUSE_ALIAS[k];
    return HOUSES.indexOf(k) >= 0 ? hit('houses', k, k) : null;
  }
  const trackAsset = (folder, track) => { const k = norm(track); return KEY_OK.test(k) && TRACKS.indexOf(k) >= 0 ? hit(folder, k, k) : null; };
  const laneAsset = (track) => trackAsset('lanes', track); // the scroll icon of a specialization: the Specializations view
  const bannerAsset = (track) => trackAsset('banners', track); // the path banner icon: the Landsraad view, beside a House
  // decreeAsset(id) always answers: the picture of a known decree, or the neutral picture marked fallback: true.
  function decreeAsset(id) {
    const k = norm(typeof id === 'number' ? '' : id);
    if (KEY_OK.test(k) && Object.prototype.hasOwnProperty.call(DECREE_ART, k)) { const f = DECREE_ART[k]; return hit('decrees', f, f); }
    return Object.freeze({ key: null, file: 'decrees/' + DECREE_FALLBACK + '.svg', src: artBase + 'decrees/' + DECREE_FALLBACK + '.svg', fallback: true });
  }

  // ---- the 25 seats --------------------------------------------------------------------------------------------------------------------------------------------------------------
  const SEATS = 25;
  const CENTER = 12; // index of the 13th seat (zero based), the gold one
  const isCount = (n) => Number.isInteger(n) && n >= 0 && n <= SEATS;
  // seatStates(a, h): the tally of Houses as 25 seats. Atreides fill from the left, Harkonnen from the right, the rest is open; the 13th seat is always the gold one. Counts that are not
  // whole numbers, or that do not fit in 25 together, give null (the track says it is unavailable and lights nothing).
  function seatStates(a, h) {
    if (!isCount(a) || !isCount(h) || a + h > SEATS) return null;
    const out = [];
    for (let i = 0; i < SEATS; i++) out.push({ index: i, slot: i + 1, state: i < a ? 'a' : i >= SEATS - h ? 'h' : 'open', center: i === CENTER });
    return out;
  }
  const FACTION = Object.freeze({ a: 'Atreides', h: 'Harkonnen' });
  function seatLabel(s, total) {
    const who = s.state === 'a' ? FACTION.a : s.state === 'h' ? FACTION.h : 'open';
    return 'Slot ' + s.slot + ' of ' + (total || SEATS) + ' — ' + who + (s.center ? ' (the gold middle seat)' : '');
  }
  // A short factual line for the middle of the plaque. It reports the current count and never a result: a lead is "current", and the vote is not confirmed here (decision D05).
  function standingLine(a, h) {
    if (!isCount(a) || !isCount(h) || a + h > SEATS) return 'HOUSE COUNT UNAVAILABLE';
    const open = SEATS - a - h;
    const tail = open + (open === 1 ? ' HOUSE OPEN' : ' HOUSES OPEN');
    if (a === h) return 'HOUSE COUNT EVEN AT ' + a + ' · ' + tail;
    const lead = a > h ? 'ATREIDES' : 'HARKONNEN';
    const hi = Math.max(a, h), lo = Math.min(a, h);
    return lead + ' LEAD ' + hi + ' TO ' + lo + ' · ' + tail + ' · VOTE NOT CONFIRMED';
  }

  // ---- small element helpers ----------------------------------------------------------------------------------------------------------------------------------------------
  const NS = 'http://www.w3.org/2000/svg';
  const doc = () => root.document;
  function el(tag, cls, text, attrs) {
    const e = doc().createElement(tag);
    if (cls) e.className = cls;
    if (text != null && text !== '') e.textContent = String(text);
    if (attrs) for (const k of Object.keys(attrs)) if (attrs[k] != null && attrs[k] !== false) e.setAttribute(k, attrs[k] === true ? '' : String(attrs[k]));
    return e;
  }
  function sv(tag, attrs, kids) {
    const e = doc().createElementNS(NS, tag);
    if (attrs) for (const k of Object.keys(attrs)) if (attrs[k] != null) e.setAttribute(k, String(attrs[k]));
    if (kids) for (const c of kids) if (c) e.appendChild(c);
    return e;
  }
  const plus = (parent, kids) => { for (const k of kids) if (k) parent.appendChild(k); return parent; };
  let uid = 0;

  // A picture that never breaks the page: the picture has no alt (the text beside it names it), and a failed load hides the picture, so the text stands alone.
  function art(asset, cls, size) {
    const img = el('img', cls, '', { alt: '', width: size, height: size, decoding: 'async' });
    if (asset) img.setAttribute('src', asset.src);
    img.addEventListener('error', () => { img.hidden = true; });
    return img;
  }

  // ---- the small parts -----------------------------------------------------------------------------------------------------------------------------------------------------
  // A gold nameplate (the HOUSE ATREIDES plate): pointed ends, gold fill, dark letters.
  const pill = (text, cls) => el('span', 'lcc-pill' + (cls ? ' ' + cls : ''), text);
  // The vertical gold rule of the HELD BY block: one element, one continuous line, never drawn over by anything else.
  const ruleV = () => el('span', 'lcc-rule-v', '', { 'aria-hidden': 'true' });
  // HELD BY and the faction that holds the term. name is the faction's name from the term, or empty when the term gives none.
  function heldBy(name, faction) {
    const w = el('p', 'lcc-heldby');
    w.appendChild(el('span', 'lcc-heldby-l', 'HELD BY'));
    w.appendChild(el('strong', 'lcc-heldby-n' + (faction ? ' lcc-f-' + faction : ''), name || 'Not recorded'));
    return w;
  }
  // The cloth standard of a faction: a gold-edged banner with the faction's mark, a heading and an honest list (or the empty state). lines is an array of strings or null.
  function standard(faction, lines, heading, emptyText) {
    const f = faction === 'h' ? 'h' : 'a';
    const w = el('div', 'lcc-standard lcc-f-' + f);
    w.appendChild(el('span', 'lcc-standard-mark', '', { 'aria-hidden': 'true' }));
    w.appendChild(el('p', 'lcc-standard-h', heading || 'TOP CONTRIBUTORS'));
    if (Array.isArray(lines) && lines.length) {
      const ol = el('ol', 'lcc-standard-list');
      for (const s of lines.slice(0, 5)) ol.appendChild(el('li', '', s));
      w.appendChild(ol);
    } else w.appendChild(el('p', 'lcc-standard-none', emptyText || 'Not in the save'));
    return w;
  }
  // A label that says what the data is. kind: 'demo' (fixture values on a preview page), 'readonly' (shown, never written here) or 'ref' (a reference value, not the save's).
  const BADGE = Object.freeze({ demo: 'FIXTURE · DEMO DATA', readonly: 'READ-ONLY', ref: 'REFERENCE' });
  const badge = (kind, text) => el('span', 'lcc-badge lcc-badge-' + (BADGE[kind] ? kind : 'readonly'), text || BADGE[kind] || BADGE.readonly);
  // A reachable tooltip: the trigger carries aria-describedby, the tip is real text beside it (shown on hover and on focus by the stylesheet; the module's own tooltip rules do the rest).
  function richTip(triggerText, tipText) {
    const id = 'lcc-tip-' + (++uid);
    const w = el('span', 'lcc-tt');
    w.appendChild(el('button', 'lcc-tt-b', triggerText, { type: 'button', 'aria-describedby': id }));
    w.appendChild(el('span', 'lcc-tip', tipText, { role: 'tooltip', id }));
    return w;
  }
  // The cut-corner gold frame: returns the outer element; put content in .lcc-frame-in.
  function frame(content, cls) {
    const o = el('div', 'lcc-frame' + (cls ? ' ' + cls : ''));
    const i = el('div', 'lcc-frame-in');
    if (content) plus(i, [].concat(content));
    o.appendChild(i);
    return o;
  }
  // The two victory outlines, drawn on a House card by a data attribute; no evaluation here, the caller says which one applies.
  //   circuit : the card is one of the five connected Houses of a line (a single bright gold outline and a LINE badge)
  //   dominion: the card belongs to the faction that holds the House-count majority (a double border and a DOMINION badge); cards are not connected
  const VICTORY = Object.freeze({ circuit: 'LINE OF FIVE', dominion: 'MAJORITY HOUSE' });
  function victory(elm, kind) {
    if (kind === 'circuit' || kind === 'dominion') elm.setAttribute('data-victory', kind); else elm.removeAttribute('data-victory');
    return elm;
  }

  // ---- the plaque ----------------------------------------------------------------------------------------------------------------------------------------------------------------
  // The geometry is one coordinate system, 2172 by 724, the size of the approved artwork. The decorative layer and the track are drawn in it as vectors; the words are real text
  // above them. Nothing here is a picture with text in it.
  const W = 2172, H = 724;
  const TRACK = Object.freeze({ cx: W / 2, cy: 396, pitch: 52, r: 19, haloR: 30, tipL: 334, tipR: 1838, wallL: 410, wallR: 1762, top: 330, bottom: 462 });
  // x of a seat's centre. Equal pitch for all 25 seats; the 13th sits on the middle of the track, so the two ends are inset by the same amount.
  const seatX = (i) => TRACK.cx + (i - CENTER) * TRACK.pitch;
  const GOLD = Object.freeze({ hi: '#fff0b8', mid: '#e8b84e', lo: '#8e5f1c' });

  function cut(x, y, w, h, c) {
    return 'M' + (x + c) + ',' + y + 'H' + (x + w - c) + 'L' + (x + w) + ',' + (y + c) + 'V' + (y + h - c) + 'L' + (x + w - c) + ',' + (y + h) + 'H' + (x + c) + 'L' + x + ',' + (y + h - c) + 'V' + (y + c) + 'Z';
  }
  function grad(id, x1, y1, x2, y2, stops) {
    return sv('linearGradient', { id, x1, y1, x2, y2 }, stops.map((s) => sv('stop', { offset: s[0], 'stop-color': s[1], 'stop-opacity': s[2] == null ? 1 : s[2] })));
  }
  function rgrad(id, stops) {
    return sv('radialGradient', { id, cx: '38%', cy: '34%', r: '75%' }, stops.map((s) => sv('stop', { offset: s[0], 'stop-color': s[1] })));
  }
  function glow(id, dev) {
    return sv('filter', { id, x: '-10%', y: '-10%', width: '120%', height: '120%' }, [sv('feGaussianBlur', { stdDeviation: dev })]);
  }
  const strokePath = (d, stroke, w, extra) => sv('path', Object.assign({ d, fill: 'none', stroke, 'stroke-width': w, 'stroke-linejoin': 'miter' }, extra || {}));
  const mirror = (x) => W - x;

  // The faction marks: simple original vector marks, not game insignia. Atreides: a spread hawk (a V with two wings); Harkonnen: a ring with a spur.
  function markA(cx, cy, s, fill) {
    const d = 'M' + (cx - 30 * s) + ',' + (cy - 22 * s) + ' L' + (cx - 8 * s) + ',' + (cy - 6 * s) + ' L' + cx + ',' + (cy - 26 * s) + ' L' + (cx + 8 * s) + ',' + (cy - 6 * s) + ' L' + (cx + 30 * s) + ',' + (cy - 22 * s) +
      ' L' + (cx + 22 * s) + ',' + (cy + 4 * s) + ' L' + (cx + 8 * s) + ',' + (cy + 8 * s) + ' L' + cx + ',' + (cy + 28 * s) + ' L' + (cx - 8 * s) + ',' + (cy + 8 * s) + ' L' + (cx - 22 * s) + ',' + (cy + 4 * s) + ' Z';
    return sv('path', { d, fill: fill || '#efe9d6', stroke: '#0a1a14', 'stroke-width': 1.5 * s, 'stroke-linejoin': 'round' });
  }
  function markH(cx, cy, s, fill) {
    const f = fill || '#efe9d6';
    return sv('g', null, [
      sv('circle', { cx, cy: cy + 2 * s, r: 17 * s, fill: 'none', stroke: f, 'stroke-width': 7 * s }),
      sv('path', { d: 'M' + (cx - 4 * s) + ',' + (cy - 18 * s) + ' L' + (cx + 4 * s) + ',' + (cy - 18 * s) + ' L' + (cx + 26 * s) + ',' + (cy - 34 * s) + ' L' + (cx + 12 * s) + ',' + (cy - 8 * s) + ' Z', fill: f }),
      sv('path', { d: 'M' + (cx - 24 * s) + ',' + (cy + 28 * s) + ' L' + (cx - 8 * s) + ',' + (cy + 18 * s) + ' L' + (cx - 6 * s) + ',' + (cy + 30 * s) + ' Z', fill: f }),
    ]);
  }

  // The decorative layer: frame, header band, the two shields, the decree plate, the two gold rules, the two cloth standards with their rods, the nameplate shapes, the two hollow diamonds
  // with their lines, the session plate and the footer rail. It carries no text, and no dynamic value.
  function underlay(p) {
    const defs = sv('defs', null, [
      grad(p + 'gold', 0, 0, 0, 1, [[0, GOLD.hi], [0.45, GOLD.mid], [1, GOLD.lo]]),
      grad(p + 'goldh', 0, 0, 1, 0, [[0, GOLD.lo, 0], [0.18, GOLD.mid], [0.5, GOLD.hi], [0.82, GOLD.mid], [1, GOLD.lo, 0]]),
      grad(p + 'field', 0, 0, 0, 1, [[0, '#0e1030'], [0.5, '#0a0b1e'], [1, '#07070f']]),
      grad(p + 'em', 0, 0, 0, 1, [[0, '#0f6a54'], [1, '#063a2e']]),
      grad(p + 'cr', 0, 0, 0, 1, [[0, '#8c1219'], [1, '#4a070c']]),
      grad(p + 'plate', 0, 0, 0, 1, [[0, '#2a1d0c'], [1, '#120b04']]),
      grad(p + 'vr', 0, 0, 0, 1, [[0, '#f0a020', 0], [0.2, '#f6b73a'], [0.5, '#ffd36b'], [0.8, '#f6b73a'], [1, '#f0a020', 0]]),
      glow(p + 'blur', 5),
    ]);
    const g = (id) => 'url(#' + p + id + ')';
    const kids = [defs];
    kids.push(sv('rect', { x: 0, y: 0, width: W, height: H, fill: g('field') }));
    // faint geometric pattern of the field (decoration only)
    kids.push(sv('g', { fill: 'none', stroke: '#7a82e0', 'stroke-width': 1.6, opacity: 0.1 }, [
      sv('path', { d: 'M330,700 L760,200 L1086,700 L1412,200 L1842,700' }),
      sv('path', { d: 'M470,700 L840,260 L1086,560 L1332,260 L1702,700' }),
      sv('path', { d: 'M20,520 L420,200 M2152,520 L1752,200' }),
    ]));
    // outer frame, header rule, body frame, footer rail (each with a soft glow copy under it)
    for (const d of [cut(6, 6, 2160, 712, 26), 'M16,166 H2156']) kids.push(strokePath(d, GOLD.mid, 12, { opacity: 0.22, filter: g('blur') }));
    // (a straight line cannot carry a gradient that is sized by its own box, so the straight rules are thin rectangles)
    kids.push(strokePath(cut(6, 6, 2160, 712, 26), g('gold'), 5));
    kids.push(strokePath(cut(16, 16, 2140, 692, 22), GOLD.lo, 1.5, { opacity: 0.8 }));
    kids.push(sv('rect', { x: 16, y: 164.2, width: 2140, height: 3.6, fill: g('goldh') }));
    kids.push(sv('rect', { x: 70, y: 686.5, width: 2032, height: 3, fill: g('goldh') }));
    kids.push(strokePath('M16,166 V690 M2156,166 V690', GOLD.mid, 3, { opacity: 0.9 }));
    // shields (Atreides left, Harkonnen right)
    const shield = (x0, fill, mark) => {
      const d = 'M' + x0 + ',14 H' + (x0 + 156) + ' V118 L' + (x0 + 78) + ',182 L' + x0 + ',118 Z';
      return sv('g', null, [sv('path', { d, fill: g(fill) }), strokePath(d, g('gold'), 4.5), strokePath('M' + (x0 + 10) + ',24 H' + (x0 + 146) + ' V114 L' + (x0 + 78) + ',168 L' + (x0 + 10) + ',114 Z', GOLD.mid, 1.5, { opacity: 0.7 }), mark(x0 + 78, 86, 1.45)]);
    };
    kids.push(shield(50, 'em', markA));
    kids.push(shield(W - 50 - 156, 'cr', markH));
    // the ACTIVE DECREE plate and the two rules (the HELD BY rule is the second one and is a single element)
    const plate = 'M232,94 L262,60 H654 L684,94 L654,128 H262 Z';
    kids.push(sv('path', { d: plate, fill: g('plate') }), strokePath(plate, g('gold'), 3.5), strokePath('M244,94 L270,68 H646 L672,94 L646,120 H270 Z', GOLD.lo, 1.5, { opacity: 0.8 }));
    // the two cloth standards: a rod with two finials, the cloth with a pointed foot, a gold edge and an inner edge
    const standardArt = (cx, fill, mark) => {
      const l = cx - 126, r = cx + 126;
      const cloth = 'M' + (l + 2) + ',206 H' + (r - 2) + ' V486 L' + cx + ',574 L' + (l + 2) + ',486 Z';
      return sv('g', null, [
        sv('rect', { x: l - 20, y: 189, width: 292, height: 10, rx: 5, fill: g('gold') }),
        sv('circle', { cx: l - 22, cy: 194, r: 10, fill: g('gold') }), sv('circle', { cx: r + 22, cy: 194, r: 10, fill: g('gold') }),
        sv('path', { d: cloth, fill: g(fill) }),
        sv('path', { d: cloth, fill: 'none', stroke: '#000', 'stroke-width': 18, opacity: 0.18 }),
        strokePath(cloth, g('gold'), 6), strokePath('M' + (l + 14) + ',218 H' + (r - 14) + ' V480 L' + cx + ',556 L' + (l + 14) + ',480 Z', GOLD.mid, 1.6, { opacity: 0.75 }),
        mark(cx, 252, 1.05),
        sv('path', { d: 'M' + (cx - 28) + ',520 q14,-26 28,-4 q14,-22 28,4 q-14,8 -28,18 q-14,-10 -28,-18 Z', fill: GOLD.mid, opacity: 0.85 }),
      ]);
    };
    kids.push(standardArt(192, 'em', markA), standardArt(mirror(192), 'cr', markH));
    // the nameplate shapes, the two hollow diamonds with their lines (between the plate and the count), the session plate, the footer
    const name = (x0, w) => {
      const d = 'M' + x0 + ',602 L' + (x0 + 30) + ',578 H' + (x0 + w - 30) + ' L' + (x0 + w) + ',602 L' + (x0 + w - 30) + ',626 H' + (x0 + 30) + ' Z';
      return sv('g', null, [sv('path', { d, fill: g('gold') }), strokePath(d, GOLD.lo, 2), strokePath('M' + (x0 + 36) + ',583 H' + (x0 + w - 36), GOLD.hi, 1.5, { opacity: 0.8 })]);
    };
    kids.push(name(32, 336), name(W - 32 - 336, 336));
    const diamond = (cx) => sv('path', { d: 'M' + cx + ',616 L' + (cx + 18) + ',635 L' + cx + ',654 L' + (cx - 18) + ',635 Z', fill: '#090914', stroke: g('gold'), 'stroke-width': 3.5 });
    kids.push(sv('rect', { class: 'lcc-art-line', x: 52, y: 633.8, width: 464, height: 2.4, fill: g('goldh') }));
    kids.push(sv('rect', { class: 'lcc-art-line', x: mirror(516), y: 633.8, width: 464, height: 2.4, fill: g('goldh') }));
    kids.push(diamond(556), diamond(mirror(556)));
    const session = 'M706,523 L742,490 H1430 L1466,523 L1430,556 H742 Z';
    kids.push(sv('path', { d: session, fill: g('plate') }), strokePath(session, g('gold'), 3.5), strokePath('M720,523 L750,498 H1422 L1452,523 L1422,548 H750 Z', GOLD.lo, 1.4, { opacity: 0.8 }));
    // the two vertical rules are drawn last, so nothing in this layer is ever painted over them; the one at x 1577 is the HELD BY rule
    kids.push(sv('rect', { class: 'lcc-art-rule', x: 711, y: 36, width: 3.5, height: 98, rx: 1.5, fill: g('vr') }));
    kids.push(sv('rect', { class: 'lcc-art-rule lcc-art-rule-held', x: 1577, y: 36, width: 3.5, height: 98, rx: 1.5, fill: g('vr') }));
    return sv('svg', { class: 'lcc-art', viewBox: '0 0 ' + W + ' ' + H, preserveAspectRatio: 'xMidYMid meet', 'aria-hidden': 'true', focusable: 'false' }, kids);
  }

  // The track: the pointed body, the middle gem and its lower point, and the 25 seats. a and h are validated counts or null (the track is then drawn with all seats open and marked unavailable).
  function track(p, a, h) {
    const T = TRACK;
    const g = (id) => 'url(#' + p + id + ')';
    const defs = sv('defs', null, [
      grad(p + 'body', 0, 0, 1, 0, [[0, '#04382d'], [0.42, '#0a2a22'], [0.5, '#1d1409'], [0.58, '#3a0b0e'], [1, '#52060a']]),
      grad(p + 'rail', 0, 0, 1, 0, [[0, '#2de2c0'], [0.34, '#bda24a'], [0.5, '#ffe28a'], [0.66, '#d67a3a'], [1, '#ff3b30']]),
      grad(p + 'tg', 0, 0, 0, 1, [[0, GOLD.hi], [0.45, GOLD.mid], [1, GOLD.lo]]),
      rgrad(p + 'sa', [[0, '#5ff0c8'], [0.55, '#14a07c'], [1, '#08513f']]),
      rgrad(p + 'sh', [[0, '#ff7b6e'], [0.55, '#c4242a'], [1, '#620a0f']]),
      rgrad(p + 'so', [[0, '#f1dcb4'], [0.6, '#b89a62'], [1, '#6f5a34']]),
      rgrad(p + 'sc', [[0, '#fff3c4'], [0.55, '#e0a93a'], [1, '#8e5f1c']]),
      glow(p + 'tblur', 4),
    ]);
    const body = 'M' + T.tipL + ',' + T.cy + ' L' + T.wallL + ',' + T.top + ' H' + T.wallR + ' L' + T.tipR + ',' + T.cy + ' L' + T.wallR + ',' + T.bottom + ' H' + T.wallL + ' Z';
    const inner = 'M' + (T.tipL + 24) + ',' + T.cy + ' L' + (T.wallL + 6) + ',' + (T.top + 12) + ' H' + (T.wallR - 6) + ' L' + (T.tipR - 24) + ',' + T.cy + ' L' + (T.wallR - 6) + ',' + (T.bottom - 12) + ' H' + (T.wallL + 6) + ' Z';
    const kids = [defs,
      sv('path', { d: body, fill: 'none', stroke: g('rail'), 'stroke-width': 14, opacity: 0.35, filter: g('tblur') }),
      sv('path', { d: body, fill: g('body') }),
      sv('path', { d: inner, fill: 'none', stroke: '#000', 'stroke-width': 10, opacity: 0.25 }),
      sv('path', { d: body, fill: 'none', stroke: g('tg'), 'stroke-width': 7, 'stroke-linejoin': 'miter' }),
      sv('path', { d: inner, fill: 'none', stroke: g('rail'), 'stroke-width': 3, 'stroke-linejoin': 'miter' }),
    ];
    // the middle gem above the hub and the point below it
    const gx = T.cx;
    kids.push(sv('path', { d: 'M' + gx + ',226 L' + (gx + 34) + ',270 L' + gx + ',312 L' + (gx - 34) + ',270 Z', fill: g('sc'), stroke: GOLD.lo, 'stroke-width': 3, 'stroke-linejoin': 'round' }));
    kids.push(sv('path', { d: 'M' + gx + ',226 L' + (gx + 34) + ',270 L' + gx + ',286 L' + (gx - 34) + ',270 Z', fill: '#fff6cf', opacity: 0.35 }));
    kids.push(sv('path', { d: 'M' + (gx - 16) + ',462 L' + gx + ',488 L' + (gx + 16) + ',462 Z', fill: g('tg'), stroke: GOLD.lo, 'stroke-width': 2 }));
    // seats
    const states = seatStates(a, h) || seatStates(0, 0);
    const seats = sv('g', { class: 'lcc-seats' });
    for (const s of states) {
      const x = seatX(s.index);
      const sg = sv('g', { class: 'lcc-seat lcc-seat-' + s.state + (s.center ? ' lcc-seat-center' : ''), transform: 'translate(' + x + ',' + T.cy + ')', 'data-slot': s.slot, 'data-state': s.state, 'data-center': s.center ? 'true' : null });
      sg.appendChild(sv('title', null, [doc().createTextNode(seatLabel(s))]));
      if (s.center) sg.appendChild(sv('circle', { class: 'lcc-seat-halo', r: T.haloR, fill: 'none', stroke: g('tg'), 'stroke-width': 5 }));
      sg.appendChild(sv('circle', { class: 'lcc-seat-base', r: T.r, fill: g(s.state === 'a' ? 'sa' : s.state === 'h' ? 'sh' : s.center ? 'sc' : 'so'), stroke: s.center ? GOLD.hi : '#2a2112', 'stroke-width': s.center ? 3.5 : 2 }));
      // a mark inside a claimed seat tells the two factions apart without colour: a hawk V for Atreides, a ring for Harkonnen
      if (s.state === 'a') sg.appendChild(sv('path', { class: 'lcc-seat-mark', d: 'M-9,-6 L0,10 L9,-6 L0,-1 Z', fill: '#f4efe0' }));
      if (s.state === 'h') sg.appendChild(sv('circle', { class: 'lcc-seat-mark', r: 6, fill: 'none', stroke: '#f4efe0', 'stroke-width': 3 }));
      seats.appendChild(sg);
    }
    kids.push(seats);
    return sv('svg', { class: 'lcc-track-svg', viewBox: '0 0 ' + W + ' ' + H, preserveAspectRatio: 'xMidYMid meet', focusable: 'false', role: 'img' }, kids);
  }

  // summaryText(a, h): the whole tally in words, for the track's label and for readers who do not see the seats.
  function summaryText(a, h) {
    if (!isCount(a) || !isCount(h) || a + h > SEATS) return 'The tally of the 25 seats is not available.';
    return 'Tally of the 25 seats: Atreides ' + a + ', Harkonnen ' + h + ', open ' + (SEATS - a - h) + '. Seats fill from the left for Atreides and from the right for Harkonnen. Seat 13 is the gold middle seat.';
  }

  const TEXT = {
    decreeLabel: 'ACTIVE DECREE', noDecree: 'No active decree', noEffect: '', contributors: 'TOP CONTRIBUTORS', noContributors: 'Not in the save',
    session: 'VOTING SESSION · SEAT 13 OF 25', atreides: 'HOUSE ATREIDES', harkonnen: 'HOUSE HARKONNEN', houses: 'HOUSES', unavailable: 'Tally not available',
  };

  // renderPlaque(vm): the approved decree and voting plaque as a read-only view of plain values.
  //   vm = { a, h, decree: { name, effect } | null, holder: 'a' | 'h' | null, contributors: { a: string[] | null, h: string[] | null } | null, sessionLabel, standing, demo }
  // It never decides a winner: the standing line is a caption the caller may replace (vm.standing), and nothing here reads the save.
  function renderPlaque(vm) {
    vm = vm || {};
    const p = 'lcc' + (++uid) + '-';
    const ok = isCount(vm.a) && isCount(vm.h) && vm.a + vm.h <= SEATS;
    const a = ok ? vm.a : null, h = ok ? vm.h : null;
    const host = el('section', 'lcc lcc-plaque-host', '', { 'aria-label': 'Landsraad decree and voting plaque', 'data-seats': ok ? SEATS : 'unavailable', 'data-a': ok ? a : null, 'data-h': ok ? h : null });
    const root_ = el('div', 'lcc-plaque');
    host.appendChild(root_);
    root_.appendChild(underlay(p));
    const tr = el('div', 'lcc-track');
    const svgTrack = track(p, a, h);
    svgTrack.setAttribute('aria-label', summaryText(a, h));
    tr.appendChild(svgTrack);
    // header text
    const d = vm.decree || null;
    const head = el('div', 'lcc-pl-head');
    head.appendChild(el('p', 'lcc-pl-decree-l', TEXT.decreeLabel));
    const title = el('div', 'lcc-pl-decree');
    title.appendChild(el('h3', 'lcc-pl-title', d && d.name ? d.name : TEXT.noDecree));
    if (d && d.effect) title.appendChild(el('p', 'lcc-pl-effect', d.effect));
    head.appendChild(title);
    head.appendChild(ruleV());
    const holdName = vm.holder === 'a' ? TEXT.atreides : vm.holder === 'h' ? TEXT.harkonnen : '';
    const hb = heldBy(holdName, vm.holder === 'a' || vm.holder === 'h' ? vm.holder : '');
    hb.classList.add('lcc-pl-held');
    head.appendChild(hb);
    root_.appendChild(head);
    root_.appendChild(tr);
    root_.appendChild(el('p', 'lcc-pl-session', vm.sessionLabel || TEXT.session));
    const foot = el('div', 'lcc-pl-foot');
    const c = vm.contributors || {};
    const sa = standard('a', c.a, TEXT.contributors, TEXT.noContributors), sh = standard('h', c.h, TEXT.contributors, TEXT.noContributors);
    sa.classList.add('lcc-pl-std-a'); sh.classList.add('lcc-pl-std-h');
    const faction = (key, n, name) => {
      const box = el('div', 'lcc-pl-faction lcc-pl-faction-' + key);
      box.appendChild(pill(name, 'lcc-pl-nameplate'));
      const pc = el('p', 'lcc-pl-count');
      pc.appendChild(el('b', '', ok ? n : '—'));
      pc.appendChild(doc().createTextNode(' ' + TEXT.houses));
      box.appendChild(pc);
      return box;
    };
    foot.appendChild(faction('a', a, TEXT.atreides));
    foot.appendChild(faction('h', h, TEXT.harkonnen));
    foot.appendChild(sa);
    foot.appendChild(sh);
    foot.appendChild(el('p', 'lcc-pl-standing', vm.standing != null ? vm.standing : ok ? standingLine(a, h) : TEXT.unavailable, { role: 'status' }));
    root_.appendChild(foot);
    if (vm.demo) root_.appendChild(badge('demo'));
    return host;
  }

  // ---- House card ------------------------------------------------------------------------------------------------------------------------------------------------------------------
  const OWNER = Object.freeze({ a: 'Atreides', h: 'Harkonnen', open: 'Open', unknown: 'Unreadable' });
  const OWNER_GLYPH = Object.freeze({ a: 'A', h: 'H', open: '○', unknown: '?' });
  const TRACK_LABEL = Object.freeze({ combat: 'Combat', crafting: 'Crafting', exploration: 'Exploration', gathering: 'Gathering', sabotage: 'Sabotage' });
  const houseTitle = (name) => String(name == null ? '' : name).replace(/^DA_House/i, '').replace(/^./, (c) => c.toUpperCase());
  // renderHouseCard(vm): vm = { name, owner: 'a'|'h'|'open'|'unknown', track, selected, oneAway, inLine, dominion, waiting }. A button: the whole card is the target.
  function renderHouseCard(vm) {
    vm = vm || {};
    const owner = OWNER[vm.owner] ? vm.owner : 'unknown';
    const title = houseTitle(vm.name) || 'Unknown House';
    const b = el('button', 'lcc lcc-house', '', { type: 'button', 'aria-pressed': vm.selected ? 'true' : 'false', 'data-owner': owner, 'data-one-away': vm.oneAway ? 'true' : null, 'data-line': vm.inLine ? 'true' : null });
    if (vm.inLine) victory(b, 'circuit'); else if (vm.dominion) victory(b, 'dominion');
    if (vm.selected) b.setAttribute('data-selected', 'true');
    const crest = el('span', 'lcc-house-crest');
    const sym = houseAsset(vm.name);
    if (sym) crest.appendChild(art(sym, 'lcc-house-img', 40)); else crest.appendChild(el('span', 'lcc-house-initials', title.slice(0, 2).toUpperCase()));
    b.appendChild(crest);
    const body = el('span', 'lcc-house-body');
    body.appendChild(el('span', 'lcc-house-name', title));
    body.appendChild(el('span', 'lcc-owner lcc-owner-' + owner, OWNER_GLYPH[owner] + ' ' + OWNER[owner]));
    const tr = trackAsset('banners', vm.track);
    if (tr) {
      const ph = el('span', 'lcc-house-path');
      ph.appendChild(art(tr, 'lcc-house-path-img', 20));
      ph.appendChild(el('span', '', TRACK_LABEL[norm(vm.track)]));
      body.appendChild(ph);
    }
    b.appendChild(body);
    const flags = el('span', 'lcc-house-flags');
    const notes = [];
    if (vm.oneAway) { flags.appendChild(el('span', 'lcc-flag lcc-flag-one', 'ONE AWAY')); notes.push('one House away from a line'); }
    if (vm.inLine) { flags.appendChild(el('span', 'lcc-flag lcc-flag-line', VICTORY.circuit)); notes.push('part of a line of five'); }
    else if (vm.dominion) { flags.appendChild(el('span', 'lcc-flag lcc-flag-dom', VICTORY.dominion)); notes.push('a House of the majority'); }
    if (vm.waiting > 0) { flags.appendChild(el('span', 'lcc-flag lcc-flag-wait', vm.waiting + ' WAITING')); notes.push(vm.waiting + ' reward' + (vm.waiting === 1 ? '' : 's') + ' waiting'); }
    if (flags.childNodes.length) b.appendChild(flags);
    b.setAttribute('aria-label', [title + ', ' + (owner === 'a' || owner === 'h' ? 'claimed by ' + OWNER[owner] : owner === 'open' ? 'open' : 'owner unreadable')].concat(vm.track && TRACK_LABEL[norm(vm.track)] ? ['path ' + TRACK_LABEL[norm(vm.track)]] : [], notes, vm.selected ? ['selected'] : []).join(', '));
    return b;
  }

  // ---- Specialization lane card ---------------------------------------------------------------------------------------------------------------------------------------
  const fmt = (n) => Number(n).toLocaleString('en-US');
  const num = (n) => typeof n === 'number' && Number.isFinite(n);
  // laneState(vm): the labels of one lane, kept apart so XP and keystones are never one word. LV100 (the XP cap) is not "all keystones"; "full path" needs both, proved by both numbers.
  function laneState(vm) {
    const maxXp = num(vm.maxXp) ? vm.maxXp : 0, xp = num(vm.xp) ? vm.xp : 0;
    const capped = maxXp > 0 && xp >= maxXp;
    const total = num(vm.keystonesTotal) ? vm.keystonesTotal : 0, owned = num(vm.keystonesOwned) ? vm.keystonesOwned : 0;
    const complete = total > 0 && owned >= total;
    return { capped, complete, full: capped && complete, unknownKeystones: total === 0, pct: maxXp > 0 ? Math.max(0, Math.min(100, Math.floor((xp / maxXp) * 100))) : 0 };
  }
  // renderLaneCard(vm): vm = { track, level, maxLevel, xp, maxXp, keystonesOwned, keystonesTotal, selected, unresolved }.
  function renderLaneCard(vm) {
    vm = vm || {};
    const label = TRACK_LABEL[norm(vm.track)] || 'Unknown path';
    const st = laneState(vm);
    const b = el('button', 'lcc lcc-lane', '', { type: 'button', 'aria-pressed': vm.selected ? 'true' : 'false', 'data-track': TRACKS.indexOf(norm(vm.track)) >= 0 ? norm(vm.track) : null, 'data-xpcap': st.capped ? 'true' : null, 'data-ks-complete': st.complete ? 'true' : null, 'data-full': st.full ? 'true' : null, 'data-unresolved': vm.unresolved ? 'true' : null });
    const icon = el('span', 'lcc-lane-icon');
    const la = laneAsset(vm.track);
    if (la) icon.appendChild(art(la, 'lcc-lane-img', 72)); else icon.appendChild(el('span', 'lcc-lane-initials', label.slice(0, 2).toUpperCase()));
    b.appendChild(icon);
    b.appendChild(el('span', 'lcc-lane-name', label));
    const lv = el('span', 'lcc-lane-level');
    lv.appendChild(doc().createTextNode('LV '));
    lv.appendChild(el('b', '', num(vm.level) ? vm.level : '—'));
    lv.appendChild(el('small', '', ' / ' + (num(vm.maxLevel) ? vm.maxLevel : 100)));
    b.appendChild(lv);
    const bar = el('span', 'lcc-lane-bar', '', { role: 'progressbar', 'aria-valuemin': 0, 'aria-valuemax': 100, 'aria-valuenow': st.pct, 'aria-label': 'XP toward the cap of this path' });
    const fill = el('i', 'lcc-lane-fill');
    fill.style.setProperty('--lcc-v', st.pct + '%'); // a value that changes at run time goes through the style object, never a style attribute in markup (the content security policy)
    bar.appendChild(fill);
    b.appendChild(bar);
    b.appendChild(el('span', 'lcc-lane-nums', (num(vm.xp) ? fmt(vm.xp) : '—') + ' XP · ' + (st.unknownKeystones ? 'keystones unknown' : fmt(vm.keystonesOwned) + '/' + fmt(vm.keystonesTotal) + ' keystones')));
    const tags = el('span', 'lcc-lane-tags');
    if (vm.unresolved) tags.appendChild(el('span', 'lcc-flag lcc-flag-warn', 'TRACK NOT IDENTIFIED'));
    else if (st.full) tags.appendChild(el('span', 'lcc-flag lcc-flag-cap', '✦ FULL PATH COMPLETE'));
    else {
      if (st.capped) tags.appendChild(el('span', 'lcc-flag lcc-flag-cap', '✦ XP CAP REACHED'));
      if (st.complete) tags.appendChild(el('span', 'lcc-flag lcc-flag-ks', 'KEYSTONES COMPLETE'));
      if (!st.capped && !st.complete) tags.appendChild(el('span', 'lcc-flag', 'PATH IN PROGRESS'));
    }
    b.appendChild(tags);
    b.setAttribute('aria-label', label + ', level ' + (num(vm.level) ? vm.level : 'unknown') + ' of ' + (num(vm.maxLevel) ? vm.maxLevel : 100) + ', ' + (num(vm.xp) ? fmt(vm.xp) : 'unknown') + ' XP' + (st.capped ? ', XP cap reached' : '') + ', ' + (st.unknownKeystones ? 'keystones unknown' : vm.keystonesOwned + ' of ' + vm.keystonesTotal + ' keystones' + (st.complete ? ', all keystones bought' : '')) + (vm.selected ? ', selected' : ''));
    return b;
  }

  const api = {
    HOUSES, TRACKS, DECREE_FILES, DECREE_FALLBACK, SEATS, CENTER, TRACK, GEOMETRY: Object.freeze({ W, H }),
    config, houseAsset, laneAsset, bannerAsset, decreeAsset,
    seatStates, seatLabel, seatX, standingLine, summaryText, laneState,
    pill, ruleV, heldBy, standard, badge, richTip, frame, victory, VICTORY,
    renderPlaque, renderHouseCard, renderLaneCard,
  };
  if (typeof module !== 'undefined' && module.exports) module.exports = api;
  else root.LCC = api;
})(typeof window !== 'undefined' ? window : globalThis);
