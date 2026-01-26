# Production Deployment Guide

This guide covers a production-ready Clubhouse deployment with TLS, backups, observability, and rollback steps.

## 1) Prerequisites

- Docker Engine + Docker Compose v2
- A public domain with DNS records pointing to your server
- SSH access to the host

## 2) Create the production environment file

Copy the template and fill in all values:

```bash
cp .env.production.example .env.production
```

Required environment values:

- `CLUBHOUSE_APP_DOMAIN`, `CLUBHOUSE_GRAFANA_DOMAIN`, `ACME_EMAIL`
- `WS_ORIGIN_ALLOWLIST` (set to your frontend origin, e.g. `https://clubhouse.example.com`)
- `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB`
- `REDIS_PASSWORD`
- `GF_SECURITY_ADMIN_USER`, `GF_SECURITY_ADMIN_PASSWORD`, `GF_SERVER_ROOT_URL`

Bootstrap (first run only):

- `CLUBHOUSE_BOOTSTRAP_ADMIN_USERNAME`, `CLUBHOUSE_BOOTSTRAP_ADMIN_PASSWORD`
- Optional: `CLUBHOUSE_BOOTSTRAP_ADMIN_EMAIL`

Optional environment values (override defaults or enable features):

- `OTEL_SERVICE_NAME`, `OTEL_SERVICE_VERSION` (defaults set in `docker-compose.prod.yml`)
- `CLUBHOUSE_TOTP_ENCRYPTION_KEY` (base64-encoded 32-byte key for admin TOTP)
- `BACKUP_DIR`, `BACKUP_RETENTION_DAYS` (only needed if you run `scripts/backup-postgres.sh`)

Required secrets to generate (do not use defaults):

- `POSTGRES_PASSWORD`
- `REDIS_PASSWORD`
- `GF_SECURITY_ADMIN_PASSWORD`

Generate strong values:

```bash
openssl rand -base64 32
```

Set your public domains and ACME email:

- `CLUBHOUSE_APP_DOMAIN=clubhouse.example.com`
- `CLUBHOUSE_GRAFANA_DOMAIN=grafana.example.com`
- `ACME_EMAIL=admin@example.com`

## 3) Database setup

The production compose file runs Postgres in a container and the backend entrypoint
automatically runs migrations on startup. The first admin is created via the
bootstrap flow (see below) and no default admin credentials are shipped.
You do not need to run migrations manually for the default setup.

If you use an external Postgres instance:

- Point the backend to it by setting `POSTGRES_HOST` and `POSTGRES_PORT` in
  `docker-compose.prod.yml` (or by overriding those env vars at deploy time).
- Ensure the database and user from `.env.production` already exist.
- Keep `sslmode` requirements in mind (the default entrypoint uses `sslmode=disable`).

## 4) Build and tag production images

Build and tag images on the deploy host or in CI, then push to your registry.

```bash
docker build -t clubhouse-backend:2026-01-22 ./backend
docker build -t clubhouse-frontend:2026-01-22 ./frontend
```

Optional: push to a registry and set these in `.env.production`:

```
BACKEND_IMAGE=ghcr.io/your-org/clubhouse-backend:2026-01-22
FRONTEND_IMAGE=ghcr.io/your-org/clubhouse-frontend:2026-01-22
```

## 5) Start the production stack

```bash
docker compose -f docker-compose.prod.yml --env-file .env.production up -d
```

Verify status:

```bash
docker compose -f docker-compose.prod.yml ps
```

## 6) Bootstrap the first admin

Clubhouse does not ship default admin credentials. On first startup (when no admin exists), set the bootstrap credentials via env or CLI flags so the server can create the initial admin user.

Using `.env.production`:

```
CLUBHOUSE_BOOTSTRAP_ADMIN_USERNAME=admin
CLUBHOUSE_BOOTSTRAP_ADMIN_PASSWORD=ChangeMe123
CLUBHOUSE_BOOTSTRAP_ADMIN_EMAIL=admin@example.com
```

Or with CLI flags:

```
./clubhouse-server   --bootstrap-admin-username=admin   --bootstrap-admin-password=ChangeMe123   --bootstrap-admin-email=admin@example.com
```

After the admin is created, remove the bootstrap values and restart. The bootstrap flow is idempotent and skips if an admin already exists.

## 7) TLS and reverse proxy

TLS is terminated by Caddy using `Caddyfile` and the `CLUBHOUSE_APP_DOMAIN` / `CLUBHOUSE_GRAFANA_DOMAIN` values.

