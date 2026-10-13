// server/src/routes/ask.js
import { answerQuery } from '../lib/answer.js';

export default async function askRoutes(fastify) {
  fastify.post(
    '/api/ask',
    {
      schema: {
        body: {
          type: 'object',
          required: ['query'],
          properties: {
            query: { type: 'string', minLength: 1 },
            courseId: { type: 'string', nullable: true },
            topK: { type: 'integer', minimum: 1, maximum: 1000 },
            mmrK: { type: 'integer', minimum: 1, maximum: 100 },
            minScore: { type: 'number', minimum: 0, maximum: 1 },
            strictMMR: { type: 'boolean' },
          },
        },
      },
    },
    async (req, reply) => {
      const {
        query,
        courseId = null,
        topK = 80,
        mmrK = 12,
        minScore = 0.55,
        strictMMR = true,
      } = req.body;

      req.log.info({ query, courseId, topK, mmrK, minScore }, 'Incoming /api/ask request');
      const startTime = Date.now();

      try {
        // enforce your “don’t know” behavior inside answerQuery
        const out = await answerQuery({ query, courseId, topK, mmrK, minScore, strictMMR });
        const durationMs = Date.now() - startTime;
        req.log.info(
          {
            durationMs,
            citationsCount: out.citations?.length || 0,
            hasAnswer: Boolean(out.answer),
          },
          '/api/ask completed successfully'
        );
        return reply.send(out);
      } catch (err) {
        const durationMs = Date.now() - startTime;
        req.log.error(
          {
            err: err.message,
            stack: err.stack,
            durationMs,
            query,
          },
          '/api/ask failed with error'
        );

        return reply.status(500).send({
          error: 'Failed to process question',
          message: err.message,
          hint: err.message?.includes('API_KEY')
            ? 'Please check your GEMINI_API_KEY in server/.env'
            : err.message?.includes('404')
            ? 'Model not found. Please verify CHAT_MODEL in server/.env'
            : undefined,
        });
      }
    },

  );
}
