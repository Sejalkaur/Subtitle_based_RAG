// server/scripts/embed_upsert.js
import 'dotenv/config';
import fg from 'fast-glob';
import fs from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { v5 as uuidv5 } from 'uuid';
import { getEmbedder } from '../src/lib/embeddings.js';
import { getQdrant, ensureCollection, collectionName } from '../src/lib/qdrant.js';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

// Use a fixed namespace for v5 UUIDs (can be any valid UUID). Put in .env if you want to customize.
const NAMESPACE = process.env.CHUNK_ID_NAMESPACE || '6ba7b811-9dad-11d1-80b4-00c04fd430c8';
const pointIdFromChunkId = (chunkId) => uuidv5(String(chunkId), NAMESPACE);

function chunkArray(arr, size) {
  const out = [];
  for (let i = 0; i < arr.length; i += size) out.push(arr.slice(i, i + size));
  return out;
}

async function loadChunks(files) {
  const all = [];
  for (const f of files) {
    const raw = JSON.parse(await fs.readFile(f, 'utf8'));
    for (const c of raw) all.push({ ...c, sourceFile: path.basename(f) });
  }
  return all;
}

async function main() {
  console.log('[embed] collection =', process.env.QDRANT_COLLECTION);
  console.log('[embed] qdrant url =', process.env.QDRANT_URL);
  console.log(
    '[embed] provider   =',
    process.env.EMBEDDINGS_PROVIDER,
    process.env.EMBEDDINGS_MODEL,
  );

  const argv = process.argv.slice(2);
  const patterns = argv.length > 0 ? argv : ['../../data/parsed/chunks_*.json'];

  const files = await fg(patterns, { cwd: __dirname, absolute: true });
  console.log(
    '[embed] matched chunk files:',
    files.map((f) => f.split('/').slice(-2).join('/')),
  );
  if (files.length === 0) {
    console.error('No chunk files found. Did you run the chunker first?');
    process.exit(1);
  }

  const embedder = getEmbedder();
  const client = getQdrant();
  await ensureCollection(client, embedder.dim);
  console.log('[embed] ensuring collection with dim =', embedder.dim);

  const chunks = await loadChunks(files);
  console.log('[embed] total chunks loaded =', chunks.length);

  const points = chunks.map((c) => ({
    id: pointIdFromChunkId(c.chunkId),
    payload: {
      chunkId: c.chunkId,
      courseId: c.courseId,
      fileId: c.fileId,
      startMs: c.startMs,
      endMs: c.endMs,
      text: c.text,
      lineIndices: c.lineIndices,
      sourceFile: c.sourceFile,
    },
  }));

  const BATCH = 32;
  let done = 0;

  for (const batch of chunkArray(points, BATCH)) {
    console.log(`[embed] batching: ${batch.length} texts…`);
    const texts = batch.map((p) => p.payload.text);
    const vectors = await embedder.embedMany(texts);

    const payloadForUpsert = batch.map((p, i) => ({
      id: p.id,
      vector: vectors[i],
      payload: p.payload,
    }));

    await client.upsert(collectionName, { points: payloadForUpsert });
    done += batch.length;
    console.log(`[upsert] ${done}/${points.length}`);
  }

  console.log(`\nDone. Upserted ${done} chunks into collection "${collectionName}".`);
}

main().catch((e) => {
  console.error(e);
  process.exit(1);
});
