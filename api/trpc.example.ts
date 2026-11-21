// tRPC example (TypeScript/Bun). Requires a JS runtime in the Wolfi image.
import { initTRPC } from '@trpc/server';
const t = initTRPC.create();
export const appRouter = t.router({
  list: t.procedure.query(() => [] as { id: string; title: string }[]),
});
export type AppRouter = typeof appRouter;