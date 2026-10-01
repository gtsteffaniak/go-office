import type { FullConfig } from "@playwright/test";

const HEARTBEAT_MS = Number(process.env.PLAYWRIGHT_HEARTBEAT_MS ?? 10_000);

let heartbeat: ReturnType<typeof setInterval> | null = null;
const startedAt = Date.now();

export default async function globalSetup(_config: FullConfig): Promise<void> {
  if (process.env.PLAYWRIGHT_HEARTBEAT === "0") {
    return;
  }
  heartbeat = setInterval(() => {
    const elapsed = Math.round((Date.now() - startedAt) / 1000);
    process.stderr.write(`playwright: heartbeat elapsed=${elapsed}s\n`);
  }, HEARTBEAT_MS);
  // Do not pin the Node event loop if teardown is skipped on an abnormal exit path.
  heartbeat.unref();
}

export async function globalTeardown(): Promise<void> {
  if (heartbeat) {
    clearInterval(heartbeat);
    heartbeat = null;
  }
}
