// The reset buttons (#339, part of #112): a two-step flow with a typed sentence that names the area and the number, a minimum gap before
// the OK counts, the number sent with the request, a redraw after a refusal, and a disabled button (with its reason) when there is nothing to reset.
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import { boot, closeAll, GAP } from './boot.mjs';

afterEach(closeAll);

const SKILL = (o) => ({ id: 'Skills.Ability.X', name: 'X', school: 'Trooper', kind: 'Ability', rank: 1, max: 3, spent: 1, ladder: [1, 3, 6], known: true, ...o });
const skills = (resetCount, extra = {}) => ({ total: 10, unspent: 5, spent: 4, schools: [{ key: 'Trooper', label: 'Trooper' }], modules: [SKILL({})], resetCount, ...extra });

async function openSkills(ui) {
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="player:skills"]'); await ui.settle();
}
const posts = (ui, p) => ui.posted.filter((x) => x.path === p);

test('Reset skills: a typed sentence with the area and the count, a gap before OK counts, and the confirmed count goes with the request', async () => {
  const ui = await boot({ get: { '/api/player/skills': skills(12) }, post: { '/api/player/skills/reset': { ok: true, changed: 12, noop: false } } });
  await openSkills(ui);
  const btn = ui.doc.querySelector('[data-act="skReset"]');
  assert.ok(btn, 'the Skills tab has the reset button');
  assert.equal(btn.disabled, false);
  assert.ok(btn.classList.contains('bad'), 'a destructive button looks like the others');
  assert.equal(btn.textContent.trim(), 'Reset all skills to rank 0 (12)');
  assert.ok(ui.doc.getElementById(btn.getAttribute('aria-describedby')), 'the button is described by a note on the page');
  assert.equal(ui.doc.querySelectorAll('[data-act="skReset"]').length, 1, 'one button, no "reset everything"');

  ui.click('[data-act="skReset"]'); await ui.settle();
  const dlg = ui.doc.querySelector('#ask [role="dialog"]');
  assert.ok(dlg, 'the first press only asks');
  assert.equal(dlg.getAttribute('aria-modal'), 'true');
  assert.equal(posts(ui, '/api/player/skills/reset').length, 0, 'nothing is sent before the question is answered');
  const text = dlg.textContent;
  assert.ok(text.includes('Reset 12 skills to rank 0?'), 'the title names the area and the count');
  assert.ok(text.includes('Type reset 12 skills to continue'), 'the sentence to type names both');
  assert.ok(/backup/i.test(text) && text.includes('Restore'), 'it says the backup puts it back');
  assert.ok(text.includes('do not change'), 'with the box unticked the unspent points are said to stay');
  assert.equal(ui.doc.querySelector('#aok').disabled, true, 'OK is off until the sentence is typed');
  ui.type('reset 11 skills');
  assert.equal(ui.doc.querySelector('#aok').disabled, true, 'the wrong count does not enable it');
  ui.type('reset skills');
  assert.equal(ui.doc.querySelector('#aok').disabled, true);
  ui.type('Reset 12 skills ');
  assert.equal(ui.doc.querySelector('#aok').disabled, false);

  // a press that comes at once (the second half of a double click) is ignored; the question stays open
  ui.click('#aok'); await ui.settle();
  assert.equal(posts(ui, '/api/player/skills/reset').length, 0, 'an OK sooner than the gap is not a decision');
  assert.ok(ui.doc.querySelector('#ask [role="dialog"]'), 'the question is still open');
  // so is Enter in the box
  ui.doc.querySelector('#atype').dispatchEvent(new ui.w.KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
  await ui.settle();
  assert.equal(posts(ui, '/api/player/skills/reset').length, 0, 'Enter sooner than the gap is ignored too');

  await ui.wait(GAP);
  ui.click('#aok'); await ui.settle();
  assert.deepEqual(posts(ui, '/api/player/skills/reset').map((p) => p.body), [{ expected: 12, charge: false }]);
  assert.ok(ui.doc.querySelector('#ask').classList.contains('hide'), 'the question is closed');
  assert.ok(ui.doc.querySelector('#toast').textContent.includes('12 skills reset to rank 0'));
  ui.dom.window.close();
});

test('Reset skills: Enter in the box completes it after the gap, Escape and Cancel send nothing, and the ticked box is said and sent', async () => {
  const ui = await boot({ get: { '/api/player/skills': skills(1) }, post: { '/api/player/skills/reset': { ok: true, changed: 1, noop: false } } });
  await openSkills(ui);
  ui.doc.querySelector('#skCharge').checked = true;
  ui.doc.querySelector('#skCharge').dispatchEvent(new ui.w.Event('change', { bubbles: true })); await ui.settle();
  assert.equal(ui.doc.querySelector('[data-act="skReset"]').textContent.trim(), 'Reset all skills to rank 0 (1)');
  ui.click('[data-act="skReset"]'); await ui.settle();
  assert.ok(ui.doc.querySelector('#ask').textContent.includes('Type reset 1 skill to continue'), 'singular for one');
  assert.ok(ui.doc.querySelector('#ask').textContent.includes('given back'), 'the ticked box says the points are given back');
  ui.doc.dispatchEvent(new ui.w.KeyboardEvent('keydown', { key: 'Escape', bubbles: true })); await ui.settle();
  assert.ok(ui.doc.querySelector('#ask').classList.contains('hide'));
  ui.click('[data-act="skReset"]'); await ui.settle();
  ui.click('[data-act="askNo"]'); await ui.settle();
  assert.equal(posts(ui, '/api/player/skills/reset').length, 0, 'Escape and Cancel send nothing');
  ui.click('[data-act="skReset"]'); await ui.settle();
  ui.type('reset 1 skill');
  await ui.wait(GAP);
  ui.doc.querySelector('#atype').dispatchEvent(new ui.w.KeyboardEvent('keydown', { key: 'Enter', bubbles: true })); await ui.settle();
  assert.deepEqual(posts(ui, '/api/player/skills/reset').map((p) => p.body), [{ expected: 1, charge: true }]);
  ui.dom.window.close();
});

test('Reset skills: with nothing to reset the button is off and the page says why', async () => {
  const ui = await boot({ get: { '/api/player/skills': skills(0) } });
  await openSkills(ui);
  const btn = ui.doc.querySelector('[data-act="skReset"]');
  assert.equal(btn.disabled, true);
  assert.equal(btn.textContent.trim(), 'Reset all skills to rank 0 (0)');
  assert.ok(ui.doc.getElementById(btn.getAttribute('aria-describedby')).textContent.includes('already at rank 0'));
  ui.click('[data-act="skReset"]'); await ui.settle();
  assert.ok(ui.doc.querySelector('#ask').classList.contains('hide'), 'a disabled button opens nothing');
  ui.dom.window.close();
});

test('Reset skills: a refusal because the count changed is shown and the page is read again', async () => {
  const ui = await boot({ get: { '/api/player/skills': skills(3) }, post: { '/api/player/skills/reset': { __fail: 'the number of skills to reset is now 2, not the 3 you confirmed, so nothing was changed: reload the page and look again', __code: 'conflict', __status: 409 } } });
  await openSkills(ui);
  const reads = () => ui.fetched.filter((p) => p === '/api/player/skills').length;
  const before = reads();
  ui.click('[data-act="skReset"]'); await ui.settle();
  ui.type('reset 3 skills'); await ui.wait(GAP);
  ui.click('#aok'); await ui.settle();
  await ui.wait(20); await ui.settle();
  const err = ui.doc.querySelector('#toast .err');
  assert.ok(err && err.textContent.includes('you confirmed'), 'the refusal is shown, in the editor\'s words');
  assert.ok(reads() > before, 'the page is read again so that the new number is on it');
  ui.dom.window.close();
});

test('Reset skills: a no-op answer says nothing was changed', async () => {
  const ui = await boot({ get: { '/api/player/skills': skills(2) }, post: { '/api/player/skills/reset': { ok: true, changed: 0, noop: true } } });
  await openSkills(ui);
  ui.click('[data-act="skReset"]'); await ui.settle();
  ui.type('reset 2 skills'); await ui.wait(GAP);
  ui.click('#aok'); await ui.settle();
  assert.ok(ui.doc.querySelector('#toast').textContent.includes('already at rank 0'));
  ui.dom.window.close();
});

// ---- Level and XP, on Character > Overview

const summary = (o = {}) => ({ name: 'Tester', status: 'Offline', map: 'HaggaBasin', faction: 'Neutral', level: 71, xp: 41221, skillPoints: { unspent: 9, total: 70 },
  intel: { points: 1, max: 2779 }, vitals: {}, identity: {}, ids: {}, currency: [], ...o });

test('Reset level and XP: names the XP, says what happens to the skill points, and sends the XP that was confirmed', async () => {
  const ui = await boot({ get: { '/api/player/summary': summary() }, post: { '/api/player/level/reset': { ok: true, changed: 41221, noop: false, levelBefore: 71 } } });
  ui.click('[data-tab="player"]'); await ui.settle();
  const btn = ui.doc.querySelector('[data-act="lvReset"]');
  assert.ok(btn, 'the Character overview has the button');
  assert.equal(btn.textContent.trim(), 'Reset level and XP (41,221 XP)');
  assert.equal(btn.disabled, false);
  assert.ok(btn.classList.contains('bad'));
  assert.ok(ui.doc.getElementById(btn.getAttribute('aria-describedby')));
  ui.click('[data-act="lvReset"]'); await ui.settle();
  const text = ui.doc.querySelector('#ask').textContent;
  assert.ok(text.includes('Reset level 71 and 41,221 XP to 0?'));
  assert.ok(text.includes('Type reset 41221 xp to continue'));
  assert.ok(text.includes('now 70') && text.includes('now 9'), 'the skill points that will be taken back are named');
  assert.ok(/backup/i.test(text));
  assert.equal(posts(ui, '/api/player/level/reset').length, 0);
  ui.type('reset 41221 xp');
  ui.click('#aok'); await ui.settle();
  assert.equal(posts(ui, '/api/player/level/reset').length, 0, 'an OK within the gap is ignored');
  await ui.wait(GAP);
  ui.click('#aok'); await ui.settle();
  assert.deepEqual(posts(ui, '/api/player/level/reset').map((p) => p.body), [{ expected: 41221 }]);
  assert.ok(ui.doc.querySelector('#toast').textContent.includes('Level 71 and 41,221 XP reset to 0'));
});

test('Reset level and XP: off, with its reason, when the character has no XP or the save has no level record', async () => {
  for (const [sm, why] of [[summary({ xp: 0, level: 0 }), 'no XP'], [(() => { const s = summary(); delete s.xp; delete s.level; return s; })(), 'no level record']]) {
    const ui = await boot({ get: { '/api/player/summary': sm } });
    ui.click('[data-tab="player"]'); await ui.settle();
    const btn = ui.doc.querySelector('[data-act="lvReset"]');
    assert.equal(btn.disabled, true, why);
    assert.ok(ui.doc.getElementById(btn.getAttribute('aria-describedby')).textContent.length > 20, why);
    ui.click('[data-act="lvReset"]'); await ui.settle();
    assert.ok(ui.doc.querySelector('#ask').classList.contains('hide'), why);
    closeAll();
  }
});

// ---- XP of the specialization tracks

const specs = (resetCount, unassigned = []) => ({ rows: ['Combat', 'Crafting', 'Exploration', 'Gathering', 'Sabotage'].map((track, i) => ({ track, number: i < 2 ? i + 2 : null, xp: i < 2 ? 500 : 0, level: 3, keystoneOwned: 0, keystoneTotal: 2, granted: false })),
  unassigned, maxXp: 44182, tracks: ['Combat', 'Crafting', 'Exploration', 'Gathering', 'Sabotage'], resetCount });

async function openSpecs(ui) {
  ui.click('[data-tab="player"]'); await ui.settle();
  ui.click('[data-sub="player:specialization"]'); await ui.settle();
}

test('Reset XP of all known tracks: says the unknown rows are left alone, and sends the confirmed count', async () => {
  const ui = await boot({ get: { '/api/player/specs': specs(2, [{ number: 7, xp: 900, level: 8 }]) }, post: { '/api/player/specs/reset-all': { ok: true, changed: 2, noop: false } } });
  await openSpecs(ui);
  const btn = ui.doc.querySelector('[data-act="spResetAll"]');
  assert.equal(btn.textContent.trim(), 'Reset XP of all known tracks (2)');
  assert.equal(btn.disabled, false);
  assert.ok(ui.doc.getElementById(btn.getAttribute('aria-describedby')).textContent.includes('left alone'));
  ui.click('[data-act="spResetAll"]'); await ui.settle();
  const text = ui.doc.querySelector('#ask').textContent;
  assert.ok(text.includes('Reset the XP of 2 tracks?') && text.includes('Type reset 2 tracks to continue'));
  assert.ok(text.includes('1 row of the save whose track is not known yet is left alone'));
  assert.ok(text.includes('Keystones are not touched'));
  ui.type('reset 2 tracks'); await ui.wait(GAP);
  ui.click('#aok'); await ui.settle();
  assert.deepEqual(posts(ui, '/api/player/specs/reset-all').map((p) => p.body), [{ expected: 2 }]);
});

test('Reset XP of all known tracks: off when no known track has XP', async () => {
  const ui = await boot({ get: { '/api/player/specs': specs(0) } });
  await openSpecs(ui);
  const btn = ui.doc.querySelector('[data-act="spResetAll"]');
  assert.equal(btn.disabled, true);
  assert.ok(ui.doc.getElementById(btn.getAttribute('aria-describedby')).textContent.includes('Nothing to reset'));
});

test('Reset level and XP: a refusal because the XP changed is shown and the page is read again', async () => {
  const ui = await boot({ get: { '/api/player/summary': summary() }, post: { '/api/player/level/reset': { __fail: 'the XP to reset is now 100, not the 41221 you confirmed, so nothing was changed: reload the page and look again', __code: 'conflict', __status: 409 } } });
  ui.click('[data-tab="player"]'); await ui.settle();
  const reads = () => ui.fetched.filter((p) => p === '/api/player/summary').length;
  const before = reads();
  ui.click('[data-act="lvReset"]'); await ui.settle();
  ui.type('reset 41221 xp'); await ui.wait(GAP);
  ui.click('#aok'); await ui.settle(); await ui.wait(20); await ui.settle();
  assert.ok(ui.doc.querySelector('#toast .err').textContent.includes('you confirmed'));
  assert.ok(reads() > before);
});