- App URL: `https://<CLUBHOUSE_APP_DOMAIN>`
- Grafana URL: `https://<CLUBHOUSE_GRAFANA_DOMAIN>`

Caddy forwards `/api` and `/health` to the backend and everything else to the frontend.

## 8) Secure internal services

The production compose file does not publish ports for Postgres, Redis, Grafana, Loki, Tempo, or Prometheus. Access is only via the internal Docker network or the Caddy reverse proxy.

## 9) Secret management

- Store `.env.production` in a secrets manager (1Password, Vault, SSM Parameter Store, etc.) and
  lock down file permissions on the host.
- Rotate `POSTGRES_PASSWORD`, `REDIS_PASSWORD`, and `GF_SECURITY_ADMIN_PASSWORD` regularly.
- Avoid checking secrets into source control or CI logs; use CI secret injection instead.

## 10) Persistence and backups

All stateful services use named volumes:

- Postgres: `postgres_data`
- Redis: `redis_data`
- Grafana: `grafana_data`
- Loki: `loki_data`
- Prometheus: `prometheus_data`
- Tempo: `tempo_data`
- Caddy: `caddy_data`

### Automated Postgres backups

Use the included script and schedule it with cron:

```bash
./scripts/backup-postgres.sh
```

Example cron (daily at 02:00 UTC):

```
0 2 * * * cd /opt/clubhouse && ./scripts/backup-postgres.sh
```

`BACKUP_DIR` and `BACKUP_RETENTION_DAYS` are read from `.env.production`.
The backup/restore scripts source `.env.production` automatically when present.

### Restore procedure

```bash
./scripts/restore-postgres.sh /path/to/backup.sql.gz
```

## 11) Scaling considerations

- **Backend**: You can scale horizontally by running multiple backend containers and
  putting Caddy (or another load balancer) in front. Sessions are Redis-backed, and
  WebSocket events are distributed through Redis pub/sub.
- **Postgres**: For larger communities, move Postgres to a managed service and
  provision enough CPU/RAM/IOPS. Backups become the database provider's responsibility.
- **Redis**: Start with a single instance; upgrade to a managed Redis or Redis cluster
  if session storage or pub/sub throughput becomes a bottleneck.

## 12) Observability

- Traces: OTLP gRPC -> Tempo (`OTEL_EXPORTER_OTLP_ENDPOINT=tempo:4317`)
- Logs: OTLP HTTP -> Loki (`OTEL_EXPORTER_OTLP_LOGS_ENDPOINT=http://loki:3100/otlp/v1/logs`)
- Metrics: `/metrics` scraped by Prometheus

Grafana dashboards are provisioned from `grafana/dashboards`.

Pinned observability image versions (see `docker-compose.yml` and `docker-compose.prod.yml`):

- Grafana: `grafana/grafana:11.2.0`
- Loki: `grafana/loki:3.0.1`
- Prometheus: `prom/prometheus:2.54.1`
- Tempo: `grafana/tempo:2.6.1`

## 13) Security hardening checklist

- Bootstrap the first admin via env/CLI and remove the bootstrap values once created.
- Confirm Caddy is setting `X-Forwarded-Proto: https` so secure cookies are issued.
- Sessions are Redis-backed (no JWT secret today). If you add JWT auth later, ensure the signing key is unique and rotated.
- Keep frontend and backend on the same origin to avoid permissive CORS. If you add cross-origin access, implement a strict allowlist.
- Keep the deployment host firewall locked down (only 80/443 open).
- Ensure `.env.production` is stored securely and never committed.
- Review environment values for production: `ENVIRONMENT=production`, `LOG_LEVEL=info`.
- Use a least-privilege database user for the app (separate admin/maintenance credentials).

## 14) Health checks and uptime monitoring

The backend exposes `GET /health`. Caddy forwards `/health` to the backend, so you can monitor:

```
https://<CLUBHOUSE_APP_DOMAIN>/health
```

Use your uptime monitor of choice (Grafana Synthetic Monitoring, Uptime Kuma, or a managed service).

## 15) Rollback plan

1. Keep prior image tags (e.g., `clubhouse-backend:2026-01-21`).
2. Update `.env.production` to point to the previous image tags.
3. Redeploy:

```bash
docker compose -f docker-compose.prod.yml --env-file .env.production up -d
```

4. Verify `/health` and logs:

```bash
docker compose -f docker-compose.prod.yml logs -f backend
```

## 16) Troubleshooting

- Check logs: `docker compose -f docker-compose.prod.yml logs -f`
- Verify containers are healthy: `docker compose -f docker-compose.prod.yml ps`
