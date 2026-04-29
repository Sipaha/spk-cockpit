import type { ApiEvent } from "./types";

export type EventHandler = (e: ApiEvent) => void;

export class EventStream {
  private es: EventSource | null = null;
  private handlers: EventHandler[] = [];
  private retry = 1000;
  private retryTimer: ReturnType<typeof setTimeout> | null = null;
  private stopped = false;

  start() {
    this.stopped = false;
    this.connect();
  }

  // stop closes the current EventSource AND cancels any pending reconnect
  // timer. Without this, a stop() during the backoff window would leave a
  // setTimeout that later constructs a fresh EventSource nobody owns.
  stop() {
    this.stopped = true;
    this.es?.close();
    this.es = null;
    if (this.retryTimer !== null) {
      clearTimeout(this.retryTimer);
      this.retryTimer = null;
    }
  }

  on(h: EventHandler) {
    this.handlers.push(h);
    return () => {
      this.handlers = this.handlers.filter((x) => x !== h);
    };
  }

  private connect() {
    if (this.stopped) return;
    this.es = new EventSource("/api/events");
    this.es.onmessage = (ev) => {
      try {
        const data = JSON.parse(ev.data) as ApiEvent;
        // Snapshot before iteration so a handler synchronously calling off()
        // doesn't perturb the live array we're walking.
        for (const h of [...this.handlers]) h(data);
      } catch {
        // ignore malformed
      }
    };
    this.es.onerror = () => {
      this.es?.close();
      this.es = null;
      if (this.stopped) return;
      this.retryTimer = setTimeout(() => {
        this.retryTimer = null;
        this.connect();
      }, this.retry);
      this.retry = Math.min(this.retry * 2, 30_000);
    };
    this.es.onopen = () => {
      this.retry = 1000;
    };
  }
}
