// server/src/lib/llm.js
import 'dotenv/config';
import { GoogleGenerativeAI } from '@google/generative-ai';

const GEMINI_API_KEY = process.env.GEMINI_API_KEY;
const CHAT_MODEL = process.env.CHAT_MODEL || 'gemini-2.5-flash'; // fast & cheap; swap if you like

if (!GEMINI_API_KEY) {
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

  const res = await model.generateContent({
    contents,
    generationConfig: {
      temperature,
    },
  });

  const text = res.response?.text?.() ?? '';
  return { text, raw: res };
}
