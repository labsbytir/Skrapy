interface Env {
  DATA_BUCKET: R2Bucket;
  VERIFY_QUEUE: Queue;
}

const BLOCKED_DOMAINS = new Set([
  'example.test',
  'localhost',
  '127.0.0.1',
]);

function json(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
}

// Simple PII scan: emails, phones, SSNs-ish patterns. Not exhaustive.
const EMAIL_RE = /[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}/;
const PHONE_RE = /\b\d{3}[-.\s]?\d{3}[-.\s]?\d{4}\b/;

function scanLine(line: string): { clean: boolean; reasons: string[] } {
  const reasons: string[] = [];
  if (EMAIL_RE.test(line)) reasons.push('possible_email');
  if (PHONE_RE.test(line)) reasons.push('possible_phone');
  return { clean: reasons.length === 0, reasons };
}

function parseDomain(url: string): string | null {
  try {
    return new URL(url).hostname;
  } catch {
    return null;
  }
}

async function verifyObject(key: string, env: Env): Promise<{ verified: number; quarantined: number }> {
  const obj = await env.DATA_BUCKET.get(key);
  if (!obj) return { verified: 0, quarantined: 0 };

  const text = await obj.text();
  const lines = text.split('\n').filter((l) => l.trim());
  let verified = 0;
  let quarantined = 0;
  const outLines: string[] = [];
  const quarLines: string[] = [];

  const meta = obj.customMetadata ?? {};
  const sourceDomain = parseDomain(meta.source || '');
  if (sourceDomain && BLOCKED_DOMAINS.has(sourceDomain)) {
    quarLines.push(...lines);
    quarantined += lines.length;
  } else {
    for (const line of lines) {
      let ok = true;
      try {
        JSON.parse(line);
      } catch {
        ok = false;
      }
      if (!ok) {
        quarLines.push(line);
        quarantined += 1;
        continue;
      }
      const scan = scanLine(line);
      if (!scan.clean) {
        quarLines.push(line);
        quarantined += 1;
        continue;
      }
      outLines.push(line);
      verified += 1;
    }
  }

  const base = key.replace(/^uploads\//, '');
  if (outLines.length > 0) {
    await env.DATA_BUCKET.put(`verified/${base}`, outLines.join('\n'), {
      httpMetadata: { contentType: 'application/jsonl' },
    });
  }
  if (quarLines.length > 0) {
    await env.DATA_BUCKET.put(`quarantined/${base}`, quarLines.join('\n'), {
      httpMetadata: { contentType: 'application/jsonl' },
    });
  }

  // Delete original upload after processing
  await env.DATA_BUCKET.delete(key);

  return { verified, quarantined };
}

export default {
  async fetch(request: Request, env: Env): Promise<Response> {
    const url = new URL(request.url);
    if (request.method === 'POST' && url.pathname === '/verify') {
      const { prefix } = (await request.json()) as { prefix?: string };
      const listed = await env.DATA_BUCKET.list({ prefix: prefix || 'uploads/' });
      let v = 0, q = 0;
      for (const obj of listed.objects) {
        const r = await verifyObject(obj.key, env);
        v += r.verified;
        q += r.quarantined;
      }
      return json({ verified: v, quarantined: q });
    }
    return json({ error: 'not found' }, 404);
  },

  async queue(batch: MessageBatch, env: Env): Promise<void> {
    let v = 0, q = 0;
    for (const msg of batch.messages) {
      const body = msg.body as { object_key?: string };
      if (body.object_key) {
        const r = await verifyObject(body.object_key, env);
        v += r.verified;
        q += r.quarantined;
        msg.ack();
      } else {
        msg.retry();
      }
    }
    console.log(`verified=${v} quarantined=${q}`);
  },
} satisfies ExportedHandler<Env>;