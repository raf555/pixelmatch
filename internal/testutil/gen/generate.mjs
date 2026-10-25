// Regenerates ../testdata/cases.json and the PNG files under ../testdata/cases/
// from the reference JavaScript implementation.
//
// The Go test suite replays these vectors and asserts byte-for-byte equality
// with the diff buffers produced here, so they must be regenerated whenever
// upstream mapbox/pixelmatch changes its output.
//
// Pin upstream to the tag this port targets so regenerations are
// reproducible — using `main` has caused silent drift before:
//
//   git clone --branch v7.2.0 https://github.com/mapbox/pixelmatch /tmp/pixelmatch
//   (cd /tmp/pixelmatch && npm install)
//   node generate.mjs /tmp/pixelmatch/index.js
//
// Existing cases keep their input PNGs and options and only have their
// expected diff PNG and n recomputed, so a regeneration diff shows exactly
// what upstream changed. `pngjs` is resolved from the upstream repo's
// node_modules, so no local install is required here.

import fs from 'fs';
import path from 'path';
import {fileURLToPath, pathToFileURL} from 'url';
import {createRequire} from 'module';

const here = path.dirname(fileURLToPath(import.meta.url));
const upstream = process.argv[2];
if (!upstream) {
    console.error('usage: node generate.mjs <path to mapbox/pixelmatch index.js>');
    process.exit(1);
}
const upstreamAbs = path.resolve(upstream);
// Dynamic import() on Windows requires a file:// URL for absolute paths —
// bare `D:\...` is rejected with ERR_UNSUPPORTED_ESM_URL_SCHEME.
const match = (await import(pathToFileURL(upstreamAbs).href)).default;
const {PNG} = createRequire(upstreamAbs)('pngjs');

const casesPath = path.join(here, '..', 'testdata', 'cases.json');
const casesDir = path.join(here, '..', 'testdata', 'cases');
const cases = JSON.parse(fs.readFileSync(casesPath, 'utf8'));

function readPNG(p) {
    return PNG.sync.read(fs.readFileSync(p));
}
function writePNG(p, data, w, h) {
    const png = new PNG({width: w, height: h});
    png.data = Buffer.from(data);
    fs.writeFileSync(p, PNG.sync.write(png));
}

for (const c of cases) {
    const a = readPNG(path.join(casesDir, `${c.name}-a.png`));
    const b = readPNG(path.join(casesDir, `${c.name}-b.png`));
    if (a.width !== c.w || a.height !== c.h) throw new Error(`${c.name}-a: size ${a.width}x${a.height} != ${c.w}x${c.h}`);
    if (b.width !== c.w || b.height !== c.h) throw new Error(`${c.name}-b: size ${b.width}x${b.height} != ${c.w}x${c.h}`);
    const out = Buffer.alloc(a.data.length);
    const before = c.n;
    c.n = match(a.data, b.data, out, c.w, c.h, {...c.opts});
    writePNG(path.join(casesDir, `${c.name}-diff.png`), out, c.w, c.h);
    const mark = before === c.n ? ' ' : '*';
    console.log(`${mark} ${c.name}: n=${c.n}${before === c.n ? '' : ` (was ${before})`}`);
}

fs.writeFileSync(casesPath, JSON.stringify(cases, null, 4) + '\n');
console.log(`\nwrote ${cases.length} cases to ${casesPath}`);
