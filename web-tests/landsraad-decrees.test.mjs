// The term and decrees panel of the Landsraad tab (part 4 of the tab rework; refs #448, #608, #634): the decree pool tells the decree in effect apart from the pool flag and the five
// special vendor decrees apart from each other, decrees are shown by a readable name with the game's internal id as a small second line, and the controls write what they say.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { boot, closeAll } from './boot.mjs';

afterEach(closeAll);

// The plain SpecialVendorActive row (id 7 here) is not shown anywhere on the page (operator order); the rows below are the data, the page leaves id 7 out.
// the decree rows and the term of a save read in the game: term 10 has decree 8 (SpecialVendorActive_Vehicles) active, and the rotation held 1, 10 and 4
const NAMES = ['ExperienceRateIncrease', 'RangedDamageIncreased', 'MeleeDamageIncreased', 'CraftingCostReduced', 'DropInventoryOnDefeatActive', 'RepairAndRefiningTimes',
  'SpecialVendorActive', 'SpecialVendorActive_Vehicles', 'SpecialVendorActive_Weapons', 'SpecialVendorActive_Armor', 'SpecialVendorActive_Utilities'];
const FLAG = [1, 1, 1, 1, 0, 1, 0, 1, 1, 1, 1];
// what the server adds to every decree row (#608): the game's name and effect line, from the same file the server reads
const GAME = JSON.parse(readFileSync(new URL('../internal/ops/decree-names.json', import.meta.url), 'utf8')).decrees;
const named = (n) => { const g = GAME[n]; return g ? { display_name: g.name + (g.variant ? ` (${g.variant})` : ''), variant: g.variant || '', effect: g.effect || '', name_known: !g.derived } : { display_name: '', variant: '', effect: '', name_known: false }; };
const decrees = () => NAMES.map((n, i) => ({ id: i + 1, decree_name: n, disabled: FLAG[i], weight: 1, active: i + 1 === 8, elected: i + 1 === 5, in_rotation: [1, 10, 4].includes(i + 1), ...named(n) }));

const landsraad = (over = {}) => ({
  term: { term_id: 10, start_time: 1, end_time: 2, reigning_faction_id: 2, active_decree_id: 8, elected_decree_id: 5 },
  factions: [{ id: 1, name: 'Atreides' }, { id: 2, name: 'Harkonnen' }],
  character: { player_id: 1, name: 'Tester', faction_id: 1 },
  decrees: decrees(), rotation: [{ decree_id: 1 }, { decree_id: 10 }, { decree_id: 4 }], tasks: [],
  ...over,
});

async function open(data = landsraad(), post = {}) {
  const ui = await boot({ get: { '/api/landsraad': data }, post: { '/api/landsraad/decree': { ok: true, outcome: 'written' }, '/api/landsraad/term': { ok: true, outcome: 'written' }, ...post } });
  ui.click('[data-tab="landsraad"]'); await ui.settle();
  return ui;
}

const poolRow = (ui, id) => ui.doc.querySelector(`#lsb-tdp .dp [data-lb="tflag"][data-id="${id}"]`).closest('li');
const text = (el) => el.textContent.replace(/\s+/g, ' ').trim();
const parts = (el) => [...el.children].map(text).join(' ');
const posted = (ui, path) => ui.posted.filter((p) => p.path === path);
const select = (ui, sel, v) => { const el = ui.doc.querySelector(sel); el.value = String(v); el.dispatchEvent(new ui.w.Event('change', { bubbles: true })); };

test('the panel is closed, its heading line names the term, the reigning faction and the active decree, and the facts are in it', async () => {
  const ui = await open();
  const d = ui.doc.querySelector('#lsb-tdp');
  assert.ok(d && !d.open, 'closed by default');
  assert.equal(text(d.querySelector('summary')), 'Term and decrees Term 10 · Harkonnen · Exclusive Vehicle Import Rights');
  const kv = text(d.querySelector('.tkv'));
  assert.ok(kv.includes('Term10') && kv.includes('Reigning factionHarkonnen') && kv.includes('Active decreeExclusive Vehicle Import RightsSpecialVendorActive_Vehicles') && kv.includes('Next term\'s decreeThe Right of SalvageDropInventoryOnDefeatActive'), kv);
  assert.ok(!ui.doc.querySelector('.lsb-prev'), 'the old section of previous controls is gone');
  // the term's time is one value, Ends in, never seconds (#981); with no play clock in this answer it is not known
  assert.ok(kv.includes('Ends innot known'), kv);
  assert.ok(!/\d\s?s\b|Start|End\b/.test(kv.replace('Reigning factionHarkonnen', '')), kv);
});

