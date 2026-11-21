// #1073: the words of the Landsraad and of a House, by state. The Landsraad as a whole is "still divided", "swayed" by a faction whose decree is yet to be chosen, or has the decree chosen; a House
// is "Pledged to <Faction>", "Undecided" or "Wavering", never "won" (a faction does not win a House, and nothing is won in the game's own words). Real app.js and lcc scripts, synthetic answer.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import { boot, closeAll } from './boot.mjs';
import { synthetic } from './lcc-fixture.mjs';

afterEach(closeAll);
const text = (el) => (el ? el.textContent.replace(/\s+/g, ' ').trim() : '');
async function open(mod, local) {
  const a = synthetic();
  if (mod) mod(a);
  const ui = await boot({ get: { '/api/landsraad': a }, local: Object.assign({ tab: 'landsraad', 'lsb.view': 'board', 'lsb.layout': 'grid' }, local || {}) });
  await ui.settle();
  return { ui, a };
}
const fact = (ui, k) => ui.doc.querySelector('#lcc-term [data-fact="' + k + '"]');
const nextCC = (ui) => fact(ui, 'elected').querySelector('.lcc-tm-v');
const nextClassic = (ui) => ui.doc.querySelector('#lsb [data-next]');

const STATES = {
  divided: { mod: (a) => { a.term.reigning_faction_id = null; a.term.elected_decree_id = null; }, text: 'The Landsraad is still divided' },
  swayed: { mod: (a) => { a.term.reigning_faction_id = 2; a.term.elected_decree_id = null; }, text: 'Harkonnen have swayed the Landsraad; their decree is yet to be chosen', tip: "The controlling faction has not selected next term's decree." },
  unreadable: { mod: (a) => { a.term.elected_decree_id = 99999; }, text: '—', tip: 'Not recorded in the save.' },
};

for (const [name, s] of Object.entries(STATES)) {
  test('Next term\'s decree, ' + name + ': the Command Center Term section says it, with its hover (#1073)', async () => {
    const { ui } = await open(s.mod);
    const v = nextCC(ui);
    assert.equal(text(v), s.text);
    assert.equal(v.getAttribute('data-next'), name);
    if (s.tip) assert.equal(v.getAttribute('title').replace(/’/g, "'"), s.tip);
  });
  test('Next term\'s decree, ' + name + ': the Classic Term panel says the same words (#1073)', async () => {
    const { ui } = await open(s.mod, { 'lsb.view': 'classic' });
    const v = nextClassic(ui);
    assert.ok(v, 'the Classic next-term value');
    assert.equal(text(v), s.text);
    assert.equal(v.getAttribute('data-next'), name);
    if (s.tip) assert.equal(v.getAttribute('title').replace(/’/g, "'"), s.tip);
  });
}

test('Next term\'s decree, decree chosen: the decree is shown as before, no state sentence (#1073)', async () => {
  const { ui } = await open();
  assert.match(text(fact(ui, 'elected')), /Ranged/i);
  assert.doesNotMatch(text(fact(ui, 'elected')), /divided|swayed/);
  const c = await open(null, { 'lsb.view': 'classic' });
  assert.equal(c.ui.doc.querySelector('#lsb [data-next]'), null, 'no state word when a decree is chosen');
});

