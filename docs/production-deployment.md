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
- `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB`
- `REDIS_PASSWORD`
- `GF_SECURITY_ADMIN_USER`, `GF_SECURITY_ADMIN_PASSWORD`, `GF_SERVER_ROOT_URL`

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

## 3) Build and tag production images

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

## 4) Start the production stack

```bash
docker compose -f docker-compose.prod.yml --env-file .env.production up -d
```

Verify status:

```bash
docker compose -f docker-compose.prod.yml ps
```

## 5) TLS and reverse proxy

TLS is terminated by Caddy using `Caddyfile` and the `CLUBHOUSE_APP_DOMAIN` / `CLUBHOUSE_GRAFANA_DOMAIN` values.

- App URL: `https://<CLUBHOUSE_APP_DOMAIN>`
- Grafana URL: `https://<CLUBHOUSE_GRAFANA_DOMAIN>`

Caddy forwards `/api` and `/health` to the backend and everything else to the frontend.

## 6) Secure internal services

The production compose file does not publish ports for Postgres, Redis, Grafana, Loki, Tempo, or Prometheus. Access is only via the internal Docker network or the Caddy reverse proxy.

## 7) Persistence and backups

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

## 8) Observability

- Traces: OTLP gRPC -> Tempo (`OTEL_EXPORTER_OTLP_ENDPOINT=tempo:4317`)
- Logs: OTLP HTTP -> Loki (`OTEL_EXPORTER_OTLP_LOGS_ENDPOINT=http://loki:3100/otlp/v1/logs`)
- Metrics: `/metrics` scraped by Prometheus

Grafana dashboards are provisioned from `grafana/dashboards`.

## 9) Security hardening checklist

- Replace the default admin password on first login. The seed user is created from `backend/migrations/seed_admin.sql`.
- Confirm Caddy is setting `X-Forwarded-Proto: https` so secure cookies are issued.
- Sessions are Redis-backed (no JWT secret today). If you add JWT auth later, ensure the signing key is unique and rotated.
- Keep frontend and backend on the same origin to avoid permissive CORS. If you add cross-origin access, implement a strict allowlist.
- Keep the deployment host firewall locked down (only 80/443 open).
- Ensure `.env.production` is stored securely and never committed.
- Review environment values for production: `ENVIRONMENT=production`, `LOG_LEVEL=info`.
- Use a least-privilege database user for the app (separate admin/maintenance credentials).

## 10) Health checks and uptime monitoring

The backend exposes `GET /health`. Caddy forwards `/health` to the backend, so you can monitor:

```
https://<CLUBHOUSE_APP_DOMAIN>/health
```

Use your uptime monitor of choice (Grafana Synthetic Monitoring, Uptime Kuma, or a managed service).

## 11) Rollback plan

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

## 12) Troubleshooting

- Check logs: `docker compose -f docker-compose.prod.yml logs -f`
- Verify containers are healthy: `docker compose -f docker-compose.prod.yml ps`
