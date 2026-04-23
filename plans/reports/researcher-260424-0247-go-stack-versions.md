---
title: Go Stack Version Research — Phase 1 Foundation
date: 2026-04-24
---

# Go Stack Version Research — Phase 1 Foundation

## 1. Go Toolchain

**Latest stable: Go 1.26.2** (released 2026-04-07). Go 1.23 is EOL as of April 2026 — only 1.25 and 1.26 are actively supported. **Recommendation**: Upgrade to Go 1.26 for Phase 1. Go 1.23 (specified in MASTER_PROMPT) entered maintenance mode in late 2024 and receives no more patch updates. If you must stay on 1.23, pin to **Go 1.23.3** (last stable patch, released 2024-11-06), but expect zero security updates after April 2026.

**Compat break to avoid**: None between 1.23→1.26 for standard library usage with pgx/redis/Fiber. Minor GOARCH detection changes in CGO, irrelevant here (CGO=0).

Source: [Go Release Dashboard](https://go.dev/doc/devel/release)

---

## 2. Fiber v2

**Latest stable: v2.52.6** (confirmed in releases, exact publish date not stated but post-June 2024). Fiber v2 is production-ready. **Built-in middleware note**: Fiber's `logger.New()` and `recover.New()` middleware are built-in, but they do NOT natively output to Zap. **Recommendation for zap integration**: Use `github.com/gofiber/contrib/fiberzap/v2` (Fiber community contrib package). Custom `logger.New(Config{CustomTags: map[string]LogFunc{...}})` allows field injection, but full Zap integration requires `fiberzap`. The built-in recover middleware can be customized via `PanicHandler` callback but doesn't log to Zap by default.

**Gotcha**: fiberzap is third-party contrib, not bundled. Requires separate `go get github.com/gofiber/contrib/fiberzap/v2` + `go.uber.org/zap`.

Sources: [Fiber Releases](https://github.com/gofiber/fiber/releases), [fiberzap docs](https://docs.gofiber.io/contrib/fiberzap/), [Logger Middleware](https://docs.gofiber.io/next/middleware/logger/), [Recover Middleware](https://docs.gofiber.io/next/middleware/recover/)

---

## 3. jackc/pgx v5

**Latest stable: v5.9.1** (released before 2026-04-19 per pkg.go.dev timestamp). pgx v5 is pure Go, **no CGO required**. **pgxpool config for 1GB Fly shared-cpu-1x**:
- `MaxConns`: 15–20 (conservative; Fly shared-cpu-1x has ~1GB mem, each conn ~5–10MB overhead)
- `MinConns`: 2–3 (warm pool)
- `MaxConnLifetime`: 30 minutes (default 1h is fine)
- `MaxConnIdleTime`: 5 minutes (close idle fast to free memory)
- Health check: Built-in via `pool.Ping(ctx)` every 30s recommended in app init loop

**Example**:
```go
config, _ := pgxpool.ParseConfig("postgres://...?pool_max_conns=15&pool_min_conns=3&pool_max_conn_lifetime=30m")
```

**Gotcha**: pgx v5 is pure Go ✓, but **does NOT suppress SSL by default**. Fly Postgres sets `sslmode=require` implicitly; ensure `.env` includes it or pooling hangs on non-SSL setup.

Sources: [pgxpool docs](https://pkg.go.dev/github.com/jackc/pgx/v5/pgxpool), [pgx GitHub](https://github.com/jackc/pgx), [PGX v5 Upgrade Notes](https://brandur.org/fragments/pgx-v5-sqlc-upgrade)

---

## 4. redis/go-redis v9

**Latest stable: v9.18.0** (published 2026-02-16). Minimum Go requirement: 1.21. Works with Redis 7.0+ (Fly Redis 7 ✓). **Fly Redis 256MB config**: Set `MaxRetries: 3`, `PoolSize: 5` (conservative for 256MB shared), `IdleTimeout: 300s`. Connection pool uses ~50KB base overhead, safe for 256MB instance.

**Example**:
```go
client := redis.NewClient(&redis.Options{
  Addr: "redis.fly.dev:6379",
  PoolSize: 5,
  MaxRetries: 3,
})
```

**Gotcha**: Fly Redis 7 does NOT persist by default in free tier. For local dev, use `redis:7-alpine` in docker-compose with `--appendonly yes` to enable AOF persistence.

Sources: [redis/go-redis GitHub](https://github.com/redis/go-redis), [pkg.go.dev v9](https://pkg.go.dev/github.com/redis/go-redis/v9), [Redis Releases](https://github.com/redis/go-redis/releases)

---

## 5. goose v3

**Latest stable: v3.x** (published 2026-02-22, exact minor version not specified in releases, likely v3.20+). **CLI vs embed**: For Phase 1, **recommend embedding as Go lib** (`github.com/pressly/goose/v3`) — allows programmatic migrations in app startup, easier to test. CLI (`go install github.com/pressly/goose/v3/cmd/goose@latest`) useful for manual ops.

**Recommended structure**:
```
internal/migrations/
  ├── 20260424_001_init.up.sql
  ├── 20260424_001_init.down.sql
  └── embed.go        // Embed SQL files via go:embed
```

Then in `main.go`:
```go
//go:embed migrations/*.sql
var embedMigrations embed.FS
goose.SetBaseFS(embedMigrations)
goose.Up(db, "migrations")
```

**Gotcha**: goose embedded migrations are **relative to embed root**, not absolute paths. Use `SetBaseFS()` to configure correctly.

Sources: [goose GitHub](https://github.com/pressly/goose), [goose Docs](https://pressly.github.io/goose/), [pkg.go.dev](https://pkg.go.dev/github.com/pressly/goose/v3)

---

## 6. sqlc v1.30+

**Latest stable: v1.30.0** (stable branch as of search date; v1.31.1 available as latest but v1.30 recommended for stability). **YAML v2 config with pgx/v5**:

```yaml
version: "2"
sql:
  - engine: "postgresql"
    queries: "internal/db/queries/*.sql"
    schema: "internal/db/schema.sql"
gen:
  go:
    package: "db"
    sql_package: "pgx/v5"
    out: "internal/db/sqlc"
    emit_pointers_for_null_types: true
```

**nullable UUID gotcha**: If UUID columns are nullable, sqlc by default maps to `pgtype.UUID` (from pgx). To use `*uuid.UUID` (pointer) instead, add `emit_pointers_for_null_types: true`. **Alternative**: Use type override:
```yaml
overrides:
  - db_type: "uuid"
    nullable: true
    go_type:
      import: "github.com/google/uuid"
      type: "*UUID"
```

Per MASTER_PROMPT §3.1 DDL, several FK/campaign UUIDs are nullable; handle via emit_pointers.

**Gotcha**: sqlc **cannot override nullable and non-nullable separately** in v1.30 — requires two override blocks if you need different types for nullable vs non-nullable same base type.

Sources: [sqlc Config Docs v1.30](https://docs.sqlc.dev/en/v1.27.0/reference/config.html), [sqlc pgx Guide](https://docs.sqlc.dev/en/v1.27.0/guides/using-go-and-pgx.html), [sqlc Datatypes](https://docs.sqlc.dev/en/stable/reference/datatypes.html), [GitHub Issue #3712](https://github.com/sqlc-dev/sqlc/issues/3712)

---

## 7. uber-go/zap v1

**Latest stable: v1.27.0** (released 2025-11-19, pre-April 2026). Production config for Fly stdout JSON:

```go
config := zap.NewProductionConfig()
config.OutputPaths = []string{"stdout"}
config.Encoding = "json"
logger, _ := config.Build(
  zap.AddCallerSkip(1),
)
```

This outputs JSON to stdout (captured by Fly logs), no files needed. Zap is 4–10x faster than alternatives; Fly's log aggregation reads stdout JSON natively.

**Gotcha**: Production config enables sampling (drop repeated logs within 1s). Disable if needed via `config.Sampling = nil`.

Sources: [zap GitHub](https://github.com/uber-go/zap), [zap pkg.go.dev](https://pkg.go.dev/go.uber.org/zap), [zap Releases](https://github.com/uber-go/zap/releases)

---

## 8. Fiber Middleware Strategy for Phase 1

**Built-in to enable**: `compress.New()` (gzip), `cors.New()` (set allowed origins), `recover.New()` (panic handler — custom PanicHandler for logging). **Custom impl**:
1. **Logger**: Replace built-in logger with `fiberzap` for structured JSON
2. **RealIP**: Implement custom middleware for Fly proxy headers. Fly injects `Fly-Client-IP` (trusted); if Cloudflare in front, parse `X-Forwarded-For` rightmost. Snippet:
```go
app.Use(func(c fiber.Ctx) error {
  ip := c.Get("Fly-Client-IP")
  if ip == "" {
    ip = c.IP() // fallback to conn IP
  }
  c.Locals("client_ip", ip)
  return c.Next()
})
```

**Gotcha**: Fiber's built-in `logger.New()` with `LogFormat` template is NOT Zap-integrated. Must use `fiberzap` contrib or custom middleware.

Sources: [Fiber Logger Middleware](https://docs.gofiber.io/next/middleware/logger/), [Fly Request Headers](https://fly.io/docs/networking/request-headers/), [fiberzap](https://docs.gofiber.io/contrib/fiberzap/)

---

## 9. Dockerfile Multi-stage

**Confirmed in MASTER_PROMPT §12.3**: 
```dockerfile
FROM golang:1.23-alpine AS builder
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/api ./cmd/api

FROM gcr.io/distroless/static-debian12
COPY --from=builder /out/api /api
ENTRYPOINT ["/api"]
```

**Latest distroless/static-debian12**: No immutable "latest" digest provided in public docs. Use pinned digest or `@sha256:...` from GCR. Image size ~2MB, perfect for Fly (shared-cpu-1x). **CGO_ENABLED=0**: ✓ works with pgx v5 (pure Go), confirmed safe.

**Gotcha**: distroless/static has NO shell, package manager, or libc. pgx pure Go ✓. If future deps need libc, switch to `distroless/base-debian12` (+50MB).

Sources: [distroless GitHub](https://github.com/GoogleContainerTools/distroless), [MASTER_PROMPT §12.3](MASTER_PROMPT.md)

---

## 10. testcontainers-go v0.31+

**Latest**: Confirmed support for Postgres 16 and Redis 7 via `testcontainers.Run()`. Example:
```go
req := testcontainers.ContainerRequest{
  Image: "postgres:16-alpine",
  Env: map[string]string{"POSTGRES_PASSWORD": "test"},
}
postgres, _ := testcontainers.GenericContainer(ctx, tcReq)

req = testcontainers.ContainerRequest{
  Image: "redis:7-alpine",
}
redis, _ := testcontainers.GenericContainer(ctx, tcReq)
```

**Gotcha**: `postgres:16-alpine` has modest image size (~170MB). Redis 7 Alpine is ~40MB. Both pull on first test run; cache locally or pin digests in CI.

Sources: [testcontainers Postgres Module](https://golang.testcontainers.org/modules/postgres/), [testcontainers Redis Module](https://golang.testcontainers.org/modules/redis/), [GitHub Releases](https://github.com/testcontainers/testcontainers-go/releases)

---

## 11. PostgreSQL 16 Extensions (Alpine)

**Available in `postgres:16-alpine` out-of-box**:
- **uuid-ossp**: ✓ (standard extension, no extra packages)
- **pgcrypto**: ✓ (standard, no packages)
- **citext**: ✓ (standard)
- **pg_trgm**: ✓ (standard, no packages)

**Installation in migration**:
```sql
CREATE EXTENSION IF NOT EXISTS uuid_ossp;
CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS citext;
CREATE EXTENSION IF NOT EXISTS pg_trgm;
```

Per MASTER_PROMPT §3.1 DDL, uuid-ossp is used for `uuid_generate_v4()`. No custom image build needed.

Sources: [PostgreSQL uuid-ossp Docs](https://www.postgresql.org/docs/current/uuid-ossp.html), [How to Use PostgreSQL Extensions](https://oneuptime.com/blog/post/2026-01-27-postgresql-extensions/)

---

## 12. Redis 7 Docker Compose (Dev)

**Recommended for local dev**:
```yaml
services:
  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    volumes:
      - redis_data:/data
    command: redis-server --appendonly yes --appendfsync everysec
    environment:
      REDIS_ARGS: "--maxmemory 256mb"
volumes:
  redis_data:
```

Enables **AOF persistence** (append-only file) for durability; `--appendfsync everysec` balances safety vs performance. For RDB snapshots, add `--save 60 1000` (save if 1000 keys changed in 60s). 

**Gotcha**: `redis:7-alpine` has no built-in config file persistence. Use command-line args or mount a `redis.conf`. For Fly.io production, Redis 7 shared (256MB) has no persistence; accept data loss risk or upgrade to dedicated Redis.

Sources: [How to Run Redis in Docker with Persistence](https://oneuptime.com/blog/post/2026-01-25-redis-docker-persistence/), [Redis Docker Hub](https://hub.docker.com/_/redis), [Understanding AOF & RDB](https://medium.com/redis-with-raphael-de-lio/understanding-persistence-in-redis-aof-rdb-on-docker-dcc176ea439)

---

## Critical Gotchas Summary

1. **Go 1.23 is EOL**: Upgrade to 1.26 or accept zero security patches post-April 2026. Phase 1 cannot ship on unsupported Go.

2. **pgx v5 + pgxpool SSL mode**: Fly Postgres requires `sslmode=require`; omitting it hangs connections. Verify in connection string.

3. **sqlc nullable UUID**: `emit_pointers_for_null_types: true` essential for MASTER_PROMPT campaign/target nullable UUIDs; otherwise type mismatch at runtime.

4. **Fiber logger ≠ Zap**: Built-in logger doesn't integrate Zap. Must use `fiberzap` contrib or write custom middleware for structured logging.

5. **distroless has no shell**: If future dependencies require libc (e.g., C bindings), rebuild Dockerfile; `pgx` pure Go is safe now.

---

## Recommended go.mod Dependency List (Phase 1)

```
github.com/gofiber/fiber/v2 v2.52.6
github.com/gofiber/contrib/fiberzap/v2 v2.x.x (check latest on github.com/gofiber/contrib)
github.com/jackc/pgx/v5 v5.9.1
github.com/redis/go-redis/v9 v9.18.0
github.com/pressly/goose/v3 v3.20.0 (or latest v3.x from releases)
go.uber.org/zap v1.27.0
github.com/google/uuid v1.x.x (for UUID overrides in sqlc)
```

Run `go mod tidy` to resolve transitive deps. Lock versions in `go.sum` for reproducible builds.

---

## Unresolved Questions

1. **Exact fiberzap version**: Search did not yield specific latest version. Check [gofiber/contrib releases](https://github.com/gofiber/contrib/releases) for `fiberzap/v2` tag.
2. **goose v3 exact latest minor**: Release was Feb 2026 but specific v3.X.Y not stated in public releases. Recommend checking [GitHub releases](https://github.com/pressly/goose/releases).
3. **distroless/static-debian12 latest digest**: No immutable "latest" SHA256 in public docs. GCR provides digest per pull; use `crane digest gcr.io/distroless/static-debian12:latest` or pin in CI.
4. **sqlc v1.31.1 vs v1.30.0 stability**: v1.31.1 available but v1.30.0 recommended. Assess changelog impact before upgrade.
