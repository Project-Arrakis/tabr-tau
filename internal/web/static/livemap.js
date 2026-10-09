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
    for (const v of d.vehicles || []) add('vehicles', v.x, v.y, v.name);
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
    const byType = {};
    for (const p of lm.points) if (p.type) { const t = byType[p.type] || (byType[p.type] = { type: p.type, group: p.g, n: 0, found: 0 }); t.n++; if (p.d > 0) t.found++; }
    const types = Object.values(byType).sort((a, b) => b.n - a.n);
    const groups = LM_GROUPS.filter((g) => counts[g.id]);
    setHTML($('#main'), html`
      <div class="card"><h3>Live Map</h3>
        <div class="row">${(d.maps || []).map((m) => html`<button class="b ${m.key == d.config.key ? '' : 'sec'}" data-lmmap="${m.key}">${m.label}</button>`)}
          <button class="b sec" data-lmund="1">${lm.showUndiscovered ? 'Hide' : 'Show'} undiscovered markers</button></div>
        <p class="mut">The map as of your last save, not the running game. Markers you have not discovered yet are shown faded.</p>
        <div class="lm-wrap"><canvas id="lmc" class="lm"></canvas></div>
        <p class="mut" id="lminfo">Hover a marker for details. Drag to move, scroll to zoom.</p>
        <div class="row">${groups.map((g) => html`<button class="lm-layer ${lm.hidden.has(g.id) ? 'off' : 'on'}" data-lmg="${g.id}"><span class="lm-sw lm-c-${g.id}"></span>${g.label} (${counts[g.id]})</button>`)}</div></div>
      <div class="card"><h3>Markers on this map (${types.length} types)</h3>${tbl([{ k: 'type', label: 'Type' }, { k: 'group', label: 'Group', f: (r) => (LM_GROUPS.find((g) => g.id == r.group) || {}).label || r.group }, { k: 'n', label: 'Markers' }, { k: 'found', label: 'Discovered' }], types)}</div>`);
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
