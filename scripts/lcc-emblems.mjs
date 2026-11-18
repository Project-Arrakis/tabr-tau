// Derives the five specialization emblems of the Landsraad board from the extracted path banner pictures (issue #976). The banners carry a soft shadow under the emblem and two faded copies of it
// at the sides (partial alpha); the House card draws the emblem in one tint (the crest filter turns every visible pixel to one colour), so those faint parts used to show as a haze behind it.
// This tool keeps only the emblem itself: the connected pieces of the centre whose alpha is high, with their anti-aliased edge, trimmed, padded to a square and scaled down to 96 px.
//
//   node scripts/lcc-emblems.mjs [--check] [--report]
//
// Input:  docs/art/landsraad/banners/<track>.png (the lossless 256 px reference copies of internal/web/static/lcc/art/banners/<track>.webp: the same pixels without the WebP step, which a
//         program with no dependencies can read)
// Output: internal/web/static/lcc/art/emblems/<track>.png (96 x 96, RGBA, committed; this tool is run by hand when the banners change, never in CI)
//   --check   write nothing; exit 1 when a committed emblem differs from what this tool makes
//   --report  print the pieces found in each banner
// Needs only Node (zlib is built in). Game artwork, Funcom's: the same terms as the banners (docs/art-notice.md).
import fs from 'node:fs';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';

const repo = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const TRACKS = ['combat', 'crafting', 'exploration', 'gathering', 'sabotage'];
const SRC = path.join(repo, 'docs', 'art', 'landsraad', 'banners');
const OUT = path.join(repo, 'internal', 'web', 'static', 'lcc', 'art', 'emblems');
const SIZE = 96;       // the emblem pictures are SIZE x SIZE
const PAD = 4;         // transparent margin at SIZE, so a filter's soft edge is never cut by the picture's box
const CORE = 128;      // alpha (0..255) from which a pixel belongs to the emblem; the shadow and the side copies stay below it
const EDGE_LO = 56;    // the soft edge of the emblem: alpha at or under this is cut, above it is stretched to the full range
const MIN_PIECE = 0.02; // a piece smaller than this share of the biggest piece is a speck, not a part of the emblem
const args = new Set(process.argv.slice(2));

function decodePng(buf) {
  if (buf.readUInt32BE(0) !== 0x89504e47) throw new Error('not a PNG');
  let p = 8, w = 0, h = 0; const idat = [];
  while (p < buf.length) {
    const len = buf.readUInt32BE(p), type = buf.toString('latin1', p + 4, p + 8), d = buf.subarray(p + 8, p + 8 + len);
    if (type === 'IHDR') { w = d.readUInt32BE(0); h = d.readUInt32BE(4); if (d[8] !== 8 || d[9] !== 6 || d[12] !== 0) throw new Error('only 8-bit RGBA, not interlaced'); }
    if (type === 'IDAT') idat.push(d);
    p += 12 + len;
  }
  const raw = zlib.inflateSync(Buffer.concat(idat)); const st = w * 4; const out = Buffer.alloc(h * st);
  for (let y = 0; y < h; y++) {
    const f = raw[y * (st + 1)];
    for (let x = 0; x < st; x++) {
      const r = raw[y * (st + 1) + 1 + x];
      const a = x >= 4 ? out[y * st + x - 4] : 0, b = y ? out[(y - 1) * st + x] : 0, c = x >= 4 && y ? out[(y - 1) * st + x - 4] : 0;
      let v;
      if (f === 0) v = r; else if (f === 1) v = r + a; else if (f === 2) v = r + b; else if (f === 3) v = r + ((a + b) >> 1);
      else { const pp = a + b - c, pa = Math.abs(pp - a), pb = Math.abs(pp - b), pc = Math.abs(pp - c); v = r + (pa <= pb && pa <= pc ? a : pb <= pc ? b : c); }
      out[y * st + x] = v & 255;
    }
  }
  return { w, h, data: out };
}

const crcTable = new Uint32Array(256).map((_, n) => { let c = n; for (let k = 0; k < 8; k++) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1; return c >>> 0; });
const crc32 = (b) => { let c = 0xffffffff; for (let i = 0; i < b.length; i++) c = crcTable[(c ^ b[i]) & 255] ^ (c >>> 8); return (c ^ 0xffffffff) >>> 0; };
function encodePng(w, h, data) {
  const chunk = (type, body) => { const b = Buffer.alloc(12 + body.length); b.writeUInt32BE(body.length, 0); b.write(type, 4, 'latin1'); body.copy(b, 8); b.writeUInt32BE(crc32(b.subarray(4, 8 + body.length)), 8 + body.length); return b; };
  const ihdr = Buffer.alloc(13); ihdr.writeUInt32BE(w, 0); ihdr.writeUInt32BE(h, 4); ihdr[8] = 8; ihdr[9] = 6;
  const raw = Buffer.alloc(h * (w * 4 + 1));
  for (let y = 0; y < h; y++) data.copy(raw, y * (w * 4 + 1) + 1, y * w * 4, (y + 1) * w * 4); // filter type 0 on every row
  return Buffer.concat([Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]), chunk('IHDR', ihdr), chunk('IDAT', zlib.deflateSync(raw, { level: 9 })), chunk('IEND', Buffer.alloc(0))]);
}

