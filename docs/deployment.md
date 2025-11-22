# Deployment Guide: PIM System

## Overview

The PIM System is designed to be deployed as a containerized application. It requires a PostgreSQL database and can run on any container orchestration platform (Docker Compose, Kubernetes, ECS, etc.).

## Prerequisites

- **Docker**: To build and run the application container
- **PostgreSQL 15+**: Database with `pg_trgm` extension enabled
- **Storage**: Persistent volume for local filesystem storage (MVP) or S3 credentials (Future)

## Configuration

The application is configured via environment variables.

| Variable | Description | Default | Required |
|----------|-------------|---------|:--------:|
| `PORT` | HTTP server port | `8080` | No |
| `DATABASE_URL` | PostgreSQL connection string | `postgres://user:pass@host:5432/pim?sslmode=disable` | **Yes** |
| `LOG_LEVEL` | Logging level (debug, info, warn, error) | `info` | No |
| `ENVIRONMENT` | Environment name (development, production) | `production` | No |

### Database Configuration

The database user must have permissions to:
1. Create/alter tables (for auto-migration)
2. Create indexes
3. Enable extensions (`pg_trgm` for search)

## Building the Container

```bash
# Build the Docker image
docker build -t pim-api:latest .
```

## Running with Docker Compose (Recommended)

Create a `docker-compose.yml` file:

```yaml
version: '3.8'

services:
  api:
    image: pim-api:latest
    ports:
      - "8080:8080"
    environment:
      - DATABASE_URL=postgres://pim:pimpassword@db:5432/pim?sslmode=disable
      - LOG_LEVEL=info
    depends_on:
      - db
    volumes:
      - pim-assets:/var/pim/assets

  db:
    image: postgres:15-alpine
    environment:
      - POSTGRES_USER=pim
      - POSTGRES_PASSWORD=pimpassword
      - POSTGRES_DB=pim
    volumes:
      - postgres-data:/var/lib/postgresql/data

volumes:
  postgres-data:
  pim-assets:
```

Start the services:

```bash
docker-compose up -d
```

## Health Checks

The application exposes a health check endpoint at `/health`.

```bash
curl http://localhost:8080/health
# Response: {"status":"healthy", ...}
```

## Production Considerations

1. **Database Backups**: Ensure automated backups for the PostgreSQL database.
2. **Asset Backup**: Backup the `pim-assets` volume or configure S3 storage (when available).
3. **Security**:
   - Run behind a reverse proxy (Nginx, Traefik, Load Balancer) for TLS termination.
   - Restrict database access to the application container only.
   - Use strong passwords for database credentials.
4. **Monitoring**:
   - Configure OpenTracing collector (Jaeger/Zipkin) if distributed tracing is needed.
   - Monitor logs for errors and performance issues.

## Troubleshooting

### Database Connection Failed

If the application fails to start with "failed to connect to database":
- Verify `DATABASE_URL` is correct.
- Ensure the database container is running and accessible.
- Check firewall rules allowing traffic on port 5432.

### Migrations Failed

If migrations fail:
- Check if the database user has sufficient permissions.
- Check logs for specific SQL errors.

### Asset Upload Failed

If asset uploads fail:
- Check volume permissions for `/var/pim/assets`.
- Ensure sufficient disk space.
- Check file size limits (10MB for images, 100MB for videos).

