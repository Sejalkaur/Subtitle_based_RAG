import 'dotenv/config';
import Fastify from 'fastify';
import cors from '@fastify/cors';
import askRoutes from './routes/ask.js';

import { getQdrant } from './lib/qdrant.js';

const app = Fastify({
  logger: {
    transport: {
      target: 'pino-pretty',
      options: {
        colorize: true,
        translateTime: 'yyyy-mm-dd HH:MM:ss',
        ignore: 'pid,hostname',
      },
    },
  },
});

await app.register(cors, { origin: true });

app.get('/health', async (_req, _reply) => {
  return {
    ok: true,
    service: 'subtitle-rag-api',
    version: '0.1.0',
    chatModel: process.env.CHAT_MODEL || 'gemini-3.8-flash',
    embeddingsModel: process.env.EMBEDDINGS_MODEL || 'text-embedding-004',
  };
});

app.get('/', async () => {
  return {
    ok: true,
    service: 'subtitle-rag-api',
    message: 'Welcome. Try GET /health or POST /api/ask',
  };
});

// register ask routes
await app.register(askRoutes);

// Global unhandled error handler
app.setErrorHandler((error, request, reply) => {
  request.log.error({ err: error.message, stack: error.stack }, 'Unhandled Server Error');
  reply.status(error.statusCode || 500).send({
    ok: false,
    error: error.name || 'InternalServerError',
    message: error.message || 'Something went wrong',
  });
});

const port = process.env.PORT ? Number(process.env.PORT) : 3001;

try {
  await app.listen({ port, host: '0.0.0.0' });

  app.log.info('==========================================');
  app.log.info(`🚀 API running on http://localhost:${port}`);
  app.log.info(`🤖 Chat Model: ${process.env.CHAT_MODEL || 'gemini-3.8-flash'}`);
  app.log.info(`📐 Embeddings: ${process.env.EMBEDDINGS_MODEL || 'text-embedding-004'} (${process.env.EMBEDDINGS_PROVIDER || 'gemini'})`);
  app.log.info(`📦 Qdrant DB:  ${process.env.QDRANT_URL || 'http://localhost:6333'} [Collection: ${process.env.QDRANT_COLLECTION || 'subs_chunks'}]`);
  app.log.info('==========================================');

  // Verify Qdrant connection in the background
  try {
    const qdrant = getQdrant();
    const res = await qdrant.getCollections();
    const names = res.collections?.map((c) => c.name) || [];
    app.log.info({ collections: names }, '✅ Successfully connected to Qdrant');
  } catch (dbErr) {
    app.log.warn(
      `⚠️  Cannot reach Qdrant at ${process.env.QDRANT_URL || 'http://localhost:6333'}. Make sure Docker is running: run "docker compose up -d"`
    );
  }
} catch (err) {
  app.log.error(err, 'Failed to start server');
  process.exit(1);
}

