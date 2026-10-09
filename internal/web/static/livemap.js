// Live Map tab: the save's own view of a map (Hagga Basin, Deep Desert): the character, your vehicles, your base, its storage
// containers and the game's own map markers. It shows the save as of its last write, not the running game. The picture and the
// world-to-picture conversion are the Dune Docker console's (see the notices); everything text comes from the save and is
// escaped by the html template or set as text.

(function () {
  // Self-contained: html and setHTML come from window.TabrHTML (app.js's own constants are not shared between scripts), and only
  // liveMapView is published. api(), tbl() and render() are app.js functions, looked up when the tab is opened.
  const { html, setHTML } = window.TabrHTML;
  const $ = (s) => document.querySelector(s);

  // One colour per group; the same palette is in app.css (lm-c-*) for the legend swatches, because inline styles are blocked.
  const LM_GROUPS = [
    { id: 'character', label: 'Character', color: '#ffd24a' },
    { id: 'vehicles', label: 'Your vehicles', color: '#4ec9ff' },
    { id: 'bases', label: 'Your base', color: '#ff7a45' },
    { id: 'storage', label: 'Storage containers', color: '#d6a45a' },
    { id: 'ore', label: 'Ore and minerals', color: '#c58bff' },
    { id: 'salvage', label: 'Salvage', color: '#9aa4b2' },
    { id: 'flora', label: 'Plants', color: '#6fd36f' },
    { id: 'enemy', label: 'Enemy camps', color: '#ff4d4d' },
    { id: 'places', label: 'Places', color: '#f5e6a8' },
    { id: 'npcs', label: 'Trainers and House representatives', color: '#ffb3de' },
    { id: 'hazard', label: 'Hazards', color: '#ff9f1a' },
    { id: 'other', label: 'Other', color: '#7f8c8d' },
  ];
  const LM_COLOR = Object.fromEntries(LM_GROUPS.map((g) => [g.id, g.color]));

  // Marker pictures: the console's, by marker subtype (lower case). A type with no picture keeps its coloured dot. The files are
  // the ones in maps/icons (see the notices); the names below are fixed here, never taken from the save.
  const LM_ICONS = {
    assaultornithopter: 'assaultornivehicle.webp',
    atreidesfortress: 'atreidesfortress.webp',
    azuriteore: 'azuriteore.webp',
    azuritepickup: 'azuriteore.webp',
    basaltore: 'basaltore.webp',
    basaltpickup: 'basaltore.webp',
    bauxiteore: 'bauxiteore.webp',
    bauxitepickup: 'bauxiteore.webp',
    buggy: 'buggyvehicle.webp',
    cave: 'cave.webp',
    containervehicle: 'cargocontainervehicle.webp',
    dolomitepickup: 'dolomiterock.webp',
    dolomiterock: 'dolomiterock.webp',
    ecolab: 'ecolab.webp',
    enemycamp: 'enemycamp.webp',
    enemylaboroutpost: 'enemylaboroutpost.webp',
    enemyoutpost: 'enemyoutpost.webp',
    erythriteore: 'erythriteore.webp',
    erythritepickup: 'erythriteore.webp',
    fuelcellpart: 'fuelcellwreckage.webp',
    fuelcellwreckage: 'fuelcellwreckage.webp',
    harkonnenfortress: 'harkonnenfortress.webp',
    houserepresentativeargosaz: 'houserepresentativeargosaz.webp',
    houserepresentativedyvetz: 'houserepresentativedyvetz.webp',
    houserepresentativeecaz: 'houserepresentativeecaz.webp',
    houserepresentativehagal: 'houserepresentativehagal.webp',
    houserepresentativehurata: 'houserepresentativehurata.webp',
    houserepresentativeimota: 'houserepresentativeimota.webp',
    houserepresentativekenola: 'houserepresentativekenola.webp',
    houserepresentativelindaren: 'houserepresentativelindaren.webp',
    houserepresentativemaros: 'houserepresentativemaros.webp',
    houserepresentativemikarrol: 'houserepresentativemikarrol.webp',
    houserepresentativemoritani: 'houserepresentativemoritani.webp',
    houserepresentativenovebruns: 'houserepresentativenovebruns.webp',
    houserepresentativerichese: 'houserepresentativerichese.webp',
    houserepresentativesor: 'houserepresentativesor.webp',
    houserepresentativetaligari: 'houserepresentativetaligari.webp',
    houserepresentativethorvald: 'houserepresentativethorvald.webp',
    houserepresentativevernius: 'houserepresentativevernius.webp',
    houserepresentativewayku: 'houserepresentativewayku.webp',
    houserepresentativewydras: 'houserepresentativewydras.webp',
    jasmiumore: 'jasmiumore.webp',
    jasmiumpickup: 'jasmiumore.webp',
    lightornithopter: 'ornithoptervehicle.webp',
    magnetiteore: 'magnetiteore.webp',
    magnetitepickup: 'magnetiteore.webp',
    mediumornithopter: 'ornithoptervehicle.webp',
    ornithopter: 'ornithoptervehicle.webp',
    primrosefield: 'PrimroseField.png',
    rhyoliteore: 'rhyoliteore.webp',
    rhyolitepickup: 'rhyoliteore.webp',
    saguaroseed: 'resourcesaguarorawr.webp',
    sandbike: 'sandbikevehicle.webp',
    sandcrawler: 'sandcrawlervehicle.webp',
    scrapmetalpart: 'scrapmetalwreckage.webp',
    scrapmetalwreckage: 'scrapmetalwreckage.webp',
    shipwreck: 'shipwreck.webp',
    sietch: 'sietch.webp',
    stravidiumore: 'stravidiumore.webp',
    stravidiumpickup: 'stravidiumore.webp',
    taxiservice: 'taxiservice.webp',
    titaniumore: 'titaniumore.webp',
    titaniumpickup: 'titaniumore.webp',
    tradingpost: 'tradingpost.webp',
    trainerbenegesserit: 'trainerbenegesserit.webp',
    trainermentat: 'trainermentat.webp',
    trainerplanetologist: 'trainerplanetologist.webp',
    trainerswordmaster: 'trainerswordmaster.webp',
    trainertrooper: 'trainertrooper.webp',
    transportornithopter: 'ornithoptervehicle.webp',
    treadwheel: 'treadwheelvehicle.webp',
  };
  const LM_GROUP_ICON = { character: 'Characters.webp', bases: 'Base.webp', storage: 'storage.png' };
  const LM_ICON_SIZE = { character: 36, bases: 36, vehicles: 32, storage: 20 }; // screen pixels, as the console sizes them; other markers 22
  // A picture by marker type. The type comes from the save, so it is looked up as an own key only ("__proto__" or "constructor" must
  // not find anything).
  function lmIconByType(type) {
    const k = String(type || '').toLowerCase();
    return Object.hasOwn(LM_ICONS, k) ? LM_ICONS[k] : '';
  }
  const lmImgs = new Map();
  function lmImage(file) {
    let i = lmImgs.get(file);
    if (!i) { i = new Image(); i.onload = lmDraw; i.src = '/maps/icons/' + file; lmImgs.set(file, i); }
    return i.complete && i.naturalWidth ? i : null;
  }
  // Which picture a point gets, or '' for none. A vehicle's picture follows its blueprint name.
  function lmIconFile(p) {
    if (LM_GROUP_ICON[p.g]) return LM_GROUP_ICON[p.g];
    if (p.g === 'vehicles') {
      const c = String(p.cls || '').toLowerCase();
      for (const k of ['assaultornithopter', 'ornithopter', 'sandcrawler', 'treadwheel', 'buggy', 'sandbike']) if (c.includes(k)) return LM_ICONS[k];
      return LM_ICONS.sandbike;
    }
    return lmIconByType(p.type);
  }

  const lm = { map: 'HaggaBasin', data: null, points: [], hidden: new Set(), showUndiscovered: true, scale: 1, ox: 0, oy: 0, img: null, hover: null };

  // Marker type (as the game names it) to a group.
  function lmGroupOf(type) {
    const t = String(type || '');
    if (/^(Hazard_)/.test(t)) return 'hazard';
    if (/^(Enemy)/.test(t)) return 'enemy';
    if (/^(HouseRepresentative|Trainer)/.test(t)) return 'npcs';
    if (/(Ore|Pickup|Rock)$/.test(t) && !/^(ScrapMetal|FuelCell)/.test(t)) return 'ore';
    if (/^(ScrapMetal|FuelCell|Shipwreck)/.test(t)) return 'salvage';
    if (/^(PrimroseField|BrittleBush)/.test(t)) return 'flora';
    if (/^(Cave|Ecolab|Sietch|TradingPost|HomeBase|AtreidesFortress|HarkonnenFortress|ControlPoint|ExplorationPointOfInterest|SurveyPoint)/.test(t)) return 'places';
    return 'other';
  }

  function lmBuildPoints(d) {
    const pts = [];
    const add = (g, x, y, label, extra) => {
      const px = Number(x), py = Number(y);
      if (Number.isFinite(px) && Number.isFinite(py)) pts.push({ g, x: px, y: py, label: String(label ?? ''), d: 3, ...extra });
    };
    if (d.character) add('character', d.character.x, d.character.y, d.character.name || 'Character');
    for (const v of d.vehicles || []) add('vehicles', v.x, v.y, v.name, { cls: String(v.class ?? '') });
    for (const b of d.bases || []) add('bases', b.x, b.y, 'Your base');
    for (const s of d.storage || []) add('storage', s.x, s.y, s.name);
    for (const m of d.markers || []) add(lmGroupOf(m.t), m.x, m.y, m.t, { d: Number(m.d) || 0, type: String(m.t ?? '') });
    return pts;
  }

  // World to picture pixels (the console's liveMapGeometry): linear, no flip.
  function lmToPixel(cfg, x, y) {
    return { px: ((x - cfg.minX) / (cfg.maxX - cfg.minX)) * cfg.width, py: ((y - cfg.minY) / (cfg.maxY - cfg.minY)) * cfg.height };
  }
  function lmVisible(p) { return !lm.hidden.has(p.g) && (lm.showUndiscovered || p.d > 0); }

  function lmCanvas() { return document.querySelector('#lmc'); }
  function lmFit(c) {
    const cfg = lm.data.config;
    lm.scale = Math.min(c.width / cfg.width, c.height / cfg.height);
    lm.ox = (c.width - cfg.width * lm.scale) / 2;
    lm.oy = (c.height - cfg.height * lm.scale) / 2;
  }

  function lmDraw() {
    const c = lmCanvas();
    if (!c || !lm.data) return;
    let ctx = null;
    try { ctx = c.getContext('2d'); } catch (e) { ctx = null; }
    if (!ctx) return; // no canvas (a test engine): the legend and tables still work
    const cfg = lm.data.config;
    ctx.setTransform(1, 0, 0, 1, 0, 0);
    ctx.fillStyle = '#0d0f12';
    ctx.fillRect(0, 0, c.width, c.height);
    ctx.setTransform(lm.scale, 0, 0, lm.scale, lm.ox, lm.oy);
    if (lm.img && lm.img.complete && lm.img.naturalWidth) ctx.drawImage(lm.img, 0, 0, cfg.width, cfg.height);
    else { ctx.strokeStyle = '#2a2f36'; ctx.lineWidth = 2 / lm.scale; ctx.strokeRect(0, 0, cfg.width, cfg.height); }
    const r = Math.max(2.5, 4 / lm.scale * 1.0);
    // Your own things are drawn last so the thousands of markers never cover them.
    const MINE = { bases: 1, storage: 2, vehicles: 3, character: 4 }; // draw order among your own things; the big base ring is the bottom layer, its storage sits on it
    const isMine = (p) => p.g in MINE;
    for (const p of [...lm.points.filter((q) => !isMine(q)), ...lm.points.filter(isMine).sort((a, b) => MINE[a.g] - MINE[b.g])]) {
      if (!lmVisible(p)) continue;
      const { px, py } = lmToPixel(cfg, p.x, p.y);
      const file = lmIconFile(p);
      const img = file ? lmImage(file) : null;
      if (img) { // a picture, drawn at a fixed size on screen whatever the zoom
        // your own things keep their size; the thousands of map markers shrink when the whole map is in view, so they do not hide it
        const size = LM_ICON_SIZE[p.g] || Math.min(22, Math.max(10, 60 * lm.scale + 6));
        ctx.save();
        ctx.setTransform(1, 0, 0, 1, 0, 0);
        ctx.globalAlpha = p.d > 0 ? 1 : 0.4;
        ctx.drawImage(img, px * lm.scale + lm.ox - size / 2, py * lm.scale + lm.oy - size / 2, size, size);
        ctx.restore();
        continue;
      }
      ctx.beginPath();
      const big = isMine(p);
      ctx.arc(px, py, big ? r * (p.g === 'bases' ? 2.4 : 1.8) : r, 0, Math.PI * 2);
      ctx.fillStyle = LM_COLOR[p.g] || '#7f8c8d';
      ctx.globalAlpha = p.d > 0 ? 1 : 0.35;
      ctx.fill();
      ctx.globalAlpha = 1;
      if (big) { ctx.lineWidth = 1.5 / lm.scale; ctx.strokeStyle = '#000'; ctx.stroke(); }
    }
  }

  function lmNearest(c, mx, my) {
    const cfg = lm.data.config;
    let best = null, bd = 10 * 10;
    for (const p of lm.points) {
      if (!lmVisible(p)) continue;
      const { px, py } = lmToPixel(cfg, p.x, p.y);
      const sx = px * lm.scale + lm.ox, sy = py * lm.scale + lm.oy;
      const d2 = (sx - mx) ** 2 + (sy - my) ** 2;
      if (d2 < bd) { bd = d2; best = p; }
    }
    return best;
  }

  function lmMount() {
    const c = lmCanvas();
    if (!c) return;
    c.width = Math.max(300, c.parentElement.clientWidth || 900);
    c.height = 560;
    lmFit(c);
    const cfg = lm.data.config;
    lm.img = new Image();
    lm.img.onload = lmDraw;
    lm.img.src = cfg.image;
    const rect = (e) => { const b = c.getBoundingClientRect(); return { x: (e.clientX - b.left) * (c.width / (b.width || c.width)), y: (e.clientY - b.top) * (c.height / (b.height || c.height)) }; };
    let drag = null;
    c.addEventListener('mousedown', (e) => { drag = { x: e.clientX, y: e.clientY, ox: lm.ox, oy: lm.oy }; });
    window.addEventListener('mouseup', () => { drag = null; });
    c.addEventListener('mousemove', (e) => {
      if (drag) { lm.ox = drag.ox + (e.clientX - drag.x); lm.oy = drag.oy + (e.clientY - drag.y); lmDraw(); return; }
      const m = rect(e);
      const p = lmNearest(c, m.x, m.y);
      lm.hover = p;
      const info = document.querySelector('#lminfo');
      if (info) info.textContent = p ? `${p.label}  (${Math.round(p.x)}, ${Math.round(p.y)})${p.d > 0 ? '' : '  not discovered yet'}` : 'Hover a marker for details. Drag to move, scroll to zoom.';
    });
    c.addEventListener('wheel', (e) => {
      e.preventDefault();
      const m = rect(e);
      const k = e.deltaY < 0 ? 1.2 : 1 / 1.2;
      const next = Math.min(Math.max(lm.scale * k, 0.05), 6);
      const f = next / lm.scale;
      lm.ox = m.x - (m.x - lm.ox) * f;
      lm.oy = m.y - (m.y - lm.oy) * f;
      lm.scale = next;
      lmDraw();
    }, { passive: false });
    lmDraw();
  }

  async function liveMapView() {
    const d = await api('/api/livemap?map=' + encodeURIComponent(lm.map));
    lm.data = d;
    lm.points = lmBuildPoints(d);
    const counts = {};
    for (const p of lm.points) counts[p.g] = (counts[p.g] || 0) + 1;
    const byType = new Map(); // a Map: the type comes from the save, and a plain object would lose a type named "__proto__"
    for (const p of lm.points) if (p.type) { let t = byType.get(p.type); if (!t) { t = { type: p.type, group: p.g, n: 0, found: 0 }; byType.set(p.type, t); } t.n++; if (p.d > 0) t.found++; }
    const types = [...byType.values()].sort((a, b) => b.n - a.n);
    const groups = LM_GROUPS.filter((g) => counts[g.id]);
    setHTML($('#main'), html`
      <div class="card"><h3>Live Map</h3>
        <div class="row">${(d.maps || []).map((m) => html`<button class="b ${m.key == d.config.key ? '' : 'sec'}" data-lmmap="${m.key}">${m.label}</button>`)}
          <button class="b sec" data-lmund="1">${lm.showUndiscovered ? 'Hide' : 'Show'} undiscovered markers</button></div>
        <p class="mut">The map as of your last save, not the running game. Markers you have not discovered yet are shown faded.</p>
        <div class="lm-wrap"><canvas id="lmc" class="lm"></canvas></div>
        <p class="mut" id="lminfo">Hover a marker for details. Drag to move, scroll to zoom.</p>
        <div class="row">${groups.map((g) => html`<button class="lm-layer ${lm.hidden.has(g.id) ? 'off' : 'on'}" data-lmg="${g.id}">${LM_GROUP_ICON[g.id] ? html`<span class="lm-ic lm-i-${LM_GROUP_ICON[g.id].replace(/\.[a-z]+$/, '').toLowerCase()}"></span>` : html`<span class="lm-sw lm-c-${g.id}"></span>`}${g.label} (${counts[g.id]})</button>`)}</div></div>
      <div class="card"><h3>Markers on this map (${types.length} types)</h3>${tbl([{ k: 'type', label: 'Type', f: (r) => { const f = lmIconByType(r.type); return f ? html`<span class="lm-ic lm-i-${f.replace(/\.[a-z]+$/, '').toLowerCase()}"></span>${r.type}` : r.type; } }, { k: 'group', label: 'Group', f: (r) => (LM_GROUPS.find((g) => g.id == r.group) || {}).label || r.group }, { k: 'n', label: 'Markers' }, { k: 'found', label: 'Discovered' }], types)}</div>`);
    lmMount();
  }

  document.addEventListener('click', (e) => {
    const m = e.target.closest('[data-lmmap]');
    if (m) { lm.map = m.dataset.lmmap; render(); return; }
    const g = e.target.closest('[data-lmg]');
    if (g) { const id = g.dataset.lmg; if (lm.hidden.has(id)) lm.hidden.delete(id); else lm.hidden.add(id); g.classList.toggle('off'); g.classList.toggle('on'); lmDraw(); return; }
    if (e.target.closest('[data-lmund]')) { lm.showUndiscovered = !lm.showUndiscovered; render(); }
  });

  window.liveMapView = liveMapView;
})();
