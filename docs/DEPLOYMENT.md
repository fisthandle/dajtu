# Dajtu Deployment Guide

## Resource Limits

### Disk Storage
- **Max storage:** 50 GB (configurable via `MAX_DISK_GB`)
- **Cleanup trigger:** When usage exceeds `MAX_DISK_GB`
- **Cleanup target:** 45 GB (configurable via `CLEANUP_TARGET_GB`)
- **Strategy:** Deletes oldest images first (FIFO by `created_at`)
- **Interval:** Every 5 minutes

### CPU & Memory (Docker - Production only)
- **CPU limit:** 2 cores
- **Memory limit:** 2 GB
- **Memory reservation:** 256 MB

### Network Bandwidth (Production only)
- **Limit:** 10 Mbps
- **Method:** Traffic control (`tc`) on host
- **Setup:** Run `sudo ./scripts/setup-network-limit.sh` after container starts

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | 8080 | HTTP server port |
| `DATA_DIR` | ./data | Storage directory |
| `MAX_FILE_SIZE_MB` | 20 | Max single file upload size |
| `MAX_DISK_GB` | 50 | Disk usage limit (triggers cleanup) |
| `CLEANUP_TARGET_GB` | 45 | Target size after cleanup |
| `BASE_URL` | (empty) | Public URL for generated links |
| `KEEP_ORIGINAL_FORMAT` | true | Keep original image format |

## Configuration Files

| File | Purpose |
|------|---------|
| `docker-compose.yml` | Local development (10GB disk, localhost) |
| `docker-compose.prod.yml` | Production overrides (50GB, CPU/RAM limits) |

## Local vs Production

| Setting | Local | Production |
|---------|-------|------------|
| BASE_URL | http://localhost:8080 | https://dajtu.com |
| MAX_DISK_GB | 10 GB | 50 GB |
| CLEANUP_TARGET_GB | 8 GB | 45 GB |
| CPU limit | none | 2 cores |
| Memory limit | none | 2 GB |
| Network limit | none | 10 Mbps |
| Port exposure | 8080:8080 | via reverse proxy |

## Quick Start

### Local Development

```bash
docker compose up -d
docker compose logs -f
```

Access at http://localhost:8080

### Production

```bash
# Build and run with production config
docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d

# Apply network limit (requires root)
sudo ./scripts/setup-network-limit.sh

# Check logs
docker compose logs -f
```

## Monitoring

### Check disk usage

```bash
du -sh ./data/
```

### Check container resource usage

```bash
docker stats dajtu_app
```

### Check cleanup daemon activity

```bash
docker compose logs -f | grep -i cleanup
```

## Troubleshooting

### Container uses too much disk
- Verify `MAX_DISK_GB` is set correctly
- Check if cleanup daemon is running (logs every 5 min)
- Manually trigger cleanup by restarting container

### Network limit not working
- Ensure script runs as root: `sudo ./scripts/setup-network-limit.sh`
- Verify container is running before applying limit
- Check with: `tc qdisc show`

### Config validation
- `CLEANUP_TARGET_GB` must be less than `MAX_DISK_GB`
- If invalid, auto-corrects to 90% of `MAX_DISK_GB`
