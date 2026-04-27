import type { ApiEvent } from "./types";

export type EventHandler = (e: ApiEvent) => void;

export class EventStream {
  private es: EventSource | null = null;
  private handlers: EventHandler[] = [];
  private retry = 1000;

  start() {
    this.connect();
  }

  stop() {
    this.es?.close();
    this.es = null;
  }

  on(h: EventHandler) {
    this.handlers.push(h);
    return () => {
      this.handlers = this.handlers.filter((x) => x !== h);
    };
  }

  private connect() {
    this.es = new EventSource("/api/events");
    this.es.onmessage = (ev) => {
      try {
        const data = JSON.parse(ev.data) as ApiEvent;
        for (const h of this.handlers) h(data);
      } catch {
        // ignore malformed
      }
    };
    this.es.onerror = () => {
      this.es?.close();
      setTimeout(() => this.connect(), this.retry);
      this.retry = Math.min(this.retry * 2, 30_000);
    };
    this.es.onopen = () => {
      this.retry = 1000;
    };
  }
}
