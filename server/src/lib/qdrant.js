// Qdrant helpers

import { QdrantClient } from '@qdrant/js-client-rest';

const QDRANT_URL = process.env.QDRANT_URL || 'http://localhost:6333';
const QDRANT_API_KEY = process.env.QDRANT_API_KEY || '';
const COLLECTION = process.env.QDRANT_COLLECTION || 'subs_chunks';

export function getQdrant() {
  return new QdrantClient({
    url: QDRANT_URL,
    apiKey: QDRANT_API_KEY || undefined,
  });
}

/**
 * Ensure a collection exists with the right vector size and cosine metric.
 * @param {QdrantClient} client
 * @param {number} dim - vector dimension (e.g., 768)
 */
export async function ensureCollection(client, dim) {
  const exists = await client
    .getCollections()
    .then((r) => r.collections?.some((c) => c.name === COLLECTION));

  if (!exists) {
    await client.createCollection(COLLECTION, {
      vectors: { size: dim, distance: 'Cosine' },
      optimizers_config: { default_segment_number: 2 },
    });
    return;
  }

  // Optional: validate dimension (skip strict check for simplicity)
}

export const collectionName = COLLECTION;
