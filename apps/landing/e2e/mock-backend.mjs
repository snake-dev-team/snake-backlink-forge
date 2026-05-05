import http from "node:http";

const port = Number(process.env.E2E_MOCK_BACKEND_PORT ?? 3101);

function json(res, status, body) {
  res.writeHead(status, { "content-type": "application/json" });
  res.end(JSON.stringify(body));
}

function readJson(req, callback) {
  let body = "";
  req.on("data", (chunk) => {
    body += chunk;
  });
  req.on("end", () => {
    callback(JSON.parse(body || "{}"));
  });
}

const server = http.createServer((req, res) => {
  const url = new URL(req.url ?? "/", `http://${req.headers.host}`);

  if (req.method === "POST" && url.pathname === "/api/v1/auth/verify") {
    readJson(req, (body) => {
      if (body.key === "sbf_live_bad_e2e_key") {
        json(res, 401, { error: "invalid_key" });
        return;
      }
      json(res, 200, {
        ok: true,
        user: { id: "00000000-0000-0000-0000-000000000001", key_prefix: "sbf_live_pha" },
      });
    });
    return;
  }

  if (url.pathname === "/api/v1/me") {
    json(res, 200, {
      user_id: "00000000-0000-0000-0000-000000000001",
      telegram_id: 990000000001,
      telegram_username: "phase8_e2e",
      language: "vi",
      is_verified: true,
      balance_credits: 42,
      premium_credits: 7,
      standard_credits: 35,
      key_prefix: "sbf_live_pha",
    });
    return;
  }

  if (url.pathname === "/api/v1/transactions") {
    json(res, 200, { items: [], limit: Number(url.searchParams.get("limit") ?? 5), offset: 0 });
    return;
  }

  if (url.pathname === "/api/v1/wp-sites") {
    if (req.method === "POST") {
      readJson(req, (body) => {
        if (body.app_password === "wrong-password") {
          json(res, 401, { error: "wp_invalid_credentials" });
          return;
        }
        if (body.base_url === "https://server-error.example.com") {
          json(res, 502, { error: "wp_server_error_502" });
          return;
        }
        json(res, 201, {
          item: {
            id: "00000000-0000-0000-0000-000000000002",
            base_url: "https://example.com",
            app_username: "admin",
            label: "E2E site",
            status: "connected",
            last_validated_at: "2026-04-28T00:00:00Z",
            last_error: null,
            created_at: "2026-04-28T00:00:00Z",
            updated_at: "2026-04-28T00:00:00Z",
          },
        });
      });
      return;
    }
    json(res, 200, { items: [] });
    return;
  }

  if (url.pathname === "/api/v1/echo") {
    let size = 0;
    req.on("data", (chunk) => {
      size += chunk.length;
    });
    req.on("end", () => json(res, 200, { size }));
    return;
  }

  json(res, 404, { error: "not_found" });
});

server.listen(port, "127.0.0.1", () => {
  console.log(`mock backend ready on ${port}`);
});
