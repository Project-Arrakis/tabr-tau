// The art lookups of the Command Center (M2): a picture is found in a fixed list by its key and a file name is never built from a name. The tests cover the 25 Houses and the
// Mikkarol / Mikarrol alias, the lanes, the banners, the decrees with their neutral picture, and the strings that must never become a path (traversal, case, odd types).
import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { load, lccDir, read } from './lcc-load.mjs';

const { L } = load();
const manifest = JSON.parse(read('docs', 'design', 'landsraad-command-center', 'art-manifest.json'));
const onDisk = (folder) => fs.readdirSync(path.join(lccDir, 'art', folder)).sort();

test('the House list is the 25 embedded symbols, the lane and banner lists are the five paths, the decree list is the embedded decree pictures', () => {
  assert.deepEqual([...L.HOUSES].sort(), onDisk('houses').map((f) => f.replace('.webp', '')));
  assert.equal(L.HOUSES.length, 25);
  assert.deepEqual([...L.TRACKS].sort(), onDisk('lanes').map((f) => f.replace('.webp', '')));
  assert.deepEqual([...L.TRACKS].sort(), onDisk('banners').map((f) => f.replace('.webp', '')));
  assert.equal(fs.existsSync(path.join(lccDir, 'art', 'specs')), false, 'the 84 px badge crops are gone (#963): the House card draws the banners');
  assert.equal(L.specAsset, undefined);
  assert.deepEqual([...L.DECREE_FILES].sort(), onDisk('decrees').filter((f) => f !== 'fallback.svg').map((f) => f.replace('.webp', '')));
  assert.deepEqual(manifest.files.filter((f) => f.role === 'house').map((f) => f.key).sort(), [...L.HOUSES].sort());
});

test('every lookup resolves to a file that is embedded and listed in the manifest', () => {
  const listed = new Set(manifest.files.map((f) => f.file));
  const all = [
    ...L.HOUSES.map((k) => L.houseAsset(k)), ...L.HOUSES.map((k) => L.houseAsset('DA_House' + k[0].toUpperCase() + k.slice(1))),
    ...L.TRACKS.map((k) => L.laneAsset(k)), ...L.TRACKS.map((k) => L.bannerAsset(k)),
    ...Object.keys(manifest.decreeArt).map((id) => L.decreeAsset(id)), L.decreeAsset('SpecialVendorActive'),
  ];
  for (const a of all) {
    assert.ok(a, 'a lookup found nothing');
    assert.ok(listed.has(a.file), a.file + ' is not in the manifest');
    assert.ok(fs.existsSync(path.join(lccDir, 'art', a.file)), a.file + ' is not on disk');
    assert.equal(a.src, '/lcc/art/' + a.file);
  }
  // each decree the manifest matches resolves to exactly the file the manifest names
  for (const [id, d] of Object.entries(manifest.decreeArt)) assert.equal(L.decreeAsset(id).file, 'decrees/' + d.file + '.webp', id);
});

test('the save and the reference spell one House two ways and both find the one file', () => {
  for (const n of ['Mikkarol', 'mikkarol', 'DA_HouseMikkarol', 'da_housemikkarol', 'Mikarrol', 'mikarrol', 'DA_HouseMikarrol', ' MIKARROL ']) {
    assert.equal(L.houseAsset(n).file, 'houses/mikkarol.webp', n);
  }
  assert.deepEqual(manifest.houseAlias, { mikarrol: 'mikkarol' });
});

test('a House name is read without its prefix and without regard to case, and nothing else', () => {
  assert.equal(L.houseAsset('DA_HouseEcaz').file, 'houses/ecaz.webp');
  assert.equal(L.houseAsset('ECAZ').file, 'houses/ecaz.webp');
  assert.equal(L.houseAsset('  ecaz\t').file, 'houses/ecaz.webp');
  assert.equal(L.houseAsset('da_househagal').key, 'hagal');
  for (const n of ['', '   ', 'DA_House', 'nobody', 'ecaz2', 'ec az', 'DA_DA_HouseEcaz', 'HouseEcaz']) assert.equal(L.houseAsset(n), null, JSON.stringify(n));
});

