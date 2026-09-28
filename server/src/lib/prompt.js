// server/src/lib/prompt.js
import { msToTimestamp } from './time.js';

/**
 * Build the system+user prompt from query + retrieved chunks
 * Citations should be [fileId start–end], using timestamps.
 */
export function buildPrompt({ query, chunks }) {
  // compact the context: show only what the model needs (file/time + text)
  const context = chunks
    .map((c, i) => {
      const range = `${msToTimestamp(c.startMs)}–${msToTimestamp(c.endMs)}`;
      return `[#${i + 1}] [${c.fileId} ${range}]\n${c.text}`;
    })
    .join('\n\n');

  const system = [
    'You are a helpful tutor answering strictly from the provided context.',
    'Rules:',
    '- Use ONLY the context snippets below; do not invent details.',
    '- If the answer is not clear from the context, say: "I dont know, but I could help in other areas like nodejs and python."',
    '- Always include citations in square brackets like [fileId start–end] next to the claim they support.',
    '- Prefer concise, step-by-step explanations and in easy language.',
  ].join('\n');

  const user = [`User question: ${query}`, '', 'Context:', context || '(empty)'].join('\n');

  return { system, user };
}
