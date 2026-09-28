import 'dotenv/config';
import Fastify from 'fastify';
import cors from '@fastify/cors';
import askRoutes from './routes/ask.js';

const app = Fastify({
  logger: {
    transport: {
      target: 'pino-pretty',
      options: { translateTime: 'SYS:standard' },
    },
  },
});

await app.register(cors, { origin: true });

app.get('/health', async (_req, _reply) => {
  return { ok: true, service: 'subtitle-rag-api', version: '0.1.0' };
});

app.get('/', async () => {
  return {
    ok: true,
    service: 'subtitle-rag-api',
    message: 'Welcome. Try GET /health',
  };
});
// register ask routes
await app.register(askRoutes);

const port = process.env.PORT ? Number(process.env.PORT) : 3001;
try {
  await app.listen({ port, host: '0.0.0.0' });
  app.log.info(`API listening on http://localhost:${port}`);
} catch (err) {
  app.log.error(err);
  process.exit(1);
}
