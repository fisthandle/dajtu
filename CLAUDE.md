# Dajtu - Image Hosting Service

## Deploy

```bash
# Commit + push + deploy
ssh staging "cd /var/www/dajtu && git pull && docker compose up --build -d"

# Tylko restart (bez rebuild)
ssh staging "cd /var/www/dajtu && docker compose up -d --force-recreate"

# Logi
ssh staging "docker logs dajtu_app --tail 100 -f"
```

**Serwer:** staging (SSH config)
**Ścieżka:** `/var/www/dajtu`
**URL:** https://dajtu.com

## Stack

- Go 1.22
- SQLite (data/dajtu.db)
- libvips (przetwarzanie obrazów)
- Docker + Caddy (reverse proxy w /var/www/infra)

## Konfiguracja produkcyjna

Env vars w `/var/www/dajtu/docker-compose.yml`:
- `BASE_URL=https://dajtu.com`
- `MAX_DISK_GB=50`
- `BRAT_*` - klucze SSO Braterstwo
