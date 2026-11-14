// A guard on the Landsraad mock-up README (docs/design/mock/landsraad/README.md): the Kept decisions are the design record. A change to the page keeps every one of them, so each
// is pinned here by a phrase of its own. #641 replaced the staging decisions with "every click writes at once"; a revision of the list must not drop any other.
import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';

const readme = fs.readFileSync(new URL('../docs/design/mock/landsraad/README.md', import.meta.url), 'utf8').replace(/\r\n/g, '\n');
const kept = (() => {
  const i = readme.indexOf('## Kept decisions'), j = readme.indexOf('\n## ', i + 5);
  return readme.slice(i, j < 0 ? undefined : j);
})();
const bullets = kept.split('\n').filter((l) => l.startsWith('* '));

// one phrase per decision, in the order of the list
const DECISIONS = [
  'Houses are always alphabetical by default',
  'Each row has a house symbol',
  'The bar above the list has: Show',
  'Three glance cards and a proportional bar',
  'Reward tiers are clickable in the collapsed meter and in the opened row',
  'Per house: Fill faction goal, Complete and Reopen in the opened row',
  'A faction drop-down that starts on your own faction',
  'Fill changes your contribution and tiers',
  '"Term and decrees" panel, closed by default',
  'Location: the row shows the area name',
  'House name card (experimental)',
  'Wording of states',
  'Each tier shows its reward',
  'Long explanations live only in closed "About ..." disclosures',
  // new with #641 and #634
  '**Every click writes at once**',
  '**Reopen does not undo a Complete completely.**',
  '**The plain Special Vendor decree is hidden from the pool list**',
];

test('every Kept decision of the Landsraad mock-up is still in the README', () => {
  for (const d of DECISIONS) assert.ok(bullets.some((b) => b.includes(d)), 'Kept decision missing: ' + d);
});

test('the staging decisions are replaced by "every click writes at once": no pending state, no Undo, no Write button, no queue, no writes badge', () => {
  const w = bullets.find((b) => b.includes('**Every click writes at once**'));
  assert.ok(w);
  for (const s of ['no staged or pending state', 'no Undo', 'no Write button', 'no queue', 'Complete 7 houses?', 'disabled while its write runs']) assert.ok(w.includes(s), 'the decision lacks: ' + s);
  for (const stale of ['a pending state with Undo', 'stages its changes', 'with the Write button', 'A pending change shows as a row tint', 'and writes marks']) assert.ok(!kept.includes(stale), 'a staging decision is still listed: ' + stale);
});

test('the hidden Special Vendor decision names the label, the four vendors that stay and that the row is never changed', () => {
  const h = bullets.find((b) => b.includes('**The plain Special Vendor decree is hidden'));
  assert.ok(h);
  for (const s of ['Hidden: Special Vendor, no category (effect unverified)', 'Vehicles, Weapons, Armor, Utilities', 'never changed or deleted', 'same Make active and Set flag controls']) assert.ok(h.includes(s), s);
});

test('Reopen does not promise an undo: the README, and the table of what it writes, say what it leaves', () => {
  const r = bullets.find((b) => b.includes('**Reopen does not undo a Complete completely.**'));
  assert.ok(r && r.includes('progress that Complete raised to the goal stays') && r.includes('no reward rows'));
  assert.ok(readme.includes('so it does not restore the progress Complete raised or a previous winner, and it touches no reward row'));
});