test('a path, a traversal, an extension or an odd type never becomes a picture', () => {
  const bad = ['../app.js', '..\\app.js', '../../houses/ecaz', 'houses/ecaz', '/houses/ecaz', 'ecaz/', 'ecaz/../ecaz', './ecaz', 'ecaz.webp', 'ecaz.png', 'ecaz%00', 'ecaz\u0000', '%2e%2e/ecaz', '%2e%2e%2fecaz', 'ecaz;', 'ecaz?x=1', 'ecaz#x', 'http://x/ecaz', '//x/ecaz', 'javascript:ecaz', 'data:image/png;base64,AA', 'ecаz' /* a Cyrillic a */, 'e​caz', 'a'.repeat(5000), '__proto__', 'constructor', 'toString', 'hasOwnProperty', 'valueOf', 'prototype'];
  for (const n of bad) {
    assert.equal(L.houseAsset(n), null, 'house ' + JSON.stringify(n).slice(0, 40));
    assert.equal(L.laneAsset(n), null, 'lane ' + JSON.stringify(n).slice(0, 40));
    assert.equal(L.bannerAsset(n), null, 'banner ' + JSON.stringify(n).slice(0, 40));
    assert.equal(L.decreeAsset(n).fallback, true, 'decree ' + JSON.stringify(n).slice(0, 40));
  }
  const evil = { toString() { return 'ecaz'; }, valueOf() { return 'ecaz'; } };
  for (const v of [null, undefined, 0, 5, NaN, true, [], ['ecaz'], {}, evil, Symbol.iterator, () => 'ecaz']) {
    assert.equal(L.houseAsset(v), null, 'house of a ' + typeof v);
    assert.equal(L.laneAsset(v), null);
    assert.equal(L.bannerAsset(v), null);
    assert.equal(L.decreeAsset(v).fallback, true, 'decree of a ' + typeof v);
  }
});

test('lane and banner pictures are found by the five path names, in any case', () => {
  for (const k of ['combat', 'crafting', 'exploration', 'gathering', 'sabotage']) {
    assert.equal(L.laneAsset(k).file, 'lanes/' + k + '.webp');
    assert.equal(L.bannerAsset(k.toUpperCase()).file, 'banners/' + k + '.webp');
    assert.equal(L.bannerAsset(k).src, '/lcc/art/banners/' + k + '.webp');
  }
  for (const n of ['harvesting', 'combat2', 'lanes/combat', 'combat.png']) { assert.equal(L.laneAsset(n), null); assert.equal(L.bannerAsset(n), null); }
});

test('a decree id finds its picture; an unmatched decree, an unknown id and a hostile string get the neutral picture, never a 404 and never a guess', () => {
  assert.equal(L.decreeAsset('SpecialVendorActive_Vehicles').file, 'decrees/vendor-vehicles.webp');
  assert.equal(L.decreeAsset('specialvendoractive_vehicles').file, 'decrees/vendor-vehicles.webp');
  assert.equal(L.decreeAsset('RangedDamageIncreased').file, L.decreeAsset('MeleeDamageIncreased').file);
  for (const id of ['SpecialVendorActive', 'vendor-special', 'NotADecreeId', '', '../vendor-armor', 'vendor-armor', 'xp', 'choam', '1', '12']) {
    const a = L.decreeAsset(id);
    assert.equal(a.fallback, true, id);
    assert.equal(a.file, 'decrees/fallback.svg');
    assert.equal(a.key, null);
    assert.ok(fs.existsSync(path.join(lccDir, 'art', a.file)));
  }
  assert.equal(L.decreeAsset(12).fallback, true);
  // every id the editor's decree table has: only the plain SpecialVendorActive row is without a picture
  const names = JSON.parse(read('internal', 'ops', 'decree-names.json')).decrees;
  const without = Object.keys(names).filter((id) => L.decreeAsset(id).fallback);
  assert.deepEqual(without, ['SpecialVendorActive']);
});

test('the base of the art folder is a path on this origin: an address with a scheme, a host or a script is refused', () => {
  const { L: M } = load();
  assert.equal(M.config({ artBase: '../../x/art/' }).artBase, '../../x/art/');
  for (const b of ['http://x/', 'https://x/art/', '//x/art/', 'javascript:alert(1)//', 'data:,', '/a//b/', 'a b/', 'a\\b/', 'x/art/?q=1/', 'x/art', '/lcc/art']) {
    assert.throws(() => M.config({ artBase: b }), /artBase/, JSON.stringify(b));
  }
  M.config({ artBase: '/lcc/art/' });
  assert.equal(M.houseAsset('ecaz').src, '/lcc/art/houses/ecaz.webp');
});

