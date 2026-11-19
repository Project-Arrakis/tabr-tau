// The Command Center's visual foundation (milestone M2): art lookups from fixed lists, the 25-seat model of the voting track, and the rendering helpers for the
// plaque, the House card, the specialization lane card and the small parts they share.
//
// What this file is not: it holds no demo data, it fetches nothing, it writes nothing to the save and it is not linked from any live page yet. Every helper takes plain values
// and returns an element built with createElement / createElementNS and textContent (never a string of HTML), so a House name or a decree name from a save cannot become markup.
// The picture of a House, a lane, a banner, a decree or a faction is looked up in a fixed table by its key; a file name is never built from a name, so a strange name can only miss the table.
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
  // The two faction crests (a separate set: never looked up through the House or path tables). 'mark' is the white crest of the header shields, the cloth standards and a claimed 13th seat;
  // 'medal' is the round medallion of a claimed seat.
  const FACTION_FILES = Object.freeze({ a: Object.freeze({ mark: 'atreides', medal: 'atreides-medallion' }), h: Object.freeze({ mark: 'harkonnen', medal: 'harkonnen-medallion' }) });

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
  // emblemAsset(track): the clean emblem of a path (96 x 96 PNG, only the emblem, no shadow and no faded side copies: cut from the banner picture by scripts/lcc-emblems.mjs), which the House card draws small and in one tint (#963, #976)
  const emblemAsset = (track) => { const k = norm(track); return KEY_OK.test(k) && TRACKS.indexOf(k) >= 0 ? Object.freeze({ key: k, file: 'emblems/' + k + '.png', src: artBase + 'emblems/' + k + '.png', fallback: false }) : null; };
  // itemAsset(id): the picture of a reward item (64 x 64 WebP, art/items/<id>.webp, #977), by the item id the save gives, from the fixed list of the pictures that were embedded: an id that is not on the
  // list (or is not shaped like an id) has no picture and answers null, so no request is ever made for a file that does not exist; the id is never turned into a path by anything but this list.
  const ITEM_PICS = new Set([
    'Bloodsack_Unique_Durable_06_Schematic', 'BodyFluidExtractor_Unique_Water_05_Schematic', 'BuggyEngine_Unique_Accelerate_06_Schematic',
    'BuggyInventory_Unique_Capacity_05_Schematic', 'BuggyMining_Unique_YieldIncrease_05_Schematic', 'BuggyMining_Unique_YieldIncrease_06_Schematic',
    'Combat_Heavy_Unique_PowerEfficient_Gloves_06_Schematic', 'Combat_Heavy_Unique_PowerIncrease_Top_06_Schematic', 'Combat_Light_Unique_DewReap_Gloves_06_Schematic',
    'Combat_Light_Unique_ReduceSuspensor_Top_06_Schematic', 'Combat_Light_Unique_WormThreat_Boots_06_Schematic', 'Combat_Neut_AtreidesDeserterUnique04_Boots_Schematic',
    'Combat_Neut_AtreidesDeserterUnique04_Bottom_Schematic', 'Combat_Neut_AtreidesDeserterUnique04_Gloves_Schematic',
    'Combat_Neut_AtreidesDeserterUnique04_Helmet_Schematic', 'Combat_Neut_AtreidesDeserterUnique04_Top_Schematic', 'Combat_Neut_SmugglerDeserterUnique06_Boots_Schematic',
    'Combat_Neut_SmugglerDeserterUnique06_Bottom_Schematic', 'Combat_Neut_SmugglerDeserterUnique06_Gloves_Schematic',
    'Combat_Neut_SmugglerDeserterUnique06_Helmet_Schematic', 'Combat_Neut_SmugglerDeserterUnique06_Top_Schematic', 'DewReaper_1h_Unique_Compact_06_Schematic',
    'DewReaper_2h_Unique_YieldIncrease_06_Schematic', 'DewReaper_Unique_04_Schematic', 'DewReaper_Unique_05_Schematic', 'GlidePartialStabilizationBelt_05_Schematic',
    'HeavyPistol_Unique_Headshot_05_Schematic', 'HighCapacityLiterjon_05_Schematic', 'HighCapacityLiterjon_06_Schematic', 'Kindjal_Unique_Blood_05_Schematic',
    'Kindjal_Unique_Stamina_05_Schematic', 'LMG_Unique_Power_06_Schematic', 'LMG_Unique_RapidFire_05_Schematic', 'LongRifle_Unique_LargeMag_05_Schematic',
    'MiningTool_2h_Unique_04_Schematic', 'OrnithopterLightLocomotion_Unique_Speed_6_Schematic', 'OrnithopterMediumBoost_Unique_LessHeat_5_Schematic',
    'OrnithopterMediumBoost_Unique_LessHeat_6_Schematic', 'OrnithopterMediumLocomotion_Unique_Strafe_5_Schematic', 'OrnithopterMediumLocomotion_Unique_Strafe_6_Schematic',
    'PowerPack_Unique_Regen_05_Schematic', 'PowerPack_Unique_Regen_06_Schematic', 'SMG_Unique_LargeMag_06_Schematic', 'SandbikeEngine_Unique_Speed_5_Schematic',
    'SandcrawlerEngine_Unique_Speed_06_Schematic', 'SandcrawlerLocomotion_Unique_WormThreat_06_Schematic', 'Schematic_UniqueCutteray5', 'Schematic_UniqueFlamethrower',
    'Schematic_UniqueForgeChest', 'Schematic_UniqueForgeFeet', 'Schematic_UniqueForgeHands', 'Schematic_UniqueForgeHead', 'Schematic_UniqueForgeLegs',
    'Shotgun_Unique_Blood_06_Schematic', 'Shotgun_Unique_Explosive_06_Schematic', 'StaticCompactor_Unique_Compact_06_Schematic',
    'Stillsuit_Unique_Efficient_05_Boots_Schematic', 'Stillsuit_Unique_Efficient_05_Gloves_Schematic', 'Stillsuit_Unique_Efficient_05_Mask_Schematic',
    'Stillsuit_Unique_Efficient_05_Top_Schematic', 'Stillsuit_Unique_Efficient_06_Boots_Schematic', 'Stillsuit_Unique_Efficient_06_Gloves_Schematic',
    'Stillsuit_Unique_Efficient_06_Mask_Schematic', 'Stillsuit_Unique_Efficient_06_Top_Schematic', 'T6ArmorPlating', 'T6BalisticWeave', 'T6BladePart', 'T6CarbidePladeParts',
    'T6DiamodineBladeParts', 'T6FilteredFabric', 'T6GunPart', 'T6HeavyCalliberCompressor', 'T6HoltzmanActuator', 'T6HydraulicPiston', 'T6IndustrialPump', 'T6IrradiatedCore',
    'T6LightCalliberCompressor', 'T6Machinery', 'T6PowerRegulator', 'T6RangeFinder', 'T6RayAmplifier', 'T6Watertube', 'UniqueAr_Burst_05_Schematic',
    'UniqueAr_Burst_06_Schematic', 'UniqueDirk_03_Schematic', 'UniqueDirk_04_Schematic', 'UniqueRapier_02_Schematic', 'UniqueRapier_03_Schematic',
    'UniqueScattergun5_Schematic', 'UniqueScattergun_Fire_05_Schematic', 'UniqueSda5_Schematic', 'UniqueSda6_Schematic', 'UniqueSda_Doubleshot_06_Schematic',
    'UniqueSmg2_Schematic', 'UniqueSword_04_Schematic', 'UniqueSword_05_Schematic',
  ]);
  const ITEM_OK = /^[A-Za-z][A-Za-z0-9_]{1,80}$/;
  const itemAsset = (id) => (typeof id === 'string' && ITEM_OK.test(id) && ITEM_PICS.has(id) ? Object.freeze({ key: id, file: 'items/' + id + '.webp', src: artBase + 'items/' + id + '.webp', fallback: false, width: 64, height: 64 }) : null);
  // decreeAsset(id) always answers: the picture of a known decree, or the neutral picture marked fallback: true.
  function decreeAsset(id) {
    const k = norm(typeof id === 'number' ? '' : id);
    if (KEY_OK.test(k) && Object.prototype.hasOwnProperty.call(DECREE_ART, k)) { const f = DECREE_ART[k]; return hit('decrees', f, f); }
    return Object.freeze({ key: null, file: 'decrees/' + DECREE_FALLBACK + '.svg', src: artBase + 'decrees/' + DECREE_FALLBACK + '.svg', fallback: true });
  }

  // factionAsset('a' | 'h' | 'Atreides' | 'Harkonnen', 'mark' | 'medal') always answers: the crest of that faction, or { key: null, file: null, src: '', fallback: true } for anything else (a caller
  // then draws no picture and the words and state cues stand alone). The kind is 'mark' unless it is exactly 'medal'.
  function factionAsset(faction, kind) {
    const k = norm(typeof faction === 'string' ? faction : '');
    const f = k === 'a' || k === 'atreides' ? 'a' : k === 'h' || k === 'harkonnen' ? 'h' : '';
    if (!f) return Object.freeze({ key: null, file: null, src: '', fallback: true });
    return hit('factions', FACTION_FILES[f][kind === 'medal' ? 'medal' : 'mark'], f);
  }

  // The foot of a cloth standard: the V-edged gold point with its wheat ornament, a picture of the approved design (a crop of the plaque, 111 pixels tall, Atreides 320 wide and Harkonnen 319 wide), one
  // for each faction because the two were painted apart. Same rules as the other lookups: a fixed list, never a name built into a path. TIP_H is the picture's height.
  const TIP_FILES = Object.freeze({ a: 'atreides-tip', h: 'harkonnen-tip' });
  const TIP_W = Object.freeze({ a: 320, h: 319 }), TIP_H = 111;
  function standardTipAsset(faction) {
    const k = norm(typeof faction === 'string' ? faction : '');
    const f = k === 'a' || k === 'atreides' ? 'a' : k === 'h' || k === 'harkonnen' ? 'h' : '';
    if (!f) return Object.freeze({ key: null, file: null, src: '', fallback: true });
    return Object.freeze({ key: f, file: 'standards/' + TIP_FILES[f] + '.png', src: artBase + 'standards/' + TIP_FILES[f] + '.png', fallback: false, width: TIP_W[f], height: TIP_H });
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
  // The words of the Sysselraad (who has won, the headline, the count line) are written once, in lcc-model.js (WIN); this file takes them from there and builds none of its own. The model is
  // loaded before the first plaque or card is drawn (the page loads it with this file); a caller that has not loaded it gets the error at once instead of a different phrase.
  const WORDS = () => { if (!root.LCCM || !root.LCCM.WORDS) throw new Error('lcc-model.js must be loaded before lcc.js draws a card'); return root.LCCM.WORDS; };
  const WIN = () => { if (!root.LCCM || !root.LCCM.WIN) throw new Error('lcc-model.js must be loaded before lcc.js draws a plaque or a card'); return root.LCCM.WIN; };
  // standingLine(a, h): the count line for two counts, as it always read ('HARKONNEN LEAD 11 TO 4 · 10 HOUSES OPEN'); the line of a state with a winner is WIN.tally.
  function standingLine(a, h) { return WIN().standing(a, h); }

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
  // The cloth standard of a faction: a gold-edged banner with the faction's crest picture, a heading and an honest list (or the empty state). lines is an array of entries or null; an entry is a string
  // (shown as it is) or { name } (#1043: the banner lists the names only; a { name, amount } entry is drawn as its name, the amount is never shown, so the #972 always-visible amount is gone).
  function standard(faction, lines, heading, emptyText) {
    const f = faction === 'h' ? 'h' : 'a';
    const w = el('div', 'lcc-standard lcc-f-' + f);
    const cloth = el('div', 'lcc-standard-cloth'); // the cloth and its words; the foot picture is a sibling below it, so the cloth ends where the picture starts
    const mk = el('span', 'lcc-standard-mark', '', { 'aria-hidden': 'true' });
    const crest = factionAsset(f, 'mark');
    if (!crest.fallback) mk.appendChild(art(crest, 'lcc-standard-img', 44));
    cloth.appendChild(mk);
    cloth.appendChild(el('p', 'lcc-standard-h', heading || 'TOP CONTRIBUTORS'));
    if (Array.isArray(lines) && lines.length) {
      const ol = el('ol', 'lcc-standard-list', '', { role: 'list' });
      // the name is text in its own span (the stylesheet ellipsizes it in the wide layout and wraps it in the narrow one); the full name stays in the title
      for (const s of lines.slice(0, 5)) {
        const li = el('li');
        if (s && typeof s === 'object') {
          const name = String(s.name);
          li.appendChild(el('span', 'lcc-standard-name', name, { title: name }));
        } else { const name = String(s); li.appendChild(el('span', 'lcc-standard-name', name, { title: name })); }
        ol.appendChild(li);
      }
      cloth.appendChild(ol);
    } else cloth.appendChild(el('p', 'lcc-standard-none', emptyText || 'Not in the save'));
    w.appendChild(cloth);
    // the foot of the narrow cloth: the approved picture of the gold-edged point with its wheat ornament (the wide layout draws the same picture in the artwork and hides this one)
    const tip = standardTipAsset(f);
    if (!tip.fallback) {
      const foot = el('span', 'lcc-standard-foot', '', { 'aria-hidden': 'true' });
      foot.appendChild(art(tip, 'lcc-standard-tip', TIP_H));
      w.appendChild(foot);
    }
    return w;
  }
  // A label that says what the data is. kind: 'demo' (fixture values on a preview page), 'readonly' (shown, never written here) or 'ref' (a reference value, not the save's).
  // 'save' and 'derived' (M3) say where a live figure comes from: read from the save, or worked out by the page from what the save holds; 'unverified' marks a result the game has not been seen to confirm.
  const BADGE = Object.freeze({ demo: 'FIXTURE · DEMO DATA', readonly: 'READ-ONLY', ref: 'REFERENCE', save: 'FROM THE SAVE', derived: 'WORKED OUT HERE', unverified: 'NOT CHECKED IN THE GAME' });
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
  // The one win outline (operator decision of 2026-11-18): a House that makes up a win, one of the five of a line or one of the 13 or more Houses of the winner, has the same single gold outline and
  // the same flag. Which way it was won (a line of five, or 13 Houses) is said by the words of the plaque and the victory panel, not by a second kind of outline.
  const VICTORY = Object.freeze({ get win() { return WIN().TEXT.won; } }); // the flag of a winning House
  // The animated border (class lcc-win-anim and a .lcc-win-sweep child that the style moves by transform) is added only when Motion is Full and the system has not asked for reduced motion.
  // The style checks the same two things again, so this is the first of two gates, not the only one. Calm, Off and reduced motion get the still outline and nothing else.
  function winAnimOn() {
    const root = typeof document !== 'undefined' ? document.documentElement : null;
    if (!root || root.getAttribute('data-motion') !== 'full') return false;
    try { if (typeof window !== 'undefined' && window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches) return false; } catch (e) { /* no media query support: treat as not asked */ }
    return true;
  }
  function victory(elm, on) {
    on = !!on;
    if (on) elm.setAttribute('data-victory', 'win'); else elm.removeAttribute('data-victory');
    for (const old of [...elm.children]) if (old.classList && old.classList.contains('lcc-win-sweep')) old.remove();
    if (on && winAnimOn()) { elm.classList.add('lcc-win-anim'); elm.appendChild(el('span', 'lcc-win-sweep', '', { 'aria-hidden': 'true' })); } else elm.classList.remove('lcc-win-anim');
    return elm;
  }

  // ---- the plaque ----------------------------------------------------------------------------------------------------------------------------------------------------------------
  // The geometry is one coordinate system, 2172 by 724, the size of the approved artwork. The decorative layer and the track are drawn in it as vectors; the words are real text
  // above them. Nothing here is a picture with text in it.
  const W = 2172, H = 724;
  // The seats are spaced by the gap between their OUTER edges, so that all 24 gaps are equal (#965): seat is the outer radius of an ordinary seat (its circle and its 2 unit outline), halo the outer
  // radius of the ring of the 13th seat (the ring and its stroke, still or won, so the same size in every state), gap the clear space between any two neighbouring outer edges. pitch is the distance
  // between the centres of two ordinary neighbours, and mid the distance from the 13th seat's centre to the centre of its two neighbours: both follow from the three.
  // The 13th seat is measured from the approved artwork itself (#983, #982), which is in this same 2172 by 724 coordinate system, not from the operator's words: its ring is 112 units across (the rim 5.5)
  // and touches the rail on both sides. Its two wedges are NOT the same: the upward one on the ring's top is the smaller (30 wide, 18 above the ring, about 1.7 : 1) and its tip ends just above the rail,
  // the downward one on the bottom is bigger (43 wide, 28 below the ring, about 1.5 : 1) and hangs past the rail. wedgeUp and wedgeDown are the distance of a tip's OUTER edge from the centre, wedgeHalfUp
  // and wedgeHalfDown the half width of the base, whose corners are on the ring, so the wedges run up and down and never take room from the gap to a neighbour.
  const SEAT_OUT = 20, HALO_OUT = 56, SEAT_GAP = 11, HALO_W = 5, HALO_W_WIN = 6, WEDGE_UP = 74, WEDGE_UP_HALF = 15, WEDGE_DOWN = 84, WEDGE_DOWN_HALF = 21.5;
  // the faceted diamond above the upward wedge: half width and half height of its outer bevel (70 by 94 units in the artwork), the share of the height where it is widest, the share of the bevel that the
  // inner diamond of the four facets takes, the clear space between the outer edge of the wedge's tip and the gem's lower point (#983: 1 to 2 px, 2 to 3 units in the artwork), and the stroke of the gem's pale edge
  const GEM_HALF_W = 35, GEM_HALF_H = 47, GEM_MID = 0.52, GEM_BEVEL = 0.72, GEM_GAP = 2, GEM_STROKE = 2;
  const TRACK = Object.freeze({ cx: W / 2, cy: 396, r: 19, seat: SEAT_OUT, halo: HALO_OUT, wedgeUp: WEDGE_UP, wedgeDown: WEDGE_DOWN, wedgeHalfUp: WEDGE_UP_HALF, wedgeHalfDown: WEDGE_DOWN_HALF, gap: SEAT_GAP, pitch: 2 * SEAT_OUT + SEAT_GAP, mid: HALO_OUT + SEAT_GAP + SEAT_OUT, tipL: 334, tipR: 1838, wallL: 410, wallR: 1762, top: 330, bottom: 462 });
  // x of a seat's centre. The 13th sits on the middle of the track and the others step away from it by TRACK.pitch, the first one by TRACK.mid (the gold ring is wider than a seat), so every gap between two
  // neighbouring outer edges is TRACK.gap and the two ends are inset by the same amount.
  const seatX = (i) => { const k = i - CENTER; return k === 0 ? TRACK.cx : TRACK.cx + Math.sign(k) * (TRACK.mid + (Math.abs(k) - 1) * TRACK.pitch); };
  // the outer radius of a seat's drawing (circle, outline, ring), the 13th being the wider one
  const seatOuter = (i) => (i === CENTER ? TRACK.halo : TRACK.seat);
  const GOLD = Object.freeze({ hi: '#fff0b8', mid: '#e8b84e', lo: '#8e5f1c' });

  function cut(x, y, w, h, c) {
    return 'M' + (x + c) + ',' + y + 'H' + (x + w - c) + 'L' + (x + w) + ',' + (y + c) + 'V' + (y + h - c) + 'L' + (x + w - c) + ',' + (y + h) + 'H' + (x + c) + 'L' + x + ',' + (y + h - c) + 'V' + (y + c) + 'Z';
  }
  function grad(id, x1, y1, x2, y2, stops) {
    return sv('linearGradient', { id, x1, y1, x2, y2 }, stops.map((s) => sv('stop', { offset: s[0], 'stop-color': s[1], 'stop-opacity': s[2] == null ? 1 : s[2] })));
  }
  function rgrad(id, stops) {
    // centred on the seat, so the edge of the circle is the same colour all the way round (#971): a gradient lit from the upper left made the right edge of every seat darker than the left
    return sv('radialGradient', { id, cx: '50%', cy: '50%', r: '50%' }, stops.map((s) => sv('stop', { offset: s[0], 'stop-color': s[1] })));
  }
  function glow(id, dev, pad) {
    const m = pad == null ? 10 : pad; // percent of the box on every side: the glow of a small shape needs more than the 10 that a long rail does
    return sv('filter', { id, x: '-' + m + '%', y: '-' + m + '%', width: (100 + 2 * m) + '%', height: (100 + 2 * m) + '%' }, [sv('feGaussianBlur', { stdDeviation: dev })]);
  }
  // The 13th seat's outline (#983): the whole ring as a circle and the sides of the two wedges, as ONE path, so the rim runs from the ring into the wedge sides without a seam. r is the radius of the ring's
  // centre line, hw the stroke; the base corners of a wedge are on the ring, the tips are WEDGE_UP and WEDGE_DOWN from the centre (outer edge, the stroke's round join counted). Coordinates are relative to
  // the seat's centre. top and bottom are the filled triangles of the two wedges (their base is the chord between the two corners; the ring is drawn over it).
  function centreOutline(hw) {
    const r = HALO_OUT - hw / 2, n = (v) => Math.round(v * 1000) / 1000;
    const wedge = (half, out, sign) => { const ty = Math.sqrt(r * r - half * half), tip = out - hw / 2; return { half, ty: n(ty), tip: n(tip), sides: 'M' + half + ',' + n(sign * ty) + ' L0,' + n(sign * tip) + ' L' + -half + ',' + n(sign * ty), fill: 'M' + -half + ',' + n(sign * ty) + ' L' + half + ',' + n(sign * ty) + ' L0,' + n(sign * tip) + ' Z' }; };
    const up = wedge(WEDGE_UP_HALF, WEDGE_UP, -1), down = wedge(WEDGE_DOWN_HALF, WEDGE_DOWN, 1);
    const ring = 'M' + n(r) + ',0 A' + n(r) + ',' + n(r) + ' 0 1 1 ' + n(-r) + ',0 A' + n(r) + ',' + n(r) + ' 0 1 1 ' + n(r) + ',0 Z';
    return { r, up, down, d: ring + ' ' + up.sides + ' ' + down.sides, top: up.fill, bottom: down.fill };
  }
  // the faceted gem above the 13th seat, relative to the seat's centre: its lower point is GEM_GAP above the outer edge of the upward wedge's tip, a pale bevel round four facets that meet in the middle
  function centreGem() {
    const bottom = -(WEDGE_UP + GEM_GAP + GEM_STROKE / 2), top = bottom - GEM_HALF_H * 2, mid = top + (bottom - top) * GEM_MID, w = GEM_HALF_W;
    const n = (v) => Math.round(v * 100) / 100, P = (x, y) => n(x) + ',' + n(y);
    const iw = w * GEM_BEVEL, it = mid + (top - mid) * GEM_BEVEL, ib = mid + (bottom - mid) * GEM_BEVEL; // the inner diamond the facets fill
    return { top, bottom, mid, outline: 'M' + P(0, top) + ' L' + P(w, mid) + ' L' + P(0, bottom) + ' L' + P(-w, mid) + ' Z', inner: 'M' + P(0, it) + ' L' + P(iw, mid) + ' L' + P(0, ib) + ' L' + P(-iw, mid) + ' Z',
      cross: 'M' + P(0, it) + ' V' + n(ib) + ' M' + P(-iw, mid) + ' H' + n(iw),
      facets: ['M' + P(0, it) + ' L' + P(-iw, mid) + ' L' + P(0, mid) + ' Z', 'M' + P(0, it) + ' L' + P(iw, mid) + ' L' + P(0, mid) + ' Z', 'M' + P(-iw, mid) + ' L' + P(0, ib) + ' L' + P(0, mid) + ' Z', 'M' + P(iw, mid) + ' L' + P(0, ib) + ' L' + P(0, mid) + ' Z'] };
  }
  const strokePath = (d, stroke, w, extra) => sv('path', Object.assign({ d, fill: 'none', stroke, 'stroke-width': w, 'stroke-linejoin': 'miter' }, extra || {}));
  const mirror = (x) => W - x;
  // The size of the crest pictures in the 2172 x 724 drawing (the picture is a 256 square with its margin: the visible crest is about three quarters of it), as the approved V20 plaque shows them.
  const SHIELD_MARK = 120, STANDARD_MARK = 88, SEAT_MEDAL = 2 * TRACK.r + 2, SEAT_MARK = 64;
  // The medallion pictures carry their own lighting: the rim of the Atreides medallion is bright on its left and dull on its right (about 20 percent), the Harkonnen one the other way round. A flat ring of the
  // faction's colour is drawn over that rim, centred on the rim's own middle, so a claimed seat is the same all the way round (#971). RIM_R is the middle of the picture's rim, RIM_W its width, both in the
  // drawing's units; the ring stays inside the seat's outer edge (RIM_R + RIM_W / 2 is below SEAT_OUT).
  const SEAT_RIM = Object.freeze({ a: '#1fc08c', h: '#d02a2f' }), RIM_R = 17.9, RIM_W = 2.2;

  // A picture placed in the vector layers: an SVG <image> centred on (cx, cy), or null when there is no picture (a faction the table does not know). A picture that fails to load draws nothing, so
  // what is behind it (the shield or cloth colour, the seat circle) and the words beside it stand alone.
  function svgPicture(asset, cx, cy, size, cls, onLoad) {
    if (!asset || asset.fallback) return null;
    const im = sv('image', { class: cls, href: asset.src, x: cx - size / 2, y: cy - size / 2, width: size, height: size, preserveAspectRatio: 'xMidYMid meet' });
    if (onLoad) im.addEventListener('load', onLoad);
    im.addEventListener('error', () => { im.setAttribute('visibility', 'hidden'); });
    return im;
  }

  // The decorative layer: frame, header band, the two shields with their crests, the decree plate, the two gold rules, the two cloth standards with their crests and rods, the nameplate shapes, the two hollow diamonds
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
      grad(p + 'shade', 0, 0, 0, 1, [[0, '#000', 0], [1, '#000', 0.45]]),
      // the foot pictures fade into the drawing at the top (over our cloth) and out at the sides and the bottom (over the field), so no edge of the crop shows
      grad(p + 'tfv', 0, 0, 0, 1, [[0, '#fff', 0], [0.16, '#fff', 1], [0.92, '#fff', 1], [1, '#fff', 0]]),
      grad(p + 'tfh', 0, 0, 1, 0, [[0, '#fff', 0], [0.07, '#fff', 1], [0.93, '#fff', 1], [1, '#fff', 0]]),
      sv('mask', { id: p + 'tipv', maskContentUnits: 'objectBoundingBox' }, [sv('rect', { x: 0, y: 0, width: 1, height: 1, fill: 'url(#' + p + 'tfv)' })]),
      sv('mask', { id: p + 'tip', maskContentUnits: 'objectBoundingBox' }, [sv('rect', { x: 0, y: 0, width: 1, height: 1, fill: 'url(#' + p + 'tfh)', mask: 'url(#' + p + 'tipv)' })]),
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
    for (const d of [cut(6, 6, 2160, 712, 26), 'M16,166 H2156']) kids.push(strokePath(d, GOLD.mid, 12, { opacity: 0.22, filter: g('blur'), class: 'lcc-glow lcc-glow-frame' }));
    // (a straight line cannot carry a gradient that is sized by its own box, so the straight rules are thin rectangles)
    kids.push(strokePath(cut(6, 6, 2160, 712, 26), g('gold'), 5));
    kids.push(strokePath(cut(16, 16, 2140, 692, 22), GOLD.lo, 1.5, { opacity: 0.8 }));
    kids.push(sv('rect', { class: 'lcc-glow-line', x: 16, y: 164.2, width: 2140, height: 3.6, fill: g('goldh') }));
    kids.push(sv('rect', { class: 'lcc-glow-line', x: 70, y: 686.5, width: 2032, height: 3, fill: g('goldh') }));
    kids.push(strokePath('M16,166 V690 M2156,166 V690', GOLD.mid, 3, { opacity: 0.9 }));
    // shields (Atreides left, Harkonnen right)
    const shield = (x0, fill, faction) => {
      const d = 'M' + x0 + ',14 H' + (x0 + 156) + ' V118 L' + (x0 + 78) + ',182 L' + x0 + ',118 Z';
      return sv('g', null, [sv('path', { d, fill: g(fill) }), strokePath(d, g('gold'), 4.5), strokePath('M' + (x0 + 10) + ',24 H' + (x0 + 146) + ' V114 L' + (x0 + 78) + ',168 L' + (x0 + 10) + ',114 Z', GOLD.mid, 1.5, { opacity: 0.7 }), svgPicture(factionAsset(faction, 'mark'), x0 + 78, 84, SHIELD_MARK, 'lcc-art-crest')]);
    };
    kids.push(shield(50, 'em', 'a'));
    kids.push(shield(W - 50 - 156, 'cr', 'h'));
    // the ACTIVE DECREE plate and the two rules (the HELD BY rule is the second one and is a single element)
    const plate = 'M232,94 L262,60 H654 L684,94 L654,128 H262 Z';
    kids.push(sv('path', { d: plate, fill: g('plate') }), strokePath(plate, g('gold'), 3.5), strokePath('M244,94 L270,68 H646 L672,94 L646,120 H270 Z', GOLD.lo, 1.5, { opacity: 0.8 }));
    // the two cloth standards: a rod with two finials, the cloth (gold edge, then the faction colour) down to the point, and the point itself, which is the approved picture of the gold-edged V with its
    // wheat ornament, placed where the approved plaque has it. The picture starts at y 464, under the last line of names, and fades into the cloth above it.
    const standardArt = (cx, fill, faction, tipX) => {
      const l = cx - 126, r = cx + 126;
      const tip = standardTipAsset(faction);
      const pic = tip.fallback ? null : sv('image', { class: 'lcc-art-tip', href: tip.src, x: tipX, y: 464, width: tip.width, height: tip.height, preserveAspectRatio: 'none', mask: 'url(#' + p + 'tip)' });
      if (pic) pic.addEventListener('error', () => { pic.setAttribute('visibility', 'hidden'); });
      return sv('g', null, [
        sv('rect', { x: l - 20, y: 189, width: 292, height: 10, rx: 5, fill: g('gold') }),
        sv('circle', { cx: l - 22, cy: 194, r: 10, fill: g('gold') }), sv('circle', { cx: r + 22, cy: 194, r: 10, fill: g('gold') }),
        sv('path', { d: 'M' + l + ',206 H' + r + ' V494 H' + l + ' Z', fill: g('gold') }),
        sv('path', { d: 'M' + (l + 6) + ',212 H' + (r - 6) + ' V494 H' + (l + 6) + ' Z', fill: g(fill) }),
        sv('path', { d: 'M' + (l + 6) + ',390 H' + (r - 6) + ' V494 H' + (l + 6) + ' Z', fill: g('shade') }),
        svgPicture(factionAsset(faction, 'mark'), cx, 246, STANDARD_MARK, 'lcc-art-crest'),
        pic,
      ]);
    };
    kids.push(standardArt(190, 'em', 'a', 36), standardArt(mirror(190), 'cr', 'h', 1819));
    // the nameplate shapes, the two hollow diamonds with their lines (between the plate and the count), the session plate, the footer
    const name = (x0, w) => {
      const d = 'M' + x0 + ',602 L' + (x0 + 30) + ',578 H' + (x0 + w - 30) + ' L' + (x0 + w) + ',602 L' + (x0 + w - 30) + ',626 H' + (x0 + 30) + ' Z';
      return sv('g', null, [sv('path', { d, fill: g('gold') }), strokePath(d, GOLD.lo, 2), strokePath('M' + (x0 + 36) + ',583 H' + (x0 + w - 36), GOLD.hi, 1.5, { opacity: 0.8 })]);
    };
    kids.push(name(32, 336), name(W - 32 - 336, 336));
    const diamond = (cx) => sv('path', { d: 'M' + cx + ',616 L' + (cx + 18) + ',635 L' + cx + ',654 L' + (cx - 18) + ',635 Z', class: 'lcc-glow-edge', fill: '#090914', stroke: g('gold'), 'stroke-width': 3.5 });
    kids.push(sv('rect', { class: 'lcc-art-line lcc-glow-line', x: 52, y: 633.8, width: 464, height: 2.4, fill: g('goldh') }));
    kids.push(sv('rect', { class: 'lcc-art-line lcc-glow-line', x: mirror(516), y: 633.8, width: 464, height: 2.4, fill: g('goldh') }));
    kids.push(diamond(556), diamond(mirror(556)));
    const session = 'M646,523 L682,490 H1490 L1526,523 L1490,556 H682 Z';
    kids.push(sv('path', { d: session, fill: g('plate') }), strokePath(session, g('gold'), 3.5), strokePath('M660,523 L690,498 H1482 L1512,523 L1482,548 H690 Z', GOLD.lo, 1.4, { opacity: 0.8 }));
    // the two vertical rules are drawn last, so nothing in this layer is ever painted over them; the one at x 1577 is the HELD BY rule
    kids.push(sv('rect', { class: 'lcc-art-rule lcc-glow-line', x: 711, y: 36, width: 3.5, height: 98, rx: 1.5, fill: g('vr') }));
    kids.push(sv('rect', { class: 'lcc-art-rule lcc-art-rule-held lcc-glow-line', x: 1577, y: 36, width: 3.5, height: 98, rx: 1.5, fill: g('vr') }));
    return sv('svg', { class: 'lcc-art', viewBox: '0 0 ' + W + ' ' + H, preserveAspectRatio: 'xMidYMid meet', 'aria-hidden': 'true', focusable: 'false' }, kids);
  }

  // The track: the pointed body, the middle gem and its lower point, and the 25 seats. a and h are validated counts or null (the track is then drawn with all seats open and marked unavailable).
  // win13: the faction that has won by 13 Houses ('a' or 'h'), else null. Only then does the gold middle seat (the 13th) carry data-win13 and the ring that the style animates (the same gold arc as the
  // winning Houses, under the same two gates: Motion Full and no reduced-motion request). The still ring is the halo itself, drawn bolder; nothing moves the layout, it is all inside the seat's own circle.
  function track(p, a, h, win13) {
    const T = TRACK;
    const g = (id) => 'url(#' + p + id + ')';
    const defs = sv('defs', null, [
      grad(p + 'body', 0, 0, 1, 0, [[0, '#04382d'], [0.42, '#0a2a22'], [0.5, '#1d1409'], [0.58, '#3a0b0e'], [1, '#52060a']]),
      grad(p + 'rail', 0, 0, 1, 0, [[0, '#2de2c0'], [0.34, '#bda24a'], [0.5, '#ffe28a'], [0.66, '#d67a3a'], [1, '#ff3b30']]),
      grad(p + 'tg', 0, 0, 0, 1, [[0, GOLD.hi], [0.45, GOLD.mid], [1, GOLD.lo]]),
      rgrad(p + 'sa', [[0, '#5ff0c8'], [0.6, '#14a07c'], [1, '#0b6a52']]),
      rgrad(p + 'sh', [[0, '#ff7b6e'], [0.6, '#c4242a'], [1, '#8a1218']]),
      rgrad(p + 'so', [[0, '#f1dcb4'], [0.7, '#c9aa70'], [1, '#a08450']]),
      rgrad(p + 'sc', [[0, '#3a1a02'], [0.5, '#3e1d02'], [0.66, '#5a2a03'], [0.8, '#9a4e07'], [0.92, '#e08a0c'], [1, '#ffa514']]), // the dark brown interior of the 13th seat's ring with a warm orange light inside its rim (measured from the approved picture)
      grad(p + 'rg', 0, 0, 0, 1, [[0, '#ffd84a'], [0.114, '#fff6c8'], [0.2, '#ffd84a'], [0.5, '#f2b81c'], [0.8, '#ffd84a'], [0.823, '#fff6c8'], [1, '#ffd84a']]), // the ring's rim and wedge sides: white-hot where the ring is topmost and lowest (the gradient runs over the whole outline, tip to tip), deeper gold at the sides, gold along the wedge sides
      grad(p + 'wd', 0, 0, 0, 1, [[0, '#fff6b0'], [0.5, '#ffd641'], [1, '#e09f14']]), // the downward wedge: pale next to the ring, deeper gold towards the tip
      grad(p + 'wu', 0, 1, 0, 0, [[0, '#fff6b0'], [0.5, '#ffd641'], [1, '#e09f14']]), // the upward wedge, the same flipped
      grad(p + 'gb', 0, 0, 0, 1, [[0, '#ffe98a'], [0.5, '#f7c630'], [1, '#e8a213']]), // the bevel of the gem
      glow(p + 'tblur', 4),
      glow(p + 'cblur', 6, 30),
    ]);
    const body = 'M' + T.tipL + ',' + T.cy + ' L' + T.wallL + ',' + T.top + ' H' + T.wallR + ' L' + T.tipR + ',' + T.cy + ' L' + T.wallR + ',' + T.bottom + ' H' + T.wallL + ' Z';
    const inner = 'M' + (T.tipL + 24) + ',' + T.cy + ' L' + (T.wallL + 6) + ',' + (T.top + 12) + ' H' + (T.wallR - 6) + ' L' + (T.tipR - 24) + ',' + T.cy + ' L' + (T.wallR - 6) + ',' + (T.bottom - 12) + ' H' + (T.wallL + 6) + ' Z';
    const kids = [defs,
      sv('path', { class: 'lcc-glow lcc-glow-rail', d: body, fill: 'none', stroke: g('rail'), 'stroke-width': 14, opacity: 0.35, filter: g('tblur') }),
      sv('path', { d: body, fill: g('body') }),
      sv('path', { d: inner, fill: 'none', stroke: '#000', 'stroke-width': 10, opacity: 0.25 }),
      sv('path', { d: body, fill: 'none', stroke: g('tg'), 'stroke-width': 7, 'stroke-linejoin': 'miter' }),
      sv('path', { class: 'lcc-glow-line', d: inner, fill: 'none', stroke: g('rail'), 'stroke-width': 3, 'stroke-linejoin': 'miter' }),
    ];
    // the middle gem above the hub and the two wedges are part of the 13th seat now (drawn in its group, #983)
    // seats
    const states = seatStates(a, h) || seatStates(0, 0);
    const seats = sv('g', { class: 'lcc-seats' });
    for (const s of states) {
      const x = seatX(s.index);
      const sg = sv('g', { class: 'lcc-seat lcc-seat-' + s.state + (s.center ? ' lcc-seat-center' : ''), transform: 'translate(' + x + ',' + T.cy + ')', 'data-slot': s.slot, 'data-state': s.state, 'data-center': s.center ? 'true' : null });
      sg.appendChild(sv('title', null, [doc().createTextNode(seatLabel(s))]));
      if (s.center) {
        // the ring's outer edge is T.halo in both states: a bolder stroke (a won seat) is drawn inward
        const hw = win13 ? HALO_W_WIN : HALO_W;
        const o = centreOutline(hw), gem = centreGem();
        // behind: the glow of the outline; then the two wedges, the dark interior with the warm light inside its rim, the ring and the wedge sides as one outline (the ring runs over the wedge bases),
        // the creases of the wedges, then the separate faceted gem
        sg.appendChild(sv('path', { class: 'lcc-glow lcc-glow-seat', d: o.d, fill: 'none', stroke: GOLD.mid, 'stroke-width': hw + 2.5, 'stroke-linejoin': 'round', filter: g('cblur'), 'aria-hidden': 'true' }));
        sg.appendChild(sv('path', { class: 'lcc-seat-wedge lcc-seat-wedge-top', d: o.top, fill: g('wu') }));
        sg.appendChild(sv('path', { class: 'lcc-seat-wedge lcc-seat-wedge-bottom', d: o.bottom, fill: g('wd') }));
        sg.appendChild(sv('circle', { class: 'lcc-seat-base', r: o.r, fill: g('sc'), stroke: GOLD.hi, 'stroke-width': 1.5, 'stroke-opacity': 0.5 }));
        sg.appendChild(sv('path', { class: 'lcc-seat-halo', d: o.d, fill: 'none', stroke: g('rg'), 'stroke-width': hw, 'stroke-linejoin': 'round' }));
        for (const w of [o.up, o.down]) sg.appendChild(sv('path', { class: 'lcc-seat-crease', d: 'M0,' + (w === o.up ? -w.ty : w.ty) + ' V' + (w === o.up ? -w.tip : w.tip), fill: 'none', stroke: '#fff8d0', 'stroke-width': 1.2, opacity: 0.8, 'aria-hidden': 'true' }));
        const gm = sv('g', { class: 'lcc-seat-gem', 'aria-hidden': 'true' });
        gm.appendChild(sv('path', { class: 'lcc-glow lcc-glow-seat', d: gem.outline, fill: GOLD.mid, stroke: 'none', filter: g('cblur') }));
        gm.appendChild(sv('path', { class: 'lcc-seat-bevel', d: gem.outline, fill: g('gb') }));
        const tones = ['#ffee7a', '#ffdc3c', '#fcc41c', '#f2a50c']; // upper left, upper right, lower left, lower right: pale at the top, deeper gold below
        gem.facets.forEach((d, i) => gm.appendChild(sv('path', { class: 'lcc-seat-facet', d, fill: tones[i] })));
        gm.appendChild(sv('path', { class: 'lcc-seat-cross', d: gem.cross, fill: 'none', stroke: '#fffbe0', 'stroke-width': 1.2, opacity: 0.75 }));
        gm.appendChild(sv('path', { class: 'lcc-seat-inner', d: gem.inner, fill: 'none', stroke: '#b87a0c', 'stroke-width': 0.9, 'stroke-linejoin': 'round' }));
        gm.appendChild(sv('path', { class: 'lcc-glow-edge', d: gem.outline, fill: 'none', stroke: '#fbe39a', 'stroke-width': GEM_STROKE, 'stroke-linejoin': 'round' }));
        sg.appendChild(gm);
        if (win13) {
          sg.setAttribute('data-win13', win13);
          sg.setAttribute('class', sg.getAttribute('class') + ' lcc-seat-win13');
          // the travelling arc runs round the ring only: the two wedges are cut out of its clip, so it never crosses a wedge
          sg.appendChild(sv('clipPath', { id: p + 'ringclip' }, [sv('path', { d: 'M-90,-130 H90 V130 H-90 Z ' + o.top + ' ' + o.bottom, 'clip-rule': 'evenodd' })]));
          const sweepG = sv('g', { 'clip-path': g('ringclip') });
          sweepG.appendChild(sv('circle', { class: 'lcc-seat-win-sweep', r: T.halo - HALO_W_WIN / 2, fill: 'none', 'stroke-width': HALO_W_WIN, 'stroke-linecap': 'round', 'stroke-dasharray': Math.round(0.36 * 2 * Math.PI * (T.halo - HALO_W_WIN / 2)) + ' ' + Math.round(0.64 * 2 * Math.PI * (T.halo - HALO_W_WIN / 2)), 'aria-hidden': 'true' }));
          sg.appendChild(sweepG);
        }
      }
      if (!s.center) sg.appendChild(sv('circle', { class: 'lcc-seat-base', r: T.r, fill: g(s.state === 'a' ? 'sa' : s.state === 'h' ? 'sh' : 'so'), stroke: '#2a2112', 'stroke-width': 2 }));
      // a mark inside a claimed seat tells the two factions apart without colour: a V for Atreides, a ring for Harkonnen. It is the drawn fallback: the faction's picture covers it once the picture
      // has loaded (the round medallion on a claimed seat, the white crest on the gold 13th seat, as in the approved plaque), and a forced-colours view shows it instead of the picture.
      if (s.state === 'a') sg.appendChild(sv('path', { class: 'lcc-seat-mark', d: 'M-9,-6 L0,10 L9,-6 L0,-1 Z', fill: '#f4efe0' }));
      if (s.state === 'h') sg.appendChild(sv('circle', { class: 'lcc-seat-mark', r: 6, fill: 'none', stroke: '#f4efe0', 'stroke-width': 3 }));
      if (s.state === 'a' || s.state === 'h') {
        const pic = svgPicture(factionAsset(s.state, s.center ? 'mark' : 'medal'), 0, 0, s.center ? SEAT_MARK : SEAT_MEDAL, 'lcc-seat-art', () => { sg.setAttribute('class', sg.getAttribute('class') + ' lcc-seat-has-art'); });
        if (pic) { pic.setAttribute('overflow', 'visible'); sg.appendChild(pic); }
        if (pic && !s.center) sg.appendChild(sv('circle', { class: 'lcc-seat-rim', r: RIM_R, fill: 'none', stroke: SEAT_RIM[s.state], 'stroke-width': RIM_W, 'aria-hidden': 'true' })); // the picture's own box never cuts its edge (a fractional pixel scale could shave the ring)
      }
      seats.appendChild(sg);
    }
    kids.push(seats);
    return sv('svg', { class: 'lcc-track-svg', viewBox: '0 0 ' + W + ' ' + H, preserveAspectRatio: 'xMidYMid meet', focusable: 'false', role: 'img' }, kids);
  }

  // summaryText(a, h): the whole tally in words, for the track's label and for readers who do not see the seats.
  function summaryText(a, h) {
    if (!isCount(a) || !isCount(h) || a + h > SEATS) return 'The tally of the 25 seats is not available.';
    return 'Tally of the 25 seats: Atreides ' + a + ', Harkonnen ' + h + ', open ' + (SEATS - a - h) + '. Seats fill from the left for Atreides and from the right for Harkonnen. Seat 13 is the gold middle seat, the majority mark: 13 of 25 Houses.';
  }

  const TEXT = {
    decreeLabel: 'ACTIVE DECREE', noDecree: 'No active decree', noEffect: '', contributors: 'TOP CONTRIBUTORS', noContributors: 'Not in the save',
    atreides: 'HOUSE ATREIDES', harkonnen: 'HOUSE HARKONNEN', houses: 'HOUSES', unavailable: 'Tally not available',
  };

  // renderPlaque(vm): the approved decree and voting plaque as a read-only view of plain values.
  //   vm = { a, h, decree: { name, effect } | null, holder: 'a' | 'h' | null, contributors: { a: Entry[] | null, h: Entry[] | null } | null (an entry: a string, or { name, amount }), win, sessionLabel, standing, demo }
  // It decides nothing itself: the headline and the tally line come from WIN for the reading the caller gives (vm.win, from WIN.decide) or, with none, for the two counts alone; a caller may still replace
  // either line (vm.sessionLabel, vm.standing), and nothing here reads the save.
  function renderPlaque(vm) {
    vm = vm || {};
    const p = 'lcc' + (++uid) + '-';
    const ok = isCount(vm.a) && isCount(vm.h) && vm.a + vm.h <= SEATS;
    const a = ok ? vm.a : null, h = ok ? vm.h : null;
    const host = el('section', 'lcc lcc-plaque-host', '', { 'aria-label': 'Landsraad decree and voting plaque', 'data-seats': ok ? SEATS : 'unavailable', 'data-a': ok ? a : null, 'data-h': ok ? h : null });
    const root_ = el('div', 'lcc-plaque');
    host.appendChild(root_);
    root_.appendChild(underlay(p));
    const wd = vm.win || WIN().decideCounts(a, h);
    const tr = el('div', 'lcc-track');
    const svgTrack = track(p, a, h, wd.state === 'won' && wd.routes.includes('houses') ? wd.winner : null); // the orb moves for a win by 13 Houses only
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
    const sess = el('p', 'lcc-pl-session', vm.sessionLabel || WIN().headline(wd), { 'data-win': wd.state });
    if (wd.state === 'won') sess.setAttribute('title', WIN().TEXT.facts);
    root_.appendChild(sess);
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
    foot.appendChild(el('p', 'lcc-pl-standing', vm.standing != null ? vm.standing : vm.win ? WIN().tally(vm.win) : ok ? WIN().tally(wd) : TEXT.unavailable, { role: 'status' }));
    root_.appendChild(foot);
    if (vm.demo) root_.appendChild(badge('demo'));
    return host;
  }

  // ---- House card ------------------------------------------------------------------------------------------------------------------------------------------------------------------
  const OWNER = { a: 'Atreides', h: 'Harkonnen', get open() { return WORDS().notWon; }, get unknown() { return WORDS().unreadable; } }; // the words of favor come from lcc-model.js (#952)
  const OWNER_GLYPH = Object.freeze({ a: 'A', h: 'H', open: '○', unknown: '?' });
  const TRACK_LABEL = Object.freeze({ combat: 'Combat', crafting: 'Crafting', exploration: 'Exploration', gathering: 'Gathering', sabotage: 'Sabotage' });
  const houseTitle = (name) => String(name == null ? '' : name).replace(/^DA_House/i, '').replace(/^./, (c) => c.toUpperCase());
  // renderHouseCard(vm): vm = { name, owner: 'a'|'h'|'open'|'unknown', track, selected, oneAway, inLine (a House of a held line: the data-line mark), win (the House makes up a win), waiting }. A button: the whole card is the target.
  // The live board (M3) adds four optional fields, all off when absent: number (the tile number 01 to 25, a board position), meter ({ reached, total }: the tiers of the character's own
  // contribution), dim (the filters leave this House out: it stays on the board and says so) and compact (the five-by-five tile: the crest over the name, drawn small).
  const pipText = (m) => (m && Number.isInteger(m.total) && m.total > 0 && Number.isInteger(m.reached) && m.reached >= 0 && m.reached <= m.total ? m : null);
  function renderHouseCard(vm) {
    vm = vm || {};
    const owner = OWNER[vm.owner] ? vm.owner : 'unknown';
    const title = houseTitle(vm.name) || 'Unknown House';
    const b = el('button', 'lcc lcc-house', '', { type: 'button', 'aria-pressed': vm.selected ? 'true' : 'false', 'data-owner': owner, 'data-one-away': vm.oneAway ? 'true' : null, 'data-line': vm.inLine ? 'true' : null });
    if (vm.win) victory(b, true);
    if (vm.selected) b.setAttribute('data-selected', 'true');
    if (vm.dim) b.setAttribute('data-dim', 'true');
    if (vm.compact) b.setAttribute('data-compact', 'true');
    const no = typeof vm.number === 'string' && /^[0-9]{2}$/.test(vm.number) ? vm.number : '';
    if (no) b.appendChild(el('span', 'lcc-house-no', no, { 'aria-hidden': 'true' }));
    const crest = el('span', 'lcc-house-crest');
    const sym = houseAsset(vm.name);
    if (sym) crest.appendChild(art(sym, 'lcc-house-img', 40)); else crest.appendChild(el('span', 'lcc-house-initials', title.slice(0, 2).toUpperCase()));
    b.appendChild(crest);
    const body = el('span', 'lcc-house-body');
    body.appendChild(el('span', 'lcc-house-name', title));
    body.appendChild(el('span', 'lcc-owner lcc-owner-' + owner, owner === 'a' || owner === 'h' ? OWNER[owner] : OWNER_GLYPH[owner] + ' ' + OWNER[owner], { 'data-g': OWNER_GLYPH[owner] })); // one mark (#950): a faction is its name in its colour, never a letter and the name; data-g: the glyph alone, for the narrow tile
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
    if (vm.win) { flags.appendChild(el('span', 'lcc-flag lcc-flag-win', VICTORY.win)); notes.push('one of the Houses that won the Sysselraad'); }
    if (vm.waiting > 0) { flags.appendChild(el('span', 'lcc-flag lcc-flag-wait', vm.waiting + ' WAITING')); notes.push(vm.waiting + ' reward' + (vm.waiting === 1 ? '' : 's') + ' waiting'); }
    const mt = pipText(vm.meter);
    if (mt) {
      const pips = el('span', 'lcc-house-meter', '', { 'aria-hidden': 'true' });
      for (let i = 0; i < mt.total; i++) pips.appendChild(el('i', i < mt.reached ? 'on' : ''));
      b.appendChild(pips);
      notes.push(mt.reached + ' of ' + mt.total + ' ' + WORDS().tiersEarned);
    }
    if (flags.childNodes.length) b.appendChild(flags);
    b.setAttribute('aria-label', [(no ? WORDS().square + ' ' + no + ', ' : '') + title + ', ' + (owner === 'a' || owner === 'h' ? WORDS().favorWon.toLowerCase() + ' by ' + OWNER[owner] : owner === 'open' ? WORDS().favorNotWon.toLowerCase() : WORDS().favorUnreadable.toLowerCase())].concat(vm.track && TRACK_LABEL[norm(vm.track)] ? [WORDS().specialization.toLowerCase() + ' ' + TRACK_LABEL[norm(vm.track)]] : [], notes, vm.dim ? ['not in the current filter'] : [], vm.selected ? ['selected'] : []).join(', '));
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
    config, houseAsset, laneAsset, bannerAsset, emblemAsset, itemAsset, ITEM_PICS, decreeAsset, factionAsset, standardTipAsset,
    seatStates, seatLabel, seatX, seatOuter, centreOutline, centreGem, standingLine, summaryText, laneState,
    pill, ruleV, heldBy, standard, badge, richTip, frame, victory, VICTORY, winAnimOn,
    renderPlaque, renderHouseCard, renderLaneCard, el, houseTitle,
  };
  if (typeof module !== 'undefined' && module.exports) module.exports = api;
  else root.LCC = api;
})(typeof window !== 'undefined' ? window : globalThis);