// the pieces of the pixels whose alpha is at CORE or more (8-connected): { id, area, box } per piece, and a label per pixel (0 = none)
function pieces(im) {
  const { w, h, data } = im; const label = new Int32Array(w * h); const list = [];
  for (let s = 0; s < w * h; s++) {
    if (label[s] || data[s * 4 + 3] < CORE) continue;
    const id = list.length + 1; const stack = [s]; label[s] = id; let area = 0, x0 = w, y0 = h, x1 = -1, y1 = -1;
    while (stack.length) {
      const i = stack.pop(); const x = i % w, y = (i - x) / w; area++;
      if (x < x0) x0 = x; if (x > x1) x1 = x; if (y < y0) y0 = y; if (y > y1) y1 = y;
      for (let dy = -1; dy <= 1; dy++) for (let dx = -1; dx <= 1; dx++) {
        const nx = x + dx, ny = y + dy; if (nx < 0 || ny < 0 || nx >= w || ny >= h) continue;
        const j = ny * w + nx; if (!label[j] && data[j * 4 + 3] >= CORE) { label[j] = id; stack.push(j); }
      }
    }
    list.push({ id, area, box: [x0, y0, x1, y1] });
  }
  return { label, list };
}

// the emblem: every piece that is not a speck (the faded side copies and the shadow never reach CORE, so they are not pieces at all), each with one pixel of anti-aliased edge around it;
// inside that, alpha at or under EDGE_LO is cut and the rest is stretched to the full range, so the edge stays soft but nothing faint is left
function emblem(im) {
  const { w, h, data } = im; const { label, list } = pieces(im);
  if (!list.length) throw new Error('no emblem found');
  const big = list.reduce((m, p) => (p.area > m.area ? p : m));
  const keep = new Set(list.filter((p) => p.area >= big.area * MIN_PIECE).map((p) => p.id));
  const near = new Uint8Array(w * h); // the kept pieces and one pixel around them (the anti-aliased edge)
  for (let i = 0; i < w * h; i++) if (keep.has(label[i])) { const x = i % w, y = (i - x) / w; for (let dy = -1; dy <= 1; dy++) for (let dx = -1; dx <= 1; dx++) { const nx = x + dx, ny = y + dy; if (nx >= 0 && ny >= 0 && nx < w && ny < h) near[ny * w + nx] = 1; } }
  const out = Buffer.alloc(w * h * 4); let x0 = w, y0 = h, x1 = -1, y1 = -1;
  for (let i = 0; i < w * h; i++) {
    if (!near[i]) continue;
    const a = data[i * 4 + 3]; const na = Math.max(0, Math.min(255, Math.round(((a - EDGE_LO) * 255) / (255 - EDGE_LO))));
    if (!na) continue;
    out[i * 4] = data[i * 4]; out[i * 4 + 1] = data[i * 4 + 1]; out[i * 4 + 2] = data[i * 4 + 2]; out[i * 4 + 3] = na;
    const x = i % w, y = (i - x) / w; if (x < x0) x0 = x; if (x > x1) x1 = x; if (y < y0) y0 = y; if (y > y1) y1 = y;
  }
  return { w, h, data: out, box: [x0, y0, x1 + 1, y1 + 1], kept: keep.size, pieces: list.length };
}

// the box [x0,y0,x1,y1) of e, centred in a square, scaled to SIZE with PAD of margin (area weighted, premultiplied, so the edge has no dark or light fringe)
function toSquare(e) {
  const [x0, y0, x1, y1] = e.box; const bw = x1 - x0, bh = y1 - y0; const side = Math.max(bw, bh);
  const inner = SIZE - 2 * PAD; const k = side / inner; // source pixels per output pixel
  const sx = x0 - (side - bw) / 2 - PAD * k, sy = y0 - (side - bh) / 2 - PAD * k; // the source position of the output's top left
  const out = Buffer.alloc(SIZE * SIZE * 4);
  for (let oy = 0; oy < SIZE; oy++) for (let ox = 0; ox < SIZE; ox++) {
    const ax = sx + ox * k, bx = ax + k, ay = sy + oy * k, by = ay + k; let r = 0, g = 0, b = 0, a = 0, wsum = 0;
    for (let y = Math.floor(ay); y < Math.ceil(by); y++) for (let x = Math.floor(ax); x < Math.ceil(bx); x++) {
      const cw = (Math.min(x + 1, bx) - Math.max(x, ax)) * (Math.min(y + 1, by) - Math.max(y, ay)); wsum += cw;
      if (x < 0 || y < 0 || x >= e.w || y >= e.h || cw <= 0) continue;
      const i = (y * e.w + x) * 4; const pa = e.data[i + 3] / 255; r += e.data[i] * pa * cw; g += e.data[i + 1] * pa * cw; b += e.data[i + 2] * pa * cw; a += pa * cw;
    }
    const o = (oy * SIZE + ox) * 4; if (a <= 0 || wsum <= 0) continue;
    out[o] = Math.round(r / a); out[o + 1] = Math.round(g / a); out[o + 2] = Math.round(b / a); out[o + 3] = Math.round((a / wsum) * 255);
  }
  return out;
}

let bad = 0;
if (!args.has('--check')) fs.mkdirSync(OUT, { recursive: true });
for (const t of TRACKS) {
  const im = decodePng(fs.readFileSync(path.join(SRC, t + '.png')));
  if (args.has('--report')) { const { list } = pieces(im); console.log(t, list.map((p) => `#${p.id} area ${p.area} box ${p.box.join(',')}`).join(' | ')); }
  const e = emblem(im);
  const png = encodePng(SIZE, SIZE, toSquare(e));
  const file = path.join(OUT, t + '.png');
  if (args.has('--check')) { if (!fs.existsSync(file) || !fs.readFileSync(file).equals(png)) { console.error(t + ': the committed emblem differs from the tool output'); bad++; } continue; }
  fs.writeFileSync(file, png);
  console.log(`${t}: ${e.kept} of ${e.pieces} pieces kept, box ${e.box.join(',')}, ${png.length} bytes`);
}
process.exit(bad ? 1 : 0);
