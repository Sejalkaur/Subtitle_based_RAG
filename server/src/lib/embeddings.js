// Embeddings provider wrapper (Gemini) — FIXED content shape + batching

import 'dotenv/config';
import { GoogleGenerativeAI, TaskType } from '@google/generative-ai';

const PROVIDER = process.env.EMBEDDINGS_PROVIDER || 'gemini';
const GEMINI_API_KEY = process.env.GEMINI_API_KEY;
const MODEL = process.env.EMBEDDINGS_MODEL || 'text-embedding-004';
const DIM = Number(process.env.EMBEDDINGS_DIM || 768);

function contentOf(text) {
  // Gemini expects a Content object with parts[].text
  return { parts: [{ text: String(text ?? '') }] };
}

export function getEmbedder() {
  if (PROVIDER !== 'gemini') {
    throw new Error(`Only 'gemini' is implemented right now. Set EMBEDDINGS_PROVIDER=gemini`);
  }
  if (!GEMINI_API_KEY) {
    throw new Error(`GEMINI_API_KEY is missing in your .env`);
  }

  const genAI = new GoogleGenerativeAI(GEMINI_API_KEY);
  const model = genAI.getGenerativeModel({ model: MODEL });

  return {
    // Embed a single document chunk (for upserting to Qdrant)
    async embedOne(text) {
      const r = await model.embedContent({
        content: contentOf(text),
        taskType: TaskType.RETRIEVAL_DOCUMENT,
        outputDimensionality: DIM,
      });
      return r.embedding.values;
    },

    // Embed many document chunks (batch). Tries batch endpoint, falls back to per-item loop.
    async embedMany(texts) {
      if (!Array.isArray(texts) || texts.length === 0) return [];
      // Prefer batch API if available in this SDK version
      if (typeof model.batchEmbedContents === 'function') {
        const requests = texts.map((t) => ({
          content: contentOf(t),
          taskType: TaskType.RETRIEVAL_DOCUMENT,
          outputDimensionality: DIM,
        }));
        const res = await model.batchEmbedContents({ requests });
        // res.embeddings is an array of { values: number[] }
        return res.embeddings.map((e) => e.values);
      }
      // Fallback: one-by-one
      const out = [];
      for (const t of texts) {
        const r = await model.embedContent({
          content: contentOf(t),
          taskType: TaskType.RETRIEVAL_DOCUMENT,
          outputDimensionality: DIM,
        });
        out.push(r.embedding.values);
      }
      return out;
    },

    // Embed a user query (slightly different taskType improves retrieval)
    async embedQuery(text) {
      const r = await model.embedContent({
        content: contentOf(text),
        taskType: TaskType.RETRIEVAL_QUERY,
        outputDimensionality: DIM,
      });
      return r.embedding.values;
    },

    dim: DIM,
    model: MODEL,
    provider: 'gemini',
  };
}