test('decrees are shown by the name the game shows with the internal id as a small second line', async () => {
  const ui = await open();
  const got = (id) => parts(poolRow(ui, id).querySelector('.dc'));
  assert.equal(got(1), 'Scholarly Collaboration Experience gain is increased by 50% ExperienceRateIncrease');
  assert.equal(got(5), 'The Right of Salvage ' + GAME.DropInventoryOnDefeatActive.effect + ' DropInventoryOnDefeatActive');
  assert.equal(got(6), 'CHOAM Refining Contract Repair costs and refining times are reduced by 75% RepairAndRefiningTimes');
  assert.equal(got(8), 'Exclusive Vehicle Import Rights Access to a Landsraad Vehicle Vendor SpecialVendorActive_Vehicles');
  assert.ok(poolRow(ui, 6).querySelector('.dn').title.includes('75%'));
  assert.ok(poolRow(ui, 1).querySelector('.dn').title.includes('50%'), 'the effect line is the tooltip');
  assert.equal(poolRow(ui, 1).querySelector('.du'), null, 'a name the game shows carries no unconfirmed note');
  assert.equal(poolRow(ui, 8).querySelector('.did').title, "The game's internal id, kept for support");
});

test('every decree the editor knows shows its game name on the page, in the pool, the drop-down and the term, and the two Honor of the Fallen decrees are told apart', async () => {
  const ui = await open();
  const optionText = (id) => text(ui.doc.querySelector(`#lsb-tad option[value="${id}"]`));
  NAMES.forEach((n, i) => {
    const id = i + 1, want = named(n).display_name;
    assert.ok(want && want !== n, `${n} has a name in decree-names.json`);
    if (n === 'SpecialVendorActive') { assert.equal(ui.doc.querySelector(`#lsb-tad option[value="${id}"]`), null, 'the plain row is not in the drop-down'); assert.equal(ui.doc.querySelector(`[data-lb="tflag"][data-id="${id}"]`), null, 'nor in the pool'); return; }
    assert.equal(optionText(id), want + (named(n).name_known ? '' : " (not the game's name)"), `the drop-down names decree ${id}`);
    const row = poolRow(ui, id);
    assert.equal(text(row.querySelector('.dn')), want, `the pool names decree ${id}`);
    assert.equal(text(row.querySelector('.did')), n, `the id stays as the second line of decree ${id}`);
  });
  assert.notEqual(optionText(2), optionText(3), 'Ranged and Melee share one game name and show a qualifier');
  assert.equal(optionText(2), 'In Honor of the Fallen (Ranged)');
  assert.ok(poolRow(ui, 2).querySelector('.dn').title.includes('is added here to tell them apart'), 'the tooltip says the bracket is the editors addition');
  assert.equal(optionText(3), 'In Honor of the Fallen (Melee)');
});

test('a name that is only the editors wording, and an id the editor has never seen, still show and are marked unconfirmed', async () => {
  const extra = { id: 99, decree_name: 'BrandNewDecree_FromAPatch', disabled: 0, weight: 1, active: false, elected: false, in_rotation: false, ...named('BrandNewDecree_FromAPatch') };
  const ui = await open(landsraad({ decrees: [...decrees(), extra] }));
  const row = poolRow(ui, 99);
  assert.equal(text(row.querySelector('.dn')), 'Brand New Decree: FromAPatch', 'a readable form of the id');
  assert.equal(text(row.querySelector('.did')), 'BrandNewDecree_FromAPatch');
  assert.equal(text(row.querySelector('.du')), 'no game name for this id');
  assert.ok(row.querySelector('.dn').title.includes('No game name is known'));
  assert.ok(row.querySelector('[data-lb="tflag"]'), 'its controls still draw');
  assert.equal(text(ui.doc.querySelector('#lsb-tad option[value="99"]')), 'Brand New Decree: FromAPatch (no game name)');
  // a server that sends no display_name at all (an older one) still draws every row
  const bare = decrees().map(({ display_name, variant, effect, name_known, ...r }) => r);
  const ui2 = await open(landsraad({ decrees: bare }));
  assert.equal(text(poolRow(ui2, 1).querySelector('.dn')), 'Experience Rate Increase');
});