test('the manifest and the notice agree: banners are 256, not 512, the missing decree picture is recorded, and the art folder is in the notice', () => {
  assert.ok(manifest.files.filter((f) => f.role === 'banner').every((f) => f.width === 256 && f.height === 256));
  assert.deepEqual(manifest.decreeMissing, ['vendor-special']);
  const notice = read('docs', 'art-notice.md');
  assert.match(notice, /internal\/web\/static\/lcc\/art\//);
  assert.match(notice, /Funcom/);
});

test('the faction crests: two factions, two kinds, always an answer, and nothing else becomes a picture', () => {
  const listed = new Map(manifest.files.map((f) => [f.file, f]));
  const want = { a: ['atreides', 'atreides-medallion'], h: ['harkonnen', 'harkonnen-medallion'] };
  for (const [f, [mark, medal]] of Object.entries(want)) {
    for (const spell of [f, f.toUpperCase(), ' ' + f + ' ', f === 'a' ? 'Atreides' : 'Harkonnen', f === 'a' ? 'ATREIDES' : 'harkonnen']) {
      assert.equal(L.factionAsset(spell, 'mark').file, 'factions/' + mark + '.webp', spell);
      assert.equal(L.factionAsset(spell).file, 'factions/' + mark + '.webp', 'the kind defaults to the white crest');
      assert.equal(L.factionAsset(spell, 'medal').file, 'factions/' + medal + '.webp', spell);
    }
    for (const kind of ['mark', 'medal']) {
      const a = L.factionAsset(f, kind);
      assert.equal(a.fallback, false);
      assert.equal(a.src, '/lcc/art/' + a.file);
      assert.ok(listed.has(a.file), a.file + ' is not in the manifest');
      assert.ok(fs.existsSync(path.join(lccDir, 'art', a.file)), a.file + ' is not on disk');
    }
  }
  assert.deepEqual(onDisk('factions'), ['atreides-medallion.webp', 'atreides.webp', 'harkonnen-medallion.webp', 'harkonnen.webp']);
  assert.deepEqual(manifest.files.filter((f) => f.role.startsWith('faction')).map((f) => f.file).sort(), onDisk('factions').map((f) => 'factions/' + f));
  // a kind that is not 'medal' is the white crest; anything that is not one of the two factions answers with no picture (never a path, never a guess)
  assert.equal(L.factionAsset('a', '../x').file, 'factions/atreides.webp');
  const bad = ['', 'x', 'ab', 'atreides.webp', 'factions/atreides', '../atreides', 'atreides/', 'open', 'none', 'ecaz', '__proto__', 'constructor', null, undefined, 0, 7, {}, [], ['a'], { toString() { return 'a'; } }];
  for (const v of bad) {
    const a = L.factionAsset(v, 'medal');
    assert.deepEqual({ key: a.key, file: a.file, src: a.src, fallback: a.fallback }, { key: null, file: null, src: '', fallback: true }, JSON.stringify(v));
  }
  // kept apart from the House and path tables
  assert.equal(L.houseAsset('atreides'), null);
  assert.equal(L.laneAsset('atreides'), null);
});

test('the foot pictures of the two cloth standards: two files, one per faction, found in a fixed list, listed in the manifest, nothing else becomes a path', () => {
  assert.deepEqual(onDisk('standards'), ['atreides-tip.png', 'harkonnen-tip.png']);
  assert.deepEqual(manifest.files.filter((f) => f.role === 'standard-tip').map((f) => f.file).sort(), onDisk('standards').map((f) => 'standards/' + f));
  for (const [spell, file] of [['a', 'atreides-tip'], ['A', 'atreides-tip'], ['Atreides', 'atreides-tip'], ['h', 'harkonnen-tip'], [' HARKONNEN ', 'harkonnen-tip']]) {
    const a = L.standardTipAsset(spell);
    assert.equal(a.fallback, false, spell);
    assert.equal(a.file, 'standards/' + file + '.png', spell);
    assert.equal(a.src, '/lcc/art/' + a.file);
    assert.equal(a.height, 111);
    assert.ok(fs.existsSync(path.join(lccDir, 'art', a.file)));
  }
  for (const v of ['', 'x', 'atreides-tip', '../atreides', 'atreides.png', 'standards/atreides', null, undefined, 0, {}, ['a']]) {
    const a = L.standardTipAsset(v);
    assert.deepEqual({ key: a.key, file: a.file, src: a.src, fallback: a.fallback }, { key: null, file: null, src: '', fallback: true }, JSON.stringify(v));
  }
});
