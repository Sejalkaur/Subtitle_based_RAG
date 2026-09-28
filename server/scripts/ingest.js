// Subtitle Ingestion CLI (VTT-only) with courseId tagging

import fg from 'fast-glob';
import fs from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { parseVttToNormalized } from '../src/lib/normalize.js';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

function baseId(p) {
  return path.basename(p, path.extname(p));
}
async function ensureDir(dir) {
  await fs.mkdir(dir, { recursive: true });
}

// derive courseId from abs path: .../data/raw/<courseId>/subfolder/file.vtt
function deriveCourseId(absPath) {
  const parts = absPath.split(path.sep);
  const rawIdx = parts.lastIndexOf('raw');
  if (rawIdx >= 0 && rawIdx + 1 < parts.length) {
    return String(parts[rawIdx + 1])
      .trim()
      .toLowerCase(); // e.g. "nodejs"
  }
  return 'default';
}

async function main() {
  const argv = process.argv.slice(2);
  const defaults = [
    '../../data/raw/**/*.vtt',
    '../../data/raw/*.vtt',
    '../data/raw/**/*.vtt',
    '../data/raw/*.vtt',
  ];
  const patterns = argv.length > 0 ? argv : defaults;

  const entries = await fg(patterns, {
    cwd: __dirname,
    onlyFiles: true,
    absolute: true,
    caseSensitiveMatch: false,
  });
  if (entries.length === 0) {
    console.error('No .vtt files found. Put them under data/raw/<courseId>/.');
    process.exit(1);
  }

  const outDir = path.resolve(__dirname, '../../data/parsed');
  await ensureDir(outDir);

  let ok = 0,
    fail = 0;
  for (const absPath of entries) {
    try {
      const courseId = deriveCourseId(absPath); // <- NEW
      const fileBase = baseId(absPath);
      const fileId = `${courseId}__${fileBase}`; // <- NEW

      const text = await fs.readFile(absPath, 'utf8');
      const cues = parseVttToNormalized(text, fileId, courseId); // <- pass courseId

      const outPath = path.join(outDir, `${fileId}.json`);
      await fs.writeFile(outPath, JSON.stringify(cues, null, 2), 'utf8');

      console.log(
        `[ok] ${path.relative(process.cwd(), absPath)} -> ${path.relative(process.cwd(), outPath)} (${cues.length} cues)`,
      );
      ok++;
    } catch (err) {
      console.error(`[fail] ${absPath}:`, err?.message || err);
      fail++;
    }
  }

  console.log(`\nDone. success=${ok}, failed=${fail}`);
}

main().catch((e) => {
  console.error(e);
  process.exit(1);
});