test('the effect of a decree is visible text in its pool row and under the drop-down, not only a tooltip, and the drop-down is sorted by name', async () => {
  const ui = await open();
  assert.equal(text(poolRow(ui, 1).querySelector('.de')), 'Experience gain is increased by 50%', 'read without a hover');
  assert.equal(text(poolRow(ui, 5).querySelector('.de')), GAME.DropInventoryOnDefeatActive.effect);
  // under the drop-down: the effect of the chosen decree follows the choice
  const lab = () => ui.doc.querySelector('#lsb-tad').closest('label');
  assert.equal(text(lab().querySelector('.de')), 'Access to a Landsraad Vehicle Vendor');
  select(ui, '#lsb-tad', 1);
  assert.equal(text(lab().querySelector('.de')), 'Experience gain is increased by 50%');
  select(ui, '#lsb-tad', 6);
  assert.equal(text(lab().querySelector('.du')), '');
  // the order: by the name a player knows, the name that is not the game's last, "none" first
  const opts = [...ui.doc.querySelectorAll('#lsb-tad option')].map((o) => text(o));
  assert.deepEqual(opts, ['none', 'CHOAM Production Contract', 'CHOAM Refining Contract', 'Exclusive Armor Import Rights', 'Exclusive Tool Import Rights', 'Exclusive Vehicle Import Rights', 'Exclusive Weapon Import Rights',
    'In Honor of the Fallen (Melee)', 'In Honor of the Fallen (Ranged)', 'Scholarly Collaboration', 'The Right of Salvage']);
  // the pool stays in id order, keyed by the ID column
  assert.deepEqual([...ui.doc.querySelectorAll('#lsb-tdp .tbody > ul.dp [data-lb="tflag"]')].map((b) => +b.dataset.id), [1, 2, 3, 4, 5, 6, 8, 9, 10, 11]);
});

test('the buttons of a pool row carry the decree name in their accessible name, and the unconfirmed note is in the summary line and the Applied result', async () => {
  const ui = await open();
  assert.equal(poolRow(ui, 2).querySelector('[data-lb="tflag"]').getAttribute('aria-label'), 'Set flag to 0: In Honor of the Fallen (Ranged)');
  assert.equal(poolRow(ui, 2).querySelector('[data-lb="tmk"]').getAttribute('aria-label'), 'Make active for Atreides: In Honor of the Fallen (Ranged)');
  // a decree with no game name: the note is in the summary line and the result
  const odd = { id: 99, decree_name: 'BrandNewDecree_FromAPatch', disabled: 0, weight: 1, active: true, elected: false, in_rotation: false, ...named('BrandNewDecree_FromAPatch') };
  const ui2 = await open(landsraad({ decrees: [...decrees(), odd], term: { term_id: 10, start_time: 1, end_time: 2, reigning_faction_id: 2, active_decree_id: 99, elected_decree_id: 5 } }));
  assert.equal(text(ui2.doc.querySelector('#lsb-tdp summary')), "Term and decrees Term 10 · Harkonnen · Brand New Decree: FromAPatch (no game name)");
  ui2.doc.querySelector('#lsb-tdp').open = true;
  select(ui2, '#lsb-tad', 99);
  ui2.click('[data-lb="tapply"]'); await ui2.settle();
  assert.ok(text(ui2.doc.querySelector('#lsb-tresult')).includes("Applied: active decree Brand New Decree: FromAPatch (no game name)"), text(ui2.doc.querySelector('#lsb-tresult')));
});

test('only the decree the term has active is marked Active in the pool, and it is the Vehicles variant, not SpecialVendorActive', async () => {
  const ui = await open();
  const tags = (id) => [...poolRow(ui, id).querySelectorAll('.tag')].map((t) => t.textContent.trim());
  assert.deepEqual(tags(8), ['Active']);
  assert.deepEqual(tags(5), ['Next term\'s decree']);
  assert.deepEqual(tags(10), ['In rotation']);
  assert.equal(ui.doc.querySelector('[data-lb="tflag"][data-id="7"]'), null, 'the base row SpecialVendorActive has no row on the page');
});

