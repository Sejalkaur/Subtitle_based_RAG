import { getEmbedder } from './embeddings.js';
import { getQdrant, collectionName } from './qdrant.js';
import { msToTimestamp } from './time.js';
import { logger } from './logger.js';


function buildFilter({ courseId, fileId } = {}) {
  const must = [];
  if (courseId) must.push({ key: 'courseId', match: { value: courseId } });
  if (fileId) must.push({ key: 'fileId', match: { value: fileId } });
  return must.length ? { must } : undefined;
}

function cosine(a, b) {
  let dot = 0,
    na = 0,
    nb = 0;
  for (let i = 0; i < a.length; i++) {
    dot += a[i] * b[i];
    na += a[i] * a[i];
    nb += b[i] * b[i];
  }
  if (na === 0 || nb === 0) return 0;
  return dot / (Math.sqrt(na) * Math.sqrt(nb));
}
function mmr(candidates, queryVec, k = 10, lambda = 0.6) {
  if (!candidates.length) return [];
  const picked = [];
  const cand = candidates.slice();

  // helper: cosine similarity (you already have it)
  const sim = (a, b) => cosine(a, b);

  while (picked.length < k && cand.length > 0) {
    let bestIdx = 0,
      bestScore = -Infinity;

    for (let i = 0; i < cand.length; i++) {
      const rel = sim(queryVec, cand[i].vector); // compute on the fly for this cand[i]

      let div = 0;
      if (picked.length) {
        let maxSim = -Infinity;
        for (const p of picked) {
          const s = sim(cand[i].vector, p.vector);
          if (s > maxSim) maxSim = s;
        }
        div = maxSim;
      }

      const score = lambda * rel - (1 - lambda) * div;
      if (score > bestScore) {
        bestScore = score;
        bestIdx = i;
      }
    }

    picked.push(cand.splice(bestIdx, 1)[0]);
  }

  return picked;
}

function toPretty(hit) {
  const p = hit.payload;
  return {
    fileId: p.fileId,
    courseId: p.courseId,
    startMs: p.startMs,
    endMs: p.endMs,
    start: msToTimestamp(p.startMs),
    end: msToTimestamp(p.endMs),
    preview: p.text.slice(0, 180) + (p.text.length > 180 ? '…' : ''),
    text: p.text,
    lineIndices: p.lineIndices,
    score: hit.score,
  };
}

/**
 * Search subtitle chunks by semantic similarity.
 * Strict behavior: no silent fallback to no-MMR; allow a confidence gate.
 *
 * @param {object} opts
 * @param {string} opts.query
 * @param {string} [opts.courseId]
 * @param {string} [opts.fileId]
 * @param {number} [opts.topK=80]
 * @param {number} [opts.mmrK=12]
 * @param {boolean} [opts.useMMR=true]
 * @param {boolean} [opts.strictMMR=true] - if MMR picks 0, return no results
 * @param {number} [opts.minScore=0.52]   - confidence threshold on the best hit
 */
export async function searchChunks({
  query,
  courseId,
  fileId,
  topK = 80,
  mmrK = 12,
  useMMR = true,
  strictMMR = true,
  minScore = 0.52,
}) {
  if (!query || !query.trim()) throw new Error('query is required');

  const embedder = getEmbedder();
  const client = getQdrant();

  logger.info({ query, courseId, topK, mmrK }, '[Retrieval] Starting chunk search...');
  const qVec = await embedder.embedQuery(query);

  let res;
  try {
    res = await client.search(collectionName, {
      vector: qVec,
      limit: topK,
      with_payload: true,
      with_vector: useMMR, // only fetch vectors when we plan MMR
      filter: buildFilter({ courseId, fileId }),
    });
  } catch (err) {
    logger.error({ error: err.message, collection: collectionName }, '[Retrieval] Qdrant search failed');
    throw err;
  }

  logger.info({ candidatesFound: res.length, topCandidateScore: res[0]?.score }, '[Retrieval] Qdrant candidates returned');

  if (!res.length) {
    logger.info('[Retrieval] No candidates matched query/filter');
    return {
      query,
      results: [],
      meta: {
        topK,
        mmr: useMMR,
        reason: 'no_candidates',
        filter: { courseId: courseId ?? null, fileId: fileId ?? null },
      },
    };
  }


  // Confidence gate on the best raw result
  const bestScore = res[0]?.score ?? 0;
  if (bestScore < minScore) {
    return {
      query,
      results: [],
      meta: {
        topK,
        mmr: useMMR,
        reason: 'low_confidence',
        bestScore,
        minScore,
        filter: { courseId: courseId ?? null, fileId: fileId ?? null },
      },
    };
  }

  // after you get `res` from Qdrant
  let finalHits = res;

  if (useMMR) {
    const withVecs = res
      .map((h) => {
        const v = extractVector(h);
        return v ? { ...h, vector: v } : null;
      })
      .filter(Boolean);

    const picked = mmr(withVecs, qVec, Math.min(mmrK, withVecs.length), 0.6);
    logger.info({ pickedCount: picked.length, requestedMMRK: mmrK }, '[Retrieval] MMR diversification complete');

    if (picked.length === 0 && strictMMR) {
      logger.warn('[Retrieval] MMR returned 0 results with strictMMR=true');
      return {
        query,
        results: [],
        meta: {
          topK,
          mmr: true,
          reason: 'mmr_empty',
          bestScore,
          minScore,
          filter: { courseId: courseId ?? null, fileId: fileId ?? null },
        },
      };
    }
    finalHits = picked.length ? picked : res; // only falls back when strictMMR=false
  }


  const results = finalHits.map(toPretty);
  return {
    query,
    results,
    meta: {
      topK,
      mmr: useMMR,
      mmrKept: results.length,
      bestScore,
      minScore,
      filter: { courseId: courseId ?? null, fileId: fileId ?? null },
    },
  };
}
function extractVector(hit) {
  if (Array.isArray(hit.vector)) return hit.vector; // unnamed vector
  if (hit.vectors && typeof hit.vectors === 'object') {
    const first = Object.values(hit.vectors)[0];
    if (Array.isArray(first)) return first; // named vector
  }
  return null;
}
