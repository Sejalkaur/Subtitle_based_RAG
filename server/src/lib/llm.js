// server/src/lib/llm.js
import 'dotenv/config';
import { GoogleGenerativeAI } from '@google/generative-ai';

import { logger } from './logger.js';

const GEMINI_API_KEY = process.env.GEMINI_API_KEY;
const CHAT_MODEL = process.env.CHAT_MODEL || 'gemini-3.8-flash';

if (!GEMINI_API_KEY) {
  logger.error('[LLM] GEMINI_API_KEY is missing in your .env');
  throw new Error('GEMINI_API_KEY missing');
}

const genAI = new GoogleGenerativeAI(GEMINI_API_KEY);
const model = genAI.getGenerativeModel({ model: CHAT_MODEL });

/**
 * Chat with Gemini using OpenAI-like messages -> {role, content}
 * @param {Array<{role:'system'|'user'|'assistant', content:string}>} messages
 * @param {object} [opts]
 * @param {number} [opts.temperature=0.2]
 */
export async function chat(messages, { temperature = 0.2 } = {}) {
  // Convert to Gemini "parts" format
  const contents = messages.map((m) => ({
    role: m.role === 'system' ? 'user' : m.role, // Gemini lacks explicit system, we fold it in
    parts: [{ text: m.content }],
  }));

  logger.info({ model: CHAT_MODEL, messageCount: messages.length }, '[LLM] Sending prompt to Gemini model...');
  const startTime = Date.now();

  // Try configured CHAT_MODEL, fallback to gemini-3.5-flash if high-demand 503 occurs
  const modelsToTry = [CHAT_MODEL];
  if (CHAT_MODEL !== 'gemini-3.5-flash') {
    modelsToTry.push('gemini-3.5-flash');
  }

  let lastErr;
  for (const mName of modelsToTry) {
    try {
      const activeModel = genAI.getGenerativeModel({ model: mName });
      const res = await activeModel.generateContent({
        contents,
        generationConfig: {
          temperature,
        },
      });

      const durationMs = Date.now() - startTime;
      const text = res.response?.text?.() ?? '';
      logger.info({ model: mName, durationMs, responseLength: text.length }, '[LLM] Response received successfully');

      return { text, raw: res };
    } catch (err) {
      lastErr = err;
      if (err.status === 503 || err.message?.includes('503') || err.message?.includes('high demand')) {
        logger.warn(`[LLM] Model "${mName}" busy (503 High Demand). Retrying with fallback model...`);
        continue;
      }
      break;
    }
  }

  const durationMs = Date.now() - startTime;
  logger.error({ model: CHAT_MODEL, durationMs, error: lastErr.message }, '[LLM] Error from Gemini API');

  if (lastErr.message?.includes('404') || lastErr.message?.includes('not found')) {
    logger.error(
      `[LLM HINT] Model "${CHAT_MODEL}" was not found or is not available for your API key. Check server/.env`
    );
  } else if (lastErr.message?.includes('API_KEY_INVALID') || lastErr.message?.includes('API key not valid')) {
    logger.error(
      `[LLM HINT] Your GEMINI_API_KEY in server/.env appears invalid. Verify your key at https://aistudio.google.com/apikey`
    );
  }

  throw lastErr;
}