test('the pool flag is shown as the stored number, never as enabled or disabled, and the About text says it does not mean in effect', async () => {
  const ui = await open();
  const pool = text(ui.doc.querySelector('#lsb-tdp .dp'));
  assert.ok(!/\benabled\b/i.test(pool) && !/\bdisabled\b/i.test(pool), pool);
  assert.equal(text(poolRow(ui, 8).querySelector('.fl')), '1');
  assert.ok(/does not say whether a decree is in effect/.test(ui.doc.querySelector('#lsdecnote').textContent));
  assert.ok(ui.doc.querySelector('#lsdecnote').closest('details.about'), 'the long text is in an About disclosure');
});

test('the four category vendors are listed under their own names with their own ids; the plain fifth row is not shown', async () => {
  const ui = await open();
  const got = [8, 9, 10, 11].map((id) => text(poolRow(ui, id).querySelector('.did')));
  assert.deepEqual(got, ['SpecialVendorActive_Vehicles', 'SpecialVendorActive_Weapons', 'SpecialVendorActive_Armor', 'SpecialVendorActive_Utilities']);
});

test('the flag button of one row sends that row only', async () => {
  const ui = await open();
  assert.match(text(poolRow(ui, 5).querySelector('[data-lb="tflag"]')), /Set flag to 1/);
  assert.match(text(poolRow(ui, 8).querySelector('[data-lb="tflag"]')), /Set flag to 0/);
  ui.click('#lsb-tdp [data-lb="tflag"][data-id="5"]'); await ui.settle();
  const sent = posted(ui, '/api/landsraad/decree');
  assert.equal(sent.length, 1);
  assert.deepEqual(sent[0].body, { id: 5, disabled: true });
  assert.deepEqual(posted(ui, '/api/landsraad/term'), [], 'the flag does not touch the term');
});

test('the reigning faction drop-down starts on the character\'s own faction; Apply sends the two drop-downs in one request', async () => {
  const ui = await open();
  assert.equal(ui.doc.querySelector('#lsb-trf').value, '1', 'the character is in Atreides');
  assert.ok(text(ui.doc.querySelector('#lsb-trf')).includes('Atreides (your faction)'));
  assert.equal(ui.doc.querySelector('#lsb-tad').value, '8');
  const panel = ui.doc.querySelector('#lsb-tdp'); panel.open = true; panel.dispatchEvent(new ui.w.Event('toggle'));
  select(ui, '#lsb-tad', 6);
  assert.equal(text(ui.doc.querySelector('#lsb-tad').closest('label').querySelector('.did')), 'RepairAndRefiningTimes', 'the id line follows the choice');
  select(ui, '#lsb-trf', 2);
  ui.click('#lsb-tdp [data-lb="tapply"]'); await ui.settle();
  const sent = posted(ui, '/api/landsraad/term');
  assert.equal(sent.length, 1);
  assert.deepEqual(sent[0].body, { active_decree_id: '6', reigning_faction_id: '2' });
  assert.ok(ui.doc.querySelector('#lsb-tdp').open, 'the panel stays open after the write');
  assert.ok(text(ui.doc.querySelector('#lsb-tresult')).includes('Applied: active decree CHOAM Refining Contract, reigning faction Harkonnen'), text(ui.doc.querySelector('#lsb-tresult')));
});

test('none clears the decree and keeps the faction; the faction drop-down has no none (#960)', async () => {
  const ui = await open();
  select(ui, '#lsb-tad', '');
  ui.click('#lsb-tdp [data-lb="tapply"]'); await ui.settle();
  assert.deepEqual(posted(ui, '/api/landsraad/term')[0].body, { active_decree_id: '', reigning_faction_id: '1' });
  // the faction drop-down offers the two factions of the Landsraad and nothing else while one is chosen
  assert.deepEqual([...ui.doc.querySelectorAll('#lsb-trf option')].map((o) => text(o)), ['Atreides (your faction)', 'Harkonnen']);
});

