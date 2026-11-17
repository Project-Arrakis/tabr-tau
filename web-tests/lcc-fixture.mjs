// The synthetic answer of the Landsraad read (milestone M1), for the lcc board tests: the document's own fixture with its comment keys taken off. No game save is read.
import fs from 'node:fs';
import path from 'node:path';
import { root } from './lcc-load.mjs';

const clean = (o) => JSON.parse(JSON.stringify(o, (k, v) => (k.startsWith('_') ? undefined : v)));
export const synthetic = () => clean(JSON.parse(fs.readFileSync(path.join(root, 'docs', 'design', 'landsraad-command-center', 'fixtures', 'api-landsraad.synthetic.json'), 'utf8')));
