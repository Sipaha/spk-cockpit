import { defineConfig } from "vite";
import type { Plugin } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import http from "node:http";
import os from "node:os";
import path from "node:path";

const socketPath = path.join(
  process.env.SPK_COCKPIT_STATE_DIR ||
    path.join(os.homedir(), ".spk", "spk-cockpit", "state"),
  "cockpit.sock",
);

// udsApiProxy forwards /api/* requests from the dev server to the running
// daemon's Unix socket. Vite's built-in proxy expects a host:port target,
// which doesn't compose with a Unix socket; using a raw http.request with
// `socketPath` is the simplest and most reliable workaround.
function udsApiProxy(): Plugin {
  return {
    name: "spk-cockpit-uds-api-proxy",
    configureServer(server) {
      server.middlewares.use((req, res, next) => {
        if (!req.url || !req.url.startsWith("/api/")) {
          next();
          return;
        }
        const proxyReq = http.request(
          {
            socketPath,
            path: req.url,
            method: req.method,
            headers: req.headers,
          },
          (proxyRes) => {
            res.writeHead(proxyRes.statusCode ?? 502, proxyRes.headers);
            proxyRes.pipe(res);
          },
        );
        proxyReq.on("error", (err) => {
          // Daemon isn't running or path is wrong — surface a 502 so the UI
          // shows a real error message instead of "Unexpected token <".
          if (!res.headersSent) {
            res.writeHead(502, { "Content-Type": "application/json" });
          }
          res.end(
            JSON.stringify({
              error: { code: "uds_proxy", message: err.message },
            }),
          );
        });
        req.pipe(proxyReq);
      });
    },
  };
}

export default defineConfig({
  plugins: [react(), tailwindcss(), udsApiProxy()],
  server: { port: 5173 },
  build: {
    outDir: "dist",
    emptyOutDir: true,
  },
  test: {
    environment: "jsdom",
    globals: true,
    setupFiles: ["./src/test-setup.ts"],
  },
});