test('the operator\'s save state: Harkonnen in control, no decree chosen, reads as swayed on both pages (#1073)', async () => {
  const mod = (a) => { a.term.reigning_faction_id = 2; a.term.elected_decree_id = null; };
  const cc = await open(mod);
  assert.doesNotMatch(text(fact(cc.ui, 'elected')), /none yet/i);
  assert.match(text(fact(cc.ui, 'elected')), /^Next term's decrees*Harkonnen have swayed the Landsraad; their decree is yet to be chosen$/);
  const cl = await open(mod, { 'lsb.view': 'classic' });
  assert.match(text(cl.ui.doc.querySelector('#lsb .tkv')), /Harkonnen have swayed the Landsraad; their decree is yet to be chosen/);
});

// ---- a House: Pledged to X / Undecided / Wavering ------------------------------------------------------------------------------------------------------------------------------------------
const card = (ui, name) => [...ui.doc.querySelectorAll('.lcc-hc')].find((c) => text(c.querySelector('.lcc-hc-name, h3, .lcc-hc-title')).endsWith(name));
const KEN = 'DA_HouseKenola';

test('a House card says Pledged to <Faction> when a faction has it, Undecided when none has progress, Wavering when both have progress and neither has it (#1073)', async () => {
  const { ui, a } = await open((x) => {
    const k = x.tasks.find((t) => t.house_name === KEN); // both factions have progress in the synthetic save
    const r = x.tasks.find((t) => t.house_name === 'DA_HouseRichese'); // pledged to Atreides
    const v = x.tasks.find((t) => t.house_name === 'DA_HouseVernius'); // pledged to Harkonnen
    assert.ok(k && r && v);
    const u = x.tasks.find((t) => !t.completed && t !== k && t.contributions.length >= 0 && t.house_name !== KEN);
    u.contributions = []; u.completed = 0; u.winning_faction_id = null; u.__undecided = true;
  });
  const own = (h) => text(card(ui, h.house_name.replace('DA_House', ''))?.querySelector('.lcc-hc-owner'));
  const find = (n) => a.tasks.find((t) => t.house_name === n);
  assert.equal(own(find('DA_HouseRichese')), 'Pledged to Atreides');
  assert.equal(own(find('DA_HouseVernius')), 'Pledged to Harkonnen');
  assert.equal(own(find(KEN)), 'Wavering');
  assert.equal(own(a.tasks.find((t) => t.__undecided)), 'Undecided');
});

test('the legend, the House filter and the list column use the same words (#1073)', async () => {
  const { ui } = await open();
  const legend = text(ui.doc.querySelector('.lcc-bd-headlines'));
  assert.match(legend, /PLEDGED TO ATREIDES/);
  assert.match(legend, /PLEDGED TO HARKONNEN/);
  assert.match(legend, /UNDECIDED/);
  const opts = [...ui.doc.querySelectorAll('#lcc-bd-menu [role="radiogroup"][aria-label="House"] .lcc-k-tab')].map((o) => o.firstChild.textContent); // the pledge row of the Filter menu (tau #974)
  assert.ok(opts.includes('Undecided') && opts.includes('Pledged'), 'the filter values: ' + opts.join(' | '));
  const list = await open(null, { 'lsb.layout': 'list' });
  assert.match(text(list.ui.doc.querySelector('.lcc-bd-list, .lcc-bd-listhead, [role="table"], .lcc-bd-board')), /Pledged to/);
});

test('the Inspector heading says what the House is: Pledged to X, Undecided or Wavering, never "favor not won" (#1073)', async () => {
  const { ui, a } = await open();
  const pick = async (name) => { ui.doc.querySelector('[data-fk="ht:' + a.tasks.find((t) => t.house_name === name).id + '"]').click(); await ui.settle(); return text(ui.doc.querySelector('.lcc-bd-insp .lcc-bd-banner')); };
  assert.match(await pick('DA_HouseRichese'), /PLEDGED TO HOUSE ATREIDES/);
  assert.match(await pick(KEN), /WAVERING/);
  assert.doesNotMatch(text(ui.doc.querySelector('.lcc-bd-insp')), /\bwon\b|NOT WON|NO FAVOR/i);
});

test('no House or Landsraad text of the board says "won" or "no favor": a faction pledges a House, the Sysselraad is secured (#1073)', async () => {
  for (const view of ['board', 'classic']) {
    const { ui, a } = await open(null, { 'lsb.view': view });
    // open the first House so the Inspector words are on the page too
    if (view === 'board') { ui.doc.querySelector('[data-fk="ht:' + a.tasks.find((t) => t.house_name === KEN).id + '"]').click(); await ui.settle(); }
    const root = ui.doc.querySelector('#lsb') || ui.doc.body;
    const all = text(root) + ' | ' + [...root.querySelectorAll('[title],[aria-label]')].map((e) => (e.getAttribute('title') || '') + ' ' + (e.getAttribute('aria-label') || '')).join(' | ');
    // the one old line kept on purpose is the write-gate sentence of the Classic list (a golden of request equivalence names it), which says a House "won by" nothing the user sees
    const rest = all.replace(/Pick a faction[^|]*won[^|.]*\./gi, '');
    assert.doesNotMatch(rest, /\bwon\b|\bwon by\b|favor won|not won|no favor\b/i, view + ': ' + (/(.{40})\b(won|no favor)\b(.{20})/i.exec(rest) || [])[0]);
  }
});

test('the Classic copy of the House words (for a page without the Command Center scripts) says what the model says (#1073)', async () => {
  const fs = await import('node:fs'), path = await import('node:path'), url = await import('node:url');
  const root = path.join(path.dirname(url.fileURLToPath(import.meta.url)), '..');
  const app = fs.readFileSync(path.join(root, 'internal', 'web', 'static', 'app.js'), 'utf8');
  const m = /const LB_WORDS = Object\.freeze\(\{([^}]*)\}\)/.exec(app);
  assert.ok(m, 'the copy is there');
  const { ui } = await open();
  const W = ui.w.LCCM.WORDS;
  for (const [, k, v] of m[1].matchAll(/(\w+): '([^']*)'/g)) assert.equal(v, W[k], k);
});
