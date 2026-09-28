import fg from 'fast-glob';
import fs from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { chunkCues } from '../src/lib/chunker.js';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

async function main() {
  const argv = process.argv.slice(2);
  const patterns = argv.length > 0 ? argv : ['../../data/parsed/*.json'];

  // expand globs into files
  const files = await fg(patterns, { cwd: __dirname, absolute: true });
  if (files.length === 0) {
    console.error('No parsed JSON found. Run ingest first.');
    process.exit(1);
  }

  for (const absPath of files) {
    try {
      const raw = JSON.parse(await fs.readFile(absPath, 'utf8'));
      const chunks = chunkCues(raw, { windowMs: 60000, overlapMs: 10000 });

      const dir = path.dirname(absPath);
      const base = path.basename(absPath);
      const outPath = path.join(dir, `chunks_${base}`);

      await fs.writeFile(outPath, JSON.stringify(chunks, null, 2), 'utf8');
      console.log(`[ok] ${base} -> ${path.basename(outPath)} (${chunks.length} chunks)`);
    } catch (err) {
      console.error(`[fail] ${absPath}:`, err.message);
    }
  }
}

main();
