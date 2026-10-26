// What a page shows by default (#426). jsdom lays nothing out, so "shown" is defined here: the text of the page without the tooltip boxes (.tipbox, drawn only on hover or focus), without the
// body of a <details> that is not open (its <summary> stays), without [hidden] and without the screen-reader-only .sr. A line that moved to a tooltip or to "About this" is in the page but
// not in visibleText(); a line that stays (state, result, error, queued) is in both.
export function visibleText(root) {
  const out = [];
  const walk = (n) => {
    if (n.nodeType === 3) { out.push(n.nodeValue); return; }
    if (n.nodeType !== 1) return;
    const tag = n.tagName.toLowerCase();
    if (tag === 'script' || tag === 'style' || tag === 'template') return;
    if (n.hasAttribute('hidden') || n.classList.contains('tipbox') || n.classList.contains('sr')) return;
    if (tag === 'details' && !n.open) {
      const s = [...n.children].find((c) => c.tagName.toLowerCase() === 'summary');
      if (s) walk(s);
      return;
    }
    for (const c of n.childNodes) walk(c);
    if (/^(p|div|li|h3|tr|button|label)$/.test(tag)) out.push(' ');
  };
  walk(root);
  return out.join('').replace(/\s+/g, ' ').trim();
}

// The text of every tooltip box and every closed or open "About this" body in a root: where moved text is found.
export function movedText(root) {
  const out = [];
  for (const t of root.querySelectorAll('.tipbox')) out.push(t.textContent);
  for (const d of root.querySelectorAll('details.about')) out.push(d.textContent);
  return out.join(' ').replace(/\s+/g, ' ').trim();
}

// Every page of the editor, one after the other: calls fn(label) once each time a page is drawn. Used by the checks that must hold on all of them.
export async function eachPage(ui, fn) {
  for (const t of ['player', 'livemap', 'landsraad', 'config', 'db']) {
    ui.click(`[data-tab="${t}"]`); await ui.settle(); await fn(t);
    if (t === 'db') {
      for (const s of ['sql', 'backups', 'browse']) { ui.click(`[data-sub="db:${s}"]`); await ui.settle(); await fn('db:' + s); }
    }
    if (t !== 'player') continue;
    for (const s of ['character', 'crafting', 'research', 'buildingsets', 'customizations', 'skills', 'specialization', 'journey', 'bases', 'vehicles', 'admin']) {
      ui.click(`[data-sub="player:${s}"]`); await ui.settle(); await fn('player:' + s);
      if (s === 'character') {
        for (const c of ['inventory', 'reputation', 'carepackage', 'overview']) { ui.click(`[data-sub="char:${c}"]`); await ui.settle(); await fn('char:' + c); }
      }
      if (s === 'bases') {
        for (const b of ['water', 'inventory', 'claim', 'structures', 'power']) { const el = ui.doc.querySelector(`[data-bstab="${b}"]`); if (!el) continue; ui.click(`[data-bstab="${b}"]`); await ui.settle(); await fn('bases:' + b); }
      }
    }
  }
}
