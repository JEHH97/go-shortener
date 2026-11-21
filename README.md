# Go Shortener

A small, production-minded URL shortener that keeps redirect handling fast while recording useful aggregate click signals asynchronously. It pairs a JSON API with an embedded web UI and local observability stack.

Repository: [github.com/JEHH97/go-shortener](https://github.com/JEHH97/go-shortener)

**Demo:** _coming soon_

## The problem

Long campaign and article URLs are awkward to share, and teams often need a lightweight view of redirect activity without putting analytics work on the critical redirect path. This project offers compact links, protected creation and deletion surfaces, basic click aggregates, and an inspectable local stack.

## Architecture

```mermaid
flowchart LR
  Browser[Browser / client] --> API[Go HTTP API + embedded UI]
  API --> Validate[URL policy and alias validation]
  API --> Redis[(Redis cache + sliding-window limiter)]
  API --> Postgres[(PostgreSQL links + click aggregates)]
  API --> Queue[Buffered click channel]
  Queue --> Workers[Batching worker pool]
  Workers --> Postgres
  API --> Metrics[/Prometheus metrics/]
  Prometheus[Prometheus] --> Metrics
  Grafana[Grafana dashboard] --> Prometheus
```

## Stack

- Go, `net/http`, chi router, `slog`, embedded HTML.
- PostgreSQL with embedded, idempotent SQL migration; pgx connection pool.
- Redis with go-redis for cache-aside lookups and an atomic Lua rate limiter.
- Prometheus client instrumentation; Prometheus and provisioned Grafana in Compose.
- Docker multi-stage build with a distroless non-root runtime.

## Technical decisions

- **Async click pipeline:** redirects enqueue a compact event into a bounded channel. A configurable worker pool batches inserts, leaving database aggregation writes off the redirect response path. Queue overload is logged and drops only analytics events. Shutdown closes the channel and workers flush their pending batches within the graceful shutdown window.
- **Rate limiter:** a Redis Lua script implements a per-client-IP sliding-window log using a sorted set. The script prunes old timestamps, counts the active window, and admits or rejects atomically across app instances. Rejections include `Retry-After`.
- **Demo safeguards:** link creation is rate-limited to 10 requests per client IP per minute in the hosted blueprint (the application default is also 10/minute). Destination URLs are capped at 2,048 bytes. Public `/metrics` contains aggregate service counters and latency only; per-link click aggregates at `/api/links/{code}/stats` are public to anyone who knows the short code. Use the protected delete endpoint to remove links (see [Deployment](DEPLOY.md)).
- **SSRF and open-redirect safeguards:** only HTTP(S) URLs without embedded credentials are accepted. URL length is capped; localhost suffixes and private, loopback, link-local, unspecified, and multicast IPs are rejected. Hostnames are resolved at creation and rejected if any answer is non-public. This reduces abuse risk but cannot eliminate DNS rebinding after validation; use network egress controls for stronger isolation.
- **Cache-aside:** redirects check Redis then PostgreSQL; successful database reads populate cache with an expiry-aware TTL. Link deletion invalidates the cache.
- **Analytics privacy:** country is read only from `CF-IPCountry` or `X-Country` when provided; referer and coarse user-agent family are aggregated. No IP address is stored with click events.

## Run locally

Docker Compose is the one-command local environment (app, PostgreSQL, Redis, Prometheus, and Grafana):

```sh
docker compose up --build
```

Then visit `http://localhost:8080`, metrics at `http://localhost:8080/metrics`, Prometheus at `http://localhost:9090`, and Grafana at `http://localhost:3000` (local login `admin` / `local-demo-password`). The compose credentials are for local development only. Stop and remove local data with `docker compose down -v`.

To configure a local process, copy `.env.example` to `.env` and supply development values; `.env*` files are ignored. Compose accepts `API_KEY` and `BASE_URL` overrides from the environment.

### API

| Method | Path | Purpose |
|---|---|---|
| `POST` | `/api/links` | Create from `{"url":"https://example.org","alias":"optional","expiry":"2030-01-01T00:00:00Z"}`. Returns `code` and `short_url`. |
| `GET` | `/{code}` | Redirect (302), 404 when absent, 410 when expired. |
| `GET` | `/api/links/{code}/stats` | Total clicks, daily counts, top referers, country, and browser-family aggregates. |
| `DELETE` | `/api/links/{code}` | Delete with `Authorization: Bearer <API_KEY>`. |
| `GET` | `/healthz` | Liveness response. |
| `GET` | `/metrics` | Public aggregate redirect totals and latency histogram, rate-limited count, and cache-hit ratio; contains no URL or client-IP labels. |

Create requests are rate limited by client IP. The server uses the peer address rather than trusting arbitrary forwarded-IP headers.

## Checks and benchmark

The repository is formatted, vetted, linted, tested, benchmarked, and scanned for prohibited terms with one command:

```sh
make check
```

If Go is unavailable on the host, `make check` runs the Go toolchain in Docker using the project wrapper and non-root UID/GID bind mounts. To run compose-backed integration tests after `docker compose up -d postgres redis`, use:

```sh
INTEGRATION_DATABASE_URL='postgres://shortener:shortener@postgres:5432/shortener?sslmode=disable' \
INTEGRATION_REDIS_ADDR=redis:6379 GO_DOCKER_NETWORK=go-shortener_default \
./scripts/in-docker test
```

The optional integration tests skip unless those environment variables are present. Unit coverage for `internal/domain` is enforced at 70% or higher. Benchmark target is the in-process redirect handler (`go test ./internal/httpapi -run '^$' -bench BenchmarkRedirectPath`).

**Benchmark result (local run, Intel Core Ultra 7 265H):** 2,868 ns/op, 7,628 B/op, 27 allocs/op for the in-process handler path; `make check` prints fresh `ns/op`, `B/op`, and `allocs/op` values. This harness uses an in-memory backing store and is not an end-to-end network benchmark.

## Screenshots

![Embedded UI with a newly created link and click statistics](docs/ui.png)

![Provisioned Grafana dashboard](docs/grafana.png)

## Deployment

See [DEPLOY.md](DEPLOY.md) and the reference `render.yaml`. This project intentionally contains no deploy automation.

## License

MIT. See [LICENSE](LICENSE).
