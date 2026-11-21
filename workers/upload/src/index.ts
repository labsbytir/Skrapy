import { createHash } from 'crypto';

interface Env {
  API_KEYS: KVNamespace;
  DATA_BUCKET: R2Bucket;
}

const CHUNK_MAX_MB = 90;
const CHUNK_MAX_BYTES = CHUNK_MAX_MB * 1024 * 1024;

function json(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
}

function keyPath(k: string): string {
  return `keys/${createHash('sha256').update(k).digest('hex')}`;
}

export default {
  async fetch(request: Request, env: Env): Promise<Response> {
    const url = new URL(request.url);

    // Health check
    if (request.method === 'GET' && url.pathname === '/health') {
      return json({ ok: true });
    }

    // Issue a new API key (admin). Protected by ADMIN_TOKEN secret.
    if (request.method === 'POST' && url.pathname === '/keys') {
      const admin = request.headers.get('x-admin-token');
      if (admin !== env.ADMIN_TOKEN) {
        return json({ error: 'unauthorized' }, 401);
      }
      const { machine_id } = await request.json() as { machine_id?: string };
      if (!machine_id) {
        return json({ error: 'machine_id required' }, 400);
      }
      const token = `gk_${createHash('sha256').update(`${machine_id}:${Date.now()}`).digest('hex').slice(0, 32)}`;
      await env.API_KEYS.put(keyPath(token), JSON.stringify({ machine_id, created_at: new Date().toISOString() }));
      return json({ api_key: token });
    }

    // Validate a key
    if (request.method === 'GET' && url.pathname === '/keys/validate') {
      const token = request.headers.get('x-api-key');
      if (!token) return json({ error: 'missing api key' }, 401);
      const raw = await env.API_KEYS.get(keyPath(token));
      if (!raw) return json({ valid: false }, 401);
      return json({ valid: true, ...JSON.parse(raw) });
    }

    // Upload a batch chunk
    if (request.method === 'POST' && url.pathname === '/upload') {
      const token = request.headers.get('x-api-key');
      if (!token) return json({ error: 'missing api key' }, 401);
      const raw = await env.API_KEYS.get(keyPath(token));
      if (!raw) return json({ error: 'invalid api key' }, 401);
      const keyMeta = JSON.parse(raw);

      const batch_id = request.headers.get('x-batch-id');
      const chunk_index = request.headers.get('x-chunk-index');
      const chunk_total = request.headers.get('x-chunk-total');
      const source = request.headers.get('x-source') || 'unknown';
      if (!batch_id || !chunk_index || !chunk_total) {
        return json({ error: 'missing batch headers' }, 400);
      }

      const body = request.body;
      if (!body) return json({ error: 'no body' }, 400);

      const objectKey = `uploads/${batch_id}/chunk-${chunk_index.padStart(4, '0')}`;
      const data = await request.arrayBuffer();

      if (data.byteLength > CHUNK_MAX_BYTES) {
        return json({ error: `chunk exceeds ${CHUNK_MAX_MB}MB` }, 413);
      }

      await env.DATA_BUCKET.put(objectKey, data, {
        httpMetadata: {
          contentType: 'application/jsonl',
          customMetadata: {
            machine_id: keyMeta.machine_id,
            source,
            batch_id,
            chunk_index,
            chunk_total,
            uploaded_at: new Date().toISOString(),
          },
        },
      });

      return json({
        ok: true,
        object_key: objectKey,
        bytes: data.byteLength,
      });
    }

    return json({ error: 'not found' }, 404);
  },
} satisfies ExportedHandler<Env>;