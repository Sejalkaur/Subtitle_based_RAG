// Embeddings provider wrapper (Gemini) — FIXED content shape + batching

import 'dotenv/config';
import { GoogleGenerativeAI, TaskType } from '@google/generative-ai';
import { logger } from './logger.js';

const PROVIDER = process.env.EMBEDDINGS_PROVIDER || 'gemini';
const GEMINI_API_KEY = process.env.GEMINI_API_KEY;
const MODEL = process.env.EMBEDDINGS_MODEL || 'gemini-embedding-2';
const DIM = Number(process.env.EMBEDDINGS_DIM || 768);

function contentOf(text) {
  // Gemini expects a Content object with parts[].text
  return { parts: [{ text: String(text ?? '') }] };
}

async function embedWithRetry(model, text, taskType, dim, maxRetries = 5) {
  let attempt = 0;
  while (true) {
    try {
      const r = await model.embedContent({
        content: contentOf(text),
        taskType,
        outputDimensionality: dim,
      });
      return r.embedding.values;
    } catch (err) {
      attempt++;
      const isRateLimit =
        err.status === 429 ||
        err.message?.includes('429') ||
        err.message?.includes('Too Many Requests') ||
        err.message?.includes('Quota exceeded');

      if (isRateLimit && attempt <= maxRetries) {
        let delayMs = 31000; // default 31s
        const match = err.message?.match(/retry in ([0-9.]+)s/i);
        if (match && match[1]) {
          delayMs = Math.ceil(parseFloat(match[1]) * 1000) + 2000;
        }
        logger.warn(
          `[Embeddings] Rate limit (100 RPM) hit. Pausing for ${(delayMs / 1000).toFixed(1)}s before retry (attempt ${attempt}/${maxRetries})...`
        );
        await new Promise((resolve) => setTimeout(resolve, delayMs));
        continue;
      }
      throw err;
    }
  }
}

export function getEmbedder() {
  if (PROVIDER !== 'gemini') {
    logger.error(`[Embeddings] Unsupported provider: ${PROVIDER}`);
    throw new Error(`Only 'gemini' is implemented right now. Set EMBEDDINGS_PROVIDER=gemini`);
  }
  if (!GEMINI_API_KEY) {
    logger.error('[Embeddings] GEMINI_API_KEY is missing in your .env');
    throw new Error(`GEMINI_API_KEY is missing in your .env`);
  }

  const genAI = new GoogleGenerativeAI(GEMINI_API_KEY);
  const model = genAI.getGenerativeModel({ model: MODEL });

  return {
    // Embed a single document chunk (for upserting to Qdrant)
    async embedOne(text) {
      try {
        return await embedWithRetry(model, text, TaskType.RETRIEVAL_DOCUMENT, DIM);
      } catch (err) {
        logger.error({ error: err.message, model: MODEL }, '[Embeddings] Error embedding single text chunk');
        throw err;
      }
    },

    // Embed many document chunks (batch). Runs paced sub-batches with auto-retry.
    async embedMany(texts) {
      if (!Array.isArray(texts) || texts.length === 0) return [];
      logger.info({ count: texts.length, model: MODEL }, '[Embeddings] Generating embeddings for chunk batch...');
      const startTime = Date.now();

      try {
        // Run in sub-batches of 4 concurrent requests with slight delay to avoid rate limit spikes
        const CONCURRENCY = 4;
        const out = [];

        for (let i = 0; i < texts.length; i += CONCURRENCY) {
          const slice = texts.slice(i, i + CONCURRENCY);
          const sliceResults = await Promise.all(
            slice.map((t) => embedWithRetry(model, t, TaskType.RETRIEVAL_DOCUMENT, DIM))
          );
          out.push(...sliceResults);

          if (i + CONCURRENCY < texts.length) {
            await new Promise((resolve) => setTimeout(resolve, 200));
          }
        }

        logger.info({ count: texts.length, durationMs: Date.now() - startTime }, '[Embeddings] Batch embeddings complete');
        return out;
      } catch (err) {
        logger.error({ count: texts.length, error: err.message, model: MODEL }, '[Embeddings] Batch embedding error');
        throw err;
      }
    },

    // Embed a user query (slightly different taskType improves retrieval)
    async embedQuery(text) {
      const startTime = Date.now();
      try {
        const values = await embedWithRetry(model, text, TaskType.RETRIEVAL_QUERY, DIM);
        logger.info({ durationMs: Date.now() - startTime, model: MODEL }, '[Embeddings] Query vector generated');
        return values;
      } catch (err) {
        logger.error({ error: err.message, model: MODEL }, '[Embeddings] Query embedding error');
        if (err.message?.includes('API_KEY_INVALID') || err.message?.includes('API key not valid')) {
          logger.error('[Embeddings HINT] Your GEMINI_API_KEY in server/.env is invalid. Update it from Google AI Studio.');
        }
        throw err;
      }
    },

    dim: DIM,
    model: MODEL,
    provider: 'gemini',
  };
}

