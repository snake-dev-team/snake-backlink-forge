import { crx } from "@crxjs/vite-plugin";
import { type Connect, defineConfig, type Plugin } from "vite";
import manifest from "./src/manifest.config";

const allowedDevOrigins = new Set(["http://127.0.0.1:5173", "http://localhost:5173"]);

function devOriginGuard(): Plugin {
  return {
    name: "sbf-dev-origin-guard",
    configureServer(server) {
      server.middlewares.use((req: Connect.IncomingMessage, res, next) => {
        const source = req.headers.origin ?? req.headers.referer;
        if (source) {
          try {
            const origin = new URL(source).origin;
            if (!allowedDevOrigins.has(origin)) {
              res.statusCode = 403;
              res.end("forbidden_origin");
              return;
            }
          } catch {
            res.statusCode = 403;
            res.end("forbidden_origin");
            return;
          }
        }
        next();
      });
    },
  };
}

export default defineConfig({
  plugins: [devOriginGuard(), crx({ manifest })],
  server: {
    host: "127.0.0.1",
    port: 5173,
    strictPort: true,
    cors: false,
    fs: {
      strict: true,
      allow: ["src", "dist"],
      deny: [".env", ".env.*", "*.pem", "*.key", "*.crt", "package-lock.json"],
    },
  },
});
