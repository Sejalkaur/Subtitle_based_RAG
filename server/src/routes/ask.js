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

      // enforce your “don’t know” behavior inside answerQuery
      const out = await answerQuery({ query, courseId, topK, mmrK, minScore, strictMMR });
      return reply.send(out);
    },
  );
}