test('no faction chosen: the drop-down shows a neutral placeholder that cannot be chosen, Apply is refused with the reason, nothing is sent (#960)', async () => {
  const ui = await open();
  select(ui, '#lsb-trf', '');
  const opts = [...ui.doc.querySelectorAll('#lsb-trf option')];
  assert.deepEqual(opts.map((o) => text(o)), ['Choose a faction', 'Atreides (your faction)', 'Harkonnen']);
  assert.ok(opts[0].disabled && opts[0].selected && opts[0].value === '', 'the placeholder is the selected option and cannot be picked');
  assert.ok(!opts.some((o) => /none/i.test(text(o))), 'no option says none');
  ui.click('#lsb-tdp [data-lb="tapply"]'); await ui.settle();
  assert.deepEqual(posted(ui, '/api/landsraad/term'), [], 'nothing is written');
  const r = ui.doc.querySelector('#lsb-tresult');
  assert.ok(text(r).startsWith('Choose a reigning faction first'), text(r));
  assert.ok(r.classList.contains('bad'), 'shown as a refusal');
  // choosing a faction removes the placeholder and Apply writes again
  select(ui, '#lsb-trf', 2);
  assert.deepEqual([...ui.doc.querySelectorAll('#lsb-trf option')].map((o) => text(o)), ['Atreides (your faction)', 'Harkonnen']);
  ui.click('#lsb-tdp [data-lb="tapply"]'); await ui.settle();
  assert.deepEqual(posted(ui, '/api/landsraad/term').map((p) => p.body), [{ active_decree_id: '8', reigning_faction_id: '2' }]);
});

test('Make active on a pool row sends the same request as Apply for that decree and the chosen faction; the row that is already active shows Active', async () => {
  const ui = await open(landsraad({ term: { term_id: 10, start_time: 1, end_time: 2, reigning_faction_id: 1, active_decree_id: 8, elected_decree_id: 5 } }));
  assert.ok(text(poolRow(ui, 8)).includes('✓ Active'), 'decree 8 is active for Atreides, the faction the drop-down starts on');
  assert.ok(!poolRow(ui, 8).querySelector('[data-lb="tmk"]'));
  const b = poolRow(ui, 9).querySelector('[data-lb="tmk"]');
  assert.equal(text(b), 'Make active for Atreides');
  ui.click('#lsb-tdp [data-lb="tmk"][data-id="9"]'); await ui.settle();
  assert.deepEqual(posted(ui, '/api/landsraad/term').map((p) => p.body), [{ active_decree_id: '9', reigning_faction_id: '1' }]);
});

test('Make active follows the faction drop-down, and with none chosen it is disabled and sends nothing', async () => {
  const ui = await open();
  select(ui, '#lsb-trf', 2);
  assert.equal(text(poolRow(ui, 9).querySelector('[data-lb="tmk"]')), 'Make active for Harkonnen');
  assert.ok(text(poolRow(ui, 8)).includes('✓ Active'), 'decree 8 is active for Harkonnen, the faction the term has');
  select(ui, '#lsb-trf', '');
  const b = poolRow(ui, 9).querySelector('[data-lb="tmk"]');
  assert.equal(b.getAttribute('aria-disabled'), 'true');
  ui.click('#lsb-tdp [data-lb="tmk"][data-id="9"]'); await ui.settle();
  assert.deepEqual(posted(ui, '/api/landsraad/term'), []);
  assert.ok(text(ui.doc.querySelector('#lsb-tresult')).includes('Choose a reigning faction'));
});

test('a reigning faction the list does not offer is shown as none, and Apply leaves it alone: the request names no faction and no button names it', async () => {
  // a character with no faction, and a term whose reigning faction (4) is not one the Landsraad has: the list offers 1 and 2 only
  const ui = await open(landsraad({ term: { term_id: 10, start_time: 1, end_time: 2, reigning_faction_id: 4, active_decree_id: 8, elected_decree_id: 5 }, character: { player_id: 1, name: 'Tester', faction_id: null } }));
  assert.equal(ui.doc.querySelector('#lsb-trf').value, '', 'the drop-down shows none, not a faction that is not offered');
  const panel = ui.doc.querySelector('#lsb-tdp'); panel.open = true; panel.dispatchEvent(new ui.w.Event('toggle'));
  const b = poolRow(ui, 9).querySelector('[data-lb="tmk"]');
  assert.equal(text(b), 'Make active (choose a faction)');
  assert.ok(!/Faction 4/.test(text(ui.doc.querySelector('#lsb-tdp .dp'))), 'no pool row names the faction that is not offered');
  select(ui, '#lsb-tad', 6);
  ui.click('#lsb-tdp [data-lb="tapply"]'); await ui.settle();
  assert.deepEqual(posted(ui, '/api/landsraad/term').map((p) => p.body), [{ active_decree_id: '6' }], 'the faction is not part of the request');
  assert.ok(text(ui.doc.querySelector('#lsb-tresult')).includes('reigning faction left as it is'), text(ui.doc.querySelector('#lsb-tresult')));
  // choosing a faction that is offered sends it; no faction chosen on purpose is no longer a request at all (a decree needs a faction, #960)
  select(ui, '#lsb-trf', 2);
  ui.click('#lsb-tdp [data-lb="tapply"]'); await ui.settle();
  select(ui, '#lsb-trf', '');
  ui.click('#lsb-tdp [data-lb="tapply"]'); await ui.settle();
  assert.deepEqual(posted(ui, '/api/landsraad/term').slice(1).map((p) => p.body), [{ active_decree_id: '6', reigning_faction_id: '2' }]);
  assert.ok(text(ui.doc.querySelector('#lsb-tresult')).startsWith('Choose a reigning faction first'));
});

