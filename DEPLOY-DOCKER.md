# Deploy the Docker demo

This guide runs the prebuilt `ghcr.io/jehh97/go-shortener` image with PostgreSQL and Redis on a server you control. The application listens on container port `8080`; only its port is published. PostgreSQL and Redis remain on the private Compose network.

## Images and runtime settings

The publishing workflow creates `vX.Y.Z` and `sha-<short-commit>` tags; it does not publish a floating `latest` tag. The default release is `v1.0.1`. The image is built for `linux/amd64`, runs as a non-root user, and has a Docker healthcheck against `/healthz`.

If the GHCR package is private, authenticate on the deployment host with a GitHub PAT that has `read:packages`:

```sh
echo "$GHCR_READ_TOKEN" | docker login ghcr.io -u YOUR_GITHUB_USER --password-stdin
```

Keep the token outside shell history and revoke it when no longer needed. Public packages can be pulled anonymously.

Copy and edit the environment example. Generate each secret locally with `openssl rand -hex 32`; use separate values for the API key, PostgreSQL, and Redis. The `DATABASE_URL` and `REDIS_URL` passwords must match the corresponding service password. Hex secrets need no URL escaping.

```sh
cp deploy/.env.example deploy/.env
openssl rand -hex 32 # repeat separately for API_KEY, POSTGRES_PASSWORD, REDIS_PASSWORD
```

Set `BASE_URL` to the public HTTPS origin without a trailing slash. `RATE_LIMIT` and `RATE_WINDOW` control link creation limits (defaults: 10 and `1m`). `APP_BIND_ADDRESS` and `APP_PORT` control the host binding (defaults `127.0.0.1` and `8080`); the app's internal port remains `8080`.

Start the services:

```sh
docker compose --env-file deploy/.env -f deploy/docker-compose.prod.yml up -d
docker compose --env-file deploy/.env -f deploy/docker-compose.prod.yml ps
```

The application healthcheck requires `/healthz` to return HTTP 200. The app, PostgreSQL, and Redis have read-only root filesystems, dropped capabilities, no-new-privileges, and resource limits. PostgreSQL and Redis have readiness healthchecks; neither publishes a host port. Use `docker compose ... logs -f app` to inspect startup.

## TLS reverse proxy

Terminate TLS at a reverse proxy and point it at the private app port. The application binds to loopback by default (`APP_BIND_ADDRESS=127.0.0.1`). For Caddy, use:

```caddyfile
short.example.com {
    reverse_proxy 127.0.0.1:8080
}
```

Allow inbound HTTPS only at the proxy/firewall. Set `BASE_URL=https://short.example.com` so generated short URLs use the public origin.

## Verify the demo

Replace the hostname as appropriate. Health should return `{"status":"ok"}`:

```sh
curl -fsS https://short.example.com/healthz
curl -i -X POST https://short.example.com/api/links \
  -H 'Content-Type: application/json' \
  --data '{"url":"https://example.org/"}'
```

Use the returned `code` to check the redirect and aggregate statistics:

```sh
curl -i https://short.example.com/CODE
curl -fsS https://short.example.com/api/links/CODE/stats
```

The root path serves the embedded UI, and `/metrics` exposes aggregate service metrics. Link creation is public but rate-limited; anyone who knows a code can follow its redirect and read its click aggregates. Deletion requires `Authorization: Bearer <API_KEY>`.

## Public-demo security notes

- Use unique generated secrets and keep `deploy/.env` private; never commit it, put secrets in image build arguments, or share them in logs. The image contains no runtime credentials.
- The demo is public and not intended for sensitive destinations or production workloads. Keep the rate limit enabled, monitor abuse, and delete unwanted links.
- Public link creation is protected by URL validation and DNS checks, but those checks cannot fully prevent DNS rebinding; apply outbound network controls if stronger isolation is needed.
- Restrict administrative access and server ports. Only the reverse proxy should be internet-facing; do not expose database or Redis ports.
- Back up PostgreSQL data and use TLS database connections when PostgreSQL is remote. The included Compose deployment is intended for same-host services.

Stop without deleting data with `docker compose --env-file deploy/.env -f deploy/docker-compose.prod.yml down`. Add `-v` only when intentionally deleting database and Redis volumes.
