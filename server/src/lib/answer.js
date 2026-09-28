// server/src/lib/answer.js
import { searchChunks } from './retrieval.js';
import { chat } from './llm.js';
import { msToTimestamp } from './time.js';
import { buildPrompt } from './prompt.js';

/**
 * Answer a query grounded on subtitles.
 * Enforces your "don't know" policy when retrieval is empty/low-confidence.
 */
export async function answerQuery({
  query,
  courseId,
  topK = 80,
  mmrK = 12,
  minScore = 0.55,
  strictMMR = true,
}) {
  // 1) retrieve
  const ret = await searchChunks({
    query,
    courseId,
    topK,
    mmrK,
    useMMR: true,
    strictMMR,
    minScore,
  });

  // 2) decide: if no results, return a "don't know" directly
  if (!ret.results.length) {
    return {
      answer: 'I don’t know, but I could help in other areas.',
      citations: [],
      used: { query, courseId, meta: ret.meta },
    };
  }

  // 3) build prompt using a compact slice of each chunk
  // Use the 'results' from retrieval; they already have start/end pretty strings,
  // but we also want raw ms to compute exact ranges when needed.
  // We need the *full* text for grounding, so fetch from payload shape we returned in searchChunks.
  // searchChunks.toPretty() kept text only as preview; so let's call retrieval again for payload text
  // OR (simpler) tweak searchChunks to include text in pretty result. Let's assume we add it:
  //   preview: ..., text: p.text
  // If you haven’t added it yet, please add `text: p.text` in toPretty().
  const chunks = ret.results.map((r) => ({
    fileId: r.fileId,
    startMs: r.startMs,
    endMs: r.endMs,
    text: r.preview && r.preview.length < 180 ? r.preview : r.text || r.preview, // fallback
  }));

  const { system, user } = buildPrompt({ query, chunks });

  // 4) LLM call
  const { text: answer } = await chat(
    [
      { role: 'system', content: system },
      { role: 'user', content: user },
    ],
    { temperature: 0.2 },
  );

  // 5) craft citations (we return a structured list the UI can render)
  const citations = ret.results.map((r) => ({
    courseId: r.courseId,
    fileId: r.fileId,
    start: r.start, // "hh:mm:ss.mmm"
    end: r.end,
    startMs: r.startMs,
    endMs: r.endMs,
    score: r.score,
  }));

  return {
    answer,
    citations,
    used: {
      query,
      courseId: courseId || null,
      topK,
      mmrK,
      minScore,
      strictMMR,
      meta: ret.meta,
    },
  };
}