test('a save with a decree and no faction can still have its decree cleared: none sends the decree only, and a decree still needs a faction (#960)', async () => {
  // the game wrote a term with an active decree and no reigning faction; the character is in no faction, so the drop-down shows the placeholder
  const ui = await open(landsraad({ term: { term_id: 10, start_time: 1, end_time: 2, reigning_faction_id: null, active_decree_id: 8, elected_decree_id: 5 }, character: { player_id: 1, name: 'Tester', faction_id: null } }));
  assert.equal(ui.doc.querySelector('#lsb-trf').value, '');
  assert.equal(text(ui.doc.querySelector('#lsb-trf option')), 'Choose a faction');
  // keeping the decree with no faction chosen is still refused
  ui.click('#lsb-tdp [data-lb="tapply"]'); await ui.settle();
  assert.deepEqual(posted(ui, '/api/landsraad/term'), []);
  assert.ok(text(ui.doc.querySelector('#lsb-tresult')).startsWith('Choose a reigning faction first'));
  // none for the decree: the request names the decree only, so no faction is written
  select(ui, '#lsb-tad', '');
  ui.click('#lsb-tdp [data-lb="tapply"]'); await ui.settle();
  assert.deepEqual(posted(ui, '/api/landsraad/term').map((p) => p.body), [{ active_decree_id: '' }]);
  assert.ok(text(ui.doc.querySelector('#lsb-tresult')).includes('active decree none, reigning faction left as it is'), text(ui.doc.querySelector('#lsb-tresult')));
});

test('a save without a term says so, and a hostile decree name is shown as text', async () => {
  let ui = await open(landsraad({ term: null }));
  assert.ok(text(ui.doc.querySelector('#lsb-term-h')).includes('no Landsraad term'));
  const evil = decrees(); evil[0].decree_name = '<img src=x onerror=alert(1)>';
  ui = await open(landsraad({ decrees: evil }));
  assert.ok(!ui.doc.querySelector('#lsb-tdp img'), 'no element was made from the name');
  assert.ok(text(poolRow(ui, 1)).includes('<img src=x onerror=alert(1)>'));
});

test('the plain SpecialVendorActive row is shown nowhere on the page: not in the pool, no closed list, not in the Active decree drop-down; the four category vendors stay', async () => {
  const ui = await open();
  const inPool = [...ui.doc.querySelectorAll('#lsb-tdp ul.dp [data-lb="tflag"]')].map((b) => +b.dataset.id);
  assert.deepEqual(inPool, [1, 2, 3, 4, 5, 6, 8, 9, 10, 11], 'the other ten stay, with the four category vendors');
  assert.equal(ui.doc.querySelector('#lsb-dphid'), null, 'no closed list under the pool');
  assert.equal(ui.doc.querySelector('[data-id="7"]'), null, 'no control of row 7');
  assert.equal(ui.doc.querySelector('#lsb-tad option[value="7"]'), null, 'not in the Active decree drop-down');
  const all = ui.doc.body.textContent.replace(/SpecialVendorActive_\w+/g, '');
  assert.ok(!/Special Vendor/.test(all), 'no "Special Vendor" wording');
  assert.ok(!/SpecialVendorActive/.test(all), 'no plain SpecialVendorActive id');
  assert.ok(!/Hidden: the plain/.test(all), 'no hidden-row heading');
  assert.ok(/Exclusive Vehicle Import Rights/.test(ui.doc.body.textContent) && /SpecialVendorActive_Vehicles/.test(ui.doc.body.textContent), 'the category vendors still show');
});

