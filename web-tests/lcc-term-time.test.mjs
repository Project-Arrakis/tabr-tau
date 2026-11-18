// #922: the term's start and end in days, hours and minutes of play from the point where the save was written, and the real time of the save in the viewer's local time. The save holds the
// start and the end as play-clock seconds and no calendar date, so no date is worked out for them. Real app.js and lcc scripts, the synthetic answer (no game save is read).
import test, { afterEach } from 'node:test';
import assert from 'node:assert/strict';
import { boot, closeAll } from './boot.mjs';
import { synthetic } from './lcc-fixture.mjs';

afterEach(closeAll);
const text = (el) => (el ? el.textContent.replace(/\s+/g, ' ').trim() : '');
async function open(mod) {
  const a = synthetic();
  if (mod) mod(a);
  const ui = await boot({ get: { '/api/landsraad': a }, local: { tab: 'landsraad', 'lsb.view': 'board', 'lsb.layout': 'grid' } });
  await ui.settle();
  return ui;
}
const fact = (ui, k) => ui.doc.querySelector('#lcc-term [data-fact="' + k + '"]');

test('span: days, hours and minutes, the zero cases in words', async () => {
  const ui = await open();
  const T = ui.w.LCCT;
  assert.equal(T.span(2 * 86400 + 4 * 3600 + 15 * 60), '2 d 4 h 15 min');
  assert.equal(T.span(2 * 86400 + 59), '2 d 0 h 0 min');
  assert.equal(T.span(3600 + 5 * 60), '1 h 5 min');
  assert.equal(T.span(10 * 3600), '10 h 0 min');
  assert.equal(T.span(5 * 60 + 59), '5 min');
  assert.equal(T.span(59), 'less than a minute');
  assert.equal(T.span(0), 'less than a minute');
  assert.equal(T.away(1000, 1000), 'at the save (less than a minute away)');
  assert.equal(T.away(1000 + 2 * 86400 + 4 * 3600 + 15 * 60, 1000), '2 d 4 h 15 min of play after the save');
  assert.equal(T.away(1000, 1000 + 3 * 3600), '3 h 0 min of play before the save');
  assert.equal(T.away(null, 5), null);
  assert.equal(T.away(5, undefined), null);
});

test('the start and the end are play time before and after the save, with the play-clock seconds as secondary text, and the length of the term', async () => {
  // the synthetic term: starts at 1,000,000 s, ends at 1,604,800 s (seven days); the save was written at 1,100,000 s
  const ui = await open();
  assert.equal(text(fact(ui, 'start').querySelector('.lcc-tm-v')), '1 d 3 h 46 min of play before the save');
  assert.equal(text(fact(ui, 'start').querySelector('.lcc-tm-sub')), 'play clock 1,000,000 s');
  assert.equal(text(fact(ui, 'end').querySelector('.lcc-tm-v')), '5 d 20 h 13 min of play after the save');
  assert.equal(text(fact(ui, 'end').querySelector('.lcc-tm-sub')), 'play clock 1,604,800 s');
  assert.match(fact(ui, 'end').querySelector('.lcc-tm-v').getAttribute('title'), /1,604,800 s.*1,100,000 s/);
  const note = text(ui.doc.querySelector('#lcc-tm-clock'));
  assert.match(note, /The term is 7 d 0 h 0 min long on the play clock\./);
  assert.match(note, /^Saved .* \(your local time\), with the play clock at 1,100,000 s\./);
  assert.match(note, /no calendar date for them/);
});

test('the real time of the save is shown in the viewer\'s own time zone', async () => {
  const ui = await open();
  const want = new ui.w.Date(1770000000 * 1000).toLocaleString();
  assert.ok(text(ui.doc.querySelector('#lcc-tm-clock')).startsWith('Saved ' + want + ' (your local time)'), text(ui.doc.querySelector('#lcc-tm-clock')));
});

test('a term that has ended, and one that ends within a minute', async () => {
  const over = await open((a) => { a.clock.play_seconds = 1700000; });
  assert.equal(text(fact(over, 'end').querySelector('.lcc-tm-v')), '1 d 2 h 26 min of play before the save');
  const now = await open((a) => { a.clock.play_seconds = 1604800 - 20; });
  assert.equal(text(fact(now, 'end').querySelector('.lcc-tm-v')), 'at the save (less than a minute away)');
});

test('with no position of the save on the play clock, nothing is worked out: the seconds are shown as stored, with the length of the term, and the page says what the save lacks', async () => {
  const ui = await open((a) => { delete a.clock; });
  assert.equal(text(fact(ui, 'start').querySelector('.lcc-tm-v')), '1,000,000 s');
  assert.equal(text(fact(ui, 'end').querySelector('.lcc-tm-v')), '1,604,800 s');
  const note = text(ui.doc.querySelector('#lcc-tm-clock'));
  assert.match(note, /does not hold where the play clock stood/);
  assert.match(note, /The term is 7 d 0 h 0 min long/);
  assert.match(note, /No calendar date is stored/);
  assert.doesNotMatch(note, /Saved /);
});

test('a start or an end the save does not hold says so', async () => {
  const ui = await open((a) => { a.term.start_time = null; a.term.end_time = null; });
  assert.equal(text(fact(ui, 'start').querySelector('.lcc-tm-v')), 'Not in the save');
  assert.equal(text(fact(ui, 'end').querySelector('.lcc-tm-v')), 'Not in the save');
  assert.doesNotMatch(text(ui.doc.querySelector('#lcc-tm-clock')), /is \d+ .* long/);
});

test('no date is invented: nothing of the section is a calendar date but the real time of the save', async () => {
  const ui = await open();
  const t = text(fact(ui, 'start')) + ' ' + text(fact(ui, 'end'));
  assert.doesNotMatch(t, /\b(19|20)\d\d\b|\b(Mon|Tue|Wed|Thu|Fri|Sat|Sun)\b|\d{1,2}[/.]\d{1,2}[/.]\d{2,4}|\d:\d\d/);
});
