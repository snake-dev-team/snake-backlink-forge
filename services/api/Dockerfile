# ── Stage 1: Builder ──────────────────────────────────────────────────────────
# golang:1.26-alpine provides a minimal build environment.
# apk add git build-base: required by some CGO-less transitive deps that use
# go:generate or embed; safe to include even with CGO_ENABLED=0.
FROM golang:1.26-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git build-base

# Cache dependency downloads in a separate layer to speed up rebuilds.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGO_ENABLED=0 produces a fully static binary compatible with distroless/static.
# -ldflags='-s -w' strips debug info and DWARF tables (~30% size reduction).
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags='-s -w' -o /out/api ./cmd/api

# ── Stage 2: Runtime ──────────────────────────────────────────────────────────
# distroless/static-debian12:nonroot has no shell, no package manager, no libc.
# Target image size: ~2.5 MB base + ~15-20 MB Go binary = well under 25 MB [M2].
FROM gcr.io/distroless/static-debian12:nonroot

# Copy only the compiled binary from the builder stage.
COPY --from=builder /out/api /api

EXPOSE 8080

# Run as nonroot (uid 65532) — no privileged operations needed.
USER nonroot:nonroot

ENTRYPOINT ["/api"]
