# Deploy the demo on Neon and Render

For deployment on a server with your own Docker-hosted PostgreSQL and Redis, see [DEPLOY-DOCKER.md](DEPLOY-DOCKER.md).

This walkthrough deploys the Go API and its Redis cache on Render's free plans, with PostgreSQL hosted by Neon. It does not configure a custom domain or run any deployment automatically from this repository.

## Secrets and generated values

- `API_KEY`: Render generates this in the blueprint (`generateValue: true`). If you need to supply or rotate it yourself, generate it locally with `openssl rand -hex 32` and enter the result only in the Render Web Service environment settings. This key is required to delete a link.
- `DATABASE_URL`: contains the Neon database password. Copy it from Neon's **Connect** dialog into Render; do not generate or commit a replacement value.
- `REDIS_URL`: supplied from the Render Key Value service's internal connection string; do not create or expose a separate Redis password for this blueprint.

## 1. Create the Neon database

1. Create a Neon project and a database named `shortener` (or choose another name and use it consistently in the URL).
2. In the Neon project dashboard, open **Connect** and copy the PostgreSQL connection string for the database and role you created. Choose the pooled connection option if available.
3. Confirm the value is a PostgreSQL URL with TLS required. Its format is:

   ```text
   postgresql://USER:PASSWORD@HOST/DATABASE?sslmode=require
   ```

   A concrete-shaped example is `postgresql://shortener:YOUR_NEON_PASSWORD@ep-example.neon.tech/shortener?sslmode=require`. Use Neon's actual copied host, role, password, and database name. Preserve `sslmode=require`; URL-encode special characters in credentials if needed. This value is supplied as `DATABASE_URL` and is not committed to the repository.

## 2. Create the Render services

1. In Render, choose **New → Blueprint** and connect the published `go-shortener` repository.
2. Review the blueprint. It defines one Docker **Web Service** (`go-shortener`) on the **Free** plan and one **Key Value** service (`go-shortener-redis`) on the **Free** plan. The Key Value service has an empty `ipAllowList` so the app can reach it over Render's private network only.
3. For the Web Service's `DATABASE_URL` environment field, paste the Neon connection string from step 1. Do not create a Render Postgres database.
4. Set `BASE_URL` to the public URL Render assigns to the Web Service, in the form `https://go-shortener.onrender.com` (use the actual service hostname shown in its dashboard). This must be the origin visitors will use; do not include a trailing slash.
5. The blueprint wires `REDIS_URL` to the Key Value service's internal `connectionString`. Do not replace it with an external Redis address. The application accepts `redis://` and `rediss://` URLs; it also retains `REDIS_ADDR` plus `REDIS_PASSWORD` for older/self-managed installations.

   **Shared Key Value instance.** Render's free plan allows one Key Value instance per workspace, so `go-shortener-redis` is shared with `checkout-service`. This service owns the instance and uses the default database index `0` (no path in `REDIS_URL`). `checkout-service` uses database index `1` on the same instance. Do not delete, rename, or recreate `go-shortener-redis` without also updating `REDIS_URL` in `checkout-service`. go-shortener keys are prefixed `shortener:`, so they do not collide with checkout's keys.
6. `API_KEY` is marked `generateValue: true` in the blueprint, so Render generates a value. If you choose to set or rotate it manually instead, generate a unique key locally with `openssl rand -hex 32`, then save it in the Render Web Service's `API_KEY` environment field. Never put the generated value in Git or a public issue.
7. Leave the blueprint's `RATE_LIMIT=10` and `RATE_WINDOW=1m` in place for the public demo. Deploy the blueprint from the Render dashboard and wait for `/healthz` to become healthy.

The app runs its idempotent database migration at startup. `DATABASE_URL` must be a Neon PostgreSQL URI with `sslmode=require`; the application does not need a separate schema setup command.

## 3. Verify the demo

Replace `https://go-shortener.onrender.com` below with the actual Render service URL.

1. Check health: `curl -i https://go-shortener.onrender.com/healthz`. Expect HTTP 200 and `{"status":"ok"}`.
2. Open `https://go-shortener.onrender.com/` in a browser to load the embedded UI.
3. Create a link with `curl -i -X POST https://go-shortener.onrender.com/api/links -H 'Content-Type: application/json' --data '{"url":"https://example.org/"}'`. Expect HTTP 201 and a JSON response containing `code` and `short_url`.
4. Visit the returned `short_url`; it should redirect to `https://example.org/`. Check click aggregates at `https://go-shortener.onrender.com/api/links/CODE/stats`, substituting the returned code.
5. Check the public, aggregate-only metrics endpoint at `https://go-shortener.onrender.com/metrics`.

### Remove a link

Link creation is public but rate-limited (10 requests per minute per request IP). Destination URLs are limited to 2,048 bytes and must pass the HTTP(S), DNS, and private-address checks. Short codes are unguessable by design, but do not use this demo for sensitive destinations.

To delete a link, send the generated API key as a bearer token. Copy it from the Render service's environment settings without putting it in shell history or shared logs, then run:

```sh
read -rsp 'Render API_KEY: ' API_KEY; echo
curl -i -X DELETE "https://go-shortener.onrender.com/api/links/CODE" \
  -H "Authorization: Bearer ${API_KEY}"
unset API_KEY
```

Expect HTTP 204. `/metrics` is public and contains only service-level counters, latency, and cache-hit ratio (no URL or client-IP labels). `/api/links/{code}/stats` is also public; anyone who knows a short code can read its aggregate click data. The API key protects deletion, not statistics. Remove links you no longer want by code as shown above; the app does not currently have an automatic demo-link cleanup schedule.

## Free-plan behavior

Render's free Web Service sleeps after 15 minutes without traffic. The first request after sleeping can take about 50 seconds while it cold-starts, so allow that request to finish and retry if needed. Keep this in mind when sharing the demo. Check the current provider terms before relying on free-plan limits.

## Local Compose check

The local Compose app uses the same `REDIS_URL` configuration path as the hosted blueprint:

```sh
docker compose up --build
curl -i http://localhost:8080/healthz
docker compose down
```

Use `docker compose down -v` only when you also want to remove local PostgreSQL and Redis data.
