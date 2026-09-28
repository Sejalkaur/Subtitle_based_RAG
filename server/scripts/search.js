// server/scripts/search.js
import 'dotenv/config';
import { searchChunks } from '../src/lib/retrieval.js';

function parseArgs(argv) {
  const out = {
    query: null,
    course: null,
    file: null,
    top: 80,
    keep: 12,
    mmr: true,
    min: 0.52,
    strict: true,
    print: 10,
  };
  const args = argv.slice(2);
  out.query = args.find((a) => !a.startsWith('--')) || '';
  for (let i = 0; i < args.length; i++) {
    const a = args[i];
    if (a === '--course') out.course = args[++i];
    else if (a === '--file') out.file = args[++i];
    else if (a === '--top') out.top = Number(args[++i] || 80);
    else if (a === '--keep') out.keep = Number(args[++i] || 12);
    else if (a === '--no-mmr') out.mmr = false;
    else if (a === '--min') out.min = Number(args[++i] || 0.52);
    else if (a === '--no-strict') out.strict = false;
    else if (a === '--print') out.print = Number(args[++i] || 10);
  }
  return out;
}

async function main() {
  const opts = parseArgs(process.argv);
  if (!opts.query) {
    console.error(
      'Usage: node scripts/search.js "<query>" [--course <id>] [--file <id>] [--top 80] [--keep 12] [--min 0.52] [--no-mmr] [--no-strict]',
    );
    process.exit(1);
  }

  const res = await searchChunks({
    query: opts.query,
    courseId: opts.course || undefined,
    fileId: opts.file || undefined,
    topK: opts.top,
    mmrK: opts.keep,
    useMMR: opts.mmr,
    strictMMR: opts.strict,
    minScore: opts.min,
  });
  if (res.results.length === 0) {
    console.log('\n(no pretty results)');
  } else {
    console.log('\n[debug] raw first hit =', JSON.stringify(res.results[0], null, 2));
  }

  console.log('\nQuery:', res.query);
  console.log('Meta :', res.meta);

  if (!res.results.length) {
    console.log('\nI don’t know, but I could help in other areas.\n'); // your desired message
    process.exit(0);
  }

  const limit = Math.min(opts.print, res.results.length);
  console.log(`\nTop results (showing ${limit} of ${res.results.length}):`);
  res.results.slice(0, limit).forEach((r, i) => {
    console.log(
      `\n#${i + 1}  [${r.courseId}] ${r.fileId}  ${r.start}–${r.end}\n` +
        `     score: ${r.score?.toFixed?.(4)}\n` +
        `     text : ${r.preview}`,
    );
  });
}

main().catch((e) => {
  console.error(e);
  process.exit(1);
});
