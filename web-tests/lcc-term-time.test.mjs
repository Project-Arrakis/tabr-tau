// #981: the term's time is ONE row, ENDS IN, in hours and minutes of the time the term has left (the term's end on the play clock minus where the play clock stood when the save was written).
// No Starts row, no play-clock seconds, no real time of the save, no length of the term anywhere in the Term section, on the board or in the classic list. The decree elected during the term is
// labelled Next term's decree. Real app.js and lcc scripts, the synthetic answer (no game save is read).
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
  return ui;
}
const fact = (ui, k) => ui.doc.querySelector('#lcc-term [data-fact="' + k + '"]');
const ends = (ui) => text(fact(ui, 'ends').querySelector('.lcc-tm-v'));
const END = 1604800; // the synthetic term ends at this second of the play clock
const at = (left) => (a) => { a.clock.play_seconds = END - left; };

test('the term has one time row, ENDS IN, with the hours and minutes left; the synthetic save was written 140 h 13 m before the end', async () => {
  const ui = await open();
  assert.equal(text(fact(ui, 'ends').querySelector('.lcc-tm-k')), 'ENDS IN');
  assert.equal(ends(ui), '140 h 13 m');
  assert.equal(fact(ui, 'start'), null, 'no Starts row');
  assert.equal(fact(ui, 'end'), null, 'no separate end row');
  assert.equal(fact(ui, 'ends').querySelector('.lcc-tm-sub'), null, 'nothing else on the row');
  assert.equal(fact(ui, 'ends').querySelector('.lcc-tm-v').getAttribute('title'), 'Counts down only while the game is running; as of the last save.');
});

test('the value reads in hours and minutes: 18 h 37 m, an exact hour, under an hour, under a minute, at the end, past the end', async () => {
  assert.equal(ends(await open(at(18 * 3600 + 37 * 60))), '18 h 37 m');
  assert.equal(ends(await open(at(10 * 3600))), '10 h 0 m');
  assert.equal(ends(await open(at(37 * 60 + 59))), '37 m');
  assert.equal(ends(await open(at(60))), '1 m');
  assert.equal(ends(await open(at(59))), 'less than a minute');
  assert.equal(ends(await open(at(1))), 'less than a minute');
  assert.equal(ends(await open(at(0))), 'ended');
  assert.equal(ends(await open(at(-5000))), 'ended');
});

test('a clock or an end the save does not give is "not known", never a guess', async () => {
  assert.equal(ends(await open((a) => { delete a.clock; })), '—');
  assert.equal(ends(await open((a) => { a.term.end_time = null; })), '—');
  assert.equal(ends(await open((a) => { a.term.end_time = 1770000000; })), '—', 'a real time is not a play-clock second');
});

test('no seconds, no play clock, no calendar date and no real time of the save anywhere in the Term section', async () => {
  const ui = await open();
  const t = text(ui.doc.querySelector('#lcc-term'));
  for (const d of [...ui.doc.querySelectorAll('#lcc-term details')]) d.open = true;
  const all = text(ui.doc.querySelector('#lcc-term')) + ' ' + [...ui.doc.querySelectorAll('#lcc-term [title], #lcc-term [aria-label]')].map((n) => (n.getAttribute('title') || '') + ' ' + (n.getAttribute('aria-label') || '')).join(' ');
  assert.doesNotMatch(all, /play clock|play time|seconds|\d\s?s\b|1,000,000|1,604,800|1,100,000|STARTS|\bStarts\b/i, 'no play-clock text: ' + t.slice(0, 200));
  assert.doesNotMatch(all, /\b(19|20)\d\d\b|\b(Mon|Tue|Wed|Thu|Fri|Sat|Sun)\b|\d{1,2}[/.]\d{1,2}[/.]\d{2,4}|\d:\d\d/, 'no date');
  assert.doesNotMatch(all, /Saved .*your local time|No calendar date|is \d+ .* long/, 'no clock note');
});

test('the decree chosen during the term is labelled Next term\'s decree, "none yet" before the vote, and the pool tag says the same', async () => {
  const ui = await open();
  assert.equal(text(fact(ui, 'elected').querySelector('.lcc-tm-k')), 'Next term\'s decree');
  assert.match(text(fact(ui, 'elected')), /RangedDamageIncreased|Ranged/i);
  assert.doesNotMatch(text(ui.doc.querySelector('#lcc-term')), /\bElected\b/i, 'the word Elected is gone from the section');
  const row = ui.doc.querySelector('#lcc-term .lcc-tm-row[data-decree="2"]');
  assert.match(text(row.querySelector('.lcc-tm-tags')), /Next term's decree/);
  const none = await open((a) => { a.term.elected_decree_id = null; });
  assert.equal(text(fact(none, 'elected').querySelector('.lcc-tm-v')), 'none yet');
});

test('the intro line, the NOT IN THE SAVE box, the scope line and the clock line are gone; a save with no term still says so', async () => {
  const ui = await open();
  const s = ui.doc.querySelector('#lcc-term');
  const t = text(s);
  assert.doesNotMatch(t, /The decree in force, who reigns/);
  assert.doesNotMatch(t, /editor's own term control|plaque above only shows/);
  assert.equal(s.querySelector('.lcc-tm-nosave'), null);
  assert.doesNotMatch(t, /NOT IN THE SAVE|Active Landsraad missions|A ranking of contributors/);
  assert.equal(s.querySelector('.lcc-tm-scope'), null);
  assert.equal(s.querySelector('.lcc-tm-clock'), null);
  assert.equal(s.querySelector('.lcc-tm-lead'), null, 'no lead under the heading while there is a term');
  const none = await open((a) => { a.term = null; });
  assert.match(text(none.doc.querySelector('#lcc-term')), /This save has no Landsraad term/);
});

test('the Earlier terms rows carry no play-clock line', async () => {
  const ui = await open();
  const h = ui.doc.querySelector('#lcc-term .lcc-tm-hist');
  for (const d of h.querySelectorAll('details')) d.open = true;
  assert.ok(h.querySelector('.lcc-tm-term'), 'there is an earlier term');
  assert.doesNotMatch(text(h), /Play clock|play time| s \(|No calendar date/);
});

test('the classic list says the same: one Ends in value, Next term\'s decree, no Start or End seconds, no play-clock line under Earlier terms', async () => {
  const ui = await open(null, { 'lsb.view': 'classic' });
  const root = ui.doc.querySelector('#lsb');
  const t = text(root);
  assert.match(t, /Ends in\s*140 h 13 m/);
  assert.match(t, /Next term's decree/);
  assert.doesNotMatch(t, /Play clock|1,000,000|1,604,800|\bStart\b|\bElected decree\b/);
});