test('a term whose active decree is the plain row says "Another decree" with no id; the drop-down keeps that one value, and Apply writes it back unchanged', async () => {
  const ui = await open(landsraad({ term: { term_id: 10, start_time: 1, end_time: 2, reigning_faction_id: 2, active_decree_id: 7, elected_decree_id: 7 } }));
  assert.equal(text(ui.doc.querySelector('#lsb-tdp summary')), 'Term and decrees Term 10 · Harkonnen · Another decree');
  const kv = text(ui.doc.querySelector('#lsb-tdp .tkv'));
  assert.ok(kv.includes('Active decreeAnother decree') && kv.includes("Next term's decreeAnother decree"), kv);
  assert.ok(!/Special Vendor|SpecialVendorActive/.test(ui.doc.body.textContent.replace(/SpecialVendorActive_\w+/g, '')), 'no plain name or id anywhere');
  const sel = ui.doc.querySelector('#lsb-tad');
  assert.equal(sel.value, '7', "the choice stays on the term's decree, so the drop-down does not fall back to none");
  assert.equal(text(sel.querySelector('option[value="7"]')), 'Another decree');
  ui.doc.querySelector('#lsb-tdp').open = true;
  ui.click('[data-lb="tapply"]'); await ui.settle();
  assert.deepEqual(posted(ui, '/api/landsraad/term').map((p) => p.body), [{ active_decree_id: '7', reigning_faction_id: '1' }], 'the row is written as the save holds it, never as none');
  assert.ok(text(ui.doc.querySelector('#lsb-tresult')).includes('Applied: active decree Another decree'), text(ui.doc.querySelector('#lsb-tresult')));
});

test('a save without the plain vendor decree draws the same page', async () => {
  const without = decrees().filter((d) => d.decree_name !== 'SpecialVendorActive');
  const ui = await open(landsraad({ decrees: without }));
  assert.equal(ui.doc.querySelector('#lsb-dphid'), null);
  assert.equal([...ui.doc.querySelectorAll('#lsb-tdp ul.dp [data-lb="tflag"]')].length, 10);
});

// #904: a click on Make active (or Apply) while a write is running changes nothing, and does not leave the other row's decree in the Active decree drop-down: the drop-down keeps what was chosen for the write that runs.
test('Make active on another row during a pending Apply sends nothing and leaves the drop-downs as they were (#904)', async () => {
  const ui = await open();
  const orig = ui.w.fetch; let release = null;
  ui.w.fetch = (p, opt = {}) => (opt.method === 'POST' && String(p).includes('/api/landsraad/term') ? new Promise((res) => { release = () => res(orig(p, opt)); }) : orig(p, opt));
  select(ui, '#lsb-tad', 2);
  ui.click('[data-lb="tapply"]'); await ui.settle();
  assert.ok(release, 'the Apply request is held');
  ui.click('#lsb-tdp [data-lb="tmk"][data-id="9"]'); await ui.settle();
  assert.ok(text(ui.doc.querySelector('#lsb-tresult')).includes('A write is still running'), text(ui.doc.querySelector('#lsb-tresult')));
  release(); await ui.settle();
  assert.deepEqual(posted(ui, '/api/landsraad/term').map((p) => p.body.active_decree_id), ['2'], 'one request, for the decree that was applied');
  assert.equal(ui.doc.querySelector('#lsb-tad').value, '2', 'the drop-down shows the decree of the write that ran, not the row that was clicked meanwhile');
});

// #1019: the running Apply is drawn aria-disabled; a second click on it says a write is running (the busy check comes first), it does not replace that line with the button's own title.
test('a second click on the running Apply says a write is running, not the title of the button (#1019)', async () => {
  const ui = await open();
  const orig = ui.w.fetch; let release = null;
  ui.w.fetch = (p, opt = {}) => (opt.method === 'POST' && String(p).includes('/api/landsraad/term') ? new Promise((res) => { release = () => res(orig(p, opt)); }) : orig(p, opt));
  select(ui, '#lsb-tad', 2);
  const b = ui.doc.querySelector('[data-lb="tapply"]');
  ui.click('[data-lb="tapply"]'); await ui.settle();
  assert.ok(release, 'the Apply request is held');
  assert.equal(b.getAttribute('aria-disabled'), 'true', 'the running button is drawn off');
  b.title = 'THE BUTTON TITLE'; // what the old order would have shown instead of the wait line
  ui.click('[data-lb="tapply"]'); await ui.settle();
  const said = text(ui.doc.querySelector('#lsb-tresult'));
  assert.ok(said.includes('A write is still running') && !said.includes('THE BUTTON TITLE'), said);
  release(); await ui.settle();
  assert.equal(posted(ui, '/api/landsraad/term').length, 1, 'one request');
});
